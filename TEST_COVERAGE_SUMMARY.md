# termtap Test Coverage Summary

Generated from:

```bash
go test -coverprofile=/tmp/termtap.cover ./...
go tool cover -func=/tmp/termtap.cover
```

## Package coverage snapshot

| Package | Coverage |
|---|---:|
| `cmd/tap` | 100.0% |
| `internal/app` | 98.1% |
| `internal/cli` | 93.4% |
| `internal/process` | 95.8% |
| `internal/proxy` | 90.0% |
| `internal/tui` | 96.2% |
| `examples/echo` | 0.0% (example app; intentionally not covered) |
| `internal/model` | no test files (pure data structs) |

Total statements in module: **77.9%**.

## Notable lower-coverage targets (production code)

- `internal/proxy/handlers.go:handleConnect` — 75.9%
- `internal/proxy/certs.go:writeFileAtomically` — 76.2%
- `internal/proxy/certs.go:load` — 90.5%
- `internal/proxy/certs.go:create` — 80.8%
- `internal/proxy/certs.go:IsTrustedBySystem` — 76.9%
- `internal/cli/run.go:runCert` — 87.5%

## Interpretation

- Core runtime paths (`internal/app`, `internal/process`, `internal/tui`) are high confidence.
- Proxy package has broad behavior coverage including HTTP and HTTPS MITM integration flow, and now clears 90% package coverage.
- CLI command routing and fatal/stdout/stderr seams are covered, including `Run` success/error branches.
- TUI pane rendering coverage now includes error/PID branch behavior.

## Next optional improvements

1. Add deeper CONNECT tunnel write/flush/read failure permutations inside `handleConnect` loop.
2. Add additional deterministic edge cases for `IsTrustedBySystem` non-unknown-authority verify failures.
3. Optionally add non-unix signal file coverage in CI matrix (currently unix-focused).
