package eos

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |C|o|n|f|i|g| |P|a|r|s|e|r|s|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The following functions parse Arista EOS configuration data.
 * EOS configuration is ==-based (each sub-command is indented
 * under its parent). They extract Layer-2 and Layer-3 objects
 * for topology model consumption.
 */

// ===========================
// Interface Config Parsing
// ===========================

// interfaceBlock captures an EOS interface stanza's parsed attributes
type interfaceBlock struct {
	Mode     string // access / trunk / routed / dot1q-tunnel
	Access   int
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

// parseEOSInterfaceBlock walks an interface stanza and extracts attributes
func parseEOSInterfaceBlock(lines []string) interfaceBlock {
	data := interfaceBlock{}
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
			allowed := expandEOSVLANList(remainder)
			if len(allowed) > 0 {
				data.TrunkAllowed = mergeUniqueInts(data.TrunkAllowed, allowed)
			}

		case strings.HasPrefix(lower, "channel-group "):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.Atoi(fields[1]); err == nil {
					data.ChannelGroup = v
				}
			}

		case strings.HasPrefix(lower, "lacp port-priority ") ||
			strings.HasPrefix(lower, "lacp rate "):
			if strings.Contains(lower, "fast") {
				data.LACPTimeout = "fast"
			} else if strings.Contains(lower, "normal") {
				data.LACPTimeout = "normal"
			}

		case lower == "lacp mode active" || lower == "port-channel lacp mode active" ||
			strings.Contains(lower, "lacp mode active"):
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
				data.IPAddress = fields[1]
			}

		case strings.HasPrefix(lower, "lldp transmit"):
			data.LLDPTransmit = true

		case strings.HasPrefix(lower, "lldp receive"):
			data.LLDPReceive = true

		case strings.HasPrefix(lower, "storm-control "):
			data.StormControl = strings.TrimSpace(line[len("storm-control "):])

		case strings.HasPrefix(lower, "dot1q tunnel") ||
			strings.HasPrefix(lower, "switchport dot1q tunnel"):
			data.Mode = "dot1q-tunnel"
		}
	}
	return data
}

// expandEOSVLANList parses EOS VLAN list notation (1,10-12,100-110)
func expandEOSVLANList(raw string) []int {
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
	sort.Ints(result)
	dedup := result[:0]
	for _, v := range result {
		if len(dedup) == 0 || dedup[len(dedup)-1] != v {
			dedup = append(dedup, v)
		}
	}
	return dedup
}

// ===========================
// VLAN Config Parsing
// ===========================

// parseEOSVLANBlock extracts VLAN info from a "vlan X" stanza
func parseEOSVLANBlock(lines []string, vlanID int) models.EOSVLAN {
	vlan := models.EOSVLAN{
		ID:    vlanID,
		State: "active",
	}
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "name ") {
			vlan.Name = strings.TrimSpace(line[len("name "):])
		} else if strings.HasPrefix(lower, "state ") {
			vlan.State = strings.TrimSpace(lower[len("state "):])
		} else if strings.HasPrefix(lower, "trunk group ") {
			vlan.TrunkGroups = append(vlan.TrunkGroups, strings.TrimSpace(line[len("trunk group "):]))
		}
	}
	return vlan
}

// ===========================
// MLAG Config Parsing
// ===========================

// parseEOSMLAGConfig extracts MLAG configuration from parsed stanzas
func parseEOSMLAGConfig(configLines []string) *models.EOSMLAGConfig {
	mlag := &models.EOSMLAGConfig{
		VLANs:   []int{},
		MLAGIDs: make(map[int]int),
	}
	inMLAG := false
	for _, raw := range configLines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}
		lower := strings.ToLower(line)

		if strings.HasPrefix(lower, "mlag configuration") {
			inMLAG = true
			continue
		}
		if !inMLAG {
			continue
		}

		switch {
		case strings.HasPrefix(lower, "domain-id "):
			mlag.DomainID = strings.TrimSpace(line[len("domain-id "):])

		case strings.HasPrefix(lower, "local-interface "):
			mlag.LocalInterface = strings.TrimSpace(line[len("local-interface "):])

		case strings.HasPrefix(lower, "peer-address "):
			mlag.PeerAddress = strings.TrimSpace(line[len("peer-address "):])

		case strings.HasPrefix(lower, "peer-link "):
			mlag.PeerLink = strings.TrimSpace(line[len("peer-link "):])

		case strings.HasPrefix(lower, "l3-peer-link "):
			if strings.Contains(lower, "l3-peer-link l3") || strings.Contains(lower, "no l3-peer-link") {
				// Parse l3-peer-link status
				mlag.L3PeerLink = strings.Contains(lower, "no ") == false
			}

		case strings.HasPrefix(lower, "heartbeat-interval "):
			if v, err := strconv.Atoi(strings.Fields(line)[1]); err == nil {
				mlag.HeartbeatTimeout = v
			}

		case strings.HasPrefix(lower, "reload-delay "):
			if v, err := strconv.Atoi(strings.Fields(line)[1]); err == nil {
				mlag.ReloadDelay = v
			}

		case lower == "no shutdown":
			mlag.Enabled = true

		case lower == "shutdown":
			mlag.Enabled = false

		case strings.Contains(lower, "mlag ") && strings.Contains(lower, "vlan "):
			// "mlag vlan 10,20" or "mlag vlan 10-12"
			if idx := strings.Index(lower, "vlan "); idx >= 0 {
				vlanStr := strings.TrimSpace(lower[idx+5:])
				mlag.VLANs = expandEOSVLANList(vlanStr)
			}
		}
	}

	if mlag.DomainID == "" && !inMLAG {
		return nil
	}
	return mlag
}

// ===========================
// VXLAN Config Parsing
// ===========================

// parseEOSVXLANConfig extracts VXLAN configuration from "interface Vxlan X" stanzas
func parseEOSVXLANConfig(lines []string) *models.EOSVXLANConfig {
	vxlan := &models.EOSVXLANConfig{
		VNItoVLAN: make(map[int]int),
		VLANtoVNI: make(map[int]int),
		VNItoGroup: make(map[int]string),
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}
		lower := strings.ToLower(line)

		switch {
		case strings.HasPrefix(lower, "vxlan source-interface "):
			vxlan.SourceInterface = strings.TrimSpace(line[len("vxlan source-interface "):])

		case strings.HasPrefix(lower, "vxlan udp-port "):
			// VXLAN UDP port, just note it's enabled
			vxlan.Enabled = true

		case strings.HasPrefix(lower, "vxlan vlan "):
			// "vxlan vlan 10 vni 10010"
			fields := strings.Fields(line)
			for i := 0; i < len(fields)-3; i++ {
				if fields[i] == "vlan" {
					vlanID, err1 := strconv.Atoi(fields[i+1])
					if err1 != nil {
						continue
					}
					for j := i + 2; j < len(fields)-1; j++ {
						if fields[j] == "vni" {
							vni, err2 := strconv.Atoi(fields[j+1])
							if err2 != nil {
								continue
							}
							vxlan.VNItoVLAN[vni] = vlanID
							vxlan.VLANtoVNI[vlanID] = vni
							vxlan.Enabled = true
						}
					}
				}
			}

		case strings.HasPrefix(lower, "vxlan flood vtep "):
			// "vxlan flood vtep 10.0.0.1 10.0.0.2"
			fields := strings.Fields(line)
			for i := 0; i < len(fields); i++ {
				if net.ParseIP(fields[i]) != nil {
					vxlan.FloodList = append(vxlan.FloodList, fields[i])
				}
			}

		case strings.HasPrefix(lower, "vxlan multicast group "):
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				vni, err1 := strconv.Atoi(fields[len(fields)-2])
				group := fields[len(fields)-1]
				if err1 == nil && net.ParseIP(group) != nil {
					vxlan.VNItoGroup[vni] = group
				}
			}

		case strings.HasPrefix(lower, "vxlan learn "):
			// "vxlan learn from vtep" or "vxlan learn from arp"
			vxlan.LearningMode = strings.TrimSpace(lower[len("vxlan learn"):])
		}
	}

	if !vxlan.Enabled && len(vxlan.VNItoVLAN) == 0 && len(vxlan.FloodList) == 0 {
		return nil
	}
	return vxlan
}

// ===========================
// Static Route Parsing
// ===========================

// parseEOSStaticRoute parses a single "ip route" command
func parseEOSStaticRoute(line string) models.EOSStaticRoute {
	route := models.EOSStaticRoute{Raw: line}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return route
	}

	// ip route prefix mask [next-hop] [distance] [tag N] [name NAME] [vrf VRF]
	idx := 1 // after "ip route"

	if idx < len(fields) {
		route.Prefix = fields[idx]
		idx++
	}
	if idx < len(fields) {
		mask := fields[idx]
		// Could be a netmask like "255.255.255.0" or prefix length
		if strings.Contains(mask, ".") {
			// Convert netmask to prefix length
			route.PrefixLen = netmaskToPrefixLen(mask)
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
			// Try as distance (integer)
			if v, err := strconv.Atoi(f); err == nil && route.Distance == 0 {
				route.Distance = v
			}
			idx++
		}
	}
	return route
}

// netmaskToPrefixLen converts a dotted netmask to CIDR prefix length
func netmaskToPrefixLen(mask string) int {
	ip := net.ParseIP(mask)
	if ip == nil {
		return 0
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return 0
	}
	ones, _ := net.IPv4Mask(ipv4[0], ipv4[1], ipv4[2], ipv4[3]).Size()
	return ones
}

// ===========================
// ACL Parsing
// ===========================

// parseEOSExtendedACL parses an "ip access-list" block
func parseEOSExtendedACL(lines []string) models.EOSExtendedACL {
	acl := models.EOSExtendedACL{}
	if len(lines) == 0 {
		return acl
	}

	header := strings.TrimSpace(lines[0])
	fields := strings.Fields(header)
	if len(fields) >= 3 {
		// ip access-list <name>
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

		rule := models.EOSExtendedACLRule{
			Order: order,
			Raw:   line,
		}
		order += 10

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		idx := 0
		// Optional sequence number
		if v, err := strconv.Atoi(fields[idx]); err == nil && v < 1000 {
			rule.Order = v
			idx++
		}

		if idx < len(fields) {
			rule.Action = fields[idx]
			idx++
		}
		if idx < len(fields) {
			rule.Protocol = fields[idx]
			idx++
		}

		// Parse source and dest
		if idx+1 < len(fields) {
			source := fields[idx]
			if source == "host" {
				idx++
				source = fields[idx]
				rule.Source = source + "/32"
			} else {
				rule.Source = source
				if idx+1 < len(fields) && !strings.Contains(fields[idx+1], ".") {
					// Could be port
					rule.SourceWildcard = getWildcardFor(rule.Source)
				} else if idx+1 < len(fields) {
					rule.SourceWildcard = fields[idx+1]
					idx++
				}
			}
			idx++
		}

		// Check for source port qualifiers
		for idx < len(fields) && isPortQualifier(fields[idx]) {
			q := fields[idx]
			idx++
			if idx < len(fields) {
				rule.SourcePorts = append(rule.SourcePorts, q+" "+fields[idx])
				idx++
			}
		}

		// Destination
		if idx < len(fields) {
			dest := fields[idx]
			if dest == "host" {
				idx++
				dest = fields[idx]
				rule.Dest = dest + "/32"
			} else {
				rule.Dest = dest
				if idx+1 < len(fields) && !strings.Contains(fields[idx+1], ".") {
					rule.DestWildcard = getWildcardFor(rule.Dest)
				} else if idx+1 < len(fields) {
					rule.DestWildcard = fields[idx+1]
					idx++
				}
			}
			idx++
		}

		// Destination port qualifiers
		for idx < len(fields) && isPortQualifier(fields[idx]) {
			q := fields[idx]
			idx++
			if idx < len(fields) {
				rule.DestPorts = append(rule.DestPorts, q+" "+fields[idx])
				idx++
			}
		}

		// Log
		for idx < len(fields) {
			if fields[idx] == "log" {
				rule.Log = true
			}
			idx++
		}

		acl.Rules = append(acl.Rules, rule)
	}
	return acl
}

// ===========================
// Helpers
// ===========================

func getWildcardFor(addr string) string {
	if addr == "any" || addr == "host" {
		return ""
	}
	// Default: host wildcard
	return "0.0.0.0"
}

func isPortQualifier(s string) bool {
	lower := strings.ToLower(s)
	return lower == "eq" || lower == "neq" || lower == "gt" ||
		lower == "lt" || lower == "range"
}

func mergeUniqueInts(left, right []int) []int {
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
	sort.Ints(merged)
	return merged
}

func computeEOSSnippetHash(lines []string) string {
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "!") {
			continue
		}
		normalized = append(normalized, strings.ToLower(trimmed))
	}
	sort.Strings(normalized)
	h := sha256.Sum256([]byte(strings.Join(normalized, "\n")))
	return "sha256:" + hex.EncodeToString(h[:])
}

func intPtr(v int) *int { return &v }

func stringPtr(v string) *string { return &v }

// ===========================
// EOS L2 Security Parsers
// ===========================

// parseEOSPortSecurity parses "show port-security" output for EOS
type EOSPortSecurityEntry struct {
	Interface         string
	MaxMACCount       int
	CurrentMACCount   int
	SecurityViolation string
	Status            string
	ViolationCount    int
	LastSource        string
	CollectionTS      string
}

func ParseEOSPortSecurity(output string, collectionTime time.Time) []EOSPortSecurityEntry {
	var entries []EOSPortSecurityEntry
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Port") ||
			strings.HasPrefix(line, "---") || strings.HasPrefix(line, "Maximum") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		entry := EOSPortSecurityEntry{
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
