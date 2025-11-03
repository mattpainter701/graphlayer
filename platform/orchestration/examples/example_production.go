package connectors

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/api"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/registry"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/ssh"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/types"
)

// ProductionSSHExample demonstrates production SSH usage with interactive shell
func ProductionSSHExample() {
	ctx := context.Background()

	// Configure SSH connection
	config := &base.ConnectionConfig{
		Host:       "192.168.1.100",
		Port:       22,
		Username:   "admin",
		Password:   "password",
		Timeout:    30 * time.Second,
		RetryCount: 3,
		RetryDelay: 2 * time.Second,
	}

	// Create SSH connector
	connector, err := ssh.NewSSHConnector(config, types.DeviceTypeCiscoIOS)
	if err != nil {
		log.Fatalf("Failed to create connector: %v", err)
	}

	// Connect to device
	if err := connector.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer connector.Disconnect(ctx)

	fmt.Println("✓ Connected to device")

	// Method 1: Direct command execution (creates new session per command)
	result, err := connector.ExecuteCommand(ctx, "show version", 10*time.Second)
	if err != nil {
		log.Fatalf("Failed to execute command: %v", err)
	}
	fmt.Printf("Command output length: %d bytes\n", len(result.Output))

	// Method 2: Interactive shell (paramiko-style invoke_shell)
	shell, err := connector.InvokeShell(ctx)
	if err != nil {
		log.Fatalf("Failed to invoke shell: %v", err)
	}
	defer shell.Close()

	fmt.Println("✓ Interactive shell started")

	// Execute multiple commands in the same shell session
	commands := []string{
		"terminal length 0",
		"show running-config",
		"show interfaces",
		"show ip route",
	}

	for _, cmd := range commands {
		output, err := shell.SendCommand(cmd, 30*time.Second)
		if err != nil {
			log.Printf("Command failed: %s - %v", cmd, err)
			continue
		}
		fmt.Printf("✓ Executed: %s (%d bytes)\n", cmd, len(output))
	}

	// Get device information
	deviceInfo, err := connector.GetDeviceInfo(ctx)
	if err != nil {
		log.Printf("Failed to get device info: %v", err)
	} else {
		fmt.Printf("Device: %s %s (%s)\n", deviceInfo.Vendor, deviceInfo.Model, deviceInfo.OSVersion)
	}

	// Get running configuration
	config, err := connector.GetRunningConfig(ctx, 60*time.Second)
	if err != nil {
		log.Printf("Failed to get running config: %v", err)
	} else {
		fmt.Printf("✓ Retrieved running config: %d bytes\n", len(config.ConfigData))
		fmt.Printf("  Config hash: %s\n", config.Hash)
	}
}

// ProductionSSHWithKeyExample demonstrates SSH with key-based authentication
func ProductionSSHWithKeyExample() {
	ctx := context.Background()

	// Configure SSH with private key
	config := &base.ConnectionConfig{
		Host:        "192.168.1.100",
		Port:        22,
		Username:    "admin",
		KeyFile:     "/home/user/.ssh/id_rsa",
		KeyPassword: "key-passphrase", // Optional
		Timeout:     30 * time.Second,
	}

	connector, err := ssh.NewSSHConnector(config, types.DeviceTypeCiscoIOS)
	if err != nil {
		log.Fatalf("Failed to create connector: %v", err)
	}

	if err := connector.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer connector.Disconnect(ctx)

	fmt.Println("✓ Connected with key-based authentication")

	// Use the connection...
}

// ProductionAPIExample demonstrates production API usage with Cisco FMC
func ProductionAPIExample() {
	ctx := context.Background()

	// Configure API connection
	config := &base.ConnectionConfig{
		Host:       "fmc.example.com",
		Port:       443,
		Username:   "api_user",
		Password:   "api_password",
		Timeout:    30 * time.Second,
		RetryCount: 3,
		RetryDelay: 2 * time.Second,
		// For custom CA certificates
		CACert:  "/path/to/ca.crt",
	}

	// Create API connector
	connector, err := api.NewAPIConnector(config, types.DeviceTypeCiscoFMC)
	if err != nil {
		log.Fatalf("Failed to create connector: %v", err)
	}

	// Connect and authenticate
	if err := connector.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer connector.Disconnect(ctx)

	fmt.Println("✓ Connected to Cisco FMC")

	// Get managed devices
	devices, err := connector.GetDeviceInventory(ctx)
	if err != nil {
		log.Fatalf("Failed to get device inventory: %v", err)
	}
	fmt.Printf("✓ FMC manages %d devices\n", len(devices))

	for _, device := range devices {
		fmt.Printf("  - %s (%s) - %s\n", device.Hostname, device.IPAddress, device.Model)
	}

	// Get configurations for specific devices
	deviceIDs := make([]string, 0, len(devices))
	for _, device := range devices {
		deviceIDs = append(deviceIDs, device.ID)
	}

	configs, err := connector.GetDeviceConfigs(ctx, deviceIDs)
	if err != nil {
		log.Printf("Failed to get device configs: %v", err)
	} else {
		fmt.Printf("✓ Retrieved configs for %d devices\n", len(configs))
	}

	// Execute commands
	result, err := connector.ExecuteCommand(ctx, "get_device_list", 30*time.Second)
	if err != nil {
		log.Printf("Command failed: %v", err)
	} else {
		fmt.Printf("✓ Command executed: %s\n", result.Command)
	}
}

// ProductionPanoramaExample demonstrates Palo Alto Panorama integration
func ProductionPanoramaExample() {
	ctx := context.Background()

	// Configure Panorama API connection
	config := &base.ConnectionConfig{
		Host:    "panorama.example.com",
		Port:    443,
		APIKey:  "your-panorama-api-key",
		Timeout: 30 * time.Second,
	}

	connector, err := api.NewAPIConnector(config, types.DeviceTypePaloAltoPanorama)
	if err != nil {
		log.Fatalf("Failed to create connector: %v", err)
	}

	if err := connector.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer connector.Disconnect(ctx)

	fmt.Println("✓ Connected to Panorama")

	// Get managed firewalls
	devices, err := connector.GetDeviceInventory(ctx)
	if err != nil {
		log.Fatalf("Failed to get device inventory: %v", err)
	}
	fmt.Printf("✓ Panorama manages %d firewalls\n", len(devices))

	// Get device groups
	result, err := connector.ExecuteCommand(ctx, "get_device_groups", 30*time.Second)
	if err != nil {
		log.Printf("Failed to get device groups: %v", err)
	} else {
		fmt.Printf("✓ Retrieved device groups\n")
	}

	// Get templates
	result, err = connector.ExecuteCommand(ctx, "get_templates", 30*time.Second)
	if err != nil {
		log.Printf("Failed to get templates: %v", err)
	} else {
		fmt.Printf("✓ Retrieved templates\n")
	}
}

// ProductionManagerExample demonstrates using the connector manager for multiple devices
func ProductionManagerExample() {
	ctx := context.Background()

	// Create connector manager
	manager := registry.NewConnectorManager()

	// Define device configurations
	devices := []struct {
		ID         string
		DeviceType types.DeviceType
		Config     *base.ConnectionConfig
	}{
		{
			ID:         "core-switch-01",
			DeviceType: types.DeviceTypeCiscoIOS,
			Config: &base.ConnectionConfig{
				Host:     "10.1.1.1",
				Port:     22,
				Username: "admin",
				Password: "password",
				Timeout:  30 * time.Second,
			},
		},
		{
			ID:         "firewall-01",
			DeviceType: types.DeviceTypeCiscoASA,
			Config: &base.ConnectionConfig{
				Host:     "10.1.1.2",
				Port:     22,
				Username: "admin",
				Password: "password",
				Timeout:  30 * time.Second,
			},
		},
		{
			ID:         "fmc-01",
			DeviceType: types.DeviceTypeCiscoFMC,
			Config: &base.ConnectionConfig{
				Host:     "10.1.1.10",
				Port:     443,
				Username: "api_user",
				Password: "api_password",
				Timeout:  30 * time.Second,
			},
		},
	}

	// Register and connect to all devices
	for _, dev := range devices {
		connector, err := manager.CreateAndRegisterConnector(dev.ID, dev.DeviceType, dev.Config)
		if err != nil {
			log.Printf("Failed to create connector for %s: %v", dev.ID, err)
			continue
		}

		if err := connector.Connect(ctx); err != nil {
			log.Printf("Failed to connect to %s: %v", dev.ID, err)
			continue
		}

		fmt.Printf("✓ Connected to %s\n", dev.ID)
	}

	// Get all connected devices
	allConnectors := manager.GetAllConnectors()
	fmt.Printf("\nManaging %d device connections\n", len(allConnectors))

	// Execute commands on all SSH devices
	for deviceID, connector := range allConnectors {
		if connector.GetType() == "ssh" {
			fmt.Printf("\nGetting config from %s...\n", deviceID)
			
			config, err := connector.GetRunningConfig(ctx, 60*time.Second)
			if err != nil {
				log.Printf("Failed to get config from %s: %v", deviceID, err)
				continue
			}

			fmt.Printf("✓ Retrieved config from %s: %d bytes (hash: %s)\n", 
				deviceID, len(config.ConfigData), config.Hash[:16]+"...")
		}
	}

	// Get device inventory from management systems
	for deviceID, connector := range allConnectors {
		if connector.GetType() == "api" {
			fmt.Printf("\nGetting managed devices from %s...\n", deviceID)
			
			devices, err := connector.GetDeviceInventory(ctx)
			if err != nil {
				log.Printf("Failed to get inventory from %s: %v", deviceID, err)
				continue
			}

			fmt.Printf("✓ %s manages %d devices\n", deviceID, len(devices))
		}
	}

	// Cleanup: disconnect all
	for deviceID := range allConnectors {
		if err := manager.RemoveConnector(deviceID); err != nil {
			log.Printf("Failed to disconnect %s: %v", deviceID, err)
		}
	}
}

// ProductionBulkConfigCollection demonstrates collecting configs from multiple devices
func ProductionBulkConfigCollection() {
	ctx := context.Background()
	manager := registry.NewConnectorManager()

	// Device list to collect from
	deviceList := []struct {
		Hostname string
		IP       string
		Type     types.DeviceType
	}{
		{"core-sw-01", "10.1.1.1", types.DeviceTypeCiscoIOS},
		{"core-sw-02", "10.1.1.2", types.DeviceTypeCiscoIOS},
		{"dist-sw-01", "10.1.2.1", types.DeviceTypeCiscoIOS},
		{"dist-sw-02", "10.1.2.2", types.DeviceTypeCiscoIOS},
		{"fw-01", "10.1.1.10", types.DeviceTypeCiscoASA},
		{"fw-02", "10.1.1.11", types.DeviceTypeCiscoASA},
	}

	fmt.Printf("Collecting configurations from %d devices...\n\n", len(deviceList))

	successCount := 0
	failCount := 0

	for _, device := range deviceList {
		fmt.Printf("Processing %s (%s)...\n", device.Hostname, device.IP)

		config := &base.ConnectionConfig{
			Host:       device.IP,
			Port:       22,
			Username:   "admin",
			Password:   "password",
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 2 * time.Second,
		}

		connector, err := manager.CreateAndRegisterConnector(device.Hostname, device.Type, config)
		if err != nil {
			log.Printf("  ✗ Failed to create connector: %v", err)
			failCount++
			continue
		}

		// Connect with timeout
		connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = connector.Connect(connectCtx)
		cancel()

		if err != nil {
			log.Printf("  ✗ Failed to connect: %v", err)
			failCount++
			continue
		}

		// Get running config with timeout
		configCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		configResult, err := connector.GetRunningConfig(configCtx, 60*time.Second)
		cancel()

		if err != nil {
			log.Printf("  ✗ Failed to get config: %v", err)
			failCount++
			continue
		}

		fmt.Printf("  ✓ Config retrieved: %d bytes (hash: %s)\n", 
			len(configResult.ConfigData), configResult.Hash[:16]+"...")

		// TODO: Save config to database or file
		// saveConfig(device.Hostname, configResult)

		successCount++
	}

	fmt.Printf("\n=== Collection Summary ===\n")
	fmt.Printf("Total devices: %d\n", len(deviceList))
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", failCount)

	// Cleanup
	for _, device := range deviceList {
		manager.RemoveConnector(device.Hostname)
	}
}

// ProductionErrorHandlingExample demonstrates proper error handling
func ProductionErrorHandlingExample() {
	ctx := context.Background()

	config := &base.ConnectionConfig{
		Host:       "192.168.1.100",
		Port:       22,
		Username:   "admin",
		Password:   "password",
		Timeout:    10 * time.Second,
		RetryCount: 3,
		RetryDelay: time.Second,
	}

	connector, err := ssh.NewSSHConnector(config, types.DeviceTypeCiscoIOS)
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Connect with timeout and retry
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := connector.Connect(connectCtx); err != nil {
		// Handle connection errors
		errStr := err.Error()
		switch {
		case contains(errStr, "timeout"):
			log.Printf("Connection timeout - device may be unreachable")
		case contains(errStr, "authentication"):
			log.Printf("Authentication failed - check credentials")
		case contains(errStr, "refused"):
			log.Printf("Connection refused - check if SSH is enabled")
		default:
			log.Printf("Connection failed: %v", err)
		}
		return
	}
	defer connector.Disconnect(ctx)

	// Check connection state
	if connector.GetConnectionState() != base.ConnectionStateConnected {
		log.Printf("Unexpected connection state: %s", connector.GetConnectionState())
		return
	}

	// Execute command with timeout
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 30*time.Second)
	defer cmdCancel()

	result, err := connector.ExecuteCommand(cmdCtx, "show running-config", 30*time.Second)
	if err != nil {
		log.Printf("Command execution failed: %v", err)
		return
	}

	// Check command result
	if result.Error != "" {
		log.Printf("Command returned error: %s", result.Error)
		return
	}

	if result.ExitCode != 0 {
		log.Printf("Command failed with exit code: %d", result.ExitCode)
		return
	}

	fmt.Printf("✓ Command executed successfully: %d bytes in %v\n", 
		len(result.Output), result.Duration)
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

