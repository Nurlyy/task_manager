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

// UserRepository реализует репозиторий пользователей с использованием PostgreSQL
type UserRepository struct {
	db     *sqlx.DB
	logger logger.Logger
}

// NewUserRepository создает новый экземпляр UserRepository
func NewUserRepository(db *sqlx.DB, logger logger.Logger) *UserRepository {
	return &UserRepository{
		db:     db,
		logger: logger,
	}
}

// Create создает нового пользователя
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (
			id, email, hashed_password, first_name, last_name, role, 
			avatar, position, department, is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		) RETURNING id
	`

	err := r.db.QueryRowxContext(
		ctx,
		query,
		user.ID,
		user.Email,
		user.HashedPassword,
		user.FirstName,
		user.LastName,
		user.Role,
		user.Avatar,
		user.Position,
		user.Department,
		user.IsActive,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&user.ID)

	if err != nil {
		r.logger.Error("Failed to create user", err, map[string]interface{}{
			"email": user.Email,
		})
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByID возвращает пользователя по ID
func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT 
			id, email, hashed_password, first_name, last_name, role, 
			avatar, position, department, is_active, last_login_at, created_at, updated_at
		FROM users 
		WHERE id = $1
	`

	var user domain.User
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get user by ID", err, map[string]interface{}{
			"id": id,
		})
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return &user, nil
}

// GetByEmail возвращает пользователя по email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT 
			id, email, hashed_password, first_name, last_name, role, 
			avatar, position, department, is_active, last_login_at, created_at, updated_at
		FROM users 
		WHERE email = $1
	`

	var user domain.User
	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get user by email", err, map[string]interface{}{
			"email": email,
		})
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}

// Update обновляет данные пользователя
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users 
		SET 
			email = $1,
			first_name = $2,
			last_name = $3,
			role = $4,
			avatar = $5,
			position = $6,
			department = $7,
			is_active = $8,
			updated_at = $9
		WHERE id = $10
	`

	user.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(
		ctx,
		query,
		user.Email,
		user.FirstName,
		user.LastName,
		user.Role,
		user.Avatar,
		user.Position,
		user.Department,
		user.IsActive,
		user.UpdatedAt,
		user.ID,
	)

	if err != nil {
		r.logger.Error("Failed to update user", err, map[string]interface{}{
			"id": user.ID,
		})
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// Delete удаляет пользователя по ID
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete user", err, map[string]interface{}{
			"id": id,
		})
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// List возвращает список пользователей с фильтрацией
func (r *UserRepository) List(ctx context.Context, filter repository.UserFilter) ([]*domain.User, error) {
	whereClause, args := r.buildWhereClause(filter)
	orderClause := r.buildOrderClause(filter)
	limitOffset := fmt.Sprintf("LIMIT %d OFFSET %d", filter.Limit, filter.Offset)

	query := fmt.Sprintf(`
		SELECT 
			id, email, hashed_password, first_name, last_name, role, 
			avatar, position, department, is_active, last_login_at, created_at, updated_at
		FROM users
		%s
		%s
		%s
	`, whereClause, orderClause, limitOffset)

	users := []*domain.User{}
	err := r.db.SelectContext(ctx, &users, query, args...)
	if err != nil {
		r.logger.Error("Failed to list users", err)
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return users, nil
}

// Count возвращает количество пользователей с фильтрацией
func (r *UserRepository) Count(ctx context.Context, filter repository.UserFilter) (int, error) {
	whereClause, args := r.buildWhereClause(filter)

	query := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM users
		%s
	`, whereClause)

	var count int
	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		r.logger.Error("Failed to count users", err)
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

// UpdateLastLogin обновляет время последнего входа пользователя
func (r *UserRepository) UpdateLastLogin(ctx context.Context, id string) error {
	query := `UPDATE users SET last_login_at = NOW() WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to update last login", err, map[string]interface{}{
			"id": id,
		})
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

// Вспомогательные функции для построения SQL-запросов

func (r *UserRepository) buildWhereClause(filter repository.UserFilter) (string, []interface{}) {
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

	if filter.Email != nil {
		conditions = append(conditions, fmt.Sprintf("email = $%d", argIndex))
		args = append(args, *filter.Email)
		argIndex++
	}

	if filter.Role != nil {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIndex))
		args = append(args, *filter.Role)
		argIndex++
	}

	if filter.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *filter.IsActive)
		argIndex++
	}

	if filter.Department != nil {
		conditions = append(conditions, fmt.Sprintf("department = $%d", argIndex))
		args = append(args, *filter.Department)
		argIndex++
	}

	if filter.SearchText != nil {
		conditions = append(conditions, fmt.Sprintf("(first_name ILIKE $%d OR last_name ILIKE $%d OR email ILIKE $%d)", argIndex, argIndex, argIndex))
		searchPattern := "%" + *filter.SearchText + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	if len(conditions) > 0 {
		return "WHERE " + strings.Join(conditions, " AND "), args
	}
	return "", args
}

func (r *UserRepository) buildOrderClause(filter repository.UserFilter) string {
	if filter.OrderBy != nil {
		direction := "ASC"
		if filter.OrderDir != nil && strings.ToUpper(*filter.OrderDir) == "DESC" {
			direction = "DESC"
		}

		// Проверяем, что поле сортировки допустимо
		allowedFields := map[string]bool{
			"id":         true,
			"email":      true,
			"first_name": true,
			"last_name":  true,
			"role":       true,
			"is_active":  true,
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