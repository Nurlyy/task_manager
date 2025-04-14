package handlers

import (
	"errors"
	"net/http"

	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/service"
)

// CommentHandler обрабатывает запросы, связанные с комментариями
type CommentHandler struct {
	BaseHandler
	commentService *service.CommentService
}

// NewCommentHandler создает новый экземпляр CommentHandler
func NewCommentHandler(base BaseHandler, commentService *service.CommentService) *CommentHandler {
	return &CommentHandler{
		BaseHandler:    base,
		commentService: commentService,
	}
}

// CreateComment обрабатывает запрос на создание нового комментария
func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID задачи из URL
	taskID := h.GetURLParam(r, "task_id")
	if taskID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Task ID is required", "missing_task_id")
		return
	}

	var req domain.CommentCreateRequest
	req.TaskID = taskID

	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse create comment request", err)
		h.RespondWithError(w, r, http.StatusBadRequest, "Invalid request format", "invalid_format")
		return
	}

	// Валидация запроса
	if validationErrors, err := h.ValidateRequest(req); err != nil {
		h.Logger.Error("Request validation error", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Validation failed", "validation_error")
		return
	} else if len(validationErrors) > 0 {
		h.RespondWithValidationErrors(w, r, validationErrors)
		return
	}

	// Создаем комментарий
	comment, err := h.commentService.Create(r.Context(), req, userID)
	if err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Task not found", "task_not_found")
			return
		}
		if errors.Is(err, service.ErrTaskAccessDenied) {
			h.RespondWithError(w, r, http.StatusForbidden, "Access denied to the task", "access_denied")
			return
		}
		h.Logger.Error("Failed to create comment", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to create comment", "creation_failed")
		return
	}

	h.RespondWithSuccess(w, r, comment)
}

// GetComment возвращает информацию о комментарии по ID
func (h *CommentHandler) GetComment(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID комментария из URL
	commentID := h.GetURLParam(r, "id")
	if commentID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Comment ID is required", "missing_id")
		return
	}

	// Получаем данные комментария
	comment, err := h.commentService.GetByID(r.Context(), commentID, userID)
	if err != nil {
		if errors.Is(err, service.ErrCommentNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Comment not found", "comment_not_found")
			return
		}
		if errors.Is(err, service.ErrCommentAccessDenied) {
			h.RespondWithError(w, r, http.StatusForbidden, "Access denied to the comment", "access_denied")
			return
		}
		h.Logger.Error("Failed to get comment", err, map[string]interface{}{
			"id": commentID,
		})
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get comment info", "comment_fetch_failed")
		return
	}

	h.RespondWithSuccess(w, r, comment)
}

// UpdateComment обновляет информацию о комментарии
func (h *CommentHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID комментария из URL
	commentID := h.GetURLParam(r, "id")
	if commentID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Comment ID is required", "missing_id")
		return
	}

	var req domain.CommentUpdateRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse update comment request", err)
		h.RespondWithError(w, r, http.StatusBadRequest, "Invalid request format", "invalid_format")
		return
	}

	// Валидация запроса
	if validationErrors, err := h.ValidateRequest(req); err != nil {
		h.Logger.Error("Request validation error", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Validation failed", "validation_error")
		return
	} else if len(validationErrors) > 0 {
		h.RespondWithValidationErrors(w, r, validationErrors)
		return
	}

	// Обновляем данные комментария
	comment, err := h.commentService.Update(r.Context(), commentID, req, userID)
	if err != nil {
		if errors.Is(err, service.ErrCommentNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Comment not found", "comment_not_found")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Only comment author can update comment", "insufficient_rights")
			return
		}
		h.Logger.Error("Failed to update comment", err, map[string]interface{}{
			"id": commentID,
		})
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to update comment", "update_failed")
		return
	}

	h.RespondWithSuccess(w, r, comment)
}

// DeleteComment удаляет комментарий
func (h *CommentHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID комментария из URL
	commentID := h.GetURLParam(r, "id")
	if commentID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Comment ID is required", "missing_id")
		return
	}

	// Удаляем комментарий
	if err := h.commentService.Delete(r.Context(), commentID, userID); err != nil {
		if errors.Is(err, service.ErrCommentNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Comment not found", "comment_not_found")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Only comment author can delete comment", "insufficient_rights")
			return
		}
		h.Logger.Error("Failed to delete comment", err, map[string]interface{}{
			"id": commentID,
		})
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to delete comment", "delete_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]bool{"success": true})
}

// GetCommentsByTask возвращает комментарии к задаче
func (h *CommentHandler) GetCommentsByTask(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID задачи из URL
	taskID := h.GetURLParam(r, "task_id")
	if taskID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Task ID is required", "missing_task_id")
		return
	}

	// Параметры пагинации
	page, pageSize := h.GetPaginationParams(r)

	// Получаем комментарии к задаче
	result, err := h.commentService.GetCommentsByTask(r.Context(), taskID, userID, page, pageSize)
	if err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Task not found", "task_not_found")
			return
		}
		if errors.Is(err, service.ErrTaskAccessDenied) {
			h.RespondWithError(w, r, http.StatusForbidden, "Access denied to the task", "access_denied")
			return
		}
		h.Logger.Error("Failed to get comments by task", err, map[string]interface{}{
			"task_id": taskID,
		})
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get comments", "comments_fetch_failed")
		return
	}

	h.RespondWithPagination(w, r, result.Items, result)
}
