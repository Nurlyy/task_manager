package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// LoggingMiddleware предоставляет middleware для логирования HTTP запросов
type LoggingMiddleware struct {
	logger logger.Logger
}

// NewLoggingMiddleware создает новый экземпляр LoggingMiddleware
func NewLoggingMiddleware(logger logger.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

// LogRequest логирует информацию о входящих HTTP запросах и ответах
func (m *LoggingMiddleware) LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Генерируем уникальный ID запроса
		requestID := uuid.New().String()

		// Создаем ResponseWriter, который может отслеживать код статуса
		rwWithStatus := newResponseWriterWithStatus(w)

		// Фиксируем время начала обработки запроса
		startTime := time.Now()

		// Логируем информацию о входящем запросе
		m.logger.Info("Incoming request",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)

		// Получаем информацию о пользователе из контекста (если есть)
		userID, userExists := r.Context().Value("user_id").(string)

		// Добавляем ID запроса в заголовок ответа
		w.Header().Set("X-Request-ID", requestID)

		// Вызываем следующий обработчик
		next.ServeHTTP(rwWithStatus, r)

		// Вычисляем длительность обработки запроса
		duration := time.Since(startTime)

		// Логируем информацию о завершении запроса
		logData := []interface{}{
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rwWithStatus.statusCode,
			"duration", duration.String(),
			"duration_ms", duration.Milliseconds(),
		}

		// Добавляем информацию о пользователе, если она есть
		if userExists {
			logData = append(logData, "user_id", userID)
		}

		// Выбираем уровень логирования в зависимости от кода статуса
		if rwWithStatus.statusCode >= 500 {
			m.logger.Error("Request completed with server error", logData...)
		} else if rwWithStatus.statusCode >= 400 {
			m.logger.Warn("Request completed with client error", logData...)
		} else {
			m.logger.Info("Request completed successfully", logData...)
		}
	})
}

// responseWriterWithStatus - обертка для http.ResponseWriter, которая отслеживает код статуса
type responseWriterWithStatus struct {
	http.ResponseWriter
	statusCode int
}

// Создает новый responseWriterWithStatus
func newResponseWriterWithStatus(w http.ResponseWriter) *responseWriterWithStatus {
	return &responseWriterWithStatus{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // По умолчанию 200 OK
	}
}

// WriteHeader переопределяет метод для отслеживания кода статуса
func (rw *responseWriterWithStatus) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Обеспечиваем поддержку http.Hijacker, http.Flusher и http.CloseNotifier, если они поддерживаются базовым ResponseWriter
func (rw *responseWriterWithStatus) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support Hijack")
}

func (rw *responseWriterWithStatus) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *responseWriterWithStatus) CloseNotify() <-chan bool {
	if closeNotifier, ok := rw.ResponseWriter.(http.CloseNotifier); ok {
		return closeNotifier.CloseNotify()
	}
	return nil
}