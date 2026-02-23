package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"rfguard/internal/config"
	"rfguard/internal/model"
)

type Store interface {
	Init(ctx context.Context) error
	Close() error
	SaveAlert(ctx context.Context, alert model.Alert) error
	SaveMetrics(ctx context.Context, readerID string, metrics []model.WindowMetrics) error
}

func NewStore(cfg config.StorageConfig) (Store, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	switch strings.ToLower(cfg.Driver) {
	case "sqlite":
		return NewSQLite(cfg.DSN)
	case "postgres", "postgresql":
		return NewPostgres(cfg.DSN)
	default:
		return nil, errors.New("unsupported storage driver")
	}
}

type baseStore struct {
	db *sql.DB
}

func (b *baseStore) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

func encodeJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
