package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yourusername/task-tracker/internal/domain"
	"github.com/yourusername/task-tracker/internal/messaging"
	"github.com/yourusername/task-tracker/internal/repository"
	"github.com/yourusername/task-tracker/pkg/logger"
)

// Стандартные ошибки
var (
	ErrCommentNotFound     = errors.New("comment not found")
	ErrCommentAccessDenied = errors.New("access to comment denied")
)

// CommentService представляет бизнес-логику для работы с комментариями
type CommentService struct {
	commentRepo repository.CommentRepository
	taskRepo    repository.TaskRepository
	userRepo    repository.UserRepository
	taskSvc     *TaskService
	producer    *messaging.KafkaProducer
	logger      logger.Logger
}

// NewCommentService создает новый экземпляр CommentService
func NewCommentService(
	commentRepo repository.CommentRepository,
	taskRepo repository.TaskRepository,
	userRepo repository.UserRepository,
	taskSvc *TaskService,
	producer *messaging.KafkaProducer,
	logger logger.Logger,
) *CommentService {
	return &CommentService{
		commentRepo: commentRepo,
		taskRepo:    taskRepo,
		userRepo:    userRepo,
		taskSvc:     taskSvc,
		producer:    producer,
		logger:      logger,
	}
}

// Create создает новый комментарий
func (s *CommentService) Create(ctx context.Context, req domain.CommentCreateRequest, userID string) (*domain.CommentResponse, error) {
	// Проверяем, существует ли задача
	task, err := s.taskRepo.GetByID(ctx, req.TaskID)
	if err != nil {
		s.logger.Error("Failed to get task by ID for comment creation", err, "task_id", req.TaskID)
		return nil, ErrTaskNotFound
	}

	// Проверяем доступ пользователя к задаче
	if !s.taskSvc.hasAccessToTask(ctx, task.ProjectID, userID) {
		return nil, ErrTaskAccessDenied
	}

	// Создаем новый комментарий
	now := time.Now()
	comment := &domain.Comment{
		ID:        uuid.New().String(),
		TaskID:    req.TaskID,
		UserID:    userID,
		Content:   req.Content,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Сохраняем комментарий в БД
	if err := s.commentRepo.Create(ctx, comment); err != nil {
		s.logger.Error("Failed to create comment", err)
		return nil, err
	}

	// Получаем данные пользователя
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user for comment response", err, "user_id", userID)
		return nil, err
	}

	// Формируем UserBrief для ответа
	userBrief := domain.UserBrief{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Avatar:    user.Avatar,
	}

	// Отправляем событие о добавлении комментария
	event := &messaging.CommentEvent{
		TaskID:    req.TaskID,
		TaskTitle: task.Title,
		CommentID: comment.ID,
		UserID:    userID,
		Content:   req.Content,
		CreatedAt: now,
		Type:      messaging.EventTypeTaskCommented,
	}

	if err := s.producer.PublishCommentEvent(ctx, messaging.EventTypeTaskCommented, event); err != nil {
		s.logger.Warn("Failed to publish comment event", "comment_id", comment.ID, "error", err)
	}

	// Отправляем уведомление о комментарии автору и исполнителю задачи (если они не являются автором комментария)
	s.notifyAboutComment(ctx, task, comment, userID)

	// Формируем ответ
	return &comment.ToResponse(userBrief), nil
}

// GetByID возвращает комментарий по ID
func (s *CommentService) GetByID(ctx context.Context, id string, userID string) (*domain.CommentResponse, error) {
	// Получаем комментарий из БД
	comment, err := s.commentRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get comment by ID", err, "id", id)
		return nil, ErrCommentNotFound
	}

	// Получаем задачу для проверки доступа
	task, err := s.taskRepo.GetByID(ctx, comment.TaskID)
	if err != nil {
		s.logger.Error("Failed to get task for comment access check", err, "task_id", comment.TaskID)
		return nil, ErrTaskNotFound
	}

	// Проверяем доступ пользователя к задаче
	if !s.taskSvc.hasAccessToTask(ctx, task.ProjectID, userID) {
		return nil, ErrCommentAccessDenied
	}

	// Получаем данные пользователя-автора комментария
	user, err := s.userRepo.GetByID(ctx, comment.UserID)
	if err != nil {
		s.logger.Error("Failed to get comment author", err, "user_id", comment.UserID)
		return nil, err
	}

	// Формируем UserBrief для ответа
	userBrief := domain.UserBrief{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Avatar:    user.Avatar,
	}

	// Формируем ответ
	return &comment.ToResponse(userBrief), nil
}

// Update обновляет комментарий
func (s *CommentService) Update(ctx context.Context, id string, req domain.CommentUpdateRequest, userID string) (*domain.CommentResponse, error) {
	// Получаем комментарий из БД
	comment, err := s.commentRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get comment by ID for update", err, "id", id)
		return nil, ErrCommentNotFound
	}

	// Проверяем, является ли пользователь автором комментария
	if comment.UserID != userID {
		// Проверяем, является ли пользователь администратором
		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil || user.Role != domain.UserRoleAdmin {
			s.logger.Warn("User attempted to update another user's comment", "user_id", userID, "comment_id", id)
			return nil, ErrInsufficientRights
		}
	}

	// Обновляем содержимое комментария
	comment.Content = req.Content
	comment.UpdatedAt = time.Now()

	// Сохраняем изменения в БД
	if err := s.commentRepo.Update(ctx, comment); err != nil {
		s.logger.Error("Failed to update comment", err, "id", id)
		return nil, err
	}

	// Получаем данные пользователя-автора комментария
	user, err := s.userRepo.GetByID(ctx, comment.UserID)
	if err != nil {
		s.logger.Error("Failed to get comment author", err, "user_id", comment.UserID)
		return nil, err
	}

	// Формируем UserBrief для ответа
	userBrief := domain.UserBrief{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Avatar:    user.Avatar,
	}

	// Формируем ответ
	return &comment.ToResponse(userBrief), nil
}

// Delete удаляет комментарий
func (s *CommentService) Delete(ctx context.Context, id string, userID string) error {
	// Получаем комментарий из БД
	comment, err := s.commentRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get comment by ID for delete", err, "id", id)
		return ErrCommentNotFound
	}

	// Проверяем, является ли пользователь автором комментария
	if comment.UserID != userID {
		// Проверяем, является ли пользователь администратором
		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil || user.Role != domain.UserRoleAdmin {
			s.logger.Warn("User attempted to delete another user's comment", "user_id", userID, "comment_id", id)
			return ErrInsufficientRights
		}
	}

	// Удаляем комментарий из БД
	if err := s.commentRepo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete comment", err, "id", id)
		return err
	}

	return nil
}

// GetCommentsByTask возвращает комментарии к задаче
func (s *CommentService) GetCommentsByTask(ctx context.Context, taskID string, userID string, page, pageSize int) (*domain.PagedResponse, error) {
	// Проверяем, существует ли задача
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		s.logger.Error("Failed to get task by ID for comments", err, "task_id", taskID)
		return nil, ErrTaskNotFound
	}

	// Проверяем доступ пользователя к задаче
	if !s.taskSvc.hasAccessToTask(ctx, task.ProjectID, userID) {
		return nil, ErrTaskAccessDenied
	}

	// Настраиваем фильтр
	filter := repository.CommentFilter{
		TaskIDs: []string{taskID},
		OrderBy: func() *string { s := "created_at"; return &s }(),
		OrderDir: func() *string { s := "desc"; return &s }(),
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
	}

	// Получаем комментарии к задаче
	comments, err := s.commentRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to get comments by task", err, "task_id", taskID)
		return nil, err
	}

	// Получаем общее количество комментариев
	total, err := s.commentRepo.CountCommentsByTask(ctx, taskID)
	if err != nil {
		s.logger.Error("Failed to count comments by task", err, "task_id", taskID)
		return nil, err
	}

	// Формируем ответы для комментариев
	commentResponses := make([]domain.CommentResponse, len(comments))
	for i, comment := range comments {
		// Получаем данные пользователя-автора комментария
		user, err := s.userRepo.GetByID(ctx, comment.UserID)
		if err != nil {
			s.logger.Error("Failed to get comment author", err, "user_id", comment.UserID)
			continue
		}

		// Формируем UserBrief для ответа
		userBrief := domain.UserBrief{
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Avatar:    user.Avatar,
		}

		commentResponses[i] = comment.ToResponse(userBrief)
	}

	// Формируем ответ с пагинацией
	return &domain.PagedResponse{
		Items:      commentResponses,
		TotalItems: total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}, nil
}

// notifyAboutComment отправляет уведомление о новом комментарии
func (s *CommentService) notifyAboutComment(ctx context.Context, task *domain.Task, comment *domain.Comment, userID string) {
	// Формируем список получателей уведомления
	recipients := make([]string, 0, 2)
	
	// Добавляем автора задачи, если он не является автором комментария
	if task.CreatedBy != userID {
		recipients = append(recipients, task.CreatedBy)
	}
	
	// Добавляем исполнителя задачи, если он не является автором комментария
	// и не является автором задачи (чтобы избежать дублирования)
	if task.AssigneeID != nil && *task.AssigneeID != userID && *task.AssigneeID != task.CreatedBy {
		recipients = append(recipients, *task.AssigneeID)
	}
	
	// Если нет получателей, выходим
	if len(recipients) == 0 {
		return
	}
	
	// Получаем данные пользователя-автора комментария
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get comment author for notification", err, "user_id", userID)
		return
	}
	
	// Создаем событие для отправки уведомления
	notificationEvent := &messaging.NotificationEvent{
		UserIDs:    recipients,
		Title:      "New comment on task: " + task.Title,
		Content:    user.FullName() + " commented: " + comment.Content,
		Type:       string(domain.NotificationTypeTaskCommented),
		EntityID:   comment.ID,
		EntityType: "comment",
		CreatedAt:  time.Now(),
		MetaData: map[string]string{
			"task_id":     task.ID,
			"task_title":  task.Title,
			"comment_id":  comment.ID,
			"user_id":     userID,
			"user_name":   user.FullName(),
			"project_id":  task.ProjectID,
		},
	}
	
	if err := s.producer.PublishNotificationEvent(ctx, messaging.EventTypeNotification, notificationEvent); err != nil {
		s.logger.Error("Failed to publish notification event", err, "comment_id", comment.ID)
	}
}