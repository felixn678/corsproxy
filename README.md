# corsproxy

CORS reverse proxy for local development — test your app on a phone over LAN without CORS errors.

## The problem

Your front-end runs on `localhost:3000`, your backend on `localhost:8080` (CORS allows `localhost:3000` only). Everything works fine — until you open the app on your phone via `http://192.168.1.5:3000`: every API call fails because the backend doesn't allow origin `192.168.1.5`.

## The solution

```
Phone ──→ http://192.168.1.5:3000  (front-end)
  │
  └─ API calls ──→ http://192.168.1.5:3001  (corsproxy)
                        │  injects CORS headers,
                        │  forwards request
                        ▼
                   http://localhost:8080  (backend, no changes needed)
```

corsproxy sits in between: it receives requests from the browser, forwards them to the backend (server-to-server, so no CORS applies), then returns the response with valid CORS headers attached.

## Install

```bash
go install github.com/felix-nguyen/corsproxy@latest
```

## Usage

**With flags:**

```bash
corsproxy --target http://localhost:8080 --port 3001 --origin "*"
```

**Interactive (no flags needed):**

```bash
corsproxy
# → prompts for backend URL, port, and allowed origin
```

When running:

```
  corsproxy is running

  Local:    http://localhost:3001
  Network:  http://192.168.1.5:3001   ← use this URL on your phone
  Forward:  → http://localhost:8080
  Origin:   *

  Press Ctrl+C to stop

GET /api/users → 200 (23ms)
POST /api/login → 401 (12ms)
```

Point your front-end's API base URL to `http://<lan-ip>:3001` and you're done.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--target` | (required) | Backend URL to forward requests to |
| `--port` | `3001` | Port the proxy listens on |
| `--origin` | `*` | `*` = allow all, or a specific IP/origin (e.g. `192.168.1.5`) |

## How it works

- **Preflight (OPTIONS):** answered with `204` at the proxy, not forwarded — prevents backend redirects from breaking preflight.
- **Backend CORS headers:** stripped and replaced by the proxy's own headers — prevents duplicate `Access-Control-Allow-Origin` that browsers reject.
- **`--origin "*"`:** echoes back the request's `Origin` header instead of a literal `*`, so `fetch` with `credentials: 'include'` (cookie auth) still works.
- **Backend down:** returns `502` with CORS headers — you see a real 502 in DevTools instead of a misleading CORS error.
- **WebSocket / SSE:** passed through automatically.

## Limitations

- For **development only**. Echo-origin + credentials is intentionally permissive — do not run in production.
- HTTPS backends with self-signed certificates are not yet supported.
- Cookies with `Secure` or `SameSite=Strict` flags may not work when accessed via HTTP/LAN IP — this is browser behavior, not a proxy limitation.
