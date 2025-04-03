package handlers

import (
	"errors"
	"net/http"

	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/service"
)

// AuthHandler обрабатывает запросы, связанные с аутентификацией
type AuthHandler struct {
	BaseHandler
	userService *service.UserService
}

// NewAuthHandler создает новый экземпляр AuthHandler
func NewAuthHandler(base BaseHandler, userService *service.UserService) *AuthHandler {
	return &AuthHandler{
		BaseHandler: base,
		userService: userService,
	}
}

// Register обрабатывает запрос на регистрацию нового пользователя
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req domain.UserCreateRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse register request", err)
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

	// Создаем пользователя
	user, err := h.userService.Create(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrEmailAlreadyExists) {
			h.RespondWithError(w, r, http.StatusConflict, "Email already exists", "email_exists")
			return
		}
		h.Logger.Error("Failed to create user", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to create user", "creation_failed")
		return
	}

	h.RespondWithSuccess(w, r, user)
}

// Login обрабатывает запрос на вход пользователя
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse login request", err)
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

	// Аутентификация пользователя
	response, err := h.userService.Login(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			h.RespondWithError(w, r, http.StatusUnauthorized, "Invalid credentials", "invalid_credentials")
			return
		}
		h.Logger.Error("Login failed", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Login failed", "login_failed")
		return
	}

	h.RespondWithSuccess(w, r, response)
}

// RefreshToken обрабатывает запрос на обновление токена доступа
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req domain.RefreshTokenRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse refresh token request", err)
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

	// Обновление токенов
	response, err := h.userService.RefreshToken(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) || errors.Is(err, service.ErrInvalidCredentials) {
			h.RespondWithError(w, r, http.StatusUnauthorized, "Invalid refresh token", "invalid_token")
			return
		}
		h.Logger.Error("Token refresh failed", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Token refresh failed", "refresh_failed")
		return
	}

	h.RespondWithSuccess(w, r, response)
}

// ChangePassword обрабатывает запрос на изменение пароля
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	var req domain.ChangePasswordRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse change password request", err)
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

	// Изменение пароля
	if err := h.userService.ChangePassword(r.Context(), userID, req); err != nil {
		if errors.Is(err, service.ErrInvalidPassword) {
			h.RespondWithError(w, r, http.StatusBadRequest, "Invalid old password", "invalid_password")
			return
		}
		h.Logger.Error("Change password failed", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Change password failed", "password_change_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]bool{"success": true})
}

// GetCurrentUser возвращает информацию о текущем пользователе
func (h *AuthHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	user, err := h.userService.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "User not found", "user_not_found")
			return
		}
		h.Logger.Error("Failed to get current user", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get user info", "user_fetch_failed")
		return
	}

	h.RespondWithSuccess(w, r, user)
}