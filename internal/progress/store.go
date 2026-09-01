package progress

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS progress (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		level INTEGER NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO progress (id, level) VALUES (1, 1)`); err != nil {
		return nil, fmt.Errorf("seed row: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) CurrentLevel(ctx context.Context) (int, error) {
	var level int
	if err := s.db.QueryRowContext(ctx, `SELECT level FROM progress WHERE id = 1`).Scan(&level); err != nil {
		return 0, fmt.Errorf("query level: %w", err)
	}
	return level, nil
}

func (s *Store) SetLevel(ctx context.Context, level int) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE progress SET level = ? WHERE id = 1`, level); err != nil {
		return fmt.Errorf("update level: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
