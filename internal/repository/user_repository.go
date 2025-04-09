package repository

import (
	"context"

	"github.com/nurlyy/task_manager/internal/domain"
)

// ProjectRepository определяет интерфейс для работы с хранилищем проектов
type ProjectRepository interface {
	// Create создает новый проект
	Create(ctx context.Context, project *domain.Project) error

	// GetByID возвращает проект по ID
	GetByID(ctx context.Context, id string) (*domain.Project, error)

	// Update обновляет данные проекта
	Update(ctx context.Context, project *domain.Project) error

	// Delete удаляет проект по ID
	Delete(ctx context.Context, id string) error

	// List возвращает список проектов с фильтрацией
	List(ctx context.Context, filter ProjectFilter) ([]*domain.Project, error)

	// Count возвращает количество проектов с фильтрацией
	Count(ctx context.Context, filter ProjectFilter) (int, error)

	// AddMember добавляет пользователя в проект
	AddMember(ctx context.Context, member *domain.ProjectMember) error

	// UpdateMember обновляет роль пользователя в проекте
	UpdateMember(ctx context.Context, projectID, userID string, role domain.ProjectRole) error

	// RemoveMember удаляет пользователя из проекта
	RemoveMember(ctx context.Context, projectID, userID string) error

	// GetMembers возвращает список участников проекта
	GetMembers(ctx context.Context, projectID string) ([]*domain.ProjectMember, error)

	// GetMember возвращает информацию об участнике проекта
	GetMember(ctx context.Context, projectID, userID string) (*domain.ProjectMember, error)

	// GetUserProjects возвращает список проектов пользователя
	GetUserProjects(ctx context.Context, userID string, filter ProjectFilter) ([]*domain.Project, error)

	// CountUserProjects возвращает количество проектов пользователя
	CountUserProjects(ctx context.Context, userID string, filter ProjectFilter) (int, error)
}

// ProjectFilter содержит параметры для фильтрации проектов
type ProjectFilter struct {
	IDs        []string              `json:"ids,omitempty"`
	Status     *domain.ProjectStatus `json:"status,omitempty"`
	CreatedBy  *string               `json:"created_by,omitempty"`
	StartAfter *string               `json:"start_after,omitempty"`
	EndBefore  *string               `json:"end_before,omitempty"`
	SearchText *string               `json:"search_text,omitempty"`
	OrderBy    *string               `json:"order_by,omitempty"`
	OrderDir   *string               `json:"order_dir,omitempty"`
	Limit      int                   `json:"limit"`
	Offset     int                   `json:"offset"`
	MemberID   *string               `json:"member_id,omitempty"`
}
