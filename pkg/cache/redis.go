package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/yourusername/task-tracker/pkg/config"
	"github.com/yourusername/task-tracker/pkg/logger"
)

// Redis представляет клиент для работы с Redis
type Redis struct {
	Client  *redis.Client
	Config  *config.RedisConfig
	Logger  logger.Logger
}

// NewRedis создает новое подключение к Redis
func NewRedis(ctx context.Context, cfg *config.RedisConfig, log logger.Logger) (*Redis, error) {
	log.Info("Connecting to Redis", map[string]interface{}{
		"host": cfg.Host,
		"port": cfg.Port,
		"db":   cfg.DB,
	})

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Проверяем соединение
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	log.Info("Successfully connected to Redis")

	return &Redis{
		Client: client,
		Config: cfg,
		Logger: log,
	}, nil
}

// Close закрывает соединение с Redis
func (r *Redis) Close() error {
	r.Logger.Info("Closing Redis connection")
	return r.Client.Close()
}

// Set сохраняет значение в кэше
func (r *Redis) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = r.Config.DefaultTTL
	}

	err := r.Client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		r.Logger.Error("Failed to set Redis key", err, map[string]interface{}{
			"key": key,
		})
		return fmt.Errorf("failed to set Redis key %s: %w", key, err)
	}

	r.Logger.Debug("Redis key set successfully", map[string]interface{}{
		"key": key,
		"ttl": ttl.String(),
	})
	return nil
}

// Get получает значение из кэша
func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	value, err := r.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		r.Logger.Debug("Redis key not found", map[string]interface{}{
			"key": key,
		})
		return "", nil
	} else if err != nil {
		r.Logger.Error("Failed to get Redis key", err, map[string]interface{}{
			"key": key,
		})
		return "", fmt.Errorf("failed to get Redis key %s: %w", key, err)
	}

	r.Logger.Debug("Redis key retrieved successfully", map[string]interface{}{
		"key": key,
	})
	return value, nil
}

// Delete удаляет значение из кэша
func (r *Redis) Delete(ctx context.Context, key string) error {
	err := r.Client.Del(ctx, key).Err()
	if err != nil {
		r.Logger.Error("Failed to delete Redis key", err, map[string]interface{}{
			"key": key,
		})
		return fmt.Errorf("failed to delete Redis key %s: %w", key, err)
	}

	r.Logger.Debug("Redis key deleted successfully", map[string]interface{}{
		"key": key,
	})
	return nil
}

// GetLock получает блокировку с таймаутом
func (r *Redis) GetLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := r.Client.SetNX(ctx, key, 1, ttl).Result()
	if err != nil {
		r.Logger.Error("Failed to acquire Redis lock", err, map[string]interface{}{
			"key": key,
			"ttl": ttl.String(),
		})
		return false, fmt.Errorf("failed to acquire Redis lock %s: %w", key, err)
	}

	if ok {
		r.Logger.Debug("Redis lock acquired successfully", map[string]interface{}{
			"key": key,
			"ttl": ttl.String(),
		})
	} else {
		r.Logger.Debug("Redis lock already acquired", map[string]interface{}{
			"key": key,
		})
	}

	return ok, nil
}

// ReleaseLock освобождает блокировку
func (r *Redis) ReleaseLock(ctx context.Context, key string) error {
	return r.Delete(ctx, key)
}