package router

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/DavidHoenisch/abbie/internal/config"
)

// Router handles request routing based on configuration
type Router struct {
	config       *config.Config
	roundRobinIdx atomic.Uint64
}

// NewRouter creates a new router with the given configuration
func NewRouter(cfg *config.Config) *Router {
	return &Router{
		config: cfg,
	}
}

// SelectBackend selects the appropriate backend based on the routing strategy
func (r *Router) SelectBackend(req *http.Request) (*config.Backend, error) {
	if len(r.config.Backends) == 0 {
		return nil, fmt.Errorf("no backends configured")
	}

	switch r.config.Routing.Strategy {
	case config.RoundRobin:
		return r.selectRoundRobin(), nil
	case config.QueryParam:
		return r.selectByQueryParam(req), nil
	case config.Header:
		return r.selectByHeader(req), nil
	case config.Cookie:
		return r.selectByCookie(req), nil
	case config.Static:
		return &r.config.Backends[0], nil
	default:
		return nil, fmt.Errorf("unknown routing strategy: %s", r.config.Routing.Strategy)
	}
}

// selectRoundRobin implements round-robin load balancing
func (r *Router) selectRoundRobin() *config.Backend {
	idx := r.roundRobinIdx.Add(1) - 1
	return &r.config.Backends[idx%uint64(len(r.config.Backends))]
}

// selectByQueryParam routes based on a query parameter, with cookie fallback
func (r *Router) selectByQueryParam(req *http.Request) *config.Backend {
	paramValue := req.URL.Query().Get(r.config.Routing.ParamName)

	// If no query param is present, check for a cookie with the same name
	if paramValue == "" {
		cookie, err := req.Cookie(r.config.Routing.ParamName)
		if err == nil {
			paramValue = cookie.Value
		}
	}

	return r.findBackendByGroup(paramValue)
}

// selectByHeader routes based on a request header
func (r *Router) selectByHeader(req *http.Request) *config.Backend {
	headerValue := req.Header.Get(r.config.Routing.ParamName)
	return r.findBackendByGroup(headerValue)
}

// selectByCookie routes based on a cookie value
func (r *Router) selectByCookie(req *http.Request) *config.Backend {
	cookie, err := req.Cookie(r.config.Routing.ParamName)
	if err != nil {
		return r.getDefaultBackend()
	}
	return r.findBackendByGroup(cookie.Value)
}

// findBackendByGroup finds a backend that belongs to the specified group
func (r *Router) findBackendByGroup(group string) *config.Backend {
	if group == "" {
		return r.getDefaultBackend()
	}

	// Find backend matching the group
	for i := range r.config.Backends {
		backend := &r.config.Backends[i]
		for _, g := range backend.Groups {
			if g == group {
				return backend
			}
		}
	}

	// No match found, return default
	return r.getDefaultBackend()
}

// getDefaultBackend returns the backend for the default group or the first backend
func (r *Router) getDefaultBackend() *config.Backend {
	if r.config.Routing.DefaultGroup != "" {
		for i := range r.config.Backends {
			backend := &r.config.Backends[i]
			for _, g := range backend.Groups {
				if g == r.config.Routing.DefaultGroup {
					return backend
				}
			}
		}
	}

	// Fallback to first backend
	if len(r.config.Backends) > 0 {
		return &r.config.Backends[0]
	}

	return nil
}

// GetBackendURL returns the full URL for a backend
func (r *Router) GetBackendURL(backend *config.Backend) string {
	return fmt.Sprintf("http://%s:%d", backend.Host, backend.Port)
}
