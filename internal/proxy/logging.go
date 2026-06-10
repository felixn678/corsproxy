package proxy

import (
	"fmt"
	"net/http"
	"time"
)

// statusRecorder wraps http.ResponseWriter to capture the status code for the
// request log line.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush forwards to the underlying writer so ReverseProxy's FlushInterval
// keeps working for SSE/streaming responses.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap exposes the underlying writer to http.ResponseController. Without it
// the controller can't reach Hijacker and WebSocket upgrades fail with 502.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// loggingMiddleware prints one line per request: "GET /api/users → 200 (23ms)".
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Default 200: handlers may Write without calling WriteHeader.
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		// Deferred so the line is printed even when the handler panics
		// (e.g. http.ErrAbortHandler when the client disconnects).
		defer func() {
			fmt.Printf("%s %s → %d (%s)\n",
				r.Method, r.URL.Path, rec.status,
				time.Since(start).Round(time.Millisecond))
		}()

		next.ServeHTTP(rec, r)
	})
}
