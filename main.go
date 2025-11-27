package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

// Helper to create a proxy with proper header handling
func newProxy(target string) *httputil.ReverseProxy {
	url, _ := url.Parse(target)

	proxy := httputil.NewSingleHostReverseProxy(url)

	// The Director modifies the request before it is sent to the backend
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// CRITICAL: Overwrite the Host header to match the target.
		// Many frameworks/servers reject requests if Host doesn't match.
		req.Host = url.Host

		// Set X-Forwarded headers so the backend knows the real IP/Proto
		// Note: Fly.io edge proxies set these, but we append/ensure they pass through
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default Go-http-client
			req.Header.Set("User-Agent", "")
		}
	}

	// Optional: Custom Error Handler (instead of just printing to stderr)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error: %v", err)
		w.WriteHeader(http.StatusBadGateway)
	}

	return proxy
}

func main() {
	// Using Fly.io internal addresses
	proxyA := newProxy("http://landing-page-a.internal:8080")
	proxyB := newProxy("http://landing-page-b.internal:8080")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// (Insert your cookie logic here...)
		group := "B" // hardcoded for brevity

		if group == "B" {
			proxyB.ServeHTTP(w, r)
		} else {
			proxyA.ServeHTTP(w, r)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Router listening on %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
