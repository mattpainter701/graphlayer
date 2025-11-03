package cisco

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |L|2| |P|a|r|s|e|r| |F|u|n|c|t|i|o|n|s|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The following methods parse Cisco IOS Layer-2 configuration data.
 * They extract switch segmentation, security posture, and evidence hashes.
 */

// buildCiscoL2Config walks the parsed IOS config and materialises Layer-2 objects.
// L2 requirement: captures switch segmentation, security posture, and evidence hashes per stanza.
func (p *CiscoIOSProcessor) buildCiscoL2Config() models.CiscoL2Config {
	stanzas := extractIOSStanzas(p.iosConfig.RawConfig)

	cfg := models.CiscoL2Config{
		Hostname:            p.iosConfig.Hostname,
		Version:             p.iosConfig.Version,
		RawConfig:           p.iosConfig.RawConfig,
		ConfigHash:          computeConfigHash(p.iosConfig.RawConfig),
		SourceSnippetHashes: make(map[string]string),
	}

	portChannelMembers := make(map[int]map[string]struct{})

	for _, iface := range p.iosConfig.Interfaces {
		header := "interface " + iface.Name
		block := stanzas[header]
		lowerName := strings.ToLower(iface.Name)

		if strings.HasPrefix(lowerName, "vlan") {
			svi := models.CiscoSVI{
				VLANID:        deriveSVIVLANID(iface),
				InterfaceName: iface.Name,
				IPAddress:     iface.IPAddress,
				Subnet:        iface.Subnet,
				Description:   iface.Description,
				SourceBlock:   header,
			}
			hashInput := block
			if len(hashInput) == 0 {
				hashInput = []string{header}
			}
			svi.SourceSnippetHash = computeSnippetHash(hashInput)
			cfg.SVIs = append(cfg.SVIs, svi)
			cfg.SourceSnippetHashes[header] = svi.SourceSnippetHash
			continue
		}

		if strings.HasPrefix(lowerName, "port-channel") {
			continue
		}

		l2Iface := models.CiscoL2Interface{
			Name:              iface.Name,
			AdminState:        deriveAdminState(iface.Shutdown),
			Mode:              deriveSwitchportMode(iface),
			Description:       iface.Description,
			SourceBlock:       header,
			TrunkAllowedVLANs: expandVLANListFromInts(iface.TrunkVLANs),
		}
		if iface.AccessVLAN != 0 {
			l2Iface.AccessVLAN = intPtr(iface.AccessVLAN)
		}

		parsed := parseInterfaceBlock(block)
		if parsed.Mode != "" {
			l2Iface.Mode = parsed.Mode
		}
		if parsed.AccessVLAN != nil {
			l2Iface.AccessVLAN = parsed.AccessVLAN
		}
		if len(parsed.TrunkAllowed) > 0 {
			l2Iface.TrunkAllowedVLANs = parsed.TrunkAllowed
		}
		if parsed.TrunkNative != nil {
			l2Iface.TrunkNativeVLAN = parsed.TrunkNative
		}
		if parsed.PortChannelID != nil {
			l2Iface.PortChannelID = parsed.PortChannelID
			addPortChannelMember(portChannelMembers, *parsed.PortChannelID, iface.Name)
		}
		l2Iface.DHCPSnoopingTrust = parsed.DHCPSnoopingTrust
		l2Iface.ARPInspectionTrust = parsed.ARPInspectionTrust
		l2Iface.IPSourceGuard = parsed.IPSourceGuard
		l2Iface.Dot1xRequired = parsed.Dot1xRequired
		l2Iface.MABEnabled = parsed.MABEnabled
		l2Iface.UnauthVLAN = parsed.UnauthVLAN
		l2Iface.PACLInName = parsed.PACLInName
		l2Iface.PACLOutName = parsed.PACLOutName
		l2Iface.PVLANRole = parsed.PVLANRole
		l2Iface.PVLANPrimaryVLAN = parsed.PVLANPrimaryVLAN
		l2Iface.PVLANSecondaryVLAN = parsed.PVLANSecondaryVLAN

		hashInput := block
		if len(hashInput) == 0 {
			hashInput = []string{header}
		}
		l2Iface.SourceSnippetHash = computeSnippetHash(hashInput)

		cfg.Interfaces = append(cfg.Interfaces, l2Iface)
		cfg.SourceSnippetHashes[header] = l2Iface.SourceSnippetHash
	}

	for _, vlan := range p.iosConfig.VLANs {
		header := "vlan " + strconv.Itoa(vlan.ID)
		block := stanzas[header]
		l2vlan := models.CiscoL2VLAN{
			ID:          vlan.ID,
			Name:        vlan.Name,
			State:       vlan.State,
			SourceBlock: header,
		}
		hashInput := block
		if len(hashInput) == 0 {
			hashInput = []string{header}
		}
		l2vlan.SourceSnippetHash = computeSnippetHash(hashInput)
		if vacl := extractVACLFromBlock(block); vacl != nil {
			l2vlan.VACLName = vacl
		}
		cfg.VLANs = append(cfg.VLANs, l2vlan)
		cfg.SourceSnippetHashes[header] = l2vlan.SourceSnippetHash
	}

	for header, block := range stanzas {
		lowerHeader := strings.ToLower(header)
		if !strings.HasPrefix(lowerHeader, "interface port-channel") {
			continue
		}
		name := strings.TrimPrefix(header, "interface ")
		idStr := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(name), "port-channel"))
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}

		parsed := parseInterfaceBlock(block)
		po := models.CiscoPortChannel{
			ID:                id,
			Name:              name,
			Mode:              parsed.Mode,
			AccessVLAN:        parsed.AccessVLAN,
			TrunkAllowedVLANs: parsed.TrunkAllowed,
			TrunkNativeVLAN:   parsed.TrunkNative,
			SourceBlock:       header,
		}
		if po.Mode == "" {
			switch {
			case len(po.TrunkAllowedVLANs) > 0:
				po.Mode = "trunk"
			case po.AccessVLAN != nil:
				po.Mode = "access"
			}
		}
		if members, ok := portChannelMembers[id]; ok {
			po.MemberInterfaces = setMemberSetToSlice(members)
		}
		hashInput := block
		if len(hashInput) == 0 {
			hashInput = []string{header}
		}
		po.SourceSnippetHash = computeSnippetHash(hashInput)
		cfg.PortChannels = append(cfg.PortChannels, po)
		cfg.SourceSnippetHashes[header] = po.SourceSnippetHash
	}

	cfg.GlobalL2Controls = parseGlobalL2Controls(stanzas)
	if cfg.GlobalL2Controls.SourceBlock != "" {
		cfg.SourceSnippetHashes[cfg.GlobalL2Controls.SourceBlock] = cfg.GlobalL2Controls.SourceSnippetHash
	}

	return cfg
}

func deriveAdminState(shutdown bool) string {
	if shutdown {
		return "down"
	}
	return "up"
}

func deriveSwitchportMode(iface models.CiscoIOSInterface) string {
	switch {
	case len(iface.TrunkVLANs) > 0:
		return "trunk"
	case iface.AccessVLAN != 0:
		return "access"
	default:
		return "routed"
	}
}

func deriveSVIVLANID(iface models.CiscoIOSInterface) int {
	if iface.VLAN != 0 {
		return iface.VLAN
	}
	if id, ok := parseTrailingInt(iface.Name); ok {
		return id
	}
	return 0
}

func parseTrailingInt(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	end := len(value) - 1
	start := end
	for start >= 0 && value[start] >= '0' && value[start] <= '9' {
		start--
	}
	if start == end {
		return 0, false
	}
	id, err := strconv.Atoi(value[start+1:])
	if err != nil {
		return 0, false
	}
	return id, true
}

func addPortChannelMember(members map[int]map[string]struct{}, id int, iface string) {
	if iface == "" {
		return
	}
	if _, ok := members[id]; !ok {
		members[id] = make(map[string]struct{})
	}
	members[id][iface] = struct{}{}
}

func setMemberSetToSlice(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	result := make([]string, 0, len(set))
	for name := range set {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func extractVACLFromBlock(block []string) *string {
	if len(block) == 0 {
		return nil
	}
	for _, raw := range block[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "ip access-group ") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				name := fields[2]
				return stringPtr(name)
			}
		}
	}
	return nil
}

type interfaceControlExtract struct {
	Mode               string
	AccessVLAN         *int
	TrunkAllowed       []int
	TrunkNative        *int
	PortChannelID      *int
	DHCPSnoopingTrust  bool
	ARPInspectionTrust bool
	IPSourceGuard      bool
	Dot1xRequired      bool
	MABEnabled         bool
	UnauthVLAN         *int
	PACLInName         *string
	PACLOutName        *string
	PVLANRole          *string
	PVLANPrimaryVLAN   *int
	PVLANSecondaryVLAN *int
}

func parseInterfaceBlock(block []string) interfaceControlExtract {
	data := interfaceControlExtract{}
	if len(block) == 0 {
		return data
	}

	for _, raw := range block[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}
		lower := strings.ToLower(line)

		switch {
		case strings.HasPrefix(lower, "switchport mode "):
			data.Mode = strings.TrimSpace(lower[len("switchport mode "):])
		case strings.HasPrefix(lower, "switchport access vlan "):
			if id, err := strconv.Atoi(strings.TrimSpace(line[len("switchport access vlan "):])); err == nil {
				data.AccessVLAN = intPtr(id)
			}
		case strings.HasPrefix(lower, "switchport trunk native vlan "):
			if id, err := strconv.Atoi(strings.TrimSpace(line[len("switchport trunk native vlan "):])); err == nil {
				data.TrunkNative = intPtr(id)
			}
		case strings.HasPrefix(lower, "switchport trunk allowed vlan"):
			remainder := strings.TrimSpace(line[len("switchport trunk allowed vlan"):])
			remainder = strings.TrimPrefix(remainder, "add ")
			allowed := expandVLANList(remainder)
			if len(allowed) > 0 {
				data.TrunkAllowed = mergeUniqueInts(data.TrunkAllowed, allowed)
			}
		case strings.HasPrefix(lower, "channel-group "):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if id, err := strconv.Atoi(fields[1]); err == nil {
					data.PortChannelID = intPtr(id)
				}
			}
		case strings.HasPrefix(lower, "ip dhcp snooping trust"):
			data.DHCPSnoopingTrust = true
		case strings.HasPrefix(lower, "ip arp inspection trust"):
			data.ARPInspectionTrust = true
		case strings.HasPrefix(lower, "ip verify source"):
			data.IPSourceGuard = true
		case strings.HasPrefix(lower, "dot1x port-control"):
			data.Dot1xRequired = true
		case strings.HasPrefix(lower, "authentication port-control auto"):
			data.Dot1xRequired = true
		case strings.HasPrefix(lower, "dot1x pae"):
			data.Dot1xRequired = true
		case strings.HasPrefix(lower, "authentication order "):
			if strings.Contains(lower, "dot1x") {
				data.Dot1xRequired = true
			}
			if strings.Contains(lower, "mab") {
				data.MABEnabled = true
			}
		case lower == "mab":
			data.MABEnabled = true
		}

		if strings.Contains(lower, "authorize vlan") {
			fields := strings.Fields(line)
			for i := 0; i < len(fields)-1; i++ {
				if strings.EqualFold(fields[i], "vlan") {
					if id, err := strconv.Atoi(stripTrailingComma(fields[i+1])); err == nil {
						data.UnauthVLAN = intPtr(id)
					}
				}
			}
		}

		if strings.HasPrefix(lower, "ip access-group ") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				name := fields[2]
				dir := strings.ToLower(fields[3])
				if dir == "in" {
					data.PACLInName = stringPtr(name)
				} else if dir == "out" {
					data.PACLOutName = stringPtr(name)
				}
			}
		}

		if strings.HasPrefix(lower, "switchport mode private-vlan ") {
			role := strings.TrimSpace(line[len("switchport mode private-vlan "):])
			if role != "" {
				data.PVLANRole = stringPtr(strings.ToLower(role))
			}
		}

		if strings.HasPrefix(lower, "switchport private-vlan host-association") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				if primary, err := strconv.Atoi(fields[len(fields)-2]); err == nil {
					data.PVLANPrimaryVLAN = intPtr(primary)
				}
				if secondary, err := strconv.Atoi(fields[len(fields)-1]); err == nil {
					data.PVLANSecondaryVLAN = intPtr(secondary)
				}
			}
		}
	}

	return data
}

func stripTrailingComma(value string) string {
	return strings.TrimRight(value, ",;")
}

func intPtr(v int) *int { return &v }

func stringPtr(v string) *string { return &v }

func expandVLANListFromInts(vlans []int) []int {
	if len(vlans) == 0 {
		return nil
	}
	tmp := make([]int, len(vlans))
	copy(tmp, vlans)
	sort.Ints(tmp)
	dedup := tmp[:0]
	for _, v := range tmp {
		if len(dedup) == 0 || dedup[len(dedup)-1] != v {
			dedup = append(dedup, v)
		}
	}
	return dedup
}

func expandVLANList(raw string) []int {
	if raw == "" {
		return nil
	}
	tokens := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
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

func computeSnippetHash(lines []string) string {
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

func appendUniqueLines(lines []string, seen map[string]struct{}, block []string) []string {
	if len(block) == 0 {
		return lines
	}
	for _, raw := range block {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || trimmed == "!" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		lines = append(lines, trimmed)
		seen[trimmed] = struct{}{}
	}
	return lines
}

func extractIOSStanzas(raw string) map[string][]string {
	stanzas := make(map[string][]string)
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	var currentHeader string
	var current []string

	flush := func() {
		if currentHeader != "" {
			stanzas[currentHeader] = append([]string{}, current...)
		}
	}

	for _, rawLine := range lines {
		clean := strings.TrimRight(rawLine, "\r")
		trimmed := strings.TrimSpace(clean)

		if trimmed == "!" {
			flush()
			currentHeader = ""
			current = nil
			continue
		}

		if trimmed == "" {
			if currentHeader != "" {
				current = append(current, "")
			}
			continue
		}

		if strings.HasPrefix(clean, " ") || strings.HasPrefix(clean, "\t") {
			if currentHeader != "" {
				current = append(current, trimmed)
			}
			continue
		}

		flush()
		currentHeader = trimmed
		current = []string{trimmed}
	}

	flush()
	return stanzas
}

func computeConfigHash(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "!") {
			continue
		}
		cleaned = append(cleaned, strings.ToLower(trimmed))
	}
	h := sha256.Sum256([]byte(strings.Join(cleaned, "\n")))
	return "sha256:" + hex.EncodeToString(h[:])
}

func parseGlobalL2Controls(stanzas map[string][]string) models.CiscoGlobalL2Controls {
	controls := models.CiscoGlobalL2Controls{
		DAIACLByVLAN: make(map[int]string),
	}
	now := time.Now().Format(time.RFC3339)
	var blockLines []string
	lineSet := make(map[string]struct{})

	for header, lines := range stanzas {
		lower := strings.ToLower(header)

		switch {
		case strings.HasPrefix(lower, "ip dhcp snooping vlan"):
			vlans := expandVLANList(strings.TrimSpace(header[len("ip dhcp snooping vlan"):]))
			controls.SnoopingEnabled = true
			controls.SnoopingVLANs = mergeUniqueInts(controls.SnoopingVLANs, vlans)
			blockLines = appendUniqueLines(blockLines, lineSet, lines)
		case strings.HasPrefix(lower, "ip dhcp snooping"):
			controls.SnoopingEnabled = true
			blockLines = appendUniqueLines(blockLines, lineSet, lines)
		case strings.HasPrefix(lower, "ip arp inspection vlan"):
			vlans := expandVLANList(strings.TrimSpace(header[len("ip arp inspection vlan"):]))
			controls.DAIEnabledVLANs = mergeUniqueInts(controls.DAIEnabledVLANs, vlans)
			blockLines = appendUniqueLines(blockLines, lineSet, lines)
		case strings.HasPrefix(lower, "ip arp inspection filter"):
			fields := strings.Fields(header)
			var acl string
			var vlanTokens []string
			for i := 0; i < len(fields); i++ {
				switch strings.ToLower(fields[i]) {
				case "filter":
					if i+1 < len(fields) {
						acl = fields[i+1]
						i++
					}
				case "vlan":
					if i+1 < len(fields) {
						vlanTokens = fields[i+1:]
					}
				}
			}
			if acl != "" && len(vlanTokens) > 0 {
				vlans := expandVLANList(strings.Join(vlanTokens, " "))
				for _, vlan := range vlans {
					controls.DAIACLByVLAN[vlan] = acl
				}
			}
			blockLines = appendUniqueLines(blockLines, lineSet, lines)
		case strings.HasPrefix(lower, "ip arp inspection"):
			blockLines = appendUniqueLines(blockLines, lineSet, lines)
		case strings.HasPrefix(lower, "ip device tracking"):
			controls.IPSGGlobal = true
			blockLines = appendUniqueLines(blockLines, lineSet, lines)
		case strings.HasPrefix(lower, "dot1x system-auth-control"):
			controls.Dot1xSystemAuthControl = true
			blockLines = appendUniqueLines(blockLines, lineSet, lines)
		case strings.HasPrefix(lower, "authentication mac-move"):
			controls.MABGlobal = true
			blockLines = appendUniqueLines(blockLines, lineSet, lines)
		}
	}

	if len(blockLines) > 0 {
		controls.SourceBlock = strings.Join(blockLines, "\n")
		controls.SourceSnippetHash = computeSnippetHash(blockLines)
	}

	controls.LastUpdated = now
	if len(controls.DAIACLByVLAN) == 0 {
		controls.DAIACLByVLAN = nil
	}
	return controls
}

