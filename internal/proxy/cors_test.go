package proxy

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/felix-nguyen/corsproxy/internal/config"
)

// backendWithCounter returns a live backend and a hit counter, to prove
// whether a request reached it or was answered at the proxy layer.
func backendWithCounter(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte("backend"))
	}))
	t.Cleanup(backend.Close)
	return backend, &hits
}

func doRequest(t *testing.T, backend *httptest.Server, origin string, build func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	cfg, err := config.New(backend.URL, 3001, origin)
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	build(req)
	New(cfg).ServeHTTP(rec, req)
	return rec
}

func TestPreflightAnsweredAtProxyNotForwarded(t *testing.T) {
	backend, hits := backendWithCounter(t)

	rec := doRequest(t, backend, "*", func(r *http.Request) {
		r.Method = http.MethodOptions
		r.Header.Set("Origin", "http://192.168.1.5:3000")
		r.Header.Set("Access-Control-Request-Method", "POST")
		r.Header.Set("Access-Control-Request-Headers", "X-Custom-Token")
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if hits.Load() != 0 {
		t.Errorf("backend hits = %d, preflight must not be forwarded", hits.Load())
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "X-Custom-Token" {
		t.Errorf("Allow-Headers = %q, want echoed request headers", got)
	}
}

func TestPlainOptionsForwardedToBackend(t *testing.T) {
	backend, hits := backendWithCounter(t)

	// No Access-Control-Request-Method → not a preflight; the backend may
	// legitimately serve OPTIONS itself.
	doRequest(t, backend, "*", func(r *http.Request) {
		r.Method = http.MethodOptions
	})

	if hits.Load() != 1 {
		t.Errorf("backend hits = %d, plain OPTIONS must be forwarded", hits.Load())
	}
}

func TestWildcardEchoesOriginWithCredentials(t *testing.T) {
	backend, _ := backendWithCounter(t)

	rec := doRequest(t, backend, "*", func(r *http.Request) {
		r.Header.Set("Origin", "http://192.168.1.5:3000")
	})

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://192.168.1.5:3000" {
		t.Errorf("ACAO = %q, want echoed origin (literal * breaks credentialed fetch)", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("credentials = %q, want true", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin", got)
	}
}

func TestWildcardWithoutOriginHeaderStaysLiteral(t *testing.T) {
	backend, _ := backendWithCounter(t)

	// curl / same-origin requests carry no Origin header.
	rec := doRequest(t, backend, "*", func(r *http.Request) {})

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("ACAO = %q, want literal *", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("credentials = %q, spec forbids credentials with literal *", got)
	}
}

func TestSpecificOriginMatching(t *testing.T) {
	tests := []struct {
		name          string
		configured    string
		requestOrigin string
		wantAllowed   bool
	}{
		// Regression: url.Parse("192.168.1.5") puts the value in Path, not
		// Host — naive parsing makes scheme-less config silently never match.
		{"scheme-less IP matches full origin", "192.168.1.5", "http://192.168.1.5:3000", true},
		{"full origin exact match", "http://192.168.1.5:3000", "http://192.168.1.5:3000", true},
		{"hostname match ignores port", "http://192.168.1.5:9999", "http://192.168.1.5:3000", true},
		{"different host rejected", "192.168.1.5", "http://evil.example", false},
		{"no origin header rejected", "192.168.1.5", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, _ := backendWithCounter(t)
			rec := doRequest(t, backend, tt.configured, func(r *http.Request) {
				if tt.requestOrigin != "" {
					r.Header.Set("Origin", tt.requestOrigin)
				}
			})

			acao := rec.Header().Get("Access-Control-Allow-Origin")
			if tt.wantAllowed && acao != tt.requestOrigin {
				t.Errorf("ACAO = %q, want %q", acao, tt.requestOrigin)
			}
			if !tt.wantAllowed && acao != "" {
				t.Errorf("ACAO = %q, want unset for disallowed origin", acao)
			}
		})
	}
}
