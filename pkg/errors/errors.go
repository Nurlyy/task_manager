package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Типы ошибок
var (
	ErrInternalServer   = errors.New("internal server error")
	ErrNotFound         = errors.New("resource not found")
	ErrBadRequest       = errors.New("bad request")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrConflict         = errors.New("conflict")
	ErrValidation       = errors.New("validation error")
	ErrTimeout          = errors.New("request timeout")
	ErrServiceUnavailable = errors.New("service unavailable")
)

// AppError представляет ошибку приложения
type AppError struct {
	Err        error
	StatusCode int
	Message    string
	Code       string
	Data       interface{}
}

// Error реализует интерфейс error
func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

// Unwrap возвращает оборачиваемую ошибку
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError создает новую ошибку приложения
func NewAppError(err error, statusCode int, message, code string, data interface{}) *AppError {
	return &AppError{
		Err:        err,
		StatusCode: statusCode,
		Message:    message,
		Code:       code,
		Data:       data,
	}
}

// FromError создает AppError из обычной ошибки
func FromError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	// Проверяем стандартные ошибки
	switch {
	case errors.Is(err, ErrNotFound):
		return NewAppError(err, http.StatusNotFound, "Resource not found", "not_found", nil)
	case errors.Is(err, ErrBadRequest):
		return NewAppError(err, http.StatusBadRequest, "Bad request", "bad_request", nil)
	case errors.Is(err, ErrUnauthorized):
		return NewAppError(err, http.StatusUnauthorized, "Unauthorized", "unauthorized", nil)
	case errors.Is(err, ErrForbidden):
		return NewAppError(err, http.StatusForbidden, "Forbidden", "forbidden", nil)
	case errors.Is(err, ErrConflict):
		return NewAppError(err, http.StatusConflict, "Conflict", "conflict", nil)
	case errors.Is(err, ErrValidation):
		return NewAppError(err, http.StatusUnprocessableEntity, "Validation error", "validation_error", nil)
	case errors.Is(err, ErrTimeout):
		return NewAppError(err, http.StatusRequestTimeout, "Request timeout", "request_timeout", nil)
	case errors.Is(err, ErrServiceUnavailable):
		return NewAppError(err, http.StatusServiceUnavailable, "Service unavailable", "service_unavailable", nil)
	default:
		return NewAppError(err, http.StatusInternalServerError, "Internal server error", "internal_error", nil)
	}
}

// Error создает и возвращает новую ошибку
func Error(err error, message string) error {
	return fmt.Errorf("%s: %w", message, err)
}

// NotFound создает ошибку 404 Not Found
func NotFound(entity string, id interface{}) *AppError {
	msg := fmt.Sprintf("%s with ID %v not found", entity, id)
	return NewAppError(ErrNotFound, http.StatusNotFound, msg, "not_found", nil)
}

// BadRequest создает ошибку 400 Bad Request
func BadRequest(message string) *AppError {
	return NewAppError(ErrBadRequest, http.StatusBadRequest, message, "bad_request", nil)
}

// Unauthorized создает ошибку 401 Unauthorized
func Unauthorized(message string) *AppError {
	if message == "" {
		message = "Authentication required"
	}
	return NewAppError(ErrUnauthorized, http.StatusUnauthorized, message, "unauthorized", nil)
}

// Forbidden создает ошибку 403 Forbidden
func Forbidden(message string) *AppError {
	if message == "" {
		message = "You don't have permission to perform this action"
	}
	return NewAppError(ErrForbidden, http.StatusForbidden, message, "forbidden", nil)
}

// Conflict создает ошибку 409 Conflict
func Conflict(entity string, field string, value interface{}) *AppError {
	msg := fmt.Sprintf("%s with %s %v already exists", entity, field, value)
	return NewAppError(ErrConflict, http.StatusConflict, msg, "conflict", nil)
}

// ValidationError создает ошибку 422 Unprocessable Entity
func ValidationError(data interface{}) *AppError {
	return NewAppError(ErrValidation, http.StatusUnprocessableEntity, "Validation failed", "validation_error", data)
}

// InternalServer создает ошибку 500 Internal Server Error
func InternalServer(err error) *AppError {
	return NewAppError(err, http.StatusInternalServerError, "Internal server error", "internal_error", nil)
}

// ServiceUnavailable создает ошибку 503 Service Unavailable
func ServiceUnavailable(message string) *AppError {
	if message == "" {
		message = "Service is temporarily unavailable"
	}
	return NewAppError(ErrServiceUnavailable, http.StatusServiceUnavailable, message, "service_unavailable", nil)
}