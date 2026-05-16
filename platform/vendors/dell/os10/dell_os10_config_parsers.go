package os10

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |C|o|n|f|i|g| |P|a|r|s|e|r|s|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The following functions parse Dell OS10 configuration data.
 * OS10 configuration is indented block-style similar to Cisco IOS.
 * Each sub-command is indented under its parent.
 * Extracts Layer-2 and Layer-3 objects for topology model consumption.
 */

// ===========================
// Interface Config Parsing
// ===========================

// os10InterfaceBlock captures an OS10 interface stanza's parsed attributes
type os10InterfaceBlock struct {
	Mode         string   // access / trunk / routed
	Access       int
	TrunkAllowed []int
	TrunkNative  int
	ChannelGroup int
	LACPActive   bool
	LACPTimeout  string
	Flowcontrol  string
	MTU          int
	LLDPTransmit bool
	LLDPReceive  bool
	StormControl string
	IPAddress    string
	Netmask      string
	Description  string
	Shutdown     bool
}

// parseOS10InterfaceBlock walks an interface stanza and extracts attributes
func parseOS10InterfaceBlock(lines []string) os10InterfaceBlock {
	data := os10InterfaceBlock{}
	if len(lines) < 1 {
		return data
	}

	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)

		switch {
		case strings.HasPrefix(lower, "description "):
			data.Description = strings.TrimSpace(line[len("description "):])

		case strings.HasPrefix(lower, "shutdown"):
			data.Shutdown = true

		case strings.HasPrefix(lower, "no shutdown"):
			data.Shutdown = false

		case strings.HasPrefix(lower, "mtu "):
			if v, err := strconv.Atoi(strings.Fields(line)[1]); err == nil {
				data.MTU = v
			}

		case strings.HasPrefix(lower, "switchport mode "):
			data.Mode = strings.TrimSpace(lower[len("switchport mode "):])

		case strings.HasPrefix(lower, "switchport access vlan "):
			if v, err := strconv.Atoi(strings.TrimSpace(line[len("switchport access vlan "):])); err == nil {
				data.Access = v
			}

		case strings.HasPrefix(lower, "switchport trunk native vlan "):
			if v, err := strconv.Atoi(strings.TrimSpace(line[len("switchport trunk native vlan "):])); err == nil {
				data.TrunkNative = v
			}

		case strings.HasPrefix(lower, "switchport trunk allowed vlan"):
			remainder := strings.TrimSpace(line[len("switchport trunk allowed vlan"):])
			remainder = strings.TrimPrefix(remainder, "add ")
			remainder = strings.TrimPrefix(remainder, "remove ")
			remainder = strings.TrimPrefix(remainder, "except ")
			allowed := expandOS10VLANList(remainder)
			if len(allowed) > 0 {
				data.TrunkAllowed = mergeUniqueIntsOS10(data.TrunkAllowed, allowed)
			}

		case strings.HasPrefix(lower, "channel-group "):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.Atoi(fields[1]); err == nil {
					data.ChannelGroup = v
				}
			}
			if strings.Contains(lower, "mode active") {
				data.LACPActive = true
			}

		case strings.HasPrefix(lower, "lacp port-priority ") ||
			strings.HasPrefix(lower, "lacp rate "):
			if strings.Contains(lower, "fast") {
				data.LACPTimeout = "fast"
			} else if strings.Contains(lower, "normal") {
				data.LACPTimeout = "normal"
			}

		case lower == "lacp mode active" || strings.Contains(lower, "lacp mode active"):
			data.LACPActive = true

		case strings.HasPrefix(lower, "flowcontrol "):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				data.Flowcontrol = fields[1]
			}

		case strings.HasPrefix(lower, "ip address "):
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				data.IPAddress = fields[2]
				data.Netmask = fields[3]
			} else if len(fields) >= 2 {
				// Handle slash notation: ip address 10.0.0.1/24
				if strings.Contains(fields[len(fields)-1], "/") {
					parts := strings.Split(fields[len(fields)-1], "/")
					data.IPAddress = parts[0]
					prefixLen, _ := strconv.Atoi(parts[1])
					data.Netmask = prefixLenToNetmask(prefixLen)
				} else {
					data.IPAddress = fields[len(fields)-1]
				}
			}

		case strings.HasPrefix(lower, "lldp transmit") || strings.HasPrefix(lower, "no lldp disable"):
			data.LLDPTransmit = true

		case strings.HasPrefix(lower, "lldp receive"):
			data.LLDPReceive = true

		case strings.HasPrefix(lower, "storm-control "):
			data.StormControl = strings.TrimSpace(line[len("storm-control "):])
		}
	}
	return data
}

// expandOS10VLANList parses OS10 VLAN list notation (1,10-12,100-110)
func expandOS10VLANList(raw string) []int {
	if raw == "" {
		return nil
	}
	raw = strings.ReplaceAll(raw, " ", "")
	tokens := strings.Split(raw, ",")
	var result []int
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if strings.Contains(token, "-") {
			parts := strings.Split(token, "-")
			if len(parts) != 2 {
				continue
			}
			start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 != nil || err2 != nil || end < start {
				continue
			}
			for v := start; v <= end; v++ {
				result = append(result, v)
			}
		} else if v, err := strconv.Atoi(token); err == nil {
			result = append(result, v)
		}
	}
	if len(result) == 0 {
		return nil
	}
	// sort and dedup
	sorted := make([]int, len(result))
	copy(sorted, result)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	dedup := sorted[:0]
	for _, v := range sorted {
		if len(dedup) == 0 || dedup[len(dedup)-1] != v {
			dedup = append(dedup, v)
		}
	}
	return dedup
}

// mergeUniqueIntsOS10 merges two int slices with dedup
func mergeUniqueIntsOS10(left, right []int) []int {
	set := make(map[int]struct{}, len(left)+len(right))
	for _, v := range left {
		set[v] = struct{}{}
	}
	for _, v := range right {
		set[v] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	merged := make([]int, 0, len(set))
	for v := range set {
		merged = append(merged, v)
	}
	// sort
	for i := 0; i < len(merged)-1; i++ {
		for j := i + 1; j < len(merged); j++ {
			if merged[i] > merged[j] {
				merged[i], merged[j] = merged[j], merged[i]
			}
		}
	}
	return merged
}

// prefixLenToNetmask converts CIDR prefix length to dotted netmask
func prefixLenToNetmask(prefixLen int) string {
	if prefixLen < 0 || prefixLen > 32 {
		return ""
	}
	mask := net.CIDRMask(prefixLen, 32)
	return net.IP(mask).String()
}

// ===========================
// VLAN Config Parsing
// ===========================

// parseOS10VLANBlock extracts VLAN info from an "interface vlan X" stanza
func parseOS10VLANBlock(lines []string, vlanID int) models.OS10VLAN {
	vlan := models.OS10VLAN{
		ID:    vlanID,
		State: "active",
	}
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "description ") {
			vlan.Name = strings.TrimSpace(line[len("description "):])
		} else if strings.HasPrefix(lower, "name ") {
			vlan.Name = strings.TrimSpace(line[len("name "):])
		} else if strings.HasPrefix(lower, "shutdown") {
			vlan.State = "suspend"
		}
	}
	return vlan
}

// ===========================
// VLT Config Parsing
// ===========================

// parseOS10VLTConfig extracts VLT configuration from "vlt-domain" stanzas
func parseOS10VLTConfig(lines []string) *models.OS10VLTConfig {
	vlt := &models.OS10VLTConfig{
		VLANs:  []int{},
		VLTIDs: make(map[int]int),
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}
		lower := strings.ToLower(line)

		switch {
		case strings.HasPrefix(lower, "vlt-domain "):
			vlt.DomainID = strings.TrimSpace(line[len("vlt-domain "):])

		case strings.HasPrefix(lower, "discovery-interface "):
			vlt.DiscoveryInterface = strings.TrimSpace(line[len("discovery-interface "):])

		case strings.HasPrefix(lower, "peer-link "):
			vlt.PeerLink = strings.TrimSpace(line[len("peer-link "):])

		case strings.HasPrefix(lower, "backup destination "):
			vlt.BackupDestination = strings.TrimSpace(line[len("backup destination "):])

		case strings.HasPrefix(lower, "vlt-mac "):
			vlt.VLTMACAddress = strings.TrimSpace(line[len("vlt-mac "):])

		case strings.HasPrefix(lower, "delay-restore "):
			if v, err := strconv.Atoi(strings.Fields(line)[1]); err == nil {
				vlt.DelayRestore = v
			}

		case strings.HasPrefix(lower, "peer-routing"):
			vlt.PeerRouting = true

		case strings.HasPrefix(lower, "peer-routing-timeout "):
			if v, err := strconv.Atoi(strings.Fields(line)[1]); err == nil {
				vlt.PeerRoutingTimeout = v
			}

		case strings.HasPrefix(lower, "backup heartbeat-interval "):
			if v, err := strconv.Atoi(strings.Fields(line)[1]); err == nil {
				vlt.BackupHeartbeatInterval = v
			}
		}
	}

	if vlt.DomainID == "" && vlt.PeerLink == "" {
		return nil
	}
	vlt.Enabled = true
	return vlt
}

// ===========================
// Static Route Parsing
// ===========================

// parseOS10StaticRoute parses a single "ip route" command
func parseOS10StaticRoute(line string) models.OS10StaticRoute {
	route := models.OS10StaticRoute{Raw: line}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return route
	}

	idx := 1 // after "ip route"

	if idx < len(fields) {
		route.Prefix = fields[idx]
		idx++
	}
	if idx < len(fields) {
		mask := fields[idx]
		if strings.Contains(mask, ".") {
			route.PrefixLen = netmaskToPrefixLenOS10(mask)
			idx++
		} else if strings.Contains(mask, "/") {
			parts := strings.Split(mask, "/")
			route.Prefix = parts[0]
			if len(parts) > 1 {
				if v, err := strconv.Atoi(parts[1]); err == nil {
					route.PrefixLen = v
				}
			}
			idx++
		} else if v, err := strconv.Atoi(mask); err == nil && v <= 32 {
			route.PrefixLen = v
			idx++
		}
	}

	// Parse optional fields
	for idx < len(fields) {
		f := fields[idx]
		switch {
		case strings.Contains(f, ".") && net.ParseIP(f) != nil:
			if route.NextHop == "" {
				route.NextHop = f
			}
			idx++
		case f == "vrf" && idx+1 < len(fields):
			route.VRF = fields[idx+1]
			idx += 2
		case f == "name" && idx+1 < len(fields):
			route.Name = fields[idx+1]
			idx += 2
		case f == "tag" && idx+1 < len(fields):
			if v, err := strconv.Atoi(fields[idx+1]); err == nil {
				route.Tag = v
			}
			idx += 2
		case f == "interface" && idx+1 < len(fields):
			route.Interface = fields[idx+1]
			idx += 2
		default:
			if v, err := strconv.Atoi(f); err == nil && route.Distance == 0 {
				route.Distance = v
			}
			idx++
		}
	}
	return route
}

// netmaskToPrefixLenOS10 converts a dotted netmask to CIDR prefix length
func netmaskToPrefixLenOS10(mask string) int {
	maskIP := net.ParseIP(mask)
	if maskIP == nil {
		return 0
	}
	maskIPv4 := maskIP.To4()
	if maskIPv4 == nil {
		return 0
	}
	ones, _ := net.IPv4Mask(maskIPv4[0], maskIPv4[1], maskIPv4[2], maskIPv4[3]).Size()
	return ones
}

// ===========================
// ACL Parsing
// ===========================

// parseOS10ExtendedACL parses an "ip access-list" block
func parseOS10ExtendedACL(lines []string) models.OS10ExtendedACL {
	acl := models.OS10ExtendedACL{}
	if len(lines) == 0 {
		return acl
	}

	header := strings.TrimSpace(lines[0])
	fields := strings.Fields(header)
	if len(fields) >= 3 {
		acl.Name = fields[2]
	} else if len(fields) >= 2 {
		acl.Name = fields[1]
	}

	order := 10
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}

		rule := models.OS10ExtendedACLRule{
			Order: order,
			Raw:   line,
		}
		order += 10

		tokens := strings.Fields(line)
		if len(tokens) < 3 {
			continue
		}

		idx := 0
		// Optional sequence number
		if v, err := strconv.Atoi(tokens[idx]); err == nil && v < 1000 {
			rule.Order = v
			idx++
		}

		if idx < len(tokens) {
			rule.Action = tokens[idx]
			idx++
		}
		if idx < len(tokens) {
			rule.Protocol = tokens[idx]
			idx++
		}

		// Parse source
		if idx+1 < len(tokens) {
			source := tokens[idx]
			if source == "host" {
				idx++
				source = tokens[idx]
				rule.Source = source + "/32"
			} else {
				rule.Source = source
				if idx+1 < len(tokens) && !strings.Contains(tokens[idx+1], ".") {
					rule.SourceWildcard = getWildcardForOS10(rule.Source)
				} else if idx+1 < len(tokens) {
					rule.SourceWildcard = tokens[idx+1]
					idx++
				}
			}
			idx++
		}

		// Source port qualifiers
		for idx < len(tokens) && isPortQualifierOS10(tokens[idx]) {
			q := tokens[idx]
			idx++
			if idx < len(tokens) {
				rule.SourcePorts = append(rule.SourcePorts, q+" "+tokens[idx])
				idx++
			}
		}

		// Destination
		if idx < len(tokens) {
			dest := tokens[idx]
			if dest == "host" {
				idx++
				dest = tokens[idx]
				rule.Dest = dest + "/32"
			} else {
				rule.Dest = dest
				if idx+1 < len(tokens) && !strings.Contains(tokens[idx+1], ".") {
					rule.DestWildcard = getWildcardForOS10(rule.Dest)
				} else if idx+1 < len(tokens) {
					rule.DestWildcard = tokens[idx+1]
					idx++
				}
			}
			idx++
		}

		// Destination port qualifiers
		for idx < len(tokens) && isPortQualifierOS10(tokens[idx]) {
			q := tokens[idx]
			idx++
			if idx < len(tokens) {
				rule.DestPorts = append(rule.DestPorts, q+" "+tokens[idx])
				idx++
			}
		}

		// Log / established
		for idx < len(tokens) {
			if tokens[idx] == "log" {
				rule.Log = true
			}
			if tokens[idx] == "established" {
				rule.Established = true
			}
			idx++
		}

		acl.Rules = append(acl.Rules, rule)
	}
	return acl
}

// ===========================
// Port Security Parsing
// ===========================

// OS10PortSecurityEntry represents port-security state
type OS10PortSecurityEntry struct {
	Interface         string
	MaxMACCount       int
	CurrentMACCount   int
	SecurityViolation string
	Status            string
	ViolationCount    int
	LastSource        string
	CollectionTS      string
}

// ParseOS10PortSecurity parses "show port-security" output for OS10
func ParseOS10PortSecurity(output string, collectionTime time.Time) []OS10PortSecurityEntry {
	var entries []OS10PortSecurityEntry
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Interface") ||
			strings.HasPrefix(line, "---") || strings.HasPrefix(line, "Maximum") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		entry := OS10PortSecurityEntry{
			Interface: fields[0],
			Status:    fields[1],
		}
		if v, err := strconv.Atoi(fields[2]); err == nil {
			entry.MaxMACCount = v
		}
		if v, err := strconv.Atoi(fields[3]); err == nil {
			entry.CurrentMACCount = v
		}
		if v, err := strconv.Atoi(fields[4]); err == nil {
			entry.ViolationCount = v
		}
		if len(fields) > 5 {
			entry.SecurityViolation = fields[5]
		}
		if len(fields) > 6 {
			entry.LastSource = fields[6]
		}
		entry.CollectionTS = collectionTime.Format(time.RFC3339)
		entries = append(entries, entry)
	}
	return entries
}

// ===========================
// Helpers
// ===========================

// getWildcardForOS10 returns the wildcard mask for an address
func getWildcardForOS10(addr string) string {
	if addr == "any" || addr == "host" {
		return ""
	}
	return "0.0.0.0"
}

// isPortQualifierOS10 checks for tcp/udp port qualifier keywords
func isPortQualifierOS10(s string) bool {
	lower := strings.ToLower(s)
	return lower == "eq" || lower == "neq" || lower == "gt" ||
		lower == "lt" || lower == "range"
}
