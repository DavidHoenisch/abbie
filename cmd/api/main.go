package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/DavidHoenisch/abbie/internal/config"
	"github.com/DavidHoenisch/abbie/internal/router"
)

var (
	settings  *config.Config
	appRouter *router.Router
	proxies   map[string]*httputil.ReverseProxy
)

func newProxy(target string) *httputil.ReverseProxy {
	url, _ := url.Parse(target)

	proxy := httputil.NewSingleHostReverseProxy(url)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalHost := req.Host
		originalDirector(req)

		req.Host = url.Host

		log.Printf("Proxying request to: %s%s", url.String(), req.URL.Path)

		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default Go-http-client
			req.Header.Set("User-Agent", "")
		}

		if req.Header.Get("X-Forwarded-Host") == "" {
			req.Header.Set("X-Forwarded-Host", originalHost)
		}

		if req.Header.Get("X-Forwarded-Proto") == "" {
			if req.TLS != nil {
				req.Header.Set("X-Forwarded-Proto", "https")
			} else {
				req.Header.Set("X-Forwarded-Proto", "http")
			}
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error for %s: %v", url.String(), err)
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("Backend service unavailable"))
	}

	return proxy
}

func main() {
	// Parse CLI flags
	configPath := flag.String("config", "", "Path to config file (defaults to config.yaml, can also use ABBIE_CONFIG env var)")
	port := flag.String("port", "", "Port to listen on (overrides config file, can also use ABBIE_PORT env var)")
	flag.Parse()

	// Set config path from flag if provided
	if *configPath != "" {
		os.Setenv("ABBIE_CONFIG", *configPath)
	}

	// Set port from flag if provided
	if *port != "" {
		os.Setenv("ABBIE_PORT", *port)
	}

	// Load configuration
	var err error
	settings, err = config.NewConfigFactory()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize router
	appRouter = router.NewRouter(settings)

	// Create proxies for all configured backends
	proxies = make(map[string]*httputil.ReverseProxy)
	for _, backend := range settings.Backends {
		backendURL := appRouter.GetBackendURL(&backend)
		proxies[backend.Name] = newProxy(backendURL)
		log.Printf("Configured backend: %s -> %s (groups: %v)", backend.Name, backendURL, backend.Groups)
	}

	// Main request handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Select backend based on routing strategy
		backend, err := appRouter.SelectBackend(r)
		if err != nil {
			log.Printf("Error selecting backend: %v", err)
			http.Error(w, "No backends available", http.StatusServiceUnavailable)
			return
		}

		// Get the proxy for the selected backend
		proxy, ok := proxies[backend.Name]
		if !ok {
			log.Printf("Proxy not found for backend: %s", backend.Name)
			http.Error(w, "Backend configuration error", http.StatusInternalServerError)
			return
		}

		log.Printf("Routing request to backend: %s", backend.Name)
		proxy.ServeHTTP(w, r)
	})

	log.Printf("Router listening on %s", settings.App.Port)
	log.Printf("Routing strategy: %s", settings.Routing.Strategy)
	log.Fatal(http.ListenAndServe(":"+settings.App.Port, nil))
}
