package room

import (
	"net/http/httptest"
	"testing"
)

func TestTailscaleAllowlist(t *testing.T) {
	t.Setenv("TAILSCALE_ALLOWED_USERS", "owner@example.com, second@example.com")
	server := &Server{Addr: "127.0.0.1:3319"}
	allowed := httptest.NewRequest("GET", "http://localhost/", nil)
	allowed.Header.Set(tailscaleLoginHeader, "owner@example.com")
	if !server.authorizeTailUser(allowed) {
		t.Fatal("allowed Tailscale user rejected")
	}
	denied := httptest.NewRequest("GET", "http://localhost/", nil)
	denied.Header.Set(tailscaleLoginHeader, "intruder@example.com")
	if server.authorizeTailUser(denied) {
		t.Fatal("unknown Tailscale user allowed")
	}
	missing := httptest.NewRequest("GET", "http://localhost/", nil)
	if server.authorizeTailUser(missing) {
		t.Fatal("missing Tailscale identity allowed")
	}
}

func TestTailscaleAllowlistRequiresLoopback(t *testing.T) {
	t.Setenv("TAILSCALE_ALLOWED_USERS", "owner@example.com")
	if err := (&Server{Addr: "0.0.0.0:3319"}).validateTailAuthConfig(); err == nil {
		t.Fatal("non-loopback listener accepted with proxy identity auth")
	}
	if err := (&Server{Addr: "127.0.0.1:3319"}).validateTailAuthConfig(); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyTailscaleAllowlistKeepsLocalCompatibility(t *testing.T) {
	t.Setenv("TAILSCALE_ALLOWED_USERS", "")
	if !(&Server{}).authorizeTailUser(httptest.NewRequest("GET", "http://localhost/", nil)) {
		t.Fatal("empty allowlist broke local development")
	}
}
