package cisco

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
	"github.com/mattpainter701/graphlayer/platform/core/constants"
	helper "github.com/mattpainter701/graphlayer/platform/core/helpers"
	"github.com/mattpainter701/graphlayer/platform/core/processor"
)

const defaultIOSContextName = "default-ios-context"

// +-+-+-+-+-+-+-+
// |m|a|p|p|e|r|s|
// +-+-+-+-+-+-+-+
/*
 * The following methods are responsible for mapping a cisco IOS specific configuration to
 * common models. They assume that the IOS configuration has already been parsed into the
 * Cisco specific model.
 */

// BuildNetworkTopologyModels maps internal cisco IOS models to the standard network topology models
func (p *CiscoIOSProcessor) BuildNetworkTopologyModels() error {
	// Initialize the topology model maps
	p.TopologyDevice = &models.Device{}
	p.TopologyInterfaces = make(map[string]models.Interface)
	p.TopologyAddresses = make(map[string]models.Address)
	p.TopologyAddressGroups = make(map[string]models.AddressGroup)
	p.TopologyServices = make(map[string]models.Service)
	p.TopologyServiceGroups = make(map[string]models.ServiceGroup)
	p.TopologyRoutingEntries = make(map[string]models.RoutingEntry)
	p.TopologyNATs = make(map[string]models.NAT)
	p.TopologyVIPs = make(map[string]models.VIP)
	p.TopologyFirewallPolicies = make(map[string]models.FirewallPolicy)
	p.TopologyZones = make(map[string]models.Zone)
	p.TopologyVirtualContexts = make(map[string]models.VirtualContext)

	// Build the Device model
	err := p.buildIOSDeviceModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building network topology models(devices)")
		return err
	}

	// Build interfaces and addresses
	err = p.buildIOSInterfaceModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building network topology models(interfaces)")
		return err
	}

	// Build virtual address objects from ACLs, routes, and NAT
	err = p.buildIOSAddressModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building network topology models(addresses)")
		return err
	}

	// Build services from ACL port specifications
	err = p.buildIOSServiceModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building network topology models(services)")
		return err
	}

	// Build firewall policies from ACLs
	err = p.buildIOSFirewallPolicyModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building network topology models(firewall policies)")
		return err
	}

	// Build routing entries
	err = p.buildIOSRoutingModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building network topology models(routes)")
		return err
	}

	// Build NAT entries
	err = p.buildIOSNATModel()
	if err != nil {
		helper.Logger.Error().Err(err).Msg("error building network topology models(NAT)")
		return err
	}

	return nil
}

// buildIOSDeviceModel creates the device model for IOS routers
func (p *CiscoIOSProcessor) buildIOSDeviceModel() error {
	p.TopologyDevice = &models.Device{
		ID:       uuid.New().String(),
		Hostname: p.iosConfig.Hostname,
		Vendor:   models.IntegrationCisco.String(),
		Model:    "ios",
		Version:  p.iosConfig.Version,
		Role:     "router", // IOS devices are typically routers
	}

	// Create default virtual context for IOS (IOS doesn't have multiple contexts like ASA)
	if _, exists := p.TopologyVirtualContexts[defaultIOSContextName]; !exists {
		defaultContextID := uuid.New().String()
		p.TopologyVirtualContexts[defaultIOSContextName] = models.VirtualContext{
			ID:           defaultContextID,
			Name:         defaultIOSContextName,
			Description:  fmt.Sprintf("Default IOS context for %s", p.iosConfig.Hostname),
			Type:         models.CISCO_VIRTUAL_CONTEXT,
			ParentDevice: p.TopologyDevice.ID,
		}
	}
	p.TopologyDevice.VirtualContexts = []string{p.TopologyVirtualContexts[defaultIOSContextName].Name}

	return nil
}

// buildIOSInterfaceModel maps Cisco IOS interfaces to standard network topology interfaces
func (p *CiscoIOSProcessor) buildIOSInterfaceModel() error {
	// Loop through all interfaces in the Cisco IOS configuration
	for _, ciscoInterface := range p.iosConfig.Interfaces {
		if ciscoInterface.Shutdown || !ciscoInterface.Enabled {
			continue
		}

		// Create a unique ID for the interface
		interfaceID := uuid.New().String()

		var addresses []string
		if ciscoInterface.IPAddress != "" && ciscoInterface.Netmask != "" {
			addresses = append(addresses, ciscoInterface.Subnet)
		}

		// Determine interface type
		interfaceType := "physical" // default is physical
		if strings.Contains(strings.ToLower(ciscoInterface.Name), "vlan") {
			interfaceType = "vlan"
		} else if strings.Contains(strings.ToLower(ciscoInterface.Name), "loopback") {
			interfaceType = "loopback"
		} else if strings.Contains(strings.ToLower(ciscoInterface.Name), "tunnel") {
			interfaceType = "tunnel"
		} else if strings.Contains(strings.ToLower(ciscoInterface.Name), "port-channel") {
			interfaceType = "aggregated"
		} else if ciscoInterface.Type == "switchport" {
			interfaceType = "switchport"
		}

		// Determine interface role based on interface type and naming conventions
		role := "internal" // default role
		if strings.Contains(strings.ToLower(ciscoInterface.Description), "wan") ||
			strings.Contains(strings.ToLower(ciscoInterface.Name), "serial") {
			role = "wan"
		} else if strings.Contains(strings.ToLower(ciscoInterface.Description), "lan") ||
			strings.Contains(strings.ToLower(ciscoInterface.Name), "ethernet") {
			role = "lan"
		} else if strings.Contains(strings.ToLower(ciscoInterface.Description), "dmz") {
			role = "dmz"
		}

		// Determine if interface is public facing (basic heuristics)
		publicFacing := false
		if role == "wan" || strings.Contains(strings.ToLower(ciscoInterface.Description), "internet") ||
			strings.Contains(strings.ToLower(ciscoInterface.Description), "public") {
			publicFacing = true
		}

		// Map to the standardized Interface model
		interfaceModel := models.Interface{
			ID:              interfaceID,
			Name:            ciscoInterface.Name,
			Addresses:       addresses,
			VlanID:          ciscoInterface.VLAN,
			Role:            role,
			Type:            interfaceType,
			Description:     ciscoInterface.Description,
			PublicFacing:    &publicFacing,
			VirtualFirewall: p.TopologyVirtualContexts[defaultIOSContextName].Name,
		}

		// Store the interface in the map
		p.TopologyInterfaces[interfaceID] = interfaceModel

		// Create a zone for the interface (IOS uses interface-based access control)
		zoneType := "layer3"
		if ciscoInterface.Type == "switchport" {
			zoneType = "layer2"
		}

		// Check if this interface has any ACLs attached
		hasACLs := len(ciscoInterface.AccessGroups) > 0

		zone := models.Zone{
			ID:             uuid.New().String(),
			Name:           fmt.Sprintf("zone_%s", ciscoInterface.Name),
			VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
			Description:    fmt.Sprintf("IOS zone for interface %s", ciscoInterface.Name),
			Interfaces:     []string{ciscoInterface.Name},
			Type:           zoneType,
			HasACLs:        hasACLs,
		}
		p.TopologyZones[ciscoInterface.Name] = zone

		p.TopologyDevice.Interfaces = append(p.TopologyDevice.Interfaces, interfaceModel.Name)
	}

	return nil
}

// buildIOSAddressModel creates virtual address objects from ACLs, routes, and interface configurations
func (p *CiscoIOSProcessor) buildIOSAddressModel() error {
	addressMap := make(map[string]models.Address) // Use map to avoid duplicates
	interfaceGroups := make(map[string][]string)  // Group addresses by interface subnet

	// 1. Extract addresses from interface configurations
	for _, iface := range p.iosConfig.Interfaces {
		if iface.IPAddress == "" || iface.Netmask == "" {
			continue
		}

		// Create address for the interface subnet
		addressName := fmt.Sprintf("subnet_%s", strings.ReplaceAll(iface.Subnet, "/", "_"))
		if _, exists := addressMap[addressName]; !exists {
			addressMap[addressName] = models.Address{
				ID:             uuid.New().String(),
				Name:           addressName,
				VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
				Value:          iface.Subnet,
				Type:           "subnet",
				Description:    fmt.Sprintf("Interface %s subnet", iface.Name),
				Interface:      iface.Name,
			}
		}

		// Create address for the interface host IP
		hostName := fmt.Sprintf("host_%s", strings.ReplaceAll(iface.IPAddress, ".", "_"))
		if _, exists := addressMap[hostName]; !exists {
			addressMap[hostName] = models.Address{
				ID:             uuid.New().String(),
				Name:           hostName,
				VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
				Value:          fmt.Sprintf("%s/32", iface.IPAddress),
				Type:           "host",
				Description:    fmt.Sprintf("Interface %s host address", iface.Name),
				Interface:      iface.Name,
			}
		}

		// Track interface groups
		interfaceGroups[iface.Name] = append(interfaceGroups[iface.Name], addressName, hostName)
	}

	// 2. Extract addresses from Standard ACLs
	for _, acl := range p.iosConfig.StandardACLs {
		for _, rule := range acl.Rules {
			if rule.Source == "" {
				continue
			}

			addressName, addressValue, addressType := p.parseACLAddress(rule.Source, rule.Wildcard)
			if addressName != "" && addressValue != "" {
				if _, exists := addressMap[addressName]; !exists {
					addressMap[addressName] = models.Address{
						ID:             uuid.New().String(),
						Name:           addressName,
						VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
						Value:          addressValue,
						Type:           addressType,
						Description:    fmt.Sprintf("From Standard ACL %d", acl.Number),
					}
				}
			}
		}
	}

	// 3. Extract addresses from Extended ACLs
	for _, acl := range p.iosConfig.ExtendedACLs {
		for _, rule := range acl.Rules {
			// Process source addresses
			if rule.Source != "" {
				addressName, addressValue, addressType := p.parseACLAddress(rule.Source, rule.SourceWildcard)
				if addressName != "" && addressValue != "" {
					if _, exists := addressMap[addressName]; !exists {
						addressMap[addressName] = models.Address{
							ID:             uuid.New().String(),
							Name:           addressName,
							VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
							Value:          addressValue,
							Type:           addressType,
							Description:    fmt.Sprintf("From Extended ACL %d (source)", acl.Number),
						}
					}
				}
			}

			// Process destination addresses
			if rule.Destination != "" {
				addressName, addressValue, addressType := p.parseACLAddress(rule.Destination, rule.DestWildcard)
				if addressName != "" && addressValue != "" {
					if _, exists := addressMap[addressName]; !exists {
						addressMap[addressName] = models.Address{
							ID:             uuid.New().String(),
							Name:           addressName,
							VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
							Value:          addressValue,
							Type:           addressType,
							Description:    fmt.Sprintf("From Extended ACL %d (destination)", acl.Number),
						}
					}
				}
			}
		}
	}

	// 4. Extract addresses from Named ACLs
	for _, acl := range p.iosConfig.NamedACLs {
		for _, rule := range acl.Rules {
			// Process source addresses
			if rule.Source != "" {
				addressName, addressValue, addressType := p.parseACLAddress(rule.Source, rule.SourceWildcard)
				if addressName != "" && addressValue != "" {
					if _, exists := addressMap[addressName]; !exists {
						addressMap[addressName] = models.Address{
							ID:             uuid.New().String(),
							Name:           addressName,
							VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
							Value:          addressValue,
							Type:           addressType,
							Description:    fmt.Sprintf("From Named ACL %s (source)", acl.Name),
						}
					}
				}
			}

			// Process destination addresses
			if rule.Destination != "" {
				addressName, addressValue, addressType := p.parseACLAddress(rule.Destination, rule.DestWildcard)
				if addressName != "" && addressValue != "" {
					if _, exists := addressMap[addressName]; !exists {
						addressMap[addressName] = models.Address{
							ID:             uuid.New().String(),
							Name:           addressName,
							VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
							Value:          addressValue,
							Type:           addressType,
							Description:    fmt.Sprintf("From Named ACL %s (destination)", acl.Name),
						}
					}
				}
			}
		}
	}

	// 5. Extract addresses from static routes
	for _, route := range p.iosConfig.Routes {
		if route.Network != "" && route.Netmask != "" {
			// Convert network and netmask to CIDR
			cidr, err := helper.ConvertToCIDR(route.Network, route.Netmask)
			if err != nil {
				p.addIntegrationMessage(models.IntegrationMessageTypeWarning,
					fmt.Sprintf("Error converting route %s %s to CIDR: %v", route.Network, route.Netmask, err))
				continue
			}

			addressName := fmt.Sprintf("route_%s", strings.ReplaceAll(cidr, "/", "_"))
			if _, exists := addressMap[addressName]; !exists {
				addressMap[addressName] = models.Address{
					ID:             uuid.New().String(),
					Name:           addressName,
					VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
					Value:          cidr,
					Type:           "subnet",
					Description:    fmt.Sprintf("Static route destination via %s", route.NextHop),
				}
			}
		}

		// Also create address for next-hop
		if route.NextHop != "" && net.ParseIP(route.NextHop) != nil {
			addressName := fmt.Sprintf("nexthop_%s", strings.ReplaceAll(route.NextHop, ".", "_"))
			if _, exists := addressMap[addressName]; !exists {
				addressMap[addressName] = models.Address{
					ID:             uuid.New().String(),
					Name:           addressName,
					VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
					Value:          fmt.Sprintf("%s/32", route.NextHop),
					Type:           "host",
					Description:    "Static route next-hop",
				}
			}
		}
	}

	// Convert map to topology addresses
	for _, address := range addressMap {
		p.TopologyAddresses[address.Name] = address
	}

	// 6. Create address groups for interface subnets
	for interfaceName, addressNames := range interfaceGroups {
		if len(addressNames) > 1 {
			groupID := uuid.New().String()

			// Get actual member addresses and their values (similar to ASA)
			var actualMembers []models.Address
			var memberValues []string
			var memberNames []string
			for _, addrName := range addressNames {
				if addr, exists := p.TopologyAddresses[addrName]; exists {
					actualMembers = append(actualMembers, addr)
					memberValues = append(memberValues, addr.Value)
					memberNames = append(memberNames, addr.Name)
				}
			}

			// Only create group if we have valid members
			if len(memberValues) > 0 {
				group := models.AddressGroup{
					ID:                    groupID,
					Name:                  fmt.Sprintf("interface_%s_group", interfaceName),
					VirtualContext:        p.TopologyVirtualContexts[defaultIOSContextName].Name,
					Members:               memberNames,
					Description:           fmt.Sprintf("Address group for interface %s", interfaceName),
					ActualMemberAddresses: actualMembers,
				}

				p.TopologyAddressGroups[groupID] = group

				// Update the addresses to include the group name (like ASA does)
				for _, addr := range actualMembers {
					if existingAddr, exists := p.TopologyAddresses[addr.Name]; exists {
						// Check if group is already added to avoid duplicates
						groupAlreadyAdded := false
						for _, existingGroup := range existingAddr.Group {
							if existingGroup == group.Name {
								groupAlreadyAdded = true
								break
							}
						}
						if !groupAlreadyAdded {
							existingAddr.Group = append(existingAddr.Group, group.Name)
							p.TopologyAddresses[addr.Name] = existingAddr
						}
					}
				}
			}
		}
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d virtual address objects from IOS configuration", len(p.TopologyAddresses)))

	return nil
}

// parseACLAddress parses an ACL address and wildcard mask into a standardized format
func (p *CiscoIOSProcessor) parseACLAddress(address, wildcard string) (name, value, addressType string) {
	// Handle special cases
	if address == "0.0.0.0" && wildcard == "255.255.255.255" {
		return "any", "0.0.0.0/0", "any"
	}

	if address == "any" {
		return "any", "0.0.0.0/0", "any"
	}

	// Handle host addresses (wildcard 0.0.0.0 or missing wildcard)
	if wildcard == "" || wildcard == "0.0.0.0" {
		if net.ParseIP(address) != nil {
			name = fmt.Sprintf("host_%s", strings.ReplaceAll(address, ".", "_"))
			value = fmt.Sprintf("%s/32", address)
			addressType = "host"
			return
		}
	}

	// Handle subnet with wildcard mask
	if wildcard != "" && net.ParseIP(address) != nil && net.ParseIP(wildcard) != nil {
		// Convert wildcard mask to CIDR
		cidr, err := p.convertWildcardToCIDR(address, wildcard)
		if err != nil {
			p.addIntegrationMessage(models.IntegrationMessageTypeWarning,
				fmt.Sprintf("Error converting wildcard %s %s to CIDR: %v", address, wildcard, err))
			return "", "", ""
		}

		name = fmt.Sprintf("subnet_%s", strings.ReplaceAll(cidr, "/", "_"))
		value = cidr
		addressType = "subnet"
		return
	}

	// If we can't parse it, treat as unknown
	return "", "", ""
}

// buildIOSServiceModel creates service objects from ACL port specifications
func (p *CiscoIOSProcessor) buildIOSServiceModel() error {
	serviceMap := make(map[string]models.Service) // Use map to avoid duplicates

	// Create standard protocol services
	standardProtocols := []string{"tcp", "udp", "icmp", "ip"}
	for _, protocol := range standardProtocols {
		serviceID := fmt.Sprintf("protocol_%s", protocol)
		if _, exists := serviceMap[serviceID]; !exists {
			var destPorts []models.PortMap
			if protocol != "icmp" && protocol != "ip" {
				// Get common ports for this protocol
				if ports, ok := constants.CommonProtocolPorts[protocol]; ok {
					portStrings := make([]string, len(ports))
					for i, port := range ports {
						portStrings[i] = strconv.Itoa(port)
					}
					destPorts = append(destPorts, models.PortMap{
						Protocol: protocol,
						Ports:    portStrings,
					})
				}
			}

			serviceMap[serviceID] = models.Service{
				ID:             uuid.New().String(),
				Name:           serviceID,
				VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
				DestPorts:      destPorts,
				Description:    fmt.Sprintf("Standard %s protocol service", protocol),
			}
		}
	}

	// Extract services from Extended ACLs
	for _, acl := range p.iosConfig.ExtendedACLs {
		for _, rule := range acl.Rules {
			if rule.Protocol != "" && rule.Protocol != "ip" {
				// Create service for protocol + ports combination
				serviceID := fmt.Sprintf("acl_%d_%s", acl.Number, rule.Protocol)

				var destPorts []models.PortMap
				if len(rule.DestPorts) > 0 {
					destPorts = append(destPorts, models.PortMap{
						Protocol: rule.Protocol,
						Ports:    rule.DestPorts,
					})
					serviceID = fmt.Sprintf("%s_ports_%s", serviceID, strings.Join(rule.DestPorts, "_"))
				}

				if _, exists := serviceMap[serviceID]; !exists {
					service := models.Service{
						ID:             uuid.New().String(),
						Name:           serviceID,
						VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
						DestPorts:      destPorts,
						Description:    fmt.Sprintf("Service from Extended ACL %d", acl.Number),
					}

					serviceMap[serviceID] = service
				}
			}
		}
	}

	// Extract services from Named ACLs
	for _, acl := range p.iosConfig.NamedACLs {
		for _, rule := range acl.Rules {
			if rule.Protocol != "" && rule.Protocol != "ip" {
				// Create service for protocol + ports combination
				serviceID := fmt.Sprintf("acl_%s_%s", acl.Name, rule.Protocol)

				var destPorts []models.PortMap
				if len(rule.DestPorts) > 0 {
					destPorts = append(destPorts, models.PortMap{
						Protocol: rule.Protocol,
						Ports:    rule.DestPorts,
					})
					serviceID = fmt.Sprintf("%s_ports_%s", serviceID, strings.Join(rule.DestPorts, "_"))
				}

				if _, exists := serviceMap[serviceID]; !exists {
					service := models.Service{
						ID:             uuid.New().String(),
						Name:           serviceID,
						VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
						DestPorts:      destPorts,
						Description:    fmt.Sprintf("Service from Named ACL %s", acl.Name),
					}

					serviceMap[serviceID] = service
				}
			}
		}
	}

	// Convert map to topology services
	for _, service := range serviceMap {
		p.TopologyServices[service.Name] = service
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d service objects from IOS ACLs", len(p.TopologyServices)))

	return nil
}

// buildIOSFirewallPolicyModel converts ACL rules to firewall policies
func (p *CiscoIOSProcessor) buildIOSFirewallPolicyModel() error {
	// Process Standard ACLs
	for _, acl := range p.iosConfig.StandardACLs {
		for _, rule := range acl.Rules {
			if rule.Action == "" {
				continue
			}

			policyID := uuid.New().String()
			action := "deny"
			if strings.ToLower(rule.Action) == "permit" {
				action = "allow"
			}

			// Resolve source address
			var sourceAddresses []string
			var isSourceAny bool
			sourceName, sourceValue, _ := p.parseACLAddress(rule.Source, rule.Wildcard)
			if sourceName != "" && sourceValue != "" {
				if sourceName == "any" {
					sourceAddresses = append(sourceAddresses, "any")
					isSourceAny = true
				} else {
					sourceAddresses = append(sourceAddresses, sourceValue)
				}
			}

			// For standard ACLs, destination is "any"
			destAddresses := []string{"any"}
			isDestAny := true

			// Standard ACLs apply to all IP traffic
			services := []string{"protocol_ip"}

			// Find which interfaces this ACL is applied to and determine zones
			var sourceZones, destZones []string

			// First, find the source zones from ACL bindings
			for _, iface := range p.iosConfig.Interfaces {
				for _, accessGroup := range iface.AccessGroups {
					if accessGroup.ACLName == strconv.Itoa(acl.Number) {
						if zone, exists := p.TopologyZones[iface.Name]; exists {
							if accessGroup.Direction == "in" {
								sourceZones = append(sourceZones, zone.Name)
							}
							// For standard ACLs, destination is always "any"
						}
					}
				}
			}

			// For standard ACLs, destination is always "any", so include all zones
			for _, zone := range p.TopologyZones {
				destZones = append(destZones, zone.Name)
			}

			policy := models.FirewallPolicy{
				ID:                   policyID,
				Name:                 fmt.Sprintf("Standard_ACL_%d_Rule_%d", acl.Number, rule.Order),
				VirtualContext:       p.TopologyVirtualContexts[defaultIOSContextName].Name,
				Enabled:              len(sourceZones) > 0 || len(destZones) > 0, // Only enabled if bound to interface
				SourceZones:          sourceZones,
				DestinationZones:     destZones,
				SourceAddresses:      sourceAddresses,
				DestinationAddresses: destAddresses,
				Services:             services,
				Action:               action,
				Log:                  models.LogConfig{AtStart: rule.Log, AtEnd: rule.Log},
				Description:          rule.Raw,
				RuleOrder:            rule.Order,
				Firewall:             p.iosConfig.Hostname,
				AnySource:            isSourceAny,
				AnyDestination:       isDestAny,
			}

			p.TopologyFirewallPolicies[policyID] = policy
		}
	}

	// Process Extended ACLs
	for _, acl := range p.iosConfig.ExtendedACLs {
		for _, rule := range acl.Rules {
			if rule.Action == "" {
				continue
			}

			policyID := uuid.New().String()
			action := "deny"
			if strings.ToLower(rule.Action) == "permit" {
				action = "allow"
			}

			// Resolve source address
			var sourceAddresses []string
			var isSourceAny bool
			sourceName, sourceValue, _ := p.parseACLAddress(rule.Source, rule.SourceWildcard)
			if sourceName != "" && sourceValue != "" {
				if sourceName == "any" {
					sourceAddresses = append(sourceAddresses, "any")
					isSourceAny = true
				} else {
					sourceAddresses = append(sourceAddresses, sourceValue)
				}
			}

			// Resolve destination address
			var destAddresses []string
			var isDestAny bool
			destName, destValue, _ := p.parseACLAddress(rule.Destination, rule.DestWildcard)
			if destName != "" && destValue != "" {
				if destName == "any" {
					destAddresses = append(destAddresses, "any")
					isDestAny = true
				} else {
					destAddresses = append(destAddresses, destValue)
				}
			}

			// Resolve service
			var services []string
			if rule.Protocol != "" {
				serviceID := fmt.Sprintf("protocol_%s", rule.Protocol)
				if len(rule.DestPorts) > 0 {
					serviceID = fmt.Sprintf("acl_%d_%s_ports_%s", acl.Number, rule.Protocol, strings.Join(rule.DestPorts, "_"))
				}
				services = append(services, serviceID)
			} else {
				services = append(services, "protocol_ip")
			}

			// Find which interfaces ("zones") this ACL is applied to and determine zones
			var sourceZones, destZones []string

			if isSourceAny {
				for _, zone := range p.TopologyZones {
					sourceZones = append(sourceZones, zone.Name)
				}
			} else {
				for _, iface := range p.iosConfig.Interfaces {
					for _, accessGroup := range iface.AccessGroups {
						if accessGroup.ACLName == strconv.Itoa(acl.Number) {
							if zone, exists := p.TopologyZones[iface.Name]; exists {
								if accessGroup.Direction == "in" {
									sourceZones = append(sourceZones, zone.Name)
								}
							}
						}
					}
				}

				// If no zones from ACL bindings, try to find zone containing the source address
				if len(sourceZones) == 0 {
					for _, addr := range sourceAddresses {
						for _, iface := range p.TopologyInterfaces {
							if len(iface.Addresses) == 0 {
								continue
							}
							if p.isAddressInInterfaceSubnet(addr, iface.Addresses[0]) {
								if zone, exists := p.TopologyZones[iface.Name]; exists {
									sourceZones = append(sourceZones, zone.Name)
									break
								}
							}
						}
					}
				}
			}

			// Determine destination zones based on destination addresses
			if isDestAny {
				// For "any" destination, include all available zones
				for _, zone := range p.TopologyZones {
					destZones = append(destZones, zone.Name)
				}
			} else {
				// Find the zone of the interface containing the destination address
				for _, addr := range destAddresses {
					for _, iface := range p.TopologyInterfaces {
						if len(iface.Addresses) == 0 {
							continue
						}
						// Use the isAddressInSubnet helper function
						if p.isAddressInInterfaceSubnet(addr, iface.Addresses[0]) {
							if zone, exists := p.TopologyZones[iface.Name]; exists {
								destZones = append(destZones, zone.Name)
								break
							}
						}
					}
				}
			}

			policy := models.FirewallPolicy{
				ID:                   policyID,
				Name:                 fmt.Sprintf("Extended_ACL_%d_Rule_%d", acl.Number, rule.Order),
				VirtualContext:       p.TopologyVirtualContexts[defaultIOSContextName].Name,
				Enabled:              len(sourceZones) > 0 || len(destZones) > 0, // Only enabled if bound to interface
				SourceZones:          sourceZones,
				DestinationZones:     destZones,
				SourceAddresses:      sourceAddresses,
				DestinationAddresses: destAddresses,
				Services:             services,
				Action:               action,
				Log:                  models.LogConfig{AtStart: rule.Log, AtEnd: rule.Log},
				Description:          rule.Raw,
				RuleOrder:            rule.Order,
				Firewall:             p.iosConfig.Hostname,
				AnySource:            isSourceAny,
				AnyDestination:       isDestAny,
			}

			p.TopologyFirewallPolicies[policyID] = policy
		}
	}

	// Process Named ACLs
	for _, acl := range p.iosConfig.NamedACLs {
		for _, rule := range acl.Rules {
			if rule.Action == "" {
				continue
			}

			policyID := uuid.New().String()
			action := "deny"
			if strings.ToLower(rule.Action) == "permit" {
				action = "allow"
			}

			// Resolve source address
			var sourceAddresses []string
			var isSourceAny bool
			sourceName, sourceValue, _ := p.parseACLAddress(rule.Source, rule.SourceWildcard)
			if sourceName != "" && sourceValue != "" {
				if sourceName == "any" {
					sourceAddresses = append(sourceAddresses, "any")
					isSourceAny = true
				} else {
					sourceAddresses = append(sourceAddresses, sourceValue)
				}
			}

			// Resolve destination address
			var destAddresses []string
			var isDestAny bool
			destName, destValue, _ := p.parseACLAddress(rule.Destination, rule.DestWildcard)
			if destName != "" && destValue != "" {
				if destName == "any" {
					destAddresses = append(destAddresses, "any")
					isDestAny = true
				} else {
					destAddresses = append(destAddresses, destValue)
				}
			}

			// Resolve service
			var services []string
			if rule.Protocol != "" {
				serviceID := fmt.Sprintf("protocol_%s", rule.Protocol)
				if len(rule.DestPorts) > 0 {
					serviceID = fmt.Sprintf("acl_%s_%s_ports_%s", acl.Name, rule.Protocol, strings.Join(rule.DestPorts, "_"))
				}
				services = append(services, serviceID)
			} else {
				services = append(services, "protocol_ip")
			}

			// Find which interfaces this ACL is applied to and determine zones
			var sourceZones, destZones []string

			// Determine source zones based on source addresses and ACL bindings
			if isSourceAny {
				// For "any" source, include all available zones (like ASA logic)
				for _, zone := range p.TopologyZones {
					sourceZones = append(sourceZones, zone.Name)
				}
			} else {
				// For specific source addresses, find zones from ACL bindings or address-to-zone mapping
				// First try ACL interface bindings
				for _, iface := range p.iosConfig.Interfaces {
					for _, accessGroup := range iface.AccessGroups {
						if accessGroup.ACLName == acl.Name {
							if zone, exists := p.TopologyZones[iface.Name]; exists {
								if accessGroup.Direction == "in" {
									sourceZones = append(sourceZones, zone.Name)
								}
							}
						}
					}
				}

				// If no zones from ACL bindings, try to find zone containing the source address
				if len(sourceZones) == 0 {
					for _, addr := range sourceAddresses {
						for _, iface := range p.TopologyInterfaces {
							if len(iface.Addresses) == 0 {
								continue
							}
							if p.isAddressInInterfaceSubnet(addr, iface.Addresses[0]) {
								if zone, exists := p.TopologyZones[iface.Name]; exists {
									sourceZones = append(sourceZones, zone.Name)
									break
								}
							}
						}
					}
				}
			}

			// Determine destination zones based on destination addresses
			if isDestAny {
				// For "any" destination, include all available zones
				for _, zone := range p.TopologyZones {
					destZones = append(destZones, zone.Name)
				}
			} else {
				// Find the zone of the interface containing the destination address
				for _, addr := range destAddresses {
					for _, iface := range p.TopologyInterfaces {
						if len(iface.Addresses) == 0 {
							continue
						}
						// Use the isAddressInSubnet helper function
						if p.isAddressInInterfaceSubnet(addr, iface.Addresses[0]) {
							if zone, exists := p.TopologyZones[iface.Name]; exists {
								destZones = append(destZones, zone.Name)
								break
							}
						}
					}
				}
			}

			policy := models.FirewallPolicy{
				ID:                   policyID,
				Name:                 fmt.Sprintf("Named_ACL_%s_Rule_%d", acl.Name, rule.Order),
				VirtualContext:       p.TopologyVirtualContexts[defaultIOSContextName].Name,
				Enabled:              len(sourceZones) > 0 || len(destZones) > 0, // Only enabled if bound to interface
				SourceZones:          sourceZones,
				DestinationZones:     destZones,
				SourceAddresses:      sourceAddresses,
				DestinationAddresses: destAddresses,
				Services:             services,
				Action:               action,
				Log:                  models.LogConfig{AtStart: rule.Log, AtEnd: rule.Log},
				Description:          rule.Raw,
				RuleOrder:            rule.Order,
				Firewall:             p.iosConfig.Hostname,
				AnySource:            isSourceAny,
				AnyDestination:       isDestAny,
			}

			p.TopologyFirewallPolicies[policyID] = policy
		}
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d firewall policies from IOS ACLs", len(p.TopologyFirewallPolicies)))

	return nil
}

// buildIOSRoutingModel maps IOS static routes to standard topology routing models
func (p *CiscoIOSProcessor) buildIOSRoutingModel() error {
	for _, route := range p.iosConfig.Routes {
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

		routingEntry := models.RoutingEntry{
			ID:             routeID,
			VirtualContext: p.TopologyVirtualContexts[defaultIOSContextName].Name,
			Destination:    destination,
			NextHop:        route.NextHop,
			Interface:      route.Interface,
			Type:           "static",
			AdminDistance:  route.AdminDistance,
			Name:           fmt.Sprintf("static_route_%s", strings.ReplaceAll(destination, "/", "_")),
		}

		p.TopologyRoutingEntries[routeID] = routingEntry
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d static routing entries", len(p.TopologyRoutingEntries)))

	return nil
}

// buildIOSNATModel maps IOS NAT rules to standard topology NAT models
func (p *CiscoIOSProcessor) buildIOSNATModel() error {
	for i, natRule := range p.iosConfig.NATRules {
		natID := uuid.New().String()

		nat := models.NAT{
			ID:                natID,
			Name:              fmt.Sprintf("NAT_Rule_%d", i+1),
			VirtualContext:    p.TopologyVirtualContexts[defaultIOSContextName].Name,
			Type:              natRule.Type,
			OriginalSource:    natRule.InsideLocal,
			TranslatedSource:  natRule.InsideGlobal,
			InboundInterface:  natRule.InsideInterface,
			OutboundInterface: natRule.OutsideInterface,
			Bidirectional:     false, // IOS NAT is typically unidirectional
		}

		// Handle outside NAT
		if natRule.OutsideLocal != "" {
			nat.OriginalDestination = natRule.OutsideLocal
		}
		if natRule.OutsideGlobal != "" {
			nat.TranslatedDestination = natRule.OutsideGlobal
		}

		p.TopologyNATs[natID] = nat
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Created %d NAT rules", len(p.TopologyNATs)))

	return nil
}

// getOwnedAddresses identifies addresses that are "owned" by this IOS router
func (p *CiscoIOSProcessor) getOwnedAddresses(allAddresses []models.Address, parsedInterfaces []models.Interface) []models.Address {
	ownedAddresses := make(map[string]models.Address)

	// Build a map of interface subnets for quick lookup
	interfaceSubnets := make(map[string][]string)
	for _, interfaceNode := range parsedInterfaces {
		interfaceSubnets[interfaceNode.Name] = interfaceNode.Addresses
	}

	// Check each address to see if it's owned by this router
	for _, address := range allAddresses {
		// Skip addresses that are already marked as owned
		if _, exists := ownedAddresses[address.Name]; exists {
			continue
		}

		// Check if address is directly associated with an interface
		if address.Interface != "" {
			// Address has an explicit interface association, it's owned
			ownedAddresses[address.Name] = address
			continue
		}

		// Check if address falls within any interface subnet
		for _, subnets := range interfaceSubnets {
			for _, subnet := range subnets {
				if processor.IsAddressInSubnet(address.Value, subnet) {
					ownedAddresses[address.Name] = address
					break
				}
			}
			if _, exists := ownedAddresses[address.Name]; exists {
				break
			}
		}
	}

	// Convert map to slice
	var result []models.Address
	for _, address := range ownedAddresses {
		result = append(result, address)
	}

	helper.Logger.Info().Msgf("Found %d owned addresses out of %d total addresses", len(result), len(allAddresses))
	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Identified %d owned addresses out of %d total addresses", len(result), len(allAddresses)))

	return result
}

// isAddressInInterfaceSubnet checks if an address is within an interface subnet
func (p *CiscoIOSProcessor) isAddressInInterfaceSubnet(addressValue, subnet string) bool {
	_, subnetIPNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return false
	}

	// Try parsing address as CIDR first
	_, addrIPNet, err := net.ParseCIDR(addressValue)
	if err == nil {
		// Address is in CIDR format, check if networks overlap
		ones, bits := addrIPNet.Mask.Size()
		if bits == 32 && ones == 32 {
			// /32 subnet, just check if the single IP is in the interface subnet
			return subnetIPNet.Contains(addrIPNet.IP)
		}
		// Check if the address subnet is a subset of the interface subnet
		firstIP := addrIPNet.IP
		lastIP := make(net.IP, len(firstIP))
		copy(lastIP, firstIP)
		for i := range lastIP {
			mask := addrIPNet.Mask[i]
			lastIP[i] |= ^mask
		}
		return subnetIPNet.Contains(firstIP) && subnetIPNet.Contains(lastIP)
	}

	// Try parsing as single IP
	ip := net.ParseIP(addressValue)
	if ip != nil {
		return subnetIPNet.Contains(ip)
	}

	return false
}

// convertWildcardToCIDR converts Cisco wildcard mask format to CIDR notation
func (p *CiscoIOSProcessor) convertWildcardToCIDR(address, wildcard string) (string, error) {
	// Parse the address
	ip := net.ParseIP(address)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", address)
	}

	// Parse the wildcard mask
	wildIP := net.ParseIP(wildcard)
	if wildIP == nil {
		return "", fmt.Errorf("invalid wildcard mask: %s", wildcard)
	}

	// Convert wildcard mask to subnet mask by inverting the bits
	wildcardBytes := wildIP.To4()
	if wildcardBytes == nil {
		return "", fmt.Errorf("wildcard mask is not IPv4: %s", wildcard)
	}

	// Invert the wildcard mask to get the subnet mask
	subnetMask := make([]byte, 4)
	for i := 0; i < 4; i++ {
		subnetMask[i] = ^wildcardBytes[i]
	}

	// Count the number of leading 1-bits in the subnet mask
	ones := 0
	for i := 0; i < 4; i++ {
		for j := 7; j >= 0; j-- {
			if (subnetMask[i] & (1 << j)) != 0 {
				ones++
			} else {
				// Once we hit a 0 bit, stop counting
				goto done
			}
		}
	}
done:

	// Get the network address by applying the subnet mask
	networkIP := ip.Mask(net.IPMask(subnetMask))
	if networkIP == nil {
		return "", fmt.Errorf("failed to calculate network address")
	}

	return fmt.Sprintf("%s/%d", networkIP.String(), ones), nil
}

// CiscoIOSMapper creates the final ParsedNetworkTopology from the processor's topology models
func CiscoIOSMapper(p *CiscoIOSProcessor) models.ParsedNetworkTopology {
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
		FirewallName:     p.iosConfig.Hostname,
		Metadata: models.NodeMetadata{
			Parser:     processor.ParserTypeCiscoIOS,
			SourceType: constants.SOURCE_TYPE_VENDOR,
		},
		InsertedTime: time.Now().Format(time.RFC3339),
		LastModified: time.Now().Format(time.RFC3339),
		Active:       true,
	}
}

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |L|2| |M|a|p|p|e|r| |F|u|n|c|t|i|o|n|s|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The following methods map Cisco L2 configurations to vendor-agnostic Layer-2 models.
 * They support switch topology, VLAN cataloging, security posture, and host placement.
 */

// buildL2SecurityControls maps L2 security data (DHCP snooping, 802.1X, DAI, IPSG, port security) to models.
// This function processes operational data from security-related show commands and populates
// per-interface security state for the L2 decision engine.
func (p *CiscoIOSProcessor) buildL2SecurityControls(
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

	// Build per-interface security state
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

	// Aggregate all interfaces from all sources
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
			// Note: Trust status would come from config parsing
		}

		// Authentication (802.1X/MAB)
		if sessions, exists := authByInterface[iface]; exists && len(sessions) > 0 {
			session := sessions[0] // Use first session for interface-level status
			security.Dot1XEnabled = true
			security.AuthenticationStatus = session.Status
			security.AuthenticatedMAC = session.MACAddress
			security.AuthenticationMethod = session.Method

			// Determine if MAB is enabled
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

		// Calculate security score (0-100) based on enabled controls
		security.SecurityScore = calculateInterfaceSecurityScore(security)
		security.SecurityLevel = classifySecurityLevel(security.SecurityScore)

		interfaceSecurity[iface] = security
	}

	return globalControls, interfaceSecurity
}

// calculateInterfaceSecurityScore computes a 0-100 security score based on enabled controls.
// Scoring algorithm:
// - DHCP Snooping bindings: +20 (primary evidence source)
// - 802.1X/MAB authentication: +30 (critical for Allowed decisions)
// - DAI trust: +15
// - IPSG enabled: +15
// - Port security enabled: +10
// - PVLAN isolation: +10
func calculateInterfaceSecurityScore(sec CiscoL2InterfaceSecurity) int {
	score := 0

	// DHCP Snooping
	if sec.DHCPSnoopingBindingCount > 0 {
		score += 20
	}

	// Authentication (802.1X/MAB) - CRITICAL for Allowed decisions
	if sec.AuthenticationStatus == "Authc Success" || sec.AuthenticationStatus == "Authorized" {
		score += 30
	} else if sec.Dot1XEnabled || sec.MABEnabled {
		score += 10 // Enabled but not authenticated
	}

	// Dynamic ARP Inspection
	if sec.ARPInspectionTrusted {
		score += 15
	}

	// IP Source Guard
	if sec.IPSGEnabled && sec.IPSGBindingCount > 0 {
		score += 15
	}

	// Port Security
	if sec.PortSecurityEnabled && sec.PortSecurityViolationCount == 0 {
		score += 10
	}

	// Private VLAN isolation
	if sec.PrivateVLANType == "isolated" || sec.PrivateVLANType == "community" {
		score += 10
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}

// classifySecurityLevel maps security score to high/medium/low/none.
func classifySecurityLevel(score int) string {
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

// buildEnhancedFDBEntries creates FDB entries with confidence scoring based on multiple evidence sources.
// Evidence hierarchy (from highest to lowest confidence):
// 1. DHCP snooping binding (primary evidence)
// 2. 802.1X/MAB authentication session
// 3. IPSG binding
// 4. Port security learned MAC
// 5. MAC table entry alone
func (p *CiscoIOSProcessor) buildEnhancedFDBEntries(
	macTable []models.MACTableEntry,
	dhcpBindings []DHCPSnoopingBinding,
	authSessions []AuthenticationSession,
	ipsgBindings []IPVerifySourceBinding,
	arpEntries []models.ARPEntry,
) []CiscoFDBEntry {

	// Create lookup maps for evidence sources
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

	arpByMAC := make(map[string]models.ARPEntry)
	for _, entry := range arpEntries {
		arpByMAC[strings.ToLower(entry.MACAddress)] = entry
	}

	// Build enhanced FDB entries
	var fdbEntries []CiscoFDBEntry

	for _, mac := range macTable {
		macLower := strings.ToLower(mac.MACAddress)

		fdb := CiscoFDBEntry{
			DeviceID:        p.iosConfig.Hostname,
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

		// Check DHCP snooping binding (PRIMARY evidence)
		if dhcp, exists := dhcpByMAC[macLower]; exists {
			fdb.DHCPSnoopingBound = true
			fdb.IPAddress = dhcp.IPAddress
			fdb.DHCPLeaseTime = dhcp.LeaseSeconds
			fdb.EvidenceSources = append(fdb.EvidenceSources, "dhcp-snooping")
			fdb.ConfidenceScore += 40 // DHCP snooping is highest confidence
		}

		// Check 802.1X/MAB authentication (CRITICAL for Allowed decisions)
		if auth, exists := authByMAC[macLower]; exists {
			fdb.Authenticated = true
			fdb.AuthMethod = auth.Method
			if auth.IPv4Address != "" && auth.IPv4Address != "Unknown" {
				fdb.IPAddress = auth.IPv4Address
			}
			fdb.EvidenceSources = append(fdb.EvidenceSources, "dot1x")

			// Authenticated sessions get high confidence boost
			if auth.Status == "Authc Success" || auth.Status == "Authorized" {
				fdb.ConfidenceScore += 35
			} else {
				fdb.ConfidenceScore += 15 // Auth enabled but not successful
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

		// Base confidence from MAC table entry itself
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

		// Classify freshness based on MAC table age
		fdb.Freshness = classifyFreshnessFromAge(mac.Age)

		fdbEntries = append(fdbEntries, fdb)
	}

	return fdbEntries
}

// classifyFreshnessFromAge determines freshness tier based on MAC table age in seconds.
func classifyFreshnessFromAge(ageSeconds int) string {
	switch {
	case ageSeconds <= 600: // 10 minutes
		return "fresh"
	case ageSeconds <= 3600: // 1 hour
		return "stale"
	default:
		return "old"
	}
}

// buildL2DecisionCache builds cached L2 decisions (Allowed/Conditional/Blocked) for performance.
// Decision logic:
// - Allowed: Authenticated (802.1X/MAB success) + DHCP snooping bound + high confidence
// - Conditional: DHCP snooping bound OR authenticated but not both, medium confidence
// - Blocked: No authentication, no DHCP binding, or security violations detected
func (p *CiscoIOSProcessor) buildL2DecisionCache(
	fdbEntries []CiscoFDBEntry,
	interfaceSecurity map[string]CiscoL2InterfaceSecurity,
	ttlSeconds int,
) []L2DecisionCacheEntry {

	var decisions []L2DecisionCacheEntry
	now := time.Now()

	for _, fdb := range fdbEntries {
		// Get interface security state
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
			Metadata:            make(map[string]string),
		}

		// Populate security controls state
		decision.Authenticated = fdb.Authenticated
		decision.DHCPSnoopingBound = fdb.DHCPSnoopingBound
		decision.IPSGValid = fdb.IPSGBound
		decision.PortSecurityValid = fdb.PortSecurityLearned

		// ARPInspectionValid requires correlation with DAI state (not implemented yet)
		decision.ARPInspectionValid = false

		// Calculate security score
		if hasSecurityState {
			decision.SecurityScore = ifaceSec.SecurityScore
		} else {
			decision.SecurityScore = fdb.ConfidenceScore
		}

		// Decision logic
		decision.DecisionConfidence = fdb.ConfidenceScore

		// ALLOWED: Authenticated + DHCP bound + high confidence
		if decision.Authenticated && decision.DHCPSnoopingBound && fdb.ConfidenceScore >= 70 {
			decision.Decision = "Allowed"
			decision.DecisionReason = "Authenticated (802.1X/MAB) + DHCP snooping bound + high confidence"
		} else if decision.Authenticated && fdb.ConfidenceScore >= 60 {
			// CONDITIONAL: Authenticated but missing DHCP binding
			decision.Decision = "Conditional"
			decision.DecisionReason = "Authenticated but missing DHCP snooping binding"
		} else if decision.DHCPSnoopingBound && fdb.ConfidenceScore >= 50 {
			// CONDITIONAL: DHCP bound but not authenticated
			decision.Decision = "Conditional"
			decision.DecisionReason = "DHCP snooping bound but not authenticated"
		} else if fdb.ConfidenceScore >= 40 {
			// CONDITIONAL: Medium confidence with partial evidence
			decision.Decision = "Conditional"
			decision.DecisionReason = fmt.Sprintf("Medium confidence (%d) with partial evidence: %v",
				fdb.ConfidenceScore, fdb.EvidenceSources)
		} else {
			// BLOCKED: Low confidence or security violations
			decision.Decision = "Blocked"
			decision.DecisionReason = fmt.Sprintf("Low confidence (%d), insufficient evidence",
				fdb.ConfidenceScore)

			// Check for security violations
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

// buildIOSLayer2Models converts the parsed Cisco L2 config into vendor-agnostic models.
// L2 visibility focus: emits switch topology, device fingerprinting, and host placement records.
func (p *CiscoIOSProcessor) buildIOSLayer2Models(cfg models.CiscoL2Config) (models.L2Switch, []models.FDBEntry) {
	l2Switch := models.L2Switch{
		DeviceID:     cfg.Hostname,
		Vendor:       models.IntegrationCisco.String(),
		OSName:       "ios",
		OSVersion:    cfg.Version,
		ConfigHash:   cfg.ConfigHash,
		VLANs:        make([]models.L2VLAN, 0, len(cfg.VLANs)),
		Interfaces:   make([]models.L2SwitchPort, 0, len(cfg.Interfaces)),
		PortChannels: make([]models.L2PortChannel, 0, len(cfg.PortChannels)),
		SVIs:         make([]models.L3HandoffMarker, 0, len(cfg.SVIs)),
	}

	now := time.Now().Format(time.RFC3339)
	l2Switch.InsertedTime = now
	l2Switch.LastModified = now
	l2Switch.Active = true

	// VLANs (Layer-2 segmentation inventory for attack-path reasoning)
	for _, vlan := range cfg.VLANs {
		l2Switch.VLANs = append(l2Switch.VLANs, models.L2VLAN{
			VLANID:            vlan.ID,
			VLANName:          vlan.Name,
			State:             vlan.State,
			VACLName:          vlan.VACLName,
			SourceBlock:       vlan.SourceBlock,
			SourceSnippetHash: vlan.SourceSnippetHash,
		})
	}

	// Interfaces (port posture: VLAN propagation, security controls, device evidence)
	for _, iface := range cfg.Interfaces {
		port := models.L2SwitchPort{
			InterfaceName:        iface.Name,
			AdminState:           iface.AdminState,
			Mode:                 iface.Mode,
			TrunkAllowedVLANs:    iface.TrunkAllowedVLANs,
			TrunkNativeVLAN:      iface.TrunkNativeVLAN,
			PortChannelID:        iface.PortChannelID,
			DHCPSnoopingTrust:    iface.DHCPSnoopingTrust,
			ARPInspectionTrust:   iface.ARPInspectionTrust,
			IPSourceGuard:        iface.IPSourceGuard,
			Dot1xRequired:        iface.Dot1xRequired,
			MABEnabled:           iface.MABEnabled,
			UnauthVLAN:           iface.UnauthVLAN,
			PVLANRole:            iface.PVLANRole,
			PVLANPrimaryVLAN:     iface.PVLANPrimaryVLAN,
			PVLANSecondaryVLAN:   iface.PVLANSecondaryVLAN,
			Description:          iface.Description,
			SourceBlock:          iface.SourceBlock,
			SourceSnippetHash:    iface.SourceSnippetHash,
		}
		if iface.AccessVLAN != nil {
			port.AccessVLAN = iface.AccessVLAN
		}
		l2Switch.Interfaces = append(l2Switch.Interfaces, port)
	}

	// Port-channels (L2 visibility requirement: bundled links impact reachability paths)
	for _, po := range cfg.PortChannels {
		l2Switch.PortChannels = append(l2Switch.PortChannels, models.L2PortChannel{
			PortChannelID:     po.ID,
			Name:              po.Name,
			MemberInterfaces:  po.MemberInterfaces,
			Mode:              po.Mode,
			AccessVLAN:        po.AccessVLAN,
			TrunkAllowedVLANs: po.TrunkAllowedVLANs,
			TrunkNativeVLAN:   po.TrunkNativeVLAN,
			SourceBlock:       po.SourceBlock,
			SourceSnippetHash: po.SourceSnippetHash,
		})
	}

	// Global controls (device-level integrity toggles for Conditional/Blocked decisions)
	l2Switch.GlobalL2Controls = models.L2GlobalControls{
		SnoopingEnabled:        cfg.GlobalL2Controls.SnoopingEnabled,
		SnoopingVLANs:          cfg.GlobalL2Controls.SnoopingVLANs,
		DAIEnabledVLANs:        cfg.GlobalL2Controls.DAIEnabledVLANs,
		IPSGGlobal:             cfg.GlobalL2Controls.IPSGGlobal,
		Dot1xSystemAuthControl: cfg.GlobalL2Controls.Dot1xSystemAuthControl,
		MABGlobal:              cfg.GlobalL2Controls.MABGlobal,
		SourceBlock:            cfg.GlobalL2Controls.SourceBlock,
		SourceSnippetHash:      cfg.GlobalL2Controls.SourceSnippetHash,
	}

	// SVIs (L3 handoff markers to stitch L2 segments into routed attack paths)
	for _, svi := range cfg.SVIs {
		l2Switch.SVIs = append(l2Switch.SVIs, models.L3HandoffMarker{
			SVIID:             svi.VLANID,
			InterfaceName:     svi.InterfaceName,
			IPAddress:         svi.IPAddress,
			Subnet:            svi.Subnet,
			Description:       svi.Description,
			SourceBlock:       svi.SourceBlock,
			SourceSnippetHash: svi.SourceSnippetHash,
		})
	}

	// Prepare FDB entries (host-to-port evidence driving placement confidence)
	var fdbEntries []models.FDBEntry
	for _, entry := range cfg.FDBEntries {
		fdbEntries = append(fdbEntries, models.FDBEntry{
			DeviceID:       cfg.Hostname,
			MAC:            entry.MAC,
			LastSeenPort:   entry.LastSeenPort,
			VLANID:         entry.VLANID,
			LastSeenTS:     entry.LastSeenTS,
			EvidenceSource: entry.EvidenceSource,
			Confidence:     entry.Confidence,
			Freshness:      classifyFreshness(entry.LastSeenTS),
		})
	}

	return l2Switch, fdbEntries
}

// classifyFreshness determines freshness tier based on timestamp recency.
// Supports freshness-based decision downgrade per spec §6.
func classifyFreshness(ts string) string {
	if ts == "" {
		return "unknown"
	}
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "unknown"
	}
	delta := time.Since(parsed)
	switch {
	case delta <= 10*time.Minute:
		return "fresh"
	case delta <= time.Hour:
		return "stale"
	default:
		return "old"
	}
}

// persistLayer2Models persists Layer-2 switch/device footprints and host placements to storage.
func (p *CiscoIOSProcessor) persistLayer2Models(l2Switch models.L2Switch, fdbEntries []models.FDBEntry) error {
	if err := p.Store.SaveLayer2Switch(l2Switch); err != nil {
		return fmt.Errorf("save L2 switch: %w", err)
	}
	if len(fdbEntries) > 0 {
		if err := p.Store.BulkUpsertFDBEntries(fdbEntries); err != nil {
			return fmt.Errorf("upsert FDB entries: %w", err)
		}
	}
	return nil
}
