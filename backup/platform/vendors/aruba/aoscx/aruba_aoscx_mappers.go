package aoscx

import (
	"fmt"
	"github.com/mattpainter701/graphlayer/platform/models"
	"github.com/mattpainter701/graphlayer/platform/vendors/aruba"
	"log/slog"
)

// Aruba AOS-CX Mappers
// Transform Aruba-specific data structures into unified platform models

// MapArubaToL2Switch maps Aruba L2 config to unified L2Switch model
func MapArubaToL2Switch(arubaConfig *aruba.ArubaL2Config) (*models.L2Switch, error) {
	logger := slog.Default().With(
		"vendor", "aruba",
		"platform", "aos-cx",
		"device_id", arubaConfig.Device.DeviceID,
	)

	l2Switch := &models.L2Switch{
		DeviceID:   arubaConfig.Device.DeviceID,
		Hostname:   arubaConfig.Device.Hostname,
		Vendor:     "aruba",
		OSName:     "aos-cx",
		OSVersion:  arubaConfig.Device.OSVersion,
		Model:      arubaConfig.Device.Model,
		ConfigHash: arubaConfig.Device.ConfigHash,
		VLANs:      []models.L2VLAN{},
		Ports:      []models.L2SwitchPort{},
		PortChannels: []models.L2PortChannel{},
		GlobalControls: models.L2GlobalControls{},
		L3Handoffs:  []models.L3HandoffMarker{},
	}

	// Map VLANs
	for _, vlan := range arubaConfig.VLANs {
		l2Switch.VLANs = append(l2Switch.VLANs, mapArubaVLAN(vlan))
	}

	logger.Info("Mapped VLANs", "count", len(l2Switch.VLANs))

	// Map interfaces
	for _, iface := range arubaConfig.Interfaces {
		l2Switch.Ports = append(l2Switch.Ports, mapArubaInterface(iface))
	}

	logger.Info("Mapped interfaces", "count", len(l2Switch.Ports))

	// Map port-channels
	for _, pc := range arubaConfig.PortChannels {
		l2Switch.PortChannels = append(l2Switch.PortChannels, mapArubaPortChannel(pc))
	}

	logger.Info("Mapped port-channels", "count", len(l2Switch.PortChannels))

	// Map global controls
	l2Switch.GlobalControls = mapArubaGlobalControls(arubaConfig.GlobalControls)

	// Map SVIs (L3 handoffs)
	for _, svi := range arubaConfig.SVIs {
		l2Switch.L3Handoffs = append(l2Switch.L3Handoffs, mapArubaSVI(svi))
	}

	logger.Info("Mapped SVIs", "count", len(l2Switch.L3Handoffs))

	return l2Switch, nil
}

// mapArubaVLAN maps Aruba VLAN to unified model
func mapArubaVLAN(vlan aruba.ArubaVLAN) models.L2VLAN {
	return models.L2VLAN{
		VLANID:            vlan.VLANID,
		VLANName:          vlan.Name,
		State:             vlan.State,
		VACLName:          vlan.VACLName,
		SourceBlock:       vlan.SourceBlock,
		SourceSnippetHash: vlan.SourceSnippetHash,
	}
}

// mapArubaInterface maps Aruba interface to unified model
func mapArubaInterface(iface aruba.ArubaInterface) models.L2SwitchPort {
	return models.L2SwitchPort{
		InterfaceName:      iface.InterfaceName,
		Description:        iface.Description,
		AdminState:         iface.AdminState,
		Mode:               iface.Mode,
		AccessVLAN:         iface.AccessVLAN,
		TrunkNativeVLAN:    iface.TrunkNativeVLAN,
		TrunkAllowedVLANs:  iface.TrunkAllowedVLANs,
		VoiceVLAN:          iface.VoiceVLAN,
		PortChannelID:      iface.PortChannelID,
		DHCPSnoopingTrust:  iface.DHCPSnoopingTrust,
		ARPInspectionTrust: iface.ARPInspectionTrust,
		IPSourceGuard:      iface.IPSourceGuard,
		PortSecurityEnabled: iface.PortSecurityEnabled,
		PortSecurityMaxMAC: iface.PortSecurityMaxMAC,
		Dot1XRequired:      iface.Dot1XRequired,
		MABEnabled:         iface.MABEnabled,
		UnauthVLAN:         iface.UnauthVLAN,
		PACLInName:         iface.PACLInName,
		PACLOutName:        iface.PACLOutName,
		SourceBlock:        iface.SourceBlock,
		SourceSnippetHash:  iface.SourceSnippetHash,
	}
}

// mapArubaPortChannel maps Aruba port-channel to unified model
func mapArubaPortChannel(pc aruba.ArubaPortChannel) models.L2PortChannel {
	return models.L2PortChannel{
		PortChannelID:     pc.PortChannelID,
		PortChannelName:   pc.PortChannelName,
		MemberInterfaces:  pc.MemberInterfaces,
		Mode:              pc.Mode,
		AccessVLAN:        pc.AccessVLAN,
		TrunkNativeVLAN:   pc.TrunkNativeVLAN,
		TrunkAllowedVLANs: pc.TrunkAllowedVLANs,
		LACPMode:          pc.LACPMode,
		SourceBlock:       pc.SourceBlock,
		SourceSnippetHash: pc.SourceSnippetHash,
	}
}

// mapArubaGlobalControls maps Aruba global controls to unified model
func mapArubaGlobalControls(controls aruba.ArubaGlobalL2Controls) models.L2GlobalControls {
	return models.L2GlobalControls{
		DHCPSnoopingEnabled:    controls.DHCPSnoopingEnabled,
		DHCPSnoopingVLANs:      controls.DHCPSnoopingVLANs,
		DAIEnabledVLANs:        controls.DAIEnabledVLANs,
		DAIACLNameByVLAN:       controls.DAIACLNameByVLAN,
		IPSGEnabled:            controls.IPSGEnabled,
		Dot1XSystemAuthControl: controls.Dot1XSystemAuthControl,
		MABGlobalEnabled:       controls.MABGlobalEnabled,
		SourceBlock:            controls.SourceBlock,
		SourceSnippetHash:      controls.SourceSnippetHash,
	}
}

// mapArubaSVI maps Aruba SVI to L3 handoff marker
func mapArubaSVI(svi aruba.ArubaSVI) models.L3HandoffMarker {
	return models.L3HandoffMarker{
		VLANID:            svi.VLANID,
		InterfaceName:     svi.InterfaceName,
		IPAddress:         fmt.Sprintf("%s/%s", svi.IPAddress, svi.SubnetMask),
		AdminState:        svi.AdminState,
		SourceBlock:       svi.SourceBlock,
		SourceSnippetHash: svi.SourceSnippetHash,
	}
}

// MapArubaFDBToEntries maps Aruba FDB entries to unified model
func MapArubaFDBToEntries(arubaEntries []aruba.ArubaFDBEntry, deviceID string) []models.FDBEntry {
	entries := make([]models.FDBEntry, 0, len(arubaEntries))

	for _, arubaEntry := range arubaEntries {
		entry := models.FDBEntry{
			DeviceID:       deviceID,
			MACAddress:     arubaEntry.MACAddress,
			VLANID:         arubaEntry.VLANID,
			Port:           arubaEntry.Port,
			Type:           arubaEntry.Type,
			LastSeenTime:   arubaEntry.LastSeenTime,
			EvidenceSource: arubaEntry.EvidenceSource,
			Confidence:     arubaEntry.Confidence,
		}

		entries = append(entries, entry)
	}

	return entries
}

// MapArubaLLDPToNeighbors maps Aruba LLDP neighbors to unified model
func MapArubaLLDPToNeighbors(arubaNeighbors []aruba.ArubaLLDPNeighbor, deviceID string) []models.LLDPNeighbor {
	neighbors := make([]models.LLDPNeighbor, 0, len(arubaNeighbors))

	for _, arubaNeighbor := range arubaNeighbors {
		neighbor := models.LLDPNeighbor{
			DeviceID:         deviceID,
			LocalInterface:   arubaNeighbor.LocalInterface,
			RemoteDeviceID:   arubaNeighbor.RemoteDeviceID,
			RemotePortID:     arubaNeighbor.RemotePortID,
			RemoteSystemName: arubaNeighbor.RemoteSystemName,
			RemoteSystemDesc: arubaNeighbor.RemoteSystemDesc,
			ChassisID:        arubaNeighbor.ChassisID,
			Capabilities:     arubaNeighbor.Capabilities,
			ManagementIP:     arubaNeighbor.ManagementIP,
		}

		neighbors = append(neighbors, neighbor)
	}

	return neighbors
}

// ValidateArubaL2Config validates an Aruba L2 configuration
// Returns warnings for common misconfigurations
func ValidateArubaL2Config(config *aruba.ArubaL2Config) []string {
	var warnings []string

	// Check for default VLAN
	hasDefaultVLAN := false
	for _, vlan := range config.VLANs {
		if vlan.VLANID == 1 {
			hasDefaultVLAN = true
			break
		}
	}
	if !hasDefaultVLAN {
		warnings = append(warnings, "No default VLAN (VLAN 1) found")
	}

	// Check for suspended VLANs with active interfaces
	suspendedVLANs := make(map[int]bool)
	for _, vlan := range config.VLANs {
		if vlan.State == "suspended" {
			suspendedVLANs[vlan.VLANID] = true
		}
	}

	for _, iface := range config.Interfaces {
		if iface.AdminState == "up" {
			if iface.Mode == "access" && suspendedVLANs[iface.AccessVLAN] {
				warnings = append(warnings, fmt.Sprintf("Interface %s is up but access VLAN %d is suspended", iface.InterfaceName, iface.AccessVLAN))
			}
		}
	}

	// Check for LAG members with mismatched settings
	lagMap := make(map[int][]string)
	for _, iface := range config.Interfaces {
		if iface.PortChannelID > 0 {
			lagMap[iface.PortChannelID] = append(lagMap[iface.PortChannelID], iface.InterfaceName)
		}
	}

	for lagID, members := range lagMap {
		if len(members) < 2 {
			warnings = append(warnings, fmt.Sprintf("LAG %d has only %d member(s) - redundancy compromised", lagID, len(members)))
		}
	}

	// Check for DHCP snooping without trusted uplinks
	if config.GlobalControls.DHCPSnoopingEnabled {
		hasTrustedUplink := false
		for _, iface := range config.Interfaces {
			if iface.Mode == "trunk" && iface.DHCPSnoopingTrust {
				hasTrustedUplink = true
				break
			}
		}
		if !hasTrustedUplink {
			warnings = append(warnings, "DHCP snooping enabled but no trusted trunk ports found - may block legitimate DHCP")
		}
	}

	// Check for 802.1X without RADIUS configuration
	if config.GlobalControls.Dot1XSystemAuthControl {
		if len(config.GlobalControls.RADIUSServers) == 0 {
			warnings = append(warnings, "802.1X enabled but no RADIUS servers configured")
		}
	}

	return warnings
}

// EnrichArubaInterfacesWithStatus enriches interface config with operational status
func EnrichArubaInterfacesWithStatus(interfaces []aruba.ArubaInterface, status map[string]map[string]string) []aruba.ArubaInterface {
	enriched := make([]aruba.ArubaInterface, len(interfaces))
	copy(enriched, interfaces)

	for i := range enriched {
		iface := &enriched[i]
		if ifStatus, exists := status[iface.InterfaceName]; exists {
			// Update with operational state
			if operState, ok := ifStatus["oper_state"]; ok && operState == "down" {
				// Interface is administratively up but operationally down
				if iface.AdminState == "up" {
					iface.AdminState = "up/down" // Indicates admin up, oper down
				}
			}
		}
	}

	return enriched
}
