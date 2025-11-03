package cisco

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestComputeSnippetHash tests the evidence hashing function for determinism and normalization.
func TestComputeSnippetHash(t *testing.T) {
	tests := []struct {
		name           string
		input1         []string
		input2         []string
		shouldBeEqual  bool
		description    string
	}{
		{
			name: "Identical inputs produce identical hashes (determinism)",
			input1: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
				" ip dhcp snooping trust",
			},
			input2: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
				" ip dhcp snooping trust",
			},
			shouldBeEqual: true,
			description:   "Same input → same hash (determinism requirement)",
		},
		{
			name: "Different whitespace produces identical hashes (normalization)",
			input1: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				"  switchport access vlan 100",
				"   ip dhcp snooping trust",
			},
			input2: []string{
				"interface GigabitEthernet1/0/1",
				"switchport mode access",
				"\tswitchport access vlan 100",
				"\t\tip dhcp snooping trust",
			},
			shouldBeEqual: true,
			description:   "Different whitespace → same hash (normalization)",
		},
		{
			name: "Different order produces identical hashes (order-independent)",
			input1: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
				" ip dhcp snooping trust",
			},
			input2: []string{
				"interface GigabitEthernet1/0/1",
				" ip dhcp snooping trust",
				" switchport access vlan 100",
				" switchport mode access",
			},
			shouldBeEqual: true,
			description:   "Different command order → same hash (order-independent)",
		},
		{
			name: "Comments are ignored",
			input1: []string{
				"interface GigabitEthernet1/0/1",
				"! This is a comment",
				" switchport mode access",
				"! Another comment",
				" switchport access vlan 100",
			},
			input2: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
			},
			shouldBeEqual: true,
			description:   "Comments are ignored in hash calculation",
		},
		{
			name: "Empty lines are ignored",
			input1: []string{
				"interface GigabitEthernet1/0/1",
				"",
				" switchport mode access",
				"",
				"",
				" switchport access vlan 100",
			},
			input2: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
			},
			shouldBeEqual: true,
			description:   "Empty lines are ignored in hash calculation",
		},
		{
			name: "Case normalization (lowercase)",
			input1: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode ACCESS",
				" SWITCHPORT ACCESS VLAN 100",
			},
			input2: []string{
				"interface gigabitethernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
			},
			shouldBeEqual: true,
			description:   "Case differences are normalized (lowercase)",
		},
		{
			name: "Different content produces different hashes",
			input1: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
			},
			input2: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode trunk",
				" switchport trunk allowed vlan 100",
			},
			shouldBeEqual: false,
			description:   "Different commands → different hash (change detection)",
		},
		{
			name: "VLAN ID change produces different hashes",
			input1: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
			},
			input2: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 200",
			},
			shouldBeEqual: false,
			description:   "VLAN ID change detected (drift detection)",
		},
		{
			name: "Security control addition produces different hashes",
			input1: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
			},
			input2: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
				" ip dhcp snooping trust",
			},
			shouldBeEqual: false,
			description:   "Security control addition detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := computeSnippetHash(tt.input1)
			hash2 := computeSnippetHash(tt.input2)

			if tt.shouldBeEqual {
				assert.Equal(t, hash1, hash2,
					"%s: Hashes should be equal but are different\nHash1: %s\nHash2: %s",
					tt.description, hash1, hash2)
			} else {
				assert.NotEqual(t, hash1, hash2,
					"%s: Hashes should be different but are equal\nHash: %s",
					tt.description, hash1)
			}

			// Verify hash format
			assert.Contains(t, hash1, "sha256:",
				"Hash should have 'sha256:' prefix")
			assert.Greater(t, len(hash1), 10,
				"Hash should be substantial length (SHA-256)")
		})
	}
}

// TestComputeSnippetHashDeterminism tests that the same input always produces the same hash.
func TestComputeSnippetHashDeterminism(t *testing.T) {
	input := []string{
		"interface GigabitEthernet1/0/1",
		" description Access port for user workstations",
		" switchport mode access",
		" switchport access vlan 100",
		" switchport port-security",
		" switchport port-security maximum 2",
		" ip dhcp snooping trust",
		" ip arp inspection trust",
		" ip verify source",
		" authentication port-control auto",
		" mab",
	}

	// Compute hash multiple times
	hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		hashes[i] = computeSnippetHash(input)
	}

	// All hashes should be identical
	for i := 1; i < len(hashes); i++ {
		assert.Equal(t, hashes[0], hashes[i],
			"Hash %d differs from hash 0 (determinism violation)", i)
	}

	t.Logf("Determinism test passed: All 10 hash computations produced: %s", hashes[0])
}

// TestComputeSnippetHashRealWorldScenarios tests real-world config scenarios.
func TestComputeSnippetHashRealWorldScenarios(t *testing.T) {
	tests := []struct {
		name        string
		scenario    string
		config1     []string
		config2     []string
		expectEqual bool
	}{
		{
			name:     "VLAN configuration drift detection",
			scenario: "VLAN name changed from 'Engineering' to 'Engineering_Dept'",
			config1: []string{
				"vlan 100",
				" name Engineering",
			},
			config2: []string{
				"vlan 100",
				" name Engineering_Dept",
			},
			expectEqual: false,
		},
		{
			name:     "Security control added",
			scenario: "DHCP snooping enabled on VLAN",
			config1: []string{
				"vlan 100",
				" name Engineering",
			},
			config2: []string{
				"vlan 100",
				" name Engineering",
				"ip dhcp snooping vlan 100",
			},
			expectEqual: false,
		},
		{
			name:     "Formatting-only change (no drift)",
			scenario: "Admin reformatted config with different spacing",
			config1: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
			},
			config2: []string{
				"interface GigabitEthernet1/0/1",
				"  switchport mode access",
				"   switchport access vlan 100",
			},
			expectEqual: true,
		},
		{
			name:     "Comment-only change (no drift)",
			scenario: "Admin added clarifying comments",
			config1: []string{
				"interface GigabitEthernet1/0/1",
				" switchport mode access",
				" switchport access vlan 100",
			},
			config2: []string{
				"interface GigabitEthernet1/0/1",
				"! Updated 2025-10-31",
				" switchport mode access",
				"! User workstation port",
				" switchport access vlan 100",
			},
			expectEqual: true,
		},
		{
			name:     "Port security violation mode changed",
			scenario: "Changed violation mode from shutdown to restrict",
			config1: []string{
				"interface GigabitEthernet1/0/1",
				" switchport port-security",
				" switchport port-security violation shutdown",
			},
			config2: []string{
				"interface GigabitEthernet1/0/1",
				" switchport port-security",
				" switchport port-security violation restrict",
			},
			expectEqual: false,
		},
		{
			name:     "Trunk allowed VLANs modified",
			scenario: "Added VLAN 200 to trunk allowed list",
			config1: []string{
				"interface GigabitEthernet1/0/24",
				" switchport mode trunk",
				" switchport trunk allowed vlan 100",
			},
			config2: []string{
				"interface GigabitEthernet1/0/24",
				" switchport mode trunk",
				" switchport trunk allowed vlan 100,200",
			},
			expectEqual: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := computeSnippetHash(tt.config1)
			hash2 := computeSnippetHash(tt.config2)

			if tt.expectEqual {
				assert.Equal(t, hash1, hash2,
					"Scenario: %s\nExpected no drift but hashes differ", tt.scenario)
			} else {
				assert.NotEqual(t, hash1, hash2,
					"Scenario: %s\nExpected drift but hashes are equal", tt.scenario)
			}

			t.Logf("Scenario: %s", tt.scenario)
			t.Logf("Config1 hash: %s", hash1)
			t.Logf("Config2 hash: %s", hash2)
			t.Logf("Drift detected: %v", !tt.expectEqual)
		})
	}
}

// TestComputeSnippetHashEdgeCases tests edge cases and boundary conditions.
func TestComputeSnippetHashEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string // empty means just verify it doesn't panic
	}{
		{
			name:  "Empty input",
			input: []string{},
		},
		{
			name:  "Single empty string",
			input: []string{""},
		},
		{
			name:  "Only comments",
			input: []string{"!", "! Comment line 1", "! Comment line 2"},
		},
		{
			name:  "Only whitespace",
			input: []string{"   ", "\t\t", "  \t  "},
		},
		{
			name: "Very long line",
			input: []string{
				"interface GigabitEthernet1/0/1",
				" description This is a very long description that contains a lot of text to test how the hashing function handles long lines with many characters and spaces and other content that goes on and on and on",
			},
		},
		{
			name: "Special characters",
			input: []string{
				"interface GigabitEthernet1/0/1",
				" description Port@#$%^&*()_+-=[]{}|;':\",./<>?",
			},
		},
		{
			name: "Unicode characters (if supported)",
			input: []string{
				"interface GigabitEthernet1/0/1",
				" description Büro™ Network® Port№1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			hash := computeSnippetHash(tt.input)

			// Verify hash format is valid
			assert.Contains(t, hash, "sha256:",
				"Hash should have 'sha256:' prefix")
			assert.Greater(t, len(hash), 10,
				"Hash should be non-trivial length")

			t.Logf("Edge case '%s' hash: %s", tt.name, hash)
		})
	}
}

// BenchmarkComputeSnippetHash benchmarks the hashing function performance.
func BenchmarkComputeSnippetHash(b *testing.B) {
	input := []string{
		"interface GigabitEthernet1/0/1",
		" description Access port for user workstations",
		" switchport mode access",
		" switchport access vlan 100",
		" switchport port-security",
		" switchport port-security maximum 2",
		" switchport port-security violation restrict",
		" ip dhcp snooping trust",
		" ip arp inspection trust",
		" ip verify source",
		" authentication port-control auto",
		" authentication periodic",
		" authentication timer reauthenticate 3600",
		" mab",
		" spanning-tree portfast",
		" spanning-tree bpduguard enable",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = computeSnippetHash(input)
	}
}

// BenchmarkComputeSnippetHashLargeInput benchmarks with a large config stanza.
func BenchmarkComputeSnippetHashLargeInput(b *testing.B) {
	// Simulate a large interface config with many commands
	input := make([]string, 100)
	input[0] = "interface GigabitEthernet1/0/1"
	for i := 1; i < 100; i++ {
		input[i] = " some command line " + string(rune(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = computeSnippetHash(input)
	}
}
