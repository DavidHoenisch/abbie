package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

func newProxy(target string) *httputil.ReverseProxy {
	url, _ := url.Parse(target)

	proxy := httputil.NewSingleHostReverseProxy(url)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.Host = url.Host

		if _, ok := req.Header["User-Agent"]; !ok {
			req.Header.Set("User-Agent", "")
		}
	}

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
