# Policy workspace increment contract

This increment extends the local ASA application with bounded service expansion, a searchable rule/object inventory, named dependency inspection and per-predicate result explanations. It preserves the previous policy-only trust boundary. It adds no device access, routing/NAT claims, enterprise accounts or external resources.

## Ownership

The root owns `firewall/types.go`, investigation store/API/validation/migration and documentation. The ASA agent owns new running-config/service parsing and the firewall adapter/fixtures/tests. The evaluator agent owns `policy` Go sources/tests/package guide. The UI agent owns only embedded `investigation/web` files. Follow-on QA and review agents own separate tests or remain read-only. All work uses the root-owned `codex/asa-policy-workspace` worktree.

## Snapshot additions

`Config.objects` (optional for older snapshots) is an array of `firewall.Object`: `id`, `name`, `kind` (`network_object`, `network_group`, `service_object`, `service_group`), optional `protocol`, boolean `resolved`, `members`, `evidence_refs`, optional `warnings`. IDs are stable kind/name identifiers within this snapshot; same names in different kinds must not collide. A missing reference may use its intended ID even without an object record. This must be shown explicitly, never rebound to a same-named object in another namespace.

Each `ObjectMember` has `kind` (`address`, `service`, `reference`, `unsupported`), optional `value`, optional referenced `object_id`, optional normalized `protocol`, `source_port`, `destination_port`, and `evidence_refs`. Values contain normalized policy data or bounded generic unsupported labels, never arbitrary unknown command content or secret values. Evidence holds sanitized policy source locators. Dependency cycles, missing references, unsupported members and budgets make the entire object's expansion unresolved.

`Config.rule_origins` links each expanded rule to `rule_id`, `acl_name`, 1-based physical `line`, and optional `source_object_ids`, `destination_object_ids`, `service_object_ids`. Repeated expansions of an ACE have separate rule IDs, the same source line, and consistent dependency references. Every new v2 adapter rule has one origin. Missing objects remain explicit references. Object/member evidence locators must exist in the same snapshot. Parser version becomes `asa-config-v2`; previous snapshots are immutable and keep their v1 parser version.

The existing snapshot endpoint returns these fields; the first bounded local inventory uses the loaded snapshot, explicit ACL selection and UI filters. Filtering never changes evaluation inputs or implies the filtered rule list is complete policy. Old snapshots have no fabricated object inventory; their UI explains that limitation.

## Evaluation additions

Engine contract `graphlayer-policy-v3` retains prior decision semantics and adds optional `rule_traces`: `rule_id`, `position`, `outcome` (`matched`, `not_matched`, `unresolved`), ordered `predicates`, optional `evidence_refs`. Each predicate has `dimension`, `state` (`match`, `not_match`, `unresolved`) and a bounded static `reason`. Every considered enabled rule gets one trace in evaluation order; disabled and unvisited rules are not mislabeled as evaluated. A supported mismatch may eliminate a rule despite other unknown predicates. Traces never copy raw unresolved values.

The UI shows the exact evaluated tuple and selected snapshot/chain. A predicate explanation is derived from the saved result, not reconstructed heuristically from normalized values. Incomplete chains can contain a matching predicate trace while the overall result remains indeterminate; explain both distinctly.

## Persistence compatibility

New records use `graphlayer-workspace-v2`. The application reads prior v1 records without rewriting their bytes, source IDs or checksums. Prior v2 evaluator results are verified by replaying the unchanged semantics and comparing the legacy projection (v2 contract, no new traces). New records must use the current result contract and complete traces. A legacy result without traces is visibly labeled; it is not presented as if traced under the new engine. Record identity continues to hash the exact stored result projection. Unknown record/result versions, tampering, invalid references and replay differences fail explicitly.

## Required proof

Service objects, nested/mixed TCP/UDP service groups, source/destination ports and bounded alternatives need parser/evaluator tests, including cycles, unknown members, missing groups, limits and an earlier unresolved deny before a later permit. Inventory references and exact service/object source lines must survive store/API round trips. A real browser test must import service-bearing ASA syntax, search/inspect rule and group dependencies, obtain permit/deny/unknown results with predicate explanations, and reopen history. Legacy v1 snapshots/v2 results must load and replay without on-disk mutation; corrupt or downgraded new records must be rejected. Existing Linux/Windows tests, race checks, browser workflows and the 70% coverage gate stay green.
