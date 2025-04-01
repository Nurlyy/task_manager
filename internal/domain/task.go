package domain

import (
	"time"
)

// TaskStatus определяет статус задачи
type TaskStatus string

const (
	// TaskStatusNew - новая задача
	TaskStatusNew TaskStatus = "new"
	// TaskStatusInProgress - задача в процессе выполнения
	TaskStatusInProgress TaskStatus = "in_progress"
	// TaskStatusOnHold - задача на паузе
	TaskStatusOnHold TaskStatus = "on_hold"
	// TaskStatusReview - задача на проверке
	TaskStatusReview TaskStatus = "review"
	// TaskStatusCompleted - завершенная задача
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusCancelled - отмененная задача
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskPriority определяет приоритет задачи
type TaskPriority string

const (
	// TaskPriorityLow - низкий приоритет
	TaskPriorityLow TaskPriority = "low"
	// TaskPriorityMedium - средний приоритет
	TaskPriorityMedium TaskPriority = "medium"
	// TaskPriorityHigh - высокий приоритет
	TaskPriorityHigh TaskPriority = "high"
	// TaskPriorityCritical - критический приоритет
	TaskPriorityCritical TaskPriority = "critical"
)

// Task представляет модель задачи
type Task struct {
	ID           string       `json:"id" db:"id"`
	Title        string       `json:"title" db:"title"`
	Description  string       `json:"description" db:"description"`
	ProjectID    string       `json:"project_id" db:"project_id"`
	Status       TaskStatus   `json:"status" db:"status"`
	Priority     TaskPriority `json:"priority" db:"priority"`
	AssigneeID   *string      `json:"assignee_id,omitempty" db:"assignee_id"`
	CreatedBy    string       `json:"created_by" db:"created_by"`
	DueDate      *time.Time   `json:"due_date,omitempty" db:"due_date"`
	EstimatedHours *float64   `json:"estimated_hours,omitempty" db:"estimated_hours"`
	SpentHours   *float64     `json:"spent_hours,omitempty" db:"spent_hours"`
	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at" db:"updated_at"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty" db:"completed_at"`
	Tags         []string     `json:"tags,omitempty" db:"-"` // Теги хранятся в отдельной таблице
}

// TaskHistory представляет запись об изменении задачи
type TaskHistory struct {
	ID           string    `json:"id" db:"id"`
	TaskID       string    `json:"task_id" db:"task_id"`
	UserID       string    `json:"user_id" db:"user_id"`
	Field        string    `json:"field" db:"field"`
	OldValue     string    `json:"old_value" db:"old_value"`
	NewValue     string    `json:"new_value" db:"new_value"`
	ChangedAt    time.Time `json:"changed_at" db:"changed_at"`
}

// TaskCreateRequest представляет данные для создания задачи
type TaskCreateRequest struct {
	Title        string       `json:"title" validate:"required,min=3,max=200"`
	Description  string       `json:"description" validate:"required"`
	ProjectID    string       `json:"project_id" validate:"required,uuid"`
	Priority     TaskPriority `json:"priority" validate:"required,oneof=low medium high critical"`
	AssigneeID   *string      `json:"assignee_id,omitempty" validate:"omitempty,uuid"`
	DueDate      *time.Time   `json:"due_date,omitempty"`
	EstimatedHours *float64   `json:"estimated_hours,omitempty" validate:"omitempty,gte=0"`
	Tags         []string     `json:"tags,omitempty" validate:"omitempty,dive,min=1,max=50"`
}

// TaskUpdateRequest представляет данные для обновления задачи
type TaskUpdateRequest struct {
	Title        *string       `json:"title,omitempty" validate:"omitempty,min=3,max=200"`
	Description  *string       `json:"description,omitempty"`
	Status       *TaskStatus   `json:"status,omitempty" validate:"omitempty,oneof=new in_progress on_hold review completed cancelled"`
	Priority     *TaskPriority `json:"priority,omitempty" validate:"omitempty,oneof=low medium high critical"`
	AssigneeID   *string       `json:"assignee_id,omitempty" validate:"omitempty,uuid"`
	DueDate      *time.Time    `json:"due_date,omitempty"`
	EstimatedHours *float64    `json:"estimated_hours,omitempty" validate:"omitempty,gte=0"`
	SpentHours   *float64      `json:"spent_hours,omitempty" validate:"omitempty,gte=0"`
	Tags         *[]string     `json:"tags,omitempty" validate:"omitempty,dive,min=1,max=50"`
}

// TaskResponse представляет данные задачи для API-ответов
type TaskResponse struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	ProjectID    string       `json:"project_id"`
	Status       TaskStatus   `json:"status"`
	Priority     TaskPriority `json:"priority"`
	AssigneeID   *string      `json:"assignee_id,omitempty"`
	Assignee     *UserBrief   `json:"assignee,omitempty"`
	CreatedBy    string       `json:"created_by"`
	Creator      *UserBrief   `json:"creator,omitempty"`
	DueDate      *time.Time   `json:"due_date,omitempty"`
	EstimatedHours *float64   `json:"estimated_hours,omitempty"`
	SpentHours   *float64     `json:"spent_hours,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	Tags         []string     `json:"tags,omitempty"`
	Comments     []CommentResponse `json:"comments,omitempty"`
	History      []TaskHistoryResponse `json:"history,omitempty"`
}

// UserBrief представляет краткую информацию о пользователе
type UserBrief struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Avatar    *string `json:"avatar,omitempty"`
}

// TaskHistoryResponse представляет историю изменения задачи для API-ответов
type TaskHistoryResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	User      UserBrief `json:"user"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	ChangedAt time.Time `json:"changed_at"`
}

// ToResponse преобразует Task в TaskResponse
func (t *Task) ToResponse() TaskResponse {
	return TaskResponse{
		ID:            t.ID,
		Title:         t.Title,
		Description:   t.Description,
		ProjectID:     t.ProjectID,
		Status:        t.Status,
		Priority:      t.Priority,
		AssigneeID:    t.AssigneeID,
		CreatedBy:     t.CreatedBy,
		DueDate:       t.DueDate,
		EstimatedHours: t.EstimatedHours,
		SpentHours:    t.SpentHours,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
		CompletedAt:   t.CompletedAt,
		Tags:          t.Tags,
	}
}

// IsCompleted проверяет, завершена ли задача
func (t *Task) IsCompleted() bool {
	return t.Status == TaskStatusCompleted
}

// IsOverdue проверяет, просрочена ли задача
func (t *Task) IsOverdue() bool {
	if t.DueDate == nil || t.IsCompleted() {
		return false
	}
	return time.Now().After(*t.DueDate)
}

// TaskTag представляет связь задачи с тегом
type TaskTag struct {
	TaskID string `json:"task_id" db:"task_id"`
	Tag    string `json:"tag" db:"tag"`
}

// AddCommentRequest представляет запрос на добавление комментария к задаче
type AddCommentRequest struct {
	Content string `json:"content" validate:"required,min=1"`
}

// LogTimeRequest представляет запрос на добавление затраченного времени
type LogTimeRequest struct {
	Hours       float64    `json:"hours" validate:"required,gt=0"`
	Description string     `json:"description" validate:"required"`
	Date        *time.Time `json:"date,omitempty"`
}

// TaskFilterOptions представляет параметры для фильтрации задач
type TaskFilterOptions struct {
	ProjectID  *string       `json:"project_id,omitempty"`
	Status     *TaskStatus   `json:"status,omitempty"`
	Priority   *TaskPriority `json:"priority,omitempty"`
	AssigneeID *string       `json:"assignee_id,omitempty"`
	CreatedBy  *string       `json:"created_by,omitempty"`
	DueBefore  *time.Time    `json:"due_before,omitempty"`
	DueAfter   *time.Time    `json:"due_after,omitempty"`
	Tags       []string      `json:"tags,omitempty"`
	SearchText *string       `json:"search_text,omitempty"`
	SortBy     *string       `json:"sort_by,omitempty"`
	SortOrder  *string       `json:"sort_order,omitempty"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
}