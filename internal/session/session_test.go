package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixedTime keeps assertions deterministic across the suite.
var fixedTime = time.Date(2026, 6, 11, 7, 0, 0, 0, time.UTC)

func sampleSession(id, name string, port int) Session {
	return Session{
		ID:         id,
		Name:       name,
		Target:     "http://localhost:8080",
		Port:       port,
		Origin:     "*",
		LastUsedAt: fixedTime,
	}
}

func TestStoreSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "sessions.json")
	store := NewStore(path)

	want := []Session{
		sampleSession("id-1", "myapi", 3001),
		sampleSession("id-2", "", 3002), // empty name must survive the trip
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("loaded %d sessions, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("session[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestStoreLoadMissingFileIsEmpty(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "does-not-exist.json"))

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v, want nil error", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d sessions, want empty store", len(got))
	}
}

func TestStoreLoadCorruptFileReturnsErrCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	if err := os.WriteFile(path, []byte(`{"oops`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, err := NewStore(path).Load()
	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("Load error = %v, want ErrCorrupt", err)
	}
}

func TestUpsert(t *testing.T) {
	later := fixedTime.Add(time.Hour)

	tests := []struct {
		name        string
		existing    []Session
		incoming    Session
		wantLen     int
		wantID      string // ID of the stored session returned by Upsert
		wantName    string // Name of the stored session
		wantLastUse time.Time
	}{
		{
			name:        "new entry appended to empty store",
			existing:    nil,
			incoming:    sampleSession("new-id", "myapi", 3001),
			wantLen:     1,
			wantID:      "new-id",
			wantName:    "myapi",
			wantLastUse: fixedTime,
		},
		{
			name:     "same target+port updates in place and keeps existing ID",
			existing: []Session{sampleSession("old-id", "myapi", 3001)},
			incoming: Session{
				ID: "new-id", Name: "renamed", Target: "http://localhost:8080",
				Port: 3001, Origin: "*", LastUsedAt: later,
			},
			wantLen:     1,
			wantID:      "old-id",
			wantName:    "renamed",
			wantLastUse: later,
		},
		{
			name:     "empty incoming name preserves the saved name",
			existing: []Session{sampleSession("old-id", "myapi", 3001)},
			incoming: Session{
				ID: "new-id", Name: "", Target: "http://localhost:8080",
				Port: 3001, Origin: "*", LastUsedAt: later,
			},
			wantLen:     1,
			wantID:      "old-id",
			wantName:    "myapi",
			wantLastUse: later,
		},
		{
			name:        "same target different port appends a second entry",
			existing:    []Session{sampleSession("old-id", "myapi", 3001)},
			incoming:    sampleSession("new-id", "other", 3002),
			wantLen:     2,
			wantID:      "new-id",
			wantName:    "other",
			wantLastUse: fixedTime,
		},
		{
			name:     "same port different target appends a second entry",
			existing: []Session{sampleSession("old-id", "myapi", 3001)},
			incoming: Session{
				ID: "new-id", Name: "other", Target: "http://localhost:9090",
				Port: 3001, Origin: "*", LastUsedAt: fixedTime,
			},
			wantLen:     2,
			wantID:      "new-id",
			wantName:    "other",
			wantLastUse: fixedTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, stored := Upsert(tt.existing, tt.incoming)
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tt.wantLen)
			}
			if stored.ID != tt.wantID {
				t.Errorf("stored.ID = %q, want %q", stored.ID, tt.wantID)
			}
			if stored.Name != tt.wantName {
				t.Errorf("stored.Name = %q, want %q", stored.Name, tt.wantName)
			}
			if !stored.LastUsedAt.Equal(tt.wantLastUse) {
				t.Errorf("stored.LastUsedAt = %v, want %v", stored.LastUsedAt, tt.wantLastUse)
			}
		})
	}
}

func TestFind(t *testing.T) {
	sessions := []Session{
		sampleSession("id-1", "myapi", 3001),
		sampleSession("id-2", "", 3002),
		sampleSession("id-3", "dup", 3003),
		sampleSession("id-4", "dup", 3004),
	}

	tests := []struct {
		name    string
		key     string
		wantID  string
		wantErr error // nil means success; checked via errors.Is
	}{
		{"by exact uuid", "id-2", "id-2", nil},
		{"by name", "myapi", "id-1", nil},
		{"by name case-insensitive", "MyAPI", "id-1", nil},
		{"not found", "ghost", "", ErrNotFound},
		{"empty key not found", "", "", ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Find(sessions, tt.key)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Find: %v", err)
			}
			if got.ID != tt.wantID {
				t.Errorf("got.ID = %q, want %q", got.ID, tt.wantID)
			}
		})
	}

	t.Run("ambiguous name lists all matches", func(t *testing.T) {
		_, err := Find(sessions, "dup")
		var ambig *AmbiguousError
		if !errors.As(err, &ambig) {
			t.Fatalf("err = %v, want *AmbiguousError", err)
		}
		if len(ambig.Matches) != 2 {
			t.Errorf("matches = %d, want 2", len(ambig.Matches))
		}
	})
}
