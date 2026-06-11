---
phase: 2
title: Naming & Auto-Save
status: completed
priority: P1
effort: 45m
dependencies:
  - 1
---

# Phase 2: Naming & Auto-Save

## Overview

Wire the store into normal start-up: `--name` flag, "Name (optional)" field in the huh
form, auto-save every successful start (dedupe via Upsert), and print the resume hint
after graceful shutdown.

## Key Insights

- Save must happen **after `net.Listen` succeeds** — a port-conflict failure must not
  pollute the store. That means the glue lives around `runServer`, not before it.
- Persistence failures (no HOME, read-only disk) must never kill the proxy — the
  feature is a convenience layer. Warn to stderr and continue.
- `config.Config` stays untouched: name/uuid are session concerns, not proxy config
  (verified separation in plan Key Decision 8).

## Requirements

- Functional: `corsproxy --target ... --name myapi` saves/updates a session; interactive
  mode asks for an optional name; exit prints `Resume later with: corsproxy resume <key>`.
- Non-functional: name field optional with no validation beyond trimming spaces;
  English-only strings; each modified file stays < 200 LOC.

## Architecture

Flow on every start (flags or interactive):

```
config.New OK
  └─ runServer(cfg, name)
       ├─ net.Listen OK
       ├─ sess := persistSession(cfg, name)   // load → upsert → save; warns, never fails
       ├─ banner (+ "Name: myapi" line when named)
       ├─ serve ... Ctrl+C ... Shutdown
       └─ print "Resume later with: corsproxy resume <sess.Name or sess.ID>"
```

`persistSession` (new helper, `cmd/persist.go` if root.go nears 200 LOC, else inline in
server.go): builds `session.Session{Target: cfg.Target.String(), Port, Origin, Name,
LastUsedAt: time.Now(), ID: uuid.NewString()}`, loads store (ErrCorrupt → warn + empty),
calls `session.Upsert`, saves. Returns the stored session (post-upsert, so the ID is the
*existing* one on dedupe — needed for a correct resume hint).

**Subtlety:** Upsert keeps the existing ID on dedupe, so `persistSession` must return the
session as stored, not the freshly-built one. Easiest: have `Upsert` return
`([]Session, Session)` or re-`Find` after upsert — pick one in implementation, test it.

## Related Code Files

- Create: none expected (`cmd/persist.go` only if LOC pressure)
- Modify: `cmd/root.go` (flag `--name`, pass through), `cmd/interactive.go` (4th input),
  `cmd/server.go` (persist call + hint print)

## Implementation Steps

1. `root.go`: add `flagName string` + `--name` flag (`name this server so you can resume
   it later`); pass to `runServer(cfg, flagName)`.
2. `interactive.go`: append 4th `huh.NewInput()` — Title "Name (optional)", Description
   "Label this server for resume (leave empty to use a generated id)". No validator.
3. `server.go`: after successful `net.Listen`, call `persistSession`; print hint after
   clean `Shutdown` (skip on serve error path). Trim/normalize name once here.
4. Banner: add `Name: <name>` line when named (keeps output self-explanatory).
5. Manual smoke: start twice with same target+port (second run without `--name`) →
   `sessions.json` has ONE entry, name preserved, lastUsedAt bumped.
6. `go build ./... && go vet ./... && go test ./... -race`

## Todo List

- [ ] `--name` flag + plumbing
- [ ] Interactive form name field
- [ ] persistSession glue (warn-only on store errors)
- [ ] Resume hint on clean shutdown
- [ ] Smoke: dedupe + name preservation verified against real sessions.json
- [ ] Build + vet + tests green

## Success Criteria

- [ ] Start → Ctrl+C prints `Resume later with: corsproxy resume myapi` (or uuid when unnamed)
- [ ] Restart same target+port without `--name` does NOT erase the saved name
- [ ] Proxy still starts normally when the store is corrupt or unwritable (warning only)

## Risk Assessment

- **runServer signature change** breaks no external callers (only root.go calls it) —
  verified single call site.
- **Name collisions** are allowed here by design; ambiguity is resolved at resume time
  (Phase 3), not at save time.

## Security Considerations

Name is echoed back to the terminal only; stored plaintext alongside non-secret config.

## Next Steps

Phase 3 adds the `resume` subcommand consuming the now-populated store.
