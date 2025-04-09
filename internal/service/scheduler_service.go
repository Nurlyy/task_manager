package service

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/messaging"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/pkg/config"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// SchedulerService представляет сервис планировщика задач
type SchedulerService struct {
	taskRepo          repository.TaskRepository
	userRepo          repository.UserRepository
	projectRepo       repository.ProjectRepository
	notificationRepo  repository.NotificationRepository
	producer          *messaging.KafkaProducer
	cron              *cron.Cron
	logger            logger.Logger
	config            *config.SchedulerConfig
}

// NewSchedulerService создает новый экземпляр сервиса планировщика
func NewSchedulerService(
	taskRepo repository.TaskRepository,
	userRepo repository.UserRepository,
	projectRepo repository.ProjectRepository,
	notificationRepo repository.NotificationRepository,
	producer *messaging.KafkaProducer,
	config *config.SchedulerConfig,
	logger logger.Logger,
) *SchedulerService {
	// Создаем планировщик с поддержкой секунд
	cronScheduler := cron.New(cron.WithSeconds())

	return &SchedulerService{
		taskRepo:          taskRepo,
		userRepo:          userRepo,
		projectRepo:       projectRepo,
		notificationRepo:  notificationRepo,
		producer:          producer,
		cron:              cronScheduler,
		logger:            logger,
		config:            config,
	}
}

// Start запускает планировщик задач
func (s *SchedulerService) Start(ctx context.Context) error {
	s.logger.Info("Starting scheduler service")

	// Регистрируем задачи по расписанию
	s.registerTasks()

	// Запускаем планировщик
	s.cron.Start()

	// Слушаем сигнал завершения
	go func() {
		<-ctx.Done()
		s.logger.Info("Stopping scheduler service")
		s.cron.Stop()
	}()

	return nil
}

// registerTasks регистрирует все задачи в планировщике
func (s *SchedulerService) registerTasks() {
	// Задача для отправки ежедневных дайджестов
	if _, err := s.cron.AddFunc(s.config.DailyDigestCron, s.sendDailyDigests); err != nil {
		s.logger.Error("Failed to schedule daily digest task", err)
	}

	// Задача для отправки напоминаний о сроках
	if _, err := s.cron.AddFunc(s.config.DeadlineReminderCron, s.sendDeadlineReminders); err != nil {
		s.logger.Error("Failed to schedule deadline reminder task", err)
	}

	// Задача для проверки просроченных задач (каждый час)
	if _, err := s.cron.AddFunc("0 0 * * * *", s.checkOverdueTasks); err != nil {
		s.logger.Error("Failed to schedule overdue tasks check", err)
	}

	// Задача для автоматического архивирования завершенных проектов (раз в неделю)
	if _, err := s.cron.AddFunc("0 0 0 * * 0", s.archiveCompletedProjects); err != nil {
		s.logger.Error("Failed to schedule project archiving task", err)
	}
}

// sendDailyDigests отправляет ежедневные дайджесты задач
func (s *SchedulerService) sendDailyDigests() {
	ctx := context.Background()
	s.logger.Info("Running daily digest task")

	// Получаем всех активных пользователей
	filter := repository.UserFilter{
		IsActive: getBoolPtr(true),
	}
	users, err := s.userRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to get users for daily digest", err)
		return
	}

	// Текущая дата
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Для каждого пользователя формируем и отправляем дайджест
	for _, user := range users {
		// Проверяем настройки уведомлений пользователя
		settings, err := s.notificationRepo.GetUserNotificationSettings(ctx, user.ID)
		if err != nil {
			s.logger.Error("Failed to get notification settings", err, "user_id", user.ID)
			continue
		}

		// Проверяем, включены ли дайджесты для пользователя
		digestEnabled := false
		for _, setting := range settings {
			if setting.NotificationType == domain.NotificationTypeDigest && 
               (setting.EmailEnabled || setting.WebEnabled) {
				digestEnabled = true
				break
			}
		}

		if !digestEnabled {
			continue
		}

		// Получаем задачи, назначенные пользователю
		taskFilter := repository.TaskFilter{
			AssigneeID: &user.ID,
			DueAfter:   &today,
			OrderBy:    getStringPtr("due_date"),
			OrderDir:   getStringPtr("asc"),
		}
		tasks, err := s.taskRepo.GetTasksByAssignee(ctx, user.ID, taskFilter)
		if err != nil {
			s.logger.Error("Failed to get tasks for daily digest", err, "user_id", user.ID)
			continue
		}

		// Если нет активных задач, пропускаем
		if len(tasks) == 0 {
			continue
		}

		// Формируем содержимое дайджеста
		content := formatDailyDigest(tasks)

		// Создаем уведомление
		notification := &domain.Notification{
			UserID:     user.ID,
			Type:       domain.NotificationTypeDigest,
			Title:      "Ваш ежедневный отчет по задачам",
			Content:    content,
			Status:     domain.NotificationStatusUnread,
			EntityType: "digest",
			EntityID:   user.ID,
			CreatedAt:  time.Now(),
		}

		// Сохраняем уведомление
		if err := s.notificationRepo.Create(ctx, notification); err != nil {
			s.logger.Error("Failed to create digest notification", err, "user_id", user.ID)
			continue
		}

		// Отправляем событие для обработки уведомления
		event := &messaging.NotificationEvent{
			UserIDs:    []string{user.ID},
			Title:      notification.Title,
			Content:    notification.Content,
			Type:       string(notification.Type),
			EntityID:   user.ID,
			EntityType: "digest",
			CreatedAt:  notification.CreatedAt,
			MetaData: map[string]string{
				"user_id":   user.ID,
				"task_count": fmt.Sprintf("%d", len(tasks)),
			},
		}

		if err := s.producer.PublishNotificationEvent(ctx, messaging.EventTypeNotification, event); err != nil {
			s.logger.Error("Failed to publish digest notification event", err, "user_id", user.ID)
		}
	}

	s.logger.Info("Daily digest task completed")
}

// sendDeadlineReminders отправляет напоминания о приближающихся сроках задач
func (s *SchedulerService) sendDeadlineReminders() {
	ctx := context.Background()
	s.logger.Info("Running deadline reminder task")

	// Получаем задачи с дедлайном в ближайшие 24 часа
	now := time.Now()
	dayAfter := now.Add(24 * time.Hour)

	filter := repository.TaskFilter{
		DueBefore:  &dayAfter,
		DueAfter:   &now,
		Status:     getTaskStatusPtr(domain.TaskStatusNew, domain.TaskStatusInProgress, domain.TaskStatusOnHold),
	}

	tasks, err := s.taskRepo.GetUpcomingTasks(ctx, 1, filter) // 1 день
	if err != nil {
		s.logger.Error("Failed to get upcoming tasks", err)
		return
	}

	// Группируем задачи по исполнителям
	tasksByAssignee := make(map[string][]*domain.Task)
	for _, task := range tasks {
		if task.AssigneeID != nil {
			tasksByAssignee[*task.AssigneeID] = append(tasksByAssignee[*task.AssigneeID], task)
		}
	}

	// Отправляем уведомления для каждого исполнителя
	for assigneeID, assigneeTasks := range tasksByAssignee {
		// Проверяем настройки уведомлений
		settings, err := s.notificationRepo.GetUserNotificationSettings(ctx, assigneeID)
		if err != nil {
			s.logger.Error("Failed to get notification settings", err, "user_id", assigneeID)
			continue
		}

		// Проверяем, включены ли уведомления о дедлайнах
		dueSoonEnabled := false
		for _, setting := range settings {
			if setting.NotificationType == domain.NotificationTypeTaskDueSoon && 
               (setting.EmailEnabled || setting.WebEnabled) {
				dueSoonEnabled = true
				break
			}
		}

		if !dueSoonEnabled {
			continue
		}

		// Создаем уведомления для каждой задачи
		for _, task := range assigneeTasks {
			// Форматируем сообщение
			hoursLeft := int(task.DueDate.Sub(now).Hours())
			content := fmt.Sprintf("Срок выполнения задачи \"%s\" истекает через %d часов", task.Title, hoursLeft)

			// Создаем уведомление
			notification := &domain.Notification{
				UserID:     assigneeID,
				Type:       domain.NotificationTypeTaskDueSoon,
				Title:      "Приближается срок выполнения задачи",
				Content:    content,
				Status:     domain.NotificationStatusUnread,
				EntityType: "task",
				EntityID:   task.ID,
				CreatedAt:  time.Now(),
				MetaData: map[string]string{
					"task_id":    task.ID,
					"task_title": task.Title,
					"project_id": task.ProjectID,
					"due_date":   task.DueDate.Format(time.RFC3339),
					"hours_left": fmt.Sprintf("%d", hoursLeft),
				},
			}

			// Сохраняем уведомление
			if err := s.notificationRepo.Create(ctx, notification); err != nil {
				s.logger.Error("Failed to create deadline notification", err, "task_id", task.ID)
				continue
			}

			// Отправляем событие для обработки уведомления
			event := &messaging.NotificationEvent{
				UserIDs:    []string{assigneeID},
				Title:      notification.Title,
				Content:    notification.Content,
				Type:       string(notification.Type),
				EntityID:   task.ID,
				EntityType: "task",
				CreatedAt:  notification.CreatedAt,
				MetaData:   notification.MetaData,
			}

			if err := s.producer.PublishNotificationEvent(ctx, messaging.EventTypeNotification, event); err != nil {
				s.logger.Error("Failed to publish deadline notification event", err, "task_id", task.ID)
			}
		}
	}

	s.logger.Info("Deadline reminder task completed")
}

// checkOverdueTasks проверяет просроченные задачи и отправляет уведомления
func (s *SchedulerService) checkOverdueTasks() {
	ctx := context.Background()
	s.logger.Info("Running overdue tasks check")

	// Получаем просроченные, но не завершенные задачи
	now := time.Now()
	filter := repository.TaskFilter{
		DueBefore: &now,
		Status:    getTaskStatusPtr(domain.TaskStatusNew, domain.TaskStatusInProgress, domain.TaskStatusOnHold),
	}

	tasks, err := s.taskRepo.GetOverdueTasks(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to get overdue tasks", err)
		return
	}

	// Для каждой задачи отправляем уведомление
	for _, task := range tasks {
		// Пропускаем задачи без исполнителя
		if task.AssigneeID == nil {
			continue
		}

		// Проверяем, было ли уже отправлено уведомление о просрочке
		notificationFilter := repository.NotificationFilter{
			EntityID:   &task.ID,
			EntityType: getStringPtr("task"),
			Types:      []domain.NotificationType{domain.NotificationTypeTaskOverdue},
		}
		
		existingNotifications, err := s.notificationRepo.GetUserNotifications(ctx, *task.AssigneeID, notificationFilter)
		if err != nil {
			s.logger.Error("Failed to check existing notifications", err, "task_id", task.ID)
			continue
		}

		// Если уведомление уже есть, пропускаем
		if len(existingNotifications) > 0 {
			continue
		}

		// Проверяем настройки уведомлений
		settings, err := s.notificationRepo.GetUserNotificationSettings(ctx, *task.AssigneeID)
		if err != nil {
			s.logger.Error("Failed to get notification settings", err, "user_id", *task.AssigneeID)
			continue
		}

		// Проверяем, включены ли уведомления о просроченных задачах
		overdueEnabled := false
		for _, setting := range settings {
			if setting.NotificationType == domain.NotificationTypeTaskOverdue && 
               (setting.EmailEnabled || setting.WebEnabled) {
				overdueEnabled = true
				break
			}
		}

		if !overdueEnabled {
			continue
		}

		// Создаем уведомление
		content := fmt.Sprintf("Срок выполнения задачи \"%s\" истек", task.Title)
		
		notification := &domain.Notification{
			UserID:     *task.AssigneeID,
			Type:       domain.NotificationTypeTaskOverdue,
			Title:      "Задача просрочена",
			Content:    content,
			Status:     domain.NotificationStatusUnread,
			EntityType: "task",
			EntityID:   task.ID,
			CreatedAt:  time.Now(),
			MetaData: map[string]string{
				"task_id":    task.ID,
				"task_title": task.Title,
				"project_id": task.ProjectID,
				"due_date":   task.DueDate.Format(time.RFC3339),
			},
		}

		// Сохраняем уведомление
		if err := s.notificationRepo.Create(ctx, notification); err != nil {
			s.logger.Error("Failed to create overdue notification", err, "task_id", task.ID)
			continue
		}

		// Отправляем событие для обработки уведомления
		event := &messaging.NotificationEvent{
			UserIDs:    []string{*task.AssigneeID},
			Title:      notification.Title,
			Content:    notification.Content,
			Type:       string(notification.Type),
			EntityID:   task.ID,
			EntityType: "task",
			CreatedAt:  notification.CreatedAt,
			MetaData:   notification.MetaData,
		}

		if err := s.producer.PublishNotificationEvent(ctx, messaging.EventTypeNotification, event); err != nil {
			s.logger.Error("Failed to publish overdue notification event", err, "task_id", task.ID)
		}

		// Также уведомляем создателя задачи, если это не исполнитель
		if task.CreatedBy != *task.AssigneeID {
			creatorNotification := &domain.Notification{
				UserID:     task.CreatedBy,
				Type:       domain.NotificationTypeTaskOverdue,
				Title:      "Задача просрочена",
				Content:    fmt.Sprintf("Срок выполнения задачи \"%s\" истек", task.Title),
				Status:     domain.NotificationStatusUnread,
				EntityType: "task",
				EntityID:   task.ID,
				CreatedAt:  time.Now(),
				MetaData: map[string]string{
					"task_id":     task.ID,
					"task_title":  task.Title,
					"project_id":  task.ProjectID,
					"assignee_id": *task.AssigneeID,
					"due_date":    task.DueDate.Format(time.RFC3339),
				},
			}

			if err := s.notificationRepo.Create(ctx, creatorNotification); err != nil {
				s.logger.Error("Failed to create creator overdue notification", err, "task_id", task.ID)
				continue
			}

			event.UserIDs = []string{task.CreatedBy}
			event.Content = creatorNotification.Content
			event.MetaData = creatorNotification.MetaData

			if err := s.producer.PublishNotificationEvent(ctx, messaging.EventTypeNotification, event); err != nil {
				s.logger.Error("Failed to publish creator overdue notification event", err, "task_id", task.ID)
			}
		}
	}

	s.logger.Info("Overdue tasks check completed")
}

// archiveCompletedProjects архивирует завершенные проекты
func (s *SchedulerService) archiveCompletedProjects() {
	ctx := context.Background()
	s.logger.Info("Running project archiving task")

	// Получаем завершенные проекты, которые не архивированы
	filter := repository.ProjectFilter{
		Status: getProjectStatusPtr(domain.ProjectStatusCompleted),
	}

	projects, err := s.projectRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to get completed projects", err)
		return
	}

	// Для каждого проекта проверяем, что все задачи завершены и проект не обновлялся более недели
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)

	for _, project := range projects {
		// Пропускаем проекты, обновленные менее недели назад
		if project.UpdatedAt.After(weekAgo) {
			continue
		}

		// Проверяем статусы всех задач проекта
		taskFilter := repository.TaskFilter{
			ProjectIDs: []string{project.ID},
		}
		tasks, err := s.taskRepo.GetTasksByProject(ctx, project.ID, taskFilter)
		if err != nil {
			s.logger.Error("Failed to get project tasks", err, "project_id", project.ID)
			continue
		}

		// Проверяем, что все задачи завершены или отменены
		allCompleted := true
		for _, task := range tasks {
			if task.Status != domain.TaskStatusCompleted && task.Status != domain.TaskStatusCancelled {
				allCompleted = false
				break
			}
		}

		if !allCompleted {
			continue
		}

		// Архивируем проект
		project.Status = domain.ProjectStatusArchived
		project.UpdatedAt = now

		if err := s.projectRepo.Update(ctx, project); err != nil {
			s.logger.Error("Failed to archive project", err, "project_id", project.ID)
			continue
		}

		// Получаем список участников проекта
		members, err := s.projectRepo.GetMembers(ctx, project.ID)
		if err != nil {
			s.logger.Error("Failed to get project members", err, "project_id", project.ID)
			continue
		}

		// Отправляем уведомления участникам проекта
		for _, member := range members {
			notification := &domain.Notification{
				UserID:     member.UserID,
				Type:       domain.NotificationTypeProjectUpdated,
				Title:      "Проект архивирован",
				Content:    fmt.Sprintf("Проект \"%s\" был автоматически архивирован", project.Name),
				Status:     domain.NotificationStatusUnread,
				EntityType: "project",
				EntityID:   project.ID,
				CreatedAt:  now,
				MetaData: map[string]string{
					"project_id":   project.ID,
					"project_name": project.Name,
					"old_status":   string(domain.ProjectStatusCompleted),
					"new_status":   string(domain.ProjectStatusArchived),
				},
			}

			if err := s.notificationRepo.Create(ctx, notification); err != nil {
				s.logger.Error("Failed to create archive notification", err, "user_id", member.UserID)
				continue
			}

			// Отправляем событие для обработки уведомления
			event := &messaging.NotificationEvent{
				UserIDs:    []string{member.UserID},
				Title:      notification.Title,
				Content:    notification.Content,
				Type:       string(notification.Type),
				EntityID:   project.ID,
				EntityType: "project",
				CreatedAt:  notification.CreatedAt,
				MetaData:   notification.MetaData,
			}

			if err := s.producer.PublishNotificationEvent(ctx, messaging.EventTypeNotification, event); err != nil {
				s.logger.Error("Failed to publish archive notification event", err, "user_id", member.UserID)
			}
		}

		s.logger.Info("Project archived", "project_id", project.ID, "project_name", project.Name)
	}

	s.logger.Info("Project archiving task completed")
}

// Вспомогательные функции

func formatDailyDigest(tasks []*domain.Task) string {
	var dueTodayCount, dueTomorrowCount, overdueCount, inProgressCount int
	
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.AddDate(0, 0, 1)
	
	for _, task := range tasks {
		if task.Status == domain.TaskStatusInProgress {
			inProgressCount++
		}
		
		if task.DueDate != nil {
			dueDate := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, task.DueDate.Location())
			
			if dueDate.Before(today) {
				overdueCount++
			} else if dueDate.Equal(today) {
				dueTodayCount++
			} else if dueDate.Equal(tomorrow) {
				dueTomorrowCount++
			}
		}
	}
	
	// Формируем сообщение
	digest := fmt.Sprintf("У вас %d активных задач:\n", len(tasks))
	digest += fmt.Sprintf("- %d в процессе выполнения\n", inProgressCount)
	digest += fmt.Sprintf("- %d со сроком сегодня\n", dueTodayCount)
	digest += fmt.Sprintf("- %d со сроком завтра\n", dueTomorrowCount)
	digest += fmt.Sprintf("- %d просроченных задач\n\n", overdueCount)
	
	digest += "Задачи на сегодня:\n"
	for _, task := range tasks {
		if task.DueDate != nil && task.DueDate.Day() == today.Day() && 
           task.DueDate.Month() == today.Month() && task.DueDate.Year() == today.Year() {
			digest += fmt.Sprintf("- %s (приоритет: %s)\n", task.Title, task.Priority)
		}
	}
	
	return digest
}

func getBoolPtr(b bool) *bool {
	return &b
}

func getStringPtr(s string) *string {
	return &s
}

func getTaskStatusPtr(statuses ...domain.TaskStatus) *domain.TaskStatus {
	if len(statuses) == 0 {
		return nil
	}
	// Если передано несколько статусов, возвращаем первый
	// (в репозитории нужно будет изменить фильтрацию для поддержки множественных статусов)
	return &statuses[0]
}

func getProjectStatusPtr(status domain.ProjectStatus) *domain.ProjectStatus {
	return &status
}