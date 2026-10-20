# Multi-layer topology and the scale lab

Open **Network map** and choose **Load 1,000-host lab**. This transient lab is
separate from the active saved dataset. It never changes imported snapshots,
source hashes, evidence IDs or saved investigation records. Every lab record is
synthetic; these records are examples, not observations of a customer network.

The corpus contains 1,000 deterministic hosts distributed evenly across ten
private IPv4 /24 networks, four named zones and explicit access/distribution
switches. Two synthetic firewall configurations exercise the existing Rust
PAN-OS and FortiOS parsers. Auxiliary records supply facts a firewall
configuration cannot establish alone: host addresses, locally administered MAC
addresses, VLANs, switch ports, attachments and declared uplinks.

## What each view establishes

| View | Shows | Boundary |
| --- | --- | --- |
| Host & L2 | A bounded page of hosts, switch attachments and explicit VLAN links; selected host-path hops and ports | A modeled path uses explicit same-VLAN links. It does not prove live forwarding, STP state or packet delivery. Different network/VLAN endpoints require L3 analysis. |
| Layer 3 | Subnet aggregates, host counts, gateways and declared/configuration-backed device relationships | An aggregate map does not evaluate firewall policy, NAT or all possible routes. Use Network path for the existing saved routing/NAT investigations. |
| Zones | Zone aggregates and explicit cross-zone topology relationships when supplied | Zone membership and physical/configured relationships do not establish policy permission. Missing links are not invented. |

Click a zone to narrow the Layer 3 view, then a subnet to open host detail.
Search reaches the entire matching inventory, including hosts outside the
current page. Host paths show source references; a route-required or
indeterminate result must not retain an earlier successful path overlay.

## Source and model ownership

`topology.layer_sample` generates the corpus. `topology.project_layers` validates
the supplied dataset, resolves source ownership, filters and aggregates the
graph, and evaluates bounded L2 paths. Both operations run in Rust. The HTTP
adapter verifies the local session, CSRF token and JSON/body limits, then calls
the required worker. JavaScript renders the returned graph and manages view
state; it does not implement network decisions.

The auxiliary source records include physical line numbers and SHA-256 hashes.
Host and link facts must agree with their referenced records. Configuration
relationships must agree with the parsed configuration. Hashes identify bytes;
they do not authenticate a device, collector or publisher. All exported lab
sources remain explicitly synthetic.

**Export dataset** downloads the generated corpus, including its two
configuration sources and auxiliary records, for inspection and reuse in tests.
The current lab is transient: reload it to regenerate the same identities.
This milestone does not add a production auxiliary connector or migrate the
saved-workspace format.

## Scale verification

The map uses aggregation and bounded host pages rather than placing 1,000 host
cards in the browser at once. Rust enforces input/model limits and uses an
adjacency traversal over explicit links for L2 investigations. Pagination does
not drop hosts from inventory counts or prevent search and endpoint selection.

`topology_layers_scale` measures five projections per view, reports median and
maximum duration plus corpus/response byte counts, and asserts bounded node,
host and response sizes. CI runs it in release mode with a generous five-second
failure budget. Browser tests exercise the real Rust worker, graph element
counts, pagination, late-host search, paths, source evidence, export and mobile
layout. Recorded timings describe those CI hosts and this synthetic corpus;
they are not a claim of production throughput at larger scales.

Run the focused engine measurement on a host that permits the required worker:

```sh
cargo test --locked --release -p graphlayer-engine --test topology_layers_scale -- --nocapture
```

Run browser coverage with the built worker configured as described in
[RUST_WORKER.md](RUST_WORKER.md). These tests do not disable App Control or
establish that unsigned Windows builds are trusted by Smart App Control.
