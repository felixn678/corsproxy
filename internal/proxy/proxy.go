// Package proxy builds the HTTP handler chain of corsproxy:
// logging → CORS → reverse proxy to the backend.
package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/felix-nguyen/corsproxy/internal/config"
)

// New returns the complete handler chain for the given config.
func New(cfg *config.Config) http.Handler {
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(cfg.Target)
			pr.SetXForwarded()
			// Host header is intentionally left as the target's host:
			// forwarding the client's Host (a LAN IP) gets rejected by
			// backends with host checks (Django ALLOWED_HOSTS, Vite, Rails).
		},
		ModifyResponse: func(res *http.Response) error {
			// The proxy owns CORS. Drop the backend's CORS headers so the
			// browser never sees duplicates (it rejects responses with two
			// Access-Control-Allow-Origin values).
			for _, name := range corsHeaderNames {
				res.Header.Del(name)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// CORS headers were already set by corsMiddleware, so the browser
			// reports a real 502 instead of a misleading CORS error.
			fmt.Printf("proxy error: %s %s: %v\n", r.Method, r.URL.Path, err)
			http.Error(w, "corsproxy: backend unreachable: "+err.Error(), http.StatusBadGateway)
		},
		// Flush each write immediately so SSE/streaming responses are not buffered.
		FlushInterval: -1,
	}

	return loggingMiddleware(corsMiddleware(rp, cfg.Origin))
}
