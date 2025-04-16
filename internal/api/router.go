package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/nurlyy/task_manager/internal/api/handlers"
	mw "github.com/nurlyy/task_manager/internal/api/middleware"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/internal/service"
	"github.com/nurlyy/task_manager/pkg/auth"
	"github.com/nurlyy/task_manager/pkg/config"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// Server представляет HTTP сервер API
type Server struct {
	router       chi.Router
	logger       logger.Logger
	config       *config.Config
	jwtManager   *auth.JWTManager
	baseHandler  handlers.BaseHandler
	services     *Services
	repositories *Repositories
}

// Services содержит все сервисы для обработчиков API
type Services struct {
	UserService         *service.UserService
	ProjectService      *service.ProjectService
	TaskService         *service.TaskService
	CommentService      *service.CommentService
	NotificationService *service.NotificationService
	TelegramService     *service.TelegramSender
}

type Repositories struct {
	TelegramRepository repository.TelegramRepository
}

// NewServer создает новый экземпляр сервера API
func NewServer(config *config.Config, logger logger.Logger, jwtManager *auth.JWTManager, services *Services, repositories *Repositories) *Server {
	baseHandler := handlers.NewBaseHandler(logger, jwtManager)

	server := &Server{
		router:       chi.NewRouter(),
		logger:       logger,
		config:       config,
		jwtManager:   jwtManager,
		baseHandler:  baseHandler,
		services:     services,
		repositories: repositories,
	}

	// Настраиваем маршрутизацию
	server.setupRoutes()

	return server
}

// setupRoutes настраивает маршруты API
func (s *Server) setupRoutes() {
	// Инициализируем обработчики
	authHandler := handlers.NewAuthHandler(s.baseHandler, s.services.UserService)
	userHandler := handlers.NewUserHandler(s.baseHandler, s.services.UserService)
	projectHandler := handlers.NewProjectHandler(s.baseHandler, s.services.ProjectService)
	taskHandler := handlers.NewTaskHandler(s.baseHandler, s.services.TaskService)
	commentHandler := handlers.NewCommentHandler(s.baseHandler, s.services.CommentService)
	notificationHandler := handlers.NewNotificationHandler(s.baseHandler, s.services.NotificationService)

	telegramHandler := handlers.NewTelegramHandler(
		s.baseHandler,
		s.repositories.TelegramRepository,
		s.services.TelegramService,
		s.services.UserService,
	)

	// Инициализируем middleware
	authMiddleware := mw.NewAuthMiddleware(s.jwtManager, s.logger)
	loggingMiddleware := mw.NewLoggingMiddleware(s.logger)

	// Настраиваем Rate Limiter с параметрами из конфигурации
	rateLimiter := mw.NewRateLimiter(mw.RateLimiterConfig{
		Limit:    100,            // Ограничение запросов
		Period:   60,             // Период в секундах
		Strategy: mw.RateLimitIP, // Стратегия по IP
	}, nil, s.logger) // nil - без Redis, используем in-memory

	// Запускаем задачу очистки для Rate Limiter
	go rateLimiter.StartCleanupTask(s.config.App.Context)

	// Настраиваем middleware для всех запросов
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(loggingMiddleware.LogRequest)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))
	s.router.Use(rateLimiter.Limit)

	// Настраиваем CORS
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // Разрешаем все источники
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Максимальное время кеширования CORS preflight запросов
	}))

	// Базовый маршрут для проверки работоспособности API
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"OK"}`))
	})

	// API v1
	s.router.Route("/api/v1", func(r chi.Router) {
		// Публичные маршруты (без аутентификации)
		r.Group(func(r chi.Router) {
			r.Post("/auth/register", authHandler.Register)
			r.Post("/auth/login", authHandler.Login)
			r.Post("/auth/refresh", authHandler.RefreshToken)
			// r.Post("/webhook/telegram", telegramHandler.WebhookHandler)
		})

		// Защищенные маршруты (требуют аутентификации)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)

			// Маршруты для текущего пользователя
			r.Get("/auth/me", authHandler.GetCurrentUser)
			r.Post("/auth/change-password", authHandler.ChangePassword)

			// Маршруты для пользователей
			r.Route("/users", func(r chi.Router) {
				r.Get("/{id}", userHandler.GetUser)
				r.Put("/{id}", userHandler.UpdateUser)
				r.Delete("/{id}", userHandler.DeleteUser)
				r.Get("/", userHandler.ListUsers)
			})

			// Маршруты для проектов
			r.Route("/projects", func(r chi.Router) {
				r.Post("/", projectHandler.CreateProject)
				r.Get("/{id}", projectHandler.GetProject)
				r.Put("/{id}", projectHandler.UpdateProject)
				r.Delete("/{id}", projectHandler.DeleteProject)
				r.Get("/", projectHandler.ListProjects)
				r.Get("/{id}/metrics", projectHandler.GetProjectMetrics)

				// Маршруты для участников проекта
				r.Post("/{id}/members", projectHandler.AddProjectMember)
				r.Put("/{id}/members/{member_id}", projectHandler.UpdateProjectMember)
				r.Delete("/{id}/members/{member_id}", projectHandler.RemoveProjectMember)
			})

			// Маршруты для задач
			r.Route("/tasks", func(r chi.Router) {
				r.Post("/", taskHandler.CreateTask)
				r.Get("/{id}", taskHandler.GetTask)
				r.Put("/{id}", taskHandler.UpdateTask)
				r.Delete("/{id}", taskHandler.DeleteTask)
				r.Get("/", taskHandler.ListTasks)
				r.Put("/{id}/status", taskHandler.UpdateTaskStatus)
				r.Put("/{id}/assignee", taskHandler.UpdateTaskAssignee)
				r.Post("/{id}/time", taskHandler.LogTime)
				r.Get("/{id}/time", taskHandler.GetTimeLogs)
			})

			// Маршруты для комментариев
			r.Route("/comments", func(r chi.Router) {
				r.Get("/{id}", commentHandler.GetComment)
				r.Put("/{id}", commentHandler.UpdateComment)
				r.Delete("/{id}", commentHandler.DeleteComment)
			})

			// Комментарии к задаче
			r.Route("/tasks/{task_id}/comments", func(r chi.Router) {
				r.Post("/", commentHandler.CreateComment)
				r.Get("/", commentHandler.GetCommentsByTask)
			})

			// Маршруты для уведомлений
			r.Route("/notifications", func(r chi.Router) {
				r.Get("/", notificationHandler.ListNotifications)
				r.Get("/count", notificationHandler.GetUnreadCount)
				r.Get("/{id}", notificationHandler.GetNotification)
				r.Put("/{id}/read", notificationHandler.MarkAsRead)
				r.Put("/read-all", notificationHandler.MarkAllAsRead)
				r.Delete("/{id}", notificationHandler.DeleteNotification)
				r.Get("/settings", notificationHandler.GetNotificationSettings)
				r.Put("/settings", notificationHandler.UpdateNotificationSettings)
			})

			// Маршруты для Telegram
			r.Route("/telegram", func(r chi.Router) {
				r.Get("/status", telegramHandler.GetTelegramStatus)
				r.Post("/connect", telegramHandler.GenerateConnectToken)
				r.Delete("/disconnect", telegramHandler.DisconnectTelegram)
			})
		})
	})
}

// ServeHTTP реализует интерфейс http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Start запускает HTTP сервер
func (s *Server) Start() error {
	s.logger.Info("Starting API server", map[string]interface{}{
		"port": s.config.HTTP.Port,
	})

	server := &http.Server{
		Addr:         ":" + s.config.HTTP.Port,
		Handler:      s.router,
		ReadTimeout:  s.config.HTTP.ReadTimeout,
		WriteTimeout: s.config.HTTP.WriteTimeout,
	}

	return server.ListenAndServe()
}

// Shutdown корректно останавливает HTTP сервер
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down API server")

	// Здесь можно добавить дополнительную логику для корректного завершения работы
	// Например, ожидание завершения текущих запросов и т.д.

	return nil
}
