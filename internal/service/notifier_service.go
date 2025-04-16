package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/messaging"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/pkg/config"
	"github.com/nurlyy/task_manager/pkg/logger"
	"github.com/segmentio/kafka-go"
)

// NotifierService представляет сервис уведомлений
type NotifierService struct {
	notificationRepo repository.NotificationRepository
	userRepo         repository.UserRepository
	taskRepo         repository.TaskRepository
	projectRepo      repository.ProjectRepository
	telegramSender   *TelegramSender
	kafkaReader      *kafka.Reader
	logger           logger.Logger
	config           *config.NotifierConfig
}

// NewNotifierService создает новый экземпляр сервиса уведомлений
func NewNotifierService(
	notificationRepo repository.NotificationRepository,
	userRepo repository.UserRepository,
	taskRepo repository.TaskRepository,
	projectRepo repository.ProjectRepository,
	telegramRepo repository.TelegramRepository,
	kafkaBrokers []string,
	config *config.NotifierConfig,
	logger logger.Logger,
) *NotifierService {
	// Создаем Kafka reader для чтения уведомлений
	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:         kafkaBrokers,
		Topic:           "notifications",
		GroupID:         "notifier-group",
		MinBytes:        10e3, // 10KB
		MaxBytes:        10e6, // 10MB
		MaxWait:         time.Second,
		CommitInterval:  time.Second,
		ReadLagInterval: -1,
	})

	// Инициализируем отправителя уведомлений Telegram
	telegramSender := NewTelegramSender(config.Telegram.Token, telegramRepo, logger)

	return &NotifierService{
		notificationRepo: notificationRepo,
		userRepo:         userRepo,
		taskRepo:         taskRepo,
		projectRepo:      projectRepo,
		telegramSender:   telegramSender,
		kafkaReader:      kafkaReader,
		logger:           logger,
		config:           config,
	}
}

// Start запускает сервис уведомлений
func (s *NotifierService) Start(ctx context.Context) error {
	s.logger.Info("Starting notifier service")

	// Запускаем чтение сообщений из Kafka
	go s.consumeNotifications(ctx)

	return nil
}

// Stop останавливает сервис уведомлений
func (s *NotifierService) Stop() error {
	s.logger.Info("Stopping notifier service")
	return s.kafkaReader.Close()
}

// consumeNotifications читает и обрабатывает уведомления из Kafka
func (s *NotifierService) consumeNotifications(ctx context.Context) {
	for {
		// Проверяем, не завершен ли контекст
		select {
		case <-ctx.Done():
			s.logger.Info("Notification consumer stopped due to context cancellation")
			return
		default:
			// Продолжаем работу
		}

		// Читаем сообщение из Kafka
		message, err := s.kafkaReader.ReadMessage(ctx)
		if err != nil {
			s.logger.Error("Failed to read message from Kafka", err)
			continue
		}

		// Обрабатываем сообщение
		// s.logger.Info("Received notification event",
		// 	"offset", message.Offset,
		// 	"partition", message.Partition,
		// 	"key", string(message.Key))

		// Обрабатываем уведомление асинхронно
		go func(m kafka.Message) {
			if err := s.processNotificationEvent(ctx, m.Value); err != nil {
				s.logger.Error("Failed to process notification event", err)
			}
		}(message)
	}
}

// processNotificationEvent обрабатывает событие уведомления
func (s *NotifierService) processNotificationEvent(ctx context.Context, data []byte) error {
	// Разбираем событие
	var event messaging.NotificationEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal notification event: %w", err)
	}

	// Обрабатываем уведомление для каждого пользователя
	for _, userID := range event.UserIDs {
		// Получаем настройки уведомлений пользователя
		settings, err := s.notificationRepo.GetUserNotificationSettings(ctx, userID)
		if err != nil {
			s.logger.Error("Failed to get user notification settings", err, map[string]interface{}{
				"user_id": userID,
			})
			continue
		}

		// Определяем тип уведомления и каналы отправки
		notificationType := domain.NotificationType(event.Type)
		var telegramEnabled bool

		// Находим настройку для данного типа уведомлений
		for _, setting := range settings {
			if setting.NotificationType == notificationType {
				telegramEnabled = setting.TelegramEnabled
				break
			}
		}

		// Получаем данные пользователя
		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			s.logger.Error("Failed to get user", err, map[string]interface{}{
				"user_id": userID,
			})
			continue
		}

		// Формируем уведомление
		notification := &domain.Notification{
			ID:         uuid.New().String(),
			UserID:     userID,
			Type:       notificationType,
			Title:      event.Title,
			Content:    event.Content,
			Status:     domain.NotificationStatusUnread,
			EntityID:   event.EntityID,
			EntityType: event.EntityType,
			MetaData:   event.MetaData,
			CreatedAt:  event.CreatedAt,
		}

		// Отправляем Telegram, если включено
		if telegramEnabled {
			if err := s.telegramSender.SendNotification(ctx, user, notification); err != nil {
				s.logger.Error("Failed to send Telegram notification", err, map[string]interface{}{
					"user_id": userID,
				})
			}
		}

		// Добавляем дополнительную информацию к уведомлению, если нужно
		if notification.EntityType == "task" && notification.EntityID != "" {
			// Получаем информацию о задаче
			task, err := s.taskRepo.GetByID(ctx, notification.EntityID)
			if err == nil {
				// Добавляем информацию о задаче в метаданные
				if notification.MetaData == nil {
					notification.MetaData = make(map[string]string)
				}
				notification.MetaData["task_title"] = task.Title
				notification.MetaData["task_status"] = string(task.Status)
				notification.MetaData["project_id"] = task.ProjectID
			}
		} else if notification.EntityType == "project" && notification.EntityID != "" {
			// Получаем информацию о проекте
			project, err := s.projectRepo.GetByID(ctx, notification.EntityID)
			if err == nil {
				// Добавляем информацию о проекте в метаданные
				if notification.MetaData == nil {
					notification.MetaData = make(map[string]string)
				}
				notification.MetaData["project_name"] = project.Name
				notification.MetaData["project_status"] = string(project.Status)
			}
		}

		// Сохраняем уведомление в базе данных (если еще не сохранено)
		if notification.ID == "" {
			if err := s.notificationRepo.Create(ctx, notification); err != nil {
				s.logger.Error("Failed to save notification", err, map[string]interface{}{
					"user_id": userID,
				})
			}
		}
	}

	return nil
}
