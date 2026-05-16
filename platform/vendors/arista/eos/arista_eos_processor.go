package eos

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |E|O|S| |P|r|o|c|e|s|s|o|r|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The AristaEOSProcessor handles parsing and processing of Arista EOS
 * configuration files. It follows the same pattern as CiscoIOSProcessor
 * but is adapted for EOS-specific syntax and features (MLAG, VXLAN).
 */

// AristaEOSProcessor handles EOS config parsing and topology model building
type AristaEOSProcessor struct {
	filePath  string
	eosConfig *models.EOSConfig
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

// NewAristaEOSProcessor creates a new processor instance
func NewAristaEOSProcessor(filePath string) *AristaEOSProcessor {
	return &AristaEOSProcessor{
		filePath: filePath,
	}
}

// ProcessEOSFile processes an EOS configuration file
func (p *AristaEOSProcessor) ProcessEOSFile() error {
	eosConfig, err := p.ParseEOSConfig()
	if err != nil {
		return fmt.Errorf("parse EOS config: %w", err)
	}
	p.eosConfig = eosConfig
	p.FileHash = computeEOSSnippetHash(strings.Split(eosConfig.RawConfig, "\n"))

	if err := p.BuildNetworkTopologyModels(); err != nil {
		return fmt.Errorf("build topology models: %w", err)
	}

	return nil
}

// ParseEOSConfig parses an EOS configuration file from disk
func (p *AristaEOSProcessor) ParseEOSConfig() (*models.EOSConfig, error) {
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

	cfg := &models.EOSConfig{
		RawConfig: configText,
	}

	// Extract stanzas (EOS uses indented blocks akin to Cisco)
	stanzas := extractEOSStanzas(lines)

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
			route := parseEOSStaticRoute(trimmed)
			cfg.StaticRoutes = append(cfg.StaticRoutes, route)
		}
	}

	// Process block stanzas
	for header, block := range stanzas {
		lower := strings.ToLower(header)

		switch {
		case strings.HasPrefix(lower, "interface "):
			iface := p.parseInterface(header, block)
			if iface.Name != "" {
				cfg.Interfaces = append(cfg.Interfaces, iface)
			}

		case strings.HasPrefix(lower, "vlan "):
			vlanIDStr := strings.TrimSpace(header[len("vlan "):])
			if vlanID, err := strconv.Atoi(vlanIDStr); err == nil {
				vlan := parseEOSVLANBlock(block, vlanID)
				cfg.VLANs = append(cfg.VLANs, vlan)
			}

		case strings.HasPrefix(lower, "mlag configuration"):
			mlag := parseEOSMLAGConfig(block)
			if mlag != nil {
				cfg.MLAGConfig = mlag
			}

		case strings.HasPrefix(lower, "interface vxlan"):
			vxlan := parseEOSVXLANConfig(block)
			if vxlan != nil {
				cfg.VXLANConfig = vxlan
			}

		case strings.HasPrefix(lower, "interface port-channel"):
			po := p.parsePortChannel(header, block)
			if po.ID != 0 {
				cfg.PortChannels = append(cfg.PortChannels, po)
			}

		case strings.HasPrefix(lower, "ip access-list "):
			acl := parseEOSExtendedACL(block)
			if acl.Name != "" {
				cfg.ExtendedACLs = append(cfg.ExtendedACLs, acl)
			}

		case strings.HasPrefix(lower, "router ospf "):
			// Parse OSPF router config from block
			cfg.OSPFRouter = parseEOSOSPFRouter(header, block)

		case strings.HasPrefix(lower, "router bgp "):
			// Parse BGP router config from block
			cfg.BGPConfig = parseEOSBGPConfig(header, block)

		case strings.HasPrefix(lower, "vrf instance "):
			vrf := parseEOSVRF(header, block)
			if vrf.Name != "" {
				cfg.VRFConfigs = append(cfg.VRFConfigs, vrf)
			}
		}
	}

	return cfg, nil
}

// extractEOSStanzas splits EOS config into named blocks by top-level commands
func extractEOSStanzas(lines []string) map[string][]string {
	stanzas := make(map[string][]string)
	var currentHeader string
	var currentBlock []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "!") {
			// Save current block on blank/comment line
			if currentHeader != "" && len(currentBlock) > 0 {
				stanzas[currentHeader] = currentBlock
			}
			currentHeader = ""
			currentBlock = nil
			continue
		}

		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// Save previous block
			if currentHeader != "" && len(currentBlock) > 0 {
				stanzas[currentHeader] = currentBlock
			}
			currentHeader = strings.TrimSpace(line)
			currentBlock = []string{line}
		} else if currentHeader != "" {
			currentBlock = append(currentBlock, line)
		}
	}

	// Save last block
	if currentHeader != "" && len(currentBlock) > 0 {
		stanzas[currentHeader] = currentBlock
	}

	return stanzas
}

func (p *AristaEOSProcessor) parseInterface(header string, block []string) models.EOSInterface {
	iface := models.EOSInterface{
		Name: strings.TrimSpace(header[len("interface "):]),
	}

	parsed := parseEOSInterfaceBlock(block)
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
	isVXLAN := strings.HasPrefix(lowerName, "vxlan")
	isManagement := strings.HasPrefix(lowerName, "management")

	if isSVI || isLoopback || isVXLAN || isManagement {
		iface.Switchport = false
		iface.SwitchportMode = "routed"
		if isSVI {
			iface.VLAN = deriveEOSSVIVID(iface.Name)
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

func (p *AristaEOSProcessor) parsePortChannel(header string, block []string) models.EOSPortChannel {
	po := models.EOSPortChannel{
		Name: strings.TrimSpace(header[len("interface "):]),
	}

	idStr := strings.TrimSpace(strings.ToLower(strings.TrimPrefix(po.Name, "Port-Channel")))
	if id, err := strconv.Atoi(idStr); err == nil {
		po.ID = id
	}

	parsed := parseEOSInterfaceBlock(block)
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

func deriveEOSSVIVID(name string) int {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "vlan") {
		idStr := strings.TrimSpace(name[4:])
		if id, err := strconv.Atoi(idStr); err == nil {
			return id
		}
	}
	return 0
}

// Route Config Parser: Router OSPF
func parseEOSOSPFRouter(header string, lines []string) *models.EOSOSPFRouter {
	// header: "router ospf <process-id>"
	fields := strings.Fields(header)
	ospf := &models.EOSOSPFRouter{}
	if len(fields) >= 3 {
		if v, err := strconv.Atoi(fields[2]); err == nil {
			ospf.ProcessID = v
		}
	}
	ospf.Areas = make(map[string]models.EOSOSPFArea)

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
			// "network 10.0.0.0 0.0.0.255 area 0.0.0.0"
			parts := strings.Fields(line)
			if len(parts) >= 5 && parts[len(parts)-2] == "area" {
				network := models.EOSOSPFNetwork{
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

// Route Config Parser: Router BGP
func parseEOSBGPConfig(header string, lines []string) *models.EOSBGPConfig {
	// header: "router bgp <asn>"
	fields := strings.Fields(header)
	bgp := &models.EOSBGPConfig{}
	if len(fields) >= 3 {
		if v, err := strconv.Atoi(fields[2]); err == nil {
			bgp.ASN = v
		}
	}

	var currentVRF *models.EOSBGPVRF

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
			peer := parseEOSBGPPeerLine(line)
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
				currentVRF = &models.EOSBGPVRF{Name: parts[1]}
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

func parseEOSBGPPeerLine(line string) models.EOSBGPPeer {
	peer := models.EOSBGPPeer{}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return peer
	}

	// "neighbor <ip> remote-as <asn>"
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
		case fields[i] == "multihop" && i+1 < len(fields):
			if v, err := strconv.Atoi(fields[i+1]); err == nil {
				peer.Multihop = v
			}
			i++
		}
	}
	return peer
}

// VRF instance parser
func parseEOSVRF(header string, lines []string) models.EOSVRF {
	vrf := models.EOSVRF{}
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
// L2 Config Builder
// ===========================

// BuildEOSL2Config builds an EOS L2 config object from parsed data
func (p *AristaEOSProcessor) BuildEOSL2Config() models.EOSL2Config {
	cfg := models.EOSL2Config{
		Hostname:            p.eosConfig.Hostname,
		Version:             p.eosConfig.Version,
		RawConfig:           p.eosConfig.RawConfig,
		ConfigHash:          p.FileHash,
		SourceSnippetHashes: make(map[string]string),
	}

	stanzas := extractEOSStanzas(strings.Split(p.eosConfig.RawConfig, "\n"))

	for _, iface := range p.eosConfig.Interfaces {
		lowerName := strings.ToLower(iface.Name)
		if strings.HasPrefix(lowerName, "vlan") {
			svi := models.EOSSVI{
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

		l2Iface := models.EOSL2Interface{
			Name:            iface.Name,
			AdminState:      map[bool]string{true: "up", false: "down"}[!iface.Shutdown],
			Mode:            iface.SwitchportMode,
			AccessVLAN:      intPtr(iface.AccessVLAN),
			TrunkAllowedVLANs: iface.TrunkVLANs,
			TrunkNativeVLAN: intPtr(iface.TrunkNative),
			PortChannelID:   intPtr(iface.LAGGroup),
			Description:     iface.Description,
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

	for _, vlan := range p.eosConfig.VLANs {
		l2vlan := models.EOSL2VLAN{
			ID:    vlan.ID,
			Name:  vlan.Name,
			State: vlan.State,
		}
		cfg.VLANs = append(cfg.VLANs, l2vlan)
	}

	for _, po := range p.eosConfig.PortChannels {
		cfg.PortChannels = append(cfg.PortChannels, models.EOSPortChannelSummary{
			ID:   po.ID,
			Name: po.Name,
			Mode: po.Mode,
		})
	}

	if p.eosConfig.MLAGConfig != nil {
		cfg.MLAGConfig = p.eosConfig.MLAGConfig
	}

	return cfg
}

// ===========================
// MLAG Topology Inference
// ===========================

// InferMLAGTopology analyzes MLAG config to infer topology links.
// Returns inferred links (device pair interconnects) and per-MLAG port info.
type MLAGTopologyLink struct {
	FromDevice    string
	FromInterface string
	ToDevice      string
	ToInterface   string
	MLAGID        int
	LinkType      string // "peer-link", "mlag-member"
}

// InferMLAGLinksFromConfig derives MLAG topology links from parsed config
func InferMLAGLinksFromConfig(hostname string, mlag *models.EOSMLAGConfig, interfaces []models.EOSInterface) []MLAGTopologyLink {
	var links []MLAGTopologyLink

	if mlag == nil {
		return links
	}

	// Peer-link
	if mlag.PeerLink != "" {
		links = append(links, MLAGTopologyLink{
			FromDevice:    hostname,
			FromInterface: mlag.PeerLink,
			ToDevice:      "<mlag-peer>",
			ToInterface:   mlag.PeerLink,
			LinkType:      "peer-link",
		})
	}

	// Interfaces that are MLAG members
	for _, iface := range interfaces {
		lowerName := strings.ToLower(iface.Name)
		if strings.HasPrefix(lowerName, "port-channel") || strings.HasPrefix(lowerName, "vlan") ||
			strings.HasPrefix(lowerName, "loopback") || strings.HasPrefix(lowerName, "management") {
			continue
		}
		if iface.LAGGroup > 0 {
			links = append(links, MLAGTopologyLink{
				FromDevice:    hostname,
				FromInterface: iface.Name,
				ToInterface:   fmt.Sprintf("Port-Channel%d", iface.LAGGroup),
				LinkType:      "mlag-member",
			})
		}
	}

	return links
}

// EOSL2Config holds EOS L2 topology data (analogous to CiscoL2Config)
type EOSL2Config struct {
	Hostname            string
	Version             string
	VLANs               []models.EOSL2VLAN
	Interfaces          []models.EOSL2Interface
	PortChannels        []models.EOSPortChannelSummary
	SVIs                []models.EOSSVI
	MLAGConfig          *models.EOSMLAGConfig
	RawConfig           string
	ConfigHash          string
	SourceSnippetHashes map[string]string
}
