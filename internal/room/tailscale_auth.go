package room

import (
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"strings"
)

const tailscaleLoginHeader = "Tailscale-User-Login"

func configuredTailUsers() map[string]bool {
	out := map[string]bool{}
	for _, raw := range strings.Split(os.Getenv("TAILSCALE_ALLOWED_USERS"), ",") {
		login := strings.ToLower(strings.TrimSpace(raw))
		if login != "" {
			out[login] = true
		}
	}
	return out
}

func (s *Server) validateTailAuthConfig() error {
	if len(configuredTailUsers()) == 0 {
		return nil
	}
	addr := strings.TrimSpace(s.Addr)
	if addr == "" {
		addr = "127.0.0.1:3319"
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("tailscale allowlist requires host:port ROOM_ADDR: %w", err)
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("tailscale allowlist requires loopback ROOM_ADDR")
	}
	return nil
}

func (s *Server) authorizeTailUser(r *http.Request) bool {
	allowed := configuredTailUsers()
	if len(allowed) == 0 {
		return true
	}
	login := strings.TrimSpace(r.Header.Get(tailscaleLoginHeader))
	if decoded, err := new(mime.WordDecoder).DecodeHeader(login); err == nil {
		login = decoded
	}
	return allowed[strings.ToLower(strings.TrimSpace(login))]
}
