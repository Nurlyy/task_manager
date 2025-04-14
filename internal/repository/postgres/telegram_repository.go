package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// TelegramRepository реализует репозиторий связей пользователей с Telegram с использованием PostgreSQL
type TelegramRepository struct {
	db     *sqlx.DB
	logger logger.Logger
}

// NewTelegramRepository создает новый экземпляр TelegramRepository
func NewTelegramRepository(db *sqlx.DB, logger logger.Logger) *TelegramRepository {
	return &TelegramRepository{
		db:     db,
		logger: logger,
	}
}

// CreateOrUpdate создает или обновляет связь пользователя с Telegram
func (r *TelegramRepository) CreateOrUpdate(ctx context.Context, link *repository.TelegramLink) error {
	query := `
		INSERT INTO telegram_links (
			user_id, telegram_id, chat_id, username, first_name, last_name, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		) ON CONFLICT (user_id) 
		DO UPDATE SET 
			telegram_id = $2,
			chat_id = $3,
			username = $4,
			first_name = $5,
			last_name = $6,
			updated_at = $8
		RETURNING user_id
	`

	now := time.Now()

	if link.CreatedAt == "" {
		link.CreatedAt = now.Format(time.RFC3339)
	}

	link.UpdatedAt = now.Format(time.RFC3339)

	err := r.db.QueryRowxContext(
		ctx,
		query,
		link.UserID,
		link.TelegramID,
		link.ChatID,
		link.Username,
		link.FirstName,
		link.LastName,
		link.CreatedAt,
		link.UpdatedAt,
	).Scan(&link.UserID)

	if err != nil {
		r.logger.Error("Failed to create or update telegram link", err, map[string]interface{}{
			"user_id":     link.UserID,
			"telegram_id": link.TelegramID,
		})
		return fmt.Errorf("failed to create or update telegram link: %w", err)
	}

	return nil
}

// GetByUserID возвращает связь пользователя с Telegram по ID пользователя
func (r *TelegramRepository) GetByUserID(ctx context.Context, userID string) (*repository.TelegramLink, error) {
	query := `
		SELECT 
			user_id, telegram_id, chat_id, username, first_name, last_name, created_at, updated_at
		FROM telegram_links 
		WHERE user_id = $1
	`

	var link repository.TelegramLink
	err := r.db.GetContext(ctx, &link, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get telegram link by user ID", err, map[string]interface{}{
			"user_id": userID,
		})
		return nil, fmt.Errorf("failed to get telegram link by user ID: %w", err)
	}

	return &link, nil
}

// GetByTelegramID возвращает связь пользователя с Telegram по Telegram ID
func (r *TelegramRepository) GetByTelegramID(ctx context.Context, telegramID string) (*repository.TelegramLink, error) {
	query := `
		SELECT 
			user_id, telegram_id, chat_id, username, first_name, last_name, created_at, updated_at
		FROM telegram_links 
		WHERE telegram_id = $1
	`

	var link repository.TelegramLink
	err := r.db.GetContext(ctx, &link, query, telegramID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get telegram link by telegram ID", err, map[string]interface{}{
			"telegram_id": telegramID,
		})
		return nil, fmt.Errorf("failed to get telegram link by telegram ID: %w", err)
	}

	return &link, nil
}

// Delete удаляет связь пользователя с Telegram
func (r *TelegramRepository) Delete(ctx context.Context, userID string) error {
	query := `DELETE FROM telegram_links WHERE user_id = $1`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to delete telegram link", err, map[string]interface{}{
			"user_id": userID,
		})
		return fmt.Errorf("failed to delete telegram link: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("telegram link not found")
	}

	return nil
}

// List возвращает список связей пользователей с Telegram
func (r *TelegramRepository) List(ctx context.Context, limit, offset int) ([]*repository.TelegramLink, error) {
	query := `
		SELECT 
			user_id, telegram_id, chat_id, username, first_name, last_name, created_at, updated_at
		FROM telegram_links
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	links := []*repository.TelegramLink{}
	err := r.db.SelectContext(ctx, &links, query, limit, offset)
	if err != nil {
		r.logger.Error("Failed to list telegram links", err, map[string]interface{}{
			"limit":  limit,
			"offset": offset,
		})
		return nil, fmt.Errorf("failed to list telegram links: %w", err)
	}

	return links, nil
}

// Count возвращает количество связей пользователей с Telegram
func (r *TelegramRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM telegram_links`

	var count int
	err := r.db.GetContext(ctx, &count, query)
	if err != nil {
		r.logger.Error("Failed to count telegram links", err)
		return 0, fmt.Errorf("failed to count telegram links: %w", err)
	}

	return count, nil
}
