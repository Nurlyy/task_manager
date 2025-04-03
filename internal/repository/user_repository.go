package repository

import (
	"context"

	"github.com/nurlyy/task_manager/internal/domain"
)

// UserRepository определяет интерфейс для работы с хранилищем пользователей
type UserRepository interface {
	// Create создает нового пользователя
	Create(ctx context.Context, user *domain.User) error

	// GetByID возвращает пользователя по ID
	GetByID(ctx context.Context, id string) (*domain.User, error)

	// GetByEmail возвращает пользователя по email
	GetByEmail(ctx context.Context, email string) (*domain.User, error)

	// Update обновляет данные пользователя
	Update(ctx context.Context, user *domain.User) error

	// Delete удаляет пользователя по ID
	Delete(ctx context.Context, id string) error

	// List возвращает список пользователей с фильтрацией
	List(ctx context.Context, filter UserFilter) ([]*domain.User, error)

	// Count возвращает количество пользователей с фильтрацией
	Count(ctx context.Context, filter UserFilter) (int, error)

	// UpdateLastLogin обновляет время последнего входа пользователя
	UpdateLastLogin(ctx context.Context, id string) error
}

// UserFilter содержит параметры для фильтрации пользователей
type UserFilter struct {
	IDs        []string        `json:"ids,omitempty"`
	Email      *string         `json:"email,omitempty"`
	Role       *domain.UserRole `json:"role,omitempty"`
	IsActive   *bool           `json:"is_active,omitempty"`
	Department *string         `json:"department,omitempty"`
	SearchText *string         `json:"search_text,omitempty"`
	OrderBy    *string         `json:"order_by,omitempty"`
	OrderDir   *string         `json:"order_dir,omitempty"`
	Limit      int             `json:"limit"`
	Offset     int             `json:"offset"`
}