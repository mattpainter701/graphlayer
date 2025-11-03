package examples

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/integration"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/orchestrator"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/services"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/types"
)

// EnterpriseWorkflowExample demonstrates the complete enterprise workflow
// from configuration collection to parsing and persistence
func EnterpriseWorkflowExample() {
	ctx := context.Background()

	// 1. Initialize enterprise services
	fmt.Println("=== Initializing Enterprise Services ===")
	
	credService, err := services.NewCredentialService("your-32-byte-encryption-key-here")
	if err != nil {
		log.Fatalf("Failed to create credential service: %v", err)
	}
	fmt.Println("✓ Credential service initialized")

	taskService := services.NewTaskService()
	fmt.Println("✓ Task service initialized")

	// 2. Create orchestrator with working directory
	fmt.Println("\n=== Creating Connector Orchestrator ===")
	orch, err := orchestrator.NewConnectorOrchestrator(
		"/mount/connector", // Base directory for file storage
		credService,
		taskService,
	)
	if err != nil {
		log.Fatalf("Failed to create orchestrator: %v", err)
	}
	fmt.Println("✓ Orchestrator created with working directory: /mount/connector")

	// 3. Create connector job for a Cisco IOS device
	fmt.Println("\n=== Configuring Connector Job ===")
	job := &orchestrator.ConnectorJob{
		GUID:          "conn-cisco-core-sw-01",
		Name:          "Core Switch 01",
		ConnectorType: "ssh",
		DeviceType:    types.DeviceTypeCiscoIOS,
		Config: &base.ConnectionConfig{
			Host:       "10.1.1.1",
			Port:       22,
			Username:   "admin",
			Password:   "password",
			Timeout:    30 * time.Second,
			RetryCount: 3,
			RetryDelay: 2 * time.Second,
		},
		Devices:        []string{"core-sw-01"},
		Workspaces:     []string{"production"},
		TaskGUID:       "task-manual-collect-001",
		Enabled:        true,
		AuxDataEnabled: true, // Collect auxiliary data (ARP, MAC tables, etc.)
	}
	fmt.Printf("✓ Job configured: %s (%s)\n", job.Name, job.DeviceType)
	fmt.Printf("  - Target: %s:%d\n", job.Config.Host, job.Config.Port)
	fmt.Printf("  - Workspaces: %v\n", job.Workspaces)
	fmt.Printf("  - Auxiliary data: %v\n", job.AuxDataEnabled)

	// 4. Create task for tracking
	fmt.Println("\n=== Creating Task ===")
	task := taskService.CreateTask(job.TaskGUID, false) // false = manual task
	fmt.Printf("✓ Task created: %s (manual collection)\n", task.GUID)

	// 5. Execute pull operation (collect configurations)
	fmt.Println("\n=== Executing Configuration Collection ===")
	fmt.Println("Connecting to device and retrieving configurations...")
	
	startTime := time.Now()
	result, err := orch.ExecutePull(ctx, job)
	if err != nil {
		log.Printf("Pull operation failed: %v", err)
	}

	if result.Success {
		fmt.Printf("✓ Successfully collected %d files in %v\n", 
			len(result.ConfigFiles), result.Duration)
		
		fmt.Println("\nCollected files:")
		for i, file := range result.ConfigFiles {
			fmt.Printf("  %d. %s\n", i+1, file)
		}
	} else {
		fmt.Println("✗ Configuration collection had errors:")
		for _, errMsg := range result.Errors {
			fmt.Printf("  - %s\n", errMsg)
		}
	}

	if len(result.Errors) > 0 {
		fmt.Println("\nWarnings/Errors encountered:")
		for _, errMsg := range result.Errors {
			fmt.Printf("  ⚠ %s\n", errMsg)
		}
	}

	// 6. Determine files to commit to workspace
	fmt.Println("\n=== Determining Files to Commit ===")
	workspace := "production"
	filesToCommit, err := orch.GetFilesToCommit(job, workspace)
	if err != nil {
		log.Printf("Failed to get files to commit: %v", err)
	} else {
		fmt.Printf("✓ %d files to commit to workspace '%s'\n", 
			len(filesToCommit), workspace)
		
		if len(filesToCommit) < len(result.ConfigFiles) {
			fmt.Printf("  (Filtered %d unchanged files based on MD5 hash)\n", 
				len(result.ConfigFiles)-len(filesToCommit))
		}
	}

	// 7. Parse configurations with your existing parsers
	fmt.Println("\n=== Parsing Configurations ===")
	
	// Create parser bridge (integrates with your existing parsers)
	parserBridge := integration.NewParserBridge(orch)
	
	fmt.Println("Parsing files through Cisco IOS processor...")
	for i, file := range filesToCommit {
		fmt.Printf("  [%d/%d] Parsing: %s\n", i+1, len(filesToCommit), file)
		
		// This would integrate with your actual parser:
		// processor := cisco.NewCiscoIOSProcessor(
		//     file,
		//     store,
		//     messageBroker,
		//     logger,
		//     messageHelper,
		// )
		// err := processor.ProcessIOSFile()
		
		// For now, simulate parsing
		if err := parserBridge.ParseFiles(ctx, []string{file}); err != nil {
			fmt.Printf("    ⚠ Parse warning: %v\n", err)
		} else {
			fmt.Printf("    ✓ Successfully parsed\n")
		}
	}

	// 8. Track committed files
	fmt.Println("\n=== Tracking Committed Files ===")
	if err := orch.TrackCommittedFiles(job, workspace, filesToCommit); err != nil {
		log.Printf("Failed to track committed files: %v", err)
	} else {
		fmt.Println("✓ Files tracked successfully")
		fmt.Println("  - MD5 hashes stored for change detection")
		fmt.Println("  - Device associations recorded")
	}

	// 9. Update task status
	fmt.Println("\n=== Updating Task Status ===")
	if err := taskService.UpdateTaskStatus(job.TaskGUID, "completed"); err != nil {
		log.Printf("Failed to update task status: %v", err)
	} else {
		fmt.Println("✓ Task marked as completed")
	}

	// 10. Summary
	fmt.Println("\n=== Workflow Summary ===")
	fmt.Printf("Total execution time: %v\n", time.Since(startTime))
	fmt.Printf("Files collected: %d\n", len(result.ConfigFiles))
	fmt.Printf("Files committed: %d\n", len(filesToCommit))
	fmt.Printf("Working directory: %s\n", orch.GetStorage().GetWorkingFolder(job.GUID))
	fmt.Println("\n✓ Enterprise workflow completed successfully")
}

// MultiDeviceCollectionExample demonstrates collecting from multiple devices
func MultiDeviceCollectionExample() {
	ctx := context.Background()

	fmt.Println("=== Multi-Device Collection Example ===\n")

	// Initialize services
	credService, _ := services.NewCredentialService("your-32-byte-encryption-key-here")
	taskService := services.NewTaskService()
	orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)

	// Define multiple devices
	devices := []struct {
		Name string
		IP   string
		Type types.DeviceType
	}{
		{"Core Switch 01", "10.1.1.1", types.DeviceTypeCiscoIOS},
		{"Core Switch 02", "10.1.1.2", types.DeviceTypeCiscoIOS},
		{"Distribution Switch 01", "10.1.2.1", types.DeviceTypeCiscoIOS},
		{"Firewall 01", "10.1.1.10", types.DeviceTypeCiscoASA},
	}

	fmt.Printf("Collecting configurations from %d devices\n\n", len(devices))

	successCount := 0
	failCount := 0

	for i, device := range devices {
		fmt.Printf("[%d/%d] Processing %s (%s)...\n", i+1, len(devices), device.Name, device.IP)

		job := &orchestrator.ConnectorJob{
			GUID:       fmt.Sprintf("conn-%s", device.Name),
			Name:       device.Name,
			DeviceType: device.Type,
			Config: &base.ConnectionConfig{
				Host:       device.IP,
				Port:       22,
				Username:   "admin",
				Password:   "password",
				Timeout:    30 * time.Second,
				RetryCount: 3,
			},
			Devices:        []string{device.Name},
			Workspaces:     []string{"production"},
			TaskGUID:       fmt.Sprintf("task-%s", device.Name),
			Enabled:        true,
			AuxDataEnabled: true,
		}

		// Create task
		taskService.CreateTask(job.TaskGUID, true) // true = scheduled task

		// Execute collection
		result, err := orch.ExecutePull(ctx, job)
		if err != nil || !result.Success {
			fmt.Printf("  ✗ Collection failed\n")
			failCount++
			continue
		}

		// Commit files
		filesToCommit, _ := orch.GetFilesToCommit(job, "production")
		if len(filesToCommit) > 0 {
			orch.TrackCommittedFiles(job, "production", filesToCommit)
			fmt.Printf("  ✓ Collected and committed %d files\n", len(filesToCommit))
			successCount++
		} else {
			fmt.Printf("  ○ No changes detected (configs unchanged)\n")
			successCount++
		}

		taskService.UpdateTaskStatus(job.TaskGUID, "completed")
	}

	fmt.Printf("\n=== Collection Summary ===\n")
	fmt.Printf("Total devices: %d\n", len(devices))
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", failCount)
	fmt.Printf("Success rate: %.1f%%\n", float64(successCount)/float64(len(devices))*100)
}

// ScheduledCollectionExample demonstrates scheduled/automated collection
func ScheduledCollectionExample() {
	ctx := context.Background()

	fmt.Println("=== Scheduled Collection Example ===\n")
	fmt.Println("This demonstrates automated collection with change detection\n")

	// Initialize services
	credService, _ := services.NewCredentialService("your-32-byte-encryption-key-here")
	taskService := services.NewTaskService()
	orch, _ := orchestrator.NewConnectorOrchestrator("/mount/connector", credService, taskService)

	// Create scheduled job
	job := &orchestrator.ConnectorJob{
		GUID:          "conn-scheduled-core-sw-01",
		Name:          "Scheduled Collection: Core SW 01",
		DeviceType:    types.DeviceTypeCiscoIOS,
		Config: &base.ConnectionConfig{
			Host:       "10.1.1.1",
			Port:       22,
			Username:   "admin",
			Password:   "password",
			Timeout:    30 * time.Second,
			RetryCount: 3,
		},
		Devices:        []string{"core-sw-01"},
		Workspaces:     []string{"production"},
		TaskGUID:       "task-scheduled-001",
		Enabled:        true,
		AuxDataEnabled: true,
	}

	// Mark as scheduled task
	task := taskService.CreateTask(job.TaskGUID, true) // true = scheduled
	fmt.Printf("Created scheduled task: %s\n", task.GUID)
	fmt.Println("Mode: Automatic change detection enabled\n")

	// Simulate multiple runs
	for run := 1; run <= 3; run++ {
		fmt.Printf("--- Run #%d (simulating scheduled execution) ---\n", run)

		result, _ := orch.ExecutePull(ctx, job)
		
		if result.Success {
			fmt.Printf("✓ Collection successful: %d files retrieved\n", len(result.ConfigFiles))

			// For scheduled tasks, only commit if changed
			filesToCommit, _ := orch.GetFilesToCommit(job, "production")
			
			if len(filesToCommit) > 0 {
				orch.TrackCommittedFiles(job, "production", filesToCommit)
				fmt.Printf("✓ Changes detected: %d files committed\n", len(filesToCommit))
				fmt.Println("  → Parsers will process updated configurations")
			} else {
				fmt.Println("○ No changes detected (MD5 hashes match)")
				fmt.Println("  → Skipping parsing (optimization)")
			}
		}

		taskService.UpdateTaskStatus(job.TaskGUID, "completed")
		fmt.Println()

		// Simulate time between runs
		if run < 3 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	fmt.Println("=== Key Benefits ===")
	fmt.Println("✓ Change detection prevents unnecessary parsing")
	fmt.Println("✓ MD5 hashing ensures only modified configs are processed")
	fmt.Println("✓ Reduces database writes and processing overhead")
	fmt.Println("✓ Maintains audit trail of configuration changes")
}

