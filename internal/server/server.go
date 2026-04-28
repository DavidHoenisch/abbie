package server

import (
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/DavidHoenisch/abbie/internal/config"
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

	backend, err := s.Router.SelectBackend(r)
	if err != nil {
		log.Printf("Error selecting backend: %v", err)
		http.Error(w, "No backends available", http.StatusServiceUnavailable)
		return
	}

	p, ok := s.Proxies[backend.Name]
	if !ok {
		log.Printf("Proxy not found for backend: %s", backend.Name)
		http.Error(w, "Backend configuration error", http.StatusInternalServerError)
		return
	}

	log.Printf("Routing request to backend: %s", backend.Name)
	p.ServeHTTP(w, r)
}
