//go:build suggestions

package cisco

/*
Suggestions for Cisco L2 Parsing & Mapping
==========================================

Parser Extensions (cisco.ios.parsers.go)
----------------------------------------
1. Extend `parseBlockConfig` usage to capture switchport lines into
   `models.CiscoL2Interface`:
   - `switchport mode access|trunk`
   - `switchport access vlan <id>`
   - `switchport trunk allowed vlan <list>` (expand ranges → []int)
   - `switchport trunk native vlan <id>`
   - `ip dhcp snooping trust`
   - `ip arp inspection trust`
   - `ip verify source`
   - `authentication port-control auto`
   - `mab`
   - `authentication event fail action authorize vlan <id>`

2. Compute snippet hashes via helper:
   ```go
   func computeSnippetHash(lines []string) string
   ```
   Normalize lines (strip comments/blank, lower-case, sort order-insensitive
   elements) → `sha256:<hex>`.

3. Parse VLAN blocks (`vlan <id>`), capturing `name`, `shutdown`, hashing the
   stanza. NX-OS support uses similar logic. // L2 segmentation catalogue

4. Detect Port-channel membership via `channel-group <id>` and map to
   `CiscoPortChannel` plus reference from member interfaces. // L2 aggregation visibility

5. Capture global L2 controls:
   - `ip dhcp snooping [vlan ...]`
   - `ip arp inspection vlan ...`
   - `ip device tracking`
   - `dot1x system-auth-control`
   // Device integrity state for Conditional/Blocked logic

Mapper Extensions (cisco.ios.mappers.go)
---------------------------------------
* After parsing, build `models.L2Switch` by transforming `CiscoL2Config`.
* Map interfaces to `models.L2SwitchPort`, preserving security flags and VLANs.
* Store `models.FDBEntry` from telemetry ingestion into Mongo collection.
* Augment `TopologyDevice` with new L2 data while keeping existing L3 behaviour.

FDB Ingestion
-------------
* Implement helper reading CLI output of `show mac address-table dynamic`.
* Provide SNMP collector using BRIDGE-MIB/Q-BRIDGE-MIB OIDs.
* Convert results to `models.CiscoFDBEntry` with `EvidenceSource` and
  confidence scoring (1.0 for switch, 0.6 for Claroty).
* Upsert into `l2_fdb_entries` collection and update `asset_nodes.l2_placement`.

Testing Strategy
----------------
* Unit tests for VLAN list expansion, interface parsing, global control parsing.
* Integration test with sample IOS/NX-OS configs verifying L2Switch output.
* FDB ingestion tests using mocked CLI/SNMP responses.

Phase Breakdown
---------------
* MVP: VLANs, access/trunk ports, DHCP snooping, DAI, IPSG, 802.1X, FDB ingestion.
* Phase 2: PVLAN parsing, PACL references, config drift detection alerts.
*/

