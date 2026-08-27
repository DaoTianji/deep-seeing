package contextsource_test

import (
	"testing"

	"deep-seeing/internal/contextsource"
)

func TestFixedSourceRoles(t *testing.T) {
	cases := map[contextsource.Source]contextsource.Role{
		contextsource.Bond:      contextsource.Baseline,
		contextsource.SceneNorm: contextsource.Guidance,
		contextsource.Workspace: contextsource.Task,
		contextsource.Intent:    contextsource.Plan,
		contextsource.Proposal:  contextsource.Hypothesis,
		contextsource.Episode:   contextsource.Evidence,
	}
	for source, want := range cases {
		if got := contextsource.RoleFor(source); got != want || !contextsource.ValidSource(source) {
			t.Fatalf("%s role=%s valid=%v, want %s", source, got, contextsource.ValidSource(source), want)
		}
	}
	if contextsource.ValidSource("unknown") || contextsource.RoleFor("unknown") != "" {
		t.Fatal("unknown source was accepted")
	}
}

func TestOrderedSourcesIsStableAndComplete(t *testing.T) {
	want := []contextsource.Source{
		contextsource.Bond, contextsource.SceneNorm, contextsource.Workspace,
		contextsource.Intent, contextsource.Proposal, contextsource.Episode,
	}
	got := contextsource.OrderedSources()
	if len(got) != len(want) {
		t.Fatalf("ordered sources=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordered sources=%v, want %v", got, want)
		}
	}
}
