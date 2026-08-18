// Package graph is the Neo4j L2 context graph (Bond / Episode pointers / CALLS).
package graph

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"deep-seeing/internal/identity"
)

// Config holds Neo4j connection settings.
type Config struct {
	URI      string
	User     string
	Password string
	Database string
}

// ConfigFromEnv reads NEO4J_* and optional LTM_GRAPH toggle.
// Returns ok=false when graph should stay offline (missing creds or LTM_GRAPH=0).
func ConfigFromEnv() (Config, bool) {
	toggle := strings.TrimSpace(strings.ToLower(os.Getenv("LTM_GRAPH")))
	if toggle == "0" || toggle == "false" || toggle == "off" {
		return Config{}, false
	}
	cfg := Config{
		URI:      strings.TrimSpace(os.Getenv("NEO4J_URI")),
		User:     strings.TrimSpace(os.Getenv("NEO4J_USER")),
		Password: os.Getenv("NEO4J_PASSWORD"),
		Database: strings.TrimSpace(os.Getenv("NEO4J_DATABASE")),
	}
	if cfg.Database == "" {
		cfg.Database = "neo4j"
	}
	if cfg.URI == "" || cfg.User == "" || strings.TrimSpace(cfg.Password) == "" {
		if toggle == "1" || toggle == "true" || toggle == "on" {
			return cfg, false
		}
		return Config{}, false
	}
	return cfg, true
}

// Store is the Neo4j-backed graph API used by tools and SideQuery.
type Store struct {
	driver   neo4j.DriverWithContext
	database string
}

// Open connects and pings Neo4j.
func Open(ctx context.Context, cfg Config) (*Store, error) {
	if strings.TrimSpace(cfg.URI) == "" {
		return nil, fmt.Errorf("neo4j: empty URI")
	}
	if strings.TrimSpace(cfg.Database) == "" {
		cfg.Database = "neo4j"
	}
	drv, err := neo4j.NewDriverWithContext(cfg.URI, neo4j.BasicAuth(cfg.User, cfg.Password, ""))
	if err != nil {
		return nil, err
	}
	s := &Store{driver: drv, database: cfg.Database}
	if err := s.Ping(ctx); err != nil {
		_ = drv.Close(ctx)
		return nil, err
	}
	return s, nil
}

// OpenFromEnv opens when ConfigFromEnv says graph is enabled; otherwise returns (nil, nil).
func OpenFromEnv(ctx context.Context) (*Store, error) {
	cfg, ok := ConfigFromEnv()
	if !ok {
		return nil, nil
	}
	return Open(ctx, cfg)
}

// Ping verifies connectivity.
func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.driver == nil {
		return fmt.Errorf("neo4j: nil store")
	}
	return s.driver.VerifyConnectivity(ctx)
}

// Close releases the driver.
func (s *Store) Close(ctx context.Context) error {
	if s == nil || s.driver == nil {
		return nil
	}
	return s.driver.Close(ctx)
}

func (s *Store) session(ctx context.Context) neo4j.SessionWithContext {
	return s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.database})
}

func (s *Store) write(ctx context.Context, cypher string, params map[string]any) error {
	sess := s.session(ctx)
	defer sess.Close(ctx)
	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, cypher, params)
		return nil, err
	})
	return err
}

func (s *Store) read(ctx context.Context, cypher string, params map[string]any, scan func(*neo4j.Record) error) error {
	sess := s.session(ctx)
	defer sess.Close(ctx)
	_, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		for res.Next(ctx) {
			if err := scan(res.Record()); err != nil {
				return nil, err
			}
		}
		return nil, res.Err()
	})
	return err
}

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []any:
		parts := make([]string, 0, len(t))
		for _, x := range t {
			parts = append(parts, asString(x))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(t)
	}
}

func asStringSlice(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			s := strings.TrimSpace(asString(x))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return strings.Split(t, ",")
	default:
		return nil
	}
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}

func asTime(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case neo4j.LocalDateTime:
		return t.Time()
	case neo4j.Date:
		return t.Time()
	case string:
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return ts
		}
	}
	return time.Time{}
}

func normalizePersonID(scope identity.TenantScope, personID string) string {
	personID = strings.TrimSpace(personID)
	if personID == "" {
		return scope.PersonID()
	}
	if !strings.Contains(personID, ":") {
		return "user:" + personID
	}
	return personID
}

func displayNameFromPersonID(personID string) string {
	personID = strings.TrimPrefix(personID, "user:")
	personID = strings.TrimPrefix(personID, "person:")
	return personID
}
