package repository

import (
	"context"
	"time"

	"github.com/nurlyy/task_manager/internal/domain"
)

// NotificationRepository определяет интерфейс для работы с хранилищем уведомлений
type NotificationRepository interface {
	// Create создает новое уведомление
	Create(ctx context.Context, notification *domain.Notification) error

	// CreateBatch создает несколько уведомлений за раз
	CreateBatch(ctx context.Context, notifications []*domain.Notification) error

	// GetByID возвращает уведомление по ID
	GetByID(ctx context.Context, id string) (*domain.Notification, error)

	// Update обновляет данные уведомления
	Update(ctx context.Context, notification *domain.Notification) error

	// Delete удаляет уведомление по ID (soft delete)
	Delete(ctx context.Context, id string) error

	// GetUserNotifications возвращает уведомления пользователя
	GetUserNotifications(ctx context.Context, userID string, filter NotificationFilter) ([]*domain.Notification, error)

	// CountUserNotifications возвращает количество уведомлений пользователя
	CountUserNotifications(ctx context.Context, userID string, filter NotificationFilter) (int, error)

	// MarkAsRead отмечает уведомление как прочитанное
	MarkAsRead(ctx context.Context, id string) error

	// MarkAllAsRead отмечает все уведомления пользователя как прочитанные
	MarkAllAsRead(ctx context.Context, userID string) error

	// DeleteAllByUser удаляет все уведомления пользователя
	DeleteAllByUser(ctx context.Context, userID string) error

	// GetUserUnreadCount возвращает количество непрочитанных уведомлений пользователя
	GetUserUnreadCount(ctx context.Context, userID string) (int, error)

	// GetUserNotificationSettings возвращает настройки уведомлений пользователя
	GetUserNotificationSettings(ctx context.Context, userID string) ([]*NotificationSetting, error)

	// UpdateUserNotificationSettings обновляет настройки уведомлений пользователя
	UpdateUserNotificationSettings(ctx context.Context, userID string, settings []*NotificationSetting) error
}

// NotificationSetting представляет настройки уведомлений для пользователя
type NotificationSetting struct {
	UserID           string                   `json:"user_id" db:"user_id"`
	NotificationType domain.NotificationType  `json:"notification_type" db:"notification_type"`
	EmailEnabled     bool                     `json:"email_enabled" db:"email_enabled"`
	WebEnabled       bool                     `json:"web_enabled" db:"web_enabled"`
	TelegramEnabled  bool                     `json:"telegram_enabled" db:"telegram_enabled"`
}

// NotificationFilter содержит параметры для фильтрации уведомлений
type NotificationFilter struct {
	IDs         []string                   `json:"ids,omitempty"`
	Types       []domain.NotificationType  `json:"types,omitempty"`
	Status      *domain.NotificationStatus `json:"status,omitempty"`
	EntityID    *string                    `json:"entity_id,omitempty"`
	EntityType  *string                    `json:"entity_type,omitempty"`
	StartDate   *time.Time                 `json:"start_date,omitempty"`
	EndDate     *time.Time                 `json:"end_date,omitempty"`
	OrderBy     *string                    `json:"order_by,omitempty"`
	OrderDir    *string                    `json:"order_dir,omitempty"`
	Limit       int                        `json:"limit"`
	Offset      int                        `json:"offset"`
}