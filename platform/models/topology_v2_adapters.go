package models

import "fmt"

// LegacyDeviceToV2 adapts the existing platform Device model to the normalized
// topology v2 record without changing vendor parser output contracts.
func LegacyDeviceToV2(device Device, evidence []EvidenceRefV2) DeviceV2 {
	recordID := device.ID
	if recordID == "" {
		recordID = stableRecordIDV2(TopologyRecordKindDevice, device.Hostname)
	}
	meta := NewTopologyRecordMetaV2(TopologyRecordKindDevice, recordID, evidence, nil)
	meta.SourceVendor = device.Vendor
	meta.DeviceID = recordID
	return DeviceV2{
		Meta:            meta,
		Hostname:        device.Hostname,
		Vendor:          device.Vendor,
		Model:           device.Model,
		OSName:          firstNonEmptyV2(device.OSName, device.Vendor),
		OSVersion:       firstNonEmptyV2(device.OSVersion, device.Version),
		ManagementIPs:   compactStringsV2([]string{device.ManagementIP}),
		Site:            device.Site,
		Role:            device.Role,
		VirtualContexts: append([]string{}, device.VirtualContexts...),
		InterfaceIDs:    append([]string{}, device.Interfaces...),
		Attributes: map[string]string{
			"config_hash": device.ConfigHash,
		},
	}
}

// LegacyInterfaceToV2 adapts the existing Interface model. deviceID is supplied
// by parser context because the legacy type only stores virtual context.
func LegacyInterfaceToV2(iface Interface, deviceID string, evidence []EvidenceRefV2) InterfaceV2 {
	recordID := iface.ID
	if recordID == "" {
		recordID = stableRecordIDV2(TopologyRecordKindInterface, deviceID, iface.Name)
	}
	meta := NewTopologyRecordMetaV2(TopologyRecordKindInterface, recordID, evidence, nil)
	meta.DeviceID = deviceID
	meta.VirtualContext = iface.VirtualFirewall
	return InterfaceV2{
		Meta:              meta,
		DeviceID:          deviceID,
		Name:              iface.Name,
		Type:              iface.Type,
		AdminStatus:       iface.Status,
		OperStatus:        iface.LinkStatus,
		IPAddresses:       parseIPAssignmentsV2(iface.Addresses),
		VLANIDs:           optionalVLANV2(iface.VlanID),
		ParentInterfaceID: iface.ParentInterface,
		ZoneIDs:           append([]string{}, iface.Zones...),
		Description:       iface.Description,
	}
}

// LegacyL2VLANToV2 adapts L2 VLAN parser output to the universal VLAN record.
func LegacyL2VLANToV2(vlan L2VLAN, deviceID string, evidence []EvidenceRefV2) VLANV2 {
	recordID := stableRecordIDV2(TopologyRecordKindVLAN, deviceID, fmt.Sprintf("%d", vlan.VLANID))
	if len(evidence) == 0 && (vlan.SourceBlock != "" || vlan.SourceSnippetHash != "") {
		evidence = []EvidenceRefV2{{
			SourceType:        ConfidenceSourceConfig,
			SourceSystem:      "legacy-l2-vlan",
			SourceBlock:       vlan.SourceBlock,
			SourceSnippetHash: vlan.SourceSnippetHash,
			Fields:            []string{"vlan_id", "name", "state"},
			Quality:           1.0,
		}}
	}
	meta := NewTopologyRecordMetaV2(TopologyRecordKindVLAN, recordID, evidence, nil)
	meta.DeviceID = deviceID
	return VLANV2{
		Meta:      meta,
		VLANID:    vlan.VLANID,
		Name:      vlan.VLANName,
		DeviceIDs: compactStringsV2([]string{deviceID}),
		State:     vlan.State,
	}
}

// LegacyRoutingEntryToV2 adapts existing firewall/router routing records.
func LegacyRoutingEntryToV2(route RoutingEntry, deviceID string, evidence []EvidenceRefV2) RouteV2 {
	recordID := route.ID
	if recordID == "" {
		recordID = stableRecordIDV2(TopologyRecordKindRoute, deviceID, route.VirtualContext, route.Destination, route.NextHop, route.Interface)
	}
	meta := NewTopologyRecordMetaV2(TopologyRecordKindRoute, recordID, evidence, nil)
	meta.DeviceID = deviceID
	meta.VirtualContext = route.VirtualContext
	return RouteV2{
		Meta:              meta,
		DestinationPrefix: route.Destination,
		AddressFamily:     addressFamilyForPrefixV2(route.Destination),
		VRF:               route.VirtualRouter,
		Protocol:          route.Protocol,
		Type:              route.Type,
		NextHops: []RouteNextHopV2{{
			IPAddress: route.NextHop,
			Interface: route.Interface,
		}},
		Metric:        route.Metric,
		AdminDistance: route.AdminDistance,
	}
}

// LegacyFirewallPolicyToV2 adapts access/security policy records.
func LegacyFirewallPolicyToV2(policy FirewallPolicy, evidence []EvidenceRefV2) SecurityPolicyV2 {
	recordID := policy.ID
	if recordID == "" {
		recordID = stableRecordIDV2(TopologyRecordKindSecurityPolicy, policy.Firewall, policy.VirtualContext, policy.Name, fmt.Sprintf("%d", policy.RuleOrder))
	}
	meta := NewTopologyRecordMetaV2(TopologyRecordKindSecurityPolicy, recordID, evidence, nil)
	meta.DeviceID = policy.Firewall
	meta.VirtualContext = policy.VirtualContext
	return SecurityPolicyV2{
		Meta:                 meta,
		Name:                 policy.Name,
		Enabled:              policy.Enabled,
		Action:               policy.Action,
		Direction:            policy.Direction,
		Order:                policy.RuleOrder,
		SourceZones:          append([]string{}, policy.SourceZones...),
		DestinationZones:     append([]string{}, policy.DestinationZones...),
		SourceAddresses:      append([]string{}, policy.SourceAddresses...),
		DestinationAddresses: append([]string{}, policy.DestinationAddresses...),
		Services:             append([]string{}, policy.Services...),
		Applications:         append([]string{}, policy.Applications...),
		Log:                  policy.Log.AtStart || policy.Log.AtEnd,
		Description:          policy.Description,
	}
}

// LegacyNATToV2 adapts legacy NAT objects to normalized NATRule records.
func LegacyNATToV2(nat NAT, deviceID string, evidence []EvidenceRefV2) NATRuleV2 {
	recordID := nat.ID
	if recordID == "" {
		recordID = stableRecordIDV2(TopologyRecordKindNATRule, deviceID, nat.VirtualContext, nat.Name, nat.Type)
	}
	meta := NewTopologyRecordMetaV2(TopologyRecordKindNATRule, recordID, evidence, nil)
	meta.DeviceID = deviceID
	meta.VirtualContext = nat.VirtualContext
	return NATRuleV2{
		Meta:                  meta,
		Name:                  nat.Name,
		Type:                  nat.Type,
		OriginalSource:        compactStringsV2([]string{nat.OriginalSource}),
		OriginalDestination:   compactStringsV2([]string{nat.OriginalDestination}),
		TranslatedSource:      compactStringsV2([]string{nat.TranslatedSource}),
		TranslatedDestination: compactStringsV2([]string{nat.TranslatedDestination}),
		OriginalService:       compactStringsV2([]string{nat.OriginalService}),
		TranslatedService:     compactStringsV2([]string{nat.TranslatedService}),
		InboundInterfaceID:    nat.InboundInterface,
		OutboundInterfaceID:   nat.OutboundInterface,
		Bidirectional:         nat.Bidirectional,
		Enabled:               true,
	}
}

// LegacyTunnelToVPNV2 adapts tunnel records to VPN records.
func LegacyTunnelToVPNV2(tunnel Tunnel, evidence []EvidenceRefV2) VPNV2 {
	recordID := tunnel.ID
	if recordID == "" {
		recordID = stableRecordIDV2(TopologyRecordKindVPN, tunnel.VirtualContext, tunnel.Name, tunnel.RemoteEndpoint)
	}
	meta := NewTopologyRecordMetaV2(TopologyRecordKindVPN, recordID, evidence, nil)
	meta.VirtualContext = tunnel.VirtualContext
	routing := "static"
	if tunnel.RouteInjection {
		routing = "route_injection"
	}
	return VPNV2{
		Meta:              meta,
		Name:              tunnel.Name,
		Type:              tunnel.Type,
		LocalEndpoint:     tunnel.LocalEndpoint,
		RemoteEndpoint:    tunnel.RemoteEndpoint,
		LocalNetworks:     append([]string{}, tunnel.LocalNetworks...),
		RemoteNetworks:    append([]string{}, tunnel.RemoteNetworks...),
		EncryptionProfile: tunnel.EncryptionProfile,
		Routing:           routing,
		FailoverGroup:     tunnel.FailoverGroup,
	}
}

func parseIPAssignmentsV2(addresses []string) []IPAssignmentV2 {
	assignments := make([]IPAssignmentV2, 0, len(addresses))
	for i, address := range addresses {
		if address == "" {
			continue
		}
		assignments = append(assignments, IPAssignmentV2{
			Address:       address,
			AddressFamily: addressFamilyForPrefixV2(address),
			Primary:       i == 0,
		})
	}
	return assignments
}

func optionalVLANV2(vlanID int) []int {
	if vlanID <= 0 {
		return nil
	}
	return []int{vlanID}
}

func compactStringsV2(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func firstNonEmptyV2(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func stableRecordIDV2(kind TopologyRecordKindV2, parts ...string) string {
	id := string(kind)
	for _, part := range parts {
		if part != "" {
			id += ":" + part
		}
	}
	return id
}

func addressFamilyForPrefixV2(value string) string {
	for _, ch := range value {
		if ch == ':' {
			return "ipv6"
		}
	}
	return "ipv4"
}
