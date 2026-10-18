# Topology workspace

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

All assets are embedded, served through an explicit HTTP allowlist and covered by
the existing restrictive CSP. There are no external fonts, scripts or graph APIs.
Browser tests cover imports, dataset activation, route navigation, draft graphs,
branch/return selection, search/inspection, canvas controls, snapshot-qualified
evidence, export and exact replay on desktop and mobile. HTTP tests check every
new script and stylesheet's availability, content type and security headers.
Container and native Windows packaging checks launch the actual app with its
sibling Rust worker, load all six embedded assets and import the ASA path sample
through HTTP with the worker environment override removed.

Project licensing is still undecided. This UI work does not assign a license or
publish the repository or packages.
