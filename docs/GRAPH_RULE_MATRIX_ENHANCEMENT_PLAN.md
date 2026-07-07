# Graphing/Topology & Rule Matrix — Enhancement Plan

**Date:** July 7, 2026
**Status:** Proposed
**Scope:** Deep-click (drill-down) UI and fast, scalable UX for the topology graph and a new firewall Rule Matrix

---

## 1. Where We Are Today

Grounded in the current codebase, not the aspirational docs:

| Layer | State | Key files |
|---|---|---|
| Parsers / models | ✅ Mature. 9 vendors, unified `Device`, `Zone`, `FirewallPolicy`, `L2Link`, `L3RoutingTable` | `platform/vendors/*`, `platform/models/network_topology_models.go` |
| Graph build | ✅ Working but static. `GraphBuilder` → `TopologyGraph`, view filters, 4 overlays | `platform/graph/builder.go`, `types.go` |
| Graph export | ✅ One-shot file exports only: GraphML, GEXF, Cytoscape JSON, D3 JSON, DOT | `platform/graph/{graphml,gexf,cytoscape,d3,dot}.go` |
| Graph algorithms | ✅ Rich: centrality, components, articulation, PageRank, MST, RWR, coloring | `platform/orchestration/graph/*` |
| Decision engines | ✅ L2/L3/L4 with explainable verdicts (`FirewallDecision.MatchedRule`, `RuleIndex`) | `platform/orchestration/decision/*` |
| Query DSL | ✅ Small SQL-like path/evidence DSL (Python) | `platform/query_dsl.py` |
| HTTP API | ❌ None. No server exists anywhere in the repo | — |
| Frontend | ❌ None. Planning docs only | `docs/FRONTEND_PROPOSAL_V1.md`, `docs/FRONTEND_ARCHITECTURE_SUMMARY.md` |
| Rule matrix | ❌ None. Policies only appear as zone-pair overlay edges (`AddFirewallPolicy`) | `platform/graph/builder.go:168` |

**The two gaps this plan closes:**

1. The graph pipeline is *export-shaped* (build once → dump a file). Deep-click UX needs a *query-shaped* pipeline: stable IDs, server-side aggregation, and incremental delivery.
2. Firewall policies are flattened into one edge per zone pair with no aggregation, ordering, shadowing analysis, or drill-down path back to the source config line. A Rule Matrix is the natural analyst view over `FirewallPolicy` and it doesn't exist yet.

---

## 2. The Deep-Click Interaction Model

One consistent contract across both surfaces: **every visual element is a door, and every door opens onto evidence.** No dead-end pixels.

### 2.1 Topology drill-down ladder

```
L0  Site / Purdue overview      (≤ 50 supernodes: sites, zones, Purdue levels)
     │ click supernode
L1  Segment view                (VLANs / subnets / zones inside the site)
     │ click segment
L2  Device view                 (devices + assets in the segment, L2/L3 edges)
     │ click device
L3  Device inspector            (interfaces, VLANs, routes, policies on device)
     │ click interface / policy / neighbor
L4  Evidence view               (raw config lines, parse hashes, connector run,
                                 first/last seen, decision-engine explain trace)
```

Rules of the ladder:

- **Every level is bookmarkable.** URL encodes `{graph, level, focusID, filters}` so any drill state is shareable and back/forward works.
- **Double-click descends, breadcrumb ascends.** Single click = select + inspector panel; double-click/enter = expand in place (neighborhood loads around the clicked node without a full re-layout).
- **Edges drill too.** Click an L2 edge → port pair, allowed VLANs, native VLAN. Click a policy edge → the rule list behind it (jumps into the Rule Matrix cell — see §3). Click an L3 edge → route entries that produced it.
- **Expand-in-place, not page navigation.** Descending replaces the clicked supernode with its children, pinned near the parent's position; siblings stay collapsed. This keeps the analyst's spatial memory intact.
- **Right-click context menu** on any element: *Expand / Collapse / Isolate (ego network, N hops) / Trace path from here / Show rules touching this / Copy ID / Open evidence.*

### 2.2 Rule Matrix drill-down ladder

```
M0  Zone × Zone matrix           (cell = aggregate verdict for src→dst zone pair)
     │ click cell
M1  Rule list for the pair       (ordered as the firewall evaluates them,
                                  shadowed/dead rules flagged)
     │ click rule
M2  Rule detail                  (expanded objects: address groups → members,
                                  service groups → ports, NAT, logging, hit
                                  context, which devices carry the rule)
     │ click object / evidence link
M3  Evidence                     (source config lines + hash, object definitions,
                                  connector snapshot that produced them)
```

Cross-links make the two surfaces one product:

- Matrix cell → "Show on graph" highlights the zone pair and the firewall(s) enforcing it on the topology.
- Topology policy edge → "Open in matrix" lands on the corresponding M1 rule list.
- Path analysis verdict → each hop's rule links to its M2 detail.

---

## 3. Rule Matrix — Design

### 3.1 Backend: new `platform/policy/matrix` package (Go)

A pure computation package, testable without any server:

```go
// MatrixScope selects the axis granularity.
type MatrixScope string
const (
    ScopeZone    MatrixScope = "zone"     // default: SourceZones × DestinationZones
    ScopeSegment MatrixScope = "segment"  // VLAN/subnet segments (joins Zone→NetworkNode)
    ScopeDevice  MatrixScope = "device"   // firewall-per-cell for multi-fw estates
)

type CellVerdict string
const (
    VerdictAllow   CellVerdict = "allow"    // at least one enabled allow, no broad deny first
    VerdictDeny    CellVerdict = "deny"     // explicit deny or implicit deny only
    VerdictMixed   CellVerdict = "mixed"    // allows and denies interleaved
    VerdictNoRule  CellVerdict = "no-rule"  // falls to implicit default
)

type MatrixCell struct {
    SrcZone, DstZone string
    Verdict          CellVerdict
    RuleCount        int
    AllowCount       int
    DenyCount        int
    DisabledCount    int
    ShadowedCount    int      // rules that can never match (see 3.2)
    AnyAnyRisk       bool     // an enabled allow with AnySource && AnyDestination
    Services         []string // aggregated, capped at N with "+k more"
    Firewalls        []string // devices contributing rules to this cell
    RuleIDs          []string // ordered by effective evaluation order — the drill-down key
}

type RuleMatrix struct {
    Scope    MatrixScope
    SrcZones []string      // ordered axes (stable sort: by risk desc, then name)
    DstZones []string
    Cells    map[string]MatrixCell // "src|dst"
    Stats    MatrixStats           // totals for header chips
}

func BuildRuleMatrix(policies []models.FirewallPolicy, opts MatrixOptions) RuleMatrix
```

Implementation notes:

- Cell verdict is computed by walking rules in `RuleOrder` per firewall/`VirtualContext` — the same semantics `L4FirewallDecisionEngine` uses, so the matrix and path analysis can never disagree. Extract the shared ordering/first-match logic into a helper both consume.
- A rule with multiple src/dst zones contributes to every cell in its cross product (mirrors `AddFirewallPolicy`'s existing expansion), but the matrix stores rule *references*, not copies.
- `MatrixOptions` carries filters (vendor, firewall, context, enabled-only, service, address substring) so filtering happens server-side — the client never receives more than the visible matrix.

### 3.2 Shadowed & risky rule detection (the analyst payoff)

New `platform/policy/analysis.go`:

- **Shadowing:** rule B is shadowed if an earlier rule A (same cell walk) matches a superset of B's src/dst/service tuple with any action. Report `ShadowedBy: ruleID` so M1 renders it struck-through with a "shadowed by ⟶" link.
- **Redundancy:** later rule with same action fully covered by an earlier one.
- **Risk flags per rule:** any/any allow, allow without logging, disabled-but-referenced, overly broad service (`any`, `0-65535`), deny-after-broad-allow (dead deny).

These annotations ride along in M1 and roll up into cell badges at M0 (`ShadowedCount`, `AnyAnyRisk`). This is what makes the matrix *more* than a pivot table.

### 3.3 Matrix UX & scalability

- **Rendering:** virtualized canvas grid (only visible cells drawn). A 200×200 zone matrix is 40k cells — trivial on canvas, fatal as DOM nodes. Row/column virtualization + sticky axis headers.
- **Cell encoding:** background = verdict color (allow green / deny red-tinted / mixed amber / no-rule neutral); corner badge = shadowed count; dot = any/any risk. Numbers appear at zoom ≥ threshold (LOD for the matrix too).
- **Axis management at scale:** collapse zones into zone-groups (site or Purdue level) with expandable axis headers — the same expand-in-place gesture as the graph. Search-to-pin: typing a zone pins its row/column to the edge.
- **Diff mode:** two snapshots → cells colored by change (new rule / removed / verdict changed). Reuses `platform/configdiff` machinery.
- **M1 rule list:** virtualized table, evaluation order preserved, shadowed rows dimmed, inline chips for services/addresses that expand on click (M2) without leaving the list.

---

## 4. Topology Graph — Enhancements

### 4.1 Backend: from export-shaped to query-shaped

New `platform/graph/service` package with a thin HTTP layer (`platform/cmd/graphlayer-serve`, stdlib `net/http` — no framework dependency needed yet):

```
GET  /api/v1/graphs/:id/summary                  # counts, sites, zones, levels → renders L0
GET  /api/v1/graphs/:id/nodes?level=&parent=&bbox=&cursor=   # children of a supernode, paged
GET  /api/v1/graphs/:id/neighborhood/:nodeID?hops=1..3       # expand-in-place payload
GET  /api/v1/graphs/:id/node/:nodeID             # L3 inspector (interfaces, routes, policies)
GET  /api/v1/graphs/:id/evidence/:objectID       # L4 evidence (config lines, hashes, run)
GET  /api/v1/graphs/:id/matrix?scope=&filters=   # RuleMatrix (M0)
GET  /api/v1/graphs/:id/matrix/cell/:src/:dst    # ordered annotated rules (M1)
GET  /api/v1/rules/:id                           # rule detail (M2)
POST /api/v1/graphs/:id/query                    # query_dsl-style path/filter queries
POST /api/v1/graphs/:id/analyze                  # existing L2/L3/L4 decision engines
```

Supporting changes in `platform/graph`:

- **Stable, typed IDs.** Current `deviceNodeID`/`zoneNodeID` conventions are close; formalize as `kind:scope:name` (e.g. `zone:fw1:dmz`, `rule:fw1:vd1:104`) and use them everywhere — node IDs, matrix `RuleIDs`, evidence keys. Deep-click is only as good as ID stability.
- **Hierarchy in the model.** Add `ParentID string` + `ChildCount int` to `GraphNode`. `applyVLANGrouping` already builds group nodes but serializes `children` as a `fmt.Sprintf("%v", ...)` string blob (`builder.go:326`) — replace with real structure. Add grouping levels: site → zone/Purdue → segment → device → asset.
- **Server-side aggregation.** `Summarize(graph, level)` collapses to supernodes with rolled-up counts, aggregate edge weights (N underlying links → one edge with `weight=N`), and dominant-attribute coloring. L0/L1 payloads stay small no matter how big the estate is.
- **Fix `confidenceFromLifecycle`.** It returns a hardcoded 0.5 (`builder.go:390`); the confidence-thickness overlay is currently decorative. Derive from lifecycle recency + corroborating-source count so confidence drill-down means something.
- **Precomputed metrics as node attributes.** Betweenness/PageRank/articulation already exist in `platform/orchestration/graph`; run at build time and attach, so the UI can offer "size by criticality" and flag articulation points (single points of failure) without client-side computation.

### 4.2 Frontend: scalable rendering strategy

Stack per the existing proposal (Next.js + TypeScript), with the graph engine chosen for scale:

| Estate size | Strategy |
|---|---|
| ≤ 2k visible elements | Cytoscape.js or Sigma.js default renderer — fastest to ship, rich interaction for free |
| 2k–50k | Sigma.js v3 (WebGL) with graphology backing store |
| Beyond | Never happens on screen: L0/L1 aggregation guarantees the viewport holds supernodes, not the raw estate |

**The core scalability principle: the client never holds the whole graph.** Aggregation on the server (§4.1) plus these client techniques:

1. **Level-of-detail:** zoomed out → dots + aggregate edges, no labels; mid → shapes + edge bundling; close → labels, icons, badges, port annotations.
2. **Viewport culling** via graphology-index quadtree; off-screen elements skip the render loop entirely.
3. **Incremental layout:** layout in a Web Worker (force-directed for segments, dagre/ELK layered for Purdue/hierarchical views). Expanded children are seeded at the parent's position and only the local neighborhood re-simulates — no global re-layout on drill.
4. **Progressive loading:** L0 renders in one round-trip (<50 nodes); deeper levels stream on demand via the neighborhood endpoint with cursor pagination.
5. **Optimistic expand:** on double-click, immediately show ghost children (count known from `ChildCount`) while the fetch is in flight; hydrate when data lands. Perceived latency ≈ 0.

**Interaction inventory (beyond the ladder):**

- Persistent **inspector panel** (right side): selection details at every level, with tabs Overview / Interfaces / Rules / Evidence mirroring L3→L4.
- **Search-first navigation:** ⌘K palette matching hostname/IP/subnet/zone/rule name → flies to and auto-expands the result path (drives the same drill APIs).
- **Isolate mode:** ego network N hops from selection; everything else fades. Escape restores.
- **Overlay toggles** as first-class layer chips (Security policy / VLAN / Confidence / Flow / Risk), mapping to existing `GraphOverlay` — but applied server-side per request instead of at one-shot build time.
- **Saved views:** named `{level, focus, filters, overlays, positions}` snapshots, shareable by URL.

### 4.3 What stays

The exporters (GraphML/GEXF/DOT/Cytoscape/D3) remain as the "download" path — they now consume the same `TopologyGraph` the service serves, so exports always match what's on screen (export honors current filters/level).

---

## 5. Performance Budgets (acceptance criteria)

| Interaction | Budget |
|---|---|
| L0 overview first paint | < 1s (payload < 100 KB) |
| Drill (expand supernode, ≤ 500 children) | < 300 ms server + ghost-fill immediately |
| Inspector open (L3) | < 200 ms |
| Evidence fetch (L4) | < 500 ms |
| Matrix M0 (200×200 zones) | < 1s compute + < 16 ms/frame scroll |
| M1 cell open (≤ 5k rules) | < 400 ms, virtualized scroll at 60 fps |
| Pan/zoom at any level | 60 fps (WebGL path), never < 30 |
| Path analysis round trip | < 5 s with per-hop rule links |

Add a `testdata/`-driven benchmark harness: synthetic estate generator (S/M/L/XL: 100 / 2k / 20k / 100k assets, 10 / 50 / 200 zones, 1k / 20k / 200k rules) + Go benchmarks for `BuildRuleMatrix`, `Summarize`, and neighborhood queries. Budgets above become CI assertions on the L size.

---

## 6. Phased Roadmap

### Phase A — Backend foundations (2–3 wks) *← highest leverage, zero UI risk*
1. `platform/policy/matrix`: `BuildRuleMatrix` + cell verdict walk shared with L4 engine. Tests against existing vendor testdata.
2. `platform/policy/analysis.go`: shadowing/redundancy/risk flags. Tests.
3. Graph model upgrades: typed stable IDs, `ParentID`/`ChildCount`, structured children, real `confidenceFromLifecycle`, `Summarize(level)`.
4. Synthetic estate generator + benchmarks.

### Phase B — Serving layer (1–2 wks)
5. `platform/cmd/graphlayer-serve`: endpoints in §4.1, stdlib HTTP, JSON, cursor pagination, ETag caching on immutable snapshots.
6. Wire decision engines behind `/analyze`; port `query_dsl` semantics behind `/query` (Go re-implementation or subprocess bridge — decide by benchmark).

### Phase C — Frontend MVP (3–4 wks)
7. Next.js scaffold, dark theme per existing proposal.
8. Topology viewer: Sigma.js/graphology, L0→L2 drill ladder, inspector panel, search palette, URL state.
9. Rule Matrix: virtualized canvas grid, M0→M1, verdict coloring + badges.

### Phase D — Deep-click completion (2–3 wks)
10. L3 inspector tabs + L4 evidence view (config-line provenance).
11. M2 rule detail + object expansion; M3 evidence; graph⇄matrix cross-links.
12. Isolate mode, saved views, overlay chips, export-current-view.

### Phase E — Scale & polish (2 wks)
13. LOD tuning, worker layout, optimistic expand, culling.
14. Matrix zone-grouping + diff mode.
15. Budget CI, e2e tests (Playwright) over the synthetic L estate.

Dependency shape: A → B → C; D and E overlap C's tail. Phases A–B are pure Go in this repo and independently shippable (matrix + summary usable from the CLI immediately, e.g. `graphlayer-matrix --format json|csv`).

---

## 7. Decisions Taken / Open Questions

**Taken (revisit only with cause):**
- Stdlib `net/http` over a framework — the API surface is small and read-heavy.
- Sigma.js v3 over PIXI-from-scratch — proven WebGL graph renderer, keeps Phase C short; PIXI remains the escape hatch if custom drawing outgrows it.
- Matrix computed in Go next to the decision engines — one source of truth for rule-order semantics.
- Aggregation-first scalability (server clamps payloads) over client-side heroics.

**Open:**
- [ ] Snapshot store: filesystem JSON (Phase B) vs PostgreSQL (per the k8s proposal) — start with filesystem, the API contract doesn't change.
- [ ] Zone identity across firewalls: same zone name on two firewalls — one axis entry or two? Proposal: two by default (`fw:zone`), with an opt-in name-merge toggle.
- [ ] Hit-count ingestion (rule usage data) — powerful for the matrix (unused-rule detection) but requires operational connectors; slot as Phase F.

---

*Related docs: `docs/FRONTEND_PROPOSAL_V1.md`, `docs/FRONTEND_ARCHITECTURE_SUMMARY.md` (stack & deployment context), `docs/graph_rust_migration_plan.md`.*
