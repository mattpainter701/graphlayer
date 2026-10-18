# Changelog

## Unreleased

- Add named local workspaces, sites and snapshot sets with reviewed multi-vendor batch imports, sanitized previews, revision conflict checks and atomic publication that preserves the previous baseline on failure.
- Redesign the workspace shell around onboarding, active datasets and source inspection, with persisted baseline selection, exact investigation replay and desktop/mobile end-to-end coverage. Preserve released workspace-v1 through v6 records while adding validated workspace-v7 transactions.

- Add multi-vendor configured path exploration with bounded equal-best route branches, independent asymmetric return candidates, per-branch routing/policy/NAT source evidence and explicit session uncertainty.
- Add an interactive topology view, branch selection, saved exploration replay/export and a native four-firewall diamond sample. Preserve released workspace-v1 through v5 records while adding validated workspace-v6 exploration artifacts.

- Add native PAN-OS set-format and FortiOS CLI imports to multi-vendor path investigations, with vendor-specific zone/interface policy, configured static routing/NAT, established-session return analysis and snapshot-qualified source evidence.
- Add a parsed Palo Alto → ASA → FortiGate workflow, vendor format selection, mixed-vendor replay/export and end-to-end coverage. Preserve released workspace-v1 through v4 artifacts while introducing validated workspace-v5 vendor models; unsupported controls remain indeterminate.

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

The local operator UI supports bounded ASA, PAN-OS and FortiOS configuration investigations. Broader vendor syntax, routing/NAT and enterprise service deployment remain planned work. See `docs/MULTI_VENDOR_INVESTIGATIONS.md` for current supported scope.
