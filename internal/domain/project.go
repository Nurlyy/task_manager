package domain

import (
	"time"
)

// ProjectStatus определяет статус проекта
type ProjectStatus string

const (
	// ProjectStatusActive - активный проект
	ProjectStatusActive ProjectStatus = "active"
	// ProjectStatusOnHold - проект на паузе
	ProjectStatusOnHold ProjectStatus = "on_hold"
	// ProjectStatusCompleted - завершенный проект
	ProjectStatusCompleted ProjectStatus = "completed"
	// ProjectStatusArchived - архивированный проект
	ProjectStatusArchived ProjectStatus = "archived"
)

// ProjectRole определяет роль пользователя в проекте
type ProjectRole string

const (
	// ProjectRoleOwner - создатель проекта
	ProjectRoleOwner ProjectRole = "owner"
	// ProjectRoleManager - менеджер проекта
	ProjectRoleManager ProjectRole = "manager"
	// ProjectRoleMember - участник проекта
	ProjectRoleMember ProjectRole = "member"
	// ProjectRoleViewer - наблюдатель проекта
	ProjectRoleViewer ProjectRole = "viewer"
)

// Project представляет модель проекта
type Project struct {
	ID          string        `json:"id" db:"id"`
	Name        string        `json:"name" db:"name"`
	Description string        `json:"description" db:"description"`
	Status      ProjectStatus `json:"status" db:"status"`
	CreatedBy   string        `json:"created_by" db:"created_by"`
	StartDate   *time.Time    `json:"start_date,omitempty" db:"start_date"`
	EndDate     *time.Time    `json:"end_date,omitempty" db:"end_date"`
	CreatedAt   time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" db:"updated_at"`
}

// ProjectMember представляет связь пользователя с проектом
type ProjectMember struct {
	ProjectID  string      `json:"project_id" db:"project_id"`
	UserID     string      `json:"user_id" db:"user_id"`
	Role       ProjectRole `json:"role" db:"role"`
	JoinedAt   time.Time   `json:"joined_at" db:"joined_at"`
	InvitedBy  string      `json:"invited_by" db:"invited_by"`
}

// ProjectCreateRequest представляет данные для создания проекта
type ProjectCreateRequest struct {
	Name        string        `json:"name" validate:"required,min=3,max=100"`
	Description string        `json:"description" validate:"required"`
	Status      ProjectStatus `json:"status" validate:"required,oneof=active on_hold completed archived"`
	StartDate   *time.Time    `json:"start_date,omitempty"`
	EndDate     *time.Time    `json:"end_date,omitempty" validate:"omitempty,gtfield=StartDate"`
}

// ProjectUpdateRequest представляет данные для обновления проекта
type ProjectUpdateRequest struct {
	Name        *string        `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
	Description *string        `json:"description,omitempty"`
	Status      *ProjectStatus `json:"status,omitempty" validate:"omitempty,oneof=active on_hold completed archived"`
	StartDate   *time.Time     `json:"start_date,omitempty"`
	EndDate     *time.Time     `json:"end_date,omitempty" validate:"omitempty,gtfield=StartDate"`
}

// ProjectResponse представляет данные проекта для API-ответов
type ProjectResponse struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Status      ProjectStatus `json:"status"`
	CreatedBy   string        `json:"created_by"`
	StartDate   *time.Time    `json:"start_date,omitempty"`
	EndDate     *time.Time    `json:"end_date,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Members     []ProjectMemberResponse `json:"members,omitempty"`
	Metrics     *ProjectMetrics `json:"metrics,omitempty"`
}

// ProjectMemberResponse представляет данные участника проекта для API-ответов
type ProjectMemberResponse struct {
	UserID    string      `json:"user_id"`
	Email     string      `json:"email"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Role      ProjectRole `json:"role"`
	JoinedAt  time.Time   `json:"joined_at"`
}

// ProjectMetrics представляет метрики проекта
type ProjectMetrics struct {
	TaskCount      int            `json:"task_count"`
	CompletedTasks int            `json:"completed_tasks"`
	OverdueTasks   int            `json:"overdue_tasks"`
	TasksByStatus  map[string]int `json:"tasks_by_status"`
	TasksByUser    map[string]int `json:"tasks_by_user,omitempty"`
}

// AddMemberRequest представляет запрос на добавление участника в проект
type AddMemberRequest struct {
	UserID string      `json:"user_id" validate:"required"`
	Role   ProjectRole `json:"role" validate:"required,oneof=owner manager member viewer"`
}

// UpdateMemberRequest представляет запрос на обновление роли участника
type UpdateMemberRequest struct {
	Role ProjectRole `json:"role" validate:"required,oneof=owner manager member viewer"`
}

// ToResponse преобразует Project в ProjectResponse
func (p *Project) ToResponse() ProjectResponse {
	return ProjectResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Status:      p.Status,
		CreatedBy:   p.CreatedBy,
		StartDate:   p.StartDate,
		EndDate:     p.EndDate,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// IsActive проверяет, является ли проект активным
func (p *Project) IsActive() bool {
	return p.Status == ProjectStatusActive
}

// IsCompleted проверяет, является ли проект завершенным
func (p *Project) IsCompleted() bool {
	return p.Status == ProjectStatusCompleted || p.Status == ProjectStatusArchived
}

// HasMember проверяет, является ли пользователь участником проекта с указанной ролью
func (pm *ProjectMember) HasRole(role ProjectRole) bool {
	return pm.Role == role
}

// CanManageProject проверяет, имеет ли участник права на управление проектом
func (pm *ProjectMember) CanManageProject() bool {
	return pm.Role == ProjectRoleOwner || pm.Role == ProjectRoleManager
}