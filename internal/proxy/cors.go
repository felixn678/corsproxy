package proxy

import (
	"net/http"
	"net/url"
	"strings"
)

// corsHeaderNames are the end-to-end CORS headers the proxy owns. They are
// stripped from upstream responses (ModifyResponse) so the browser never sees
// two conflicting sets.
var corsHeaderNames = []string{
	"Access-Control-Allow-Origin",
	"Access-Control-Allow-Credentials",
	"Access-Control-Allow-Methods",
	"Access-Control-Allow-Headers",
	"Access-Control-Expose-Headers",
	"Access-Control-Max-Age",
}

const allowedMethods = "GET,POST,PUT,PATCH,DELETE,OPTIONS,HEAD"

// corsMiddleware sets CORS headers on every response and answers preflight
// requests directly with 204 instead of forwarding them: preflights don't
// follow redirects, so a backend redirect would silently fail the CORS check.
func corsMiddleware(next http.Handler, allowedOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, r, allowedOrigin)

		// A real preflight always carries Access-Control-Request-Method.
		// Plain OPTIONS requests are forwarded — the backend may use them.
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// setCORSHeaders decides which Access-Control-* headers the response gets.
//
// With allowedOrigin "*" the request's Origin is echoed back instead of the
// literal "*": browsers reject "Access-Control-Allow-Origin: *" for requests
// sent with credentials (cookies), which would break cookie-auth apps.
func setCORSHeaders(w http.ResponseWriter, r *http.Request, allowedOrigin string) {
	h := w.Header()
	requestOrigin := r.Header.Get("Origin")

	switch {
	case allowedOrigin == "*" && requestOrigin == "":
		// Non-browser client (curl) or same-origin: no credentials concern.
		h.Set("Access-Control-Allow-Origin", "*")
		// Wildcard expose is only valid without credentials.
		h.Set("Access-Control-Expose-Headers", "*")
	case allowedOrigin == "*" || originMatches(allowedOrigin, requestOrigin):
		h.Set("Access-Control-Allow-Origin", requestOrigin)
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Add("Vary", "Origin")
	default:
		// Origin not allowed: send no ACAO header and let the browser block.
		return
	}

	h.Set("Access-Control-Allow-Methods", allowedMethods)
	if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
		h.Set("Access-Control-Allow-Headers", reqHeaders)
	} else {
		h.Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
	}
}

// originMatches reports whether the request's Origin header is allowed by the
// configured value. It matches either the full origin string or just the
// hostname, so users can pass "192.168.1.5" as well as "http://192.168.1.5:3000".
func originMatches(configured, requestOrigin string) bool {
	if requestOrigin == "" {
		return false
	}
	if configured == requestOrigin {
		return true
	}
	return hostnameOf(configured) == hostnameOf(requestOrigin)
}

// hostnameOf extracts the bare hostname from a full origin or a host[:port]
// string. Plain values like "192.168.1.5" must NOT go through url.Parse: it
// would put them in Path, not Host, and matching would silently fail.
func hostnameOf(s string) string {
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil {
			return u.Hostname()
		}
		return ""
	}
	host := s
	// Strip a :port suffix if present (keep IPv6 brackets intact).
	if i := strings.LastIndex(s, ":"); i != -1 && !strings.Contains(s[i:], "]") {
		host = s[:i]
	}
	return strings.Trim(host, "[]")
}
