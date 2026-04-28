package config

import (
	"os"
	"testing"
)

func TestParse_valid(t *testing.T) {
	data := []byte(`
app:
  port: "9000"
backends:
  - name: a
    host: localhost
    port: 1
    groups: [g1]
routing:
  strategy: static
`)
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.App.Port != "9000" {
		t.Errorf("port: got %q", cfg.App.Port)
	}
	if len(cfg.Backends) != 1 || cfg.Backends[0].Name != "a" {
		t.Errorf("backends: %+v", cfg.Backends)
	}
}

func TestParse_defaultPort(t *testing.T) {
	data := []byte(`
app: {}
backends:
  - name: a
    host: h
    port: 1
routing:
  strategy: static
`)
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.App.Port != "8080" {
		t.Errorf("default port: got %q", cfg.App.Port)
	}
}

func TestParse_noBackends(t *testing.T) {
	data := []byte(`
app: {}
backends: []
routing:
  strategy: static
`)
	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected error for no backends")
	}
}

func TestParse_invalidYAML(t *testing.T) {
	_, err := Parse([]byte("not: yaml: :"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_envPortOverride(t *testing.T) {
	// Use a temp file - can't rely on repo config.yaml in test
	dir := t.TempDir()
	path := dir + "/c.yaml"
	if err := os.WriteFile(path, []byte(`
app:
  port: "1111"
backends:
  - name: x
    host: h
    port: 2
routing:
  strategy: static
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ABBIE_CONFIG", path)
	t.Setenv("ABBIE_PORT", "2222")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Port != "2222" {
		t.Errorf("ABBIE_PORT override: got %q", cfg.App.Port)
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(&Config{
		Backends: []Backend{{Name: "a"}},
		Routing:  RoutingList{{Strategy: Static}},
	}); err != nil {
		t.Errorf("unexpected: %v", err)
	}
	if err := Validate(&Config{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParse_routingList(t *testing.T) {
	data := []byte(`
app: { port: "8080" }
backends:
  - name: a
    host: h
    port: 1
routing:
  - strategy: query-param
    param_name: aud
    default_group: def
  - strategy: round-robin
`)
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.Routing) != 2 || cfg.Routing[0].Strategy != QueryParam || cfg.Routing[1].Strategy != RoundRobin {
		t.Fatalf("routing: %+v", cfg.Routing)
	}
}

func TestParse_rejectsEmptyRoutingList(t *testing.T) {
	data := []byte(`
app: {}
backends:
  - name: a
    host: h
    port: 1
routing: []
`)
	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected error for empty routing list")
	}
}

func TestParse_rejectsRoundRobinUnknownTarget(t *testing.T) {
	data := []byte(`
app: {}
backends:
  - name: a
    host: h
    port: 1
routing:
  - strategy: round-robin
    targets:
      - a
      - missing
`)
	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected error for unknown round-robin target")
	}
}
