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

// ProjectRepository реализует репозиторий проектов с использованием PostgreSQL
type ProjectRepository struct {
	db     *sqlx.DB
	logger logger.Logger
}

// NewProjectRepository создает новый экземпляр ProjectRepository
func NewProjectRepository(db *sqlx.DB, logger logger.Logger) *ProjectRepository {
	return &ProjectRepository{
		db:     db,
		logger: logger,
	}
}

// Create создает новый проект
func (r *ProjectRepository) Create(ctx context.Context, project *domain.Project) error {
	query := `
		INSERT INTO projects (
			id, name, description, status, created_by, start_date, end_date, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		) RETURNING id
	`

	err := r.db.QueryRowxContext(
		ctx,
		query,
		project.ID,
		project.Name,
		project.Description,
		project.Status,
		project.CreatedBy,
		project.StartDate,
		project.EndDate,
		project.CreatedAt,
		project.UpdatedAt,
	).Scan(&project.ID)

	if err != nil {
		r.logger.Error("Failed to create project", err, map[string]interface{}{
			"name": project.Name,
		})
		return fmt.Errorf("failed to create project: %w", err)
	}

	return nil
}

// GetByID возвращает проект по ID
func (r *ProjectRepository) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	query := `
		SELECT 
			id, name, description, status, created_by, start_date, end_date, created_at, updated_at
		FROM projects 
		WHERE id = $1
	`

	var project domain.Project
	err := r.db.GetContext(ctx, &project, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get project by ID", err, map[string]interface{}{
			"id": id,
		})
		return nil, fmt.Errorf("failed to get project by ID: %w", err)
	}

	return &project, nil
}

// Update обновляет данные проекта
func (r *ProjectRepository) Update(ctx context.Context, project *domain.Project) error {
	query := `
		UPDATE projects 
		SET 
			name = $1,
			description = $2,
			status = $3,
			start_date = $4,
			end_date = $5,
			updated_at = $6
		WHERE id = $7
	`

	project.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(
		ctx,
		query,
		project.Name,
		project.Description,
		project.Status,
		project.StartDate,
		project.EndDate,
		project.UpdatedAt,
		project.ID,
	)

	if err != nil {
		r.logger.Error("Failed to update project", err, map[string]interface{}{
			"id": project.ID,
		})
		return fmt.Errorf("failed to update project: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}

// Delete удаляет проект по ID
func (r *ProjectRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM projects WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete project", err, map[string]interface{}{
			"id": id,
		})
		return fmt.Errorf("failed to delete project: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}

// List возвращает список проектов с фильтрацией
func (r *ProjectRepository) List(ctx context.Context, filter repository.ProjectFilter) ([]*domain.Project, error) {
	whereClause, args := r.buildWhereClause(filter)
	orderClause := r.buildOrderClause(filter)
	limitOffset := fmt.Sprintf("LIMIT %d OFFSET %d", filter.Limit, filter.Offset)

	query := fmt.Sprintf(`
		SELECT 
			id, name, description, status, created_by, start_date, end_date, created_at, updated_at
		FROM projects
		%s
		%s
		%s
	`, whereClause, orderClause, limitOffset)

	projects := []*domain.Project{}
	err := r.db.SelectContext(ctx, &projects, query, args...)
	if err != nil {
		r.logger.Error("Failed to list projects", err)
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	return projects, nil
}

// Count возвращает количество проектов с фильтрацией
func (r *ProjectRepository) Count(ctx context.Context, filter repository.ProjectFilter) (int, error) {
	whereClause, args := r.buildWhereClause(filter)

	query := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM projects
		%s
	`, whereClause)

	var count int
	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		r.logger.Error("Failed to count projects", err)
		return 0, fmt.Errorf("failed to count projects: %w", err)
	}

	return count, nil
}

// AddMember добавляет пользователя в проект
func (r *ProjectRepository) AddMember(ctx context.Context, member *domain.ProjectMember) error {
	query := `
		INSERT INTO project_members (
			project_id, user_id, role, joined_at, invited_by
		) VALUES (
			$1, $2, $3, $4, $5
		) ON CONFLICT (project_id, user_id) DO UPDATE
		SET role = $3, invited_by = $5
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		member.ProjectID,
		member.UserID,
		member.Role,
		member.JoinedAt,
		member.InvitedBy,
	)

	if err != nil {
		r.logger.Error("Failed to add project member", err, map[string]interface{}{
			"project_id": member.ProjectID,
			"user_id":    member.UserID,
		})
		return fmt.Errorf("failed to add project member: %w", err)
	}

	return nil
}

// UpdateMember обновляет роль пользователя в проекте
func (r *ProjectRepository) UpdateMember(ctx context.Context, projectID, userID string, role domain.ProjectRole) error {
	query := `
		UPDATE project_members 
		SET role = $1
		WHERE project_id = $2 AND user_id = $3
	`

	result, err := r.db.ExecContext(ctx, query, role, projectID, userID)
	if err != nil {
		r.logger.Error("Failed to update project member", err, map[string]interface{}{
			"project_id": projectID,
			"user_id":    userID,
		})
		return fmt.Errorf("failed to update project member: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("project member not found")
	}

	return nil
}

// RemoveMember удаляет пользователя из проекта
func (r *ProjectRepository) RemoveMember(ctx context.Context, projectID, userID string) error {
	query := `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`

	result, err := r.db.ExecContext(ctx, query, projectID, userID)
	if err != nil {
		r.logger.Error("Failed to remove project member", err, map[string]interface{}{
			"project_id": projectID,
			"user_id":    userID,
		})
		return fmt.Errorf("failed to remove project member: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("project member not found")
	}

	return nil
}

// GetMembers возвращает список участников проекта
func (r *ProjectRepository) GetMembers(ctx context.Context, projectID string) ([]*domain.ProjectMember, error) {
	query := `
		SELECT 
			project_id, user_id, role, joined_at, invited_by
		FROM project_members
		WHERE project_id = $1
	`

	members := []*domain.ProjectMember{}
	err := r.db.SelectContext(ctx, &members, query, projectID)
	if err != nil {
		r.logger.Error("Failed to get project members", err, map[string]interface{}{
			"project_id": projectID,
		})
		return nil, fmt.Errorf("failed to get project members: %w", err)
	}

	return members, nil
}

// GetMember возвращает информацию об участнике проекта
func (r *ProjectRepository) GetMember(ctx context.Context, projectID, userID string) (*domain.ProjectMember, error) {
	query := `
		SELECT 
			project_id, user_id, role, joined_at, invited_by
		FROM project_members
		WHERE project_id = $1 AND user_id = $2
	`

	var member domain.ProjectMember
	err := r.db.GetContext(ctx, &member, query, projectID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get project member", err, map[string]interface{}{
			"project_id": projectID,
			"user_id":    userID,
		})
		return nil, fmt.Errorf("failed to get project member: %w", err)
	}

	return &member, nil
}

// GetUserProjects возвращает список проектов пользователя
func (r *ProjectRepository) GetUserProjects(ctx context.Context, userID string, filter repository.ProjectFilter) ([]*domain.Project, error) {
	whereClause, args := r.buildWhereClause(filter)
	if whereClause == "" {
		whereClause = "WHERE p.id IN (SELECT project_id FROM project_members WHERE user_id = $1)"
		args = []interface{}{userID}
	} else {
		whereClause = whereClause + " AND p.id IN (SELECT project_id FROM project_members WHERE user_id = $" + fmt.Sprintf("%d", len(args)+1) + ")"
		args = append(args, userID)
	}

	orderClause := r.buildOrderClause(filter)
	limitOffset := fmt.Sprintf("LIMIT %d OFFSET %d", filter.Limit, filter.Offset)

	query := fmt.Sprintf(`
		SELECT 
			p.id, p.name, p.description, p.status, p.created_by, p.start_date, p.end_date, p.created_at, p.updated_at
		FROM projects p
		%s
		%s
		%s
	`, whereClause, orderClause, limitOffset)

	projects := []*domain.Project{}
	err := r.db.SelectContext(ctx, &projects, query, args...)
	if err != nil {
		r.logger.Error("Failed to get user projects", err, map[string]interface{}{
			"user_id": userID,
		})
		return nil, fmt.Errorf("failed to get user projects: %w", err)
	}

	return projects, nil
}

// CountUserProjects возвращает количество проектов пользователя
func (r *ProjectRepository) CountUserProjects(ctx context.Context, userID string, filter repository.ProjectFilter) (int, error) {
	whereClause, args := r.buildWhereClause(filter)
	if whereClause == "" {
		whereClause = "WHERE p.id IN (SELECT project_id FROM project_members WHERE user_id = $1)"
		args = []interface{}{userID}
	} else {
		whereClause = whereClause + " AND p.id IN (SELECT project_id FROM project_members WHERE user_id = $" + fmt.Sprintf("%d", len(args)+1) + ")"
		args = append(args, userID)
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM projects p
		%s
	`, whereClause)

	var count int
	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		r.logger.Error("Failed to count user projects", err, map[string]interface{}{
			"user_id": userID,
		})
		return 0, fmt.Errorf("failed to count user projects: %w", err)
	}

	return count, nil
}

// Вспомогательные функции

func (r *ProjectRepository) buildWhereClause(filter repository.ProjectFilter) (string, []interface{}) {
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

	if filter.CreatedBy != nil {
		conditions = append(conditions, fmt.Sprintf("created_by = $%d", argIndex))
		args = append(args, *filter.CreatedBy)
		argIndex++
	}

	if filter.StartAfter != nil {
		conditions = append(conditions, fmt.Sprintf("start_date >= $%d", argIndex))
		args = append(args, *filter.StartAfter)
		argIndex++
	}

	if filter.EndBefore != nil {
		conditions = append(conditions, fmt.Sprintf("end_date <= $%d", argIndex))
		args = append(args, *filter.EndBefore)
		argIndex++
	}

	if filter.SearchText != nil {
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR description ILIKE $%d)", argIndex, argIndex))
		searchPattern := "%" + *filter.SearchText + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	if len(conditions) > 0 {
		return "WHERE " + strings.Join(conditions, " AND "), args
	}
	return "", args
}

func (r *ProjectRepository) buildOrderClause(filter repository.ProjectFilter) string {
	if filter.OrderBy != nil {
		direction := "ASC"
		if filter.OrderDir != nil && strings.ToUpper(*filter.OrderDir) == "DESC" {
			direction = "DESC"
		}

		// Проверяем, что поле сортировки допустимо
		allowedFields := map[string]bool{
			"id":         true,
			"name":       true,
			"status":     true,
			"created_by": true,
			"start_date": true,
			"end_date":   true,
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