package solarwinds

import (
	"fmt"
	"strings"
)

// ConfigProcessor processes configurations imported from SolarWinds NCM
type ConfigProcessor struct {
	// Parser registry could be injected here for actual parsing
}

// VendorType represents the detected vendor type
type VendorType string

const (
	VendorCiscoIOS      VendorType = "cisco_ios"
	VendorCiscoASA      VendorType = "cisco_asa"
	VendorCiscoNXOS     VendorType = "cisco_nxos"
	VendorPaloAlto      VendorType = "palo_alto"
	VendorFortinet      VendorType = "fortinet"
	VendorJuniper       VendorType = "juniper"
	VendorArista        VendorType = "arista"
	VendorAruba         VendorType = "aruba"
	VendorCheckPoint    VendorType = "checkpoint"
	VendorF5            VendorType = "f5"
	VendorUnknown       VendorType = "unknown"
)

// ProcessedConfig represents a processed configuration ready for parsing
type ProcessedConfig struct {
	NodeID       int
	NodeName     string
	NodeIP       string
	Vendor       VendorType
	ConfigText   string
	ConfigType   string
	DetectedOS   string
	Confidence   float64    // 0.0 to 1.0 confidence in vendor detection
}

// NewConfigProcessor creates a new configuration processor
func NewConfigProcessor() *ConfigProcessor {
	return &ConfigProcessor{}
}

// ProcessConfig processes a configuration from NCM and detects the vendor
func (cp *ConfigProcessor) ProcessConfig(config NodeConfigData) (*ProcessedConfig, error) {
	// Detect vendor from config content
	vendor, detectedOS, confidence := cp.DetectVendor(config.ConfigText, config.NodeVendor)

	processed := &ProcessedConfig{
		NodeID:     config.NodeID,
		NodeName:   config.NodeName,
		NodeIP:     config.NodeIPAddress,
		Vendor:     vendor,
		ConfigText: config.ConfigText,
		ConfigType: config.ConfigType,
		DetectedOS: detectedOS,
		Confidence: confidence,
	}

	return processed, nil
}

// DetectVendor detects the vendor and OS from configuration text
// Uses both NCM vendor metadata and config content analysis
func (cp *ConfigProcessor) DetectVendor(configText string, ncmVendor string) (VendorType, string, float64) {
	configLower := strings.ToLower(configText)
	ncmVendorLower := strings.ToLower(ncmVendor)

	// Extract first 2000 characters for banner/header detection
	header := configText
	if len(header) > 2000 {
		header = header[:2000]
	}
	headerLower := strings.ToLower(header)

	// Cisco IOS Detection
	if cp.isCiscoIOS(headerLower, configLower, ncmVendorLower) {
		osVersion := cp.extractCiscoIOSVersion(header)
		return VendorCiscoIOS, osVersion, 0.95
	}

	// Cisco ASA Detection
	if cp.isCiscoASA(headerLower, configLower, ncmVendorLower) {
		osVersion := cp.extractCiscoASAVersion(header)
		return VendorCiscoASA, osVersion, 0.95
	}

	// Cisco NXOS Detection
	if cp.isCiscoNXOS(headerLower, configLower, ncmVendorLower) {
		osVersion := cp.extractCiscoNXOSVersion(header)
		return VendorCiscoNXOS, osVersion, 0.95
	}

	// Palo Alto PAN-OS Detection
	if cp.isPaloAlto(headerLower, configLower, ncmVendorLower) {
		return VendorPaloAlto, "pan-os", 0.90
	}

	// Fortinet FortiOS Detection
	if cp.isFortinet(headerLower, configLower, ncmVendorLower) {
		return VendorFortinet, "fortios", 0.90
	}

	// Juniper Junos Detection
	if cp.isJuniper(headerLower, configLower, ncmVendorLower) {
		return VendorJuniper, "junos", 0.90
	}

	// Arista EOS Detection
	if cp.isArista(headerLower, configLower, ncmVendorLower) {
		return VendorArista, "eos", 0.90
	}

	// Aruba AOS Detection
	if cp.isAruba(headerLower, configLower, ncmVendorLower) {
		return VendorAruba, "aos", 0.90
	}

	// Check Point Gaia Detection
	if cp.isCheckPoint(headerLower, configLower, ncmVendorLower) {
		return VendorCheckPoint, "gaia", 0.85
	}

	// F5 TMOS Detection
	if cp.isF5(headerLower, configLower, ncmVendorLower) {
		return VendorF5, "tmos", 0.85
	}

	// Fallback to NCM vendor if available
	if ncmVendorLower != "" {
		if strings.Contains(ncmVendorLower, "cisco") {
			return VendorCiscoIOS, "unknown", 0.50
		}
		if strings.Contains(ncmVendorLower, "palo") || strings.Contains(ncmVendorLower, "alto") {
			return VendorPaloAlto, "unknown", 0.50
		}
		if strings.Contains(ncmVendorLower, "fortinet") || strings.Contains(ncmVendorLower, "forti") {
			return VendorFortinet, "unknown", 0.50
		}
	}

	return VendorUnknown, "unknown", 0.0
}

// ========================================
// Vendor Detection Functions
// ========================================

func (cp *ConfigProcessor) isCiscoIOS(header, config, ncmVendor string) bool {
	// Check for IOS-specific patterns
	iosIndicators := []string{
		"cisco ios",
		"ios software",
		"ios-xe",
		"c2960", "c3560", "c3750", "c3850", "c9300", "c9500", // Catalyst models
		"isr4", "asr1", "asr9", // Router models
	}

	for _, indicator := range iosIndicators {
		if strings.Contains(header, indicator) {
			return true
		}
	}

	// Check for IOS-specific commands in config
	iosCommands := []string{
		"version 12.", "version 15.", "version 16.", "version 17.",
		"interface gigabitethernet",
		"interface fastethernet",
		"ip routing",
		"spanning-tree mode",
	}

	matchCount := 0
	for _, cmd := range iosCommands {
		if strings.Contains(config, cmd) {
			matchCount++
		}
	}

	return matchCount >= 2
}

func (cp *ConfigProcessor) isCiscoASA(header, config, ncmVendor string) bool {
	// ASA-specific indicators
	asaIndicators := []string{
		"cisco adaptive security appliance",
		"asa version",
		"asav", "asa5", "asa 5", // ASA models
	}

	for _, indicator := range asaIndicators {
		if strings.Contains(header, indicator) {
			return true
		}
	}

	// ASA-specific commands
	asaCommands := []string{
		"nameif",
		"security-level",
		"access-list extended",
		"object network",
		"nat (", // NAT syntax unique to ASA
		"interface management",
	}

	matchCount := 0
	for _, cmd := range asaCommands {
		if strings.Contains(config, cmd) {
			matchCount++
		}
	}

	return matchCount >= 3
}

func (cp *ConfigProcessor) isCiscoNXOS(header, config, ncmVendor string) bool {
	// NXOS-specific indicators
	nxosIndicators := []string{
		"nx-os",
		"nxos",
		"nexus",
		"n9k", "n7k", "n5k", "n3k", "n2k", // Nexus models
	}

	for _, indicator := range nxosIndicators {
		if strings.Contains(header, indicator) {
			return true
		}
	}

	// NXOS-specific commands
	nxosCommands := []string{
		"feature vpc",
		"feature lacp",
		"feature interface-vlan",
		"vdc",
		"system default switchport",
		"fabricpath domain",
	}

	matchCount := 0
	for _, cmd := range nxosCommands {
		if strings.Contains(config, cmd) {
			matchCount++
		}
	}

	return matchCount >= 2
}

func (cp *ConfigProcessor) isPaloAlto(header, config, ncmVendor string) bool {
	// Palo Alto indicators
	indicators := []string{
		"palo alto",
		"pan-os",
		"pa-220", "pa-850", "pa-3220", "pa-5220", // PAN-OS models
		"<config", "<entry", // XML-based config
	}

	for _, indicator := range indicators {
		if strings.Contains(header, indicator) {
			return true
		}
	}

	// XML config structure
	if strings.Contains(config, "<devices>") && strings.Contains(config, "<vsys>") {
		return true
	}

	return false
}

func (cp *ConfigProcessor) isFortinet(header, config, ncmVendor string) bool {
	// Fortinet indicators
	indicators := []string{
		"fortinet",
		"fortigate",
		"fortios",
		"fg-", "fgt-", // FortiGate model prefixes
	}

	for _, indicator := range indicators {
		if strings.Contains(header, indicator) {
			return true
		}
	}

	// FortiOS-specific commands
	fortiCommands := []string{
		"config system global",
		"config firewall policy",
		"config system interface",
		"set vdom",
		"end", // FortiOS uses "end" to close config blocks
	}

	matchCount := 0
	for _, cmd := range fortiCommands {
		if strings.Contains(config, cmd) {
			matchCount++
		}
	}

	return matchCount >= 3
}

func (cp *ConfigProcessor) isJuniper(header, config, ncmVendor string) bool {
	// Juniper indicators
	indicators := []string{
		"juniper",
		"junos",
		"ex4", "qfx", "mx", "srx", // Juniper models
	}

	for _, indicator := range indicators {
		if strings.Contains(header, indicator) {
			return true
		}
	}

	// Junos-specific syntax
	junosPatterns := []string{
		"set system",
		"set interfaces",
		"set routing-options",
		"set security",
		"set protocols",
	}

	matchCount := 0
	for _, pattern := range junosPatterns {
		if strings.Contains(config, pattern) {
			matchCount++
		}
	}

	return matchCount >= 3
}

func (cp *ConfigProcessor) isArista(header, config, ncmVendor string) bool {
	// Arista indicators
	indicators := []string{
		"arista",
		"eos",
		"dcs-", // Arista model prefix
	}

	for _, indicator := range indicators {
		if strings.Contains(header, indicator) {
			return true
		}
	}

	// EOS-specific commands (similar to IOS but with EOS-specific features)
	eosCommands := []string{
		"transceiver qsfp",
		"vxlan",
		"router bgp", // While not unique, combined with others
		"interface ethernet",
		"mlag configuration",
	}

	matchCount := 0
	for _, cmd := range eosCommands {
		if strings.Contains(config, cmd) {
			matchCount++
		}
	}

	return matchCount >= 2
}

func (cp *ConfigProcessor) isAruba(header, config, ncmVendor string) bool {
	// Aruba indicators
	indicators := []string{
		"aruba",
		"arubaos",
		"procurve",
		"hp-aruba",
	}

	for _, indicator := range indicators {
		if strings.Contains(header, indicator) {
			return true
		}
	}

	return false
}

func (cp *ConfigProcessor) isCheckPoint(header, config, ncmVendor string) bool {
	// Check Point indicators
	indicators := []string{
		"check point",
		"checkpoint",
		"gaia",
		"secureplatform",
	}

	for _, indicator := range indicators {
		if strings.Contains(header, indicator) {
			return true
		}
	}

	return false
}

func (cp *ConfigProcessor) isF5(header, config, ncmVendor string) bool {
	// F5 indicators
	indicators := []string{
		"f5 networks",
		"big-ip",
		"tmos",
		"ltm",
	}

	for _, indicator := range indicators {
		if strings.Contains(header, indicator) {
			return true
		}
	}

	// F5 TMSH commands
	f5Commands := []string{
		"ltm pool",
		"ltm virtual",
		"ltm node",
		"net self",
		"net vlan",
	}

	matchCount := 0
	for _, cmd := range f5Commands {
		if strings.Contains(config, cmd) {
			matchCount++
		}
	}

	return matchCount >= 2
}

// ========================================
// Version Extraction Functions
// ========================================

func (cp *ConfigProcessor) extractCiscoIOSVersion(header string) string {
	// Extract version from banner
	lines := strings.Split(header, "\n")
	for _, line := range lines {
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, "ios") && strings.Contains(lineLower, "version") {
			// Try to extract version number
			if strings.Contains(line, "Version ") {
				parts := strings.Split(line, "Version ")
				if len(parts) > 1 {
					versionPart := strings.TrimSpace(parts[1])
					versionFields := strings.Fields(versionPart)
					if len(versionFields) > 0 {
						return "IOS " + versionFields[0]
					}
				}
			}
		}
	}
	return "IOS (version unknown)"
}

func (cp *ConfigProcessor) extractCiscoASAVersion(header string) string {
	lines := strings.Split(header, "\n")
	for _, line := range lines {
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, "asa version") {
			parts := strings.Split(line, "ASA Version ")
			if len(parts) > 1 {
				version := strings.TrimSpace(parts[1])
				return "ASA " + strings.Fields(version)[0]
			}
		}
	}
	return "ASA (version unknown)"
}

func (cp *ConfigProcessor) extractCiscoNXOSVersion(header string) string {
	lines := strings.Split(header, "\n")
	for _, line := range lines {
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, "nxos") || strings.Contains(lineLower, "nx-os") {
			if strings.Contains(line, "version") {
				parts := strings.Fields(line)
				for i, part := range parts {
					if strings.ToLower(part) == "version" && i+1 < len(parts) {
						return "NXOS " + parts[i+1]
					}
				}
			}
		}
	}
	return "NXOS (version unknown)"
}
