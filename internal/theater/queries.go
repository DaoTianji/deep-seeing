package theater

import (
	"context"
	"os"
	"path/filepath"
	"sort"
)

func (s *Store) ListSources(ctx context.Context, roleID string) ([]RoleSource, error) {
	d, err := s.GetDefinition(ctx, roleID)
	if err != nil {
		return nil, err
	}
	out := make([]RoleSource, 0, len(d.SourceIDs))
	for _, id := range d.SourceIDs {
		source, _, err := s.GetSource(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, source)
	}
	return out, nil
}

func (s *Store) ListWorldlines(_ context.Context, instanceID string) ([]RoleWorldline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths, err := filepath.Glob(filepath.Join(s.root, "worldlines", "*.json"))
	if err != nil {
		return nil, err
	}
	var out []RoleWorldline
	for _, path := range paths {
		var item RoleWorldline
		if readJSON(path, &item) != nil || item.RoleInstanceID != instanceID {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) ListSessions(_ context.Context, roleID string, limit int) ([]RoleSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths, err := filepath.Glob(filepath.Join(s.root, "sessions", "*.json"))
	if err != nil {
		return nil, err
	}
	var out []RoleSession
	for _, path := range paths {
		var item RoleSession
		if readJSON(path, &item) != nil || item.RoleID != roleID {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func isNotFound(err error) bool { return os.IsNotExist(err) }
