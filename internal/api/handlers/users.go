package handlers

import (
	"errors"
	"net/http"

	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/internal/service"
)

// UserHandler обрабатывает запросы, связанные с пользователями
type UserHandler struct {
	BaseHandler
	userService *service.UserService
}

// NewUserHandler создает новый экземпляр UserHandler
func NewUserHandler(base BaseHandler, userService *service.UserService) *UserHandler {
	return &UserHandler{
		BaseHandler: base,
		userService: userService,
	}
}

// GetUser возвращает информацию о пользователе по ID
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	// Получаем ID текущего пользователя из контекста
	currentUserID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID запрашиваемого пользователя из URL
	userID := h.GetURLParam(r, "id")
	if userID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "User ID is required", "missing_id")
		return
	}

	// Получаем данные пользователя
	user, err := h.userService.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "User not found", "user_not_found")
			return
		}
		h.Logger.Error("Failed to get user", err, "id", userID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get user info", "user_fetch_failed")
		return
	}

	h.RespondWithSuccess(w, r, user)
}

// UpdateUser обновляет информацию о пользователе
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	// Получаем ID текущего пользователя из контекста
	currentUserID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID пользователя для обновления из URL
	userID := h.GetURLParam(r, "id")
	if userID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "User ID is required", "missing_id")
		return
	}

	// Проверяем, что пользователь обновляет свой профиль или является администратором
	currentUser, err := h.userService.GetByID(r.Context(), currentUserID)
	if err != nil {
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get user info", "user_fetch_failed")
		return
	}

	// Только администратор может обновлять других пользователей
	if userID != currentUserID && currentUser.Role != domain.UserRoleAdmin {
		h.RespondWithError(w, r, http.StatusForbidden, "Permission denied", "permission_denied")
		return
	}

	var req domain.UserUpdateRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse update user request", err)
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

	// Только администратор может менять роль пользователя
	if req.Role != nil && currentUser.Role != domain.UserRoleAdmin {
		h.RespondWithError(w, r, http.StatusForbidden, "Permission denied to change role", "permission_denied")
		return
	}

	// Обновляем данные пользователя
	user, err := h.userService.Update(r.Context(), userID, req)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "User not found", "user_not_found")
			return
		}
		h.Logger.Error("Failed to update user", err, "id", userID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to update user", "update_failed")
		return
	}

	h.RespondWithSuccess(w, r, user)
}

// DeleteUser удаляет пользователя
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	// Получаем ID текущего пользователя из контекста
	currentUserID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID пользователя для удаления из URL
	userID := h.GetURLParam(r, "id")
	if userID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "User ID is required", "missing_id")
		return
	}

	// Проверяем, что пользователь удаляет свой профиль или является администратором
	currentUser, err := h.userService.GetByID(r.Context(), currentUserID)
	if err != nil {
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get user info", "user_fetch_failed")
		return
	}

	// Только администратор может удалять других пользователей
	if userID != currentUserID && currentUser.Role != domain.UserRoleAdmin {
		h.RespondWithError(w, r, http.StatusForbidden, "Permission denied", "permission_denied")
		return
	}

	// Удаляем пользователя
	if err := h.userService.Delete(r.Context(), userID); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "User not found", "user_not_found")
			return
		}
		h.Logger.Error("Failed to delete user", err, "id", userID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to delete user", "delete_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]bool{"success": true})
}

// ListUsers возвращает список пользователей с фильтрацией
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	// Получаем ID текущего пользователя из контекста
	currentUserID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Проверяем, что пользователь имеет права на просмотр списка пользователей
	currentUser, err := h.userService.GetByID(r.Context(), currentUserID)
	if err != nil {
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get user info", "user_fetch_failed")
		return
	}

	// Только администраторы и менеджеры могут просматривать список всех пользователей
	if currentUser.Role != domain.UserRoleAdmin && currentUser.Role != domain.UserRoleManager {
		h.RespondWithError(w, r, http.StatusForbidden, "Permission denied", "permission_denied")
		return
	}

	// Параметры пагинации
	page, pageSize := h.GetPaginationParams(r)

	// Создаем фильтр
	filter := repository.UserFilter{
		SearchText: getStringPtr(r.URL.Query().Get("search")),
	}

	// Фильтр по роли
	if role := r.URL.Query().Get("role"); role != "" {
		userRole := domain.UserRole(role)
		filter.Role = &userRole
	}

	// Фильтр по активности
	if isActive := r.URL.Query().Get("is_active"); isActive != "" {
		active := isActive == "true"
		filter.IsActive = &active
	}

	// Фильтр по отделу
	if department := r.URL.Query().Get("department"); department != "" {
		filter.Department = &department
	}

	// Получаем список пользователей
	result, err := h.userService.List(r.Context(), filter, page, pageSize)
	if err != nil {
		h.Logger.Error("Failed to list users", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get users", "users_fetch_failed")
		return
	}

	h.RespondWithPagination(w, r, result.Items, result)
}

// Вспомогательная функция для получения указателя на строку
func getStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}