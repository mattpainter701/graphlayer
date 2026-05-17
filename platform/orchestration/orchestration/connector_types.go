package orchestrator

import (
	"time"

	"github.com/mattpainter701/graphlayer/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/orchestration/types"
)

// ConnectorJob represents a job configuration
type ConnectorJob struct {
	GUID                 string                 `json:"guid"`
	Name                 string                 `json:"name"`
	ConnectorType        string                 `json:"connector_type"`
	DeviceType           types.DeviceType       `json:"device_type"`
	Config               *base.ConnectionConfig `json:"config"`
	Devices              []string               `json:"devices"`
	Workspaces           []string               `json:"workspaces"`
	TaskGUID             string                 `json:"task_guid"`
	Enabled              bool                   `json:"enabled"`
	AuxDataEnabled       bool                   `json:"aux_data_enabled"`
	ConditionalExecution bool                   `json:"conditional_execution"` // Enable smart command skipping
	CapabilityDetection  bool                   `json:"capability_detection"`  // Enable capability detection
	CollectAppID         bool                   `json:"collect_app_id"`        // Collect App-ID database (PAN-OS only)
	CollectISDB          bool                   `json:"collect_isdb"`          // Collect ISDB database (FortiGate only)
}

// DeviceCapabilities tracks detected device capabilities and feature states
type DeviceCapabilities struct {
	DeviceID          string            `json:"device_id"`
	DetectedAt        time.Time         `json:"detected_at"`
	IOSVersion        string            `json:"ios_version"`
	Features          map[string]bool   `json:"features"`           // feature_name -> enabled
	SupportedCommands map[string]bool   `json:"supported_commands"` // command_name -> supported
	Metadata          map[string]string `json:"metadata"`           // additional info
}

// CollectionResult represents the result of a collection operation
type CollectionResult struct {
	ConnectorGUID      string                   `json:"connector_guid"`
	DeviceID           string                   `json:"device_id"`
	Success            bool                     `json:"success"`
	ConfigFiles        []string                 `json:"config_files"`
	Errors             []string                 `json:"errors"`
	Duration           time.Duration            `json:"duration"`
	AuxCommandResults  []CommandExecutionResult `json:"aux_command_results,omitempty"`
	AuxCommandsSummary *CommandExecutionSummary `json:"aux_commands_summary,omitempty"`
	AppIDCollected     bool                     `json:"app_id_collected,omitempty"`      // Whether App-ID collection was attempted
	AppIDSuccess       bool                     `json:"app_id_success,omitempty"`        // Whether App-ID collection succeeded
	AppIDCount         int                      `json:"app_id_count,omitempty"`          // Number of App-ID entries collected
	ISDBCollected      bool                     `json:"isdb_collected,omitempty"`        // Whether ISDB collection was attempted
	ISDBSuccess        bool                     `json:"isdb_success,omitempty"`          // Whether ISDB collection succeeded
	ISDBCount          int                      `json:"isdb_count,omitempty"`            // Number of ISDB entries collected
}

// CommandExecutionResult represents the result of a single command execution
type CommandExecutionResult struct {
	CommandName   string                `json:"command_name"`
	Command       string                `json:"command"`
	Success       bool                  `json:"success"`
	FailureReason string                `json:"failure_reason,omitempty"`
	FailureType   CommandFailureType    `json:"failure_type,omitempty"`
	OutputFile    string                `json:"output_file,omitempty"`
	Duration      time.Duration         `json:"duration"`
	Timestamp     time.Time             `json:"timestamp"`
	Priority      string                `json:"priority,omitempty"`
	Category      types.CommandCategory `json:"category,omitempty"`
	Skipped       bool                  `json:"skipped"`
	SkipReason    string                `json:"skip_reason,omitempty"`
}

// CommandFailureType categorizes command failure reasons
type CommandFailureType string

const (
	FailureTypeUnsupported     CommandFailureType = "unsupported"      // Command not available on this IOS version
	FailureTypeFeatureDisabled CommandFailureType = "feature_disabled" // Feature exists but is disabled
	FailureTypeInvalidSyntax   CommandFailureType = "invalid_syntax"   // Command syntax error
	FailureTypeTimeout         CommandFailureType = "timeout"          // Command timed out
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
