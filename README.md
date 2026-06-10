# corsproxy

CORS reverse proxy cho local development — test app trên điện thoại qua mạng LAN mà không dính lỗi CORS.

## Vấn đề

Front-end chạy `localhost:3000`, backend chạy `localhost:8080` và chỉ allow origin `localhost:3000`. Mọi thứ ổn — cho đến khi bạn mở app trên điện thoại qua `http://192.168.1.5:3000`: mọi API call đều fail vì backend không allow origin `192.168.1.5`.

## Giải pháp

```
Phone ──→ http://192.168.1.5:3000  (front-end)
  │
  └─ API calls ──→ http://192.168.1.5:3001  (corsproxy)
                        │  inject CORS headers,
                        │  forward request
                        ▼
                   http://localhost:8080  (backend, không cần sửa gì)
```

corsproxy đứng giữa: nhận request từ browser, forward đến backend (server-to-server nên không có khái niệm CORS), rồi trả response về kèm CORS headers hợp lệ.

## Cài đặt

```bash
go install github.com/felix-nguyen/corsproxy@latest
```

## Sử dụng

**Với flags:**

```bash
corsproxy --target http://localhost:8080 --port 3001 --origin "*"
```

**Interactive (không cần nhớ flags):**

```bash
corsproxy
# → form hỏi từng bước: backend URL, port, origin
```

Khi chạy:

```
  corsproxy đang chạy

  Local:    http://localhost:3001
  Network:  http://192.168.1.5:3001   ← dùng URL này trên phone
  Forward:  → http://localhost:8080
  Origin:   *

  Ctrl+C để dừng

GET /api/users → 200 (23ms)
POST /api/login → 401 (12ms)
```

Trỏ API base URL của front-end sang `http://<lan-ip>:3001` là xong.

### Flags

| Flag | Default | Mô tả |
|------|---------|-------|
| `--target` | (bắt buộc) | Backend URL để forward đến |
| `--port` | `3001` | Port proxy lắng nghe |
| `--origin` | `*` | `*` = allow tất cả, hoặc IP/origin cụ thể (vd `192.168.1.5`) |

## Hoạt động thế nào

- **Preflight (OPTIONS):** trả lời `204` ngay tại proxy, không forward — tránh backend redirect làm preflight fail.
- **CORS headers của backend:** bị strip và thay bằng headers của proxy — tránh duplicate khiến browser reject.
- **`--origin "*"`:** echo lại `Origin` của request thay vì literal `*`, nên `fetch` với `credentials: 'include'` (cookie auth) vẫn hoạt động.
- **Backend chết:** trả `502` kèm CORS headers — bạn thấy lỗi 502 thật trong DevTools thay vì lỗi CORS đánh lạc hướng.
- **WebSocket / SSE:** pass-through tự động.

## Giới hạn

- Chỉ dành cho **development**. Echo-origin + credentials là permissive có chủ đích — đừng chạy trên production.
- Backend HTTPS với self-signed certificate chưa được hỗ trợ.
- Cookie có flag `Secure` hoặc `SameSite=Strict` có thể không hoạt động khi truy cập qua HTTP/LAN IP — đó là hành vi của browser, không phải của proxy.
