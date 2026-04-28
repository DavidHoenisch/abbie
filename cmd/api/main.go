package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/DavidHoenisch/abbie/internal/config"
	"github.com/DavidHoenisch/abbie/internal/proxy"
	"github.com/DavidHoenisch/abbie/internal/router"
	"github.com/DavidHoenisch/abbie/internal/server"
)

func main() {
	configPath := flag.String("config", "", "Path to config file (defaults to config.yaml, can also use ABBIE_CONFIG env var)")
	port := flag.String("port", "", "Port to listen on (overrides config file, can also use ABBIE_PORT env var)")
	flag.Parse()

	if *configPath != "" {
		os.Setenv("ABBIE_CONFIG", *configPath)
	}
	if *port != "" {
		os.Setenv("ABBIE_PORT", *port)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	rtr, err := router.NewRouter(cfg)
	if err != nil {
		log.Fatalf("Router: %v", err)
	}

	proxies := make(map[string]*httputil.ReverseProxy)
	for _, backend := range cfg.Backends {
		backendURL := rtr.GetBackendURL(&backend)
		p, err := proxy.New(backendURL, backend.Name)
		if err != nil {
			log.Fatalf("Invalid backend URL for %s: %v", backend.Name, err)
		}
		proxies[backend.Name] = p
		log.Printf("Configured backend: %s -> %s (groups: %v)", backend.Name, backendURL, backend.Groups)
	}

	srv := server.New(cfg, rtr, proxies)
	http.Handle("/", srv)

	log.Printf("Router listening on %s", cfg.App.Port)
	for i, rule := range cfg.Routing {
		log.Printf("Routing rule [%d]: %s", i, rule.Strategy)
	}
	rs := cfg.State.Redis
	if rs.URLEnv != "" || rs.Addr != "" {
		if rs.URLEnv != "" {
			log.Printf("Round-robin state: redis connection string from env %q", rs.URLEnv)
		} else {
			log.Printf("Round-robin state: redis at %s (prefix %q)", rs.Addr, rs.KeyPrefix)
		}
	} else {
		log.Printf("Round-robin state: in-memory (per process)")
	}
	log.Fatal(http.ListenAndServe(":"+cfg.App.Port, nil))
}
