package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/yourusername/task-tracker/pkg/logger"
)

// RateLimitStrategy определяет стратегию ограничения запросов
type RateLimitStrategy string

const (
	// RateLimitIP ограничивает запросы по IP-адресу
	RateLimitIP RateLimitStrategy = "ip"
	// RateLimitUser ограничивает запросы по ID пользователя
	RateLimitUser RateLimitStrategy = "user"
	// RateLimitCombined ограничивает запросы по комбинации IP и ID пользователя
	RateLimitCombined RateLimitStrategy = "combined"
)

// RateLimiterConfig содержит настройки для ограничителя запросов
type RateLimiterConfig struct {
	// Максимальное количество запросов в период
	Limit int
	// Период времени для ограничения (в секундах)
	Period int
	// Стратегия ограничения
	Strategy RateLimitStrategy
}

// RateLimiter предоставляет middleware для ограничения частоты запросов
type RateLimiter struct {
	config     RateLimiterConfig
	logger     logger.Logger
	redis      *redis.Client
	inMemLimit map[string]*limitInfo
	mu         sync.Mutex
}

// limitInfo хранит информацию о лимитах для in-memory реализации
type limitInfo struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter создает новый экземпляр RateLimiter
func NewRateLimiter(config RateLimiterConfig, redisClient *redis.Client, logger logger.Logger) *RateLimiter {
	return &RateLimiter{
		config:     config,
		redis:      redisClient,
		logger:     logger,
		inMemLimit: make(map[string]*limitInfo),
	}
}

// Limit применяет ограничение частоты запросов
func (m *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Определяем ключ для ограничения в зависимости от стратегии
		key := m.getKey(r)

		// Проверяем, превышен ли лимит
		remaining, resetTime, limited, err := m.isLimited(r.Context(), key)
		if err != nil {
			m.logger.Error("Rate limiter error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Добавляем информацию о лимитах в заголовки ответа
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(m.config.Limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

		// Если лимит превышен, возвращаем ошибку
		if limited {
			w.Header().Set("Retry-After", strconv.Itoa(int(resetTime.Sub(time.Now()).Seconds())))
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Вызываем следующий обработчик
		next.ServeHTTP(w, r)
	})
}

// getKey формирует ключ для ограничения в зависимости от стратегии
func (m *RateLimiter) getKey(r *http.Request) string {
	var key string
	ip := getClientIP(r)

	switch m.config.Strategy {
	case RateLimitUser:
		// Если пользователь аутентифицирован, используем его ID
		if userID, ok := r.Context().Value("user_id").(string); ok && userID != "" {
			key = fmt.Sprintf("rate_limit:user:%s", userID)
		} else {
			// Если пользователь не аутентифицирован, используем IP
			key = fmt.Sprintf("rate_limit:ip:%s", ip)
		}
	case RateLimitCombined:
		// Комбинируем IP и ID пользователя (если есть)
		if userID, ok := r.Context().Value("user_id").(string); ok && userID != "" {
			key = fmt.Sprintf("rate_limit:combined:%s:%s", ip, userID)
		} else {
			key = fmt.Sprintf("rate_limit:ip:%s", ip)
		}
	default:
		// По умолчанию используем IP
		key = fmt.Sprintf("rate_limit:ip:%s", ip)
	}

	return key
}

// isLimited проверяет, превышен ли лимит для данного ключа
func (m *RateLimiter) isLimited(ctx context.Context, key string) (int, time.Time, bool, error) {
	// Если есть Redis, используем его
	if m.redis != nil {
		return m.isLimitedRedis(ctx, key)
	}
	// Иначе используем in-memory реализацию
	return m.isLimitedInMemory(key)
}

// isLimitedRedis проверяет лимит с использованием Redis
func (m *RateLimiter) isLimitedRedis(ctx context.Context, key string) (int, time.Time, bool, error) {
	now := time.Now()
	windowKey := fmt.Sprintf("%s:%d", key, now.Unix()/int64(m.config.Period))
	
	// Используем транзакцию для атомарного обновления счетчика
	pipe := m.redis.TxPipeline()
	incr := pipe.Incr(ctx, windowKey)
	pipe.Expire(ctx, windowKey, time.Duration(m.config.Period)*time.Second)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, now, false, err
	}

	count, err := incr.Result()
	if err != nil {
		return 0, now, false, err
	}

	resetTime := now.Add(time.Duration(m.config.Period) * time.Second)
	remaining := m.config.Limit - int(count)
	if remaining < 0 {
		remaining = 0
	}

	return remaining, resetTime, count > int64(m.config.Limit), nil
}

// isLimitedInMemory проверяет лимит с использованием in-memory хранилища
func (m *RateLimiter) isLimitedInMemory(key string) (int, time.Time, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	
	// Проверяем существующее ограничение
	info, exists := m.inMemLimit[key]
	if exists {
		// Если время сброса прошло, сбрасываем счетчик
		if now.After(info.resetTime) {
			info.count = 1
			info.resetTime = now.Add(time.Duration(m.config.Period) * time.Second)
		} else {
			// Иначе увеличиваем счетчик
			info.count++
		}
	} else {
		// Создаем новую запись
		info = &limitInfo{
			count:     1,
			resetTime: now.Add(time.Duration(m.config.Period) * time.Second),
		}
		m.inMemLimit[key] = info
	}

	remaining := m.config.Limit - info.count
	if remaining < 0 {
		remaining = 0
	}

	return remaining, info.resetTime, info.count > m.config.Limit, nil
}

// Очистка устаревших записей для in-memory реализации
func (m *RateLimiter) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, info := range m.inMemLimit {
		if now.After(info.resetTime) {
			delete(m.inMemLimit, key)
		}
	}
}

// StartCleanupTask запускает периодическую очистку устаревших записей
func (m *RateLimiter) StartCleanupTask(ctx context.Context) {
	// Очищаем устаревшие записи каждую минуту
	ticker := time.NewTicker(time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.cleanupExpired()
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// getClientIP возвращает IP-адрес клиента
func getClientIP(r *http.Request) string {
	// Проверяем заголовок X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Берем первый IP (самый дальний клиент в цепочке)
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	// Проверяем X-Real-IP
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	
	// Используем RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}