package progress

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStore_DefaultsToLevelOne(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "progress.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	level, err := s.CurrentLevel(context.Background())
	if err != nil {
		t.Fatalf("CurrentLevel: %v", err)
	}
	if level != 1 {
		t.Errorf("level = %d, want 1", level)
	}
}

func TestStore_SetLevel_Persists(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "progress.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	if err := s.SetLevel(ctx, 4); err != nil {
		t.Fatalf("SetLevel: %v", err)
	}
	level, err := s.CurrentLevel(ctx)
	if err != nil {
		t.Fatalf("CurrentLevel: %v", err)
	}
	if level != 4 {
		t.Errorf("level = %d, want 4", level)
	}
}

func TestStore_Open_ClosesDBOnSchemaError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "progress.db")
	// A file that exists but isn't a valid SQLite database makes the
	// CREATE TABLE step in Open fail deterministically, exercising the
	// error path that used to return without closing the *sql.DB.
	if err := os.WriteFile(dbPath, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatalf("seed corrupt db file: %v", err)
	}

	if _, err := Open(dbPath); err == nil {
		t.Fatal("expected Open to fail against a corrupted database file")
	}
}
