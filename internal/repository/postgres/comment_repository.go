package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// CommentRepository реализует репозиторий комментариев с использованием PostgreSQL
type CommentRepository struct {
	db     *sqlx.DB
	logger logger.Logger
}

// NewCommentRepository создает новый экземпляр CommentRepository
func NewCommentRepository(db *sqlx.DB, logger logger.Logger) *CommentRepository {
	return &CommentRepository{
		db:     db,
		logger: logger,
	}
}

// Create создает новый комментарий
func (r *CommentRepository) Create(ctx context.Context, comment *domain.Comment) error {
	query := `
		INSERT INTO comments (
			id, task_id, user_id, content, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6
		) RETURNING id
	`

	err := r.db.QueryRowxContext(
		ctx,
		query,
		comment.ID,
		comment.TaskID,
		comment.UserID,
		comment.Content,
		comment.CreatedAt,
		comment.UpdatedAt,
	).Scan(&comment.ID)

	if err != nil {
		r.logger.Error("Failed to create comment", err, map[string]interface{}{
			"task_id": comment.TaskID,
			"user_id": comment.UserID,
		})
		return fmt.Errorf("failed to create comment: %w", err)
	}

	return nil
}

// GetByID возвращает комментарий по ID
func (r *CommentRepository) GetByID(ctx context.Context, id string) (*domain.Comment, error) {
	query := `
		SELECT 
			id, task_id, user_id, content, created_at, updated_at
		FROM comments 
		WHERE id = $1
	`

	var comment domain.Comment
	err := r.db.GetContext(ctx, &comment, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get comment by ID", err, map[string]interface{}{
			"id": id,
		})
		return nil, fmt.Errorf("failed to get comment by ID: %w", err)
	}

	return &comment, nil
}

// Update обновляет данные комментария
func (r *CommentRepository) Update(ctx context.Context, comment *domain.Comment) error {
	query := `
		UPDATE comments 
		SET 
			content = $1,
			updated_at = $2
		WHERE id = $3
	`

	comment.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(
		ctx,
		query,
		comment.Content,
		comment.UpdatedAt,
		comment.ID,
	)

	if err != nil {
		r.logger.Error("Failed to update comment", err, map[string]interface{}{
			"id": comment.ID,
		})
		return fmt.Errorf("failed to update comment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("comment not found")
	}

	return nil
}

// Delete удаляет комментарий по ID
func (r *CommentRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM comments WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete comment", err, map[string]interface{}{
			"id": id,
		})
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("comment not found")
	}

	return nil
}

// List возвращает список комментариев с фильтрацией
func (r *CommentRepository) List(ctx context.Context, filter repository.CommentFilter) ([]*domain.Comment, error) {
	whereClause, args := r.buildWhereClause(filter)
	orderClause := r.buildOrderClause(filter)
	limitOffset := fmt.Sprintf("LIMIT %d OFFSET %d", filter.Limit, filter.Offset)

	query := fmt.Sprintf(`
		SELECT 
			id, task_id, user_id, content, created_at, updated_at
		FROM comments
		%s
		%s
		%s
	`, whereClause, orderClause, limitOffset)

	comments := []*domain.Comment{}
	err := r.db.SelectContext(ctx, &comments, query, args...)
	if err != nil {
		r.logger.Error("Failed to list comments", err)
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	return comments, nil
}

// Count возвращает количество комментариев с фильтрацией
func (r *CommentRepository) Count(ctx context.Context, filter repository.CommentFilter) (int, error) {
	whereClause, args := r.buildWhereClause(filter)

	query := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM comments
		%s
	`, whereClause)

	var count int
	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		r.logger.Error("Failed to count comments", err)
		return 0, fmt.Errorf("failed to count comments: %w", err)
	}

	return count, nil
}

// GetCommentsByTask возвращает комментарии к задаче
func (r *CommentRepository) GetCommentsByTask(ctx context.Context, taskID string, filter repository.CommentFilter) ([]*domain.Comment, error) {
	// Создаем копию фильтра, чтобы не изменять исходный
	taskFilter := filter
	taskFilter.TaskIDs = append(taskFilter.TaskIDs, taskID)
	return r.List(ctx, taskFilter)
}

// CountCommentsByTask возвращает количество комментариев к задаче
func (r *CommentRepository) CountCommentsByTask(ctx context.Context, taskID string) (int, error) {
	query := `SELECT COUNT(*) FROM comments WHERE task_id = $1`

	var count int
	err := r.db.GetContext(ctx, &count, query, taskID)
	if err != nil {
		r.logger.Error("Failed to count comments by task", err, map[string]interface{}{
			"task_id": taskID,
		})
		return 0, fmt.Errorf("failed to count comments by task: %w", err)
	}

	return count, nil
}

// GetCommentsByUser возвращает комментарии пользователя
func (r *CommentRepository) GetCommentsByUser(ctx context.Context, userID string, filter repository.CommentFilter) ([]*domain.Comment, error) {
	// Создаем копию фильтра, чтобы не изменять исходный
	userFilter := filter
	userFilter.UserIDs = append(userFilter.UserIDs, userID)
	return r.List(ctx, userFilter)
}

// CountCommentsByUser возвращает количество комментариев пользователя
func (r *CommentRepository) CountCommentsByUser(ctx context.Context, userID string) (int, error) {
	query := `SELECT COUNT(*) FROM comments WHERE user_id = $1`

	var count int
	err := r.db.GetContext(ctx, &count, query, userID)
	if err != nil {
		r.logger.Error("Failed to count comments by user", err, map[string]interface{}{
			"user_id": userID,
		})
		return 0, fmt.Errorf("failed to count comments by user: %w", err)
	}

	return count, nil
}

// Вспомогательные функции

func (r *CommentRepository) buildWhereClause(filter repository.CommentFilter) (string, []interface{}) {
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

	if len(filter.TaskIDs) > 0 {
		placeholders := make([]string, len(filter.TaskIDs))
		for i, id := range filter.TaskIDs {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, id)
			argIndex++
		}
		conditions = append(conditions, fmt.Sprintf("task_id IN (%s)", strings.Join(placeholders, ", ")))
	}

	if len(filter.UserIDs) > 0 {
		placeholders := make([]string, len(filter.UserIDs))
		for i, id := range filter.UserIDs {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, id)
			argIndex++
		}
		conditions = append(conditions, fmt.Sprintf("user_id IN (%s)", strings.Join(placeholders, ", ")))
	}

	if filter.SearchText != nil {
		conditions = append(conditions, fmt.Sprintf("content ILIKE $%d", argIndex))
		args = append(args, "%"+*filter.SearchText+"%")
		argIndex++
	}

	if len(conditions) > 0 {
		return "WHERE " + strings.Join(conditions, " AND "), args
	}
	return "", args
}

func (r *CommentRepository) buildOrderClause(filter repository.CommentFilter) string {
	if filter.OrderBy != nil {
		direction := "ASC"
		if filter.OrderDir != nil && strings.ToUpper(*filter.OrderDir) == "DESC" {
			direction = "DESC"
		}

		// Проверяем, что поле сортировки допустимо
		allowedFields := map[string]bool{
			"id":         true,
			"task_id":    true,
			"user_id":    true,
			"created_at": true,
			"updated_at": true,
		}

		if allowedFields[*filter.OrderBy] {
			return fmt.Sprintf("ORDER BY %s %s", *filter.OrderBy, direction)
		}
	}

	// По умолчанию сортируем по дате создания
	return "ORDER BY created_at DESC"
}