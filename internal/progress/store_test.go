package progress

import (
	"context"
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
