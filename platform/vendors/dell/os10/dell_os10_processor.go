package os10

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |O|S|1|0| |P|r|o|c|e|s|s|o|r|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The DellOS10Processor handles parsing and processing of Dell OS10
 * configuration files. It follows the same pattern as AristaEOSProcessor
 * and CiscoIOSProcessor but is adapted for OS10-specific syntax
 * (VLT, OS10 interface naming, FTOS backward compatibility).
 *
 * OS10 uses an indented block structure similar to Cisco IOS.
 * Comments use "!" prefix.
 */

// DellOS10Processor handles OS10 config parsing and topology model building
type DellOS10Processor struct {
	filePath   string
	os10Config *models.OS10Config
	// Topology model maps (populated by BuildNetworkTopologyModels)
	TopologyDevice         *models.Device
	TopologyInterfaces     map[string]models.Interface
	TopologyAddresses      map[string]models.Address
	TopologyAddressGroups  map[string]models.AddressGroup
	TopologyServices       map[string]models.Service
	TopologyServiceGroups  map[string]models.ServiceGroup
	TopologyRoutingEntries map[string]models.RoutingEntry
	TopologyZones          map[string]models.Zone
	FileHash               string
}

// NewDellOS10Processor creates a new processor instance
func NewDellOS10Processor(filePath string) *DellOS10Processor {
	return &DellOS10Processor{
		filePath: filePath,
	}
}

// ProcessOS10File processes an OS10 configuration file
func (p *DellOS10Processor) ProcessOS10File() error {
	os10Config, err := p.ParseOS10Config()
	if err != nil {
		return fmt.Errorf("parse OS10 config: %w", err)
	}
	p.os10Config = os10Config
	p.FileHash = computeOS10SnippetHash(strings.Split(os10Config.RawConfig, "\n"))

	if err := p.BuildNetworkTopologyModels(); err != nil {
		return fmt.Errorf("build topology models: %w", err)
	}

	return nil
}

// ParseOS10Config parses an OS10 configuration file from disk
func (p *DellOS10Processor) ParseOS10Config() (*models.OS10Config, error) {
	content, err := os.ReadFile(p.filePath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	configText := string(content)
	scanner := bufio.NewScanner(strings.NewReader(configText))

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan config file: %w", err)
	}

	cfg := &models.OS10Config{
		RawConfig: configText,
	}

	// Extract stanzas (OS10 uses indented blocks like Cisco/Arista)
	stanzas := extractOS10Stanzas(lines)

	// Process single-line config items
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "!") {
			continue
		}
		lower := strings.ToLower(trimmed)

		switch {
		case strings.HasPrefix(lower, "hostname "):
			cfg.Hostname = strings.TrimSpace(trimmed[len("hostname "):])

		case strings.HasPrefix(lower, "version "):
			cfg.Version = strings.TrimSpace(trimmed[len("version "):])

		case strings.HasPrefix(lower, "ip route "):
			route := parseOS10StaticRoute(trimmed)
			cfg.StaticRoutes = append(cfg.StaticRoutes, route)
		}
	}

	// Process block stanzas
	for header, block := range stanzas {
		lower := strings.ToLower(header)

		switch {
		case strings.HasPrefix(lower, "interface "):
			iface := p.parseOS10Interface(header, block)
			if iface.Name != "" {
				cfg.Interfaces = append(cfg.Interfaces, iface)
			}

		case strings.HasPrefix(lower, "interface vlan"):
			vlanIDStr := strings.TrimSpace(header[len("interface vlan"):])
			if vlanID, err := strconv.Atoi(vlanIDStr); err == nil {
				vlan := parseOS10VLANBlock(block, vlanID)
				cfg.VLANs = append(cfg.VLANs, vlan)
			}

		case strings.HasPrefix(lower, "vlt-domain "):
			vlt := parseOS10VLTConfig(block)
			if vlt != nil {
				cfg.VLTConfig = vlt
			}

		case strings.HasPrefix(lower, "interface port-channel"):
			po := p.parseOS10PortChannel(header, block)
			if po.ID != 0 {
				cfg.PortChannels = append(cfg.PortChannels, po)
			}

		case strings.HasPrefix(lower, "ip access-list "):
			acl := parseOS10ExtendedACL(block)
			if acl.Name != "" {
				cfg.ExtendedACLs = append(cfg.ExtendedACLs, acl)
			}

		case strings.HasPrefix(lower, "router ospf "):
			cfg.OSPFRouter = parseOS10OSPFRouter(header, block)

		case strings.HasPrefix(lower, "router bgp "):
			cfg.BGPConfig = parseOS10BGPConfig(header, block)

		case strings.HasPrefix(lower, "ip vrf "):
			vrf := parseOS10VRF(header, block)
			if vrf.Name != "" {
				cfg.VRFConfigs = append(cfg.VRFConfigs, vrf)
			}
		}
	}

	return cfg, nil
}

// extractOS10Stanzas splits OS10 config into named blocks by top-level commands
func extractOS10Stanzas(lines []string) map[string][]string {
	stanzas := make(map[string][]string)
	var currentHeader string
	var currentBlock []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "!") {
			if currentHeader != "" && len(currentBlock) > 0 {
				stanzas[currentHeader] = currentBlock
			}
			currentHeader = ""
			currentBlock = nil
			continue
		}

		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			if currentHeader != "" && len(currentBlock) > 0 {
				stanzas[currentHeader] = currentBlock
			}
			currentHeader = strings.TrimSpace(line)
			currentBlock = []string{line}
		} else if currentHeader != "" {
			currentBlock = append(currentBlock, line)
		}
	}

	if currentHeader != "" && len(currentBlock) > 0 {
		stanzas[currentHeader] = currentBlock
	}

	return stanzas
}

// parseOS10Interface parses an OS10 interface stanza
func (p *DellOS10Processor) parseOS10Interface(header string, block []string) models.OS10Interface {
	name := strings.TrimSpace(header[len("interface "):])
	iface := models.OS10Interface{
		Name: name,
	}

	parsed := parseOS10InterfaceBlock(block)
	iface.Description = parsed.Description
	iface.Shutdown = parsed.Shutdown
	iface.IPAddress = parsed.IPAddress
	iface.Netmask = parsed.Netmask
	iface.MTU = parsed.MTU
	iface.LACPTimeout = parsed.LACPTimeout
	iface.Flowcontrol = parsed.Flowcontrol
	iface.LLDPTransmit = parsed.LLDPTransmit
	iface.LLDPReceive = parsed.LLDPReceive
	iface.StormControl = parsed.StormControl
	iface.LAGGroup = parsed.ChannelGroup
	iface.LAGLACP = parsed.LACPActive

	// Switchport / L2 detection
	lowerName := strings.ToLower(iface.Name)
	isSVI := strings.HasPrefix(lowerName, "vlan")
	isLoopback := strings.HasPrefix(lowerName, "loopback")
	isManagement := strings.HasPrefix(lowerName, "mgmt") || strings.HasPrefix(lowerName, "management")
	isPortChannel := strings.HasPrefix(lowerName, "port-channel")

	if isSVI || isLoopback || isManagement {
		iface.Switchport = false
		iface.SwitchportMode = "routed"
		if isSVI {
			iface.VLAN = deriveOS10SVIID(iface.Name)
		}
	} else if isPortChannel {
		iface.Switchport = false
		if parsed.Mode != "" && parsed.Mode != "routed" {
			iface.Switchport = true
			iface.SwitchportMode = parsed.Mode
		} else if parsed.IPAddress != "" {
			iface.SwitchportMode = "routed"
		} else {
			iface.Switchport = true
			iface.SwitchportMode = "trunk"
		}
	} else if parsed.Mode != "" {
		iface.Switchport = true
		iface.SwitchportMode = parsed.Mode
		iface.AccessVLAN = parsed.Access
		iface.TrunkVLANs = parsed.TrunkAllowed
		iface.TrunkNative = parsed.TrunkNative
	} else if parsed.IPAddress != "" {
		iface.Switchport = false
		iface.SwitchportMode = "routed"
	} else {
		// Default: physical switchport
		iface.Switchport = true
		iface.SwitchportMode = "access"
	}

	return iface
}

// parseOS10PortChannel parses a port-channel interface
func (p *DellOS10Processor) parseOS10PortChannel(header string, block []string) models.OS10PortChannel {
	po := models.OS10PortChannel{
		Name: strings.TrimSpace(header[len("interface "):]),
	}

	// Extract port-channel number from name
	lowerName := strings.ToLower(po.Name)
	idStr := strings.TrimPrefix(lowerName, "port-channel")
	idStr = strings.TrimSpace(idStr)
	if id, err := strconv.Atoi(idStr); err == nil {
		po.ID = id
	}

	parsed := parseOS10InterfaceBlock(block)
	po.Mode = parsed.Mode
	po.Description = parsed.Description
	if parsed.ChannelGroup != 0 {
		po.ID = parsed.ChannelGroup
	}
	if parsed.LACPTimeout != "" {
		po.LACPTimeout = parsed.LACPTimeout
	}
	return po
}

// deriveOS10SVIID extracts VLAN ID from SVI interface name
func deriveOS10SVIID(name string) int {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "vlan") {
		idStr := strings.TrimSpace(name[4:])
		if id, err := strconv.Atoi(idStr); err == nil {
			return id
		}
	}
	return 0
}

// ===========================
// Route Config Parsers
// ===========================

// parseOS10OSPFRouter parses "router ospf <id>" block
func parseOS10OSPFRouter(header string, lines []string) *models.OS10OSPFRouter {
	fields := strings.Fields(header)
	ospf := &models.OS10OSPFRouter{}
	if len(fields) >= 3 {
		if v, err := strconv.Atoi(fields[2]); err == nil {
			ospf.ProcessID = v
		}
	}
	ospf.Areas = make(map[string]models.OS10OSPFArea)

	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}
		lower := strings.ToLower(line)

		switch {
		case strings.HasPrefix(lower, "router-id "):
			ospf.RouterID = strings.TrimSpace(line[len("router-id "):])

		case strings.HasPrefix(lower, "network "):
			parts := strings.Fields(line)
			if len(parts) >= 5 && parts[len(parts)-2] == "area" {
				network := models.OS10OSPFNetwork{
					Network:  parts[1],
					Wildcard: parts[2],
					Area:     parts[len(parts)-1],
				}
				ospf.Networks = append(ospf.Networks, network)

				area := ospf.Areas[network.Area]
				area.ID = network.Area
				area.Networks = append(area.Networks, network.Network)
				ospf.Areas[network.Area] = area
			}
		}
	}

	if ospf.ProcessID == 0 && ospf.RouterID == "" {
		return nil
	}
	return ospf
}

// parseOS10BGPConfig parses "router bgp <asn>" block
func parseOS10BGPConfig(header string, lines []string) *models.OS10BGPConfig {
	fields := strings.Fields(header)
	bgp := &models.OS10BGPConfig{}
	if len(fields) >= 3 {
		if v, err := strconv.Atoi(fields[2]); err == nil {
			bgp.ASN = v
		}
	}

	var currentVRF *models.OS10BGPVRF

	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") {
			if currentVRF != nil {
				bgp.VRFs = append(bgp.VRFs, *currentVRF)
				currentVRF = nil
			}
			continue
		}
		lower := strings.ToLower(line)

		switch {
		case strings.HasPrefix(lower, "router-id "):
			bgp.RouterID = strings.TrimSpace(line[len("router-id "):])

		case strings.HasPrefix(lower, "network "):
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				bgp.Networks = append(bgp.Networks, parts[1])
			}

		case strings.HasPrefix(lower, "maximum-paths "):
			if v, err := strconv.Atoi(strings.Fields(line)[1]); err == nil {
				bgp.MaximumPaths = v
			}

		case strings.HasPrefix(lower, "neighbor "):
			peer := parseOS10BGPPeerLine(line)
			if currentVRF != nil {
				currentVRF.Peers = append(currentVRF.Peers, peer)
			} else {
				bgp.Peers = append(bgp.Peers, peer)
			}

		case strings.HasPrefix(lower, "vrf "):
			if currentVRF != nil {
				bgp.VRFs = append(bgp.VRFs, *currentVRF)
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentVRF = &models.OS10BGPVRF{Name: parts[1]}
			}

		case strings.HasPrefix(lower, "rd "):
			if currentVRF != nil {
				currentVRF.RD = strings.TrimSpace(line[len("rd "):])
			}
		}
	}

	if currentVRF != nil {
		bgp.VRFs = append(bgp.VRFs, *currentVRF)
	}

	if bgp.ASN == 0 {
		return nil
	}
	return bgp
}

// parseOS10BGPPeerLine parses a BGP neighbor line
func parseOS10BGPPeerLine(line string) models.OS10BGPPeer {
	peer := models.OS10BGPPeer{}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return peer
	}

	peer.IP = fields[1]
	for i := 2; i < len(fields); i++ {
		switch {
		case fields[i] == "remote-as" && i+1 < len(fields):
			if v, err := strconv.Atoi(fields[i+1]); err == nil {
				peer.ASN = v
			}
			i++
		case fields[i] == "description" && i+1 < len(fields):
			peer.Description = strings.Join(fields[i+1:], " ")
			return peer
		case fields[i] == "update-source" && i+1 < len(fields):
			peer.UpdateSource = fields[i+1]
			i++
		case fields[i] == "peer-group" && i+1 < len(fields):
			peer.PeerGroup = fields[i+1]
			i++
		case fields[i] == "ebgp-multihop" && i+1 < len(fields):
			if v, err := strconv.Atoi(fields[i+1]); err == nil {
				peer.Multihop = v
			}
			i++
		}
	}
	return peer
}

// parseOS10VRF parses "ip vrf <name>" block
func parseOS10VRF(header string, lines []string) models.OS10VRF {
	vrf := models.OS10VRF{}
	fields := strings.Fields(header)
	if len(fields) >= 3 {
		vrf.Name = fields[2]
	} else if len(fields) >= 2 {
		vrf.Name = fields[1]
	}

	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}
		lower := strings.ToLower(line)

		switch {
		case strings.HasPrefix(lower, "rd "):
			vrf.RD = strings.TrimSpace(line[len("rd "):])

		case strings.HasPrefix(lower, "route-target import "):
			vrf.RouteTargetImport = append(vrf.RouteTargetImport,
				strings.TrimSpace(line[len("route-target import "):]))

		case strings.HasPrefix(lower, "route-target export "):
			vrf.RouteTargetExport = append(vrf.RouteTargetExport,
				strings.TrimSpace(line[len("route-target export "):]))
		}
	}
	return vrf
}

// ===========================
// OS10 L2 Config Builder
// ===========================

// BuildOS10L2Config builds an OS10 L2 config object from parsed data
func (p *DellOS10Processor) BuildOS10L2Config() models.OS10L2Config {
	cfg := models.OS10L2Config{
		Hostname:            p.os10Config.Hostname,
		Version:             p.os10Config.Version,
		RawConfig:           p.os10Config.RawConfig,
		ConfigHash:          p.FileHash,
		SourceSnippetHashes: make(map[string]string),
	}

	for _, iface := range p.os10Config.Interfaces {
		lowerName := strings.ToLower(iface.Name)
		if strings.HasPrefix(lowerName, "vlan") {
			svi := models.OS10SVI{
				VLANID:        iface.VLAN,
				InterfaceName: iface.Name,
				IPAddress:     iface.IPAddress,
				Description:   iface.Description,
			}
			cfg.SVIs = append(cfg.SVIs, svi)
			continue
		}
		if strings.HasPrefix(lowerName, "port-channel") {
			continue
		}

		l2Iface := models.OS10L2Interface{
			Name:              iface.Name,
			AdminState:        map[bool]string{true: "up", false: "down"}[!iface.Shutdown],
			Mode:              iface.SwitchportMode,
			AccessVLAN:        intPtr(iface.AccessVLAN),
			TrunkAllowedVLANs: iface.TrunkVLANs,
			TrunkNativeVLAN:   intPtr(iface.TrunkNative),
			PortChannelID:     intPtr(iface.LAGGroup),
			Description:       iface.Description,
		}
		if iface.AccessVLAN == 0 {
			l2Iface.AccessVLAN = nil
		}
		if iface.TrunkNative == 0 {
			l2Iface.TrunkNativeVLAN = nil
		}
		if iface.LAGGroup == 0 {
			l2Iface.PortChannelID = nil
		}
		cfg.Interfaces = append(cfg.Interfaces, l2Iface)
	}

	for _, vlan := range p.os10Config.VLANs {
		l2vlan := models.OS10L2VLAN{
			ID:    vlan.ID,
			Name:  vlan.Name,
			State: vlan.State,
		}
		cfg.VLANs = append(cfg.VLANs, l2vlan)
	}

	for _, po := range p.os10Config.PortChannels {
		cfg.PortChannels = append(cfg.PortChannels, models.OS10PortChannelSummary{
			ID:   po.ID,
			Name: po.Name,
			Mode: po.Mode,
		})
	}

	if p.os10Config.VLTConfig != nil {
		cfg.VLTConfig = p.os10Config.VLTConfig
	}

	return cfg
}

// ===========================
// VLT Topology Inference
// ===========================

// InferVLTTopology analyzes VLT config to infer topology links.
type VLTTopologyLink struct {
	FromDevice    string
	FromInterface string
	ToDevice      string
	ToInterface   string
	VLTID         int
	LinkType      string // "peer-link", "vlt-member", "backup-link"
}

// InferVLTLinksFromConfig derives VLT topology links from parsed config
func InferVLTLinksFromConfig(hostname string, vlt *models.OS10VLTConfig, interfaces []models.OS10Interface) []VLTTopologyLink {
	var links []VLTTopologyLink

	if vlt == nil {
		return links
	}

	// Peer-link
	if vlt.PeerLink != "" {
		links = append(links, VLTTopologyLink{
			FromDevice:    hostname,
			FromInterface: vlt.PeerLink,
			ToDevice:      "<vlt-peer>",
			ToInterface:   vlt.PeerLink,
			LinkType:      "peer-link",
		})
	}

	// Discovery interface
	if vlt.DiscoveryInterface != "" {
		links = append(links, VLTTopologyLink{
			FromDevice:    hostname,
			FromInterface: vlt.DiscoveryInterface,
			ToDevice:      "<vlt-peer>",
			ToInterface:   "<vlti>",
			LinkType:      "peer-link",
		})
	}

	// Backup destination
	if vlt.BackupDestination != "" {
		links = append(links, VLTTopologyLink{
			FromDevice:    hostname,
			FromInterface: "<mgmt>",
			ToDevice:      "<vlt-peer>",
			ToInterface:   vlt.BackupDestination,
			LinkType:      "backup-link",
		})
	}

	// VLT member interfaces
	for _, iface := range interfaces {
		lowerName := strings.ToLower(iface.Name)
		if strings.HasPrefix(lowerName, "port-channel") || strings.HasPrefix(lowerName, "vlan") ||
			strings.HasPrefix(lowerName, "loopback") || strings.HasPrefix(lowerName, "mgmt") ||
			strings.HasPrefix(lowerName, "management") {
			continue
		}
		if iface.LAGGroup > 0 {
			links = append(links, VLTTopologyLink{
				FromDevice:    hostname,
				FromInterface: iface.Name,
				ToInterface:   fmt.Sprintf("port-channel%d", iface.LAGGroup),
				LinkType:      "vlt-member",
			})
		}
	}

	return links
}

// ===========================
// Helpers
// ===========================

// computeOS10SnippetHash computes a hash of config lines
func computeOS10SnippetHash(lines []string) string {
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

// intPtr returns a pointer to an int
func intPtr(v int) *int { return &v }

// stringPtr returns a pointer to a string
func stringPtr(v string) *string { return &v }

// ===========================
// OS10 L2 Config
// ===========================

// OS10L2Config holds OS10 L2 topology data (analogous to EOSL2Config)
type OS10L2Config struct {
	Hostname            string
	Version             string
	VLANs               []models.OS10L2VLAN
	Interfaces          []models.OS10L2Interface
	PortChannels        []models.OS10PortChannelSummary
	SVIs                []models.OS10SVI
	VLTConfig           *models.OS10VLTConfig
	RawConfig           string
	ConfigHash          string
	SourceSnippetHashes map[string]string
}

// ===========================
// FTOS backward compatibility
// ===========================

// NormalizeFTOSInterface renames FTOS-style interface names to OS10 format.
// FTOS: TenGigabitEthernet 0/1 -> OS10: ethernet1/1/1
// FTOS: GigabitEthernet 0/1   -> OS10: ethernet1/1/1
// FTOS: fortyGigE 0/1          -> OS10: ethernet1/1/1
// FTOS: Port-channel 1         -> OS10: port-channel1 (unchanged)
func NormalizeFTOSInterface(ftosName string) string {
	lower := strings.ToLower(ftosName)
	lower = strings.ReplaceAll(lower, " ", "")

	switch {
	case strings.HasPrefix(lower, "tengigabitethernet"):
		rest := strings.TrimPrefix(lower, "tengigabitethernet")
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			slot := parts[0]
			port := parts[1]
			return fmt.Sprintf("ethernet%s/1/%s", slot, port)
		}
		return strings.Replace(lower, "tengigabitethernet", "ethernet", 1)

	case strings.HasPrefix(lower, "gigabitethernet"):
		rest := strings.TrimPrefix(lower, "gigabitethernet")
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			slot := parts[0]
			port := parts[1]
			return fmt.Sprintf("ethernet%s/1/%s", slot, port)
		}
		return strings.Replace(lower, "gigabitethernet", "ethernet", 1)

	case strings.HasPrefix(lower, "fortygige"):
		rest := strings.TrimPrefix(lower, "fortygige")
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			slot := parts[0]
			port := parts[1]
			return fmt.Sprintf("ethernet%s/1/%s", slot, port)
		}
		return strings.Replace(lower, "fortygige", "ethernet", 1)

	case strings.HasPrefix(lower, "twentyfivegige"):
		rest := strings.TrimPrefix(lower, "twentyfivegige")
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			slot := parts[0]
			port := parts[1]
			return fmt.Sprintf("ethernet%s/1/%s", slot, port)
		}
		return strings.Replace(lower, "twentyfivegige", "ethernet", 1)

	case strings.HasPrefix(lower, "hundredgige"):
		rest := strings.TrimPrefix(lower, "hundredgige")
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			slot := parts[0]
			port := parts[1]
			return fmt.Sprintf("ethernet%s/1/%s", slot, port)
		}
		return strings.Replace(lower, "hundredgige", "ethernet", 1)

	default:
		return lower
	}
}

// DetectOSVersion attempts to detect whether a config is FTOS or OS10
// based on command patterns.
func DetectOSVersion(config string) string {
	lower := strings.ToLower(config)
	if strings.Contains(lower, "vlt-domain") {
		return "OS10"
	}
	if strings.Contains(lower, "tengigabitethernet") || strings.Contains(lower, "fortygige") {
		return "FTOS"
	}
	if strings.Contains(lower, "interface ethernet") {
		return "OS10"
	}
	return "FTOS"
}
