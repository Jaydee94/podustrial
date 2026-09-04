package progress

import (
	"context"
	"os"
	"path/filepath"
	"sync"
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

func TestStore_ConcurrentSetLevel_NoErrors(t *testing.T) {
	// Regression: modernc.org/sqlite doesn't serialize writers on its own,
	// and database/sql defaults to a connection pool of more than one. Two
	// SetLevel calls landing close together (e.g. from the HTTP API and the
	// WebSocket hub) used to fail with "database is locked" (SQLITE_BUSY)
	// rather than being queued.
	dbPath := filepath.Join(t.TempDir(), "progress.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(level int) {
			defer wg.Done()
			errs <- s.SetLevel(ctx, level+1)
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("concurrent SetLevel: %v", err)
		}
	}
}

func TestStore_SetLevel_RejectsBelowOne(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "progress.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	for _, level := range []int{0, -1, -100} {
		if err := s.SetLevel(ctx, level); err == nil {
			t.Errorf("SetLevel(%d) = nil error, want an error", level)
		}
	}

	// Rejecting an invalid level must not have touched the stored value.
	got, err := s.CurrentLevel(ctx)
	if err != nil {
		t.Fatalf("CurrentLevel: %v", err)
	}
	if got != 1 {
		t.Errorf("level = %d after rejected SetLevel calls, want unchanged default 1", got)
	}
}
