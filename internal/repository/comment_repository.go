package repository

import (
	"context"

	"github.com/nurlyy/task_manager/internal/domain"
)

// CommentRepository определяет интерфейс для работы с хранилищем комментариев
type CommentRepository interface {
	// Create создает новый комментарий
	Create(ctx context.Context, comment *domain.Comment) error

	// GetByID возвращает комментарий по ID
	GetByID(ctx context.Context, id string) (*domain.Comment, error)

	// Update обновляет данные комментария
	Update(ctx context.Context, comment *domain.Comment) error

	// Delete удаляет комментарий по ID
	Delete(ctx context.Context, id string) error

	// List возвращает список комментариев с фильтрацией
	List(ctx context.Context, filter CommentFilter) ([]*domain.Comment, error)

	// Count возвращает количество комментариев с фильтрацией
	Count(ctx context.Context, filter CommentFilter) (int, error)

	// GetCommentsByTask возвращает комментарии к задаче
	GetCommentsByTask(ctx context.Context, taskID string, filter CommentFilter) ([]*domain.Comment, error)

	// CountCommentsByTask возвращает количество комментариев к задаче
	CountCommentsByTask(ctx context.Context, taskID string) (int, error)

	// GetCommentsByUser возвращает комментарии пользователя
	GetCommentsByUser(ctx context.Context, userID string, filter CommentFilter) ([]*domain.Comment, error)

	// CountCommentsByUser возвращает количество комментариев пользователя
	CountCommentsByUser(ctx context.Context, userID string) (int, error)
}

// CommentFilter содержит параметры для фильтрации комментариев
type CommentFilter struct {
	IDs        []string `json:"ids,omitempty"`
	TaskIDs    []string `json:"task_ids,omitempty"`
	UserIDs    []string `json:"user_ids,omitempty"`
	SearchText *string  `json:"search_text,omitempty"`
	OrderBy    *string  `json:"order_by,omitempty"`
	OrderDir   *string  `json:"order_dir,omitempty"`
	Limit      int      `json:"limit"`
	Offset     int      `json:"offset"`
}