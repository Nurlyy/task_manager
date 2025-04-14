package domain

import "errors"

// Стандартные ошибки приложения
var (
	// ErrNotFound возвращается, когда запрашиваемый ресурс не найден
	ErrNotFound = errors.New("resource not found")

	// ErrInvalidInput возвращается при невалидных входных данных
	ErrInvalidInput = errors.New("invalid input")

	// ErrUnauthorized возвращается при отсутствии аутентификации
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden возвращается при недостаточных правах доступа
	ErrForbidden = errors.New("forbidden")

	// ErrConflict возвращается при конфликте данных
	ErrConflict = errors.New("conflict")

	// ErrInternal возвращается при внутренней ошибке сервера
	ErrInternal = errors.New("internal server error")

	// ErrBadRequest возвращается при некорректном запросе
	ErrBadRequest = errors.New("bad request")
)
