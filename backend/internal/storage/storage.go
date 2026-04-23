package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

type Store struct {
	DB    *sql.DB
	Redis *redis.Client
}

type DeveloperAPIKeyRow struct {
	ID        string
	Name      string
	Key       string
	CreatedAt time.Time
}

type DeveloperWebhookRow struct {
	ID        string
	URL       string
	Event     string
	CreatedAt time.Time
}

type Config struct {
	MySQLDSN      string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
}

func New(ctx context.Context, cfg Config) (*Store, error) {
	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Store{
		DB:    db,
		Redis: rdb,
	}, nil
}

func (s *Store) ListDeveloperAPIKeys(limit int) []DeveloperAPIKeyRow {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.DB.Query(`
SELECT id, name, api_key, created_at
FROM developer_api_key
ORDER BY created_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]DeveloperAPIKeyRow, 0, limit)
	for rows.Next() {
		var item DeveloperAPIKeyRow
		if err := rows.Scan(&item.ID, &item.Name, &item.Key, &item.CreatedAt); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *Store) CreateDeveloperAPIKey(name string) (DeveloperAPIKeyRow, error) {
	id := fmt.Sprintf("key_%d", time.Now().UnixNano())
	key := fmt.Sprintf("ak_live_%d", time.Now().UnixNano())
	if _, err := s.DB.Exec(`
INSERT INTO developer_api_key (api_key_id, name, api_key, created_at)
VALUES (?, ?, ?, UTC_TIMESTAMP())`, id, name, key); err != nil {
		return DeveloperAPIKeyRow{}, err
	}
	return DeveloperAPIKeyRow{
		ID:        id,
		Name:      name,
		Key:       key,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (s *Store) ListDeveloperWebhooks(limit int) []DeveloperWebhookRow {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.DB.Query(`
SELECT id, url, event, created_at
FROM developer_webhook
ORDER BY created_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]DeveloperWebhookRow, 0, limit)
	for rows.Next() {
		var item DeveloperWebhookRow
		if err := rows.Scan(&item.ID, &item.URL, &item.Event, &item.CreatedAt); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *Store) CreateDeveloperWebhook(url string, event string) (DeveloperWebhookRow, error) {
	id := fmt.Sprintf("wh_%d", time.Now().UnixNano())
	if _, err := s.DB.Exec(`
INSERT INTO developer_webhook (id, url, event, created_at)
VALUES (?, ?, ?, UTC_TIMESTAMP())`, id, url, event); err != nil {
		return DeveloperWebhookRow{}, err
	}
	return DeveloperWebhookRow{
		ID:        id,
		URL:       url,
		Event:     event,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (s *Store) Close() error {
	var errs []error
	if s.Redis != nil {
		if err := s.Redis.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.DB != nil {
		if err := s.DB.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
