package server

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
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

func TestServer_stickyRoundRobin_setCookieInResponse(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!doctype html><html></html>"))
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

	const stickyName = "abbie_rr"
	cfg := &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{
				Strategy:     config.RoundRobin,
				Targets:      []string{"only"},
				StickyCookie: stickyName,
				StickyMaxAge: 120,
			},
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
	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == stickyName && c.Value == be.Name {
			found = true
			if c.MaxAge != 120 {
				t.Fatalf("MaxAge: got %d want 120", c.MaxAge)
			}
		}
	}
	if !found {
		t.Fatalf("no sticky cookie; Set-Cookie raw: %q", strings.Join(resp.Header.Values("Set-Cookie"), "; "))
	}
}

func TestServer_roundRobinFallback_nonIdempotentDoesNotRetry(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	var upHits atomic.Int32
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upHits.Add(1)
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

	req, err := http.NewRequest(http.MethodPost, front.URL+"/submit", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status: %d want %d", resp.StatusCode, http.StatusBadGateway)
	}
	if upHits.Load() != 0 {
		t.Fatalf("expected no retry to second backend for POST; hits=%d", upHits.Load())
	}
}

func TestServer_queryParamCookie_secureWhenForwardedHTTPS(t *testing.T) {
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
			{Strategy: config.QueryParam, ParamName: "audience", DefaultGroup: "health"},
		},
		Backends: []config.Backend{
			{Name: "b-health", Host: host, Port: port, Groups: []string{"health"}},
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

	req, err := http.NewRequest(http.MethodGet, front.URL+"/?audience=health", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "audience" {
			found = true
			if !c.Secure {
				t.Fatal("expected query-param cookie to be Secure on forwarded https")
			}
		}
	}
	if !found {
		t.Fatalf("expected audience cookie; Set-Cookie raw: %q", strings.Join(resp.Header.Values("Set-Cookie"), "; "))
	}
}

func TestServer_stickyRoundRobinCookie_secureWhenForwardedHTTPS(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!doctype html><html></html>"))
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

	const stickyName = "abbie_rr_secure"
	cfg := &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{
				Strategy:     config.RoundRobin,
				Targets:      []string{"only"},
				StickyCookie: stickyName,
				StickyMaxAge: 120,
			},
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

	req, err := http.NewRequest(http.MethodGet, front.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == stickyName {
			found = true
			if !c.Secure {
				t.Fatal("expected sticky round-robin cookie to be Secure on forwarded https")
			}
		}
	}
	if !found {
		t.Fatalf("expected sticky cookie; Set-Cookie raw: %q", strings.Join(resp.Header.Values("Set-Cookie"), "; "))
	}
}
