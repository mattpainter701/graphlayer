# Multi-vendor path exploration

GraphLayer explores bounded configured route alternatives across snapshot-pinned ASA, PAN-OS and FortiOS firewalls. Each forward branch keeps its own policy decision, NAT tuple and source evidence. Its return paths follow the configured routes independently, including routes through other firewalls. A route candidate does not establish live connectivity or session admission.

## Run the complete workflow

Start the local executable using the [launch guide](FIREWALL_INVESTIGATION.md). In the Network path section choose **Load branch sample**, review the four imported native configurations and declared links, then choose **Explore branches**. The sample uses the normal import APIs and vendor parsers.

The diamond has two equal-best PAN-OS static routes. A flow from `10.1.0.42:52000` to `203.0.113.15:443` receives source NAT to `198.51.100.42`, traverses either the ASA `north` or FortiGate `south`, and reaches a FortiGate `core` VIP mapping the destination to `10.2.0.15`. The core's return route goes through south. The south forward branch can model the exact symmetric return under its stated session assumption. The north branch's configured reply traverses a firewall absent from that branch's forward evidence; its state remains unproven and the aggregate result remains indeterminate.

Select a forward branch and return alternative to compare hops and tuples. The topology distinguishes supplied links from selected forward/return paths, highlights blockers and uncertainty, and supports keyboard node focus alongside a list. Selecting a node focuses its hop evidence; physical source links retain the owning snapshot. History restores the full pinned topology and flow, and JSON export contains the exact saved request/result.

The existing **Trace network path** workflow remains available with its released strict single-path contract. Exploration is a separate result type; opening older saved results does not rerun them through a newer contract.

## Route choices and search coverage

Only equal-best connected/static IPv4 routes are alternatives: longest prefix first, then lowest supported administrative distance. Worse routes are not treated as active alternatives. Vendor policy and NAT stages still govern each candidate, including PAN-OS original-address security rules and pre/post-NAT zones, FortiGate VIP dependencies, and ASA real-address ACL handling. See the [native format and NAT boundaries](MULTI_VENDOR_INVESTIGATIONS.md).

PAN-OS pre-NAT and post-NAT route choices are bounded; repeated lookups for the same destination keep a consistent selected route. Each chosen route retains its configured ID and source lines. Shared-subnet links may reuse an interface endpoint only when the selected static next hop proves exactly one linked peer. Missing peers, duplicate/ambiguous peer addresses, unsupported route inputs and cycles remain uncertainty.

These are configured route alternatives. GraphLayer does not prove ECMP activation, calculate a device's hash, predict session pinning, simulate link failure or activate a backup route. Palo Alto documents that [ECMP selection occurs per session](https://docs.paloaltonetworks.com/ngfw/networking/ecmp/ecmp-load-balancing-algorithms); the supplied evidence here does not establish that runtime choice.

Search limits are eight local route candidates, 32 terminal branches and 128 hop candidate evaluations across both directions, with 16 hops per direction, 16 snapshots/nodes and 32 declared links. Omitted alternatives make truncation explicit and prevent an overall round-trip verdict. Search coverage means the bounded candidate set was explored; it does not mean all configuration facts or session state were resolved. Results also remain within the existing 8 MiB artifact budget and shared 256-investigation history quota. An oversized record is rejected without partial publication.

The aggregate is a modeled round trip only when all explored alternatives establish supported exact symmetric returns and coverage is complete. Distinct permit, block and unknown branches remain visible. A reachable branch cannot hide a blocked or unresolved alternative, and a truncated search cannot establish a universal verdict.

## Asymmetric return analysis

A supplied forward permit is revalidated against the same immutable configuration before it can support an established TCP/UDP return assumption. Selected forward translations are reversed exactly; established reply analysis does not select fresh NAT rules. A concrete alternate egress may be traversed as a configured return candidate while its session status remains uncertain.

An unfamiliar return firewall can contribute supported no-NAT routing and fresh-policy evidence, but that evidence does not establish an existing session. Such uncertainty remains sticky through later firewalls. Missing forward NAT state prevents tuple invention. Different node/interface/link sequences or non-inverse tuples prevent a symmetric round-trip claim. The model does not infer shared state, HA session synchronization, state bypass or asymmetric inspection support. Cisco's [connection settings reference](https://www.cisco.com/c/en/us/td/docs/security/asa/asa922/asdm722/firewall/asdm-722-firewall-config/conns-connlimits.html) explains why traffic crossing different ASA devices can require separate state-bypass configuration; GraphLayer does not assume that configuration.

## API, persistence and verification

`POST /api/network-explorations` accepts the same bounded topology/flow request shape as `/api/network-investigations`. It returns `{exploration: {id, created_at, request, result}}`. `GET /api/network-explorations` lists summaries and `GET /api/network-explorations/:id` returns the exact saved record. The sample endpoint is `/api/sample/network-exploration`. Local session/CSRF, strict JSON, immutable snapshot scope, private single-writer storage and existing request limits apply.

The additive result contract is `graphlayer-network-exploration-v1`. Workspace-v6 stores the new record kind and verifies its identity and exact replay on reopen. Released workspace-v1 through v5 records remain readable without byte rewriting or changing their original policy/path/network contracts. Back up before upgrading: older executables cannot open new-format records.

Verification covers parser-to-store-to-UI branching and return routes, source ownership, mixed branch outcomes, NAT tuple continuity, peer ambiguity, loops, budgets, deterministic immutable results, forged replay, native format limits, history/export, late responses and actual 390px topology layout. Hosted Linux coverage/race and Windows-native tests include the exploration packages. Local host application-control restrictions are preserved; no alternate launch path is required by this feature.
