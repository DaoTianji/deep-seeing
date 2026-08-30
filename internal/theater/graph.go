package theater

import (
	"context"

	"deep-seeing/internal/graph"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
)

type RoleGraphIndexer interface {
	UpsertRolePointer(context.Context, identity.TenantScope, graph.RolePointer) error
	UpsertRoleSourcePointer(context.Context, graph.RoleSourcePointer, bool) error
	UpsertRoleEpisodePointer(context.Context, identity.TenantScope, graph.RoleEpisodePointer) error
}

func IndexRole(ctx context.Context, target RoleGraphIndexer, scope identity.TenantScope, store *Store, roleID string) error {
	if target == nil || store == nil {
		return nil
	}
	d, err := store.GetDefinition(ctx, roleID)
	if err != nil {
		return err
	}
	base := graph.RolePointer{
		ID: d.ID, DisplayName: d.DisplayName, Kind: string(d.Kind),
		SubjectClass: string(d.SubjectClass), Status: string(d.Status),
		PrivateSandbox: d.PrivateSandbox, Version: d.Version,
	}
	if d.MainInstanceID == "" {
		if err := target.UpsertRolePointer(ctx, scope, base); err != nil {
			return err
		}
	} else {
		inst, err := store.GetInstance(ctx, d.MainInstanceID)
		if err != nil {
			return err
		}
		worlds, err := store.ListWorldlines(ctx, inst.ID)
		if err != nil {
			return err
		}
		if len(worlds) == 0 {
			item := base
			item.InstanceID, item.InstanceStatus, item.PersonID = inst.ID, string(inst.Status), inst.PersonID
			if err := target.UpsertRolePointer(ctx, scope, item); err != nil {
				return err
			}
		}
		for _, world := range worlds {
			item := base
			item.InstanceID, item.InstanceStatus, item.PersonID = inst.ID, string(inst.Status), inst.PersonID
			item.WorldlineID, item.ParentWorldlineID, item.WorldlineLabel = world.ID, world.ParentWorldlineID, world.Label
			if err := target.UpsertRolePointer(ctx, scope, item); err != nil {
				return err
			}
		}
	}
	claims, err := store.ListClaims(ctx, d.ID)
	if err != nil {
		return err
	}
	challenged := map[string]bool{}
	for _, claim := range claims {
		if claim.Kind == ClaimContested {
			for _, id := range claim.SourceIDs {
				challenged[id] = true
			}
		}
	}
	sources, err := store.ListSources(ctx, d.ID)
	if err != nil {
		return err
	}
	for _, source := range sources {
		if err := target.UpsertRoleSourcePointer(ctx, graph.RoleSourcePointer{
			ID: source.ID, RoleID: source.RoleID, Title: source.Title, Kind: source.Kind, URL: source.URL,
		}, challenged[source.ID]); err != nil {
			return err
		}
	}
	return nil
}

func IndexRoleEpisode(ctx context.Context, target RoleGraphIndexer, scope identity.TenantScope, ep memory.Episode) error {
	if target == nil || ep.RoleID == "" {
		return nil
	}
	return target.UpsertRoleEpisodePointer(ctx, scope, graph.RoleEpisodePointer{
		ID: ep.ID, RoleID: ep.RoleID, RoleInstanceID: ep.RoleInstanceID,
		RoleSessionID: ep.RoleSessionID, WorldlineID: ep.WorldlineID,
		Kind: string(ep.Kind), MemoryClass: ep.RoleMemoryClass,
		Summary: graph.SummaryFromContent(ep.Content, 160), CreatedAt: ep.CreatedAt, Status: string(ep.Status),
	})
}

func IndexRoleMemories(ctx context.Context, target RoleGraphIndexer, scope identity.TenantScope, episodes *memory.EpisodeStore, roleID string) error {
	if target == nil || episodes == nil {
		return nil
	}
	items, err := episodes.Search(ctx, scope, memory.Query{Limit: 500, IncludeRole: true, RoleID: roleID})
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := IndexRoleEpisode(ctx, target, scope, item); err != nil {
			return err
		}
	}
	return nil
}
