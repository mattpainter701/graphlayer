package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/registry"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/storage"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/types"
)

// ConnectorJob represents a job configuration
type ConnectorJob struct {
	GUID                    string                   `json:"guid"`
	Name                    string                   `json:"name"`
	ConnectorType           string                   `json:"connector_type"`
	DeviceType              types.DeviceType         `json:"device_type"`
	Config                  *base.ConnectionConfig   `json:"config"`
	Devices                 []string                 `json:"devices"`
	Workspaces              []string                 `json:"workspaces"`
	TaskGUID                string                   `json:"task_guid"`
	Enabled                 bool                     `json:"enabled"`
	AuxDataEnabled          bool                     `json:"aux_data_enabled"`
	ConditionalExecution    bool                     `json:"conditional_execution"`     // Enable smart command skipping
	CapabilityDetection     bool                     `json:"capability_detection"`      // Enable capability detection
}

// DeviceCapabilities tracks detected device capabilities and feature states
type DeviceCapabilities struct {
	DeviceID          string                 `json:"device_id"`
	DetectedAt        time.Time              `json:"detected_at"`
	IOSVersion        string                 `json:"ios_version"`
	Features          map[string]bool        `json:"features"`          // feature_name -> enabled
	SupportedCommands map[string]bool        `json:"supported_commands"` // command_name -> supported
	Metadata          map[string]string      `json:"metadata"`          // additional info
}

// CollectionResult represents the result of a collection operation
type CollectionResult struct {
	ConnectorGUID      string                    `json:"connector_guid"`
	DeviceID           string                    `json:"device_id"`
	Success            bool                      `json:"success"`
	ConfigFiles        []string                  `json:"config_files"`
	Errors             []string                  `json:"errors"`
	Duration           time.Duration             `json:"duration"`
	AuxCommandResults  []CommandExecutionResult  `json:"aux_command_results,omitempty"`
	AuxCommandsSummary *CommandExecutionSummary  `json:"aux_commands_summary,omitempty"`
}

// CommandExecutionResult represents the result of a single command execution
type CommandExecutionResult struct {
	CommandName   string                 `json:"command_name"`
	Command       string                 `json:"command"`
	Success       bool                   `json:"success"`
	FailureReason string                 `json:"failure_reason,omitempty"`
	FailureType   CommandFailureType     `json:"failure_type,omitempty"`
	OutputFile    string                 `json:"output_file,omitempty"`
	Duration      time.Duration          `json:"duration"`
	Timestamp     time.Time              `json:"timestamp"`
	Priority      string                 `json:"priority,omitempty"`
	Category      types.CommandCategory  `json:"category,omitempty"`
	Skipped       bool                   `json:"skipped"`
	SkipReason    string                 `json:"skip_reason,omitempty"`
}

// CommandFailureType categorizes command failure reasons
type CommandFailureType string

const (
	FailureTypeUnsupported      CommandFailureType = "unsupported"       // Command not available on this IOS version
	FailureTypeFeatureDisabled  CommandFailureType = "feature_disabled"  // Feature exists but is disabled
	FailureTypeInvalidSyntax    CommandFailureType = "invalid_syntax"    // Command syntax error
	FailureTypeTimeout          CommandFailureType = "timeout"           // Command timed out
	FailureTypePermissionDenied CommandFailureType = "permission_denied" // Insufficient privileges
	FailureTypeDeviceError      CommandFailureType = "device_error"      // Device returned an error
	FailureTypeNetworkError     CommandFailureType = "network_error"     // Network connectivity issue
	FailureTypeUnknown          CommandFailureType = "unknown"           // Unknown failure
)

// CommandExecutionSummary provides aggregate statistics for command execution
type CommandExecutionSummary struct {
	TotalCommands      int                           `json:"total_commands"`
	SuccessfulCommands int                           `json:"successful_commands"`
	FailedCommands     int                           `json:"failed_commands"`
	SkippedCommands    int                           `json:"skipped_commands"`
	ByPriority         map[string]CommandStats       `json:"by_priority"`
	ByFailureType      map[CommandFailureType]int    `json:"by_failure_type"`
	ExecutionTime      time.Duration                 `json:"execution_time"`
}

// CommandStats tracks statistics for a group of commands
type CommandStats struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
}

// ConnectorOrchestrator orchestrates connector operations
type ConnectorOrchestrator struct {
	manager            *registry.ConnectorManager
	storage            *storage.WorkspaceManager
	credService        CredentialService
	taskService        TaskService
	capabilityCache    map[string]*DeviceCapabilities // deviceID -> capabilities
	mu                 sync.RWMutex
	capabilityCacheMu  sync.RWMutex                   // Separate lock for capability cache
}

// CredentialService interface for credential retrieval
type CredentialService interface {
	GetCredential(key string, decrypt bool) (*Credential, error)
}

// TaskService interface for task tracking
type TaskService interface {
	GetTask(taskGUID string) (*Task, error)
	UpdateTaskStatus(taskGUID, status string) error
}

// Credential represents a decrypted credential
type Credential struct {
	CredentialType string                 `json:"credential_type"`
	Payload        map[string]interface{} `json:"payload"`
}

// Task represents a task
type Task struct {
	GUID      string `json:"guid"`
	Scheduled bool   `json:"scheduled"`
	Status    string `json:"status"`
}

// NewConnectorOrchestrator creates a new orchestrator
func NewConnectorOrchestrator(baseDir string, credService CredentialService, taskService TaskService) (*ConnectorOrchestrator, error) {
	storageManager, err := storage.NewWorkspaceManager(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage manager: %w", err)
	}

	return &ConnectorOrchestrator{
		manager:         registry.NewConnectorManager(),
		storage:         storageManager,
		credService:     credService,
		taskService:     taskService,
		capabilityCache: make(map[string]*DeviceCapabilities),
	}, nil
}

// ExecutePull performs a pull operation (download configurations)
func (co *ConnectorOrchestrator) ExecutePull(ctx context.Context, job *ConnectorJob) (*CollectionResult, error) {
	co.mu.Lock()
	defer co.mu.Unlock()

	start := time.Now()
	result := &CollectionResult{
		ConnectorGUID: job.GUID,
		ConfigFiles:   []string{},
		Errors:        []string{},
	}

	// Create working folder
	workingFolder, err := co.storage.CreateWorkingFolder(job.GUID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to create working folder: %v", err))
		result.Duration = time.Since(start)
		return result, err
	}

	// Create connector
	connector, err := co.manager.CreateAndRegisterConnector(job.GUID, job.DeviceType, job.Config)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to create connector: %v", err))
		result.Duration = time.Since(start)
		return result, err
	}

	// Connect to device
	if err := connector.Connect(ctx); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to connect: %v", err))
		result.Duration = time.Since(start)
		return result, err
	}
	defer connector.Disconnect(ctx)

	// Get running configuration
	configResult, err := connector.GetRunningConfig(ctx, 60*time.Second)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to get running config: %v", err))
	} else {
		// Write config to file
		filePath, hash, err := co.storage.WriteConfigFile(
			job.GUID,
			job.Config.Host,
			"running_config",
			configResult.ConfigData,
		)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to write config file: %v", err))
		} else {
			result.ConfigFiles = append(result.ConfigFiles, filePath)
			result.Success = true
			_ = hash // Hash can be used for tracking
		}
	}

	// Get startup configuration if supported
	caps := types.GetDeviceCapabilities(job.DeviceType)
	if caps.CanGetStartupConfig {
		startupResult, err := connector.GetStartupConfig(ctx, 60*time.Second)
		if err == nil {
			filePath, _, err := co.storage.WriteConfigFile(
				job.GUID,
				job.Config.Host,
				"startup_config",
				startupResult.ConfigData,
			)
			if err == nil {
				result.ConfigFiles = append(result.ConfigFiles, filePath)
			}
		}
	}

	// Execute auxiliary commands if enabled
	if job.AuxDataEnabled {
		auxFiles, auxErrors, auxResults, auxSummary := co.executeAuxiliaryCommands(ctx, connector, job)
		result.ConfigFiles = append(result.ConfigFiles, auxFiles...)
		result.Errors = append(result.Errors, auxErrors...)
		result.AuxCommandResults = auxResults
		result.AuxCommandsSummary = auxSummary
	}

	result.Duration = time.Since(start)
	return result, nil
}

// GetFilesToCommit returns files that should be committed to a workspace
func (co *ConnectorOrchestrator) GetFilesToCommit(job *ConnectorJob, workspace string) ([]string, error) {
	files, err := co.storage.GetDownloadedFiles(job.GUID)
	if err != nil {
		return nil, err
	}

	// Check if task is scheduled
	task, err := co.taskService.GetTask(job.TaskGUID)
	if err != nil {
		// If task service unavailable, commit all files
		return files, nil
	}

	if !task.Scheduled {
		// Manual tasks commit all files
		return files, nil
	}

	// For scheduled tasks, only commit changed files
	var filesToCommit []string
	for _, file := range files {
		shouldCommit, err := co.storage.ShouldCommitFile(file, workspace, job.Devices)
		if err != nil {
			continue
		}
		if shouldCommit {
			filesToCommit = append(filesToCommit, file)
		}
	}

	return filesToCommit, nil
}

// TrackCommittedFiles marks files as committed
func (co *ConnectorOrchestrator) TrackCommittedFiles(job *ConnectorJob, workspace string, files []string) error {
	for _, file := range files {
		if err := co.storage.TrackCommittedFile(file, workspace, job.Config.Host, "config"); err != nil {
			return err
		}
	}
	return nil
}

// executeAuxiliaryCommands executes additional diagnostic commands with enhanced error handling and conditional execution
func (co *ConnectorOrchestrator) executeAuxiliaryCommands(
	ctx context.Context,
	connector base.Connector,
	job *ConnectorJob,
) ([]string, []string, []CommandExecutionResult, *CommandExecutionSummary) {
	var files []string
	var errors []string
	var cmdResults []CommandExecutionResult
	
	startTime := time.Now()
	
	// Get auxiliary commands based on device type
	auxCommands := getAuxiliaryCommands(job.DeviceType)
	
	// Initialize summary
	summary := &CommandExecutionSummary{
		TotalCommands: len(auxCommands),
		ByPriority:    make(map[string]CommandStats),
		ByFailureType: make(map[CommandFailureType]int),
	}
	
	// Detect device capabilities if enabled
	var capabilities *DeviceCapabilities
	if job.CapabilityDetection || job.ConditionalExecution {
		capabilities = co.detectDeviceCapabilities(ctx, connector, job.Config.Host)
	}

	for _, cmd := range auxCommands {
		cmdStart := time.Now()
		priority := cmd.Tags["priority"]
		if priority == "" {
			priority = "p2" // Default to lowest priority
		}
		
		cmdResult := CommandExecutionResult{
			CommandName: cmd.Name,
			Command:     cmd.Command,
			Timestamp:   cmdStart,
			Priority:    priority,
			Category:    cmd.Category,
		}
		
		// Check if we should skip this command based on capabilities
		if job.ConditionalExecution && capabilities != nil {
			shouldSkip, skipReason := shouldSkipCommand(cmd, capabilities)
			if shouldSkip {
				cmdResult.Skipped = true
				cmdResult.SkipReason = skipReason
				cmdResult.Duration = time.Since(cmdStart)
				errors = append(errors, fmt.Sprintf("[INFO] Command '%s' skipped: %s", cmd.Name, skipReason))
				summary.SkippedCommands++
				
				// Update priority-based stats
				stats := summary.ByPriority[priority]
				stats.Total++
				stats.Skipped++
				summary.ByPriority[priority] = stats
				
				cmdResults = append(cmdResults, cmdResult)
				continue
			}
		}
		
		// Execute the command
		result, err := connector.ExecuteCommand(ctx, cmd.Command, cmd.Timeout)
		cmdResult.Duration = time.Since(cmdStart)
		
		if err != nil {
			// Command execution failed - analyze the error
			cmdResult.Success = false
			cmdResult.FailureType, cmdResult.FailureReason = categorizeCommandError(err, result)
			
			// Log based on failure type
			if cmdResult.FailureType == FailureTypeUnsupported || cmdResult.FailureType == FailureTypeFeatureDisabled {
				// Non-critical failures - log as info
				errors = append(errors, fmt.Sprintf("[INFO] Command '%s' not available: %s", cmd.Name, cmdResult.FailureReason))
			} else {
				// Critical failures - log as error
				errors = append(errors, fmt.Sprintf("[ERROR] Command '%s' failed: %s", cmd.Name, cmdResult.FailureReason))
			}
			
			summary.FailedCommands++
			summary.ByFailureType[cmdResult.FailureType]++
		} else {
			// Check if output indicates an error condition
			if isCommandOutputError(result.Output) {
				cmdResult.Success = false
				cmdResult.FailureType, cmdResult.FailureReason = categorizeOutputError(result.Output)
				
				if cmdResult.FailureType == FailureTypeUnsupported || cmdResult.FailureType == FailureTypeFeatureDisabled {
					errors = append(errors, fmt.Sprintf("[INFO] Command '%s' not supported: %s", cmd.Name, cmdResult.FailureReason))
				} else {
					errors = append(errors, fmt.Sprintf("[WARN] Command '%s' returned error: %s", cmd.Name, cmdResult.FailureReason))
				}
				
				summary.FailedCommands++
				summary.ByFailureType[cmdResult.FailureType]++
			} else {
				// Success - write the output file
				filePath, _, writeErr := co.storage.WriteConfigFile(
					job.GUID,
					job.Config.Host,
					cmd.Name,
					result.Output,
				)
				
				if writeErr != nil {
					cmdResult.Success = false
					cmdResult.FailureType = FailureTypeUnknown
					cmdResult.FailureReason = fmt.Sprintf("Failed to write output file: %v", writeErr)
					errors = append(errors, fmt.Sprintf("[ERROR] Failed to write aux file '%s': %v", cmd.Name, writeErr))
					summary.FailedCommands++
					summary.ByFailureType[FailureTypeUnknown]++
				} else {
					cmdResult.Success = true
					cmdResult.OutputFile = filePath
					files = append(files, filePath)
					summary.SuccessfulCommands++
				}
			}
		}
		
		// Update priority-based stats
		stats := summary.ByPriority[priority]
		stats.Total++
		if cmdResult.Success {
			stats.Successful++
		} else if cmdResult.Skipped {
			stats.Skipped++
		} else {
			stats.Failed++
		}
		summary.ByPriority[priority] = stats
		
		cmdResults = append(cmdResults, cmdResult)
	}
	
	summary.ExecutionTime = time.Since(startTime)
	
	return files, errors, cmdResults, summary
}

// getAuxiliaryCommands returns auxiliary commands for a device type
func getAuxiliaryCommands(deviceType types.DeviceType) []types.CommandTemplate {
	switch deviceType {
	case types.DeviceTypeCiscoIOS:
		return getCiscoIOSAuxCommands()
	case types.DeviceTypeCiscoASA:
		return getCiscoASAAuxCommands()
	case types.DeviceTypeCiscoNXOS:
		return getCiscoNXOSAuxCommands()
	default:
		return []types.CommandTemplate{}
	}
}

// getCiscoIOSAuxCommands returns comprehensive Cisco IOS operational commands
// Organized by priority: P0 (Critical for L2 topology), P1 (Security), P2 (System info)
func getCiscoIOSAuxCommands() []types.CommandTemplate {
	return []types.CommandTemplate{
		// === P0 CRITICAL: L2 Topology & Adjacency ===
		{
			Name:        "show_mac_address_table_dynamic",
			Description: "MAC-to-port bindings for L2 adjacency and FDB",
			Category:    types.CommandCategorySecurity,
			Command:     "show mac address-table dynamic",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"l2": "true", "fdb": "true", "priority": "p0"},
		},
		{
			Name:        "show_lldp_neighbors_detail",
			Description: "LLDP neighbor discovery for topology stitching",
			Category:    types.CommandCategoryOperational,
			Command:     "show lldp neighbors detail",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"topology": "true", "discovery": "true", "priority": "p0"},
		},
		{
			Name:        "show_cdp_neighbors_detail",
			Description: "CDP neighbor discovery (fallback if LLDP disabled)",
			Category:    types.CommandCategoryOperational,
			Command:     "show cdp neighbors detail",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"topology": "true", "discovery": "true", "priority": "p0"},
		},
		{
			Name:        "show_spanning_tree_detail",
			Description: "STP roles, costs, and blocked ports for attack path analysis",
			Category:    types.CommandCategorySecurity,
			Command:     "show spanning-tree vlan 1-4094 detail",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"l2": "true", "stp": "true", "spof": "true", "priority": "p0"},
		},
		{
			Name:        "show_interfaces_switchport",
			Description: "Switchport mode verification (access/trunk/routed)",
			Category:    types.CommandCategoryOperational,
			Command:     "show interfaces switchport",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"interfaces": "true", "switchport": "true", "priority": "p0"},
		},

		// === P0 CRITICAL: Interface & VLAN State ===
		{
			Name:        "show_interfaces_status",
			Description: "Operational interface status and speed/duplex",
			Category:    types.CommandCategoryOperational,
			Command:     "show interfaces status",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"interfaces": "true", "priority": "p0"},
		},
		{
			Name:        "show_interfaces_trunk",
			Description: "Trunk configuration and allowed VLANs",
			Category:    types.CommandCategoryOperational,
			Command:     "show interfaces trunk",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"trunks": "true", "vlans": "true", "priority": "p0"},
		},
		{
			Name:        "show_vlan",
			Description: "Complete VLAN configuration and port assignments",
			Category:    types.CommandCategoryOperational,
			Command:     "show vlan",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"vlans": "true", "priority": "p0"},
		},
		{
			Name:        "show_etherchannel_summary",
			Description: "Port-channel LAG membership and status",
			Category:    types.CommandCategoryOperational,
			Command:     "show etherchannel summary",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"lag": "true", "etherchannel": "true", "priority": "p0"},
		},

		// === P0 CRITICAL: L3 Handoff & ARP ===
		{
			Name:        "show_ip_arp",
			Description: "ARP table for IP-to-MAC binding and SVI correlation",
			Category:    types.CommandCategoryOperational,
			Command:     "show ip arp",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"l3": "true", "arp": "true", "priority": "p0"},
		},
		{
			Name:        "show_ip_interface_brief",
			Description: "IP interface summary for L3 handoff detection",
			Category:    types.CommandCategoryOperational,
			Command:     "show ip interface brief",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"l3": "true", "interfaces": "true", "priority": "p0"},
		},

		// === P1 HIGH: Security Controls ===
		{
			Name:        "show_ip_dhcp_snooping_binding",
			Description: "DHCP snooping bindings for IP-MAC ground truth",
			Category:    types.CommandCategorySecurity,
			Command:     "show ip dhcp snooping binding",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"security": "true", "dhcp": "true", "priority": "p1"},
		},
		{
			Name:        "show_ip_arp_inspection",
			Description: "Dynamic ARP Inspection status and violations",
			Category:    types.CommandCategorySecurity,
			Command:     "show ip arp inspection",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"security": "true", "dai": "true", "priority": "p1"},
		},
		{
			Name:        "show_port_security",
			Description: "Port security violations and MAC limits",
			Category:    types.CommandCategorySecurity,
			Command:     "show port-security",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"security": "true", "port-security": "true", "priority": "p1"},
		},
		{
			Name:        "show_ip_igmp_snooping_groups",
			Description: "IGMP snooping multicast groups for ICS protocol containment",
			Category:    types.CommandCategorySecurity,
			Command:     "show ip igmp snooping groups",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"security": "true", "multicast": "true", "ics": "true", "priority": "p1"},
		},
		{
			Name:        "show_authentication_sessions",
			Description: "802.1X and MAB authentication session status",
			Category:    types.CommandCategorySecurity,
			Command:     "show authentication sessions",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"security": "true", "dot1x": "true", "mab": "true", "priority": "p1"},
		},
		{
			Name:        "show_ip_verify_source",
			Description: "IP Source Guard binding status",
			Category:    types.CommandCategorySecurity,
			Command:     "show ip verify source",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"security": "true", "ipsg": "true", "priority": "p1"},
		},

		// === P2 MEDIUM: System Information ===
		{
			Name:        "show_version",
			Description: "System version and hardware information",
			Category:    types.CommandCategoryDiagnostic,
			Command:     "show version",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"system": "true", "priority": "p2"},
		},
	}
}

// getCiscoASAAuxCommands returns Cisco ASA operational commands
func getCiscoASAAuxCommands() []types.CommandTemplate {
	return []types.CommandTemplate{
		{
			Name:        "show_interface",
			Description: "Interface status and configuration",
			Category:    types.CommandCategoryOperational,
			Command:     "show interface",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"interfaces": "true", "priority": "p0"},
		},
		{
			Name:        "show_route",
			Description: "Routing table",
			Category:    types.CommandCategoryOperational,
			Command:     "show route",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"routing": "true", "priority": "p0"},
		},
		{
			Name:        "show_xlate",
			Description: "NAT/PAT translations",
			Category:    types.CommandCategorySecurity,
			Command:     "show xlate",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"nat": "true", "priority": "p0"},
		},
		{
			Name:        "show_arp",
			Description: "ARP table for IP-to-MAC binding",
			Category:    types.CommandCategoryOperational,
			Command:     "show arp",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"arp": "true", "priority": "p0"},
		},
		{
			Name:        "show_nat",
			Description: "NAT configuration summary",
			Category:    types.CommandCategorySecurity,
			Command:     "show nat",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"nat": "true", "priority": "p0"},
		},
		{
			Name:        "show_conn",
			Description: "Active connections for traffic flow analysis",
			Category:    types.CommandCategorySecurity,
			Command:     "show conn",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"connections": "true", "priority": "p0"},
		},
		{
			Name:        "show_local_host",
			Description: "Hosts behind ASA with their connections",
			Category:    types.CommandCategoryOperational,
			Command:     "show local-host",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"hosts": "true", "priority": "p0"},
		},
		{
			Name:        "show_nameif",
			Description: "Interface name mappings and security zones",
			Category:    types.CommandCategoryOperational,
			Command:     "show nameif",
			Timeout:     5 * time.Second,
			Tags:        map[string]string{"zones": "true", "priority": "p0"},
		},
		{
			Name:        "show_access_list",
			Description: "Access control lists with hit counts",
			Category:    types.CommandCategorySecurity,
			Command:     "show access-list",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"acl": "true", "priority": "p1"},
		},
		{
			Name:        "show_service_policy",
			Description: "Service policies and inspection statistics",
			Category:    types.CommandCategorySecurity,
			Command:     "show service-policy",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"policy": "true", "priority": "p1"},
		},
		{
			Name:        "show_failover",
			Description: "Failover/HA status",
			Category:    types.CommandCategoryOperational,
			Command:     "show failover",
			Timeout:     5 * time.Second,
			Tags:        map[string]string{"ha": "true", "priority": "p1"},
		},
		{
			Name:        "show_vpn_sessiondb",
			Description: "VPN sessions and tunnels",
			Category:    types.CommandCategorySecurity,
			Command:     "show vpn-sessiondb",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"vpn": "true", "priority": "p1"},
		},
		{
			Name:        "show_object_group",
			Description: "Object groups (network, service, protocol)",
			Category:    types.CommandCategoryOperational,
			Command:     "show object-group",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"objects": "true", "priority": "p1"},
		},
	}
}

// getCiscoNXOSAuxCommands returns comprehensive Cisco NXOS operational commands
// Organized by priority: P0 (Critical for L2 topology), P1 (Advanced features)
func getCiscoNXOSAuxCommands() []types.CommandTemplate {
	return []types.CommandTemplate{
		// === P0 CRITICAL: L2 Topology & MAC Table ===
		{
			Name:        "show_mac_address_table",
			Description: "MAC-to-port bindings for L2 adjacency and FDB",
			Category:    types.CommandCategoryOperational,
			Command:     "show mac address-table",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"l2": "true", "fdb": "true", "priority": "p0"},
		},
		{
			Name:        "show_vlan",
			Description: "VLAN operational state and port assignments",
			Category:    types.CommandCategoryOperational,
			Command:     "show vlan",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"vlans": "true", "priority": "p0"},
		},
		{
			Name:        "show_interface_status",
			Description: "Operational interface status (speed, duplex, state)",
			Category:    types.CommandCategoryOperational,
			Command:     "show interface status",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"interfaces": "true", "priority": "p0"},
		},
		{
			Name:        "show_interface_switchport",
			Description: "Switchport mode and VLAN assignment",
			Category:    types.CommandCategoryOperational,
			Command:     "show interface switchport",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"switchport": "true", "priority": "p0"},
		},
		{
			Name:        "show_interface_trunk",
			Description: "Trunk configuration and allowed VLANs",
			Category:    types.CommandCategoryOperational,
			Command:     "show interface trunk",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"trunks": "true", "priority": "p0"},
		},
		{
			Name:        "show_port_channel_summary",
			Description: "Port-channel (LAG) membership and status",
			Category:    types.CommandCategoryOperational,
			Command:     "show port-channel summary",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"lag": "true", "etherchannel": "true", "priority": "p0"},
		},
		{
			Name:        "show_spanning_tree_summary",
			Description: "STP topology summary and root bridge info",
			Category:    types.CommandCategoryOperational,
			Command:     "show spanning-tree summary",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"stp": "true", "priority": "p0"},
		},
		{
			Name:        "show_lldp_neighbors_detail",
			Description: "LLDP neighbor discovery for topology stitching",
			Category:    types.CommandCategoryOperational,
			Command:     "show lldp neighbors detail",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"topology": "true", "discovery": "true", "priority": "p0"},
		},
		{
			Name:        "show_cdp_neighbors_detail",
			Description: "CDP neighbor discovery (fallback if LLDP disabled)",
			Category:    types.CommandCategoryOperational,
			Command:     "show cdp neighbors detail",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"topology": "true", "discovery": "true", "priority": "p0"},
		},
		{
			Name:        "show_ip_arp",
			Description: "ARP table for IP-to-MAC correlation",
			Category:    types.CommandCategoryOperational,
			Command:     "show ip arp",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"arp": "true", "priority": "p0"},
		},
		{
			Name:        "show_ip_route",
			Description: "Routing table for L3 topology",
			Category:    types.CommandCategoryOperational,
			Command:     "show ip route",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"routing": "true", "priority": "p0"},
		},

		// === P0 CRITICAL: L2 Security Controls ===
		{
			Name:        "show_ip_dhcp_snooping_binding",
			Description: "DHCP snooping bindings for host placement confidence scoring",
			Category:    types.CommandCategorySecurity,
			Command:     "show ip dhcp snooping binding",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"security": "true", "dhcp": "true", "l2": "true", "priority": "p0", "confidence_scoring": "true"},
		},
		{
			Name:        "show_ip_arp_inspection_statistics",
			Description: "Dynamic ARP Inspection statistics and operational state",
			Category:    types.CommandCategorySecurity,
			Command:     "show ip arp inspection statistics",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"security": "true", "dai": "true", "l2": "true", "priority": "p0"},
		},
		{
			Name:        "show_authentication_sessions",
			Description: "802.1X and MAB authentication session status for Allowed vs Conditional decisions",
			Category:    types.CommandCategorySecurity,
			Command:     "show authentication sessions",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"security": "true", "dot1x": "true", "mab": "true", "l2": "true", "priority": "p0", "decision_critical": "true"},
		},
		{
			Name:        "show_port_security",
			Description: "Port security status and violations for MAC limiting control",
			Category:    types.CommandCategorySecurity,
			Command:     "show port-security",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"security": "true", "port-security": "true", "l2": "true", "priority": "p0"},
		},
		{
			Name:        "show_port_security_interface",
			Description: "Per-interface port security details",
			Category:    types.CommandCategorySecurity,
			Command:     "show port-security interface",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"security": "true", "port-security": "true", "l2": "true", "priority": "p0"},
		},
		{
			Name:        "show_ip_verify_source",
			Description: "IP Source Guard bindings and enforcement status",
			Category:    types.CommandCategorySecurity,
			Command:     "show ip verify source",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"security": "true", "ipsg": "true", "l2": "true", "priority": "p0"},
		},

		// === P0 CRITICAL: VPC (Virtual Port Channel) ===
		{
			Name:        "show_vpc",
			Description: "VPC domain status and peer information (CRITICAL for dual-homed topology)",
			Category:    types.CommandCategoryOperational,
			Command:     "show vpc",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"vpc": "true", "ha": "true", "priority": "p0"},
		},
		{
			Name:        "show_vpc_peer_keepalive",
			Description: "VPC peer keepalive health and status",
			Category:    types.CommandCategoryOperational,
			Command:     "show vpc peer-keepalive",
			Timeout:     5 * time.Second,
			Tags:        map[string]string{"vpc": "true", "ha": "true", "priority": "p0"},
		},

		// === P1 HIGH: Advanced Features ===
		{
			Name:        "show_vpc_consistency_parameters",
			Description: "VPC consistency parameter validation",
			Category:    types.CommandCategoryOperational,
			Command:     "show vpc consistency-parameters global",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"vpc": "true", "priority": "p1"},
		},
		{
			Name:        "show_vdc",
			Description: "Virtual Device Context information (Nexus 7K)",
			Category:    types.CommandCategoryOperational,
			Command:     "show vdc",
			Timeout:     5 * time.Second,
			Tags:        map[string]string{"vdc": "true", "priority": "p1"},
		},
		{
			Name:        "show_fabricpath_topology",
			Description: "FabricPath IS-IS topology (if FabricPath enabled)",
			Category:    types.CommandCategoryOperational,
			Command:     "show fabricpath topology",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"fabricpath": "true", "topology": "true", "priority": "p1"},
		},
		{
			Name:        "show_hsrp",
			Description: "HSRP (Hot Standby Router Protocol) status",
			Category:    types.CommandCategoryOperational,
			Command:     "show hsrp",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"hsrp": "true", "ha": "true", "priority": "p1"},
		},
		{
			Name:        "show_vlan_private_vlan",
			Description: "Private VLAN mappings for isolation modeling",
			Category:    types.CommandCategorySecurity,
			Command:     "show vlan private-vlan",
			Timeout:     10 * time.Second,
			Tags:        map[string]string{"security": "true", "pvlan": "true", "l2": "true", "priority": "p1"},
		},
	}
}

// getPaloAltoAuxCommands returns auxiliary commands for Palo Alto PAN-OS firewalls
// PAN-OS uses XML API for configuration and operational data collection
func getPaloAltoAuxCommands() []types.CommandTemplate {
	return []types.CommandTemplate{
		// ========================================
		// CONFIGURATION COMMANDS (XML API)
		// ========================================

		// === P0 CRITICAL: Full Configuration Export ===
		{
			Name:        "show_config_full",
			Description: "Full PAN-OS configuration export (XML)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config",
			Timeout:     60 * time.Second,
			Tags:        map[string]string{"config": "true", "xml": "true", "priority": "p0", "critical": "true"},
		},

		// === P0 CRITICAL: Virtual Systems Configuration ===
		{
			Name:        "show_config_vsys",
			Description: "Virtual systems (vsys) configuration",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "vsys": "true", "xml": "true", "priority": "p0"},
		},

		// === P0 CRITICAL: Network Configuration ===
		{
			Name:        "show_config_network",
			Description: "Network configuration (interfaces, zones, VLANs, virtual-routers)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/network",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "network": "true", "xml": "true", "priority": "p0"},
		},

		// === NOTE: Per-vsys commands are now generated dynamically ===
		// The show_config_vsys command above discovers all virtual systems
		// Then getDynamicPANOSVsysCommands() generates commands for each vsys
		// This replaces the hardcoded vsys1 commands from Phase 3

		// ========================================
		// OPERATIONAL COMMANDS (XML API)
		// ========================================

		// === P1: System Information ===
		{
			Name:        "show_system_info",
			Description: "System information (hostname, version, serial, uptime)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><system><info></info></system></show>",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "system": "true", "xml": "true", "priority": "p1"},
		},

		// === P1: Interface Status ===
		{
			Name:        "show_interface_all",
			Description: "All interface operational status",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><interface>all</interface></show>",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"operational": "true", "interface": "true", "xml": "true", "priority": "p1"},
		},

		// === P1: Routing Table ===
		{
			Name:        "show_routing_route",
			Description: "Routing table for all virtual routers",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><routing><route></route></routing></show>",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"operational": "true", "routing": "true", "xml": "true", "priority": "p1"},
		},

		// === P1: ARP Table ===
		{
			Name:        "show_arp_all",
			Description: "ARP table for all interfaces",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><arp><entry name='all'/></arp></show>",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"operational": "true", "arp": "true", "xml": "true", "priority": "p1"},
		},

		// === P1: High Availability Status ===
		{
			Name:        "show_high_availability_all",
			Description: "HA status (active/passive state, peer health)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><high-availability>all</high-availability></show>",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "ha": "true", "xml": "true", "priority": "p1"},
		},

		// === P2: Active Sessions ===
		{
			Name:        "show_session_all",
			Description: "Active sessions (for policy validation and traffic analysis)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><session>all</session></show>",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"operational": "true", "session": "true", "xml": "true", "priority": "p2"},
		},

		// ========================================
		// VPN TUNNEL MAPPING (Sprint 3.1.4)
		// ========================================

		// === P1: IPsec Tunnel Configuration ===
		{
			Name:        "show_config_ipsec_tunnels",
			Description: "IPsec tunnel configuration (tunnel interface, IKE gateway, crypto profile)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/network/tunnel/ipsec",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"config": "true", "vpn": "true", "ipsec": "true", "xml": "true", "priority": "p1"},
		},

		// === P1: IKE Gateway Configuration ===
		{
			Name:        "show_config_ike_gateways",
			Description: "IKE gateway configuration (peer IP, authentication, crypto profile)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/network/ike/gateway",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"config": "true", "vpn": "true", "ike": "true", "xml": "true", "priority": "p1"},
		},

		// === P1: IPsec Tunnel Status (Operational) ===
		{
			Name:        "show_vpn_ipsec_tunnel",
			Description: "IPsec VPN tunnel operational status (state, bytes in/out, peer IP)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><vpn><ipsec-tunnel></ipsec-tunnel></vpn></show>",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "vpn": "true", "ipsec": "true", "xml": "true", "priority": "p1"},
		},

		// === P1: IKE SA Status ===
		{
			Name:        "show_vpn_ike_sa",
			Description: "IKE Security Association status (phase 1)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><vpn><ike-sa></ike-sa></vpn></show>",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "vpn": "true", "ike": "true", "xml": "true", "priority": "p1"},
		},

		// === P2: GlobalProtect SSL VPN Users ===
		{
			Name:        "show_globalprotect_current_users",
			Description: "GlobalProtect SSL VPN current users (username, IP, login time)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><global-protect-gateway><current-user></current-user></global-protect-gateway></show>",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"operational": "true", "vpn": "true", "globalprotect": "true", "ssl": "true", "xml": "true", "priority": "p2"},
		},

		// ========================================
		// BGP/OSPF ROUTING PROTOCOLS (Sprint 3.1.3)
		// ========================================

		// === P2: BGP Summary ===
		{
			Name:        "show_routing_protocol_bgp_summary",
			Description: "BGP routing protocol summary (router ID, AS, peer count)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><routing><protocol><bgp><summary></summary></bgp></protocol></routing></show>",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "routing": "true", "bgp": "true", "xml": "true", "priority": "p2"},
		},

		// === P2: BGP Neighbors ===
		{
			Name:        "show_routing_protocol_bgp_peer",
			Description: "BGP peer status (state, prefixes received/sent, uptime)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><routing><protocol><bgp><peer></peer></bgp></protocol></routing></show>",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"operational": "true", "routing": "true", "bgp": "true", "neighbors": "true", "xml": "true", "priority": "p2"},
		},

		// === P2: BGP RIB In (Received Routes) ===
		{
			Name:        "show_routing_protocol_bgp_rib_in",
			Description: "BGP routes received from peers (RIB-in)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><routing><protocol><bgp><rib-in></rib-in></bgp></protocol></routing></show>",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"operational": "true", "routing": "true", "bgp": "true", "rib": "true", "xml": "true", "priority": "p2"},
		},

		// === P2: BGP RIB Out (Advertised Routes) ===
		{
			Name:        "show_routing_protocol_bgp_rib_out",
			Description: "BGP routes advertised to peers (RIB-out)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><routing><protocol><bgp><rib-out></rib-out></bgp></protocol></routing></show>",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"operational": "true", "routing": "true", "bgp": "true", "rib": "true", "xml": "true", "priority": "p2"},
		},

		// === P2: OSPF Neighbors ===
		{
			Name:        "show_routing_protocol_ospf_neighbor",
			Description: "OSPF neighbor adjacency status",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><routing><protocol><ospf><neighbor></neighbor></ospf></protocol></routing></show>",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "routing": "true", "ospf": "true", "neighbors": "true", "xml": "true", "priority": "p2"},
		},

		// === P2: OSPF Database (LSAs) ===
		{
			Name:        "show_routing_protocol_ospf_database",
			Description: "OSPF link-state database (LSAs)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><routing><protocol><ospf><database></database></ospf></protocol></routing></show>",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"operational": "true", "routing": "true", "ospf": "true", "database": "true", "xml": "true", "priority": "p2"},
		},

		// === P2: OSPF RIB (OSPF Routes) ===
		{
			Name:        "show_routing_protocol_ospf_rib",
			Description: "OSPF routing information base (learned routes)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><routing><protocol><ospf><rib></rib></ospf></protocol></routing></show>",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"operational": "true", "routing": "true", "ospf": "true", "rib": "true", "xml": "true", "priority": "p2"},
		},

		// ========================================
		// PANORAMA-SPECIFIC COMMANDS (if applicable)
		// ========================================

		// === P0: Managed Devices (Panorama only) ===
		{
			Name:        "show_config_devices_panorama",
			Description: "Panorama managed devices list (Panorama mode only)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/devices",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "panorama": "true", "devices": "true", "xml": "true", "priority": "p0", "panorama_only": "true"},
		},

		// === P0: Shared Objects (Panorama only) ===
		{
			Name:        "show_config_shared_objects_panorama",
			Description: "Shared objects across device groups (Panorama mode only)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/shared",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "panorama": "true", "shared": "true", "xml": "true", "priority": "p0", "panorama_only": "true"},
		},
	}
}

// getPanoramaAuxCommands returns auxiliary commands for Palo Alto Panorama centralized management
// Panorama manages multiple PAN-OS firewalls with device groups, templates, and policy packages
func getPanoramaAuxCommands() []types.CommandTemplate {
	return []types.CommandTemplate{
		// ========================================
		// CONFIGURATION COMMANDS (XML API)
		// ========================================

		// === P0 CRITICAL: Device Groups ===
		{
			Name:        "show_config_device_groups",
			Description: "Device groups for policy inheritance (Panorama)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/device-group",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "panorama": "true", "device-groups": "true", "xml": "true", "priority": "p0"},
		},

		// === P0 CRITICAL: Templates ===
		{
			Name:        "show_config_templates",
			Description: "Network and device configuration templates (Panorama)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/template",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "panorama": "true", "templates": "true", "xml": "true", "priority": "p0"},
		},

		// === P0 CRITICAL: Managed Devices ===
		{
			Name:        "show_config_managed_devices",
			Description: "Firewalls managed by Panorama",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/panorama/managed-devices",
			Timeout:     45 * time.Second,
			Tags:        map[string]string{"config": "true", "panorama": "true", "managed-devices": "true", "xml": "true", "priority": "p0"},
		},

		// === P0: Shared Address Objects ===
		{
			Name:        "show_config_shared_addresses",
			Description: "Shared address objects across all device groups",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/shared/address",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "panorama": "true", "shared": "true", "address": "true", "xml": "true", "priority": "p0"},
		},

		// === P0: Shared Service Objects ===
		{
			Name:        "show_config_shared_services",
			Description: "Shared service objects across all device groups",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/shared/service",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "panorama": "true", "shared": "true", "service": "true", "xml": "true", "priority": "p0"},
		},

		// === P0: Device Group Pre-Rules ===
		// Note: This is a template command. Actual command requires device-group name substitution
		{
			Name:        "show_config_device_group_pre_rules",
			Description: "Pre-rules for device group (applied before local firewall rules)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/device-group/entry[@name='{device-group}']/pre-rulebase/security/rules",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "panorama": "true", "device-group": "true", "pre-rules": "true", "xml": "true", "priority": "p0", "requires_substitution": "true"},
		},

		// ========================================
		// OPERATIONAL COMMANDS (XML API)
		// ========================================

		// === P1: Connected Devices ===
		{
			Name:        "show_devices_connected",
			Description: "Currently connected managed devices",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><devices><connected></connected></devices></show>",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"operational": "true", "panorama": "true", "devices": "true", "xml": "true", "priority": "p1"},
		},

		// === P1: Panorama HA Status ===
		{
			Name:        "show_panorama_ha_status",
			Description: "Panorama high availability status (active/passive)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><high-availability><state></state></high-availability></show>",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "panorama": "true", "ha": "true", "xml": "true", "priority": "p1"},
		},

		// === P2: Commit Status ===
		{
			Name:        "show_commit_locks",
			Description: "Pending commits and commit locks",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><commit-locks></commit-locks></show>",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "panorama": "true", "commit": "true", "xml": "true", "priority": "p2"},
		},

		// === P1: Device Health Summary ===
		{
			Name:        "show_devices_all",
			Description: "Health status of all managed devices",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/?type=op&cmd=<show><devices><all></all></devices></show>",
			Timeout:     45 * time.Second,
			Tags:        map[string]string{"operational": "true", "panorama": "true", "devices": "true", "health": "true", "xml": "true", "priority": "p1"},
		},
	}
}

// getDynamicPANOSVsysCommands generates per-vsys commands dynamically for multi-vsys firewalls
// This replaces the hardcoded vsys1 commands from Phase 3 with dynamic vsys iteration
// Phase 3.1.2: Dynamic Vsys Discovery
func getDynamicPANOSVsysCommands(vsysList []string) []types.CommandTemplate {
	var commands []types.CommandTemplate

	// For each discovered vsys, generate the full set of per-vsys commands
	for _, vsys := range vsysList {
		// === Security Policies ===
		commands = append(commands, types.CommandTemplate{
			Name:        fmt.Sprintf("show_config_security_policies_%s", vsys),
			Description: fmt.Sprintf("Security policies for %s", vsys),
			Category:    types.CommandCategoryConfiguration,
			Command:     fmt.Sprintf("/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='%s']/rulebase/security", vsys),
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "security": "true", "policies": "true", "xml": "true", "priority": "p0", "vsys": vsys, "dynamic": "true"},
		})

		// === NAT Policies ===
		commands = append(commands, types.CommandTemplate{
			Name:        fmt.Sprintf("show_config_nat_policies_%s", vsys),
			Description: fmt.Sprintf("NAT policies for %s", vsys),
			Category:    types.CommandCategoryConfiguration,
			Command:     fmt.Sprintf("/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='%s']/rulebase/nat", vsys),
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "nat": "true", "policies": "true", "xml": "true", "priority": "p0", "vsys": vsys, "dynamic": "true"},
		})

		// === Address Objects ===
		commands = append(commands, types.CommandTemplate{
			Name:        fmt.Sprintf("show_config_address_objects_%s", vsys),
			Description: fmt.Sprintf("Address and address-group objects for %s", vsys),
			Category:    types.CommandCategoryConfiguration,
			Command:     fmt.Sprintf("/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='%s']/address", vsys),
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"config": "true", "address": "true", "objects": "true", "xml": "true", "priority": "p0", "vsys": vsys, "dynamic": "true"},
		})

		// === Service Objects ===
		commands = append(commands, types.CommandTemplate{
			Name:        fmt.Sprintf("show_config_service_objects_%s", vsys),
			Description: fmt.Sprintf("Service and service-group objects for %s", vsys),
			Category:    types.CommandCategoryConfiguration,
			Command:     fmt.Sprintf("/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='%s']/service", vsys),
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"config": "true", "service": "true", "objects": "true", "xml": "true", "priority": "p0", "vsys": vsys, "dynamic": "true"},
		})

		// === Security Zones ===
		commands = append(commands, types.CommandTemplate{
			Name:        fmt.Sprintf("show_config_zones_%s", vsys),
			Description: fmt.Sprintf("Security zones configuration for %s", vsys),
			Category:    types.CommandCategoryConfiguration,
			Command:     fmt.Sprintf("/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='%s']/zone", vsys),
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "zone": "true", "xml": "true", "priority": "p0", "vsys": vsys, "dynamic": "true"},
		})

		// ========================================
		// SECURITY PROFILES (Sprint 3.1.5)
		// ========================================

		// === URL Filtering Profiles ===
		commands = append(commands, types.CommandTemplate{
			Name:        fmt.Sprintf("show_config_url_filtering_%s", vsys),
			Description: fmt.Sprintf("URL filtering profiles for %s", vsys),
			Category:    types.CommandCategoryConfiguration,
			Command:     fmt.Sprintf("/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='%s']/profiles/url-filtering", vsys),
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "security": "true", "url-filtering": "true", "xml": "true", "priority": "p2", "vsys": vsys, "dynamic": "true"},
		})

		// === Antivirus Profiles ===
		commands = append(commands, types.CommandTemplate{
			Name:        fmt.Sprintf("show_config_antivirus_%s", vsys),
			Description: fmt.Sprintf("Antivirus profiles for %s", vsys),
			Category:    types.CommandCategoryConfiguration,
			Command:     fmt.Sprintf("/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='%s']/profiles/virus", vsys),
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "security": "true", "antivirus": "true", "xml": "true", "priority": "p2", "vsys": vsys, "dynamic": "true"},
		})

		// === Anti-Spyware Profiles ===
		commands = append(commands, types.CommandTemplate{
			Name:        fmt.Sprintf("show_config_antispyware_%s", vsys),
			Description: fmt.Sprintf("Anti-spyware profiles for %s", vsys),
			Category:    types.CommandCategoryConfiguration,
			Command:     fmt.Sprintf("/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='%s']/profiles/spyware", vsys),
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "security": "true", "anti-spyware": "true", "xml": "true", "priority": "p2", "vsys": vsys, "dynamic": "true"},
		})

		// === Vulnerability Protection Profiles ===
		commands = append(commands, types.CommandTemplate{
			Name:        fmt.Sprintf("show_config_vulnerability_%s", vsys),
			Description: fmt.Sprintf("Vulnerability protection profiles for %s", vsys),
			Category:    types.CommandCategoryConfiguration,
			Command:     fmt.Sprintf("/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='%s']/profiles/vulnerability", vsys),
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "security": "true", "vulnerability": "true", "xml": "true", "priority": "p2", "vsys": vsys, "dynamic": "true"},
		})

		// === File Blocking Profiles ===
		commands = append(commands, types.CommandTemplate{
			Name:        fmt.Sprintf("show_config_file_blocking_%s", vsys),
			Description: fmt.Sprintf("File blocking profiles for %s", vsys),
			Category:    types.CommandCategoryConfiguration,
			Command:     fmt.Sprintf("/api/?type=config&action=show&xpath=/config/devices/entry[@name='localhost.localdomain']/vsys/entry[@name='%s']/profiles/file-blocking", vsys),
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "security": "true", "file-blocking": "true", "xml": "true", "priority": "p2", "vsys": vsys, "dynamic": "true"},
		})
	}

	return commands
}

// getFortiGateAuxCommands returns auxiliary commands for Fortinet FortiGate firewalls
// FortiGate uses REST API (JSON) for configuration and operational data collection
// Phase 4.2: FortiGate Command Implementation
func getFortiGateAuxCommands() []types.CommandTemplate {
	return []types.CommandTemplate{
		// ========================================
		// SYSTEM & VDOM COMMANDS
		// ========================================

		// === P0 CRITICAL: System Status ===
		{
			Name:        "show_system_status",
			Description: "System information (hostname, version, serial, VDOM mode)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/v2/cmdb/system/status",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "system": "true", "json": "true", "priority": "p0"},
		},

		// === P0 CRITICAL: VDOM List ===
		{
			Name:        "show_system_vdom",
			Description: "Virtual Domain (VDOM) list and configuration",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/system/vdom",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "vdom": "true", "json": "true", "priority": "p0"},
		},

		// === P0: Global Configuration ===
		{
			Name:        "show_system_global",
			Description: "Global system configuration",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/system/global",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "system": "true", "json": "true", "priority": "p0"},
		},

		// ========================================
		// NETWORK CONFIGURATION COMMANDS
		// ========================================

		// === P0 CRITICAL: Interfaces ===
		{
			Name:        "show_system_interface",
			Description: "System interfaces configuration (all VDOMs)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/system/interface",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"config": "true", "interface": "true", "json": "true", "priority": "p0"},
		},

		// === P0: Security Zones ===
		{
			Name:        "show_system_zone",
			Description: "Security zones configuration",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/system/zone",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "zone": "true", "json": "true", "priority": "p0"},
		},

		// === P0: Static Routes ===
		{
			Name:        "show_router_static",
			Description: "Static routing table configuration",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/router/static",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "routing": "true", "json": "true", "priority": "p0"},
		},

		// === P1: BGP Configuration ===
		{
			Name:        "show_router_bgp",
			Description: "BGP routing configuration",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/router/bgp",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"config": "true", "routing": "true", "bgp": "true", "json": "true", "priority": "p1"},
		},

		// ========================================
		// SECURITY POLICY COMMANDS
		// ========================================

		// === P0 CRITICAL: IPv4 Security Policies ===
		{
			Name:        "show_firewall_policy",
			Description: "IPv4 security policies (firewall rules)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/firewall/policy",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "policy": "true", "firewall": "true", "json": "true", "priority": "p0"},
		},

		// === P1: IPv6 Security Policies ===
		{
			Name:        "show_firewall_policy6",
			Description: "IPv6 security policies (firewall rules)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/firewall/policy6",
			Timeout:     30 * time.Second,
			Tags:        map[string]string{"config": "true", "policy": "true", "firewall": "true", "ipv6": "true", "json": "true", "priority": "p1"},
		},

		// === P0: Central NAT Policies ===
		{
			Name:        "show_firewall_central_snat",
			Description: "Central Source NAT policies",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/firewall/central-snat-map",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"config": "true", "nat": "true", "snat": "true", "json": "true", "priority": "p0"},
		},

		// === P0: Virtual IPs (DNAT) ===
		{
			Name:        "show_firewall_vip",
			Description: "Virtual IPs (destination NAT)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/firewall/vip",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "nat": "true", "dnat": "true", "vip": "true", "json": "true", "priority": "p0"},
		},

		// === P0: IP Pools (Source NAT) ===
		{
			Name:        "show_firewall_ippool",
			Description: "IP pools for source NAT",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/firewall/ippool",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "nat": "true", "snat": "true", "ippool": "true", "json": "true", "priority": "p0"},
		},

		// ========================================
		// OBJECT COMMANDS
		// ========================================

		// === P0 CRITICAL: Address Objects ===
		{
			Name:        "show_firewall_address",
			Description: "Firewall address objects (IPv4)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/firewall/address",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"config": "true", "address": "true", "objects": "true", "json": "true", "priority": "p0"},
		},

		// === P0: Address Groups ===
		{
			Name:        "show_firewall_addrgrp",
			Description: "Firewall address groups",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/firewall/addrgrp",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"config": "true", "address": "true", "groups": "true", "json": "true", "priority": "p0"},
		},

		// === P0: Service Objects ===
		{
			Name:        "show_firewall_service_custom",
			Description: "Custom service objects (TCP/UDP ports)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/firewall/service/custom",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "service": "true", "objects": "true", "json": "true", "priority": "p0"},
		},

		// === P0: Service Groups ===
		{
			Name:        "show_firewall_service_group",
			Description: "Service groups",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/firewall/service/group",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "service": "true", "groups": "true", "json": "true", "priority": "p0"},
		},

		// ========================================
		// SD-WAN COMMANDS (Phase 4 Specific)
		// ========================================

		// === P1: SD-WAN Configuration ===
		{
			Name:        "show_system_sdwan",
			Description: "SD-WAN configuration (zones, members, health checks)",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/system/sdwan",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"config": "true", "sdwan": "true", "json": "true", "priority": "p1"},
		},

		// === P1: SD-WAN Health Checks ===
		{
			Name:        "show_system_sdwan_health_check",
			Description: "SD-WAN health check configuration",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/system/sdwan/health-check",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "sdwan": "true", "health": "true", "json": "true", "priority": "p1"},
		},

		// ========================================
		// VPN COMMANDS
		// ========================================

		// === P1: IPsec Phase 1 (IKE) ===
		{
			Name:        "show_vpn_ipsec_phase1",
			Description: "IPsec VPN Phase 1 (IKE gateway) configuration",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/vpn.ipsec/phase1-interface",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "vpn": "true", "ipsec": "true", "phase1": "true", "json": "true", "priority": "p1"},
		},

		// === P1: IPsec Phase 2 (Tunnel) ===
		{
			Name:        "show_vpn_ipsec_phase2",
			Description: "IPsec VPN Phase 2 (tunnel) configuration",
			Category:    types.CommandCategoryConfiguration,
			Command:     "/api/v2/cmdb/vpn.ipsec/phase2-interface",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"config": "true", "vpn": "true", "ipsec": "true", "phase2": "true", "json": "true", "priority": "p1"},
		},

		// ========================================
		// OPERATIONAL COMMANDS
		// ========================================

		// === P1: Interface Operational Status ===
		{
			Name:        "show_system_interface_status",
			Description: "Interface operational status and statistics",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/v2/monitor/system/interface",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "interface": "true", "json": "true", "priority": "p1"},
		},

		// === P1: Routing Table ===
		{
			Name:        "show_router_ipv4",
			Description: "IPv4 routing table (operational)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/v2/monitor/router/ipv4",
			Timeout:     20 * time.Second,
			Tags:        map[string]string{"operational": "true", "routing": "true", "ipv4": "true", "json": "true", "priority": "p1"},
		},

		// === P1: BGP Neighbors ===
		{
			Name:        "show_router_bgp_neighbors",
			Description: "BGP neighbor status (operational)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/v2/monitor/router/bgp/neighbors",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "routing": "true", "bgp": "true", "json": "true", "priority": "p1"},
		},

		// === P1: SD-WAN Health Status ===
		{
			Name:        "show_system_sdwan_health_status",
			Description: "SD-WAN health check status (operational)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/v2/monitor/system/sdwan/health-check",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "sdwan": "true", "health": "true", "json": "true", "priority": "p1"},
		},

		// === P1: IPsec VPN Status ===
		{
			Name:        "show_vpn_ipsec_tunnel",
			Description: "IPsec VPN tunnel status (operational)",
			Category:    types.CommandCategoryOperational,
			Command:     "/api/v2/monitor/vpn/ipsec",
			Timeout:     15 * time.Second,
			Tags:        map[string]string{"operational": "true", "vpn": "true", "ipsec": "true", "json": "true", "priority": "p1"},
		},
	}
}

// categorizeCommandError analyzes an error from command execution
func categorizeCommandError(err error, result *base.CommandResult) (CommandFailureType, string) {
	errMsg := err.Error()
	
	// Check for timeout
	if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "timed out") || strings.Contains(errMsg, "deadline exceeded") {
		return FailureTypeTimeout, fmt.Sprintf("Command timed out: %v", err)
	}
	
	// Check for network errors
	if strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "network") {
		return FailureTypeNetworkError, fmt.Sprintf("Network error: %v", err)
	}
	
	// Check for permission errors
	if strings.Contains(errMsg, "permission") || strings.Contains(errMsg, "not authorized") {
		return FailureTypePermissionDenied, fmt.Sprintf("Permission denied: %v", err)
	}
	
	// If we have output, check for error patterns
	if result != nil && result.Output != "" {
		failureType, reason := categorizeOutputError(result.Output)
		if failureType != FailureTypeUnknown {
			return failureType, reason
		}
	}
	
	return FailureTypeUnknown, fmt.Sprintf("Unknown error: %v", err)
}

// categorizeOutputError analyzes command output for error patterns
func categorizeOutputError(output string) (CommandFailureType, string) {
	// Normalize output for case-insensitive matching
	lowerOutput := strings.ToLower(output)
	
	// Check for "Invalid input" - command syntax not recognized
	if strings.Contains(output, "% Invalid input detected") || 
	   strings.Contains(output, "% Unrecognized command") ||
	   strings.Contains(lowerOutput, "invalid command") {
		return FailureTypeUnsupported, "Command not recognized (likely unsupported in this IOS version)"
	}
	
	// Check for "Unknown command" - command doesn't exist
	if strings.Contains(lowerOutput, "unknown command") ||
	   strings.Contains(lowerOutput, "command not found") {
		return FailureTypeUnsupported, "Command not available on this device"
	}
	
	// Check for disabled features - LLDP
	if strings.Contains(output, "LLDP is not enabled") ||
	   strings.Contains(output, "%LLDP is not enabled") {
		return FailureTypeFeatureDisabled, "LLDP is not enabled (configure: 'lldp run')"
	}
	
	// Check for disabled features - CDP
	if strings.Contains(output, "CDP is not enabled") ||
	   strings.Contains(output, "%CDP is not enabled") {
		return FailureTypeFeatureDisabled, "CDP is not enabled (configure: 'cdp run')"
	}
	
	// Check for disabled features - DHCP Snooping
	if strings.Contains(output, "DHCP snooping is configured, but not enabled") ||
	   strings.Contains(lowerOutput, "dhcp snooping is disabled") {
		return FailureTypeFeatureDisabled, "DHCP Snooping is not enabled"
	}
	
	// Check for disabled features - Port Security
	if strings.Contains(output, "Port Security is not enabled") ||
	   strings.Contains(lowerOutput, "no port security") {
		return FailureTypeFeatureDisabled, "Port Security is not enabled"
	}
	
	// Check for disabled features - 802.1X
	if strings.Contains(output, "dot1x system-auth-control is not enabled") ||
	   strings.Contains(lowerOutput, "802.1x is not enabled") {
		return FailureTypeFeatureDisabled, "802.1X is not enabled"
	}
	
	// Check for disabled features - IGMP Snooping
	if strings.Contains(lowerOutput, "igmp snooping is disabled") ||
	   strings.Contains(lowerOutput, "igmp snooping not enabled") {
		return FailureTypeFeatureDisabled, "IGMP Snooping is not enabled"
	}
	
	// Check for empty output (might indicate feature disabled)
	if strings.TrimSpace(output) == "" {
		return FailureTypeFeatureDisabled, "Command returned no output (feature may be disabled)"
	}
	
	// Check for "No entries" or empty tables (not an error, but informational)
	if strings.Contains(lowerOutput, "no entries") ||
	   strings.Contains(lowerOutput, "no active") ||
	   strings.Contains(lowerOutput, "none") {
		// This is actually success with empty results, not an error
		return FailureTypeUnknown, "" // Let caller handle as success
	}
	
	// Check for incomplete command
	if strings.Contains(output, "% Incomplete command") {
		return FailureTypeInvalidSyntax, "Incomplete command syntax"
	}
	
	// Check for ambiguous command
	if strings.Contains(output, "% Ambiguous command") {
		return FailureTypeInvalidSyntax, "Ambiguous command"
	}
	
	// Check for privilege level errors
	if strings.Contains(lowerOutput, "command authorization failed") ||
	   strings.Contains(lowerOutput, "not authorized") ||
	   strings.Contains(output, "% Permission denied") {
		return FailureTypePermissionDenied, "Insufficient privileges"
	}
	
	// Check for device errors
	if strings.Contains(output, "% Error") {
		// Extract error message if possible
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "% Error") {
				return FailureTypeDeviceError, strings.TrimSpace(line)
			}
		}
		return FailureTypeDeviceError, "Device returned an error"
	}
	
	return FailureTypeUnknown, ""
}

// isCommandOutputError determines if command output contains an error
func isCommandOutputError(output string) bool {
	// Check for common error patterns
	errorPatterns := []string{
		"% Invalid input detected",
		"% Unrecognized command",
		"% Unknown command",
		"% Incomplete command",
		"% Ambiguous command",
		"% Permission denied",
		"LLDP is not enabled",
		"CDP is not enabled",
		"not enabled",
	}
	
	lowerOutput := strings.ToLower(output)
	
	for _, pattern := range errorPatterns {
		if strings.Contains(strings.ToLower(pattern), strings.ToLower(lowerOutput)) ||
		   strings.Contains(output, pattern) {
			return true
		}
	}
	
	// Empty output might indicate a problem (but not always)
	if strings.TrimSpace(output) == "" {
		return false // Let caller decide
	}
	
	return false
}

// detectDeviceCapabilities detects device capabilities through lightweight checks
func (co *ConnectorOrchestrator) detectDeviceCapabilities(
	ctx context.Context,
	connector base.Connector,
	deviceID string,
) *DeviceCapabilities {
	// Check cache first
	co.capabilityCacheMu.RLock()
	cached := co.capabilityCache[deviceID]
	co.capabilityCacheMu.RUnlock()
	
	// Return cached if recent (less than 1 hour old)
	if cached != nil && time.Since(cached.DetectedAt) < time.Hour {
		return cached
	}
	
	// Create new capabilities structure
	capabilities := &DeviceCapabilities{
		DeviceID:          deviceID,
		DetectedAt:        time.Now(),
		Features:          make(map[string]bool),
		SupportedCommands: make(map[string]bool),
		Metadata:          make(map[string]string),
	}
	
	// Detect IOS version using lightweight 'show version'
	versionResult, err := connector.ExecuteCommand(ctx, "show version", 10*time.Second)
	if err == nil {
		capabilities.IOSVersion = extractIOSVersion(versionResult.Output)
		capabilities.Metadata["version_output_length"] = fmt.Sprintf("%d", len(versionResult.Output))
	}
	
	// Detect LLDP capability (lightweight check)
	lldpResult, err := connector.ExecuteCommand(ctx, "show lldp", 5*time.Second)
	if err == nil && !isCommandOutputError(lldpResult.Output) {
		if strings.Contains(lldpResult.Output, "LLDP is not enabled") {
			capabilities.Features["lldp"] = false
		} else if !strings.Contains(lldpResult.Output, "% Invalid input") {
			capabilities.Features["lldp"] = true
		}
	}
	
	// Detect CDP capability (lightweight check)
	cdpResult, err := connector.ExecuteCommand(ctx, "show cdp", 5*time.Second)
	if err == nil && !isCommandOutputError(cdpResult.Output) {
		if strings.Contains(cdpResult.Output, "CDP is not enabled") {
			capabilities.Features["cdp"] = false
		} else if !strings.Contains(cdpResult.Output, "% Invalid input") {
			capabilities.Features["cdp"] = true
		}
	}
	
	// Detect DHCP Snooping (check global config)
	dhcpResult, err := connector.ExecuteCommand(ctx, "show ip dhcp snooping", 5*time.Second)
	if err == nil && !isCommandOutputError(dhcpResult.Output) {
		if strings.Contains(strings.ToLower(dhcpResult.Output), "enabled") {
			capabilities.Features["dhcp_snooping"] = true
		} else {
			capabilities.Features["dhcp_snooping"] = false
		}
	}
	
	// Detect Port Security (lightweight check)
	portSecResult, err := connector.ExecuteCommand(ctx, "show port-security", 5*time.Second)
	if err == nil && !isCommandOutputError(portSecResult.Output) {
		if !strings.Contains(portSecResult.Output, "not enabled") {
			capabilities.Features["port_security"] = true
		}
	}
	
	// Detect 802.1X/Authentication
	dot1xResult, err := connector.ExecuteCommand(ctx, "show authentication sessions", 5*time.Second)
	if err == nil && !isCommandOutputError(dot1xResult.Output) {
		if !strings.Contains(dot1xResult.Output, "% Invalid input") {
			capabilities.Features["dot1x"] = true
		}
	}
	
	// Cache the capabilities
	co.capabilityCacheMu.Lock()
	co.capabilityCache[deviceID] = capabilities
	co.capabilityCacheMu.Unlock()
	
	return capabilities
}

// shouldSkipCommand determines if a command should be skipped based on capabilities
func shouldSkipCommand(cmd types.CommandTemplate, capabilities *DeviceCapabilities) (bool, string) {
	// Check for feature-specific commands
	
	// LLDP commands
	if strings.Contains(cmd.Command, "lldp") {
		if enabled, exists := capabilities.Features["lldp"]; exists && !enabled {
			return true, "LLDP is disabled on this device"
		}
	}
	
	// CDP commands
	if strings.Contains(cmd.Command, "cdp") {
		if enabled, exists := capabilities.Features["cdp"]; exists && !enabled {
			return true, "CDP is disabled on this device"
		}
	}
	
	// DHCP Snooping commands
	if strings.Contains(cmd.Command, "dhcp snooping") {
		if enabled, exists := capabilities.Features["dhcp_snooping"]; exists && !enabled {
			return true, "DHCP Snooping is not enabled"
		}
	}
	
	// ARP Inspection commands
	if strings.Contains(cmd.Command, "arp inspection") {
		if enabled, exists := capabilities.Features["dai"]; exists && !enabled {
			return true, "Dynamic ARP Inspection (DAI) is not enabled"
		}
	}
	
	// Port Security commands
	if strings.Contains(cmd.Command, "port-security") || strings.Contains(cmd.Command, "port security") {
		if enabled, exists := capabilities.Features["port_security"]; exists && !enabled {
			return true, "Port Security is not configured"
		}
	}
	
	// Authentication commands (802.1X, MAB)
	if strings.Contains(cmd.Command, "authentication sessions") {
		if enabled, exists := capabilities.Features["dot1x"]; exists && !enabled {
			return true, "802.1X/MAB authentication not available"
		}
	}
	
	// IGMP Snooping commands
	if strings.Contains(cmd.Command, "igmp snooping") {
		if enabled, exists := capabilities.Features["igmp_snooping"]; exists && !enabled {
			return true, "IGMP Snooping is not enabled"
		}
	}
	
	// Check IOS version for command support
	if capabilities.IOSVersion != "" {
		// Example: authentication sessions requires IOS 15.3+
		if strings.Contains(cmd.Command, "authentication sessions") {
			if !isIOSVersionSupported(capabilities.IOSVersion, "15.3") {
				return true, fmt.Sprintf("Command requires IOS 15.3+, device is running %s", capabilities.IOSVersion)
			}
		}
	}
	
	return false, ""
}

// extractIOSVersion extracts IOS version from 'show version' output
func extractIOSVersion(output string) string {
	// Look for common IOS version patterns
	patterns := []string{
		`Version ([0-9]+\.[0-9]+\([0-9]+[A-Za-z]*\)[A-Za-z0-9]*)`,
		`IOS.*Version ([0-9]+\.[0-9]+)`,
		`Software.*Version ([0-9]+\.[0-9]+)`,
	}
	
	for _, pattern := range patterns {
		// Simple pattern matching (in production, use regexp)
		if idx := strings.Index(output, "Version "); idx != -1 {
			start := idx + 8
			end := start
			for end < len(output) && output[end] != '\n' && output[end] != ',' {
				end++
			}
			if end > start {
				version := strings.TrimSpace(output[start:end])
				if len(version) > 0 && len(version) < 50 {
					return version
				}
			}
		}
	}
	
	return "unknown"
}

// isIOSVersionSupported checks if current IOS version meets minimum requirement
func isIOSVersionSupported(currentVersion, minVersion string) bool {
	// Simple version comparison (in production, implement proper semantic versioning)
	// Extract major.minor from versions like "15.2(4)E7" -> "15.2"
	
	current := extractMajorMinor(currentVersion)
	required := extractMajorMinor(minVersion)
	
	return current >= required
}

// extractMajorMinor extracts major.minor version (e.g., "15.2" from "15.2(4)E7")
func extractMajorMinor(version string) string {
	// Find first two numbers separated by dot
	var result []rune
	dotCount := 0
	
	for _, char := range version {
		if char >= '0' && char <= '9' {
			result = append(result, char)
		} else if char == '.' && dotCount == 0 {
			result = append(result, char)
			dotCount++
		} else if dotCount > 0 {
			break
		}
	}
	
	return string(result)
}

// GetDeviceCapabilities returns cached capabilities for a device
func (co *ConnectorOrchestrator) GetDeviceCapabilities(deviceID string) *DeviceCapabilities {
	co.capabilityCacheMu.RLock()
	defer co.capabilityCacheMu.RUnlock()
	return co.capabilityCache[deviceID]
}

// ClearCapabilityCache clears the capability cache for a device or all devices
func (co *ConnectorOrchestrator) ClearCapabilityCache(deviceID string) {
	co.capabilityCacheMu.Lock()
	defer co.capabilityCacheMu.Unlock()
	
	if deviceID == "" {
		// Clear all
		co.capabilityCache = make(map[string]*DeviceCapabilities)
	} else {
		// Clear specific device
		delete(co.capabilityCache, deviceID)
	}
}

// CleanupWorkspace removes working folder for a connector
func (co *ConnectorOrchestrator) CleanupWorkspace(connectorGUID string) error {
	return co.storage.CleanupWorkingFolder(connectorGUID)
}

// GetManager returns the connector manager
func (co *ConnectorOrchestrator) GetManager() *registry.ConnectorManager {
	return co.manager
}

// GetStorage returns the storage manager
func (co *ConnectorOrchestrator) GetStorage() *storage.WorkspaceManager {
	return co.storage
}

