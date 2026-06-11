---
phase: 1
title: Session Store
status: completed
priority: P1
effort: 1h
dependencies: []
---

# Phase 1: Session Store

## Overview

Build `internal/session`: the Session model, pure Upsert/Find logic, and a JSON store
with atomic writes at `os.UserConfigDir()/corsproxy/sessions.json`. Fully unit-tested;
no CLI wiring yet.

## Key Insights

- Pure functions (`Upsert`, `Find`) over slices = trivially table-driven-testable without
  touching the filesystem; only `Load`/`Save` need `t.TempDir()`.
- `os.UserConfigDir()` (stdlib) already honors `$XDG_CONFIG_HOME` on Linux — do NOT
  hand-roll env parsing.
- Atomic save = write to `os.CreateTemp(dir, ...)` then `os.Rename` onto the target.
  Rename within the same directory is atomic on POSIX; prevents a half-written JSON when
  two instances exit near-simultaneously. Last-writer-wins is acceptable for a dev tool.

## Requirements

- Functional: persist sessions as a JSON array; dedupe by target+port; find by uuid or
  name; survive missing/corrupt store files.
- Non-functional: zero panics on bad input; `session.go` + `store.go` each well under
  200 LOC; English-only code and messages.

## Architecture

```go
// session.go — model + pure logic
type Session struct {
    ID         string    `json:"id"`             // uuid, generated at first save
    Name       string    `json:"name,omitempty"` // optional user label
    Target     string    `json:"target"`         // backend URL as string
    Port       int       `json:"port"`
    Origin     string    `json:"origin"`
    LastUsedAt time.Time `json:"lastUsedAt"`
}

// Upsert deduplicates by (Target, Port). On match: update LastUsedAt + Origin,
// keep the existing ID, and overwrite Name ONLY if incoming Name != "".
// On no match: append (caller pre-fills ID via uuid.NewString()).
func Upsert(sessions []Session, incoming Session) []Session

// Find resolves key: exact ID match first, then case-insensitive Name match.
// Errors: ErrNotFound; ErrAmbiguous carries all matches so the caller can list them.
func Find(sessions []Session, key string) (Session, error)

// store.go — IO
func DefaultPath() (string, error)              // UserConfigDir()/corsproxy/sessions.json
type Store struct{ path string }
func NewStore(path string) *Store
func (s *Store) Load() ([]Session, error)       // missing file → (nil, nil)
func (s *Store) Save(sessions []Session) error  // MkdirAll + temp + rename
```

- Corrupt JSON: `Load` returns a wrapped sentinel (e.g. `ErrCorrupt`) so callers can
  warn-and-continue with an empty slice instead of crashing.
- `ErrAmbiguous` should expose the matching sessions (custom error type with a
  `Matches []Session` field) — Phase 3 prints them.

## Related Code Files

- Create: `internal/session/session.go`, `internal/session/store.go`,
  `internal/session/session_test.go`
- Modify: `go.mod` (+ `github.com/google/uuid`)

## Implementation Steps

1. `go get github.com/google/uuid`
2. `session.go`: Session struct, `Upsert`, `Find`, `ErrNotFound`, `ErrCorrupt`,
   `AmbiguousError` type. Comment the "why" on name-preservation in Upsert.
3. `store.go`: `DefaultPath`, `NewStore`, `Load` (missing → empty; bad JSON → ErrCorrupt
   wrap), `Save` (MkdirAll 0o755, CreateTemp in same dir, write, chmod 0o644, rename).
4. `session_test.go` — table-driven, `t.TempDir()`, no hand-rolled mocks:
   - Save → Load round-trip preserves all fields incl. zero/empty Name
   - Upsert: new entry appended; same target+port updates LastUsedAt and keeps ID;
     name preserved when incoming Name is empty; name overwritten when provided
   - Same target different port → two entries (and vice versa)
   - Find: by exact uuid; by name case-insensitive; not found; ambiguous (two sessions
     same name) returns AmbiguousError with both matches
   - Load on missing file → empty, no error; Load on corrupt file (`{"oops`) → ErrCorrupt
5. `go build ./... && go vet ./... && go test ./internal/session/ -race`

## Todo List

- [ ] Add uuid dependency
- [ ] session.go (model + pure logic)
- [ ] store.go (Load/Save atomic)
- [ ] session_test.go table-driven suite
- [ ] Build + vet + race tests green

## Success Criteria

- [ ] All listed test cases pass with `-race`
- [ ] Corrupt store file never returns a panic or hard failure from Load (sentinel error)
- [ ] Save is atomic (temp file + rename, same directory)

## Risk Assessment

- **Two instances saving concurrently** → atomic rename means last-writer-wins; document
  in a comment, acceptable for a dev tool.
- **`os.UserConfigDir` error (rare: no HOME)** → propagate; Phase 2 treats it as
  warn-and-skip persistence.

## Security Considerations

Store contains only local dev URLs/ports — no secrets. File mode 0o644, dir 0o755.

## Next Steps

Phase 2 wires this store into start-up; Phase 3 consumes `Find`/picker.
