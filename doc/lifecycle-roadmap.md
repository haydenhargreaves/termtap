# Process and Session Lifecycle Roadmap

##### Generated via AI on April 14th, 2026

## Why this exists

The current implementation now has a minimum-correct ownership model:

- `Session` owns both the proxy server and spawned process.
- `Session.Stop()` directly requests process shutdown and proxy shutdown.
- process exit status is emitted for both success (`ExitCode=0`) and failures.

This document captures the next iteration so lifecycle behavior remains predictable as the project grows.

## Current baseline (after the minimal fix)

- Process startup and waiting are split:
  - startup returns a process handle immediately
  - waiting happens in a background goroutine
- Unix signaling still targets process groups (`Setpgid` + negative PID) with TERM -> KILL escalation.
- Proxy shutdown is called explicitly by session stop.
- `http.ErrServerClosed` is treated as normal during proxy shutdown.

## Known limitations we should address next

1. Exit reason ambiguity
   - We emit `ProcessSignaled` when stopping and `ProcessExited` when wait returns, but we do not explicitly encode whether exit was natural, requested by user, or forced by kill escalation.

2. Platform parity
   - Non-Unix builds only signal the direct process and cannot reliably kill full process trees.

3. Shutdown ordering is not coordinated
   - process stop and proxy shutdown are both requested, but there is no single orchestrated timeout policy across the whole session.

4. Session completion is implicit
   - no explicit "session done" event or `Wait()` API to know when all workers have quiesced.

5. Message channel lifecycle
   - channel is currently long-lived and not explicitly closed; this is safe for current flow, but not ideal for future composition/testing.

## Proposed future design

### 1) Introduce lifecycle controllers

Add small controller types with clear contracts:

- `ProcessController`
  - `Start(cmd, env) error`
  - `Stop(ctx) error` (TERM then KILL by deadline)
  - `Wait() ProcessResult`
- `ProxyController`
  - `Start(listener) error`
  - `Stop(ctx) error`
  - `Wait() error`

Reasoning:

- encapsulates resource ownership and synchronization
- avoids session-level ad hoc goroutines
- easier to unit-test

### 2) Move to context-driven shutdown

Use a parent context for a session and cancellation for coordinated stop.

Reasoning:

- one source of truth for shutdown intent
- natural propagation to future subcomponents
- easier timeout management than cross-goroutine signal channels

### 3) Add explicit process exit metadata

Define fields such as:

- `ExitReason`: `natural`, `signal`, `forced_kill`, `start_failed`, `runtime_error`
- `Signal`: optional signal value when applicable

Reasoning:

- accurate UI and logs
- better postmortem behavior for flaky commands

### 4) Add session-level graceful stop policy

Implement deterministic sequence with deadlines, for example:

1. request process graceful stop
2. wait up to `X` for process completion
3. force kill process group if needed
4. shutdown proxy with timeout `Y`
5. wait for goroutines/controllers to finish
6. emit `SessionStopped`

Reasoning:

- easier reasoning about final state
- avoids races between TUI exit and backend teardown

### 5) Improve non-Unix behavior

If Windows support becomes a requirement, evaluate job objects or equivalent process-tree control.

Reasoning:

- current direct-process signaling can leak descendants
- parity with Unix lifecycle expectations

## Implementation notes for the next pass

- Keep `internal/process/signal_unix.go` logic as the Unix baseline.
- Rename `Destory` to `Destroy` with a compatibility shim or bulk rename.
- Handle `net.ErrClosed` and `http.ErrServerClosed` as expected proxy shutdown outcomes.
- Add targeted tests:
  - clean exit (`ExitCode=0`)
  - non-zero exit
  - TERM then forced KILL
  - spawned child process cleanup on Unix process groups
  - proxy shutdown does not emit fatal on normal close

## Suggested milestones

1. Controller extraction without behavior change
2. Exit reason metadata and event model updates
3. Context-based coordinated stop
4. Platform-specific process-tree improvements (if needed)
