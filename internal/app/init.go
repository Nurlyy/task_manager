package app

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/nurlyy/task_manager/internal/repository/cache"
	"github.com/nurlyy/task_manager/internal/repository/postgres"
	"github.com/nurlyy/task_manager/internal/messaging"
	"github.com/nurlyy/task_manager/pkg/config"
	"github.com/nurlyy/task_manager/pkg/database"
	redisClient "github.com/nurlyy/task_manager/pkg/cache"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// Repositories содержит все репозитории для работы с хранилищами данных
type Repositories struct {
	UserRepository         *postgres.UserRepository
	ProjectRepository      *postgres.ProjectRepository
	TaskRepository         *postgres.TaskRepository
	CommentRepository      *postgres.CommentRepository
	NotificationRepository *postgres.NotificationRepository
	CacheRepository        *cache.RedisRepository
}

// Messaging содержит все клиенты для работы с сообщениями
type Messaging struct {
	Producer *messaging.KafkaProducer
}

// Application содержит все компоненты приложения
type Application struct {
	Config       *config.Config
	DB           *sqlx.DB
	Redis        *redisClient.Redis
	Logger       logger.Logger
	Repositories *Repositories
	Messaging    *Messaging
}

// NewApplication создает новое приложение с инициализированными компонентами
func NewApplication(ctx context.Context, cfg *config.Config, log logger.Logger) (*Application, error) {
	// Инициализация базы данных PostgreSQL
	postgresDB, err := initPostgres(ctx, &cfg.Database, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PostgreSQL: %w", err)
	}

	// Инициализация Redis
	redisCache, err := initRedis(ctx, &cfg.Redis, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis: %w", err)
	}

	// Инициализация репозиториев
	repos, err := initRepositories(postgresDB, redisCache, log, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repositories: %w", err)
	}

	// Инициализация Kafka
	msgClients, err := initMessaging(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize messaging: %w", err)
	}

	return &Application{
		Config:       cfg,
		DB:           postgresDB,
		Redis:        redisCache,
		Logger:       log,
		Repositories: repos,
		Messaging:    msgClients,
	}, nil
}

// Close закрывает все соединения с внешними сервисами
func (app *Application) Close() {
	if app.DB != nil {
		if err := app.DB.Close(); err != nil {
			app.Logger.Error("Error closing PostgreSQL connection", err)
		}
	}

	if app.Redis != nil {
		if err := app.Redis.Close(); err != nil {
			app.Logger.Error("Error closing Redis connection", err)
		}
	}

	if app.Messaging.Producer != nil {
		if err := app.Messaging.Producer.Close(); err != nil {
			app.Logger.Error("Error closing Kafka producer", err)
		}
	}
}

// Инициализация PostgreSQL
func initPostgres(ctx context.Context, cfg *config.DatabaseConfig, log logger.Logger) (*sqlx.DB, error) {
	postgres, err := database.NewPostgres(ctx, cfg, log)
	if err != nil {
		return nil, err
	}
	return postgres.DB, nil
}

// Инициализация Redis
func initRedis(ctx context.Context, cfg *config.RedisConfig, log logger.Logger) (*redisClient.Redis, error) {
	redis, err := redisClient.NewRedis(ctx, cfg, log)
	if err != nil {
		return nil, err
	}
	return redis, nil
}

// Инициализация репозиториев
func initRepositories(db *sqlx.DB, redis *redisClient.Redis, log logger.Logger, cfg *config.Config) (*Repositories, error) {
	// Инициализация PostgreSQL репозиториев
	userRepo := postgres.NewUserRepository(db, log)
	projectRepo := postgres.NewProjectRepository(db, log)
	taskRepo := postgres.NewTaskRepository(db, log)
	commentRepo := postgres.NewCommentRepository(db, log)
	notificationRepo := postgres.NewNotificationRepository(db, log)

	// Инициализация Redis репозитория
	cacheRepo := cache.NewRedisRepository(redis.Client, log, cfg.Redis.DefaultTTL)

	return &Repositories{
		UserRepository:         userRepo,
		ProjectRepository:      projectRepo,
		TaskRepository:         taskRepo,
		CommentRepository:      commentRepo,
		NotificationRepository: notificationRepo,
		CacheRepository:        cacheRepo,
	}, nil
}

// Инициализация Kafka
func initMessaging(cfg *config.Config, log logger.Logger) (*Messaging, error) {
	// Определяем топики Kafka
	topics := map[string]string{
		"task_created":         cfg.Kafka.Topics.TaskCreated,
		"task_updated":         cfg.Kafka.Topics.TaskUpdated,
		"task_assigned":        cfg.Kafka.Topics.TaskAssigned,
		"task_commented":       cfg.Kafka.Topics.TaskCommented,
		"project_created":      "project_created",
		"project_updated":      "project_updated",
		"project_member_added": "project_member_added",
		"notifications":        cfg.Kafka.Topics.Notifications,
	}

	// Инициализация Kafka продюсера
	producer := messaging.NewKafkaProducer(cfg.Kafka.Brokers, topics, log)

	return &Messaging{
		Producer: producer,
	}, nil
}