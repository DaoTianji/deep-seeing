package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"deep-seeing/internal/identity"
	"deep-seeing/internal/transcript"
)

const defaultSTMTTL = 24 * time.Hour

// RedisSTM stores session transcripts in Redis with TTL.
// Concurrent writers to the same session use read-modify-write without locks
// (adequate for single-writer CLI; multi-writer needs stronger concurrency control).
type RedisSTM struct {
	client      *redis.Client
	scope       identity.TenantScope
	MaxMessages int
	TTL         time.Duration
}

// RedisSTMConfig holds connection and window settings.
type RedisSTMConfig struct {
	Addr        string
	Password    string
	DB          int
	TTL         time.Duration
	MaxMessages int
	Scope       identity.TenantScope
}

// RedisSTMConfigFromEnv reads REDIS_* and STM_MAX_MESSAGES / REDIS_STM_TTL_SEC.
func RedisSTMConfigFromEnv(scope identity.TenantScope) RedisSTMConfig {
	db, _ := strconv.Atoi(strings.TrimSpace(os.Getenv("REDIS_DB")))
	ttlSec, _ := strconv.Atoi(strings.TrimSpace(os.Getenv("REDIS_STM_TTL_SEC")))
	ttl := defaultSTMTTL
	if ttlSec > 0 {
		ttl = time.Duration(ttlSec) * time.Second
	}
	maxMsg := STMMaxMessagesFromEnv()
	return RedisSTMConfig{
		Addr:        strings.TrimSpace(os.Getenv("REDIS_ADDR")),
		Password:    os.Getenv("REDIS_PASSWORD"),
		DB:          db,
		TTL:         ttl,
		MaxMessages: maxMsg,
		Scope:       scope,
	}
}

// STMMaxMessagesFromEnv returns STM_MAX_MESSAGES (default 100).
func STMMaxMessagesFromEnv() int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("STM_MAX_MESSAGES")))
	if err != nil || n <= 0 {
		return 100
	}
	return n
}

// NewRedisSTM dials Redis and pings. Returns error if unreachable.
func NewRedisSTM(ctx context.Context, cfg RedisSTMConfig) (*RedisSTM, error) {
	if strings.TrimSpace(cfg.Addr) == "" {
		return nil, fmt.Errorf("REDIS_ADDR empty")
	}
	if err := cfg.Scope.Validate(); err != nil {
		return nil, err
	}
	maxMsg := cfg.MaxMessages
	if maxMsg <= 0 {
		maxMsg = 100
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultSTMTTL
	}
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &RedisSTM{
		client:      client,
		scope:       cfg.Scope,
		MaxMessages: maxMsg,
		TTL:         ttl,
	}, nil
}

// NewRedisSTMFromEnv builds RedisSTM from environment variables.
func NewRedisSTMFromEnv(ctx context.Context, scope identity.TenantScope) (*RedisSTM, error) {
	return NewRedisSTM(ctx, RedisSTMConfigFromEnv(scope))
}

// Close closes the underlying Redis client.
func (s *RedisSTM) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisSTM) key(sessionID string) string {
	return fmt.Sprintf("ds:stm:%s:%s:%s", s.scope.UserID, s.scope.AgentID, sessionID)
}

func (s *RedisSTM) Get(sessionID string) ([]transcript.Message, error) {
	raw, err := s.client.Get(context.Background(), s.key(sessionID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var msgs []transcript.Message
	if err := json.Unmarshal(raw, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func (s *RedisSTM) Append(sessionID string, msgs ...transcript.Message) error {
	cur, err := s.Get(sessionID)
	if err != nil {
		return err
	}
	cur = append(cur, msgs...)
	return s.Replace(sessionID, cur)
}

func (s *RedisSTM) Replace(sessionID string, msgs []transcript.Message) error {
	cur := trimToMax(append([]transcript.Message(nil), msgs...), s.MaxMessages)
	raw, err := json.Marshal(cur)
	if err != nil {
		return err
	}
	return s.client.Set(context.Background(), s.key(sessionID), raw, s.TTL).Err()
}
