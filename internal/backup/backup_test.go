package backup_test

import (
	"os"
	"path/filepath"
	"testing"

	"deep-seeing/internal/backup"
)

func TestSnapshotCopiesPresentRoots(t *testing.T) {
	ws := t.TempDir()
	ep := filepath.Join(ws, "data", "memory", "episodes")
	if err := os.MkdirAll(ep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ep, "index.md"), []byte("# idx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest, err := backup.Snapshot(ws, "", []string{"data/memory/episodes"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "data", "memory", "episodes", "index.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "BACKUP_META.txt")); err != nil {
		t.Fatal(err)
	}
}
