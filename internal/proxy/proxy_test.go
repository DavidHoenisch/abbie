package proxy

import (
	"net/http"
	"testing"
)

func TestApplyCacheAndVaryHeaders_html(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
	}
	if err := ApplyCacheAndVaryHeaders(resp, "b1"); err != nil {
		t.Fatal(err)
	}
	if resp.Header.Get("Cache-Control") == "" {
		t.Fatal("expected Cache-Control")
	}
}

func TestApplyCacheAndVaryHeaders_cssVary(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/css"}},
	}
	if err := ApplyCacheAndVaryHeaders(resp, "edge"); err != nil {
		t.Fatal(err)
	}
	if got := resp.Header.Get("X-Backend-Name"); got != "edge" {
		t.Fatalf("X-Backend-Name: got %q", got)
	}
	if got := resp.Header.Get("Vary"); got != "X-Backend-Name" {
		t.Fatalf("Vary: got %q", got)
	}
}

func TestApplyCacheAndVaryHeaders_cssAppendsVary(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/css"},
			"Vary":         []string{"Accept"},
		},
	}
	if err := ApplyCacheAndVaryHeaders(resp, "b"); err != nil {
		t.Fatal(err)
	}
	if got := resp.Header.Get("Vary"); got != "Accept, X-Backend-Name" {
		t.Fatalf("Vary: got %q", got)
	}
}

func TestApplyCacheAndVaryHeaders_nonOK(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"text/html"}},
	}
	if err := ApplyCacheAndVaryHeaders(resp, "b"); err != nil {
		t.Fatal(err)
	}
	if resp.Header.Get("Cache-Control") != "" {
		t.Fatal("should not set cache headers for non-200")
	}
}

func TestNew_badURL(t *testing.T) {
	_, err := New(":", "x")
	if err == nil {
		t.Fatal("expected error")
	}
}
