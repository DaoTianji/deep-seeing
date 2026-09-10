package memory

import (
	"context"
	"deep-seeing/internal/identity"
	"errors"
	"path/filepath"
	"testing"
)

type fakeRetainer struct {
	calls int
	fail  bool
}
type deletingRetainer struct {
	fakeRetainer
	deleted int
}

func (f *deletingRetainer) DeleteEpisode(context.Context, identity.TenantScope, string) error {
	f.deleted++
	return nil
}

func TestIndexWithdrawalIsPropagatedWithoutDeletingSource(t *testing.T) {
	ctx := context.Background()
	scope := identity.LocalCLI()
	store, _ := NewEpisodeStore(t.TempDir())
	ep, _ := store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "fictional withdrawn record"})
	f := &deletingRetainer{}
	s := &IndexSync{Store: store, Scope: scope, Client: f, Path: filepath.Join(t.TempDir(), "index.json")}
	_, err := s.SyncPending(ctx, 2, 1024)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SetStatus(ctx, ep.ID, EpisodeArchived, "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.SyncPending(ctx, 2, 1024)
	if err != nil || f.deleted != 1 || p.Entries[ep.ID].Status != "deleted" {
		t.Fatal(p, err)
	}
	if _, err = store.Get(ctx, ep.ID); err != nil {
		t.Fatal("source deleted")
	}
	_, _ = s.SyncPending(ctx, 2, 1024)
	if f.deleted != 1 {
		t.Fatal("repeated deletion")
	}
}

func (f *fakeRetainer) RetainEpisode(context.Context, identity.TenantScope, Episode) error {
	f.calls++
	if f.fail {
		return errors.New("upstream")
	}
	return nil
}
func TestIndexSyncBoundedDurableAndNoRetry(t *testing.T) {
	ctx := context.Background()
	scope := identity.LocalCLI()
	store, _ := NewEpisodeStore(t.TempDir())
	for i := 0; i < 3; i++ {
		_, _ = store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "fictional test memory", PersonIDs: []string{scope.PersonID()}})
	}
	_, _ = store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "role secret", RoleID: "role", ExperienceMode: ExperienceSimulatedRoleplay})
	f := &fakeRetainer{}
	s := &IndexSync{Store: store, Scope: scope, Client: f, Path: filepath.Join(t.TempDir(), "index.json")}
	p, err := s.SyncPending(ctx, 1, 1024)
	if err != nil || f.calls != 1 || p.Attempts != 1 {
		t.Fatal(p, err)
	}
	f.fail = true
	p, err = s.SyncPending(ctx, 1, 1024)
	if err == nil || f.calls != 2 {
		t.Fatal(p, err)
	}
	// New owner resumes the remaining revision, never repeats the failed one.
	s = &IndexSync{Store: store, Scope: scope, Client: f, Path: s.Path}
	f.fail = false
	p, err = s.SyncPending(ctx, 3, 1024)
	if err != nil || f.calls != 3 || len(p.Entries) != 3 {
		t.Fatal(p, err)
	}
	_, err = s.SyncPending(ctx, 3, 1024)
	if err != nil || f.calls != 3 {
		t.Fatal("repeated work")
	}
	other := &IndexSync{Store: store, Scope: identity.TenantScope{UserID: "other", AgentID: "deep-seeing"}, Client: f, Path: s.Path}
	if _, err = other.SyncPending(ctx, 1, 1024); err == nil {
		t.Fatal("cross-scope journal allowed")
	}
}
func TestIndexSyncDailyBudgetAndInflightRecovery(t *testing.T) {
	ctx := context.Background()
	scope := identity.LocalCLI()
	store, _ := NewEpisodeStore(t.TempDir())
	ep, _ := store.WriteEpisode(ctx, scope, EpisodeWrite{Content: "synthetic"})
	ep, _ = store.Get(ctx, ep.ID)
	f := &fakeRetainer{}
	s := &IndexSync{Store: store, Scope: scope, Client: f, Path: filepath.Join(t.TempDir(), "index.json")}
	p, _ := s.load()
	p.Attempts = 50
	_ = s.save(p)
	_, err := s.SyncPending(ctx, 1, 1024)
	if err != nil || f.calls != 0 {
		t.Fatal("daily cap ignored")
	}
	p.Attempts = 0
	p.Entries[ep.ID] = IndexEntry{Revision: EpisodeRevision(ep), Status: "inflight"}
	_ = s.save(p)
	_, err = s.SyncPending(ctx, 1, 1024)
	if err != nil || f.calls != 0 {
		t.Fatal("uncertain write retried")
	}
}
