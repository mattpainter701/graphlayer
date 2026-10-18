# Changelog

## Unreleased

- Add multi-firewall ASA path investigations with explicit snapshot-pinned topology, routing and NAT at each hop, configured return-path checks under a stated TCP/UDP session assumption, source evidence, saved history and export. Missing or asymmetric topology and unsupported semantics remain indeterminate.
- Preserve released workspace-v1/v2/v3 records while adding exact multi-snapshot replay in workspace-v4, shared history quotas, parser-to-browser forward/return tests and new engine coverage in Linux/Windows gates.

- Add routing and NAT-aware ASA path investigations with explicit ingress, static object translations, connected/static route selection, real-address ACL evaluation, stage evidence, saved history and JSON export. Unsupported or ambiguous path semantics remain indeterminate.
- Preserve released workspace-v1/v2 evidence while introducing validated workspace-v3 path records, exact saved-path replay and shared history quotas. Add real path workflow tests and include the path engine in supported Go, race and Windows release gates.
- Extend the ASA investigation app with service objects/groups, searchable rule and object inventories, original-ACE/dependency inspection, and saved predicate-level explanations. Preserve older workspace records and investigations through verified legacy replay without inventing new evidence.
- Correct ASA-specific named-port normalization and keep unresolved service members, ambiguous namespaces and expansion limits conservative. Add versioned inventory validation and explicit oversized-artifact errors.
- Add a local Cisco ASA firewall investigation application: import running configurations, select an attached ACL, evaluate concrete flows, inspect ordered rule traces and sanitized source evidence, and reopen or export saved investigations.
- Repair ASA running-config parsing, canonical routing model integration, ACL order and unsupported-object handling. Add real parser-to-browser tests and Linux/Windows app checks; preserve indeterminate results when policy cannot be safely resolved.
- Extend the versioned policy contract to `graphlayer-policy-v2` with rule-only IP protocol wildcards and family-specific `any4`/`any6` addresses. Concrete query protocols remain required.
- Define the enterprise firewall and network operations overhaul, its staged backlog, evidence requirements and comparative release gates.
- Begin the offline policy-evaluation foundation with explicit scope, ordered matching and an indeterminate outcome for unsupported or unresolved conditions. This evaluates policy bundles and does not establish end-to-end network reachability.
- Repair confidence propagation, corroboration accounting and staleness handling, plus reference lookup and build/test configuration defects identified during the baseline audit.

The first operator UI and ASA adapter are available locally. Multi-vendor investigation, broader routing/NAT and enterprise service deployment remain planned work. See `docs/FIREWALL_INVESTIGATION.md` for current supported scope.
