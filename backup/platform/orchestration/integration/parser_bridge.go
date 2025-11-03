package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/orchestrator"
)

// ParserBridge connects connectors to parsers
// This provides the integration point between the connector layer and your existing parsers
type ParserBridge struct {
	orchestrator *orchestrator.ConnectorOrchestrator
	// These would be your actual parser interfaces from your existing code
	// store           docinterfaces.IParserStore
	// messageBroker   connectorInterfaces.IMessageBroker
	// logger          *zerolog.Logger
}

// NewParserBridge creates a new parser bridge
func NewParserBridge(orch *orchestrator.ConnectorOrchestrator) *ParserBridge {
	return &ParserBridge{
		orchestrator: orch,
		// store:         store,
		// messageBroker: broker,
		// logger:        logger,
	}
}

// ParseCollectionResult processes collected files with appropriate parser
func (pb *ParserBridge) ParseCollectionResult(ctx context.Context, result *orchestrator.CollectionResult) error {
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

// parseFile routes file to appropriate parser based on device type
func (pb *ParserBridge) parseFile(ctx context.Context, filePath string) error {
	// Determine parser type from filename/path
	filename := filepath.Base(filePath)
	
	// Route to appropriate parser based on file characteristics
	if strings.Contains(filename, "running_config") || strings.Contains(filename, "startup_config") {
		return pb.parseConfigFile(ctx, filePath)
	}
	
	// Handle auxiliary data files
	if strings.Contains(filename, "show_") {
		return pb.parseOperationalData(ctx, filePath)
	}
	
	return fmt.Errorf("unknown file type: %s", filename)
}

// parseConfigFile parses configuration files
func (pb *ParserBridge) parseConfigFile(ctx context.Context, filePath string) error {
	// This is where you would call your existing Cisco IOS processor
	// Example integration:
	//
	// processor := cisco.NewCiscoIOSProcessor(
	//     filePath,
	//     pb.store,
	//     pb.messageBroker,
	//     pb.logger,
	//     pb.messageHelper,
	// )
	//
	// return processor.ProcessIOSFile()
	
	fmt.Printf("Would parse config file: %s\n", filePath)
	return nil
}

// parseOperationalData parses operational/diagnostic output
func (pb *ParserBridge) parseOperationalData(ctx context.Context, filePath string) error {
	// Parse auxiliary command outputs (ARP tables, MAC tables, etc.)
	fmt.Printf("Would parse operational data: %s\n", filePath)
	return nil
}

// ParseFiles parses a list of files
func (pb *ParserBridge) ParseFiles(ctx context.Context, files []string) error {
	for _, file := range files {
		if err := pb.parseFile(ctx, file); err != nil {
			return fmt.Errorf("failed to parse %s: %w", file, err)
		}
	}
	return nil
}

