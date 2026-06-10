# Red-Team Plan Review: corsproxy Go CLI

Plan: `plans/260610-2249-corsproxy-go-cli/` (4 phases). Advisory only — no plan/code edits.

## Verified-sound decisions (no change needed)

- **Middleware-set CORS headers DO survive ReverseProxy**: `ReverseProxy.ServeHTTP` copies upstream headers into `w.Header()` via `Add` without clearing pre-set headers. Strip-in-ModifyResponse + inject-in-middleware avoids duplicates. Design correct.
- **ErrorHandler 502 DOES carry middleware CORS headers**: `http.Error` writes pre-set `w.Header()` entries. Key Decision 4 is valid.
- **Preflight detection** (`OPTIONS` + `Access-Control-Request-Method` present) matches Fetch spec; forwarding plain OPTIONS is right.
- Rewrite API, FlushInterval -1, listen-before-banner, `*`+credentials prohibition, huh fallback chain: all correct.

## Critical

**C1. `Out.Host = In.Host` breaks host-checking backends (plan.md arch, phase-02 Rewrite).**
Client hits `http://192.168.1.5:3001` → backend receives `Host: 192.168.1.5:3001`. Django (`ALLOWED_HOSTS`), Rails 6+ host authorization, Vite `server.allowedHosts`, Spring strict-host setups reject with 400/403 — exactly the LAN-mobile scenario this tool exists for. `SetURL` already rewrites Host to target host by default; the explicit preserve line is the wrong default here. Fix: drop `Out.Host = In.Host` (or gate behind a future flag). One-line change, prevents the tool's core use case failing against common dev backends.

## Major

**M1. WebSocket pass-through claim is false with the planned `statusRecorder` (phase-02 step 3 + non-functional req).**
Upgrade handling calls `http.NewResponseController(rw).Hijack()`. A wrapper implementing only `Flusher` passthrough hides `Hijacker` → WS upgrade fails → 502. Fix: add `Unwrap() http.ResponseWriter` to `statusRecorder` (Go 1.20+ idiom) — `ResponseController` then resolves BOTH Hijack and Flush through the chain, making the manual `Flush()` passthrough optional. Also: no success criterion in any phase verifies WS or SSE; either add a quick manual check (e.g. `websocat`/EventSource) or soften the requirement wording.

**M2. Default `--origin "*"` breaks cookie/credentialed requests — the common case.**
`fetch(..., {credentials:'include'})` + `ACAO: *` → browser rejects. Default config thus fails for any cookie-auth backend, showing the exact CORS error the tool promises to remove. Standard dev-proxy fix without changing the flag spec: in `*` mode, when request has an `Origin` header, echo it into ACAO + `Vary: Origin` + `Allow-Credentials: true`; fall back to literal `*` only when no Origin header. Keeps `--origin "*"` semantics, makes credentials work.

**M3. Parsing configured origin without scheme is a silent-fail trap (phase-02 origin matching).**
`url.Parse("192.168.1.5")` yields `Host=""`, `Path="192.168.1.5"` → host comparison never matches, ACAO never set, user sees mystery CORS errors. Plan explicitly allows scheme-less origin values. Fix: if configured value lacks `://`, treat whole string as host (or prepend `//` and use `url.Parse` host extraction). Add a phase-4 test case for scheme-less origin.

## Minor

**m1. `statusRecorder` must default status to 200** — handlers writing body without `WriteHeader` will log `0` otherwise. Not mentioned in phase-02.

**m2. Log via `defer`** — mid-body copy failures make ReverseProxy panic with `http.ErrAbortHandler`; a non-deferred log line after `next.ServeHTTP` is skipped. Deferring keeps the request log complete.

**m3. Phase-2 success criterion "upstream CORS stripped → only 1 set of headers" is unverifiable in-phase**: the manual backend (`python3 -m http.server`) never sets CORS headers. Either note it's deferred to phase-4 tests or use a 2-line Go/python backend that sets ACAO.

**m4. Research report snippet conflicts with plan**: researcher's `ModifyResponse` example *injects* CORS there; plan injects in middleware only. Plan is right — implementer should follow plan architecture, not the research snippet, or duplicates return.

**m5. Missing `Access-Control-Expose-Headers`**: FE can't read custom response headers (`X-Total-Count`, `X-Request-Id`, etc.) — only safelisted ones. One-liner: set `Access-Control-Expose-Headers: *` (non-credential mode) / echo a permissive list in credential mode.

**m6. HTTPS target with self-signed cert** → TLS verify fails → 502 with raw x509 error text. Acceptable for v1, but worth a README "known limitations" line so users don't debug blindly (same for the cookie `Secure`/SameSite caveat over plain-HTTP LAN).

**m7. Port-in-use detection**: string match works; slightly sturdier hybrid is `errors.Is(err, syscall.EADDRINUSE)` with string fallback for Windows (`WSAEADDRINUSE` doesn't map to `syscall.EADDRINUSE` via `errors.Is`). Current plan's dual-string check is fine for a dev tool — informational only.

## Phase ordering / verifiability

- Dependency chain 1→2→3→4 sound; no ordering errors. Phase-2's temporary `ListenAndServe` superseded in phase-3 is acknowledged — fine.
- Only unverifiable criteria found: m3 above, and WS/SSE claim (M1).

## YAGNI check

Scope is appropriately small; no over-engineering found. 5-package layout is slightly heavy for ~300 LOC but matches stated learning goals (user-confirmed) — no cut proposed. All user-confirmed features (two modes, port-in-use, reachability warning, log, banner) intact in plan.

## Unresolved questions

1. M2 (echo-origin in `*` mode): keep literal `*` per user-confirmed flag semantics, or adopt echo behavior? Behavioral change, recommend confirming with user (1-line either way).
2. C1: any known need for preserving client Host (e.g. backend generating absolute URLs)? If not, default-to-target-host is the safe pick.

**Status:** DONE
