package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yourusername/task-tracker/internal/domain"
	"github.com/yourusername/task-tracker/internal/messaging"
	"github.com/yourusername/task-tracker/internal/repository"
	"github.com/yourusername/task-tracker/pkg/logger"
)

// Стандартные ошибки
var (
	ErrProjectNotFound      = errors.New("project not found")
	ErrMemberAlreadyExists  = errors.New("member already exists in project")
	ErrMemberNotFound       = errors.New("member not found in project")
	ErrInsufficientRights   = errors.New("insufficient rights to perform this action")
)

// ProjectService представляет бизнес-логику для работы с проектами
type ProjectService struct {
	projectRepo repository.ProjectRepository
	userRepo    repository.UserRepository
	taskRepo    repository.TaskRepository
	cacheRepo   *repository.CacheRepository
	producer    *messaging.KafkaProducer
	logger      logger.Logger
}

// NewProjectService создает новый экземпляр ProjectService
func NewProjectService(
	projectRepo repository.ProjectRepository,
	userRepo repository.UserRepository,
	taskRepo repository.TaskRepository,
	cacheRepo *repository.CacheRepository,
	producer *messaging.KafkaProducer,
	logger logger.Logger,
) *ProjectService {
	return &ProjectService{
		projectRepo: projectRepo,
		userRepo:    userRepo,
		taskRepo:    taskRepo,
		cacheRepo:   cacheRepo,
		producer:    producer,
		logger:      logger,
	}
}

// Create создает новый проект
func (s *ProjectService) Create(ctx context.Context, req domain.ProjectCreateRequest, userID string) (*domain.ProjectResponse, error) {
	// Получаем данные пользователя
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user by ID for project creation", err, "user_id", userID)
		return nil, ErrUserNotFound
	}

	// Создаем новый проект
	now := time.Now()
	project := &domain.Project{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		CreatedBy:   userID,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Сохраняем проект в БД
	if err := s.projectRepo.Create(ctx, project); err != nil {
		s.logger.Error("Failed to create project", err)
		return nil, err
	}

	// Добавляем создателя как владельца проекта
	member := &domain.ProjectMember{
		ProjectID: project.ID,
		UserID:    userID,
		Role:      domain.ProjectRoleOwner,
		JoinedAt:  now,
		InvitedBy: userID,
	}

	if err := s.projectRepo.AddMember(ctx, member); err != nil {
		s.logger.Error("Failed to add owner to project", err, "project_id", project.ID, "user_id", userID)
		return nil, err
	}

	// Преобразуем к ProjectResponse
	resp := project.ToResponse()

	// Отправляем событие о создании проекта
	event := &messaging.ProjectEvent{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		Status:      string(project.Status),
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Type:        messaging.EventTypeProjectCreated,
	}

	if err := s.producer.PublishProjectEvent(ctx, messaging.EventTypeProjectCreated, event); err != nil {
		s.logger.Warn("Failed to publish project creation event", "project_id", project.ID, "error", err)
	}

	return &resp, nil
}

// GetByID возвращает проект по ID
func (s *ProjectService) GetByID(ctx context.Context, id string, userID string) (*domain.ProjectResponse, error) {
	// Пытаемся получить из кэша
	cacheKey := "project:" + id
	var projectResp domain.ProjectResponse
	if err := s.cacheRepo.Get(ctx, cacheKey, &projectResp); err == nil {
		// Проверяем, имеет ли пользователь доступ к проекту
		if s.hasAccessToProject(ctx, id, userID) {
			return &projectResp, nil
		}
		return nil, ErrInsufficientRights
	}

	// Получаем проект из БД
	project, err := s.projectRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get project by ID", err, "id", id)
		return nil, ErrProjectNotFound
	}

	// Проверяем, имеет ли пользователь доступ к проекту
	if !s.hasAccessToProject(ctx, id, userID) {
		return nil, ErrInsufficientRights
	}

	// Получаем участников проекта
	members, err := s.projectRepo.GetMembers(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get project members", err, "project_id", id)
		return nil, err
	}

	// Получаем метрики проекта
	metrics, err := s.taskRepo.GetTaskMetrics(ctx, id)
	if err != nil {
		s.logger.Warn("Failed to get project metrics", "project_id", id, "error", err)
	}

	// Преобразуем участников к ProjectMemberResponse
	memberResponses := make([]domain.ProjectMemberResponse, len(members))
	for i, member := range members {
		user, err := s.userRepo.GetByID(ctx, member.UserID)
		if err != nil {
			s.logger.Error("Failed to get user for project member", err, "user_id", member.UserID)
			continue
		}

		memberResponses[i] = domain.ProjectMemberResponse{
			UserID:    user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Role:      member.Role,
			JoinedAt:  member.JoinedAt,
		}
	}

	// Формируем полный ответ
	resp := project.ToResponse()
	resp.Members = memberResponses
	resp.Metrics = metrics

	// Сохраняем в кэш
	if err := s.cacheRepo.Set(ctx, cacheKey, resp); err != nil {
		s.logger.Warn("Failed to cache project", "id", id, "error", err)
	}

	return &resp, nil
}

// Update обновляет данные проекта
func (s *ProjectService) Update(ctx context.Context, id string, req domain.ProjectUpdateRequest, userID string) (*domain.ProjectResponse, error) {
	// Получаем проект из БД
	project, err := s.projectRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get project by ID for update", err, "id", id)
		return nil, ErrProjectNotFound
	}

	// Проверяем права на редактирование
	if !s.canManageProject(ctx, id, userID) {
		return nil, ErrInsufficientRights
	}

	// Фиксируем изменения для события
	changes := make(map[string]interface{})

	// Обновляем поля, которые были переданы
	if req.Name != nil {
		changes["name"] = map[string]interface{}{"old": project.Name, "new": *req.Name}
		project.Name = *req.Name
	}
	if req.Description != nil {
		changes["description"] = map[string]interface{}{"old": project.Description, "new": *req.Description}
		project.Description = *req.Description
	}
	if req.Status != nil {
		changes["status"] = map[string]interface{}{"old": project.Status, "new": *req.Status}
		project.Status = *req.Status
	}
	if req.StartDate != nil {
		changes["start_date"] = map[string]interface{}{"old": project.StartDate, "new": *req.StartDate}
		project.StartDate = req.StartDate
	}
	if req.EndDate != nil {
		changes["end_date"] = map[string]interface{}{"old": project.EndDate, "new": *req.EndDate}
		project.EndDate = req.EndDate
	}

	project.UpdatedAt = time.Now()

	// Сохраняем изменения в БД
	if err := s.projectRepo.Update(ctx, project); err != nil {
		s.logger.Error("Failed to update project", err, "id", id)
		return nil, err
	}

	// Удаляем проект из кэша
	cacheKey := "project:" + id
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete project from cache", "id", id, "error", err)
	}

	// Отправляем событие об обновлении проекта, если были изменения
	if len(changes) > 0 {
		event := &messaging.ProjectEvent{
			ID:          project.ID,
			Name:        project.Name,
			Description: project.Description,
			Status:      string(project.Status),
			CreatedBy:   project.CreatedBy,
			UpdatedAt:   project.UpdatedAt,
			Type:        messaging.EventTypeProjectUpdated,
			Changes:     changes,
		}

		if err := s.producer.PublishProjectEvent(ctx, messaging.EventTypeProjectUpdated, event); err != nil {
			s.logger.Warn("Failed to publish project update event", "project_id", project.ID, "error", err)
		}
	}

	// Преобразуем к ProjectResponse
	resp := project.ToResponse()
	return &resp, nil
}

// Delete удаляет проект
func (s *ProjectService) Delete(ctx context.Context, id string, userID string) error {
	// Проверяем, существует ли проект
	project, err := s.projectRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get project by ID for delete", err, "id", id)
		return ErrProjectNotFound
	}

	// Проверяем, является ли пользователь владельцем проекта
	member, err := s.projectRepo.GetMember(ctx, id, userID)
	if err != nil || member.Role != domain.ProjectRoleOwner {
		s.logger.Warn("User attempted to delete project without owner rights", "user_id", userID, "project_id", id)
		return ErrInsufficientRights
	}

	// Удаляем проект из БД
	if err := s.projectRepo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete project", err, "id", id)
		return err
	}

	// Удаляем проект из кэша
	cacheKey := "project:" + id
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete project from cache", "id", id, "error", err)
	}

	return nil
}

// List возвращает список проектов с фильтрацией
func (s *ProjectService) List(ctx context.Context, filter repository.ProjectFilter, userID string, page, pageSize int) (*domain.PagedResponse, error) {
	// Настраиваем пагинацию
	filter.Limit = pageSize
	filter.Offset = (page - 1) * pageSize

	// Если не передан фильтр по участнику, добавляем текущего пользователя
	if filter.MemberID == nil {
		filter.MemberID = &userID
	}

	// Получаем список проектов, в которых участвует пользователь
	projects, err := s.projectRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list projects", err)
		return nil, err
	}

	// Получаем общее количество проектов
	total, err := s.projectRepo.Count(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to count projects", err)
		return nil, err
	}

	// Преобразуем к ProjectResponse
	projectResponses := make([]domain.ProjectResponse, len(projects))
	for i, project := range projects {
		projectResponses[i] = project.ToResponse()
	}

	// Формируем ответ с пагинацией
	return &domain.PagedResponse{
		Items:      projectResponses,
		TotalItems: total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}, nil
}

// AddMember добавляет участника в проект
func (s *ProjectService) AddMember(ctx context.Context, projectID string, req domain.AddMemberRequest, userID string) (*domain.ProjectMemberResponse, error) {
	// Проверяем, существует ли проект
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		s.logger.Error("Failed to get project by ID for adding member", err, "id", projectID)
		return nil, ErrProjectNotFound
	}

	// Проверяем права на добавление участников
	if !s.canManageProject(ctx, projectID, userID) {
		return nil, ErrInsufficientRights
	}

	// Проверяем, существует ли пользователь, которого добавляем
	newUser, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		s.logger.Error("Failed to get user by ID for adding to project", err, "user_id", req.UserID)
		return nil, ErrUserNotFound
	}

	// Проверяем, не является ли пользователь уже участником проекта
	_, err = s.projectRepo.GetMember(ctx, projectID, req.UserID)
	if err == nil {
		return nil, ErrMemberAlreadyExists
	}

	// Добавляем участника в проект
	member := &domain.ProjectMember{
		ProjectID: projectID,
		UserID:    req.UserID,
		Role:      req.Role,
		JoinedAt:  time.Now(),
		InvitedBy: userID,
	}

	if err := s.projectRepo.AddMember(ctx, member); err != nil {
		s.logger.Error("Failed to add member to project", err, "project_id", projectID, "user_id", req.UserID)
		return nil, err
	}

	// Удаляем проект из кэша
	cacheKey := "project:" + projectID
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete project from cache", "id", projectID, "error", err)
	}

	// Отправляем событие о добавлении участника
	event := &messaging.ProjectMemberEvent{
		ProjectID:   projectID,
		ProjectName: project.Name,
		UserID:      req.UserID,
		Role:        string(req.Role),
		InvitedBy:   userID,
		JoinedAt:    member.JoinedAt,
		Type:        messaging.EventTypeProjectMemberAdded,
	}

	if err := s.producer.PublishProjectEvent(ctx, messaging.EventTypeProjectMemberAdded, event); err != nil {
		s.logger.Warn("Failed to publish project member added event", "project_id", projectID, "user_id", req.UserID, "error", err)
	}

	// Формируем ответ
	return &domain.ProjectMemberResponse{
		UserID:    newUser.ID,
		Email:     newUser.Email,
		FirstName: newUser.FirstName,
		LastName:  newUser.LastName,
		Role:      req.Role,
		JoinedAt:  member.JoinedAt,
	}, nil
}


// UpdateMember обновляет роль участника проекта
func (s *ProjectService) UpdateMember(ctx context.Context, projectID string, memberID string, req domain.UpdateMemberRequest, userID string) (*domain.ProjectMemberResponse, error) {
	// Проверяем, существует ли проект
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		s.logger.Error("Failed to get project by ID for updating member", err, "id", projectID)
		return nil, ErrProjectNotFound
	}

	// Проверяем права на управление участниками
	if !s.canManageProject(ctx, projectID, userID) {
		return nil, ErrInsufficientRights
	}

	// Проверяем, является ли пользователь участником проекта
	member, err := s.projectRepo.GetMember(ctx, projectID, memberID)
	if err != nil {
		s.logger.Error("Failed to get member", err, "project_id", projectID, "user_id", memberID)
		return nil, ErrMemberNotFound
	}

	// Проверяем, не пытается ли пользователь изменить роль владельца
	if member.Role == domain.ProjectRoleOwner && req.Role != domain.ProjectRoleOwner {
		currentUserMember, err := s.projectRepo.GetMember(ctx, projectID, userID)
		if err != nil || currentUserMember.Role != domain.ProjectRoleOwner {
			return nil, ErrInsufficientRights
		}
	}

	// Обновляем роль участника
	oldRole := member.Role
	member.Role = req.Role

	if err := s.projectRepo.UpdateMember(ctx, member); err != nil {
		s.logger.Error("Failed to update member role", err, "project_id", projectID, "user_id", memberID)
		return nil, err
	}

	// Удаляем проект из кэша
	cacheKey := "project:" + projectID
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete project from cache", "id", projectID, "error", err)
	}

	// Получаем данные пользователя для ответа
	memberUser, err := s.userRepo.GetByID(ctx, memberID)
	if err != nil {
		s.logger.Error("Failed to get user by ID for response", err, "user_id", memberID)
		return nil, ErrUserNotFound
	}

	// Отправляем событие об изменении роли
	event := &messaging.ProjectMemberEvent{
		ProjectID:   projectID,
		ProjectName: project.Name,
		UserID:      memberID,
		Role:        string(req.Role),
		InvitedBy:   userID,
		JoinedAt:    member.JoinedAt,
		Type:        "project_member_updated",
	}

	if err := s.producer.PublishProjectEvent(ctx, "project_member_updated", event); err != nil {
		s.logger.Warn("Failed to publish project member updated event", "project_id", projectID, "user_id", memberID, "error", err)
	}

	// Формируем ответ
	return &domain.ProjectMemberResponse{
		UserID:    memberUser.ID,
		Email:     memberUser.Email,
		FirstName: memberUser.FirstName,
		LastName:  memberUser.LastName,
		Role:      req.Role,
		JoinedAt:  member.JoinedAt,
	}, nil
}

// RemoveMember удаляет участника из проекта
func (s *ProjectService) RemoveMember(ctx context.Context, projectID string, memberID string, userID string) error {
	// Проверяем, существует ли проект
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		s.logger.Error("Failed to get project by ID for removing member", err, "id", projectID)
		return ErrProjectNotFound
	}

	// Проверяем права на управление участниками
	if !s.canManageProject(ctx, projectID, userID) && userID != memberID {
		return ErrInsufficientRights
	}

	// Проверяем, является ли пользователь участником проекта
	member, err := s.projectRepo.GetMember(ctx, projectID, memberID)
	if err != nil {
		s.logger.Error("Failed to get member", err, "project_id", projectID, "user_id", memberID)
		return ErrMemberNotFound
	}

	// Не позволяем удалять владельца проекта
	if member.Role == domain.ProjectRoleOwner {
		// Если владелец хочет выйти, он должен сначала передать права другому участнику
		return errors.New("cannot remove project owner, transfer ownership first")
	}

	// Удаляем участника из проекта
	if err := s.projectRepo.RemoveMember(ctx, projectID, memberID); err != nil {
		s.logger.Error("Failed to remove member from project", err, "project_id", projectID, "user_id", memberID)
		return err
	}

	// Удаляем проект из кэша
	cacheKey := "project:" + projectID
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete project from cache", "id", projectID, "error", err)
	}

	// Отправляем событие об удалении участника
	event := &messaging.ProjectMemberEvent{
		ProjectID:   projectID,
		ProjectName: project.Name,
		UserID:      memberID,
		Role:        string(member.Role),
		InvitedBy:   userID,
		JoinedAt:    member.JoinedAt,
		Type:        "project_member_removed",
	}

	if err := s.producer.PublishProjectEvent(ctx, "project_member_removed", event); err != nil {
		s.logger.Warn("Failed to publish project member removed event", "project_id", projectID, "user_id", memberID, "error", err)
	}

	return nil
}

// TransferOwnership передает владение проектом другому участнику
func (s *ProjectService) TransferOwnership(ctx context.Context, projectID string, newOwnerID string, userID string) error {
	// Проверяем, существует ли проект
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		s.logger.Error("Failed to get project by ID for transferring ownership", err, "id", projectID)
		return ErrProjectNotFound
	}

	// Проверяем, является ли текущий пользователь владельцем проекта
	currentOwner, err := s.projectRepo.GetMember(ctx, projectID, userID)
	if err != nil || currentOwner.Role != domain.ProjectRoleOwner {
		s.logger.Warn("User attempted to transfer project ownership without owner rights", "user_id", userID, "project_id", projectID)
		return ErrInsufficientRights
	}

	// Проверяем, является ли новый владелец участником проекта
	newOwner, err := s.projectRepo.GetMember(ctx, projectID, newOwnerID)
	if err != nil {
		s.logger.Error("Failed to get new owner as member", err, "project_id", projectID, "user_id", newOwnerID)
		return ErrMemberNotFound
	}

	// Меняем роль текущего владельца на Manager
	currentOwner.Role = domain.ProjectRoleManager
	if err := s.projectRepo.UpdateMember(ctx, currentOwner); err != nil {
		s.logger.Error("Failed to update current owner role", err, "project_id", projectID, "user_id", userID)
		return err
	}

	// Меняем роль нового владельца на Owner
	newOwner.Role = domain.ProjectRoleOwner
	if err := s.projectRepo.UpdateMember(ctx, newOwner); err != nil {
		s.logger.Error("Failed to update new owner role", err, "project_id", projectID, "user_id", newOwnerID)
		// Если не удалось обновить нового владельца, восстанавливаем старого
		currentOwner.Role = domain.ProjectRoleOwner
		_ = s.projectRepo.UpdateMember(ctx, currentOwner)
		return err
	}

	// Удаляем проект из кэша
	cacheKey := "project:" + projectID
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete project from cache", "id", projectID, "error", err)
	}

	// Отправляем событие о передаче владения
	event := &messaging.ProjectEvent{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		Status:      string(project.Status),
		CreatedBy:   project.CreatedBy,
		UpdatedAt:   time.Now(),
		Type:        "project_ownership_transferred",
		Changes: map[string]interface{}{
			"previous_owner": userID,
			"new_owner":      newOwnerID,
		},
	}

	if err := s.producer.PublishProjectEvent(ctx, "project_ownership_transferred", event); err != nil {
		s.logger.Warn("Failed to publish ownership transfer event", "project_id", projectID, "error", err)
	}

	return nil
}



// Добавляем вспомогательные методы для проверки прав

// hasAccessToProject проверяет, имеет ли пользователь доступ к проекту
func (s *ProjectService) hasAccessToProject(ctx context.Context, projectID string, userID string) bool {
	// Администраторы имеют доступ ко всем проектам
	user, err := s.userRepo.GetByID(ctx, userID)
	if err == nil && user.IsAdmin() {
		return true
	}

	// Проверяем, является ли пользователь участником проекта
	_, err = s.projectRepo.GetMember(ctx, projectID, userID)
	return err == nil
}

// canManageProject проверяет, может ли пользователь управлять проектом
func (s *ProjectService) canManageProject(ctx context.Context, projectID string, userID string) bool {
	// Администраторы могут управлять всеми проектами
	user, err := s.userRepo.GetByID(ctx, userID)
	if err == nil && user.IsAdmin() {
		return true
	}

	// Проверяем, является ли пользователь владельцем или менеджером проекта
	member, err := s.projectRepo.GetMember(ctx, projectID, userID)
	if err != nil {
		return false
	}
	
	return member.Role == domain.ProjectRoleOwner || member.Role == domain.ProjectRoleManager
}

// GetProjectMetrics возвращает метрики проекта
func (s *ProjectService) GetProjectMetrics(ctx context.Context, projectID string, userID string) (*domain.ProjectMetrics, error) {
	// Проверяем, существует ли проект
	if _, err := s.projectRepo.GetByID(ctx, projectID); err != nil {
		s.logger.Error("Failed to get project by ID for metrics", err, "id", projectID)
		return nil, ErrProjectNotFound
	}

	// Проверяем доступ пользователя к проекту
	if !s.hasAccessToProject(ctx, projectID, userID) {
		return nil, ErrInsufficientRights
	}

	// Получаем метрики проекта
	metrics, err := s.taskRepo.GetTaskMetrics(ctx, projectID)
	if err != nil {
		s.logger.Error("Failed to get project metrics", err, "project_id", projectID)
		return nil, err
	}

	return metrics, nil
}