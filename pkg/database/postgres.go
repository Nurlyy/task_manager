package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // драйвер PostgreSQL

	"github.com/yourusername/task-tracker/pkg/config"
	"github.com/yourusername/task-tracker/pkg/logger"
)

// Postgres представляет клиент для работы с PostgreSQL
type Postgres struct {
	DB     *sqlx.DB
	Config *config.DatabaseConfig
	Logger logger.Logger
}

// NewPostgres создает новое подключение к PostgreSQL
func NewPostgres(ctx context.Context, cfg *config.DatabaseConfig, log logger.Logger) (*Postgres, error) {
	log.Info("Connecting to PostgreSQL", map[string]interface{}{
		"host": cfg.Host,
		"port": cfg.Port,
		"user": cfg.Username,
		"db":   cfg.Database,
	})

	db, err := sqlx.ConnectContext(ctx, "postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Настройка пула соединений
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLife)

	// Проверяем соединение
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	log.Info("Successfully connected to PostgreSQL")

	return &Postgres{
		DB:     db,
		Config: cfg,
		Logger: log,
	}, nil
}

// Close закрывает соединение с базой данных
func (p *Postgres) Close() error {
	p.Logger.Info("Closing PostgreSQL connection")
	return p.DB.Close()
}

// Ping проверяет соединение с базой данных
func (p *Postgres) Ping(ctx context.Context) error {
	start := time.Now()
	err := p.DB.PingContext(ctx)
	elapsed := time.Since(start)

	if err != nil {
		p.Logger.Error("Failed to ping PostgreSQL", err, map[string]interface{}{
			"elapsed": elapsed.String(),
		})
		return fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	p.Logger.Debug("PostgreSQL ping successful", map[string]interface{}{
		"elapsed": elapsed.String(),
	})
	return nil
}

// ExecTx выполняет функцию внутри транзакции
func (p *Postgres) ExecTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := p.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				p.Logger.Error("Transaction rollback failed", rbErr)
			}
			return
		}
		err = tx.Commit()
	}()

	err = fn(tx)
	return err
}