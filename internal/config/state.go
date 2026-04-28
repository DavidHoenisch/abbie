package config

// State configures optional shared persistence for routing (e.g. round-robin).
type State struct {
	Redis RedisState `yaml:"redis"`
}

// RedisState enables Redis-backed monotonic counters for round-robin.
// If URLEnv is set, the Redis connection URI (redis:// or rediss://) is read from
// that environment variable at runtime—do not put secrets in YAML.
//
// Addr/Password/DB are optional alternate fields when not using URLEnv (in-memory-only
// round-robin if URLEnv, Addr, are both unset).
type RedisState struct {
	URLEnv    string `yaml:"url_env"`
	Addr      string `yaml:"addr"`
	Password  string `yaml:"password"`
	DB        int    `yaml:"db"`
	KeyPrefix string `yaml:"key_prefix"`
}
