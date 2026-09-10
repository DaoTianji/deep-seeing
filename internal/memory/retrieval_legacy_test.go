package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotSkipsAppleDoubleAndLegacyRevisionIsStable(t *testing.T) {
	root := t.TempDir()
	s, _ := NewEpisodeStore(root)
	_ = os.WriteFile(filepath.Join(root, "by_id", "._ep_legacy.md"), []byte("apple metadata"), 0600)
	p := filepath.Join(root, "by_id", "ep_legacy.md")
	_ = os.WriteFile(p, []byte("Legacy synthetic record without timestamps"), 0600)
	stamp := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	_ = os.Chtimes(p, stamp, stamp)
	a, err := s.Snapshot(context.Background())
	if err != nil || len(a) != 1 {
		t.Fatal(a, err)
	}
	b, err := s.Get(context.Background(), "ep_legacy")
	if err != nil {
		t.Fatal(err)
	}
	if EpisodeRevision(a[0]) != EpisodeRevision(b) || !b.CreatedAt.Equal(stamp) {
		t.Fatal("unstable revision")
	}
}
