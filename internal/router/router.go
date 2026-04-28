package router

import (
	"fmt"
	"net/http"

	"github.com/DavidHoenisch/abbie/internal/config"
	"github.com/DavidHoenisch/abbie/internal/state"
)

// Router handles request routing based on configuration (ordered routing rules).
type Router struct {
	config *config.Config
	rr     state.RoundRobin
}

// NewRouter builds a Router. Round-robin state comes from cfg.State.REDIS when
// configured (see internal/state).
func NewRouter(cfg *config.Config) (*Router, error) {
	rr, err := state.NewRoundRobin(cfg)
	if err != nil {
		return nil, err
	}
	return &Router{config: cfg, rr: rr}, nil
}

// SelectBackend evaluates routing rules in config order until one selects a backend.
func (r *Router) SelectBackend(req *http.Request) (*config.Backend, error) {
	if len(r.config.Backends) == 0 {
		return nil, fmt.Errorf("no backends configured")
	}
	if len(r.config.Routing) == 0 {
		return nil, fmt.Errorf("routing: no rules configured")
	}

	rules := r.config.Routing
	last := len(rules) - 1
	for i := range rules {
		isLast := i == last
		b, skip, err := r.applyRule(&rules[i], req, isLast)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		if b != nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("no backend selected")
}

func (r *Router) applyRule(rule *config.RoutingRule, req *http.Request, isLast bool) (backend *config.Backend, tryNextRule bool, err error) {
	switch rule.Strategy {
	case config.RoundRobin:
		names, err := r.roundRobinOrderedNames(rule)
		if err != nil {
			return nil, false, err
		}
		idx, err := r.rr.NextPool(names)
		if err != nil {
			return nil, false, err
		}
		b := r.backendByName(names[idx])
		if b == nil {
			return nil, false, fmt.Errorf("round-robin: no backend %q", names[idx])
		}
		return b, false, nil

	case config.Static:
		return &r.config.Backends[0], false, nil

	case config.QueryParam:
		v := req.URL.Query().Get(rule.ParamName)
		if v == "" {
			if ck, cerr := req.Cookie(rule.ParamName); cerr == nil {
				v = ck.Value
			}
		}
		if v == "" {
			if isLast {
				return r.getDefaultBackend(rule), false, nil
			}
			return nil, true, nil
		}
		return r.findBackendByGroup(v, rule), false, nil

	case config.Header:
		v := req.Header.Get(rule.ParamName)
		if v == "" {
			if isLast {
				return r.getDefaultBackend(rule), false, nil
			}
			return nil, true, nil
		}
		return r.findBackendByGroup(v, rule), false, nil

	case config.Cookie:
		ck, cerr := req.Cookie(rule.ParamName)
		if cerr != nil || ck.Value == "" {
			if isLast {
				return r.getDefaultBackend(rule), false, nil
			}
			return nil, true, nil
		}
		return r.findBackendByGroup(ck.Value, rule), false, nil

	default:
		return nil, false, fmt.Errorf("unknown routing strategy: %s", rule.Strategy)
	}
}

func (r *Router) findBackendByGroup(group string, rule *config.RoutingRule) *config.Backend {
	if group == "" {
		return r.getDefaultBackend(rule)
	}
	for i := range r.config.Backends {
		backend := &r.config.Backends[i]
		for _, g := range backend.Groups {
			if g == group {
				return backend
			}
		}
	}
	return r.getDefaultBackend(rule)
}

func (r *Router) getDefaultBackend(rule *config.RoutingRule) *config.Backend {
	if rule.DefaultGroup != "" {
		for i := range r.config.Backends {
			backend := &r.config.Backends[i]
			for _, g := range backend.Groups {
				if g == rule.DefaultGroup {
					return backend
				}
			}
		}
	}
	if len(r.config.Backends) > 0 {
		return &r.config.Backends[0]
	}
	return nil
}

func (r *Router) roundRobinOrderedNames(rule *config.RoutingRule) ([]string, error) {
	if len(rule.Targets) == 0 {
		if len(r.config.Backends) == 0 {
			return nil, fmt.Errorf("no backends for round-robin")
		}
		out := make([]string, len(r.config.Backends))
		for i := range r.config.Backends {
			out[i] = r.config.Backends[i].Name
		}
		return out, nil
	}
	byName := make(map[string]struct{}, len(r.config.Backends))
	for i := range r.config.Backends {
		byName[r.config.Backends[i].Name] = struct{}{}
	}
	var pool []string
	seen := make(map[string]struct{})
	for _, name := range rule.Targets {
		if _, ok := seen[name]; ok {
			continue
		}
		if _, ok := byName[name]; !ok {
			return nil, fmt.Errorf("unknown backend %q in round-robin targets", name)
		}
		seen[name] = struct{}{}
		pool = append(pool, name)
	}
	if len(pool) == 0 {
		return nil, fmt.Errorf("round-robin: empty pool")
	}
	return pool, nil
}

func (r *Router) backendByName(name string) *config.Backend {
	for i := range r.config.Backends {
		if r.config.Backends[i].Name == name {
			return &r.config.Backends[i]
		}
	}
	return nil
}

// GetBackendURL returns the full URL for a backend
func (r *Router) GetBackendURL(backend *config.Backend) string {
	return fmt.Sprintf("http://%s:%d", backend.Host, backend.Port)
}
