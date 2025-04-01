package domain

import (
	"time"
)

// UserRole определяет роль пользователя в системе
type UserRole string

const (
	// UserRoleAdmin имеет доступ ко всем функциям системы
	UserRoleAdmin UserRole = "admin"
	// UserRoleManager может управлять проектами и задачами
	UserRoleManager UserRole = "manager"
	// UserRoleDeveloper может работать с задачами
	UserRoleDeveloper UserRole = "developer"
	// UserRoleViewer имеет доступ только для чтения
	UserRoleViewer UserRole = "viewer"
)

// User представляет модель пользователя
type User struct {
	ID             string    `json:"id" db:"id"`
	Email          string    `json:"email" db:"email"`
	HashedPassword string    `json:"-" db:"hashed_password"`
	FirstName      string    `json:"first_name" db:"first_name"`
	LastName       string    `json:"last_name" db:"last_name"`
	Role           UserRole  `json:"role" db:"role"`
	Avatar         *string   `json:"avatar,omitempty" db:"avatar"`
	Position       *string   `json:"position,omitempty" db:"position"`
	Department     *string   `json:"department,omitempty" db:"department"`
	IsActive       bool      `json:"is_active" db:"is_active"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// UserCreateRequest представляет данные для создания пользователя
type UserCreateRequest struct {
	Email     string   `json:"email" validate:"required,email"`
	Password  string   `json:"password" validate:"required,min=8"`
	FirstName string   `json:"first_name" validate:"required"`
	LastName  string   `json:"last_name" validate:"required"`
	Role      UserRole `json:"role" validate:"required,oneof=admin manager developer viewer"`
	Position  *string  `json:"position,omitempty"`
	Department *string `json:"department,omitempty"`
	Avatar    *string  `json:"avatar,omitempty"`
}

// UserUpdateRequest представляет данные для обновления пользователя
type UserUpdateRequest struct {
	FirstName  *string   `json:"first_name,omitempty"`
	LastName   *string   `json:"last_name,omitempty"`
	Role       *UserRole `json:"role,omitempty" validate:"omitempty,oneof=admin manager developer viewer"`
	Position   *string   `json:"position,omitempty"`
	Department *string   `json:"department,omitempty"`
	Avatar     *string   `json:"avatar,omitempty"`
	IsActive   *bool     `json:"is_active,omitempty"`
}

// UserResponse представляет данные пользователя для API-ответов
type UserResponse struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	Role       UserRole  `json:"role"`
	Avatar     *string   `json:"avatar,omitempty"`
	Position   *string   `json:"position,omitempty"`
	Department *string   `json:"department,omitempty"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ToResponse преобразует User в UserResponse
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:         u.ID,
		Email:      u.Email,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Role:       u.Role,
		Avatar:     u.Avatar,
		Position:   u.Position,
		Department: u.Department,
		IsActive:   u.IsActive,
		CreatedAt:  u.CreatedAt,
		UpdatedAt:  u.UpdatedAt,
	}
}

// FullName возвращает полное имя пользователя
func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}

// HasRole проверяет, имеет ли пользователь указанную роль
func (u *User) HasRole(role UserRole) bool {
	return u.Role == role
}

// IsAdmin проверяет, является ли пользователь администратором
func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

// LoginRequest представляет данные для входа пользователя
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse представляет ответ при успешном входе
type LoginResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresAt    time.Time    `json:"expires_at"`
}

// RefreshTokenRequest представляет запрос на обновление токенов
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ChangePasswordRequest представляет запрос на изменение пароля
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,nefield=OldPassword"`
}