package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// CustomValidator - структура для валидации данных
type CustomValidator struct {
	validator *validator.Validate
}

// ValidationError представляет ошибку валидации
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors содержит список ошибок валидации
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

// Error реализует интерфейс error
func (ve ValidationErrors) Error() string {
	var errMsgs []string
	for _, err := range ve.Errors {
		errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return strings.Join(errMsgs, "; ")
}

// NewValidator создает новый экземпляр валидатора
func NewValidator() *CustomValidator {
	v := validator.New()

	// Регистрируем функцию для получения JSON-тега вместо имени структуры
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	return &CustomValidator{
		validator: v,
	}
}

// RegisterCustomValidations регистрирует кастомные валидации
func (cv *CustomValidator) RegisterCustomValidations() {
	// Пример регистрации кастомной валидации
	cv.validator.RegisterValidation("task_status", validateTaskStatus)
}

// Пример кастомной валидации
func validateTaskStatus(fl validator.FieldLevel) bool {
	status := fl.Field().String()
	validStatuses := map[string]bool{
		"new":        true,
		"in_progress": true,
		"on_hold":    true,
		"completed":  true,
		"cancelled":  true,
	}
	return validStatuses[status]
}

// Validate проверяет структуру на соответствие правилам валидации
func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		var validationErrors ValidationErrors

		for _, err := range err.(validator.ValidationErrors) {
			field := err.Field()
			message := getErrorMessage(err)

			validationErrors.Errors = append(validationErrors.Errors, ValidationError{
				Field:   field,
				Message: message,
			})
		}

		return validationErrors
	}
	return nil
}

// getErrorMessage возвращает понятное сообщение об ошибке на основе тега валидации
func getErrorMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		if err.Type().Kind() == reflect.String {
			return fmt.Sprintf("Must be at least %s characters long", err.Param())
		}
		return fmt.Sprintf("Must be at least %s", err.Param())
	case "max":
		if err.Type().Kind() == reflect.String {
			return fmt.Sprintf("Must be at most %s characters long", err.Param())
		}
		return fmt.Sprintf("Must be at most %s", err.Param())
	case "task_status":
		return "Invalid task status"
	default:
		return fmt.Sprintf("Failed validation for '%s'", err.Tag())
	}
}