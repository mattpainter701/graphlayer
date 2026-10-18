# Rust forwarding parser and application packaging

The ASA interface, connected/static route, static object NAT, completeness,
warning and forwarding evidence parser runs in Rust. The production Go
implementation of that parser has been removed. Go still normalizes ASA ACLs
and supplies network object summaries; PAN-OS/FortiOS parsing, path evaluation,
graph analysis and Python connectors remain legacy migration work. See
[the required language boundary](RUST_ARCHITECTURE.md).

This port preserves the `asa-config-v3` and `asa-path-v1` data contracts. It
changes the implementation language without changing released snapshot or
investigation identities. Tests compare actual released parser outputs,
including ordered arrays, warning text, evidence IDs and sanitized lines.
Language choice alone is not a measured scalability or latency result.

## Build and launch

Install the Rust toolchain pinned in `rust-toolchain.toml` and Go from
`platform/go.mod`. From the repository root:

```powershell
cargo build --locked --release -p graphlayer-engine
New-Item -ItemType Directory -Force dist | Out-Null
go -C platform build -trimpath -o ../dist/graphlayer-investigate.exe ./cmd/graphlayer-investigate
Copy-Item target/release/graphlayer-engine.exe dist/graphlayer-engine.exe
.\dist\graphlayer-investigate.exe --listen 127.0.0.1:8088
```

For development with `go run` or `go test`, set `GRAPHLAYER_ENGINE` to the
absolute path of the built Rust executable. Packaged applications locate an
exact `graphlayer-engine` / `graphlayer-engine.exe` sibling of the application;
they do not search PATH. Both executables are required. CI produces a Windows
bundle with SHA256 sums and exercises real imports on Linux and Windows.

`Dockerfile.investigate` packages both executables. On Linux use host networking
to access this intentionally loopback-bound service; publishing a Docker bridge
port does not expose a loopback listener. Persist its private workspace on a
volume writable by UID 10001. This is a local single-user app, not a shared
service or an authentication boundary.

## Process contract

The one-shot worker receives exactly one JSON object on stdin:

```json
{
  "protocol_version": "graphlayer-engine-v1",
  "operation": "parse_asa_path",
  "input": {
    "artifact_sha256": "<64 lowercase hex characters>",
    "lines": [{"number": 1, "text": "<source line>"}],
    "network_objects": {
      "<name>": {"group": false, "members": [], "unsupported_lines": []}
    }
  }
}
```

This is an internal parser boundary. The Go compatibility layer derives the
hash and object projection from the exact uploaded bytes. The worker does not
authenticate device origin. It returns matching protocol/operation fields and
`result: {path, evidence, source_line_updates}`, or a fixed `error` code.
The response contains normalized forwarding data and sanitized evidence only.

Source lines are contiguous and numbered from one. Input is bounded to 12 MiB
of encoded JSON, 2 MiB of normalized source, 50,000 source lines and 8,192 bytes
per line. The worker bounds interfaces to 1,024 and routes/NAT rules to 4,096
each, marking coverage incomplete on truncation. Output is bounded to 8 MiB.
Unknown fields and malformed/trailing JSON are rejected. Raw input stays on
stdin, never in command arguments, environment values, files or diagnostic logs.
Panic details and process stderr are not exposed to the operator.

The Go transport applies a ten-second deadline and a bounded stdout buffer,
validates the returned protocol/model/evidence/source updates and reports fixed
errors. Missing workers, timeouts, invalid output and version skew fail the
import. There is no fallback Go forwarding parser. Requests currently use one
process per parse within the app's bounded import concurrency; a worker pool
and measured large-input performance are later scalability work.

## Validation

CI builds the required worker before Go integration and browser tests. Rust
formatting, Clippy, unit tests and the language-boundary ratchet are gates.
The released corpus is captured with `e2e/capture-released-asa-path.js` from
the actual prior Go implementation; normal CI consumes committed fixtures.
Process tests cover failure modes and redaction, while application tests run
configuration imports through the real Rust executable into source-backed
investigations and saved history. No executable tests or server launch are
performed on hosts where application-control policy blocks them.
