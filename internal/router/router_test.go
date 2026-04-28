package router

import (
	"net/http"
	"testing"

	"github.com/DavidHoenisch/abbie/internal/config"
)

func sampleCfg(strategy config.RoutingStrategy, param string) *config.Config {
	return &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{
				Strategy:     strategy,
				ParamName:    param,
				DefaultGroup: "def",
			},
		},
		Backends: []config.Backend{
			{Name: "b-def", Host: "h", Port: 1, Groups: []string{"def"}},
			{Name: "b-health", Host: "h", Port: 2, Groups: []string{"health"}},
		},
	}
}

func newTestRouter(tb testing.TB, cfg *config.Config) *Router {
	tb.Helper()
	r, err := NewRouter(cfg)
	if err != nil {
		tb.Fatal(err)
	}
	return r
}

func TestSelectBackend_noBackends(t *testing.T) {
	cfg := sampleCfg(config.Static, "")
	cfg.Backends = nil
	cfg.Routing = nil
	r, err := NewRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.SelectBackend(httptestReq(t, "GET", "/", nil))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSelectBackend_unknownStrategy(t *testing.T) {
	cfg := sampleCfg(config.Static, "")
	cfg.Routing[0].Strategy = config.RoutingStrategy("nope")
	r := newTestRouter(t, cfg)
	_, err := r.SelectBackend(httptestReq(t, "GET", "/", nil))
	if err == nil {
		t.Fatal("expected error from unknown strategy")
	}
}

func TestSelectBackend_static(t *testing.T) {
	r := newTestRouter(t, sampleCfg(config.Static, ""))
	sel, err := r.SelectBackend(httptestReq(t, "GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if sel.Backend.Name != "b-def" {
		t.Fatalf("backend: %+v", sel.Backend)
	}
}

func TestSelectBackend_roundRobin_targetsSubset(t *testing.T) {
	cfg := &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{Strategy: config.RoundRobin, Targets: []string{"b-def", "b-health"}},
		},
		Backends: []config.Backend{
			{Name: "b-def", Host: "h", Port: 1},
			{Name: "b-health", Host: "h", Port: 2},
			{Name: "b-other", Host: "h", Port: 3},
		},
	}
	r := newTestRouter(t, cfg)
	seen := make(map[string]int)
	for i := 0; i < 60; i++ {
		sel, err := r.SelectBackend(httptestReq(t, "GET", "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		seen[sel.Backend.Name]++
		if sel.Backend.Name == "b-other" {
			t.Fatalf("unexpected backend %s", sel.Backend.Name)
		}
	}
	if seen["b-def"] == 0 || seen["b-health"] == 0 {
		t.Fatalf("expected only targets in rotation, got %+v", seen)
	}
}

func TestSelectBackend_roundRobin(t *testing.T) {
	r := newTestRouter(t, sampleCfg(config.RoundRobin, ""))
	seen := make(map[string]int)
	for i := 0; i < 20; i++ {
		sel, err := r.SelectBackend(httptestReq(t, "GET", "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		seen[sel.Backend.Name]++
	}
	if seen["b-def"] == 0 || seen["b-health"] == 0 {
		t.Fatalf("expected rotation, got %+v", seen)
	}
}

func TestSelectBackend_roundRobin_sticky(t *testing.T) {
	cfg := &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{Strategy: config.RoundRobin, StickyCookie: "rr_host", StickyMaxAge: 60},
		},
		Backends: []config.Backend{
			{Name: "b-def", Host: "h", Port: 1},
			{Name: "b-health", Host: "h", Port: 2},
		},
	}
	r := newTestRouter(t, cfg)

	sel1, err := r.SelectBackend(httptestReq(t, "GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if sel1.StickyRoundRobinCookie == nil || sel1.StickyRoundRobinCookie.Value != sel1.Backend.Name {
		t.Fatalf("expected sticky cookie for new assign: %+v", sel1)
	}
	name := sel1.Backend.Name
	for i := 0; i < 30; i++ {
		req := httptestReq(t, "GET", "/", map[string]string{"Cookie": "rr_host=" + name})
		sel, err := r.SelectBackend(req)
		if err != nil {
			t.Fatal(err)
		}
		if sel.Backend.Name != name {
			t.Fatalf("sticky broke at %d: got %s want %s", i, sel.Backend.Name, name)
		}
		if sel.StickyRoundRobinCookie == nil || sel.StickyRoundRobinCookie.Value != name {
			t.Fatal("expected cookie refresh")
		}
	}
	seen := map[string]int{}
	for i := 0; i < 100; i++ {
		sel, err := r.SelectBackend(httptestReq(t, "GET", "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		seen[sel.Backend.Name]++
	}
	if seen["b-def"] == 0 || seen["b-health"] == 0 {
		t.Fatalf("expected global RR for new clients: %+v", seen)
	}
}

func TestSelectBackend_roundRobin_sticky_invalidCookieUsesCounter(t *testing.T) {
	cfg := &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{Strategy: config.RoundRobin, StickyCookie: "rr", StickyMaxAge: 60},
		},
		Backends: []config.Backend{
			{Name: "b-def", Host: "h", Port: 1},
			{Name: "b-health", Host: "h", Port: 2},
		},
	}
	r := newTestRouter(t, cfg)
	req := httptestReq(t, "GET", "/", map[string]string{"Cookie": "rr=not-a-backend"})
	sel, err := r.SelectBackend(req)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Backend.Name != "b-def" && sel.Backend.Name != "b-health" {
		t.Fatalf("backend: %s", sel.Backend.Name)
	}
}

func TestSelectBackend_roundRobin_sticky_defaultMaxAge(t *testing.T) {
	cfg := &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{Strategy: config.RoundRobin, StickyCookie: "rr"},
		},
		Backends: []config.Backend{
			{Name: "b-def", Host: "h", Port: 1},
			{Name: "b-health", Host: "h", Port: 2},
		},
	}
	r := newTestRouter(t, cfg)
	sel, err := r.SelectBackend(httptestReq(t, "GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if sel.StickyRoundRobinCookie.MaxAge != 3600 {
		t.Fatalf("default max age: %d", sel.StickyRoundRobinCookie.MaxAge)
	}
}

func TestSelectBackend_queryParam(t *testing.T) {
	r := newTestRouter(t, sampleCfg(config.QueryParam, "aud"))
	req := httptestReq(t, "GET", "/?aud=health", nil)
	sel, err := r.SelectBackend(req)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Backend.Name != "b-health" {
		t.Fatalf("got %+v", sel.Backend)
	}
}

func TestSelectBackend_queryParam_cookieFallback(t *testing.T) {
	r := newTestRouter(t, sampleCfg(config.QueryParam, "aud"))
	req := httptestReq(t, "GET", "/", map[string]string{"Cookie": "aud=health"})
	sel, err := r.SelectBackend(req)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Backend.Name != "b-health" {
		t.Fatalf("got %+v", sel.Backend)
	}
}

func TestSelectBackend_header(t *testing.T) {
	r := newTestRouter(t, sampleCfg(config.Header, "X-A"))
	req := httptestReq(t, "GET", "/", map[string]string{"X-A": "health"})
	sel, err := r.SelectBackend(req)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Backend.Name != "b-health" {
		t.Fatalf("got %+v", sel.Backend)
	}
}

func TestSelectBackend_cookie(t *testing.T) {
	r := newTestRouter(t, sampleCfg(config.Cookie, "ab"))
	req := httptestReq(t, "GET", "/", map[string]string{"Cookie": "ab=health"})
	sel, err := r.SelectBackend(req)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Backend.Name != "b-health" {
		t.Fatalf("got %+v", sel.Backend)
	}
}

func TestSelectBackend_cookie_missingUsesDefaultGroup(t *testing.T) {
	r := newTestRouter(t, sampleCfg(config.Cookie, "missing"))
	req := httptestReq(t, "GET", "/", nil)
	sel, err := r.SelectBackend(req)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Backend.Name != "b-def" {
		t.Fatalf("got %+v", sel.Backend.Name)
	}
}

func TestSelectBackend_chain_queryThenRR(t *testing.T) {
	cfg := &config.Config{
		App: config.App{Port: "8080"},
		Routing: config.RoutingList{
			{Strategy: config.QueryParam, ParamName: "aud", DefaultGroup: "def"},
			{Strategy: config.RoundRobin},
		},
		Backends: []config.Backend{
			{Name: "b-def", Host: "h", Port: 1, Groups: []string{"def"}},
			{Name: "b-health", Host: "h", Port: 2, Groups: []string{"health"}},
		},
	}
	r := newTestRouter(t, cfg)

	seen := make(map[string]int)
	for i := 0; i < 40; i++ {
		sel, err := r.SelectBackend(httptestReq(t, "GET", "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		seen[sel.Backend.Name]++
	}
	if seen["b-def"] == 0 || seen["b-health"] == 0 {
		t.Fatalf("expected RR when query empty, got %+v", seen)
	}

	req := httptestReq(t, "GET", "/?aud=health", nil)
	sel, err := r.SelectBackend(req)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Backend.Name != "b-health" {
		t.Fatalf("query should match first: %s", sel.Backend.Name)
	}
}

func TestGetBackendURL(t *testing.T) {
	r := newTestRouter(t, &config.Config{
		Routing: config.RoutingList{{Strategy: config.Static}},
		Backends: []config.Backend{
			{Host: "10.0.0.1", Port: 99},
		},
	})
	url := r.GetBackendURL(&r.config.Backends[0])
	if url != "http://10.0.0.1:99" {
		t.Fatalf("got %s", url)
	}
}

func httptestReq(t *testing.T, method, path string, hdr map[string]string) *http.Request {
	t.Helper()
	req := httptestNewRequest(method, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return req
}

func httptestNewRequest(method, url string, body interface{}) *http.Request {
	r, _ := http.NewRequest(method, url, nil)
	return r
}
