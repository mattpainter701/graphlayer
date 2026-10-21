# Topology explorer

GraphLayer provides two complementary maps. **Network map** explores the
explicitly synthetic host lab through Rust Layer 3, zone and paged Layer 2
projections. **Network topology** inspects declared links between snapshot-pinned
devices and configured forward and return paths. Neither view establishes live
reachability, traffic volume or an active session.

## Navigate large maps

Start with Layer 3 or Zones to see an aggregate overview. Selecting a network
opens its host and Layer 2 projection; selecting a zone narrows the Layer 3 view.
Search by host name, IP or MAC, or use network and zone filters. Reset filters
returns to the unfiltered projection. Host pages contain 25, 50 or 100 hosts;
the total dataset count remains visible independently of the current page.

Pan on empty canvas space, zoom around the pointer, or use the controls. Focus
the canvas to use the following keys:

| Action | Network map | Network topology |
| --- | --- | --- |
| Pan | Arrow keys; Shift for larger steps | Arrow keys |
| Zoom | `+` / `-` | `+` / `-` |
| Fit | `Home` | `0` or `Home` |
| Inspect a focused device or link | `Enter` or Space | `Enter` or Space |

The topology overview shows the full layout and current viewport. Click it to
recenter, or activate it with the keyboard to fit the graph. Fine labels are
reduced at overview zoom levels; zoom in for port and status details. These
controls update presentation without rerunning path analysis.

## Inspect configuration and evidence

Select a device, host or link to open its details. On wide screens, the network
map places details beside the canvas; on smaller screens they stack. The Details
control hides or reveals the current selection without changing the projection.

Host details include the projected IP, MAC, network, zone, VLAN and attachment
information when present, plus connections and source references. Device
shortcuts open interfaces, routes, selected path hops and source evidence. Saved
configuration and path groups retain their bounded lists and report omitted
records instead of implying complete display. Snapshot identity and source
ownership stay attached to the inspected record.

Use the forward branch and return candidate controls to compare configured
paths. An asymmetric return or an unproven session remains explicit. Link
inspectors identify endpoint interfaces and declared provenance. Source buttons
open the existing line-addressed evidence; an icon annotation changes display
only and does not alter the saved model.

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
