package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/DavidHoenisch/abbie/internal/config"
)

var (
	settings *config.Config
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
	settings = config.NewConfigFactory()

	// Using Fly.io internal addresses
	proxyA := newProxy("http://landing-page-a.internal:3000")
	proxyB := newProxy("http://localhost:5173")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// (Insert your cookie logic here...)
		group := "B" // hardcoded for brevity

		if group == "B" {
			proxyB.ServeHTTP(w, r)
		} else {
			proxyA.ServeHTTP(w, r)
		}
	})

	log.Printf("Router listening on %s", settings.App.Port)
	log.Fatal(http.ListenAndServe(":"+settings.App.Port, nil))
}
