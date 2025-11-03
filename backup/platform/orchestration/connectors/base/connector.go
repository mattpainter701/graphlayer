package base

import (
	"context"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

// ConnectionState represents the state of a connector connection
type ConnectionState string

const (
	ConnectionStateDisconnected ConnectionState = "disconnected"
	ConnectionStateConnecting   ConnectionState = "connecting"
	ConnectionStateConnected     ConnectionState = "connected"
	ConnectionStateError         ConnectionState = "error"
)

// ConnectionConfig holds connection configuration
type ConnectionConfig struct {
	// Common fields
	Host         string            `json:"host"`
	Port         int               `json:"port"`
	Timeout      time.Duration     `json:"timeout"`
	RetryCount   int               `json:"retry_count"`
	RetryDelay   time.Duration     `json:"retry_delay"`

	// SSH specific
	Username     string            `json:"username,omitempty"`
	Password     string            `json:"password,omitempty"`
	KeyFile      string            `json:"key_file,omitempty"`
	KeyPassword  string            `json:"key_password,omitempty"`

	// API specific
	APIKey       string            `json:"api_key,omitempty"`
	Token        string            `json:"token,omitempty"`
	CertFile     string            `json:"cert_file,omitempty"`
	KeyFileAPI   string            `json:"key_file_api,omitempty"`
	CACert       string            `json:"ca_cert,omitempty"`

	// Headers for API requests
	Headers      map[string]string `json:"headers,omitempty"`

	// Connection-specific options
	Options      map[string]interface{} `json:"options,omitempty"`
}

// DeviceInfo represents information about a network device
type DeviceInfo struct {
	// Basic device information
	ID           string            `json:"id"`
	Hostname     string            `json:"hostname"`
	IPAddress    string            `json:"ip_address"`
	MACAddress   string            `json:"mac_address,omitempty"`

	// Device classification
	Vendor       string            `json:"vendor"`
	Model        string            `json:"model"`
	OSType       string            `json:"os_type"`
	OSVersion    string            `json:"os_version,omitempty"`

	// Network information
	BusinessUnit string            `json:"business_unit,omitempty"`
	Site         string            `json:"site,omitempty"`
	Network      string            `json:"network,omitempty"`

	// Connection information
	ConnectionType string          `json:"connection_type"` // "ssh", "api", "management"
	ManagementIP   string          `json:"management_ip,omitempty"`

	// Device capabilities
	Capabilities   []string        `json:"capabilities,omitempty"`
	SupportedCommands []string     `json:"supported_commands,omitempty"`

	// Metadata
	Tags         map[string]string `json:"tags,omitempty"`
	LastSeen     time.Time         `json:"last_seen"`
}

// CommandResult represents the result of executing a command
type CommandResult struct {
	DeviceID     string    `json:"device_id"`
	Command      string    `json:"command"`
	Output       string    `json:"output"`
	Error        string    `json:"error,omitempty"`
	ExitCode     int       `json:"exit_code"`
	ExecutedAt   time.Time `json:"executed_at"`
	Duration     time.Duration `json:"duration"`
}

// ConfigResult represents the result of retrieving a configuration
type ConfigResult struct {
	DeviceID     string    `json:"device_id"`
	ConfigType   string    `json:"config_type"` // "running", "startup", "candidate"
	ConfigData   string    `json:"config_data"`
	Hash         string    `json:"hash"`
	RetrievedAt  time.Time `json:"retrieved_at"`
	Duration     time.Duration `json:"duration"`
}

// Connector defines the interface that all connector types must implement
type Connector interface {
	// Connection management
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	IsConnected() bool
	GetConnectionState() ConnectionState

	// Device information
	GetDeviceInfo(ctx context.Context) (*DeviceInfo, error)
	GetSupportedCommands(ctx context.Context) ([]string, error)

	// Command execution
	ExecuteCommand(ctx context.Context, command string, timeout time.Duration) (*CommandResult, error)
	ExecuteCommands(ctx context.Context, commands []string, timeout time.Duration) ([]*CommandResult, error)

	// Configuration retrieval
	GetRunningConfig(ctx context.Context, timeout time.Duration) (*ConfigResult, error)
	GetStartupConfig(ctx context.Context, timeout time.Duration) (*ConfigResult, error)

	// Batch operations
	GetDeviceInventory(ctx context.Context) ([]*DeviceInfo, error)

	// Health check
	Ping(ctx context.Context, timeout time.Duration) error

	// Configuration management (for management servers)
	GetDeviceConfigs(ctx context.Context, deviceIDs []string) (map[string]*ConfigResult, error)
	SendCommandsToDevice(ctx context.Context, deviceID string, commands []string, timeout time.Duration) ([]*CommandResult, error)

	// Get connector type and configuration
	GetType() string
	GetConfig() *ConnectionConfig

	// File-based operations (enterprise integration)
	WriteConfigToFile(ctx context.Context, configType string, filePath string) error
	GetConfigFilePath(configType string) string

	// Working folder management
	SetWorkingFolder(folder string) error
	GetWorkingFolder() string

	// Multi-device operations
	GetManagedDevices(ctx context.Context) ([]*DeviceInfo, error)
	CollectDeviceConfigs(ctx context.Context, deviceIDs []string, workingFolder string) (map[string][]string, error)
}
