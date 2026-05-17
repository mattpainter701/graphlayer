package eos

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// +-+-+-+-+-+-+-+
// |m|a|p|p|e|r|s|
// +-+-+-+-+-+-+-+
/*
 * The following functions map Arista EOS parsed config to the
 * standard network topology models.
 */

const defaultEOSContextName = "default-eos-context"

// BuildNetworkTopologyModels maps EOS internal models to standard topology models
func (p *AristaEOSProcessor) BuildNetworkTopologyModels() error {
	// Initialize topology maps
	p.TopologyInterfaces = make(map[string]models.Interface)
	p.TopologyAddresses = make(map[string]models.Address)
	p.TopologyAddressGroups = make(map[string]models.AddressGroup)
	p.TopologyServices = make(map[string]models.Service)
	p.TopologyServiceGroups = make(map[string]models.ServiceGroup)
	p.TopologyRoutingEntries = make(map[string]models.RoutingEntry)
	p.TopologyZones = make(map[string]models.Zone)

	// Build device model
	if err := p.buildEOSDeviceModel(); err != nil {
		return err
	}

	// Build interfaces
	if err := p.buildEOSInterfaceModel(); err != nil {
		return err
	}

	// Build address objects
	if err := p.buildEOSAddressModel(); err != nil {
		return err
	}

	// Build routing entries
	if err := p.buildEOSRoutingModel(); err != nil {
		return err
	}

	return nil
}

// buildEOSDeviceModel creates the device model
func (p *AristaEOSProcessor) buildEOSDeviceModel() error {
	p.TopologyDevice = &models.Device{
		Hostname: p.eosConfig.Hostname,
		Vendor:   "arista",
		Model:    p.eosConfig.Model,
		Version:  p.eosConfig.Version,
		Role:     "switch",
		Type:     "switch",
	}

	if p.eosConfig.MLAGConfig != nil {
		p.TopologyDevice.MLAGEnabled = true
		p.TopologyDevice.MLAGDomainID = p.eosConfig.MLAGConfig.DomainID
	}

	return nil
}

// buildEOSInterfaceModel maps EOS interfaces to standard topology interfaces
func (p *AristaEOSProcessor) buildEOSInterfaceModel() error {
	// Build topology interfaces from config interfaces + port channels + SVIs
	for _, iface := range p.eosConfig.Interfaces {
		lowerName := strings.ToLower(iface.Name)
		interfaceType := "physical"
		if strings.HasPrefix(lowerName, "vlan") {
			interfaceType = "vlan"
		} else if strings.HasPrefix(lowerName, "loopback") {
			interfaceType = "loopback"
		} else if strings.HasPrefix(lowerName, "management") {
			interfaceType = "management"
		} else if strings.HasPrefix(lowerName, "vxlan") {
			interfaceType = "tunnel"
		} else if strings.HasPrefix(lowerName, "port-channel") {
			interfaceType = "aggregated"
		}

		role := "lan"
		if strings.Contains(strings.ToLower(iface.Description), "wan") ||
			strings.Contains(strings.ToLower(iface.Description), "uplink") {
			role = "wan"
		}

		ipList := []string{}
		if iface.IPAddress != "" {
			cidr := iface.IPAddress
			if iface.Netmask != "" {
				prefixLen := netmaskToPrefixLen(iface.Netmask)
				if prefixLen > 0 {
					cidr = fmt.Sprintf("%s/%d", iface.IPAddress, prefixLen)
				}
			}
			ipList = append(ipList, cidr)
		}

		status := "up"
		if iface.Shutdown {
			status = "down"
		}

		topoIface := models.Interface{
			Name:        iface.Name,
			Addresses:   ipList,
			Role:        role,
			Type:        interfaceType,
			Description: iface.Description,
			Status:      status,
			VlanID:      iface.VLAN,
		}

		id := fmt.Sprintf("iface_%s", iface.Name)
		p.TopologyInterfaces[id] = topoIface

		// Create zone for this interface
		zone := models.Zone{
			Name:        fmt.Sprintf("zone_%s", iface.Name),
			Description: fmt.Sprintf("EOS zone for interface %s", iface.Name),
			Interfaces:  []string{iface.Name},
			Type:        "layer2",
			HasACLs:     false,
		}
		if !iface.Switchport {
			zone.Type = "layer3"
		}
		p.TopologyZones[iface.Name] = zone

		p.TopologyDevice.Interfaces = append(p.TopologyDevice.Interfaces, topoIface.Name)
	}

	return nil
}

// buildEOSAddressModel creates address objects from interface IPs and routes
func (p *AristaEOSProcessor) buildEOSAddressModel() error {
	addressMap := make(map[string]models.Address)

	// 1. Interface subnets
	for _, iface := range p.eosConfig.Interfaces {
		if iface.IPAddress == "" || iface.Netmask == "" {
			continue
		}
		cidr := iface.IPAddress
		if prefixLen := netmaskToPrefixLen(iface.Netmask); prefixLen > 0 {
			cidr = fmt.Sprintf("%s/%d", iface.IPAddress, prefixLen)
		}
		name := fmt.Sprintf("subnet_%s_%s", iface.Name, strings.ReplaceAll(cidr, "/", "_"))
		addressMap[name] = models.Address{
			Name:        name,
			Value:       cidr,
			Type:        "subnet",
			Description: fmt.Sprintf("Interface %s subnet", iface.Name),
			Interface:   iface.Name,
		}
	}

	// 2. Static route destinations
	for _, route := range p.eosConfig.StaticRoutes {
		cidr := route.Prefix
		if route.PrefixLen > 0 {
			cidr = fmt.Sprintf("%s/%d", route.Prefix, route.PrefixLen)
		}
		name := fmt.Sprintf("route_%s", strings.ReplaceAll(cidr, "/", "_"))
		if _, exists := addressMap[name]; !exists {
			addressMap[name] = models.Address{
				Name:        name,
				Value:       cidr,
				Type:        "subnet",
				Description: fmt.Sprintf("Static route via %s", route.NextHop),
			}
		}

		// Next-hop address
		if route.NextHop != "" && net.ParseIP(route.NextHop) != nil {
			nhName := fmt.Sprintf("nexthop_%s", strings.ReplaceAll(route.NextHop, ".", "_"))
			if _, exists := addressMap[nhName]; !exists {
				addressMap[nhName] = models.Address{
					Name:        nhName,
					Value:       fmt.Sprintf("%s/32", route.NextHop),
					Type:        "host",
					Description: "Static route next-hop",
				}
			}
		}
	}

	for _, addr := range addressMap {
		p.TopologyAddresses[addr.Name] = addr
	}

	return nil
}

// buildEOSRoutingModel maps EOS routes to topology routing entries
func (p *AristaEOSProcessor) buildEOSRoutingModel() error {
	// Process static routes
	for _, route := range p.eosConfig.StaticRoutes {
		cidr := route.Prefix
		if route.PrefixLen > 0 {
			cidr = fmt.Sprintf("%s/%d", route.Prefix, route.PrefixLen)
		}

		entry := models.RoutingEntry{
			Prefix:      cidr,
			NextHop:     route.NextHop,
			Protocol:    "static",
			AdminDist:   route.Distance,
			Interface:   route.Interface,
			Description: route.Name,
			Active:      true,
		}

		id := fmt.Sprintf("static_%s_via_%s", cidr, route.NextHop)
		p.TopologyRoutingEntries[id] = entry
	}

	// Process OSPF routes
	if p.eosConfig.OSPFRouter != nil {
		for _, network := range p.eosConfig.OSPFRouter.Networks {
			cidr := network.Network
			if network.Wildcard != "" {
				cidr = fmt.Sprintf("%s/%s", network.Network, network.Wildcard)
			}
			entry := models.RoutingEntry{
				Prefix:   cidr,
				Protocol: "ospf",
				Area:     network.Area,
				Active:   true,
			}
			id := fmt.Sprintf("ospf_%s_%s", cidr, network.Area)
			p.TopologyRoutingEntries[id] = entry
		}
	}

	// Process BGP neighbors as reachability entries
	if p.eosConfig.BGPConfig != nil {
		for _, peer := range p.eosConfig.BGPConfig.Peers {
			entry := models.RoutingEntry{
				NextHop:   peer.IP,
				Protocol:  "bgp",
				AdminDist: 200,
				Active:    true,
			}
			id := fmt.Sprintf("bgp_peer_%s_as%d", peer.IP, peer.ASN)
			p.TopologyRoutingEntries[id] = entry
		}
	}

	return nil
}

// EOSMapper is the top-level mapping function (matching CiscoIOSMapper pattern)
func EOSMapper(p *AristaEOSProcessor) models.ParsedNetworkTopology {
	topology := models.ParsedNetworkTopology{
		Device:         *p.TopologyDevice,
		Interfaces:     make([]models.Interface, 0, len(p.TopologyInterfaces)),
		Addresses:      make([]models.Address, 0, len(p.TopologyAddresses)),
		AddressGroups:  make([]models.AddressGroup, 0, len(p.TopologyAddressGroups)),
		RoutingEntries: make([]models.RoutingEntry, 0, len(p.TopologyRoutingEntries)),
		Zones:          make([]models.Zone, 0, len(p.TopologyZones)),
	}

	for _, iface := range p.TopologyInterfaces {
		topology.Interfaces = append(topology.Interfaces, iface)
	}
	for _, addr := range p.TopologyAddresses {
		topology.Addresses = append(topology.Addresses, addr)
	}
	for _, group := range p.TopologyAddressGroups {
		topology.AddressGroups = append(topology.AddressGroups, group)
	}
	for _, route := range p.TopologyRoutingEntries {
		topology.RoutingEntries = append(topology.RoutingEntries, route)
	}
	for _, zone := range p.TopologyZones {
		topology.Zones = append(topology.Zones, zone)
	}

	return topology
}
