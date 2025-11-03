package fortinet

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
	"github.com/mattpainter701/graphlayer/platform/core/constants"
	helper "github.com/mattpainter701/graphlayer/platform/core/helpers"
	"github.com/mattpainter701/graphlayer/platform/core/processor"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |F|O|R|T|I|G|A|T|E| |M|A|P|P|E|R|S|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * FortiGate mappers transform FortiOS configuration to vendor-agnostic topology models
 *
 * Key mapping challenges:
 * 1. VDOM → VirtualContext: Map FortiOS Virtual Domains to common model
 * 2. Central SNAT: FortiGate has separate Central SNAT table (unique architecture)
 * 3. VIP → VIP + NAT: Map FortiGate VIPs (DNAT) to both VIP and NAT models
 * 4. Policy NAT: Inline NAT within firewall policies needs mapping to separate NAT rules
 * 5. SD-WAN: Map SD-WAN members to interfaces with routing policies
 * 6. IPv4 vs IPv6: Separate policy tables need unified mapping
 */

// FortiGateProcessor is the processor for FortiGate configuration
type FortiGateProcessor struct {
	// Parsed Configuration (from parsers)
	systemStatus     *FortiGateSystemStatus
	vdoms            []FortiGateVDOM
	interfaces       []FortiGateInterface
	zones            []FortiGateZone
	staticRoutes     []FortiGateStaticRoute
	bgpConfig        *FortiGateBGPConfig
	ipv4Policies     []FortiGateFirewallPolicy
	ipv6Policies     []FortiGateIPv6Policy
	centralSNAT      []FortiGateCentralSNAT
	vips             []FortiGateVIP
	ipPools          []FortiGateIPPool
	addresses        []FortiGateAddress
	addressGroups    []FortiGateAddressGroup
	services         []FortiGateService
	serviceGroups    []FortiGateServiceGroup
	sdwanConfig      *FortiGateSDWAN
	ipsecPhase1      []FortiGateIPsecPhase1
	ipsecPhase2      []FortiGateIPsecPhase2

	// Operational Status
	interfaceStatus  []FortiGateInterfaceStatus
	routingTable     []FortiGateRoute
	bgpNeighbors     []FortiGateBGPNeighborStatus
	vpnStatus        []FortiGateVPNStatus
	sdwanHealth      []FortiGateSDWANHealthCheckStatus

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
	TopologyVPNTunnels      map[string]models.VPNTunnel
	TopologyBGPSessions     map[string]models.BGPSession

	// Metadata
	FileHash            string
	IntegrationMessages []models.IntegrationMessage
}

// NewFortiGateProcessor creates a new FortiGate processor
func NewFortiGateProcessor(systemStatus *FortiGateSystemStatus) *FortiGateProcessor {
	return &FortiGateProcessor{
		systemStatus:            systemStatus,
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
		TopologyVPNTunnels:      make(map[string]models.VPNTunnel),
		TopologyBGPSessions:     make(map[string]models.BGPSession),
	}
}

// BuildNetworkTopologyModels maps FortiGate configuration to standard network topology models
func (p *FortiGateProcessor) BuildNetworkTopologyModels() error {
	// Initialize the topology model maps
	p.TopologyDevice = &models.Device{}

	// Build the Device model
	err := p.buildFortiGateDeviceModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate device model")
		return err
	}

	// Build Virtual Contexts (VDOMs)
	err = p.buildFortiGateVirtualContextModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate virtual context model")
		return err
	}

	// Build Zones
	err = p.buildFortiGateZoneModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate zone model")
		return err
	}

	// Build Interfaces
	err = p.buildFortiGateInterfaceModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate interface model")
		return err
	}

	// Build Address objects
	err = p.buildFortiGateAddressModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate address model")
		return err
	}

	// Build Service objects
	err = p.buildFortiGateServiceModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate service model")
		return err
	}

	// Build Security Policies (IPv4 + IPv6)
	err = p.buildFortiGateSecurityPolicyModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate security policy model")
		return err
	}

	// Build NAT (Central SNAT, Policy NAT, VIPs)
	err = p.buildFortiGateNATModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate NAT model")
		return err
	}

	// Build VIP models (DNAT)
	err = p.buildFortiGateVIPModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate VIP model")
		return err
	}

	// Build Routing
	err = p.buildFortiGateRoutingModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate routing model")
		return err
	}

	// Build BGP Sessions
	err = p.buildFortiGateBGPModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate BGP model")
		return err
	}

	// Build VPN Tunnels
	err = p.buildFortiGateVPNModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate VPN model")
		return err
	}

	// Build SD-WAN (maps to interfaces + routing)
	err = p.buildFortiGateSDWANModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building FortiGate SD-WAN model")
		return err
	}

	return nil
}

// ========================================
// Device Mapper
// ========================================

// buildFortiGateDeviceModel maps FortiGate system information to Device model
func (p *FortiGateProcessor) buildFortiGateDeviceModel() error {
	if p.systemStatus == nil {
		return fmt.Errorf("system status is nil")
	}

	deviceID := uuid.New().String()

	p.TopologyDevice = &models.Device{
		ID:           deviceID,
		Hostname:     p.systemStatus.Hostname,
		Vendor:       constants.VendorFortinet,
		Model:        p.systemStatus.Model,
		Version:      p.systemStatus.Version,
		SerialNumber: p.systemStatus.SerialNumber,
		DeviceType:   constants.DeviceTypeFirewall,
		CollectionTime: time.Now(),
		HARole:       p.mapHAMode(p.systemStatus.HAMode),
		HAGroupID:    fmt.Sprintf("%d", p.systemStatus.HAGroupID),
		HAGroupName:  p.systemStatus.HAGroupName,
		Metadata: map[string]interface{}{
			"vdom_mode":      p.systemStatus.VDOMMode,
			"operation_mode": p.systemStatus.OperationMode,
			"build":          p.systemStatus.Build,
			"admin_users":    p.systemStatus.AdminUsers,
		},
	}

	return nil
}

// mapHAMode maps FortiGate HA mode to standard HA role
func (p *FortiGateProcessor) mapHAMode(haMode string) string {
	switch strings.ToLower(haMode) {
	case "standalone":
		return constants.HAModeStandalone
	case "active-passive", "a-p":
		return constants.HAModeActiveStandby
	case "active-active", "a-a":
		return constants.HAModeActiveActive
	default:
		return constants.HAModeStandalone
	}
}

// ========================================
// Virtual Context Mapper (VDOMs)
// ========================================

// buildFortiGateVirtualContextModel maps FortiGate VDOMs to VirtualContext models
func (p *FortiGateProcessor) buildFortiGateVirtualContextModel() error {
	// If no VDOMs, create default "root" virtual context
	if len(p.vdoms) == 0 {
		vctxID := uuid.New().String()
		p.TopologyVirtualContexts["root"] = models.VirtualContext{
			ID:          vctxID,
			DeviceID:    p.TopologyDevice.ID,
			Name:        "root",
			Description: "Default VDOM (single VDOM mode)",
			VRFName:     "root",
			Enabled:     true,
			IsDefault:   true,
		}
		return nil
	}

	// Map each VDOM to VirtualContext
	for _, vdom := range p.vdoms {
		vctxID := uuid.New().String()

		p.TopologyVirtualContexts[vdom.Name] = models.VirtualContext{
			ID:          vctxID,
			DeviceID:    p.TopologyDevice.ID,
			Name:        vdom.Name,
			Description: vdom.Comment,
			VRFName:     vdom.Name,
			Enabled:     vdom.Enabled == "enable",
			IsDefault:   vdom.Name == "root",
			Metadata: map[string]interface{}{
				"vcluster_id": vdom.VClusterID,
			},
		}
	}

	return nil
}

// ========================================
// Zone Mapper
// ========================================

// buildFortiGateZoneModel maps FortiGate zones to Zone models
func (p *FortiGateProcessor) buildFortiGateZoneModel() error {
	for _, zone := range p.zones {
		zoneID := uuid.New().String()

		// Get virtual context ID
		vctxID := p.getVirtualContextID(zone.VDOM)

		p.TopologyZones[zone.Name] = models.Zone{
			ID:               zoneID,
			DeviceID:         p.TopologyDevice.ID,
			VirtualContextID: vctxID,
			Name:             zone.Name,
			Description:      zone.Description,
			Interfaces:       zone.Interfaces,
			SecurityLevel:    0, // FortiGate doesn't have security levels like ASA
			Metadata: map[string]interface{}{
				"intrazone":      zone.Intrazone,
				"vdom":           zone.VDOM,
				"interface_count": len(zone.Interfaces),
			},
		}
	}

	return nil
}

// ========================================
// Interface Mapper
// ========================================

// buildFortiGateInterfaceModel maps FortiGate interfaces to Interface models
func (p *FortiGateProcessor) buildFortiGateInterfaceModel() error {
	for _, iface := range p.interfaces {
		interfaceID := uuid.New().String()

		// Get virtual context ID
		vctxID := p.getVirtualContextID(iface.VDOM)

		// Parse IP/subnet
		var ipAddress, cidr string
		var ipNet *net.IPNet
		if iface.IP != "" && iface.Netmask != "" {
			ipAddress = iface.IP
			cidr = fmt.Sprintf("%s/%s", iface.IP, p.convertNetmaskToCIDR(iface.Netmask))
			_, ipNet, _ = net.ParseCIDR(cidr)
		}

		// Get operational status
		adminStatus := "up"
		if iface.Status == "down" {
			adminStatus = "down"
		}

		operStatus := p.getInterfaceOperationalStatus(iface.Name)

		// Determine interface type
		interfaceType := p.mapInterfaceType(iface.Type)

		p.TopologyInterfaces[iface.Name] = models.Interface{
			ID:               interfaceID,
			DeviceID:         p.TopologyDevice.ID,
			VirtualContextID: vctxID,
			Name:             iface.Name,
			Description:      iface.Description,
			Type:             interfaceType,
			IPAddress:        ipAddress,
			Subnet:           cidr,
			IPNetwork:        ipNet,
			AdminStatus:      adminStatus,
			OperStatus:       operStatus,
			VlanID:           iface.VlanID,
			ParentInterface:  iface.Interface, // Parent for VLANs
			Members:          iface.Members,    // For aggregates
			Speed:            p.parseSpeed(iface.Speed),
			Duplex:           iface.Duplex,
			MTU:              iface.MTU,
			Metadata: map[string]interface{}{
				"vdom":         iface.VDOM,
				"type":         iface.Type,
				"alias":        iface.Alias,
				"role":         iface.Role,
				"mode":         iface.Mode,
				"allow_access": iface.AllowAccess,
				"mtu_override": iface.MTUOverride,
			},
		}
	}

	return nil
}

// mapInterfaceType maps FortiGate interface type to standard interface type
func (p *FortiGateProcessor) mapInterfaceType(ifaceType string) string {
	switch strings.ToLower(ifaceType) {
	case "physical":
		return constants.InterfaceTypePhysical
	case "vlan":
		return constants.InterfaceTypeVLAN
	case "loopback":
		return constants.InterfaceTypeLoopback
	case "tunnel":
		return constants.InterfaceTypeTunnel
	case "aggregate":
		return constants.InterfaceTypeAggregate
	default:
		return constants.InterfaceTypeOther
	}
}

// getInterfaceOperationalStatus gets operational status from operational data
func (p *FortiGateProcessor) getInterfaceOperationalStatus(interfaceName string) string {
	for _, status := range p.interfaceStatus {
		if status.Name == interfaceName {
			if status.Link == "up" && status.Status == "up" {
				return "up"
			}
			return "down"
		}
	}
	return "unknown"
}

// parseSpeed converts speed string to integer (Mbps)
func (p *FortiGateProcessor) parseSpeed(speedStr string) int {
	// Parse strings like "1000" or "10000"
	speed := 0
	fmt.Sscanf(speedStr, "%d", &speed)
	return speed
}

// convertNetmaskToCIDR converts dotted decimal netmask to CIDR prefix
func (p *FortiGateProcessor) convertNetmaskToCIDR(netmask string) string {
	ip := net.ParseIP(netmask)
	if ip == nil {
		return "32"
	}

	mask := net.IPMask(ip.To4())
	ones, _ := mask.Size()
	return fmt.Sprintf("%d", ones)
}

// ========================================
// Address Mapper
// ========================================

// buildFortiGateAddressModel maps FortiGate address objects to Address models
func (p *FortiGateProcessor) buildFortiGateAddressModel() error {
	// Map address objects
	for _, addr := range p.addresses {
		addressID := uuid.New().String()
		vctxID := p.getVirtualContextID(addr.VDOM)

		// Determine address type and value
		addressType, addressValue := p.mapAddressType(addr)

		p.TopologyAddresses[addr.Name] = models.Address{
			ID:               addressID,
			DeviceID:         p.TopologyDevice.ID,
			VirtualContextID: vctxID,
			Name:             addr.Name,
			Type:             addressType,
			Value:            addressValue,
			Description:      addr.Comment,
			Metadata: map[string]interface{}{
				"vdom":                addr.VDOM,
				"uuid":                addr.UUID,
				"original_type":       addr.Type,
				"associated_interface": addr.Associated,
			},
		}
	}

	// Map address groups
	for _, group := range p.addressGroups {
		groupID := uuid.New().String()
		vctxID := p.getVirtualContextID(group.VDOM)

		p.TopologyAddressGroups[group.Name] = models.AddressGroup{
			ID:               groupID,
			DeviceID:         p.TopologyDevice.ID,
			VirtualContextID: vctxID,
			Name:             group.Name,
			Members:          group.Members,
			Description:      group.Comment,
			Metadata: map[string]interface{}{
				"vdom": group.VDOM,
				"uuid": group.UUID,
			},
		}
	}

	return nil
}

// mapAddressType maps FortiGate address type to standard address type
func (p *FortiGateProcessor) mapAddressType(addr FortiGateAddress) (string, string) {
	switch addr.Type {
	case "ipmask":
		// Subnet in CIDR notation
		return constants.AddressTypeNetwork, addr.Subnet
	case "iprange":
		// IP range
		return constants.AddressTypeRange, fmt.Sprintf("%s-%s", addr.StartIP, addr.EndIP)
	case "fqdn":
		// FQDN
		return constants.AddressTypeFQDN, addr.FQDN
	case "geography":
		// Geographic location (country)
		return constants.AddressTypeGeography, addr.Country
	case "wildcard":
		// Wildcard address
		return constants.AddressTypeWildcard, addr.Wildcard
	default:
		return constants.AddressTypeHost, addr.Subnet
	}
}

// ========================================
// Service Mapper
// ========================================

// buildFortiGateServiceModel maps FortiGate service objects to Service models
func (p *FortiGateProcessor) buildFortiGateServiceModel() error {
	// Map service objects
	for _, svc := range p.services {
		serviceID := uuid.New().String()
		vctxID := p.getVirtualContextID(svc.VDOM)

		// Parse port ranges
		tcpPorts := p.parsePortRanges(svc.TCPPortRange)
		udpPorts := p.parsePortRanges(svc.UDPPortRange)
		sctpPorts := p.parsePortRanges(svc.SCTPPortRange)

		// Determine service protocol
		protocol := p.mapServiceProtocol(svc.Protocol)

		p.TopologyServices[svc.Name] = models.Service{
			ID:               serviceID,
			DeviceID:         p.TopologyDevice.ID,
			VirtualContextID: vctxID,
			Name:             svc.Name,
			Protocol:         protocol,
			DestinationPorts: p.combinePortRanges(tcpPorts, udpPorts, sctpPorts),
			Description:      svc.Comment,
			Metadata: map[string]interface{}{
				"vdom":            svc.VDOM,
				"uuid":            svc.UUID,
				"category":        svc.Category,
				"tcp_ports":       tcpPorts,
				"udp_ports":       udpPorts,
				"sctp_ports":      sctpPorts,
				"icmp_type":       svc.ICMPType,
				"icmp_code":       svc.ICMPCode,
				"protocol_number": svc.Protocol_Number,
			},
		}
	}

	// Map service groups
	for _, group := range p.serviceGroups {
		groupID := uuid.New().String()
		vctxID := p.getVirtualContextID(group.VDOM)

		p.TopologyServiceGroups[group.Name] = models.ServiceGroup{
			ID:               groupID,
			DeviceID:         p.TopologyDevice.ID,
			VirtualContextID: vctxID,
			Name:             group.Name,
			Members:          group.Members,
			Description:      group.Comment,
			Metadata: map[string]interface{}{
				"vdom": group.VDOM,
				"uuid": group.UUID,
			},
		}
	}

	return nil
}

// mapServiceProtocol maps FortiGate service protocol to standard protocol
func (p *FortiGateProcessor) mapServiceProtocol(protocol string) string {
	protocol = strings.ToUpper(protocol)
	switch {
	case strings.Contains(protocol, "TCP"):
		return constants.ProtocolTCP
	case strings.Contains(protocol, "UDP"):
		return constants.ProtocolUDP
	case strings.Contains(protocol, "ICMP6"):
		return constants.ProtocolICMPv6
	case strings.Contains(protocol, "ICMP"):
		return constants.ProtocolICMP
	case strings.Contains(protocol, "SCTP"):
		return "SCTP"
	case protocol == "IP":
		return constants.ProtocolIP
	default:
		return protocol
	}
}

// parsePortRanges parses port range strings like "0-65535:80" into structured format
func (p *FortiGateProcessor) parsePortRanges(portRanges []string) []string {
	if len(portRanges) == 0 {
		return nil
	}

	parsed := []string{}
	for _, portRange := range portRanges {
		// Format is "src:dst", we care about dst
		parts := strings.Split(portRange, ":")
		if len(parts) == 2 {
			parsed = append(parsed, parts[1])
		}
	}
	return parsed
}

// combinePortRanges combines TCP, UDP, SCTP port ranges into single list
func (p *FortiGateProcessor) combinePortRanges(tcp, udp, sctp []string) []string {
	combined := []string{}
	combined = append(combined, tcp...)
	combined = append(combined, udp...)
	combined = append(combined, sctp...)
	return combined
}

// ========================================
// Security Policy Mapper
// ========================================

// buildFortiGateSecurityPolicyModel maps FortiGate firewall policies to FirewallPolicy models
func (p *FortiGateProcessor) buildFortiGateSecurityPolicyModel() error {
	// Map IPv4 policies
	for _, policy := range p.ipv4Policies {
		policyID := uuid.New().String()
		vctxID := p.getVirtualContextID(policy.VDOM)

		// Map action
		action := p.mapPolicyAction(policy.Action)

		// Map logging
		logging := p.mapLogging(policy.LogTraffic)

		p.TopologyFirewallPolicies[fmt.Sprintf("ipv4-%d", policy.PolicyID)] = models.FirewallPolicy{
			ID:                policyID,
			DeviceID:          p.TopologyDevice.ID,
			VirtualContextID:  vctxID,
			Name:              policy.Name,
			RuleNumber:        policy.PolicyID,
			Enabled:           policy.Status == "enable",
			Action:            action,
			SourceZones:       policy.SourceIntf,
			DestinationZones:  policy.DestIntf,
			SourceAddresses:   policy.SourceAddr,
			DestAddresses:     policy.DestAddr,
			Services:          policy.Service,
			Logging:           logging,
			Schedule:          policy.Schedule,
			Comments:          policy.Comments,
			Metadata: map[string]interface{}{
				"vdom":               policy.VDOM,
				"uuid":               policy.UUID,
				"nat_enabled":        policy.NATEnabled == "enable",
				"nat_type":           policy.NATType,
				"poolname":           policy.Poolname,
				"av_profile":         policy.AVProfile,
				"webfilter_profile":  policy.WebfilterProfile,
				"ips_profile":        policy.IPSProfile,
				"application_list":   policy.ApplicationList,
				"voip_profile":       policy.VoIPProfile,
				"dnsfilter_profile":  policy.DNSFilterProfile,
				"sshfilter_profile":  policy.SSHFilterProfile,
			},
		}
	}

	// Map IPv6 policies
	for _, policy := range p.ipv6Policies {
		policyID := uuid.New().String()
		vctxID := p.getVirtualContextID(policy.VDOM)

		action := p.mapPolicyAction(policy.Action)
		logging := p.mapLogging(policy.LogTraffic)

		p.TopologyFirewallPolicies[fmt.Sprintf("ipv6-%d", policy.PolicyID)] = models.FirewallPolicy{
			ID:                policyID,
			DeviceID:          p.TopologyDevice.ID,
			VirtualContextID:  vctxID,
			Name:              policy.Name,
			RuleNumber:        policy.PolicyID,
			Enabled:           policy.Status == "enable",
			Action:            action,
			SourceZones:       policy.SourceIntf,
			DestinationZones:  policy.DestIntf,
			SourceAddresses:   policy.SourceAddr,
			DestAddresses:     policy.DestAddr,
			Services:          policy.Service,
			Logging:           logging,
			Schedule:          policy.Schedule,
			Comments:          policy.Comments,
			Metadata: map[string]interface{}{
				"vdom":       policy.VDOM,
				"uuid":       policy.UUID,
				"ip_version": "ipv6",
			},
		}
	}

	return nil
}

// mapPolicyAction maps FortiGate policy action to standard action
func (p *FortiGateProcessor) mapPolicyAction(action string) string {
	switch strings.ToLower(action) {
	case "accept":
		return constants.PolicyActionAllow
	case "deny":
		return constants.PolicyActionDeny
	case "ipsec":
		return constants.PolicyActionVPN
	default:
		return action
	}
}

// mapLogging maps FortiGate log traffic setting to boolean
func (p *FortiGateProcessor) mapLogging(logTraffic string) bool {
	return logTraffic == "all" || logTraffic == "utm"
}

// ========================================
// NAT Mapper
// ========================================

// buildFortiGateNATModel maps FortiGate NAT (Central SNAT + Policy NAT) to NAT models
func (p *FortiGateProcessor) buildFortiGateNATModel() error {
	// Map Central SNAT policies
	for _, snat := range p.centralSNAT {
		natID := uuid.New().String()
		vctxID := p.getVirtualContextID(snat.VDOM)

		p.TopologyNATs[fmt.Sprintf("snat-%d", snat.PolicyID)] = models.NAT{
			ID:                natID,
			DeviceID:          p.TopologyDevice.ID,
			VirtualContextID:  vctxID,
			RuleNumber:        snat.PolicyID,
			Type:              constants.NATTypeSource,
			Enabled:           snat.Status == "enable",
			SourceZones:       snat.SourceIntf,
			DestinationZones:  snat.DestIntf,
			OriginalAddresses: snat.OrigAddr,
			TranslatedAddresses: snat.Poolname,
			Comments:          snat.Comments,
			Metadata: map[string]interface{}{
				"vdom":        snat.VDOM,
				"uuid":        snat.UUID,
				"nat_type":    snat.NATType,
				"nat_ip_pool": snat.NATIPPool,
			},
		}
	}

	// Map Policy NAT (inline NAT within firewall policies)
	for _, policy := range p.ipv4Policies {
		if policy.NATEnabled == "enable" {
			natID := uuid.New().String()
			vctxID := p.getVirtualContextID(policy.VDOM)

			p.TopologyNATs[fmt.Sprintf("policy-nat-%d", policy.PolicyID)] = models.NAT{
				ID:                  natID,
				DeviceID:            p.TopologyDevice.ID,
				VirtualContextID:    vctxID,
				RuleNumber:          policy.PolicyID,
				Type:                constants.NATTypeSource,
				Enabled:             policy.Status == "enable",
				SourceZones:         policy.SourceIntf,
				DestinationZones:    policy.DestIntf,
				OriginalAddresses:   policy.SourceAddr,
				TranslatedAddresses: policy.Poolname,
				Comments:            fmt.Sprintf("Policy NAT from firewall policy: %s", policy.Name),
				Metadata: map[string]interface{}{
					"vdom":            policy.VDOM,
					"policy_id":       policy.PolicyID,
					"policy_name":     policy.Name,
					"nat_type":        policy.NATType,
					"source_type":     "firewall_policy",
				},
			}
		}
	}

	return nil
}

// ========================================
// VIP Mapper (DNAT)
// ========================================

// buildFortiGateVIPModel maps FortiGate VIPs to VIP models
func (p *FortiGateProcessor) buildFortiGateVIPModel() error {
	for _, vip := range p.vips {
		vipID := uuid.New().String()
		vctxID := p.getVirtualContextID(vip.VDOM)

		// Parse external and mapped IPs
		externalIP := vip.ExtIP
		mappedIPs := vip.MappedIP

		// Parse ports
		externalPort := vip.ExtPort
		mappedPort := vip.MappedPort

		p.TopologyVIPs[vip.Name] = models.VIP{
			ID:               vipID,
			DeviceID:         p.TopologyDevice.ID,
			VirtualContextID: vctxID,
			Name:             vip.Name,
			Type:             p.mapVIPType(vip.Type),
			ExternalIP:       externalIP,
			ExternalInterface: vip.ExtIntf,
			MappedIPs:        mappedIPs,
			Protocol:         p.mapServiceProtocol(vip.Protocol),
			ExternalPort:     externalPort,
			MappedPort:       mappedPort,
			Comments:         vip.Comment,
			Metadata: map[string]interface{}{
				"vdom": vip.VDOM,
				"uuid": vip.UUID,
				"type": vip.Type,
			},
		}

		// Also create a NAT entry for the VIP (DNAT)
		natID := uuid.New().String()
		p.TopologyNATs[fmt.Sprintf("vip-%s", vip.Name)] = models.NAT{
			ID:                  natID,
			DeviceID:            p.TopologyDevice.ID,
			VirtualContextID:    vctxID,
			Type:                constants.NATTypeDestination,
			Enabled:             true,
			OriginalAddresses:   []string{externalIP},
			TranslatedAddresses: mappedIPs,
			Comments:            fmt.Sprintf("DNAT from VIP: %s", vip.Name),
			Metadata: map[string]interface{}{
				"vdom":          vip.VDOM,
				"vip_name":      vip.Name,
				"source_type":   "vip",
				"external_port": externalPort,
				"mapped_port":   mappedPort,
			},
		}
	}

	return nil
}

// mapVIPType maps FortiGate VIP type to standard VIP type
func (p *FortiGateProcessor) mapVIPType(vipType string) string {
	switch vipType {
	case "static-nat":
		return constants.VIPTypeStaticNAT
	case "server-load-balance":
		return constants.VIPTypeLoadBalance
	case "dns-translation":
		return constants.VIPTypeDNS
	default:
		return vipType
	}
}

// ========================================
// Routing Mapper
// ========================================

// buildFortiGateRoutingModel maps FortiGate routes to RoutingEntry models
func (p *FortiGateProcessor) buildFortiGateRoutingModel() error {
	// Map static routes
	for _, route := range p.staticRoutes {
		routeID := uuid.New().String()
		vctxID := p.getVirtualContextID(route.VDOM)

		_, destNet, _ := net.ParseCIDR(route.Destination)

		p.TopologyRoutingEntries[fmt.Sprintf("static-%d", route.SeqNum)] = models.RoutingEntry{
			ID:               routeID,
			DeviceID:         p.TopologyDevice.ID,
			VirtualContextID: vctxID,
			Protocol:         constants.RoutingProtocolStatic,
			DestinationNetwork: destNet,
			NextHop:          route.Gateway,
			Interface:        route.Device,
			AdminDistance:    route.Distance,
			Metric:           route.Weight,
			Enabled:          route.Status == "enable",
			Comments:         route.Comment,
			Metadata: map[string]interface{}{
				"vdom":     route.VDOM,
				"seq_num":  route.SeqNum,
				"priority": route.Priority,
			},
		}
	}

	// Map operational routing table
	for _, route := range p.routingTable {
		routeID := uuid.New().String()
		vctxID := p.getVirtualContextID(route.VDOM)

		_, destNet, _ := net.ParseCIDR(route.Destination)

		protocol := p.mapRoutingProtocol(route.Type)

		p.TopologyRoutingEntries[fmt.Sprintf("%s-%s", route.Type, route.Destination)] = models.RoutingEntry{
			ID:                 routeID,
			DeviceID:           p.TopologyDevice.ID,
			VirtualContextID:   vctxID,
			Protocol:           protocol,
			DestinationNetwork: destNet,
			NextHop:            route.Gateway,
			Interface:          route.Interface,
			AdminDistance:      route.Distance,
			Metric:             route.Metric,
			Age:                route.Age,
			Enabled:            true,
			Metadata: map[string]interface{}{
				"vdom":   route.VDOM,
				"source": "routing_table",
			},
		}
	}

	return nil
}

// mapRoutingProtocol maps FortiGate routing protocol to standard protocol
func (p *FortiGateProcessor) mapRoutingProtocol(protocol string) string {
	switch strings.ToLower(protocol) {
	case "connected":
		return constants.RoutingProtocolConnected
	case "static":
		return constants.RoutingProtocolStatic
	case "rip":
		return constants.RoutingProtocolRIP
	case "ospf":
		return constants.RoutingProtocolOSPF
	case "bgp":
		return constants.RoutingProtocolBGP
	case "isis":
		return constants.RoutingProtocolISIS
	default:
		return protocol
	}
}

// ========================================
// BGP Mapper
// ========================================

// buildFortiGateBGPModel maps FortiGate BGP configuration to BGPSession models
func (p *FortiGateProcessor) buildFortiGateBGPModel() error {
	if p.bgpConfig == nil {
		return nil
	}

	// Map BGP neighbors from configuration
	for _, neighbor := range p.bgpConfig.Neighbors {
		sessionID := uuid.New().String()
		vctxID := p.getVirtualContextID(p.bgpConfig.VDOM)

		// Get operational status if available
		state := "configured"
		prefixReceived := 0
		prefixAccepted := 0
		uptime := 0

		for _, operNeighbor := range p.bgpNeighbors {
			if operNeighbor.NeighborIP == neighbor.IP {
				state = operNeighbor.State
				prefixReceived = operNeighbor.PrefixReceived
				prefixAccepted = operNeighbor.PrefixAccepted
				uptime = operNeighbor.UptimeSeconds
				break
			}
		}

		p.TopologyBGPSessions[neighbor.IP] = models.BGPSession{
			ID:               sessionID,
			DeviceID:         p.TopologyDevice.ID,
			VirtualContextID: vctxID,
			LocalAS:          p.bgpConfig.AS,
			LocalRouterID:    p.bgpConfig.RouterID,
			PeerIP:           neighbor.IP,
			PeerAS:           neighbor.RemoteAS,
			State:            state,
			UptimeSeconds:    uptime,
			PrefixReceived:   prefixReceived,
			PrefixAccepted:   prefixAccepted,
			Description:      neighbor.Description,
			Metadata: map[string]interface{}{
				"vdom":                     p.bgpConfig.VDOM,
				"update_source":            neighbor.UpdateSource,
				"next_hop_self":            neighbor.NextHopSelf == "enable",
				"route_map_in":             neighbor.RouteMapIn,
				"route_map_out":            neighbor.RouteMapOut,
				"prefix_list_in":           neighbor.PrefixListIn,
				"prefix_list_out":          neighbor.PrefixListOut,
				"maximum_prefix":           neighbor.MaximumPrefix,
				"maximum_prefix_threshold": neighbor.MaximumPrefixThreshold,
			},
		}
	}

	return nil
}

// ========================================
// VPN Mapper
// ========================================

// buildFortiGateVPNModel maps FortiGate IPsec VPN to VPNTunnel models
func (p *FortiGateProcessor) buildFortiGateVPNModel() error {
	// Map Phase 1 (IKE) configuration
	for _, phase1 := range p.ipsecPhase1 {
		tunnelID := uuid.New().String()
		vctxID := p.getVirtualContextID(phase1.VDOM)

		// Get operational status if available
		status := "configured"
		established := time.Time{}
		txBytes := int64(0)
		rxBytes := int64(0)

		for _, vpnStatus := range p.vpnStatus {
			if vpnStatus.Name == phase1.Name {
				status = vpnStatus.Status
				established = vpnStatus.Established
				txBytes = vpnStatus.TxBytes
				rxBytes = vpnStatus.RxBytes
				break
			}
		}

		// Find corresponding Phase 2 configuration
		phase2List := p.findPhase2ForPhase1(phase1.Name)

		p.TopologyVPNTunnels[phase1.Name] = models.VPNTunnel{
			ID:               tunnelID,
			DeviceID:         p.TopologyDevice.ID,
			VirtualContextID: vctxID,
			Name:             phase1.Name,
			Type:             p.mapVPNType(phase1.Type),
			LocalGateway:     phase1.LocalGW,
			RemoteGateway:    phase1.RemoteGW,
			Status:           status,
			Established:      established,
			TxBytes:          txBytes,
			RxBytes:          rxBytes,
			IKEVersion:       phase1.IKEVersion,
			AuthMethod:       phase1.AuthMethod,
			Interface:        phase1.Interface,
			Enabled:          true,
			Comments:         phase1.Comments,
			Metadata: map[string]interface{}{
				"vdom":              phase1.VDOM,
				"peer_type":         phase1.PeerType,
				"proposals":         phase1.Proposal,
				"dh_groups":         phase1.DHGroup,
				"mode":              phase1.Mode,
				"nat_traversal":     phase1.NATTraversal,
				"dpd":               phase1.DPD,
				"dpd_retry_count":   phase1.DPDRetryCount,
				"dpd_retry_interval": phase1.DPDRetryInterval,
				"net_device":        phase1.NetDevice == "enable",
				"add_route":         phase1.AddRoute == "enable",
				"phase2_count":      len(phase2List),
			},
		}
	}

	return nil
}

// mapVPNType maps FortiGate VPN type to standard VPN type
func (p *FortiGateProcessor) mapVPNType(vpnType string) string {
	switch strings.ToLower(vpnType) {
	case "static":
		return constants.VPNTypeSiteToSite
	case "dynamic", "dialup":
		return constants.VPNTypeRemoteAccess
	case "ddns":
		return constants.VPNTypeDynamicDNS
	default:
		return vpnType
	}
}

// findPhase2ForPhase1 finds all Phase 2 configurations for a given Phase 1
func (p *FortiGateProcessor) findPhase2ForPhase1(phase1Name string) []FortiGateIPsecPhase2 {
	result := []FortiGateIPsecPhase2{}
	for _, phase2 := range p.ipsecPhase2 {
		if phase2.Phase1Name == phase1Name {
			result = append(result, phase2)
		}
	}
	return result
}

// ========================================
// SD-WAN Mapper
// ========================================

// buildFortiGateSDWANModel maps FortiGate SD-WAN to interface and routing models
func (p *FortiGateProcessor) buildFortiGateSDWANModel() error {
	if p.sdwanConfig == nil || p.sdwanConfig.Status != "enable" {
		return nil
	}

	// SD-WAN members are already mapped as interfaces
	// Add SD-WAN-specific metadata to existing interfaces
	for _, member := range p.sdwanConfig.Members {
		if iface, exists := p.TopologyInterfaces[member.Interface]; exists {
			if iface.Metadata == nil {
				iface.Metadata = make(map[string]interface{})
			}

			iface.Metadata["sdwan_enabled"] = true
			iface.Metadata["sdwan_seq_num"] = member.SeqNum
			iface.Metadata["sdwan_zone"] = member.Zone
			iface.Metadata["sdwan_gateway"] = member.Gateway
			iface.Metadata["sdwan_cost"] = member.Cost
			iface.Metadata["sdwan_weight"] = member.Weight
			iface.Metadata["sdwan_priority"] = member.Priority

			// Get health status if available
			for _, healthCheck := range p.sdwanHealth {
				for _, memberHealth := range healthCheck.Members {
					if memberHealth.Interface == member.Interface {
						iface.Metadata["sdwan_health_state"] = memberHealth.State
						iface.Metadata["sdwan_latency"] = memberHealth.Latency
						iface.Metadata["sdwan_jitter"] = memberHealth.Jitter
						iface.Metadata["sdwan_packet_loss"] = memberHealth.PacketLoss
						iface.Metadata["sdwan_sla_uptime"] = memberHealth.SLAUptime
					}
				}
			}

			p.TopologyInterfaces[member.Interface] = iface
		}
	}

	return nil
}

// ========================================
// Helper Functions
// ========================================

// getVirtualContextID gets the virtual context ID for a given VDOM name
func (p *FortiGateProcessor) getVirtualContextID(vdomName string) string {
	// If VDOM name is empty, use "root" as default
	if vdomName == "" {
		vdomName = "root"
	}

	if vctx, exists := p.TopologyVirtualContexts[vdomName]; exists {
		return vctx.ID
	}

	// If not found, return empty string (will be handled by caller)
	return ""
}

// AddIntegrationMessage adds a message to the integration messages list
func (p *FortiGateProcessor) AddIntegrationMessage(severity, message string) {
	p.IntegrationMessages = append(p.IntegrationMessages, models.IntegrationMessage{
		Severity: severity,
		Message:  message,
		Time:     time.Now(),
	})
}
