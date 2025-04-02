package handlers

import (
	"errors"
	"net/http"

	"github.com/yourusername/task-tracker/internal/domain"
	"github.com/yourusername/task-tracker/internal/repository"
	"github.com/yourusername/task-tracker/internal/service"
)

// ProjectHandler обрабатывает запросы, связанные с проектами
type ProjectHandler struct {
	BaseHandler
	projectService *service.ProjectService
}

// NewProjectHandler создает новый экземпляр ProjectHandler
func NewProjectHandler(base BaseHandler, projectService *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{
		BaseHandler:    base,
		projectService: projectService,
	}
}

// CreateProject обрабатывает запрос на создание нового проекта
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	var req domain.ProjectCreateRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse create project request", err)
		h.RespondWithError(w, r, http.StatusBadRequest, "Invalid request format", "invalid_format")
		return
	}

	// Валидация запроса
	if validationErrors, err := h.ValidateRequest(req); err != nil {
		h.Logger.Error("Request validation error", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Validation failed", "validation_error")
		return
	} else if len(validationErrors) > 0 {
		h.RespondWithValidationErrors(w, r, validationErrors)
		return
	}

	// Обновляем данные проекта
	project, err := h.projectService.Update(r.Context(), projectID, req, userID)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Project not found", "project_not_found")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Insufficient rights to update project", "insufficient_rights")
			return
		}
		h.Logger.Error("Failed to update project", err, "id", projectID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to update project", "update_failed")
		return
	}

	h.RespondWithSuccess(w, r, project)
}

// DeleteProject удаляет проект
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID проекта из URL
	projectID := h.GetURLParam(r, "id")
	if projectID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Project ID is required", "missing_id")
		return
	}

	// Удаляем проект
	if err := h.projectService.Delete(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Project not found", "project_not_found")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Only project owner can delete project", "insufficient_rights")
			return
		}
		h.Logger.Error("Failed to delete project", err, "id", projectID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to delete project", "delete_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]bool{"success": true})
}

// ListProjects возвращает список проектов пользователя
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Параметры пагинации
	page, pageSize := h.GetPaginationParams(r)

	// Создаем фильтр
	filter := repository.ProjectFilter{
		MemberID: &userID,
	}

	// Фильтр по статусу
	if status := r.URL.Query().Get("status"); status != "" {
		projectStatus := domain.ProjectStatus(status)
		filter.Status = &projectStatus
	}

	// Фильтр по поисковому тексту
	if search := r.URL.Query().Get("search"); search != "" {
		filter.SearchText = &search
	}

	// Получаем список проектов
	result, err := h.projectService.List(r.Context(), filter, userID, page, pageSize)
	if err != nil {
		h.Logger.Error("Failed to list projects", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get projects", "projects_fetch_failed")
		return
	}

	h.RespondWithPagination(w, r, result.Items, result)
}

// AddProjectMember добавляет участника в проект
func (h *ProjectHandler) AddProjectMember(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID проекта из URL
	projectID := h.GetURLParam(r, "id")
	if projectID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Project ID is required", "missing_id")
		return
	}

	var req domain.AddMemberRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse add member request", err)
		h.RespondWithError(w, r, http.StatusBadRequest, "Invalid request format", "invalid_format")
		return
	}

	// Валидация запроса
	if validationErrors, err := h.ValidateRequest(req); err != nil {
		h.Logger.Error("Request validation error", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Validation failed", "validation_error")
		return
	} else if len(validationErrors) > 0 {
		h.RespondWithValidationErrors(w, r, validationErrors)
		return
	}

	// Добавляем участника в проект
	member, err := h.projectService.AddMember(r.Context(), projectID, req, userID)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Project not found", "project_not_found")
			return
		}
		if errors.Is(err, service.ErrUserNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "User not found", "user_not_found")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Insufficient rights to add members", "insufficient_rights")
			return
		}
		if errors.Is(err, service.ErrMemberAlreadyExists) {
			h.RespondWithError(w, r, http.StatusConflict, "User is already a member of the project", "member_exists")
			return
		}
		h.Logger.Error("Failed to add member to project", err, "project_id", projectID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to add member", "add_member_failed")
		return
	}

	h.RespondWithSuccess(w, r, member)
}

// UpdateProjectMember обновляет роль участника проекта
func (h *ProjectHandler) UpdateProjectMember(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID проекта из URL
	projectID := h.GetURLParam(r, "id")
	if projectID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Project ID is required", "missing_id")
		return
	}

	// Получаем ID участника из URL
	memberID := h.GetURLParam(r, "member_id")
	if memberID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Member ID is required", "missing_member_id")
		return
	}

	var req domain.UpdateMemberRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse update member request", err)
		h.RespondWithError(w, r, http.StatusBadRequest, "Invalid request format", "invalid_format")
		return
	}

	// Валидация запроса
	if validationErrors, err := h.ValidateRequest(req); err != nil {
		h.Logger.Error("Request validation error", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Validation failed", "validation_error")
		return
	} else if len(validationErrors) > 0 {
		h.RespondWithValidationErrors(w, r, validationErrors)
		return
	}

	// Обновляем роль участника проекта
	member, err := h.projectService.UpdateMember(r.Context(), projectID, memberID, req, userID)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Project not found", "project_not_found")
			return
		}
		if errors.Is(err, service.ErrMemberNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Member not found", "member_not_found")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Insufficient rights to update member role", "insufficient_rights")
			return
		}
		h.Logger.Error("Failed to update member role", err, "project_id", projectID, "member_id", memberID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to update member role", "update_role_failed")
		return
	}

	h.RespondWithSuccess(w, r, member)
}

// RemoveProjectMember удаляет участника из проекта
func (h *ProjectHandler) RemoveProjectMember(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID проекта из URL
	projectID := h.GetURLParam(r, "id")
	if projectID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Project ID is required", "missing_id")
		return
	}

	// Получаем ID участника из URL
	memberID := h.GetURLParam(r, "member_id")
	if memberID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Member ID is required", "missing_member_id")
		return
	}

	// Удаляем участника из проекта
	if err := h.projectService.RemoveMember(r.Context(), projectID, memberID, userID); err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Project not found", "project_not_found")
			return
		}
		if errors.Is(err, service.ErrMemberNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Member not found", "member_not_found")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Insufficient rights to remove members", "insufficient_rights")
			return
		}
		h.Logger.Error("Failed to remove member from project", err, "project_id", projectID, "member_id", memberID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to remove member", "remove_member_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]bool{"success": true})
}

// GetProjectMetrics возвращает метрики проекта
func (h *ProjectHandler) GetProjectMetrics(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID проекта из URL
	projectID := h.GetURLParam(r, "id")
	if projectID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Project ID is required", "missing_id")
		return
	}

	// Получаем метрики проекта
	metrics, err := h.projectService.GetProjectMetrics(r.Context(), projectID, userID)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Project not found", "project_not_found")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Access denied to the project", "access_denied")
			return
		}
		h.Logger.Error("Failed to get project metrics", err, "id", projectID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get project metrics", "metrics_fetch_failed")
		return
	}

	

// GetProject возвращает информацию о проекте по ID
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID проекта из URL
	projectID := h.GetURLParam(r, "id")
	if projectID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Project ID is required", "missing_id")
		return
	}

	// Получаем данные проекта
	project, err := h.projectService.GetByID(r.Context(), projectID, userID)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Project not found", "project_not_found")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Access denied to the project", "access_denied")
			return
		}
		h.Logger.Error("Failed to get project", err, "id", projectID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get project info", "project_fetch_failed")
		return
	}

	h.RespondWithSuccess(w, r, project)
}

// UpdateProject обновляет информацию о проекте
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID проекта из URL
	projectID := h.GetURLParam(r, "id")
	if projectID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Project ID is required", "missing_id")
		return
	}

	var req domain.ProjectUpdateRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse update project request", err)
		h.RespondWithError(w, r, http.StatusBadRequest, "Invalid request format", "invalid_format")
		return
	}

	// Валидация запроса
	if validationErrors, err := h.ValidateRequest(req); err != nil {
		h.Logger.h.RespondWithSuccess(w, r, metrics)
	}Error("Request validation error", err)
			h.RespondWithError(w, r, http.StatusInternalServerError, "Validation failed", "validation_error")
			return
		} else if len(validationErrors) > 0 {
			h.RespondWithValidationErrors(w, r, validationErrors)
			return
		}

		// Создаем проект
		project, err := h.projectService.Create(r.Context(), req, userID)
		if err != nil {
			h.Logger.Error("Failed to create project", err)
			h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to create project", "creation_failed")
			return
		}

		h.RespondWithSuccess(w, r, project)
	}