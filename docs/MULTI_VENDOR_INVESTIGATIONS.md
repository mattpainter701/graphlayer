# Multi-vendor path investigations

This guide records the released workspace-v5 milestone. [Path exploration](PATH_EXPLORATION.md) adds configured route branching, asymmetric return candidates and a topology view in workspace-v6, while preserving these native adapters and saved results.

GraphLayer imports a bounded subset of native Cisco ASA, Palo Alto PAN-OS and FortiGate FortiOS configuration exports into snapshot-pinned forward/return investigations. Each hop uses its vendor's policy and translation semantics. The model uses supplied configuration and operator-declared links; it does not query devices or establish live connectivity.

## Operator workflow

Use the [launch guide](FIREWALL_INVESTIGATION.md), then choose **Load multi-vendor sample**. Three synthetic configurations pass through the actual import API and parser:

1. Palo Alto `edge`: `10.1.0.42:52000 → 203.0.113.15:443` matches the trust-to-untrust policy, a static route and static source NAT to `198.51.100.42`.
2. ASA `transit`: the translated tuple crosses an explicit link and the selected static route, without additional translation.
3. FortiGate `core`: a policy-bound VIP translates `203.0.113.15` to `10.2.0.15`; interface policy authorizes the real destination and the connected terminal network.

The return trace assumes an established TCP/UDP session admitted by those forward permits. It checks configured reverse routes and links, restores the selected forward translations, and requires the original interface pairs and exact reverse tuple. Fresh reverse connections are separate policy questions. The trace does not infer bidirectional PAN-OS NAT, shared state, session synchronization or asymmetric state bypass.

Inspect vendor labels, original/policy/egress tuples, route and NAT stages, and source links for all three devices. Evidence references carry their snapshot ID. The source viewer shows sanitized, allowlisted commands at their original physical line numbers; unknown commands and sensitive fields are omitted. Saved history restores all pinned snapshots, topology, endpoints and flow, and JSON export contains the exact request and result.

For your own configurations, select a format or use native-format detection. Import every device, select its immutable snapshot in the network editor, supply direct interface pairs, and choose source ingress and terminal egress interfaces. Configured next hops must equal the linked peer interface address on a consistent enabled subnet. Missing links, ambiguous facts and cycles remain uncertainty.

## Supported configuration formats

| Vendor | Supported input | Initial policy / NAT subset |
|---|---|---|
| Cisco ASA | Running-config text | Existing attached ingress ACL, connected/static IPv4 route and one-to-one static object NAT contract |
| Palo Alto PAN-OS | Native `set` configuration commands for one local `vsys1` / virtual router | L3 Ethernet interfaces and zones, IPv4 address/custom service objects, ordered security rules with application `any`, ordered static source or destination NAT |
| FortiGate FortiOS | Single-VDOM native `config` / `edit` / `set` / `next` / `end` CLI text | Static IPv4 physical interfaces/routes, subnet addresses/custom services, ordered interface policies, static VIP destination NAT and policy-bound single-address one-to-one source pools |

PAN-OS XML and Panorama inherited/pre/post rulebases are outside this input adapter. Export native set-format commands rather than relabeling XML as supported. FortiOS multi-VDOM, central NAT, SD-WAN, dynamic routing and policy routing are outside this subset. Composed source and destination NAT on the same vendor hop and same-interface hairpin forwarding remain indeterminate. Unsupported or incomplete relevant commands do not become wildcard permits.

All adapters retain physical line identity and original-upload SHA-256 while persisting sanitized source. A checksum identifies bytes; it does not authenticate a device or establish that a supplied fragment is complete. Use a complete supported export. Parser versions and path coverage/warnings are visible in the snapshot.

## Vendor semantics

**PAN-OS:** the original destination route determines the original destination zone used for NAT matching. NAT rules are considered in configured order; the first supported matching rule selects a static translation or identity behavior. After destination translation, routing supplies the actual egress zone. Security policy uses original source/destination addresses with the post-NAT zone pair. Source translation is applied after the policy decision. A policy that matches only the real translated destination cannot authorize a PAN-OS DNAT flow whose security rule requires its original published address. These distinctions follow Palo Alto's [NAT policy rules](https://docs.paloaltonetworks.com/ngfw/networking/nat/nat-policy-rules) and [one-to-one destination NAT example](https://docs.paloaltonetworks.com/ngfw/networking/nat/destination-nat-exampleone-to-one-mapping).

**FortiOS:** a static VIP is selected for its external interface/address and retained as an explicit policy dependency. Routing follows the translated destination; the interface policy must reference the selected VIP. A plain allow rule cannot silently authorize that VIP. Policy SNAT is tied to the actual selected policy and its supported one-to-one pool; interface PAT and dynamic pool allocation are not invented. See Fortinet's [static VIP guide](https://docs.fortinet.com/document/fortigate/8.0.0/administration-guide/510402/static-virtual-ips). VIP overlap, unsupported matching controls and central NAT remain unresolved.

The source-pool subset requires one literal client host, one pool address, type `one-to-one` and explicit `arp-reply disable`. Default/enabled pool ARP behavior can make even an unused pool address local to the firewall, depending on FortiOS version; those configurations remain indeterminate. Reusing a pool address for distinct client hosts also remains indeterminate because allocation is runtime state. See Fortinet's [dynamic SNAT guide](https://docs.fortinet.com/document/fortigate/7.4.4/administration-guide/29961/dynamic-snat) and [one-to-one pool allocation reference](https://docs.fortinet.com/document/fortigate/7.4.6/fortinet-carrier-grade-nat-field-reference-architecture-guide/101228/one-to-one-cgnat).

**ASA:** the [existing routing/NAT contract](ROUTING_NAT_CONTRACT.md) remains unchanged. No PAN-OS zone or FortiGate policy semantics are applied to ASA records. Existing saved ASA results retain their original contracts and bytes.

Configured TCP/UDP return analysis revalidates the forward decision before assuming session admission. It reverses the selected forward mappings instead of selecting fresh NAT rules, requires exact tuple restoration and symmetric interfaces/links, and retains the relevant policy, NAT and route evidence. Unsupported protocols or asymmetric paths remain indeterminate. A complete supported model with no route or a decisive forward policy deny is blocked. An ACL/policy permit alone cannot resolve missing NAT, routing or topology facts.

## Limits, storage and release proof

The import API accepts an optional `vendor` field: `auto`, `cisco-asa`, `paloalto-panos` or `fortinet-fortigate`. Ambiguous native signatures and mismatched formats are rejected. Samples are available at `/api/sample/panos`, `/api/sample/fortigate` and `/api/sample/network-multivendor`. The existing path/network endpoints retain their request shape; each node derives its policy scope from the selected snapshot, not caller-supplied scope.

New parser versions are `panos-config-v1` and `fortios-config-v1`, with path metadata `panos-path-v1` and `fortios-path-v1`. Vendor hop results use `graphlayer-vendor-path-v1`, and networks containing a non-ASA node use `graphlayer-network-path-v2`. All-ASA results retain their released versions and behavior. New envelopes use workspace-v5; actual released workspace-v1 through v4 artifacts are validated and replayed without rewriting them. Back up before upgrading: older executables cannot open new-format records.

Existing limits remain: 2 MiB / 50,000 lines per import, 32 snapshots, 8 MiB saved artifact, 16 nodes / 32 links / 16 traversed firewalls per direction, and a shared 256-investigation quota. Object/rule expansion and parser inventories are bounded. Oversized or invalid records fail without partial publication. Local session/CSRF, strict JSON, private storage and single-writer protections apply to all new endpoints and formats.

Release proof covers native parsing and readable sanitized evidence, vendor-specific original/translated policy tuples, static route selection and translation order, mixed-vendor round trips, unsupported controls, blockers, corrupted replay, actual released-v4 compatibility, history/export, stale responses and mobile layout. Supported Go coverage/race, vet, Windows-native and full browser gates remain required. This milestone does not establish broad vendor parity, live reachability, App-ID decisions, dynamic/PAT/service NAT, tunnels, HA or enterprise service deployment.
