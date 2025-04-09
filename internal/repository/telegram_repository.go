package repository

import (
	"context"
)

// TelegramLink представляет запись связи пользователя с Telegram
type TelegramLink struct {
	UserID     string `json:"user_id" db:"user_id"`
	TelegramID string `json:"telegram_id" db:"telegram_id"`
	ChatID     string `json:"chat_id" db:"chat_id"`
	Username   string `json:"username,omitempty" db:"username"`
	FirstName  string `json:"first_name,omitempty" db:"first_name"`
	LastName   string `json:"last_name,omitempty" db:"last_name"`
	CreatedAt  string `json:"created_at" db:"created_at"`
	UpdatedAt  string `json:"updated_at" db:"updated_at"`
}

// TelegramRepository определяет интерфейс для работы с хранилищем связей пользователей с Telegram
type TelegramRepository interface {
	// CreateOrUpdate создает или обновляет связь пользователя с Telegram
	CreateOrUpdate(ctx context.Context, link *TelegramLink) error

	// GetByUserID возвращает связь пользователя с Telegram по ID пользователя
	GetByUserID(ctx context.Context, userID string) (*TelegramLink, error)

	// GetByTelegramID возвращает связь пользователя с Telegram по Telegram ID
	GetByTelegramID(ctx context.Context, telegramID string) (*TelegramLink, error)

	// Delete удаляет связь пользователя с Telegram
	Delete(ctx context.Context, userID string) error

	// List возвращает список связей пользователей с Telegram
	List(ctx context.Context, limit, offset int) ([]*TelegramLink, error)

	// Count возвращает количество связей пользователей с Telegram
	Count(ctx context.Context) (int, error)
}
