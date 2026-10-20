# Topology workspace

The topology explorer uses an embedded D3 7.9.0 bundle and SVG device cards.
Firewalls, routers, switches, hosts, servers and generic network devices have
distinct symbols. The current imported ASA/PAN-OS/FortiOS snapshots are classified
as firewalls by Rust. **Display icon** is an explicitly labeled, tab-local
annotation; changing it does not add router/switch parser support, alter a device
model or change an investigation result.

The map receives a disposable `project_topology` view from the Rust worker through
the session/CSRF-protected `POST /api/topology-view` endpoint. The HTTP adapter loads
sanitized snapshots from the local store. For a saved investigation identity it
uses the saved request and result, ignoring client request replacements. Rust
constructs the graph, lays out declared fan-out levels, resolves filter metadata
and checks source-reference ownership. JavaScript uses D3 for canvas geometry and
collision spacing, and manages presentation filters and selection. No JavaScript
configuration parser, route evaluator or alternate analysis engine is involved.

## Read and explore the map

- Device cards show a role-specific symbol and distinct selected-path state.
  Declared links retain both interface names. Forward and return overlays have
  separate arrows and colors; uncertain return paths stay visibly uncertain.
- Source/destination host symbols are **query endpoints**, drawn with logical
  endpoint links. They are addresses from the submitted flow, not discovered
  inventory assets or proof of a physical host/server connection. NAT tuple
  changes remain in the selected hop details and saved analysis.
- Clicking a device opens expandable identity, configuration, selected-path and
  evidence sections. Clicking a link opens its endpoint ports, provenance and
  snapshot-qualified source links. Evidence opens the shared physical-line drawer.
  The canvas starts at full width; Hide details closes the inspector again.
  Full screen expands the workbench. Escape closes source evidence first, then
  exits the expanded map and restores focus to its control.
- Search, exact facet filters, layer toggles, focus, fit, wheel zoom and background
  drag change only the current view. Filtered nodes/links are hidden without
  recomputing or rewriting saved analysis. Clearing filters restores the map.
- Traffic observations are explicitly unavailable for the current offline input
  contract. Flow addresses, protocol and transport ports are query/model facts;
  they are not packet counts, rates, live sessions or utilization measurements.

## Filters and provenance

| Filter | Available evidence | Boundaries |
| --- | --- | --- |
| Zone | Parsed interface zone with owned interface evidence | Primarily PAN-OS; no inferred zone names from ASA security levels |
| Subnet | Network prefix normalized in Rust from an evidence-backed interface address | Configuration prefix, not a discovered host population |
| VRF / virtual router | PAN-OS virtual-router interface membership with the exact sanitized source line and attached parser reference | Unsupported, detached or conflicting memberships remain unavailable |
| VDOM | Supported FortiGate interface VDOM or the root context of a single-VDOM snapshot | Context-derived root is labeled; this does not add multi-VDOM import support |

Facet counts refer to matching devices, not interface counts. Missing dimensions
are shown as unavailable. Choosing several filters intersects their device matches;
filtering does not establish policy or connectivity between those groups.
Each device matches values from all of its projected interfaces, including
secondary interface subnets.

The current analysis limits remain 16 snapshot-pinned devices and 32 declared
links, plus up to two query endpoint symbols in the view. This redesign does not
expand the imported-vendor contract or claim arbitrary enterprise graph scale.
Saved workspace-v1 through v7 bytes, snapshot IDs and source references remain
unchanged. Presentation annotations are not included in investigation exports.

The D3 bundle is loaded from the application itself under the existing CSP; no
CDN is contacted. Its ISC license, exact upstream package and integrity hashes are
recorded in [third_party/d3](../third_party/d3/README.md). Project licensing remains
undecided. D3's [collision force](https://d3js.org/d3-force/collide) is used for view
spacing; it does not perform network path analysis.

The local investigation app has four focused pages. Workspace home manages the
workspace name, named datasets and active snapshot set. Sources & imports owns
site creation, reviewed batches and single configuration imports. Investigation
owns the selected snapshot, policy queries, single-firewall routing/NAT, rule and
object inventories and saved investigations. Network path owns declared
multi-device topology, forward/return analysis and saved path explorations.

The page URL is shareable within the same local app: `#workspace`, `#imports`,
`#investigation` and `#network-path` (`#topology` is an alias). Inventory and object
anchors still open their containing page. Browser back/forward changes pages;
navigation preserves form drafts and the selected snapshot in the current tab.
Dataset membership remains pinned to immutable snapshot IDs. This does not add
remote sharing, multiple workspaces in one process or enterprise user accounts.

## Explore a dataset

1. In Sources & imports, load the labeled network batch or select exported ASA,
   PAN-OS set-format and FortiOS single-VDOM configuration files. Validate every
   item, review sanitized source previews, name the dataset and publish it.
2. Workspace home shows the activated dataset. Choose its network action to open
   Network path with one declared device per member. Configure links, endpoint
   interfaces and a concrete flow. Imported files do not establish physical links.
3. The canvas shows the declared draft before analysis. Its caption explicitly
   says analysis has not run. Device selection opens the saved configuration
   model; it does not infer a path or permit.
4. Run a deterministic trace or explore configured route alternatives. The
   exploration's branch and return selectors project the saved server result onto
   the same canvas. Asymmetric route and session uncertainty remain visible.
5. Search by device ID, hostname, vendor or context. Select a device from the
   canvas or search results; use Focus selected, Fit, zoom buttons, wheel zoom or
   background drag to navigate. Forward and Return toggles only hide overlays.
6. Open source links in the inspector or hop cards. The shared source drawer
   displays sanitized physical lines from that exact snapshot. Close it or press
   Escape to return focus to the source trigger. Evidence from a changed draft or
   replaced result is rejected.

The inspector displays bounded configuration inventories, coverage warnings,
selected route/NAT tuples and source references. Omitted items remain available
in the snapshot/result API and JSON export. A declared link, route candidate or
green overlay does not prove live reachability. Session state, packet captures,
counters and other runtime observations are unavailable in this offline workflow.

## Architecture and verification

The new JavaScript modules own page state, canvas geometry and result rendering.
They do not parse configurations or evaluate route, NAT, policy or graph rules.
The existing server analysis contract and workspace-v7 records are unchanged.
The Rust language policy and legacy migration inventory remain in force; see
[Rust architecture](RUST_ARCHITECTURE.md) for the current implementation boundary.

All assets, including the pinned D3 bundle, are embedded, served through an explicit HTTP allowlist and covered by
the existing restrictive CSP. There are no external fonts, scripts or graph APIs.
Browser tests cover imports, dataset activation, route navigation, draft graphs,
branch/return selection, search/inspection, canvas controls, snapshot-qualified
evidence, export and exact replay on desktop and mobile. HTTP tests check every
new script and stylesheet's availability, content type and security headers.
Container and native Windows packaging checks launch the actual app with its
sibling Rust worker, load the embedded assets and import the ASA path sample
through HTTP with the worker environment override removed.

Project licensing is still undecided. This UI work does not assign a license or
publish the repository or packages.
