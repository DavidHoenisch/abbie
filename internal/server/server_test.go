package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"testing"

	"github.com/DavidHoenisch/abbie/internal/config"
	"github.com/DavidHoenisch/abbie/internal/proxy"
	"github.com/DavidHoenisch/abbie/internal/router"
)

func TestServer_routesToBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(backend.Close)

	u, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{Strategy: config.Static},
		},
		Backends: []config.Backend{
			{Name: "only", Host: host, Port: port, Groups: nil},
		},
	}
	rtr, err := router.NewRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	proxies := make(map[string]*httputil.ReverseProxy)
	be := cfg.Backends[0]
	bu := rtr.GetBackendURL(&be)
	p, err := proxy.New(bu, be.Name)
	if err != nil {
		t.Fatal(err)
	}
	proxies[be.Name] = p

	srv := New(cfg, rtr, proxies)
	front := httptest.NewServer(srv)
	t.Cleanup(front.Close)

	resp, err := http.Get(front.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

func TestServer_missingProxy(t *testing.T) {
	cfg := sampleThreeBackendConfig()
	rtr, err := router.NewRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	proxies := make(map[string]*httputil.ReverseProxy)

	srv := New(cfg, rtr, proxies)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code: %d", rec.Code)
	}
}

func sampleThreeBackendConfig() *config.Config {
	return &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{Strategy: config.Static},
		},
		Backends: []config.Backend{
			{Name: "a", Host: "h", Port: 1},
		},
	}
}
