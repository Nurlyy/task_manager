package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/nurlyy/task_manager/pkg/config"
)

// Стандартные ошибки
var (
	ErrInvalidToken   = errors.New("token is invalid")
	ErrExpiredToken   = errors.New("token has expired")
	ErrTokenNotFound  = errors.New("token not found")
	ErrInvalidClaims  = errors.New("invalid token claims")
)

// TokenType определяет тип токена
type TokenType string

const (
	// AccessToken используется для аутентификации запросов API
	AccessToken TokenType = "access"
	// RefreshToken используется для обновления access токена
	RefreshToken TokenType = "refresh"
)

// Claims содержит информацию о пользователе в JWT
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

// JWTManager предоставляет функции для работы с JWT
type JWTManager struct {
	config *config.JWTConfig
}

// NewJWTManager создает новый менеджер JWT токенов
func NewJWTManager(cfg *config.JWTConfig) *JWTManager {
	return &JWTManager{
		config: cfg,
	}
}

// GenerateToken создает новый JWT токен для пользователя
func (m *JWTManager) GenerateToken(userID, email, role string, tokenType TokenType) (string, time.Time, error) {
	var expiration time.Time

	// Определяем срок действия токена
	if tokenType == AccessToken {
		expiration = time.Now().Add(m.config.AccessExpiresIn)
	} else {
		expiration = time.Now().Add(m.config.RefreshExpiresIn)
	}

	// Создаем claims
	claims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		Type:   string(tokenType),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiration),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    m.config.Issuer,
			Subject:   userID,
		},
	}

	// Создаем токен
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Подписываем токен
	tokenString, err := token.SignedString([]byte(m.config.Secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiration, nil
}

// GenerateTokenPair создает пару токенов (access и refresh)
func (m *JWTManager) GenerateTokenPair(userID, email, role string) (accessToken, refreshToken string, err error) {
	// Создаем access токен
	accessToken, _, err = m.GenerateToken(userID, email, role, AccessToken)
	if err != nil {
		return "", "", err
	}

	// Создаем refresh токен
	refreshToken, _, err = m.GenerateToken(userID, email, role, RefreshToken)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// VerifyToken проверяет валидность JWT токена
func (m *JWTManager) VerifyToken(tokenString string) (*Claims, error) {
	// Парсим токен
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			// Проверяем алгоритм подписи
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(m.config.Secret), nil
		},
	)

	if err != nil {
		// Обрабатываем стандартные ошибки
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, ErrInvalidToken
		} else if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, ErrExpiredToken
		} else {
			return nil, fmt.Errorf("failed to parse token: %w", err)
		}
	}

	// Получаем claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}

// RefreshTokens обновляет пару токенов, используя refresh токен
func (m *JWTManager) RefreshTokens(refreshToken string) (accessToken, newRefreshToken string, err error) {
	// Проверяем refresh токен
	claims, err := m.VerifyToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	// Проверяем, что это действительно refresh токен
	if claims.Type != string(RefreshToken) {
		return "", "", ErrInvalidToken
	}

	// Создаем новую пару токенов
	return m.GenerateTokenPair(claims.UserID, claims.Email, claims.Role)
}