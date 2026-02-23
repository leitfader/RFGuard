package storage

import (
	"context"
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"

	"rfguard/internal/model"
)

type sqliteStore struct {
	baseStore
}

func NewSQLite(dsn string) (Store, error) {
	if strings.TrimSpace(dsn) == "" {
		dsn = "file:rfguard.db?_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	return &sqliteStore{baseStore{db: db}}, nil
}

func (s *sqliteStore) Init(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts TEXT NOT NULL,
			reader_id TEXT NOT NULL,
			severity TEXT NOT NULL,
			alert_type TEXT NOT NULL,
			window_sec INTEGER NOT NULL,
			score REAL NOT NULL,
			rules_json TEXT NOT NULL,
			metrics_json TEXT NOT NULL,
			context_json TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_ts ON alerts(ts)`,
		`CREATE TABLE IF NOT EXISTS metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts TEXT NOT NULL,
			reader_id TEXT NOT NULL,
			window_sec INTEGER NOT NULL,
			attempts INTEGER NOT NULL,
			failures INTEGER NOT NULL,
			aps REAL NOT NULL,
			fr REAL NOT NULL,
			uds REAL NOT NULL,
			tv REAL NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_reader_window ON metrics(reader_id, window_sec)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *sqliteStore) SaveAlert(ctx context.Context, alert model.Alert) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO alerts (ts, reader_id, severity, alert_type, window_sec, score, rules_json, metrics_json, context_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		alert.Timestamp.UTC(),
		alert.ReaderID,
		alert.Severity,
		alert.AlertType,
		alert.WindowSec,
		alert.Score,
		encodeJSON(alert.Rules),
		encodeJSON(alert.Metrics),
		encodeJSON(alert.Context),
	)
	return err
}

func (s *sqliteStore) SaveMetrics(ctx context.Context, readerID string, metrics []model.WindowMetrics) error {
	if s.db == nil || readerID == "" || len(metrics) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO metrics (ts, reader_id, window_sec, attempts, failures, aps, fr, uds, tv)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, wm := range metrics {
		if _, err := stmt.ExecContext(ctx,
			nowUTC(),
			readerID,
			wm.WindowSec,
			wm.Attempts,
			wm.Failures,
			wm.APS,
			wm.FR,
			wm.UDS,
			wm.TV,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
