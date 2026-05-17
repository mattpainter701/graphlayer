package cisco

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mattpainter701/graphlayer/platform/models"
	"github.com/mattpainter701/graphlayer/platform/core/constants"
	helper "github.com/mattpainter701/graphlayer/platform/core/helpers"
	"github.com/mattpainter701/graphlayer/platform/core/processor"
)

const defaultNXOSContextName = "default-nxos-context"

// +-+-+-+-+-+-+-+-+-+-+-+-+
// |N|X|O|S| |M|a|p|p|e|r|s|
// +-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The following methods are responsible for mapping Cisco NXOS operational data
 * (collected from show commands) to common topology models.
 *
 * NXOS-specific features:
 * - VPC (Virtual Port Channel) dual-homing topology
 * - FabricPath IS-IS topology
 * - VDC (Virtual Device Context) on Nexus 7K
 * - Enhanced L2/L3 topology with advanced VLAN features
 */

// CiscoNXOSProcessor is the processor for NXOS operational data
type CiscoNXOSProcessor struct {
	// Operational Data from show commands
	nxosConfig *NXOSConfig

	// Topology Models (vendor-agnostic)
	TopologyDevice          *models.Device
	TopologyInterfaces      map[string]models.Interface
	TopologyAddresses       map[string]models.Address
	TopologyAddressGroups   map[string]models.AddressGroup
	TopologyServices        map[string]models.Service
	TopologyServiceGroups   map[string]models.ServiceGroup
	TopologyRoutingEntries  map[string]models.RoutingEntry
	TopologyNATs            map[string]models.NAT
	TopologyVIPs            map[string]models.VIP
	TopologyFirewallPolicies map[string]models.FirewallPolicy
	TopologyZones           map[string]models.Zone
	TopologyVirtualContexts map[string]models.VirtualContext

	// NXOS-specific topology (VPC, FabricPath)
	TopologyVPC             *models.VPCTopology
	TopologyPortChannels    map[string]models.L2PortChannel
	TopologyVLANs           map[string]models.L2VLAN

	// Metadata
	FileHash string
	IntegrationMessages []models.IntegrationMessage
}

// NewCiscoNXOSProcessor creates a new NXOS processor
func NewCiscoNXOSProcessor(nxosConfig *NXOSConfig) *CiscoNXOSProcessor {
	return &CiscoNXOSProcessor{
		nxosConfig:              nxosConfig,
		TopologyInterfaces:      make(map[string]models.Interface),
		TopologyAddresses:       make(map[string]models.Address),
		TopologyAddressGroups:   make(map[string]models.AddressGroup),
		TopologyServices:        make(map[string]models.Service),
		TopologyServiceGroups:   make(map[string]models.ServiceGroup),
		TopologyRoutingEntries:  make(map[string]models.RoutingEntry),
		TopologyNATs:            make(map[string]models.NAT),
		TopologyVIPs:            make(map[string]models.VIP),
		TopologyFirewallPolicies: make(map[string]models.FirewallPolicy),
		TopologyZones:           make(map[string]models.Zone),
		TopologyVirtualContexts: make(map[string]models.VirtualContext),
		TopologyPortChannels:    make(map[string]models.L2PortChannel),
		TopologyVLANs:           make(map[string]models.L2VLAN),
	}
}

// BuildNetworkTopologyModels maps NXOS operational data to standard network topology models
func (p *CiscoNXOSProcessor) BuildNetworkTopologyModels() error {
	// Initialize topology model
	p.TopologyDevice = &models.Device{}

	// Build the Device model
	err := p.buildNXOSDeviceModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building NXOS device model")
		return err
	}

	// Build interfaces from operational data (show interface status)
	err = p.buildNXOSInterfaceModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building NXOS interface model")
		return err
	}

	// Build VLANs from operational data (show vlan)
	err = p.buildNXOSVLANModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building NXOS VLAN model")
		return err
	}

	// Build port-channels/LAGs (show port-channel summary)
	err = p.buildNXOSPortChannelModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building NXOS port-channel model")
		return err
	}

	// Build VPC topology (show vpc, show vpc peer-keepalive)
	err = p.buildNXOSVPCModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building NXOS VPC model")
		return err
	}

	// Build addresses from ARP table and interface IPs
	err = p.buildNXOSAddressModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building NXOS address model")
		return err
	}

	// Build routing entries from routing table
	err = p.buildNXOSRoutingModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building NXOS routing model")
		return err
	}

	// Build zones from VLANs
	err = p.buildNXOSZoneModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building NXOS zone model")
		return err
	}

	return nil
}

// buildNXOSDeviceModel creates the device model for NXOS switches
func (p *CiscoNXOSProcessor) buildNXOSDeviceModel() error {
	deviceRole := "switch" // NXOS devices are typically switches

	// Determine role based on operational data
	if p.nxosConfig.VPCStatus != nil {
		deviceRole = "vpc-switch" // VPC-enabled switch
	} else if p.nxosConfig.FabricPathTopology != nil {
		deviceRole = "fabricpath-switch" // FabricPath switch
	}

	p.TopologyDevice = &models.Device{
		ID:       uuid.New().String(),
		Hostname: p.nxosConfig.Hostname,
		Vendor:   models.IntegrationCisco.String(),
		Model:    "nxos",
		Version:  p.nxosConfig.Version,
		Role:     deviceRole,
	}

	// Create default virtual context for NXOS
	// Note: NXOS uses VDCs on Nexus 7K, but we'll use default context for simplicity
	if _, exists := p.TopologyVirtualContexts[defaultNXOSContextName]; !exists {
		defaultContextID := uuid.New().String()
		p.TopologyVirtualContexts[defaultNXOSContextName] = models.VirtualContext{
			ID:           defaultContextID,
			Name:         defaultNXOSContextName,
			Description:  fmt.Sprintf("Default NXOS context for %s", p.nxosConfig.Hostname),
			Type:         models.CISCO_VIRTUAL_CONTEXT,
			ParentDevice: p.TopologyDevice.ID,
		}
	}
	p.TopologyDevice.VirtualContexts = []string{p.TopologyVirtualContexts[defaultNXOSContextName].Name}

	return nil
}

// buildNXOSInterfaceModel maps NXOS interfaces from operational data (show interface status)
func (p *CiscoNXOSProcessor) buildNXOSInterfaceModel() error {
	// Map interface status to topology interfaces
	for _, ifaceStatus := range p.nxosConfig.InterfaceStatus {
		// Skip disconnected interfaces
		if ifaceStatus.Status != "connected" {
			continue
		}

		interfaceID := uuid.New().String()

		// Determine interface type
		interfaceType := "physical"
		if strings.Contains(strings.ToLower(ifaceStatus.Port), "vlan") {
			interfaceType = "vlan"
		} else if strings.Contains(strings.ToLower(ifaceStatus.Port), "loopback") {
			interfaceType = "loopback"
		} else if strings.Contains(strings.ToLower(ifaceStatus.Port), "port-channel") ||
		          strings.Contains(strings.ToLower(ifaceStatus.Port), "po") {
			interfaceType = "aggregated"
		}

		// Determine role based on VLAN assignment and naming
		role := "internal"
		if ifaceStatus.VLAN == "trunk" {
			role = "trunk"
		} else if strings.Contains(strings.ToLower(ifaceStatus.Name), "uplink") {
			role = "uplink"
		} else if strings.Contains(strings.ToLower(ifaceStatus.Name), "downlink") {
			role = "downlink"
		}

		// Check if this is a VPC peer-link
		isVPCPeerLink := false
		if p.nxosConfig.VPCStatus != nil &&
		   strings.Contains(strings.ToLower(ifaceStatus.Port), p.nxosConfig.VPCStatus.PeerLinkPort) {
			isVPCPeerLink = true
			role = "vpc-peer-link"
		}

		// Get VLAN ID
		vlanID := 0
		if ifaceStatus.VLAN != "trunk" && ifaceStatus.VLAN != "" {
			fmt.Sscanf(ifaceStatus.VLAN, "%d", &vlanID)
		}

		interfaceModel := models.Interface{
			ID:              interfaceID,
			Name:            ifaceStatus.Port,
			VlanID:          vlanID,
			Role:            role,
			Type:            interfaceType,
			Description:     ifaceStatus.Name,
			VirtualFirewall: p.TopologyVirtualContexts[defaultNXOSContextName].Name,
		}

		// Add metadata for VPC peer-link
		if isVPCPeerLink {
			interfaceModel.Tags = map[string]string{
				"vpc_peer_link": "true",
			}
		}

		p.TopologyInterfaces[interfaceID] = interfaceModel
		p.TopologyDevice.Interfaces = append(p.TopologyDevice.Interfaces, interfaceModel.Name)
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Mapped %d NXOS interfaces from operational data", len(p.TopologyInterfaces)))

	return nil
}

// buildNXOSVLANModel maps VLANs from operational data (show vlan)
func (p *CiscoNXOSProcessor) buildNXOSVLANModel() error {
	for _, vlan := range p.nxosConfig.VLANOperational {
		vlanModel := models.L2VLAN{
			VLANID:   vlan.ID,
			VLANName: vlan.Name,
			State:    vlan.Status,
		}

		p.TopologyVLANs[fmt.Sprintf("vlan_%d", vlan.ID)] = vlanModel
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Mapped %d VLANs from NXOS operational data", len(p.TopologyVLANs)))

	return nil
}

// buildNXOSPortChannelModel maps port-channels from operational data
func (p *CiscoNXOSProcessor) buildNXOSPortChannelModel() error {
	for _, pc := range p.nxosConfig.PortChannels {
		// Determine mode from protocol
		mode := "static"
		if pc.Protocol == "LACP" {
			mode = "lacp"
		}

		pcModel := models.L2PortChannel{
			PortChannelID:    pc.GroupID,
			Name:             pc.Name,
			MemberInterfaces: pc.Members,
			Mode:             mode,
		}

		p.TopologyPortChannels[pc.Name] = pcModel

		// Check if this port-channel is a VPC peer-link
		if p.nxosConfig.VPCStatus != nil && pc.Name == p.nxosConfig.VPCStatus.PeerLinkPort {
			pcModel.Mode = "vpc-peer-link"
		}
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Mapped %d port-channels from NXOS operational data", len(p.TopologyPortChannels)))

	return nil
}

// buildNXOSVPCModel maps VPC topology from operational data
func (p *CiscoNXOSProcessor) buildNXOSVPCModel() error {
	if p.nxosConfig.VPCStatus == nil {
		// VPC not configured
		return nil
	}

	vpc := p.nxosConfig.VPCStatus

	// Create VPC topology model
	p.TopologyVPC = &models.VPCTopology{
		DomainID:         vpc.DomainID,
		Role:             vpc.VPCRole,
		PeerStatus:       vpc.PeerStatus,
		KeepaliveStatus:  vpc.KeepaliveStatus,
		PeerLinkPort:     vpc.PeerLinkPort,
		PeerLinkStatus:   vpc.PeerLinkStatus,
		NumberOfVPCs:     vpc.NumberOfVPCs,
		PeerGateway:      vpc.PeerGateway,
		ConfigConsistent: vpc.ConfigConsistencyStatus == "success",
	}

	// Add keepalive information if available
	if p.nxosConfig.VPCKeepalive != nil {
		ka := p.nxosConfig.VPCKeepalive
		p.TopologyVPC.KeepaliveDestination = ka.Destination
		p.TopologyVPC.KeepaliveInterval = ka.Interval
		p.TopologyVPC.KeepaliveVRF = ka.VRF
	}

	// Create a zone for the VPC peer-link
	if vpc.PeerLinkPort != "" {
		zone := models.Zone{
			ID:             uuid.New().String(),
			Name:           fmt.Sprintf("vpc_peer_link_%s", vpc.PeerLinkPort),
			VirtualContext: p.TopologyVirtualContexts[defaultNXOSContextName].Name,
			Description:    fmt.Sprintf("VPC peer-link zone for %s", vpc.PeerLinkPort),
			Type:           "vpc-peer-link",
		}
		p.TopologyZones[zone.Name] = zone
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Mapped VPC topology: domain_id=%d, role=%s, peer_status=%s",
			vpc.DomainID, vpc.VPCRole, vpc.PeerStatus))

	return nil
}

// buildNXOSAddressModel creates address objects from ARP table and operational data
func (p *CiscoNXOSProcessor) buildNXOSAddressModel() error {
	addressMap := make(map[string]models.Address)

	// 1. Extract addresses from ARP table (show ip arp)
	for _, arpEntry := range p.nxosConfig.ARPEntries {
		// Create host address
		addressName := fmt.Sprintf("host_%s", strings.ReplaceAll(arpEntry.IPAddress, ".", "_"))
		if _, exists := addressMap[addressName]; !exists {
			addressMap[addressName] = models.Address{
				ID:             uuid.New().String(),
				Name:           addressName,
				VirtualContext: p.TopologyVirtualContexts[defaultNXOSContextName].Name,
				Value:          fmt.Sprintf("%s/32", arpEntry.IPAddress),
				Type:           "host",
				Description:    fmt.Sprintf("Host from ARP table (MAC: %s, Interface: %s)",
					arpEntry.MACAddress, arpEntry.Interface),
				Interface:      arpEntry.Interface,
			}
		}
	}

	// 2. Extract addresses from interface IPs (if available from config or SVI)
	// Note: This would be populated from config parsing, not operational data
	// For now, we'll use the ARP data as primary source

	// 3. Extract subnet addresses from routing table
	for _, route := range p.nxosConfig.Routes {
		// Skip default routes
		if route.Network == "0.0.0.0" {
			continue
		}

		addressName := fmt.Sprintf("subnet_%s", strings.ReplaceAll(route.Network, ".", "_"))
		if route.Netmask != "" {
			cidr, err := helper.ConvertToCIDR(route.Network, route.Netmask)
			if err == nil {
				addressName = fmt.Sprintf("subnet_%s", strings.ReplaceAll(cidr, "/", "_"))
				if _, exists := addressMap[addressName]; !exists {
					addressMap[addressName] = models.Address{
						ID:             uuid.New().String(),
						Name:           addressName,
						VirtualContext: p.TopologyVirtualContexts[defaultNXOSContextName].Name,
						Value:          cidr,
						Type:           "subnet",
						Description:    fmt.Sprintf("Subnet from routing table via %s", route.NextHop),
					}
				}
			}
		}
	}

	// Convert map to topology addresses
	for _, address := range addressMap {
		p.TopologyAddresses[address.Name] = address
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d address objects from NXOS operational data", len(p.TopologyAddresses)))

	return nil
}

// buildNXOSRoutingModel maps routing entries from operational data (show ip route)
func (p *CiscoNXOSProcessor) buildNXOSRoutingModel() error {
	for _, route := range p.nxosConfig.Routes {
		routeID := uuid.New().String()

		// Convert network and netmask to CIDR
		destination := route.Network
		if route.Netmask != "" {
			cidr, err := helper.ConvertToCIDR(route.Network, route.Netmask)
			if err != nil {
				p.addIntegrationMessage(models.IntegrationMessageTypeWarning,
					fmt.Sprintf("Error converting route %s %s to CIDR: %v", route.Network, route.Netmask, err))
				continue
			}
			destination = cidr
		}

		// Determine route type from NXOS route code
		routeType := "unknown"
		switch route.Type {
		case "C":
			routeType = "connected"
		case "S":
			routeType = "static"
		case "O":
			routeType = "ospf"
		case "B":
			routeType = "bgp"
		case "E":
			routeType = "eigrp"
		case "R":
			routeType = "rip"
		}

		routingEntry := models.RoutingEntry{
			ID:             routeID,
			VirtualContext: p.TopologyVirtualContexts[defaultNXOSContextName].Name,
			Destination:    destination,
			NextHop:        route.NextHop,
			Interface:      route.Interface,
			Type:           routeType,
			AdminDistance:  route.AdminDistance,
			Metric:         route.Metric,
			Name:           fmt.Sprintf("%s_route_%s", routeType, strings.ReplaceAll(destination, "/", "_")),
		}

		p.TopologyRoutingEntries[routeID] = routingEntry
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d routing entries from NXOS operational data", len(p.TopologyRoutingEntries)))

	return nil
}

// buildNXOSZoneModel creates zones from VLANs for segmentation modeling
func (p *CiscoNXOSProcessor) buildNXOSZoneModel() error {
	// Create a zone for each VLAN
	for _, vlan := range p.nxosConfig.VLANOperational {
		zoneName := fmt.Sprintf("vlan_%d_zone", vlan.ID)

		zone := models.Zone{
			ID:             uuid.New().String(),
			Name:           zoneName,
			VirtualContext: p.TopologyVirtualContexts[defaultNXOSContextName].Name,
			Description:    fmt.Sprintf("VLAN %d (%s) zone", vlan.ID, vlan.Name),
			Type:           "layer2",
			Interfaces:     vlan.Ports,
		}

		p.TopologyZones[zoneName] = zone
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d zones from NXOS VLANs", len(p.TopologyZones)))

	return nil
}

// addIntegrationMessage adds an integration message to the processor
func (p *CiscoNXOSProcessor) addIntegrationMessage(msgType models.IntegrationMessageType, message string) {
	p.IntegrationMessages = append(p.IntegrationMessages, models.IntegrationMessage{
		Type:    msgType,
		Message: message,
	})
}

// CiscoNXOSMapper creates the final ParsedNetworkTopology from the processor's topology models
func CiscoNXOSMapper(p *CiscoNXOSProcessor) models.ParsedNetworkTopology {
	return models.ParsedNetworkTopology{
		ID:               p.FileHash,
		VirtualContexts:  helper.SliceFromMap(p.TopologyVirtualContexts),
		Interfaces:       helper.SliceFromMap(p.TopologyInterfaces),
		Zones:            helper.SliceFromMap(p.TopologyZones),
		Addresses:        helper.SliceFromMap(p.TopologyAddresses),
		AddressGroups:    helper.SliceFromMap(p.TopologyAddressGroups),
		Services:         helper.SliceFromMap(p.TopologyServices),
		ServiceGroups:    helper.SliceFromMap(p.TopologyServiceGroups),
		RoutingEntries:   helper.SliceFromMap(p.TopologyRoutingEntries),
		NATs:             helper.SliceFromMap(p.TopologyNATs),
		FirewallPolicies: helper.SliceFromMap(p.TopologyFirewallPolicies),
		Devices:          []models.Device{*p.TopologyDevice},
		FirewallName:     p.nxosConfig.Hostname,
		Metadata: models.L3NodeMetadata{
			Parser:     processor.ParserTypeCiscoNXOS,
			SourceType: constants.SOURCE_TYPE_VENDOR,
		},
		InsertedTime: time.Now().Format(time.RFC3339),
		LastModified: time.Now().Format(time.RFC3339),
		Active:       true,
	}
}

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |L|2| |M|a|p|p|e|r| |F|u|n|c|t|i|o|n|s| |N|X|O|S|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * NXOS Layer-2 topology mapping functions
 * Focus: Switch topology, VPC dual-homing, VLAN segmentation, host placement
 */

// buildNXOSL2SecurityControls maps NXOS L2 security data to models.
// NXOS uses similar security controls to IOS but with different command syntax.
func (p *CiscoNXOSProcessor) buildNXOSL2SecurityControls(
	dhcpBindings []DHCPSnoopingBinding,
	daiStats ARPInspectionStatistics,
	authSessions []AuthenticationSession,
	portSecEntries []PortSecurityEntry,
	ipsgBindings []IPVerifySourceBinding,
	pvlanInfo []PrivateVLANInfo,
) (CiscoGlobalL2Controls, map[string]CiscoL2InterfaceSecurity) {

	// Build global L2 controls
	globalControls := CiscoGlobalL2Controls{
		DHCPSnoopingEnabled: len(dhcpBindings) > 0,
		DAIEnabled:          daiStats.Forwarded > 0 || daiStats.Dropped > 0,
		IPSGEnabled:         len(ipsgBindings) > 0,
		Dot1XEnabled:        len(authSessions) > 0,
		CollectedAt:         time.Now().Format(time.RFC3339),
	}

	// Build per-interface security state (reuse IOS logic)
	interfaceSecurity := make(map[string]CiscoL2InterfaceSecurity)

	// Process DHCP snooping bindings
	dhcpByInterface := make(map[string][]DHCPSnoopingBinding)
	for _, binding := range dhcpBindings {
		dhcpByInterface[binding.Interface] = append(dhcpByInterface[binding.Interface], binding)
	}

	// Process authentication sessions by interface
	authByInterface := make(map[string][]AuthenticationSession)
	for _, session := range authSessions {
		authByInterface[session.Interface] = append(authByInterface[session.Interface], session)
	}

	// Process port security by interface
	portSecByInterface := make(map[string]PortSecurityEntry)
	for _, entry := range portSecEntries {
		portSecByInterface[entry.Interface] = entry
	}

	// Process IPSG bindings by interface
	ipsgByInterface := make(map[string][]IPVerifySourceBinding)
	for _, binding := range ipsgBindings {
		ipsgByInterface[binding.Interface] = append(ipsgByInterface[binding.Interface], binding)
	}

	// Aggregate all interfaces
	allInterfaces := make(map[string]bool)
	for iface := range dhcpByInterface {
		allInterfaces[iface] = true
	}
	for iface := range authByInterface {
		allInterfaces[iface] = true
	}
	for iface := range portSecByInterface {
		allInterfaces[iface] = true
	}
	for iface := range ipsgByInterface {
		allInterfaces[iface] = true
	}

	// Build security state per interface
	for iface := range allInterfaces {
		security := CiscoL2InterfaceSecurity{
			Interface:   iface,
			CollectedAt: time.Now().Format(time.RFC3339),
		}

		// DHCP Snooping
		if bindings, exists := dhcpByInterface[iface]; exists {
			security.DHCPSnoopingBindingCount = len(bindings)
		}

		// Authentication (802.1X/MAB)
		if sessions, exists := authByInterface[iface]; exists && len(sessions) > 0 {
			session := sessions[0]
			security.Dot1XEnabled = true
			security.AuthenticationStatus = session.Status
			security.AuthenticatedMAC = session.MACAddress
			security.AuthenticationMethod = session.Method
			if session.Method == "mab" {
				security.MABEnabled = true
			}
		}

		// Port Security
		if entry, exists := portSecByInterface[iface]; exists {
			security.PortSecurityEnabled = true
			security.PortSecurityMaxMAC = entry.MaximumMAC
			security.PortSecurityCurrentMAC = entry.TotalMAC
			security.PortSecurityViolationMode = entry.SecurityViolation
			security.PortSecurityViolationCount = entry.SecurityViolationCount
		}

		// IP Source Guard
		if bindings, exists := ipsgByInterface[iface]; exists {
			security.IPSGEnabled = true
			security.IPSGBindingCount = len(bindings)
			if len(bindings) > 0 {
				security.IPSGFilterType = bindings[0].Filter
			}
		}

		// Calculate security score
		security.SecurityScore = calculateNXOSInterfaceSecurityScore(security)
		security.SecurityLevel = classifyNXOSSecurityLevel(security.SecurityScore)

		interfaceSecurity[iface] = security
	}

	return globalControls, interfaceSecurity
}

// calculateNXOSInterfaceSecurityScore computes security score for NXOS interfaces.
// Uses same scoring algorithm as IOS for consistency.
func calculateNXOSInterfaceSecurityScore(sec CiscoL2InterfaceSecurity) int {
	score := 0

	if sec.DHCPSnoopingBindingCount > 0 {
		score += 20
	}

	if sec.AuthenticationStatus == "Authc Success" || sec.AuthenticationStatus == "Authorized" {
		score += 30
	} else if sec.Dot1XEnabled || sec.MABEnabled {
		score += 10
	}

	if sec.ARPInspectionTrusted {
		score += 15
	}

	if sec.IPSGEnabled && sec.IPSGBindingCount > 0 {
		score += 15
	}

	if sec.PortSecurityEnabled && sec.PortSecurityViolationCount == 0 {
		score += 10
	}

	if sec.PrivateVLANType == "isolated" || sec.PrivateVLANType == "community" {
		score += 10
	}

	if score > 100 {
		score = 100
	}

	return score
}

// classifyNXOSSecurityLevel maps security score to level.
func classifyNXOSSecurityLevel(score int) string {
	switch {
	case score >= 70:
		return "high"
	case score >= 40:
		return "medium"
	case score >= 20:
		return "low"
	default:
		return "none"
	}
}

// buildNXOSEnhancedFDBEntries creates NXOS FDB entries with confidence scoring.
// NXOS MAC table includes VPC-specific markers (+ for peer-link, * for primary).
func (p *CiscoNXOSProcessor) buildNXOSEnhancedFDBEntries(
	macTable []NXOSMACEntry,
	dhcpBindings []DHCPSnoopingBinding,
	authSessions []AuthenticationSession,
	ipsgBindings []IPVerifySourceBinding,
	arpEntries []NXOSARPEntry,
) []CiscoFDBEntry {

	// Create lookup maps
	dhcpByMAC := make(map[string]DHCPSnoopingBinding)
	for _, binding := range dhcpBindings {
		dhcpByMAC[strings.ToLower(binding.MACAddress)] = binding
	}

	authByMAC := make(map[string]AuthenticationSession)
	for _, session := range authSessions {
		authByMAC[strings.ToLower(session.MACAddress)] = session
	}

	ipsgByInterface := make(map[string][]IPVerifySourceBinding)
	for _, binding := range ipsgBindings {
		ipsgByInterface[binding.Interface] = append(ipsgByInterface[binding.Interface], binding)
	}

	arpByMAC := make(map[string]NXOSARPEntry)
	for _, entry := range arpEntries {
		arpByMAC[strings.ToLower(entry.MACAddress)] = entry
	}

	// Build enhanced FDB entries
	var fdbEntries []CiscoFDBEntry

	for _, mac := range macTable {
		macLower := strings.ToLower(mac.MACAddress)

		fdb := CiscoFDBEntry{
			DeviceID:        p.nxosConfig.Hostname,
			MAC:             mac.MACAddress,
			VLAN:            mac.VLAN,
			Interface:       mac.Port,
			Type:            mac.Type,
			Age:             mac.Age,
			EvidenceSources: []string{"mac-table"},
			LastSeen:        time.Now().Format(time.RFC3339),
			CollectedAt:     time.Now().Format(time.RFC3339),
			Metadata:        make(map[string]string),
		}

		// NXOS-specific: VPC peer-link marker
		if mac.IsVPCPeerLink {
			fdb.Metadata["vpc_peer_link"] = "true"
			fdb.EvidenceSources = append(fdb.EvidenceSources, "vpc-peer-link")
			fdb.ConfidenceScore += 5 // VPC adds minor confidence boost
		}

		// NXOS-specific: Primary entry marker
		if mac.IsPrimary {
			fdb.Metadata["primary_entry"] = "true"
		}

		// Check DHCP snooping binding (PRIMARY evidence)
		if dhcp, exists := dhcpByMAC[macLower]; exists {
			fdb.DHCPSnoopingBound = true
			fdb.IPAddress = dhcp.IPAddress
			fdb.DHCPLeaseTime = dhcp.LeaseSeconds
			fdb.EvidenceSources = append(fdb.EvidenceSources, "dhcp-snooping")
			fdb.ConfidenceScore += 40
		}

		// Check 802.1X/MAB authentication
		if auth, exists := authByMAC[macLower]; exists {
			fdb.Authenticated = true
			fdb.AuthMethod = auth.Method
			if auth.IPv4Address != "" && auth.IPv4Address != "Unknown" {
				fdb.IPAddress = auth.IPv4Address
			}
			fdb.EvidenceSources = append(fdb.EvidenceSources, "dot1x")

			if auth.Status == "Authc Success" || auth.Status == "Authorized" {
				fdb.ConfidenceScore += 35
			} else {
				fdb.ConfidenceScore += 15
			}
		}

		// Check IPSG binding
		if ipsgList, exists := ipsgByInterface[mac.Port]; exists {
			for _, ipsg := range ipsgList {
				if strings.EqualFold(ipsg.MACAddress, mac.MACAddress) {
					fdb.IPSGBound = true
					if fdb.IPAddress == "" {
						fdb.IPAddress = ipsg.IPAddress
					}
					fdb.EvidenceSources = append(fdb.EvidenceSources, "ipsg")
					fdb.ConfidenceScore += 15
					break
				}
			}
		}

		// Check ARP table correlation
		if arp, exists := arpByMAC[macLower]; exists {
			if fdb.IPAddress == "" {
				fdb.IPAddress = arp.IPAddress
			}
			fdb.EvidenceSources = append(fdb.EvidenceSources, "arp")
			fdb.ConfidenceScore += 10
		}

		// Base confidence from MAC table entry type
		if mac.Type == "dynamic" {
			fdb.ConfidenceScore += 5
		} else if mac.Type == "static" {
			fdb.ConfidenceScore += 10
		} else if mac.Type == "secure" {
			fdb.PortSecurityLearned = true
			fdb.EvidenceSources = append(fdb.EvidenceSources, "port-security")
			fdb.ConfidenceScore += 20
		}

		// Classify confidence level
		switch {
		case fdb.ConfidenceScore >= 70:
			fdb.Confidence = "high"
		case fdb.ConfidenceScore >= 40:
			fdb.Confidence = "medium"
		case fdb.ConfidenceScore >= 20:
			fdb.Confidence = "low"
		default:
			fdb.Confidence = "low"
		}

		// Classify freshness
		fdb.Freshness = classifyNXOSFreshnessFromAge(mac.Age)

		fdbEntries = append(fdbEntries, fdb)
	}

	return fdbEntries
}

// classifyNXOSFreshnessFromAge determines freshness tier for NXOS.
func classifyNXOSFreshnessFromAge(ageSeconds int) string {
	switch {
	case ageSeconds <= 600: // 10 minutes
		return "fresh"
	case ageSeconds <= 3600: // 1 hour
		return "stale"
	default:
		return "old"
	}
}

// buildNXOSL2DecisionCache builds L2 decisions for NXOS switches.
// Uses same decision logic as IOS for consistency across Cisco platforms.
func (p *CiscoNXOSProcessor) buildNXOSL2DecisionCache(
	fdbEntries []CiscoFDBEntry,
	interfaceSecurity map[string]CiscoL2InterfaceSecurity,
	ttlSeconds int,
) []L2DecisionCacheEntry {

	var decisions []L2DecisionCacheEntry
	now := time.Now()

	for _, fdb := range fdbEntries {
		ifaceSec, hasSecurityState := interfaceSecurity[fdb.Interface]

		decision := L2DecisionCacheEntry{
			DeviceID:            fdb.DeviceID,
			MAC:                 fdb.MAC,
			IPAddress:           fdb.IPAddress,
			VLAN:                fdb.VLAN,
			Interface:           fdb.Interface,
			EvidenceSources:     fdb.EvidenceSources,
			CachedAt:            now.Format(time.RFC3339),
			ExpiresAt:           now.Add(time.Duration(ttlSeconds) * time.Second).Format(time.RFC3339),
			TTLSeconds:          ttlSeconds,
			Metadata:            fdb.Metadata, // Preserve VPC metadata
		}

		decision.Authenticated = fdb.Authenticated
		decision.DHCPSnoopingBound = fdb.DHCPSnoopingBound
		decision.IPSGValid = fdb.IPSGBound
		decision.PortSecurityValid = fdb.PortSecurityLearned
		decision.ARPInspectionValid = false

		if hasSecurityState {
			decision.SecurityScore = ifaceSec.SecurityScore
		} else {
			decision.SecurityScore = fdb.ConfidenceScore
		}

		decision.DecisionConfidence = fdb.ConfidenceScore

		// Decision logic (same as IOS)
		if decision.Authenticated && decision.DHCPSnoopingBound && fdb.ConfidenceScore >= 70 {
			decision.Decision = "Allowed"
			decision.DecisionReason = "Authenticated (802.1X/MAB) + DHCP snooping bound + high confidence"
		} else if decision.Authenticated && fdb.ConfidenceScore >= 60 {
			decision.Decision = "Conditional"
			decision.DecisionReason = "Authenticated but missing DHCP snooping binding"
		} else if decision.DHCPSnoopingBound && fdb.ConfidenceScore >= 50 {
			decision.Decision = "Conditional"
			decision.DecisionReason = "DHCP snooping bound but not authenticated"
		} else if fdb.ConfidenceScore >= 40 {
			decision.Decision = "Conditional"
			decision.DecisionReason = fmt.Sprintf("Medium confidence (%d) with partial evidence: %v",
				fdb.ConfidenceScore, fdb.EvidenceSources)
		} else {
			decision.Decision = "Blocked"
			decision.DecisionReason = fmt.Sprintf("Low confidence (%d), insufficient evidence",
				fdb.ConfidenceScore)

			if hasSecurityState {
				if ifaceSec.PortSecurityViolationCount > 0 {
					decision.DecisionReason = fmt.Sprintf("Port security violations detected (%d)",
						ifaceSec.PortSecurityViolationCount)
				}
			}
		}

		decisions = append(decisions, decision)
	}

	return decisions
}

// BuildNXOSLayer2Models converts NXOS operational data into L2 switch topology models
func (p *CiscoNXOSProcessor) BuildNXOSLayer2Models() (models.L2Switch, []models.FDBEntry) {
	l2Switch := models.L2Switch{
		DeviceID:     p.nxosConfig.Hostname,
		Vendor:       models.IntegrationCisco.String(),
		OSName:       "nxos",
		OSVersion:    p.nxosConfig.Version,
		ConfigHash:   p.FileHash,
		VLANs:        make([]models.L2VLAN, 0, len(p.nxosConfig.VLANOperational)),
		Interfaces:   make([]models.L2SwitchPort, 0, len(p.nxosConfig.InterfaceStatus)),
		PortChannels: make([]models.L2PortChannel, 0, len(p.nxosConfig.PortChannels)),
	}

	now := time.Now().Format(time.RFC3339)
	l2Switch.InsertedTime = now
	l2Switch.LastModified = now
	l2Switch.Active = true

	// Map VLANs
	for _, vlan := range p.nxosConfig.VLANOperational {
		l2Switch.VLANs = append(l2Switch.VLANs, models.L2VLAN{
			VLANID:   vlan.ID,
			VLANName: vlan.Name,
			State:    vlan.Status,
		})
	}

	// Map Interfaces
	for _, iface := range p.nxosConfig.InterfaceStatus {
		port := models.L2SwitchPort{
			InterfaceName: iface.Port,
			AdminState:    iface.Status,
			Description:   iface.Name,
		}

		// Determine mode from VLAN assignment
		if iface.VLAN == "trunk" {
			port.Mode = "trunk"
		} else {
			port.Mode = "access"
			// Parse access VLAN
			vlanID := 0
			fmt.Sscanf(iface.VLAN, "%d", &vlanID)
			port.AccessVLAN = &vlanID
		}

		l2Switch.Interfaces = append(l2Switch.Interfaces, port)
	}

	// Map Port-Channels
	for _, pc := range p.nxosConfig.PortChannels {
		mode := "static"
		if pc.Protocol == "LACP" {
			mode = "lacp"
		}

		l2Switch.PortChannels = append(l2Switch.PortChannels, models.L2PortChannel{
			PortChannelID:    pc.GroupID,
			Name:             pc.Name,
			MemberInterfaces: pc.Members,
			Mode:             mode,
		})
	}

	// Build FDB entries from MAC address table
	var fdbEntries []models.FDBEntry
	for _, macEntry := range p.nxosConfig.MACTable {
		// Determine evidence source
		evidenceSource := "mac-table"
		if macEntry.IsVPCPeerLink {
			evidenceSource = "vpc-peer-link"
		}

		// Determine confidence based on entry type and age
		confidence := "high"
		if macEntry.Type == "static" {
			confidence = "static"
		} else if macEntry.Age > 300 { // Older than 5 minutes
			confidence = "medium"
		}

		// Calculate freshness
		freshness := "fresh"
		if macEntry.Age > 600 { // Older than 10 minutes
			freshness = "stale"
		} else if macEntry.Age > 3600 { // Older than 1 hour
			freshness = "old"
		}

		fdbEntry := models.FDBEntry{
			DeviceID:       p.nxosConfig.Hostname,
			MAC:            macEntry.MACAddress,
			LastSeenPort:   macEntry.Port,
			VLANID:         macEntry.VLAN,
			LastSeenTS:     macEntry.CollectedAt,
			EvidenceSource: evidenceSource,
			Confidence:     confidence,
			Freshness:      freshness,
		}

		// Add VPC-specific metadata
		if macEntry.IsVPCPeerLink {
			fdbEntry.Metadata = map[string]string{
				"vpc_peer_link": "true",
			}
		}
		if macEntry.IsPrimary {
			fdbEntry.Metadata = map[string]string{
				"primary_entry": "true",
			}
		}

		fdbEntries = append(fdbEntries, fdbEntry)
	}

	return l2Switch, fdbEntries
}

// BuildNXOSVPCTopology builds VPC-specific topology for dual-homed architectures
func (p *CiscoNXOSProcessor) BuildNXOSVPCTopology() *models.VPCTopology {
	if p.nxosConfig.VPCStatus == nil {
		return nil
	}

	vpc := p.nxosConfig.VPCStatus

	vpcTopology := &models.VPCTopology{
		DomainID:         vpc.DomainID,
		Role:             vpc.VPCRole, // "primary" or "secondary"
		PeerStatus:       vpc.PeerStatus,
		KeepaliveStatus:  vpc.KeepaliveStatus,
		PeerLinkPort:     vpc.PeerLinkPort,
		PeerLinkStatus:   vpc.PeerLinkStatus,
		NumberOfVPCs:     vpc.NumberOfVPCs,
		PeerGateway:      vpc.PeerGateway,
		ConfigConsistent: vpc.ConfigConsistencyStatus == "success",
	}

	// Add keepalive information
	if p.nxosConfig.VPCKeepalive != nil {
		ka := p.nxosConfig.VPCKeepalive
		vpcTopology.KeepaliveDestination = ka.Destination
		vpcTopology.KeepaliveInterval = ka.Interval
		vpcTopology.KeepaliveVRF = ka.VRF
		vpcTopology.KeepaliveHealthy = ka.Status == "peer is alive"
	}

	// Add consistency parameters
	vpcTopology.ConsistencyParameters = make(map[string]bool)
	for _, param := range p.nxosConfig.VPCConsistency {
		vpcTopology.ConsistencyParameters[param.Name] = param.Consistent
	}

	return vpcTopology
}

// BuildNXOSNeighborTopology builds neighbor topology from LLDP/CDP data
func (p *CiscoNXOSProcessor) BuildNXOSNeighborTopology() []models.TopologyLink {
	var links []models.TopologyLink

	// Build links from LLDP neighbors
	for _, lldp := range p.nxosConfig.LLDPNeighbors {
		link := models.TopologyLink{
			ID:                  uuid.New().String(),
			LocalDevice:         p.nxosConfig.Hostname,
			LocalInterface:      lldp.LocalPort,
			RemoteDevice:        lldp.SystemName,
			RemoteInterface:     lldp.PortID,
			Protocol:            "lldp",
			RemoteManagementIP:  lldp.ManagementAddress,
			RemoteDescription:   lldp.SystemDescription,
			RemoteCapabilities:  lldp.EnabledCapabilities,
		}
		links = append(links, link)
	}

	// Build links from CDP neighbors (if LLDP not available)
	for _, cdp := range p.nxosConfig.CDPNeighbors {
		link := models.TopologyLink{
			ID:                 uuid.New().String(),
			LocalDevice:        p.nxosConfig.Hostname,
			LocalInterface:     cdp.LocalInterface,
			RemoteDevice:       cdp.DeviceID,
			RemoteInterface:    cdp.RemotePort,
			Protocol:           "cdp",
			RemoteManagementIP: cdp.IPAddress,
			RemotePlatform:     cdp.Platform,
		}
		links = append(links, link)
	}

	return links
}
