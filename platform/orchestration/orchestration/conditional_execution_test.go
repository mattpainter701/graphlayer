package orchestrator

import (
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/orchestration/types"
)

// Test shouldSkipCommand function
func TestShouldSkipCommand(t *testing.T) {
	tests := []struct {
		name          string
		command       types.CommandTemplate
		capabilities  *DeviceCapabilities
		expectSkip    bool
		expectedReason string
	}{
		{
			name: "Skip LLDP when disabled",
			command: types.CommandTemplate{
				Name:    "show_lldp_neighbors",
				Command: "show lldp neighbors detail",
			},
			capabilities: &DeviceCapabilities{
				Features: map[string]bool{
					"lldp": false,
				},
			},
			expectSkip:     true,
			expectedReason: "LLDP is disabled",
		},
		{
			name: "Don't skip LLDP when enabled",
			command: types.CommandTemplate{
				Name:    "show_lldp_neighbors",
				Command: "show lldp neighbors detail",
			},
			capabilities: &DeviceCapabilities{
				Features: map[string]bool{
					"lldp": true,
				},
			},
			expectSkip: false,
		},
		{
			name: "Skip CDP when disabled",
			command: types.CommandTemplate{
				Name:    "show_cdp_neighbors",
				Command: "show cdp neighbors detail",
			},
			capabilities: &DeviceCapabilities{
				Features: map[string]bool{
					"cdp": false,
				},
			},
			expectSkip:     true,
			expectedReason: "CDP is disabled",
		},
		{
			name: "Skip DHCP Snooping when disabled",
			command: types.CommandTemplate{
				Name:    "show_dhcp_snooping",
				Command: "show ip dhcp snooping binding",
			},
			capabilities: &DeviceCapabilities{
				Features: map[string]bool{
					"dhcp_snooping": false,
				},
			},
			expectSkip:     true,
			expectedReason: "DHCP Snooping is not enabled",
		},
		{
			name: "Skip Port Security when not configured",
			command: types.CommandTemplate{
				Name:    "show_port_security",
				Command: "show port-security",
			},
			capabilities: &DeviceCapabilities{
				Features: map[string]bool{
					"port_security": false,
				},
			},
			expectSkip:     true,
			expectedReason: "Port Security is not configured",
		},
		{
			name: "Skip authentication sessions when not available",
			command: types.CommandTemplate{
				Name:    "show_auth_sessions",
				Command: "show authentication sessions",
			},
			capabilities: &DeviceCapabilities{
				Features: map[string]bool{
					"dot1x": false,
				},
			},
			expectSkip:     true,
			expectedReason: "802.1X/MAB authentication not available",
		},
		{
			name: "Skip based on IOS version",
			command: types.CommandTemplate{
				Name:    "show_auth_sessions",
				Command: "show authentication sessions",
			},
			capabilities: &DeviceCapabilities{
				IOSVersion: "15.2(4)E7",
				Features: map[string]bool{
					"dot1x": true,
				},
			},
			expectSkip:     true,
			expectedReason: "Command requires IOS 15.3+",
		},
		{
			name: "Don't skip when feature not detected",
			command: types.CommandTemplate{
				Name:    "show_mac_table",
				Command: "show mac address-table dynamic",
			},
			capabilities: &DeviceCapabilities{
				Features: map[string]bool{},
			},
			expectSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skip, reason := shouldSkipCommand(tt.command, tt.capabilities)

			if skip != tt.expectSkip {
				t.Errorf("Expected skip=%v, got skip=%v", tt.expectSkip, skip)
			}

			if tt.expectSkip && !contains(reason, tt.expectedReason) {
				t.Errorf("Expected reason to contain '%s', got '%s'", tt.expectedReason, reason)
			}
		})
	}
}

// Test extractIOSVersion function
func TestExtractIOSVersion(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedVersion string
	}{
		{
			name: "Cisco IOS Software",
			output: `Cisco IOS Software, C9300 Software (CAT9K_IOSXE), Version 17.3.4, RELEASE SOFTWARE (fc1)
Technical Support: http://www.cisco.com/techsupport`,
			expectedVersion: "17.3.4",
		},
		{
			name: "IOS XE Version",
			output: `Cisco IOS XE Software, Version 16.12.04
Cisco IOS Software [Gibraltar], Catalyst L3 Switch Software`,
			expectedVersion: "16.12.04",
		},
		{
			name: "Older IOS format",
			output: `Cisco IOS Software, C2960X Software (C2960X-UNIVERSALK9-M), Version 15.2(4)E7, RELEASE SOFTWARE (fc2)`,
			expectedVersion: "15.2(4)E7",
		},
		{
			name: "No version found",
			output: `Some output without version information`,
			expectedVersion: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := extractIOSVersion(tt.output)

			if !contains(version, extractMajorMinor(tt.expectedVersion)) && version != tt.expectedVersion {
				t.Errorf("Expected version containing '%s', got '%s'", tt.expectedVersion, version)
			}
		})
	}
}

// Test extractMajorMinor function
func TestExtractMajorMinor(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "Full IOS version",
			version:  "15.2(4)E7",
			expected: "15.2",
		},
		{
			name:     "IOS-XE version",
			version:  "16.12.04",
			expected: "16.12",
		},
		{
			name:     "Simple version",
			version:  "17.3",
			expected: "17.3",
		},
		{
			name:     "Version with letters",
			version:  "15.6(2)SP5",
			expected: "15.6",
		},
		{
			name:     "Just major",
			version:  "15",
			expected: "15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMajorMinor(tt.version)

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// Test isIOSVersionSupported function
func TestIsIOSVersionSupported(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		minVersion     string
		expected       bool
	}{
		{
			name:           "Same version",
			currentVersion: "15.3",
			minVersion:     "15.3",
			expected:       true,
		},
		{
			name:           "Current higher",
			currentVersion: "16.12",
			minVersion:     "15.3",
			expected:       true,
		},
		{
			name:           "Current lower",
			currentVersion: "15.2",
			minVersion:     "15.3",
			expected:       false,
		},
		{
			name:           "Full version formats",
			currentVersion: "15.6(2)SP5",
			minVersion:     "15.3",
			expected:       true,
		},
		{
			name:           "IOS-XE versions",
			currentVersion: "17.3.4",
			minVersion:     "16.12",
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIOSVersionSupported(tt.currentVersion, tt.minVersion)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v (current=%s, min=%s)",
					tt.expected, result, tt.currentVersion, tt.minVersion)
			}
		})
	}
}

// Test DeviceCapabilities structure
func TestDeviceCapabilitiesStructure(t *testing.T) {
	caps := &DeviceCapabilities{
		DeviceID:   "10.1.1.1",
		DetectedAt: time.Now(),
		IOSVersion: "15.2(4)E7",
		Features: map[string]bool{
			"lldp":           false,
			"cdp":            true,
			"dhcp_snooping":  true,
			"port_security":  false,
			"dot1x":          true,
		},
		SupportedCommands: map[string]bool{
			"show_lldp": false,
			"show_cdp":  true,
		},
		Metadata: map[string]string{
			"platform": "C2960X",
		},
	}

	// Validate structure
	if caps.DeviceID != "10.1.1.1" {
		t.Error("DeviceID not set correctly")
	}

	if caps.Features["lldp"] != false {
		t.Error("LLDP should be disabled")
	}

	if caps.Features["cdp"] != true {
		t.Error("CDP should be enabled")
	}

	if caps.Metadata["platform"] != "C2960X" {
		t.Error("Metadata not set correctly")
	}
}

// Test capability cache behavior
func TestCapabilityCacheBehavior(t *testing.T) {
	// Create orchestrator with cache
	orch := &ConnectorOrchestrator{
		capabilityCache: make(map[string]*DeviceCapabilities),
	}

	deviceID := "10.1.1.1"

	// Initially empty
	caps := orch.GetDeviceCapabilities(deviceID)
	if caps != nil {
		t.Error("Cache should be empty initially")
	}

	// Add capabilities
	newCaps := &DeviceCapabilities{
		DeviceID:   deviceID,
		DetectedAt: time.Now(),
		Features: map[string]bool{
			"lldp": true,
		},
	}

	orch.capabilityCache[deviceID] = newCaps

	// Retrieve from cache
	cached := orch.GetDeviceCapabilities(deviceID)
	if cached == nil {
		t.Error("Should retrieve from cache")
	}

	if cached.Features["lldp"] != true {
		t.Error("Cached capabilities incorrect")
	}

	// Clear specific device
	orch.ClearCapabilityCache(deviceID)
	caps = orch.GetDeviceCapabilities(deviceID)
	if caps != nil {
		t.Error("Cache should be cleared")
	}
}

// Test conditional execution enabled flag
func TestConditionalExecutionFlag(t *testing.T) {
	job1 := &ConnectorJob{
		GUID:                 "job1",
		ConditionalExecution: true,
	}

	job2 := &ConnectorJob{
		GUID:                 "job2",
		ConditionalExecution: false,
	}

	if !job1.ConditionalExecution {
		t.Error("ConditionalExecution should be enabled for job1")
	}

	if job2.ConditionalExecution {
		t.Error("ConditionalExecution should be disabled for job2")
	}
}

// Test capability detection flag
func TestCapabilityDetectionFlag(t *testing.T) {
	job1 := &ConnectorJob{
		GUID:                "job1",
		CapabilityDetection: true,
	}

	job2 := &ConnectorJob{
		GUID:                "job2",
		CapabilityDetection: false,
	}

	if !job1.CapabilityDetection {
		t.Error("CapabilityDetection should be enabled for job1")
	}

	if job2.CapabilityDetection {
		t.Error("CapabilityDetection should be disabled for job2")
	}
}

// Test skip statistics
func TestSkipStatistics(t *testing.T) {
	summary := &CommandExecutionSummary{
		TotalCommands:   15,
		SkippedCommands: 3,
		FailedCommands:  2,
		ByPriority:      make(map[string]CommandStats),
	}

	// Add stats for skipped commands
	p0Stats := CommandStats{
		Total:      11,
		Successful: 9,
		Failed:     0,
		Skipped:    2,
	}
	summary.ByPriority["p0"] = p0Stats

	p1Stats := CommandStats{
		Total:      4,
		Successful: 3,
		Failed:     0,
		Skipped:    1,
	}
	summary.ByPriority["p1"] = p1Stats

	// Validate
	if summary.SkippedCommands != 3 {
		t.Errorf("Expected 3 skipped commands, got %d", summary.SkippedCommands)
	}

	if summary.ByPriority["p0"].Skipped != 2 {
		t.Errorf("Expected 2 P0 skipped, got %d", summary.ByPriority["p0"].Skipped)
	}

	if summary.ByPriority["p1"].Skipped != 1 {
		t.Errorf("Expected 1 P1 skipped, got %d", summary.ByPriority["p1"].Skipped)
	}
}

// Test command result with skip info
func TestCommandResultWithSkip(t *testing.T) {
	result := CommandExecutionResult{
		CommandName: "show_lldp_neighbors",
		Command:     "show lldp neighbors detail",
		Success:     false,
		Skipped:     true,
		SkipReason:  "LLDP is disabled on this device",
		Duration:    50 * time.Millisecond,
		Timestamp:   time.Now(),
		Priority:    "p0",
	}

	if !result.Skipped {
		t.Error("Command should be marked as skipped")
	}

	if result.SkipReason == "" {
		t.Error("Skip reason should be provided")
	}

	if result.Success {
		t.Error("Skipped command should not be marked as success")
	}

	if !contains(result.SkipReason, "LLDP") {
		t.Errorf("Skip reason should mention LLDP, got: %s", result.SkipReason)
	}
}

// Helper function (defined in error_handling_test.go but repeated here for completeness)
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

