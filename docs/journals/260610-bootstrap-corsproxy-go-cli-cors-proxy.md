# Bootstrap & Ship: corsproxy Go CLI CORS Reverse Proxy

**Date**: 2026-06-10 22:49
**Severity**: Low (success; lessons recorded)
**Component**: corsproxy (full project)
**Status**: Resolved

## What Happened

Bootstrapped, researched, planned, implemented, tested, reviewed, and shipped corsproxy — a Go CLI CORS reverse proxy for LAN-mobile dev testing. First Go learning project for the user (FE background). Flow: requirements → researcher → `/ck:plan --hard` (4 phases) → red-team audit → implementation → 18 tests pass with `-race` → code review (zero critical/high findings) → commit 43c3f72. ~750 LOC including tests.

## The Brutal Truth

This was smooth because we did the hard stuff right: research phase killed misconceptions early, red-team caught what research missed, and staged testing (unit → integration → race detector) prevented shipping broken code. No firefighting post-launch. Boring is good.

## Technical Details

Key fixes & learnings shipped:

1. **CORS headers**: `*` origin cannot be literal with credentials. Must echo request `Origin` + `Vary: Origin` (spec: `https://fetch.spec.whatwg.org/`).
2. **Host header trap**: Research recommended preserving inbound Host; red-team proved it breaks Django/Rails/Vite host-checks in LAN-mobile scenario → reverted to SetURL default (target Host). Adversarial review caught research blind spot.
3. **WebSocket 502**: `http.ResponseWriter` wrappers require `.Unwrap()` for `http.ResponseController`; missing this fails silently on upgrade.
4. **url.Parse scheme-less**: `url.Parse("192.168.1.5")` puts value in `.Path` not `.Host` — trap when parsing origins without `http://` prefix.
5. **pkill -f bracket trick**: `pkill -f "patter[n]"` prevents matching invoking shell's own command line.

## Lessons Learned

- **Red-team adversarial review > isolated research**: Caught real-world failure mode (host header checks) that research assumed was safe.
- **Stage testing from atomic → race**: Unit tests pass; integration tests under `-race` flag expose data races. Both matter.
- **HTTP spec details matter**: CORS `*` behavior with credentials is subtle; reading spec once saved a production bug.

## Next Steps

None. Project complete and shipped.

**Status**: DONE
