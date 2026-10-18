# Changelog

## Unreleased

- Add a local Cisco ASA firewall investigation application: import running configurations, select an attached ACL, evaluate concrete flows, inspect ordered rule traces and sanitized source evidence, and reopen or export saved investigations.
- Repair ASA running-config parsing, canonical routing model integration, ACL order and unsupported-object handling. Add real parser-to-browser tests and Linux/Windows app checks; preserve indeterminate results when policy cannot be safely resolved.
- Extend the versioned policy contract to `graphlayer-policy-v2` with rule-only IP protocol wildcards and family-specific `any4`/`any6` addresses. Concrete query protocols remain required.
- Define the enterprise firewall and network operations overhaul, its staged backlog, evidence requirements and comparative release gates.
- Begin the offline policy-evaluation foundation with explicit scope, ordered matching and an indeterminate outcome for unsupported or unresolved conditions. This evaluates policy bundles and does not establish end-to-end network reachability.
- Repair confidence propagation, corroboration accounting and staleness handling, plus reference lookup and build/test configuration defects identified during the baseline audit.

The first operator UI and ASA adapter are available locally. Multi-vendor investigation, multi-hop routing/NAT analysis and enterprise service deployment remain planned work. See `docs/FIREWALL_INVESTIGATION.md` for current supported scope.
