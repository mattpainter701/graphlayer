# ASA policy workspace

The local investigation app now connects three views of the same immutable snapshot: an evaluated flow, the full ordered ACL inventory, and named object dependencies. These views share snapshot/chain identity. Searching an inventory does not change the rules used to evaluate a flow.

Use the [launch guide](FIREWALL_INVESTIGATION.md) to run `graphlayer-investigate`. Import the original ASA sample or the new labeled service-group sample through the same live parser. Select an attached ACL in the flow form before browsing its inventory.

## Workflows

- **Investigate:** enter a concrete tuple, evaluate, and inspect saved predicate states for each considered rule. A known mismatch can exclude a rule even when another predicate is unknown. A matching trace within an incomplete chain still leaves the overall decision indeterminate.
- **Browse policy:** search rule IDs, addresses, ports and named dependencies; filter action, enabled state or unresolved data. The complete chain includes disabled/unvisited rules without labeling them evaluated. Counts distinguish filtered results from total rules; pagination bounds browser rendering.
- **Inspect a rule:** select its inventory row to see normalized source/destination ports, the original ACE's physical line, and source/destination/service dependency links. Group expansion creates multiple equivalent rules with one original ACE origin.
- **Inspect an object:** search its name/kind and follow typed dependency edges, resolution warnings and exact member evidence. Missing objects, unsupported members, cycles and depth limits are explicit. A service object is never substituted for a same-named group.
- **Replay:** reopen history to restore the saved snapshot, ACL, tuple and explanation. Export retains the exact saved result. Prior results without predicate traces are labeled as legacy; the UI does not invent explanations for them.

Rule inventory and considered-rule traces render 100 rows per page; object inventory renders 50. Dependency trees stop after 500 rendered members or depth 16, with links for continued inspection. Source evidence preserves physical line numbers and pages spans at 200 lines. These display limits do not change evaluation inputs or saved evidence.

The new service sample demonstrates nested services and source-port-sensitive matching. TCP from `192.0.2.42:52000` to `10.20.4.15:443` permits; destination port 22 denies; destination 8443 permits through a service object whose source range is 49152–65535. A source port outside that range does not match that service. The original sample still demonstrates a retained unresolved destination object on port 8443.

## Semantics and budgets

The ASA adapter adds common TCP/UDP service objects, service groups, nested group references and source/destination port predicates. It preserves protocol-qualified alternatives and keeps explicit TCP members separate from UDP. Unknown/missing/cyclic service members invalidate the whole expansion; supported siblings cannot silently replace the missing portion of an earlier deny. Group namespace and forward declarations are handled before normalization.

Service objects accept TCP or UDP definitions; generic service-group members can also use TCP-UDP. Protocol-qualified port groups accept `port-object eq` and `port-object range`. Source and destination port groups on a TCP/UDP ACE are combined within the expansion budget. Additional ACE port constraints on a protocol-slot service object/group remain unresolved. Unsupported operators or grammar in a particular position remain explicit instead of being discarded.

Named ports follow ASA definitions, rather than assuming every name is an IANA alias. Cisco's [ASA object guide](https://www.cisco.com/c/en/us/td/docs/security/asa/asa920/configuration/firewall/asa-920-firewall-config/access-objects.html) and [ASA port reference](https://www.cisco.com/c/en/us/td/docs/security/asa/asa912/configuration/general/asa-912-general-config/ref-ports.html) define the source syntax and literals. For example, ASA's `radius`/`radius-acct` literals use 1645/1646, `dns` refers to DNSIX 195, and `kerberos` uses 750. Use `domain` for port 53. Unsupported alias/protocol combinations remain unresolved. Reimport original configurations to apply corrected parsing; old normalized snapshots retain their historical parser semantics.

Expansion is bounded by depth 16, 256 source × destination × service alternatives per ACE and 10,000 normalized rules per chain. An exceeded budget remains unresolved rather than publishing a truncated permit. Existing input/workspace quotas remain. Expanded snapshots and saved results each have an 8 MiB storage budget; an oversized artifact fails explicitly without saving a partial record. This is a local inventory over a bounded loaded snapshot, not a server-side large-fleet search service.

This increment still evaluates a selected supported interface ACL. Service inspection does not add route, NAT, stateful/return-path, application/user identity or device-runtime conclusions. Wider unsupported grammar remains explicit. There is no new network collection, device write, shared account model or external UI resource.

## Version compatibility

This policy-workspace increment introduced parser `asa-config-v2`, decisions `graphlayer-policy-v3`, and disk envelopes `graphlayer-workspace-v2`. The [contract](POLICY_WORKSPACE_CONTRACT.md) records its inventory/trace fields. The subsequent [routing/NAT increment](ROUTING_NAT_INVESTIGATIONS.md) records the current parser-v3/storage-v3 versions; ACL decisions retain policy-v3.

The store reads released workspace-v1/parser-v1/policy-v2 records without rewriting them. It verifies their checksums, scope and identities and replays their original result projection using the preserved decision semantics. An upgrade does not claim the old parser produced today's metadata: old snapshots have no fabricated object inventory and old results have no fabricated predicate trace. A new evaluation of an old snapshot may save a v3 result alongside the original v2 result.

Current records require the current result contract and exact replayed traces. Unknown versions, incompatible version combinations, altered traces, invalid evidence/origin references and falsely resolved dependency graphs fail validation. Checksums are corruption detection, not signatures or device-authenticity proof. Back up the workspace before an upgrade; after writing v2 records, the older executable cannot open that mixed workspace.

Compatibility fixtures were generated by the actual PR #12 source at merge `d7d8299`, including permit, explicit deny, indeterminate and default deny. Tests copy these artifacts and assert exact old bytes before and after reopen; they do not construct the fixture by stripping new trace fields.

## Validation and next increment

The supported Go CI manifest, 70% coverage threshold, Linux race tests, Windows-native tests and real Chromium workflow remain release gates. New tests cover typed service dependencies, bad/missing/cyclic groups, protocol/port semantics, expansion limits, inventory/evidence round trips, legacy replay and current-record corruption. Browser tests exercise the actual executable and parser, with no mocked policy result.

This delivers more of I11/P02/P06 from the [overhaul backlog](ENTERPRISE_OVERHAUL_BACKLOG.md). Wider vendor-certified corpora, service/application semantics, durable import jobs, enterprise identity, multi-vendor operations and multi-hop routing/NAT remain separate acceptance work. No competitor-superiority claim follows from this increment.
