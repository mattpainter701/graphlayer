package fortimanager

import (
	"fmt"
	"time"
)

// ImportService provides high-level FortiManager import functionality
type ImportService struct {
	Client *FortiManagerClient
}

// ImportRequest represents a request to import configurations from FortiManager
type ImportRequest struct {
	ADOMs         []string // Filter by specific ADOMs (empty = all)
	DeviceNames   []string // Filter by device names
	IncludeGlobalObjects bool // Include ADOM-level global objects
	IncludePolicyPackages bool // Include policy packages
}

// ImportSummary provides a summary of the import operation
type ImportSummary struct {
	TotalADOMs       int
	TotalDevices     int
	ConnectedDevices int
	ImportedConfigs  int
	FailedImports    int
	ADOMDetails      []ADOMImportDetail
	Errors           []ImportError
}

// ADOMImportDetail provides details for a single ADOM import
type ADOMImportDetail struct {
	ADOMName         string
	DeviceCount      int
	PackageCount     int
	GlobalObjectCount map[string]int // object type -> count
}

// ImportError represents an error during import
type ImportError struct {
	ADOM      string
	Device    string
	Error     string
	Timestamp time.Time
}

// NewImportService creates a new FortiManager import service
func NewImportService(config FortiManagerConfig) (*ImportService, error) {
	client := NewFortiManagerClient(config)

	// Test connection and login
	if err := client.Login(); err != nil {
		return nil, fmt.Errorf("failed to login to FortiManager: %w", err)
	}

	return &ImportService{
		Client: client,
	}, nil
}

// Close logs out from FortiManager
func (is *ImportService) Close() error {
	return is.Client.Logout()
}

// ImportConfigurations imports configurations from FortiManager based on request criteria
func (is *ImportService) ImportConfigurations(request ImportRequest) (*ImportSummary, error) {
	summary := &ImportSummary{
		ADOMDetails: make([]ADOMImportDetail, 0),
		Errors:      make([]ImportError, 0),
	}

	// Get list of ADOMs
	adoms, err := is.Client.GetADOMs()
	if err != nil {
		return nil, fmt.Errorf("failed to get ADOMs: %w", err)
	}

	summary.TotalADOMs = len(adoms)

	// Filter ADOMs if specified
	if len(request.ADOMs) > 0 {
		adoms = filterADOMs(adoms, request.ADOMs)
	}

	// Process each ADOM
	for _, adom := range adoms {
		detail := ADOMImportDetail{
			ADOMName:          adom.Name,
			GlobalObjectCount: make(map[string]int),
		}

		// Get managed devices in this ADOM
		devices, err := is.Client.GetManagedDevices(adom.Name)
		if err != nil {
			summary.Errors = append(summary.Errors, ImportError{
				ADOM:      adom.Name,
				Error:     fmt.Sprintf("Failed to get devices: %v", err),
				Timestamp: time.Now(),
			})
			summary.FailedImports++
			continue
		}

		// Filter devices if specified
		if len(request.DeviceNames) > 0 {
			devices = filterDevices(devices, request.DeviceNames)
		}

		detail.DeviceCount = len(devices)
		summary.TotalDevices += len(devices)

		// Count connected devices
		for _, device := range devices {
			if device.Connected == 1 {
				summary.ConnectedDevices++
			}
		}

		// Import device configurations
		for _, device := range devices {
			// Only import from connected devices
			if device.Connected != 1 {
				summary.Errors = append(summary.Errors, ImportError{
					ADOM:      adom.Name,
					Device:    device.Name,
					Error:     "Device not connected",
					Timestamp: time.Now(),
				})
				summary.FailedImports++
				continue
			}

			// Get device configuration
			_, err := is.Client.GetDeviceConfig(adom.Name, device.Name)
			if err != nil {
				summary.Errors = append(summary.Errors, ImportError{
					ADOM:      adom.Name,
					Device:    device.Name,
					Error:     fmt.Sprintf("Failed to get config: %v", err),
					Timestamp: time.Now(),
				})
				summary.FailedImports++
				continue
			}

			summary.ImportedConfigs++
		}

		// Import global objects if requested
		if request.IncludeGlobalObjects {
			objectTypes := []string{
				"firewall/address",
				"firewall/addrgrp",
				"firewall/service/custom",
				"firewall/service/group",
			}

			for _, objType := range objectTypes {
				objects, err := is.Client.GetGlobalObjects(adom.Name, objType)
				if err != nil {
					summary.Errors = append(summary.Errors, ImportError{
						ADOM:      adom.Name,
						Error:     fmt.Sprintf("Failed to get %s: %v", objType, err),
						Timestamp: time.Now(),
					})
					continue
				}
				detail.GlobalObjectCount[objType] = len(objects)
			}
		}

		// Import policy packages if requested
		if request.IncludePolicyPackages {
			packages, err := is.Client.GetPolicyPackages(adom.Name)
			if err != nil {
				summary.Errors = append(summary.Errors, ImportError{
					ADOM:      adom.Name,
					Error:     fmt.Sprintf("Failed to get policy packages: %v", err),
					Timestamp: time.Now(),
				})
			} else {
				detail.PackageCount = len(packages)
			}
		}

		summary.ADOMDetails = append(summary.ADOMDetails, detail)
	}

	return summary, nil
}

// ListADOMs lists available ADOMs
func (is *ImportService) ListADOMs() ([]ADOMInfo, error) {
	return is.Client.GetADOMs()
}

// ListDevices lists managed devices for a specific ADOM
func (is *ImportService) ListDevices(adomName string) ([]ManagedDevice, error) {
	return is.Client.GetManagedDevices(adomName)
}

// GetDeviceConfiguration retrieves configuration for a specific device
func (is *ImportService) GetDeviceConfiguration(adomName, deviceName string) (map[string]interface{}, error) {
	return is.Client.GetDeviceConfig(adomName, deviceName)
}

// GetADOMStatistics returns statistics for an ADOM
func (is *ImportService) GetADOMStatistics(adomName string) (*ADOMStatistics, error) {
	stats := &ADOMStatistics{
		ADOMName: adomName,
	}

	// Get devices
	devices, err := is.Client.GetManagedDevices(adomName)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	stats.TotalDevices = len(devices)
	for _, device := range devices {
		if device.Connected == 1 {
			stats.ConnectedDevices++
		}
	}

	// Get policy packages
	packages, err := is.Client.GetPolicyPackages(adomName)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy packages: %w", err)
	}
	stats.PolicyPackages = len(packages)

	return stats, nil
}

// ADOMStatistics represents statistics for an ADOM
type ADOMStatistics struct {
	ADOMName          string
	TotalDevices      int
	ConnectedDevices  int
	PolicyPackages    int
}

// FormatSummary returns a human-readable summary string
func (summary *ImportSummary) FormatSummary() string {
	result := fmt.Sprintf("FortiManager Import Summary:\n")
	result += fmt.Sprintf("  Total ADOMs: %d\n", summary.TotalADOMs)
	result += fmt.Sprintf("  Total Devices: %d\n", summary.TotalDevices)
	result += fmt.Sprintf("  Connected Devices: %d\n", summary.ConnectedDevices)
	result += fmt.Sprintf("  Imported Configs: %d\n", summary.ImportedConfigs)
	result += fmt.Sprintf("  Failed Imports: %d\n", summary.FailedImports)
	result += fmt.Sprintf("\nADOM Details:\n")

	for _, detail := range summary.ADOMDetails {
		result += fmt.Sprintf("  %s:\n", detail.ADOMName)
		result += fmt.Sprintf("    Devices: %d\n", detail.DeviceCount)
		result += fmt.Sprintf("    Packages: %d\n", detail.PackageCount)
		if len(detail.GlobalObjectCount) > 0 {
			result += fmt.Sprintf("    Global Objects:\n")
			for objType, count := range detail.GlobalObjectCount {
				result += fmt.Sprintf("      %s: %d\n", objType, count)
			}
		}
	}

	if len(summary.Errors) > 0 {
		result += fmt.Sprintf("\nErrors:\n")
		for i, err := range summary.Errors {
			if i >= 5 { // Limit to first 5 errors
				result += fmt.Sprintf("  ... and %d more errors\n", len(summary.Errors)-5)
				break
			}
			if err.Device != "" {
				result += fmt.Sprintf("  [%s/%s] %s\n", err.ADOM, err.Device, err.Error)
			} else {
				result += fmt.Sprintf("  [%s] %s\n", err.ADOM, err.Error)
			}
		}
	}

	return result
}

// Helper functions

func filterADOMs(adoms []ADOMInfo, filter []string) []ADOMInfo {
	if len(filter) == 0 {
		return adoms
	}

	filtered := make([]ADOMInfo, 0)
	filterMap := make(map[string]bool)
	for _, name := range filter {
		filterMap[name] = true
	}

	for _, adom := range adoms {
		if filterMap[adom.Name] {
			filtered = append(filtered, adom)
		}
	}

	return filtered
}

func filterDevices(devices []ManagedDevice, filter []string) []ManagedDevice {
	if len(filter) == 0 {
		return devices
	}

	filtered := make([]ManagedDevice, 0)
	filterMap := make(map[string]bool)
	for _, name := range filter {
		filterMap[name] = true
	}

	for _, device := range devices {
		if filterMap[device.Name] {
			filtered = append(filtered, device)
		}
	}

	return filtered
}
