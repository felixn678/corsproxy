package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felix-nguyen/corsproxy/internal/config"
)

// newTestProxy wires the full handler chain against a real httptest backend.
func newTestProxy(t *testing.T, backend *httptest.Server, origin string) http.Handler {
	t.Helper()
	cfg, err := config.New(backend.URL, 3001, origin)
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	return New(cfg)
}

func TestProxyForwardsBody(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello from backend"))
	}))
	defer backend.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Origin", "http://192.168.1.5:3000")

	newTestProxy(t, backend, "*").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "hello from backend" {
		t.Errorf("body = %q, want backend body intact", body)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://192.168.1.5:3000" {
		t.Errorf("ACAO = %q, want echoed origin", got)
	}
}

func TestProxyStripsUpstreamCORSHeaders(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Backend sets its own CORS headers — the proxy must own CORS,
		// otherwise the browser sees duplicates and rejects the response.
		w.Header().Set("Access-Control-Allow-Origin", "http://other-site.example")
		w.Header().Set("Access-Control-Allow-Credentials", "false")
		w.Write([]byte("ok"))
	}))
	defer backend.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://192.168.1.5:3000")

	newTestProxy(t, backend, "*").ServeHTTP(rec, req)

	acao := rec.Header().Values("Access-Control-Allow-Origin")
	if len(acao) != 1 {
		t.Fatalf("got %d ACAO headers (%v), want exactly 1", len(acao), acao)
	}
	if acao[0] != "http://192.168.1.5:3000" {
		t.Errorf("ACAO = %q, want proxy's value, not upstream's", acao[0])
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("credentials = %q, want proxy's %q", got, "true")
	}
}

func TestProxyBackendDownReturns502WithCORS(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	backend.Close() // dead backend: connection refused

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://192.168.1.5:3000")

	newTestProxy(t, backend, "*").ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	// Without CORS headers on the error, the browser hides the 502 behind a
	// misleading CORS error — the exact confusion this tool exists to remove.
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("502 response missing Access-Control-Allow-Origin")
	}
}
