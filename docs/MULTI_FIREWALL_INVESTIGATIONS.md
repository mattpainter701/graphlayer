# Multi-firewall path investigations

GraphLayer can compose its supported ASA IPv4 path model across explicitly linked configuration snapshots. The investigation shows forward and return hops, original/policy/egress tuples, ACL decisions, selected routes, NAT translations and physical source evidence. This is a configuration model with an explicit established-session assumption, not a live connectivity test.

## Run the workflow

Start the local app using the [launch guide](FIREWALL_INVESTIGATION.md). Choose **Load multi-firewall sample**, which imports two synthetic ASA configurations using the normal parser. The editor pins each device ID to an immutable snapshot and supplies a link between `edge/transit` and `core/transit`.

The source endpoint is the ingress interface on the first firewall. The destination endpoint is the terminal egress interface on the last firewall. These are interface boundaries; the engine does not discover hosts, links or intervening routers. For your own configurations, import each device, add a uniquely named device row, select its snapshot, and enter each direct interface pair. Links are operator assertions checked against configured interface addresses and next hops. They are not discovered adjacency evidence.

The sample TCP flow is `10.1.0.42:52000 → 203.0.113.15:443`:

| Direction / firewall | Configuration-derived behavior |
|---|---|
| Forward / edge | Inside ACL permits; static route uses core's transit address; source translates to `198.51.100.42` |
| Forward / core | Destination untranslates to `10.2.0.15`; ingress ACL matches that real address; inside connected route reaches the terminal network |
| Return / core | Assumed established reply starts at `10.2.0.15:443 → 198.51.100.42:52000`; route selects edge; source restores to `203.0.113.15` |
| Return / edge | Destination untranslates to `10.1.0.42`; inside connected route reaches the original source network |

The sample deliberately has deny rules on the return ingress interfaces. Reply traffic is analyzed under the displayed established TCP/UDP session assumption from the forward permit. A separately initiated connection in that direction would require a new ACL evaluation.

Inspect both directions and open source links at each stage. Evidence is qualified by snapshot ID so identical evidence IDs from different devices cannot select the wrong configuration. Saved history restores the full device/snapshot selection, links, endpoints and tuple. JSON export contains the exact request and saved result. A workspace restart replays the investigation against every pinned snapshot before making it available.

## Interpret the result

- **Modeled round trip:** all supported forward and reverse stages agree, using the same firewalls and reverse links with exact restoration of addresses and ports. This assumes configuration completeness, the operator-supplied physical connections and an established TCP/UDP session on each forward firewall.
- **Blocked:** a supported configuration condition prevents a direction, including an ACL deny in the forward path or no matching route in a complete supported model. The trace stops and retains the blocker.
- **Indeterminate:** a required fact is absent or unsupported. Examples include missing links, next-hop mismatch, cycles, ambiguous routes, unsupported NAT, an alternate reverse interface, or a protocol without the supported session model.

Forward success and return failure are reported separately. No final flow is claimed for an incomplete direction. Interface configuration and route facts do not prove link/ARP/neighbor state, live session state, host response, application behavior or device origin.

## Supported boundary

The [single-firewall routing/NAT contract](ROUTING_NAT_CONTRACT.md) applies independently at every hop: native IPv4, connected/static routes, explicit ingress ACLs, bounded one-to-one static object NAT, and conservative unsupported semantics. Different devices can apply source and destination NAT in sequence; simultaneous source/destination NAT on one firewall remains outside the supported subset.

Inter-firewall handoff requires a configured static route whose next hop is the linked peer's enabled interface address, with consistent connected subnets at both ends. A connected route alone does not prove that a different firewall receives the packet. The initial source and final translated destination must be on their declared endpoint networks. Interface addresses, network/broadcast addresses and local control-plane traffic are not transit endpoints.

Return analysis supports TCP and UDP under the explicit state assumption. It requires reverse tuple continuity, the inverse of the selected forward NAT rules and the same interfaces/links in reverse. Asymmetry is identified as uncertainty; the app does not infer shared state or TCP state bypass. Dynamic routing, ECMP branch exploration, dynamic/PAT/service/twice NAT, tunnels, HA state, overlapping routing domains and mixed-vendor paths remain future work.

Cisco describes [stateful forward/reverse flows](https://www.cisco.com/c/en/us/td/docs/security/asa/asa91/asdm71/general/asdm_71_general_config/intro_intro.pdf) and [the additional state-bypass requirements of asymmetric ASA paths](https://www.cisco.com/c/en/us/support/docs/security/asa-5500-x-series-next-generation-firewalls/118995-configure-asa-00.html). These sources motivate the distinction between configured reverse routing and assumed session state; GraphLayer implements a narrower subset.

## API, storage and limits

`GET /api/sample/network` returns the labeled configurations and topology. `POST /api/network-investigations` accepts `nodes` (`node_id`, `snapshot_id`), `links` (endpoint pairs `a`/`b`), `source`/`destination` endpoints (`node_id`, `interface`) and a concrete `flow`. The server derives each hop's scope from its snapshot; callers cannot supply policy scope or replace the stored configurations. `GET /api/network-investigations` lists saved summaries, and `GET /api/network-investigations/{id}` returns the exact saved record for replay/export.

The network result contract is `graphlayer-network-path-v1`. Each direction contains ordered hops and a disposition; each hop contains the single-device stage trace and qualified evidence. Return stage explanations identify the session assumption and forward evidence rather than inventing a matching return ACE. A `modeled_forward` disposition inside the `return` field means the configured reply direction succeeds under those assumptions.

New disk records use workspace-v4. Released workspace-v1/v2/v3 records are read and validated without rewriting them; existing ACL/path result contracts retain their original behavior. Back up a workspace before upgrading: old executables cannot open a workspace after new-format records are added. Record checksums detect corruption and replay inconsistency, not malicious authorship or device authenticity.

An investigation accepts 1–16 distinct snapshots, at most 32 links and at most 16 traversed firewalls per direction. Node IDs are 1–64 ASCII letters, digits, periods, underscores or hyphens. Interface names are bounded by the ASA model. Repeated link endpoints and cycles do not produce a success claim. The existing 32-snapshot, 2 MiB/50,000-line import and 8 MiB artifact limits remain. ACL, single-firewall and multi-firewall investigations share a 256-record history quota. Oversized results fail without a partial saved record.

Release checks include engine cases, real parser/API/store workflows, released workspace fixtures, corrupted trace rejection, browser forward/return/evidence/history/export flows, stale response handling and mobile layout. The supported Go, race, Windows-native, vet and browser gates include the new engine; the coverage threshold remains 70%.
