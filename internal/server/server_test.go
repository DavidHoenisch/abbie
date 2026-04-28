package server

import (
	"io"
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

func TestServer_roundRobinFallback_unreachableFirst(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("from-good"))
	}))
	t.Cleanup(good.Close)

	gu, err := url.Parse(good.URL)
	if err != nil {
		t.Fatal(err)
	}
	goodHost, goodPortStr, err := net.SplitHostPort(gu.Host)
	if err != nil {
		t.Fatal(err)
	}
	goodPort, err := strconv.Atoi(goodPortStr)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{Strategy: config.RoundRobin, Targets: []string{"b-down", "b-up"}},
		},
		Backends: []config.Backend{
			{Name: "b-down", Host: "127.0.0.1", Port: closedPort},
			{Name: "b-up", Host: goodHost, Port: goodPort},
		},
	}
	rtr, err := router.NewRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	proxies := make(map[string]*httputil.ReverseProxy)
	for i := range cfg.Backends {
		be := &cfg.Backends[i]
		bu := rtr.GetBackendURL(be)
		p, err := proxy.New(bu, be.Name)
		if err != nil {
			t.Fatal(err)
		}
		proxies[be.Name] = p
	}

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
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "from-good" {
		t.Fatalf("body: %q", body)
	}
}

func TestServer_roundRobinFallback_allUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p1 := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p2 := ln2.Addr().(*net.TCPAddr).Port
	ln2.Close()

	cfg := &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{Strategy: config.RoundRobin, Targets: []string{"b-a", "b-b"}},
		},
		Backends: []config.Backend{
			{Name: "b-a", Host: "127.0.0.1", Port: p1},
			{Name: "b-b", Host: "127.0.0.1", Port: p2},
		},
	}
	rtr, err := router.NewRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	proxies := make(map[string]*httputil.ReverseProxy)
	for i := range cfg.Backends {
		be := &cfg.Backends[i]
		bu := rtr.GetBackendURL(be)
		p, err := proxy.New(bu, be.Name)
		if err != nil {
			t.Fatal(err)
		}
		proxies[be.Name] = p
	}

	srv := New(cfg, rtr, proxies)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code: %d want %d", rec.Code, http.StatusBadGateway)
	}
}
