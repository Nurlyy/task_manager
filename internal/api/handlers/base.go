package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"github.com/yourusername/task-tracker/internal/domain"
	"github.com/yourusername/task-tracker/pkg/auth"
	"github.com/yourusername/task-tracker/pkg/logger"
)

// StandardResponseData представляет стандартную структуру ответа API
type StandardResponseData struct {
	Success      bool        `json:"success"`
	Data         interface{} `json:"data,omitempty"`
	ErrorMessage string      `json:"error,omitempty"`
	ErrorCode    string      `json:"error_code,omitempty"`
	Meta         interface{} `json:"meta,omitempty"`
}

// ErrorResponse представляет структуру ответа с ошибкой
type ErrorResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error"`
	ErrorCode    string `json:"error_code,omitempty"`
}

// ValidationError представляет ошибку валидации
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrorResponse представляет структуру ответа с ошибками валидации
type ValidationErrorResponse struct {
	Success bool              `json:"success"`
	Error   string            `json:"error"`
	Errors  []ValidationError `json:"errors"`
}

// PaginationMeta представляет метаданные для постраничной навигации
type PaginationMeta struct {
	TotalItems  int `json:"total_items"`
	TotalPages  int `json:"total_pages"`
	CurrentPage int `json:"current_page"`
	PageSize    int `json:"page_size"`
}

// BaseHandler содержит общие методы для всех обработчиков
type BaseHandler struct {
	Logger     logger.Logger
	Validator  *validator.Validate
	JWTManager *auth.JWTManager
}

// NewBaseHandler создает новый экземпляр BaseHandler
func NewBaseHandler(logger logger.Logger, jwtManager *auth.JWTManager) BaseHandler {
	return BaseHandler{
		Logger:     logger,
		Validator:  validator.New(),
		JWTManager: jwtManager,
	}
}

// Respond отправляет стандартный ответ с указанным кодом статуса
func (h *BaseHandler) Respond(w http.ResponseWriter, r *http.Request, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			h.Logger.Error("Failed to encode response", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

// RespondWithSuccess отправляет успешный ответ
func (h *BaseHandler) RespondWithSuccess(w http.ResponseWriter, r *http.Request, data interface{}) {
	response := StandardResponseData{
		Success: true,
		Data:    data,
	}
	h.Respond(w, r, http.StatusOK, response)
}

// RespondWithError отправляет ответ с ошибкой
func (h *BaseHandler) RespondWithError(w http.ResponseWriter, r *http.Request, statusCode int, errorMsg string, errorCode string) {
	response := ErrorResponse{
		Success:      false,
		ErrorMessage: errorMsg,
		ErrorCode:    errorCode,
	}
	h.Respond(w, r, statusCode, response)
}

// RespondWithValidationErrors отправляет ответ с ошибками валидации
func (h *BaseHandler) RespondWithValidationErrors(w http.ResponseWriter, r *http.Request, errors []ValidationError) {
	response := ValidationErrorResponse{
		Success: false,
		Error:   "Validation failed",
		Errors:  errors,
	}
	h.Respond(w, r, http.StatusBadRequest, response)
}

// RespondWithPagination отправляет ответ с пагинацией
func (h *BaseHandler) RespondWithPagination(w http.ResponseWriter, r *http.Request, data interface{}, pagedResponse *domain.PagedResponse) {
	meta := PaginationMeta{
		TotalItems:  pagedResponse.TotalItems,
		TotalPages:  pagedResponse.TotalPages,
		CurrentPage: pagedResponse.Page,
		PageSize:    pagedResponse.PageSize,
	}

	response := StandardResponseData{
		Success: true,
		Data:    data,
		Meta:    meta,
	}

	h.Respond(w, r, http.StatusOK, response)
}

// ParseJSON разбирает JSON из тела запроса
func (h *BaseHandler) ParseJSON(r *http.Request, dst interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	return nil
}

// ValidateRequest проверяет валидность структуры запроса
func (h *BaseHandler) ValidateRequest(data interface{}) ([]ValidationError, error) {
	if err := h.Validator.Struct(data); err != nil {
		var validationErrors []ValidationError
		if errors, ok := err.(validator.ValidationErrors); ok {
			for _, err := range errors {
				validationError := ValidationError{
					Field:   err.Field(),
					Message: getErrorMsg(err),
				}
				validationErrors = append(validationErrors, validationError)
			}
			return validationErrors, nil
		}
		return nil, err
	}
	return nil, nil
}

// GetPaginationParams извлекает параметры пагинации из запроса
func (h *BaseHandler) GetPaginationParams(r *http.Request) (int, int) {
	page := 1
	pageSize := 20

	if pageParam := r.URL.Query().Get("page"); pageParam != "" {
		if parsed, err := strconv.Atoi(pageParam); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if pageSizeParam := r.URL.Query().Get("page_size"); pageSizeParam != "" {
		if parsed, err := strconv.Atoi(pageSizeParam); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	return page, pageSize
}

// GetUserIDFromContext извлекает ID пользователя из контекста запроса
func (h *BaseHandler) GetUserIDFromContext(r *http.Request) (string, error) {
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		return "", errors.New("user ID not found in context")
	}
	return userID, nil
}

// GetURLParam извлекает параметр из URL
func (h *BaseHandler) GetURLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

// Функция для получения человекочитаемого сообщения об ошибке валидации
func getErrorMsg(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		return fmt.Sprintf("Minimum length is %s", err.Param())
	case "max":
		return fmt.Sprintf("Maximum length is %s", err.Param())
	case "uuid":
		return "Invalid UUID format"
	case "oneof":
		return fmt.Sprintf("Value must be one of: %s", err.Param())
	case "gtfield":
		return fmt.Sprintf("Value must be greater than %s field", err.Param())
	default:
		return fmt.Sprintf("Validation failed on '%s' with value '%v'", err.Tag(), err.Value())
	}
}