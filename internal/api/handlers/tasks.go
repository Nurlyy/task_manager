package handlers

import (
	"errors"
	"net/http"

	"github.com/yourusername/task-tracker/internal/domain"
	"github.com/yourusername/task-tracker/internal/service"
)

// TaskHandler обрабатывает запросы, связанные с задачами
type TaskHandler struct {
	BaseHandler
	taskService *service.TaskService
}

// NewTaskHandler создает новый экземпляр TaskHandler
func NewTaskHandler(base BaseHandler, taskService *service.TaskService) *TaskHandler {
	return &TaskHandler{
		BaseHandler: base,
		taskService: taskService,
	}
}

// CreateTask обрабатывает запрос на создание новой задачи
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	var req domain.TaskCreateRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse create task request", err)
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

	// Обновляем исполнителя задачи
	task, err := h.taskService.UpdateAssignee(r.Context(), taskID, req.AssigneeID, userID)
	if err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Task not found", "task_not_found")
			return
		}
		if errors.Is(err, service.ErrTaskAccessDenied) {
			h.RespondWithError(w, r, http.StatusForbidden, "Access denied to the task", "access_denied")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Insufficient rights to update task assignee", "insufficient_rights")
			return
		}
		h.Logger.Error("Failed to update task assignee", err, "id", taskID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to update task assignee", "assignee_update_failed")
		return
	}

	h.RespondWithSuccess(w, r, task)
}

// LogTime добавляет запись о затраченном времени на задачу
func (h *TaskHandler) LogTime(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID задачи из URL
	taskID := h.GetURLParam(r, "id")
	if taskID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Task ID is required", "missing_id")
		return
	}

	var req domain.LogTimeRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse log time request", err)
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

	// Логируем затраченное время
	if err := h.taskService.LogTime(r.Context(), taskID, req, userID); err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Task not found", "task_not_found")
			return
		}
		if errors.Is(err, service.ErrTaskAccessDenied) {
			h.RespondWithError(w, r, http.StatusForbidden, "Access denied to the task", "access_denied")
			return
		}
		h.Logger.Error("Failed to log time", err, "task_id", taskID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to log time", "log_time_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]bool{"success": true})
}

// GetTimeLogs возвращает записи о затраченном времени на задачу
func (h *TaskHandler) GetTimeLogs(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID задачи из URL
	taskID := h.GetURLParam(r, "id")
	if taskID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Task ID is required", "missing_id")
		return
	}

	// Получаем записи о затраченном времени
	timeLogs, err := h.taskService.GetTimeLogs(r.Context(), taskID, userID)
	if err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Task not found", "task_not_found")
			return
		}
		if errors.Is(err, service.ErrTaskAccessDenied) {
			h.RespondWithError(w, r, http.StatusForbidden, "Access denied to the task", "access_denied")
			return
		}
		h.Logger.Error("Failed to get time logs", err, "task_id", taskID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get time logs", "time_logs_fetch_failed")
		return
	}

	h.RespondWithSuccess(w, r, timeLogs)
}
	

// GetTask возвращает информацию о задаче по ID
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID задачи из URL
	taskID := h.GetURLParam(r, "id")
	if taskID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Task ID is required", "missing_id")
		return
	}

	// Получаем данные задачи
	task, err := h.taskService.GetByID(r.Context(), taskID, userID)
	if err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Task not found", "task_not_found")
			return
		}
		if errors.Is(err, service.ErrTaskAccessDenied) {
			h.RespondWithError(w, r, http.StatusForbidden, "Access denied to the task", "access_denied")
			return
		}
		h.Logger.Error("Failed to get task", err, "id", taskID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get task info", "task_fetch_failed")
		return
	}

	h.RespondWithSuccess(w, r, task)
}

// UpdateTask обновляет информацию о задаче
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID задачи из URL
	taskID := h.GetURLParam(r, "id")
	if taskID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Task ID is required", "missing_id")
		return
	}

	var req domain.TaskUpdateRequest
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse update task request", err)
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

	// Обновляем данные задачи
	task, err := h.taskService.Update(r.Context(), taskID, req, userID)
	if err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Task not found", "task_not_found")
			return
		}
		if errors.Is(err, service.ErrTaskAccessDenied) {
			h.RespondWithError(w, r, http.StatusForbidden, "Access denied to the task", "access_denied")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Insufficient rights to update task", "insufficient_rights")
			return
		}
		h.Logger.Error("Failed to update task", err, "id", taskID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to update task", "update_failed")
		return
	}

	h.RespondWithSuccess(w, r, task)
}

// DeleteTask удаляет задачу
func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID задачи из URL
	taskID := h.GetURLParam(r, "id")
	if taskID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Task ID is required", "missing_id")
		return
	}

	// Удаляем задачу
	if err := h.taskService.Delete(r.Context(), taskID, userID); err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Task not found", "task_not_found")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Insufficient rights to delete task", "insufficient_rights")
			return
		}
		h.Logger.Error("Failed to delete task", err, "id", taskID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to delete task", "delete_failed")
		return
	}

	h.RespondWithSuccess(w, r, map[string]bool{"success": true})
}

// ListTasks возвращает список задач с фильтрацией
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Параметры пагинации
	page, pageSize := h.GetPaginationParams(r)

	// Создаем фильтр
	filter := domain.TaskFilterOptions{
		Page:     page,
		PageSize: pageSize,
	}

	// Фильтр по проекту
	if projectID := r.URL.Query().Get("project_id"); projectID != "" {
		filter.ProjectID = &projectID
	}

	// Фильтр по статусу
	if status := r.URL.Query().Get("status"); status != "" {
		taskStatus := domain.TaskStatus(status)
		filter.Status = &taskStatus
	}

	// Фильтр по приоритету
	if priority := r.URL.Query().Get("priority"); priority != "" {
		taskPriority := domain.TaskPriority(priority)
		filter.Priority = &taskPriority
	}

	// Фильтр по исполнителю
	if assigneeID := r.URL.Query().Get("assignee_id"); assigneeID != "" {
		filter.AssigneeID = &assigneeID
	}

	// Фильтр только мои задачи
	if r.URL.Query().Get("my_tasks") == "true" {
		filter.AssigneeID = &userID
	}

	// Фильтр задачи, созданные пользователем
	if r.URL.Query().Get("created_by_me") == "true" {
		filter.CreatedBy = &userID
	}

	// Фильтр по поисковому тексту
	if search := r.URL.Query().Get("search"); search != "" {
		filter.SearchText = &search
	}

	// Фильтр по тегам
	if tags := r.URL.Query()["tag"]; len(tags) > 0 {
		filter.Tags = tags
	}

	// Настройка сортировки
	if sortBy := r.URL.Query().Get("sort_by"); sortBy != "" {
		filter.SortBy = &sortBy
		if sortOrder := r.URL.Query().Get("sort_order"); sortOrder != "" {
			filter.SortOrder = &sortOrder
		}
	}

	// Получаем список задач
	result, err := h.taskService.List(r.Context(), filter, userID, page, pageSize)
	if err != nil {
		h.Logger.Error("Failed to list tasks", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get tasks", "tasks_fetch_failed")
		return
	}

	h.RespondWithPagination(w, r, result.Items, result)
}

// UpdateTaskStatus обновляет статус задачи
func (h *TaskHandler) UpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID задачи из URL
	taskID := h.GetURLParam(r, "id")
	if taskID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Task ID is required", "missing_id")
		return
	}

	// Получаем новый статус из запроса
	var req struct {
		Status domain.TaskStatus `json:"status" validate:"required,oneof=new in_progress on_hold review completed cancelled"`
	}
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse update status request", err)
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

	// Обновляем статус задачи
	task, err := h.taskService.UpdateStatus(r.Context(), taskID, req.Status, userID)
	if err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Task not found", "task_not_found")
			return
		}
		if errors.Is(err, service.ErrTaskAccessDenied) {
			h.RespondWithError(w, r, http.StatusForbidden, "Access denied to the task", "access_denied")
			return
		}
		if errors.Is(err, service.ErrInsufficientRights) {
			h.RespondWithError(w, r, http.StatusForbidden, "Insufficient rights to update task status", "insufficient_rights")
			return
		}
		if errors.Is(err, service.ErrInvalidTaskStatus) {
			h.RespondWithError(w, r, http.StatusBadRequest, "Invalid status transition", "invalid_status")
			return
		}
		h.Logger.Error("Failed to update task status", err, "id", taskID)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to update task status", "status_update_failed")
		return
	}

	h.RespondWithSuccess(w, r, task)
}

// UpdateTaskAssignee обновляет исполнителя задачи
func (h *TaskHandler) UpdateTaskAssignee(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, err := h.GetUserIDFromContext(r)
	if err != nil {
		h.RespondWithError(w, r, http.StatusUnauthorized, "Unauthorized", "unauthorized")
		return
	}

	// Получаем ID задачи из URL
	taskID := h.GetURLParam(r, "id")
	if taskID == "" {
		h.RespondWithError(w, r, http.StatusBadRequest, "Task ID is required", "missing_id")
		return
	}

	// Получаем нового исполнителя из запроса
	var req struct {
		AssigneeID *string `json:"assignee_id" validate:"omitempty,uuid"`
	}
	if err := h.ParseJSON(r, &req); err != nil {
		h.Logger.Error("Failed to parse update assignee request", err)
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

	// Создаем задачу
	task, err := h.taskService.Create(r.Context(), req, userID)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			h.RespondWithError(w, r, http.StatusNotFound, "Project not found", "project_not_found")
			return
		}
		h.Logger.Error("Failed to create task", err)
		h.RespondWithError(w, r, http.StatusInternalServerError, "Failed to create task", "creation_failed")
		return
	}

	h.RespondWithSuccess(w, r, task)
}