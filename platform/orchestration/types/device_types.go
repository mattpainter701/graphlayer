package types

import "time"

// DeviceType represents different types of network devices
type DeviceType string

const (
	// Cisco devices
	DeviceTypeCiscoIOS     DeviceType = "cisco_ios"
	DeviceTypeCiscoASA     DeviceType = "cisco_asa"
	DeviceTypeCiscoNXOS    DeviceType = "cisco_nxos"
	DeviceTypeCiscoFMC     DeviceType = "cisco_fmc"     // Firepower Management Center

	// Palo Alto devices
	DeviceTypePaloAltoPA   DeviceType = "palo_alto_pa"
	DeviceTypePaloAltoPanorama DeviceType = "palo_alto_panorama"

	// Juniper devices
	DeviceTypeJuniperJunos DeviceType = "juniper_junos"
	DeviceTypeJuniperSRX   DeviceType = "juniper_srx"

	// Fortinet devices
	DeviceTypeFortinetFortiGate DeviceType = "fortinet_fortigate"
	DeviceTypeFortinetFortiManager DeviceType = "fortinet_fortimanager"

	// Check Point devices
	DeviceTypeCheckPointGaia DeviceType = "checkpoint_gaia"

	// F5 devices
	DeviceTypeF5BigIP      DeviceType = "f5_bigip"

	// Management systems
	DeviceTypeSolarWindsNCM DeviceType = "solarwinds_ncm"

	// Generic types
	DeviceTypeGenericSSH   DeviceType = "generic_ssh"
	DeviceTypeGenericAPI   DeviceType = "generic_api"
)

// OSType represents different operating systems
type OSType string

const (
	OSTypeIOS      OSType = "ios"
	OSTypeASA      OSType = "asa"
	OSTypeNXOS     OSType = "nxos"
	OSTypePANOS    OSType = "panos"
	OSTypeJunos    OSType = "junos"
	OSTypeFortiOS  OSType = "fortios"
	OSTypeGaia     OSType = "gaia"
	OSTypeBigIP    OSType = "bigip"
	OSTypeWindows  OSType = "windows"
	OSTypeLinux    OSType = "linux"
)

// CommandCategory represents categories of commands
type CommandCategory string

const (
	CommandCategoryConfig      CommandCategory = "config"
	CommandCategoryOperational CommandCategory = "operational"
	CommandCategoryDiagnostic  CommandCategory = "diagnostic"
	CommandCategoryMonitoring  CommandCategory = "monitoring"
	CommandCategorySecurity    CommandCategory = "security"
	CommandCategoryMaintenance CommandCategory = "maintenance"
)

// CommandTemplate defines a command template with metadata
type CommandTemplate struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Category    CommandCategory  `json:"category"`
	Command     string           `json:"command"`
	Platforms   []DeviceType     `json:"platforms"`
	Timeout     time.Duration    `json:"timeout"`
	Prompts     []string         `json:"prompts,omitempty"` // Expected prompts for interactive commands
	ParseOutput bool             `json:"parse_output"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// ConfigType represents different types of configurations
type ConfigType string

const (
	ConfigTypeRunning   ConfigType = "running"
	ConfigTypeStartup   ConfigType = "startup"
	ConfigTypeCandidate ConfigType = "candidate"
	ConfigTypeBackup    ConfigType = "backup"
	ConfigTypeTemplate  ConfigType = "template"
)

// DeviceCapabilities represents what a device can do
type DeviceCapabilities struct {
	// Configuration capabilities
	CanGetRunningConfig bool `json:"can_get_running_config"`
	CanGetStartupConfig bool `json:"can_get_startup_config"`
	CanBackupConfig     bool `json:"can_backup_config"`

	// Command execution capabilities
	CanExecuteCommands  bool `json:"can_execute_commands"`
	CanExecuteConfigCommands bool `json:"can_execute_config_commands"`

	// Management capabilities
	CanManageDevices    bool `json:"can_manage_devices"`
	CanGetDeviceList    bool `json:"can_get_device_list"`
	CanPushConfig       bool `json:"can_push_config"`
	CanGetDeviceStatus  bool `json:"can_get_device_status"`

	// Monitoring capabilities
	CanMonitorDevices   bool `json:"can_monitor_devices"`
	CanGetLogs          bool `json:"can_get_logs"`
	CanGetMetrics       bool `json:"can_get_metrics"`

	// Security capabilities
	CanGetSecurityEvents bool `json:"can_get_security_events"`
	CanGetVulnerabilities bool `json:"can_get_vulnerabilities"`
}

// DefaultCommandTemplates provides commonly used command templates
var DefaultCommandTemplates = map[DeviceType][]CommandTemplate{

	// Cisco IOS commands
	DeviceTypeCiscoIOS: {
		{
			Name:        "show_running_config",
			Description: "Get running configuration",
			Category:    CommandCategoryConfig,
			Command:     "show running-config",
			Platforms:   []DeviceType{DeviceTypeCiscoIOS},
			Timeout:     30 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_startup_config",
			Description: "Get startup configuration",
			Category:    CommandCategoryConfig,
			Command:     "show startup-config",
			Platforms:   []DeviceType{DeviceTypeCiscoIOS},
			Timeout:     30 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_interfaces",
			Description: "Show interface status and configuration",
			Category:    CommandCategoryOperational,
			Command:     "show interfaces",
			Platforms:   []DeviceType{DeviceTypeCiscoIOS},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_ip_route",
			Description: "Show routing table",
			Category:    CommandCategoryOperational,
			Command:     "show ip route",
			Platforms:   []DeviceType{DeviceTypeCiscoIOS},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_vlan",
			Description: "Show VLAN configuration",
			Category:    CommandCategoryOperational,
			Command:     "show vlan",
			Platforms:   []DeviceType{DeviceTypeCiscoIOS},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_mac_address_table",
			Description: "Show MAC address table",
			Category:    CommandCategoryOperational,
			Command:     "show mac address-table",
			Platforms:   []DeviceType{DeviceTypeCiscoIOS},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_version",
			Description: "Show system version and hardware info",
			Category:    CommandCategoryDiagnostic,
			Command:     "show version",
			Platforms:   []DeviceType{DeviceTypeCiscoIOS},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
	},

	// Cisco ASA commands
	DeviceTypeCiscoASA: {
		{
			Name:        "show_running_config",
			Description: "Get running configuration",
			Category:    CommandCategoryConfig,
			Command:     "show running-config",
			Platforms:   []DeviceType{DeviceTypeCiscoASA},
			Timeout:     30 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_startup_config",
			Description: "Get startup configuration",
			Category:    CommandCategoryConfig,
			Command:     "show startup-config",
			Platforms:   []DeviceType{DeviceTypeCiscoASA},
			Timeout:     30 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_interfaces",
			Description: "Show interface status",
			Category:    CommandCategoryOperational,
			Command:     "show interfaces",
			Platforms:   []DeviceType{DeviceTypeCiscoASA},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_route",
			Description: "Show routing table",
			Category:    CommandCategoryOperational,
			Command:     "show route",
			Platforms:   []DeviceType{DeviceTypeCiscoASA},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_version",
			Description: "Show system version",
			Category:    CommandCategoryDiagnostic,
			Command:     "show version",
			Platforms:   []DeviceType{DeviceTypeCiscoASA},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
	},

	// Palo Alto PANOS commands
	DeviceTypePaloAltoPA: {
		{
			Name:        "show_running_config",
			Description: "Get running configuration",
			Category:    CommandCategoryConfig,
			Command:     "show config running",
			Platforms:   []DeviceType{DeviceTypePaloAltoPA},
			Timeout:     30 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_system_info",
			Description: "Show system information",
			Category:    CommandCategoryDiagnostic,
			Command:     "show system info",
			Platforms:   []DeviceType{DeviceTypePaloAltoPA},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_interfaces",
			Description: "Show interface information",
			Category:    CommandCategoryOperational,
			Command:     "show interface all",
			Platforms:   []DeviceType{DeviceTypePaloAltoPA},
			Timeout:     15 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_routing_route",
			Description: "Show routing table",
			Category:    CommandCategoryOperational,
			Command:     "show routing route",
			Platforms:   []DeviceType{DeviceTypePaloAltoPA},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
	},

	// Palo Alto Panorama commands
	DeviceTypePaloAltoPanorama: {
		{
			Name:        "show_panorama_status",
			Description: "Show Panorama status",
			Category:    CommandCategoryOperational,
			Command:     "show panorama-status",
			Platforms:   []DeviceType{DeviceTypePaloAltoPanorama},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_devicegroups",
			Description: "Show device groups",
			Category:    CommandCategoryOperational,
			Command:     "show devicegroups",
			Platforms:   []DeviceType{DeviceTypePaloAltoPanorama},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
	},

	// Generic SSH commands (for any SSH-capable device)
	DeviceTypeGenericSSH: {
		{
			Name:        "show_version",
			Description: "Show system version",
			Category:    CommandCategoryDiagnostic,
			Command:     "show version",
			Platforms:   []DeviceType{DeviceTypeGenericSSH},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
		{
			Name:        "show_interfaces",
			Description: "Show interface status",
			Category:    CommandCategoryOperational,
			Command:     "show interfaces",
			Platforms:   []DeviceType{DeviceTypeGenericSSH},
			Timeout:     10 * time.Second,
			ParseOutput: true,
		},
	},
}

// DefaultDeviceCapabilities provides capability mappings for device types
var DefaultDeviceCapabilities = map[DeviceType]DeviceCapabilities{

	// Direct device connections (SSH/API)
	DeviceTypeCiscoIOS: {
		CanGetRunningConfig:      true,
		CanGetStartupConfig:      true,
		CanBackupConfig:          true,
		CanExecuteCommands:       true,
		CanExecuteConfigCommands: true,
	},

	DeviceTypeCiscoASA: {
		CanGetRunningConfig:      true,
		CanGetStartupConfig:      true,
		CanBackupConfig:          true,
		CanExecuteCommands:       true,
		CanExecuteConfigCommands: true,
	},

	DeviceTypePaloAltoPA: {
		CanGetRunningConfig:      true,
		CanGetStartupConfig:      false, // PANOS doesn't have traditional startup config
		CanBackupConfig:          true,
		CanExecuteCommands:       true,
		CanExecuteConfigCommands: true,
	},

	DeviceTypePaloAltoPanorama: {
		CanManageDevices:    true,
		CanGetDeviceList:    true,
		CanPushConfig:       true,
		CanGetDeviceStatus:  true,
		CanMonitorDevices:   true,
		CanGetLogs:          true,
	},

	DeviceTypeCiscoFMC: {
		CanManageDevices:    true,
		CanGetDeviceList:    true,
		CanPushConfig:       true,
		CanGetDeviceStatus:  true,
		CanMonitorDevices:   true,
		CanGetLogs:          true,
		CanGetSecurityEvents: true,
	},

	DeviceTypeSolarWindsNCM: {
		CanManageDevices:         true,
		CanGetDeviceList:         true,
		CanGetRunningConfig:      true,
		CanGetStartupConfig:      true,
		CanBackupConfig:          true,
		CanPushConfig:            true,
		CanGetDeviceStatus:       true,
		CanMonitorDevices:        true,
		CanGetLogs:               true,
		CanExecuteCommands:       true,
		CanExecuteConfigCommands: true,
	},

	DeviceTypeFortinetFortiManager: {
		CanManageDevices:    true,
		CanGetDeviceList:    true,
		CanPushConfig:       true,
		CanGetDeviceStatus:  true,
		CanMonitorDevices:   true,
		CanGetLogs:          true,
	},

	DeviceTypeGenericSSH: {
		CanGetRunningConfig:      true,
		CanExecuteCommands:       true,
		CanExecuteConfigCommands: true,
	},
}

// GetDeviceCapabilities returns the capabilities for a given device type
func GetDeviceCapabilities(deviceType DeviceType) DeviceCapabilities {
	caps, exists := DefaultDeviceCapabilities[deviceType]
	if !exists {
		// Return minimal capabilities for unknown devices
		return DeviceCapabilities{
			CanExecuteCommands: true,
		}
	}
	return caps
}

// GetCommandTemplates returns command templates for a given device type
func GetCommandTemplates(deviceType DeviceType) []CommandTemplate {
	templates, exists := DefaultCommandTemplates[deviceType]
	if !exists {
		return []CommandTemplate{}
	}
	return templates
}

// GetOSTypeFromDeviceType maps device type to OS type
func GetOSTypeFromDeviceType(deviceType DeviceType) OSType {
	switch deviceType {
	case DeviceTypeCiscoIOS:
		return OSTypeIOS
	case DeviceTypeCiscoASA:
		return OSTypeASA
	case DeviceTypeCiscoNXOS:
		return OSTypeNXOS
	case DeviceTypePaloAltoPA, DeviceTypePaloAltoPanorama:
		return OSTypePANOS
	case DeviceTypeJuniperJunos, DeviceTypeJuniperSRX:
		return OSTypeJunos
	case DeviceTypeFortinetFortiGate, DeviceTypeFortinetFortiManager:
		return OSTypeFortiOS
	case DeviceTypeCheckPointGaia:
		return OSTypeGaia
	case DeviceTypeF5BigIP:
		return OSTypeBigIP
	default:
		return OSTypeLinux
	}
}
