package fortimanager

import (
	"fmt"
	"log"
	"time"
)

// ExampleBasicImport demonstrates basic configuration import from FortiManager
func ExampleBasicImport() {
	// Create FortiManager configuration
	config := FortiManagerConfig{
		BaseURL:            "https://fortimanager.example.com",
		Username:           "admin",
		Password:           "password",
		InsecureSkipVerify: true, // Only for self-signed certs in dev
		Timeout:            60 * time.Second,
	}

	// Create import service
	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}
	defer service.Close()

	// Import all configurations
	request := ImportRequest{
		IncludeGlobalObjects:  true,
		IncludePolicyPackages: true,
	}

	summary, err := service.ImportConfigurations(request)
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	// Print summary
	fmt.Println(summary.FormatSummary())
}

// ExampleListADOMs demonstrates listing ADOMs
func ExampleListADOMs() {
	config := FortiManagerConfig{
		BaseURL:            "https://fortimanager.example.com",
		Username:           "admin",
		Password:           "password",
		InsecureSkipVerify: true,
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}
	defer service.Close()

	fmt.Println("=== Available ADOMs ===")
	adoms, err := service.ListADOMs()
	if err != nil {
		log.Fatalf("Failed to list ADOMs: %v", err)
	}

	for _, adom := range adoms {
		fmt.Printf("\n%s\n", adom.Name)
		fmt.Printf("  Description: %s\n", adom.Description)
		fmt.Printf("  Mode: %s\n", adom.Mode)
		fmt.Printf("  OS Version: %s\n", adom.OSVersion)
		fmt.Printf("  Status: %d\n", adom.Status)
	}
}

// ExampleListDevices demonstrates listing managed devices in an ADOM
func ExampleListDevices() {
	config := FortiManagerConfig{
		BaseURL:            "https://fortimanager.example.com",
		Username:           "admin",
		Password:           "password",
		InsecureSkipVerify: true,
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}
	defer service.Close()

	adomName := "root"
	fmt.Printf("=== Devices in ADOM: %s ===\n", adomName)

	devices, err := service.ListDevices(adomName)
	if err != nil {
		log.Fatalf("Failed to list devices: %v", err)
	}

	for _, device := range devices {
		connStatus := "Disconnected"
		if device.Connected == 1 {
			connStatus = "Connected"
		}

		installStatus := "Unknown"
		switch device.InstallStatus {
		case 1:
			installStatus = "Installed"
		case 2:
			installStatus = "Modified"
		}

		fmt.Printf("\n%s (%s)\n", device.Name, device.SerialNumber)
		fmt.Printf("  IP: %s\n", device.IPAddress)
		fmt.Printf("  Model: %s\n", device.Model)
		fmt.Printf("  Version: %s\n", device.Version)
		fmt.Printf("  Connection: %s\n", connStatus)
		fmt.Printf("  HA Mode: %s\n", device.HAMode)
		fmt.Printf("  VDOM Mode: %s\n", device.VDOMMode)
		fmt.Printf("  Install Status: %s\n", installStatus)
		if len(device.DeviceGroups) > 0 {
			fmt.Printf("  Device Groups: %v\n", device.DeviceGroups)
		}
	}
}

// ExampleFilteredImport demonstrates importing from specific ADOMs and devices
func ExampleFilteredImport() {
	config := FortiManagerConfig{
		BaseURL:  "https://fortimanager.example.com",
		Username: "admin",
		Password: "password",
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}
	defer service.Close()

	// Import only from specific ADOMs
	request := ImportRequest{
		ADOMs: []string{"root", "production"},
		DeviceNames: []string{
			"firewall-01",
			"firewall-02",
			"datacenter-fw",
		},
		IncludeGlobalObjects:  true,
		IncludePolicyPackages: false,
	}

	summary, err := service.ImportConfigurations(request)
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	fmt.Println("=== Filtered Import Results ===")
	fmt.Println(summary.FormatSummary())
}

// ExampleADOMStatistics demonstrates getting ADOM statistics
func ExampleADOMStatistics() {
	config := FortiManagerConfig{
		BaseURL:  "https://fortimanager.example.com",
		Username: "admin",
		Password: "password",
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}
	defer service.Close()

	adoms, err := service.ListADOMs()
	if err != nil {
		log.Fatalf("Failed to list ADOMs: %v", err)
	}

	fmt.Println("=== ADOM Statistics ===")
	for _, adom := range adoms {
		stats, err := service.GetADOMStatistics(adom.Name)
		if err != nil {
			log.Printf("Failed to get stats for %s: %v", adom.Name, err)
			continue
		}

		fmt.Printf("\n%s:\n", stats.ADOMName)
		fmt.Printf("  Total Devices: %d\n", stats.TotalDevices)
		fmt.Printf("  Connected: %d\n", stats.ConnectedDevices)
		fmt.Printf("  Policy Packages: %d\n", stats.PolicyPackages)
	}
}

// ExampleGetDeviceConfig demonstrates retrieving a specific device configuration
func ExampleGetDeviceConfig() {
	config := FortiManagerConfig{
		BaseURL:  "https://fortimanager.example.com",
		Username: "admin",
		Password: "password",
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}
	defer service.Close()

	adomName := "root"
	deviceName := "firewall-01"

	fmt.Printf("=== Configuration for %s/%s ===\n", adomName, deviceName)

	config_data, err := service.GetDeviceConfiguration(adomName, deviceName)
	if err != nil {
		log.Fatalf("Failed to get device config: %v", err)
	}

	// Process configuration data
	fmt.Printf("Retrieved %d configuration elements\n", len(config_data))

	// In real implementation, this would be parsed and processed
	// by FortiGate parsers to extract interfaces, policies, etc.
}

// ExampleConnectedDevicesOnly demonstrates importing only from connected devices
func ExampleConnectedDevicesOnly() {
	config := FortiManagerConfig{
		BaseURL:  "https://fortimanager.example.com",
		Username: "admin",
		Password: "password",
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}
	defer service.Close()

	adoms, err := service.ListADOMs()
	if err != nil {
		log.Fatalf("Failed to list ADOMs: %v", err)
	}

	fmt.Println("=== Connected Devices ===")
	totalConnected := 0

	for _, adom := range adoms {
		devices, err := service.ListDevices(adom.Name)
		if err != nil {
			log.Printf("Failed to list devices for %s: %v", adom.Name, err)
			continue
		}

		connectedDevices := []ManagedDevice{}
		for _, device := range devices {
			if device.Connected == 1 {
				connectedDevices = append(connectedDevices, device)
			}
		}

		if len(connectedDevices) > 0 {
			fmt.Printf("\nADOM: %s (%d connected)\n", adom.Name, len(connectedDevices))
			for _, device := range connectedDevices {
				fmt.Printf("  - %s (%s) - %s %s\n",
					device.Name,
					device.IPAddress,
					device.Model,
					device.Version)
			}
			totalConnected += len(connectedDevices)
		}
	}

	fmt.Printf("\nTotal Connected Devices: %d\n", totalConnected)
}

// ExamplePolicyPackages demonstrates listing policy packages
func ExamplePolicyPackages() {
	config := FortiManagerConfig{
		BaseURL:  "https://fortimanager.example.com",
		Username: "admin",
		Password: "password",
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}
	defer service.Close()

	adomName := "root"
	fmt.Printf("=== Policy Packages in ADOM: %s ===\n", adomName)

	packages, err := service.Client.GetPolicyPackages(adomName)
	if err != nil {
		log.Fatalf("Failed to get policy packages: %v", err)
	}

	for _, pkg := range packages {
		fmt.Printf("\n%s\n", pkg.Name)
		fmt.Printf("  Type: %s\n", pkg.Type)
		fmt.Printf("  Description: %s\n", pkg.Description)

		if len(pkg.Scope) > 0 {
			fmt.Printf("  Scope:\n")
			for _, scope := range pkg.Scope {
				fmt.Printf("    - Device: %s, VDOM: %s\n", scope.Name, scope.VDOM)
			}
		}
	}
}
