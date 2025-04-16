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
	qq := fmt.Sprintf(`SET LOCAL app.current_user_id = '%s'`, task.CreatedBy)

	if _, err = tx.ExecContext(ctx, qq); err != nil {
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
	if _, err = tx.ExecContext(ctx, "DELETE FROM task_tags WHERE task_id = $1", taskID); err != nil {
		r.logger.Error("Failed to delete task tags", err, map[string]interface{}{
			"task_id": taskID,
		})
		return fmt.Errorf("failed to delete task tags: %w", err)
	}

	// Добавляем новые теги
	for _, tag := range tags {
		if _, err = tx.ExecContext(
			ctx,
			"INSERT INTO task_tags (task_id, tag) VALUES ($1, $2)",
			taskID,
			tag,
		); err != nil {
			r.logger.Error("Failed to add task tag", err, map[string]interface{}{
				"task_id": taskID,
				"tag":     tag,
			})
			return fmt.Errorf("failed to add task tag: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// LogTaskHistory добавляет запись в историю изменений задачи
func (r *TaskRepository) LogTaskHistory(ctx context.Context, history *domain.TaskHistory) error {
	query := `
		INSERT INTO task_history (
			id, task_id, user_id, field, old_value, new_value, changed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		history.ID,
		history.TaskID,
		history.UserID,
		history.Field,
		history.OldValue,
		history.NewValue,
		history.ChangedAt,
	)

	if err != nil {
		r.logger.Error("Failed to log task history", err, map[string]interface{}{
			"task_id": history.TaskID,
			"field":   history.Field,
		})
		return fmt.Errorf("failed to log task history: %w", err)
	}

	return nil
}

// GetTaskHistory возвращает историю изменений задачи
func (r *TaskRepository) GetTaskHistory(ctx context.Context, taskID string) ([]*domain.TaskHistory, error) {
	query := `
		SELECT 
			id, task_id, user_id, field, old_value, new_value, changed_at
		FROM task_history
		WHERE task_id = $1
		ORDER BY changed_at DESC
	`

	history := []*domain.TaskHistory{}
	err := r.db.SelectContext(ctx, &history, query, taskID)
	if err != nil {
		r.logger.Error("Failed to get task history", err, map[string]interface{}{
			"task_id": taskID,
		})
		return nil, fmt.Errorf("failed to get task history: %w", err)
	}

	return history, nil
}

// GetTasksByProject возвращает задачи проекта
func (r *TaskRepository) GetTasksByProject(ctx context.Context, projectID string, filter repository.TaskFilter) ([]*domain.Task, error) {
	// Добавляем фильтр по проекту
	filter.ProjectIDs = append(filter.ProjectIDs, projectID)
	return r.List(ctx, filter)
}

// GetTasksByAssignee возвращает задачи назначенные пользователю
func (r *TaskRepository) GetTasksByAssignee(ctx context.Context, userID string, filter repository.TaskFilter) ([]*domain.Task, error) {
	assigneeID := userID
	filter.AssigneeID = &assigneeID
	return r.List(ctx, filter)
}

// CountTasksByProject возвращает количество задач в проекте
func (r *TaskRepository) CountTasksByProject(ctx context.Context, projectID string, filter repository.TaskFilter) (int, error) {
	// Добавляем фильтр по проекту
	filter.ProjectIDs = append(filter.ProjectIDs, projectID)
	return r.Count(ctx, filter)
}

// CountTasksByAssignee возвращает количество задач назначенных пользователю
func (r *TaskRepository) CountTasksByAssignee(ctx context.Context, userID string, filter repository.TaskFilter) (int, error) {
	assigneeID := userID
	filter.AssigneeID = &assigneeID
	return r.Count(ctx, filter)
}

// GetOverdueTasks возвращает просроченные задачи
func (r *TaskRepository) GetOverdueTasks(ctx context.Context, filter repository.TaskFilter) ([]*domain.Task, error) {
	isOverdue := true
	filter.IsOverdue = &isOverdue
	return r.List(ctx, filter)
}

// GetUpcomingTasks возвращает задачи с приближающимся сроком
func (r *TaskRepository) GetUpcomingTasks(ctx context.Context, daysThreshold int, filter repository.TaskFilter) ([]*domain.Task, error) {
	now := time.Now()
	thresholdDate := now.AddDate(0, 0, daysThreshold)

	filter.DueAfter = &now
	filter.DueBefore = &thresholdDate

	return r.List(ctx, filter)
}

// UpdateStatus обновляет статус задачи
func (r *TaskRepository) UpdateStatus(ctx context.Context, taskID string, status domain.TaskStatus, userID string) error {
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
	if _, err = tx.ExecContext(ctx, "SET LOCAL app.current_user_id = $1", userID); err != nil {
		return fmt.Errorf("failed to set local variable: %w", err)
	}

	query := `
		UPDATE tasks 
		SET 
			status = $1,
			updated_at = $2
		WHERE id = $3
	`

	result, err := tx.ExecContext(ctx, query, status, time.Now(), taskID)
	if err != nil {
		r.logger.Error("Failed to update task status", err, map[string]interface{}{
			"task_id": taskID,
			"status":  status,
		})
		return fmt.Errorf("failed to update task status: %w", err)
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

// UpdatePriority обновляет приоритет задачи
func (r *TaskRepository) UpdatePriority(ctx context.Context, taskID string, priority domain.TaskPriority, userID string) error {
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
	if _, err = tx.ExecContext(ctx, "SET LOCAL app.current_user_id = $1", userID); err != nil {
		return fmt.Errorf("failed to set local variable: %w", err)
	}

	query := `
		UPDATE tasks 
		SET 
			priority = $1,
			updated_at = $2
		WHERE id = $3
	`

	result, err := tx.ExecContext(ctx, query, priority, time.Now(), taskID)
	if err != nil {
		r.logger.Error("Failed to update task priority", err, map[string]interface{}{
			"task_id":  taskID,
			"priority": priority,
		})
		return fmt.Errorf("failed to update task priority: %w", err)
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

// UpdateAssignee обновляет исполнителя задачи
func (r *TaskRepository) UpdateAssignee(ctx context.Context, taskID string, assigneeID *string, userID string) error {
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
	if _, err = tx.ExecContext(ctx, "SET LOCAL app.current_user_id = $1", userID); err != nil {
		return fmt.Errorf("failed to set local variable: %w", err)
	}

	query := `
		UPDATE tasks 
		SET 
			assignee_id = $1,
			updated_at = $2
		WHERE id = $3
	`

	result, err := tx.ExecContext(ctx, query, assigneeID, time.Now(), taskID)
	if err != nil {
		r.logger.Error("Failed to update task assignee", err, map[string]interface{}{
			"task_id":     taskID,
			"assignee_id": assigneeID,
		})
		return fmt.Errorf("failed to update task assignee: %w", err)
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

// LogTime добавляет запись о затраченном времени
func (r *TaskRepository) LogTime(ctx context.Context, timeLog *repository.TimeLog) error {
	query := `
		INSERT INTO time_logs (
			id, task_id, user_id, hours, description, logged_at, log_date
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		timeLog.ID,
		timeLog.TaskID,
		timeLog.UserID,
		timeLog.Hours,
		timeLog.Description,
		timeLog.LoggedAt,
		timeLog.LogDate,
	)

	if err != nil {
		r.logger.Error("Failed to log time", err, map[string]interface{}{
			"task_id": timeLog.TaskID,
			"user_id": timeLog.UserID,
			"hours":   timeLog.Hours,
		})
		return fmt.Errorf("failed to log time: %w", err)
	}

	// Обновляем общее затраченное время в задаче
	updateQuery := `
		UPDATE tasks
		SET spent_hours = spent_hours + $1, updated_at = $2
		WHERE id = $3
	`

	_, err = r.db.ExecContext(ctx, updateQuery, timeLog.Hours, time.Now(), timeLog.TaskID)
	if err != nil {
		r.logger.Error("Failed to update task spent hours", err, map[string]interface{}{
			"task_id": timeLog.TaskID,
			"hours":   timeLog.Hours,
		})
		return fmt.Errorf("failed to update task spent hours: %w", err)
	}

	return nil
}

// GetTimeLogs возвращает записи о затраченном времени
func (r *TaskRepository) GetTimeLogs(ctx context.Context, taskID string) ([]*repository.TimeLog, error) {
	query := `
		SELECT 
			id, task_id, user_id, hours, description, logged_at, log_date
		FROM time_logs
		WHERE task_id = $1
		ORDER BY logged_at DESC
	`

	logs := []*repository.TimeLog{}
	err := r.db.SelectContext(ctx, &logs, query, taskID)
	if err != nil {
		r.logger.Error("Failed to get time logs", err, map[string]interface{}{
			"task_id": taskID,
		})
		return nil, fmt.Errorf("failed to get time logs: %w", err)
	}

	return logs, nil
}

// GetTaskMetrics возвращает метрики по задачам
func (r *TaskRepository) GetTaskMetrics(ctx context.Context, projectID string) (*domain.ProjectMetrics, error) {
	metrics := &domain.ProjectMetrics{
		TasksByStatus: make(map[string]int),
		TasksByUser:   make(map[string]int),
	}

	// Получаем общее количество задач и количество завершенных
	query := `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN due_date < NOW() AND status != 'completed' THEN 1 ELSE 0 END) as overdue
		FROM tasks
		WHERE project_id = $1
	`

	type result struct {
		Total     int `db:"total"`
		Completed int `db:"completed"`
		Overdue   int `db:"overdue"`
	}

	var res result
	err := r.db.GetContext(ctx, &res, query, projectID)
	if err != nil {
		r.logger.Error("Failed to get task metrics", err, map[string]interface{}{
			"project_id": projectID,
		})
		return nil, fmt.Errorf("failed to get task metrics: %w", err)
	}

	metrics.TaskCount = res.Total
	metrics.CompletedTasks = res.Completed
	metrics.OverdueTasks = res.Overdue

	// Получаем количество задач по статусам
	statusQuery := `
		SELECT 
			status, COUNT(*) as count
		FROM tasks
		WHERE project_id = $1
		GROUP BY status
	`

	type statusCount struct {
		Status string `db:"status"`
		Count  int    `db:"count"`
	}

	statusCounts := []statusCount{}
	err = r.db.SelectContext(ctx, &statusCounts, statusQuery, projectID)
	if err != nil {
		r.logger.Error("Failed to get task status counts", err, map[string]interface{}{
			"project_id": projectID,
		})
		return nil, fmt.Errorf("failed to get task status counts: %w", err)
	}

	for _, sc := range statusCounts {
		metrics.TasksByStatus[sc.Status] = sc.Count
	}

	// Получаем количество задач по пользователям
	userQuery := `
		SELECT 
			assignee_id, COUNT(*) as count
		FROM tasks
		WHERE project_id = $1 AND assignee_id IS NOT NULL
		GROUP BY assignee_id
	`

	type userCount struct {
		AssigneeID string `db:"assignee_id"`
		Count      int    `db:"count"`
	}

	userCounts := []userCount{}
	err = r.db.SelectContext(ctx, &userCounts, userQuery, projectID)
	if err != nil {
		r.logger.Error("Failed to get task user counts", err, map[string]interface{}{
			"project_id": projectID,
		})
		return nil, fmt.Errorf("failed to get task user counts: %w", err)
	}

	for _, uc := range userCounts {
		metrics.TasksByUser[uc.AssigneeID] = uc.Count
	}

	return metrics, nil
}

// Вспомогательные функции

func (r *TaskRepository) buildWhereClause(filter repository.TaskFilter) (string, []interface{}) {
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

	if len(filter.ProjectIDs) > 0 {
		placeholders := make([]string, len(filter.ProjectIDs))
		for i, id := range filter.ProjectIDs {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, id)
			argIndex++
		}
		conditions = append(conditions, fmt.Sprintf("project_id IN (%s)", strings.Join(placeholders, ", ")))
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *filter.Status)
		argIndex++
	}

	if filter.Priority != nil {
		conditions = append(conditions, fmt.Sprintf("priority = $%d", argIndex))
		args = append(args, *filter.Priority)
		argIndex++
	}

	if filter.AssigneeID != nil {
		conditions = append(conditions, fmt.Sprintf("assignee_id = $%d", argIndex))
		args = append(args, *filter.AssigneeID)
		argIndex++
	}

	if filter.CreatedBy != nil {
		conditions = append(conditions, fmt.Sprintf("created_by = $%d", argIndex))
		args = append(args, *filter.CreatedBy)
		argIndex++
	}

	if filter.DueBefore != nil {
		conditions = append(conditions, fmt.Sprintf("due_date <= $%d", argIndex))
		args = append(args, *filter.DueBefore)
		argIndex++
	}

	if filter.DueAfter != nil {
		conditions = append(conditions, fmt.Sprintf("due_date >= $%d", argIndex))
		args = append(args, *filter.DueAfter)
		argIndex++
	}

	if filter.IsOverdue != nil && *filter.IsOverdue {
		conditions = append(conditions, fmt.Sprintf("(due_date < $%d AND status != 'completed')", argIndex))
		args = append(args, time.Now())
		argIndex++
	}

	if len(filter.Tags) > 0 {
		// Подзапрос для фильтрации по тегам
		tagConditions := make([]string, len(filter.Tags))
		for i, tag := range filter.Tags {
			tagConditions[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, tag)
			argIndex++
		}
		conditions = append(conditions, fmt.Sprintf("id IN (SELECT task_id FROM task_tags WHERE tag IN (%s) GROUP BY task_id HAVING COUNT(DISTINCT tag) = %d)",
			strings.Join(tagConditions, ", "), len(filter.Tags)))
	}

	if filter.SearchText != nil {
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", argIndex, argIndex))
		searchPattern := "%" + *filter.SearchText + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	if len(conditions) > 0 {
		return "WHERE " + strings.Join(conditions, " AND "), args
	}
	return "", args
}

func (r *TaskRepository) buildOrderClause(filter repository.TaskFilter) string {
	if filter.OrderBy != nil {
		direction := "ASC"
		if filter.OrderDir != nil && strings.ToUpper(*filter.OrderDir) == "DESC" {
			direction = "DESC"
		}

		// Проверяем, что поле сортировки допустимо
		allowedFields := map[string]bool{
			"id":              true,
			"title":           true,
			"status":          true,
			"priority":        true,
			"assignee_id":     true,
			"created_by":      true,
			"due_date":        true,
			"created_at":      true,
			"updated_at":      true,
			"completed_at":    true,
			"estimated_hours": true,
			"spent_hours":     true,
		}

		if allowedFields[*filter.OrderBy] {
			return fmt.Sprintf("ORDER BY %s %s", *filter.OrderBy, direction)
		}
	}

	// По умолчанию сортируем по приоритету и дате создания
	return "ORDER BY priority DESC, created_at DESC"
}
