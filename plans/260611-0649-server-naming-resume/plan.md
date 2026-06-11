---
title: Server naming + resume (Claude Code-style sessions)
description: >-
  Name a proxy server like a Claude Code session, auto-save every start to
  ~/.config/corsproxy/sessions.json, and resume via `corsproxy resume
  [name|uuid]` with a huh picker fallback.
status: completed
priority: P2
branch: main
tags:
  - go
  - cli
  - learning-project
blockedBy: []
blocks: []
created: '2026-06-11T00:03:16.991Z'
createdBy: 'ck:plan'
source: skill
---

# Server naming + resume (Claude Code-style sessions)

## Overview

Mimic Claude Code sessions: every successful start is saved (uuid + optional name +
config); exit prints `Resume later with: corsproxy resume <name|uuid>`; `corsproxy resume`
with no arg shows a huh select of saved servers (name, fallback uuid). Resume = start
again with the stored config — no log replay, no runtime state.

**Learning goals:** JSON persistence, atomic file writes (temp+rename), `os.UserConfigDir`,
cobra subcommands, huh `Select`, pure-function design for testability.

## Key Decisions

1. **Store path:** `os.UserConfigDir()/corsproxy/sessions.json` — stdlib already resolves
   `$XDG_CONFIG_HOME` on Linux; correct dirs on macOS/Windows for free.
2. **ID:** `github.com/google/uuid` (new dep). Full uuid stored; exit hint prefers name.
3. **Dedupe key = target+port.** Upsert updates `lastUsedAt`/`origin`; **name is only
   overwritten when the user passed one this run** — restarting without `--name` must not
   erase an existing name. Existing ID is preserved on upsert.
4. **Corrupt store never blocks the proxy:** warn to stderr, treat as empty. Next save
   overwrites (acceptable data loss for a dev tool).
5. **Save only after `net.Listen` succeeds** — only servers that actually started get
   recorded. Save failure = warning, never fatal.
6. **`resume` is a subcommand, not a `--resume` flag** — idiomatic cobra, and one command
   covers both arg/no-arg flows. No alias (YAGNI).
7. **Lookup precedence:** exact uuid match → case-insensitive name match
   (`strings.EqualFold`). Ambiguous name → error listing matches + their uuids, tell user
   to rerun with uuid (simpler than a narrowed picker).
8. **`config.Config` stays pure proxy config.** Session concerns live in
   `internal/session` + `cmd` glue. `runServer` gains a `resumeHint string` param
   (empty → no hint line).

## Target Structure

```
internal/session/
├── session.go    # Session model + pure Upsert/Find logic
├── store.go      # DefaultPath, Load, atomic Save
└── session_test.go
cmd/
├── resume.go     # new: resume subcommand + huh picker
├── root.go       # modify: --name flag, save-on-start glue
├── interactive.go# modify: 4th field "Name (optional)"
└── server.go     # modify: resumeHint param, print after shutdown
```

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Session Store](./phase-01-session-store.md) | Completed |
| 2 | [Naming & Auto-Save](./phase-02-naming-auto-save.md) | Completed |
| 3 | [Resume Command](./phase-03-resume-command.md) | Completed |

## Dependencies

Phase 2 and 3 depend on Phase 1 (store API). Phase 3 reuses the glue helper from Phase 2.
No cross-plan dependencies (260610-2249-corsproxy-go-cli is completed).
