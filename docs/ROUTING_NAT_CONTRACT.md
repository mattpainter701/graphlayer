# Routing and NAT path milestone contract

This increment adds a configuration-derived, single-ASA IPv4 path model alongside the existing ACL-only investigation. It does not turn a static configuration into observed device state or prove a return path, application success, or multi-hop delivery.

## Ownership and isolation

Task worktree: `F:/deepseek/graphlayer-routing-nat`, branch `codex/routing-nat-investigations`, starting merge `2855cdae43e2298c12729d5cce1b044e7552ac89`. Root owns the frozen model contract, storage/API integration, validation, version compatibility, documentation and release. Parser owns new path parsing and its fixture/tests. Engine owns `platform/pathanalysis`. UI owns the three embedded web files. QA owns dedicated new workflow tests; semantic review is read-only.

## Snapshot and evaluation

Parser `asa-config-v3` adds optional `Config.path` for new snapshots: version `asa-path-v1`, explicit interfaces, connected/static routes, supported static object NAT mappings, completeness markers and physical source evidence. Historical parser-v1/v2 snapshots retain their exact metadata and have no fabricated path model. Unsupported forwarding syntax invalidates relevant completeness rather than disappearing.

`pathanalysis.Evaluate(Input)` is pure and deterministic. Input carries exact snapshot/device/context scope, an explicit ingress interface, the immutable configuration and a concrete IPv4 tuple. Result contract `graphlayer-path-v1` returns `modeled_forward`, `blocked`, or `indeterminate`, ordered stages, before/after tuples where computed, chosen route/egress, policy result, NAT rule IDs, source references, reasons and assumptions. Stages are ingress, destination NAT, ingress policy, route, source NAT and egress; later uncomputed stages must not fabricate a tuple or successful forwarding fact.

Supported NAT is bounded one-to-one static object translation with equal-size IPv4 prefixes. Destination untranslation precedes ingress ACL matching on the real tuple. NAT-selected destination egress is supported only when the real target is directly connected to that interface. Source NAT requires agreement with the modeled egress. Unsupported NAT composition, manual/twice NAT, dynamic translation state or interface-diversion interactions remain indeterminate.

Routing uses configured connected/static IPv4 prefixes, longest prefix, administrative distance and a directly connected next hop. Equal best alternatives, missing facts, dynamic routing, unsupported forwarding controls and incompatible interface/security semantics cannot yield `modeled_forward`. An explicit unambiguous supported ingress ACL is required. Configured interface state is not a claim that the link or next hop is operational.

## API and persistence

- `GET /api/sample/path` supplies a labeled synthetic ASA fixture.
- `POST /api/snapshots/{id}/path-evaluate` takes `{ingress_interface, flow:{source,destination,protocol,source_port,destination_port}}` and returns `{investigation}`.
- `GET /api/path-investigations` returns `{investigations:[PathSummary]}`; `GET /api/path-investigations/{id}` returns `{investigation:{id,created_at,snapshot_id,result}}`.

The existing local-session, exact-loopback Host/Origin, CSRF, CSP, strict JSON, input quotas, private directory and single-writer safeguards apply unchanged. UI filtering does not change a path request. Stale responses cannot attach to a changed snapshot, ingress or tuple. History/export retains the exact saved result and snapshot identity.

New envelopes use workspace-v3; released workspace-v1/parser-v1/policy-v2 and workspace-v2/parser-v2/policy-v3 records remain readable without rewriting bytes. New paths require current path metadata. Each stored path is replayed against its exact snapshot on reopen, including stage traces and identity. Invalid references, version mismatches, forged traces and oversized artifacts fail without partial publication. ACL and path investigations share the 256-record quota; snapshot and 8 MiB artifact quotas remain.

## Required proof

Tests must cover static DNAT with ACL on real addresses, source NAT, route specificity/next hop, explicit ACL deny, no route, NAT/route ambiguity, unsupported earlier NAT or routing controls, physical source evidence, stale UI responses, history/reopen/export and mobile rendering. Actual released v1/v2 fixtures must remain byte-identical through mixed-workspace reopen. All existing browser tests, supported Go manifest, Linux race tests, Windows-native tests, Python regression, fuzzing, Docker QA, benchmarks and the 70% coverage gate remain release gates.
