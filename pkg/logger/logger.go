package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger - интерфейс для логирования
type Logger interface {
	Debug(msg string, fields ...map[string]interface{})
	Info(msg string, fields ...map[string]interface{})
	Warn(msg string, fields ...map[string]interface{})
	Error(msg string, err error, fields ...map[string]interface{})
	Fatal(msg string, err error, fields ...map[string]interface{})
	With(key string, value interface{}) Logger
}

// ZeroLogger - реализация логгера на основе zerolog
type ZeroLogger struct {
	logger zerolog.Logger
}

// NewLogger создает новый экземпляр логгера
func NewLogger(level string, isJSON bool) *ZeroLogger {
	// Настройка уровня логирования
	logLevel, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	// Настройка формата времени
	zerolog.TimeFieldFormat = time.RFC3339

	var logger zerolog.Logger
	if isJSON {
		// Для продакшена используем JSON
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		// Для разработки используем консольный вывод
		output := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
		}
		logger = zerolog.New(output).With().Timestamp().Logger()
	}

	return &ZeroLogger{
		logger: logger,
	}
}

// Debug логирует отладочное сообщение
func (l *ZeroLogger) Debug(msg string, fields ...map[string]interface{}) {
	event := l.logger.Debug()
	if len(fields) > 0 {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}
	event.Msg(msg)
}

// Info логирует информационное сообщение
func (l *ZeroLogger) Info(msg string, fields ...map[string]interface{}) {
	event := l.logger.Info()
	if len(fields) > 0 {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}
	event.Msg(msg)
}

// Warn логирует предупреждение
func (l *ZeroLogger) Warn(msg string, fields ...map[string]interface{}) {
	event := l.logger.Warn()
	if len(fields) > 0 {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}
	event.Msg(msg)
}

// Error логирует ошибку
func (l *ZeroLogger) Error(msg string, err error, fields ...map[string]interface{}) {
	event := l.logger.Error()
	if err != nil {
		event = event.Err(err)
	}
	if len(fields) > 0 {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}
	event.Msg(msg)
}

// Fatal логирует критическую ошибку и завершает программу
func (l *ZeroLogger) Fatal(msg string, err error, fields ...map[string]interface{}) {
	event := l.logger.Fatal()
	if err != nil {
		event = event.Err(err)
	}
	if len(fields) > 0 {
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}
	event.Msg(msg)
}

// With добавляет постоянное поле к логгеру
func (l *ZeroLogger) With(key string, value interface{}) Logger {
	return &ZeroLogger{
		logger: l.logger.With().Interface(key, value).Logger(),
	}
}