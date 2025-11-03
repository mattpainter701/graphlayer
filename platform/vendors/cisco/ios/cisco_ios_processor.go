package cisco

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
	helper "github.com/mattpainter701/graphlayer/platform/core/helpers"
	docinterfaces "github.com/mattpainter701/graphlayer/platform/core/interfaces"
	connectorInterfaces "github.com/mattpainter701/graphlayer/platform/core/interfaces/connectors"
	"github.com/mattpainter701/graphlayer/platform/core/processor"
)

// CiscoIOSProcessor handles parsing and processing of Cisco IOS configuration files
type CiscoIOSProcessor struct {
	filePath                      string
	iosConfig                     *models.IOSConfig
	IntegrationMessages           []models.IntegrationMessage
	Store                         docinterfaces.IParserStore
	MessageBroker                 connectorInterfaces.IMessageBroker
	ParsedNetworkNodes            map[string]models.NetworkNode
	ParsedNetworkNodesByInterface map[string][]models.NetworkNode
	Logger                        *zerolog.Logger
	MessageHelper                 *helper.MessageHelper

	TopologyDevice           *models.Device
	TopologyInterfaces       map[string]models.Interface
	TopologyAddresses        map[string]models.Address
	TopologyAddressGroups    map[string]models.AddressGroup
	TopologyServices         map[string]models.Service
	TopologyServiceGroups    map[string]models.ServiceGroup
	TopologyRoutingEntries   map[string]models.RoutingEntry
	TopologyNATs             map[string]models.NAT
	TopologyVIPs             map[string]models.VIP
	TopologyFirewallPolicies map[string]models.FirewallPolicy
	TopologyZones            map[string]models.Zone
	TopologyVirtualContexts  map[string]models.VirtualContext
	FileHash                 string
	UserID                   string
}

// NewCiscoIOSProcessor creates a new instance of CiscoIOSProcessor for processing Cisco IOS configurations
func NewCiscoIOSProcessor(filePath string, store docinterfaces.IParserStore, messageBroker connectorInterfaces.IMessageBroker, logger *zerolog.Logger, messageHelper *helper.MessageHelper) *CiscoIOSProcessor {
	return &CiscoIOSProcessor{
		filePath:                      filePath,
		IntegrationMessages:           []models.IntegrationMessage{},
		Store:                         store,
		MessageBroker:                 messageBroker,
		ParsedNetworkNodes:            make(map[string]models.NetworkNode),
		ParsedNetworkNodesByInterface: make(map[string][]models.NetworkNode),
		Logger:                        logger,
		MessageHelper:                 messageHelper,
	}
}

func (p *CiscoIOSProcessor) addIntegrationMessage(msgType models.IntegrationMessageType, msg string) {
	p.MessageHelper.AddMessage(msgType, msg)
}

// ProcessIOSFile processes a Cisco IOS configuration file and extracts network topology information
func (p *CiscoIOSProcessor) ProcessIOSFile() error {
	defer func() {
		if err := p.MessageHelper.SaveMessages(); err != nil {
			helper.Logger.Error().Msgf("Failed to save integration messages: %v", err)
		}
	}()
	p.addIntegrationMessage(models.IntegrationMessageTypeInfo, "Starting to process IOS config")

	iosConfig, err := p.ParseIOSConfig(p.filePath)
	if err != nil {
		p.addIntegrationMessage(models.IntegrationMessageTypeError, fmt.Sprintf("Failed to parse IOS config: %v", err))
		return err
	}
	p.iosConfig = iosConfig
	p.FileHash = helper.EncodeToSha256(iosConfig.RawConfig)

	err = p.BuildNetworkTopologyModels()
	if err != nil {
		p.Logger.Error().Err(err).Msg(fmt.Sprintf("Failed to build network topology models: %v", err))
		return err
	}

	err = p.Store.InsertSegmentationConfig(models.SegmentationConfig{
		SegmentationType: models.SegmentationTypeRouter,
		Vendor:           models.SegmentationVendorCisco,
		DeviceName:       p.iosConfig.Hostname,
		CreatedAt:        time.Now().Format(time.RFC3339),
	})
	if err != nil {
		p.Logger.Error().Err(err).Msg(fmt.Sprintf("Failed to save segmentation config: %v", err))
		return err
	}

	// map the data to the parsed data
	parsedNetworkTopology := CiscoIOSMapper(p)

	// get owned addresses for this router topology
	ownedAddresses := p.getOwnedAddresses(parsedNetworkTopology.Addresses, parsedNetworkTopology.Interfaces)
	parsedNetworkTopology.OwnedAddresses = ownedAddresses

	networkNodes := processor.MapAddressesToNetworkNodes(&parsedNetworkTopology, models.IntegrationCiscoIOS.String())

	// insert network nodes
	if _, err := p.Store.MergeOrCreateNetworkNodes(context.TODO(), networkNodes, p.TopologyDevice.Hostname); err != nil {
		helper.Logger.Error().Msg(err.Error())
		return err
	}

	// get all network groups
	networkGroups := []string{}
	for _, networkNode := range networkNodes {
		networkGroups = append(networkGroups, networkNode.AddressGroups...)
	}

	// remove duplicates
	networkGroups = helper.RemoveDuplicates(networkGroups)

	// insert network groups
	if err := p.Store.AddNetworkGroups(networkGroups); err != nil {
		helper.Logger.Error().Msg(err.Error())
		return err
	}

	if err := p.saveDeviceAsAsset(networkNodes); err != nil {
		p.Logger.Warn().Err(err).Msg("Failed to save topology device as asset node")
	}

	err = p.Store.SaveParsedData(&parsedNetworkTopology)
	if err != nil {
		p.Logger.Error().Err(err).Msg(fmt.Sprintf("Failed to save parsed topology: %v", err))
		return err
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo, "IOS config processing completed successfully")

	return nil
}

// ParseIOSConfig parses a Cisco IOS configuration file from the given file path
func (p *CiscoIOSProcessor) ParseIOSConfig(filePath string) (*models.IOSConfig, error) {
	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		p.addIntegrationMessage(models.IntegrationMessageTypeError, fmt.Sprintf("Failed to read config file: %v", err))
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	configText := string(content)
	scanner := bufio.NewScanner(strings.NewReader(configText))
	p.iosConfig = &models.IOSConfig{
		RawConfig: configText,
	}

	// Create a slice to store all lines
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		p.addIntegrationMessage(models.IntegrationMessageTypeError, fmt.Sprintf("error scanning config file: %v", err))
		return nil, fmt.Errorf("error scanning config file: %w", err)
	}

	// Define "sub-parsers" for single-line configurations
	singleLineParsers := map[string]func(string){
		"hostname ": func(line string) {
			p.iosConfig.Hostname = strings.TrimPrefix(line, "hostname ")
			p.addIntegrationMessage(models.IntegrationMessageTypeInfo, fmt.Sprintf("Identified hostname: %s", p.iosConfig.Hostname))
		},
		"version ": func(line string) {
			p.iosConfig.Version = strings.TrimPrefix(line, "version ")
			p.addIntegrationMessage(models.IntegrationMessageTypeInfo, fmt.Sprintf("Identified version: %s", p.iosConfig.Version))
		},
		"access-list ": func(line string) {
			p.parseACLLine(line)
		},
		"ip route ": func(line string) {
			route := p.parseIOSRoute(line)
			p.iosConfig.Routes = append(p.iosConfig.Routes, route)
		},
		"ip nat ": func(line string) {
			natRule := p.parseIOSNAT(line)
			if natRule.Type != "" {
				p.iosConfig.NATRules = append(p.iosConfig.NATRules, natRule)
			}
		},
	}

	// Define "subparsers" for multi-line blocks
	blockParsers := map[string]func([]string, int) (int, bool){
		"interface ": func(lines []string, i int) (int, bool) {
			newIndex, parsedInterface := p.parseBlockConfig(lines, i, func(blockLines []string) (int, interface{}) {
				iface := p.parseIOSInterfaceFromLines(blockLines)
				// Validate the interface
				if err := iface.ValidateIOSInterface(); err != nil {
					p.addIntegrationMessage(models.IntegrationMessageTypeWarning, fmt.Sprintf("Warning: %v", err))
				}
				return len(blockLines), iface
			})

			p.iosConfig.Interfaces = append(p.iosConfig.Interfaces, parsedInterface.(models.CiscoIOSInterface))
			return newIndex, true
		},
		"ip access-list ": func(lines []string, i int) (int, bool) {
			newIndex, parsedACL := p.parseBlockConfig(lines, i, func(blockLines []string) (int, interface{}) {
				namedACL := p.parseNamedACLFromLines(blockLines)
				return len(blockLines), namedACL
			})

			namedACL := parsedACL.(models.CiscoNamedACL)
			if namedACL.Name != "" {
				p.iosConfig.NamedACLs = append(p.iosConfig.NamedACLs, namedACL)
			}
			return newIndex, true
		},
		"vlan ": func(lines []string, i int) (int, bool) {
			newIndex, parsedVLAN := p.parseBlockConfig(lines, i, func(blockLines []string) (int, interface{}) {
				vlan := p.parseVLANFromLines(blockLines)
				return len(blockLines), vlan
			})

			vlan := parsedVLAN.(models.CiscoVLAN)
			if vlan.ID != 0 {
				p.iosConfig.VLANs = append(p.iosConfig.VLANs, vlan)
			}
			return newIndex, true
		},
	}

	// Process the lines
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}

		// Check for block parsers first
		blockParsed := false
		for prefix, blockParser := range blockParsers {
			if strings.HasPrefix(line, prefix) {
				newIndex, parsed := blockParser(lines, i)
				if parsed {
					i = newIndex
					blockParsed = true
					break
				}
			}
		}
		if blockParsed {
			continue
		}

		// Then check for single line parsers
		for prefix, parser := range singleLineParsers {
			if strings.HasPrefix(line, prefix) {
				parser(line)
				break
			}
		}
	}

	// Log statistics
	p.addIntegrationMessage(models.IntegrationMessageTypeInfo, fmt.Sprintf("Identified interfaces: %d", len(p.iosConfig.Interfaces)))
	p.addIntegrationMessage(models.IntegrationMessageTypeInfo, fmt.Sprintf("Identified standard ACLs: %d", len(p.iosConfig.StandardACLs)))
	p.addIntegrationMessage(models.IntegrationMessageTypeInfo, fmt.Sprintf("Identified extended ACLs: %d", len(p.iosConfig.ExtendedACLs)))
	p.addIntegrationMessage(models.IntegrationMessageTypeInfo, fmt.Sprintf("Identified named ACLs: %d", len(p.iosConfig.NamedACLs)))
	p.addIntegrationMessage(models.IntegrationMessageTypeInfo, fmt.Sprintf("Identified static routes: %d", len(p.iosConfig.Routes)))
	p.addIntegrationMessage(models.IntegrationMessageTypeInfo, fmt.Sprintf("Identified NAT rules: %d", len(p.iosConfig.NATRules)))
	p.addIntegrationMessage(models.IntegrationMessageTypeInfo, fmt.Sprintf("Identified VLANs: %d", len(p.iosConfig.VLANs)))

	return p.iosConfig, nil
}

// parseBlockConfig parses a multi-line configuration block and returns the new line index
// this is a helper method for handling config blocks that are indented
func (p *CiscoIOSProcessor) parseBlockConfig(lines []string, startIndex int, parser func([]string) (int, interface{})) (int, interface{}) {
	// Find the end of this block
	endIndex := startIndex
	for i := startIndex + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		// Block ends when we hit an empty line, comment, or a line that doesn't start with whitespace
		if line == "" || strings.HasPrefix(line, "!") || !strings.HasPrefix(lines[i], " ") {
			endIndex = i - 1
			break
		}
		// If we reach the end of the file
		if i == len(lines)-1 {
			endIndex = i
		}
	}

	// Pass the exact block to the parser function
	blockLines := lines[startIndex : endIndex+1]
	_, result := parser(blockLines)

	// This accounts for the main loop's automatic increment
	return endIndex, result
}

// saveDeviceAsAsset converts the parsed topology device into an AssetNode and persists it
func (p *CiscoIOSProcessor) saveDeviceAsAsset(networkNodes []models.NetworkNode) error {
	if p.TopologyDevice == nil {
		return fmt.Errorf("topology device is nil")
	}

	// Build Asset using shared helper
	ifaceSlice := make([]models.Interface, 0, len(p.TopologyInterfaces))
	for _, v := range p.TopologyInterfaces {
		ifaceSlice = append(ifaceSlice, v)
	}
	asset := helper.BuildDeviceAssetNode(
		*p.TopologyDevice,
		"IOS",
		"Router",
		models.IntegrationCisco.String(),
		p.UserID,
		ifaceSlice,
		networkNodes,
	)

	if err := p.Store.PatchAssetNodes([]models.AssetNode{asset}); err != nil {
		return fmt.Errorf("patch asset nodes: %w", err)
	}
	return nil
}

// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |L|2| |W|o|r|k|f|l|o|w| |O|r|c|h|e|s|t|r|a|t|i|o|n|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
/*
 * The following methods orchestrate Layer-2 visibility processing.
 * They extend the standard L3 pipeline with switch topology, security posture,
 * and host placement ingestion.
 */

// ProcessIOSFileEnhanced executes the standard L3 pipeline plus Layer-2 visibility processing.
// Adds device fingerprinting, switch topology export, and host placement ingestion.
func (p *CiscoIOSProcessor) ProcessIOSFileEnhanced() error {
	// Execute standard L3 processing first
	if err := p.ProcessIOSFile(); err != nil {
		return err
	}

	// Build L2 configuration from parsed IOS config
	l2Config := p.buildCiscoL2Config()

	// Optionally supplement FDB entries from telemetry sources (placement evidence for L2 outcomes)
	if entries, err := p.collectFDBEntries(); err == nil {
		l2Config.FDBEntries = append(l2Config.FDBEntries, entries...)
	} else {
		p.addIntegrationMessage(models.IntegrationMessageTypeWarning, fmt.Sprintf("FDB collection failed: %v", err))
	}

	// Map to vendor-agnostic L2 models
	l2Switch, fdbEntries := p.buildIOSLayer2Models(l2Config)

	// Persist L2 topology and host placement data
	if err := p.persistLayer2Models(l2Switch, fdbEntries); err != nil {
		p.addIntegrationMessage(models.IntegrationMessageTypeError, fmt.Sprintf("Persist L2 models failed: %v", err))
		return err
	}

	p.addIntegrationMessage(models.IntegrationMessageTypeInfo,
		fmt.Sprintf("Layer2 summary: switch=%s vlans=%d ports=%d fdb=%d", l2Switch.DeviceID, len(l2Switch.VLANs), len(l2Switch.Interfaces), len(fdbEntries)))

	return nil
}

// collectFDBEntries is a placeholder for CLI/SNMP/Claroty/API ingestion logic.
// L2 placement scope: enrich switch-derived topology with telemetry/API sources.
// Returns MAC-to-port evidence with confidence and freshness metadata.
func (p *CiscoIOSProcessor) collectFDBEntries() ([]models.CiscoFDBEntry, error) {
	// TODO: Implement real FDB collection via:
	// - CLI: "show mac address-table"
	// - SNMP: dot1dTpFdbTable / dot1qTpFdbTable
	// - API: Claroty CDT, xDome, asset-source, Cisco DNA Center
	// - Telemetry: Streaming telemetry subscriptions
	
	// Example stub implementation:
	// var entries []models.CiscoFDBEntry
	// if p.Store.SupportsFDBQuery() {
	//     entries, err = p.Store.QueryFDBForDevice(p.iosConfig.Hostname)
	//     if err != nil {
	//         return nil, fmt.Errorf("query FDB: %w", err)
	//     }
	// }
	// return entries, nil

	return nil, nil
}
