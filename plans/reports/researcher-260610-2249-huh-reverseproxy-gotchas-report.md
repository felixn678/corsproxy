# Research Report: Charmbracelet/huh & net/http ReverseProxy CORS Proxy

## Executive Summary

Two major findings: **(1)** huh now has v2.0.3 stable (March 2026) with clean API, breaking from v0.x; **(2)** ReverseProxy Rewrite (Go 1.20+) with ModifyResponse callback is the modern pattern for CORS stripping, replacing deprecated Director. WebSocket upgrade is automatic since Go 1.12; preflight OPTIONS are best handled at proxy layer, not forwarded.

---

## Topic 1: charmbracelet/huh Interactive Prompt Library

### Version & Status
- **Latest Stable:** v2.0.3 (released March 2026)
- **Previous Line:** v0.8.0 (Oct 2025, unmaintained)
- **Breaking Change:** v2 moves to vanity import path `charm.land/huh/v2` (away from github.com)
- **Go Requirement:** Go 1.18+ (no explicit min stated, but Bubble Tea v2 baseline)

### Minimal API for Your Use Case

**Field constructors** (chainable, all return error on Run()):
```go
huh.NewInput().
    Title("Port:").
    Value(&port).
    Validate(func(s string) error { /* custom */ }).
    Run()

huh.NewInput().
    Title("Backend URL:").
    Value(&backendURL).
    Validate(func(s string) error { /* URL validation */ }).
    Run()
```

**Built-in validators:**
```go
ValidateNotEmpty()
ValidateMinLength(int)
ValidateMaxLength(int)
ValidateLength(min, max int)
ValidateOneOf(options ...string)
```

**Form (grouped fields):**
```go
form := huh.NewForm(
    huh.NewGroup(
        field1, field2, field3,
    ),
).
err := form.Run()
// Retrieve: form.GetString("key")
```

**Custom validation:** Pass a `func(s string) error` — return nil (valid) or error (show message, retry).

### Key Gotchas
1. **v0 → v2 break:** Don't mix versions. If upgrading existing v0 code, rename imports to `charm.land/huh/v2`.
2. **Accessibility mode exists** — auto-enabled in screen-reader contexts (no config needed).
3. **No default value pre-fill in NewInput** — use `.Value(&var)` after initializing `var` with desired default.

**Recommendation:** Use v2.0.3 for new projects. Port validation from stdlib `url.Parse()`, port validation via range check (1–65535) or regexp.

---

## Topic 2: net/http/httputil.ReverseProxy for CORS Stripping

### Modern Pattern: Rewrite + ModifyResponse (Go 1.20+)

**Struct signature:**
```go
type ReverseProxy struct {
	Rewrite        func(*ProxyRequest)              // Use this, not Director
	Transport      http.RoundTripper
	ModifyResponse func(*http.Response) error     // Strip/inject headers here
	FlushInterval  time.Duration
	ErrorHandler   func(http.ResponseWriter, *http.Request, error)
	ErrorLog       *log.Logger
	BufferPool     BufferPool
	// Director deprecated; don't use
}
```

**Host Header Handling**
- `Rewrite` receives `ProxyRequest` with `In` (read-only client req) and `Out` (modifiable proxy req).
- Default: `Out.Host = In.Host` (pass client's Host header to backend).
- If backend checks Host, set `r.Out.Host = r.In.Host` explicitly **after** `r.SetURL(upstreamURL)`.
- Use `r.SetXForwarded()` to inject X-Forwarded-For, X-Forwarded-Host, X-Forwarded-Proto.

**CORS Header Stripping (Most Important Gotcha)**
```go
proxy := &http.httputil.ReverseProxy{
    Rewrite: func(pr *httputil.ProxyRequest) {
        pr.SetURL(upstreamURL)
        pr.Out.Host = pr.In.Host  // Preserve client Host header
    },
    ModifyResponse: func(res *http.Response) error {
        // Strip upstream CORS headers to avoid duplication
        res.Header.Del("Access-Control-Allow-Origin")
        res.Header.Del("Access-Control-Allow-Credentials")
        res.Header.Del("Access-Control-Allow-Methods")
        res.Header.Del("Access-Control-Allow-Headers")
        res.Header.Del("Access-Control-Max-Age")
        
        // Inject proxy's CORS headers (permissive for dev)
        res.Header.Set("Access-Control-Allow-Origin", "*")
        res.Header.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS,PATCH")
        res.Header.Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
        return nil
    },
}
```

**Why:** Hop-by-hop headers (Connection, Upgrade, etc.) are auto-stripped by ReverseProxy before forwarding. But CORS headers are *end-to-end* — ReverseProxy preserves them. If backend also sets CORS headers, browsers reject responses with duplicate `Access-Control-Allow-Origin`. Delete upstream headers in ModifyResponse, inject proxy's own.

### OPTIONS Preflight Handling

**Best practice for dev proxy:** Handle OPTIONS at proxy, don't forward to backend.
```go
http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    if r.Method == "OPTIONS" {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
        w.WriteHeader(http.StatusNoContent)  // 204
        return
    }
    proxy.ServeHTTP(w, r)
})
```

**Why:** Preflight requests don't follow redirects. If backend redirects HTTP→HTTPS or normalizes paths, the browser fails CORS check silently. Proxy-layer OPTIONS avoids this.

### WebSocket Support

**Automatic:** ReverseProxy handles `101 Switching Protocols` (WebSocket upgrade) automatically since Go 1.12. No custom code needed — Upgrade header and Connection headers are preserved, and the proxy bridges the connection.

**Gotcha:** If you strip Upgrade/Connection headers in Rewrite, WebSocket breaks. Only strip CORS headers, not protocol-upgrade headers.

### Streaming / SSE

Use `FlushInterval`:
```go
proxy.FlushInterval = -1  // Flush immediately after each write (SSE friendly)
```
Zero = no flush, negative = flush immediately, positive = flush every N duration.

### Detecting "Port Already in Use" Error

**Idiomatic Go (Go 1.13+):**
```go
ln, err := net.Listen("tcp", ":8080")
if err != nil {
    var opErr *net.OpError
    if errors.As(err, &opErr) && opErr.Op == "listen" {
        // opErr.Unwrap() may be a syscall.Errno matching EADDRINUSE
        if syscall.EADDRINUSE is opErr.Err's type (platform-dependent)
        // Cross-platform: check error string or use go-libutil
        if strings.Contains(err.Error(), "address already in use") {
            fmt.Println("Port 8080 already in use")
        }
    }
}
```

**Cleaner:** Define a helper or use existing packages. Standard library doesn't export EADDRINUSE directly; check string or syscall module per OS.

### Detecting Local Machine LAN IP

**Net.Dial UDP trick (works, no handshake):**
```go
func getOutboundIP() net.IP {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    
    localAddr := conn.LocalAddr().(*net.UDPAddr)
    return localAddr.IP  // Primary outbound IP
}
```

**Why UDP:** No actual connection to 8.8.8.8; kernel returns the IP it would use. Works for multi-homed machines.

---

## Topic 3: Cobra + Huh Compatibility

**Finding:** No known issues. Cobra (CLI framework) and huh (interactive forms) are orthogonal:
- Cobra parses flags, subcommands, environment variables.
- huh prompts interactively when flags are missing.
- No shared state or conflicts.

**Pattern:** Use Cobra's `Run` to invoke huh forms for missing required inputs.
```go
&cobra.Command{
    Use: "proxy",
    Run: func(cmd *cobra.Command, args []string) {
        if port == "" {
            huh.NewInput().Title("Port:").Value(&port).Run()
        }
        // Start proxy...
    },
}
```

**Cobra latest stable:** v1.8.x (April 2026). v2 in planning; no breaking changes since v1.0.

---

## Recommendation Summary

| Component | Choice | Rationale |
|-----------|--------|-----------|
| **huh** | charm.land/huh/v2@v2.0.3 | Latest stable, clean API, v0 EOL |
| **ReverseProxy** | Rewrite + ModifyResponse | Modern (Go 1.20+), security-first, safer than Director |
| **Preflight** | Proxy-layer OPTIONS | Avoids backend redirects breaking CORS |
| **WebSocket** | Auto (no code) | Built-in since Go 1.12 |
| **Port check** | errors.As + string match | Idiomatic, cross-platform |
| **Cobra** | Latest v1.x | Stable, no huh conflicts |

---

## Unresolved Questions

1. **Port validation library:** Should use stdlib `strconv.ParseUint()` + range check, or custom? (No external dependency needed.)
2. **URL validation in huh:** Should validate URL format with `url.Parse()` or regex? (Recommend `url.Parse()` for robustness.)
3. **Error message locale:** huh displays errors in English; no i18n. Acceptable for dev tool?

---

**Status:** DONE

**Report generated:** 2026-06-10 22:49 UTC

Sources:
- [charmbracelet/huh releases](https://github.com/charmbracelet/huh/releases)
- [charm.land/huh/v2 Go packages](https://pkg.go.dev/charm.land/huh/v2)
- [net/http/httputil ReverseProxy](https://pkg.go.dev/net/http/httputil)
- [Go 1.20 Rewrite proposal](https://github.com/golang/go/issues/53002)
- [CORS preflight best practices](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS)
- [ReverseProxy WebSocket upgrade](https://github.com/golang/go/issues/26937)
- [Go local IP detection](https://gosamples.dev/local-ip-address/)
- [spf13/cobra GitHub](https://github.com/spf13/cobra)
