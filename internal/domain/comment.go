package domain

import (
	"time"
)

// Comment представляет модель комментария к задаче
type Comment struct {
	ID        string    `json:"id" db:"id"`
	TaskID    string    `json:"task_id" db:"task_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Content   string    `json:"content" db:"content"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CommentCreateRequest представляет данные для создания комментария
type CommentCreateRequest struct {
	TaskID  string `json:"task_id" validate:"required,uuid"`
	Content string `json:"content" validate:"required,min=1"`
}

// CommentUpdateRequest представляет данные для обновления комментария
type CommentUpdateRequest struct {
	Content string `json:"content" validate:"required,min=1"`
}

// CommentResponse представляет данные комментария для API-ответов
type CommentResponse struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	UserID    string    `json:"user_id"`
	User      UserBrief `json:"user"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToResponse преобразует Comment в CommentResponse
func (c *Comment) ToResponse(user UserBrief) CommentResponse {
	return CommentResponse{
		ID:        c.ID,
		TaskID:    c.TaskID,
		UserID:    c.UserID,
		User:      user,
		Content:   c.Content,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// CommentFilterOptions представляет параметры для фильтрации комментариев
type CommentFilterOptions struct {
	TaskID    *string    `json:"task_id,omitempty"`
	UserID    *string    `json:"user_id,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}