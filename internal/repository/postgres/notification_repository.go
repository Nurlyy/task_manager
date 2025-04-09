package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// NotificationRepository реализует репозиторий уведомлений с использованием PostgreSQL
type NotificationRepository struct {
	db     *sqlx.DB
	logger logger.Logger
}

// NewNotificationRepository создает новый экземпляр NotificationRepository
func NewNotificationRepository(db *sqlx.DB, logger logger.Logger) *NotificationRepository {
	return &NotificationRepository{
		db:     db,
		logger: logger,
	}
}

// Create создает новое уведомление
func (r *NotificationRepository) Create(ctx context.Context, notification *domain.Notification) error {
	query := `
		INSERT INTO notifications (
			id, user_id, type, title, content, status, entity_id, entity_type, meta_data, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		) RETURNING id
	`

	// Сериализуем метаданные в JSON
	metaDataJSON, err := json.Marshal(notification.MetaData)
	if err != nil {
		r.logger.Error("Failed to marshal meta data", err, map[string]interface{}{
			"notification_id": notification.ID,
		})
		return fmt.Errorf("failed to marshal meta data: %w", err)
	}

	err = r.db.QueryRowxContext(
		ctx,
		query,
		notification.ID,
		notification.UserID,
		notification.Type,
		notification.Title,
		notification.Content,
		notification.Status,
		notification.EntityID,
		notification.EntityType,
		metaDataJSON,
		notification.CreatedAt,
	).Scan(&notification.ID)

	if err != nil {
		r.logger.Error("Failed to create notification", err, map[string]interface{}{
			"user_id": notification.UserID,
			"type":    notification.Type,
		})
		return fmt.Errorf("failed to create notification: %w", err)
	}

	return nil
}

// CreateBatch создает несколько уведомлений за раз
func (r *NotificationRepository) CreateBatch(ctx context.Context, notifications []*domain.Notification) error {
	if len(notifications) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.logger.Error("Failed to rollback transaction", rbErr)
			}
			return
		}
	}()

	query := `
		INSERT INTO notifications (
			id, user_id, type, title, content, status, entity_id, entity_type, meta_data, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, notification := range notifications {
		// Сериализуем метаданные в JSON
		metaDataJSON, err := json.Marshal(notification.MetaData)
		if err != nil {
			r.logger.Error("Failed to marshal meta data", err, map[string]interface{}{
				"notification_id": notification.ID,
			})
			return fmt.Errorf("failed to marshal meta data: %w", err)
		}

		_, err = stmt.ExecContext(
			ctx,
			notification.ID,
			notification.UserID,
			notification.Type,
			notification.Title,
			notification.Content,
			notification.Status,
			notification.EntityID,
			notification.EntityType,
			metaDataJSON,
			notification.CreatedAt,
		)

		if err != nil {
			r.logger.Error("Failed to create notification in batch", err, map[string]interface{}{
				"user_id": notification.UserID,
				"type":    notification.Type,
			})
			return fmt.Errorf("failed to create notification in batch: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByID возвращает уведомление по ID
func (r *NotificationRepository) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	query := `
		SELECT 
			id, user_id, type, title, content, status, entity_id, entity_type, meta_data, created_at, read_at
		FROM notifications 
		WHERE id = $1
	`

	var notification domain.Notification
	var metaDataJSON []byte

	err := r.db.QueryRowxContext(ctx, query, id).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.Type,
		&notification.Title,
		&notification.Content,
		&notification.Status,
		&notification.EntityID,
		&notification.EntityType,
		&metaDataJSON,
		&notification.CreatedAt,
		&notification.ReadAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get notification by ID", err, map[string]interface{}{
			"id": id,
		})
		return nil, fmt.Errorf("failed to get notification by ID: %w", err)
	}

	// Десериализуем метаданные из JSON
	if metaDataJSON != nil {
		notification.MetaData = make(map[string]string)
		if err := json.Unmarshal(metaDataJSON, &notification.MetaData); err != nil {
			r.logger.Error("Failed to unmarshal meta data", err, map[string]interface{}{
				"id": id,
			})
			return nil, fmt.Errorf("failed to unmarshal meta data: %w", err)
		}
	}

	return &notification, nil
}

// Update обновляет данные уведомления
func (r *NotificationRepository) Update(ctx context.Context, notification *domain.Notification) error {
	query := `
		UPDATE notifications 
		SET 
			type = $1,
			title = $2,
			content = $3,
			status = $4,
			entity_id = $5,
			entity_type = $6,
			meta_data = $7,
			read_at = $8
		WHERE id = $9
	`

	// Сериализуем метаданные в JSON
	metaDataJSON, err := json.Marshal(notification.MetaData)
	if err != nil {
		r.logger.Error("Failed to marshal meta data", err, map[string]interface{}{
			"notification_id": notification.ID,
		})
		return fmt.Errorf("failed to marshal meta data: %w", err)
	}

	result, err := r.db.ExecContext(
		ctx,
		query,
		notification.Type,
		notification.Title,
		notification.Content,
		notification.Status,
		notification.EntityID,
		notification.EntityType,
		metaDataJSON,
		notification.ReadAt,
		notification.ID,
	)

	if err != nil {
		r.logger.Error("Failed to update notification", err, map[string]interface{}{
			"id": notification.ID,
		})
		return fmt.Errorf("failed to update notification: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

// Delete удаляет уведомление по ID (soft delete)
func (r *NotificationRepository) Delete(ctx context.Context, id string) error {
	query := `UPDATE notifications SET status = 'deleted' WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete notification", err, map[string]interface{}{
			"id": id,
		})
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

// GetUserNotifications возвращает уведомления пользователя
func (r *NotificationRepository) GetUserNotifications(ctx context.Context, userID string, filter repository.NotificationFilter) ([]*domain.Notification, error) {
	// Создаем копию фильтра и добавляем ID пользователя
	userFilter := filter
	// userIDForFilter := userID
	userFilter.IDs = nil
	userFilter.EntityID = nil
	userFilter.EntityType = nil
	userFilter.Types = nil
	userFilter.Status = nil
	userFilter.StartDate = nil
	userFilter.EndDate = nil

	whereClause, args := r.buildWhereClause(userFilter)

	// Добавляем условие для ID пользователя
	if whereClause == "" {
		whereClause = "WHERE user_id = $1"
		args = []interface{}{userID}
	} else {
		whereClause = whereClause + " AND user_id = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, userID)
	}

	// Добавляем дополнительные условия фильтрации
	if filter.Status != nil {
		whereClause = whereClause + " AND status = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, *filter.Status)
	} else {
		// По умолчанию исключаем удаленные уведомления
		whereClause = whereClause + " AND status != 'deleted'"
	}

	if len(filter.Types) > 0 {
		placeholders := make([]string, len(filter.Types))
		for i, t := range filter.Types {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+i+1)
			args = append(args, t)
		}
		whereClause = whereClause + " AND type IN (" + strings.Join(placeholders, ", ") + ")"
	}

	if filter.EntityID != nil {
		whereClause = whereClause + " AND entity_id = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, *filter.EntityID)
	}

	if filter.EntityType != nil {
		whereClause = whereClause + " AND entity_type = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, *filter.EntityType)
	}

	if filter.StartDate != nil {
		whereClause = whereClause + " AND created_at >= $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, *filter.StartDate)
	}

	if filter.EndDate != nil {
		whereClause = whereClause + " AND created_at <= $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, *filter.EndDate)
	}

	orderClause := r.buildOrderClause(filter)
	limitOffset := fmt.Sprintf("LIMIT %d OFFSET %d", filter.Limit, filter.Offset)

	query := fmt.Sprintf(`
		SELECT 
			id, user_id, type, title, content, status, entity_id, entity_type, meta_data, created_at, read_at
		FROM notifications
		%s
		%s
		%s
	`, whereClause, orderClause, limitOffset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to get user notifications", err, map[string]interface{}{
			"user_id": userID,
		})
		return nil, fmt.Errorf("failed to get user notifications: %w", err)
	}
	defer rows.Close()

	notifications := []*domain.Notification{}
	for rows.Next() {
		var notification domain.Notification
		var metaDataJSON []byte

		err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.Type,
			&notification.Title,
			&notification.Content,
			&notification.Status,
			&notification.EntityID,
			&notification.EntityType,
			&metaDataJSON,
			&notification.CreatedAt,
			&notification.ReadAt,
		)

		if err != nil {
			r.logger.Error("Failed to scan notification", err)
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}

		// Десериализуем метаданные из JSON
		if metaDataJSON != nil {
			notification.MetaData = make(map[string]string)
			if err := json.Unmarshal(metaDataJSON, &notification.MetaData); err != nil {
				r.logger.Error("Failed to unmarshal meta data", err, map[string]interface{}{
					"id": notification.ID,
				})
				return nil, fmt.Errorf("failed to unmarshal meta data: %w", err)
			}
		}

		notifications = append(notifications, &notification)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating through notifications", err)
		return nil, fmt.Errorf("error iterating through notifications: %w", err)
	}

	return notifications, nil
}

// CountUserNotifications возвращает количество уведомлений пользователя
func (r *NotificationRepository) CountUserNotifications(ctx context.Context, userID string, filter repository.NotificationFilter) (int, error) {
	// Создаем копию фильтра и добавляем ID пользователя
	userFilter := filter
	userFilter.IDs = nil
	userFilter.OrderBy = nil
	userFilter.OrderDir = nil
	userFilter.Limit = 0
	userFilter.Offset = 0

	whereClause, args := r.buildWhereClause(userFilter)

	// Добавляем условие для ID пользователя
	if whereClause == "" {
		whereClause = "WHERE user_id = $1"
		args = []interface{}{userID}
	} else {
		whereClause = whereClause + " AND user_id = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, userID)
	}

	// По умолчанию исключаем удаленные уведомления
	if filter.Status == nil {
		whereClause = whereClause + " AND status != 'deleted'"
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM notifications
		%s
	`, whereClause)

	var count int
	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		r.logger.Error("Failed to count user notifications", err, map[string]interface{}{
			"user_id": userID,
		})
		return 0, fmt.Errorf("failed to count user notifications: %w", err)
	}

	return count, nil
}

// MarkAsRead отмечает уведомление как прочитанное
func (r *NotificationRepository) MarkAsRead(ctx context.Context, id string) error {
	query := `UPDATE notifications SET status = 'read', read_at = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		r.logger.Error("Failed to mark notification as read", err, map[string]interface{}{
			"id": id,
		})
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

// MarkAllAsRead отмечает все уведомления пользователя как прочитанные
func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	query := `UPDATE notifications SET status = 'read', read_at = $1 WHERE user_id = $2 AND status = 'unread'`

	_, err := r.db.ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		r.logger.Error("Failed to mark all notifications as read", err, map[string]interface{}{
			"user_id": userID,
		})
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	return nil
}

// DeleteAllByUser удаляет все уведомления пользователя
func (r *NotificationRepository) DeleteAllByUser(ctx context.Context, userID string) error {
	query := `UPDATE notifications SET status = 'deleted' WHERE user_id = $1`

	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to delete all notifications", err, map[string]interface{}{
			"user_id": userID,
		})
		return fmt.Errorf("failed to delete all notifications: %w", err)
	}

	return nil
}

// GetUserUnreadCount возвращает количество непрочитанных уведомлений пользователя
func (r *NotificationRepository) GetUserUnreadCount(ctx context.Context, userID string) (int, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND status = 'unread'`

	var count int
	err := r.db.GetContext(ctx, &count, query, userID)
	if err != nil {
		r.logger.Error("Failed to get unread count", err, map[string]interface{}{
			"user_id": userID,
		})
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	return count, nil
}

// GetUserNotificationSettings возвращает настройки уведомлений пользователя
func (r *NotificationRepository) GetUserNotificationSettings(ctx context.Context, userID string) ([]*repository.NotificationSetting, error) {
	query := `
		SELECT 
			user_id, notification_type, email_enabled, web_enabled, telegram_enabled
		FROM user_notification_settings
		WHERE user_id = $1
	`

	settings := []*repository.NotificationSetting{}
	err := r.db.SelectContext(ctx, &settings, query, userID)
	if err != nil {
		r.logger.Error("Failed to get notification settings", err, map[string]interface{}{
			"user_id": userID,
		})
		return nil, fmt.Errorf("failed to get notification settings: %w", err)
	}

	return settings, nil
}

// UpdateUserNotificationSettings обновляет настройки уведомлений пользователя
func (r *NotificationRepository) UpdateUserNotificationSettings(ctx context.Context, userID string, settings []*repository.NotificationSetting) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.logger.Error("Failed to rollback transaction", rbErr)
			}
			return
		}
	}()

	// Удаляем текущие настройки
	_, err = tx.ExecContext(ctx, "DELETE FROM user_notification_settings WHERE user_id = $1", userID)
	if err != nil {
		r.logger.Error("Failed to delete notification settings", err, map[string]interface{}{
			"user_id": userID,
		})
		return fmt.Errorf("failed to delete notification settings: %w", err)
	}

	// Добавляем новые настройки
	query := `
		INSERT INTO user_notification_settings (
			user_id, notification_type, email_enabled, web_enabled, telegram_enabled
		) VALUES (
			$1, $2, $3, $4, $5
		)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, setting := range settings {
		_, err = stmt.ExecContext(
			ctx,
			userID,
			setting.NotificationType,
			setting.EmailEnabled,
			setting.WebEnabled,
			setting.TelegramEnabled,
		)

		if err != nil {
			r.logger.Error("Failed to insert notification setting", err, map[string]interface{}{
				"user_id": userID,
				"type":    setting.NotificationType,
			})
			return fmt.Errorf("failed to insert notification setting: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Вспомогательные функции

func (r *NotificationRepository) buildWhereClause(filter repository.NotificationFilter) (string, []interface{}) {
	conditions := []string{}
	args := []interface{}{}
	argIndex := 1

	if len(filter.IDs) > 0 {
		placeholders := make([]string, len(filter.IDs))
		for i, id := range filter.IDs {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, id)
			argIndex++
		}
		conditions = append(conditions, fmt.Sprintf("id IN (%s)", strings.Join(placeholders, ", ")))
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *filter.Status)
		argIndex++
	}

	if filter.EntityID != nil {
		conditions = append(conditions, fmt.Sprintf("entity_id = $%d", argIndex))
		args = append(args, *filter.EntityID)
		argIndex++
	}

	if filter.EntityType != nil {
		conditions = append(conditions, fmt.Sprintf("entity_type = $%d", argIndex))
		args = append(args, *filter.EntityType)
		argIndex++
	}

	if filter.StartDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filter.StartDate)
		argIndex++
	}

	if filter.EndDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filter.EndDate)
		argIndex++
	}

	if len(conditions) > 0 {
		return "WHERE " + strings.Join(conditions, " AND "), args
	}
	return "", args
}

func (r *NotificationRepository) buildOrderClause(filter repository.NotificationFilter) string {
	if filter.OrderBy != nil {
		direction := "ASC"
		if filter.OrderDir != nil && strings.ToUpper(*filter.OrderDir) == "DESC" {
			direction = "DESC"
		}

		// Проверяем, что поле сортировки допустимо
		allowedFields := map[string]bool{
			"id":         true,
			"user_id":    true,
			"type":       true,
			"status":     true,
			"entity_id":  true,
			"created_at": true,
			"read_at":    true,
		}

		if allowedFields[*filter.OrderBy] {
			return fmt.Sprintf("ORDER BY %s %s", *filter.OrderBy, direction)
		}
	}

	// По умолчанию сортируем по дате создания
	return "ORDER BY created_at DESC"
}