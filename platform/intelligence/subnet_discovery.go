package intelligence

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// SubnetDiscovery — groups devices by IP prefixes and infers gateways
// ========================================================================

// SubnetDiscovery groups devices into subnets based on their IP addresses
// and infers gateway devices for each subnet.
type SubnetDiscovery struct {
	ctx *InferenceContext
}

// NewSubnetDiscovery creates a new subnet discovery engine.
func NewSubnetDiscovery(ctx *InferenceContext) *SubnetDiscovery {
	return &SubnetDiscovery{ctx: ctx}
}

// ========================================================================
// Subnet Grouping
// ========================================================================

// DiscoverSubnets groups devices by common IP prefixes.
// Uses both interface IPs and management IPs.
func (sd *SubnetDiscovery) DiscoverSubnets() []SubnetGroup {
	// Collect all IPs and group by /24 subnets (most common in enterprise)
	// Also try /16 and /8 for larger networks
	subnetMap := make(map[string]*SubnetGroup)

	// Collect all device IPs
	type deviceIP struct {
		Device    string
		Interface string
		IP        string
		Subnet    string
	}
	var allIPs []deviceIP

	// From interfaces
	for key, iface := range sd.ctx.Interfaces {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		device := parts[0]
		for _, addr := range iface.Addresses {
			ip := strings.Split(addr, "/")[0]
			subnet := sd.inferSubnet(addr)
			if subnet != "" {
				allIPs = append(allIPs, deviceIP{
					Device:    device,
					Interface: iface.Name,
					IP:        ip,
					Subnet:    subnet,
				})
			}
		}
	}

	// From devices (management IPs)
	for hostname, dev := range sd.ctx.Devices {
		if dev.ManagementIP != "" {
			subnet := sd.inferSubnet(dev.ManagementIP + "/24")
			if subnet != "" {
				allIPs = append(allIPs, deviceIP{
					Device: hostname,
					IP:     dev.ManagementIP,
					Subnet: subnet,
				})
			}
		}
	}

	// Group by subnet
	for _, dip := range allIPs {
		key := dip.Subnet
		if group, ok := subnetMap[key]; ok {
			group.Devices = appendUnique(group.Devices, dip.Device)
			group.Interfaces = append(group.Interfaces, SubnetInterface{
				Device:    dip.Device,
				Interface: dip.Interface,
				IP:        dip.IP,
			})
		} else {
			subnetMap[key] = &SubnetGroup{
				Subnet:          key,
				Devices:         []string{dip.Device},
				Interfaces:      []SubnetInterface{{Device: dip.Device, Interface: dip.Interface, IP: dip.IP}},
				InferredAt:      time.Now(),
				Confidence:      0.85,
				EvidenceSources: []models.ConfidenceSource{models.ConfidenceSourceInferred},
			}
		}
	}

	// Filter groups with at least 2 devices (meaningful subnets)
	var groups []SubnetGroup
	for _, group := range subnetMap {
		if len(group.Devices) >= 1 { // Include even single-device subnets for completeness
			groups = append(groups, *group)
		}
	}

	// Sort by subnet
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Subnet < groups[j].Subnet
	})

	return groups
}

// inferSubnet extracts a /24 subnet from an IP address.
// For more precise CIDR notations (e.g., 10.1.1.1/28), uses the exact prefix.
func (sd *SubnetDiscovery) inferSubnet(addr string) string {
	// Try parsing as CIDR
	_, network, err := net.ParseCIDR(addr)
	if err != nil {
		// Try as bare IP with default /24
		ip := net.ParseIP(addr)
		if ip == nil {
			return ""
		}
		// Default to /24
		_, network, err = net.ParseCIDR(ip.String() + "/24")
		if err != nil {
			return ""
		}
	}
	return network.String()
}

// ========================================================================
// Gateway Inference
// ========================================================================

// InferGateway determines the likely gateway device for a subnet.
// Looks for:
//   1. A device with an interface IP ending in .1 or .254 (common gateway convention)
//   2. A device whose routing table shows a connected route for this subnet
//   3. A device configured as a router/firewall with an interface in this subnet
func (sd *SubnetDiscovery) InferGateway(group SubnetGroup) (device, ip string) {
	if len(group.Interfaces) == 0 {
		return "", ""
	}

	// Strategy 1: Look for common gateway IPs (.1, .254)
	for _, iface := range group.Interfaces {
		ip := net.ParseIP(iface.IP)
		if ip == nil {
			continue
		}
		lastOctet := ip.To4()[3]
		if lastOctet == 1 || lastOctet == 254 {
			// Check if this device has routing capabilities
			if sd.hasRoutingCapability(iface.Device) {
				return iface.Device, iface.IP
			}
		}
	}

	// Strategy 2: Check routing tables for connected routes to this subnet
	for _, rt := range sd.ctx.RoutingTables {
		for _, route := range rt.Routes {
			if route.Protocol == "connected" || route.Protocol == "local" {
				if route.Prefix == group.Subnet || sd.subnetMatches(route.Prefix, group.Subnet) {
					return rt.Hostname, ""
				}
			}
		}
	}

	// Strategy 3: Any device with routing entries is a candidate
	for _, iface := range group.Interfaces {
		if sd.hasRoutingCapability(iface.Device) {
			return iface.Device, iface.IP
		}
	}

	return "", ""
}

// hasRoutingCapability checks if a device has routing table entries.
func (sd *SubnetDiscovery) hasRoutingCapability(device string) bool {
	for _, rt := range sd.ctx.RoutingTables {
		if rt.Hostname == device && len(rt.Routes) > 0 {
			return true
		}
	}
	// Also check routing entries
	for _, re := range sd.ctx.RoutingEntries {
		if re.ID == device || re.VirtualContext == device {
			return true
		}
	}
	return false
}

// subnetMatches checks if two subnet strings represent the same or overlapping networks.
func (sd *SubnetDiscovery) subnetMatches(a, b string) bool {
	_, netA, err := net.ParseCIDR(a)
	if err != nil {
		return false
	}
	_, netB, err := net.ParseCIDR(b)
	if err != nil {
		return false
	}
	return netA.String() == netB.String()
}

// ========================================================================
// Extended Subnet Analysis
// ========================================================================

// InferVLANForSubnet attempts to associate a VLAN with a subnet group
// by matching device interfaces to their VLAN assignments.
func (sd *SubnetDiscovery) InferVLANForSubnet(group *SubnetGroup) {
	for _, l2s := range sd.ctx.L2Switches {
		for _, iface := range group.Interfaces {
			if l2s.DeviceID == iface.Device || sd.deviceNameMatches(l2s.DeviceID, iface.Device) {
				for _, vlan := range l2s.VLANs {
					// Check if any interface in this group uses this VLAN
					for _, port := range l2s.Interfaces {
						if port.InterfaceName == iface.Interface {
							if port.AccessVLAN != nil {
								group.VLANID = *port.AccessVLAN
								group.VLANName = vlan.VLANName
								return
							}
						}
					}
				}
			}
		}
	}
}

func (sd *SubnetDiscovery) deviceNameMatches(a, b string) bool {
	return strings.EqualFold(a, b) || strings.Contains(strings.ToLower(a), strings.ToLower(b))
}

// DiscoverSubnetsWithAnalysis runs full subnet discovery including VLAN association.
func (sd *SubnetDiscovery) DiscoverSubnetsWithAnalysis() []SubnetGroup {
	groups := sd.DiscoverSubnets()

	for i := range groups {
		// Infer gateway
		gwDevice, gwIP := sd.InferGateway(groups[i])
		groups[i].GatewayDevice = gwDevice
		groups[i].GatewayIP = gwIP

		// Infer VLAN
		sd.InferVLANForSubnet(&groups[i])
	}

	return groups
}

// ========================================================================
// Network Node Integration
// ========================================================================

// DiscoverFromNetworkNodes creates SubnetGroups from existing NetworkNode data.
func (sd *SubnetDiscovery) DiscoverFromNetworkNodes() []SubnetGroup {
	var groups []SubnetGroup

	for _, node := range sd.ctx.NetworkNodes {
		if node.Subnet == "" {
			continue
		}

		group := SubnetGroup{
			Subnet:          node.Subnet,
			InferredAt:      time.Now(),
			Confidence:      0.90, // High confidence from known network nodes
			EvidenceSources: []models.ConfidenceSource{models.ConfidenceSourceConnector},
		}

		// Find devices in this subnet
		_, network, err := net.ParseCIDR(node.Subnet)
		if err != nil {
			continue
		}

		for hostname, dev := range sd.ctx.Devices {
			if dev.ManagementIP != "" {
				ip := net.ParseIP(dev.ManagementIP)
				if ip != nil && network.Contains(ip) {
					group.Devices = append(group.Devices, hostname)
				}
			}
		}

		// Find interfaces in this subnet
		for key, iface := range sd.ctx.Interfaces {
			parts := strings.SplitN(key, ":", 2)
			if len(parts) != 2 {
				continue
			}
			for _, addr := range iface.Addresses {
				ipStr := strings.Split(addr, "/")[0]
				ip := net.ParseIP(ipStr)
				if ip != nil && network.Contains(ip) {
					group.Interfaces = append(group.Interfaces, SubnetInterface{
						Device:    parts[0],
						Interface: iface.Name,
						IP:        ipStr,
					})
					group.Devices = appendUnique(group.Devices, parts[0])
				}
			}
		}

		if len(group.Devices) > 0 {
			groups = append(groups, group)
		}
	}

	// Sort by subnet
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Subnet < groups[j].Subnet
	})

	return groups
}

// Ensure required import for fmt
var _ = fmt.Sprintf
