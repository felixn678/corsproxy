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
corsproxy --target http://localhost:8080 --port 3001 --origin "*" --name myapi
```

**Interactive (no flags needed):**

```bash
corsproxy
# → prompts for backend URL, port, allowed origin, and an optional name
```

When running:

```
  corsproxy is running

  Name:     myapi
  Local:    http://localhost:3001
  Network:  http://192.168.1.5:3001   ← use this URL on your phone
  Forward:  → http://localhost:8080
  Origin:   *

  Press Ctrl+C to stop

GET /api/users → 200 (23ms)
POST /api/login → 401 (12ms)
```

Point your front-end's API base URL to `http://<lan-ip>:3001` and you're done.

### Resume a server

Every started server is saved automatically. On exit, corsproxy prints how to
get it back:

```
Resume later with:  corsproxy resume myapi
```

```bash
corsproxy resume myapi    # restart by name (or uuid)
corsproxy resume          # no argument → pick from a list of saved servers
```

Restarting the same target+port updates the saved entry instead of creating a
duplicate, and keeps its name. Sessions live in `~/.config/corsproxy/sessions.json`.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--target` | (required) | Backend URL to forward requests to |
| `--port` | `3001` | Port the proxy listens on |
| `--origin` | `*` | `*` = allow all, or a specific IP/origin (e.g. `192.168.1.5`) |
| `--name` | (none) | Name this server so you can resume it later |

## How it works

- **Preflight (OPTIONS):** answered with `204` at the proxy, not forwarded — prevents backend redirects from breaking preflight.
- **Backend CORS headers:** stripped and replaced by the proxy's own headers — prevents duplicate `Access-Control-Allow-Origin` that browsers reject.
- **`--origin "*"`:** echoes back the request's `Origin` header instead of a literal `*`, so `fetch` with `credentials: 'include'` (cookie auth) still works.
- **Backend down:** returns `502` with CORS headers — you see a real 502 in DevTools instead of a misleading CORS error.
- **WebSocket / SSE:** passed through automatically.

## Limitations

corsproxy does exactly one thing: inject CORS headers for HTTP API calls to a
single backend over the LAN. The cases below are out of scope — most stem from
the fact that the proxy rewrites the **request target** only, never the response
body or headers like `Location`.

**Likely to hit you first:**

- **HTTPS front-end → mixed content.** If your front-end is served over `https://`,
  the browser blocks calls to the `http://` proxy. corsproxy serves HTTP only (no TLS).
- **Absolute `localhost` URLs in responses.** Response bodies are not rewritten, so
  a payload like `{"avatar": "http://localhost:8080/x.png"}` fails on the phone —
  `localhost` there points to the phone itself.
- **Redirects to absolute URLs.** A `302 Location: http://localhost:8080/login` is
  not rewritten, so the browser follows it to a dead address.
- **Cookies scoped to `localhost` or marked `Secure`.** `Domain=localhost` cookies
  don't apply to the LAN IP; `Secure` cookies aren't sent over HTTP — both break
  session/login.

**Depends on your architecture:**

- **Multiple backends.** Forwards to a single `--target` only — no path-based routing.
  Run one instance per backend on different ports.
- **Third-party OAuth flows.** Redirect URIs registered for `localhost:3000` mismatch
  when accessed via the LAN IP; the provider rejects the callback.
- **HTTPS backends with self-signed certificates.** Not yet supported (no `--insecure` flag).

**Other:**

- For **development only**. Echo-origin + credentials is intentionally permissive,
  and there is no auth or rate limiting — anyone on the LAN can reach your backend
  through the proxy.
- gRPC / HTTP/2 (h2c) and backends that check the WebSocket handshake `Origin` may
  not work.
- LAN IP detection (UDP dial to `8.8.8.8`) can pick the wrong interface behind a VPN,
  Docker network, or multi-NIC setup.
