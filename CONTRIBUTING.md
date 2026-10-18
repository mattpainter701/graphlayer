# Contributing

Rust is the required implementation language for GraphLayer's domain engine. Read
[`docs/RUST_ARCHITECTURE.md`](docs/RUST_ARCHITECTURE.md) and the repository
[`AGENTS.md`](AGENTS.md) before changing production code. The architecture
document describes the migration state and the boundaries for compatibility
code.

## Adding or changing domain behavior

Implement vendor parsing and connectors, graph and topology behavior, policy,
routing and NAT, network analysis, validation, and inference in Rust. The Rust
workspace is at the repository root; its application crate is named
`graphlayer-engine`. Keep pure domain decisions in Rust. Go and Python glue may
translate input/output formats or call the Rust engine while compatibility
interfaces are being migrated. Browser JavaScript and TypeScript may render
the results and manage view state.

When porting an existing implementation, preserve its external contract and
compare sanitized golden fixtures, including malformed-input cases. Switch the
caller to Rust and remove the legacy implementation as part of the port. Do
not create a Rust facade that delegates domain decisions back to the old Go or
Python implementation.

## Legacy inventory

The remaining Go and Python domain files are frozen during migration. The
guard in `scripts/check-rust-boundary.py` checks their SHA256 hashes and
rejects new production Go or Python files anywhere under `platform/`.
Deleting legacy files during a port is allowed. A narrow security fix,
correctness fix, or migration adapter must have an exact-path exception that
pins the approved SHA256 and names an owner, reason, and expiry within 90 days
in `config/rust-boundary.json`. Remove exceptions when the Rust port is
complete. Update baseline hashes only as part of a
reviewed migration.

The inventory is not a complete Rust implementation: the Cisco ASA forwarding parser is
the first port, and remaining vendor parsers, analysis code, and Python
connectors still need migration.

## Verification

Use the Rust toolchain pinned by `rust-toolchain.toml` and run the focused
crate tests for the area you changed. The Rust CI workflow runs the workspace
checks. For a legacy-boundary change, run:

```text
python scripts/check-rust-boundary.py
```

Fixtures must not contain credentials, raw customer configurations, or
unredacted logs. Keep examples synthetic or sanitized.
