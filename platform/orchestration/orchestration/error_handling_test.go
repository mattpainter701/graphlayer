package orchestrator

import (
	"errors"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/base"
)

// Test categorizeOutputError function
func TestCategorizeOutputError(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedType   CommandFailureType
		expectedReason string
	}{
		{
			name:           "LLDP not enabled",
			output:         "%LLDP is not enabled",
			expectedType:   FailureTypeFeatureDisabled,
			expectedReason: "LLDP is not enabled (configure: 'lldp run')",
		},
		{
			name:           "CDP not enabled",
			output:         "%CDP is not enabled",
			expectedType:   FailureTypeFeatureDisabled,
			expectedReason: "CDP is not enabled (configure: 'cdp run')",
		},
		{
			name: "Invalid input detected",
			output: `SW1#show lldp neighbors detail
             ^
% Invalid input detected at '^' marker.`,
			expectedType:   FailureTypeUnsupported,
			expectedReason: "Command not recognized (likely unsupported in this IOS version)",
		},
		{
			name:           "Unknown command",
			output:         "Unknown command or computer name, or unable to find computer address",
			expectedType:   FailureTypeUnsupported,
			expectedReason: "Command not available on this device",
		},
		{
			name:           "DHCP Snooping disabled",
			output:         "DHCP snooping is configured, but not enabled",
			expectedType:   FailureTypeFeatureDisabled,
			expectedReason: "DHCP Snooping is not enabled",
		},
		{
			name:           "Port Security not enabled",
			output:         "Port Security is not enabled",
			expectedType:   FailureTypeFeatureDisabled,
			expectedReason: "Port Security is not enabled",
		},
		{
			name:           "802.1X not enabled",
			output:         "dot1x system-auth-control is not enabled",
			expectedType:   FailureTypeFeatureDisabled,
			expectedReason: "802.1X is not enabled",
		},
		{
			name:           "IGMP Snooping disabled",
			output:         "IGMP snooping is disabled on VLAN 1",
			expectedType:   FailureTypeFeatureDisabled,
			expectedReason: "IGMP Snooping is not enabled",
		},
		{
			name:           "Incomplete command",
			output:         "% Incomplete command.",
			expectedType:   FailureTypeInvalidSyntax,
			expectedReason: "Incomplete command syntax",
		},
		{
			name:           "Ambiguous command",
			output:         "% Ambiguous command: \"sh int\"",
			expectedType:   FailureTypeInvalidSyntax,
			expectedReason: "Ambiguous command",
		},
		{
			name:           "Permission denied",
			output:         "% Permission denied for this command",
			expectedType:   FailureTypePermissionDenied,
			expectedReason: "Insufficient privileges",
		},
		{
			name:           "Command authorization failed",
			output:         "Command authorization failed.",
			expectedType:   FailureTypePermissionDenied,
			expectedReason: "Insufficient privileges",
		},
		{
			name:           "Device error",
			output:         "% Error: Failed to retrieve data from hardware",
			expectedType:   FailureTypeDeviceError,
			expectedReason: "% Error: Failed to retrieve data from hardware",
		},
		{
			name:           "Success with empty table",
			output:         "No entries found.",
			expectedType:   FailureTypeUnknown,
			expectedReason: "",
		},
		{
			name:           "Success with no active entries",
			output:         "No active sessions found.",
			expectedType:   FailureTypeUnknown,
			expectedReason: "",
		},
		{
			name:           "Empty output",
			output:         "",
			expectedType:   FailureTypeFeatureDisabled,
			expectedReason: "Command returned no output (feature may be disabled)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failureType, reason := categorizeOutputError(tt.output)

			if failureType != tt.expectedType {
				t.Errorf("Expected failure type %s, got %s", tt.expectedType, failureType)
			}

			if tt.expectedReason != "" && reason != tt.expectedReason {
				t.Errorf("Expected reason '%s', got '%s'", tt.expectedReason, reason)
			}
		})
	}
}

// Test categorizeCommandError function
func TestCategorizeCommandError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		result         *base.CommandResult
		expectedType   CommandFailureType
		shouldContain  string
	}{
		{
			name:          "Timeout error",
			err:           errors.New("command execution timed out"),
			result:        nil,
			expectedType:  FailureTypeTimeout,
			shouldContain: "timed out",
		},
		{
			name:          "Network error",
			err:           errors.New("network connection lost"),
			result:        nil,
			expectedType:  FailureTypeNetworkError,
			shouldContain: "Network error",
		},
		{
			name:          "Permission error",
			err:           errors.New("permission denied"),
			result:        nil,
			expectedType:  FailureTypePermissionDenied,
			shouldContain: "Permission denied",
		},
		{
			name: "Error with output containing LLDP disabled",
			err:  errors.New("command failed"),
			result: &base.CommandResult{
				Output: "%LLDP is not enabled",
			},
			expectedType:  FailureTypeFeatureDisabled,
			shouldContain: "LLDP is not enabled",
		},
		{
			name:          "Unknown error",
			err:           errors.New("something went wrong"),
			result:        nil,
			expectedType:  FailureTypeUnknown,
			shouldContain: "Unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failureType, reason := categorizeCommandError(tt.err, tt.result)

			if failureType != tt.expectedType {
				t.Errorf("Expected failure type %s, got %s", tt.expectedType, failureType)
			}

			if tt.shouldContain != "" && !contains(reason, tt.shouldContain) {
				t.Errorf("Expected reason to contain '%s', got '%s'", tt.shouldContain, reason)
			}
		})
	}
}

// Test isCommandOutputError function
func TestIsCommandOutputError(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{
			name:     "Invalid input",
			output:   "% Invalid input detected",
			expected: true,
		},
		{
			name:     "Unknown command",
			output:   "% Unknown command",
			expected: true,
		},
		{
			name:     "LLDP not enabled",
			output:   "LLDP is not enabled",
			expected: true,
		},
		{
			name:     "CDP not enabled",
			output:   "CDP is not enabled",
			expected: true,
		},
		{
			name:     "Permission denied",
			output:   "% Permission denied",
			expected: true,
		},
		{
			name:     "Success with data",
			output:   "Vlan    Mac Address       Type        Ports\n----    -----------       --------    -----",
			expected: false,
		},
		{
			name:     "Empty output",
			output:   "",
			expected: false,
		},
		{
			name:     "Success with no entries",
			output:   "Total entries: 0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCommandOutputError(tt.output)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v for output: %s", tt.expected, result, tt.output)
			}
		})
	}
}

// Test CommandExecutionSummary aggregation
func TestCommandExecutionSummaryAggregation(t *testing.T) {
	summary := &CommandExecutionSummary{
		TotalCommands: 15,
		ByPriority:    make(map[string]CommandStats),
		ByFailureType: make(map[CommandFailureType]int),
	}

	// Simulate command results
	results := []struct {
		priority string
		success  bool
		failure  CommandFailureType
	}{
		{"p0", true, ""},
		{"p0", true, ""},
		{"p0", false, FailureTypeFeatureDisabled},
		{"p0", false, FailureTypeUnsupported},
		{"p1", true, ""},
		{"p1", false, FailureTypeFeatureDisabled},
		{"p1", false, FailureTypeTimeout},
		{"p2", true, ""},
	}

	for _, r := range results {
		stats := summary.ByPriority[r.priority]
		stats.Total++
		if r.success {
			stats.Successful++
			summary.SuccessfulCommands++
		} else {
			stats.Failed++
			summary.FailedCommands++
			summary.ByFailureType[r.failure]++
		}
		summary.ByPriority[r.priority] = stats
	}

	// Validate aggregations
	if summary.SuccessfulCommands != 4 {
		t.Errorf("Expected 4 successful commands, got %d", summary.SuccessfulCommands)
	}

	if summary.FailedCommands != 4 {
		t.Errorf("Expected 4 failed commands, got %d", summary.FailedCommands)
	}

	// Check P0 stats
	p0Stats := summary.ByPriority["p0"]
	if p0Stats.Total != 4 {
		t.Errorf("Expected 4 P0 commands, got %d", p0Stats.Total)
	}
	if p0Stats.Successful != 2 {
		t.Errorf("Expected 2 successful P0 commands, got %d", p0Stats.Successful)
	}
	if p0Stats.Failed != 2 {
		t.Errorf("Expected 2 failed P0 commands, got %d", p0Stats.Failed)
	}

	// Check failure types
	if summary.ByFailureType[FailureTypeFeatureDisabled] != 2 {
		t.Errorf("Expected 2 feature_disabled failures, got %d", summary.ByFailureType[FailureTypeFeatureDisabled])
	}
	if summary.ByFailureType[FailureTypeUnsupported] != 1 {
		t.Errorf("Expected 1 unsupported failure, got %d", summary.ByFailureType[FailureTypeUnsupported])
	}
	if summary.ByFailureType[FailureTypeTimeout] != 1 {
		t.Errorf("Expected 1 timeout failure, got %d", summary.ByFailureType[FailureTypeTimeout])
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test error severity classification
func TestErrorSeverity(t *testing.T) {
	tests := []struct {
		name          string
		failureType   CommandFailureType
		expectedLevel string
	}{
		{
			name:          "Unsupported is INFO",
			failureType:   FailureTypeUnsupported,
			expectedLevel: "INFO",
		},
		{
			name:          "Feature disabled is INFO",
			failureType:   FailureTypeFeatureDisabled,
			expectedLevel: "INFO",
		},
		{
			name:          "Timeout is ERROR",
			failureType:   FailureTypeTimeout,
			expectedLevel: "ERROR",
		},
		{
			name:          "Permission denied is ERROR",
			failureType:   FailureTypePermissionDenied,
			expectedLevel: "ERROR",
		},
		{
			name:          "Network error is ERROR",
			failureType:   FailureTypeNetworkError,
			expectedLevel: "ERROR",
		},
		{
			name:          "Device error is WARN",
			failureType:   FailureTypeDeviceError,
			expectedLevel: "WARN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := getSeverityLevel(tt.failureType)
			if level != tt.expectedLevel {
				t.Errorf("Expected severity %s for %s, got %s", tt.expectedLevel, tt.failureType, level)
			}
		})
	}
}

// Helper function to determine severity level
func getSeverityLevel(failureType CommandFailureType) string {
	switch failureType {
	case FailureTypeUnsupported, FailureTypeFeatureDisabled:
		return "INFO"
	case FailureTypeTimeout, FailureTypePermissionDenied, FailureTypeNetworkError:
		return "ERROR"
	case FailureTypeDeviceError, FailureTypeInvalidSyntax:
		return "WARN"
	default:
		return "ERROR"
	}
}

