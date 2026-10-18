package firewall

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/policy"
	asa "github.com/mattpainter701/graphlayer/platform/vendors/cisco/asa"
)

const ParserVersion = "asa-config-v3"

//go:embed testdata/enterprise.asa.txt
var sampleASA []byte

//go:embed testdata/services.asa.txt
var sampleServicesASA []byte

// SampleASA returns a copy of the sanitized, syntactically real ASA sample.
func SampleASA() []byte { return append([]byte(nil), sampleASA...) }

// SampleServicesASA returns a representative ASA configuration with service objects.
func SampleServicesASA() []byte { return append([]byte(nil), sampleServicesASA...) }

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(password|passwd|secret|community|pre-shared-key|key-string|private-key|ldap-login-password|radius-common-pw|enable-password|username|crypto\s+ca\s+authenticate)\b`),
}

// ParseASAConfig uses the frozen legacy Go ASA ACL parser for evidence-linked
// access-group chains and requires the Rust worker for forwarding path data.
func ParseASAConfig(input []byte) (*Config, error) {
	parsed, err := asa.ParseRunningConfig(input)
	if err != nil {
		return nil, err
	}
	c := &Config{Vendor: "cisco-asa", ParserVersion: ParserVersion, Hostname: parsed.Hostname, Context: parsed.Context, ArtifactSHA256: parsed.ArtifactSHA256, SourceLines: []SourceLine{}, Chains: []Chain{}, Evidence: []Evidence{}, Warnings: []Warning{}, Capabilities: []string{"inbound attached extended ACL evaluation", "outbound ACLs retained as partial chains", "IPv4/IPv6 literals and CIDR", "TCP/UDP/ICMP/IP wildcard", "network and service objects and nested groups", "first-match order", "inactive ACEs"}}
	for _, l := range parsed.Lines {
		c.SourceLines = append(c.SourceLines, SourceLine{Number: l.Number, Text: "[OMITTED NON-POLICY CONFIGURATION]"})
	}
	for _, w := range parsed.Warnings {
		c.Warnings = append(c.Warnings, Warning{Code: w.Code, Message: w.Message, Line: w.Line})
	}
	ambiguousContext := false
	for _, w := range parsed.Warnings {
		if w.Code == "ambiguous_context" {
			ambiguousContext = true
		}
	}
	seen := map[string]bool{}
	seenBinding := map[string]bool{}
	bindingsByScope := map[string][]asa.ACLBinding{}
	for _, b := range parsed.Bindings {
		if b.Supported {
			key := strings.ToLower(b.Interface) + "/" + strings.ToLower(b.Direction)
			bindingsByScope[key] = append(bindingsByScope[key], b)
		}
	}
	for _, b := range parsed.Bindings {
		bindingKey := strings.ToLower(b.ACLName) + "/" + strings.ToLower(b.Interface) + "/" + strings.ToLower(b.Direction) + fmt.Sprintf("/%t", b.Supported)
		if seenBinding[bindingKey] {
			continue
		}
		seenBinding[bindingKey] = true
		aces, exists := parsed.ACLs[b.ACLName]
		if !exists || len(aces) == 0 {
			c.Warnings = append(c.Warnings, Warning{"empty_acl", "Bound ACL has no parsed ACEs", b.Line})
			continue
		}
		aces = append([]asa.ConfigACE(nil), aces...)
		sort.SliceStable(aces, func(i, j int) bool { return aces[i].Order < aces[j].Order })
		id := fmt.Sprintf("asa:%s:%s:%s", b.ACLName, b.Interface, b.Direction)
		if !b.Supported {
			id += fmt.Sprintf(":unsupported:%d", b.Line)
		}
		chain := Chain{ID: id, Name: b.ACLName, Interface: b.Interface, Direction: b.Direction, Complete: true, Rules: []policy.Rule{}, EvidenceRefs: []string{}, Warnings: []Warning{}}
		ambiguousBinding := !b.Supported
		if !b.Supported {
			chain.Complete = false
			chain.Warnings = append(chain.Warnings, Warning{"unsupported_binding", "Attachment syntax is retained but cannot be evaluated safely", b.Line})
		}
		if b.Supported {
			for _, other := range bindingsByScope[strings.ToLower(b.Interface)+"/"+strings.ToLower(b.Direction)] {
				if other.ACLName != b.ACLName {
					ambiguousBinding = true
					chain.Complete = false
					chain.Warnings = append(chain.Warnings, Warning{"ambiguous_binding", "Multiple ACLs are bound to the same interface and direction", b.Line})
					break
				}
			}
		}
		if ambiguousContext {
			chain.Complete = false
			chain.Warnings = append(chain.Warnings, Warning{"ambiguous_context", "Firewall context is ambiguous", b.Line})
		}
		ref := evidenceID(c.ArtifactSHA256, []int{b.Line})
		markSourceLines(c, []int{b.Line}, parsed.Lines)
		c.Evidence = append(c.Evidence, Evidence{ID: ref, StartLine: b.Line, EndLine: b.Line, Kind: "binding", Summary: "ACL attachment"})
		chain.EvidenceRefs = append(chain.EvidenceRefs, ref)
		for _, ace := range aces {
			for _, w := range parsed.Warnings {
				if w.Code == "unsupported_ace" && w.Line == ace.Line {
					chain.Complete = false
					chain.Warnings = append(chain.Warnings, Warning{"unsupported_ace", "ACE syntax could not be normalized and may affect any flow", ace.Line})
				}
				if w.Code == "ambiguous_acl_order" && w.ACLName == b.ACLName {
					chain.Complete = false
				}
			}
			sources, srcLines, srcErr := parsed.ExpandAddress(ace.Source)
			dests, dstLines, dstErr := parsed.ExpandAddress(ace.Destination)
			serviceTerms, serviceLines, serviceErr := serviceRuleTerms(parsed, ace)
			refsLines := append([]int{ace.Line}, srcLines...)
			refsLines = append(refsLines, dstLines...)
			refsLines = append(refsLines, serviceLines...)
			refsLines = append(refsLines, b.Line)
			refsLines = unique(refsLines)
			markSourceLines(c, refsLines, parsed.Lines)
			eref := evidenceID(c.ArtifactSHA256, refsLines)
			start, end := lineBounds(refsLines)
			c.Evidence = append(c.Evidence, Evidence{ID: eref, StartLine: start, EndLine: end, Kind: "access_rule", Summary: fmt.Sprintf("%s %s ACE %d", b.ACLName, ace.Action, ace.Order)})
			chain.EvidenceRefs = append(chain.EvidenceRefs, eref)
			if srcErr != nil {
				sources = []string{ace.Source}
				ace.Unsupported = append(ace.Unsupported, "object_expansion")
				chain.Warnings = append(chain.Warnings, Warning{"unresolved_object", "Source network object could not be safely resolved", ace.Line})
			}
			if dstErr != nil {
				dests = []string{ace.Destination}
				ace.Unsupported = append(ace.Unsupported, "object_expansion")
				chain.Warnings = append(chain.Warnings, Warning{"unresolved_object", "Destination network object could not be safely resolved", ace.Line})
			}
			if serviceErr != nil {
				serviceTerms = []serviceTerm{{protocol: "unresolved-service-protocol"}}
				ace.Unsupported = append(ace.Unsupported, "object_expansion")
				chain.Warnings = append(chain.Warnings, Warning{"unresolved_service", "Service object could not be safely resolved", ace.Line})
			}
			if len(sources)*len(dests)*len(serviceTerms) > asa.MaxACEExpansion {
				sources = []string{ace.Source}
				dests = []string{ace.Destination}
				serviceTerms = []serviceTerm{{protocol: "unresolved-service-protocol"}}
				ace.Unsupported = append(ace.Unsupported, "object_expansion")
			}
			for _, s := range sources {
				for _, d := range dests {
					for _, svc := range serviceTerms {
						position := len(chain.Rules)
						protocol := svc.protocol
						unsupported := append([]string(nil), ace.Unsupported...)
						for _, w := range parsed.Warnings {
							if w.Code == "ambiguous_acl_order" && w.ACLName == b.ACLName {
								unsupported = append(unsupported, "vendor_default")
							}
						}
						if ambiguousBinding {
							unsupported = append(unsupported, "vendor_default")
						}
						if protocol == "ip" {
							protocol = "any"
						}
						if ace.Disabled { /* represented and excluded from matching */
						}
						rid := fmt.Sprintf("%s:%d:%d", id, ace.Line, position+1)
						sourcePort, destinationPort := svc.source, svc.destination
						if protocol == "unresolved-service-protocol" {
							sourcePort, destinationPort = "", ""
						}
						rule := policy.Rule{ID: rid, Position: position, Enabled: !ace.Disabled, Action: normalizeAction(ace.Action), Source: s, Destination: d, Protocol: protocol, SourcePort: sourcePort, DestinationPort: destinationPort, EvidenceRefs: []string{eref}, Unsupported: uniqueStrings(unsupported)}
						chain.Rules = append(chain.Rules, rule)
						origin := RuleOrigin{RuleID: rid, ACLName: b.ACLName, Line: ace.Line}
						if strings.HasPrefix(ace.Source, "@") {
							origin.SourceObjectIDs = []string{addressObjectID(parsed, ace.Source)}
						}
						if strings.HasPrefix(ace.Destination, "@") {
							origin.DestinationObjectIDs = []string{addressObjectID(parsed, ace.Destination)}
						}
						origin.ServiceObjectIDs = serviceObjectIDs(ace)
						c.RuleOrigins = append(c.RuleOrigins, origin)
						if len(chain.Rules) > asa.MaxConfigRules {
							return nil, fmt.Errorf("ASA configuration has too many expanded ACL rules")
						}
					}
				}
			}
		}
		seen[b.ACLName] = true
		if strings.EqualFold(b.Direction, "out") {
			chain.Complete = false
			chain.Warnings = append(chain.Warnings, Warning{"unsupported_direction", "Outbound ACL direction is retained but not evaluatable in this milestone", b.Line})
		}
		c.Chains = append(c.Chains, chain)
	}
	unattached := make([]string, 0, len(parsed.ACLs))
	for acl := range parsed.ACLs {
		unattached = append(unattached, acl)
	}
	sort.Strings(unattached)
	for _, acl := range unattached {
		aces := parsed.ACLs[acl]
		if !seen[acl] {
			line := 0
			if len(aces) > 0 {
				line = aces[0].Line
			}
			c.Warnings = append(c.Warnings, Warning{"unattached_acl", "ACL is not attached with a supported access-group binding", line})
		}
	}
	addASAInventory(c, parsed)
	if err := addASAPathInventory(c, parsed); err != nil {
		return nil, err
	}
	return c, nil
}

func sanitizeLine(s string) string {
	if secretPatterns[0].MatchString(s) {
		return "[REDACTED SENSITIVE CONFIGURATION]"
	}
	upper := strings.ToUpper(s)
	if strings.Contains(upper, "BEGIN ") && (strings.Contains(upper, "PRIVATE KEY") || strings.Contains(upper, "CERTIFICATE")) || strings.Contains(upper, "END ") && (strings.Contains(upper, "PRIVATE KEY") || strings.Contains(upper, "CERTIFICATE")) {
		return "[REDACTED KEY OR CERTIFICATE MATERIAL]"
	}
	return s
}

func markSourceLines(c *Config, numbers []int, source []asa.ConfigLine) {
	for _, n := range numbers {
		if n > 0 && n <= len(source) && source[n-1].Number == n {
			c.SourceLines[n-1].Text = sanitizeLine(source[n-1].Text)
		}
	}
}
func evidenceID(hash string, lines []int) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s", hash)
	for _, n := range lines {
		fmt.Fprintf(h, ":%d", n)
	}
	return "sha256:" + fmt.Sprintf("%x", h.Sum(nil))
}
func lineBounds(a []int) (int, int) {
	if len(a) == 0 {
		return 0, 0
	}
	lo, hi := a[0], a[0]
	for _, n := range a[1:] {
		if n < lo {
			lo = n
		}
		if n > hi {
			hi = n
		}
	}
	return lo, hi
}
func unique(a []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, n := range a {
		if n > 0 && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}
func uniqueStrings(a []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range a {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
func normalizeAction(a string) string {
	if strings.EqualFold(a, "permit") {
		return "permit"
	}
	return "deny"
}
