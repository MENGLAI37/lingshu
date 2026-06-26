package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/lingshu/ops-ai/pkg/config"
	"github.com/lingshu/ops-ai/pkg/logger"
)

type Database struct {
	Primary   *sqlx.DB
	Fallback  *sqlx.DB
	useFallback bool
}

var (
	instance *Database
)

func Init(cfg *config.DBConfig) (*Database, error) {
	var primary *sqlx.DB
	var fallback *sqlx.DB
	var err error

	primary, err = connectPostgres(cfg)
	if err != nil {
		logger.Warn("Failed to connect to PostgreSQL, falling back to SQLite", "error", err)
		fallback, err = connectSQLite()
		if err != nil {
			return nil, fmt.Errorf("failed to connect to both PostgreSQL and SQLite: %w", err)
		}
		instance = &Database{
			Fallback:    fallback,
			useFallback: true,
		}
		return instance, nil
	}

	fallback, _ = connectSQLite()

	instance = &Database{
		Primary:   primary,
		Fallback:  fallback,
	}

	return instance, nil
}

func connectPostgres(cfg *config.DBConfig) (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	logger.Info("PostgreSQL connected successfully",
		"host", cfg.Host,
		"port", cfg.Port,
		"dbname", cfg.DBName,
	)

	return db, nil
}

func connectSQLite() (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite3", "opsai_fallback.db")
	if err != nil {
		return nil, fmt.Errorf("sqlite connect: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	logger.Info("SQLite fallback connected")
	return db, nil
}

func Get() *Database {
	return instance
}

func (d *Database) DB() *sqlx.DB {
	if d.useFallback && d.Fallback != nil {
		return d.Fallback
	}
	if d.Primary != nil {
		return d.Primary
	}
	return d.Fallback
}

func (d *Database) IsFallback() bool {
	return d.useFallback
}

func (d *Database) PingContext(ctx context.Context) error {
	if d.Primary != nil {
		return d.Primary.PingContext(ctx)
	}
	if d.Fallback != nil {
		return d.Fallback.PingContext(ctx)
	}
	return sql.ErrConnDone
}

func (d *Database) Close() error {
	var firstErr error
	if d.Primary != nil {
		if err := d.Primary.Close(); err != nil {
			firstErr = err
		}
	}
	if d.Fallback != nil {
		if err := d.Fallback.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (d *Database) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return d.DB().SelectContext(ctx, dest, query, args...)
}

func (d *Database) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return d.DB().GetContext(ctx, dest, query, args...)
}

func (d *Database) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.DB().ExecContext(ctx, query, args...)
}

func (d *Database) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	return d.DB().NamedExecContext(ctx, query, arg)
}

func (d *Database) QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	return d.DB().QueryRowxContext(ctx, query, args...)
}
