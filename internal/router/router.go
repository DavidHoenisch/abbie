package router

import (
	"fmt"
	"net/http"

	"github.com/DavidHoenisch/abbie/internal/config"
	"github.com/DavidHoenisch/abbie/internal/state"
)

// Selection is the outcome of routing a single request.
type Selection struct {
	Backend *config.Backend
	// StickyRoundRobinCookie is non-nil when round-robin used sticky_cookie; the server should SetCookie.
	StickyRoundRobinCookie *http.Cookie
	// RoundRobinPool is the ordered round-robin pool for this hop (copy of targets or all backends).
	// When non-empty, the server may try the next backend on proxy transport failure.
	RoundRobinPool []string
}

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
func (r *Router) SelectBackend(req *http.Request) (*Selection, error) {
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
		sel, skip, err := r.applyRule(&rules[i], req, isLast)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		if sel != nil && sel.Backend != nil {
			return sel, nil
		}
	}
	return nil, fmt.Errorf("no backend selected")
}

func (r *Router) applyRule(rule *config.RoutingRule, req *http.Request, isLast bool) (sel *Selection, tryNextRule bool, err error) {
	switch rule.Strategy {
	case config.RoundRobin:
		names, err := r.roundRobinOrderedNames(rule)
		if err != nil {
			return nil, false, err
		}
		pool := append([]string(nil), names...)
		if rule.StickyCookie != "" {
			if ck, cerr := req.Cookie(rule.StickyCookie); cerr == nil && ck.Value != "" {
				if b := r.backendInPool(ck.Value, names); b != nil {
					return &Selection{
						Backend:                b,
						StickyRoundRobinCookie: r.stickyRoundRobinCookie(rule, b.Name),
						RoundRobinPool:         pool,
					}, false, nil
				}
			}
			idx, err := r.rr.NextPool(names)
			if err != nil {
				return nil, false, err
			}
			b := r.backendByName(names[idx])
			if b == nil {
				return nil, false, fmt.Errorf("round-robin: no backend %q", names[idx])
			}
			return &Selection{
				Backend:                b,
				StickyRoundRobinCookie: r.stickyRoundRobinCookie(rule, b.Name),
				RoundRobinPool:         pool,
			}, false, nil
		}
		idx, err := r.rr.NextPool(names)
		if err != nil {
			return nil, false, err
		}
		b := r.backendByName(names[idx])
		if b == nil {
			return nil, false, fmt.Errorf("round-robin: no backend %q", names[idx])
		}
		return &Selection{Backend: b, RoundRobinPool: pool}, false, nil

	case config.Static:
		return &Selection{Backend: &r.config.Backends[0]}, false, nil

	case config.QueryParam:
		v := req.URL.Query().Get(rule.ParamName)
		if v == "" {
			if ck, cerr := req.Cookie(rule.ParamName); cerr == nil {
				v = ck.Value
			}
		}
		if v == "" {
			if isLast {
				return &Selection{Backend: r.getDefaultBackend(rule)}, false, nil
			}
			return nil, true, nil
		}
		return &Selection{Backend: r.findBackendByGroup(v, rule)}, false, nil

	case config.Header:
		v := req.Header.Get(rule.ParamName)
		if v == "" {
			if isLast {
				return &Selection{Backend: r.getDefaultBackend(rule)}, false, nil
			}
			return nil, true, nil
		}
		return &Selection{Backend: r.findBackendByGroup(v, rule)}, false, nil

	case config.Cookie:
		ck, cerr := req.Cookie(rule.ParamName)
		if cerr != nil || ck.Value == "" {
			if isLast {
				return &Selection{Backend: r.getDefaultBackend(rule)}, false, nil
			}
			return nil, true, nil
		}
		return &Selection{Backend: r.findBackendByGroup(ck.Value, rule)}, false, nil

	default:
		return nil, false, fmt.Errorf("unknown routing strategy: %s", rule.Strategy)
	}
}

func (r *Router) stickyRoundRobinCookie(rule *config.RoutingRule, backendName string) *http.Cookie {
	maxAge := rule.StickyMaxAge
	if maxAge <= 0 {
		maxAge = 3600
	}
	return &http.Cookie{
		Name:     rule.StickyCookie,
		Value:    backendName,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

func (r *Router) backendInPool(name string, pool []string) *config.Backend {
	for _, n := range pool {
		if n == name {
			return r.backendByName(name)
		}
	}
	return nil
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
