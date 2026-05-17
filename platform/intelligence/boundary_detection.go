package intelligence

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// BoundaryDetector — finds network edges and NAT boundaries
// ========================================================================

// BoundaryDetector identifies where the managed network ends and
// detects NAT boundaries between private and public IP spaces.
type BoundaryDetector struct {
	ctx *InferenceContext
}

// NewBoundaryDetector creates a new boundary detector.
func NewBoundaryDetector(ctx *InferenceContext) *BoundaryDetector {
	return &BoundaryDetector{ctx: ctx}
}

// ========================================================================
// Network Boundary Detection
// ========================================================================

// Private ranges (RFC 1918, RFC 6598, RFC 3927)
var privateRanges = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"100.64.0.0/10", // CGNAT / RFC 6598
	"169.254.0.0/16", // Link-local
}

// isPrivateIP checks if an IP address falls within private ranges.
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// isPublicIP checks if an IP is globally routable (not private, not loopback, not multicast).
func isPublicIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	if ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}

	return !isPrivateIP(ipStr)
}

// DetectNetworkBoundaries identifies where the managed network ends.
// A boundary exists where a device has interfaces with both private and public IPs.
func (bd *BoundaryDetector) DetectNetworkBoundaries() []NetworkBoundary {
	var boundaries []NetworkBoundary

	for key, iface := range bd.ctx.Interfaces {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		device := parts[0]

		var privateAddrs, publicAddrs []string
		for _, addr := range iface.Addresses {
			ip := strings.Split(addr, "/")[0]
			if isPrivateIP(ip) {
				privateAddrs = append(privateAddrs, ip)
			} else if isPublicIP(ip) {
				publicAddrs = append(publicAddrs, ip)
			}
		}

		// Check if this interface itself is a boundary (has both private and public)
		if len(privateAddrs) > 0 && len(publicAddrs) > 0 {
			boundary := NetworkBoundary{
				BoundaryType:    "internet-edge",
				Device:          device,
				Interface:       iface.Name,
				InsideNetworks:  bd.collectPrivateNetworks(device),
				OutsideNetworks: publicAddrs,
				Confidence:      0.90,
				EvidenceDetail: fmt.Sprintf("Interface %s:%s has both private (%v) and public (%v) IPs",
					device, iface.Name, privateAddrs, publicAddrs),
				InferredAt: time.Now(),
			}
			boundaries = append(boundaries, boundary)
		}
	}

	// Device-level boundary detection: a device with both private-only and public-only interfaces
	boundaries = append(boundaries, bd.detectDeviceLevelBoundaries()...)

	// Default route boundaries: devices with a default route pointing to a public IP
	boundaries = append(boundaries, bd.detectDefaultRouteBoundaries()...)

	return boundaries
}

// detectDeviceLevelBoundaries finds devices that bridge private and public networks.
func (bd *BoundaryDetector) detectDeviceLevelBoundaries() []NetworkBoundary {
	var boundaries []NetworkBoundary

	// Collect all interfaces per device
	deviceIfaces := make(map[string][]*models.Interface)
	for key, iface := range bd.ctx.Interfaces {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		device := parts[0]
		deviceIfaces[device] = append(deviceIfaces[device], iface)
	}

	for device, ifaces := range deviceIfaces {
		hasPrivate := false
		hasPublic := false
		var publicIfaces []string

		for _, iface := range ifaces {
			for _, addr := range iface.Addresses {
				ip := strings.Split(addr, "/")[0]
				if isPrivateIP(ip) {
					hasPrivate = true
				} else if isPublicIP(ip) {
					hasPublic = true
					publicIfaces = append(publicIfaces, iface.Name)
				}
			}
		}

		if hasPrivate && hasPublic && len(publicIfaces) > 0 {
			boundary := NetworkBoundary{
				BoundaryType:    "internet-edge",
				Device:          device,
				Interface:       publicIfaces[0],
				InsideNetworks:  bd.collectPrivateNetworks(device),
				OutsideNetworks: bd.collectPublicIPs(ifaces),
				Confidence:      0.85,
				EvidenceDetail: fmt.Sprintf("Device %s has interfaces in both private and public address spaces → network boundary",
					device),
				InferredAt: time.Now(),
			}
			boundaries = append(boundaries, boundary)
		}
	}

	return boundaries
}

// detectDefaultRouteBoundaries finds boundaries via default route analysis.
func (bd *BoundaryDetector) detectDefaultRouteBoundaries() []NetworkBoundary {
	var boundaries []NetworkBoundary

	for _, rt := range bd.ctx.RoutingTables {
		if rt.DefaultRoute != nil {
			nextHop := rt.DefaultRoute.NextHop
			if isPublicIP(nextHop) {
				boundary := NetworkBoundary{
					BoundaryType:    "internet-edge",
					Device:          rt.Hostname,
					Interface:       rt.DefaultRoute.Interface,
					OutsideNetworks: []string{nextHop},
					Confidence:      0.80,
					EvidenceDetail: fmt.Sprintf("Default route via public next-hop %s → internet boundary",
						nextHop),
					InferredAt: time.Now(),
				}
				boundaries = append(boundaries, boundary)
			}
		}

		// Check individual routes for 0.0.0.0/0
		for _, route := range rt.Routes {
			if route.Prefix == "0.0.0.0" && route.PrefixLength == 0 {
				if isPublicIP(route.NextHop) {
					boundary := NetworkBoundary{
						BoundaryType:    "internet-edge",
						Device:          rt.Hostname,
						Interface:       route.Interface,
						OutsideNetworks: []string{route.NextHop},
						Confidence:      0.80,
						EvidenceDetail: fmt.Sprintf("Default route via public next-hop %s → internet boundary",
							route.NextHop),
						InferredAt: time.Now(),
					}
					boundaries = append(boundaries, boundary)
				}
			}
		}
	}

	return boundaries
}

// ========================================================================
// NAT Boundary Inference
// ========================================================================

// InferNATBoundaries detects NAT boundaries from:
//   1. Explicit NAT rules
//   2. Private IPs on internal interfaces + public IPs on external interfaces
//   3. Routing analysis (traffic from private → public through a device)
func (bd *BoundaryDetector) InferNATBoundaries() []NATBoundary {
	var boundaries []NATBoundary

	// Strategy 1: Explicit NAT rules
	boundaries = append(boundaries, bd.inferFromNATRules()...)

	// Strategy 2: Interface analysis (private inside, public outside)
	boundaries = append(boundaries, bd.inferFromInterfaces()...)

	// Strategy 3: VIP analysis (public VIPs mapping to private IPs)
	boundaries = append(boundaries, bd.inferFromVIPs()...)

	return boundaries
}

// inferFromNATRules creates NAT boundaries from explicit NAT rule configurations.
func (bd *BoundaryDetector) inferFromNATRules() []NATBoundary {
	var boundaries []NATBoundary

	for _, nat := range bd.ctx.NATRules {
		privateIP := nat.OriginalSource
		if privateIP == "" {
			privateIP = nat.OriginalDestination
		}
		publicIP := nat.TranslatedSource
		if publicIP == "" {
			publicIP = nat.TranslatedDestination
		}

		if privateIP == "" || publicIP == "" {
			continue
		}

		boundary := NATBoundary{
			PrivateIP:      privateIP,
			PublicIP:       publicIP,
			NATType:        nat.Type,
			Confidence:     0.95, // Very high confidence from explicit NAT rules
			EvidenceSource: "nat-rule",
			InferredAt:     time.Now(),
		}

		// Try to determine the firewall device
		if nat.InboundInterface != "" {
			boundary.FirewallDevice = bd.findDeviceByInterface(nat.InboundInterface)
		} else if nat.OutboundInterface != "" {
			boundary.FirewallDevice = bd.findDeviceByInterface(nat.OutboundInterface)
		}

		boundaries = append(boundaries, boundary)
	}

	return boundaries
}

// inferFromInterfaces detects NAT boundaries from interface IP analysis.
// If a device has private IPs on internal interfaces and public IPs on external,
// it's likely performing NAT.
func (bd *BoundaryDetector) inferFromInterfaces() []NATBoundary {
	var boundaries []NATBoundary

	for key, iface := range bd.ctx.Interfaces {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		device := parts[0]

		privateIPs := bd.getPrivateIPs(device)
		publicIPs := bd.getPublicIPs(device)

		if len(privateIPs) > 0 && len(publicIPs) > 0 {
			for _, pubIP := range publicIPs {
				for _, privIP := range privateIPs {
					boundary := NATBoundary{
						PrivateIP:      privIP,
						PublicIP:       pubIP,
						FirewallDevice: device,
						NATType:        "inferred",
						Confidence:     0.60, // Medium confidence — NAT is implied but not confirmed
						EvidenceSource: "ip-correlation",
						InferredAt:     time.Now(),
					}
					boundaries = append(boundaries, boundary)
				}
			}
			_ = iface // Used for iteration
		}
	}

	return boundaries
}

// inferFromVIPs creates NAT boundaries from Virtual IP (VIP) configurations.
func (bd *BoundaryDetector) inferFromVIPs() []NATBoundary {
	var boundaries []NATBoundary

	for _, vip := range bd.ctx.VIPs {
		if vip.ExternalIP == "" || vip.MappedIP == "" {
			continue
		}

		boundary := NATBoundary{
			PrivateIP:      vip.MappedIP,
			PublicIP:       vip.ExternalIP,
			NATType:        fmt.Sprintf("vip-%s", vip.Type),
			Confidence:     0.92,
			EvidenceSource: "vip-config",
			InferredAt:     time.Now(),
		}
		boundaries = append(boundaries, boundary)
	}

	return boundaries
}

// ========================================================================
// Helpers
// ========================================================================

func (bd *BoundaryDetector) collectPrivateNetworks(device string) []string {
	var networks []string
	for key, iface := range bd.ctx.Interfaces {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 || parts[0] != device {
			continue
		}
		for _, addr := range iface.Addresses {
			ip := strings.Split(addr, "/")[0]
			if isPrivateIP(ip) {
				networks = append(networks, addr)
			}
		}
	}
	return networks
}

func (bd *BoundaryDetector) collectPublicIPs(ifaces []*models.Interface) []string {
	var ips []string
	for _, iface := range ifaces {
		for _, addr := range iface.Addresses {
			ip := strings.Split(addr, "/")[0]
			if isPublicIP(ip) {
				ips = append(ips, ip)
			}
		}
	}
	return ips
}

func (bd *BoundaryDetector) getPrivateIPs(device string) []string {
	var ips []string
	for key, iface := range bd.ctx.Interfaces {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 || parts[0] != device {
			continue
		}
		for _, addr := range iface.Addresses {
			ip := strings.Split(addr, "/")[0]
			if isPrivateIP(ip) {
				ips = append(ips, ip)
			}
		}
	}
	return ips
}

func (bd *BoundaryDetector) getPublicIPs(device string) []string {
	var ips []string
	for key, iface := range bd.ctx.Interfaces {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 || parts[0] != device {
			continue
		}
		for _, addr := range iface.Addresses {
			ip := strings.Split(addr, "/")[0]
			if isPublicIP(ip) {
				ips = append(ips, ip)
			}
		}
	}
	return ips
}

func (bd *BoundaryDetector) findDeviceByInterface(ifname string) string {
	for key := range bd.ctx.Interfaces {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 && parts[1] == ifname {
			return parts[0]
		}
	}
	return ""
}
