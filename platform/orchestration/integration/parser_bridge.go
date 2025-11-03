package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/orchestration"
	// Parser packages are imported but commented until full integration is complete
	// "github.com/mattpainter701/graphlayer/platform/platform/vendors/cisco/ios"
	// "github.com/mattpainter701/graphlayer/platform/platform/vendors/cisco/asa"
	// "github.com/mattpainter701/graphlayer/platform/platform/vendors/cisco/nxos"
	// "github.com/mattpainter701/graphlayer/platform/platform/vendors/aruba/aoscx"
	// "github.com/mattpainter701/graphlayer/platform/platform/vendors/juniper/junos"
	// "github.com/mattpainter701/graphlayer/platform/platform/vendors/paloalto/firewall"
	// "github.com/mattpainter701/graphlayer/platform/platform/vendors/fortinet/fortigate"
)

// ParserBridge connects connectors to parsers
// This provides the integration point between the connector layer and your existing parsers
type ParserBridge struct {
	orchestrator    *orchestration.ConnectorOrchestrator
	// Future: Add parser store, message broker, logger interfaces here
	// store         docinterfaces.IParserStore
	// messageBroker connectorInterfaces.IMessageBroker
	// logger        *zerolog.Logger
}

// NewParserBridge creates a new parser bridge
func NewParserBridge(orch *orchestration.ConnectorOrchestrator) *ParserBridge {
	return &ParserBridge{
		orchestrator: orch,
		// store:         store,
		// messageBroker: broker,
		// logger:        logger,
	}
}

// ParseCollectionResult processes collected files with appropriate parser
func (pb *ParserBridge) ParseCollectionResult(ctx context.Context, result *orchestration.CollectionResult) error {
	for _, filePath := range result.ConfigFiles {
		if err := pb.parseFile(ctx, filePath); err != nil {
			// pb.logger.Error().Err(err).Str("file", filePath).Msg("Failed to parse file")
			fmt.Printf("Failed to parse file %s: %v\n", filePath, err)
			continue
		}
		// pb.logger.Info().Str("file", filePath).Msg("Successfully parsed file")
		fmt.Printf("Successfully parsed file: %s\n", filePath)
	}
	return nil
}

// parseFile routes file to appropriate parser based on device type and file characteristics
func (pb *ParserBridge) parseFile(ctx context.Context, filePath string) error {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Determine device type and route to appropriate parser
	deviceType := pb.detectDeviceType(string(content), filePath)

	filename := filepath.Base(filePath)
	fmt.Printf("Parsing file: %s (detected type: %s)\n", filename, deviceType)

	// Route to vendor-specific parser
	switch deviceType {
	case "cisco-ios":
		return pb.parseCiscoIOS(ctx, string(content), filePath)
	case "cisco-asa":
		return pb.parseCiscoASA(ctx, string(content), filePath)
	case "cisco-nxos":
		return pb.parseCiscoNXOS(ctx, string(content), filePath)
	case "aruba-aoscx":
		return pb.parseArubaAOSCX(ctx, string(content), filePath)
	case "juniper-junos":
		return pb.parseJuniperJunos(ctx, string(content), filePath)
	case "paloalto-panos":
		return pb.parsePaloAltoPANOS(ctx, string(content), filePath)
	case "fortinet-fortigate":
		return pb.parseFortinetFortiGate(ctx, string(content), filePath)
	default:
		return fmt.Errorf("unsupported device type: %s", deviceType)
	}
}

// detectDeviceType determines device type from file content and path
func (pb *ParserBridge) detectDeviceType(content string, filePath string) string {
	filename := strings.ToLower(filepath.Base(filePath))

	// Check path for vendor indicators
	path := strings.ToLower(filePath)
	if strings.Contains(path, "cisco") {
		if strings.Contains(content, "ASA Version") || strings.Contains(content, "Cisco Adaptive Security Appliance") {
			return "cisco-asa"
		}
		if strings.Contains(content, "Nexus") || strings.Contains(content, "NX-OS") {
			return "cisco-nxos"
		}
		return "cisco-ios"
	}
	if strings.Contains(path, "aruba") || strings.Contains(path, "aoscx") {
		return "aruba-aoscx"
	}
	if strings.Contains(path, "juniper") || strings.Contains(path, "junos") {
		return "juniper-junos"
	}
	if strings.Contains(path, "paloalto") || strings.Contains(path, "panos") {
		return "paloalto-panos"
	}
	if strings.Contains(path, "fortinet") || strings.Contains(path, "fortigate") {
		return "fortinet-fortigate"
	}

	// Check content for vendor signatures
	if strings.Contains(content, "version") && strings.Contains(content, "interface") {
		if strings.Contains(content, "switchport") {
			return "cisco-ios"
		}
		if strings.Contains(content, "set system") || strings.Contains(content, "set interfaces") {
			return "juniper-junos"
		}
	}

	if strings.Contains(content, "ArubaOS-CX") {
		return "aruba-aoscx"
	}

	if strings.Contains(content, "config system global") || strings.Contains(content, "config firewall") {
		return "fortinet-fortigate"
	}

	return "unknown"
}

// parseCiscoIOS parses Cisco IOS configuration
func (pb *ParserBridge) parseCiscoIOS(ctx context.Context, content string, filePath string) error {
	// TODO: Complete IOS parser integration
	// For now, just log that we're parsing
	fmt.Printf("Parsing Cisco IOS config: %s\n", filepath.Base(filePath))

	// Example of how the full integration would look:
	// processor := ios.NewCiscoIOSProcessor()
	// processor.ParseConfig(content)
	// l2Config := processor.BuildCiscoL2Config()
	// Save to store/database

	return nil
}

// parseCiscoASA parses Cisco ASA configuration
func (pb *ParserBridge) parseCiscoASA(ctx context.Context, content string, filePath string) error {
	fmt.Printf("Parsing Cisco ASA config: %s\n", filepath.Base(filePath))
	return nil
}

// parseCiscoNXOS parses Cisco NX-OS configuration
func (pb *ParserBridge) parseCiscoNXOS(ctx context.Context, content string, filePath string) error {
	fmt.Printf("Parsing Cisco NX-OS config: %s\n", filepath.Base(filePath))
	return nil
}

// parseArubaAOSCX parses Aruba AOS-CX configuration
func (pb *ParserBridge) parseArubaAOSCX(ctx context.Context, content string, filePath string) error {
	fmt.Printf("Parsing Aruba AOS-CX config: %s\n", filepath.Base(filePath))

	// Example integration with actual parser:
	// deviceID := extractDeviceIDFromPath(filePath)
	// l2Config, err := aoscx.ParseAOSCXConfig(content, deviceID)
	// if err != nil {
	//     return fmt.Errorf("failed to parse Aruba config: %w", err)
	// }
	// mapper := aoscx.MapArubaToL2Switch(l2Config)
	// Save to store/database

	return nil
}

// parseJuniperJunos parses Juniper Junos configuration
func (pb *ParserBridge) parseJuniperJunos(ctx context.Context, content string, filePath string) error {
	fmt.Printf("Parsing Juniper Junos config: %s\n", filepath.Base(filePath))

	// Example integration with actual parser:
	// deviceID := extractDeviceIDFromPath(filePath)
	// l2Config, err := junos.ParseJunosConfig(content, deviceID)
	// if err != nil {
	//     return fmt.Errorf("failed to parse Junos config: %w", err)
	// }
	// mapper := junos.MapJuniperToL2Switch(l2Config)
	// Save to store/database

	return nil
}

// parsePaloAltoPANOS parses Palo Alto PAN-OS configuration
func (pb *ParserBridge) parsePaloAltoPANOS(ctx context.Context, content string, filePath string) error {
	fmt.Printf("Parsing Palo Alto PAN-OS config: %s\n", filepath.Base(filePath))
	return nil
}

// parseFortinetFortiGate parses Fortinet FortiGate configuration
func (pb *ParserBridge) parseFortinetFortiGate(ctx context.Context, content string, filePath string) error {
	fmt.Printf("Parsing Fortinet FortiGate config: %s\n", filepath.Base(filePath))
	return nil
}

// ParseFiles parses a list of files
func (pb *ParserBridge) ParseFiles(ctx context.Context, files []string) error {
	for _, file := range files {
		if err := pb.parseFile(ctx, file); err != nil {
			fmt.Printf("Warning: failed to parse %s: %v\n", file, err)
			// Continue processing other files rather than failing completely
			continue
		}
	}
	return nil
}

// extractDeviceIDFromPath extracts device ID from file path
// Helper function for parser integrations
func extractDeviceIDFromPath(filePath string) string {
	// Extract device ID from path like "/data/cisco/device123/running_config.txt"
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	for i, part := range parts {
		if i > 0 && (parts[i-1] == "cisco" || parts[i-1] == "aruba" || parts[i-1] == "juniper" ||
			parts[i-1] == "paloalto" || parts[i-1] == "fortinet") {
			return part
		}
	}
	return "unknown"
}
