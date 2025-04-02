package handlers

import (
	"errors"
	"net/http"

	"github.com/yourusername/task-tracker/internal/domain"
	"github.com/yourusername/task-tracker/internal/service"
)

// NotificationHandler обрабатывает запросы, связанные с уведомлениями
type NotificationHandler struct {
	BaseHandler
	notificationService *service.NotificationService
}

// NewNotificationHandler создает новый экземпляр NotificationHandler
func NewNotificationHandler(base BaseHandler, notificationService *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		BaseHandler:         base,
		notificationService: notificationService,
	}
}

// GetNotification возвращает информацию об уведомлении по ID
func (h *NotificationHandler) GetNotification(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID уведомления из URL
	notificationID := h.GetURLParam(r, "id")
	if notificationID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Notification ID is required", "missing_id")
		return
	}

	// Получаем данные уведомления
	notification, err := h.notificationService.GetByID(r.Context(), notificationID, userID)
	if err != nil {
		if errors.Is(err, service.ErrNotificationNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Notification not found", "notification_not_found")
			return
		}
		h.Logger.Error("Failed to get notification", err, "id", notificationID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get notification info", "notification_fetch_failed")
		return
	}

	h.RespondWithSuccess(w, r, notification)
}

// MarkAsRead отмечает уведомление как прочитанное
func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID уведомления из URL
	notificationID := h.GetURLParam(r, "id")
	if notificationID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Notification ID is required", "missing_id")
		return
	}

	// Отмечаем уведомление как прочитанное
	if err := h.notificationService.MarkAsRead(r.Context(), notificationID, userID); err != nil {
		if errors.Is(err, service.ErrNotificationNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Notification not found", "notification_not_found")
			return
		}
		h.Logger.Error("Failed to mark notification as read", err, "id", notificationID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to mark notification as read", "mark_read_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]bool{"success": true})
}

// MarkAllAsRead отмечает все уведомления пользователя как прочитанные
func (h *NotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Отмечаем все уведомления как прочитанные
	if err := h.notificationService.MarkAllAsRead(r.Context(), userID); err != nil {
		h.Logger.Error("Failed to mark all notifications as read", err, "user_id", userID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to mark all notifications as read", "mark_all_read_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]bool{"success": true})
}

// DeleteNotification удаляет уведомление
func (h *NotificationHandler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID уведомления из URL
	notificationID := h.GetURLParam(r, "id")
	if notificationID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Notification ID is required", "missing_id")
		return
	}

	// Удаляем уведомление
	if err := h.notificationService.Delete(r.Context(), notificationID, userID); err != nil {
		if errors.Is(err, service.ErrNotificationNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Notification not found", "notification_not_found")
			return
		}
		h.Logger.Error("Failed to delete notification", err, "id", notificationID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to delete notification", "delete_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]bool{"success": true})
}

// ListNotifications возвращает список уведомлений пользователя
func (h *NotificationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Параметры пагинации
	page, pageSize := h.GetPaginationParams(r)

	// Создаем фильтр
	filter := domain.NotificationFilterOptions{
		UserID: &userID,
	}

	// Фильтр по статусу
	if status := r.URL.Query().Get("status"); status != "" {
		notificationStatus := domain.NotificationStatus(status)
		filter.Status = &notificationStatus
	}

	// Фильтр по типу
	if typeName := r.URL.Query().Get("type"); typeName != "" {
		notificationType := domain.NotificationType(typeName)
		filter.Type = &notificationType
	}

	// Фильтр по ID сущности
	if entityID := r.URL.Query().Get("entity_id"); entityID != "" {
		filter.EntityID = &entityID
	}

	// Фильтр по типу сущности
	if entityType := r.URL.Query().Get("entity_type"); entityType != "" {
		filter.EntityType = &entityType
	}

	// Получаем список уведомлений
	result, err := h.notificationService.GetUserNotifications(r.Context(), userID, filter, page, pageSize)
	if err != nil {
		h.Logger.Error("Failed to list notifications", err, "user_id", userID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get notifications", "notifications_fetch_failed")
		return
	}

	h.RespondWithPagination(w, r, result.Items, result)
}

// GetUnreadCount возвращает количество непрочитанных уведомлений
func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем количество непрочитанных уведомлений
	count, err := h.notificationService.GetUnreadCount(r.Context(), userID)
	if err != nil {
		h.Logger.Error("Failed to get unread notifications count", err, "user_id", userID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get unread count", "unread_count_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]int{"count": count})
}

// GetNotificationSettings возвращает настройки уведомлений пользователя
func (h *NotificationHandler) GetNotificationSettings(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем настройки уведомлений
	settings, err := h.notificationService.GetUserNotificationSettings(r.Context(), userID)
	if err != nil {
		h.Logger.Error("Failed to get notification settings", err, "user_id", userID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get notification settings", "settings_fetch_failed")
		return
	}

	h.RespondWithSuccess(w, r, settings)
}

// UpdateNotificationSettings обновляет настройки уведомлений пользователя
func (h *NotificationHandler) UpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	var settings []*repository.NotificationSetting
	if err := h.ParseJSON(r, &settings); err != nil {
		h.Logger.Error("Failed to parse notification settings request", err)
		h.RespondWithError(w, r, http.StatusBadRequest, "Invalid request format", "invalid_format")
		return
	}

	// Проверка, что все настройки принадлежат текущему пользователю
	for _, setting := range settings {
		if setting.UserID != userID {
			h.RespondWithError(w, r, http.StatusBadRequest, "All settings must belong to the current user", "invalid_user_id")
			return
		}
	}

	// Обновляем настройки уведомлений
	if err := h.notificationService.UpdateUserNotificationSettings(r.Context(), userID, settings); err != nil {
		h.Logger.Error("Failed to update notification settings", err, "user_id", userID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to update notification settings", "settings_update_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]bool{"success": true})
}