package proxy

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"syscall"
)

type proxyRetryKey struct{}

// ContextWithProxyRetry attaches a marker to ctx. When the reverse proxy hits a
// retryable transport error, ErrorHandler sets *retry to true and does not write
// a response so the caller can try another backend.
func ContextWithProxyRetry(ctx context.Context, retry *atomic.Bool) context.Context {
	return context.WithValue(ctx, proxyRetryKey{}, retry)
}

// New builds a reverse proxy for target with audience-specific headers and response tuning.
func New(target string, backendName string) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(u)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalHost := req.Host
		originalDirector(req)

		req.Host = u.Host

		log.Printf("Proxying request to: %s%s", u.String(), req.URL.Path)

		if _, ok := req.Header["User-Agent"]; !ok {
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

	proxy.ModifyResponse = func(resp *http.Response) error {
		return ApplyCacheAndVaryHeaders(resp, backendName)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if ab, ok := r.Context().Value(proxyRetryKey{}).(*atomic.Bool); ok && isRetryableProxyError(err) {
			ab.Store(true)
			return
		}
		log.Printf("Proxy error for %s: %v", u.String(), err)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("Backend service unavailable"))
	}

	return proxy, nil
}

func isRetryableProxyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Timeout() {
			return true
		}
		if opErr.Op == "dial" {
			return true
		}
		var errno syscall.Errno
		if errors.As(opErr.Err, &errno) {
			switch errno {
			case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.EPIPE, syscall.ETIMEDOUT:
				return true
			}
		}
	}
	var ue *url.Error
	if errors.As(err, &ue) && ue.Timeout() {
		return true
	}
	return false
}

// ApplyCacheAndVaryHeaders sets cache-control for HTML-like responses and Vary for asset responses.
func ApplyCacheAndVaryHeaders(resp *http.Response, backendName string) error {
	contentType := resp.Header.Get("Content-Type")

	if resp.StatusCode != http.StatusOK || len(contentType) == 0 {
		return nil
	}

	if (len(contentType) >= 9 && contentType[:9] == "text/html") ||
		(len(contentType) >= 24 && contentType[:24] == "application/octet-stream") {
		resp.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		resp.Header.Set("Pragma", "no-cache")
		resp.Header.Set("Expires", "0")
	}

	if (len(contentType) >= 8 && contentType[:8] == "text/css") ||
		(len(contentType) >= 15 && contentType[:15] == "text/javascript") ||
		(len(contentType) >= 22 && contentType[:22] == "application/javascript") {
		resp.Header.Set("X-Backend-Name", backendName)
		vary := resp.Header.Get("Vary")
		if vary == "" {
			resp.Header.Set("Vary", "X-Backend-Name")
		} else {
			resp.Header.Set("Vary", vary+", X-Backend-Name")
		}
	}

	return nil
}
