# L2 Models Specification

**Purpose**: Define all L2 data structures for Cisco IOS/NXOS and generic topology models  
**Status**: Phase 1.0.1 - Foundation Document  
**Date**: 2025-10-26  
**Based On**: `l2_project.md` §5 (Data Contract & Field Specification)

---

## Overview

This document defines the Go struct models required for Layer 2 network topology and security control modeling. All models support the L2 decision engine requirements for producing `Allowed | Conditional | Blocked` outcomes with evidence-based reasoning.

---

## 1. Cisco-Specific L2 Models

### 1.1 CiscoL2Config

Top-level container for Cisco L2 configuration.

```go
// CiscoL2Config represents the Layer 2 configuration for a Cisco device
type CiscoL2Config struct {
	// Device Identity
	DeviceID             string    `json:"device_id" bson:"device_id"`
	Hostname             string    `json:"hostname" bson:"hostname"`
	Vendor               string    `json:"vendor" bson:"vendor"`                               // "cisco"
	OSName               string    `json:"os_name" bson:"os_name"`                             // "IOS", "NXOS", "IOS-XE"
	OSVersion            string    `json:"os_version" bson:"os_version"`                       // e.g., "15.2(4)E7"
	ConfigHash           string    `json:"config_hash" bson:"config_hash"`                     // SHA-256 of full config
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
	
	// Evidence
	SourceBlock          string    `json:"source_block" bson:"source_block"`                   // e.g., "hostname SW-CORE-01"
	SourceSnippetHash    string    `json:"source_snippet_hash" bson:"source_snippet_hash"`     // SHA-256 of device header stanza
	
	// L2 Components
	VLANs                []CiscoL2VLAN           `json:"vlans" bson:"vlans"`
	Interfaces           []CiscoL2Interface      `json:"interfaces" bson:"interfaces"`
	PortChannels         []CiscoPortChannel      `json:"port_channels" bson:"port_channels"`
	SVIs                 []CiscoSVI              `json:"svis" bson:"svis"`
	GlobalL2Controls     CiscoGlobalL2Controls   `json:"global_l2_controls" bson:"global_l2_controls"`
	
	// Operational Data (from show commands)
	FDBEntries           []CiscoFDBEntry         `json:"fdb_entries,omitempty" bson:"fdb_entries,omitempty"`
	DHCPSnoopingBindings []DHCPSnoopingBinding   `json:"dhcp_snooping_bindings,omitempty" bson:"dhcp_snooping_bindings,omitempty"`
	AuthSessions         []AuthenticationSession `json:"auth_sessions,omitempty" bson:"auth_sessions,omitempty"`
	PortSecurityStatus   []PortSecurityStatus    `json:"port_security_status,omitempty" bson:"port_security_status,omitempty"`
	PVLANMappings        []PVLANMapping          `json:"pvlan_mappings,omitempty" bson:"pvlan_mappings,omitempty"`
}
```

---

### 1.2 CiscoL2VLAN

VLAN definition with VACL reference.

```go
// CiscoL2VLAN represents a VLAN on a Cisco switch
type CiscoL2VLAN struct {
	// VLAN Identity (P0)
	VLANId               int       `json:"vlan_id" bson:"vlan_id"`                     // VLAN numeric ID
	VLANName             string    `json:"vlan_name" bson:"vlan_name"`                 // Friendly name
	State                string    `json:"state" bson:"state"`                         // "active" | "shutdown"
	
	// VACL (P1 - Phase 1.5)
	VACLName             string    `json:"vacl_name,omitempty" bson:"vacl_name,omitempty"`     // VLAN ACL reference (name only)
	
	// Evidence (P0)
	SourceBlock          string    `json:"source_block" bson:"source_block"`                   // e.g., "vlan 2005"
	SourceSnippetHash    string    `json:"source_snippet_hash" bson:"source_snippet_hash"`     // SHA-256 of VLAN stanza
	
	// Metadata
	DeviceID             string    `json:"device_id" bson:"device_id"`
	ParsedAt             time.Time `json:"parsed_at" bson:"parsed_at"`
}
```

---

### 1.3 CiscoL2Interface

**CRITICAL MODEL** - Interface with 15+ security control fields per `l2_project.md` §5.3.

```go
// CiscoL2Interface represents a switch interface with L2 security controls
type CiscoL2Interface struct {
	// Interface Identity (P0)
	IfName               string    `json:"if_name" bson:"if_name"`                     // e.g., "GigabitEthernet1/0/12"
	AdminState           string    `json:"admin_state" bson:"admin_state"`             // "up" | "down"
	OperState            string    `json:"oper_state,omitempty" bson:"oper_state,omitempty"`     // "up" | "down" (operational)
	Description          string    `json:"description,omitempty" bson:"description,omitempty"`
	
	// Switchport Mode (P0)
	Mode                 string    `json:"mode" bson:"mode"`                           // "access" | "trunk" | "routed"
	AccessVLAN           int       `json:"access_vlan,omitempty" bson:"access_vlan,omitempty"`   // Access VLAN (if mode=access)
	TrunkAllowedVLANs    []int     `json:"trunk_allowed_vlans,omitempty" bson:"trunk_allowed_vlans,omitempty"`   // Allowed VLANs (if mode=trunk)
	TrunkNativeVLAN      int       `json:"trunk_native_vlan,omitempty" bson:"trunk_native_vlan,omitempty"`       // Native VLAN (if mode=trunk)
	
	// Port-Channel Membership (P0)
	PortChannelID        int       `json:"port_channel_id,omitempty" bson:"port_channel_id,omitempty"`   // LAG ID (if member)
	
	// L2 Security Controls - Port-Level (P0)
	DHCPSnoopingTrust    bool      `json:"dhcp_snooping_trust" bson:"dhcp_snooping_trust"`               // "ip dhcp snooping trust"
	ARPInspectionTrust   bool      `json:"arp_inspection_trust" bson:"arp_inspection_trust"`             // "ip arp inspection trust"
	IPSourceGuard        bool      `json:"ip_source_guard" bson:"ip_source_guard"`                       // "ip verify source"
	Dot1XRequired        bool      `json:"dot1x_required" bson:"dot1x_required"`                         // "authentication port-control auto"
	MABEnabled           bool      `json:"mab_enabled" bson:"mab_enabled"`                               // "mab" or "authentication fallback"
	UnauthVLAN           int       `json:"unauth_vlan,omitempty" bson:"unauth_vlan,omitempty"`           // Guest/critical VLAN
	
	// Port ACLs (P1 - names only in Phase 1)
	PACLInName           string    `json:"pacl_in_name,omitempty" bson:"pacl_in_name,omitempty"`         // Ingress Port ACL
	PACLOutName          string    `json:"pacl_out_name,omitempty" bson:"pacl_out_name,omitempty"`       // Egress Port ACL
	
	// Port Security (P1)
	PortSecurityEnabled  bool      `json:"port_security_enabled" bson:"port_security_enabled"`
	PortSecurityMaxMAC   int       `json:"port_security_max_mac,omitempty" bson:"port_security_max_mac,omitempty"`
	
	// PVLAN (P1)
	PVLANRole            string    `json:"pvlan_role,omitempty" bson:"pvlan_role,omitempty"`             // "promiscuous" | "host" | "isolated" | "community"
	PVLANPrimaryVLAN     int       `json:"pvlan_primary_vlan,omitempty" bson:"pvlan_primary_vlan,omitempty"`
	PVLANSecondaryVLAN   int       `json:"pvlan_secondary_vlan,omitempty" bson:"pvlan_secondary_vlan,omitempty"`
	
	// STP Security (P1)
	SpanningTreePortfast bool      `json:"spanning_tree_portfast,omitempty" bson:"spanning_tree_portfast,omitempty"`
	SpanningTreeBPDUGuard bool     `json:"spanning_tree_bpdu_guard,omitempty" bson:"spanning_tree_bpdu_guard,omitempty"`
	SpanningTreeRootGuard bool     `json:"spanning_tree_root_guard,omitempty" bson:"spanning_tree_root_guard,omitempty"`
	
	// Evidence (P0)
	SourceBlock          string    `json:"source_block" bson:"source_block"`                   // e.g., "interface GigabitEthernet1/0/12"
	SourceSnippetHash    string    `json:"source_snippet_hash" bson:"source_snippet_hash"`     // SHA-256 of interface stanza
	
	// Metadata
	DeviceID             string    `json:"device_id" bson:"device_id"`
	ParsedAt             time.Time `json:"parsed_at" bson:"parsed_at"`
}
```

---

### 1.4 CiscoPortChannel

Port-channel (LAG) with VLAN propagation settings.

```go
// CiscoPortChannel represents a Link Aggregation Group (LAG)
type CiscoPortChannel struct {
	// Port-Channel Identity (P0)
	PortChannelID        int       `json:"port_channel_id" bson:"port_channel_id"`             // LAG ID
	POName               string    `json:"po_name" bson:"po_name"`                             // e.g., "Port-channel1"
	MemberIfList         []string  `json:"member_if_list" bson:"member_if_list"`               // Physical member interfaces
	
	// L2 Mode (P0)
	POMode               string    `json:"po_mode" bson:"po_mode"`                             // "access" | "trunk"
	POAccessVLAN         int       `json:"po_access_vlan,omitempty" bson:"po_access_vlan,omitempty"`
	POTrunkAllowedVLANs  []int     `json:"po_trunk_allowed_vlans,omitempty" bson:"po_trunk_allowed_vlans,omitempty"`
	POTrunkNativeVLAN    int       `json:"po_trunk_native_vlan,omitempty" bson:"po_trunk_native_vlan,omitempty"`
	
	// Protocol
	Protocol             string    `json:"protocol,omitempty" bson:"protocol,omitempty"`       // "LACP" | "PAgP" | "static"
	
	// Evidence (P0)
	SourceBlock          string    `json:"source_block" bson:"source_block"`                   // e.g., "interface Port-channel1"
	SourceSnippetHash    string    `json:"source_snippet_hash" bson:"source_snippet_hash"`
	
	// Metadata
	DeviceID             string    `json:"device_id" bson:"device_id"`
	ParsedAt             time.Time `json:"parsed_at" bson:"parsed_at"`
}
```

---

### 1.5 CiscoGlobalL2Controls

Device-wide L2 security control toggles.

```go
// CiscoGlobalL2Controls represents global L2 security settings
type CiscoGlobalL2Controls struct {
	// DHCP Snooping (P0)
	SnoopingEnabled      bool      `json:"snooping_enabled" bson:"snooping_enabled"`           // "ip dhcp snooping"
	SnoopingVLANs        []int     `json:"snooping_vlans" bson:"snooping_vlans"`               // "ip dhcp snooping vlan <list>"
	
	// Dynamic ARP Inspection (P0)
	DAIEnabledVLANs      []int     `json:"dai_enabled_vlans" bson:"dai_enabled_vlans"`         // "ip arp inspection vlan <list>"
	DAIACLNameByVLAN     map[int]string `json:"dai_acl_name_by_vlan,omitempty" bson:"dai_acl_name_by_vlan,omitempty"`   // P1
	
	// IP Source Guard (P0)
	IPSGGlobal           bool      `json:"ipsg_global" bson:"ipsg_global"`                     // "ip device tracking"
	
	// 802.1X (P0)
	Dot1XSystemAuthControl bool    `json:"dot1x_system_auth_control" bson:"dot1x_system_auth_control"`   // "dot1x system-auth-control"
	
	// MAB (P1)
	MABGlobal            bool      `json:"mab_global,omitempty" bson:"mab_global,omitempty"`
	
	// Evidence (P0)
	SourceBlock          string    `json:"source_block" bson:"source_block"`                   // "Global L2 controls block"
	SourceSnippetHash    string    `json:"source_snippet_hash" bson:"source_snippet_hash"`
	
	// Metadata
	DeviceID             string    `json:"device_id" bson:"device_id"`
	ParsedAt             time.Time `json:"parsed_at" bson:"parsed_at"`
}
```

---

### 1.6 CiscoSVI

Switch Virtual Interface (L3 handoff marker).

```go
// CiscoSVI represents an SVI (interface VlanX) - L2 to L3 handoff
type CiscoSVI struct {
	// SVI Identity (P0)
	SVIVLANId            int       `json:"svi_vlan_id" bson:"svi_vlan_id"`                     // VLAN ID
	SVIIfName            string    `json:"svi_if_name" bson:"svi_if_name"`                     // e.g., "Vlan120"
	SVIIP                string    `json:"svi_ip,omitempty" bson:"svi_ip,omitempty"`           // IP address/subnet
	AdminState           string    `json:"admin_state" bson:"admin_state"`                     // "up" | "down"
	Description          string    `json:"description,omitempty" bson:"description,omitempty"`
	
	// Helpers
	VRFName              string    `json:"vrf_name,omitempty" bson:"vrf_name,omitempty"`
	HSRPGroup            int       `json:"hsrp_group,omitempty" bson:"hsrp_group,omitempty"`
	
	// Evidence (P0)
	SourceBlock          string    `json:"source_block" bson:"source_block"`                   // e.g., "interface Vlan120"
	SourceSnippetHash    string    `json:"source_snippet_hash" bson:"source_snippet_hash"`
	
	// Metadata
	DeviceID             string    `json:"device_id" bson:"device_id"`
	ParsedAt             time.Time `json:"parsed_at" bson:"parsed_at"`
}
```

---

### 1.7 CiscoFDBEntry

MAC address table entry (from `show mac address-table dynamic`).

```go
// CiscoFDBEntry represents a MAC address table entry
type CiscoFDBEntry struct {
	// MAC Identity (P0)
	MAC                  string    `json:"mac" bson:"mac"`                                     // MAC address (normalized)
	VLANId               int       `json:"vlan_id" bson:"vlan_id"`
	LastSeenPort         string    `json:"last_seen_port" bson:"last_seen_port"`               // Interface name
	
	// Freshness (P0)
	LastSeenTs           time.Time `json:"last_seen_ts" bson:"last_seen_ts"`
	FreshnessStatus      string    `json:"freshness_status" bson:"freshness_status"`           // "fresh" | "stale" | "old"
	
	// Confidence Scoring (P0)
	EvidenceSource       string    `json:"evidence_source" bson:"evidence_source"`             // "fdb" | "snmp" | "dhcp_snooping" | "api"
	Confidence           float64   `json:"confidence" bson:"confidence"`                       // 0.0 - 1.0
	
	// Entry Type
	Type                 string    `json:"type,omitempty" bson:"type,omitempty"`               // "dynamic" | "static" | "secure"
	
	// Metadata
	DeviceID             string    `json:"device_id" bson:"device_id"`
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
}
```

---

### 1.8 DHCPSnoopingBinding

DHCP snooping binding entry (from `show ip dhcp snooping binding`).

```go
// DHCPSnoopingBinding represents a DHCP snooping binding entry
type DHCPSnoopingBinding struct {
	// Binding Identity
	MAC                  string    `json:"mac" bson:"mac"`
	IPAddress            string    `json:"ip_address" bson:"ip_address"`
	VLANId               int       `json:"vlan_id" bson:"vlan_id"`
	Interface            string    `json:"interface" bson:"interface"`
	
	// Lease Info
	LeaseSeconds         int       `json:"lease_seconds,omitempty" bson:"lease_seconds,omitempty"`
	Type                 string    `json:"type" bson:"type"`                                   // "dhcp-snooping" | "static"
	
	// Metadata
	DeviceID             string    `json:"device_id" bson:"device_id"`
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
}
```

---

### 1.9 AuthenticationSession

802.1X / MAB authentication session (from `show authentication sessions`).

```go
// AuthenticationSession represents an 802.1X or MAB session
type AuthenticationSession struct {
	// Session Identity
	Interface            string    `json:"interface" bson:"interface"`
	MACAddress           string    `json:"mac_address" bson:"mac_address"`
	Method               string    `json:"method" bson:"method"`                               // "dot1x" | "mab" | "webauth"
	Domain               string    `json:"domain,omitempty" bson:"domain,omitempty"`           // "DATA" | "VOICE"
	Status               string    `json:"status" bson:"status"`                               // "Authz Success" | "Authz Failed" | "Running"
	
	// VLAN Assignment
	VLANId               int       `json:"vlan_id,omitempty" bson:"vlan_id,omitempty"`
	
	// Identity (if available)
	Username             string    `json:"username,omitempty" bson:"username,omitempty"`
	
	// Session Timing
	SessionTime          string    `json:"session_time,omitempty" bson:"session_time,omitempty"`
	
	// Metadata
	DeviceID             string    `json:"device_id" bson:"device_id"`
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
}
```

---

### 1.10 PortSecurityStatus

Port security status (from `show port-security`).

```go
// PortSecurityStatus represents port security status
type PortSecurityStatus struct {
	// Port Identity
	Interface            string    `json:"interface" bson:"interface"`
	
	// Configuration
	MaxMACCount          int       `json:"max_mac_count" bson:"max_mac_count"`
	CurrentMACCount      int       `json:"current_mac_count" bson:"current_mac_count"`
	SecurityViolation    string    `json:"security_violation" bson:"security_violation"`       // "Shutdown" | "Restrict" | "Protect"
	
	// Status
	Status               string    `json:"status" bson:"status"`                               // "Secure-up" | "Secure-down" | "Secure-shutdown"
	ViolationCount       int       `json:"violation_count" bson:"violation_count"`
	LastSource           string    `json:"last_source,omitempty" bson:"last_source,omitempty"` // Last violating MAC
	
	// Metadata
	DeviceID             string    `json:"device_id" bson:"device_id"`
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
}
```

---

### 1.11 PVLANMapping

Private VLAN mapping (from `show vlan private-vlan`).

```go
// PVLANMapping represents a Private VLAN mapping
type PVLANMapping struct {
	// PVLAN Identity
	PrimaryVLAN          int       `json:"primary_vlan" bson:"primary_vlan"`
	SecondaryVLAN        int       `json:"secondary_vlan" bson:"secondary_vlan"`
	Type                 string    `json:"type" bson:"type"`                                   // "isolated" | "community"
	
	// Ports
	PromiscuousPorts     []string  `json:"promiscuous_ports,omitempty" bson:"promiscuous_ports,omitempty"`
	HostPorts            []string  `json:"host_ports,omitempty" bson:"host_ports,omitempty"`
	
	// Metadata
	DeviceID             string    `json:"device_id" bson:"device_id"`
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
}
```

---

## 2. Generic Topology Models

These models are vendor-agnostic and represent the normalized topology.

### 2.1 L2Switch

```go
// L2Switch represents a Layer 2 switch in the topology
type L2Switch struct {
	// Identity
	SwitchID             string    `json:"switch_id" bson:"_id"`                               // Unique ID (hostname or inventory ID)
	Hostname             string    `json:"hostname" bson:"hostname"`
	Vendor               string    `json:"vendor" bson:"vendor"`
	Model                string    `json:"model" bson:"model"`
	OSName               string    `json:"os_name" bson:"os_name"`
	OSVersion            string    `json:"os_version" bson:"os_version"`
	
	// Config Hash
	ConfigHash           string    `json:"config_hash" bson:"config_hash"`
	
	// Capabilities
	SupportsDHCPSnooping bool      `json:"supports_dhcp_snooping" bson:"supports_dhcp_snooping"`
	SupportsDAI          bool      `json:"supports_dai" bson:"supports_dai"`
	SupportsIPSG         bool      `json:"supports_ipsg" bson:"supports_ipsg"`
	Supports8021X        bool      `json:"supports_802_1x" bson:"supports_802_1x"`
	SupportsPVLAN        bool      `json:"supports_pvlan" bson:"supports_pvlan"`
	
	// Metadata
	SiteID               string    `json:"site_id,omitempty" bson:"site_id,omitempty"`
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
	UpdatedAt            time.Time `json:"updated_at" bson:"updated_at"`
}
```

---

### 2.2 L2SwitchPort

```go
// L2SwitchPort represents a switch port in the topology
type L2SwitchPort struct {
	// Identity
	PortID               string    `json:"port_id" bson:"_id"`                                 // Composite: <switch_id>:<if_name>
	SwitchID             string    `json:"switch_id" bson:"switch_id"`
	IfName               string    `json:"if_name" bson:"if_name"`
	
	// Port Configuration
	AdminState           string    `json:"admin_state" bson:"admin_state"`
	OperState            string    `json:"oper_state,omitempty" bson:"oper_state,omitempty"`
	Mode                 string    `json:"mode" bson:"mode"`                                   // "access" | "trunk" | "routed"
	AccessVLAN           int       `json:"access_vlan,omitempty" bson:"access_vlan,omitempty"`
	TrunkAllowedVLANs    []int     `json:"trunk_allowed_vlans,omitempty" bson:"trunk_allowed_vlans,omitempty"`
	TrunkNativeVLAN      int       `json:"trunk_native_vlan,omitempty" bson:"trunk_native_vlan,omitempty"`
	
	// Port-Channel
	PortChannelID        int       `json:"port_channel_id,omitempty" bson:"port_channel_id,omitempty"`
	
	// L2 Security Posture
	DHCPSnoopingTrust    bool      `json:"dhcp_snooping_trust" bson:"dhcp_snooping_trust"`
	ARPInspectionTrust   bool      `json:"arp_inspection_trust" bson:"arp_inspection_trust"`
	IPSourceGuard        bool      `json:"ip_source_guard" bson:"ip_source_guard"`
	Dot1XRequired        bool      `json:"dot1x_required" bson:"dot1x_required"`
	MABEnabled           bool      `json:"mab_enabled" bson:"mab_enabled"`
	UnauthVLAN           int       `json:"unauth_vlan,omitempty" bson:"unauth_vlan,omitempty"`
	PortSecurityEnabled  bool      `json:"port_security_enabled" bson:"port_security_enabled"`
	PVLANRole            string    `json:"pvlan_role,omitempty" bson:"pvlan_role,omitempty"`
	
	// Evidence
	SourceSnippetHash    string    `json:"source_snippet_hash" bson:"source_snippet_hash"`
	
	// Metadata
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
	UpdatedAt            time.Time `json:"updated_at" bson:"updated_at"`
}
```

---

### 2.3 L2PortChannel

```go
// L2PortChannel represents a LAG in the topology
type L2PortChannel struct {
	// Identity
	ChannelID            string    `json:"channel_id" bson:"_id"`                              // Composite: <switch_id>:Po<id>
	SwitchID             string    `json:"switch_id" bson:"switch_id"`
	PortChannelID        int       `json:"port_channel_id" bson:"port_channel_id"`
	POName               string    `json:"po_name" bson:"po_name"`
	
	// Members
	MemberPorts          []string  `json:"member_ports" bson:"member_ports"`                   // List of port_ids
	
	// Configuration
	POMode               string    `json:"po_mode" bson:"po_mode"`
	POAccessVLAN         int       `json:"po_access_vlan,omitempty" bson:"po_access_vlan,omitempty"`
	POTrunkAllowedVLANs  []int     `json:"po_trunk_allowed_vlans,omitempty" bson:"po_trunk_allowed_vlans,omitempty"`
	POTrunkNativeVLAN    int       `json:"po_trunk_native_vlan,omitempty" bson:"po_trunk_native_vlan,omitempty"`
	Protocol             string    `json:"protocol" bson:"protocol"`
	
	// Evidence
	SourceSnippetHash    string    `json:"source_snippet_hash" bson:"source_snippet_hash"`
	
	// Metadata
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
	UpdatedAt            time.Time `json:"updated_at" bson:"updated_at"`
}
```

---

### 2.4 FDBEntry (Topology)

```go
// FDBEntry represents a MAC address table entry in the topology
type FDBEntry struct {
	// Identity
	EntryID              string    `json:"entry_id" bson:"_id"`                                // Composite: <switch_id>:<vlan_id>:<mac>
	SwitchID             string    `json:"switch_id" bson:"switch_id"`
	PortID               string    `json:"port_id" bson:"port_id"`                             // <switch_id>:<if_name>
	MAC                  string    `json:"mac" bson:"mac"`
	VLANId               int       `json:"vlan_id" bson:"vlan_id"`
	
	// Freshness
	LastSeenTs           time.Time `json:"last_seen_ts" bson:"last_seen_ts"`
	FreshnessStatus      string    `json:"freshness_status" bson:"freshness_status"`           // "fresh" | "stale" | "old"
	
	// Confidence
	EvidenceSource       string    `json:"evidence_source" bson:"evidence_source"`
	Confidence           float64   `json:"confidence" bson:"confidence"`
	
	// Metadata
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
	UpdatedAt            time.Time `json:"updated_at" bson:"updated_at"`
}
```

---

### 2.5 L2GlobalControls

```go
// L2GlobalControls represents device-wide L2 security settings
type L2GlobalControls struct {
	// Identity
	ControlsID           string    `json:"controls_id" bson:"_id"`                             // switch_id
	SwitchID             string    `json:"switch_id" bson:"switch_id"`
	
	// DHCP Snooping
	SnoopingEnabled      bool      `json:"snooping_enabled" bson:"snooping_enabled"`
	SnoopingVLANs        []int     `json:"snooping_vlans" bson:"snooping_vlans"`
	
	// DAI
	DAIEnabledVLANs      []int     `json:"dai_enabled_vlans" bson:"dai_enabled_vlans"`
	
	// IPSG
	IPSGGlobal           bool      `json:"ipsg_global" bson:"ipsg_global"`
	
	// 802.1X
	Dot1XSystemAuthControl bool    `json:"dot1x_system_auth_control" bson:"dot1x_system_auth_control"`
	
	// Evidence
	SourceSnippetHash    string    `json:"source_snippet_hash" bson:"source_snippet_hash"`
	
	// Metadata
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
	UpdatedAt            time.Time `json:"updated_at" bson:"updated_at"`
}
```

---

### 2.6 L3HandoffMarker

```go
// L3HandoffMarker represents an SVI (L2 to L3 transition point)
type L3HandoffMarker struct {
	// Identity
	HandoffID            string    `json:"handoff_id" bson:"_id"`                              // Composite: <switch_id>:vlan<id>
	SwitchID             string    `json:"switch_id" bson:"switch_id"`
	VLANId               int       `json:"vlan_id" bson:"vlan_id"`
	
	// SVI Configuration
	SVIIfName            string    `json:"svi_if_name" bson:"svi_if_name"`
	SVIIP                string    `json:"svi_ip,omitempty" bson:"svi_ip,omitempty"`
	AdminState           string    `json:"admin_state" bson:"admin_state"`
	VRFName              string    `json:"vrf_name,omitempty" bson:"vrf_name,omitempty"`
	
	// Evidence
	SourceSnippetHash    string    `json:"source_snippet_hash" bson:"source_snippet_hash"`
	
	// Metadata
	CollectedAt          time.Time `json:"collected_at" bson:"collected_at"`
	UpdatedAt            time.Time `json:"updated_at" bson:"updated_at"`
}
```

---

## 3. Graph Enrichment Models

### 3.1 L2AssetPlacement

Added to `AssetNode` for graph enrichment.

```go
// L2AssetPlacement represents L2 placement information for an asset
type L2AssetPlacement struct {
	// Placement
	SwitchID             string    `json:"switch_id" bson:"switch_id"`
	PortID               string    `json:"port_id" bson:"port_id"`
	MAC                  string    `json:"mac" bson:"mac"`
	VLANId               int       `json:"vlan_id" bson:"vlan_id"`
	
	// Confidence
	Confidence           float64   `json:"confidence" bson:"confidence"`
	FreshnessStatus      string    `json:"freshness_status" bson:"freshness_status"`
	EvidenceSource       string    `json:"evidence_source" bson:"evidence_source"`
	
	// Security Posture (from port)
	DHCPSnoopingTrust    bool      `json:"dhcp_snooping_trust" bson:"dhcp_snooping_trust"`
	ARPInspectionTrust   bool      `json:"arp_inspection_trust" bson:"arp_inspection_trust"`
	IPSourceGuard        bool      `json:"ip_source_guard" bson:"ip_source_guard"`
	Dot1XRequired        bool      `json:"dot1x_required" bson:"dot1x_required"`
	MABEnabled           bool      `json:"mab_enabled" bson:"mab_enabled"`
	PortSecurityEnabled  bool      `json:"port_security_enabled" bson:"port_security_enabled"`
	
	// Auth State (if available)
	AuthStatus           string    `json:"auth_status,omitempty" bson:"auth_status,omitempty"`       // "Authz Success" | "Authz Failed" | "Unknown"
	AuthMethod           string    `json:"auth_method,omitempty" bson:"auth_method,omitempty"`       // "dot1x" | "mab" | "none"
	
	// Metadata
	LastUpdated          time.Time `json:"last_updated" bson:"last_updated"`
}
```

---

## 4. Decision Engine Models (Phase 2)

**Note**: These are stubs for Phase 2. Decision engine implementation is out of scope for multi-vendor support Phase 1.

### 4.1 L2Decision

```go
// L2Decision represents a cached L2 reachability decision (Phase 2)
type L2Decision struct {
	// Identity
	DecisionID           string    `json:"decision_id" bson:"_id"`
	SourceMAC            string    `json:"source_mac" bson:"source_mac"`
	DestMAC              string    `json:"dest_mac" bson:"dest_mac"`
	VLANId               int       `json:"vlan_id" bson:"vlan_id"`
	
	// Decision
	Outcome              string    `json:"outcome" bson:"outcome"`                             // "Allowed" | "Conditional" | "Blocked"
	ReasonCodes          []string  `json:"reason_codes" bson:"reason_codes"`
	ReasonText           string    `json:"reason_text" bson:"reason_text"`
	
	// Context
	HandoffToL3          bool      `json:"handoff_to_l3" bson:"handoff_to_l3"`
	
	// Metadata
	ComputedAt           time.Time `json:"computed_at" bson:"computed_at"`
	ExpiresAt            time.Time `json:"expires_at" bson:"expires_at"`
}
```

---

## 5. MongoDB Collections

Recommended MongoDB collection names:

```yaml
l2_switches:           L2Switch documents
l2_switch_ports:       L2SwitchPort documents
l2_port_channels:      L2PortChannel documents
l2_fdb_entries:        FDBEntry documents
l2_global_controls:    L2GlobalControls documents
l2_handoff_markers:    L3HandoffMarker documents
l2_decisions:          L2Decision documents (Phase 2)
```

---

## 6. Confidence Scoring Algorithm

Per `l2_project.md` §9:

```go
// Confidence scoring weights
const (
	ConfidenceDHCPSnooping = 1.0    // Highest confidence (authoritative binding)
	ConfidenceFDB          = 0.9    // Switch MAC table (very reliable)
	ConfidenceSNMP         = 0.85   // SNMP FDB query
	ConfidenceController   = 0.8    // Wireless controller client table
	ConfidenceClaroty      = 0.6    // OT NDR API (secondary evidence)
	ConfidenceNetFlow      = 0.5    // Flow hints (lowest)
)

// Freshness thresholds (configurable)
const (
	FreshnessThresholdFresh = 10 * time.Minute
	FreshnessThresholdStale = 60 * time.Minute
)

// GetFreshnessStatus determines freshness status
func GetFreshnessStatus(lastSeen time.Time) string {
	age := time.Since(lastSeen)
	if age <= FreshnessThresholdFresh {
		return "fresh"
	} else if age <= FreshnessThresholdStale {
		return "stale"
	}
	return "old"
}
```

---

## 7. Implementation Notes

### 7.1 Evidence Hashing

All stanzas must include:
- `source_block`: Stable identifier (e.g., "interface GigabitEthernet1/0/12")
- `source_snippet_hash`: SHA-256 of **normalized** stanza text

Normalization rules (from `l2_project.md` §6):
1. Strip comments and blank lines
2. Normalize indentation & collapse multiple spaces
3. Case-fold keywords (preserve literals)
4. Sort order-insensitive lines (e.g., VLAN lists)
5. Hash UTF-8 bytes → `sha256:<hex>`

### 7.2 VLAN List Canonicalization

Cisco VLAN ranges must be expanded and sorted:
- Input: `"10,20-22,30"`
- Parsed: `[10, 20, 21, 22, 30]`
- Keep original string in evidence if needed

### 7.3 Interface Name Canonicalization

Normalize to compact form:
- `GigabitEthernet1/0/12` → `Gi1/0/12`
- `TenGigabitEthernet1/1/1` → `Te1/1/1`
- `Port-channel10` → `Po10`

### 7.4 Timestamp Handling

All timestamps should be in UTC:
```go
time.Now().UTC()
```

---

## 8. Validation Rules

### Required Fields (P0)
- All models must have `device_id` or `switch_id`
- All models must have `source_snippet_hash` (for config-derived data)
- All models must have collection timestamps

### Constraints
- VLAN IDs: 1-4094
- Port-Channel IDs: 1-256 (vendor-dependent)
- Confidence scores: 0.0 - 1.0
- MAC addresses: Normalized to lowercase with colons (aa:bb:cc:dd:ee:ff)

---

## Status

✅ **Phase 1.0.1 Complete** - L2 models specification defined  
⏭️ **Next**: Phase 1.0.2 - Implement evidence hashing

---

**Document Version**: 1.0  
**Last Updated**: 2025-10-26

