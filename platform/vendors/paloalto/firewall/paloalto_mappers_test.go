package paloalto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"example.com/topology/models"
)

// TestBuildPANOSDeviceModel tests the device model mapper
func TestBuildPANOSDeviceModel(t *testing.T) {
	tests := []struct {
		name          string
		processor     *PANOSProcessor
		expectedError bool
		validate      func(t *testing.T, p *PANOSProcessor)
	}{
		{
			name: "Valid system info",
			processor: &PANOSProcessor{
				systemInfo: PANOSSystemInfo{
					Hostname: "fw-prod-01",
					Serial:   "015351000012345",
					Model:    "PA-3020",
					SWVersion: "10.2.3",
					IPAddress: "192.168.1.1",
				},
				TopologyDevices: make(map[string]models.Device),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyDevices))
				device := p.TopologyDevices["fw-prod-01"]
				assert.Equal(t, "fw-prod-01", device.Hostname)
				assert.Equal(t, "015351000012345", device.SerialNumber)
				assert.Equal(t, "PA-3020", device.Model)
				assert.Equal(t, "10.2.3", device.OSVersion)
				assert.Equal(t, "192.168.1.1", device.ManagementIP)
				assert.Equal(t, "palo_alto", device.Vendor)
				assert.Equal(t, "firewall", device.DeviceType)
			},
		},
		{
			name: "Empty hostname",
			processor: &PANOSProcessor{
				systemInfo: PANOSSystemInfo{
					Hostname: "",
					Serial:   "015351000012345",
					Model:    "PA-220",
					SWVersion: "9.1.0",
				},
				TopologyDevices: make(map[string]models.Device),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyDevices))
				device := p.TopologyDevices["panos-device"]
				assert.Equal(t, "panos-device", device.Hostname)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.processor.buildPANOSDeviceModel()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validate(t, tt.processor)
			}
		})
	}
}

// TestBuildPANOSVirtualContextModel tests vsys to VirtualContext mapping
func TestBuildPANOSVirtualContextModel(t *testing.T) {
	tests := []struct {
		name          string
		processor     *PANOSProcessor
		expectedError bool
		validate      func(t *testing.T, p *PANOSProcessor)
	}{
		{
			name: "Single vsys",
			processor: &PANOSProcessor{
				virtualSystems: []PANOSVirtualSystem{
					{
						Name:        "vsys1",
						DisplayName: "Production",
						Description: "Production environment",
					},
				},
				TopologyVirtualContexts: make(map[string]models.VirtualContext),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyVirtualContexts))
				ctx := p.TopologyVirtualContexts["vsys1"]
				assert.Equal(t, "vsys1", ctx.Name)
				assert.Equal(t, "Production", ctx.DisplayName)
				assert.Equal(t, "Production environment", ctx.Description)
				assert.Equal(t, "PALO_ALTO_VSYS", ctx.Type)
			},
		},
		{
			name: "Multiple vsys",
			processor: &PANOSProcessor{
				virtualSystems: []PANOSVirtualSystem{
					{
						Name:        "vsys1",
						DisplayName: "Production",
						Description: "Production environment",
					},
					{
						Name:        "vsys2",
						DisplayName: "Development",
						Description: "Dev environment",
					},
					{
						Name:        "vsys3",
						DisplayName: "DMZ",
						Description: "DMZ environment",
					},
				},
				TopologyVirtualContexts: make(map[string]models.VirtualContext),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 3, len(p.TopologyVirtualContexts))

				vsys1 := p.TopologyVirtualContexts["vsys1"]
				assert.Equal(t, "vsys1", vsys1.Name)
				assert.Equal(t, "Production", vsys1.DisplayName)

				vsys2 := p.TopologyVirtualContexts["vsys2"]
				assert.Equal(t, "vsys2", vsys2.Name)
				assert.Equal(t, "Development", vsys2.DisplayName)

				vsys3 := p.TopologyVirtualContexts["vsys3"]
				assert.Equal(t, "vsys3", vsys3.Name)
				assert.Equal(t, "DMZ", vsys3.DisplayName)
			},
		},
		{
			name: "Empty vsys list - creates default",
			processor: &PANOSProcessor{
				virtualSystems:          []PANOSVirtualSystem{},
				TopologyVirtualContexts: make(map[string]models.VirtualContext),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyVirtualContexts))
				ctx := p.TopologyVirtualContexts[defaultPANOSContextName]
				assert.Equal(t, defaultPANOSContextName, ctx.Name)
				assert.Equal(t, "PALO_ALTO_VSYS", ctx.Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.processor.buildPANOSVirtualContextModel()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validate(t, tt.processor)
			}
		})
	}
}

// TestBuildPANOSZoneModel tests zone mapping
func TestBuildPANOSZoneModel(t *testing.T) {
	tests := []struct {
		name          string
		processor     *PANOSProcessor
		expectedError bool
		validate      func(t *testing.T, p *PANOSProcessor)
	}{
		{
			name: "Layer 3 zones",
			processor: &PANOSProcessor{
				zones: []PANOSZone{
					{
						Name: "trust",
						Network: PANOSZoneNetwork{
							Layer3: []string{"ethernet1/1", "ethernet1/2"},
						},
					},
					{
						Name: "untrust",
						Network: PANOSZoneNetwork{
							Layer3: []string{"ethernet1/3"},
						},
					},
					{
						Name: "dmz",
						Network: PANOSZoneNetwork{
							Layer3: []string{"ethernet1/4", "ethernet1/5"},
						},
					},
				},
				TopologyZones: make(map[string]models.Zone),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 3, len(p.TopologyZones))

				trust := p.TopologyZones["trust"]
				assert.Equal(t, "trust", trust.Name)
				assert.Equal(t, "layer3", trust.Type)
				assert.Contains(t, trust.Interfaces, "ethernet1/1")
				assert.Contains(t, trust.Interfaces, "ethernet1/2")

				untrust := p.TopologyZones["untrust"]
				assert.Equal(t, "untrust", untrust.Name)
				assert.Contains(t, untrust.Interfaces, "ethernet1/3")

				dmz := p.TopologyZones["dmz"]
				assert.Equal(t, "dmz", dmz.Name)
				assert.Contains(t, dmz.Interfaces, "ethernet1/4")
				assert.Contains(t, dmz.Interfaces, "ethernet1/5")
			},
		},
		{
			name: "Layer 2 zones",
			processor: &PANOSProcessor{
				zones: []PANOSZone{
					{
						Name: "vwire-zone",
						Network: PANOSZoneNetwork{
							Layer2: []string{"ethernet1/6", "ethernet1/7"},
						},
					},
				},
				TopologyZones: make(map[string]models.Zone),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyZones))
				zone := p.TopologyZones["vwire-zone"]
				assert.Equal(t, "layer2", zone.Type)
				assert.Contains(t, zone.Interfaces, "ethernet1/6")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.processor.buildPANOSZoneModel()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validate(t, tt.processor)
			}
		})
	}
}

// TestBuildPANOSInterfaceModel tests interface mapping
func TestBuildPANOSInterfaceModel(t *testing.T) {
	tests := []struct {
		name          string
		processor     *PANOSProcessor
		expectedError bool
		validate      func(t *testing.T, p *PANOSProcessor)
	}{
		{
			name: "Layer 3 ethernet interfaces",
			processor: &PANOSProcessor{
				interfaces: []PANOSInterface{
					{
						Name: "ethernet1/1",
						Layer3: &PANOSLayer3Interface{
							IP: []PANOSIPAddress{
								{Address: "10.1.1.1/24"},
								{Address: "10.1.2.1/24"},
							},
							MTU: "1500",
						},
					},
					{
						Name: "ethernet1/2",
						Layer3: &PANOSLayer3Interface{
							IP: []PANOSIPAddress{
								{Address: "192.168.1.1/24"},
							},
							MTU: "9000",
						},
					},
				},
				interfaceStatus: []PANOSInterfaceStatus{
					{
						Name:   "ethernet1/1",
						State:  "up",
						MAC:    "00:1a:2b:3c:4d:5e",
						Speed:  "1000",
						Duplex: "full",
					},
					{
						Name:   "ethernet1/2",
						State:  "up",
						MAC:    "00:1a:2b:3c:4d:5f",
						Speed:  "10000",
						Duplex: "full",
					},
				},
				zones: []PANOSZone{
					{
						Name: "trust",
						Network: PANOSZoneNetwork{
							Layer3: []string{"ethernet1/1"},
						},
					},
					{
						Name: "untrust",
						Network: PANOSZoneNetwork{
							Layer3: []string{"ethernet1/2"},
						},
					},
				},
				TopologyInterfaces: make(map[string]models.Interface),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 2, len(p.TopologyInterfaces))

				eth1 := p.TopologyInterfaces["ethernet1/1"]
				assert.Equal(t, "ethernet1/1", eth1.Name)
				assert.Equal(t, "layer3", eth1.Type)
				assert.Contains(t, eth1.IPAddresses, "10.1.1.1/24")
				assert.Contains(t, eth1.IPAddresses, "10.1.2.1/24")
				assert.Equal(t, 1500, eth1.MTU)
				assert.Equal(t, "up", eth1.Status)
				assert.Equal(t, "00:1a:2b:3c:4d:5e", eth1.MACAddress)
				assert.Equal(t, "trust", eth1.SecurityZone)

				eth2 := p.TopologyInterfaces["ethernet1/2"]
				assert.Equal(t, "ethernet1/2", eth2.Name)
				assert.Equal(t, 9000, eth2.MTU)
				assert.Equal(t, "untrust", eth2.SecurityZone)
			},
		},
		{
			name: "Loopback interface",
			processor: &PANOSProcessor{
				interfaces: []PANOSInterface{
					{
						Name: "loopback.1",
						Loopback: &PANOSLoopbackInterface{
							IP: []PANOSIPAddress{
								{Address: "1.1.1.1/32"},
							},
						},
					},
				},
				TopologyInterfaces: make(map[string]models.Interface),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyInterfaces))
				lo := p.TopologyInterfaces["loopback.1"]
				assert.Equal(t, "loopback.1", lo.Name)
				assert.Equal(t, "loopback", lo.Type)
				assert.Contains(t, lo.IPAddresses, "1.1.1.1/32")
			},
		},
		{
			name: "Tunnel interface",
			processor: &PANOSProcessor{
				interfaces: []PANOSInterface{
					{
						Name: "tunnel.1",
						Tunnel: &PANOSTunnelInterface{
							IP: []PANOSIPAddress{
								{Address: "172.16.1.1/30"},
							},
							MTU: "1400",
						},
					},
				},
				TopologyInterfaces: make(map[string]models.Interface),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyInterfaces))
				tun := p.TopologyInterfaces["tunnel.1"]
				assert.Equal(t, "tunnel.1", tun.Name)
				assert.Equal(t, "tunnel", tun.Type)
				assert.Contains(t, tun.IPAddresses, "172.16.1.1/30")
				assert.Equal(t, 1400, tun.MTU)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.processor.buildPANOSInterfaceModel()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validate(t, tt.processor)
			}
		})
	}
}

// TestBuildPANOSSecurityPolicyModel tests security policy mapping with App-ID
func TestBuildPANOSSecurityPolicyModel(t *testing.T) {
	tests := []struct {
		name          string
		processor     *PANOSProcessor
		expectedError bool
		validate      func(t *testing.T, p *PANOSProcessor)
	}{
		{
			name: "Allow policy with App-ID",
			processor: &PANOSProcessor{
				securityPolicies: []PANOSSecurityPolicy{
					{
						Name:                 "allow-web-traffic",
						UUID:                 "uuid-12345",
						FromZones:            []string{"trust"},
						ToZones:              []string{"untrust"},
						SourceAddresses:      []string{"10.1.1.0/24"},
						DestinationAddresses: []string{"any"},
						Applications:         []string{"web-browsing", "ssl"},
						Services:             []string{"application-default"},
						Action:               "allow",
						LogEnd:               "yes",
						Description:          "Allow web browsing",
						Disabled:             "no",
					},
				},
				TopologyFirewallPolicies: make(map[string]models.FirewallPolicy),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyFirewallPolicies))

				var policy models.FirewallPolicy
				for _, p := range p.TopologyFirewallPolicies {
					policy = p
					break
				}

				assert.Equal(t, "allow-web-traffic", policy.Name)
				assert.Equal(t, true, policy.Enabled)
				assert.Contains(t, policy.SourceZones, "trust")
				assert.Contains(t, policy.DestinationZones, "untrust")
				assert.Contains(t, policy.SourceAddresses, "10.1.1.0/24")
				assert.Contains(t, policy.DestinationAddresses, "any")
				assert.Equal(t, "allow", policy.Action)
				assert.Equal(t, true, policy.LoggingEnabled)

				// Check App-ID mapping
				assert.Contains(t, policy.Services, "tcp_80") // web-browsing maps to tcp_80
				assert.Contains(t, policy.Services, "tcp_443") // ssl maps to tcp_443

				// Check metadata
				assert.Equal(t, "uuid-12345", policy.Metadata["panos_uuid"])
				assert.Equal(t, "web-browsing,ssl", policy.Metadata["panos_applications"])
			},
		},
		{
			name: "Deny policy with User-ID",
			processor: &PANOSProcessor{
				securityPolicies: []PANOSSecurityPolicy{
					{
						Name:                 "block-contractors",
						FromZones:            []string{"trust"},
						ToZones:              []string{"untrust"},
						SourceAddresses:      []string{"any"},
						SourceUsers:          []string{"domain\\contractors"},
						DestinationAddresses: []string{"any"},
						Applications:         []string{"any"},
						Services:             []string{"any"},
						Action:               "deny",
						LogStart:             "yes",
						Disabled:             "no",
					},
				},
				TopologyFirewallPolicies: make(map[string]models.FirewallPolicy),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyFirewallPolicies))

				var policy models.FirewallPolicy
				for _, p := range p.TopologyFirewallPolicies {
					policy = p
					break
				}

				assert.Equal(t, "block-contractors", policy.Name)
				assert.Equal(t, "deny", policy.Action)
				assert.Equal(t, "domain\\contractors", policy.Metadata["panos_source_users"])
			},
		},
		{
			name: "Disabled policy",
			processor: &PANOSProcessor{
				securityPolicies: []PANOSSecurityPolicy{
					{
						Name:                 "old-rule",
						FromZones:            []string{"trust"},
						ToZones:              []string{"untrust"},
						SourceAddresses:      []string{"any"},
						DestinationAddresses: []string{"any"},
						Applications:         []string{"any"},
						Services:             []string{"any"},
						Action:               "allow",
						Disabled:             "yes",
					},
				},
				TopologyFirewallPolicies: make(map[string]models.FirewallPolicy),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyFirewallPolicies))

				var policy models.FirewallPolicy
				for _, p := range p.TopologyFirewallPolicies {
					policy = p
					break
				}

				assert.Equal(t, false, policy.Enabled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.processor.buildPANOSSecurityPolicyModel()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validate(t, tt.processor)
			}
		})
	}
}

// TestBuildPANOSNATModel tests NAT policy mapping
func TestBuildPANOSNATModel(t *testing.T) {
	tests := []struct {
		name          string
		processor     *PANOSProcessor
		expectedError bool
		validate      func(t *testing.T, p *PANOSProcessor)
	}{
		{
			name: "Source NAT with dynamic IP",
			processor: &PANOSProcessor{
				natPolicies: []PANOSNATPolicy{
					{
						Name:                 "outbound-nat",
						FromZones:            []string{"trust"},
						ToZones:              []string{"untrust"},
						SourceAddresses:      []string{"10.1.1.0/24"},
						DestinationAddresses: []string{"any"},
						Service:              "any",
						SourceTranslation: &PANOSSourceTranslation{
							Type: "dynamic-ip-and-port",
							DynamicIPAndPort: &PANOSDynamicIPAndPort{
								TranslatedAddress: []string{"203.0.113.100"},
							},
						},
					},
				},
				TopologyNATs: make(map[string]models.NAT),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyNATs))

				var nat models.NAT
				for _, n := range p.TopologyNATs {
					nat = n
					break
				}

				assert.Equal(t, "outbound-nat_source", nat.Name)
				assert.Equal(t, "source-nat-dynamic", nat.Type)
				assert.Equal(t, "10.1.1.0/24", nat.OriginalSource)
				assert.Equal(t, "203.0.113.100", nat.TranslatedSource)
				assert.Contains(t, nat.SourceZones, "trust")
				assert.Contains(t, nat.DestinationZones, "untrust")
			},
		},
		{
			name: "Destination NAT (port forward)",
			processor: &PANOSProcessor{
				natPolicies: []PANOSNATPolicy{
					{
						Name:                 "web-server-dnat",
						FromZones:            []string{"untrust"},
						ToZones:              []string{"dmz"},
						SourceAddresses:      []string{"any"},
						DestinationAddresses: []string{"203.0.113.50"},
						Service:              "tcp_80",
						DestinationTranslation: &PANOSDestinationTranslation{
							TranslatedAddress: "10.2.2.10",
							TranslatedPort:    "8080",
						},
					},
				},
				TopologyNATs: make(map[string]models.NAT),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyNATs))

				var nat models.NAT
				for _, n := range p.TopologyNATs {
					nat = n
					break
				}

				assert.Equal(t, "web-server-dnat_destination", nat.Name)
				assert.Equal(t, "destination-nat", nat.Type)
				assert.Equal(t, "203.0.113.50", nat.OriginalDestination)
				assert.Equal(t, "10.2.2.10", nat.TranslatedDestination)
				assert.Equal(t, "8080", nat.TranslatedPort)
			},
		},
		{
			name: "Source + Destination NAT (complex rule)",
			processor: &PANOSProcessor{
				natPolicies: []PANOSNATPolicy{
					{
						Name:                 "complex-nat",
						FromZones:            []string{"trust"},
						ToZones:              []string{"dmz"},
						SourceAddresses:      []string{"10.1.1.100"},
						DestinationAddresses: []string{"192.168.1.50"},
						Service:              "tcp_443",
						SourceTranslation: &PANOSSourceTranslation{
							Type: "static-ip",
							StaticIP: &PANOSStaticIP{
								TranslatedAddress: "10.2.2.200",
							},
						},
						DestinationTranslation: &PANOSDestinationTranslation{
							TranslatedAddress: "10.2.2.10",
							TranslatedPort:    "8443",
						},
					},
				},
				TopologyNATs: make(map[string]models.NAT),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				// Should create 2 NAT entries (source + destination)
				assert.Equal(t, 2, len(p.TopologyNATs))

				foundSource := false
				foundDest := false

				for _, nat := range p.TopologyNATs {
					if nat.Type == "source-nat-static" {
						foundSource = true
						assert.Equal(t, "complex-nat_source", nat.Name)
						assert.Equal(t, "10.1.1.100", nat.OriginalSource)
						assert.Equal(t, "10.2.2.200", nat.TranslatedSource)
					} else if nat.Type == "destination-nat" {
						foundDest = true
						assert.Equal(t, "complex-nat_destination", nat.Name)
						assert.Equal(t, "192.168.1.50", nat.OriginalDestination)
						assert.Equal(t, "10.2.2.10", nat.TranslatedDestination)
						assert.Equal(t, "8443", nat.TranslatedPort)
					}
				}

				assert.True(t, foundSource, "Source NAT entry not found")
				assert.True(t, foundDest, "Destination NAT entry not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.processor.buildPANOSNATModel()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validate(t, tt.processor)
			}
		})
	}
}

// TestMapAppIDToService tests the App-ID to service mapping helper
func TestMapAppIDToService(t *testing.T) {
	tests := []struct {
		appID           string
		expectedService string
	}{
		{"ssl", "tcp_443"},
		{"web-browsing", "tcp_80"},
		{"ssh", "tcp_22"},
		{"ftp", "tcp_21"},
		{"smtp", "tcp_25"},
		{"dns", "udp_53"},
		{"http", "tcp_80"},
		{"https", "tcp_443"},
		{"telnet", "tcp_23"},
		{"snmp", "udp_161"},
		{"ldap", "tcp_389"},
		{"mysql", "tcp_3306"},
		{"postgres", "tcp_5432"},
		{"redis", "tcp_6379"},
		{"mongodb", "tcp_27017"},
		{"rdp", "tcp_3389"},
		{"vnc", "tcp_5900"},
		{"unknown-app", ""}, // Unknown App-ID
		{"custom-app-123", ""}, // Custom App-ID
	}

	for _, tt := range tests {
		t.Run(tt.appID, func(t *testing.T) {
			result := mapAppIDToService(tt.appID)
			assert.Equal(t, tt.expectedService, result)
		})
	}
}

// TestParsePortRange tests port range parsing helper
func TestParsePortRange(t *testing.T) {
	tests := []struct {
		name      string
		portSpec  string
		expected  string
		expectErr bool
	}{
		{"Single port", "80", "80", false},
		{"Port range", "1024-65535", "1024-65535", false},
		{"Empty string", "", "", false},
		{"Any", "any", "any", false},
		{"With protocol", "tcp/80", "80", false},
		{"With protocol range", "tcp/1024-2048", "1024-2048", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePortRange(tt.portSpec)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFindZoneForInterface tests zone lookup helper
func TestFindZoneForInterface(t *testing.T) {
	zones := []PANOSZone{
		{
			Name: "trust",
			Network: PANOSZoneNetwork{
				Layer3: []string{"ethernet1/1", "ethernet1/2"},
			},
		},
		{
			Name: "untrust",
			Network: PANOSZoneNetwork{
				Layer3: []string{"ethernet1/3"},
			},
		},
		{
			Name: "dmz",
			Network: PANOSZoneNetwork{
				Layer2: []string{"ethernet1/4"},
			},
		},
	}

	tests := []struct {
		interfaceName string
		expectedZone  string
	}{
		{"ethernet1/1", "trust"},
		{"ethernet1/2", "trust"},
		{"ethernet1/3", "untrust"},
		{"ethernet1/4", "dmz"},
		{"ethernet1/5", ""}, // Not in any zone
	}

	for _, tt := range tests {
		t.Run(tt.interfaceName, func(t *testing.T) {
			result := findZoneForInterface(tt.interfaceName, zones)
			assert.Equal(t, tt.expectedZone, result)
		})
	}
}

// TestBuildPANOSAddressModel tests address and address group mapping
func TestBuildPANOSAddressModel(t *testing.T) {
	tests := []struct {
		name          string
		processor     *PANOSProcessor
		expectedError bool
		validate      func(t *testing.T, p *PANOSProcessor)
	}{
		{
			name: "IP address objects",
			processor: &PANOSProcessor{
				addressObjects: []PANOSAddressObject{
					{
						Name:        "webserver-1",
						IPNetmask:   "10.1.1.10/32",
						Description: "Production web server",
						Tags:        []string{"production", "web"},
					},
					{
						Name:        "db-subnet",
						IPNetmask:   "10.2.0.0/24",
						Description: "Database subnet",
					},
				},
				TopologyAddresses:      make(map[string]models.Address),
				TopologyAddressGroups:  make(map[string]models.AddressGroup),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 2, len(p.TopologyAddresses))

				webserver := p.TopologyAddresses["webserver-1"]
				assert.Equal(t, "webserver-1", webserver.Name)
				assert.Equal(t, "10.1.1.10/32", webserver.Value)
				assert.Equal(t, "Production web server", webserver.Description)
				assert.Contains(t, webserver.Tags, "production")
				assert.Contains(t, webserver.Tags, "web")

				dbSubnet := p.TopologyAddresses["db-subnet"]
				assert.Equal(t, "10.2.0.0/24", dbSubnet.Value)
			},
		},
		{
			name: "Address groups",
			processor: &PANOSProcessor{
				addressObjects: []PANOSAddressObject{
					{
						Name:      "webserver-1",
						IPNetmask: "10.1.1.10/32",
					},
					{
						Name:      "webserver-2",
						IPNetmask: "10.1.1.11/32",
					},
				},
				addressGroups: []PANOSAddressGroup{
					{
						Name:        "web-servers",
						Static:      []string{"webserver-1", "webserver-2"},
						Description: "All web servers",
						Tags:        []string{"web"},
					},
				},
				TopologyAddresses:      make(map[string]models.Address),
				TopologyAddressGroups:  make(map[string]models.AddressGroup),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 2, len(p.TopologyAddresses))
				assert.Equal(t, 1, len(p.TopologyAddressGroups))

				group := p.TopologyAddressGroups["web-servers"]
				assert.Equal(t, "web-servers", group.Name)
				assert.Contains(t, group.Members, "webserver-1")
				assert.Contains(t, group.Members, "webserver-2")
				assert.Equal(t, "All web servers", group.Description)
			},
		},
		{
			name: "IP range address",
			processor: &PANOSProcessor{
				addressObjects: []PANOSAddressObject{
					{
						Name:    "dhcp-range",
						IPRange: "192.168.1.100-192.168.1.200",
					},
				},
				TopologyAddresses:      make(map[string]models.Address),
				TopologyAddressGroups:  make(map[string]models.AddressGroup),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyAddresses))
				addr := p.TopologyAddresses["dhcp-range"]
				assert.Equal(t, "192.168.1.100-192.168.1.200", addr.Value)
			},
		},
		{
			name: "FQDN address",
			processor: &PANOSProcessor{
				addressObjects: []PANOSAddressObject{
					{
						Name: "api-server",
						FQDN: "api.example.com",
					},
				},
				TopologyAddresses:      make(map[string]models.Address),
				TopologyAddressGroups:  make(map[string]models.AddressGroup),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyAddresses))
				addr := p.TopologyAddresses["api-server"]
				assert.Equal(t, "api.example.com", addr.Value)
				assert.Equal(t, "fqdn", addr.Metadata["address_type"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.processor.buildPANOSAddressModel()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validate(t, tt.processor)
			}
		})
	}
}

// TestBuildPANOSServiceModel tests service and service group mapping
func TestBuildPANOSServiceModel(t *testing.T) {
	tests := []struct {
		name          string
		processor     *PANOSProcessor
		expectedError bool
		validate      func(t *testing.T, p *PANOSProcessor)
	}{
		{
			name: "TCP and UDP services",
			processor: &PANOSProcessor{
				serviceObjects: []PANOSServiceObject{
					{
						Name:        "web-alt",
						Protocol:    "tcp",
						Port:        "8080",
						Description: "Alternative HTTP port",
					},
					{
						Name:     "syslog",
						Protocol: "udp",
						Port:     "514",
					},
				},
				TopologyServices:      make(map[string]models.Service),
				TopologyServiceGroups: make(map[string]models.ServiceGroup),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 2, len(p.TopologyServices))

				webAlt := p.TopologyServices["web-alt"]
				assert.Equal(t, "web-alt", webAlt.Name)
				assert.Equal(t, "tcp", webAlt.Protocol)
				assert.Equal(t, "8080", webAlt.DestinationPort)
				assert.Equal(t, "Alternative HTTP port", webAlt.Description)

				syslog := p.TopologyServices["syslog"]
				assert.Equal(t, "udp", syslog.Protocol)
				assert.Equal(t, "514", syslog.DestinationPort)
			},
		},
		{
			name: "Service groups",
			processor: &PANOSProcessor{
				serviceObjects: []PANOSServiceObject{
					{
						Name:     "http",
						Protocol: "tcp",
						Port:     "80",
					},
					{
						Name:     "https",
						Protocol: "tcp",
						Port:     "443",
					},
				},
				serviceGroups: []PANOSServiceGroup{
					{
						Name:        "web-services",
						Members:     []string{"http", "https"},
						Description: "Web protocols",
						Tags:        []string{"web"},
					},
				},
				TopologyServices:      make(map[string]models.Service),
				TopologyServiceGroups: make(map[string]models.ServiceGroup),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 2, len(p.TopologyServices))
				assert.Equal(t, 1, len(p.TopologyServiceGroups))

				group := p.TopologyServiceGroups["web-services"]
				assert.Equal(t, "web-services", group.Name)
				assert.Contains(t, group.Members, "http")
				assert.Contains(t, group.Members, "https")
				assert.Equal(t, "Web protocols", group.Description)
			},
		},
		{
			name: "Port range service",
			processor: &PANOSProcessor{
				serviceObjects: []PANOSServiceObject{
					{
						Name:     "ephemeral-ports",
						Protocol: "tcp",
						Port:     "1024-65535",
					},
				},
				TopologyServices:      make(map[string]models.Service),
				TopologyServiceGroups: make(map[string]models.ServiceGroup),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 1, len(p.TopologyServices))
				svc := p.TopologyServices["ephemeral-ports"]
				assert.Equal(t, "1024-65535", svc.DestinationPort)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.processor.buildPANOSServiceModel()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validate(t, tt.processor)
			}
		})
	}
}

// TestBuildPANOSRoutingModel tests routing model mapping
func TestBuildPANOSRoutingModel(t *testing.T) {
	tests := []struct {
		name          string
		processor     *PANOSProcessor
		expectedError bool
		validate      func(t *testing.T, p *PANOSProcessor)
	}{
		{
			name: "Virtual router with static routes",
			processor: &PANOSProcessor{
				virtualRouters: []PANOSVirtualRouter{
					{
						Name: "default",
						StaticRoute: []PANOSStaticRoute{
							{
								Name:        "default-route",
								Destination: "0.0.0.0/0",
								NextHop: PANOSNextHop{
									IPAddress: "192.168.1.1",
								},
								Interface: "ethernet1/1",
								Metric:    "10",
							},
							{
								Name:        "internal-route",
								Destination: "10.0.0.0/8",
								NextHop: PANOSNextHop{
									IPAddress: "10.1.1.1",
								},
								Interface: "ethernet1/2",
								Metric:    "5",
							},
						},
					},
				},
				TopologyRoutingEntries: make(map[string]models.RoutingEntry),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 2, len(p.TopologyRoutingEntries))

				foundDefault := false
				foundInternal := false

				for _, route := range p.TopologyRoutingEntries {
					if route.Destination == "0.0.0.0/0" {
						foundDefault = true
						assert.Equal(t, "192.168.1.1", route.NextHop)
						assert.Equal(t, "ethernet1/1", route.Interface)
						assert.Equal(t, 10, route.Metric)
						assert.Equal(t, "static", route.Protocol)
					} else if route.Destination == "10.0.0.0/8" {
						foundInternal = true
						assert.Equal(t, "10.1.1.1", route.NextHop)
						assert.Equal(t, "ethernet1/2", route.Interface)
						assert.Equal(t, 5, route.Metric)
					}
				}

				assert.True(t, foundDefault, "Default route not found")
				assert.True(t, foundInternal, "Internal route not found")
			},
		},
		{
			name: "Multiple virtual routers",
			processor: &PANOSProcessor{
				virtualRouters: []PANOSVirtualRouter{
					{
						Name: "vr-prod",
						StaticRoute: []PANOSStaticRoute{
							{
								Name:        "prod-default",
								Destination: "0.0.0.0/0",
								NextHop: PANOSNextHop{
									IPAddress: "192.168.10.1",
								},
							},
						},
					},
					{
						Name: "vr-dev",
						StaticRoute: []PANOSStaticRoute{
							{
								Name:        "dev-default",
								Destination: "0.0.0.0/0",
								NextHop: PANOSNextHop{
									IPAddress: "192.168.20.1",
								},
							},
						},
					},
				},
				TopologyRoutingEntries: make(map[string]models.RoutingEntry),
			},
			expectedError: false,
			validate: func(t *testing.T, p *PANOSProcessor) {
				assert.Equal(t, 2, len(p.TopologyRoutingEntries))

				foundProd := false
				foundDev := false

				for _, route := range p.TopologyRoutingEntries {
					if route.NextHop == "192.168.10.1" {
						foundProd = true
						assert.Equal(t, "vr-prod", route.VirtualContext)
					} else if route.NextHop == "192.168.20.1" {
						foundDev = true
						assert.Equal(t, "vr-dev", route.VirtualContext)
					}
				}

				assert.True(t, foundProd, "Production VR route not found")
				assert.True(t, foundDev, "Development VR route not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.processor.buildPANOSRoutingModel()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.validate(t, tt.processor)
			}
		})
	}
}
