package cisco

import (
	"testing"
)

// ===== P0 CRITICAL PARSER TESTS =====

// Test 1: ASA Interface Parser
func TestParseASAInterface(t *testing.T) {
	output := `Interface GigabitEthernet0/0 "outside", is up, line protocol is up
  Hardware is i82546GB rev03, BW 1000 Mbps, DLY 10 usec
  MAC address 0050.56bb.e14c, MTU 1500
  IP address 192.168.1.1, subnet mask 255.255.255.0
  
Interface GigabitEthernet0/1 "inside", is up, line protocol is up
  Hardware is i82546GB rev03, BW 1000 Mbps, DLY 10 usec
  MAC address 0050.56bb.e14d, MTU 1500
  IP address 10.0.0.1, subnet mask 255.255.255.0
  
Interface GigabitEthernet0/2 "dmz", is down, line protocol is down
  Hardware is i82546GB rev03, BW 1000 Mbps, DLY 10 usec
  MAC address 0050.56bb.e14e, MTU 1500`

	interfaces, err := ParseASAInterface(output)
	if err != nil {
		t.Fatalf("ParseASAInterface failed: %v", err)
	}

	if len(interfaces) != 3 {
		t.Fatalf("Expected 3 interfaces, got %d", len(interfaces))
	}

	// Check first interface
	if interfaces[0].Name != "GigabitEthernet0/0" {
		t.Errorf("Expected Name GigabitEthernet0/0, got %s", interfaces[0].Name)
	}
	if interfaces[0].Nameif != "outside" {
		t.Errorf("Expected Nameif outside, got %s", interfaces[0].Nameif)
	}
	if interfaces[0].IPAddress != "192.168.1.1" {
		t.Errorf("Expected IP 192.168.1.1, got %s", interfaces[0].IPAddress)
	}
	if interfaces[0].MACAddress != "00:50:56:bb:e1:4c" {
		t.Errorf("Expected MAC 00:50:56:bb:e1:4c, got %s", interfaces[0].MACAddress)
	}
	if interfaces[0].AdminState != "up" {
		t.Errorf("Expected AdminState up, got %s", interfaces[0].AdminState)
	}
	if interfaces[0].Bandwidth != 1000 {
		t.Errorf("Expected Bandwidth 1000, got %d", interfaces[0].Bandwidth)
	}

	// Check third interface (down)
	if interfaces[2].AdminState != "down" {
		t.Errorf("Expected AdminState down for interface 3, got %s", interfaces[2].AdminState)
	}
}

// Test 2: ASA Route Parser
func TestParseASARoute(t *testing.T) {
	output := `Codes: C - connected, S - static, I - IGRP, R - RIP, M - mobile, B - BGP
Gateway of last resort is 192.168.1.254 to network 0.0.0.0

C    192.168.1.0 255.255.255.0 is directly connected, outside
C    10.0.0.0 255.255.255.0 is directly connected, inside
S    0.0.0.0 0.0.0.0 [1/0] via 192.168.1.254, outside
S    172.16.0.0 255.255.0.0 [1/0] via 10.0.0.254, inside`

	routes, err := ParseASARoute(output)
	if err != nil {
		t.Fatalf("ParseASARoute failed: %v", err)
	}

	if len(routes) != 4 {
		t.Fatalf("Expected 4 routes, got %d", len(routes))
	}

	// Check connected route
	if routes[0].RouteType != "C" {
		t.Errorf("Expected RouteType C, got %s", routes[0].RouteType)
	}
	if routes[0].Network != "192.168.1.0" {
		t.Errorf("Expected Network 192.168.1.0, got %s", routes[0].Network)
	}
	if routes[0].Interface != "outside" {
		t.Errorf("Expected Interface outside, got %s", routes[0].Interface)
	}

	// Check static route
	if routes[2].RouteType != "S" {
		t.Errorf("Expected RouteType S, got %s", routes[2].RouteType)
	}
	if routes[2].NextHop != "192.168.1.254" {
		t.Errorf("Expected NextHop 192.168.1.254, got %s", routes[2].NextHop)
	}
	if routes[2].AdminDistance != 1 {
		t.Errorf("Expected AdminDistance 1, got %d", routes[2].AdminDistance)
	}
}

// Test 3: ASA ARP Parser
func TestParseASAARP(t *testing.T) {
	output := `outside 192.168.1.254 0050.56bb.0001 3599
inside 10.0.0.10 00:11:22:33:44:55 3599
dmz 172.16.1.50 00:aa:bb:cc:dd:ee 3599`

	entries, err := ParseASAARP(output)
	if err != nil {
		t.Fatalf("ParseASAARP failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("Expected 3 ARP entries, got %d", len(entries))
	}

	// Check first entry
	if entries[0].Interface != "outside" {
		t.Errorf("Expected Interface outside, got %s", entries[0].Interface)
	}
	if entries[0].IPAddress != "192.168.1.254" {
		t.Errorf("Expected IP 192.168.1.254, got %s", entries[0].IPAddress)
	}
	if entries[0].MACAddress != "00:50:56:bb:00:01" {
		t.Errorf("Expected MAC 00:50:56:bb:00:01, got %s", entries[0].MACAddress)
	}
	if entries[0].Age != 3599 {
		t.Errorf("Expected Age 3599, got %d", entries[0].Age)
	}
}

// Test 4: ASA Xlate Parser (CRITICAL)
func TestParseASAXlate(t *testing.T) {
	output := `2 in use, 2 most used
Flags: D - DNS, i - dynamic, r - portmap, s - static, I - identity, T - twice
NAT from inside:10.0.0.10 to outside:192.168.1.100
    flags s idle 0:00:05 timeout 0:00:00
NAT from inside:10.0.0.20/443 to outside:192.168.1.101/443
    flags si idle 0:00:12 timeout 3:00:00`

	entries, err := ParseASAXlate(output)
	if err != nil {
		t.Fatalf("ParseASAXlate failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 xlate entries, got %d", len(entries))
	}

	// Check first entry (static NAT without ports)
	if entries[0].SourceZone != "inside" {
		t.Errorf("Expected SourceZone inside, got %s", entries[0].SourceZone)
	}
	if entries[0].SourceIP != "10.0.0.10" {
		t.Errorf("Expected SourceIP 10.0.0.10, got %s", entries[0].SourceIP)
	}
	if entries[0].DestZone != "outside" {
		t.Errorf("Expected DestZone outside, got %s", entries[0].DestZone)
	}
	if entries[0].DestIP != "192.168.1.100" {
		t.Errorf("Expected DestIP 192.168.1.100, got %s", entries[0].DestIP)
	}
	if entries[0].Flags != "s" {
		t.Errorf("Expected Flags s, got %s", entries[0].Flags)
	}

	// Check second entry (with ports)
	if entries[1].SourcePort != 443 {
		t.Errorf("Expected SourcePort 443, got %d", entries[1].SourcePort)
	}
	if entries[1].DestPort != 443 {
		t.Errorf("Expected DestPort 443, got %d", entries[1].DestPort)
	}
	if entries[1].Flags != "si" {
		t.Errorf("Expected Flags si, got %s", entries[1].Flags)
	}
}

// Test 5: ASA NAT Policy Parser
func TestParseASANAT(t *testing.T) {
	output := `Manual NAT Policies (Section 1)
1 (inside) to (outside) source static obj-10.0.0.10 obj-192.168.1.100
    translate_hits = 150, untranslate_hits = 145

Auto NAT Policies (Section 2)
1 (inside) to (outside) source dynamic inside-network interface
    translate_hits = 5000`

	rules, err := ParseASANAT(output)
	if err != nil {
		t.Fatalf("ParseASANAT failed: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("Expected 2 NAT rules, got %d", len(rules))
	}

	// Check manual NAT
	if rules[0].Section != "Manual NAT" {
		t.Errorf("Expected Section 'Manual NAT', got %s", rules[0].Section)
	}
	if rules[0].NATType != "static" {
		t.Errorf("Expected NATType static, got %s", rules[0].NATType)
	}
	if rules[0].TranslateHits != 150 {
		t.Errorf("Expected TranslateHits 150, got %d", rules[0].TranslateHits)
	}
	if rules[0].UntranslateHits != 145 {
		t.Errorf("Expected UntranslateHits 145, got %d", rules[0].UntranslateHits)
	}

	// Check auto NAT
	if rules[1].Section != "Auto NAT" {
		t.Errorf("Expected Section 'Auto NAT', got %s", rules[1].Section)
	}
	if rules[1].NATType != "dynamic" {
		t.Errorf("Expected NATType dynamic, got %s", rules[1].NATType)
	}
	if rules[1].TranslateHits != 5000 {
		t.Errorf("Expected TranslateHits 5000, got %d", rules[1].TranslateHits)
	}
}

// Test 6: ASA Connection Parser
func TestParseASAConn(t *testing.T) {
	output := `TCP outside 192.168.1.50:54321 inside 10.0.0.10:443, idle 0:00:05, bytes 5000, flags UIO
UDP inside 10.0.0.10:53 outside 8.8.8.8:53, idle 0:00:01, bytes 150, flags -`

	connections, err := ParseASAConn(output)
	if err != nil {
		t.Fatalf("ParseASAConn failed: %v", err)
	}

	if len(connections) != 2 {
		t.Fatalf("Expected 2 connections, got %d", len(connections))
	}

	// Check TCP connection
	if connections[0].Protocol != "TCP" {
		t.Errorf("Expected Protocol TCP, got %s", connections[0].Protocol)
	}
	if connections[0].SourceZone != "outside" {
		t.Errorf("Expected SourceZone outside, got %s", connections[0].SourceZone)
	}
	if connections[0].SourceIP != "192.168.1.50" {
		t.Errorf("Expected SourceIP 192.168.1.50, got %s", connections[0].SourceIP)
	}
	if connections[0].SourcePort != 54321 {
		t.Errorf("Expected SourcePort 54321, got %d", connections[0].SourcePort)
	}
	if connections[0].DestPort != 443 {
		t.Errorf("Expected DestPort 443, got %d", connections[0].DestPort)
	}
	if connections[0].Bytes != 5000 {
		t.Errorf("Expected Bytes 5000, got %d", connections[0].Bytes)
	}
	if connections[0].Flags != "UIO" {
		t.Errorf("Expected Flags UIO, got %s", connections[0].Flags)
	}

	// Check UDP connection
	if connections[1].Protocol != "UDP" {
		t.Errorf("Expected Protocol UDP, got %s", connections[1].Protocol)
	}
}

// Test 7: ASA Local Host Parser
func TestParseASALocalHost(t *testing.T) {
	output := `Interface inside: 50 active, 100 maximum active, 0 denied
local host: <10.0.0.10>,
  TCP flow count/limit = 10/unlimited
  UDP flow count/limit = 5/unlimited
  TCP embryonic count = 0

Interface outside: 20 active, 100 maximum active, 0 denied
local host: <192.168.1.100>,
  TCP flow count/limit = 5/100
  UDP flow count/limit = 2/50
  TCP embryonic count = 1`

	hosts, err := ParseASALocalHost(output)
	if err != nil {
		t.Fatalf("ParseASALocalHost failed: %v", err)
	}

	if len(hosts) != 2 {
		t.Fatalf("Expected 2 local hosts, got %d", len(hosts))
	}

	// Check first host
	if hosts[0].Interface != "inside" {
		t.Errorf("Expected Interface inside, got %s", hosts[0].Interface)
	}
	if hosts[0].IPAddress != "10.0.0.10" {
		t.Errorf("Expected IPAddress 10.0.0.10, got %s", hosts[0].IPAddress)
	}
	if hosts[0].TCPFlowCount != 10 {
		t.Errorf("Expected TCPFlowCount 10, got %d", hosts[0].TCPFlowCount)
	}
	if hosts[0].TCPFlowLimit != "unlimited" {
		t.Errorf("Expected TCPFlowLimit unlimited, got %s", hosts[0].TCPFlowLimit)
	}

	// Check second host
	if hosts[1].TCPFlowLimit != "100" {
		t.Errorf("Expected TCPFlowLimit 100, got %s", hosts[1].TCPFlowLimit)
	}
	if hosts[1].TCPEmbryonicCount != 1 {
		t.Errorf("Expected TCPEmbryonicCount 1, got %d", hosts[1].TCPEmbryonicCount)
	}
}

// Test 8: ASA Nameif (Zone) Parser
func TestParseASANameif(t *testing.T) {
	output := `Interface                Name                     Security
GigabitEthernet0/0       outside                  0
GigabitEthernet0/1       inside                   100
GigabitEthernet0/2       dmz                      50`

	zones, err := ParseASANameif(output)
	if err != nil {
		t.Fatalf("ParseASANameif failed: %v", err)
	}

	if len(zones) != 3 {
		t.Fatalf("Expected 3 zones, got %d", len(zones))
	}

	// Check zones
	if zones[0].Interface != "GigabitEthernet0/0" {
		t.Errorf("Expected Interface GigabitEthernet0/0, got %s", zones[0].Interface)
	}
	if zones[0].ZoneName != "outside" {
		t.Errorf("Expected ZoneName outside, got %s", zones[0].ZoneName)
	}
	if zones[0].SecurityLevel != 0 {
		t.Errorf("Expected SecurityLevel 0, got %d", zones[0].SecurityLevel)
	}

	if zones[1].SecurityLevel != 100 {
		t.Errorf("Expected SecurityLevel 100 for inside, got %d", zones[1].SecurityLevel)
	}
	if zones[2].SecurityLevel != 50 {
		t.Errorf("Expected SecurityLevel 50 for dmz, got %d", zones[2].SecurityLevel)
	}
}

// ===== P1 HIGH PRIORITY PARSER TESTS =====

// Test 9: ASA ACL Parser
func TestParseASAACL(t *testing.T) {
	output := `access-list outside_access_in; 3 elements; name hash: 0x12345678
access-list outside_access_in line 1 extended permit tcp any host 192.168.1.100 eq https (hitcnt=150)
access-list outside_access_in line 2 extended permit tcp any host 192.168.1.101 eq ssh (hitcnt=50)
access-list outside_access_in line 3 extended deny ip any any (hitcnt=500)`

	entries, err := ParseASAACL(output)
	if err != nil {
		t.Fatalf("ParseASAACL failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("Expected 3 ACL entries, got %d", len(entries))
	}

	// Check first entry
	if entries[0].ACLName != "outside_access_in" {
		t.Errorf("Expected ACLName outside_access_in, got %s", entries[0].ACLName)
	}
	if entries[0].LineNumber != 1 {
		t.Errorf("Expected LineNumber 1, got %d", entries[0].LineNumber)
	}
	if entries[0].Action != "permit" {
		t.Errorf("Expected Action permit, got %s", entries[0].Action)
	}
	if entries[0].Protocol != "tcp" {
		t.Errorf("Expected Protocol tcp, got %s", entries[0].Protocol)
	}
	if entries[0].HitCount != 150 {
		t.Errorf("Expected HitCount 150, got %d", entries[0].HitCount)
	}
	if entries[0].HashID != "0x12345678" {
		t.Errorf("Expected HashID 0x12345678, got %s", entries[0].HashID)
	}

	// Check deny entry
	if entries[2].Action != "deny" {
		t.Errorf("Expected Action deny, got %s", entries[2].Action)
	}
	if entries[2].HitCount != 500 {
		t.Errorf("Expected HitCount 500, got %d", entries[2].HitCount)
	}
}

// Test 10: ASA Service Policy Parser
func TestParseASAServicePolicy(t *testing.T) {
	output := `Global policy:
  Service-policy: global_policy
    Class-map: inspection_default
      Inspect: dns preset_dns_map, packet 1000, drop 0, reset-drop 0
      Inspect: http, packet 5000, drop 10, reset-drop 5`

	policies, err := ParseASAServicePolicy(output)
	if err != nil {
		t.Fatalf("ParseASAServicePolicy failed: %v", err)
	}

	if len(policies) != 2 {
		t.Fatalf("Expected 2 service policies, got %d", len(policies))
	}

	// Check first policy
	if policies[0].PolicyName != "global_policy" {
		t.Errorf("Expected PolicyName global_policy, got %s", policies[0].PolicyName)
	}
	if policies[0].ClassMap != "inspection_default" {
		t.Errorf("Expected ClassMap inspection_default, got %s", policies[0].ClassMap)
	}
	if policies[0].Inspection != "dns" {
		t.Errorf("Expected Inspection dns, got %s", policies[0].Inspection)
	}
	if policies[0].Packets != 1000 {
		t.Errorf("Expected Packets 1000, got %d", policies[0].Packets)
	}
	if policies[0].Drops != 0 {
		t.Errorf("Expected Drops 0, got %d", policies[0].Drops)
	}

	// Check second policy
	if policies[1].Inspection != "http" {
		t.Errorf("Expected Inspection http, got %s", policies[1].Inspection)
	}
	if policies[1].ResetDrops != 5 {
		t.Errorf("Expected ResetDrops 5, got %d", policies[1].ResetDrops)
	}
}

// Test 11: ASA Failover Parser
func TestParseASAFailover(t *testing.T) {
	output := `Failover On
Failover unit Primary
Failover LAN Interface: Failover GigabitEthernet0/3 (up)
Unit Poll frequency 1 seconds, holdtime 15 seconds
Interface Poll frequency 5 seconds, holdtime 25 seconds
This host: Primary - Active
  Active time: 3600 (sec)
  Standby ready time: 300 (sec)
Other host: Secondary - Standby Ready`

	state, err := ParseASAFailover(output)
	if err != nil {
		t.Fatalf("ParseASAFailover failed: %v", err)
	}

	if !state.FailoverEnabled {
		t.Errorf("Expected FailoverEnabled true, got false")
	}
	if state.ThisUnit != "Primary" {
		t.Errorf("Expected ThisUnit Primary, got %s", state.ThisUnit)
	}
	if state.ThisState != "Active" {
		t.Errorf("Expected ThisState Active, got %s", state.ThisState)
	}
	if state.OtherUnit != "Secondary" {
		t.Errorf("Expected OtherUnit Secondary, got %s", state.OtherUnit)
	}
	if state.OtherState != "Standby Ready" {
		t.Errorf("Expected OtherState 'Standby Ready', got %s", state.OtherState)
	}
	if state.LANInterface != "GigabitEthernet0/3" {
		t.Errorf("Expected LANInterface GigabitEthernet0/3, got %s", state.LANInterface)
	}
	if state.LANStatus != "up" {
		t.Errorf("Expected LANStatus up, got %s", state.LANStatus)
	}
	if state.ActiveTime != 3600 {
		t.Errorf("Expected ActiveTime 3600, got %d", state.ActiveTime)
	}
}

// Test 12: ASA VPN Session Parser
func TestParseASAVPN(t *testing.T) {
	output := `Session Type: IPsec
Username     : user1                  Index    : 1
Public IP    : 203.0.113.50           Protocol : IKEv2 IPsec
Session ID   : 1000                   Duration : 0h:30m:15s
Bytes Tx     : 50000                  Bytes Rx : 150000

Session Type: SSL
Username     : user2                  Index    : 2
Public IP    : 203.0.113.51           Protocol : SSL-Tunnel
Session ID   : 1001                   Duration : 1h:15m:30s
Bytes Tx     : 120000                 Bytes Rx : 300000`

	sessions, err := ParseASAVPN(output)
	if err != nil {
		t.Fatalf("ParseASAVPN failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("Expected 2 VPN sessions, got %d", len(sessions))
	}

	// Check IPsec session
	if sessions[0].SessionType != "IPsec" {
		t.Errorf("Expected SessionType IPsec, got %s", sessions[0].SessionType)
	}
	if sessions[0].Username != "user1" {
		t.Errorf("Expected Username user1, got %s", sessions[0].Username)
	}
	if sessions[0].PublicIP != "203.0.113.50" {
		t.Errorf("Expected PublicIP 203.0.113.50, got %s", sessions[0].PublicIP)
	}
	if sessions[0].Protocol != "IKEv2 IPsec" {
		t.Errorf("Expected Protocol 'IKEv2 IPsec', got %s", sessions[0].Protocol)
	}
	if sessions[0].SessionID != 1000 {
		t.Errorf("Expected SessionID 1000, got %d", sessions[0].SessionID)
	}
	if sessions[0].Duration != "0h:30m:15s" {
		t.Errorf("Expected Duration 0h:30m:15s, got %s", sessions[0].Duration)
	}
	if sessions[0].BytesTx != 50000 {
		t.Errorf("Expected BytesTx 50000, got %d", sessions[0].BytesTx)
	}
	if sessions[0].BytesRx != 150000 {
		t.Errorf("Expected BytesRx 150000, got %d", sessions[0].BytesRx)
	}

	// Check SSL session
	if sessions[1].SessionType != "SSL" {
		t.Errorf("Expected SessionType SSL, got %s", sessions[1].SessionType)
	}
	if sessions[1].Username != "user2" {
		t.Errorf("Expected Username user2, got %s", sessions[1].Username)
	}
}

// Test 13: ASA Object Group Parser
func TestParseASAObjectGroup(t *testing.T) {
	output := `object-group network inside-networks
 network-object 10.0.0.0 255.255.255.0
 network-object 10.1.0.0 255.255.255.0
object-group service web-services
 service-object tcp eq www
 service-object tcp eq https
object-group protocol allowed-protocols
 protocol-object tcp
 protocol-object udp`

	groups, err := ParseASAObjectGroup(output)
	if err != nil {
		t.Fatalf("ParseASAObjectGroup failed: %v", err)
	}

	if len(groups) != 3 {
		t.Fatalf("Expected 3 object groups, got %d", len(groups))
	}

	// Check network object group
	if groups[0].Type != "network" {
		t.Errorf("Expected Type network, got %s", groups[0].Type)
	}
	if groups[0].Name != "inside-networks" {
		t.Errorf("Expected Name inside-networks, got %s", groups[0].Name)
	}
	if len(groups[0].Members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(groups[0].Members))
	}
	if groups[0].Members[0] != "10.0.0.0 255.255.255.0" {
		t.Errorf("Expected member '10.0.0.0 255.255.255.0', got %s", groups[0].Members[0])
	}

	// Check service object group
	if groups[1].Type != "service" {
		t.Errorf("Expected Type service, got %s", groups[1].Type)
	}
	if groups[1].Name != "web-services" {
		t.Errorf("Expected Name web-services, got %s", groups[1].Name)
	}
	if len(groups[1].Members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(groups[1].Members))
	}

	// Check protocol object group
	if groups[2].Type != "protocol" {
		t.Errorf("Expected Type protocol, got %s", groups[2].Type)
	}
}

// ===== EDGE CASE TESTS =====

// Test empty output
func TestParseASAInterface_EmptyOutput(t *testing.T) {
	interfaces, err := ParseASAInterface("")
	if err != nil {
		t.Fatalf("ParseASAInterface failed on empty output: %v", err)
	}
	if len(interfaces) != 0 {
		t.Errorf("Expected 0 interfaces for empty output, got %d", len(interfaces))
	}
}

// Test malformed output
func TestParseASARoute_MalformedOutput(t *testing.T) {
	output := `This is not a valid route output
Random text here
More random text`

	routes, err := ParseASARoute(output)
	if err != nil {
		t.Fatalf("ParseASARoute should not fail on malformed output: %v", err)
	}
	if len(routes) != 0 {
		t.Errorf("Expected 0 routes for malformed output, got %d", len(routes))
	}
}

// Test MAC address normalization
func TestMACAddressNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"0050.56bb.e14c", "00:50:56:bb:e1:4c"},
		{"00:50:56:bb:e1:4c", "00:50:56:bb:e1:4c"},
		{"00-50-56-bb-e1-4c", "00:50:56:bb:e1:4c"},
		{"005056bbe14c", "00:50:56:bb:e1:4c"},
	}

	for _, test := range tests {
		result := normalizeMACAddress(test.input)
		if result != test.expected {
			t.Errorf("normalizeMACAddress(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

// ===== BENCHMARK TESTS =====

func BenchmarkParseASAInterface(b *testing.B) {
	output := `Interface GigabitEthernet0/0 "outside", is up, line protocol is up
  Hardware is i82546GB rev03, BW 1000 Mbps, DLY 10 usec
  MAC address 0050.56bb.e14c, MTU 1500
  IP address 192.168.1.1, subnet mask 255.255.255.0`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseASAInterface(output)
	}
}

func BenchmarkParseASAXlate(b *testing.B) {
	output := `2 in use, 2 most used
Flags: D - DNS, i - dynamic, r - portmap, s - static, I - identity, T - twice
NAT from inside:10.0.0.10 to outside:192.168.1.100
    flags s idle 0:00:05 timeout 0:00:00`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseASAXlate(output)
	}
}

func BenchmarkParseASAACL(b *testing.B) {
	output := `access-list outside_access_in; 3 elements; name hash: 0x12345678
access-list outside_access_in line 1 extended permit tcp any host 192.168.1.100 eq https (hitcnt=150)
access-list outside_access_in line 2 extended deny ip any any (hitcnt=500)`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseASAACL(output)
	}
}

