package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nurlyy/task_manager/internal/app"
	"github.com/nurlyy/task_manager/internal/service"
	"github.com/nurlyy/task_manager/pkg/config"
	applogger "github.com/nurlyy/task_manager/pkg/logger"
)

func main() {
	// Инициализируем контекст приложения
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Обновляем контекст приложения в конфигурации
	cfg.App.Context = ctx

	// Инициализируем логгер
	logger, err := applogger.NewLogger(cfg.App.LogLevel, false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	logger.Info("Starting scheduler service")

	// Инициализируем основное приложение
	application, err := app.NewApplication(ctx, cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize application", err)
	}
	defer application.Close()

	// Инициализируем сервис планировщика
	schedulerService := service.NewSchedulerService(
		application.Repositories.TaskRepository,
		application.Repositories.UserRepository,
		application.Repositories.ProjectRepository,
		application.Repositories.NotificationRepository,
		application.Messaging.Producer,
		&cfg.Scheduler,
		logger,
	)

	// Запускаем планировщик
	if err := schedulerService.Start(ctx); err != nil {
		logger.Fatal("Failed to start scheduler service", err)
	}

	// Создаем канал для перехвата сигналов остановки
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Блокируем основную горутину до получения сигнала остановки
	<-stop
	logger.Info("Shutting down scheduler service")

	// Создаем контекст с таймаутом для остановки
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Ожидаем завершения всех задач
	<-shutdownCtx.Done()
	logger.Info("Scheduler service stopped")
}
