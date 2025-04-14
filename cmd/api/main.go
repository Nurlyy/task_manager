package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nurlyy/task_manager/internal/api"
	"github.com/nurlyy/task_manager/internal/app"
	"github.com/nurlyy/task_manager/internal/service"
	"github.com/nurlyy/task_manager/pkg/auth"
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

	// Инициализируем менеджер JWT
	jwtManager := auth.NewJWTManager(&cfg.JWT)

	// Инициализируем основное приложение
	application, err := app.NewApplication(ctx, cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize application", err)
	}
	defer application.Close()

	// Инициализируем сервисы
	services, err := initServices(application, jwtManager)
	if err != nil {
		logger.Fatal("Failed to initialize services", err)
	}

	// Инициализируем API сервер
	server := api.NewServer(cfg, logger, jwtManager, services)

	// Создаем канал для перехвата сигналов остановки
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Запускаем сервер в отдельной горутине
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.Start(); err != nil {
			logger.Error("Server error", err)
		}
	}()

	// Ожидаем сигнала остановки
	<-stop
	logger.Info("Shutting down...")

	// Создаем контекст с таймаутом для остановки
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Останавливаем сервер
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown error", err)
	}

	// Ожидаем завершения всех горутин
	wg.Wait()
	logger.Info("Server stopped")
}

// initServices инициализирует все сервисы для API
func initServices(application *app.Application, jwtManager *auth.JWTManager) (*api.Services, error) {
	// Инициализация сервисов

	telegramSender := service.NewTelegramSender(
		application.Config.Telegram.Token,
		app.Repositories.TelegramRepository,
		app.Logger,
	)

	// Настраиваем webhook для Telegram
	webhookURL := fmt.Sprintf("%s/api/v1/webhook/telegram", app.Config.App.BaseURL)
	if err := telegramSender.SetupWebhook(webhookURL); err != nil {
		app.Logger.Warn("Failed to setup Telegram webhook", map[string]interface{}{
			"error": err.Error(),
		})
		// Продолжаем работу даже при ошибке настройки webhook
	}

	userService := service.NewUserService(
		application.Repositories.UserRepository,
		jwtManager,
		application.Repositories.CacheRepository,
		application.Logger,
	)

	projectService := service.NewProjectService(
		application.Repositories.ProjectRepository,
		application.Repositories.UserRepository,
		application.Repositories.TaskRepository,
		application.Repositories.CacheRepository,
		application.Messaging.Producer,
		application.Logger,
	)

	taskService := service.NewTaskService(
		application.Repositories.TaskRepository,
		application.Repositories.ProjectRepository,
		application.Repositories.UserRepository,
		application.Repositories.CommentRepository,
		application.Repositories.CacheRepository,
		application.Messaging.Producer,
		projectService,
		application.Logger,
	)

	commentService := service.NewCommentService(
		application.Repositories.CommentRepository,
		application.Repositories.TaskRepository,
		application.Repositories.UserRepository,
		taskService,
		application.Messaging.Producer,
		application.Logger,
	)

	notificationService := service.NewNotificationService(
		application.Repositories.NotificationRepository,
		application.Repositories.UserRepository,
		application.Repositories.CacheRepository,
		application.Logger,
	)

	return &api.Services{
		UserService:         userService,
		ProjectService:      projectService,
		TaskService:         taskService,
		CommentService:      commentService,
		NotificationService: notificationService,
	}, nil
}
