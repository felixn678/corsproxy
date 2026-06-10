---
phase: 2
title: Proxy Core
status: completed
priority: P1
effort: 1h
dependencies:
  - 1
---

# Phase 2: Proxy Core

## Overview

Trái tim của tool: ReverseProxy + CORS middleware + preflight + request logging. Cuối phase: server chạy được bằng flags mode, curl qua proxy thấy CORS headers đúng.

## Requirements

- Functional:
  - Forward mọi request đến target (giữ path, query, body, headers)
  - Inject CORS headers; strip CORS headers từ upstream (tránh duplicate)
  - OPTIONS preflight → 204 tại proxy, KHÔNG forward
  - Log mỗi request: `GET /api/users → 200 (23ms)`
  - Backend chết giữa chừng → 502 + CORS headers (không phải connection reset)
- Non-functional: WebSocket tự pass-through (stdlib lo), SSE hoạt động (FlushInterval -1)

## Architecture

Handler chain: `loggingMiddleware(corsMiddleware(reverseProxy))`

```
request → logging (đo latency, capture status)
        → cors (set ACAO headers; nếu OPTIONS preflight → 204 return luôn)
        → ReverseProxy
            Rewrite:        SetURL(target), SetXForwarded()
                            (KHÔNG set Out.Host = In.Host — xem plan.md Key Decision 7:
                             backend check Host sẽ reject LAN IP. Default = target host.)
            ModifyResponse: xóa Access-Control-* từ upstream
            ErrorHandler:   log lỗi + 502 (CORS headers đã set ở middleware trước đó)
            FlushInterval:  -1
```

**Origin matching trong corsMiddleware:**
- `cfg.Origin == "*"`:
  - Request CÓ `Origin` header → echo nguyên giá trị đó vào ACAO + `Vary: Origin` + `Access-Control-Allow-Credentials: true` (literal `*` bị browser reject khi FE dùng `credentials: 'include'` — Key Decision 8)
  - Request KHÔNG có `Origin` header (curl, same-origin) → `Access-Control-Allow-Origin: *`, không set credentials (spec cấm combo `*` + credentials)
- Origin cụ thể (vd `192.168.1.5` hoặc `http://192.168.1.5:3000`):
  - **Parse configured value đúng cách:** chứa `://` → `url.Parse` lấy `.Hostname()`; KHÔNG chứa `://` → treat nguyên string là `host[:port]`, strip port lấy host. (Bẫy: `url.Parse("192.168.1.5")` bỏ value vào `.Path` chứ không phải `.Host` → match silent fail nếu parse ngây thơ.)
  - Lấy `Origin` header từ request, parse hostname
  - Match nếu request-origin hostname == configured hostname (bỏ qua port + scheme để dễ dùng), HOẶC full origin string match
  - Match → echo request `Origin` value vào ACAO + `Vary: Origin` + `Access-Control-Allow-Credentials: true`
  - Không match → không set ACAO (browser tự block, đúng hành vi CORS)
- `Access-Control-Allow-Methods`: list `GET,POST,PUT,PATCH,DELETE,OPTIONS,HEAD` (wildcard `*` không hợp lệ với credentials)
- `Access-Control-Allow-Headers`: echo `Access-Control-Request-Headers` nếu có (cover custom headers như Authorization, X-Request-Id), fallback `Content-Type,Authorization`
- `Access-Control-Expose-Headers: *` CHỈ khi ACAO là literal `*` (wildcard expose không hợp lệ khi credentials true); khi echo origin → bỏ qua (đủ dùng cho dev tool)

## Related Code Files

- Create: `internal/proxy/proxy.go` (ReverseProxy setup), `internal/proxy/cors.go` (CORS middleware + preflight), `internal/proxy/logging.go` (log middleware)
- Modify: `cmd/root.go` (thay placeholder print bằng start server cơ bản `http.ListenAndServe` — UX hoàn chỉnh ở phase 3)

## Implementation Steps

1. `internal/proxy/proxy.go`:
   - `func New(cfg *config.Config) http.Handler` — build ReverseProxy theo architecture trên, wrap bằng cors + logging middleware, return handler chain hoàn chỉnh
   - ErrorHandler: `http.Error(w, "proxy: backend unreachable: "+err.Error(), http.StatusBadGateway)` + log
2. `internal/proxy/cors.go`:
   - `func corsMiddleware(next http.Handler, origin string) http.Handler`
   - Logic matching như Architecture. Preflight: `r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""` → set headers + 204 (OPTIONS thường không có header này thì cứ forward — backend có thể dùng OPTIONS thật)
3. `internal/proxy/logging.go`:
   - `statusRecorder` wrap `http.ResponseWriter` capture status code, **init default status = 200** (handler có thể Write mà không gọi WriteHeader)
   - **Lưu ý Go quan trọng (2 interface bắt buộc):**
     - `Flush()` passthrough (SSE) — nếu không `FlushInterval` vô dụng
     - `Unwrap() http.ResponseWriter` return writer gốc — ReverseProxy dùng `http.ResponseController.Hijack()` cho WebSocket upgrade; wrapper không có Unwrap → ResponseController không tìm thấy Hijacker → WebSocket fail 502 silent (red-team M1)
   - **Log line đặt trong `defer`** — handler panic (vd `http.ErrAbortHandler` khi client ngắt kết nối) vẫn log được request
   - Log format: `%s %s → %d (%s)` với latency `time.Since(start).Round(time.Millisecond)`
4. `cmd/root.go`: build handler, `http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), handler)` (tạm — phase 3 đổi sang net.Listen + graceful shutdown)
5. Manual test với backend giả:
   - `python3 -m http.server 8080` làm backend
   - `go run . --target http://localhost:8080 --port 3001`
   - `curl -i http://localhost:3001/` → thấy `Access-Control-Allow-Origin: *` + log line xuất hiện
   - `curl -i -X OPTIONS -H "Access-Control-Request-Method: GET" -H "Origin: http://x" http://localhost:3001/` → 204 + CORS headers
   - Tắt backend → curl → 502 kèm CORS headers

## Success Criteria

- [ ] `go build ./...` + `go vet ./...` sạch
- [ ] Curl checks ở step 5 pass đủ 3 case (normal, preflight, backend down)
- [ ] Upstream-CORS-strip: verify bằng unit test ở phase 4 (python http.server không set CORS headers nên không manual-check được ở phase này)
- [ ] Log line đúng format kèm latency

## Risk Assessment

- **Duplicate CORS headers** nếu quên strip upstream → test case riêng ở phase 4
- **statusRecorder che mất Flusher** → SSE treo. Mitigation: implement Flush() passthrough ngay từ đầu (step 3)
- **OPTIONS thật của backend bị nuốt** → chỉ trả 204 khi có `Access-Control-Request-Method` (dấu hiệu preflight thật)
