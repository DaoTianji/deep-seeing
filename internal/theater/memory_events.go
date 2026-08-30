package theater

import (
	"context"

	"deep-seeing/internal/memory"
)

func (r *Router) roleMemoryIDs(ctx context.Context, roleID, instanceID, sessionID string) map[string]bool {
	out := map[string]bool{}
	if r == nil || r.Reviewer == nil || r.Reviewer.Episodes == nil {
		return out
	}
	items, err := r.Reviewer.Episodes.Search(ctx, r.Scope, memory.Query{
		Limit: 500, IncludeRole: true, RoleID: roleID, RoleInstanceID: instanceID, RoleSessionID: sessionID,
	})
	if err != nil {
		return out
	}
	for _, item := range items {
		out[item.ID] = true
	}
	return out
}

func (r *Router) emitNewRoleMemories(ctx context.Context, d RoleDefinition, inst RoleInstance, session RoleSession, known map[string]bool, emit func(RoleEvent)) {
	if r == nil || r.Reviewer == nil || r.Reviewer.Episodes == nil {
		return
	}
	items, err := r.Reviewer.Episodes.Search(ctx, r.Scope, memory.Query{
		Limit: 500, IncludeRole: true, RoleID: d.ID, RoleInstanceID: inst.ID, RoleSessionID: session.ID,
	})
	if err != nil {
		return
	}
	for _, item := range items {
		if known[item.ID] {
			continue
		}
		emitRole(emit, RoleEvent{
			Type: "role_memory_written", RoleID: d.ID, RoleInstanceID: inst.ID,
			RoleSessionID: session.ID, WorldlineID: item.WorldlineID,
			Data: map[string]any{"episode_id": item.ID, "memory_class": item.RoleMemoryClass},
		})
	}
}
