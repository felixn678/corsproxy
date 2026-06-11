package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrCorrupt marks a store file containing invalid JSON. Callers should warn
// and continue with an empty store — a broken store must never prevent the
// proxy from running.
var ErrCorrupt = errors.New("session store is corrupt")

// DefaultPath returns the per-user store location. os.UserConfigDir already
// honors $XDG_CONFIG_HOME on Linux and platform conventions elsewhere, so no
// manual env parsing is needed.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot locate user config dir: %w", err)
	}
	return filepath.Join(dir, "corsproxy", "sessions.json"), nil
}

// Store reads and writes the sessions file at a fixed path.
type Store struct{ path string }

// NewStore returns a Store backed by the given file path.
func NewStore(path string) *Store { return &Store{path: path} }

// Load returns all saved sessions. A missing file is an empty store, not an
// error. Invalid JSON returns an ErrCorrupt-wrapped error so callers can
// recover by treating the store as empty.
func (s *Store) Load() ([]Session, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sessions []Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return sessions, nil
}

// Save writes sessions atomically: write a temp file in the same directory,
// then rename it over the target. Rename within one directory is atomic on
// POSIX, so a concurrent reader never sees a half-written file. Two instances
// saving at once means last-writer-wins — acceptable for a dev tool.
func (s *Store) Save(sessions []Session) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "sessions-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// CreateTemp uses 0o600; widen to the conventional config-file mode.
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}
