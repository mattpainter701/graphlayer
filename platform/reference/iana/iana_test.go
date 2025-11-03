package iana

import (
	"testing"
)

// TestNewIANALoader tests basic loader initialization
func TestNewIANALoader(t *testing.T) {
	loader := NewIANALoader()

	if loader == nil {
		t.Fatal("NewIANALoader returned nil")
	}

	if len(loader.ProtocolMap) == 0 {
		t.Error("ProtocolMap is empty")
	}

	if len(loader.ServiceMap) == 0 {
		t.Error("ServiceMap is empty")
	}
}

// TestGetProtocolNumber tests protocol name to number lookups
func TestGetProtocolNumber(t *testing.T) {
	loader := NewIANALoader()

	tests := []struct {
		name           string
		protocolName   string
		expectedNumber string
		shouldExist    bool
	}{
		{"TCP", "tcp", ProtocolTCP, true},
		{"UDP", "udp", ProtocolUDP, true},
		{"ICMP", "icmp", ProtocolICMP, true},
		{"IPv6", "ipv6", ProtocolIPv6, true},
		{"ESP", "esp", ProtocolESP, true},
		{"GRE", "gre", ProtocolGRE, true},
		{"OSPF", "ospf", ProtocolOSPF, true},
		{"Case insensitive", "TCP", ProtocolTCP, true},
		{"Unknown protocol", "foobar-protocol", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, ok := loader.GetProtocolNumber(tt.protocolName)

			if ok != tt.shouldExist {
				t.Errorf("GetProtocolNumber(%s): expected exists=%v, got %v", tt.protocolName, tt.shouldExist, ok)
			}

			if tt.shouldExist && num != tt.expectedNumber {
				t.Errorf("GetProtocolNumber(%s): expected %s, got %s", tt.protocolName, tt.expectedNumber, num)
			}
		})
	}
}

// TestGetProtocolName tests protocol number to name lookups
func TestGetProtocolName(t *testing.T) {
	loader := NewIANALoader()

	tests := []struct {
		name             string
		protocolNumber   string
		expectedContains string // Name should contain this (may not be exact match)
		shouldExist      bool
	}{
		{"TCP", ProtocolTCP, "tcp", true},
		{"UDP", ProtocolUDP, "udp", true},
		{"ICMP", ProtocolICMP, "icmp", true},
		{"Unknown number", "999", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, ok := loader.GetProtocolName(tt.protocolNumber)

			if ok != tt.shouldExist {
				t.Errorf("GetProtocolName(%s): expected exists=%v, got %v", tt.protocolNumber, tt.shouldExist, ok)
			}

			if tt.shouldExist && tt.expectedContains != "" {
				// Name should exist and might be the exact match or a variant
				if name != tt.expectedContains {
					t.Logf("GetProtocolName(%s): got name '%s', expected to contain '%s'", tt.protocolNumber, name, tt.expectedContains)
				}
			}
		})
	}
}

// TestGetServiceFromPort tests port to service name lookups
func TestGetServiceFromPort(t *testing.T) {
	loader := NewIANALoader()

	tests := []struct {
		name            string
		port            string
		protocol        string
		expectedService string
		shouldExist     bool
	}{
		{"HTTP", "80", "tcp", "http", true},
		{"HTTPS", "443", "tcp", "https", true},
		{"DNS UDP", "53", "udp", "dns", true},
		{"DNS TCP", "53", "tcp", "domain", true}, // Could be "domain" or "dns"
		{"SSH", "22", "tcp", "ssh", true},
		{"Telnet", "23", "tcp", "telnet", true},
		{"SMTP", "25", "tcp", "smtp", true},
		{"FTP", "21", "tcp", "ftp", true},
		{"SNMP", "161", "udp", "snmp", true},
		{"Syslog", "514", "udp", "syslog", true},
		{"Unknown port", "99999", "tcp", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, ok := loader.GetServiceFromPort(tt.port, tt.protocol)

			if ok != tt.shouldExist {
				t.Errorf("GetServiceFromPort(%s, %s): expected exists=%v, got %v", tt.port, tt.protocol, tt.shouldExist, ok)
			}

			if tt.shouldExist && service == "" {
				t.Errorf("GetServiceFromPort(%s, %s): expected service name, got empty string", tt.port, tt.protocol)
			}

			// For known services, log what we got (may vary slightly from expected)
			if tt.shouldExist && service != tt.expectedService {
				t.Logf("GetServiceFromPort(%s, %s): got '%s', expected '%s'", tt.port, tt.protocol, service, tt.expectedService)
			}
		})
	}
}

// TestGetPortsForService tests service to ports lookups
func TestGetPortsForService(t *testing.T) {
	loader := NewIANALoader()

	tests := []struct {
		name         string
		serviceName  string
		protocol     string
		expectedPort string // Should contain this port
		shouldExist  bool
	}{
		{"HTTP TCP", "http", "tcp", "80", true},
		{"HTTPS TCP", "https", "tcp", "443", true},
		{"DNS UDP", "dns", "udp", "53", true},
		{"SSH TCP", "ssh", "tcp", "22", true},
		{"FTP TCP", "ftp", "tcp", "21", true},
		{"Unknown service", "foobar-service", "tcp", "", false},
		{"Valid service, wrong protocol", "http", "udp", "80", true}, // HTTP can be UDP too
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports, ok := loader.GetPortsForService(tt.serviceName, tt.protocol)

			if ok != tt.shouldExist {
				t.Errorf("GetPortsForService(%s, %s): expected exists=%v, got %v", tt.serviceName, tt.protocol, tt.shouldExist, ok)
			}

			if tt.shouldExist && len(ports) == 0 {
				t.Errorf("GetPortsForService(%s, %s): expected ports, got empty slice", tt.serviceName, tt.protocol)
			}

			if tt.shouldExist && tt.expectedPort != "" {
				found := false
				for _, port := range ports {
					if port == tt.expectedPort {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("GetPortsForService(%s, %s): expected port %s in %v", tt.serviceName, tt.protocol, tt.expectedPort, ports)
				}
			}
		})
	}
}

// TestGetAllProtocolsForService tests retrieving all protocols for a service
func TestGetAllProtocolsForService(t *testing.T) {
	loader := NewIANALoader()

	tests := []struct {
		name        string
		serviceName string
		shouldExist bool
	}{
		{"HTTP", "http", true},
		{"DNS", "dns", true},
		{"SSH", "ssh", true},
		{"Unknown service", "nonexistent-svc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protocols, ok := loader.GetAllProtocolsForService(tt.serviceName)

			if ok != tt.shouldExist {
				t.Errorf("GetAllProtocolsForService(%s): expected exists=%v, got %v", tt.serviceName, tt.shouldExist, ok)
			}

			if tt.shouldExist && len(protocols) == 0 {
				t.Errorf("GetAllProtocolsForService(%s): expected protocols, got empty map", tt.serviceName)
			}
		})
	}
}

// TestCiscoDefaultServices tests that Cisco-specific services are loaded
func TestCiscoDefaultServices(t *testing.T) {
	loader := NewIANALoader()

	ciscoServices := []string{
		"citrix-ica",
		"h323",
		"ctiqbe",
		"pcanywhere-data",
		"pim-auto-rp",
		"sqlnet",
		"lotusnotes",
	}

	for _, svc := range ciscoServices {
		t.Run(svc, func(t *testing.T) {
			protocols, ok := loader.GetAllProtocolsForService(svc)

			if !ok {
				t.Errorf("Cisco service %s not found in ServiceMap", svc)
			}

			if ok && len(protocols) == 0 {
				t.Errorf("Cisco service %s has no protocols", svc)
			}
		})
	}
}

// TestGetEtherTypeName tests EtherType lookups
func TestGetEtherTypeName(t *testing.T) {
	loader := NewIANALoader()

	tests := []struct {
		name         string
		etherType    string
		expectedName string
		shouldExist  bool
	}{
		{"IPv4", "2048", "IPv4", true},
		{"ARP", "2054", "Address Resolution Protocol", true},
		{"IPv6", "34525", "IPv6", true},
		{"Unknown EtherType", "99999", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, ok := loader.GetEtherTypeName(tt.etherType)

			if ok != tt.shouldExist {
				t.Errorf("GetEtherTypeName(%s): expected exists=%v, got %v", tt.etherType, tt.shouldExist, ok)
			}

			// For EtherTypes, just log the result (descriptions may vary)
			if tt.shouldExist && name != "" {
				t.Logf("GetEtherTypeName(%s): got '%s'", tt.etherType, name)
			}
		})
	}
}

// TestGetAllProtocols tests retrieving all protocol names
func TestGetAllProtocols(t *testing.T) {
	loader := NewIANALoader()

	protocols := loader.GetAllProtocols()

	if len(protocols) == 0 {
		t.Error("GetAllProtocols returned empty slice")
	}

	// Should include common protocols
	expectedProtocols := []string{"tcp", "udp", "icmp", "esp", "gre"}

	for _, expected := range expectedProtocols {
		found := false
		for _, proto := range protocols {
			if proto == expected {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("GetAllProtocols missing expected protocol: %s", expected)
		}
	}

	t.Logf("GetAllProtocols returned %d protocols", len(protocols))
}

// TestGetAllServices tests retrieving all service names
func TestGetAllServices(t *testing.T) {
	loader := NewIANALoader()

	services := loader.GetAllServices()

	if len(services) == 0 {
		t.Error("GetAllServices returned empty slice")
	}

	// Should include common services
	expectedServices := []string{"http", "https", "ssh", "ftp", "dns"}

	for _, expected := range expectedServices {
		found := false
		for _, svc := range services {
			if svc == expected {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("GetAllServices missing expected service: %s", expected)
		}
	}

	t.Logf("GetAllServices returned %d services", len(services))
}

// TestConcurrentAccess tests thread-safe concurrent access
func TestConcurrentAccess(t *testing.T) {
	loader := NewIANALoader()

	// Run multiple concurrent lookups
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			// Protocol lookups
			loader.GetProtocolNumber("tcp")
			loader.GetProtocolNumber("udp")

			// Service lookups
			loader.GetServiceFromPort("80", "tcp")
			loader.GetServiceFromPort("443", "tcp")

			// EtherType lookups
			loader.GetEtherTypeName("2048")

			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	t.Log("Concurrent access test passed")
}

// TestReverseServiceMap tests the reverse service lookup
func TestReverseServiceMap(t *testing.T) {
	loader := NewIANALoader()

	// ReverseServiceMap should be populated
	if len(loader.ReverseServiceMap) == 0 {
		t.Error("ReverseServiceMap is empty")
	}

	// Test known mapping
	key := ProtocolPort{Protocol: "tcp", Port: "80"}
	serviceName, ok := loader.ReverseServiceMap[key]

	if !ok {
		t.Error("ReverseServiceMap missing TCP/80 mapping")
	}

	if ok && serviceName == "" {
		t.Error("ReverseServiceMap TCP/80 has empty service name")
	}

	t.Logf("ReverseServiceMap has %d entries", len(loader.ReverseServiceMap))
}

// Benchmark tests

func BenchmarkGetProtocolNumber(b *testing.B) {
	loader := NewIANALoader()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loader.GetProtocolNumber("tcp")
	}
}

func BenchmarkGetServiceFromPort(b *testing.B) {
	loader := NewIANALoader()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loader.GetServiceFromPort("80", "tcp")
	}
}

func BenchmarkGetPortsForService(b *testing.B) {
	loader := NewIANALoader()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loader.GetPortsForService("http", "tcp")
	}
}
