package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/vendors/checkpoint/parsers"
)

func TestParseGaiaConfig(t *testing.T) {
	testData, err := os.ReadFile(filepath.Join("..", "testdata", "gaia_config.txt"))
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	config, err := parsers.ParseGaiaConfig(string(testData))
	if err != nil {
		t.Fatalf("Failed to parse Gaia config: %v", err)
	}

	if config.Hostname != "cp-gw-01" {
		t.Errorf("Expected hostname cp-gw-01, got %s", config.Hostname)
	}

	if len(config.Interfaces) != 3 {
		t.Errorf("Expected 3 interfaces, got %d", len(config.Interfaces))
	}

	if len(config.Routes) != 2 {
		t.Errorf("Expected 2 routes, got %d", len(config.Routes))
	}

	if config.DNS.Primary != "8.8.8.8" {
		t.Errorf("Expected DNS primary 8.8.8.8, got %s", config.DNS.Primary)
	}

	if len(config.NTP) != 2 {
		t.Errorf("Expected 2 NTP servers, got %d", len(config.NTP))
	}

	if !config.SNMP.Enabled {
		t.Error("Expected SNMP to be enabled")
	}

	if len(config.Users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(config.Users))
	}

	if len(config.Services) != 3 {
		t.Errorf("Expected 3 services, got %d", len(config.Services))
	}
}

func TestParseGaiaCLISH(t *testing.T) {
	testData, err := os.ReadFile(filepath.Join("..", "testdata", "gaia_config.txt"))
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	config, err := parsers.ParseGaiaCLISH(string(testData))
	if err != nil {
		t.Fatalf("Failed to parse CLISH output: %v", err)
	}

	if config.Hostname != "cp-gw-01" {
		t.Errorf("Expected hostname cp-gw-01, got %s", config.Hostname)
	}
}

func TestParseGaiaShowConfig(t *testing.T) {
	testData, err := os.ReadFile(filepath.Join("..", "testdata", "gaia_config.txt"))
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	config, err := parsers.ParseGaiaShowConfig(string(testData))
	if err != nil {
		t.Fatalf("Failed to parse show config: %v", err)
	}

	if config.Hostname != "cp-gw-01" {
		t.Errorf("Expected hostname cp-gw-01, got %s", config.Hostname)
	}
}
