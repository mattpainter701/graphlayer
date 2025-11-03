package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// EvidenceHasher provides functions for config stanza hashing per l2_project.md §6
type EvidenceHasher struct {
	// Vendor-specific configuration
	Vendor string
}

// StanzaEvidence represents a parsed config stanza with its hash
type StanzaEvidence struct {
	SourceBlock       string // Stable identifier (e.g., "interface Gi1/0/12")
	SourceSnippetHash string // SHA-256 of normalized stanza
	RawStanza         string // Original stanza text
	NormalizedStanza  string // Normalized stanza text
}

// NewEvidenceHasher creates a new hasher for a specific vendor
func NewEvidenceHasher(vendor string) *EvidenceHasher {
	return &EvidenceHasher{
		Vendor: strings.ToLower(vendor),
	}
}

// HashStanza normalizes and hashes a config stanza
func (eh *EvidenceHasher) HashStanza(sourceBlock, rawStanza string) *StanzaEvidence {
	normalized := eh.NormalizeStanza(rawStanza)
	hash := eh.ComputeHash(normalized)
	
	return &StanzaEvidence{
		SourceBlock:       sourceBlock,
		SourceSnippetHash: hash,
		RawStanza:         rawStanza,
		NormalizedStanza:  normalized,
	}
}

// NormalizeStanza applies normalization rules per l2_project.md §6
func (eh *EvidenceHasher) NormalizeStanza(stanza string) string {
	// Step 1: Strip comments and blank lines
	lines := strings.Split(stanza, "\n")
	var cleanedLines []string
	
	for _, line := range lines {
		// Remove comments (! for Cisco/Juniper, # for others)
		if strings.Contains(line, "!") && eh.Vendor == "cisco" {
			line = strings.Split(line, "!")[0]
		}
		if strings.Contains(line, "#") {
			line = strings.Split(line, "#")[0]
		}
		
		// Trim whitespace
		line = strings.TrimSpace(line)
		
		// Skip blank lines
		if line == "" {
			continue
		}
		
		cleanedLines = append(cleanedLines, line)
	}
	
	// Step 2: Normalize indentation & collapse multiple spaces
	for i, line := range cleanedLines {
		// Collapse multiple spaces to single space
		cleanedLines[i] = collapseSpaces(line)
	}
	
	// Step 3: Case-fold keywords (vendor-specific)
	for i, line := range cleanedLines {
		cleanedLines[i] = eh.caseFoldKeywords(line)
	}
	
	// Step 4: Sort order-insensitive lines
	cleanedLines = eh.sortOrderInsensitiveLines(cleanedLines)
	
	// Join lines
	normalized := strings.Join(cleanedLines, "\n")
	
	return normalized
}

// ComputeHash computes SHA-256 hash of UTF-8 bytes
func (eh *EvidenceHasher) ComputeHash(normalized string) string {
	hasher := sha256.New()
	hasher.Write([]byte(normalized))
	hashBytes := hasher.Sum(nil)
	return "sha256:" + hex.EncodeToString(hashBytes)
}

// caseFoldKeywords applies case-folding to keywords while preserving literals
func (eh *EvidenceHasher) caseFoldKeywords(line string) string {
	switch eh.Vendor {
	case "cisco":
		return eh.caseFoldCisco(line)
	case "juniper":
		return eh.caseFoldJuniper(line)
	default:
		// Conservative: preserve case for unknown vendors
		return line
	}
}

// caseFoldCisco handles Cisco IOS/NXOS keyword normalization
func (eh *EvidenceHasher) caseFoldCisco(line string) string {
	// Cisco keywords to lowercase
	keywords := []string{
		"interface", "vlan", "switchport", "mode", "access", "trunk", "native",
		"allowed", "channel-group", "shutdown", "no", "ip", "address", "dhcp",
		"snooping", "trust", "arp", "inspection", "verify", "source", "authentication",
		"port-control", "mab", "dot1x", "system-auth-control", "spanning-tree",
		"portfast", "bpduguard", "rootguard", "description", "name",
	}
	
	words := strings.Fields(line)
	for i, word := range words {
		wordLower := strings.ToLower(word)
		for _, keyword := range keywords {
			if wordLower == keyword {
				words[i] = keyword
				break
			}
		}
	}
	
	return strings.Join(words, " ")
}

// caseFoldJuniper handles Juniper keyword normalization
func (eh *EvidenceHasher) caseFoldJuniper(line string) string {
	// Juniper keywords to lowercase
	keywords := []string{
		"interfaces", "unit", "family", "ethernet-switching", "port-mode",
		"vlan", "members", "native-vlan-id", "vlans", "vlan-id", "description",
	}
	
	words := strings.Fields(line)
	for i, word := range words {
		wordLower := strings.ToLower(word)
		for _, keyword := range keywords {
			if wordLower == keyword {
				words[i] = keyword
				break
			}
		}
	}
	
	return strings.Join(words, " ")
}

// sortOrderInsensitiveLines sorts lines that are order-insensitive
func (eh *EvidenceHasher) sortOrderInsensitiveLines(lines []string) []string {
	// Separate header from body
	if len(lines) == 0 {
		return lines
	}
	
	header := lines[0]
	body := lines[1:]
	
	// Lines that can be sorted (Cisco-specific for now)
	if eh.Vendor == "cisco" {
		// Check if this is a multi-line VLAN list that should be sorted
		// For now, only sort if all lines are single-keyword lines (e.g., "member vlan 10")
		canSort := true
		for _, line := range body {
			// If line starts with a major keyword, it's not sortable
			if startsWithMajorKeyword(line) {
				canSort = false
				break
			}
		}
		
		if canSort && len(body) > 0 {
			sort.Strings(body)
		}
	}
	
	return append([]string{header}, body...)
}

// startsWithMajorKeyword checks if a line starts with a major config keyword
func startsWithMajorKeyword(line string) bool {
	majorKeywords := []string{
		"switchport", "ip", "no", "shutdown", "channel-group", "description",
		"spanning-tree", "authentication", "dot1x", "mab", "storm-control",
	}
	
	for _, keyword := range majorKeywords {
		if strings.HasPrefix(line, keyword+" ") || line == keyword {
			return true
		}
	}
	
	return false
}

// collapseSpaces collapses multiple consecutive spaces to a single space
func collapseSpaces(s string) string {
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, " ")
}

// ExtractStanzaBoundaries extracts config stanzas based on vendor syntax
func (eh *EvidenceHasher) ExtractStanzaBoundaries(config string) []Stanza {
	switch eh.Vendor {
	case "cisco":
		return eh.extractCiscoStanzas(config)
	case "juniper":
		return eh.extractJuniperStanzas(config)
	case "fortinet":
		return eh.extractFortinetStanzas(config)
	default:
		return eh.extractGenericStanzas(config)
	}
}

// Stanza represents a config block with boundaries
type Stanza struct {
	Header      string   // Header line (e.g., "interface Gi1/0/12")
	Lines       []string // All lines including header
	StartLine   int      // Line number in original config (1-indexed)
	EndLine     int      // Line number in original config (1-indexed)
	SourceBlock string   // Stable identifier
}

// extractCiscoStanzas parses Cisco IOS/NXOS hierarchical config
func (eh *EvidenceHasher) extractCiscoStanzas(config string) []Stanza {
	lines := strings.Split(config, "\n")
	var stanzas []Stanza
	var currentStanza *Stanza
	
	headerRegexes := []*regexp.Regexp{
		regexp.MustCompile(`^interface\s+(.+)$`),
		regexp.MustCompile(`^vlan\s+(\d+)$`),
		regexp.MustCompile(`^router\s+\w+.*$`),
		regexp.MustCompile(`^line\s+\w+.*$`),
		regexp.MustCompile(`^class-map\s+.*$`),
		regexp.MustCompile(`^policy-map\s+.*$`),
		regexp.MustCompile(`^ip\s+access-list\s+.*$`),
	}
	
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "!") {
			continue
		}
		
		// Check if this is a header line
		isHeader := false
		for _, re := range headerRegexes {
			if re.MatchString(trimmed) {
				// Save previous stanza if exists
				if currentStanza != nil {
					currentStanza.EndLine = i
					stanzas = append(stanzas, *currentStanza)
				}
				
				// Start new stanza
				currentStanza = &Stanza{
					Header:      trimmed,
					Lines:       []string{trimmed},
					StartLine:   i + 1,
					SourceBlock: trimmed,
				}
				isHeader = true
				break
			}
		}
		
		// If not a header and we have a current stanza, add to it
		if !isHeader && currentStanza != nil {
			// Check if line is indented (part of current stanza)
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				currentStanza.Lines = append(currentStanza.Lines, trimmed)
			} else {
				// Non-indented line means end of stanza
				currentStanza.EndLine = i
				stanzas = append(stanzas, *currentStanza)
				currentStanza = nil
			}
		}
	}
	
	// Save last stanza if exists
	if currentStanza != nil {
		currentStanza.EndLine = len(lines)
		stanzas = append(stanzas, *currentStanza)
	}
	
	return stanzas
}

// extractJuniperStanzas parses Juniper Junos brace-delimited config
func (eh *EvidenceHasher) extractJuniperStanzas(config string) []Stanza {
	// Juniper uses braces: `interfaces ge-0/0/1 { ... }`
	// This is more complex due to nested braces
	var stanzas []Stanza
	
	lines := strings.Split(config, "\n")
	var currentStanza *Stanza
	braceDepth := 0
	
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		
		// Count braces
		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")
		
		// If we see an open brace at depth 0, start new stanza
		if openBraces > 0 && braceDepth == 0 {
			// Extract header (everything before the brace)
			header := strings.TrimSpace(strings.Split(trimmed, "{")[0])
			currentStanza = &Stanza{
				Header:      header,
				Lines:       []string{trimmed},
				StartLine:   i + 1,
				SourceBlock: header,
			}
		} else if currentStanza != nil {
			currentStanza.Lines = append(currentStanza.Lines, trimmed)
		}
		
		braceDepth += openBraces - closeBraces
		
		// If braces are balanced, end stanza
		if braceDepth == 0 && currentStanza != nil {
			currentStanza.EndLine = i + 1
			stanzas = append(stanzas, *currentStanza)
			currentStanza = nil
		}
	}
	
	return stanzas
}

// extractFortinetStanzas parses Fortinet config/edit/next blocks
func (eh *EvidenceHasher) extractFortinetStanzas(config string) []Stanza {
	// Fortinet uses: `config ... edit <name> ... next`
	var stanzas []Stanza
	var currentStanza *Stanza
	inEdit := false
	
	lines := strings.Split(config, "\n")
	
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines
		if trimmed == "" {
			continue
		}
		
		// Check for "edit <name>"
		if strings.HasPrefix(trimmed, "edit ") {
			editName := strings.TrimPrefix(trimmed, "edit ")
			editName = strings.Trim(editName, `"`)
			currentStanza = &Stanza{
				Header:      fmt.Sprintf("edit %s", editName),
				Lines:       []string{trimmed},
				StartLine:   i + 1,
				SourceBlock: fmt.Sprintf("edit %s", editName),
			}
			inEdit = true
		} else if trimmed == "next" && inEdit {
			// End of edit block
			if currentStanza != nil {
				currentStanza.Lines = append(currentStanza.Lines, trimmed)
				currentStanza.EndLine = i + 1
				stanzas = append(stanzas, *currentStanza)
				currentStanza = nil
			}
			inEdit = false
		} else if currentStanza != nil {
			currentStanza.Lines = append(currentStanza.Lines, trimmed)
		}
	}
	
	return stanzas
}

// extractGenericStanzas provides basic stanza extraction for unknown vendors
func (eh *EvidenceHasher) extractGenericStanzas(config string) []Stanza {
	// Fallback: treat each top-level line as a separate stanza
	lines := strings.Split(config, "\n")
	var stanzas []Stanza
	
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "!") {
			stanzas = append(stanzas, Stanza{
				Header:      trimmed,
				Lines:       []string{trimmed},
				StartLine:   i + 1,
				EndLine:     i + 1,
				SourceBlock: trimmed,
			})
		}
	}
	
	return stanzas
}

// HashConfigStanzas processes an entire config and returns evidence for each stanza
func (eh *EvidenceHasher) HashConfigStanzas(config string) []*StanzaEvidence {
	stanzas := eh.ExtractStanzaBoundaries(config)
	evidences := make([]*StanzaEvidence, 0, len(stanzas))
	
	for _, stanza := range stanzas {
		rawStanza := strings.Join(stanza.Lines, "\n")
		evidence := eh.HashStanza(stanza.SourceBlock, rawStanza)
		evidences = append(evidences, evidence)
	}
	
	return evidences
}

// HashFullConfig computes SHA-256 hash of entire config
func (eh *EvidenceHasher) HashFullConfig(config string) string {
	// Normalize entire config
	normalized := eh.NormalizeStanza(config)
	return eh.ComputeHash(normalized)
}

