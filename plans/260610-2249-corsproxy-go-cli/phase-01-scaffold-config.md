---
phase: 1
title: Scaffold & Config
status: completed
priority: P1
effort: 30m
dependencies: []
---

# Phase 1: Scaffold & Config

## Overview

Init Go module, install deps, tạo skeleton cobra CLI + config package với validation. Cuối phase: `go build` chạy được, binary parse flags và in config ra màn hình.

## Requirements

- Functional: `corsproxy --target http://localhost:8080 --port 3001 --origin "*"` parse + validate + in config
- Non-functional: mỗi file <200 lines, idiomatic Go

## Architecture

`main.go` → `cmd.Execute()` → cobra root command đọc flags → build `config.Config` → validate → (phase này chỉ in ra, phase 2 mới start server).

```go
// internal/config/config.go
type Config struct {
    Target *url.URL // backend URL (parsed)
    Port   int      // proxy listen port
    Origin string   // "*" hoặc origin cụ thể
}
```

## Related Code Files

- Create: `go.mod`, `go.sum`, `main.go`, `cmd/root.go`, `internal/config/config.go`

## Implementation Steps

1. `cd /home/felix-nguyen/Code/corsproxy && go mod init github.com/felix-nguyen/corsproxy`
2. `go get github.com/spf13/cobra@latest`
3. `go get charm.land/huh/v2@latest`
   - **FALLBACK:** nếu vanity import fail → `go get github.com/charmbracelet/huh/v2@latest`; nếu v2 không tồn tại trên github path → `go get github.com/charmbracelet/huh@latest` (v0.x) và điều chỉnh API calls ở phase 3 theo docs version đó. Ghi lại version thực dùng.
4. `internal/config/config.go`:
   - `Config` struct như trên
   - `func New(target string, port int, origin string) (*Config, error)` — parse + validate:
     - target: `url.Parse`, require scheme `http`/`https` và non-empty host. Error message gợi ý: `"target phải dạng http://host:port, ví dụ http://localhost:8080"`
     - port: 1–65535
     - origin: `"*"` hoặc non-empty string (giá trị cụ thể xử lý matching ở phase 2)
5. `cmd/root.go`: cobra root command
   - Flags: `--target` (string), `--port` (int, default 3001), `--origin` (string, default `"*"`)
   - `RunE`: build config từ flags, validate, tạm `fmt.Printf("%+v\n", cfg)` (placeholder, thay ở phase 2)
6. `main.go`: gọi `cmd.Execute()`, exit code 1 nếu error
7. `go build ./...` + chạy thử cả case hợp lệ và case invalid (port 99999, target thiếu scheme)

## Success Criteria

- [ ] `go build ./...` không lỗi
- [ ] `go vet ./...` sạch
- [ ] Flags hợp lệ → in config; invalid → error message rõ ràng tiếng Việt/Anh đơn giản, exit 1
- [ ] Version huh thực dùng được ghi lại (comment trong go.mod hoặc note cho phase 3)

## Risk Assessment

- **huh v2 vanity import (`charm.land/huh/v2`) có thể không resolve** → fallback chain ở step 3. Phase 3 phải đọc đúng API docs của version thực cài.
