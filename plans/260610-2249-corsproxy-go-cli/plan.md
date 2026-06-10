---
title: corsproxy — Go CLI CORS reverse proxy for local dev
description: >-
  CLI tool: forward requests to local backend, inject permissive CORS headers so
  mobile devices on LAN can call APIs without CORS errors
status: completed
priority: P2
branch: main
tags:
  - go
  - cli
  - learning-project
blockedBy: []
blocks: []
created: '2026-06-10T15:54:57.746Z'
createdBy: 'ck:plan'
source: skill
---

# corsproxy — Go CLI CORS reverse proxy for local dev

## Overview

**Problem:** FE chạy localhost:3000, BE localhost:8080 (CORS chỉ allow localhost:3000). Test trên mobile qua LAN IP → CORS error.

**Solution:** `corsproxy` lắng nghe port riêng (vd :3001), forward mọi request đến backend, strip CORS headers của backend và inject CORS headers permissive. Mobile gọi `http://<lan-ip>:3001` thay vì backend trực tiếp.

**Learning goals (user là FE dev học Go):** net/http, httputil.ReverseProxy, middleware pattern, cobra, huh, error handling, goroutine + graceful shutdown.

- Module: `github.com/felix-nguyen/corsproxy` | Go 1.26
- Stack: cobra (CLI), charm.land/huh/v2 (interactive form), stdlib proxy
- Scope: ~250-350 LOC. NO database/Docker/CI.
- Research: [researcher report](../reports/researcher-260610-2249-huh-reverseproxy-gotchas-report.md)

## Target Structure

```
corsproxy/
├── main.go                      # entry → cmd.Execute()
├── cmd/root.go                  # cobra root, flags, interactive fallback, server startup
├── internal/
│   ├── config/config.go         # Config struct + validation
│   ├── proxy/proxy.go           # ReverseProxy (Rewrite, ModifyResponse, ErrorHandler)
│   ├── proxy/cors.go            # CORS middleware + OPTIONS preflight
│   ├── proxy/logging.go         # request log middleware
│   └── netutil/lanip.go         # LAN IP detection
```

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Scaffold & Config](./phase-01-scaffold-config.md) | Completed |
| 2 | [Proxy Core](./phase-02-proxy-core.md) | Completed |
| 3 | [CLI & Interactive Mode](./phase-03-cli-interactive-mode.md) | Completed |
| 4 | [Tests & Docs](./phase-04-tests-docs.md) | Completed |

## Key Decisions

1. **Rewrite API** (Go 1.20+), không dùng Director (deprecated).
2. **Strip upstream CORS trong ModifyResponse**, inject CORS ở middleware (cover cả proxied response lẫn proxy-generated errors như 502). `verified by red-team report: ReverseProxy copyHeader dùng Add, không clear w.Header()`.
3. **OPTIONS preflight trả 204 tại proxy**, không forward (backend redirect làm preflight fail silent).
4. **ErrorHandler trả 502 kèm CORS headers** — nếu thiếu, browser hiện CORS error thay vì 502 thật → đánh lừa người debug.
5. **FlushInterval = -1** cho SSE/streaming.
6. **Interactive mode trigger:** thiếu `--target` → huh form (prefill defaults). Có `--target` → dùng flags + defaults.
7. **KHÔNG preserve inbound Host** — để SetURL default (Host = target host). Preserve `In.Host` sẽ gửi `Host: 192.168.x.x:3001` → backend check host (Django ALLOWED_HOSTS, Rails, Vite allowedHosts) reject 400/403 đúng ngay scenario LAN-mobile mà tool nhắm tới. (Red-team C1, đảo quyết định từ research report.)
8. **Origin "*" → echo request `Origin` header + `Vary: Origin`** thay vì literal `*` (literal `*` bị browser reject khi FE dùng `credentials: 'include'` — cookie-auth apps fail). Không có Origin header (curl, same-origin) → fallback literal `*`. Flag semantics không đổi: `--origin "*"` vẫn nghĩa là "cho phép tất cả". (Red-team M2.)

## Dependencies

None (no other plans in this repo).
