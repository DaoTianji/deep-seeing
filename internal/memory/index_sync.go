package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"deep-seeing/internal/identity"
)

type EpisodeRetainer interface {
	RetainEpisode(context.Context, identity.TenantScope, Episode) error
}
type episodeIndexDeleter interface {
	DeleteEpisode(context.Context, identity.TenantScope, string) error
}
type IndexEntry struct {
	Revision string          `json:"revision"`
	Status   string          `json:"status"`
	At       time.Time       `json:"at"`
	Error    string          `json:"error,omitempty"`
	Usage    *HindsightUsage `json:"retain_usage,omitempty"`
}
type IndexProgress struct {
	Bank     string                `json:"bank"`
	Day      string                `json:"day"`
	Attempts int                   `json:"attempts_today"`
	Bytes    int                   `json:"bytes_today"`
	Entries  map[string]IndexEntry `json:"entries"`
}

// IndexSync reconciles files into a disposable external index. The durable
// attempt marker is committed BEFORE billable I/O: crash/timeout/HTTP failure
// never triggers an automatic retry of the same revision. One owner per path.
type IndexSync struct {
	mu     sync.Mutex
	Store  *EpisodeStore
	Scope  identity.TenantScope
	Client EpisodeRetainer
	Path   string
}

func (s *IndexSync) load() (IndexProgress, error) {
	p := IndexProgress{Bank: HindsightBank(s.Scope), Entries: map[string]IndexEntry{}}
	b, err := os.ReadFile(s.Path)
	if err != nil && !os.IsNotExist(err) {
		return p, err
	}
	if err == nil {
		if err = json.Unmarshal(b, &p); err != nil {
			return p, err
		}
		if p.Bank != HindsightBank(s.Scope) || p.Entries == nil {
			return p, fmt.Errorf("index journal scope mismatch")
		}
	}
	today := time.Now().UTC().Format("2006-01-02")
	if p.Day != today {
		p.Day, p.Attempts, p.Bytes = today, 0, 0
	}
	return p, nil
}

func (s *IndexSync) save(p IndexProgress) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.Path), ".index-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), s.Path)
}

// SyncPending processes at most maxDocs/maxBytes this batch and at most 50
// attempts/256 KiB per UTC day. No backfill or model call occurs unless invoked.
func (s *IndexSync) SyncPending(ctx context.Context, maxDocs, maxBytes int) (IndexProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Scope.Validate(); err != nil {
		return IndexProgress{}, err
	}
	p, err := s.load()
	if err != nil {
		return p, err
	}
	if maxDocs < 1 || maxDocs > 50 || maxBytes < 1 || maxBytes > 256*1024 {
		return p, fmt.Errorf("invalid index batch bounds")
	}
	eps, err := s.Store.Snapshot(ctx)
	if err != nil {
		return p, err
	}
	sort.Slice(eps, func(i, j int) bool { return eps[i].ID < eps[j].ID })
	count, bytes := 0, 0
	active := map[string]bool{}
	for _, ep := range eps {
		if OrdinaryEpisodeAllowed(ep, s.Scope) {
			active[ep.ID] = true
		}
	}
	if deleter, ok := s.Client.(episodeIndexDeleter); ok {
		ids := make([]string, 0, len(p.Entries))
		for id := range p.Entries {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			entry := p.Entries[id]
			if active[id] || entry.Status == "deleted" || entry.Status == "delete_uncertain" {
				continue
			}
			if count >= maxDocs {
				return p, nil
			}
			entry.Status = "delete_uncertain"
			entry.Revision = "withdrawn:" + entry.Revision
			entry.At = time.Now().UTC()
			p.Entries[id] = entry
			if err = s.save(p); err != nil {
				return p, err
			}
			if err = deleter.DeleteEpisode(ctx, s.Scope, id); err != nil {
				return p, err
			}
			entry.Status = "deleted"
			p.Entries[id] = entry
			if err = s.save(p); err != nil {
				return p, err
			}
			count++
		}
	}
	for _, ep := range eps {
		if err := ctx.Err(); err != nil {
			return p, err
		}
		if !OrdinaryEpisodeAllowed(ep, s.Scope) {
			continue
		}
		revision := EpisodeRevision(ep)
		if p.Entries[ep.ID].Revision == revision {
			continue
		}
		if len(ep.Content) > 64*1024 {
			continue
		}
		if count >= maxDocs || bytes+len(ep.Content) > maxBytes || p.Attempts >= 50 || p.Bytes+len(ep.Content) > 256*1024 {
			break
		}
		entry := IndexEntry{Revision: revision, Status: "inflight", At: time.Now().UTC()}
		p.Entries[ep.ID] = entry
		p.Attempts++
		p.Bytes += len(ep.Content)
		if err = s.save(p); err != nil {
			return p, err
		}
		meter, metered := s.Client.(interface{ Usage() HindsightUsage })
		var before HindsightUsage
		if metered {
			before = meter.Usage()
		}
		err = s.Client.RetainEpisode(ctx, s.Scope, ep)
		if metered {
			after := meter.Usage()
			if after.Responses > before.Responses {
				entry.Usage = &HindsightUsage{InputTokens: after.InputTokens - before.InputTokens, OutputTokens: after.OutputTokens - before.OutputTokens, TotalTokens: after.TotalTokens - before.TotalTokens, Responses: after.Responses - before.Responses}
			}
		}
		if err != nil {
			entry.Status = "failed_or_uncertain"
			entry.Error = "retain failed; inspect before explicit retry"
		} else {
			entry.Status = "indexed"
		}
		p.Entries[ep.ID] = entry
		if saveErr := s.save(p); saveErr != nil {
			return p, saveErr
		}
		count++
		bytes += len(ep.Content)
		if err != nil {
			return p, err
		} // halt this batch; no retry storm
	}
	return p, nil
}
