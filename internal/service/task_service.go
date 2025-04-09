package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/messaging"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/pkg/logger"
	"github.com/nurlyy/task_manager/internal/repository/cache"
)

// Стандартные ошибки
var (
	ErrTaskNotFound         = errors.New("task not found")
	ErrTaskAccessDenied     = errors.New("access to task denied")
	ErrInvalidTaskStatus    = errors.New("invalid task status transition")
)

// TaskService представляет бизнес-логику для работы с задачами
type TaskService struct {
	taskRepo     repository.TaskRepository
	projectRepo  repository.ProjectRepository
	userRepo     repository.UserRepository
	commentRepo  repository.CommentRepository
	cacheRepo *cache.RedisRepository
	producer     *messaging.KafkaProducer
	projectSvc   *ProjectService
	logger       logger.Logger
}

// NewTaskService создает новый экземпляр TaskService
func NewTaskService(
	taskRepo repository.TaskRepository,
	projectRepo repository.ProjectRepository,
	userRepo repository.UserRepository,
	commentRepo repository.CommentRepository,
	cacheRepo *cache.RedisRepository,
	producer *messaging.KafkaProducer,
	projectSvc *ProjectService,
	logger logger.Logger,
) *TaskService {
	return &TaskService{
		taskRepo:    taskRepo,
		projectRepo: projectRepo,
		userRepo:    userRepo,
		commentRepo: commentRepo,
		cacheRepo:   cacheRepo,
		producer:    producer,
		projectSvc:  projectSvc,
		logger:      logger,
	}
}

// Create создает новую задачу
func (s *TaskService) Create(ctx context.Context, req domain.TaskCreateRequest, userID string) (*domain.TaskResponse, error) {
	// Проверяем доступ пользователя к проекту
	if !s.projectSvc.hasAccessToProject(ctx, req.ProjectID, userID) {
		return nil, ErrProjectNotFound
	}

	// Создаем новую задачу
	now := time.Now()
	task := &domain.Task{
		ID:             uuid.New().String(),
		Title:          req.Title,
		Description:    req.Description,
		ProjectID:      req.ProjectID,
		Status:         domain.TaskStatusNew,
		Priority:       req.Priority,
		AssigneeID:     req.AssigneeID,
		CreatedBy:      userID,
		DueDate:        req.DueDate,
		EstimatedHours: req.EstimatedHours,
		CreatedAt:      now,
		UpdatedAt:      now,
		Tags:           req.Tags,
	}

	// Сохраняем задачу в БД
	if err := s.taskRepo.Create(ctx, task); err != nil {
		s.logger.Error("Failed to create task", err)
		return nil, err
	}

	// Добавляем теги к задаче
	if len(req.Tags) > 0 {
		if err := s.taskRepo.UpdateTags(ctx, task.ID, req.Tags); err != nil {
			s.logger.Warn("Failed to add tags to task", "task_id", task.ID, "error", err)
		}
	}

	// Отправляем событие о создании задачи
	event := &messaging.TaskEvent{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		ProjectID:   task.ProjectID,
		Status:      string(task.Status),
		Priority:    string(task.Priority),
		AssigneeID:  task.AssigneeID,
		CreatedBy:   userID,
		DueDate:     task.DueDate,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
		Type:        messaging.EventTypeTaskCreated,
	}

	if err := s.producer.PublishTaskEvent(ctx, messaging.EventTypeTaskCreated, event); err != nil {
		s.logger.Warn("Failed to publish task creation event", "task_id", task.ID, "error", err)
	}

	// Если указан исполнитель, отправляем уведомление о назначении
	if task.AssigneeID != nil && *task.AssigneeID != userID {
		s.notifyTaskAssigned(ctx, task, userID)
	}

	// Формируем ответ
	resp := task.ToResponse()

	// Добавляем информацию о пользователях
	if task.AssigneeID != nil {
		assignee, err := s.userRepo.GetByID(ctx, *task.AssigneeID)
		if err == nil {
			brief := &domain.UserBrief{
				ID:        assignee.ID,
				Email:     assignee.Email,
				FirstName: assignee.FirstName,
				LastName:  assignee.LastName,
				Avatar:    assignee.Avatar,
			}
			resp.Assignee = brief
		}
	}

	creator, err := s.userRepo.GetByID(ctx, task.CreatedBy)
	if err == nil {
		brief := &domain.UserBrief{
			ID:        creator.ID,
			Email:     creator.Email,
			FirstName: creator.FirstName,
			LastName:  creator.LastName,
			Avatar:    creator.Avatar,
		}
		resp.Creator = brief
	}

	return &resp, nil
}

// GetByID возвращает задачу по ID
func (s *TaskService) GetByID(ctx context.Context, id string, userID string) (*domain.TaskResponse, error) {
	// Пытаемся получить из кэша
	cacheKey := "task:" + id
	var taskResp domain.TaskResponse
	if err := s.cacheRepo.Get(ctx, cacheKey, &taskResp); err == nil {
		// Проверяем доступ пользователя к задаче
		if s.hasAccessToTask(ctx, taskResp.ProjectID, userID) {
			return &taskResp, nil
		}
		return nil, ErrTaskAccessDenied
	}

	// Получаем задачу из БД
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get task by ID", err, "id", id)
		return nil, ErrTaskNotFound
	}

	// Проверяем доступ пользователя к задаче
	if !s.hasAccessToTask(ctx, task.ProjectID, userID) {
		return nil, ErrTaskAccessDenied
	}

	// Получаем теги задачи
	tags, err := s.taskRepo.GetTags(ctx, id)
	if err != nil {
		s.logger.Warn("Failed to get task tags", "task_id", id, "error", err)
	}
	task.Tags = tags

	// Формируем ответ
	resp := task.ToResponse()

	// Добавляем информацию о пользователях
	if task.AssigneeID != nil {
		assignee, err := s.userRepo.GetByID(ctx, *task.AssigneeID)
		if err == nil {
			resp.Assignee = &domain.UserBrief{
				ID:        assignee.ID,
				Email:     assignee.Email,
				FirstName: assignee.FirstName,
				LastName:  assignee.LastName,
				Avatar:    assignee.Avatar,
			}
		}
	}

	creator, err := s.userRepo.GetByID(ctx, task.CreatedBy)
	if err == nil {
		resp.Creator = &domain.UserBrief{
			ID:        creator.ID,
			Email:     creator.Email,
			FirstName: creator.FirstName,
			LastName:  creator.LastName,
			Avatar:    creator.Avatar,
		}
	}

	// Получаем комментарии к задаче
	comments, err := s.commentRepo.GetCommentsByTask(ctx, id, repository.CommentFilter{
		OrderBy: func() *string { s := "created_at"; return &s }(),
		OrderDir: func() *string { s := "desc"; return &s }(),
		Limit:    50,
	})
	if err == nil {
		commentResponses := make([]domain.CommentResponse, 0, len(comments))
		for _, comment := range comments {
			user, err := s.userRepo.GetByID(ctx, comment.UserID)
			if err != nil {
				continue
			}

			brief := domain.UserBrief{
				ID:        user.ID,
				Email:     user.Email,
				FirstName: user.FirstName,
				LastName:  user.LastName,
				Avatar:    user.Avatar,
			}
			commentResponses = append(commentResponses, comment.ToResponse(brief))
		}
		resp.Comments = commentResponses
	}

	// Получаем историю изменений задачи
	history, err := s.taskRepo.GetTaskHistory(ctx, id)
	if err == nil {
		historyResponses := make([]domain.TaskHistoryResponse, 0, len(history))
		for _, h := range history {
			user, err := s.userRepo.GetByID(ctx, h.UserID)
			if err != nil {
				continue
			}

			brief := domain.UserBrief{
				ID:        user.ID,
				Email:     user.Email,
				FirstName: user.FirstName,
				LastName:  user.LastName,
				Avatar:    user.Avatar,
			}
			historyResponses = append(historyResponses, domain.TaskHistoryResponse{
				ID:        h.ID,
				UserID:    h.UserID,
				User:      brief,
				Field:     h.Field,
				OldValue:  h.OldValue,
				NewValue:  h.NewValue,
				ChangedAt: h.ChangedAt,
			})
		}
		resp.History = historyResponses
	}

	// Сохраняем в кэш
	if err := s.cacheRepo.Set(ctx, cacheKey, resp); err != nil {
		s.logger.Warn("Failed to cache task", "id", id, "error", err)
	}

	return &resp, nil
}

// Update обновляет данные задачи
func (s *TaskService) Update(ctx context.Context, id string, req domain.TaskUpdateRequest, userID string) (*domain.TaskResponse, error) {
	// Получаем задачу из БД
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get task by ID for update", err, "id", id)
		return nil, ErrTaskNotFound
	}

	// Проверяем доступ пользователя к задаче
	if !s.hasAccessToTask(ctx, task.ProjectID, userID) {
		return nil, ErrTaskAccessDenied
	}

	// Для смены статуса нужно проверить, что пользователь не ниже члена проекта
	if req.Status != nil && *req.Status != task.Status {
		if !s.canManageTask(ctx, task.ProjectID, userID) {
			return nil, ErrInsufficientRights
		}
	}

	// Фиксируем изменения для события
	changes := make(map[string]interface{})

	// Обновляем поля, которые были переданы
	if req.Title != nil {
		changes["title"] = map[string]interface{}{"old": task.Title, "new": *req.Title}
		task.Title = *req.Title
	}
	if req.Description != nil {
		changes["description"] = map[string]interface{}{"old": task.Description, "new": *req.Description}
		task.Description = *req.Description
	}
	if req.Status != nil {
		changes["status"] = map[string]interface{}{"old": task.Status, "new": *req.Status}
		task.Status = *req.Status
	}
	if req.Priority != nil {
		changes["priority"] = map[string]interface{}{"old": task.Priority, "new": *req.Priority}
		task.Priority = *req.Priority
	}
	if req.AssigneeID != nil {
		oldAssigneeID := ""
		if task.AssigneeID != nil {
			oldAssigneeID = *task.AssigneeID
		}
		newAssigneeID := ""
		if req.AssigneeID != nil {
			newAssigneeID = *req.AssigneeID
		}
		changes["assignee_id"] = map[string]interface{}{"old": oldAssigneeID, "new": newAssigneeID}
		task.AssigneeID = req.AssigneeID
	}
	if req.DueDate != nil {
		changes["due_date"] = map[string]interface{}{"old": task.DueDate, "new": *req.DueDate}
		task.DueDate = req.DueDate
	}
	if req.EstimatedHours != nil {
		changes["estimated_hours"] = map[string]interface{}{"old": task.EstimatedHours, "new": *req.EstimatedHours}
		task.EstimatedHours = req.EstimatedHours
	}
	if req.SpentHours != nil {
		changes["spent_hours"] = map[string]interface{}{"old": task.SpentHours, "new": *req.SpentHours}
		task.SpentHours = req.SpentHours
	}

	task.UpdatedAt = time.Now()

	// Если статус изменен на "завершено", устанавливаем дату завершения
	if req.Status != nil && *req.Status == domain.TaskStatusCompleted && task.CompletedAt == nil {
		now := time.Now()
		task.CompletedAt = &now
	} else if req.Status != nil && *req.Status != domain.TaskStatusCompleted {
		task.CompletedAt = nil
	}

	// Обновляем задачу в БД
	if err := s.taskRepo.Update(ctx, task); err != nil {
		s.logger.Error("Failed to update task", err, "id", id)
		return nil, err
	}

	// Обновляем теги, если они были переданы
	if req.Tags != nil {
		if err := s.taskRepo.UpdateTags(ctx, task.ID, *req.Tags); err != nil {
			s.logger.Warn("Failed to update task tags", "task_id", task.ID, "error", err)
		}
		task.Tags = *req.Tags
	} else {
		// Получаем текущие теги
		tags, err := s.taskRepo.GetTags(ctx, id)
		if err == nil {
			task.Tags = tags
		}
	}

	// Удаляем задачу из кэша
	cacheKey := "task:" + id
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete task from cache", "id", id, "error", err)
	}

	// Отправляем событие об обновлении задачи, если были изменения
	if len(changes) > 0 {
		event := &messaging.TaskEvent{
			ID:          task.ID,
			Title:       task.Title,
			ProjectID:   task.ProjectID,
			Status:      string(task.Status),
			Priority:    string(task.Priority),
			AssigneeID:  task.AssigneeID,
			UpdatedAt:   task.UpdatedAt,
			Type:        messaging.EventTypeTaskUpdated,
			Changes:     changes,
		}

		if err := s.producer.PublishTaskEvent(ctx, messaging.EventTypeTaskUpdated, event); err != nil {
			s.logger.Warn("Failed to publish task update event", "task_id", task.ID, "error", err)
		}

		// Если изменился исполнитель, отправляем уведомление
		if _, ok := changes["assignee_id"]; ok && task.AssigneeID != nil && *task.AssigneeID != userID {
			s.notifyTaskAssigned(ctx, task, userID)
		}
	}

	// Формируем ответ
	resp := task.ToResponse()

	// Добавляем информацию о пользователях
	if task.AssigneeID != nil {
		assignee, err := s.userRepo.GetByID(ctx, *task.AssigneeID)
		if err == nil {
			resp.Assignee = &domain.UserBrief{
				ID:        assignee.ID,
				Email:     assignee.Email,
				FirstName: assignee.FirstName,
				LastName:  assignee.LastName,
				Avatar:    assignee.Avatar,
			}
		}
	}

	creator, err := s.userRepo.GetByID(ctx, task.CreatedBy)
	if err == nil {
		resp.Creator = &domain.UserBrief{
			ID:        creator.ID,
			Email:     creator.Email,
			FirstName: creator.FirstName,
			LastName:  creator.LastName,
			Avatar:    creator.Avatar,
		}
	}

	return &resp, nil
}

// Вспомогательные методы

// hasAccessToTask проверяет, имеет ли пользователь доступ к задаче
func (s *TaskService) hasAccessToTask(ctx context.Context, projectID string, userID string) bool {
	return s.projectSvc.hasAccessToProject(ctx, projectID, userID)
}

// canManageTask проверяет, может ли пользователь управлять задачей
func (s *TaskService) canManageTask(ctx context.Context, projectID string, userID string) bool {
	// Получаем пользователя
	user, err := s.userRepo.GetByID(ctx, userID)
	if err == nil && user.IsAdmin() {
		return true
	}

	// Проверяем, является ли пользователь участником проекта
	member, err := s.projectRepo.GetMember(ctx, projectID, userID)
	if err != nil {
		return false
	}

	// Проверяем роль пользователя в проекте
	return member.Role == domain.ProjectRoleOwner || 
		   member.Role == domain.ProjectRoleManager || 
		   member.Role == domain.ProjectRoleMember
}

// isValidStatusTransition проверяет корректность перехода из одного статуса в другой
func (s *TaskService) isValidStatusTransition(from, to domain.TaskStatus) bool {
	// Правила переходов статусов
	transitions := map[domain.TaskStatus][]domain.TaskStatus{
		domain.TaskStatusNew: {
			domain.TaskStatusInProgress,
			domain.TaskStatusOnHold,
			domain.TaskStatusCancelled,
		},
		domain.TaskStatusInProgress: {
			domain.TaskStatusOnHold,
			domain.TaskStatusReview,
			domain.TaskStatusCompleted,
			domain.TaskStatusCancelled,
		},
		domain.TaskStatusOnHold: {
			domain.TaskStatusInProgress,
			domain.TaskStatusCancelled,
		},
		domain.TaskStatusReview: {
			domain.TaskStatusInProgress,
			domain.TaskStatusCompleted,
			domain.TaskStatusCancelled,
		},
		domain.TaskStatusCompleted: {
			domain.TaskStatusInProgress,
			domain.TaskStatusReview,
		},
		domain.TaskStatusCancelled: {
			domain.TaskStatusNew,
			domain.TaskStatusInProgress,
		},
	}

	// Разрешаем переход в тот же статус
	if from == to {
		return true
	}

	// Проверяем, разрешен ли переход
	allowedTransitions, ok := transitions[from]
	if !ok {
		return false
	}

	for _, allowed := range allowedTransitions {
		if allowed == to {
			return true
		}
	}

	return false
}

// notifyTaskAssigned отправляет уведомление о назначении задачи
func (s *TaskService) notifyTaskAssigned(ctx context.Context, task *domain.Task, assignerID string) {
	if task.AssigneeID == nil {
		return
	}

	// Получаем данные о пользователях
	assignee, err := s.userRepo.GetByID(ctx, *task.AssigneeID)
	if err != nil {
		s.logger.Error("Failed to get assignee", err, "user_id", *task.AssigneeID)
		return
	}

	assigner, err := s.userRepo.GetByID(ctx, assignerID)
	if err != nil {
		s.logger.Error("Failed to get assigner", err, "user_id", assignerID)
		return
	}

	// Создаем событие для отправки уведомления
	notificationEvent := &messaging.NotificationEvent{
		UserIDs:    []string{*task.AssigneeID},
		Title:      "Task assigned to you",
		Content:    assigner.FullName() + " assigned you the task: " + task.Title,
		Type:       string(domain.NotificationTypeTaskAssigned),
		EntityID:   task.ID,
		EntityType: "task",
		CreatedAt:  time.Now(),
		MetaData: map[string]string{
			"task_id":     task.ID,
			"task_title":  task.Title,
			"project_id":  task.ProjectID,
			"assigner_id": assignerID,
		},
	}

	if err := s.producer.PublishNotificationEvent(ctx, messaging.EventTypeNotification, notificationEvent); err != nil {
		s.logger.Error("Failed to publish notification event", err, "task_id", task.ID)
	}
}

// UpdateAssignee обновляет исполнителя задачи
func (s *TaskService) UpdateAssignee(ctx context.Context, id string, assigneeID *string, userID string) (*domain.TaskResponse, error) {
	// Получаем задачу из БД
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get task by ID for assignee update", err, "id", id)
		return nil, ErrTaskNotFound
	}

	// Проверяем доступ пользователя к задаче
	if !s.hasAccessToTask(ctx, task.ProjectID, userID) {
		return nil, ErrTaskAccessDenied
	}

	// Проверяем права на изменение исполнителя
	if !s.canManageTask(ctx, task.ProjectID, userID) && task.CreatedBy != userID {
		return nil, ErrInsufficientRights
	}

	// Если указан новый исполнитель, проверяем, что он является участником проекта
	if assigneeID != nil {
		if _, err := s.projectRepo.GetMember(ctx, task.ProjectID, *assigneeID); err != nil {
			return nil, errors.New("assignee must be a member of the project")
		}
	}

	// Обновляем исполнителя задачи
	if err := s.taskRepo.UpdateAssignee(ctx, id, assigneeID, userID); err != nil {
		s.logger.Error("Failed to update task assignee", err, "id", id)
		return nil, err
	}

	// Удаляем задачу из кэша
	cacheKey := "task:" + id
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete task from cache", "id", id, "error", err)
	}

	// Получаем обновленную задачу
	updatedTask, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get updated task", err, "id", id)
		return nil, err
	}

	// Получаем теги задачи
	tags, err := s.taskRepo.GetTags(ctx, id)
	if err == nil {
		updatedTask.Tags = tags
	}

	// Отправляем событие об обновлении задачи
	var oldAssigneeID, newAssigneeID string
	if task.AssigneeID != nil {
		oldAssigneeID = *task.AssigneeID
	}
	if updatedTask.AssigneeID != nil {
		newAssigneeID = *updatedTask.AssigneeID
	}

	event := &messaging.TaskEvent{
		ID:          updatedTask.ID,
		Title:       updatedTask.Title,
		ProjectID:   updatedTask.ProjectID,
		Status:      string(updatedTask.Status),
		Priority:    string(updatedTask.Priority),
		AssigneeID:  updatedTask.AssigneeID,
		AssignerID:  userID,
		UpdatedAt:   updatedTask.UpdatedAt,
		Type:        messaging.EventTypeTaskAssigned,
		Changes: map[string]interface{}{
			"assignee_id": map[string]interface{}{
				"old": oldAssigneeID,
				"new": newAssigneeID,
			},
		},
	}

	if err := s.producer.PublishTaskEvent(ctx, messaging.EventTypeTaskAssigned, event); err != nil {
		s.logger.Warn("Failed to publish task assignee update event", "task_id", updatedTask.ID, "error", err)
	}

	// Если назначен новый исполнитель, отправляем уведомление
	if updatedTask.AssigneeID != nil && *updatedTask.AssigneeID != userID {
		s.notifyTaskAssigned(ctx, updatedTask, userID)
	}

	// Формируем ответ
	resp := updatedTask.ToResponse()

	// Добавляем информацию о пользователях
	if updatedTask.AssigneeID != nil {
		assignee, err := s.userRepo.GetByID(ctx, *updatedTask.AssigneeID)
		if err == nil {
			resp.Assignee = &domain.UserBrief{
				ID:        assignee.ID,
				Email:     assignee.Email,
				FirstName: assignee.FirstName,
				LastName:  assignee.LastName,
				Avatar:    assignee.Avatar,
			}
		}
	}

	creator, err := s.userRepo.GetByID(ctx, updatedTask.CreatedBy)
	if err == nil {
		resp.Creator = &domain.UserBrief{
			ID:        creator.ID,
			Email:     creator.Email,
			FirstName: creator.FirstName,
			LastName:  creator.LastName,
			Avatar:    creator.Avatar,
		}
	}

	return &resp, nil
}

// LogTime добавляет запись о затраченном времени
func (s *TaskService) LogTime(ctx context.Context, id string, req domain.LogTimeRequest, userID string) error {
	// Получаем задачу из БД
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get task by ID for logging time", err, "id", id)
		return ErrTaskNotFound
	}

	// Проверяем доступ пользователя к задаче
	if !s.hasAccessToTask(ctx, task.ProjectID, userID) {
		return ErrTaskAccessDenied
	}

	// Создаем запись о затраченном времени
	logDate := time.Now()
	if req.Date != nil {
		logDate = *req.Date
	}

	timeLog := &repository.TimeLog{
		ID:          uuid.New().String(),
		TaskID:      id,
		UserID:      userID,
		Hours:       req.Hours,
		Description: req.Description,
		LoggedAt:    time.Now(),
		LogDate:     logDate,
	}

	// Добавляем запись о затраченном времени
	if err := s.taskRepo.LogTime(ctx, timeLog); err != nil {
		s.logger.Error("Failed to log time", err, "task_id", id)
		return err
	}

	// Обновляем общее затраченное время в задаче
	var spentHours float64
	if task.SpentHours != nil {
		spentHours = *task.SpentHours + req.Hours
	} else {
		spentHours = req.Hours
	}
	
	task.SpentHours = &spentHours
	task.UpdatedAt = time.Now()

	if err := s.taskRepo.Update(ctx, task); err != nil {
		s.logger.Error("Failed to update task spent hours", err, "id", id)
		return err
	}

	// Удаляем задачу из кэша
	cacheKey := "task:" + id
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete task from cache", "id", id, "error", err)
	}

	return nil
}

// GetTimeLogs возвращает записи о затраченном времени
func (s *TaskService) GetTimeLogs(ctx context.Context, id string, userID string) ([]*repository.TimeLog, error) {
	// Получаем задачу из БД
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get task by ID for getting time logs", err, "id", id)
		return nil, ErrTaskNotFound
	}

	// Проверяем доступ пользователя к задаче
	if !s.hasAccessToTask(ctx, task.ProjectID, userID) {
		return nil, ErrTaskAccessDenied
	}

	// Получаем записи о затраченном времени
	timeLogs, err := s.taskRepo.GetTimeLogs(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get time logs", err, "task_id", id)
		return nil, err
	}

	return timeLogs, nil
}// Delete удаляет задачу
func (s *TaskService) Delete(ctx context.Context, id string, userID string) error {
	// Получаем задачу из БД
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get task by ID for delete", err, "id", id)
		return ErrTaskNotFound
	}

	// Проверяем права пользователя на управление задачей
	if !s.canManageTask(ctx, task.ProjectID, userID) && task.CreatedBy != userID {
		return ErrInsufficientRights
	}

	// Удаляем задачу из БД
	if err := s.taskRepo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete task", err, "id", id)
		return err
	}

	// Удаляем задачу из кэша
	cacheKey := "task:" + id
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete task from cache", "id", id, "error", err)
	}

	return nil
}

// List возвращает список задач с фильтрацией
func (s *TaskService) List(ctx context.Context, filter domain.TaskFilterOptions, userID string, page, pageSize int) (*domain.PagedResponse, error) {
	// Преобразуем фильтр доменной модели в фильтр репозитория
	repoFilter := repository.TaskFilter{
		ProjectIDs:  []string{},
		SearchText:  filter.SearchText,
		Status:      filter.Status,
		Priority:    filter.Priority,
		AssigneeID:  filter.AssigneeID,
		CreatedBy:   filter.CreatedBy,
		DueBefore:   filter.DueBefore,
		DueAfter:    filter.DueAfter,
		Tags:        filter.Tags,
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	}

	// Если указан ID проекта, проверяем доступ пользователя к нему
	if filter.ProjectID != nil {
		if !s.projectSvc.hasAccessToProject(ctx, *filter.ProjectID, userID) {
			return nil, ErrProjectNotFound
		}
		repoFilter.ProjectIDs = append(repoFilter.ProjectIDs, *filter.ProjectID)
	} else {
		// Если проект не указан, получаем все проекты пользователя
		projectFilter := repository.ProjectFilter{
			MemberID: &userID,
		}
		projects, err := s.projectRepo.List(ctx, projectFilter)
		if err != nil {
			s.logger.Error("Failed to list user projects", err, "user_id", userID)
			return nil, err
		}

		for _, project := range projects {
			repoFilter.ProjectIDs = append(repoFilter.ProjectIDs, project.ID)
		}
	}

	// Настройка сортировки
	if filter.SortBy != nil {
		repoFilter.OrderBy = filter.SortBy
		if filter.SortOrder != nil {
			repoFilter.OrderDir = filter.SortOrder
		} else {
			dir := "asc"
			repoFilter.OrderDir = &dir
		}
	} else {
		// По умолчанию сортируем по дате обновления
		orderBy := "updated_at"
		orderDir := "desc"
		repoFilter.OrderBy = &orderBy
		repoFilter.OrderDir = &orderDir
	}

	// Получаем список задач
	tasks, err := s.taskRepo.List(ctx, repoFilter)
	if err != nil {
		s.logger.Error("Failed to list tasks", err)
		return nil, err
	}

	// Получаем общее количество задач
	total, err := s.taskRepo.Count(ctx, repoFilter)
	if err != nil {
		s.logger.Error("Failed to count tasks", err)
		return nil, err
	}

	// Формируем ответы для задач
	taskResponses := make([]domain.TaskResponse, len(tasks))
	for i, task := range tasks {
		// Получаем теги задачи
		tags, err := s.taskRepo.GetTags(ctx, task.ID)
		if err == nil {
			task.Tags = tags
		}

		resp := task.ToResponse()

		// Добавляем базовую информацию о пользователях
		if task.AssigneeID != nil {
			assignee, err := s.userRepo.GetByID(ctx, *task.AssigneeID)
			if err == nil {
				resp.Assignee = &domain.UserBrief{
					ID:        assignee.ID,
					Email:     assignee.Email,
					FirstName: assignee.FirstName,
					LastName:  assignee.LastName,
					Avatar:    assignee.Avatar,
				}
			}
		}

		creator, err := s.userRepo.GetByID(ctx, task.CreatedBy)
		if err == nil {
			resp.Creator = &domain.UserBrief{
				ID:        creator.ID,
				Email:     creator.Email,
				FirstName: creator.FirstName,
				LastName:  creator.LastName,
				Avatar:    creator.Avatar,
			}
		}

		taskResponses[i] = resp
	}

	// Формируем ответ с пагинацией
	return &domain.PagedResponse{
		Items:      taskResponses,
		TotalItems: total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}, nil
}

// UpdateStatus обновляет статус задачи
func (s *TaskService) UpdateStatus(ctx context.Context, id string, status domain.TaskStatus, userID string) (*domain.TaskResponse, error) {
	// Получаем задачу из БД
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get task by ID for status update", err, "id", id)
		return nil, ErrTaskNotFound
	}

	// Проверяем доступ пользователя к задаче
	if !s.hasAccessToTask(ctx, task.ProjectID, userID) {
		return nil, ErrTaskAccessDenied
	}

	// Проверяем права на изменение статуса (должен быть хотя бы членом проекта)
	if !s.canManageTask(ctx, task.ProjectID, userID) {
		return nil, ErrInsufficientRights
	}

	// Проверяем корректность перехода статуса
	if !s.isValidStatusTransition(task.Status, status) {
		return nil, ErrInvalidTaskStatus
	}

	// Обновляем статус задачи
	if err := s.taskRepo.UpdateStatus(ctx, id, status, userID); err != nil {
		s.logger.Error("Failed to update task status", err, "id", id)
		return nil, err
	}

	// Удаляем задачу из кэша
	cacheKey := "task:" + id
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete task from cache", "id", id, "error", err)
	}

	// Получаем обновленную задачу
	updatedTask, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get updated task", err, "id", id)
		return nil, err
	}

	// Получаем теги задачи
	tags, err := s.taskRepo.GetTags(ctx, id)
	if err == nil {
		updatedTask.Tags = tags
	}

	// Отправляем событие об обновлении задачи
	event := &messaging.TaskEvent{
		ID:          updatedTask.ID,
		Title:       updatedTask.Title,
		ProjectID:   updatedTask.ProjectID,
		Status:      string(updatedTask.Status),
		Priority:    string(updatedTask.Priority),
		AssigneeID:  updatedTask.AssigneeID,
		UpdatedAt:   updatedTask.UpdatedAt,
		Type:        messaging.EventTypeTaskUpdated,
		Changes: map[string]interface{}{
			"status": map[string]interface{}{
				"old": string(task.Status),
				"new": string(status),
			},
		},
	}

	if err := s.producer.PublishTaskEvent(ctx, messaging.EventTypeTaskUpdated, event); err != nil {
		s.logger.Warn("Failed to publish task status update event", "task_id", updatedTask.ID, "error", err)
	}

	// Формируем ответ
	resp := updatedTask.ToResponse()

	// Добавляем информацию о пользователях
	if updatedTask.AssigneeID != nil {
		assignee, err := s.userRepo.GetByID(ctx, *updatedTask.AssigneeID)
		if err == nil {
			resp.Assignee = &domain.UserBrief{
				ID:        assignee.ID,
				Email:     assignee.Email,
				FirstName: assignee.FirstName,
				LastName:  assignee.LastName,
				Avatar:    assignee.Avatar,
			}
		}
	}

	creator, err := s.userRepo.GetByID(ctx, updatedTask.CreatedBy)
	if err == nil {
		resp.Creator = &domain.UserBrief{
			ID:        creator.ID,
			Email:     creator.Email,
			FirstName: creator.FirstName,
			LastName:  creator.LastName,
			Avatar:    creator.Avatar,
		}
	}

	return &resp, nil
}