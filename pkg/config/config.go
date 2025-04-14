package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config содержит все конфигурационные параметры приложения
type Config struct {
	App        AppConfig
	HTTP       HTTPConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	Kafka      KafkaConfig
	JWT        JWTConfig
	Scheduler  SchedulerConfig
	Notifier   NotifierConfig
	Monitoring MonitoringConfig
}

// AppConfig содержит общие настройки приложения
type AppConfig struct {
	Name        string
	Context     context.Context
	Environment string
	LogLevel    string
	Debug       bool
}

// HTTPConfig содержит настройки HTTP-сервера
type HTTPConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	BasePath        string
}

// DatabaseConfig содержит настройки подключения к базе данных
type DatabaseConfig struct {
	Host         string
	Port         string
	Username     string
	Password     string
	Database     string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  time.Duration
}

// RedisConfig содержит настройки подключения к Redis
type RedisConfig struct {
	Host       string
	Port       string
	Password   string
	DB         int
	DefaultTTL time.Duration
}

// KafkaConfig содержит настройки для работы с Kafka
type KafkaConfig struct {
	Brokers []string
	Topics  KafkaTopics
}

// KafkaTopics содержит названия топиков Kafka
type KafkaTopics struct {
	TaskCreated   string
	TaskUpdated   string
	TaskAssigned  string
	TaskCommented string
	Notifications string
}

// JWTConfig содержит настройки JWT-аутентификации
type JWTConfig struct {
	Secret           string
	AccessExpiresIn  time.Duration
	RefreshExpiresIn time.Duration
	Issuer           string
}

// SchedulerConfig содержит настройки для планировщика задач
type SchedulerConfig struct {
	DailyDigestCron      string
	DeadlineReminderCron string
}

// NotifierConfig содержит настройки для сервиса уведомлений
type NotifierConfig struct {
	SMTP     SMTPConfig
	Telegram TelegramConfig
}

// SMTPConfig содержит настройки SMTP-сервера для отправки email
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// TelegramConfig содержит настройки для уведомлений через Telegram
type TelegramConfig struct {
	Token string
}

// MonitoringConfig содержит настройки мониторинга
type MonitoringConfig struct {
	PrometheusEnabled bool
	PrometheusPort    string
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	// Загружаем .env файл, если он существует
	_ = godotenv.Load()

	config := &Config{
		App: AppConfig{
			Name:        getEnv("APP_NAME", "task-tracker"),
			Environment: getEnv("APP_ENV", "development"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
			Debug:       getEnvAsBool("APP_DEBUG", true),
		},
		HTTP: HTTPConfig{
			Port:            getEnv("HTTP_PORT", "8080"),
			ReadTimeout:     getEnvAsDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    getEnvAsDuration("HTTP_WRITE_TIMEOUT", 20*time.Second),
			ShutdownTimeout: getEnvAsDuration("HTTP_SHUTDOWN_TIMEOUT", 5*time.Second),
			BasePath:        getEnv("HTTP_BASE_PATH", ""),
		},
		Database: DatabaseConfig{
			Host:         getEnv("DB_HOST", "localhost"),
			Port:         getEnv("DB_PORT", "5432"),
			Username:     getEnv("DB_USER", "taskuser"),
			Password:     getEnv("DB_PASSWORD", "taskpass"),
			Database:     getEnv("DB_NAME", "tasktracker"),
			SSLMode:      getEnv("DB_SSLMODE", "disable"),
			MaxOpenConns: getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLife:  getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Host:       getEnv("REDIS_HOST", "localhost"),
			Port:       getEnv("REDIS_PORT", "6379"),
			Password:   getEnv("REDIS_PASSWORD", ""),
			DB:         getEnvAsInt("REDIS_DB", 0),
			DefaultTTL: getEnvAsDuration("REDIS_DEFAULT_TTL", 24*time.Hour),
		},
		Kafka: KafkaConfig{
			Brokers: strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
			Topics: KafkaTopics{
				TaskCreated:   getEnv("KAFKA_TOPIC_TASK_CREATED", "task_created"),
				TaskUpdated:   getEnv("KAFKA_TOPIC_TASK_UPDATED", "task_updated"),
				TaskAssigned:  getEnv("KAFKA_TOPIC_TASK_ASSIGNED", "task_assigned"),
				TaskCommented: getEnv("KAFKA_TOPIC_TASK_COMMENTED", "task_commented"),
				Notifications: getEnv("KAFKA_TOPIC_NOTIFICATIONS", "notifications"),
			},
		},
		JWT: JWTConfig{
			Secret:           getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			AccessExpiresIn:  getEnvAsDuration("JWT_ACCESS_EXPIRES_IN", 15*time.Minute),
			RefreshExpiresIn: getEnvAsDuration("JWT_REFRESH_EXPIRES_IN", 7*24*time.Hour),
			Issuer:           getEnv("JWT_ISSUER", "task-tracker"),
		},
		Scheduler: SchedulerConfig{
			DailyDigestCron:      getEnv("SCHEDULER_DAILY_DIGEST_CRON", "0 8 * * *"),
			DeadlineReminderCron: getEnv("SCHEDULER_DEADLINE_REMINDER_CRON", "0 9 * * *"),
		},
		Notifier: NotifierConfig{
			SMTP: SMTPConfig{
				Host:     getEnv("SMTP_HOST", "localhost"),
				Port:     getEnv("SMTP_PORT", "1025"),
				Username: getEnv("SMTP_USER", ""),
				Password: getEnv("SMTP_PASSWORD", ""),
				From:     getEnv("SMTP_FROM", "noreply@tasktracker.com"),
			},
			Telegram: TelegramConfig{
				Token: getEnv("TELEGRAM_TOKEN", ""),
			},
		},
		Monitoring: MonitoringConfig{
			PrometheusEnabled: getEnvAsBool("PROMETHEUS_ENABLED", false),
			PrometheusPort:    getEnv("PROMETHEUS_PORT", "9090"),
		},
	}

	return config, nil
}

// DSN возвращает строку подключения к PostgreSQL
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.Username, c.Password, c.Database, c.SSLMode)
}

// RedisAddr возвращает адрес подключения к Redis
func (c *RedisConfig) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// Утилитарные функции для получения переменных окружения

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := getEnv(key, "")
	if value, err := time.ParseDuration(valueStr); err == nil {
		return value
	}
	return defaultValue
}
