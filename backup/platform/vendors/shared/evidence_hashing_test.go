package shared

import (
	"strings"
	"testing"
)

func TestEvidenceHasher_HashStanza(t *testing.T) {
	hasher := NewEvidenceHasher("cisco")
	
	stanza := `interface GigabitEthernet1/0/12
 description Access Port
 switchport mode access
 switchport access vlan 100
 spanning-tree portfast
 spanning-tree bpduguard enable`
	
	evidence := hasher.HashStanza("interface GigabitEthernet1/0/12", stanza)
	
	if evidence.SourceBlock != "interface GigabitEthernet1/0/12" {
		t.Errorf("Expected source_block 'interface GigabitEthernet1/0/12', got '%s'", evidence.SourceBlock)
	}
	
	if !strings.HasPrefix(evidence.SourceSnippetHash, "sha256:") {
		t.Errorf("Expected hash to start with 'sha256:', got '%s'", evidence.SourceSnippetHash)
	}
	
	if len(evidence.SourceSnippetHash) != 71 { // "sha256:" (7) + 64 hex chars
		t.Errorf("Expected hash length 71, got %d", len(evidence.SourceSnippetHash))
	}
}

func TestEvidenceHasher_Determinism(t *testing.T) {
	hasher := NewEvidenceHasher("cisco")
	
	// Same config with different formatting should produce same hash
	stanza1 := `interface GigabitEthernet1/0/12
 switchport mode access
 switchport access vlan 100`
	
	stanza2 := `interface   GigabitEthernet1/0/12
  switchport   mode   access
  switchport   access   vlan   100`
	
	evidence1 := hasher.HashStanza("interface GigabitEthernet1/0/12", stanza1)
	evidence2 := hasher.HashStanza("interface GigabitEthernet1/0/12", stanza2)
	
	if evidence1.SourceSnippetHash != evidence2.SourceSnippetHash {
		t.Errorf("Determinism failed: different formatting produced different hashes\n  Hash1: %s\n  Hash2: %s",
			evidence1.SourceSnippetHash, evidence2.SourceSnippetHash)
	}
}

func TestEvidenceHasher_CommentStripping(t *testing.T) {
	hasher := NewEvidenceHasher("cisco")
	
	stanzaWithComments := `interface GigabitEthernet1/0/12
 ! This is a comment
 switchport mode access ! inline comment
 switchport access vlan 100`
	
	stanzaWithoutComments := `interface GigabitEthernet1/0/12
 switchport mode access
 switchport access vlan 100`
	
	evidence1 := hasher.HashStanza("interface GigabitEthernet1/0/12", stanzaWithComments)
	evidence2 := hasher.HashStanza("interface GigabitEthernet1/0/12", stanzaWithoutComments)
	
	if evidence1.SourceSnippetHash != evidence2.SourceSnippetHash {
		t.Errorf("Comment stripping failed: comments affected hash\n  With comments: %s\n  Without comments: %s",
			evidence1.SourceSnippetHash, evidence2.SourceSnippetHash)
	}
}

func TestEvidenceHasher_CaseFolding(t *testing.T) {
	hasher := NewEvidenceHasher("cisco")
	
	stanzaLower := `interface GigabitEthernet1/0/12
 switchport mode access
 switchport access vlan 100`
	
	stanzaUpper := `INTERFACE GigabitEthernet1/0/12
 SWITCHPORT MODE ACCESS
 SWITCHPORT ACCESS VLAN 100`
	
	evidence1 := hasher.HashStanza("interface GigabitEthernet1/0/12", stanzaLower)
	evidence2 := hasher.HashStanza("INTERFACE GigabitEthernet1/0/12", stanzaUpper)
	
	if evidence1.SourceSnippetHash != evidence2.SourceSnippetHash {
		t.Errorf("Case folding failed: case affected hash\n  Lower: %s\n  Upper: %s",
			evidence1.SourceSnippetHash, evidence2.SourceSnippetHash)
	}
}

func TestEvidenceHasher_ExtractCiscoStanzas(t *testing.T) {
	hasher := NewEvidenceHasher("cisco")
	
	config := `!
hostname SW-CORE-01
!
interface GigabitEthernet1/0/1
 description Uplink
 switchport mode trunk
!
interface GigabitEthernet1/0/2
 description Access
 switchport mode access
 switchport access vlan 100
!
vlan 100
 name DATA
!`
	
	stanzas := hasher.ExtractStanzaBoundaries(config)
	
	expectedCount := 3 // hostname is not a stanza header, so: 2 interfaces + 1 vlan
	if len(stanzas) != expectedCount {
		t.Errorf("Expected %d stanzas, got %d", expectedCount, len(stanzas))
	}
	
	// Check first interface
	if len(stanzas) > 0 && !strings.HasPrefix(stanzas[0].SourceBlock, "interface GigabitEthernet") {
		t.Errorf("First stanza should be interface, got: %s", stanzas[0].SourceBlock)
	}
	
	// Check that stanzas have multiple lines
	if len(stanzas) > 0 && len(stanzas[0].Lines) < 2 {
		t.Errorf("Interface stanza should have multiple lines, got %d", len(stanzas[0].Lines))
	}
}

func TestEvidenceHasher_HashFullConfig(t *testing.T) {
	hasher := NewEvidenceHasher("cisco")
	
	config := `hostname SW-CORE-01
!
interface GigabitEthernet1/0/1
 switchport mode trunk`
	
	hash := hasher.HashFullConfig(config)
	
	if !strings.HasPrefix(hash, "sha256:") {
		t.Errorf("Full config hash should start with 'sha256:', got: %s", hash)
	}
	
	// Same config should produce same hash
	hash2 := hasher.HashFullConfig(config)
	if hash != hash2 {
		t.Errorf("Full config hash not deterministic")
	}
}

func TestEvidenceHasher_VLANStanza(t *testing.T) {
	hasher := NewEvidenceHasher("cisco")
	
	stanza := `vlan 2005
 name SCADA
 state active`
	
	evidence := hasher.HashStanza("vlan 2005", stanza)
	
	if evidence.SourceBlock != "vlan 2005" {
		t.Errorf("Expected source_block 'vlan 2005', got '%s'", evidence.SourceBlock)
	}
	
	if !strings.Contains(evidence.NormalizedStanza, "vlan 2005") {
		t.Errorf("Normalized stanza should contain 'vlan 2005'")
	}
}

func TestEvidenceHasher_JuniperStanzas(t *testing.T) {
	hasher := NewEvidenceHasher("juniper")
	
	config := `interfaces {
    ge-0/0/1 {
        unit 0 {
            family ethernet-switching {
                port-mode access;
                vlan {
                    members 100;
                }
            }
        }
    }
}`
	
	stanzas := hasher.ExtractStanzaBoundaries(config)
	
	if len(stanzas) == 0 {
		t.Errorf("Expected at least 1 stanza, got 0")
	}
	
	if len(stanzas) > 0 && !strings.Contains(stanzas[0].SourceBlock, "interfaces") {
		t.Errorf("First stanza should contain 'interfaces', got: %s", stanzas[0].SourceBlock)
	}
}

func TestEvidenceHasher_FortinetStanzas(t *testing.T) {
	hasher := NewEvidenceHasher("fortinet")
	
	config := `config system interface
    edit "port1"
        set vdom "root"
        set ip 192.168.1.1 255.255.255.0
        set allowaccess ping https ssh
    next
    edit "port2"
        set vdom "root"
        set ip 10.0.0.1 255.255.255.0
    next
end`
	
	stanzas := hasher.ExtractStanzaBoundaries(config)
	
	expectedCount := 2 // Two "edit" blocks
	if len(stanzas) != expectedCount {
		t.Errorf("Expected %d stanzas, got %d", expectedCount, len(stanzas))
	}
	
	if len(stanzas) > 0 && !strings.Contains(stanzas[0].SourceBlock, "port1") {
		t.Errorf("First stanza should contain 'port1', got: %s", stanzas[0].SourceBlock)
	}
}

// Benchmark tests
func BenchmarkHashStanza(b *testing.B) {
	hasher := NewEvidenceHasher("cisco")
	stanza := `interface GigabitEthernet1/0/12
 description Access Port
 switchport mode access
 switchport access vlan 100
 spanning-tree portfast
 spanning-tree bpduguard enable
 ip dhcp snooping trust
 ip arp inspection trust
 ip verify source`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hasher.HashStanza("interface GigabitEthernet1/0/12", stanza)
	}
}

func BenchmarkExtractStanzas(b *testing.B) {
	hasher := NewEvidenceHasher("cisco")
	config := strings.Repeat(`interface GigabitEthernet1/0/1
 switchport mode access
 switchport access vlan 100
!
`, 100) // 100 interfaces
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hasher.ExtractStanzaBoundaries(config)
	}
}

