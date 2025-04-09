package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/internal/repository/postgres"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// TelegramSender обеспечивает отправку уведомлений в Telegram
type TelegramSender struct {
	botToken     string
	apiBaseURL   string
	client       *http.Client
	logger       logger.Logger
	telegramRepo postgres.TelegramRepository
}

// TelegramResponse представляет ответ от Telegram API
type TelegramResponse struct {
	Ok          bool            `json:"ok"`
	Description string          `json:"description,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// NewTelegramSender создает новый экземпляр TelegramSender
func NewTelegramSender(
	botToken string,
	telegramRepo repository.TelegramRepository,
	logger logger.Logger,
) *TelegramSender {
	// Создаем HTTP клиент с таймаутом
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	return &TelegramSender{
		botToken:     botToken,
		apiBaseURL:   "https://api.telegram.org/bot",
		client:       client,
		logger:       logger,
		telegramRepo: telegramRepo,
	}
}

// SendNotification отправляет уведомление в Telegram
func (s *TelegramSender) SendNotification(ctx context.Context, user *domain.User, notification *domain.Notification) error {
	// Получаем Telegram ID пользователя из репозитория
	telegramLink, err := s.telegramRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get Telegram link: %w", err)
	}

	// Формируем сообщение в зависимости от типа уведомления
	message := s.formatMessage(notification, user)

	// Отправляем сообщение
	if err := s.sendMessage(telegramLink.ChatID, message); err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}

	return nil
}

// getTelegramID получает Telegram ID пользователя из его данных
func (s *TelegramSender) getTelegramID(user *domain.User) (string, bool) {
	// Поскольку в модели User нет поля MetaData, можно:

	// 1. Получать telegram ID из таблицы user_notification_settings
	// Например, если у пользователя включены Telegram-уведомления, значит там должен быть его ID

	// 2. Либо можно хранить telegram ID в отдельной таблице, связанной с пользователем

	// 3. В качестве временного решения можно использовать какое-то существующее поле
	// Например, если вы храните Telegram ID в поле Position (это не рекомендуется для продакшена)
	if user.Position != nil && *user.Position != "" && strings.HasPrefix(*user.Position, "telegram:") {
		return strings.TrimPrefix(*user.Position, "telegram:"), true
	}

	// В реальном проекте нужно реализовать получение Telegram ID
	// из соответствующего хранилища

	// Для тестирования можно возвращать тестовый ID
	// return "123456789", true

	// Если не находим ID, возвращаем ошибку
	return "", false
}

// formatMessage форматирует сообщение в зависимости от типа уведомления
func (s *TelegramSender) formatMessage(notification *domain.Notification, user *domain.User) string {
	// Базовое сообщение
	message := fmt.Sprintf("*%s*\n\n%s\n",
		escapeMarkdown(notification.Title),
		escapeMarkdown(notification.Content))

	// Добавляем дополнительную информацию в зависимости от типа уведомления
	if notification.MetaData != nil {
		switch notification.Type {
		case domain.NotificationTypeTaskAssigned:
			if taskTitle, ok := notification.MetaData["task_title"]; ok {
				message += fmt.Sprintf("\n*Задача:* %s", escapeMarkdown(taskTitle))
			}
			if priority, ok := notification.MetaData["priority"]; ok {
				message += fmt.Sprintf("\n*Приоритет:* %s", escapeMarkdown(priority))
			}
			if dueDate, ok := notification.MetaData["due_date"]; ok {
				message += fmt.Sprintf("\n*Срок выполнения:* %s", escapeMarkdown(dueDate))
			}

		case domain.NotificationTypeTaskUpdated:
			if taskTitle, ok := notification.MetaData["task_title"]; ok {
				message += fmt.Sprintf("\n*Задача:* %s", escapeMarkdown(taskTitle))
			}
			if status, ok := notification.MetaData["status"]; ok {
				message += fmt.Sprintf("\n*Статус:* %s", escapeMarkdown(status))
			}
			if assigneeName, ok := notification.MetaData["assignee_name"]; ok {
				message += fmt.Sprintf("\n*Исполнитель:* %s", escapeMarkdown(assigneeName))
			}

		case domain.NotificationTypeTaskCommented:
			if taskTitle, ok := notification.MetaData["task_title"]; ok {
				message += fmt.Sprintf("\n*Задача:* %s", escapeMarkdown(taskTitle))
			}
			if userName, ok := notification.MetaData["user_name"]; ok {
				message += fmt.Sprintf("\n*Автор комментария:* %s", escapeMarkdown(userName))
			}
			if commentContent, ok := notification.MetaData["comment_content"]; ok {
				message += fmt.Sprintf("\n*Комментарий:* %s", escapeMarkdown(commentContent))
			}

		case domain.NotificationTypeTaskDueSoon:
			if taskTitle, ok := notification.MetaData["task_title"]; ok {
				message += fmt.Sprintf("\n*Задача:* %s", escapeMarkdown(taskTitle))
			}
			if dueDate, ok := notification.MetaData["due_date"]; ok {
				message += fmt.Sprintf("\n*Срок выполнения:* %s", escapeMarkdown(dueDate))
			}
			if hoursLeft, ok := notification.MetaData["hours_left"]; ok {
				message += fmt.Sprintf("\n*Осталось времени:* %s часов", escapeMarkdown(hoursLeft))
			}

		case domain.NotificationTypeTaskOverdue:
			if taskTitle, ok := notification.MetaData["task_title"]; ok {
				message += fmt.Sprintf("\n*Задача:* %s", escapeMarkdown(taskTitle))
			}
			if dueDate, ok := notification.MetaData["due_date"]; ok {
				message += fmt.Sprintf("\n*Срок выполнения истек:* %s", escapeMarkdown(dueDate))
			}

		case domain.NotificationTypeProjectMemberAdded:
			if projectName, ok := notification.MetaData["project_name"]; ok {
				message += fmt.Sprintf("\n*Проект:* %s", escapeMarkdown(projectName))
			}
			if role, ok := notification.MetaData["role"]; ok {
				message += fmt.Sprintf("\n*Роль:* %s", escapeMarkdown(role))
			}

		case domain.NotificationTypeProjectUpdated:
			if projectName, ok := notification.MetaData["project_name"]; ok {
				message += fmt.Sprintf("\n*Проект:* %s", escapeMarkdown(projectName))
			}
			if status, ok := notification.MetaData["status"]; ok {
				message += fmt.Sprintf("\n*Статус:* %s", escapeMarkdown(status))
			}
		}
	}

	// Добавляем дату/время
	message += fmt.Sprintf("\n\n_Отправлено: %s_", notification.CreatedAt.Format("02.01.2006 15:04"))

	return message
}

// sendMessage отправляет сообщение в Telegram
func (s *TelegramSender) sendMessage(telegramID, message string) error {
	// Формируем URL для отправки сообщения
	apiURL := fmt.Sprintf("%s%s/sendMessage", s.apiBaseURL, s.botToken)

	// Формируем данные запроса
	data := url.Values{}
	data.Set("chat_id", telegramID)
	data.Set("text", message)
	data.Set("parse_mode", "Markdown")

	// Отправляем POST-запрос
	resp, err := s.client.Post(apiURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("post request failed: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned non-OK status: %s", resp.Status)
	}

	// Разбираем ответ
	var telegramResp TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем успешность операции
	if !telegramResp.Ok {
		return fmt.Errorf("telegram API returned error: %s", telegramResp.Description)
	}

	return nil
}

// escapeMarkdown экранирует специальные символы Markdown
func escapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(text)
}

// RegisterUserTelegram регистрирует ассоциацию пользователя с Telegram
func (s *TelegramSender) RegisterUserTelegram(userID, telegramID string) error {
	// Здесь должна быть логика для сохранения ассоциации пользователя с Telegram ID
	// Например, обновление в базе данных или через API

	// Заглушка - просто логируем действие
	s.logger.Info("Registering Telegram for user", "user_id", userID, "telegram_id", telegramID)

	return nil
}
