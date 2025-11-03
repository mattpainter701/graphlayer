package connectors

import (
	"context"
	"fmt"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/api"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/registry"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/ssh"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/types"
)

// ExampleUsage demonstrates how to use the connector framework
func ExampleUsage() {
	ctx := context.Background()

	// Create a connector manager
	manager := registry.NewConnectorManager()

	// Example 1: Connect to a Cisco IOS device via SSH
	ciscoIOSConfig := &base.ConnectionConfig{
		Host:     "192.168.1.100",
		Port:     22,
		Username: "admin",
		Password: "password123",
		Timeout:  30 * time.Second,
	}

	ciscoConnector, err := manager.CreateAndRegisterConnector(
		"cisco-switch-01",
		types.DeviceTypeCiscoIOS,
		ciscoIOSConfig,
	)
	if err != nil {
		fmt.Printf("Failed to create Cisco connector: %v\n", err)
		return
	}

	// Connect to the device
	if err := ciscoConnector.Connect(ctx); err != nil {
		fmt.Printf("Failed to connect to Cisco device: %v\n", err)
		return
	}

	// Get device information
	deviceInfo, err := ciscoConnector.GetDeviceInfo(ctx)
	if err != nil {
		fmt.Printf("Failed to get device info: %v\n", err)
	} else {
		fmt.Printf("Device: %s (%s %s)\n", deviceInfo.Hostname, deviceInfo.Vendor, deviceInfo.Model)
	}

	// Get running configuration
	config, err := ciscoConnector.GetRunningConfig(ctx, 60*time.Second)
	if err != nil {
		fmt.Printf("Failed to get running config: %v\n", err)
	} else {
		fmt.Printf("Config retrieved: %d characters\n", len(config.ConfigData))
	}

	// Execute a command
	result, err := ciscoConnector.ExecuteCommand(ctx, "show interfaces", 10*time.Second)
	if err != nil {
		fmt.Printf("Failed to execute command: %v\n", err)
	} else {
		fmt.Printf("Command output: %d characters\n", len(result.Output))
	}

	// Example 2: Connect to Cisco FMC via API
	fmcConfig := &base.ConnectionConfig{
		Host:   "192.168.1.200",
		Port:   443,
		APIKey: "your-fmc-api-key",
		Timeout: 30 * time.Second,
	}

	fmcConnector, err := manager.CreateAndRegisterConnector(
		"cisco-fmc-01",
		types.DeviceTypeCiscoFMC,
		fmcConfig,
	)
	if err != nil {
		fmt.Printf("Failed to create FMC connector: %v\n", err)
		return
	}

	// Connect to FMC
	if err := fmcConnector.Connect(ctx); err != nil {
		fmt.Printf("Failed to connect to FMC: %v\n", err)
		return
	}

	// Get managed devices from FMC
	devices, err := fmcConnector.GetDeviceInventory(ctx)
	if err != nil {
		fmt.Printf("Failed to get device inventory: %v\n", err)
	} else {
		fmt.Printf("FMC manages %d devices\n", len(devices))
	}

	// Example 3: Get device configurations from FMC
	deviceIDs := []string{"managed-device-1", "managed-device-2"}
	configs, err := fmcConnector.GetDeviceConfigs(ctx, deviceIDs)
	if err != nil {
		fmt.Printf("Failed to get device configs: %v\n", err)
	} else {
		fmt.Printf("Retrieved configs for %d devices\n", len(configs))
	}

	// Cleanup
	ciscoConnector.Disconnect(ctx)
	fmcConnector.Disconnect(ctx)

	fmt.Println("Example completed successfully")
}

// ExampleConnectorCreation demonstrates creating connectors directly
func ExampleConnectorCreation() {
	// Create SSH connector for Cisco device
	config := &base.ConnectionConfig{
		Host:     "192.168.1.100",
		Port:     22,
		Username: "admin",
		Password: "password",
		Timeout:  30 * time.Second,
	}

	sshConnector, err := NewSSHConnector(config, types.DeviceTypeCiscoIOS)
	if err != nil {
		fmt.Printf("Failed to create SSH connector: %v\n", err)
		return
	}

	fmt.Printf("Created SSH connector for %s\n", sshConnector.GetType())

	// Create API connector for management system
	apiConfig := &base.ConnectionConfig{
		Host:   "192.168.1.200",
		Port:   443,
		APIKey: "api-key",
		Timeout: 30 * time.Second,
	}

	apiConnector, err := NewAPIConnector(apiConfig, types.DeviceTypeCiscoFMC)
	if err != nil {
		fmt.Printf("Failed to create API connector: %v\n", err)
		return
	}

	fmt.Printf("Created API connector for %s\n", apiConnector.GetType())
}
