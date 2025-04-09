package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/internal/repository/cache"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// Стандартные ошибки
var (
	ErrNotificationNotFound = errors.New("notification not found")
)

// NotificationService представляет бизнес-логику для работы с уведомлениями
type NotificationService struct {
	repo      repository.NotificationRepository
	userRepo  repository.UserRepository
	cacheRepo *cache.RedisRepository
	logger    logger.Logger
}

// NewNotificationService создает новый экземпляр NotificationService
func NewNotificationService(
	repo repository.NotificationRepository,
	userRepo repository.UserRepository,
	cacheRepo *cache.RedisRepository,
	logger logger.Logger,
) *NotificationService {
	return &NotificationService{
		repo:      repo,
		userRepo:  userRepo,
		cacheRepo: cacheRepo,
		logger:    logger,
	}
}

// Create создает новое уведомление
func (s *NotificationService) Create(ctx context.Context, req domain.NotificationCreateRequest) (*domain.NotificationResponse, error) {
	// Проверяем, существует ли пользователь
	if _, err := s.userRepo.GetByID(ctx, req.UserID); err != nil {
		s.logger.Error("Failed to get user by ID for notification creation", err, map[string]interface{}{
			"user_id": req.UserID,
		})
		return nil, ErrUserNotFound
	}

	// Создаем новое уведомление
	notification := &domain.Notification{
		ID:         uuid.New().String(),
		UserID:     req.UserID,
		Type:       req.Type,
		Title:      req.Title,
		Content:    req.Content,
		Status:     domain.NotificationStatusUnread,
		EntityID:   req.EntityID,
		EntityType: req.EntityType,
		MetaData:   req.MetaData,
		CreatedAt:  time.Now(),
	}

	// Сохраняем уведомление в БД
	if err := s.repo.Create(ctx, notification); err != nil {
		s.logger.Error("Failed to create notification", err)
		return nil, err
	}

	// Удаляем счетчик непрочитанных уведомлений из кэша
	cacheKey := "unread_count:" + req.UserID
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete unread count from cache", map[string]interface{}{
			"user_id": req.UserID,
		}, map[string]interface{}{
			"error": err,
		})
	}

	resp := notification.ToResponse()
	return &resp, nil
}

// CreateBatch создает несколько уведомлений за один раз
func (s *NotificationService) CreateBatch(ctx context.Context, requests []domain.NotificationCreateRequest) error {
	if len(requests) == 0 {
		return nil
	}

	notifications := make([]*domain.Notification, len(requests))
	userIDs := make(map[string]struct{})

	for i, req := range requests {
		// Добавляем ID пользователя в множество для последующей очистки кэша
		userIDs[req.UserID] = struct{}{}

		notifications[i] = &domain.Notification{
			ID:         uuid.New().String(),
			UserID:     req.UserID,
			Type:       req.Type,
			Title:      req.Title,
			Content:    req.Content,
			Status:     domain.NotificationStatusUnread,
			EntityID:   req.EntityID,
			EntityType: req.EntityType,
			MetaData:   req.MetaData,
			CreatedAt:  time.Now(),
		}
	}

	// Сохраняем уведомления в БД
	if err := s.repo.CreateBatch(ctx, notifications); err != nil {
		s.logger.Error("Failed to create batch notifications", err)
		return err
	}

	// Удаляем счетчики непрочитанных уведомлений из кэша для всех пользователей
	for userID := range userIDs {
		cacheKey := "unread_count:" + userID
		if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
			s.logger.Warn("Failed to delete unread count from cache", map[string]interface{}{
				"user_id": userID,
			}, map[string]interface{}{
				"error": err,
			})
		}
	}

	return nil
}

// GetByID возвращает уведомление по ID
func (s *NotificationService) GetByID(ctx context.Context, id string, userID string) (*domain.NotificationResponse, error) {
	// Получаем уведомление из БД
	notification, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get notification by ID", err, map[string]interface{}{
			"id": id,
		})
		return nil, ErrNotificationNotFound
	}

	// Проверяем, принадлежит ли уведомление пользователю
	if notification.UserID != userID {
		return nil, ErrNotificationNotFound
	}

	resp := notification.ToResponse()
	return &resp, nil
}

// MarkAsRead отмечает уведомление как прочитанное
func (s *NotificationService) MarkAsRead(ctx context.Context, id string, userID string) error {
	// Получаем уведомление из БД
	notification, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get notification by ID for marking as read", err, map[string]interface{}{
			"id": id,
		})
		return ErrNotificationNotFound
	}

	// Проверяем, принадлежит ли уведомление пользователю
	if notification.UserID != userID {
		return ErrNotificationNotFound
	}

	// Если уведомление уже прочитано, ничего не делаем
	if notification.IsRead() {
		return nil
	}

	// Отмечаем уведомление как прочитанное
	if err := s.repo.MarkAsRead(ctx, id); err != nil {
		s.logger.Error("Failed to mark notification as read", err, map[string]interface{}{
			"id": id,
		})
		return err
	}

	// Удаляем счетчик непрочитанных уведомлений из кэша
	cacheKey := "unread_count:" + userID
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete unread count from cache", map[string]interface{}{
			"user_id": userID,
		}, map[string]interface{}{
			"error": err,
		})
	}

	return nil
}

// MarkAllAsRead отмечает все уведомления пользователя как прочитанные
func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID string) error {
	// Отмечаем все уведомления пользователя как прочитанные
	if err := s.repo.MarkAllAsRead(ctx, userID); err != nil {
		s.logger.Error("Failed to mark all notifications as read", err, map[string]interface{}{
			"user_id": userID,
		})
		return err
	}

	// Удаляем счетчик непрочитанных уведомлений из кэша
	cacheKey := "unread_count:" + userID
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete unread count from cache", map[string]interface{}{
			"user_id": userID,
		}, map[string]interface{}{
			"error": err,
		})
	}

	return nil
}

// Delete удаляет уведомление
func (s *NotificationService) Delete(ctx context.Context, id string, userID string) error {
	// Получаем уведомление из БД
	notification, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get notification by ID for delete", err, map[string]interface{}{
			"id": id,
		})
		return ErrNotificationNotFound
	}

	// Проверяем, принадлежит ли уведомление пользователю
	if notification.UserID != userID {
		return ErrNotificationNotFound
	}

	// Удаляем уведомление из БД
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete notification", err, map[string]interface{}{
			"id": id,
		})
		return err
	}

	// Если уведомление было непрочитанным, удаляем счетчик из кэша
	if !notification.IsRead() {
		cacheKey := "unread_count:" + userID
		if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
			s.logger.Warn("Failed to delete unread count from cache", map[string]interface{}{
				"user_id": userID,
			}, map[string]interface{}{
				"error": err,
			})
		}
	}

	return nil
}

// GetUserNotifications возвращает уведомления пользователя с фильтрацией
func (s *NotificationService) GetUserNotifications(ctx context.Context, userID string, filter domain.NotificationFilterOptions, page, pageSize int) (*domain.PagedResponse, error) {
	// Преобразуем фильтр доменной модели в фильтр репозитория
	repoFilter := repository.NotificationFilter{
		Status:     filter.Status,
		Types:      []domain.NotificationType{},
		EntityID:   filter.EntityID,
		EntityType: filter.EntityType,
		StartDate:  filter.StartDate,
		EndDate:    filter.EndDate,
		Limit:      pageSize,
		Offset:     (page - 1) * pageSize,
	}

	// Если указан тип, добавляем его в фильтр
	if filter.Type != nil {
		repoFilter.Types = append(repoFilter.Types, *filter.Type)
	}

	// Настройка сортировки (по умолчанию - по дате создания, сначала новые)
	orderBy := "created_at"
	orderDir := "desc"
	repoFilter.OrderBy = &orderBy
	repoFilter.OrderDir = &orderDir

	// Получаем уведомления пользователя
	notifications, err := s.repo.GetUserNotifications(ctx, userID, repoFilter)
	if err != nil {
		s.logger.Error("Failed to get user notifications", err, map[string]interface{}{
			"user_id": userID,
		})
		return nil, err
	}

	// Получаем общее количество уведомлений
	total, err := s.repo.CountUserNotifications(ctx, userID, repoFilter)
	if err != nil {
		s.logger.Error("Failed to count user notifications", err, map[string]interface{}{
			"user_id": userID,
		})
		return nil, err
	}

	// Преобразуем к NotificationResponse
	notificationResponses := make([]domain.NotificationResponse, len(notifications))
	for i, notification := range notifications {
		notificationResponses[i] = notification.ToResponse()
	}

	// Формируем ответ с пагинацией
	return &domain.PagedResponse{
		Items:      notificationResponses,
		TotalItems: total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}, nil
}

// GetUnreadCount возвращает количество непрочитанных уведомлений
func (s *NotificationService) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	// Пытаемся получить из кэша
	cacheKey := "unread_count:" + userID
	var count int
	if err := s.cacheRepo.Get(ctx, cacheKey, &count); err == nil {
		return count, nil
	}

	// Получаем количество непрочитанных уведомлений
	count, err := s.repo.GetUserUnreadCount(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get unread notification count", err, map[string]interface{}{
			"user_id": userID,
		})
		return 0, err
	}

	// Сохраняем в кэш
	if err := s.cacheRepo.Set(ctx, cacheKey, count); err != nil {
		s.logger.Warn("Failed to cache unread count", map[string]interface{}{
			"user_id": userID,
		}, map[string]interface{}{
			"error": err,
		})
	}

	return count, nil
}

// GetUserNotificationSettings возвращает настройки уведомлений пользователя
func (s *NotificationService) GetUserNotificationSettings(ctx context.Context, userID string) ([]*repository.NotificationSetting, error) {
	// Получаем настройки уведомлений пользователя
	settings, err := s.repo.GetUserNotificationSettings(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user notification settings", err, map[string]interface{}{
			"user_id": userID,
		})
		return nil, err
	}

	return settings, nil
}

// UpdateUserNotificationSettings обновляет настройки уведомлений пользователя
func (s *NotificationService) UpdateUserNotificationSettings(ctx context.Context, userID string, settings []*repository.NotificationSetting) error {
	// Проверяем, что все настройки принадлежат указанному пользователю
	for _, setting := range settings {
		if setting.UserID != userID {
			return errors.New("settings must belong to the specified user")
		}
	}

	// Обновляем настройки уведомлений
	if err := s.repo.UpdateUserNotificationSettings(ctx, userID, settings); err != nil {
		s.logger.Error("Failed to update user notification settings", err, map[string]interface{}{
			"user_id": userID,
		})
		return err
	}

	return nil
}
