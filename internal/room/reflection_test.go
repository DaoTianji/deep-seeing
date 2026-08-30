package room

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"deep-seeing/internal/app"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/runtime"
)

func TestReflectionAPIsExposeDurableAndLivePublicState(t *testing.T) {
	store, err := memory.NewReflectionStore(filepath.Join(t.TempDir(), "reflections"))
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.LocalCLI()
	seed, err := store.Create(context.Background(), scope, memory.ReflectionSeedWrite{
		Scope: memory.ReflectionScopeTension, Statement: "虚构张力", SourceType: memory.ReflectionSourceInferred,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendRun(memory.ReflectionRun{PersonID: scope.PersonID(), Mode: memory.ReflectionModeObserve, SeedIDs: []string{seed.ID}, NoChange: true}); err != nil {
		t.Fatal(err)
	}
	live := &memory.ReflectionLiveStore{}
	live.Update(memory.ReflectionRun{ID: "live-run", PersonID: scope.PersonID(), SeedIDs: []string{seed.ID}}, "reading", true)
	server := &Server{App: &app.App{
		Scope: scope, Service: &runtime.Service{}, Reflections: store,
		Reflection: &memory.ReflectionEngine{Live: live},
	}}
	handler, err := server.Handler()
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"/api/reflections":     seed.ID,
		"/api/reflection-runs": seed.ID,
		"/api/reflection-live": `"phase":"reading"`,
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), want) {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}
