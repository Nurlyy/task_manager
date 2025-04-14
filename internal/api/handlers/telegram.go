package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/internal/service"
)

// TelegramHandler обрабатывает запросы связанные с Telegram
type TelegramHandler struct {
	baseHandler     BaseHandler
	telegramRepo    repository.TelegramRepository
	telegramService *service.TelegramSender
	userService     *service.UserService
}

// NewTelegramHandler создает новый обработчик для Telegram
func NewTelegramHandler(
	baseHandler BaseHandler,
	telegramRepo repository.TelegramRepository,
	telegramService *service.TelegramSender,
	userService *service.UserService,
) *TelegramHandler {
	return &TelegramHandler{
		baseHandler:     baseHandler,
		telegramRepo:    telegramRepo,
		telegramService: telegramService,
		userService:     userService,
	}
}

// TelegramUpdate представляет обновление от Telegram API
type TelegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  struct {
		MessageID int `json:"message_id"`
		From      struct {
			ID        int    `json:"id"`
			IsBot     bool   `json:"is_bot"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name,omitempty"`
			Username  string `json:"username,omitempty"`
		} `json:"from"`
		Chat struct {
			ID        int    `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name,omitempty"`
			Username  string `json:"username,omitempty"`
			Type      string `json:"type"`
		} `json:"chat"`
		Date int    `json:"date"`
		Text string `json:"text"`
	} `json:"message,omitempty"`
}

// WebhookHandler обрабатывает webhook запросы от Telegram
func (h *TelegramHandler) WebhookHandler(w http.ResponseWriter, r *http.Request) {
	// Читаем тело запроса
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.baseHandler.HandleError(w, r, err, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// // Логируем полученный запрос
	// h.baseHandler.logger.Info("Received Telegram webhook", map[string]interface{}{
	// 	"body": string(body),
	// })

	// Разбираем полученные данные
	var update TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		h.baseHandler.HandleError(w, r, err, http.StatusBadRequest)
		return
	}

	// Проверяем, что получили сообщение
	if update.Message.Text == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Обрабатываем команды
	ctx := r.Context()
	text := update.Message.Text
	chatID := fmt.Sprintf("%d", update.Message.Chat.ID)
	telegramID := fmt.Sprintf("%d", update.Message.From.ID)
	username := update.Message.From.Username
	firstName := update.Message.From.FirstName
	lastName := update.Message.From.LastName

	// Обрабатываем команду /start
	if text == "/start" {
		// Отправляем приветственное сообщение
		welcomeMsg := "Добро пожаловать в Task Manager! Для связи вашего аккаунта с Telegram, введите команду:\n\n/connect YOUR_TOKEN\n\nгде YOUR_TOKEN - это токен, полученный в веб-интерфейсе приложения."
		h.telegramService.SendMessage(chatID, welcomeMsg)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Обрабатываем команду /connect TOKEN
	if len(text) > 9 && text[:9] == "/connect " {
		token := text[9:]
		// Получаем пользователя по токену
		userID, err := h.userService.GetUserIDByToken(ctx, token)
		if err != nil {
			h.telegramService.SendMessage(chatID, "Невалидный токен. Пожалуйста, проверьте токен и попробуйте снова.")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Создаем или обновляем связь пользователя с Telegram
		link := &repository.TelegramLink{
			UserID:     userID,
			TelegramID: telegramID,
			ChatID:     chatID,
			Username:   username,
			FirstName:  firstName,
			LastName:   lastName,
			CreatedAt:  time.Now().Format(time.RFC3339),
			UpdatedAt:  time.Now().Format(time.RFC3339),
		}

		if err := h.telegramRepo.CreateOrUpdate(ctx, link); err != nil {
			// h.baseHandler.Error("Failed to create or update Telegram link", err)
			h.telegramService.SendMessage(chatID, "Произошла ошибка при связывании аккаунта. Пожалуйста, попробуйте позже.")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Отправляем сообщение об успешной привязке
		successMsg := "Ваш аккаунт успешно связан с Telegram! Теперь вы будете получать уведомления о задачах и проектах."
		h.telegramService.SendMessage(chatID, successMsg)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Обрабатываем другие команды
	h.telegramService.SendMessage(chatID, "Неизвестная команда. Используйте /start для получения информации.")
	w.WriteHeader(http.StatusOK)
}

// GenerateConnectToken генерирует токен для связывания аккаунта с Telegram
func (h *TelegramHandler) GenerateConnectToken(w http.ResponseWriter, r *http.Request) {
	// Получаем текущего пользователя из контекста
	user, err := h.baseHandler.GetCurrentUser(r)
	if err != nil {
		h.baseHandler.HandleError(w, r, err, http.StatusUnauthorized)
		return
	}

	// Генерируем токен
	token, err := h.userService.GenerateTelegramToken(r.Context(), user.ID)
	if err != nil {
		h.baseHandler.HandleError(w, r, err, http.StatusInternalServerError)
		return
	}

	// Возвращаем токен
	h.baseHandler.RespondJSON(w, http.StatusOK, map[string]string{
		"token":        token,
		"bot_username": h.telegramService.GetBotUsername(),
		"connection_instruction": fmt.Sprintf("Отправьте боту @%s команду: /connect %s",
			h.telegramService.GetBotUsername(), token),
	})
}

// GetTelegramStatus проверяет, связан ли аккаунт пользователя с Telegram
func (h *TelegramHandler) GetTelegramStatus(w http.ResponseWriter, r *http.Request) {
	// Получаем текущего пользователя из контекста
	user, err := h.baseHandler.GetCurrentUser(r)
	if err != nil {
		h.baseHandler.HandleError(w, r, err, http.StatusUnauthorized)
		return
	}

	// Проверяем наличие связи с Telegram
	link, err := h.telegramRepo.GetByUserID(r.Context(), user.ID)
	if err != nil {
		// Если пользователь не связан с Telegram
		h.baseHandler.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"connected": false,
		})
		return
	}

	// Возвращаем информацию о связи
	h.baseHandler.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"connected":    true,
		"username":     link.Username,
		"connected_at": link.CreatedAt,
	})
}

// DisconnectTelegram отвязывает Telegram от аккаунта пользователя
func (h *TelegramHandler) DisconnectTelegram(w http.ResponseWriter, r *http.Request) {
	// Получаем текущего пользователя из контекста
	user, err := h.baseHandler.GetCurrentUser(r)
	if err != nil {
		h.baseHandler.HandleError(w, r, err, http.StatusUnauthorized)
		return
	}

	// Удаляем связь с Telegram
	if err := h.telegramRepo.Delete(r.Context(), user.ID); err != nil {
		h.baseHandler.HandleError(w, r, err, http.StatusInternalServerError)
		return
	}

	// Возвращаем успешный ответ
	h.baseHandler.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Ваш аккаунт успешно отвязан от Telegram",
	})
}
