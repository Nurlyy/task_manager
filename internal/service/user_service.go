package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/internal/repository"
	"github.com/nurlyy/task_manager/pkg/auth"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// Стандартные ошибки
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidPassword    = errors.New("invalid password")
)

// UserService представляет бизнес-логику для работы с пользователями
type UserService struct {
	repo   repository.UserRepository
	jwtManager *auth.JWTManager
	logger logger.Logger
	cacheRepo *repository.CacheRepository
}

// NewUserService создает новый экземпляр UserService
func NewUserService(repo repository.UserRepository, jwtManager *auth.JWTManager, 
	cacheRepo *repository.CacheRepository, logger logger.Logger) *UserService {
	return &UserService{
		repo:   repo,
		jwtManager: jwtManager,
		cacheRepo: cacheRepo,
		logger: logger,
	}
}

// Create создает нового пользователя
func (s *UserService) Create(ctx context.Context, req domain.UserCreateRequest) (*domain.UserResponse, error) {
	// Проверяем, существует ли пользователь с таким email
	existingUser, err := s.repo.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, ErrEmailAlreadyExists
	}

	// Хешируем пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("Failed to hash password", err)
		return nil, err
	}

	// Создаем нового пользователя
	now := time.Now()
	user := &domain.User{
		ID:             uuid.New().String(),
		Email:          req.Email,
		HashedPassword: string(hashedPassword),
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Role:           req.Role,
		Position:       req.Position,
		Department:     req.Department,
		Avatar:         req.Avatar,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Сохраняем пользователя в БД
	if err := s.repo.Create(ctx, user); err != nil {
		s.logger.Error("Failed to create user", err)
		return nil, err
	}

	// Возвращаем представление пользователя
	return &user.ToResponse(), nil
}

// GetByID возвращает пользователя по ID
func (s *UserService) GetByID(ctx context.Context, id string) (*domain.UserResponse, error) {
	// Пытаемся получить из кэша
	cacheKey := "user:" + id
	var userResp domain.UserResponse
	if err := s.cacheRepo.Get(ctx, cacheKey, &userResp); err == nil {
		return &userResp, nil
	}

	// Получаем пользователя из БД
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get user by ID", err, "id", id)
		return nil, ErrUserNotFound
	}

	// Сохраняем в кэш
	userResp = user.ToResponse()
	if err := s.cacheRepo.Set(ctx, cacheKey, userResp); err != nil {
		s.logger.Warn("Failed to cache user", "id", id, "error", err)
	}

	return &userResp, nil
}

// GetByEmail возвращает пользователя по email
func (s *UserService) GetByEmail(ctx context.Context, email string) (*domain.UserResponse, error) {
	// Получаем пользователя из БД
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		s.logger.Error("Failed to get user by email", err, "email", email)
		return nil, ErrUserNotFound
	}

	return &user.ToResponse(), nil
}

// Update обновляет данные пользователя
func (s *UserService) Update(ctx context.Context, id string, req domain.UserUpdateRequest) (*domain.UserResponse, error) {
	// Получаем пользователя из БД
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get user by ID for update", err, "id", id)
		return nil, ErrUserNotFound
	}

	// Обновляем поля, которые были переданы
	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		user.LastName = *req.LastName
	}
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.Position != nil {
		user.Position = req.Position
	}
	if req.Department != nil {
		user.Department = req.Department
	}
	if req.Avatar != nil {
		user.Avatar = req.Avatar
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	user.UpdatedAt = time.Now()

	// Сохраняем изменения в БД
	if err := s.repo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to update user", err, "id", id)
		return nil, err
	}

	// Удаляем пользователя из кэша
	cacheKey := "user:" + id
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete user from cache", "id", id, "error", err)
	}

	return &user.ToResponse(), nil
}

// Delete удаляет пользователя
func (s *UserService) Delete(ctx context.Context, id string) error {
	// Проверяем, существует ли пользователь
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		s.logger.Error("Failed to get user by ID for delete", err, "id", id)
		return ErrUserNotFound
	}

	// Удаляем пользователя из БД
	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete user", err, "id", id)
		return err
	}

	// Удаляем пользователя из кэша
	cacheKey := "user:" + id
	if err := s.cacheRepo.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to delete user from cache", "id", id, "error", err)
	}

	return nil
}

// List возвращает список пользователей с фильтрацией
func (s *UserService) List(ctx context.Context, filter repository.UserFilter, page, pageSize int) (*domain.PagedResponse, error) {
	// Настраиваем пагинацию
	filter.Limit = pageSize
	filter.Offset = (page - 1) * pageSize

	// Получаем список пользователей
	users, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list users", err)
		return nil, err
	}

	// Получаем общее количество пользователей
	total, err := s.repo.Count(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to count users", err)
		return nil, err
	}

	// Преобразуем к UserResponse
	userResponses := make([]domain.UserResponse, len(users))
	for i, user := range users {
		userResponses[i] = user.ToResponse()
	}

	// Формируем ответ с пагинацией
	return &domain.PagedResponse{
		Items:      userResponses,
		TotalItems: total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}, nil
}

// Login выполняет вход пользователя
func (s *UserService) Login(ctx context.Context, req domain.LoginRequest) (*domain.LoginResponse, error) {
	// Получаем пользователя по email
	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		s.logger.Error("User not found during login", err, "email", req.Email)
		return nil, ErrInvalidCredentials
	}

	// Проверяем, активен ли пользователь
	if !user.IsActive {
		s.logger.Warn("Inactive user attempted to login", "email", req.Email)
		return nil, ErrInvalidCredentials
	}

	// Проверяем пароль
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password)); err != nil {
		s.logger.Warn("Invalid password during login", "email", req.Email)
		return nil, ErrInvalidCredentials
	}

	// Создаем JWT токены
	accessToken, refreshToken, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email, string(user.Role))
	if err != nil {
		s.logger.Error("Failed to generate tokens", err, "user_id", user.ID)
		return nil, err
	}

	// Обновляем время последнего входа
	if err := s.repo.UpdateLastLogin(ctx, user.ID); err != nil {
		s.logger.Warn("Failed to update last login time", "user_id", user.ID, "error", err)
	}

	// Получаем дату истечения токена
	_, expiresAt, err := s.jwtManager.GenerateToken(user.ID, user.Email, string(user.Role), auth.AccessToken)
	if err != nil {
		s.logger.Error("Failed to get token expiration", err, "user_id", user.ID)
		return nil, err
	}

	// Формируем ответ
	return &domain.LoginResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// RefreshToken обновляет пару токенов
func (s *UserService) RefreshToken(ctx context.Context, req domain.RefreshTokenRequest) (*domain.LoginResponse, error) {
	// Обновляем токены
	accessToken, refreshToken, err := s.jwtManager.RefreshTokens(req.RefreshToken)
	if err != nil {
		s.logger.Error("Failed to refresh tokens", err)
		return nil, err
	}

	// Получаем данные из токена
	claims, err := s.jwtManager.VerifyToken(accessToken)
	if err != nil {
		s.logger.Error("Failed to verify access token", err)
		return nil, err
	}

	// Получаем пользователя
	user, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		s.logger.Error("User not found during token refresh", err, "user_id", claims.UserID)
		return nil, ErrUserNotFound
	}

	// Получаем дату истечения токена
	_, expiresAt, err := s.jwtManager.GenerateToken(user.ID, user.Email, string(user.Role), auth.AccessToken)
	if err != nil {
		s.logger.Error("Failed to get token expiration", err, "user_id", user.ID)
		return nil, err
	}

	// Формируем ответ
	return &domain.LoginResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// ChangePassword изменяет пароль пользователя
func (s *UserService) ChangePassword(ctx context.Context, userID string, req domain.ChangePasswordRequest) error {
	// Получаем пользователя
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("User not found during password change", err, "user_id", userID)
		return ErrUserNotFound
	}

	// Проверяем старый пароль
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.OldPassword)); err != nil {
		s.logger.Warn("Invalid old password during password change", "user_id", userID)
		return ErrInvalidPassword
	}

	// Хешируем новый пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("Failed to hash new password", err)
		return err
	}

	// Обновляем пароль
	user.HashedPassword = string(hashedPassword)
	user.UpdatedAt = time.Now()

	// Сохраняем изменения в БД
	if err := s.repo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to update user with new password", err, "user_id", userID)
		return err
	}

	return nil
}