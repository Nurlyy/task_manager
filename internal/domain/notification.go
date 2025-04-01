package domain

import (
	"time"
)

// NotificationType определяет тип уведомления
type NotificationType string

const (
	// NotificationTypeTaskAssigned - задача назначена
	NotificationTypeTaskAssigned NotificationType = "task_assigned"
	// NotificationTypeTaskUpdated - задача обновлена
	NotificationTypeTaskUpdated NotificationType = "task_updated"
	// NotificationTypeTaskCommented - добавлен комментарий к задаче
	NotificationTypeTaskCommented NotificationType = "task_commented"
	// NotificationTypeTaskDueSoon - срок выполнения задачи скоро истекает
	NotificationTypeTaskDueSoon NotificationType = "task_due_soon"
	// NotificationTypeTaskOverdue - срок выполнения задачи истек
	NotificationTypeTaskOverdue NotificationType = "task_overdue"
	// NotificationTypeProjectMemberAdded - добавлен участник проекта
	NotificationTypeProjectMemberAdded NotificationType = "project_member_added"
	// NotificationTypeProjectUpdated - проект обновлен
	NotificationTypeProjectUpdated NotificationType = "project_updated"
	// NotificationTypeDigest - ежедневный дайджест задач
	NotificationTypeDigest NotificationType = "digest"
)

// NotificationStatus определяет статус уведомления
type NotificationStatus string

const (
	// NotificationStatusUnread - непрочитанное уведомление
	NotificationStatusUnread NotificationStatus = "unread"
	// NotificationStatusRead - прочитанное уведомление
	NotificationStatusRead NotificationStatus = "read"
	// NotificationStatusDeleted - удаленное уведомление
	NotificationStatusDeleted NotificationStatus = "deleted"
)

// Notification представляет модель уведомления
type Notification struct {
	ID         string             `json:"id" db:"id"`
	UserID     string             `json:"user_id" db:"user_id"`
	Type       NotificationType   `json:"type" db:"type"`
	Title      string             `json:"title" db:"title"`
	Content    string             `json:"content" db:"content"`
	Status     NotificationStatus `json:"status" db:"status"`
	EntityID   string             `json:"entity_id" db:"entity_id"`     // ID связанной сущности (задачи, проекта)
	EntityType string             `json:"entity_type" db:"entity_type"` // Тип связанной сущности (task, project)
	MetaData   map[string]string  `json:"meta_data,omitempty" db:"-"`   // Дополнительные данные
	CreatedAt  time.Time          `json:"created_at" db:"created_at"`
	ReadAt     *time.Time         `json:"read_at,omitempty" db:"read_at"`
}

// NotificationCreateRequest представляет данные для создания уведомления
type NotificationCreateRequest struct {
	UserID     string            `json:"user_id" validate:"required,uuid"`
	Type       NotificationType  `json:"type" validate:"required"`
	Title      string            `json:"title" validate:"required"`
	Content    string            `json:"content" validate:"required"`
	EntityID   string            `json:"entity_id" validate:"required"`
	EntityType string            `json:"entity_type" validate:"required,oneof=task project comment user"`
	MetaData   map[string]string `json:"meta_data,omitempty"`
}

// NotificationResponse представляет данные уведомления для API-ответов
type NotificationResponse struct {
	ID         string             `json:"id"`
	Type       NotificationType   `json:"type"`
	Title      string             `json:"title"`
	Content    string             `json:"content"`
	Status     NotificationStatus `json:"status"`
	EntityID   string             `json:"entity_id"`
	EntityType string             `json:"entity_type"`
	MetaData   map[string]string  `json:"meta_data,omitempty"`
	CreatedAt  time.Time          `json:"created_at"`
	ReadAt     *time.Time         `json:"read_at,omitempty"`
}

// ToResponse преобразует Notification в NotificationResponse
func (n *Notification) ToResponse() NotificationResponse {
	return NotificationResponse{
		ID:         n.ID,
		Type:       n.Type,
		Title:      n.Title,
		Content:    n.Content,
		Status:     n.Status,
		EntityID:   n.EntityID,
		EntityType: n.EntityType,
		MetaData:   n.MetaData,
		CreatedAt:  n.CreatedAt,
		ReadAt:     n.ReadAt,
	}
}

// MarkAsRead отмечает уведомление как прочитанное
func (n *Notification) MarkAsRead() {
	n.Status = NotificationStatusRead
	now := time.Now()
	n.ReadAt = &now
}

// IsRead проверяет, прочитано ли уведомление
func (n *Notification) IsRead() bool {
	return n.Status == NotificationStatusRead
}

// NotificationFilterOptions представляет параметры для фильтрации уведомлений
type NotificationFilterOptions struct {
	UserID     *string            `json:"user_id,omitempty"`
	Type       *NotificationType  `json:"type,omitempty"`
	Status     *NotificationStatus `json:"status,omitempty"`
	EntityID   *string            `json:"entity_id,omitempty"`
	EntityType *string            `json:"entity_type,omitempty"`
	StartDate  *time.Time         `json:"start_date,omitempty"`
	EndDate    *time.Time         `json:"end_date,omitempty"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
}

// NotificationEvent представляет событие для генерации уведомления
type NotificationEvent struct {
	Type       NotificationType  `json:"type"`
	UserIDs    []string          `json:"user_ids"` // Список получателей
	Title      string            `json:"title"`
	Content    string            `json:"content"`
	EntityID   string            `json:"entity_id"`
	EntityType string            `json:"entity_type"`
	MetaData   map[string]string `json:"meta_data,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}