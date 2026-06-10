---
phase: 3
title: CLI & Interactive Mode
status: completed
priority: P1
effort: 1h
dependencies:
  - 2
---

# Phase 3: CLI & Interactive Mode

## Overview

Interactive form (huh) khi thiếu `--target`, startup UX hoàn chỉnh: LAN IP banner, port-in-use error + suggest, target reachability warning, graceful shutdown Ctrl+C.

## Requirements

- Functional:
  - Chạy `corsproxy` không flags → huh form hỏi: target URL, port (default "3001"), origin (default "*")
  - Form validate inline: target là URL hợp lệ, port là số 1-65535
  - Port đang bị chiếm → error rõ ràng + suggest port kế tiếp, exit 1
  - Target không reachable lúc start → warning vàng nhưng VẪN start (backend có thể chưa bật)
  - Ctrl+C → graceful shutdown (đợi in-flight requests tối đa 5s)
  - Banner hiển thị LAN IP để user biết URL nhập trên phone
- Non-functional: 2 instance khác port chạy song song bình thường (không lock file, không singleton)

## Architecture

```
cmd/root.go RunE:
  1. nếu --target rỗng → runInteractiveForm() (huh) → điền vào biến flags
  2. config.New(...) → validate
  3. net.Listen("tcp", ":port")
     └─ lỗi "address already in use" → in error + "thử: corsproxy --port {port+1}" → exit 1
  4. checkTargetReachable(target, 2s timeout) → fail thì in warning, vẫn tiếp tục
  5. in banner (LAN IP từ internal/netutil)
  6. go server.Serve(listener)
  7. đợi signal (signal.NotifyContext: SIGINT, SIGTERM) → server.Shutdown(ctx 5s)
```

Banner mẫu:
```
  corsproxy đang chạy

  Local:    http://localhost:3001
  Network:  http://192.168.1.5:3001   ← dùng URL này trên phone
  Forward:  → http://localhost:8080
  Origin:   *

  Ctrl+C để dừng
```

## Related Code Files

- Create: `internal/netutil/lanip.go`, `cmd/interactive.go` (huh form, tách file cho gọn)
- Modify: `cmd/root.go` (startup flow như trên)

## Implementation Steps

1. `internal/netutil/lanip.go`:
   - `func LANIP() string` — UDP dial trick `net.Dial("udp", "8.8.8.8:80")` lấy outbound IP
   - Offline/error → return `""` (banner chỉ hiện Local line)
2. `cmd/interactive.go`:
   - `func runInteractiveForm(target, port, origin *string) error` dùng huh (API theo version đã cài ở phase 1)
   - 3 inputs trong 1 group: Target (validate `url.Parse` + scheme), Port (validate số + range — huh input là string, convert sau), Origin (default "*", non-empty)
   - Defaults prefill bằng cách init biến trước khi `.Value(&var)`
3. `cmd/root.go` refactor theo Architecture:
   - Port-in-use detect: ưu tiên `errors.Is(err, syscall.EADDRINUSE)` (Linux/macOS), fallback string match "address already in use" + Windows "Only one usage of each socket address"
   - `checkTargetReachable`: `net.DialTimeout("tcp", targetHostPort, 2*time.Second)`; nếu target URL không có port → default 80/443 theo scheme
   - Graceful shutdown: `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` → `<-ctx.Done()` → `server.Shutdown(shutdownCtx)` với 5s timeout
   - `http.Server{Handler: handler}` + `server.Serve(listener)` (listener từ step 3 — listen TRƯỚC khi báo thành công, tránh race in banner rồi mới fail)
4. Manual tests:
   - `go run .` không flags → form hiện, điền xong server start
   - Mở 2 terminal cùng port → instance 2 báo lỗi + suggest port mới
   - 2 terminal khác port → cả 2 chạy ok
   - Ctrl+C → "shutting down..." rồi exit sạch
   - Tắt wifi → banner vẫn hiện (không có Network line)

## Success Criteria

- [ ] `go build ./...` + `go vet ./...` sạch
- [ ] Cả 5 manual tests ở step 4 pass
- [ ] Form validation chặn input sai ngay trong form (không để crash sau khi submit)
- [ ] Binary cài được: `go install` → chạy `corsproxy` từ PATH

## Risk Assessment

- **huh API khác research nếu fallback version** → đọc pkg.go.dev của version thực trong go.mod trước khi viết
- **Port-in-use string match khác nhau giữa OS** → check cả 2 chuỗi Linux + Windows; macOS giống Linux
- **signal.NotifyContext quên defer stop()** → goroutine leak nhẹ, nhớ defer
