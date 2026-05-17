package paloalto

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
	"github.com/mattpainter701/graphlayer/platform/platform/reference/appid"
	"github.com/mattpainter701/graphlayer/platform/core/constants"
	helper "github.com/mattpainter701/graphlayer/platform/core/helpers"
	"github.com/mattpainter701/graphlayer/platform/core/processor"
)

const defaultPANOSContextName = "vsys1"

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |P|A|L|O| |A|L|T|O| |M|A|P|P|E|R|S|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * PAN-OS mappers transform vendor-specific configuration to vendor-agnostic topology models
 *
 * Key mapping challenges:
 * 1. App-ID → Service: Map Layer 7 applications to traditional port-based services
 * 2. vsys → VirtualContext: Map PAN-OS virtual systems to common model
 * 3. Zones → Firewall Policy: Map zone-based policies to address-based policies
 * 4. NAT complexity: Handle source + destination NAT in same rule
 */

// PANOSProcessor is the processor for PAN-OS configuration
type PANOSProcessor struct {
	// Parsed Configuration
	systemInfo       *PANOSSystemInfo
	virtualSystems   []PANOSVirtualSystem
	zones            []PANOSZone
	interfaces       []PANOSInterface
	addressObjects   []PANOSAddressObject
	addressGroups    []PANOSAddressGroup
	serviceObjects   []PANOSServiceObject
	serviceGroups    []PANOSServiceGroup
	securityPolicies []PANOSSecurityPolicy
	natPolicies      []PANOSNATPolicy
	virtualRouters   []PANOSVirtualRouter

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

	// Metadata
	FileHash            string
	IntegrationMessages []models.IntegrationMessage
}

// NewPANOSProcessor creates a new PAN-OS processor
func NewPANOSProcessor(systemInfo *PANOSSystemInfo) *PANOSProcessor {
	return &PANOSProcessor{
		systemInfo:              systemInfo,
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
	}
}

// BuildNetworkTopologyModels maps PAN-OS configuration to standard network topology models
func (p *PANOSProcessor) BuildNetworkTopologyModels() error {
	// Initialize the topology model maps
	p.TopologyDevice = &models.Device{}

	// Build the Device model
	err := p.buildPANOSDeviceModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building PAN-OS device model")
		return err
	}

	// Build Virtual Contexts (vsys)
	err = p.buildPANOSVirtualContextModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building PAN-OS virtual context model")
		return err
	}

	// Build Zones
	err = p.buildPANOSZoneModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building PAN-OS zone model")
		return err
	}

	// Build Interfaces
	err = p.buildPANOSInterfaceModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building PAN-OS interface model")
		return err
	}

	// Build Address objects
	err = p.buildPANOSAddressModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building PAN-OS address model")
		return err
	}

	// Build Service objects
	err = p.buildPANOSServiceModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building PAN-OS service model")
		return err
	}

	// Build Security Policies
	err = p.buildPANOSSecurityPolicyModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building PAN-OS security policy model")
		return err
	}

	// Build NAT Policies
	err = p.buildPANOSNATModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building PAN-OS NAT model")
		return err
	}

	// Build Routing
	err = p.buildPANOSRoutingModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building PAN-OS routing model")
		return err
	}

	return nil
}

// buildPANOSDeviceModel creates the device model for PAN-OS firewalls
func (p *PANOSProcessor) buildPANOSDeviceModel() error {
	if p.systemInfo == nil {
		return fmt.Errorf("system info is nil")
	}

	p.TopologyDevice = &models.Device{
		ID:       uuid.New().String(),
		Hostname: p.systemInfo.Hostname,
		Vendor:   models.IntegrationPaloAlto.String(),
		Model:    p.systemInfo.Model,
		Version:  p.systemInfo.SWVersion,
		Role:     "firewall", // PAN-OS devices are firewalls
	}

	return nil
}

// buildPANOSVirtualContextModel creates virtual contexts from vsys
func (p *PANOSProcessor) buildPANOSVirtualContextModel() error {
	// If no vsys defined, create default
	if len(p.virtualSystems) == 0 {
		defaultContextID := uuid.New().String()
		p.TopologyVirtualContexts[defaultPANOSContextName] = models.VirtualContext{
			ID:           defaultContextID,
			Name:         defaultPANOSContextName,
			Description:  fmt.Sprintf("Default PAN-OS vsys for %s", p.systemInfo.Hostname),
			Type:         models.PALO_ALTO_VSYS,
			ParentDevice: p.TopologyDevice.ID,
		}
		p.TopologyDevice.VirtualContexts = []string{defaultPANOSContextName}
		return nil
	}

	// Map each vsys to a VirtualContext
	for _, vsys := range p.virtualSystems {
		contextID := uuid.New().String()
		displayName := vsys.DisplayName
		if displayName == "" {
			displayName = vsys.Name
		}

		p.TopologyVirtualContexts[vsys.Name] = models.VirtualContext{
			ID:           contextID,
			Name:         vsys.Name,
			Description:  fmt.Sprintf("PAN-OS vsys: %s (%s)", displayName, vsys.Description),
			Type:         models.PALO_ALTO_VSYS,
			ParentDevice: p.TopologyDevice.ID,
		}
		p.TopologyDevice.VirtualContexts = append(p.TopologyDevice.VirtualContexts, vsys.Name)
	}

	return nil
}

// buildPANOSZoneModel maps PAN-OS zones to standard zone model
func (p *PANOSProcessor) buildPANOSZoneModel() error {
	for _, zone := range p.zones {
		zoneID := uuid.New().String()

		// Determine zone type based on interface assignment
		zoneType := "layer3"
		if len(zone.Network.Layer2) > 0 {
			zoneType = "layer2"
		}

		// Collect all interfaces (Layer 3 + Layer 2)
		var interfaces []string
		interfaces = append(interfaces, zone.Network.Layer3...)
		interfaces = append(interfaces, zone.Network.Layer2...)

		// Determine virtual context (default to first vsys or default)
		virtualContext := defaultPANOSContextName
		if len(p.virtualSystems) > 0 {
			virtualContext = p.virtualSystems[0].Name
		}

		zoneModel := models.Zone{
			ID:             zoneID,
			Name:           zone.Name,
			VirtualContext: virtualContext,
			Description:    fmt.Sprintf("PAN-OS %s zone", zoneType),
			Interfaces:     interfaces,
			Type:           zoneType,
			HasACLs:        false, // PAN-OS uses security policies, not interface ACLs
		}

		p.TopologyZones[zone.Name] = zoneModel
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d zones from PAN-OS configuration", len(p.TopologyZones)))

	return nil
}

// buildPANOSInterfaceModel maps PAN-OS interfaces to standard interface model
func (p *PANOSProcessor) buildPANOSInterfaceModel() error {
	for _, iface := range p.interfaces {
		interfaceID := uuid.New().String()

		var addresses []string
		var role string = "internal"
		var publicFacing bool = false

		// Extract IP addresses from Layer 3 config
		if iface.Layer3 != nil {
			for _, ipConfig := range iface.Layer3.IP {
				addresses = append(addresses, ipConfig.Name) // Name contains IP with CIDR
			}
		}

		// Determine role based on zone assignment
		zone := p.findZoneForInterface(iface.Name)
		if zone != "" {
			if strings.Contains(strings.ToLower(zone), "untrust") {
				role = "wan"
				publicFacing = true
			} else if strings.Contains(strings.ToLower(zone), "trust") {
				role = "lan"
			} else if strings.Contains(strings.ToLower(zone), "dmz") {
				role = "dmz"
			}
		}

		// Determine virtual context (default to first vsys or default)
		virtualContext := defaultPANOSContextName
		if len(p.virtualSystems) > 0 {
			virtualContext = p.virtualSystems[0].Name
		}

		interfaceModel := models.Interface{
			ID:              interfaceID,
			Name:            iface.Name,
			Addresses:       addresses,
			Role:            role,
			Type:            iface.Type,
			Description:     iface.Comment,
			PublicFacing:    &publicFacing,
			VirtualFirewall: virtualContext,
		}

		p.TopologyInterfaces[interfaceID] = interfaceModel
		p.TopologyDevice.Interfaces = append(p.TopologyDevice.Interfaces, interfaceModel.Name)

		// Create zone mapping if not already exists
		if zone != "" {
			if _, exists := p.TopologyZones[zone]; !exists {
				zoneID := uuid.New().String()
				p.TopologyZones[zone] = models.Zone{
					ID:             zoneID,
					Name:           zone,
					VirtualContext: virtualContext,
					Description:    fmt.Sprintf("PAN-OS zone for interface %s", iface.Name),
					Interfaces:     []string{iface.Name},
					Type:           "layer3",
				}
			}
		}
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d interfaces from PAN-OS configuration", len(p.TopologyInterfaces)))

	return nil
}

// buildPANOSAddressModel maps PAN-OS address objects to standard address model
func (p *PANOSProcessor) buildPANOSAddressModel() error {
	addressMap := make(map[string]models.Address)

	// Determine virtual context (default to first vsys or default)
	virtualContext := defaultPANOSContextName
	if len(p.virtualSystems) > 0 {
		virtualContext = p.virtualSystems[0].Name
	}

	// 1. Map address objects
	for _, addr := range p.addressObjects {
		var addressType string
		var addressValue string

		switch addr.Type {
		case "ip-netmask":
			addressType = "subnet"
			addressValue = addr.IPNetmask
			// Ensure CIDR notation
			if !strings.Contains(addressValue, "/") {
				addressValue = addressValue + "/32"
			}
		case "ip-range":
			addressType = "range"
			addressValue = addr.IPRange
		case "fqdn":
			addressType = "fqdn"
			addressValue = addr.FQDN
		default:
			continue
		}

		addressMap[addr.Name] = models.Address{
			ID:             uuid.New().String(),
			Name:           addr.Name,
			VirtualContext: virtualContext,
			Value:          addressValue,
			Type:           addressType,
			Description:    addr.Description,
			Tags:           addr.Tags,
		}
	}

	// 2. Extract addresses from interface IPs
	for _, iface := range p.interfaces {
		if iface.Layer3 == nil {
			continue
		}

		for _, ipConfig := range iface.Layer3.IP {
			addressName := fmt.Sprintf("interface_%s_ip", strings.ReplaceAll(iface.Name, "/", "_"))

			addressMap[addressName] = models.Address{
				ID:             uuid.New().String(),
				Name:           addressName,
				VirtualContext: virtualContext,
				Value:          ipConfig.Name, // Already in CIDR format
				Type:           "host",
				Description:    fmt.Sprintf("Interface %s address", iface.Name),
				Interface:      iface.Name,
			}
		}
	}

	// Convert map to topology addresses
	for _, address := range addressMap {
		p.TopologyAddresses[address.Name] = address
	}

	// 3. Create address groups
	for _, group := range p.addressGroups {
		groupID := uuid.New().String()

		// Static groups
		if len(group.Static) > 0 {
			var actualMembers []models.Address
			var memberValues []string
			var memberNames []string

			for _, memberName := range group.Static {
				if addr, exists := p.TopologyAddresses[memberName]; exists {
					actualMembers = append(actualMembers, addr)
					memberValues = append(memberValues, addr.Value)
					memberNames = append(memberNames, addr.Name)
				}
			}

			if len(memberNames) > 0 {
				addressGroupModel := models.AddressGroup{
					ID:                    groupID,
					Name:                  group.Name,
					VirtualContext:        virtualContext,
					Members:               memberNames,
					Description:           group.Description,
					ActualMemberAddresses: actualMembers,
					Tags:                  group.Tags,
				}

				p.TopologyAddressGroups[groupID] = addressGroupModel

				// Update addresses to include group name
				for _, addr := range actualMembers {
					if existingAddr, exists := p.TopologyAddresses[addr.Name]; exists {
						existingAddr.Group = append(existingAddr.Group, group.Name)
						p.TopologyAddresses[addr.Name] = existingAddr
					}
				}
			}
		}

		// Dynamic groups (tag-based)
		if group.Dynamic.Filter != "" {
			p.addIntegrationMessage(models.IntegrationMessageTypeWarning,
				fmt.Sprintf("Dynamic address group '%s' with filter '%s' - not fully supported",
					group.Name, group.Dynamic.Filter))
		}
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d address objects and %d address groups from PAN-OS configuration",
			len(p.TopologyAddresses), len(p.TopologyAddressGroups)))

	return nil
}

// buildPANOSServiceModel maps PAN-OS service objects to standard service model
func (p *PANOSProcessor) buildPANOSServiceModel() error {
	serviceMap := make(map[string]models.Service)

	// Determine virtual context
	virtualContext := defaultPANOSContextName
	if len(p.virtualSystems) > 0 {
		virtualContext = p.virtualSystems[0].Name
	}

	// 1. Map service objects
	for _, svc := range p.serviceObjects {
		var destPorts []models.PortMap

		// TCP ports
		if svc.Protocol.TCP != nil && svc.Protocol.TCP.Port != "" {
			destPorts = append(destPorts, models.PortMap{
				Protocol: "tcp",
				Ports:    parsePortRange(svc.Protocol.TCP.Port),
			})
		}

		// UDP ports
		if svc.Protocol.UDP != nil && svc.Protocol.UDP.Port != "" {
			destPorts = append(destPorts, models.PortMap{
				Protocol: "udp",
				Ports:    parsePortRange(svc.Protocol.UDP.Port),
			})
		}

		serviceMap[svc.Name] = models.Service{
			ID:             uuid.New().String(),
			Name:           svc.Name,
			VirtualContext: virtualContext,
			DestPorts:      destPorts,
			Description:    svc.Description,
			Tags:           svc.Tags,
		}
	}

	// 2. Create standard protocol services
	standardProtocols := []string{"tcp", "udp", "icmp", "any"}
	for _, protocol := range standardProtocols {
		serviceID := fmt.Sprintf("protocol_%s", protocol)
		if _, exists := serviceMap[serviceID]; !exists {
			serviceMap[serviceID] = models.Service{
				ID:             uuid.New().String(),
				Name:           serviceID,
				VirtualContext: virtualContext,
				Description:    fmt.Sprintf("Standard %s protocol service", protocol),
			}
		}
	}

	// Convert map to topology services
	for _, service := range serviceMap {
		p.TopologyServices[service.Name] = service
	}

	// 3. Create service groups
	for _, group := range p.serviceGroups {
		groupID := uuid.New().String()

		var memberNames []string
		for _, memberName := range group.Members {
			if _, exists := p.TopologyServices[memberName]; exists {
				memberNames = append(memberNames, memberName)
			}
		}

		if len(memberNames) > 0 {
			serviceGroupModel := models.ServiceGroup{
				ID:             groupID,
				Name:           group.Name,
				VirtualContext: virtualContext,
				Members:        memberNames,
				Description:    group.Description,
				Tags:           group.Tags,
			}

			p.TopologyServiceGroups[groupID] = serviceGroupModel
		}
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d service objects and %d service groups from PAN-OS configuration",
			len(p.TopologyServices), len(p.TopologyServiceGroups)))

	return nil
}

// buildPANOSSecurityPolicyModel maps PAN-OS security policies to firewall policy model
func (p *PANOSProcessor) buildPANOSSecurityPolicyModel() error {
	// Determine virtual context
	virtualContext := defaultPANOSContextName
	if len(p.virtualSystems) > 0 {
		virtualContext = p.virtualSystems[0].Name
	}

	for ruleOrder, policy := range p.securityPolicies {
		policyID := uuid.New().String()

		// Map action
		action := "deny"
		if policy.Action == "allow" {
			action = "allow"
		} else if policy.Action == "drop" || policy.Action == "deny" {
			action = "deny"
		} else if policy.Action == "reset-client" || policy.Action == "reset-server" {
			action = "reset"
		}

		// Map services (with App-ID handling)
		var services []string
		for _, svc := range policy.Services {
			if svc == "application-default" {
				// Application-default means use App-ID to determine ports
				// For now, map to "any" or create App-ID specific service
				services = append(services, "any")
			} else {
				services = append(services, svc)
			}
		}

		// Handle App-ID applications
		// TODO: Create App-ID → Service mapping table
		if len(policy.Applications) > 0 {
			for _, app := range policy.Applications {
				appService := mapAppIDToService(app)
				if appService != "" {
					services = append(services, appService)
				}
			}
		}

		if len(services) == 0 {
			services = append(services, "any")
		}

		// Check if source/dest are "any"
		isSourceAny := containsAny(policy.SourceAddresses)
		isDestAny := containsAny(policy.DestinationAddresses)

		// Determine enabled status
		enabled := policy.Disabled != "yes"

		// Map log settings
		logStart := policy.LogStart == "yes"
		logEnd := policy.LogEnd == "yes"

		firewallPolicy := models.FirewallPolicy{
			ID:                   policyID,
			Name:                 policy.Name,
			VirtualContext:       virtualContext,
			Enabled:              enabled,
			SourceZones:          policy.FromZones,
			DestinationZones:     policy.ToZones,
			SourceAddresses:      policy.SourceAddresses,
			DestinationAddresses: policy.DestinationAddresses,
			Services:             services,
			Action:               action,
			Log:                  models.LogConfig{AtStart: logStart, AtEnd: logEnd},
			Description:          policy.Description,
			RuleOrder:            ruleOrder + 1,
			Firewall:             p.systemInfo.Hostname,
			AnySource:            isSourceAny,
			AnyDestination:       isDestAny,
			Tags:                 policy.Tags,
		}

		// Add PAN-OS specific metadata
		if policy.UUID != "" {
			firewallPolicy.Metadata = map[string]string{
				"panos_uuid":        policy.UUID,
				"panos_rule_type":   policy.RuleType,
			}
		}

		// Add App-ID metadata
		if len(policy.Applications) > 0 {
			firewallPolicy.Metadata["panos_applications"] = strings.Join(policy.Applications, ",")
		}

		// Add User-ID metadata
		if len(policy.SourceUsers) > 0 {
			firewallPolicy.Metadata["panos_source_users"] = strings.Join(policy.SourceUsers, ",")
		}

		p.TopologyFirewallPolicies[policyID] = firewallPolicy
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d firewall policies from PAN-OS security rules", len(p.TopologyFirewallPolicies)))

	return nil
}

// buildPANOSNATModel maps PAN-OS NAT policies to NAT model
func (p *PANOSProcessor) buildPANOSNATModel() error {
	// Determine virtual context
	virtualContext := defaultPANOSContextName
	if len(p.virtualSystems) > 0 {
		virtualContext = p.virtualSystems[0].Name
	}

	for ruleOrder, natPolicy := range p.natPolicies {
		// PAN-OS can have both source and destination NAT in same rule
		// We'll create separate NAT entries for each type

		enabled := natPolicy.Disabled != "yes"

		// Handle Source NAT
		if natPolicy.SourceTranslation != nil {
			natID := uuid.New().String()
			natType := "source-nat"

			var translatedSource string

			// Dynamic-IP-and-Port (PAT)
			if natPolicy.SourceTranslation.DynamicIPAndPort != nil {
				if natPolicy.SourceTranslation.DynamicIPAndPort.InterfaceAddress != nil {
					translatedSource = fmt.Sprintf("interface:%s",
						natPolicy.SourceTranslation.DynamicIPAndPort.InterfaceAddress.Interface)
					if natPolicy.SourceTranslation.DynamicIPAndPort.InterfaceAddress.IP != "" {
						translatedSource = natPolicy.SourceTranslation.DynamicIPAndPort.InterfaceAddress.IP
					}
				} else if len(natPolicy.SourceTranslation.DynamicIPAndPort.TranslatedAddress) > 0 {
					translatedSource = strings.Join(natPolicy.SourceTranslation.DynamicIPAndPort.TranslatedAddress, ",")
				}
			}

			// Dynamic-IP (pooled NAT)
			if natPolicy.SourceTranslation.DynamicIP != nil {
				if len(natPolicy.SourceTranslation.DynamicIP.TranslatedAddress) > 0 {
					translatedSource = strings.Join(natPolicy.SourceTranslation.DynamicIP.TranslatedAddress, ",")
				}
			}

			// Static-IP (1:1 NAT)
			if natPolicy.SourceTranslation.StaticIP != nil {
				translatedSource = natPolicy.SourceTranslation.StaticIP.TranslatedAddress
				natType = "static-nat"
			}

			// Create Source NAT entry
			if translatedSource != "" {
				sourceNAT := models.NAT{
					ID:                natID,
					Name:              fmt.Sprintf("%s_source", natPolicy.Name),
					VirtualContext:    virtualContext,
					Type:              natType,
					OriginalSource:    strings.Join(natPolicy.SourceAddresses, ","),
					TranslatedSource:  translatedSource,
					OriginalDestination: strings.Join(natPolicy.DestinationAddresses, ","),
					Bidirectional:     natPolicy.SourceTranslation.StaticIP != nil &&
						natPolicy.SourceTranslation.StaticIP.BiDirectional == "yes",
					Enabled:           enabled,
					RuleOrder:         ruleOrder + 1,
					Metadata: map[string]string{
						"panos_uuid":        natPolicy.UUID,
						"panos_from_zones":  strings.Join(natPolicy.FromZones, ","),
						"panos_to_zones":    strings.Join(natPolicy.ToZones, ","),
						"panos_to_interface": natPolicy.ToInterface,
					},
				}

				p.TopologyNATs[natID] = sourceNAT
			}
		}

		// Handle Destination NAT
		if natPolicy.DestinationTranslation != nil {
			natID := uuid.New().String()

			destNAT := models.NAT{
				ID:                  natID,
				Name:                fmt.Sprintf("%s_destination", natPolicy.Name),
				VirtualContext:      virtualContext,
				Type:                "destination-nat",
				OriginalDestination: strings.Join(natPolicy.DestinationAddresses, ","),
				TranslatedDestination: natPolicy.DestinationTranslation.TranslatedAddress,
				TranslatedPort:      natPolicy.DestinationTranslation.TranslatedPort,
				Bidirectional:       false,
				Enabled:             enabled,
				RuleOrder:           ruleOrder + 1,
				Metadata: map[string]string{
					"panos_uuid":        natPolicy.UUID,
					"panos_from_zones":  strings.Join(natPolicy.FromZones, ","),
					"panos_to_zones":    strings.Join(natPolicy.ToZones, ","),
					"panos_service":     natPolicy.Service,
				},
			}

			p.TopologyNATs[natID] = destNAT
		}
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d NAT rules from PAN-OS NAT policies", len(p.TopologyNATs)))

	return nil
}

// buildPANOSRoutingModel maps PAN-OS virtual routers and routes to routing model
func (p *PANOSProcessor) buildPANOSRoutingModel() error {
	// Determine virtual context
	virtualContext := defaultPANOSContextName
	if len(p.virtualSystems) > 0 {
		virtualContext = p.virtualSystems[0].Name
	}

	for _, vr := range p.virtualRouters {
		for _, route := range vr.StaticRoutes {
			routeID := uuid.New().String()

			routingEntry := models.RoutingEntry{
				ID:             routeID,
				VirtualContext: virtualContext,
				Destination:    route.Destination,
				NextHop:        route.NextHop.IPAddress,
				Interface:      route.Interface,
				Type:           "static",
				AdminDistance:  route.AdminDistance,
				Metric:         route.Metric,
				Name:           route.Name,
				VirtualRouter:  vr.Name,
			}

			p.TopologyRoutingEntries[routeID] = routingEntry
		}
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d static routing entries from PAN-OS virtual routers", len(p.TopologyRoutingEntries)))

	return nil
}

// PANOSMapper creates the final ParsedNetworkTopology from the processor's topology models
func PANOSMapper(p *PANOSProcessor) models.ParsedNetworkTopology {
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
		FirewallName:     p.systemInfo.Hostname,
		Metadata: models.L3NodeMetadata{
			Parser:     processor.ParserTypePaloAlto,
			SourceType: constants.SOURCE_TYPE_VENDOR,
		},
		InsertedTime: time.Now().Format(time.RFC3339),
		LastModified: time.Now().Format(time.RFC3339),
		Active:       true,
	}
}

// ========================================
// HELPER FUNCTIONS
// ========================================

// findZoneForInterface finds the zone that contains a specific interface
func (p *PANOSProcessor) findZoneForInterface(interfaceName string) string {
	for _, zone := range p.zones {
		for _, iface := range zone.Network.Layer3 {
			if iface == interfaceName {
				return zone.Name
			}
		}
		for _, iface := range zone.Network.Layer2 {
			if iface == interfaceName {
				return zone.Name
			}
		}
	}
	return ""
}

// parsePortRange converts PAN-OS port specification to string array
func parsePortRange(portSpec string) []string {
	// Handle comma-separated ports and ranges
	// Examples: "80", "8080-8090", "80,443,8080-8090"
	var ports []string
	parts := strings.Split(portSpec, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ports = append(ports, part)
	}
	return ports
}

// containsAny checks if address list contains "any"
func containsAny(addresses []string) bool {
	for _, addr := range addresses {
		if strings.ToLower(addr) == "any" {
			return true
		}
	}
	return false
}

// mapAppIDToService maps PAN-OS App-ID to standard service using the App-ID cache
func mapAppIDToService(appID string) string {
	// Use global App-ID cache for lookups
	cache := appid.GetGlobalAppIDCache()

	// Try to map App-ID to protocol/ports
	protocol, ports, ok := cache.MapToService(appID)
	if !ok {
		// Fallback to hardcoded mappings for common applications
		appIDMap := map[string]string{
			"ssl":           "tcp_443",
			"web-browsing":  "tcp_80",
			"ssh":           "tcp_22",
			"ftp":           "tcp_21",
			"smtp":          "tcp_25",
			"dns":           "udp_53",
			"http":          "tcp_80",
			"https":         "tcp_443",
			"telnet":        "tcp_23",
			"pop3":          "tcp_110",
			"imap":          "tcp_143",
			"ldap":          "tcp_389",
			"mysql":         "tcp_3306",
			"ms-sql-server": "tcp_1433",
			"rdp":           "tcp_3389",
			"vnc":           "tcp_5900",
		}

		if service, exists := appIDMap[strings.ToLower(appID)]; exists {
			return service
		}

		// For unknown App-IDs, return empty (will use "application-default")
		return ""
	}

	// Format as "protocol_port" or "protocol_port1,port2,..."
	if len(ports) == 0 {
		return fmt.Sprintf("%s_any", protocol)
	}

	return fmt.Sprintf("%s_%s", protocol, strings.Join(ports, ","))
}

// addIntegrationMessage adds an integration message to the processor
func (p *PANOSProcessor) addIntegrationMessage(msgType models.IntegrationMessageType, message string) {
	p.IntegrationMessages = append(p.IntegrationMessages, models.IntegrationMessage{
		Type:    msgType,
		Message: message,
	})
}
