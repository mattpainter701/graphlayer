# Routing and NAT investigations

The local app can now investigate a bounded, configuration-derived IPv4 forward path through one ASA. This is a separate workflow from evaluating a selected ACL. It explains the interfaces, translated tuple, ingress policy, route and source translation that the supplied configuration can establish.

The input is assumed to be complete for those modeled facts. The app cannot independently prove that a pasted fragment omits no NAT, route or policy statements. The supported NAT grammar is the ASA 8.3+ object-NAT form; legacy NAT and standby/HA interface address forms remain explicit uncertainty.

Run the executable using the [launch guide](FIREWALL_INVESTIGATION.md), then load the labeled routing/NAT sample and import it. In **Path investigation**, select an ingress interface and enter the concrete tuple. The engine chooses the actual inbound ACL attached to that interface; the ACL inventory filter does not choose path policy.

## Sample investigations

The sample is synthetic ASA running-config text, parsed by the same import path as an uploaded configuration. It includes three interfaces, repeated address/NAT object stanzas, a default route, a more-specific static route and static object mappings.

| Ingress | Original tuple (TCP) | Expected modeled result |
|---|---|---|
| outside | `192.0.2.42:52000 → 198.51.100.15:443` | Destination untranslates to `10.20.4.15`; the real-address ACL permits; NAT selects directly connected dmz |
| outside | `192.0.2.42:52000 → 198.51.100.15:22` | Explicit ingress ACL deny blocks the real-address tuple |
| dmz | `10.20.4.15:52000 → 203.0.113.9:443` | The `/24` static route selects outside; source translates to `198.51.100.15` |
| inside | `10.0.0.30:52000 → 203.0.113.9:443` | Static route and explicit ingress ACL establish the modeled stages without a matching NAT rule |

Inspect each stage's before/after tuple and source links. The route stage identifies the chosen prefix, next hop and interface, or explains why a route cannot be selected. Saved path history restores the exact snapshot, ingress and tuple; JSON export contains the saved result. An unresolved or blocked path does not invent successful later stages or a final egress tuple.

## Interpretation

- **Modeled forward:** the supported configuration-derived stages agree on a forward path on this device. This does not prove that the next hop, ARP, link, remote network, application, session or return path works.
- **Blocked:** a modeled configuration condition such as an explicit ACL deny or absence of a route in a complete supported routing model prevents the path.
- **Indeterminate:** required evidence, semantics or selection is unresolved. An ACL permit alone cannot resolve unsupported NAT, route ambiguity or unmodeled forwarding controls.

Interfaces retain their configured IPv4 address, security level and shutdown state. Routes are configured connected/static IPv4 routes; selection uses longest prefix and administrative distance, with a directly connected next hop for a static route. Equal best alternatives remain unresolved. An explicit supported ingress ACL is required; implicit security-level policy, global/egress ACL semantics and local-device management traffic are not inferred.

The NAT subset is one-to-one static object NAT on explicit interface pairs with equal-size IPv4 prefixes. Destination untranslation occurs before the ingress ACL's real-address match. Destination NAT's selected real interface is supported only for directly connected real targets. Source NAT requires agreement with the modeled route egress. Simultaneous source/destination composition, manual/twice NAT, dynamic NAT/PAT, service translation and other unsupported forms are explicit uncertainty. Unknown forwarding controls or incomplete routing cannot produce a modeled forward result.

These decisions follow Cisco's [NAT rule ordering and interface behavior](https://www.cisco.com/c/en/us/td/docs/security/asa/asa920/configuration/firewall/asa-920-firewall-config/nat-basics.html), [real-address ACL example](https://www.cisco.com/c/en/us/support/docs/security/asa-5500-x-series-next-generation-firewalls/115904-asa-config-dmz-00.html), and [static route configuration](https://www.cisco.com/c/en/us/td/docs/security/asa/asa916/configuration/general/asa-916-general-config/route-static.html). The implemented subset and evidence contract are narrower than the full ASA feature set; see the [milestone contract](ROUTING_NAT_CONTRACT.md).

## Storage, limits and upgrade

New imports use parser `asa-config-v3` and path metadata `asa-path-v1`. Path results use `graphlayer-path-v1`; ACL results retain `graphlayer-policy-v3`. New disk envelopes use workspace-v3. Released workspace-v1 and workspace-v2 records remain byte-identical and replay through their original policy result contracts. They have no fabricated path inventory; reimport the original configuration to investigate paths with the current parser.

Back up a workspace before upgrading. After workspace-v3 records are written, older executables cannot open the mixed workspace. The store verifies reference structure and replays every saved path against the exact immutable snapshot on reopen. Checksums detect corruption; they are not signatures or proof of device origin.

The 2 MiB/50,000-line import budget, 32 snapshots, 8 MiB record budget and single-writer workspace remain. ACL and path history share a 256-record limit. Existing bounded inventories and source paging remain; the path UI also bounds stage, evidence and history rendering. Oversized artifacts fail without a partial record.

The release proof includes real parser-to-browser workflows, saved path replay, invalid/corrupted data, actual released-v1/v2 compatibility fixtures, Linux race tests, Windows-native tests and the unchanged 70% supported-package coverage gate. Dynamic routing state, full NAT semantics, multi-vendor paths, return paths, tunnels and multi-hop reachability remain in the [enterprise backlog](ENTERPRISE_OVERHAUL_BACKLOG.md).
