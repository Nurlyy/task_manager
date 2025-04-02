package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/yourusername/task-tracker/pkg/auth"
	"github.com/yourusername/task-tracker/pkg/logger"
)

// AuthMiddleware предоставляет middleware для аутентификации пользователей
type AuthMiddleware struct {
	jwtManager *auth.JWTManager
	logger     logger.Logger
}

// NewAuthMiddleware создает новый экземпляр AuthMiddleware
func NewAuthMiddleware(jwtManager *auth.JWTManager, logger logger.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager: jwtManager,
		logger:     logger,
	}
}

// Authenticate проверяет наличие и валидность JWT токена
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем токен из заголовка Authorization
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Проверяем формат токена
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}
		tokenString := parts[1]

		// Проверяем валидность токена
		claims, err := m.jwtManager.VerifyToken(tokenString)
		if err != nil {
			m.logger.Warn("Invalid JWT token", "error", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Проверяем, что это токен доступа
		if claims.Type != string(auth.AccessToken) {
			http.Error(w, "Invalid token type", http.StatusUnauthorized)
			return
		}

		// Добавляем информацию о пользователе в контекст запроса
		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "user_email", claims.Email)
		ctx = context.WithValue(ctx, "user_role", claims.Role)

		// Вызываем следующий обработчик с обновленным контекстом
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Optional проверяет JWT токен, если он есть, но не требует его наличия
func (m *AuthMiddleware) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем токен из заголовка Authorization
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// Токен отсутствует, но это нормально для опциональной аутентификации
			next.ServeHTTP(w, r)
			return
		}

		// Проверяем формат токена
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			// Некорректный формат, но продолжаем как неаутентифицированный запрос
			next.ServeHTTP(w, r)
			return
		}
		tokenString := parts[1]

		// Проверяем валидность токена
		claims, err := m.jwtManager.VerifyToken(tokenString)
		if err != nil {
			// Невалидный токен, но продолжаем как неаутентифицированный запрос
			next.ServeHTTP(w, r)
			return
		}

		// Проверяем, что это токен доступа
		if claims.Type != string(auth.AccessToken) {
			// Некорректный тип токена, но продолжаем как неаутентифицированный запрос
			next.ServeHTTP(w, r)
			return
		}

		// Добавляем информацию о пользователе в контекст запроса
		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "user_email", claims.Email)
		ctx = context.WithValue(ctx, "user_role", claims.Role)

		// Вызываем следующий обработчик с обновленным контекстом
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole проверяет, имеет ли пользователь требуемую роль
func (m *AuthMiddleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return m.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем роль пользователя из контекста
			userRole, ok := r.Context().Value("user_role").(string)
			if !ok {
				http.Error(w, "User role not found in context", http.StatusInternalServerError)
				return
			}

			// Проверяем соответствие роли
			if userRole != role && userRole != "admin" { // админы имеют доступ ко всем ресурсам
				http.Error(w, "Insufficient permissions", http.StatusForbidden)
				return
			}

			// Вызываем следующий обработчик
			next.ServeHTTP(w, r)
		}))
	}
}