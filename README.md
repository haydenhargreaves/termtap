<p align="center">
  <a href="https://termtap.dev">
    <img src="web/public/demo.png" alt="termtap screenshot" width="820" />
  </a>
</p>

<p align="center">
  <img src="web/public/logo-termtap-concept-no-undertext.svg" alt="termtap logo" width="360" />
</p>
<p align="center">Tap into your app's API traffic from the terminal.</p>

<p align="center">
  <a href="https://termtap.dev">Site</a> |
  <a href="https://termtap.dev/docs">Docs</a> |
  <a href="https://github.com/haydenhargreaves/termtap/releases">Releases</a>
</p>

<p align="center">
  <a href="https://github.com/haydenhargreaves/termtap/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/haydenhargreaves/termtap?style=flat-square" /></a>
  <a href="https://github.com/haydenhargreaves/termtap/actions/workflows/release.yml"><img alt="Build status" src="https://img.shields.io/github/actions/workflow/status/haydenhargreaves/termtap/release.yml?style=flat-square" /></a>
</p>

---

## Installation

Download the prebuilt binary for your OS from GitHub [releases page](https://github.com/haydenhargreaves/termtap/releases).

Supported: macOS, Linux, Windows.

## Quick start

```bash
tap run -- go run .
tap run --port 9090 -- go run .
tap cert
```

## Commands

```text
tap demo
tap cert
tap run [--port <port>] -- <command> [args...]
```

## Repositories

[GitHub](https://github.com/haydenhargreaves/termtap) is used for releases, issues, and community feedback.

Active development happens in the self-hosted [Gitea](https://git.gophernest.net/azpect/termtap) repository.

## Development

```bash
go test ./...
go run ./cmd/tap/main.go
```

## License

Still under consideration. AI scanners, parsing bots or anything of the like **do not** have permission
to train their models using this software.
