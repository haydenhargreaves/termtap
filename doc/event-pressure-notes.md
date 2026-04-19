# Event Pressure Notes

This is a quick note on potential event-channel pressure in the current proxy architecture.

## Why this matters

Proxy request handling currently emits events synchronously into a shared channel.
If producers are faster than the consumer (TUI/event loop), the channel can fill and block producers.
When that happens, request handling can stall even if upstream/downstream network paths are healthy.

## Where pressure comes from

- Every request can produce multiple lifecycle events (`started`, `finished`, `failed`, warnings).
- CONNECT + MITM flow can emit both tunnel-level and inner-request events.
- Bursty traffic (many small requests, retries, connection churn) amplifies event rate quickly.

## User-visible symptoms

- Request latency spikes that do not match upstream timings.
- Intermittent pauses during high traffic.
- Shutdown/restart feeling delayed when many events are in flight.

## Current risk profile

- Channel buffer size helps absorb bursts, but only up to a point.
- Backpressure is currently coupled to request path execution, so stalls propagate into proxy behavior.

## Mitigation options

1. Introduce non-blocking event enqueue for low-priority events.
   - Keep critical events blocking (fatal/start/stop), but drop or coalesce high-volume request events under load.
2. Add an internal event relay.
   - Proxy handlers write to a local buffered queue; a dedicated goroutine forwards to the main channel.
3. Coalesce repetitive events.
   - Aggregate similar warnings or per-interval request counters instead of per-request chatter.
4. Add lightweight metrics.
   - Track dropped/coalesced events and queue depth so pressure is visible during development.

## Practical near-term suggestion

Start with a small event relay + drop policy for non-critical request events when queue depth is high.
This contains proxy-path stalls without changing the external event model too much.
