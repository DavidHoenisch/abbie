package state

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/DavidHoenisch/abbie/internal/config"
	"github.com/redis/go-redis/v9"
)

// RoundRobin advances a monotonic counter per pool (ordered backend names).
type RoundRobin interface {
	NextPool(orderedBackendNames []string) (idx int, err error)
}

// NewRoundRobin wires memory-only or Redis-backed round-robin from config.
func NewRoundRobin(cfg *config.Config) (RoundRobin, error) {
	rs := cfg.State.Redis
	if redisDisabled(rs) {
		return newMemoryRoundRobin(), nil
	}

	opts, err := redisOptsFrom(&rs)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	return &redisRR{
		client:    client,
		keyPrefix: rs.KeyPrefix,
		fall:      newMemoryRoundRobin(),
	}, nil
}

func redisDisabled(rs config.RedisState) bool {
	return rs.URLEnv == "" && rs.Addr == ""
}

type memoryRR struct {
	mu    sync.Mutex
	pools map[string]*atomic.Uint64
}

func newMemoryRoundRobin() *memoryRR {
	return &memoryRR{pools: make(map[string]*atomic.Uint64)}
}

func (m *memoryRR) NextPool(ordered []string) (int, error) {
	n := len(ordered)
	if n <= 0 {
		return 0, fmt.Errorf("round-robin: empty pool")
	}
	key := PoolKey(ordered)
	m.mu.Lock()
	a, ok := m.pools[key]
	if !ok {
		a = new(atomic.Uint64)
		m.pools[key] = a
	}
	m.mu.Unlock()
	v := a.Add(1) - 1
	return int(v % uint64(n)), nil
}

type redisRR struct {
	client    *redis.Client
	keyPrefix string
	fall      *memoryRR
}

func (r *redisRR) NextPool(ordered []string) (int, error) {
	n := len(ordered)
	if n <= 0 {
		return 0, fmt.Errorf("round-robin: empty pool")
	}
	ctx := context.Background()
	subkey := r.keyPrefix + "pool/" + PoolKey(ordered)
	val, err := r.client.Incr(ctx, subkey).Result()
	if err != nil {
		log.Printf("redis INCR fallback to in-memory: %v", err)
		return r.fall.NextPool(ordered)
	}
	return int((val - 1) % int64(n)), nil
}
