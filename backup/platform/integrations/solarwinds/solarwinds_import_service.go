package solarwinds

import (
	"fmt"
	"sort"
	"time"
)

// ImportService provides high-level config import functionality
type ImportService struct {
	NCMClient        *NCMClient
	ConfigProcessor  *ConfigProcessor
}

// ImportRequest represents a request to import configurations
type ImportRequest struct {
	Vendors      []string  // Filter by vendors (e.g., "Cisco", "Palo Alto")
	NodeNames    []string  // Filter by node names
	ConfigType   string    // "Running", "Startup", or empty for both
	SinceTime    *time.Time
	Limit        int
}

// ImportSummary provides a summary of the import operation
type ImportSummary struct {
	TotalConfigs     int
	SuccessfulImports int
	FailedImports    int
	VendorBreakdown  map[VendorType]int
	ProcessedConfigs []ProcessedConfig
	Errors           []ImportError
}

// ImportError represents an error during import
type ImportError struct {
	NodeID    int
	NodeName  string
	Error     string
	Timestamp time.Time
}

// NewImportService creates a new import service
func NewImportService(ncmConfig NCMConfig) (*ImportService, error) {
	client := NewNCMClient(ncmConfig)

	// Test connection
	if err := client.TestConnection(); err != nil {
		return nil, fmt.Errorf("failed to connect to SolarWinds NCM: %w", err)
	}

	return &ImportService{
		NCMClient:       client,
		ConfigProcessor: NewConfigProcessor(),
	}, nil
}

// ImportConfigs imports configurations from NCM based on the request criteria
func (is *ImportService) ImportConfigs(request ImportRequest) (*ImportSummary, error) {
	// Build filter from request
	filter := ConfigFilter{
		Vendors:    request.Vendors,
		NodeNames:  request.NodeNames,
		ConfigType: request.ConfigType,
		SinceTime:  request.SinceTime,
		Limit:      request.Limit,
	}

	// Query configs from NCM
	configs, err := is.NCMClient.QueryConfigs(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query configurations: %w", err)
	}

	// Initialize summary
	summary := &ImportSummary{
		TotalConfigs:    len(configs),
		VendorBreakdown: make(map[VendorType]int),
		ProcessedConfigs: make([]ProcessedConfig, 0, len(configs)),
		Errors:          make([]ImportError, 0),
	}

	// Process each config
	for _, config := range configs {
		processed, err := is.ConfigProcessor.ProcessConfig(config)
		if err != nil {
			summary.FailedImports++
			summary.Errors = append(summary.Errors, ImportError{
				NodeID:    config.NodeID,
				NodeName:  config.NodeName,
				Error:     err.Error(),
				Timestamp: time.Now(),
			})
			continue
		}

		// Only include configs with sufficient confidence
		if processed.Confidence < 0.5 {
			summary.FailedImports++
			summary.Errors = append(summary.Errors, ImportError{
				NodeID:    config.NodeID,
				NodeName:  config.NodeName,
				Error:     fmt.Sprintf("Low confidence vendor detection (%.2f)", processed.Confidence),
				Timestamp: time.Now(),
			})
			continue
		}

		summary.SuccessfulImports++
		summary.VendorBreakdown[processed.Vendor]++
		summary.ProcessedConfigs = append(summary.ProcessedConfigs, *processed)
	}

	return summary, nil
}

// ListAvailableConfigs lists available configurations without importing them
// Useful for user selection workflow
func (is *ImportService) ListAvailableConfigs(request ImportRequest) ([]ConfigSummary, error) {
	// Build filter
	filter := ConfigFilter{
		Vendors:    request.Vendors,
		NodeNames:  request.NodeNames,
		ConfigType: request.ConfigType,
		SinceTime:  request.SinceTime,
		Limit:      request.Limit,
	}

	// Query configs
	configs, err := is.NCMClient.QueryConfigs(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query configurations: %w", err)
	}

	// Create summaries
	summaries := make([]ConfigSummary, 0, len(configs))
	for _, config := range configs {
		// Quick vendor detection
		vendor, osVersion, confidence := is.ConfigProcessor.DetectVendor(config.ConfigText, config.NodeVendor)

		summary := ConfigSummary{
			NodeID:        config.NodeID,
			NodeName:      config.NodeName,
			NodeIP:        config.NodeIPAddress,
			Vendor:        vendor,
			OSVersion:     osVersion,
			ConfigType:    config.ConfigType,
			ConfigSize:    config.ConfigSize,
			DownloadTime:  config.DownloadTime,
			Confidence:    confidence,
			Selected:      false, // Default not selected
		}

		summaries = append(summaries, summary)
	}

	// Sort by download time (most recent first)
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].DownloadTime.After(summaries[j].DownloadTime)
	})

	return summaries, nil
}

// ConfigSummary provides a summary of a configuration for user selection
type ConfigSummary struct {
	NodeID       int
	NodeName     string
	NodeIP       string
	Vendor       VendorType
	OSVersion    string
	ConfigType   string
	ConfigSize   int
	DownloadTime time.Time
	Confidence   float64
	Selected     bool
}

// ImportSelectedConfigs imports only the selected configurations
func (is *ImportService) ImportSelectedConfigs(nodeIDs []int) (*ImportSummary, error) {
	// Query only the selected configs
	filter := ConfigFilter{
		NodeIDs: nodeIDs,
	}

	configs, err := is.NCMClient.QueryConfigs(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query selected configurations: %w", err)
	}

	// Initialize summary
	summary := &ImportSummary{
		TotalConfigs:     len(configs),
		VendorBreakdown:  make(map[VendorType]int),
		ProcessedConfigs: make([]ProcessedConfig, 0, len(configs)),
		Errors:           make([]ImportError, 0),
	}

	// Process each config
	for _, config := range configs {
		processed, err := is.ConfigProcessor.ProcessConfig(config)
		if err != nil {
			summary.FailedImports++
			summary.Errors = append(summary.Errors, ImportError{
				NodeID:    config.NodeID,
				NodeName:  config.NodeName,
				Error:     err.Error(),
				Timestamp: time.Now(),
			})
			continue
		}

		summary.SuccessfulImports++
		summary.VendorBreakdown[processed.Vendor]++
		summary.ProcessedConfigs = append(summary.ProcessedConfigs, *processed)
	}

	return summary, nil
}

// GetVendorStats returns statistics about available configurations by vendor
func (is *ImportService) GetVendorStats() (map[VendorType]int, error) {
	// Query all recent configs (last 30 days)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	filter := ConfigFilter{
		SinceTime: &thirtyDaysAgo,
		Limit:     1000, // Reasonable limit
	}

	configs, err := is.NCMClient.QueryConfigs(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query configurations: %w", err)
	}

	// Count by vendor
	stats := make(map[VendorType]int)
	for _, config := range configs {
		vendor, _, confidence := is.ConfigProcessor.DetectVendor(config.ConfigText, config.NodeVendor)
		if confidence >= 0.5 {
			stats[vendor]++
		}
	}

	return stats, nil
}

// FormatSummary returns a human-readable summary string
func (summary *ImportSummary) FormatSummary() string {
	result := fmt.Sprintf("Import Summary:\n")
	result += fmt.Sprintf("  Total Configs: %d\n", summary.TotalConfigs)
	result += fmt.Sprintf("  Successful: %d\n", summary.SuccessfulImports)
	result += fmt.Sprintf("  Failed: %d\n", summary.FailedImports)
	result += fmt.Sprintf("\nVendor Breakdown:\n")

	// Sort vendors for consistent output
	vendors := make([]VendorType, 0, len(summary.VendorBreakdown))
	for vendor := range summary.VendorBreakdown {
		vendors = append(vendors, vendor)
	}
	sort.Slice(vendors, func(i, j int) bool {
		return string(vendors[i]) < string(vendors[j])
	})

	for _, vendor := range vendors {
		count := summary.VendorBreakdown[vendor]
		result += fmt.Sprintf("  %s: %d\n", vendor, count)
	}

	if len(summary.Errors) > 0 {
		result += fmt.Sprintf("\nErrors:\n")
		for i, err := range summary.Errors {
			if i >= 5 { // Limit to first 5 errors
				result += fmt.Sprintf("  ... and %d more errors\n", len(summary.Errors)-5)
				break
			}
			result += fmt.Sprintf("  [Node %d - %s] %s\n", err.NodeID, err.NodeName, err.Error)
		}
	}

	return result
}
