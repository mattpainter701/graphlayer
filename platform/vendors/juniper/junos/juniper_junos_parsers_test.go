package junos

import (
	"testing"

	"github.com/mattpainter701/graphlayer/platform/platform/vendors/juniper"
)

// Test VLAN parsing with hierarchical Junos syntax
func TestParseJunosVLANs(t *testing.T) {
	config := `
vlans {
    Engineering {
        vlan-id 10;
        description "Engineering Department";
        l3-interface irb.10;
    }
    Sales {
        vlan-id 20;
        description "Sales Department";
    }
    Guest {
        vlan-id 99;
    }
}
`

	vlans, err := ParseJunosVLANs(config)
	if err != nil {
		t.Fatalf("ParseJunosVLANs failed: %v", err)
	}

	if len(vlans) != 3 {
		t.Fatalf("Expected 3 VLANs, got %d", len(vlans))
	}

	// Check Engineering VLAN
	found := false
	for _, vlan := range vlans {
		if vlan.VLANName == "Engineering" {
			found = true
			if vlan.VLANID != 10 {
				t.Errorf("Engineering VLAN ID: expected 10, got %d", vlan.VLANID)
			}
			if vlan.Description != "Engineering Department" {
				t.Errorf("Engineering description: expected 'Engineering Department', got '%s'", vlan.Description)
			}
			if vlan.L3Interface != "irb.10" {
				t.Errorf("Engineering L3 interface: expected 'irb.10', got '%s'", vlan.L3Interface)
			}
			if vlan.SourceSnippetHash == "" {
				t.Error("Engineering VLAN missing source snippet hash")
			}
		}
	}

	if !found {
		t.Error("Engineering VLAN not found")
	}
}

// Test interface parsing with ethernet-switching family
func TestParseJunosInterfaces(t *testing.T) {
	config := `
interfaces {
    ge-0/0/1 {
        description "Access port to workstation";
        unit 0 {
            family ethernet-switching {
                interface-mode access;
                vlan {
                    members Engineering;
                }
            }
        }
    }
    ge-0/0/2 {
        description "Trunk to distribution switch";
        unit 0 {
            family ethernet-switching {
                interface-mode trunk;
                vlan {
                    members [ Engineering Sales ];
                }
                native-vlan-id 1;
            }
        }
    }
    ge-0/0/5 {
        disable;
        unit 0 {
            family ethernet-switching {
                interface-mode access;
                vlan {
                    members Engineering;
                }
            }
        }
    }
}
`

	interfaces, err := ParseJunosInterfaces(config)
	if err != nil {
		t.Fatalf("ParseJunosInterfaces failed: %v", err)
	}

	if len(interfaces) != 3 {
		t.Fatalf("Expected 3 interfaces, got %d", len(interfaces))
	}

	// Check ge-0/0/1 (access port)
	found := false
	for _, iface := range interfaces {
		if iface.InterfaceName == "ge-0/0/1" {
			found = true
			if iface.Description != "Access port to workstation" {
				t.Errorf("ge-0/0/1 description: expected 'Access port to workstation', got '%s'", iface.Description)
			}
			if iface.PortMode != "access" {
				t.Errorf("ge-0/0/1 mode: expected 'access', got '%s'", iface.PortMode)
			}
			if iface.AccessVLAN != "Engineering" {
				t.Errorf("ge-0/0/1 access VLAN: expected 'Engineering', got '%s'", iface.AccessVLAN)
			}
			if iface.Unit != 0 {
				t.Errorf("ge-0/0/1 unit: expected 0, got %d", iface.Unit)
			}
			if iface.Disabled {
				t.Error("ge-0/0/1 should not be disabled")
			}
		}
	}

	if !found {
		t.Error("ge-0/0/1 not found")
	}

	// Check ge-0/0/2 (trunk port)
	found = false
	for _, iface := range interfaces {
		if iface.InterfaceName == "ge-0/0/2" {
			found = true
			if iface.PortMode != "trunk" {
				t.Errorf("ge-0/0/2 mode: expected 'trunk', got '%s'", iface.PortMode)
			}
			if len(iface.TrunkAllowedVLANs) != 2 {
				t.Errorf("ge-0/0/2 allowed VLANs: expected 2, got %d", len(iface.TrunkAllowedVLANs))
			}
			// Check for Engineering and Sales in allowed VLANs
			hasEngineering := false
			hasSales := false
			for _, vlan := range iface.TrunkAllowedVLANs {
				if vlan == "Engineering" {
					hasEngineering = true
				}
				if vlan == "Sales" {
					hasSales = true
				}
			}
			if !hasEngineering || !hasSales {
				t.Error("ge-0/0/2 should have Engineering and Sales in allowed VLANs")
			}
		}
	}

	if !found {
		t.Error("ge-0/0/2 not found")
	}

	// Check ge-0/0/5 (disabled)
	found = false
	for _, iface := range interfaces {
		if iface.InterfaceName == "ge-0/0/5" {
			found = true
			if !iface.Disabled {
				t.Error("ge-0/0/5 should be disabled")
			}
		}
	}

	if !found {
		t.Error("ge-0/0/5 not found")
	}
}

// Test Aggregated Ethernet (LAG) parsing
func TestParseJunosAggregatedEthernet(t *testing.T) {
	config := `
interfaces {
    ge-0/0/10 {
        description "Member of ae0";
        ether-options {
            802.3ad ae0;
        }
    }
    ge-0/0/11 {
        ether-options {
            802.3ad ae0;
        }
    }
    ae0 {
        description "Uplink to core";
        aggregated-ether-options {
            lacp {
                active;
            }
        }
        unit 0 {
            family ethernet-switching {
                interface-mode trunk;
                vlan {
                    members [ Engineering Sales Guest ];
                }
            }
        }
    }
}
`

	aes, err := ParseJunosAggregatedEthernet(config)
	if err != nil {
		t.Fatalf("ParseJunosAggregatedEthernet failed: %v", err)
	}

	if len(aes) != 1 {
		t.Fatalf("Expected 1 AE interface, got %d", len(aes))
	}

	ae := aes[0]

	if ae.AEName != "ae0" {
		t.Errorf("AE name: expected 'ae0', got '%s'", ae.AEName)
	}

	if ae.Description != "Uplink to core" {
		t.Errorf("AE description: expected 'Uplink to core', got '%s'", ae.Description)
	}

	if ae.LACPMode != "active" {
		t.Errorf("LACP mode: expected 'active', got '%s'", ae.LACPMode)
	}

	if ae.PortMode != "trunk" {
		t.Errorf("Port mode: expected 'trunk', got '%s'", ae.PortMode)
	}

	if len(ae.TrunkAllowedVLANs) != 3 {
		t.Errorf("Allowed VLANs: expected 3, got %d", len(ae.TrunkAllowedVLANs))
	}

	// Member discovery should find ge-0/0/10 and ge-0/0/11
	if len(ae.MemberInterfaces) != 2 {
		t.Errorf("Member interfaces: expected 2, got %d", len(ae.MemberInterfaces))
	}

	hasGe10 := false
	hasGe11 := false
	for _, member := range ae.MemberInterfaces {
		if member == "ge-0/0/10" {
			hasGe10 = true
		}
		if member == "ge-0/0/11" {
			hasGe11 = true
		}
	}

	if !hasGe10 || !hasGe11 {
		t.Error("Expected members ge-0/0/10 and ge-0/0/11")
	}
}

// Test IRB parsing
func TestParseJunosIRBs(t *testing.T) {
	config := `
interfaces {
    irb {
        unit 10 {
            family inet {
                address 192.168.10.1/24;
            }
        }
        unit 20 {
            description "Sales network gateway";
            family inet {
                address 192.168.20.1/24;
            }
        }
        unit 99 {
            disable;
            family inet {
                address 192.168.99.1/24;
            }
        }
    }
}

vlans {
    Engineering {
        vlan-id 10;
        l3-interface irb.10;
    }
    Sales {
        vlan-id 20;
        l3-interface irb.20;
    }
    Guest {
        vlan-id 99;
        l3-interface irb.99;
    }
}
`

	irbs, err := ParseJunosIRBs(config)
	if err != nil {
		t.Fatalf("ParseJunosIRBs failed: %v", err)
	}

	if len(irbs) != 3 {
		t.Fatalf("Expected 3 IRB interfaces, got %d", len(irbs))
	}

	// Check irb.10
	found := false
	for _, irb := range irbs {
		if irb.IRBName == "irb.10" {
			found = true
			if irb.Unit != 10 {
				t.Errorf("irb.10 unit: expected 10, got %d", irb.Unit)
			}
			if irb.IPAddress != "192.168.10.1/24" {
				t.Errorf("irb.10 IP: expected '192.168.10.1/24', got '%s'", irb.IPAddress)
			}
			if irb.VLANID != 10 {
				t.Errorf("irb.10 VLAN ID: expected 10, got %d", irb.VLANID)
			}
			if irb.VLANName != "Engineering" {
				t.Errorf("irb.10 VLAN name: expected 'Engineering', got '%s'", irb.VLANName)
			}
			if irb.Disabled {
				t.Error("irb.10 should not be disabled")
			}
		}
	}

	if !found {
		t.Error("irb.10 not found")
	}

	// Check irb.99 (disabled)
	found = false
	for _, irb := range irbs {
		if irb.IRBName == "irb.99" {
			found = true
			if !irb.Disabled {
				t.Error("irb.99 should be disabled")
			}
		}
	}

	if !found {
		t.Error("irb.99 not found")
	}
}

// Test global L2 controls parsing
func TestParseJunosGlobalL2Controls(t *testing.T) {
	config := `
ethernet-switching-options {
    secure-access-port {
        dhcp-trusted-port ge-0/0/1.0;
        dhcp-trusted-port ge-0/0/2.0;
    }
}

vlans {
    Engineering {
        vlan-id 10;
        forwarding-options {
            dhcp-security {
                group dhcp-group1;
            }
        }
    }
    Sales {
        vlan-id 20;
        forwarding-options {
            dhcp-security {
                group dhcp-group1;
            }
        }
    }
}

protocols {
    dot1x {
        authenticator {
            authentication-profile-name auth-profile;
            interface {
                ge-0/0/5.0;
                ge-0/0/6.0;
            }
        }
    }
}

access {
    radius-server {
        10.1.1.10 {
            secret "$9$abc123";
        }
        10.1.1.11 {
            secret "$9$def456";
        }
    }
}
`

	controls, err := ParseJunosGlobalL2Controls(config)
	if err != nil {
		t.Fatalf("ParseJunosGlobalL2Controls failed: %v", err)
	}

	if !controls.DHCPSecurityEnabled {
		t.Error("DHCP security should be enabled")
	}

	if len(controls.DHCPSecurityVLANs) != 2 {
		t.Errorf("DHCP security VLANs: expected 2, got %d", len(controls.DHCPSecurityVLANs))
	}

	if !controls.Dot1XEnabled {
		t.Error("802.1X should be enabled")
	}

	if len(controls.RADIUSServers) != 2 {
		t.Errorf("RADIUS servers: expected 2, got %d", len(controls.RADIUSServers))
	}

	hasRadius1 := false
	hasRadius2 := false
	for _, server := range controls.RADIUSServers {
		if server == "10.1.1.10" {
			hasRadius1 = true
		}
		if server == "10.1.1.11" {
			hasRadius2 = true
		}
	}

	if !hasRadius1 || !hasRadius2 {
		t.Error("Expected RADIUS servers 10.1.1.10 and 10.1.1.11")
	}
}

// Test ethernet-switching table parsing
func TestParseJunosEthernetSwitchingTable(t *testing.T) {
	output := `
Ethernet switching table : 5 entries, 3 learned
Routing instance : default-switch

  VLAN              MAC address       Type         Age Interfaces
  Engineering       00:11:22:33:44:55 Learn          0 ge-0/0/1.0
  Engineering       00:11:22:33:44:66 Learn        120 ge-0/0/2.0
  Sales             00:aa:bb:cc:dd:ee Static         - ge-0/0/3.0
  Sales             00:aa:bb:cc:dd:ff Learn         45 ae0.0
  Guest             00:99:88:77:66:55 Learn         10 ge-0/0/10.0
`

	entries, err := ParseJunosEthernetSwitchingTable(output)
	if err != nil {
		t.Fatalf("ParseJunosEthernetSwitchingTable failed: %v", err)
	}

	if len(entries) != 5 {
		t.Fatalf("Expected 5 FDB entries, got %d", len(entries))
	}

	// Check static entry
	foundStatic := false
	for _, entry := range entries {
		if entry.MACAddress == "00:AA:BB:CC:DD:EE" {
			foundStatic = true
			if !entry.IsStatic {
				t.Error("Entry 00:AA:BB:CC:DD:EE should be static")
			}
			if entry.VLANName != "Sales" {
				t.Errorf("Static entry VLAN: expected 'Sales', got '%s'", entry.VLANName)
			}
			if entry.InterfaceName != "ge-0/0/3.0" {
				t.Errorf("Static entry interface: expected 'ge-0/0/3.0', got '%s'", entry.InterfaceName)
			}
		}
	}

	if !foundStatic {
		t.Error("Static MAC entry not found")
	}

	// Check dynamic entry
	foundDynamic := false
	for _, entry := range entries {
		if entry.MACAddress == "00:11:22:33:44:55" {
			foundDynamic = true
			if entry.IsStatic {
				t.Error("Entry 00:11:22:33:44:55 should be dynamic")
			}
			if entry.VLANName != "Engineering" {
				t.Errorf("Dynamic entry VLAN: expected 'Engineering', got '%s'", entry.VLANName)
			}
		}
	}

	if !foundDynamic {
		t.Error("Dynamic MAC entry not found")
	}
}

// Test LLDP neighbor parsing
func TestParseJunosLLDPNeighbors(t *testing.T) {
	output := `
Local Interface    Parent Interface    Chassis Id          Port info          System Name
ge-0/0/1           -                   00:11:22:33:44:55   Gi0/1              switch-core-01
ge-0/0/2           -                   00:11:22:33:44:66   Ethernet1/1        switch-core-02
ae0                -                   00:aa:bb:cc:dd:ee   Te1/1/1            switch-dist-01
`

	neighbors, err := ParseJunosLLDPNeighbors(output)
	if err != nil {
		t.Fatalf("ParseJunosLLDPNeighbors failed: %v", err)
	}

	if len(neighbors) != 3 {
		t.Fatalf("Expected 3 LLDP neighbors, got %d", len(neighbors))
	}

	// Check first neighbor
	found := false
	for _, neighbor := range neighbors {
		if neighbor.LocalInterface == "ge-0/0/1" {
			found = true
			if neighbor.RemoteChassisID != "00:11:22:33:44:55" {
				t.Errorf("Neighbor chassis ID: expected '00:11:22:33:44:55', got '%s'", neighbor.RemoteChassisID)
			}
			if neighbor.RemotePort != "Gi0/1" {
				t.Errorf("Neighbor port: expected 'Gi0/1', got '%s'", neighbor.RemotePort)
			}
			if neighbor.RemoteSystemName != "switch-core-01" {
				t.Errorf("Neighbor system name: expected 'switch-core-01', got '%s'", neighbor.RemoteSystemName)
			}
		}
	}

	if !found {
		t.Error("LLDP neighbor on ge-0/0/1 not found")
	}
}

// Test LLDP detail parsing
func TestParseJunosLLDPNeighborsDetail(t *testing.T) {
	output := `
Local Interface: ge-0/0/1
Parent Interface: -
Chassis type       : Mac address
Chassis ID         : 00:11:22:33:44:55
Port type          : Interface name
Port ID            : Gi0/1
Port description   : Uplink to access switch
System name        : switch-core-01
System description : Cisco IOS Software, Version 15.2(4)E

Local Interface: ge-0/0/2
Parent Interface: -
Chassis type       : Mac address
Chassis ID         : 00:aa:bb:cc:dd:ee
Port type          : Interface name
Port ID            : Ethernet1/1
Port description   : Distribution uplink
System name        : switch-dist-01
System description : Aruba JL322A 2930F-48G
`

	neighbors, err := ParseJunosLLDPNeighborsDetail(output)
	if err != nil {
		t.Fatalf("ParseJunosLLDPNeighborsDetail failed: %v", err)
	}

	if len(neighbors) != 2 {
		t.Fatalf("Expected 2 LLDP neighbors, got %d", len(neighbors))
	}

	// Check first neighbor with full details
	found := false
	for _, neighbor := range neighbors {
		if neighbor.LocalInterface == "ge-0/0/1" {
			found = true
			if neighbor.RemoteChassisID != "00:11:22:33:44:55" {
				t.Errorf("Chassis ID: expected '00:11:22:33:44:55', got '%s'", neighbor.RemoteChassisID)
			}
			if neighbor.RemotePort != "Gi0/1" {
				t.Errorf("Port ID: expected 'Gi0/1', got '%s'", neighbor.RemotePort)
			}
			if neighbor.RemotePortDescription != "Uplink to access switch" {
				t.Errorf("Port description: expected 'Uplink to access switch', got '%s'", neighbor.RemotePortDescription)
			}
			if neighbor.RemoteSystemName != "switch-core-01" {
				t.Errorf("System name: expected 'switch-core-01', got '%s'", neighbor.RemoteSystemName)
			}
			if neighbor.RemoteSystemDescription != "Cisco IOS Software, Version 15.2(4)E" {
				t.Errorf("System description mismatch, got '%s'", neighbor.RemoteSystemDescription)
			}
		}
	}

	if !found {
		t.Error("Detailed LLDP neighbor on ge-0/0/1 not found")
	}
}

// Test interface status parsing
func TestParseJunosInterfaceStatus(t *testing.T) {
	output := `
ge-0/0/0                up    up
ge-0/0/0.0              up    up   eth-switch
ge-0/0/1                up    up
ge-0/0/1.0              up    up   eth-switch
ge-0/0/5                down  down
ge-0/0/5.0              down  down eth-switch
ae0                     up    up
ae0.0                   up    up   eth-switch
`

	status, err := ParseJunosInterfaceStatus(output)
	if err != nil {
		t.Fatalf("ParseJunosInterfaceStatus failed: %v", err)
	}

	if len(status) != 8 {
		t.Fatalf("Expected 8 interface status entries, got %d", len(status))
	}

	// Check ge-0/0/1.0 (up/up)
	if ifStatus, exists := status["ge-0/0/1.0"]; exists {
		if ifStatus.AdminStatus != "up" {
			t.Errorf("ge-0/0/1.0 admin status: expected 'up', got '%s'", ifStatus.AdminStatus)
		}
		if ifStatus.OperStatus != "up" {
			t.Errorf("ge-0/0/1.0 oper status: expected 'up', got '%s'", ifStatus.OperStatus)
		}
		if ifStatus.Description != "eth-switch" {
			t.Errorf("ge-0/0/1.0 description: expected 'eth-switch', got '%s'", ifStatus.Description)
		}
	} else {
		t.Error("ge-0/0/1.0 status not found")
	}

	// Check ge-0/0/5 (down/down)
	if ifStatus, exists := status["ge-0/0/5"]; exists {
		if ifStatus.AdminStatus != "down" {
			t.Errorf("ge-0/0/5 admin status: expected 'down', got '%s'", ifStatus.AdminStatus)
		}
		if ifStatus.OperStatus != "down" {
			t.Errorf("ge-0/0/5 oper status: expected 'down', got '%s'", ifStatus.OperStatus)
		}
	} else {
		t.Error("ge-0/0/5 status not found")
	}
}

// Test DHCP security binding parsing
func TestParseJunosDHCPSecurityBinding(t *testing.T) {
	output := `
MAC Address        IP Address      Type     VLAN   Interface
00:11:22:33:44:55  192.168.10.100  DHCP     Engineering  ge-0/0/1.0
00:11:22:33:44:66  192.168.10.101  DHCP     Engineering  ge-0/0/2.0
00:aa:bb:cc:dd:ee  192.168.10.10   Static   Engineering  ge-0/0/3.0
00:99:88:77:66:55  192.168.20.50   DHCP     Sales        ae0.0
`

	bindings, err := ParseJunosDHCPSecurityBinding(output)
	if err != nil {
		t.Fatalf("ParseJunosDHCPSecurityBinding failed: %v", err)
	}

	if len(bindings) != 4 {
		t.Fatalf("Expected 4 DHCP bindings, got %d", len(bindings))
	}

	// Check DHCP binding
	found := false
	for _, binding := range bindings {
		if binding.MACAddress == "00:11:22:33:44:55" {
			found = true
			if binding.IPAddress != "192.168.10.100" {
				t.Errorf("IP address: expected '192.168.10.100', got '%s'", binding.IPAddress)
			}
			if binding.VLANName != "Engineering" {
				t.Errorf("VLAN: expected 'Engineering', got '%s'", binding.VLANName)
			}
			if binding.InterfaceName != "ge-0/0/1.0" {
				t.Errorf("Interface: expected 'ge-0/0/1.0', got '%s'", binding.InterfaceName)
			}
			if binding.BindingType != "DHCP" {
				t.Errorf("Binding type: expected 'DHCP', got '%s'", binding.BindingType)
			}
		}
	}

	if !found {
		t.Error("DHCP binding for 00:11:22:33:44:55 not found")
	}

	// Check static binding
	found = false
	for _, binding := range bindings {
		if binding.MACAddress == "00:AA:BB:CC:DD:EE" {
			found = true
			if binding.BindingType != "Static" {
				t.Errorf("Binding type: expected 'Static', got '%s'", binding.BindingType)
			}
		}
	}

	if !found {
		t.Error("Static binding for 00:AA:BB:CC:DD:EE not found")
	}
}

// Test 802.1X session parsing
func TestParseJunosDot1XSessions(t *testing.T) {
	output := `
Interface: ge-0/0/5.0
  Role: Authenticator
  Administrative state: Auto
  Operational state: Authenticated
  Quiet Period: 60
  Transmit Period: 30
  Supplicant MAC: 00:11:22:33:44:55
  Session Timeout: 3600
  Reauthentication Enabled: Yes
  Reauthentication Period: 3600

Interface: ge-0/0/6.0
  Role: Authenticator
  Administrative state: Auto
  Operational state: Connecting
  Quiet Period: 60
  Transmit Period: 30
  Supplicant MAC: 00:aa:bb:cc:dd:ee
  Session Timeout: 1800
  Reauthentication Enabled: No
  Reauthentication Period: 0
`

	sessions, err := ParseJunosDot1XSessions(output)
	if err != nil {
		t.Fatalf("ParseJunosDot1XSessions failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("Expected 2 802.1X sessions, got %d", len(sessions))
	}

	// Check authenticated session
	found := false
	for _, session := range sessions {
		if session.InterfaceName == "ge-0/0/5.0" {
			found = true
			if session.SupplicantMAC != "00:11:22:33:44:55" {
				t.Errorf("Supplicant MAC: expected '00:11:22:33:44:55', got '%s'", session.SupplicantMAC)
			}
			if session.AuthenticationState != "Authenticated" {
				t.Errorf("Auth state: expected 'Authenticated', got '%s'", session.AuthenticationState)
			}
			if session.SessionTimeout != 3600 {
				t.Errorf("Session timeout: expected 3600, got %d", session.SessionTimeout)
			}
			if !session.ReauthEnabled {
				t.Error("Reauthentication should be enabled")
			}
			if session.ReauthPeriod != 3600 {
				t.Errorf("Reauth period: expected 3600, got %d", session.ReauthPeriod)
			}
		}
	}

	if !found {
		t.Error("802.1X session on ge-0/0/5.0 not found")
	}

	// Check connecting session
	found = false
	for _, session := range sessions {
		if session.InterfaceName == "ge-0/0/6.0" {
			found = true
			if session.AuthenticationState != "Connecting" {
				t.Errorf("Auth state: expected 'Connecting', got '%s'", session.AuthenticationState)
			}
			if session.ReauthEnabled {
				t.Error("Reauthentication should be disabled")
			}
		}
	}

	if !found {
		t.Error("802.1X session on ge-0/0/6.0 not found")
	}
}

// Test VLAN summary parsing
func TestParseJunosVLANSummary(t *testing.T) {
	output := `
Name                 Tag     Interfaces
Engineering          10      ge-0/0/1.0, ge-0/0/2.0, ae0.0
Sales                20      ge-0/0/3.0, ge-0/0/4.0
Guest                99      ge-0/0/10.0
default              1
`

	vlans, err := ParseJunosVLANSummary(output)
	if err != nil {
		t.Fatalf("ParseJunosVLANSummary failed: %v", err)
	}

	if len(vlans) != 4 {
		t.Fatalf("Expected 4 VLANs, got %d", len(vlans))
	}

	// Check Engineering VLAN
	if vlan, exists := vlans["Engineering"]; exists {
		if vlan.VLANID != 10 {
			t.Errorf("Engineering VLAN ID: expected 10, got %d", vlan.VLANID)
		}
		if len(vlan.MemberInterfaces) != 3 {
			t.Errorf("Engineering members: expected 3, got %d", len(vlan.MemberInterfaces))
		}
		hasGe1 := false
		hasAe0 := false
		for _, iface := range vlan.MemberInterfaces {
			if iface == "ge-0/0/1.0" {
				hasGe1 = true
			}
			if iface == "ae0.0" {
				hasAe0 = true
			}
		}
		if !hasGe1 || !hasAe0 {
			t.Error("Engineering VLAN missing expected member interfaces")
		}
	} else {
		t.Error("Engineering VLAN not found")
	}

	// Check default VLAN (no interfaces)
	if vlan, exists := vlans["default"]; exists {
		if vlan.VLANID != 1 {
			t.Errorf("Default VLAN ID: expected 1, got %d", vlan.VLANID)
		}
		if len(vlan.MemberInterfaces) != 0 {
			t.Errorf("Default VLAN should have no members, got %d", len(vlan.MemberInterfaces))
		}
	} else {
		t.Error("Default VLAN not found")
	}
}

// Integration test: Full configuration parsing
func TestFullJunosConfigParsing(t *testing.T) {
	config := `
vlans {
    Engineering {
        vlan-id 10;
        description "Engineering Department";
        l3-interface irb.10;
    }
    Sales {
        vlan-id 20;
    }
}

interfaces {
    ge-0/0/1 {
        description "Access port";
        unit 0 {
            family ethernet-switching {
                interface-mode access;
                vlan {
                    members Engineering;
                }
            }
        }
    }
    ge-0/0/10 {
        ether-options {
            802.3ad ae0;
        }
    }
    ge-0/0/11 {
        ether-options {
            802.3ad ae0;
        }
    }
    ae0 {
        description "LAG uplink";
        aggregated-ether-options {
            lacp {
                active;
            }
        }
        unit 0 {
            family ethernet-switching {
                interface-mode trunk;
                vlan {
                    members [ Engineering Sales ];
                }
            }
        }
    }
    irb {
        unit 10 {
            family inet {
                address 192.168.10.1/24;
            }
        }
    }
}
`

	// Parse all components
	vlans, err := ParseJunosVLANs(config)
	if err != nil {
		t.Fatalf("Failed to parse VLANs: %v", err)
	}

	interfaces, err := ParseJunosInterfaces(config)
	if err != nil {
		t.Fatalf("Failed to parse interfaces: %v", err)
	}

	aes, err := ParseJunosAggregatedEthernet(config)
	if err != nil {
		t.Fatalf("Failed to parse AE interfaces: %v", err)
	}

	irbs, err := ParseJunosIRBs(config)
	if err != nil {
		t.Fatalf("Failed to parse IRBs: %v", err)
	}

	// Validate results
	if len(vlans) != 2 {
		t.Errorf("Expected 2 VLANs, got %d", len(vlans))
	}

	if len(interfaces) != 1 { // Only ge-0/0/1 has ethernet-switching config
		t.Errorf("Expected 1 interface, got %d", len(interfaces))
	}

	if len(aes) != 1 {
		t.Errorf("Expected 1 AE interface, got %d", len(aes))
	}

	if len(aes) > 0 && len(aes[0].MemberInterfaces) != 2 {
		t.Errorf("Expected 2 AE members, got %d", len(aes[0].MemberInterfaces))
	}

	if len(irbs) != 1 {
		t.Errorf("Expected 1 IRB interface, got %d", len(irbs))
	}

	t.Logf("Full config parsing successful: %d VLANs, %d interfaces, %d AEs, %d IRBs",
		len(vlans), len(interfaces), len(aes), len(irbs))
}
