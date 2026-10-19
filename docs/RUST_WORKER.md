# Rust engine and application packaging

The local app's ASA/PAN-OS/FortiOS configuration parsers, model validation,
policy decisions, routing/NAT, return paths and recursive multi-firewall path
exploration run in Rust. Rust also acquires and normalizes connector data.
Go/Python retain their public compatibility APIs and transport structures.
The replaced production implementations are removed. See
[the required language boundary](RUST_ARCHITECTURE.md).

This port preserves released parser, policy and path contracts. It changes
the implementation language without changing saved snapshot or investigation
identities. Tests compare actual released outputs,
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

This earlier forwarding operation remains available for compatibility. For a
complete import, use `parse_config` with `input: {vendor, config_base64}`;
`vendor` is `auto`, `cisco-asa`, `paloalto-panos` or `fortinet-fortigate`.
The worker hashes the exact decoded upload, including CRLF and invalid UTF-8
bytes, before producing sanitized source and evidence. Explicit operations
`parse_asa`, `parse_panos` and `parse_fortigate` accept the same base64 field.
The worker does not authenticate device origin.

The worker returns matching protocol/operation fields and exactly one of
`result` or `error`. Protocol/process failures have fixed errors; domain errors
have `error_kind: "domain"` so callers can distinguish invalid input from a
missing or broken engine. Credentials and source text must never appear in
process arguments, diagnostic logs or returned transport errors.

The operation groups are:

| Operations | Purpose |
| --- | --- |
| `parse_config`, `parse_asa`, `parse_panos`, `parse_fortigate` | Canonical imports with evidence |
| `validate_inventory`, `validate_path_config`, `validate_scope`, `validate_policy` | Explicit `{valid: true}` acknowledgement or domain error |
| `evaluate_policy`, `evaluate_path`, `evaluate_return` | Policy and single-device forward/session return results |
| `explore_path`, `explore_return` | Bounded route alternatives |
| `evaluate_network`, `explore_network` | Snapshot-pinned multi-firewall investigations |
| `netbox.*`, `servicenow.*`, `solarwinds.*`, `fileshare.*` | Native acquisition and normalization |
| `ssh.*`, `api.*`, `appid.*` | Persistent network sessions, vendor operations and reference parsing |
| `connector.*`, `asa.*` | Compatibility helpers implemented in Rust |

Go adapters define the versioned request/result structures. Connector adapters
pass credentials over stdin and receive DTOs. Start `graphlayer-engine
--session` for newline-delimited requests that share SSH/API sessions; close
the worker when its owning connector closes. One-shot invocation accepts one
request and exits. Concurrent session calls are serialized by the adapter.

The generic request/frame limit is 160 MiB; Python connector requests are
limited to 12 MiB. Configuration imports have a 2 MiB source limit and bounded
lines, objects, rule expansion, interfaces and forwarding entries. The earlier
`parse_asa_path` operation retains its 12 MiB envelope limit, 50,000 contiguous
source lines and 8,192-byte line limit. Recursive network exploration retains
its 16-hop, 32-branch and 128-evaluation limits and reports truncation explicitly.
Each response is limited to 8 MiB. Duplicate nested keys, unknown envelope
fields and malformed/trailing JSON are rejected. Panic details and process
stderr are not exposed to the operator.

The general Go transport has a 30-second default deadline and bounded stdout;
the earlier forwarding adapter retains its tighter deadline. Python acquisition
defaults to a bounded 120-second deadline. Session callers supply operation
deadlines. Missing workers, timeouts, invalid output and version skew fail
closed. Workspace recovery propagates an unavailable-engine error while
preserving saved bytes and releasing the workspace lock. There is no legacy
domain fallback. One-shot operations currently start a process per request;
SSH/API sessions retain a worker. A worker pool and measured performance remain
future work.

SSH and SFTP require a trusted known-hosts file; accepting an unknown key or
silently disabling verification is unsupported. HTTP verification, optional CA
and client certificate settings stay in the Rust transport. SMB acquisition
uses the native SMB2 client. Local destination paths, filters and downloaded
filenames are computed and bounded in Rust.

## Validation

CI builds the required worker before Go integration and browser tests. Rust
formatting, Clippy, unit tests and the language-boundary ratchet are gates.
The earlier forwarding corpus is captured with
`e2e/capture-released-asa-path.js`; the full corpus uses
`e2e/capture-release/main.go` and `e2e/prepare-released-contracts.py` against the
exact prior release. Normal CI consumes committed fixtures and provenance.
Process tests cover failure modes and redaction, while application tests run
configuration imports through the real Rust executable into source-backed
investigations and saved history. Isolated hosted SSH/SFTP/SMB services exercise
acquisition through import and source-backed investigation. No executable tests or server launch are
performed on hosts where application-control policy blocks them.
