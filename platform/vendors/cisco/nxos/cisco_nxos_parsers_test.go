package cisco

import (
	"testing"
)

// ===== Test Data Samples (Based on Cisco NXOS Documentation) =====

const sampleNXOSMACTable = `Legend:
        * - primary entry, G - Gateway MAC, (R) - Routed MAC, O - Overlay MAC
        age - seconds since last seen,+ - primary entry using vPC Peer-Link,
        (T) - True, (F) - False, C - ControlPlane MAC
   VLAN     MAC Address      Type      age     Secure NTFY Ports
---------+-----------------+--------+---------+------+----+------------------
*  10     0050.56bb.0001     dynamic  300        F      F    Eth1/1
*  10     0050.56bb.0002     dynamic  150        F      F    Po10+
*  20     0050.56bb.0003     dynamic  450        F      F    Eth1/2
   100    0050.56bb.0004     static   -          F      F    Eth1/10
`

const sampleNXOSVLAN = `VLAN Name                             Status    Ports
---- -------------------------------- --------- -------------------------------
1    default                          active    Eth1/10, Eth1/11, Eth1/12
10   Engineering                      active    Eth1/1, Eth1/2, Po10
20   Production                       active    Eth1/3, Eth1/4
100  Management                       active    Eth1/5
999  Quarantine                       suspended
`

const sampleNXOSInterfaceStatus = `--------------------------------------------------------------------------------
Port          Name               Status    Vlan      Duplex  Speed   Type
--------------------------------------------------------------------------------
Eth1/1        uplink-to-core     connected 10        full    10G     10Gbase-SR
Eth1/2        server-link        connected 20        full    1000    1000base-T
Eth1/3        --                 notconnect 1        auto    auto    10Gbase-SR
Po10          VPC-Peer-Link      connected trunk     full    20G     --
`

const sampleNXOSTrunk = `Port        Native  Status        Port
            Vlan                      Channel
--------------------------------------------------------------------------------
Eth1/2        1       trunking      --
Po10          1       trunking      10

Port        Vlans Allowed on Trunk
--------------------------------------------------------------------------------
Eth1/2        1,10,20,30,40,50
Po10          1-4094
`

const sampleNXOSPortChannel = `Flags:  D - Down        P - Up in port-channel (members)
        I - Individual  H - Hot-standby (LACP only)
        s - Suspended   r - Module-removed
        b - BFD Session Wait
        S - Switched    R - Routed
        U - Up (port-channel)
        p - Up in delay-lacp mode (member)
        M - Not in use. Min-links not met
--------------------------------------------------------------------------------
Group Port-       Type     Protocol  Member Ports
      Channel
--------------------------------------------------------------------------------
10    Po10(SU)    Eth      LACP      Eth1/3(P)    Eth1/4(P)
20    Po20(SD)    Eth      LACP      Eth1/5(D)    Eth1/6(D)
`

const sampleNXOSSTPSummary = `Switch is in rapid-pvst mode
Root bridge for: VLAN0001, VLAN0010, VLAN0020
Extended system ID           is enabled
Portfast Default             is disabled
Portfast Edge BPDU Guard Default is disabled
Portfast Edge BPDU Filter Default is disabled
Loopguard Default            is disabled
PVST Simulation Default      is enabled
Bridge Assurance             is enabled
Etherchannel misconfig guard is enabled
UplinkFast                   is disabled
BackboneFast                 is disabled

Name                   Blocking Listening Learning Forwarding STP Active
---------------------- -------- --------- -------- ---------- ----------
VLAN0001                     0         0        0         15         15
VLAN0010                     0         0        0         10         10
VLAN0020                     0         0        0          8          8
---------------------- -------- --------- -------- ---------- ----------
3 vlans                      0         0        0         33         33
`

const sampleNXOSLLDPNeighbors = `Chassis id: 0016.4748.ac4e
Port id: Ethernet1/1
Local Port id: Eth1/1
Port Description: uplink-to-spine-1
System Name: SPINE-01
System Description: Cisco Nexus Operating System (NX-OS) Software 9.3(7)
Time remaining: 98 seconds
System Capabilities: B, R
Enabled Capabilities: B, R
Management Address: 10.1.1.1
Vlan ID: 10

Chassis id: 0050.56bb.1234
Port id: GigabitEthernet0/1
Local Port id: Eth1/2
Port Description: connection-to-leaf
System Name: LEAF-02
System Description: Cisco IOS Software, Version 15.2(4)E7
Time remaining: 112 seconds
System Capabilities: B, R
Enabled Capabilities: B, R
Management Address: 10.1.1.2
Vlan ID: 20
`

const sampleNXOSCDP = `Device ID:SPINE-01
Interface address(es):
    IPv4 Address: 10.1.1.1
Platform: N9K-C93180YC-EX, Capabilities: Router Switch IGMP Filtering
Interface: Ethernet1/1, Port ID (outgoing port): Ethernet1/1
Holdtime: 142 sec

Version:
Cisco Nexus Operating System (NX-OS) Software, Version 9.3(7)

Native VLAN: 1
Duplex: full
`

const sampleNXOSARP = `Flags: * - Adjacencies learnt on non-active FHRP router
       + - Adjacencies synced via CFSoE
       # - Adjacencies Throttled for Glean
       CP - Added via L2RIB, Control plane Adjacencies
       PS - Added via L2RIB, Peer Sync
       RO - Re-Originated Peer Sync Entry

IP ARP Table
Total number of entries: 3
Address         Age       MAC Address     Interface       Flags
10.0.10.2       00:05:12  0050.56bb.0001  Vlan10
10.0.10.3       00:02:34  0050.56bb.0002  Vlan10
10.0.20.5       00:10:00  0050.56bb.0003  Vlan20
`

const sampleNXOSVPC = `Legend:
                (*) - local vPC is down, forwarding via vPC peer-link

vPC domain id                     : 10
Peer status                       : peer adjacency formed ok
vPC keep-alive status             : peer is alive
Configuration consistency status  : success
Per-vlan consistency status       : success
Type-2 consistency status         : success
vPC role                          : primary
Number of vPCs configured         : 5
Peer Gateway                      : Enabled
Dual-active excluded VLANs        : -
Graceful Consistency Check        : Enabled
Auto-recovery status              : Enabled
Delay-restore status              : Timer is off.(timeout = 30s)
Delay-restore SVI status          : Timer is off.(timeout = 10s)
Operational Layer3 Peer-router    : Enabled

vPC Peer-link status
---------------------------------------------------------------------
id    Port   Status Active vlans
--    ----   ------ -------------------------------------------------
1     Po10   up     1,10,20,30,40,50,100,200
`

const sampleNXOSVPCKeepalive = `vPC keep-alive status           : peer is alive
--Peer is alive for             : (166978) seconds, 1 msec
--Send status                   : Success
--Last send at                  : 2025.10.27 10:15:23 245 ms
--Sent on interface             : mgmt0
--Receive status                : Success
--Last receive at               : 2025.10.27 10:15:23 247 ms
--Received on interface         : mgmt0
--Last update from peer         : (0) seconds, (148) msec
vPC Keep-alive parameters
--Destination                   : 192.168.100.2
--Keepalive interval            : 1000 msec
--Keepalive timeout             : 5 seconds
--Keepalive hold timeout        : 3 seconds
--Keepalive vrf                 : management
--Keepalive udp port            : 3200
--Keepalive tos                 : 192
`

const sampleNXOSVPCConsistency = `    Legend:
        Type 1 : vPC will be suspended in case of mismatch

Name                        Type  Local Value            Peer Value
-------------               ----  ---------------------- -----------------------
STP MST Simulate PVST       1     Enabled                Enabled
STP MST Region Name         1     ""                     ""
STP MST Region Revision     1     0                      0
STP MST Region Instance to  1
 VLAN Mapping
STP Loopguard               1     Disabled               Disabled
STP Bridge Assurance        1     Enabled                Enabled
STP Port Type, Edge         1     Normal, Disabled,      Normal, Disabled,
BPDUGuard, Edge BPDUFilter        Disabled               Disabled
STP MST Simulate PVST       1     Enabled                Enabled
`

const sampleNXOSVDC = `vdc_id  vdc_name                          state               mac                 type        lc
------  --------                          -----               ----------          ------      ------
1       PROD-VDC                          active              00:0c:29:f2:f8:2c   Ethernet    f3-line-card
2       TEST-VDC                          suspended           00:0c:29:f2:f8:2d   Ethernet    f3-line-card

Switchwide mode: f3 fex-fabric f2e-fabric
`

const sampleNXOSFabricPath = `Fabricpath IS-IS domain: default
  Fabricpath IS-IS tag: 1
  System ID: 0016.4748.ac4e IS-Type: L1L2
  SAP: 427, Queue Handle: 14
  Maximum LSP MTU: 1492
  Graceful Restart: Disabled
  NSF: Disabled
  Topology: 0

Adjacency Database for Switch-ID 1:
-------------------------------------------------------------------------------
Sys-ID             Hostname         Port      Up Time   Bandwidth
-------------------------------------------------------------------------------
1   0016.4748.ac4e     Leaf-01         *local*     2d17h              --
10  0050.56bb.1234     Spine-01         Eth1/1     2d17h              10000000
11  0050.56bb.1235     Spine-02         Eth1/2     2d17h              10000000
`

const sampleNXOSHSRP = `     Ethernet1/1 - Group 10 (HSRP-V2) (IPv4)
       Local state is Active, priority 110 (Cfged 110), may preempt
         Forwarding threshold(for vPC), lower: 1 upper: 110
       Hellotime 3 sec, holdtime 10 sec
       Next hello sent in 1.392000 secs
       Virtual IP address is 10.0.10.1 (Cfged)
       Active router is local
       Standby router is 10.0.10.3, priority 100 expires in 8.416000 sec
       Authentication text "cisco"
       Virtual mac address is 0000.0c9f.f00a (Default MAC)
       2 state changes, last state change 2d17h
       IP redundancy name is hsrp-Eth1/1-10 (default)
`

// ===== Unit Tests =====

func TestParseNXOSMACTable(t *testing.T) {
	entries, err := ParseNXOSMACTable(sampleNXOSMACTable)
	if err != nil {
		t.Fatalf("ParseNXOSMACTable failed: %v", err)
	}

	if len(entries) != 4 {
		t.Errorf("Expected 4 entries, got %d", len(entries))
	}

	// Test first entry
	if entries[0].VLAN != 10 {
		t.Errorf("Expected VLAN 10, got %d", entries[0].VLAN)
	}
	if entries[0].MACAddress != "00:50:56:bb:00:01" {
		t.Errorf("Expected MAC 00:50:56:bb:00:01, got %s", entries[0].MACAddress)
	}
	if entries[0].Type != "dynamic" {
		t.Errorf("Expected type dynamic, got %s", entries[0].Type)
	}
	if entries[0].Age != 300 {
		t.Errorf("Expected age 300, got %d", entries[0].Age)
	}
	if entries[0].Port != "Eth1/1" {
		t.Errorf("Expected port Eth1/1, got %s", entries[0].Port)
	}
	if !entries[0].IsPrimary {
		t.Error("Expected IsPrimary to be true")
	}

	// Test VPC peer-link entry
	if !entries[1].IsVPCPeerLink {
		t.Error("Expected IsVPCPeerLink to be true for Po10+")
	}

	// Test static entry
	if entries[3].Type != "static" {
		t.Errorf("Expected type static, got %s", entries[3].Type)
	}
}

func TestParseNXOSVLAN(t *testing.T) {
	vlans, err := ParseNXOSVLAN(sampleNXOSVLAN)
	if err != nil {
		t.Fatalf("ParseNXOSVLAN failed: %v", err)
	}

	if len(vlans) != 5 {
		t.Errorf("Expected 5 VLANs, got %d", len(vlans))
	}

	// Test VLAN 10
	vlan10 := vlans[1]
	if vlan10.ID != 10 {
		t.Errorf("Expected VLAN ID 10, got %d", vlan10.ID)
	}
	if vlan10.Name != "Engineering" {
		t.Errorf("Expected name Engineering, got %s", vlan10.Name)
	}
	if vlan10.Status != "active" {
		t.Errorf("Expected status active, got %s", vlan10.Status)
	}
	if len(vlan10.Ports) != 3 {
		t.Errorf("Expected 3 ports, got %d", len(vlan10.Ports))
	}

	// Test suspended VLAN
	if vlans[4].Status != "suspended" {
		t.Errorf("Expected VLAN 999 to be suspended, got %s", vlans[4].Status)
	}
}

func TestParseNXOSInterfaceStatus(t *testing.T) {
	interfaces, err := ParseNXOSInterfaceStatus(sampleNXOSInterfaceStatus)
	if err != nil {
		t.Fatalf("ParseNXOSInterfaceStatus failed: %v", err)
	}

	if len(interfaces) != 4 {
		t.Errorf("Expected 4 interfaces, got %d", len(interfaces))
	}

	// Test Eth1/1
	eth1 := interfaces[0]
	if eth1.Port != "Eth1/1" {
		t.Errorf("Expected port Eth1/1, got %s", eth1.Port)
	}
	if eth1.Name != "uplink-to-core" {
		t.Errorf("Expected name uplink-to-core, got %s", eth1.Name)
	}
	if eth1.Status != "connected" {
		t.Errorf("Expected status connected, got %s", eth1.Status)
	}
	if eth1.VLAN != "10" {
		t.Errorf("Expected VLAN 10, got %s", eth1.VLAN)
	}
	if eth1.Speed != "10G" {
		t.Errorf("Expected speed 10G, got %s", eth1.Speed)
	}

	// Test port-channel
	po10 := interfaces[3]
	if po10.VLAN != "trunk" {
		t.Errorf("Expected VLAN trunk, got %s", po10.VLAN)
	}
}

func TestParseNXOSTrunk(t *testing.T) {
	trunks, err := ParseNXOSTrunk(sampleNXOSTrunk)
	if err != nil {
		t.Fatalf("ParseNXOSTrunk failed: %v", err)
	}

	if len(trunks) != 2 {
		t.Errorf("Expected 2 trunks, got %d", len(trunks))
	}

	// Test Po10
	po10 := trunks[1]
	if po10.Port != "Po10" {
		t.Errorf("Expected port Po10, got %s", po10.Port)
	}
	if po10.VLANsAllowed != "1-4094" {
		t.Errorf("Expected VLANs 1-4094, got %s", po10.VLANsAllowed)
	}
	if po10.Status != "trunking" {
		t.Errorf("Expected status trunking, got %s", po10.Status)
	}
}

func TestParseNXOSPortChannel(t *testing.T) {
	portChannels, err := ParseNXOSPortChannel(sampleNXOSPortChannel)
	if err != nil {
		t.Fatalf("ParseNXOSPortChannel failed: %v", err)
	}

	if len(portChannels) != 2 {
		t.Errorf("Expected 2 port-channels, got %d", len(portChannels))
	}

	// Test Po10
	po10 := portChannels[0]
	if po10.GroupID != 10 {
		t.Errorf("Expected group ID 10, got %d", po10.GroupID)
	}
	if po10.Name != "Po10" {
		t.Errorf("Expected name Po10, got %s", po10.Name)
	}
	if po10.State != "SU" {
		t.Errorf("Expected state SU, got %s", po10.State)
	}
	if po10.Protocol != "LACP" {
		t.Errorf("Expected protocol LACP, got %s", po10.Protocol)
	}
	if len(po10.Members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(po10.Members))
	}
	if po10.MemberStates["Eth1/3"] != "P" {
		t.Errorf("Expected member Eth1/3 state P, got %s", po10.MemberStates["Eth1/3"])
	}
}

func TestParseNXOSSTP(t *testing.T) {
	summary, err := ParseNXOSSTP(sampleNXOSSTPSummary)
	if err != nil {
		t.Fatalf("ParseNXOSSTP failed: %v", err)
	}

	if summary.Mode != "rapid-pvst" {
		t.Errorf("Expected mode rapid-pvst, got %s", summary.Mode)
	}

	if len(summary.RootBridgeVLANs) != 3 {
		t.Errorf("Expected 3 root bridge VLANs, got %d", len(summary.RootBridgeVLANs))
	}

	if !summary.ExtendedSystemID {
		t.Error("Expected extended system ID to be enabled")
	}

	if !summary.EtherChannelMisconfig {
		t.Error("Expected etherchannel misconfig guard to be enabled")
	}
}

func TestParseNXOSLLDP(t *testing.T) {
	neighbors, err := ParseNXOSLLDP(sampleNXOSLLDPNeighbors)
	if err != nil {
		t.Fatalf("ParseNXOSLLDP failed: %v", err)
	}

	if len(neighbors) != 2 {
		t.Errorf("Expected 2 neighbors, got %d", len(neighbors))
	}

	// Test first neighbor
	neighbor := neighbors[0]
	if neighbor.ChassisID != "0016.4748.ac4e" {
		t.Errorf("Expected chassis ID 0016.4748.ac4e, got %s", neighbor.ChassisID)
	}
	if neighbor.SystemName != "SPINE-01" {
		t.Errorf("Expected system name SPINE-01, got %s", neighbor.SystemName)
	}
	if neighbor.LocalPort != "Eth1/1" {
		t.Errorf("Expected local port Eth1/1, got %s", neighbor.LocalPort)
	}
	if neighbor.PortID != "Ethernet1/1" {
		t.Errorf("Expected port ID Ethernet1/1, got %s", neighbor.PortID)
	}
	if neighbor.ManagementAddress != "10.1.1.1" {
		t.Errorf("Expected management address 10.1.1.1, got %s", neighbor.ManagementAddress)
	}
}

func TestParseNXOSCDP(t *testing.T) {
	neighbors, err := ParseNXOSCDP(sampleNXOSCDP)
	if err != nil {
		t.Fatalf("ParseNXOSCDP failed: %v", err)
	}

	if len(neighbors) != 1 {
		t.Errorf("Expected 1 neighbor, got %d", len(neighbors))
	}

	neighbor := neighbors[0]
	if neighbor.DeviceID != "SPINE-01" {
		t.Errorf("Expected device ID SPINE-01, got %s", neighbor.DeviceID)
	}
	if neighbor.IPAddress != "10.1.1.1" {
		t.Errorf("Expected IP 10.1.1.1, got %s", neighbor.IPAddress)
	}
	if neighbor.Platform != "N9K-C93180YC-EX" {
		t.Errorf("Expected platform N9K-C93180YC-EX, got %s", neighbor.Platform)
	}
}

func TestParseNXOSARP(t *testing.T) {
	entries, err := ParseNXOSARP(sampleNXOSARP)
	if err != nil {
		t.Fatalf("ParseNXOSARP failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 ARP entries, got %d", len(entries))
	}

	// Test first entry
	entry := entries[0]
	if entry.IPAddress != "10.0.10.2" {
		t.Errorf("Expected IP 10.0.10.2, got %s", entry.IPAddress)
	}
	if entry.MACAddress != "00:50:56:bb:00:01" {
		t.Errorf("Expected MAC 00:50:56:bb:00:01, got %s", entry.MACAddress)
	}
	if entry.Interface != "Vlan10" {
		t.Errorf("Expected interface Vlan10, got %s", entry.Interface)
	}
}

// ===== VPC Tests =====

func TestParseNXOSVPC(t *testing.T) {
	vpc, err := ParseNXOSVPC(sampleNXOSVPC)
	if err != nil {
		t.Fatalf("ParseNXOSVPC failed: %v", err)
	}

	if vpc.DomainID != 10 {
		t.Errorf("Expected domain ID 10, got %d", vpc.DomainID)
	}

	if vpc.VPCRole != "primary" {
		t.Errorf("Expected role primary, got %s", vpc.VPCRole)
	}

	if vpc.NumberOfVPCs != 5 {
		t.Errorf("Expected 5 vPCs, got %d", vpc.NumberOfVPCs)
	}

	if !vpc.PeerGateway {
		t.Error("Expected peer gateway to be enabled")
	}

	if vpc.PeerLinkID != 1 {
		t.Errorf("Expected peer link ID 1, got %d", vpc.PeerLinkID)
	}

	if vpc.PeerLinkPort != "Po10" {
		t.Errorf("Expected peer link port Po10, got %s", vpc.PeerLinkPort)
	}

	if vpc.PeerLinkStatus != "up" {
		t.Errorf("Expected peer link status up, got %s", vpc.PeerLinkStatus)
	}
}

func TestParseNXOSVPCKeepalive(t *testing.T) {
	ka, err := ParseNXOSVPCKeepalive(sampleNXOSVPCKeepalive)
	if err != nil {
		t.Fatalf("ParseNXOSVPCKeepalive failed: %v", err)
	}

	if ka.Status != "peer is alive" {
		t.Errorf("Expected status 'peer is alive', got %s", ka.Status)
	}

	if ka.Destination != "192.168.100.2" {
		t.Errorf("Expected destination 192.168.100.2, got %s", ka.Destination)
	}

	if ka.Interval != "1000 msec" {
		t.Errorf("Expected interval 1000 msec, got %s", ka.Interval)
	}

	if ka.UDPPort != 3200 {
		t.Errorf("Expected UDP port 3200, got %d", ka.UDPPort)
	}

	if ka.VRF != "management" {
		t.Errorf("Expected VRF management, got %s", ka.VRF)
	}
}

func TestParseNXOSVPCConsistency(t *testing.T) {
	params, err := ParseNXOSVPCConsistency(sampleNXOSVPCConsistency)
	if err != nil {
		t.Fatalf("ParseNXOSVPCConsistency failed: %v", err)
	}

	if len(params) < 3 {
		t.Errorf("Expected at least 3 consistency parameters, got %d", len(params))
	}

	// Test first parameter
	param := params[0]
	if param.Name != "STP MST Simulate PVST" {
		t.Errorf("Expected name 'STP MST Simulate PVST', got %s", param.Name)
	}
	if param.Type != "1" {
		t.Errorf("Expected type 1, got %s", param.Type)
	}
	if param.LocalValue != "Enabled" {
		t.Errorf("Expected local value Enabled, got %s", param.LocalValue)
	}
	if param.PeerValue != "Enabled" {
		t.Errorf("Expected peer value Enabled, got %s", param.PeerValue)
	}
	if !param.Consistent {
		t.Error("Expected parameters to be consistent")
	}
}

// ===== VDC & FabricPath Tests =====

func TestParseNXOSVDC(t *testing.T) {
	vdcs, err := ParseNXOSVDC(sampleNXOSVDC)
	if err != nil {
		t.Fatalf("ParseNXOSVDC failed: %v", err)
	}

	if len(vdcs) != 2 {
		t.Errorf("Expected 2 VDCs, got %d", len(vdcs))
	}

	// Test first VDC
	vdc := vdcs[0]
	if vdc.ID != 1 {
		t.Errorf("Expected VDC ID 1, got %d", vdc.ID)
	}
	if vdc.Name != "PROD-VDC" {
		t.Errorf("Expected name PROD-VDC, got %s", vdc.Name)
	}
	if vdc.State != "active" {
		t.Errorf("Expected state active, got %s", vdc.State)
	}
	if vdc.Type != "Ethernet" {
		t.Errorf("Expected type Ethernet, got %s", vdc.Type)
	}

	// Test suspended VDC
	if vdcs[1].State != "suspended" {
		t.Errorf("Expected VDC 2 to be suspended, got %s", vdcs[1].State)
	}
}

func TestParseNXOSFabricPath(t *testing.T) {
	topo, err := ParseNXOSFabricPath(sampleNXOSFabricPath)
	if err != nil {
		t.Fatalf("ParseNXOSFabricPath failed: %v", err)
	}

	if topo.Domain != "default" {
		t.Errorf("Expected domain default, got %s", topo.Domain)
	}

	if topo.Tag != 1 {
		t.Errorf("Expected tag 1, got %d", topo.Tag)
	}

	if topo.SystemID != "0016.4748.ac4e" {
		t.Errorf("Expected system ID 0016.4748.ac4e, got %s", topo.SystemID)
	}

	if topo.LocalSwID != 1 {
		t.Errorf("Expected local switch ID 1, got %d", topo.LocalSwID)
	}

	if len(topo.Adjacencies) != 3 {
		t.Errorf("Expected 3 adjacencies, got %d", len(topo.Adjacencies))
	}

	// Test local adjacency
	local := topo.Adjacencies[0]
	if !local.IsLocal {
		t.Error("Expected first adjacency to be local")
	}
	if local.SwitchID != 1 {
		t.Errorf("Expected switch ID 1, got %d", local.SwitchID)
	}

	// Test remote adjacency
	remote := topo.Adjacencies[1]
	if remote.IsLocal {
		t.Error("Expected second adjacency to not be local")
	}
	if remote.Name != "Spine-01" {
		t.Errorf("Expected name Spine-01, got %s", remote.Name)
	}
	if remote.Port != "Eth1/1" {
		t.Errorf("Expected port Eth1/1, got %s", remote.Port)
	}
}

// ===== Edge Case Tests =====

func TestParseNXOSMACTable_EmptyOutput(t *testing.T) {
	entries, err := ParseNXOSMACTable("")
	if err != nil {
		t.Fatalf("ParseNXOSMACTable failed on empty output: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for empty output, got %d", len(entries))
	}
}

func TestParseNXOSVPC_NotConfigured(t *testing.T) {
	output := "vPC is not configured"
	vpc, err := ParseNXOSVPC(output)
	if err != nil {
		t.Fatalf("ParseNXOSVPC failed: %v", err)
	}
	// Should return empty/default VPC status
	if vpc.DomainID != 0 {
		t.Errorf("Expected domain ID 0 for unconfigured VPC, got %d", vpc.DomainID)
	}
}

// ===========================
// IP Route Tests (NEW)
// ===========================

const sampleNXOSIPRoute = `IP Route Table for VRF "default"
'*' denotes best ucast next-hop
'**' denotes best mcast next-hop
'[x/y]' denotes [preference/metric]
'%<string>' in via output denotes VRF <string>

0.0.0.0/0, ubest/mbest: 1/0
    *via 10.1.1.254, Eth1/1, [1/0], 00:05:23, static
10.0.10.0/24, ubest/mbest: 1/0, attached
    *via 10.0.10.1, Vlan10, [0/0], 2d17h, direct
10.0.20.0/24, ubest/mbest: 1/0, attached
    *via 10.0.20.1, Vlan20, [0/0], 2d17h, direct
10.1.1.0/30, ubest/mbest: 1/0, attached
    *via 10.1.1.1, Eth1/1, [0/0], 3d05h, direct
172.16.0.0/16, ubest/mbest: 2/0
    *via 10.1.1.2, Eth1/1, [110/20], 1d23h, ospf-1, intra
    *via 10.1.1.3, Eth1/2, [110/20], 1d23h, ospf-1, intra
192.168.100.0/24, ubest/mbest: 1/0
    *via 10.1.1.254, Eth1/1, [200/0], 12:34:56, bgp-65000, internal`

func TestParseNXOSIPRoute(t *testing.T) {
	routes, err := ParseNXOSIPRoute(sampleNXOSIPRoute)
	if err != nil {
		t.Fatalf("ParseNXOSIPRoute failed: %v", err)
	}

	if len(routes) == 0 {
		t.Fatal("Expected route entries, got none")
	}

	t.Logf("Parsed %d routes", len(routes))

	// Validate default route
	var defaultRoute *NXOSRoute
	for i := range routes {
		if routes[i].Network == "0.0.0.0" {
			defaultRoute = &routes[i]
			break
		}
	}

	if defaultRoute == nil {
		t.Fatal("Expected to find default route (0.0.0.0/0)")
	}

	if defaultRoute.Netmask != "0.0.0.0" {
		t.Errorf("Expected netmask '0.0.0.0', got '%s'", defaultRoute.Netmask)
	}

	if defaultRoute.NextHop != "10.1.1.254" {
		t.Errorf("Expected next-hop '10.1.1.254', got '%s'", defaultRoute.NextHop)
	}

	if defaultRoute.Interface != "Eth1/1" {
		t.Errorf("Expected interface 'Eth1/1', got '%s'", defaultRoute.Interface)
	}

	if defaultRoute.AdminDistance != 1 {
		t.Errorf("Expected AD=1 for static route, got %d", defaultRoute.AdminDistance)
	}

	if defaultRoute.Protocol != "static" {
		t.Errorf("Expected protocol='static', got '%s'", defaultRoute.Protocol)
	}

	t.Logf("Default route: NextHop=%s, Interface=%s, AD=%d, Protocol=%s",
		defaultRoute.NextHop, defaultRoute.Interface, defaultRoute.AdminDistance, defaultRoute.Protocol)

	// Validate connected route
	var vlan10Route *NXOSRoute
	for i := range routes {
		if routes[i].Network == "10.0.10.0" {
			vlan10Route = &routes[i]
			break
		}
	}

	if vlan10Route == nil {
		t.Fatal("Expected to find connected route (10.0.10.0/24)")
	}

	if vlan10Route.Netmask != "255.255.255.0" {
		t.Errorf("Expected netmask '255.255.255.0', got '%s'", vlan10Route.Netmask)
	}

	if vlan10Route.Interface != "Vlan10" {
		t.Errorf("Expected interface 'Vlan10', got '%s'", vlan10Route.Interface)
	}

	if vlan10Route.RouteType != "C" {
		t.Errorf("Expected RouteType='C' (connected), got '%s'", vlan10Route.RouteType)
	}

	if vlan10Route.Protocol != "direct" {
		t.Errorf("Expected protocol='direct', got '%s'", vlan10Route.Protocol)
	}

	t.Logf("Connected route: Network=%s/%s, Interface=%s, Type=%s",
		vlan10Route.Network, vlan10Route.Netmask, vlan10Route.Interface, vlan10Route.RouteType)

	// Validate OSPF route
	var ospfRoute *NXOSRoute
	for i := range routes {
		if routes[i].Network == "172.16.0.0" {
			ospfRoute = &routes[i]
			break
		}
	}

	if ospfRoute == nil {
		t.Fatal("Expected to find OSPF route (172.16.0.0/16)")
	}

	if ospfRoute.Netmask != "255.255.0.0" {
		t.Errorf("Expected netmask '255.255.0.0', got '%s'", ospfRoute.Netmask)
	}

	if ospfRoute.AdminDistance != 110 {
		t.Errorf("Expected AD=110 for OSPF route, got %d", ospfRoute.AdminDistance)
	}

	if ospfRoute.Metric != 20 {
		t.Errorf("Expected metric=20, got %d", ospfRoute.Metric)
	}

	t.Logf("OSPF route: Network=%s/%s, AD=%d, Metric=%d",
		ospfRoute.Network, ospfRoute.Netmask, ospfRoute.AdminDistance, ospfRoute.Metric)

	// Count routes by protocol
	protocolCounts := make(map[string]int)
	for _, route := range routes {
		protocolCounts[route.Protocol]++
	}

	for protocol, count := range protocolCounts {
		t.Logf("Protocol '%s': %d routes", protocol, count)
	}
}

func TestParseNXOSIPRoute_Empty(t *testing.T) {
	emptyOutput := `IP Route Table for VRF "default"`

	routes, err := ParseNXOSIPRoute(emptyOutput)
	if err != nil {
		t.Fatalf("ParseNXOSIPRoute failed on empty output: %v", err)
	}

	if len(routes) != 0 {
		t.Errorf("Expected 0 routes for empty output, got %d", len(routes))
	}
}

func TestCIDRToNetmask(t *testing.T) {
	tests := []struct {
		prefix   int
		expected string
	}{
		{8, "255.0.0.0"},
		{16, "255.255.0.0"},
		{24, "255.255.255.0"},
		{25, "255.255.255.128"},
		{26, "255.255.255.192"},
		{27, "255.255.255.224"},
		{28, "255.255.255.240"},
		{29, "255.255.255.248"},
		{30, "255.255.255.252"},
		{31, "255.255.255.254"},
		{32, "255.255.255.255"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("CIDR_%d", tt.prefix), func(t *testing.T) {
			result := cidrToNetmask(tt.prefix)
			if result != tt.expected {
				t.Errorf("CIDR /%d: expected '%s', got '%s'", tt.prefix, tt.expected, result)
			}
		})
	}
}

// ===========================
// HSRP Tests (NEW)
// ===========================

const sampleNXOSHSRP = `Ethernet1/1 - Group 10 (HSRP-V2) (IPv4)
  Local state is Active, priority 110 (Cfged 110), may preempt
  Forwarding threshold(for vPC), lower: 1 upper: 110
  Hellotime 3 sec, holdtime 10 sec
  Next hello sent in 0.704000 sec(s)
  Virtual IP address is 10.0.10.1 (Cfged)
  Active router is local
  Standby router is 10.0.10.3, priority 100 expires in 8.416000 sec
  Virtual mac address is 0000.0c9f.f00a (Default MAC)
  Authentication text "cisco"
  2 state changes, last state change 2d17h

Vlan20 - Group 20 (HSRP-V2) (IPv4)
  Local state is Standby, priority 100 (Cfged 100), may preempt
  Forwarding threshold(for vPC), lower: 1 upper: 100
  Hellotime 3 sec, holdtime 10 sec
  Next hello sent in 1.856000 sec(s)
  Virtual IP address is 10.0.20.1 (Cfged)
  Active router is 10.0.20.2, priority 110 expires in 9.216000 sec
  Standby router is local
  Virtual mac address is 0000.0c9f.f014 (Default MAC)
  1 state changes, last state change 3d05h

Vlan30 - Group 30 (HSRP-V2) (IPv4)
  Local state is Init, priority 100 (Cfged 100)
  Forwarding threshold(for vPC), lower: 1 upper: 100
  Hellotime 3 sec, holdtime 10 sec
  Virtual IP address is 10.0.30.1 (Cfged)
  Active router is unknown
  Standby router is unknown
  Virtual mac address is 0000.0c9f.f01e (Default MAC)
  0 state changes, last state change never`

func TestParseNXOSHSRP(t *testing.T) {
	entries, err := ParseNXOSHSRP(sampleNXOSHSRP)
	if err != nil {
		t.Fatalf("ParseNXOSHSRP failed: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("Expected HSRP entries, got none")
	}

	t.Logf("Parsed %d HSRP entries", len(entries))

	// Validate Ethernet1/1 - Group 10 (Active)
	var eth1Group10 *NXOSHSRP
	for i := range entries {
		if entries[i].Interface == "Ethernet1/1" && entries[i].Group == 10 {
			eth1Group10 = &entries[i]
			break
		}
	}

	if eth1Group10 == nil {
		t.Fatal("Expected to find Ethernet1/1 Group 10")
	}

	if eth1Group10.State != "Active" {
		t.Errorf("Expected State='Active', got '%s'", eth1Group10.State)
	}

	if eth1Group10.Priority != 110 {
		t.Errorf("Expected Priority=110, got %d", eth1Group10.Priority)
	}

	if !eth1Group10.Preempt {
		t.Error("Expected Preempt=true")
	}

	if eth1Group10.VirtualIP != "10.0.10.1" {
		t.Errorf("Expected VirtualIP='10.0.10.1', got '%s'", eth1Group10.VirtualIP)
	}

	if eth1Group10.ActiveRouter != "local" {
		t.Errorf("Expected ActiveRouter='local', got '%s'", eth1Group10.ActiveRouter)
	}

	if eth1Group10.StandbyRouter != "10.0.10.3" {
		t.Errorf("Expected StandbyRouter='10.0.10.3', got '%s'", eth1Group10.StandbyRouter)
	}

	if eth1Group10.VirtualMAC != "0000.0c9f.f00a" {
		t.Errorf("Expected VirtualMAC='0000.0c9f.f00a', got '%s'", eth1Group10.VirtualMAC)
	}

	if eth1Group10.HelloTime != 3 {
		t.Errorf("Expected HelloTime=3, got %d", eth1Group10.HelloTime)
	}

	if eth1Group10.HoldTime != 10 {
		t.Errorf("Expected HoldTime=10, got %d", eth1Group10.HoldTime)
	}

	if eth1Group10.StateChanges != 2 {
		t.Errorf("Expected StateChanges=2, got %d", eth1Group10.StateChanges)
	}

	if eth1Group10.LastStateChange != "2d17h" {
		t.Errorf("Expected LastStateChange='2d17h', got '%s'", eth1Group10.LastStateChange)
	}

	t.Logf("Eth1/1 Group 10: State=%s, Priority=%d, VirtualIP=%s, ActiveRouter=%s",
		eth1Group10.State, eth1Group10.Priority, eth1Group10.VirtualIP, eth1Group10.ActiveRouter)

	// Validate Vlan20 - Group 20 (Standby)
	var vlan20Group20 *NXOSHSRP
	for i := range entries {
		if entries[i].Interface == "Vlan20" && entries[i].Group == 20 {
			vlan20Group20 = &entries[i]
			break
		}
	}

	if vlan20Group20 == nil {
		t.Fatal("Expected to find Vlan20 Group 20")
	}

	if vlan20Group20.State != "Standby" {
		t.Errorf("Expected State='Standby', got '%s'", vlan20Group20.State)
	}

	if vlan20Group20.Priority != 100 {
		t.Errorf("Expected Priority=100, got %d", vlan20Group20.Priority)
	}

	if vlan20Group20.ActiveRouter != "10.0.20.2" {
		t.Errorf("Expected ActiveRouter='10.0.20.2', got '%s'", vlan20Group20.ActiveRouter)
	}

	if vlan20Group20.StandbyRouter != "local" {
		t.Errorf("Expected StandbyRouter='local', got '%s'", vlan20Group20.StandbyRouter)
	}

	t.Logf("Vlan20 Group 20: State=%s, Priority=%d, ActiveRouter=%s",
		vlan20Group20.State, vlan20Group20.Priority, vlan20Group20.ActiveRouter)

	// Validate Vlan30 - Group 30 (Init state)
	var vlan30Group30 *NXOSHSRP
	for i := range entries {
		if entries[i].Interface == "Vlan30" && entries[i].Group == 30 {
			vlan30Group30 = &entries[i]
			break
		}
	}

	if vlan30Group30 == nil {
		t.Fatal("Expected to find Vlan30 Group 30")
	}

	if vlan30Group30.State != "Init" {
		t.Errorf("Expected State='Init', got '%s'", vlan30Group30.State)
	}

	if vlan30Group30.Preempt {
		t.Error("Expected Preempt=false (not configured)")
	}

	if vlan30Group30.ActiveRouter != "unknown" {
		t.Errorf("Expected ActiveRouter='unknown', got '%s'", vlan30Group30.ActiveRouter)
	}

	t.Logf("Vlan30 Group 30: State=%s, Priority=%d, ActiveRouter=%s",
		vlan30Group30.State, vlan30Group30.Priority, vlan30Group30.ActiveRouter)

	// Count by state
	stateCounts := make(map[string]int)
	for _, entry := range entries {
		stateCounts[entry.State]++
	}

	for state, count := range stateCounts {
		t.Logf("State '%s': %d groups", state, count)
	}
}

func TestParseNXOSHSRP_Empty(t *testing.T) {
	emptyOutput := ``

	entries, err := ParseNXOSHSRP(emptyOutput)
	if err != nil {
		t.Fatalf("ParseNXOSHSRP failed on empty output: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 HSRP entries for empty output, got %d", len(entries))
	}
}

func TestParseNXOSHSRP_NoHSRP(t *testing.T) {
	noHSRPOutput := `No HSRP groups configured`

	entries, err := ParseNXOSHSRP(noHSRPOutput)
	if err != nil {
		t.Fatalf("ParseNXOSHSRP failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 HSRP entries when not configured, got %d", len(entries))
	}
}

// ===========================
// L2 Security Control Parser Tests (NXOS)
// ===========================

func TestParseNXOSDHCPSnoopingBinding(t *testing.T) {
	sampleOutput := `MacAddress          IpAddress        Lease(sec)  Type           VLAN  Interface
------------------  ---------------  ----------  -------------  ----  --------------------
0050.56bb.0001      10.0.10.2        86395       dhcp-snooping  10    Ethernet1/1
0050.56bb.0002      10.0.10.3        86395       dhcp-snooping  10    Ethernet1/2
0050.56bb.0003      10.0.20.5        3600        dhcp-snooping  20    Ethernet1/5`

	bindings := ParseNXOSDHCPSnoopingBinding(sampleOutput)

	if len(bindings) == 0 {
		t.Fatal("Expected DHCP snooping bindings, got none")
	}

	t.Logf("Parsed %d DHCP snooping bindings (NXOS)", len(bindings))

	// Validate first binding
	if len(bindings) > 0 {
		first := bindings[0]
		t.Logf("First binding: MAC=%s, IP=%s, VLAN=%d, Interface=%s",
			first.MACAddress, first.IPAddress, first.VLAN, first.Interface)

		if first.VLAN != 10 {
			t.Errorf("Expected VLAN 10, got %d", first.VLAN)
		}
		if first.Type != "dhcp-snooping" {
			t.Errorf("Expected type 'dhcp-snooping', got '%s'", first.Type)
		}
	}
}

func TestParseNXOSARPInspectionStatistics(t *testing.T) {
	sampleOutput := ` Vlan      Forwarded        Dropped     DHCP Drops      ACL Drops
 ----      ---------        -------     ----------      ---------
   10           2345            67            40              27
   20           6789            15            10               5`

	stats := ParseNXOSARPInspectionStatistics(sampleOutput)

	if stats == nil {
		t.Fatal("Expected ARP inspection statistics, got nil")
	}

	t.Logf("NXOS DAI Stats: Forwarded=%d, Dropped=%d, DHCP Drops=%d, ACL Drops=%d",
		stats.PacketsForwarded, stats.PacketsDropped, stats.DHCPSnoopingDenied, stats.ACLDenied)

	if len(stats.VLANConfig) != 2 {
		t.Errorf("Expected 2 VLANs configured, got %d", len(stats.VLANConfig))
	}
}

func TestParseNXOSAuthenticationSessions(t *testing.T) {
	sampleOutput := `Interface      MAC Address     IP Address      Status         Method  Domain
-------------- --------------- --------------- -------------- ------- -------
Ethernet1/1    0050.56bb.0001  10.0.10.2       Authorized     dot1x   DATA
Ethernet1/2    0050.56bb.0002  10.0.20.3       Authorized     mab     DATA
Ethernet1/3    0050.56bb.0003  10.0.30.5       Unauthorized   dot1x   DATA`

	sessions := ParseNXOSAuthenticationSessions(sampleOutput)

	if len(sessions) == 0 {
		t.Fatal("Expected authentication sessions, got none")
	}

	t.Logf("Parsed %d authentication sessions (NXOS)", len(sessions))

	// Validate first session
	if len(sessions) > 0 {
		first := sessions[0]
		t.Logf("First session: Interface=%s, MAC=%s, Status=%s, Method=%s",
			first.Interface, first.MACAddress, first.Status, first.Method)

		if first.Method != "dot1x" {
			t.Errorf("Expected method 'dot1x', got '%s'", first.Method)
		}
		if first.Status != "Authorized" {
			t.Errorf("Expected status 'Authorized', got '%s'", first.Status)
		}
	}

	// Count by method
	methodCounts := make(map[string]int)
	for _, session := range sessions {
		methodCounts[session.Method]++
	}
	t.Logf("Sessions by method: %v", methodCounts)
}

func TestParseNXOSPortSecurity(t *testing.T) {
	sampleOutput := `Port        MaxSecAddr  CurrentAddr  SecurityViolation  Security Action
----------- ----------- ------------ ------------------ ----------------
Ethernet1/1      1           1               0              Shutdown
Ethernet1/2      2           2               3              Restrict
Ethernet1/3      5           3               0              Protect`

	statuses := ParseNXOSPortSecurity(sampleOutput)

	if len(statuses) == 0 {
		t.Fatal("Expected port security statuses, got none")
	}

	t.Logf("Parsed %d port security entries (NXOS)", len(statuses))

	// Validate first entry
	if len(statuses) > 0 {
		first := statuses[0]
		t.Logf("First entry: Interface=%s, MaxMAC=%d, CurrentMAC=%d, Violations=%d, Mode=%s",
			first.Interface, first.MaxMAC, first.CurrentMAC, first.ViolationCount, first.ViolationMode)

		if first.MaxMAC != 1 {
			t.Errorf("Expected MaxMAC=1, got %d", first.MaxMAC)
		}
		if first.ViolationCount != 0 {
			t.Errorf("Expected ViolationCount=0, got %d", first.ViolationCount)
		}
	}

	// Count violations
	totalViolations := 0
	for _, status := range statuses {
		totalViolations += status.ViolationCount
	}
	t.Logf("Total violations: %d", totalViolations)
}

func TestParseNXOSIPVerifySource(t *testing.T) {
	sampleOutput := `Interface      Filter-type  Filter-mode  IP-address       Mac-address        Vlan
-------------- -----------  -----------  ---------------  -----------------  ----
Ethernet1/1    ip-mac       active       10.0.10.2        0050.56bb.0001     10
Ethernet1/2    ip           active       10.0.20.3        -                  20`

	bindings := ParseNXOSIPVerifySource(sampleOutput)

	if len(bindings) == 0 {
		t.Fatal("Expected IP Source Guard bindings, got none")
	}

	t.Logf("Parsed %d IPSG bindings (NXOS)", len(bindings))

	// Validate first binding
	if len(bindings) > 0 {
		first := bindings[0]
		t.Logf("First binding: Interface=%s, IP=%s, MAC=%s, VLAN=%d",
			first.Interface, first.IPAddress, first.MACAddress, first.VLAN)

		if first.Interface != "Ethernet1/1" {
			t.Errorf("Expected interface 'Ethernet1/1', got '%s'", first.Interface)
		}
		if first.VLAN != 10 {
			t.Errorf("Expected VLAN 10, got %d", first.VLAN)
		}
	}
}

func TestParseNXOSPrivateVLAN(t *testing.T) {
	sampleOutput := `Primary  Secondary  Type        Ports
-------  ---------  ----------  ------------------
100      101        isolated    Eth1/1-2
100      102        community   Eth1/3, Eth1/4
200      201        isolated    Eth1/10`

	pvlans := ParseNXOSPrivateVLAN(sampleOutput)

	if len(pvlans) == 0 {
		t.Fatal("Expected private VLAN entries, got none")
	}

	t.Logf("Parsed %d private VLAN mappings (NXOS)", len(pvlans))

	// Validate first entry
	if len(pvlans) > 0 {
		first := pvlans[0]
		t.Logf("First mapping: Primary=%d, Secondary=%d, Type=%s, Ports=%v",
			first.PrimaryVLAN, first.SecondaryVLAN, first.SecondaryType, first.Ports)

		if first.PrimaryVLAN != 100 {
			t.Errorf("Expected primary VLAN 100, got %d", first.PrimaryVLAN)
		}
		if first.SecondaryVLAN != 101 {
			t.Errorf("Expected secondary VLAN 101, got %d", first.SecondaryVLAN)
		}
		if first.SecondaryType != "isolated" {
			t.Errorf("Expected type 'isolated', got '%s'", first.SecondaryType)
		}
	}

	// Count by type
	typeCounts := make(map[string]int)
	for _, pvlan := range pvlans {
		typeCounts[pvlan.SecondaryType]++
	}
	t.Logf("Mappings by type: %v", typeCounts)
}

func TestParseNXOSDHCPSnoopingBinding_Empty(t *testing.T) {
	emptyOutput := `MacAddress          IpAddress        Lease(sec)  Type           VLAN  Interface
------------------  ---------------  ----------  -------------  ----  --------------------`

	bindings := ParseNXOSDHCPSnoopingBinding(emptyOutput)
	if len(bindings) != 0 {
		t.Errorf("Expected 0 bindings, got %d", len(bindings))
	}
}

func TestParseNXOSAuthenticationSessions_Empty(t *testing.T) {
	emptyOutput := `Interface      MAC Address     IP Address      Status         Method  Domain
-------------- --------------- --------------- -------------- ------- -------`

	sessions := ParseNXOSAuthenticationSessions(emptyOutput)
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(sessions))
	}
}

func TestParseNXOSPortSecurity_Empty(t *testing.T) {
	emptyOutput := `Port        MaxSecAddr  CurrentAddr  SecurityViolation  Security Action
----------- ----------- ------------ ------------------ ----------------`

	statuses := ParseNXOSPortSecurity(emptyOutput)
	if len(statuses) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(statuses))
	}
}
