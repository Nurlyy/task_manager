package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// TelegramSender обеспечивает отправку уведомлений в Telegram
type TelegramSender struct {
	botToken     string
	apiBaseURL   string
	client       *http.Client
	logger       logger.Logger
	telegramRepo repository.TelegramRepository
	botUsername  string
}

// TelegramResponse представляет ответ от Telegram API
type TelegramResponse struct {
	Ok          bool            `json:"ok"`
	Description string          `json:"description,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// TelegramUser представляет информацию о пользователе Telegram
type TelegramUser struct {
	ID        int    `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
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

	sender := &TelegramSender{
		botToken:     botToken,
		apiBaseURL:   "https://api.telegram.org/bot",
		client:       client,
		logger:       logger,
		telegramRepo: telegramRepo,
	}

	// Получаем информацию о боте
	sender.fetchBotInfo()

	return sender
}

// fetchBotInfo получает информацию о боте из Telegram API
func (s *TelegramSender) fetchBotInfo() {
	apiURL := fmt.Sprintf("%s%s/getme", s.apiBaseURL, s.botToken)
	s.logger.Info("api url: ")
	s.logger.Info(apiURL)
	resp, err := s.client.Get(apiURL)
	if err != nil {
		s.logger.Error("Failed to get bot info", err)
		return
	}
	defer resp.Body.Close()

	var telegramResp TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		s.logger.Error("Failed to decode bot info response", err)
		return
	}

	if !telegramResp.Ok {
		s.logger.Error("Telegram API returned error", fmt.Errorf(telegramResp.Description))
		return
	}

	var botInfo TelegramUser
	if err := json.Unmarshal(telegramResp.Result, &botInfo); err != nil {
		s.logger.Error("Failed to unmarshal bot info", err)
		return
	}

	s.botUsername = botInfo.Username
	s.logger.Info("Bot info fetched successfully", map[string]interface{}{
		"username": botInfo.Username,
	})
}

// SetupWebhook настраивает webhook для Telegram бота
func (s *TelegramSender) SetupWebhook(webhookURL string) error {
	apiURL := fmt.Sprintf("%s%s/setWebhook", s.apiBaseURL, s.botToken)

	// Формируем данные запроса
	data := url.Values{}
	data.Set("url", webhookURL)
	data.Set("allowed_updates", `["message"]`)

	// Отправляем POST-запрос
	resp, err := s.client.Post(apiURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("post request failed: %w", err)
	}
	defer resp.Body.Close()

	// // Проверяем статус ответа
	// if resp.StatusCode != http.StatusOK {
	// 	body, _ := ioutil.ReadAll(resp.Body)
	// 	return fmt.Errorf("telegram API returned non-OK status: %s, body: %s", resp.Status, string(body))
	// }

	// // Разбираем ответ
	// var telegramResp TelegramResponse
	// if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
	// 	return fmt.Errorf("failed to decode response: %w", err)
	// }

	// // Проверяем успешность операции
	// if !telegramResp.Ok {
	// 	return fmt.Errorf("telegram API returned error: %s", telegramResp.Description)
	// }

	// s.logger.Info("Webhook setup successfully", map[string]interface{}{
	// 	"webhook_url": webhookURL,
	// })

	return nil
}

// GetBotUsername возвращает имя пользователя бота
func (s *TelegramSender) GetBotUsername() string {
	return s.botUsername
}

// SendNotification отправляет уведомление в Telegram
func (s *TelegramSender) SendNotification(ctx context.Context, user *domain.User, notification *domain.Notification) error {
	// Получаем Telegram ID пользователя из репозитория
	telegramLink, err := s.telegramRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get Telegram link: %w", err)
	}

	// Проверяем, не nil ли telegramLink
	if telegramLink == nil {
		return fmt.Errorf("user %s has no telegram link", user.ID)
	}

	// Формируем сообщение в зависимости от типа уведомления
	message := s.formatMessage(notification, user)

	// Отправляем сообщение
	if err := s.SendMessage(telegramLink.ChatID, message); err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}

	return nil
}

// SendMessage отправляет сообщение в Telegram
func (s *TelegramSender) SendMessage(telegramID, message string) error {
	// Логируем начало отправки
	s.logger.Info("Starting to send Telegram message", map[string]interface{}{
		"chat_id":        telegramID,
		"message_length": len(message),
	})

	// Формируем URL для отправки сообщения
	apiURL := fmt.Sprintf("%s%s/sendMessage", s.apiBaseURL, s.botToken)
	s.logger.Info("Prepared API URL", map[string]interface{}{
		"api_url": apiURL,
	})

	// Формируем данные запроса
	data := url.Values{}
	data.Set("chat_id", telegramID)
	data.Set("text", message)
	data.Set("parse_mode", "Markdown")

	s.logger.Info("Prepared request data", map[string]interface{}{
		"chat_id":     telegramID,
		"parse_mode":  "Markdown",
		"data_length": len(data.Encode()),
	})

	// Отправляем POST-запрос
	s.logger.Info("Sending POST request to Telegram API")
	resp, err := s.client.Post(apiURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		s.logger.Error("POST request failed", err, map[string]interface{}{
			"chat_id": telegramID,
		})
		return fmt.Errorf("post request failed: %w", err)
	}
	defer resp.Body.Close()

	s.logger.Info("Received response from Telegram API", map[string]interface{}{
		"status_code": resp.StatusCode,
		"status":      resp.Status,
	})

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		body, readErr := ioutil.ReadAll(resp.Body)
		if readErr != nil {
			s.logger.Error("Failed to read response body", readErr)
		}
		s.logger.Error("Telegram API returned non-OK status", fmt.Errorf(resp.Status), map[string]interface{}{
			"response_body": string(body),
		})
		return fmt.Errorf("telegram API returned non-OK status: %s, body: %s", resp.Status, string(body))
	}

	// Разбираем ответ
	s.logger.Info("Decoding response body")
	var telegramResp TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		s.logger.Error("Failed to decode response", err)
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем успешность операции
	if !telegramResp.Ok {
		s.logger.Error("Telegram API returned error in response", fmt.Errorf(telegramResp.Description))
		return fmt.Errorf("telegram API returned error: %s", telegramResp.Description)
	}

	s.logger.Info("Message sent successfully to Telegram", map[string]interface{}{
		"chat_id": telegramID,
	})
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
	s.logger.Info("Registering Telegram for user", map[string]interface{}{
		"user_id": userID,
	}, map[string]interface{}{
		"telegram_id": telegramID,
	})

	return nil
}
