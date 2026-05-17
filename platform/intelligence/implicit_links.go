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
// ImplicitLinkDetector — finds unconfigured but likely links
// ========================================================================

// ImplicitLinkDetector detects possible direct links between devices
// by analyzing interface IP subnets. If two devices have interfaces
// on the same subnet, they could be directly connected.
type ImplicitLinkDetector struct {
	ctx *InferenceContext
}

// NewImplicitLinkDetector creates a new implicit link detector.
func NewImplicitLinkDetector(ctx *InferenceContext) *ImplicitLinkDetector {
	return &ImplicitLinkDetector{ctx: ctx}
}

// ========================================================================
// Implicit Link Detection
// ========================================================================

// DetectLinks finds implicit links between devices that share subnet membership
// on their interfaces but have no explicit link recorded.
func (ild *ImplicitLinkDetector) DetectLinks() []InferredLink {
	var links []InferredLink

	// Collect all interface IPs grouped by subnet
	subnetInterfaces := ild.groupInterfacesBySubnet()

	// For each subnet with interfaces on multiple devices, infer possible links
	for subnet, ifaces := range subnetInterfaces {
		if len(ifaces) < 2 {
			continue
		}

		// Generate candidate links between all device pairs in this subnet
		devices := ild.uniqueDevices(ifaces)
		for i := 0; i < len(devices); i++ {
			for j := i + 1; j < len(devices); j++ {
				devA := devices[i]
				devB := devices[j]

				// Skip if an explicit link already exists
				if ild.hasExplicitLink(devA, devB) {
					continue
				}

				// Skip if devices are the same
				if devA == devB {
					continue
				}

				// Find specific interfaces for each device in this subnet
				ifacesA := ild.findInterfacesInSubnet(ifaces, devA)
				ifacesB := ild.findInterfacesInSubnet(ifaces, devB)

				if len(ifacesA) == 0 || len(ifacesB) == 0 {
					continue
				}

				// Create implicit link
				link := InferredLink{
					ID:              fmt.Sprintf("implicit-%s-%s-%s", subnet, devA, devB),
					FromDevice:      devA,
					FromPort:        ifacesA[0],
					ToDevice:        devB,
					ToPort:          ifacesB[0],
					LinkType:        "implicit",
					InferenceMethod: "subnet-match",
					Confidence:      ild.computeImplicitConfidence(subnet, devA, devB, ifacesA, ifacesB),
					EvidenceSource:  models.ConfidenceSourceInferred,
					EvidenceDetail: fmt.Sprintf("Devices %s and %s share subnet %s with no explicit link → possible direct connection",
						devA, devB, subnet),
					SupportingData: map[string]interface{}{
						"subnet":             subnet,
						"device_a_ifaces":    ifacesA,
						"device_b_ifaces":    ifacesB,
						"device_count":       len(devices),
						"total_ifaces_in_subnet": len(ifaces),
					},
					InferredAt: time.Now(),
				}

				links = append(links, link)
			}
		}
	}

	return links
}

// groupInterfacesBySubnet collects all interfaces and groups them by their subnet.
func (ild *ImplicitLinkDetector) groupInterfacesBySubnet() map[string][]subnetIface {
	result := make(map[string][]subnetIface)

	for key, iface := range ild.ctx.Interfaces {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		device := parts[0]

		for _, addr := range iface.Addresses {
			subnet := ild.extractSubnet(addr)
			if subnet == "" {
				continue
			}

			result[subnet] = append(result[subnet], subnetIface{
				Device:    device,
				Interface: iface.Name,
				IP:        strings.Split(addr, "/")[0],
			})
		}
	}

	return result
}

// extractSubnet extracts the canonical subnet string from an IP address.
// Handles CIDR notation (e.g., "10.1.1.5/24") and bare IPs.
func (ild *ImplicitLinkDetector) extractSubnet(addr string) string {
	_, network, err := net.ParseCIDR(addr)
	if err != nil {
		ip := net.ParseIP(addr)
		if ip == nil {
			return ""
		}
		_, network, err = net.ParseCIDR(ip.String() + "/24")
		if err != nil {
			return ""
		}
	}
	return network.String()
}

// subnetIface is a lightweight struct for grouping interfaces by subnet.
type subnetIface struct {
	Device    string
	Interface string
	IP        string
}

// uniqueDevices returns unique device names from a slice of subnet interfaces.
func (ild *ImplicitLinkDetector) uniqueDevices(ifaces []subnetIface) []string {
	seen := make(map[string]bool)
	var result []string
	for _, si := range ifaces {
		if !seen[si.Device] {
			seen[si.Device] = true
			result = append(result, si.Device)
		}
	}
	sort.Strings(result)
	return result
}

// findInterfacesInSubnet returns interface names for a specific device in this subnet.
func (ild *ImplicitLinkDetector) findInterfacesInSubnet(ifaces []subnetIface, device string) []string {
	var result []string
	for _, si := range ifaces {
		if si.Device == device {
			result = append(result, si.Interface)
		}
	}
	return result
}

// hasExplicitLink checks if an explicit L2 link already exists between two devices.
func (ild *ImplicitLinkDetector) hasExplicitLink(devA, devB string) bool {
	for _, link := range ild.ctx.L2Links {
		if (link.FromDevice == devA && link.ToDevice == devB) ||
			(link.FromDevice == devB && link.ToDevice == devA) {
			return true
		}
	}
	return false
}

// ========================================================================
// Confidence Scoring for Implicit Links
// ========================================================================

// computeImplicitConfidence calculates confidence for an implicit link.
// Factors:
//   - Smaller subnets (/30, /29) are more likely to be point-to-point links
//   - Larger subnets (/24, /16) are less likely to be direct links
//   - Fewer devices in the subnet increases confidence
//   - Matching interface descriptions (e.g., "uplink", "transit") increase confidence
func (ild *ImplicitLinkDetector) computeImplicitConfidence(
	subnet string,
	devA, devB string,
	ifacesA, ifacesB []string,
) float64 {
	baseConfidence := 0.4 // Default: moderate-low confidence

	// Adjust by subnet size
	prefixLen := ild.getPrefixLen(subnet)
	switch {
	case prefixLen >= 30: // /30 or /31 — likely point-to-point
		baseConfidence += 0.25
	case prefixLen >= 28: // /28 or /29 — small subnet
		baseConfidence += 0.15
	case prefixLen >= 24: // /24 — typical LAN
		baseConfidence += 0.05
	case prefixLen >= 16: // /16 — large network
		baseConfidence -= 0.10
	default: // /8 or larger — very low confidence
		baseConfidence -= 0.20
	}

	// Adjust by device count (fewer devices = more likely direct link)
	// (This is checked upstream when determining candidates)

	// Check for uplink/transit naming patterns
	for _, name := range ifacesA {
		if ild.isUplinkInterface(name) {
			baseConfidence += 0.10
			break
		}
	}
	for _, name := range ifacesB {
		if ild.isUplinkInterface(name) {
			baseConfidence += 0.10
			break
		}
	}

	// Clamp
	if baseConfidence > 0.9 {
		baseConfidence = 0.9
	}
	if baseConfidence < 0.1 {
		baseConfidence = 0.1
	}

	return baseConfidence
}

// getPrefixLen extracts the prefix length from a CIDR subnet string.
func (ild *ImplicitLinkDetector) getPrefixLen(subnet string) int {
	_, network, err := net.ParseCIDR(subnet)
	if err != nil {
		return 24 // default
	}
	ones, _ := network.Mask.Size()
	return ones
}

// isUplinkInterface checks if an interface name suggests an uplink/transit connection.
func (ild *ImplicitLinkDetector) isUplinkInterface(name string) bool {
	lower := strings.ToLower(name)
	patterns := []string{
		"uplink", "transit", "peer", "interconnect", "p2p",
		"ix", "crosslink", "fabric", "backbone",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
