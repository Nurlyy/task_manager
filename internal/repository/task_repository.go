package repository

import (
	"context"
	"time"

	"github.com/nurlyy/task_manager/internal/domain"
)

// TaskRepository определяет интерфейс для работы с хранилищем задач
type TaskRepository interface {
	// Create создает новую задачу
	Create(ctx context.Context, task *domain.Task) error

	// GetByID возвращает задачу по ID
	GetByID(ctx context.Context, id string) (*domain.Task, error)

	// Update обновляет данные задачи
	Update(ctx context.Context, task *domain.Task) error

	// Delete удаляет задачу по ID
	Delete(ctx context.Context, id string) error

	// List возвращает список задач с фильтрацией
	List(ctx context.Context, filter TaskFilter) ([]*domain.Task, error)

	// Count возвращает количество задач с фильтрацией
	Count(ctx context.Context, filter TaskFilter) (int, error)

	// GetTags возвращает теги задачи
	GetTags(ctx context.Context, taskID string) ([]string, error)

	// AddTag добавляет тег к задаче
	AddTag(ctx context.Context, taskID, tag string) error

	// RemoveTag удаляет тег из задачи
	RemoveTag(ctx context.Context, taskID, tag string) error

	// UpdateTags обновляет теги задачи
	UpdateTags(ctx context.Context, taskID string, tags []string) error

	// LogTaskHistory добавляет запись в историю изменений задачи
	LogTaskHistory(ctx context.Context, history *domain.TaskHistory) error

	// GetTaskHistory возвращает историю изменений задачи
	GetTaskHistory(ctx context.Context, taskID string) ([]*domain.TaskHistory, error)

	// GetTasksByProject возвращает задачи проекта
	GetTasksByProject(ctx context.Context, projectID string, filter TaskFilter) ([]*domain.Task, error)

	// GetTasksByAssignee возвращает задачи назначенные пользователю
	GetTasksByAssignee(ctx context.Context, userID string, filter TaskFilter) ([]*domain.Task, error)

	// CountTasksByProject возвращает количество задач в проекте
	CountTasksByProject(ctx context.Context, projectID string, filter TaskFilter) (int, error)

	// CountTasksByAssignee возвращает количество задач назначенных пользователю
	CountTasksByAssignee(ctx context.Context, userID string, filter TaskFilter) (int, error)

	// GetOverdueTasks возвращает просроченные задачи
	GetOverdueTasks(ctx context.Context, filter TaskFilter) ([]*domain.Task, error)

	// GetUpcomingTasks возвращает задачи с приближающимся сроком
	GetUpcomingTasks(ctx context.Context, daysThreshold int, filter TaskFilter) ([]*domain.Task, error)

	// UpdateStatus обновляет статус задачи
	UpdateStatus(ctx context.Context, taskID string, status domain.TaskStatus, userID string) error

	// UpdatePriority обновляет приоритет задачи
	UpdatePriority(ctx context.Context, taskID string, priority domain.TaskPriority, userID string) error

	// UpdateAssignee обновляет исполнителя задачи
	UpdateAssignee(ctx context.Context, taskID string, assigneeID *string, userID string) error

	// LogTime добавляет запись о затраченном времени
	LogTime(ctx context.Context, timeLog *TimeLog) error

	// GetTimeLogs возвращает записи о затраченном времени
	GetTimeLogs(ctx context.Context, taskID string) ([]*TimeLog, error)

	// GetTaskMetrics возвращает метрики по задачам
	GetTaskMetrics(ctx context.Context, projectID string) (*domain.ProjectMetrics, error)
}

// TaskFilter содержит параметры для фильтрации задач
type TaskFilter struct {
	IDs         []string           `json:"ids,omitempty"`
	ProjectIDs  []string           `json:"project_ids,omitempty"`
	Status      *domain.TaskStatus `json:"status,omitempty"`
	Priority    *domain.TaskPriority `json:"priority,omitempty"`
	AssigneeID  *string            `json:"assignee_id,omitempty"`
	CreatedBy   *string            `json:"created_by,omitempty"`
	DueBefore   *time.Time         `json:"due_before,omitempty"`
	DueAfter    *time.Time         `json:"due_after,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	SearchText  *string            `json:"search_text,omitempty"`
	IsOverdue   *bool              `json:"is_overdue,omitempty"`
	OrderBy     *string            `json:"order_by,omitempty"`
	OrderDir    *string            `json:"order_dir,omitempty"`
	Limit       int                `json:"limit"`
	Offset      int                `json:"offset"`
}

// TimeLog содержит информацию о затраченном времени
type TimeLog struct {
	ID          string    `json:"id" db:"id"`
	TaskID      string    `json:"task_id" db:"task_id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Hours       float64   `json:"hours" db:"hours"`
	Description string    `json:"description" db:"description"`
	LoggedAt    time.Time `json:"logged_at" db:"logged_at"`
	LogDate     time.Time `json:"log_date" db:"log_date"`
}