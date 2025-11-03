package junos

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
	"github.com/mattpainter701/graphlayer/platform/platform/vendors/juniper"
)

// Juniper Junos Mappers
// Transform Juniper-specific data structures into unified platform models

// MapJuniperToL2Switch maps Juniper L2 config to unified L2Switch model
func MapJuniperToL2Switch(juniperConfig *juniper.JuniperL2Config) (*models.L2Switch, error) {
	logger := slog.Default().With(
		"vendor", "juniper",
		"platform", "junos",
		"device_id", juniperConfig.Device.DeviceID,
	)

	l2Switch := &models.L2Switch{
		DeviceID:     juniperConfig.Device.DeviceID,
		Hostname:     juniperConfig.Device.Hostname,
		Vendor:       "juniper",
		OSName:       "junos",
		OSVersion:    juniperConfig.Device.OSVersion,
		Model:        juniperConfig.Device.Model,
		ConfigHash:   juniperConfig.Device.ConfigHash,
		VLANs:        []models.L2VLAN{},
		Ports:        []models.L2SwitchPort{},
		PortChannels: []models.L2PortChannel{},
		GlobalControls: models.L2GlobalControls{},
		L3Handoffs:   []models.L3HandoffMarker{},
	}

	// Build VLAN name-to-ID mapping for interface resolution
	vlanNameToID := make(map[string]int)
	for _, vlan := range juniperConfig.VLANs {
		vlanNameToID[vlan.VLANName] = vlan.VLANID
	}

	// Map VLANs
	for _, vlan := range juniperConfig.VLANs {
		l2Switch.VLANs = append(l2Switch.VLANs, mapJuniperVLAN(vlan))
	}

	logger.Info("Mapped VLANs", "count", len(l2Switch.VLANs))

	// Map interfaces
	for _, iface := range juniperConfig.Interfaces {
		l2Switch.Ports = append(l2Switch.Ports, mapJuniperInterface(iface, vlanNameToID))
	}

	logger.Info("Mapped interfaces", "count", len(l2Switch.Ports))

	// Map Aggregated Ethernet (AE) interfaces
	for _, ae := range juniperConfig.AggregatedEthernets {
		l2Switch.PortChannels = append(l2Switch.PortChannels, mapJuniperAE(ae, vlanNameToID))
	}

	logger.Info("Mapped AE interfaces", "count", len(l2Switch.PortChannels))

	// Map global controls
	l2Switch.GlobalControls = mapJuniperGlobalControls(juniperConfig.GlobalControls, vlanNameToID)

	// Map IRBs (L3 handoffs)
	for _, irb := range juniperConfig.IRBs {
		l2Switch.L3Handoffs = append(l2Switch.L3Handoffs, mapJuniperIRB(irb))
	}

	logger.Info("Mapped IRBs", "count", len(l2Switch.L3Handoffs))

	return l2Switch, nil
}

// mapJuniperVLAN maps Juniper VLAN to unified model
func mapJuniperVLAN(vlan juniper.JuniperVLAN) models.L2VLAN {
	return models.L2VLAN{
		VLANID:            vlan.VLANID,
		VLANName:          vlan.VLANName,
		State:             "active", // Junos doesn't have explicit state; assume active if configured
		SourceBlock:       vlan.SourceBlock,
		SourceSnippetHash: vlan.SourceSnippetHash,
	}
}

// mapJuniperInterface maps Juniper interface to unified model
// vlanNameToID mapping is used to resolve VLAN names to IDs
func mapJuniperInterface(iface juniper.JuniperInterface, vlanNameToID map[string]int) models.L2SwitchPort {
	port := models.L2SwitchPort{
		InterfaceName:        iface.InterfaceName,
		Description:          iface.Description,
		AdminState:           "up",
		Mode:                 iface.PortMode,
		DHCPSnoopingTrust:    iface.DHCPSnoopingTrusted,
		ARPInspectionTrust:   iface.ARPInspectionTrusted,
		IPSourceGuard:        iface.IPSourceGuardEnabled,
		PortSecurityEnabled:  iface.PortSecurityEnabled,
		PortSecurityMaxMAC:   iface.PortSecurityMaxMAC,
		Dot1XRequired:        iface.Dot1XEnabled,
		MABEnabled:           iface.MABEnabled,
		SourceBlock:          iface.SourceBlock,
		SourceSnippetHash:    iface.SourceSnippetHash,
	}

	// Set admin state
	if iface.Disabled {
		port.AdminState = "down"
	}

	// Resolve VLAN names to IDs
	if iface.PortMode == "access" && iface.AccessVLAN != "" {
		if vlanID, exists := vlanNameToID[iface.AccessVLAN]; exists {
			port.AccessVLAN = vlanID
		}
	}

	if iface.PortMode == "trunk" {
		// Resolve native VLAN
		if iface.TrunkNativeVLAN != "" {
			if vlanID, exists := vlanNameToID[iface.TrunkNativeVLAN]; exists {
				port.TrunkNativeVLAN = vlanID
			}
		}

		// Resolve allowed VLANs
		for _, vlanName := range iface.TrunkAllowedVLANs {
			if vlanID, exists := vlanNameToID[vlanName]; exists {
				port.TrunkAllowedVLANs = append(port.TrunkAllowedVLANs, vlanID)
			}
		}
	}

	// Map AE membership
	if iface.AggregatedEthernetID != "" {
		// Extract numeric ID from "ae0" -> 0
		aeNum := strings.TrimPrefix(iface.AggregatedEthernetID, "ae")
		if aeID, err := strconv.Atoi(aeNum); err == nil {
			port.PortChannelID = aeID
		}
	}

	// Map guest/unauth VLAN
	if iface.GuestVLAN != "" {
		if vlanID, exists := vlanNameToID[iface.GuestVLAN]; exists {
			port.UnauthVLAN = vlanID
		}
	}

	return port
}

// mapJuniperAE maps Juniper Aggregated Ethernet to unified port-channel model
func mapJuniperAE(ae juniper.JuniperAE, vlanNameToID map[string]int) models.L2PortChannel {
	pc := models.L2PortChannel{
		PortChannelName:  ae.AEName,
		MemberInterfaces: ae.MemberInterfaces,
		Mode:             ae.PortMode,
		LACPMode:         ae.LACPMode,
		SourceBlock:      ae.SourceBlock,
		SourceSnippetHash: ae.SourceSnippetHash,
	}

	// Extract numeric ID from "ae0" -> 0
	aeNum := strings.TrimPrefix(ae.AEName, "ae")
	if aeID, err := strconv.Atoi(aeNum); err == nil {
		pc.PortChannelID = aeID
	}

	// Resolve VLAN names to IDs
	if ae.PortMode == "access" && ae.AccessVLAN != "" {
		if vlanID, exists := vlanNameToID[ae.AccessVLAN]; exists {
			pc.AccessVLAN = vlanID
		}
	}

	if ae.PortMode == "trunk" {
		// Resolve native VLAN
		if ae.TrunkNativeVLAN != "" {
			if vlanID, exists := vlanNameToID[ae.TrunkNativeVLAN]; exists {
				pc.TrunkNativeVLAN = vlanID
			}
		}

		// Resolve allowed VLANs
		for _, vlanName := range ae.TrunkAllowedVLANs {
			if vlanID, exists := vlanNameToID[vlanName]; exists {
				pc.TrunkAllowedVLANs = append(pc.TrunkAllowedVLANs, vlanID)
			}
		}
	}

	return pc
}

// mapJuniperGlobalControls maps Juniper global controls to unified model
func mapJuniperGlobalControls(controls juniper.JuniperGlobalL2Controls, vlanNameToID map[string]int) models.L2GlobalControls {
	gc := models.L2GlobalControls{
		DHCPSnoopingEnabled:    controls.DHCPSecurityEnabled,
		IPSGEnabled:            controls.IPSourceGuardEnabled,
		Dot1XSystemAuthControl: controls.Dot1XEnabled,
		SourceBlock:            controls.SourceBlock,
		SourceSnippetHash:      controls.SourceSnippetHash,
	}

	// Resolve DHCP security VLANs
	for _, vlanName := range controls.DHCPSecurityVLANs {
		if vlanID, exists := vlanNameToID[vlanName]; exists {
			gc.DHCPSnoopingVLANs = append(gc.DHCPSnoopingVLANs, vlanID)
		}
	}

	// Resolve ARP inspection VLANs
	for _, vlanName := range controls.ARPInspectionVLANs {
		if vlanID, exists := vlanNameToID[vlanName]; exists {
			gc.DAIEnabledVLANs = append(gc.DAIEnabledVLANs, vlanID)
		}
	}

	return gc
}

// mapJuniperIRB maps Juniper IRB to L3 handoff marker
func mapJuniperIRB(irb juniper.JuniperIRB) models.L3HandoffMarker {
	adminState := "up"
	if irb.Disabled {
		adminState = "down"
	}

	return models.L3HandoffMarker{
		VLANID:            irb.VLANID,
		InterfaceName:     irb.IRBName,
		IPAddress:         irb.IPAddress,
		AdminState:        adminState,
		SourceBlock:       irb.SourceBlock,
		SourceSnippetHash: irb.SourceSnippetHash,
	}
}

// MapJuniperFDBToEntries maps Juniper FDB entries to unified model
func MapJuniperFDBToEntries(juniperEntries []juniper.JuniperFDBEntry, deviceID string, vlanNameToID map[string]int) []models.FDBEntry {
	entries := make([]models.FDBEntry, 0, len(juniperEntries))

	for _, juniperEntry := range juniperEntries {
		// Resolve VLAN name to ID
		vlanID := 0
		if id, exists := vlanNameToID[juniperEntry.VLANName]; exists {
			vlanID = id
		}

		entryType := "dynamic"
		if juniperEntry.IsStatic {
			entryType = "static"
		}

		entry := models.FDBEntry{
			DeviceID:       deviceID,
			MACAddress:     juniperEntry.MACAddress,
			VLANID:         vlanID,
			Port:           juniperEntry.InterfaceName,
			Type:           entryType,
			EvidenceSource: juniperEntry.SourceSnippetHash,
			Confidence:     1.0, // Direct observation
		}

		entries = append(entries, entry)
	}

	return entries
}

// MapJuniperLLDPToNeighbors maps Juniper LLDP neighbors to unified model
func MapJuniperLLDPToNeighbors(juniperNeighbors []juniper.JuniperLLDPNeighbor, deviceID string) []models.LLDPNeighbor {
	neighbors := make([]models.LLDPNeighbor, 0, len(juniperNeighbors))

	for _, juniperNeighbor := range juniperNeighbors {
		neighbor := models.LLDPNeighbor{
			DeviceID:         deviceID,
			LocalInterface:   juniperNeighbor.LocalInterface,
			RemoteDeviceID:   juniperNeighbor.RemoteChassisID,
			RemotePortID:     juniperNeighbor.RemotePort,
			RemoteSystemName: juniperNeighbor.RemoteSystemName,
			RemoteSystemDesc: juniperNeighbor.RemoteSystemDescription,
			ChassisID:        juniperNeighbor.RemoteChassisID,
		}

		neighbors = append(neighbors, neighbor)
	}

	return neighbors
}

// ValidateJuniperL2Config validates a Juniper L2 configuration
// Returns warnings for common misconfigurations
func ValidateJuniperL2Config(config *juniper.JuniperL2Config) []string {
	var warnings []string

	// Build VLAN name-to-ID mapping
	vlanNameToID := make(map[string]int)
	vlanIDToName := make(map[int]string)
	for _, vlan := range config.VLANs {
		vlanNameToID[vlan.VLANName] = vlan.VLANID
		vlanIDToName[vlan.VLANID] = vlan.VLANName
	}

	// Check for default VLAN
	if _, exists := vlanIDToName[1]; !exists {
		warnings = append(warnings, "No default VLAN (VLAN 1) found")
	}

	// Check for interfaces referencing non-existent VLANs
	for _, iface := range config.Interfaces {
		if iface.PortMode == "access" && iface.AccessVLAN != "" {
			if _, exists := vlanNameToID[iface.AccessVLAN]; !exists {
				warnings = append(warnings, fmt.Sprintf("Interface %s references non-existent access VLAN '%s'", iface.InterfaceName, iface.AccessVLAN))
			}
		}

		for _, vlanName := range iface.TrunkAllowedVLANs {
			if _, exists := vlanNameToID[vlanName]; !exists {
				warnings = append(warnings, fmt.Sprintf("Interface %s references non-existent trunk VLAN '%s'", iface.InterfaceName, vlanName))
			}
		}
	}

	// Check for AE interfaces with few members
	aeMemberCount := make(map[string]int)
	for _, iface := range config.Interfaces {
		if iface.AggregatedEthernetID != "" {
			aeMemberCount[iface.AggregatedEthernetID]++
		}
	}

	for aeID, count := range aeMemberCount {
		if count < 2 {
			warnings = append(warnings, fmt.Sprintf("AE interface %s has only %d member(s) - redundancy compromised", aeID, count))
		}
	}

	// Check for DHCP security without trusted uplinks
	if config.GlobalControls.DHCPSecurityEnabled {
		hasTrustedUplink := false
		for _, iface := range config.Interfaces {
			if iface.PortMode == "trunk" && iface.DHCPSnoopingTrusted {
				hasTrustedUplink = true
				break
			}
		}
		if !hasTrustedUplink {
			warnings = append(warnings, "DHCP security enabled but no trusted trunk ports found - may block legitimate DHCP")
		}
	}

	// Check for 802.1X without RADIUS configuration
	if config.GlobalControls.Dot1XEnabled {
		if len(config.GlobalControls.RADIUSServers) == 0 {
			warnings = append(warnings, "802.1X enabled but no RADIUS servers configured")
		}
	}

	// Check for IRBs referencing non-existent VLANs
	for _, irb := range config.IRBs {
		if irb.VLANName != "" {
			if _, exists := vlanNameToID[irb.VLANName]; !exists {
				warnings = append(warnings, fmt.Sprintf("IRB %s references non-existent VLAN '%s'", irb.IRBName, irb.VLANName))
			}
		}
	}

	return warnings
}

// EnrichJuniperInterfacesWithStatus enriches interface config with operational status
func EnrichJuniperInterfacesWithStatus(interfaces []juniper.JuniperInterface, status map[string]juniper.JuniperInterfaceStatus) []juniper.JuniperInterface {
	enriched := make([]juniper.JuniperInterface, len(interfaces))
	copy(enriched, interfaces)

	for i := range enriched {
		iface := &enriched[i]

		// Check both physical interface and logical unit
		physicalIface := iface.InterfaceName
		logicalIface := fmt.Sprintf("%s.%d", iface.InterfaceName, iface.Unit)

		// Try logical interface first (more specific)
		if ifStatus, exists := status[logicalIface]; exists {
			if ifStatus.AdminStatus == "down" {
				iface.Disabled = true
			}
			// Could also check OperStatus for admin up/oper down scenarios
		} else if ifStatus, exists := status[physicalIface]; exists {
			if ifStatus.AdminStatus == "down" {
				iface.Disabled = true
			}
		}
	}

	return enriched
}
