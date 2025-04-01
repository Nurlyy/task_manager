package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/task-tracker/pkg/config"
	"github.com/yourusername/task-tracker/pkg/database"
	"github.com/yourusername/task-tracker/pkg/logger"
	"github.com/yourusername/task-tracker/pkg/cache"
	"github.com/yourusername/task-tracker/pkg/messaging"
)

func main() {
	// Контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Инициализация логгера
	log := logger.NewLogger(cfg.App.LogLevel, cfg.App.Environment == "production")
	log.Info("Starting API server", map[string]interface{}{
		"app_name": cfg.App.Name,
		"env":      cfg.App.Environment,
	})

	// Подключение к PostgreSQL
	db, err := database.NewPostgres(ctx, &cfg.Database, log)
	if err != nil {
		log.Fatal("Failed to connect to database", err)
	}
	defer db.Close()

	// Подключение к Redis
	redisClient, err := cache.NewRedis(ctx, &cfg.Redis, log)
	if err != nil {
		log.Fatal("Failed to connect to Redis", err)
	}
	defer redisClient.Close()

	// Инициализация Kafka Producer
	kafkaProducer := messaging.NewKafkaProducer(&cfg.Kafka, log)
	defer kafkaProducer.Close()

	// Создание топиков Kafka (если их еще нет)
	topics := []string{
		cfg.Kafka.Topics.TaskCreated,
		cfg.Kafka.Topics.TaskUpdated,
		cfg.Kafka.Topics.TaskAssigned,
		cfg.Kafka.Topics.TaskCommented,
		cfg.Kafka.Topics.Notifications,
	}
	if err := messaging.CreateTopics(ctx, cfg.Kafka.Brokers, topics, log); err != nil {
		log.Warn("Failed to create Kafka topics", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Здесь будет инициализация репозиториев, сервисов и API

	// Настройка HTTP-сервера
	addr := fmt.Sprintf(":%s", cfg.HTTP.Port)
	srv := &http.Server{
		Addr:         addr,
		// Handler будет инициализирован позже
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