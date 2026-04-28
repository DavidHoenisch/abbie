package state

import (
	"testing"

	"github.com/DavidHoenisch/abbie/internal/config"
	"github.com/alicebob/miniredis/v2"
)

func TestNewRoundRobin_memory(t *testing.T) {
	cfg := &config.Config{}
	rr, err := NewRoundRobin(cfg)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]struct{}{}
	for i := 0; i < 10; i++ {
		idx, err := rr.NextPool([]string{"a", "b"})
		if err != nil {
			t.Fatal(err)
		}
		seen[idx] = struct{}{}
	}
	if len(seen) < 2 {
		t.Fatalf("expected both indices, got %+v", seen)
	}
}

func TestNewRoundRobin_memory_disjointPools(t *testing.T) {
	cfg := &config.Config{}
	rr, err := NewRoundRobin(cfg)
	if err != nil {
		t.Fatal(err)
	}
	x, _ := rr.NextPool([]string{"a", "b"})
	y, _ := rr.NextPool([]string{"x", "y", "z"})
	if x < 0 || x > 1 || y < 0 || y > 2 {
		t.Fatalf("bad idx x=%d y=%d", x, y)
	}
}

func TestNewRoundRobin_redisSharedCounter(t *testing.T) {
	s := miniredis.RunT(t)
	cfg := &config.Config{}
	cfg.State.Redis.Addr = s.Addr()
	cfg.State.Redis.KeyPrefix = "test:"

	a, err := NewRoundRobin(cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewRoundRobin(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pool := []string{"b1", "b2", "b3"}
	x1, err := a.NextPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	x2, err := b.NextPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	if x2 != (x1+1)%3 {
		t.Fatalf("expected sequential indices: got %d then %d", x1, x2)
	}
}

func TestNewRoundRobin_redisFromEnv(t *testing.T) {
	const envName = "ABBIE_TEST_REDIS_URL"
	s := miniredis.RunT(t)
	t.Setenv(envName, "redis://"+s.Addr()+"/0")
	cfg := &config.Config{}
	cfg.State.Redis.URLEnv = envName
	cfg.State.Redis.KeyPrefix = "env:"

	a, err := NewRoundRobin(cfg)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := a.NextPool([]string{"x", "y"})
	if err != nil || (idx != 0 && idx != 1) {
		t.Fatalf("got idx=%d err=%v", idx, err)
	}
}

func TestNewRoundRobin_missingURLEnv(t *testing.T) {
	cfg := &config.Config{}
	cfg.State.Redis.URLEnv = "ABBIE_MISSING_REDIS_FOR_TEST"
	cfg.State.Redis.KeyPrefix = "x:"
	_, err := NewRoundRobin(cfg)
	if err == nil {
		t.Fatal("expected error when env is unset")
	}
}

func TestNewRoundRobin_invalidRedisURLFromEnv(t *testing.T) {
	const envName = "ABBIE_BAD_REDIS_URI"
	t.Setenv(envName, "not-a-valid-redis-uri")
	cfg := &config.Config{}
	cfg.State.Redis.URLEnv = envName
	_, err := NewRoundRobin(cfg)
	if err == nil {
		t.Fatal("expected parse error")
	}
}
