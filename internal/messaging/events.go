package messaging

import (
	"time"
)

// Типы событий
const (
	EventTypeTaskCreated        = "task_created"
	EventTypeTaskUpdated        = "task_updated"
	EventTypeTaskAssigned       = "task_assigned"
	EventTypeTaskCommented      = "task_commented"
	EventTypeProjectCreated     = "project_created"
	EventTypeProjectUpdated     = "project_updated"
	EventTypeProjectMemberAdded = "project_member_added"
	EventTypeNotification       = "notification"
)

// Event представляет базовое событие
type Event struct {
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

// TaskEvent представляет событие, связанное с задачей
type TaskEvent struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	ProjectID   string                 `json:"project_id"`
	Status      string                 `json:"status"`
	Priority    string                 `json:"priority"`
	AssigneeID  *string                `json:"assignee_id,omitempty"`
	CreatedBy   string                 `json:"created_by,omitempty"`
	AssignerID  string                 `json:"assigner_id,omitempty"`
	DueDate     *time.Time             `json:"due_date,omitempty"`
	CreatedAt   time.Time              `json:"created_at,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Type        string                 `json:"type"`
	Changes     map[string]interface{} `json:"changes,omitempty"`
}

// CommentEvent представляет событие, связанное с комментарием
type CommentEvent struct {
	TaskID    string    `json:"task_id"`
	TaskTitle string    `json:"task_title"`
	CommentID string    `json:"comment_id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Type      string    `json:"type"`
}

// ProjectEvent представляет событие, связанное с проектом
type ProjectEvent struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Status      string                 `json:"status"`
	CreatedBy   string                 `json:"created_by,omitempty"`
	CreatedAt   time.Time              `json:"created_at,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Type        string                 `json:"type"`
	Changes     map[string]interface{} `json:"changes,omitempty"`
}

// ProjectMemberEvent представляет событие, связанное с участником проекта
type ProjectMemberEvent struct {
	ProjectID   string    `json:"project_id"`
	ProjectName string    `json:"project_name"`
	UserID      string    `json:"user_id"`
	Role        string    `json:"role"`
	InvitedBy   string    `json:"invited_by"`
	JoinedAt    time.Time `json:"joined_at"`
	Type        string    `json:"type"`
}

// NotificationEvent представляет событие уведомления
type NotificationEvent struct {
	UserIDs     []string              `json:"user_ids"`
	Title       string                `json:"title"`
	Content     string                `json:"content"`
	Type        string                `json:"type"`
	EntityID    string                `json:"entity_id"`
	EntityType  string                `json:"entity_type"`
	CreatedAt   time.Time             `json:"created_at"`
	MetaData    map[string]string     `json:"meta_data,omitempty"`
}