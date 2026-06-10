package config

import "testing"

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		port    int
		origin  string
		wantErr bool
	}{
		{"valid http", "http://localhost:8080", 3001, "*", false},
		{"valid https", "https://api.local", 3001, "*", false},
		{"valid specific origin", "http://localhost:8080", 3001, "192.168.1.5", false},
		{"valid port edge low", "http://localhost:8080", 1, "*", false},
		{"valid port edge high", "http://localhost:8080", 65535, "*", false},
		{"missing scheme", "localhost:8080", 3001, "*", true},
		{"ftp scheme", "ftp://localhost", 3001, "*", true},
		{"empty target", "", 3001, "*", true},
		{"scheme only no host", "http://", 3001, "*", true},
		{"port zero", "http://localhost:8080", 0, "*", true},
		{"port negative", "http://localhost:8080", -1, "*", true},
		{"port too high", "http://localhost:8080", 65536, "*", true},
		{"empty origin", "http://localhost:8080", 3001, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := New(tt.target, tt.port, tt.origin)
			if (err != nil) != tt.wantErr {
				t.Fatalf("New(%q, %d, %q) error = %v, wantErr %v",
					tt.target, tt.port, tt.origin, err, tt.wantErr)
			}
			if !tt.wantErr && cfg.Target.String() != tt.target {
				t.Errorf("Target = %q, want %q", cfg.Target, tt.target)
			}
		})
	}
}
