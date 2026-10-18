# Enterprise overhaul execution backlog

Read with [the product/architecture plan](ENTERPRISE_OVERHAUL_PLAN.md). IDs are work packages, not externally created issues. Effort is deliberately not estimated per row before discovery. Each package ships through an isolated task branch/worktree and a reviewed PR; independent tasks may run in parallel only after file ownership and contracts are agreed.

The [Rust architecture policy](RUST_ARCHITECTURE.md) governs all remaining domain
work, including every vendor parser and Python connector. The first production
port covers ASA forwarding inventory; the remaining legacy implementations stay
frozen under the migration guard until their behavior moves to Rust.

## M0 — Trust, contracts and reproducibility

| ID | Package / owner | Depends on | Acceptance |
|---|---|---|---|
| F01 | Baseline and recovery inventory / lead | — | Record fresh main, branch/worktree ownership, local changes, PRs and LFS failures; preserve local prototype |
| F02 | Deterministic toolchain and CI / platform | F01 | Module-aware Go version; real Python config; exact commands run; no invented benchmark flags |
| F03 | Supported module/package manifest / platform | F02 | Root/core/orchestration modules enumerated; supported package tests run independently; omissions explained |
| F04 | Confidence determinism / backend | F01 | Inject/control time; propagation, corroboration, decay and filtering tests pass without wall-clock dates changing outcomes |
| F05 | Reference-catalog integrity / backend | F01 | Embed paths valid; catalogs/version/fallback explicit; unknown lookup behavior truthful; representative protocol/EtherType tests |
| F06 | Scoped evaluation contract / domain/backend | F01 | Snapshot/device/context explicit; ordered rules, concrete tuple, permit/deny/indeterminate with reasons and evidence |
| F07 | Conservative evaluation core / backend | F06 | Earlier unresolved candidate blocks later definitive result; disabled rules ignored; explicit completeness/default; never wildcard unresolved objects |
| F08 | CLI investigation proof / backend | F07 | Fixture bundle and query produce JSON result; invalid input exits nonzero; no device/network mutation; sample replay documented |
| F09 | Golden semantic corpus v1 / domain/QA | F06 | Independently labelled critical cases, supported subset and unknown cases; format/adversarial input fixtures |
| F10 | Canonical schema ADR / backend/domain | F01,F06 | One model; namespace/evidence/time schema; selective v2 recovery rationale; versioned validation contract |
| F11 | Coverage/claims manifest / QA/domain | F09,F10 | Every adapter declares OS/version/features and limitations; coverage independent of result accuracy |
| F12 | Packaging truth / platform | F02 | Packaged commands exist; builds fail on absent deps; demo vs production explicit; smoke exact entrypoints |

## M1 — One real enterprise investigation

The [workspace/import foundation](WORKSPACE_IMPORTS.md) advances I03/I08/I09/I12 with explicit local workspace/site context, reviewed batch parsing, atomic named snapshot-set publication and a redesigned onboarding shell. Unpublished previews are ephemeral; durable collection jobs, network connectors, credential providers and shared-service authorization remain open. Sites organize evidence and do not create an account boundary.

The first local delivery slice is implemented in the [ASA investigation application](FIREWALL_INVESTIGATION.md). It covers the parser-to-evidence-to-browser path and its automated tests. The guide records each delivered boundary and the remaining enterprise acceptance work; durable jobs, enterprise identity, migrations and generated API clients below are still open.

The next [policy workspace slice](POLICY_WORKSPACE.md) adds service/object dependencies, a paged local inventory and saved predicate explanations. Released v1 records remain readable with verified legacy replay. This does not close the wider vendor corpus, generalized migrations/restore, generated clients or enterprise service acceptance items.

| ID | Package / owner | Depends on | Acceptance |
|---|---|---|---|
| I01 | First-vendor parser adapter / domain/backend | F07,F10,F11 | Real sanitized config generates a scoped policy bundle; unresolved constructs reported, not discarded |
| I02 | Immutable evidence artifacts / backend | F10 | Hash + source/command/parser/version/line ranges; original versus sanitized evidence linked; restrictive access |
| I03 | Snapshot manifest/publication / backend | I01,I02 | Atomic activation; failed import preserves previous snapshot; skew/completeness visible; repeat import idempotent |
| I04 | Import job state / backend | I03 | Durable queue/lease/retry/cancel; restart recovers; bounded concurrency and input budgets |
| I05 | Persistence ADR and migrations / platform/backend | F10 | Representative query/index benchmark; schema migration/restore proof; no speculative extra databases |
| I06 | Identity and resource authorization / security/backend | F10,I05 | Valid session/role/scope on every handler/job/export; cross-site/organization substitution denied |
| I07 | Typed API/error contract / backend/frontend | F06,I03,I06 | OpenAPI and generated client; validated sizes/tuple; one base URL; consistent errors and snapshot identity |
| I08 | Operator shell/design system / frontend/design | I07 | Persistent scope/freshness, responsive nav, keyboard search, clear empty/error/partial states, accessible dialogs |
| I09 | Import validation screen / frontend | I04,I07,I08 | Parse counts/warnings/missing data; publish/retry; no fabricated logs; source evidence links |
| I10 | Single-firewall investigation UI / frontend | I01,I07,I08 | Concrete flow → real scoped evaluation → decisive rule/unknown reasons; policy-only label; save scenario |
| I11 | Rule/object/evidence inspector / frontend/backend | I02,I10 | Ordered predicates, expansion tree, exact artifact lines and hashes; raw evidence separately authorized |
| I12 | Full workflow integration test / QA/domain | I09,I10,I11 | Upload real config, publish, evaluate allow/deny/unknown, replay result; API outage and invalid import tested |

## M2 — Multi-vendor policy operations

| ID | Package / owner | Depends on | Acceptance |
|---|---|---|---|
| P01 | Address sets/groups / backend/domain | F07,F09 | IPv4/v6, prefixes/ranges/negation and nested groups; cycle/size limits; unresolved members remain unknown |
| P02 | Service/protocol sets / backend/domain | P01 | Source vs destination ports, protocol numbers, ICMP fields, ranges/groups; application-default unsupported until modeled |
| P03 | Ordered chains and vendor defaults / domain/backend | P02 | Pre/post/local rules and implicit intra/inter-zone defaults; disabled policy and schedule semantics certified |
| P04 | Second vendor adapter / domain/backend | I12,P03 | Same critical corpus/coverage contract; no shared-state leakage across vendors/devices |
| P05 | Third vendor/context inheritance / domain/backend | P04 | Panorama/FortiManager inheritance and VDOM/VSYS isolated; partial exports detectable |
| P06 | Searchable policy inventory / frontend/backend | P03,I07 | Scoped server-side filtering/pagination; source and normalized values; stable rule links |
| P07 | Matrix aggregate semantics / domain/backend | P03 | Candidate/mixed/empty/unresolved summaries; exact tuple verdict distinguished; same engine as investigations |
| P08 | Matrix UI and drill-down / frontend | P06,P07 | Virtualized rows/columns, accessible equivalent table, ordered rules/evidence, same-named contexts distinct |
| P09 | Rule hygiene findings / domain/backend | P03 | Shadow/redundancy/any-any analysis explains semantic scope; partial vs full coverage distinguished; exceptions reviewed |
| P10 | Usage evidence / integrations/domain | I02,P06 | Hit-counter interval/reset/missing telemetry explicit; zero observed hits never automatically authorizes deletion |

## M3 — Topology, identity and end-to-end paths

The [topology workspace](TOPOLOGY_WORKSPACE.md) advances the operator UI with
dedicated application pages, a draft canvas before analysis, device search,
pan/zoom/focus, saved trace and branch/return overlays, and a snapshot-qualified
configuration/source inspector. It preserves workspace-v7 and the existing
analysis contracts; live topology discovery and runtime traffic remain open.

The [path exploration slice](PATH_EXPLORATION.md) advances G06/G07/G09/G10/G11 with bounded configured route branches, independent asymmetric return candidates, sticky session uncertainty and an accessible topology view tied to snapshot-qualified evidence. Runtime ECMP selection, dynamic routing, recursive routing, HA/session synchronization, tunnels and broader vendor syntax remain open.

The [multi-vendor slice](MULTI_VENDOR_INVESTIGATIONS.md) adds native PAN-OS/FortiOS imports and vendor-specific configured policy/NAT stages to mixed forward/return investigations. It advances adapter and path acceptance with an actual three-vendor workflow; broad syntax parity, App-ID, dynamic state, tunnels, HA and mixed-vendor branch search remain open.

The [multi-firewall slice](MULTI_FIREWALL_INVESTIGATIONS.md) extends G09/G10/G11 with explicit snapshot-pinned ASA links, per-hop routing/static NAT, assumed established TCP/UDP reply traces, source evidence and local replay/export. It requires symmetric supported configuration paths and does not close branch exploration, dynamic state, tunnels, mixed vendors or the wider acceptance criteria below.

The [local routing/NAT slice](ROUTING_NAT_INVESTIGATIONS.md) begins G07/G08/G11 with single-ASA configured IPv4 routes, bounded static object NAT and evidence-backed path stages. It preserves older records and tests the complete local workflow. This does not close dynamic routing, full NAT, state/return paths, tunnels, multi-vendor or multi-hop acceptance below.

| ID | Package / owner | Depends on | Acceptance |
|---|---|---|---|
| G01 | Scoped stable entity IDs / backend | F10 | Overlapping IP/VLAN/zone names across sites/VRFs coexist; parser re-run identity stable |
| G02 | Correlation conflict/lineage repair / domain/backend | G01,F04 | Independent sources identified; contradictory observations preserved; no cross-scope transitive false merge |
| G03 | Graph integrity and typed hierarchy / backend | G01 | Every endpoint present in projection; parent/child typed and consistent; deterministic immutable builds |
| G04 | Summary/neighborhood/search queries / backend | G03 | Bounded payload/cursors; totals/truncation explicit; filters preserve relationship consistency; snapshot-keyed caches |
| G05 | Renderer/layout benchmark / frontend | G04 | Measured candidates on fixed fixtures/hardware; worker layout and accessible list; decision ADR |
| G06 | Topology investigation workspace / frontend | G05,I11 | Search/focus/isolate/expand, inspectors, real L2/L3/policy layers, URL state; graph path never labelled packet permit |
| G07 | Directed routing semantics / domain/backend | G01,F09 | Interface/VRF, longest-prefix, recursion, null routes/loops/ECMP and missing observations represented |
| G08 | NAT normalization and stages / domain/backend | P03,G07 | Pre/post tuple, static/dynamic/twice/hairpin semantics per vendor; unsupported transforms explicit |
| G09 | Stateful return and tunnels / domain/backend | G08 | Asymmetric/return paths, supported state/session assumptions and tunnel domains explained |
| G10 | Multi-hop evaluation / backend/domain | G07,G08,G09 | Scope-safe route→policy→NAT sequence; bounded branch search; mixed/unknown branches preserve uncertainty |
| G11 | Path explanation UI / frontend | G06,G10 | Hop-by-hop device/interface/rule/NAT/evidence; decisive blocker and all unresolved stages visible |
| G12 | Saved views and investigations / frontend/backend | G06,I10 | Immutable snapshot identity, focus/filters/layout/queries; authorized shared URLs replay |
| G13 | Export current scope / backend/frontend | G04,G12 | Diagram/tables/JSON use same scope; provenance/filters/coverage included; secrets redacted |

## M4 — Change assurance and collaboration

| ID | Package / owner | Depends on | Acceptance |
|---|---|---|---|
| C01 | Semantic diff / domain/backend | I03,P03,G08 | Rules/objects/routes/NAT compared with stable IDs; formatting-only differences suppressed and reviewable |
| C02 | Intent model and library / domain/backend | I10 | Required/forbidden application flows, owner, scope, expiry and evidence; repeatable scenario corpus |
| C03 | Differential evaluation / backend | C01,C02,G10 | Old/new result and dependency deltas; unsupported cases do not disappear; critical exposure regression suite |
| C04 | Change review workspace / frontend | C03 | Exposure/breakage/unknown triage, rule/evidence links, reviewer comments and baseline lock |
| C05 | Approval/audit lifecycle / security/backend | I06,C04 | Separation of proposal/approval/execution/verification; authorization and append-only audit; no device write shortcut |
| C06 | Post-change verification / integrations/backend | C05 | Fresh collection tied to approved change; drift and stale evidence shown; rollback advice human-reviewed |
| C07 | Ticket/API integration / integrations | C05 | ServiceNow/Jira/webhook payloads scoped/idempotent; delivery/auth failures explicit; no automatic external messages |
| C08 | Reviewable report bundles / frontend/backend | G13,C04 | Another authorized engineer replays inputs and results; redaction and retention rules applied |

## M5 — Enterprise pilot and operations

| ID | Package / owner | Depends on | Acceptance |
|---|---|---|---|
| O01 | Enterprise SSO and roles / security | I06 | OIDC/SAML requirements decided; viewer/operator/reviewer/admin boundaries tested; session invalidation |
| O02 | Secret provider and key rotation / security/platform | I02 | Required keys, encrypted artifacts, credential references, rotation/revocation; no secret in DTO/log/support export |
| O03 | Connector egress/transport policy / security/integrations | O02 | Approved internal networks, TLS/SSH verification, rebinding/redirect protections and limits at dial/read |
| O04 | Source run health / integrations/frontend | I04,O03 | Last attempt/success, freshness, credentials status, partial errors, retry/cancel and audit |
| O05 | Collection coverage expansion / domain/integrations | F11,O04 | Add customer-priority source only with recorded fixtures/capabilities; no unchecked vendor count claim |
| O06 | Operations observability / platform | I04,I07 | Structured redacted logs, metrics/traces, queue lag, unknown-rate and source-age dashboards/alerts |
| O07 | Production packaging / platform/security | F12,I07 | Actual entrypoints, non-root, pinned/reviewed deps, TLS ingress and bounded resources; offline update route |
| O08 | Backup/restore and migrations / platform | I05,O07 | Encrypted backup, restore drill, counts/hashes reconciliation, forward/rollback compatibility |
| O09 | Retention/deletion/export controls / security/backend | O02,O08 | Artifacts/jobs/results/caches/backups covered; scoped access audit and legal/customer retention settings |
| O10 | Security assessment / security/QA | O01,O03,O07 | Threat model plus adversarial parser/API/job tests; SBOM/vulnerability review and findings closure |
| O11 | Accessibility/usability / design/QA | I08,P08,G11,C04 | Keyboard-only completion of core tasks; named controls, focus management, table alternatives and contrast |
| O12 | Scale and restart tests / platform/QA | G04,G10,O06 | S/M gates measured; L/XL experiments reported; soak/restart/cancel/resource failure leaves ready snapshot intact |
| O13 | Pilot runbooks/support / platform/product | O08,O10,O12 | Install, onboard, diagnose unknown, rotate, restore, roll back and sanitize support bundle rehearsed |

## M6 — Demonstrate superiority and release

| ID | Package / owner | Depends on | Acceptance |
|---|---|---|---|
| B01 | Pre-registered comparison protocol / QA/product | F09,F11 | Matched versions/licenses/data/hardware; independent truth; accuracy/coverage/time metrics frozen |
| B02 | Real comparator imports/captures / domain/QA | B01 | Authorized reference/Batfish and enterprise comparator results; no synthetic self-comparison presented as proof |
| B03 | Analyst task study / design/QA | B02,O11 | Randomized/skill-stratified paired tasks; correct completion, time, wrong-action/setup costs |
| B04 | Scale/freshness comparison / platform/QA | B02,O12 | p50/p95/p99/RSS/coverage; warm/cold and incremental runs; reproducible scripts/fixtures |
| B05 | Capability and limitations release review / domain/security | B03,B04,O10 | Claims link to evidence; unknowns and unsupported features published; no unearned compliance claims |
| B06 | Release and pilot-to-production promotion / lead/platform | B05,O13 | Release notes, migration/rollback, final-head green CI, merge SHA verified, branch/worktree cleanup per policy |

## Parallel agent ownership

Use named lanes with disjoint writes: canonical/evaluator; vendor fixtures/adapters; API/jobs/storage; frontend/design; security/ops; QA/benchmark. Maximum concurrent agents follows available slots; run waves instead of pretending unlimited concurrency. Default delegated model is Luna with medium reasoning. The lead owns integration, architecture decisions and release evidence. Reviewer agents do not edit implementation files they are reviewing. Resolve shared-contract changes before dependent agents write.

Initial discovery used separate backend, frontend/workflow, security/operations, competitive benchmark, policy semantics, and confidence-integrity perspectives. Subsequent implementation/review agents receive narrow file ownership and acceptance criteria. Their reports are inputs, not independent proof that code works.
