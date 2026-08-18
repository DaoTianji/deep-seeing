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
	"deep-seeing/internal/intent"
	"deep-seeing/internal/runtime"
	"deep-seeing/internal/selfmodel"
	"deep-seeing/internal/workspace"
	"deep-seeing/internal/world"
)

func TestMindPageAndReadAPIs(t *testing.T) {
	root := t.TempDir()
	selfStore, err := selfmodel.NewStore(filepath.Join(root, "self"))
	if err != nil {
		t.Fatal(err)
	}
	workspaceStore, err := workspace.NewStore(filepath.Join(root, "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	intentStore, err := intent.OpenStore(filepath.Join(root, "runtime"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = intentStore.Close() })
	worldGateway, err := world.NewGateway(filepath.Join(root, "sources"))
	if err != nil {
		t.Fatal(err)
	}

	server := &Server{App: &app.App{
		Scope:     identity.LocalCLI(),
		Service:   &runtime.Service{},
		Self:      selfStore,
		Workspace: workspaceStore,
		Intents:   intentStore,
		World:     worldGateway,
	}}
	handler, err := server.Handler()
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/mind", "/mind/", "/mind.js", "/mind.css", "/pet", "/pet/", "/pet.js", "/pet.css", "/api/self", "/api/workspace", "/api/intents", "/api/wakes", "/api/agency", "/api/sources"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Errorf("%s: got status %d, body=%s", path, response.Code, response.Body.String())
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/pet", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	for _, controlID := range []string{"pet-open-room", "pet-clear", "pet-resize-grip"} {
		if !strings.Contains(response.Body.String(), `id="`+controlID+`"`) {
			t.Errorf("/pet: missing desktop control %s", controlID)
		}
	}
}

func TestMindDetailAPIsReturnNotFound(t *testing.T) {
	root := t.TempDir()
	selfStore, _ := selfmodel.NewStore(filepath.Join(root, "self"))
	workspaceStore, _ := workspace.NewStore(filepath.Join(root, "workspace"))
	intentStore, _ := intent.OpenStore(filepath.Join(root, "runtime"))
	t.Cleanup(func() { _ = intentStore.Close() })
	worldGateway, _ := world.NewGateway(filepath.Join(root, "sources"))

	server := &Server{App: &app.App{
		Scope: identity.LocalCLI(), Service: &runtime.Service{},
		Self: selfStore, Workspace: workspaceStore, Intents: intentStore, World: worldGateway,
	}}
	handler, err := server.Handler()
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/api/self/missing", "/api/workspace/missing", "/api/source/missing"} {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("%s: got status %d", path, response.Code)
		}
	}
}
