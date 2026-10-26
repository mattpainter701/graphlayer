# Topology explorer

GraphLayer provides two complementary maps. **Network map** explores the
explicitly synthetic host lab through Rust Layer 3, zone and paged Layer 2
projections. **Network topology** inspects declared links between snapshot-pinned
devices and configured forward and return paths. Neither view establishes live
reachability, traffic volume or an active session.

## Navigate large maps

Start with Layer 3 or Zones to see an aggregate overview. Selecting a network
opens its host and Layer 2 projection; selecting a zone narrows the Layer 3 view.
Search by host name, IP or MAC, or use network, zone, VRF and VDOM filters. Reset filters
returns to the unfiltered projection. Host pages contain 25, 50 or 100 hosts;
the total dataset count remains visible independently of the current page.

The collapsible **Host results** navigator lists the bounded host page in every
view, including Layer 3 and Zones. Its count distinguishes matching hosts from
the complete dataset. Use First, Previous, Next or Last to move through results,
or change the page size. Rust clamps stale pages and selects the page containing
a requested host; the browser does not scan or build a second topology.

Inspect a result to read its inventory and source evidence, or choose **Show on
map** to open its Layer 2 network and select that host. VRF, VDOM, zone and search
constraints remain active. The map does not clear a restrictive context to make
a host appear. **Back** restores the prior view, filters, host page, page size,
selection and map position. This bounded navigation history is local to the
current browser session and does not change saved investigations.

Network and zone inputs suggest values from the loaded synthetic dataset.
VRF and VDOM suggestions come from the Rust projection's declared context facets.
Filters intersect: selecting a network in a different VRF or VDOM produces an
empty result. Clearing one chip preserves the remaining constraints. Legacy
datasets without context metadata still load; selecting a named context does
not assign those records to an invented default.
Active filters appear as removable chips, so you can clear one constraint
without losing the others. An empty result offers a clear-filter action. Search
updates are debounced, and an earlier response cannot replace the latest query.
The map and inspector show an updating state while a new Rust projection loads.

Network and zone suggestions include Rust-calculated matching-host counts under
the other active constraints. Text search matches inventory identity, addresses,
MAC, attachment and context fields without treating evidence hashes or source
reference identifiers as host matches. Text matching ignores case; declared
VRF and VDOM filters preserve their exact case.

Pan on empty canvas space, zoom around the pointer, or use the controls. Focus
the canvas to use the following keys:

| Action | Network map | Network topology |
| --- | --- | --- |
| Pan | Arrow keys on the canvas; Shift for larger steps | Arrow keys on the canvas |
| Zoom | `+` / `-` | `+` / `-` |
| Fit | `Home` | `0` or `Home` |
| Move between graph items | Right/Down next; Left/Up previous; Home/End first/last | Right/Down next; Left/Up previous; Home/End first/last |
| Inspect the focused graph item | `Enter` or Space | `Enter` or Space |
| Return focus to canvas | `Escape` | `Escape` |

The maps use one keyboard tab stop for graph items. Arrow navigation moves
focus through visible items and centers the focused item in the canvas; Home
and End move to the first and last visible items. Enter or Space inspects the
focused item, and Escape returns focus to the canvas. With the canvas focused,
arrow keys pan instead. The overview shows the full layout and current
viewport; click or keyboard activation recenters the canvas there. Fine labels
are reduced at overview zoom levels; zoom in for port and status details. Focus
and camera controls update presentation without rerunning a projection or path
analysis. Fit returns to the whole current projection.

On dense 100-host pages, focus a host to make its card easier to inspect, then
use the overview to recenter or Fit to return to the whole projection. A mobile
viewport cannot make every label in a dense overview readable at once; use host
focus and zoom for individual details. The 25, 50 and 100 host page sizes remain
bounded, and navigation does not change the underlying topology or run domain
analysis again.
Crowded fullscreen layouts scroll the panel while retaining bounded space for
the canvas and inspector. Restoring an expanded port does not enlarge the
canvas; the explanatory footer remains below both.

## Inspect configuration and evidence

Select a device, host or link to open its details. On wide screens, the network
map places details beside the canvas; on smaller screens they stack. The Details
control hides or reveals the current selection without changing the projection.

Host details include the projected IP, MAC, network, zone, VLAN and attachment
information when present, plus declared VRF/VDOM, connections and source references. Device
shortcuts open interfaces, routes, selected path hops and source evidence. Saved
configuration and path groups retain their bounded lists and report omitted
records instead of implying complete display. Snapshot identity and source
ownership stay attached to the inspected record.

Follow a projected connection from the network-map inspector to inspect its
other endpoint, or assign a selected host as the source or destination of a
Layer 2 trace. An explicitly chosen source remains selected while you inspect
and assign the destination. In the path workbench, declared-connection buttons open connected
devices and link details; each link inspector can open either endpoint. These
actions navigate the existing projection and do not run another investigation.

Host results can assign either trace endpoint without first finding it on the
canvas. A pinned source stays selected while searching for the destination.

The host view groups its bounded fan-out by attachment using Rust-supplied
positions. Select a connected port or link to inspect both endpoint ports,
VLAN, available scope and source references. An ordered Layer 2 hop opens the
matching device or link without discarding the returned trace. Endpoints outside
the selected context produce an explicit unavailable path with no path hops or
highlighted links. Scope labels describe the synthetic inventory; they do not
establish real routing between VRFs or VDOMs.
Network, zone and search narrow the displayed map; an explicit trace follows
its selected endpoints within the VRF/VDOM boundary. A cross-network request
still reports that Layer 3 analysis is required.
Focused Layer 2 traces use a separate compact Rust layout, oriented vertically
on narrow canvases and horizontally when space permits. Ingress and egress
controls identify their returned ports separately; resizing or opening details
keeps the focused trace readable without changing the investigation.

Path-workbench facet chips narrow visible devices by zone, subnet, VRF or VDOM
when the Rust projection supplies those facets. Find highlights matching devices
and narrows the device list. The filter status explains this distinction, and
an empty map provides a reset action. Clearing a chip preserves other facets.

Use the forward branch and return candidate controls to compare configured
paths. An asymmetric return or an unproven session remains explicit. Link
inspectors identify endpoint interfaces and declared provenance. Source buttons
open the existing line-addressed evidence; an icon annotation changes display
only and does not alter the saved model.

Expand an interface in the device inspector for its returned address, mask,
zone, context, configured status, exact declared connections and selected path
hops. Expand a forward or return hop to inspect ingress/egress, the selected
route, flow tuples, returned NAT information and decision-stage evidence.
Interface buttons in a link inspector open the owning device's port details.
Missing configuration facts remain unavailable, and evidence links retain their
owning snapshot. None of these drilldowns executes another analysis or implies
an observed port state or active session.

The device inspector includes a port finder over its returned interface list.
Port summaries show the declared connection and selected forward/return use
counts. A selected port remains available while the finder is narrowed, and a
clear action recovers from an empty match. Decision stages and their source
references expand separately from the concise route and tuple summary.

Inspector breadcrumbs and Back/Forward controls follow a device, its port, a
declared link and either endpoint's port. The bounded trail retains disclosure
state for the same saved analysis and snapshots. Loading another analysis or
changing the owning snapshot discards stale history. Port and source navigation
reuses the returned result and does not submit another path investigation.

## Scale and limits

Rendering uses aggregate projections and bounded host pages. Pan and zoom writes
are coalesced into animation frames; presentation lookups use response indexes.
Rust indexes host and device identities during validation without changing the
projection or source evidence format.

The current lab has 1,000 hosts across ten networks. Host-count limits do not
guarantee that every larger dataset fits: source, request and evidence byte
budgets still apply and oversized input is rejected. Full auxiliary sources
remain in each projection for compatibility, so payload size grows with the
dataset. The map does not claim unrestricted scale or live discovery.

All bundled examples are synthetic. Keep customer configurations and collection
reports outside Git and CI. Saved workspace records, source hashes and previous
investigation identities are preserved.
