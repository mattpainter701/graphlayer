# Rust architecture and language boundary

Rust is the implementation language for GraphLayer's domain engine. That
includes vendor configuration parsing and connector behavior, graph and
topology construction, policy and route evaluation, NAT, network path
analysis, validation, inference, and performance-sensitive algorithms. Rust
implementations own the rules and decisions; other languages may call them and
adapt their inputs and outputs.

The browser layer may manage view state and render data. Go HTTP, storage, and
CLI code may remain as compatibility infrastructure while migration proceeds.
These layers may perform transport, persistence, presentation, and format
translation. They must not contain a second implementation of Rust domain
behavior. A Rust command or API that forwards work to the old Go or Python
engine does not satisfy the boundary.

The root Cargo workspace contains `crates/graphlayer-engine` and uses the
toolchain pinned in `rust-toolchain.toml`. The first port is the Cisco ASA
forwarding inventory parser. It establishes the Rust-owned engine boundary and a
fixture-based way to compare behavior. This is a first migration slice, not
completion of the Rust runtime. Other Go vendor parsers and analysis packages,
along with the Python connectors, remain to be ported. The legacy inventory is
frozen and tracked by
[`config/rust-boundary.json`](../config/rust-boundary.json); its hashes describe
grandfathered files only.

## Migration sequence

Move each domain slice by implementing its behavior in Rust, preserving its
public input and output contract, and checking representative fixtures against
the prior implementation. Switch callers to the Rust implementation, then
remove the old implementation and update the reviewed legacy inventory. Port
all vendors and Python connectors under this sequence. Do not leave a facade
that calls legacy code and label the slice migrated.

Golden fixtures should be sanitized and stable. Tests should cover parsing,
normalization, error cases, and decisions at the language boundary. Raw
customer configurations, credentials, and unredacted logs must not enter the
repository.

## Exceptions and enforcement

The repository policy in [`AGENTS.md`](../AGENTS.md) applies to all new
production code. `scripts/check-rust-boundary.py` enforces a line-ending
normalized SHA256 ratchet for the existing Go and Python domain files and
rejects new Go or Python production files anywhere under `platform/`. A
deliberate security fix, correctness fix, or migration adapter may use an
exact-path exception in `config/rust-boundary.json`; every exception must pin
the approved SHA256 and state an owner, reason, and expiry within 90 days.
Remove the exception when the
work is ported or the fix is incorporated into Rust. Deleting a legacy file
during a port is allowed.

After a completed port removes or replaces legacy implementation, update the
manifest hashes in the same reviewed change. Do not refresh hashes merely to
make an unrelated legacy edit pass. CI invokes the guard so the boundary is
checked on every change.
