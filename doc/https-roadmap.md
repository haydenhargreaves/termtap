# HTTPS Support Roadmap

##### Drafted April 18th, 2026

## Why this exists

Most modern services communicate over HTTPS. To make termtap immediately useful, HTTPS visibility should be enabled by default, while still providing safe fallback behavior when local certificate trust is not yet configured.

This roadmap defines staged delivery from tunnel-only metadata to full HTTPS payload visibility.

## Product direction

- Default behavior should attempt HTTPS interception (MITM) so users can inspect request/response data with minimal configuration friction.
- If interception is not possible yet (for example, trust root not installed), termtap should clearly guide the user and provide an explicit fallback mode.
- Non-CONNECT HTTP forwarding should continue working as it does today.

## Current baseline

- HTTP proxying for non-CONNECT requests is implemented.
- HTTPS CONNECT requests are currently rejected (`501`).
- Request/event models already support lifecycle events and request metadata previews for HTTP.

## Stage 1: HTTPS Tunnel Visibility (CONNECT passthrough)

### Goal

Support HTTPS connectivity and show tunnel-level activity in TUI/events without decrypting payloads.

### Scope

1. Add CONNECT handling:
   - accept `CONNECT host:port`
   - hijack downstream connection
   - dial upstream TCP target
   - return `200 Connection Established`
   - pipe bytes both directions until close/error

2. Emit lifecycle events:
   - `RequestStarted` when tunnel opens
   - `RequestFinished` on clean close
   - `RequestFailed` on setup/tunnel errors

3. Populate request metadata:
   - `Method=CONNECT`
   - `Host`, `RawURL`, status (usually `200` on success), duration

4. Harden tunnel behavior:
   - dial timeout
   - deterministic close on both sockets
   - clear status mapping for dial/hijack failures

### Acceptance criteria

- `curl -v https://example.com` works through termtap proxy.
- CONNECT requests show in TUI/events with status and timing.
- Existing HTTP behavior remains unchanged.

## Stage 2: Default HTTPS Interception (MITM)

### Goal

Enable decrypted HTTPS inspection by default so users can view headers and payload previews.

### Scope

1. Introduce HTTPS modes (runtime):
   - `intercept` (default)
   - `tunnel` (fallback/explicit opt-out)

2. Add CA management:
   - generate/load local root CA certificate + key
   - cache in stable app directory
   - surface trust/install status at startup

3. Implement interception pipeline:
   - on CONNECT in intercept mode:
     - terminate TLS from client using dynamic leaf cert for target host
     - establish TLS to upstream
     - proxy decrypted HTTP request/response through existing capture path

4. Reuse current HTTP capture logic for HTTPS:
   - method/path/headers
   - body preview cap
   - response status/headers/body preview
   - request duration + failures

5. UX for trust onboarding:
   - clear startup event/warning when CA not trusted
   - OS-specific trust instructions
   - command to print/export CA certificate path

### Acceptance criteria

- Trusted client requests over HTTPS show decrypted request/response metadata and previews in TUI.
- Untrusted clients fail with clear guidance (not silent breakage).
- Fallback tunnel mode is available and documented.

## Stage 3: Safety and privacy controls

### Goal

Make interception safe for daily use.

### Scope

- Header redaction defaults (`Authorization`, `Cookie`, `Set-Cookie`, API keys).
- Body preview limits and optional full-body capture toggles.
- Host allowlist/denylist for interception scope.
- Exclusion rules for sensitive domains.
- Clear event labels indicating redacted fields.

### Acceptance criteria

- Sensitive headers are redacted by default.
- Configurable capture policies are applied consistently to HTTP and HTTPS.

## Stage 4: Reliability and operability hardening

### Goal

Ensure proxy remains stable under restart, high load, and shutdown edge cases.

### Scope

- Connection deadlines and idle timeout policy.
- Goroutine/socket leak checks for tunnel + intercept paths.
- Restart behavior with active tunnels.
- Improved error taxonomy and user-facing diagnostics.
- Optional metrics counters (active tunnels, intercepted requests, failures).

### Acceptance criteria

- No resource leaks in stress runs.
- Predictable shutdown/restart with active HTTPS traffic.
- Error messages/events are actionable.

## Implementation order

1. Stage 1 CONNECT passthrough with events.
2. Stage 2 interception plumbing with default `intercept` mode and trust onboarding.
3. Stage 3 redaction/capture controls.
4. Stage 4 reliability hardening and diagnostics.

## Notes

- Interception cannot decrypt HTTPS without a trusted local CA.
- Keeping `tunnel` mode as a fallback reduces operational risk while preserving connectivity.
- Defaulting to `intercept` aligns with product intent (inspect HTTPS by default).
