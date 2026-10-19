package base

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"
)

// BaseConnector provides common functionality for all connector implementations
type BaseConnector struct {
	// Config is retained for compatibility with existing connector callers.
	Config         *ConnectionConfig
	config        *ConnectionConfig
	state         ConnectionState
	stateMux      sync.RWMutex
	connected     bool
	connMux       sync.RWMutex
	workingFolder string
}

// NewBaseConnector creates a new base connector instance
func NewBaseConnector(config *ConnectionConfig) *BaseConnector {
	return &BaseConnector{
		Config: config,
		config: config,
		state:  ConnectionStateDisconnected,
	}
}

// GetConfig returns the connection configuration
func (bc *BaseConnector) GetConfig() *ConnectionConfig {
	return bc.config
}

// GetConnectionState returns the current connection state
func (bc *BaseConnector) GetConnectionState() ConnectionState {
	bc.stateMux.RLock()
	defer bc.stateMux.RUnlock()
	return bc.state
}

// setConnectionState sets the connection state
func (bc *BaseConnector) setConnectionState(state ConnectionState) {
	bc.stateMux.Lock()
	defer bc.stateMux.Unlock()
	bc.state = state
}

// SetConnectionState updates the connector state for concrete adapters.
func (bc *BaseConnector) SetConnectionState(state ConnectionState) { bc.setConnectionState(state) }

// IsConnected returns whether the connector is connected
func (bc *BaseConnector) IsConnected() bool {
	bc.connMux.RLock()
	defer bc.connMux.RUnlock()
	return bc.connected
}

// setConnected sets the connected state
func (bc *BaseConnector) setConnected(connected bool) {
	bc.connMux.Lock()
	defer bc.connMux.Unlock()
	bc.connected = connected
}

// SetConnected updates the connection flag for concrete adapters.
func (bc *BaseConnector) SetConnected(connected bool) { bc.setConnected(connected) }

// Connect attempts to establish a connection (to be overridden by implementations)
func (bc *BaseConnector) Connect(ctx context.Context) error {
	bc.setConnectionState(ConnectionStateConnecting)
	defer func() {
		if !bc.IsConnected() {
			bc.setConnectionState(ConnectionStateDisconnected)
		}
	}()

	return nil
}

// Disconnect closes the connection
func (bc *BaseConnector) Disconnect(ctx context.Context) error {
	bc.setConnected(false)
	bc.setConnectionState(ConnectionStateDisconnected)
	return nil
}

// Ping performs a basic connectivity check
func (bc *BaseConnector) Ping(ctx context.Context, timeout time.Duration) error {
	// Default implementation - should be overridden
	return fmt.Errorf("ping method not implemented")
}

// GetDeviceInfo retrieves device information (to be overridden)
func (bc *BaseConnector) GetDeviceInfo(ctx context.Context) (*DeviceInfo, error) {
	return nil, fmt.Errorf("get device info not implemented")
}

// GetSupportedCommands returns supported commands (to be overridden)
func (bc *BaseConnector) GetSupportedCommands(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("get supported commands not implemented")
}

// ExecuteCommand executes a single command (to be overridden)
func (bc *BaseConnector) ExecuteCommand(ctx context.Context, command string, timeout time.Duration) (*CommandResult, error) {
	return nil, fmt.Errorf("execute command not implemented")
}

// ExecuteCommands executes multiple commands (to be overridden)
func (bc *BaseConnector) ExecuteCommands(ctx context.Context, commands []string, timeout time.Duration) ([]*CommandResult, error) {
	results := make([]*CommandResult, len(commands))
	for i, cmd := range commands {
		result, err := bc.ExecuteCommand(ctx, cmd, timeout)
		if err != nil {
			results[i] = &CommandResult{
				Command:    cmd,
				Error:      err.Error(),
				ExitCode:   -1,
				ExecutedAt: time.Now(),
			}
		} else {
			results[i] = result
		}
	}
	return results, nil
}

// GetRunningConfig retrieves the running configuration (to be overridden)
func (bc *BaseConnector) GetRunningConfig(ctx context.Context, timeout time.Duration) (*ConfigResult, error) {
	return nil, fmt.Errorf("get running config not implemented")
}

// GetStartupConfig retrieves the startup configuration (to be overridden)
func (bc *BaseConnector) GetStartupConfig(ctx context.Context, timeout time.Duration) (*ConfigResult, error) {
	return nil, fmt.Errorf("get startup config not implemented")
}

// GetDeviceInventory gets device inventory (to be overridden)
func (bc *BaseConnector) GetDeviceInventory(ctx context.Context) ([]*DeviceInfo, error) {
	return nil, fmt.Errorf("get device inventory not implemented")
}

// GetDeviceConfigs gets configurations for multiple devices (to be overridden)
func (bc *BaseConnector) GetDeviceConfigs(ctx context.Context, deviceIDs []string) (map[string]*ConfigResult, error) {
	return nil, fmt.Errorf("get device configs not implemented")
}

// SendCommandsToDevice sends commands to a specific device (to be overridden)
func (bc *BaseConnector) SendCommandsToDevice(ctx context.Context, deviceID string, commands []string, timeout time.Duration) ([]*CommandResult, error) {
	return nil, fmt.Errorf("send commands to device not implemented")
}

// calculateConfigHash calculates a SHA256 hash of the configuration data
func (bc *BaseConnector) calculateConfigHash(configData string) string {
	hash := sha256.Sum256([]byte(configData))
	return "sha256:" + hex.EncodeToString(hash[:])
}

// createCommandResult creates a standardized command result
func (bc *BaseConnector) createCommandResult(deviceID, command, output, errorMsg string, exitCode int, duration time.Duration) *CommandResult {
	return &CommandResult{
		DeviceID:   deviceID,
		Command:    command,
		Output:     output,
		Error:      errorMsg,
		ExitCode:   exitCode,
		ExecutedAt: time.Now(),
		Duration:   duration,
	}
}

// CreateCommandResult builds a timestamped command result for concrete adapters.
func (bc *BaseConnector) CreateCommandResult(deviceID, command, output, errorMsg string, exitCode int, duration time.Duration) *CommandResult {
	return bc.createCommandResult(deviceID, command, output, errorMsg, exitCode, duration)
}

// createConfigResult creates a standardized config result
func (bc *BaseConnector) createConfigResult(deviceID, configType, configData string, duration time.Duration) *ConfigResult {
	return &ConfigResult{
		DeviceID:    deviceID,
		ConfigType:  configType,
		ConfigData:  configData,
		Hash:        bc.calculateConfigHash(configData),
		RetrievedAt: time.Now(),
		Duration:    duration,
	}
}

// CreateConfigResult builds a hashed config result for concrete adapters.
func (bc *BaseConnector) CreateConfigResult(deviceID, configType, configData string, duration time.Duration) *ConfigResult {
	return bc.createConfigResult(deviceID, configType, configData, duration)
}

// RetryOperation retries an operation with exponential backoff
func (bc *BaseConnector) RetryOperation(ctx context.Context, operation func() error) error {
	var lastErr error

	for i := 0; i < bc.config.RetryCount; i++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't wait after the last attempt
		if i < bc.config.RetryCount-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(bc.config.RetryDelay):
				// Continue to next retry
			}
		}
	}

	return lastErr
}

// ValidateConfig validates the connection configuration
func (bc *BaseConnector) ValidateConfig() error {
	if bc.config == nil {
		return fmt.Errorf("connection configuration is nil")
	}

	if bc.config.Host == "" {
		return fmt.Errorf("host is required")
	}

	if bc.config.Port <= 0 {
		return fmt.Errorf("valid port is required")
	}

	if bc.config.Timeout <= 0 {
		bc.config.Timeout = 30 * time.Second
	}

	if bc.config.RetryCount <= 0 {
		bc.config.RetryCount = 3
	}

	if bc.config.RetryDelay <= 0 {
		bc.config.RetryDelay = time.Second
	}

	return nil
}

// WriteConfigToFile writes configuration to a file (default implementation)
func (bc *BaseConnector) WriteConfigToFile(ctx context.Context, configType string, filePath string) error {
	return fmt.Errorf("write config to file not implemented")
}

// GetConfigFilePath returns the file path for a config type (default implementation)
func (bc *BaseConnector) GetConfigFilePath(configType string) string {
	return ""
}

// SetWorkingFolder sets the working folder for file operations
func (bc *BaseConnector) SetWorkingFolder(folder string) error {
	bc.workingFolder = folder
	return os.MkdirAll(folder, 0755)
}

// GetWorkingFolder returns the current working folder
func (bc *BaseConnector) GetWorkingFolder() string {
	return bc.workingFolder
}

// GetManagedDevices returns managed devices (default implementation)
func (bc *BaseConnector) GetManagedDevices(ctx context.Context) ([]*DeviceInfo, error) {
	return nil, fmt.Errorf("get managed devices not implemented")
}

// CollectDeviceConfigs collects configs from multiple devices (default implementation)
func (bc *BaseConnector) CollectDeviceConfigs(ctx context.Context, deviceIDs []string, workingFolder string) (map[string][]string, error) {
	return nil, fmt.Errorf("collect device configs not implemented")
}
