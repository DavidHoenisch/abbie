package state

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DavidHoenisch/abbie/internal/config"
	"github.com/redis/go-redis/v9"
)

func redisOptsFrom(rs *config.RedisState) (*redis.Options, error) {
	if rs.URLEnv != "" {
		raw := strings.TrimSpace(os.Getenv(rs.URLEnv))
		if raw == "" {
			return nil, fmt.Errorf("redis: environment variable %q is unset or empty", rs.URLEnv)
		}
		opts, err := redis.ParseURL(raw)
		if err != nil {
			return nil, fmt.Errorf("parse redis url from %q: %w", rs.URLEnv, err)
		}
		applyRedisTimeouts(opts)
		return opts, nil
	}
	opts := &redis.Options{
		Addr:     rs.Addr,
		Password: rs.Password,
		DB:       rs.DB,
	}
	applyRedisTimeouts(opts)
	return opts, nil
}

func applyRedisTimeouts(opts *redis.Options) {
	// Managed Redis often closes idle TCP connections; recycling pool conns avoids EOF on reuse.
	opts.ConnMaxIdleTime = 30 * time.Second
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 5 * time.Second
	opts.WriteTimeout = 5 * time.Second
	// Transient EOF / connection reset after idle is common; allow a few retries with backoff.
	opts.MaxRetries = 5
	opts.MinRetryBackoff = 100 * time.Millisecond
	opts.MaxRetryBackoff = 2 * time.Second
}
