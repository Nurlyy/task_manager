package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nurlyy/task_manager/internal/app"
	"github.com/nurlyy/task_manager/pkg/config"
	"github.com/nurlyy/task_manager/pkg/logger"
)

func main() {
	// Создаем контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Инициализируем логгер
	log := logger.NewLogger(cfg.App.LogLevel, cfg.App.Environment == "production")
	log.Info("Starting API server", map[string]interface{}{
		"app_name": cfg.App.Name,
		"env":      cfg.App.Environment,
	})

	// Инициализируем приложение
	application, err := app.NewApplication(ctx, cfg, log)
	if err != nil {
		log.Fatal("Failed to initialize application", err)
	}
	defer application.Close()

	// Проверяем подключение к базе данных
	if err := application.DB.PingContext(ctx); err != nil {
		log.Fatal("Failed to ping database", err)
	}

	// TODO: Инициализация маршрутизатора API и HTTP-обработчиков
	// router := api.NewRouter(application)

	// Настройка HTTP-сервера
	addr := fmt.Sprintf(":%s", cfg.HTTP.Port)
	srv := &http.Server{
		Addr:         addr,
		// Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// Запуск сервера в горутине
	go func() {
		log.Info("Starting HTTP server", map[string]interface{}{
			"addr": addr,
		})

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", err)
			cancel()
		}
	}()

	// Настройка graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Ожидаем сигнал или отмену контекста
	select {
	case <-quit:
		log.Info("Shutting down server...")
	case <-ctx.Done():
		log.Info("Shutting down server due to context cancellation...")
	}

	// Создаем контекст с таймаутом для graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutdownCancel()

	// Останавливаем сервер
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server shutdown error", err)
	}

	log.Info("Server gracefully stopped")
}