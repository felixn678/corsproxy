---
phase: 3
title: Resume Command
status: completed
priority: P1
effort: 1h
dependencies:
  - 1
  - 2
---

# Phase 3: Resume Command

## Overview

`corsproxy resume [name|uuid]`: with an arg, look up the session and start immediately;
without, show a huh `Select` of saved servers (most recent first, name with uuid
fallback). Reuses Phase 2's start path so resumed servers also bump `lastUsedAt` and
print a fresh hint. Update README.

## Key Insights

- Resume goes through the same `config.New` validation as a fresh start — a stored
  session with a now-invalid target fails with the existing clear error, no special
  casing needed.
- Port-busy on resume needs zero new code: `runServer`'s existing `isAddrInUse` +
  suggest-next-port error fires naturally.
- huh `Select` options carry the full `Session` as the value
  (`huh.NewOption(label, sess)`), so no re-lookup after picking.

## Requirements

- Functional: arg flow, picker flow, sorted by `LastUsedAt` desc, label = name or uuid,
  `→ target  :port` suffix for recognition.
- Non-functional: `cmd/resume.go` < 200 LOC; English-only; no TTY → picker fails with
  the same friendly hint pattern used by the root form.

## Architecture

```go
var resumeCmd = &cobra.Command{
    Use:   "resume [name|uuid]",
    Short: "Restart a previously saved server",
    Args:  cobra.MaximumNArgs(1),
    RunE:  runResume,
}
// init(): rootCmd.AddCommand(resumeCmd)
```

`runResume`:
1. Load store. Corrupt → warn + empty (same handling as Phase 2). Empty →
   `no saved servers yet — start one first: corsproxy --target http://localhost:8080`.
2. Arg given → `session.Find`:
   - `ErrNotFound` → `no saved server matches %q — run "corsproxy resume" to pick from the list`
   - `AmbiguousError` → print each match as `<name>  <uuid>  → <target> :<port>`, then
     `name %q is ambiguous — resume by uuid instead`
3. No arg → sort by LastUsedAt desc, build huh Select:
   - Label: `myapi → http://localhost:8080  :3001`; unnamed → first 8 uuid chars as the
     label head (full uuid is one Tab away in the hint after exit, 8 chars is enough to
     disambiguate visually).
   - Form error (no TTY) → wrap with `(no TTY? use: corsproxy resume <name|uuid>)`.
4. Rebuild config via `config.New(sess.Target, sess.Port, sess.Origin)` → call the same
   start path as Phase 2 (`runServer(cfg, sess.Name)`) so lastUsedAt bumps and the hint
   prints again on exit.

## Related Code Files

- Create: `cmd/resume.go`
- Modify: `README.md` (Usage: naming + resume section, updated banner/exit example)

## Implementation Steps

1. `cmd/resume.go` per architecture above; comment the "why" on going through
   `config.New` again (stale stored config must fail loudly, not mysteriously).
2. Sort: `slices.SortFunc` by `LastUsedAt` descending (Go 1.21+ stdlib, no dep).
3. README: document `--name`, the exit hint, `corsproxy resume` both flows; refresh the
   example session output.
4. Manual smoke matrix:
   - `resume` with empty store → friendly message, exit 1
   - `resume badname` → not-found message
   - two sessions same name → ambiguous listing
   - `resume` no arg → picker shows newest first; picking starts the server
   - resume while port busy → existing suggest-next-port error
5. `go build ./... && go vet ./... && go test ./... -race`

## Todo List

- [ ] resume subcommand (arg + picker flows)
- [ ] Ambiguous/not-found/empty-store messages
- [ ] README updated (naming + resume)
- [ ] Smoke matrix executed
- [ ] Build + vet + tests green

## Success Criteria

- [ ] `corsproxy resume myapi` restarts with the exact saved target/port/origin
- [ ] Picker lists newest-first with names, uuid-prefix fallback for unnamed
- [ ] All edge messages match spec (empty store, not found, ambiguous, port busy)

## Risk Assessment

- **Stored target became invalid** (hand-edited store) → `config.New` rejects with the
  existing validation message; acceptable.
- **huh Select API drift (v2)** → same library version already pinned and used by the
  root form; pattern-match `interactive.go`.

## Security Considerations

Resume executes only locally stored, user-created configs; no remote input.

## Next Steps

Feature complete after this phase → tester + code-review gates, then commit.
