package server

import (
	"log"
	"net/http"
	"net/http/httputil"
	"sync/atomic"

	"github.com/DavidHoenisch/abbie/internal/config"
	"github.com/DavidHoenisch/abbie/internal/proxy"
	"github.com/DavidHoenisch/abbie/internal/router"
)

// Server routes incoming requests to configured reverse proxies.
type Server struct {
	Settings *config.Config
	Router   *router.Router
	Proxies  map[string]*httputil.ReverseProxy
}

// New returns an HTTP handler that applies routing and forwards to backends.
func New(cfg *config.Config, rtr *router.Router, proxies map[string]*httputil.ReverseProxy) *Server {
	return &Server{
		Settings: cfg,
		Router:   rtr,
		Proxies:  proxies,
	}
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, rule := range s.Settings.Routing {
		if rule.Strategy != config.QueryParam || rule.ParamName == "" {
			continue
		}
		if paramValue := r.URL.Query().Get(rule.ParamName); paramValue != "" {
			cookie := &http.Cookie{
				Name:     rule.ParamName,
				Value:    paramValue,
				Path:     "/",
				MaxAge:   3600,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			}
			http.SetCookie(w, cookie)
			log.Printf("Set cookie %s=%s for persistent routing", rule.ParamName, paramValue)
		}
	}

	sel, err := s.Router.SelectBackend(r)
	if err != nil {
		log.Printf("Error selecting backend: %v", err)
		http.Error(w, "No backends available", http.StatusServiceUnavailable)
		return
	}

	if len(sel.RoundRobinPool) > 0 {
		served, ok := s.serveRoundRobinWithFallback(w, r, sel)
		if !ok {
			log.Printf("Round-robin: all backends in pool failed for request %s %s", r.Method, r.URL.Path)
			http.Error(w, "Backend service unavailable", http.StatusBadGateway)
			return
		}
		if sel.StickyRoundRobinCookie != nil {
			ck := *sel.StickyRoundRobinCookie
			ck.Value = served
			http.SetCookie(w, &ck)
			log.Printf("Set sticky round-robin cookie %s=%s", ck.Name, ck.Value)
		}
		return
	}

	if sel.StickyRoundRobinCookie != nil {
		http.SetCookie(w, sel.StickyRoundRobinCookie)
		log.Printf("Set sticky round-robin cookie %s=%s", sel.StickyRoundRobinCookie.Name, sel.StickyRoundRobinCookie.Value)
	}

	backend := sel.Backend
	p, ok := s.Proxies[backend.Name]
	if !ok {
		log.Printf("Proxy not found for backend: %s", backend.Name)
		http.Error(w, "Backend configuration error", http.StatusInternalServerError)
		return
	}

	log.Printf("Routing request to backend: %s", backend.Name)
	p.ServeHTTP(w, r)
}

func (s *Server) serveRoundRobinWithFallback(w http.ResponseWriter, r *http.Request, sel *router.Selection) (servedBackend string, ok bool) {
	pool := sel.RoundRobinPool
	start := sel.Backend.Name
	startIdx := 0
	for i, name := range pool {
		if name == start {
			startIdx = i
			break
		}
	}

	for i := 0; i < len(pool); i++ {
		name := pool[(startIdx+i)%len(pool)]
		p, found := s.Proxies[name]
		if !found {
			log.Printf("Round-robin fallback: no proxy for backend %q", name)
			continue
		}

		var retry atomic.Bool
		ctx := proxy.ContextWithProxyRetry(r.Context(), &retry)
		r2 := r.Clone(ctx)
		log.Printf("Routing request to backend: %s", name)
		p.ServeHTTP(w, r2)
		if retry.Load() {
			log.Printf("Round-robin fallback: backend %s unavailable, trying next", name)
			continue
		}
		return name, true
	}
	return "", false
}
