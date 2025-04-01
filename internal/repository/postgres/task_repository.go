package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/yourusername/task-tracker/internal/domain"
	"github.com/yourusername/task-tracker/internal/repository"
	"github.com/yourusername/task-tracker/pkg/logger"
)

// TaskRepository реализует репозиторий задач с использованием PostgreSQL
type TaskRepository struct {
	db     *sqlx.DB
	logger logger.Logger
}

// NewTaskRepository создает новый экземпляр TaskRepository
func NewTaskRepository(db *sqlx.DB, logger logger.Logger) *TaskRepository {
	return &TaskRepository{
		db:     db,
		logger: logger,
	}
}

// Create создает новую задачу
func (r *TaskRepository) Create(ctx context.Context, task *domain.Task) error {
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

	// Устанавливаем значение app.current_user_id для триггера
	if _, err = tx.ExecContext(ctx, "SET LOCAL app.current_user_id = $1", task.CreatedBy); err != nil {
		return fmt.Errorf("failed to set local variable: %w", err)
	}

	// Сохраняем основные данные задачи
	query := `
		INSERT INTO tasks (
			id, title, description, project_id, status, priority, 
			assignee_id, created_by, due_date, estimated_hours, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		) RETURNING id
	`

	if err = tx.QueryRowxContext(
		ctx,
		query,
		task.ID,
		task.Title,
		task.Description,
		task.ProjectID,
		task.Status,
		task.Priority,
		task.AssigneeID,
		task.CreatedBy,
		task.DueDate,
		task.EstimatedHours,
		task.CreatedAt,
		task.UpdatedAt,
	).Scan(&task.ID); err != nil {
		r.logger.Error("Failed to create task", err, map[string]interface{}{
			"title": task.Title,
		})
		return fmt.Errorf("failed to create task: %w", err)
	}

	// Сохраняем теги задачи
	if len(task.Tags) > 0 {
		for _, tag := range task.Tags {
			if _, err = tx.ExecContext(
				ctx,
				"INSERT INTO task_tags (task_id, tag) VALUES ($1, $2)",
				task.ID,
				tag,
			); err != nil {
				r.logger.Error("Failed to add task tag", err, map[string]interface{}{
					"task_id": task.ID,
					"tag":     tag,
				})
				return fmt.Errorf("failed to add task tag: %w", err)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByID возвращает задачу по ID
func (r *TaskRepository) GetByID(ctx context.Context, id string) (*domain.Task, error) {
	query := `
		SELECT 
			id, title, description, project_id, status, priority, 
			assignee_id, created_by, due_date, estimated_hours, spent_hours, 
			created_at, updated_at, completed_at
		FROM tasks 
		WHERE id = $1
	`

	var task domain.Task
	err := r.db.GetContext(ctx, &task, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get task by ID", err, map[string]interface{}{
			"id": id,
		})
		return nil, fmt.Errorf("failed to get task by ID: %w", err)
	}

	// Получаем теги задачи
	tags, err := r.GetTags(ctx, id)
	if err != nil {
		return nil, err
	}
	task.Tags = tags

	return &task, nil
}

// Update обновляет данные задачи
func (r *TaskRepository) Update(ctx context.Context, task *domain.Task) error {
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

	// Устанавливаем значение app.current_user_id для триггера
	if _, err = tx.ExecContext(ctx, "SET LOCAL app.current_user_id = $1", task.CreatedBy); err != nil {
		return fmt.Errorf("failed to set local variable: %w", err)
	}

	// Обновляем основные данные задачи
	query := `
		UPDATE tasks 
		SET 
			title = $1,
			description = $2,
			status = $3,
			priority = $4,
			assignee_id = $5,
			due_date = $6,
			estimated_hours = $7,
			spent_hours = $8,
			updated_at = $9
		WHERE id = $10
	`

	task.UpdatedAt = time.Now()

	result, err := tx.ExecContext(
		ctx,
		query,
		task.Title,
		task.Description,
		task.Status,
		task.Priority,
		task.AssigneeID,
		task.DueDate,
		task.EstimatedHours,
		task.SpentHours,
		task.UpdatedAt,
		task.ID,
	)

	if err != nil {
		r.logger.Error("Failed to update task", err, map[string]interface{}{
			"id": task.ID,
		})
		return fmt.Errorf("failed to update task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("task not found")
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Delete удаляет задачу по ID
func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM tasks WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete task", err, map[string]interface{}{
			"id": id,
		})
		return fmt.Errorf("failed to delete task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("task not found")
	}

	return nil
}

// List возвращает список задач с фильтрацией
func (r *TaskRepository) List(ctx context.Context, filter repository.TaskFilter) ([]*domain.Task, error) {
	whereClause, args := r.buildWhereClause(filter)
	orderClause := r.buildOrderClause(filter)
	limitOffset := fmt.Sprintf("LIMIT %d OFFSET %d", filter.Limit, filter.Offset)

	query := fmt.Sprintf(`
		SELECT 
			id, title, description, project_id, status, priority, 
			assignee_id, created_by, due_date, estimated_hours, spent_hours, 
			created_at, updated_at, completed_at
		FROM tasks
		%s
		%s
		%s
	`, whereClause, orderClause, limitOffset)

	tasks := []*domain.Task{}
	err := r.db.SelectContext(ctx, &tasks, query, args...)
	if err != nil {
		r.logger.Error("Failed to list tasks", err)
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	// Получаем теги для каждой задачи
	for _, task := range tasks {
		tags, err := r.GetTags(ctx, task.ID)
		if err != nil {
			return nil, err
		}
		task.Tags = tags
	}

	return tasks, nil
}

// Count возвращает количество задач с фильтрацией
func (r *TaskRepository) Count(ctx context.Context, filter repository.TaskFilter) (int, error) {
	whereClause, args := r.buildWhereClause(filter)

	query := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM tasks
		%s
	`, whereClause)

	var count int
	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		r.logger.Error("Failed to count tasks", err)
		return 0, fmt.Errorf("failed to count tasks: %w", err)
	}

	return count, nil
}

// GetTags возвращает теги задачи
func (r *TaskRepository) GetTags(ctx context.Context, taskID string) ([]string, error) {
	query := `SELECT tag FROM task_tags WHERE task_id = $1`

	tags := []string{}
	err := r.db.SelectContext(ctx, &tags, query, taskID)
	if err != nil {
		r.logger.Error("Failed to get task tags", err, map[string]interface{}{
			"task_id": taskID,
		})
		return nil, fmt.Errorf("failed to get task tags: %w", err)
	}

	return tags, nil
}

// AddTag добавляет тег к задаче
func (r *TaskRepository) AddTag(ctx context.Context, taskID, tag string) error {
	query := `INSERT INTO task_tags (task_id, tag) VALUES ($1, $2) ON CONFLICT DO NOTHING`

	_, err := r.db.ExecContext(ctx, query, taskID, tag)
	if err != nil {
		r.logger.Error("Failed to add task tag", err, map[string]interface{}{
			"task_id": taskID,
			"tag":     tag,
		})
		return fmt.Errorf("failed to add task tag: %w", err)
	}

	return nil
}

// RemoveTag удаляет тег из задачи
func (r *TaskRepository) RemoveTag(ctx context.Context, taskID, tag string) error {
	query := `DELETE FROM task_tags WHERE task_id = $1 AND tag = $2`

	result, err := r.db.ExecContext(ctx, query, taskID, tag)
	if err != nil {
		r.logger.Error("Failed to remove task tag", err, map[string]interface{}{
			"task_id": taskID,
			"tag":     tag,
		})
		return fmt.Errorf("failed to remove task tag: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("task tag not found")
	}

	return nil
}

// UpdateTags обновляет теги задачи
func (r *TaskRepository) UpdateTags(ctx context.Context, taskID string, tags []string) error {
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

	// Удаляем все текущие теги
	if _, err = tx.