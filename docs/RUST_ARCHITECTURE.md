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
toolchain pinned in `rust-toolchain.toml`. Rust owns the configuration and
investigation workflows shipped by the local operator application:

| Domain | Rust implementation | Compatibility layer |
| --- | --- | --- |
| ASA running configurations, ACLs, nested network/service objects and forwarding inventory | `asa_config`, existing ASA path parser | Go public structures and embedded public samples |
| PAN-OS set-format and FortiOS CLI configurations | `vendor_parsers` | Go parser entry points |
| Inventory and forwarding model validation | `model_validation` | Go model structures |
| Ordered policy decisions and predicate traces | `investigation_engine::policy` | Go request/result structures |
| Routing, NAT, session return paths and route alternatives | `investigation_engine::path`, `vendor_path` | Go API adapters |
| Multi-firewall forward/return paths and recursive branch exploration | `investigation_engine::network`, `network_explore` | Go topology request structures |
| NetBox, ServiceNow, SolarWinds, SFTP and SMB acquisition | `connectors` | Python connector API adapters |
| Persistent SSH/API sessions, vendor commands, App-ID and ISDB collection | `connectors::ssh`, `api`, `appid` | Go connector API adapters |

The worker does not call Go or Python to make domain decisions. Go retains the
HTTP server, workspace transactions, checksums, history, source lookup and
versioned serialization. Python retains the public connector classes and
registry. The browser renders the returned topology, branches and evidence.

The older experimental vendor libraries, graph analytics and orchestration
prototypes outside these shipped workflows remain frozen in
[`config/rust-boundary.json`](../config/rust-boundary.json). They are not
additional Rust-supported vendors or part of the investigation app's runtime.
Do not enable or extend one of these prototypes without first porting its
behavior to Rust. A tracked legacy file is not evidence of a completed port.

Saved workspace versions 1 through 7 keep their released parser versions,
identities, sanitized source lines and evidence references. Opening a workspace
replays its existing contracts rather than rewriting history to a new model.
Worker failure is reported as an unavailable analysis engine and must never be
treated as corrupt history or trigger a fallback parser.

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

The committed released corpus contains 1,764 distinct parser, validation,
policy, path, return and network exploration contracts captured from release
`418e550f173938bad7dc7d86995d4a12945f9f23`, plus the earlier 72 ASA forwarding
contracts. Compare ordered arrays, null versus absent fields, exact error text,
evidence identities and sanitized source bytes. Do not regenerate expectations
from the replacement implementation to make a port pass. Genuine released
workspace fixtures cover versions 1 through 7; replay must leave every file
unchanged, including after an unavailable-worker failure.

PAN-OS top-level evidence, interface inventory and connected routes were
appended during Go map iteration. Corpus pairs 867/869 and
873/874 have identical inputs and evidence objects in different orders. The
parity comparator sorts only these map projections, retaining duplicates and exact
objects. Static route order, all references, source lines, rules, stages and other arrays remain
strict. The captured fixture bytes remain unchanged.

Hosted CI exercises the real Rust binary through Go, Python, the browser and
the two-executable Windows package. Connector acquisition tests use isolated
HTTP, SSH, SFTP and SMB services with ephemeral credentials. Host-key failures,
invalid credentials, output bounds and process deadlines are tested failures,
not reasons to select a legacy implementation.

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
