package solarwinds

import (
	"fmt"
	"log"
	"time"
)

// ExampleBasicImport demonstrates basic configuration import from NCM
func ExampleBasicImport() {
	// Create NCM configuration
	config := NCMConfig{
		BaseURL:            "https://solarwinds.example.com:17778",
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

	// Import all Cisco configurations from last 7 days
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	request := ImportRequest{
		Vendors:    []string{"Cisco"},
		ConfigType: "Running",
		SinceTime:  &sevenDaysAgo,
		Limit:      100,
	}

	summary, err := service.ImportConfigs(request)
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	// Print summary
	fmt.Println(summary.FormatSummary())

	// Process imported configs
	for _, config := range summary.ProcessedConfigs {
		fmt.Printf("\nProcessing: %s (%s)\n", config.NodeName, config.Vendor)
		fmt.Printf("  IP: %s\n", config.NodeIP)
		fmt.Printf("  OS: %s (confidence: %.2f)\n", config.DetectedOS, config.Confidence)
		fmt.Printf("  Config size: %d bytes\n", len(config.ConfigText))

		// Route to appropriate parser based on vendor
		switch config.Vendor {
		case VendorCiscoIOS:
			// Parse with Cisco IOS parser
			fmt.Println("  → Routing to Cisco IOS parser")
		case VendorCiscoASA:
			// Parse with Cisco ASA parser
			fmt.Println("  → Routing to Cisco ASA parser")
		case VendorCiscoNXOS:
			// Parse with Cisco NXOS parser
			fmt.Println("  → Routing to Cisco NXOS parser")
		case VendorPaloAlto:
			// Parse with Palo Alto parser
			fmt.Println("  → Routing to Palo Alto parser")
		case VendorFortinet:
			// Parse with Fortinet parser
			fmt.Println("  → Routing to FortiGate parser")
		default:
			fmt.Printf("  → Unsupported vendor: %s\n", config.Vendor)
		}
	}
}

// ExampleUserSelection demonstrates user-driven config selection
func ExampleUserSelection() {
	config := NCMConfig{
		BaseURL:            "https://solarwinds.example.com:17778",
		Username:           "admin",
		Password:           "password",
		InsecureSkipVerify: true,
		Timeout:            60 * time.Second,
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}

	// Step 1: List available configurations
	fmt.Println("=== Available Configurations ===")
	request := ImportRequest{
		Vendors: []string{"Cisco", "Palo Alto", "Fortinet"},
		Limit:   50,
	}

	availableConfigs, err := service.ListAvailableConfigs(request)
	if err != nil {
		log.Fatalf("Failed to list configs: %v", err)
	}

	// Step 2: Display configs for user selection
	fmt.Printf("\nFound %d configurations:\n\n", len(availableConfigs))
	for i, cfg := range availableConfigs {
		fmt.Printf("[%d] %s (%s)\n", i+1, cfg.NodeName, cfg.NodeIP)
		fmt.Printf("    Vendor: %s | OS: %s | Type: %s\n", cfg.Vendor, cfg.OSVersion, cfg.ConfigType)
		fmt.Printf("    Downloaded: %s | Size: %d bytes | Confidence: %.2f\n",
			cfg.DownloadTime.Format("2006-01-02 15:04:05"),
			cfg.ConfigSize,
			cfg.Confidence)
		fmt.Println()
	}

	// Step 3: Simulate user selection (in real implementation, get from UI/CLI)
	selectedNodeIDs := []int{
		availableConfigs[0].NodeID, // Select first config
		availableConfigs[2].NodeID, // Select third config
	}

	fmt.Printf("\n=== Importing %d Selected Configurations ===\n", len(selectedNodeIDs))

	// Step 4: Import only selected configs
	summary, err := service.ImportSelectedConfigs(selectedNodeIDs)
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	fmt.Println(summary.FormatSummary())
}

// ExampleVendorStats demonstrates getting vendor statistics
func ExampleVendorStats() {
	config := NCMConfig{
		BaseURL:            "https://solarwinds.example.com:17778",
		Username:           "admin",
		Password:           "password",
		InsecureSkipVerify: true,
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}

	fmt.Println("=== Vendor Statistics (Last 30 Days) ===")
	stats, err := service.GetVendorStats()
	if err != nil {
		log.Fatalf("Failed to get vendor stats: %v", err)
	}

	for vendor, count := range stats {
		fmt.Printf("%s: %d devices\n", vendor, count)
	}
}

// ExampleMultiVendorImport demonstrates importing configs from multiple vendors
func ExampleMultiVendorImport() {
	config := NCMConfig{
		BaseURL:  "https://solarwinds.example.com:17778",
		Username: "admin",
		Password: "password",
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}

	// Import configs from multiple vendors
	vendors := []string{"Cisco", "Palo Alto", "Fortinet", "Arista", "Aruba"}

	for _, vendor := range vendors {
		fmt.Printf("\n=== Importing %s Configurations ===\n", vendor)

		request := ImportRequest{
			Vendors:    []string{vendor},
			ConfigType: "Running",
			Limit:      20,
		}

		summary, err := service.ImportConfigs(request)
		if err != nil {
			log.Printf("Failed to import %s configs: %v", vendor, err)
			continue
		}

		fmt.Printf("Imported %d %s configs\n", summary.SuccessfulImports, vendor)

		// Process each vendor's configs
		for _, cfg := range summary.ProcessedConfigs {
			fmt.Printf("  - %s (%s) - %s\n", cfg.NodeName, cfg.NodeIP, cfg.DetectedOS)
		}
	}
}

// ExampleFilteredImport demonstrates filtering by node names and config type
func ExampleFilteredImport() {
	config := NCMConfig{
		BaseURL:  "https://solarwinds.example.com:17778",
		Username: "admin",
		Password: "password",
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}

	// Import configs for specific sites/locations
	request := ImportRequest{
		NodeNames: []string{
			"core-switch",      // Matches any node with "core-switch" in name
			"firewall",         // Matches any node with "firewall" in name
			"datacenter",       // Matches any node with "datacenter" in name
		},
		ConfigType: "Running",
		Limit:      100,
	}

	summary, err := service.ImportConfigs(request)
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	fmt.Println("=== Filtered Import Results ===")
	fmt.Println(summary.FormatSummary())

	// Group by vendor
	fmt.Println("\n=== Configurations by Vendor ===")
	vendorGroups := make(map[VendorType][]ProcessedConfig)
	for _, cfg := range summary.ProcessedConfigs {
		vendorGroups[cfg.Vendor] = append(vendorGroups[cfg.Vendor], cfg)
	}

	for vendor, configs := range vendorGroups {
		fmt.Printf("\n%s (%d configs):\n", vendor, len(configs))
		for _, cfg := range configs {
			fmt.Printf("  - %s (%s)\n", cfg.NodeName, cfg.NodeIP)
		}
	}
}

// ExampleErrorHandling demonstrates error handling and retry logic
func ExampleErrorHandling() {
	config := NCMConfig{
		BaseURL:  "https://solarwinds.example.com:17778",
		Username: "admin",
		Password: "password",
		Timeout:  30 * time.Second,
	}

	service, err := NewImportService(config)
	if err != nil {
		log.Fatalf("Failed to create import service: %v", err)
	}

	request := ImportRequest{
		Vendors: []string{"Cisco"},
		Limit:   100,
	}

	// Attempt import with retry logic
	maxRetries := 3
	var summary *ImportSummary

	for attempt := 1; attempt <= maxRetries; attempt++ {
		fmt.Printf("Import attempt %d/%d...\n", attempt, maxRetries)

		summary, err = service.ImportConfigs(request)
		if err == nil {
			break
		}

		if attempt < maxRetries {
			fmt.Printf("Import failed: %v. Retrying in 5 seconds...\n", err)
			time.Sleep(5 * time.Second)
		} else {
			log.Fatalf("Import failed after %d attempts: %v", maxRetries, err)
		}
	}

	// Check for partial failures
	if summary.FailedImports > 0 {
		fmt.Printf("\nWarning: %d/%d configs failed to import\n",
			summary.FailedImports, summary.TotalConfigs)

		fmt.Println("\nFailed configs:")
		for _, err := range summary.Errors {
			fmt.Printf("  - Node %d (%s): %s\n", err.NodeID, err.NodeName, err.Error)
		}
	}

	fmt.Printf("\nSuccessfully imported %d configs\n", summary.SuccessfulImports)
}
