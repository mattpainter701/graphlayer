package parser

import (
	"encoding/xml"
	"fmt"
)

// ========================================
// PANORAMA DATA STRUCTURES
// ========================================

// PanoramaDeviceGroupConfig represents the XML response for device groups
type PanoramaDeviceGroupConfig struct {
	XMLName xml.Name                 `xml:"response"`
	Status  string                   `xml:"status,attr"`
	Result  PanoramaDeviceGroupResult `xml:"result"`
}

type PanoramaDeviceGroupResult struct {
	DeviceGroup struct {
		Entry []PanoramaDeviceGroup `xml:"entry"`
	} `xml:"device-group"`
}

// PanoramaDeviceGroup represents a Panorama device group for policy inheritance
type PanoramaDeviceGroup struct {
	Name        string                         `xml:"name,attr"`
	Description string                         `xml:"description"`
	Devices     []PanoramaDeviceGroupDevice    `xml:"devices>entry"`
	ParentGroup string                         `xml:"parent-dg"`
	Tags        []string                       `xml:"tag>member"`
}

type PanoramaDeviceGroupDevice struct {
	Name string `xml:"name,attr"`
}

// PanoramaTemplateConfig represents the XML response for templates
type PanoramaTemplateConfig struct {
	XMLName xml.Name               `xml:"response"`
	Status  string                 `xml:"status,attr"`
	Result  PanoramaTemplateResult `xml:"result"`
}

type PanoramaTemplateResult struct {
	Template struct {
		Entry []PanoramaTemplate `xml:"entry"`
	} `xml:"template"`
}

// PanoramaTemplate represents a configuration template
type PanoramaTemplate struct {
	Name        string   `xml:"name,attr"`
	Description string   `xml:"description"`
	Config      struct{} `xml:"config"` // Configuration content (not parsed in detail)
	Settings    struct {
		DefaultVsys string `xml:"default-vsys"`
	} `xml:"settings"`
}

// PanoramaManagedDevicesConfig represents the XML response for managed devices
type PanoramaManagedDevicesConfig struct {
	XMLName xml.Name                    `xml:"response"`
	Status  string                      `xml:"status,attr"`
	Result  PanoramaManagedDevicesResult `xml:"result"`
}

type PanoramaManagedDevicesResult struct {
	Devices struct {
		Entry []PanoramaManagedDevice `xml:"entry"`
	} `xml:"devices>entry"` // Note: nested entry structure
}

// PanoramaManagedDevice represents a firewall managed by Panorama
type PanoramaManagedDevice struct {
	Name         string `xml:"name,attr"`
	SerialNumber string `xml:"serial"`
	IPAddress    string `xml:"ip-address"`
	Model        string `xml:"model"`
	SWVersion    string `xml:"sw-version"`
	Connected    string `xml:"connected"` // "yes" or "no"
	Unsupported  string `xml:"unsupported-version"` // "yes" or "no"
	DeviceGroup  string `xml:"device-group"`
	Template     string `xml:"template"`
	Hostname     string `xml:"hostname"`
}

// PanoramaConnectedDevicesConfig represents the XML response for connected devices (operational)
type PanoramaConnectedDevicesConfig struct {
	XMLName xml.Name                       `xml:"response"`
	Status  string                         `xml:"status,attr"`
	Result  PanoramaConnectedDevicesResult `xml:"result"`
}

type PanoramaConnectedDevicesResult struct {
	Devices struct {
		Entry []PanoramaConnectedDevice `xml:"entry"`
	} `xml:"devices>entry"`
}

// PanoramaConnectedDevice represents operational status of connected devices
type PanoramaConnectedDevice struct {
	Hostname         string `xml:"hostname"`
	SerialNumber     string `xml:"serial"`
	IPAddress        string `xml:"ip-address"`
	Model            string `xml:"model"`
	SWVersion        string `xml:"sw-version"`
	Connected        string `xml:"connected"` // "yes" or "no"
	LastConnected    string `xml:"last-reporttime"`
	ConfigStatus     string `xml:"configstatus"` // "insync", "outofsync"
	HAState          string `xml:"ha-state"` // "active", "passive", "standalone"
	MultiVsys        string `xml:"multi-vsys"` // "on" or "off"
	VsysCount        string `xml:"vsys-count"`
}

// PanoramaDeviceHealthConfig represents the XML response for device health (operational)
type PanoramaDeviceHealthConfig struct {
	XMLName xml.Name                   `xml:"response"`
	Status  string                     `xml:"status,attr"`
	Result  PanoramaDeviceHealthResult `xml:"result"`
}

type PanoramaDeviceHealthResult struct {
	Devices struct {
		Entry []PanoramaDeviceHealth `xml:"entry"`
	} `xml:"devices>entry"`
}

// PanoramaDeviceHealth represents detailed health status of managed devices
type PanoramaDeviceHealth struct {
	Name             string `xml:"name,attr"`
	SerialNumber     string `xml:"serial"`
	IPAddress        string `xml:"ip-address"`
	Model            string `xml:"model"`
	SWVersion        string `xml:"sw-version"`
	Connected        string `xml:"connected"`
	ConfigStatus     string `xml:"configstatus"`
	HAState          string `xml:"ha>state"`
	Uptime           string `xml:"uptime"`
	CPUUsage         string `xml:"cpu-usage"`
	MemoryUsage      string `xml:"memory-usage"`
	SessionCount     string `xml:"session-utilization"`
}

// ========================================
// PARSER FUNCTIONS
// ========================================

// ParsePanoramaDeviceGroups parses device groups from Panorama XML
func ParsePanoramaDeviceGroups(xmlData string) ([]PanoramaDeviceGroup, error) {
	var config PanoramaDeviceGroupConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Panorama device group XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("Panorama API returned non-success status: %s", config.Status)
	}

	return config.Result.DeviceGroup.Entry, nil
}

// ParsePanoramaTemplates parses configuration templates from Panorama XML
func ParsePanoramaTemplates(xmlData string) ([]PanoramaTemplate, error) {
	var config PanoramaTemplateConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Panorama template XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("Panorama API returned non-success status: %s", config.Status)
	}

	return config.Result.Template.Entry, nil
}

// ParsePanoramaManagedDevices parses managed devices from Panorama XML (configuration)
func ParsePanoramaManagedDevices(xmlData string) ([]PanoramaManagedDevice, error) {
	var config PanoramaManagedDevicesConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Panorama managed devices XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("Panorama API returned non-success status: %s", config.Status)
	}

	return config.Result.Devices.Entry, nil
}

// ParsePanoramaConnectedDevices parses connected devices from Panorama XML (operational)
func ParsePanoramaConnectedDevices(xmlData string) ([]PanoramaConnectedDevice, error) {
	var config PanoramaConnectedDevicesConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Panorama connected devices XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("Panorama API returned non-success status: %s", config.Status)
	}

	return config.Result.Devices.Entry, nil
}

// ParsePanoramaSharedAddresses parses shared address objects from Panorama
// This function reuses ParsePANOSAddressObjects from palo_alto.parsers.go
// Shared objects use the same XML structure as vsys-specific objects
func ParsePanoramaSharedAddresses(xmlData string) ([]PANOSAddressObject, []PANOSAddressGroup, error) {
	// Reuse existing PAN-OS parser since XML structure is identical
	return ParsePANOSAddressObjects(xmlData)
}

// ParsePanoramaSharedServices parses shared service objects from Panorama
// This function reuses ParsePANOSServiceObjects from palo_alto.parsers.go
// Shared objects use the same XML structure as vsys-specific objects
func ParsePanoramaSharedServices(xmlData string) ([]PANOSServiceObject, []PANOSServiceGroup, error) {
	// Reuse existing PAN-OS parser since XML structure is identical
	return ParsePANOSServiceObjects(xmlData)
}

// ParsePanoramaDeviceGroupPolicies parses security policies from device group pre-rules
// This function reuses ParsePANOSSecurityPolicies from palo_alto.parsers.go
// Device group policies use the same XML structure as vsys policies
func ParsePanoramaDeviceGroupPolicies(xmlData string) ([]PANOSSecurityPolicy, error) {
	// Reuse existing PAN-OS parser since XML structure is identical
	return ParsePANOSSecurityPolicies(xmlData)
}

// ParsePanoramaDeviceHealth parses device health status from Panorama XML (operational)
func ParsePanoramaDeviceHealth(xmlData string) ([]PanoramaDeviceHealth, error) {
	var config PanoramaDeviceHealthConfig
	err := xml.Unmarshal([]byte(xmlData), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Panorama device health XML: %w", err)
	}

	if config.Status != "success" {
		return nil, fmt.Errorf("Panorama API returned non-success status: %s", config.Status)
	}

	return config.Result.Devices.Entry, nil
}
