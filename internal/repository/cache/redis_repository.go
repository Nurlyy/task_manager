package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/nurlyy/task_manager/internal/domain"
	"github.com/nurlyy/task_manager/pkg/logger"
)

// Префиксы ключей для разных типов данных
const (
	keyPrefixUser           = "user:"
	keyPrefixTask           = "task:"
	keyPrefixProject        = "project:"
	keyPrefixProjectMembers = "project:members:"
	keyPrefixTaskList       = "task:list:"
	keyPrefixProjectList    = "project:list:"
	keyPrefixUserTasks      = "user:tasks:"
	keyPrefixUserProjects   = "user:projects:"
	keyPrefixTaskComments   = "task:comments:"
	keyPrefixNotifications  = "notifications:"
	keyPrefixUnreadCount    = "unread:count:"
	keyPrefixLock           = "lock:"
)

// RedisRepository реализует репозиторий кэширования с использованием Redis
type RedisRepository struct {
	client *redis.Client
	logger logger.Logger
	ttl    time.Duration
}

// NewRedisRepository создает новый экземпляр RedisRepository
func NewRedisRepository(client *redis.Client, logger logger.Logger, ttl time.Duration) *RedisRepository {
	return &RedisRepository{
		client: client,
		logger: logger,
		ttl:    ttl,
	}
}

// CacheUser сохраняет пользователя в кэш
func (r *RedisRepository) CacheUser(ctx context.Context, user *domain.User) error {
	key := fmt.Sprintf("%s%s", keyPrefixUser, user.ID)
	return r.cacheValue(ctx, key, user)
}

// GetUser получает пользователя из кэша
func (r *RedisRepository) GetUser(ctx context.Context, id string) (*domain.User, error) {
	key := fmt.Sprintf("%s%s", keyPrefixUser, id)
	var user domain.User
	if err := r.getValue(ctx, key, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// InvalidateUser удаляет пользователя из кэша
func (r *RedisRepository) InvalidateUser(ctx context.Context, id string) error {
	key := fmt.Sprintf("%s%s", keyPrefixUser, id)
	return r.deleteValue(ctx, key)
}

// CacheTask сохраняет задачу в кэш
func (r *RedisRepository) CacheTask(ctx context.Context, task *domain.Task) error {
	key := fmt.Sprintf("%s%s", keyPrefixTask, task.ID)
	return r.cacheValue(ctx, key, task)
}

// GetTask получает задачу из кэша
func (r *RedisRepository) GetTask(ctx context.Context, id string) (*domain.Task, error) {
	key := fmt.Sprintf("%s%s", keyPrefixTask, id)
	var task domain.Task
	if err := r.getValue(ctx, key, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// InvalidateTask удаляет задачу из кэша
func (r *RedisRepository) InvalidateTask(ctx context.Context, id string) error {
	key := fmt.Sprintf("%s%s", keyPrefixTask, id)
	return r.deleteValue(ctx, key)
}

// CacheProject сохраняет проект в кэш
func (r *RedisRepository) CacheProject(ctx context.Context, project *domain.Project) error {
	key := fmt.Sprintf("%s%s", keyPrefixProject, project.ID)
	return r.cacheValue(ctx, key, project)
}

// GetProject получает проект из кэша
func (r *RedisRepository) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	key := fmt.Sprintf("%s%s", keyPrefixProject, id)
	var project domain.Project
	if err := r.getValue(ctx, key, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// InvalidateProject удаляет проект из кэша
func (r *RedisRepository) InvalidateProject(ctx context.Context, id string) error {
	key := fmt.Sprintf("%s%s", keyPrefixProject, id)
	return r.deleteValue(ctx, key)
}

// CacheProjectMembers сохраняет участников проекта в кэш
func (r *RedisRepository) CacheProjectMembers(ctx context.Context, projectID string, members []*domain.ProjectMember) error {
	key := fmt.Sprintf("%s%s", keyPrefixProjectMembers, projectID)
	return r.cacheValue(ctx, key, members)
}

// GetProjectMembers получает участников проекта из кэша
func (r *RedisRepository) GetProjectMembers(ctx context.Context, projectID string) ([]*domain.ProjectMember, error) {
	key := fmt.Sprintf("%s%s", keyPrefixProjectMembers, projectID)
	var members []*domain.ProjectMember
	if err := r.getValue(ctx, key, &members); err != nil {
		return nil, err
	}
	return members, nil
}

// InvalidateProjectMembers удаляет участников проекта из кэша
func (r *RedisRepository) InvalidateProjectMembers(ctx context.Context, projectID string) error {
	key := fmt.Sprintf("%s%s", keyPrefixProjectMembers, projectID)
	return r.deleteValue(ctx, key)
}

// CacheTaskList сохраняет список задач в кэш
func (r *RedisRepository) CacheTaskList(ctx context.Context, filter string, tasks []*domain.Task) error {
	key := fmt.Sprintf("%s%s", keyPrefixTaskList, filter)
	return r.cacheValue(ctx, key, tasks)
}

// GetTaskList получает список задач из кэша
func (r *RedisRepository) GetTaskList(ctx context.Context, filter string) ([]*domain.Task, error) {
	key := fmt.Sprintf("%s%s", keyPrefixTaskList, filter)
	var tasks []*domain.Task
	if err := r.getValue(ctx, key, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// InvalidateTaskList удаляет список задач из кэша
func (r *RedisRepository) InvalidateTaskList(ctx context.Context, filter string) error {
	key := fmt.Sprintf("%s%s", keyPrefixTaskList, filter)
	return r.deleteValue(ctx, key)
}

// CacheProjectList сохраняет список проектов в кэш
func (r *RedisRepository) CacheProjectList(ctx context.Context, filter string, projects []*domain.Project) error {
	key := fmt.Sprintf("%s%s", keyPrefixProjectList, filter)
	return r.cacheValue(ctx, key, projects)
}

// GetProjectList получает список проектов из кэша
func (r *RedisRepository) GetProjectList(ctx context.Context, filter string) ([]*domain.Project, error) {
	key := fmt.Sprintf("%s%s", keyPrefixProjectList, filter)
	var projects []*domain.Project
	if err := r.getValue(ctx, key, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// InvalidateProjectList удаляет список проектов из кэша
func (r *RedisRepository) InvalidateProjectList(ctx context.Context, filter string) error {
	key := fmt.Sprintf("%s%s", keyPrefixProjectList, filter)
	return r.deleteValue(ctx, key)
}

// CacheNotifications сохраняет уведомления пользователя в кэш
func (r *RedisRepository) CacheNotifications(ctx context.Context, userID string, notifications []*domain.Notification) error {
	key := fmt.Sprintf("%s%s", keyPrefixNotifications, userID)
	return r.cacheValue(ctx, key, notifications)
}

// GetNotifications получает уведомления пользователя из кэша
func (r *RedisRepository) GetNotifications(ctx context.Context, userID string) ([]*domain.Notification, error) {
	key := fmt.Sprintf("%s%s", keyPrefixNotifications, userID)
	var notifications []*domain.Notification
	if err := r.getValue(ctx, key, &notifications); err != nil {
		return nil, err
	}
	return notifications, nil
}

// InvalidateNotifications удаляет уведомления пользователя из кэша
func (r *RedisRepository) InvalidateNotifications(ctx context.Context, userID string) error {
	key := fmt.Sprintf("%s%s", keyPrefixNotifications, userID)
	return r.deleteValue(ctx, key)
}

// CacheUnreadCount сохраняет количество непрочитанных уведомлений пользователя
func (r *RedisRepository) CacheUnreadCount(ctx context.Context, userID string, count int) error {
	key := fmt.Sprintf("%s%s", keyPrefixUnreadCount, userID)
	return r.client.Set(ctx, key, count, r.ttl).Err()
}

// GetUnreadCount получает количество непрочитанных уведомлений пользователя
func (r *RedisRepository) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	key := fmt.Sprintf("%s%s", keyPrefixUnreadCount, userID)
	val, err := r.client.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		r.logger.Error("Failed to get unread count from Redis", err, map[string]interface{}{
			"user_id": userID,
		})
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return val, nil
}

// AcquireLock получает блокировку с таймаутом
func (r *RedisRepository) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("%s%s", keyPrefixLock, key)
	ok, err := r.client.SetNX(ctx, lockKey, 1, ttl).Result()
	if err != nil {
		r.logger.Error("Failed to acquire lock", err, map[string]interface{}{
			"key": key,
		})
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	return ok, nil
}

// ReleaseLock освобождает блокировку
func (r *RedisRepository) ReleaseLock(ctx context.Context, key string) error {
	lockKey := fmt.Sprintf("%s%s", keyPrefixLock, key)
	return r.deleteValue(ctx, lockKey)
}

// InvalidateAll удаляет все данные из кэша для указанного типа
func (r *RedisRepository) InvalidateAll(ctx context.Context, prefix string) error {
	pattern := fmt.Sprintf("%s*", prefix)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		r.logger.Error("Failed to get keys for pattern", err, map[string]interface{}{
			"pattern": pattern,
		})
		return fmt.Errorf("failed to get keys for pattern: %w", err)
	}

	if len(keys) > 0 {
		if err := r.client.Del(ctx, keys...).Err(); err != nil {
			r.logger.Error("Failed to delete keys", err, map[string]interface{}{
				"count": len(keys),
			})
			return fmt.Errorf("failed to delete keys: %w", err)
		}
	}

	return nil
}

// Вспомогательные методы

// cacheValue сохраняет значение в кэш
func (r *RedisRepository) cacheValue(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		r.logger.Error("Failed to marshal value", err, map[string]interface{}{
			"key": key,
		})
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		r.logger.Error("Failed to set value in Redis", err, map[string]interface{}{
			"key": key,
		})
		return fmt.Errorf("failed to set value in Redis: %w", err)
	}

	return nil
}

// getValue получает значение из кэша
func (r *RedisRepository) getValue(ctx context.Context, key string, dest interface{}) error {
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return fmt.Errorf("key not found")
	}
	if err != nil {
		r.logger.Error("Failed to get value from Redis", err, map[string]interface{}{
			"key": key,
		})
		return fmt.Errorf("failed to get value from Redis: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		r.logger.Error("Failed to unmarshal value", err, map[string]interface{}{
			"key": key,
		})
		return fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return nil
}

// deleteValue удаляет значение из кэша
func (r *RedisRepository) deleteValue(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		r.logger.Error("Failed to delete value from Redis", err, map[string]interface{}{
			"key": key,
		})
		return fmt.Errorf("failed to delete value from Redis: %w", err)
	}
	return nil
}

// Delete удаляет значение из кэша по ключу
func (r *RedisRepository) Delete(ctx context.Context, key string) error {
	cmd := r.client.Del(ctx, key)
	if err := cmd.Err(); err != nil && err != redis.Nil {
		r.logger.Error("Failed to delete key from Redis", err, map[string]interface{}{
			"key": key,
		})
		return err
	}
	return nil
}

// Get получает значение из Redis по ключу как строку
func (r *RedisRepository) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Ключ не найден в Redis
			return errors.New("key not found")
		}
		return err // другая ошибка Redis
	}

	// Десериализуем значение из JSON в переданную переменную
	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return fmt.Errorf("failed to unmarshal cached value: %w", err)
	}

	return nil
}

func (r *RedisRepository) Set(ctx context.Context, key string, value interface{}) error {
	// Сериализуем значение в JSON
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value for cache: %w", err)
	}

	// Устанавливаем значение в Redis без TTL (или с дефолтным TTL, если хочешь)
	err = r.client.Set(ctx, key, data, 0).Err() // 0 — без истечения
	if err != nil {
		return fmt.Errorf("failed to set value in cache: %w", err)
	}

	return nil
}

// Set устанавливает значение по ключу с указанным временем жизни
func (r *RedisRepository) SetNew(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// Get получает значение по ключу
func (r *RedisRepository) GetNew(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("key not found: %s", key)
		}
		return "", err
	}
	return val, nil
}

// Delete удаляет значение по ключу
func (r *RedisRepository) DeleteNew(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
