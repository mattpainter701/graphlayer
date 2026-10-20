package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/vendors/checkpoint/parsers"
)

func TestParseCheckPointPolicy(t *testing.T) {
	testData, err := os.ReadFile(filepath.Join("..", "testdata", "security_policy.txt"))
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	policy, err := parsers.ParseCheckPointPolicy(string(testData), "cp-gw-01")
	if err != nil {
		t.Fatalf("Failed to parse security policy: %v", err)
	}

	if policy.Vendor != "Check Point" {
		t.Errorf("Expected vendor Check Point, got %s", policy.Vendor)
	}

	if policy.DeviceName != "cp-gw-01" {
		t.Errorf("Expected device cp-gw-01, got %s", policy.DeviceName)
	}

	if policy.PolicyName != "Production Policy" {
		t.Errorf("Expected policy name Production Policy, got %s", policy.PolicyName)
	}

	if len(policy.Rules) != 8 {
		t.Errorf("Expected 8 rules, got %d", len(policy.Rules))
	}

	// Check first rule
	if len(policy.Rules) > 0 {
		rule := policy.Rules[0]
		if rule.Name != "Allow HTTP" {
			t.Errorf("Expected rule name Allow HTTP, got %s", rule.Name)
		}
		if rule.Action != "accept" {
			t.Errorf("Expected action accept, got %s", rule.Action)
		}
		if !rule.Enabled {
			t.Error("Expected rule to be enabled")
		}
	}
}

func TestNormalizeToUnifiedPolicy(t *testing.T) {
	testData, err := os.ReadFile(filepath.Join("..", "testdata", "security_policy.txt"))
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	policy, err := parsers.ParseCheckPointPolicy(string(testData), "cp-gw-01")
	if err != nil {
		t.Fatalf("Failed to parse security policy: %v", err)
	}

	normalized := parsers.NormalizeToUnifiedPolicy(policy)

	if normalized.Vendor != "Check Point" {
		t.Errorf("Expected vendor Check Point, got %s", normalized.Vendor)
	}

	if normalized.RuleCount != 8 {
		t.Errorf("Expected 8 rules, got %d", normalized.RuleCount)
	}

	if len(normalized.Rules) != 8 {
		t.Errorf("Expected 8 normalized rules, got %d", len(normalized.Rules))
	}

	// Check normalized rule structure
	if len(normalized.Rules) > 0 {
		rule := normalized.Rules[0]
		if rule.ID == "" {
			t.Error("Expected non-empty rule ID")
		}
		if rule.Sequence != 1 {
			t.Errorf("Expected sequence 1, got %d", rule.Sequence)
		}
		if rule.OriginalVendor != "Check Point" {
			t.Errorf("Expected original vendor Check Point, got %s", rule.OriginalVendor)
		}
	}
}

func TestMergePolicies(t *testing.T) {
	testData, err := os.ReadFile(filepath.Join("..", "testdata", "security_policy.txt"))
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	policy1, _ := parsers.ParseCheckPointPolicy(string(testData), "cp-gw-01")
	policy2, _ := parsers.ParseCheckPointPolicy(string(testData), "cp-gw-02")

	policies := []*parsers.SecurityPolicy{policy1, policy2}
	merged := parsers.MergePolicies(policies)

	if merged.Vendor != "Multi-Vendor" {
		t.Errorf("Expected vendor Multi-Vendor, got %s", merged.Vendor)
	}

	if merged.RuleCount != 16 {
		t.Errorf("Expected 16 merged rules, got %d", merged.RuleCount)
	}

	if len(merged.Rules) != 16 {
		t.Errorf("Expected 16 merged rules, got %d", len(merged.Rules))
	}
}
