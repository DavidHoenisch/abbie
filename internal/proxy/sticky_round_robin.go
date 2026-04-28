package proxy

import (
	"context"
	"net/http"
)

type stickyRoundRobinKey struct{}

// StickyRoundRobinMeta carries cookie name and max-age for sticky round-robin pinning.
// The backend name is applied in ModifyResponse (see ContextWithStickyRoundRobin).
type StickyRoundRobinMeta struct {
	Name   string
	MaxAge int
	Secure bool
}

// ContextWithStickyRoundRobin attaches sticky RR cookie metadata to the request context.
// When present, ModifyResponse adds Set-Cookie with Value=<responding backend name>.
func ContextWithStickyRoundRobin(ctx context.Context, meta *StickyRoundRobinMeta) context.Context {
	if meta == nil || meta.Name == "" {
		return ctx
	}
	return context.WithValue(ctx, stickyRoundRobinKey{}, meta)
}

func stickyRoundRobinMetaFromRequest(req *http.Request) *StickyRoundRobinMeta {
	if req == nil {
		return nil
	}
	v, ok := req.Context().Value(stickyRoundRobinKey{}).(*StickyRoundRobinMeta)
	if !ok || v == nil {
		return nil
	}
	return v
}

func appendStickyRoundRobinSetCookie(resp *http.Response, backendName string) {
	meta := stickyRoundRobinMetaFromRequest(resp.Request)
	if meta == nil {
		return
	}
	maxAge := meta.MaxAge
	if maxAge <= 0 {
		maxAge = 3600
	}
	ck := http.Cookie{
		Name:     meta.Name,
		Value:    backendName,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   meta.Secure,
	}
	resp.Header.Add("Set-Cookie", ck.String())
}
