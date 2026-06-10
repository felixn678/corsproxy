---
phase: 4
title: Tests & Docs
status: completed
priority: P2
effort: 45m
dependencies:
  - 3
---

# Phase 4: Tests & Docs

## Overview

Unit tests cho proxy behavior + config validation (httptest, không mock tự chế), README usage. Đây là phase học `testing` package + `httptest` — pattern test HTTP rất đáng học của Go.

## Requirements

- Functional: tests cover CORS injection, preflight, upstream-strip, origin matching, config validation
- Non-functional: `go test ./...` pass, không flaky, không cần network thật (httptest.Server là server thật trên loopback — không phải mock)

## Related Code Files

- Create: `internal/proxy/proxy_test.go`, `internal/proxy/cors_test.go`, `internal/config/config_test.go`, `README.md`
- Modify: none

## Implementation Steps

1. `internal/config/config_test.go` — table-driven tests (idiomatic Go, học pattern này):
   - valid target / thiếu scheme / scheme ftp / host rỗng
   - port 0, -1, 65536, 80 (valid)
   - origin "*", "192.168.1.5", rỗng (invalid)
2. `internal/proxy/proxy_test.go` — dùng `httptest.NewServer` làm backend thật:
   - GET qua proxy → body từ backend nguyên vẹn + `Access-Control-Allow-Origin` đúng
   - Backend set sẵn `Access-Control-Allow-Origin: http://other` → response chỉ còn headers của proxy (không duplicate)
   - Backend down (đóng httptest server) → 502 + vẫn có CORS headers
3. `internal/proxy/cors_test.go`:
   - Preflight OPTIONS (có `Access-Control-Request-Method`) → 204, không chạm backend (đếm hit bằng counter trong backend handler)
   - OPTIONS thường (không có request-method header) → forward đến backend
   - Origin cụ thể: request Origin match host → echo + `Vary: Origin` + credentials true; không match → không có ACAO
   - Origin "*" + request CÓ Origin header → echo origin + `Vary: Origin` + credentials true (Key Decision 8)
   - Origin "*" + request KHÔNG có Origin header (curl) → literal `*`, KHÔNG có Allow-Credentials
   - Configured origin scheme-less (`192.168.1.5`) match được request `Origin: http://192.168.1.5:3000` (regression cho bẫy url.Parse)
4. `README.md` (root repo — repo cá nhân lên GitHub):
   - Problem statement ngắn (mobile + LAN + CORS)
   - Install: `go install github.com/felix-nguyen/corsproxy@latest`
   - Usage: flags mode + interactive mode + ví dụ output banner
   - How it works: diagram ascii 5 dòng
   - Limitations: HTTPS backend self-signed cert chưa support (cần `--insecure` flag — future); tool chỉ dành cho dev, echo-origin + credentials là permissive có chủ đích
5. `go test ./... -v` + `go vet ./...` — tất cả pass

## Success Criteria

- [ ] `go test ./...` pass toàn bộ
- [ ] Test backend-down không flaky (dùng server đã Close, không sleep)
- [ ] README đủ cho người lạ dùng được trong 1 phút đọc

## Risk Assessment

- **httptest server port ngẫu nhiên** → luôn lấy `server.URL` động, không hardcode
- **Test preflight đếm backend hits** → dùng `atomic.Int32` tránh race khi `-race` flag bật
