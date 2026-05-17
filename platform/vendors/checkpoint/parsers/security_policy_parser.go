package parsers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SecurityPolicy represents a normalized security policy
type SecurityPolicy struct {
	Vendor      string
	DeviceName  string
	PolicyName  string
	Rules       []SecurityRule
	GlobalRules []SecurityRule
	Metadata    map[string]string
}

// SecurityRule represents a normalized security rule
type SecurityRule struct {
	RuleNumber      int
	Name            string
	SourceZones     []string
	SourceAddresses []string
	DestZones       []string
	DestAddresses   []string
	Services        []string
	Applications    []string
	Action          string
	Logging         string
	Enabled         bool
	Position        int
	Comments        string
	Tags            []string
}

// NormalizedPolicy is the unified output format
type NormalizedPolicy struct {
	Vendor         string
	DeviceName     string
	PolicyType     string
	RuleCount      int
	Rules          []NormalizedRule
	RuleOrder      string
	DefaultAction  string
	Metadata       map[string]string
}

// NormalizedRule is the vendor-agnostic rule format
type NormalizedRule struct {
	ID              string
	Name            string
	Sequence        int
	SourceIPs       []string
	SourceZones     []string
	DestinationIPs  []string
	DestinationZones []string
	Ports           []string
	Protocols       []string
	Applications    []string
	Action          string
	Logging         string
	Enabled         bool
	Description     string
	OriginalVendor  string
	OriginalRuleNum int
}

// ParseCheckPointPolicy parses Check Point security policy
func ParseCheckPointPolicy(policyText string, deviceName string) (*SecurityPolicy, error) {
	policy := &SecurityPolicy{
		Vendor:      "Check Point",
		DeviceName:  deviceName,
		PolicyName:  "Default Policy",
		Rules:       []SecurityRule{},
		GlobalRules: []SecurityRule{},
		Metadata:    make(map[string]string),
	}

	lines := strings.Split(policyText, "\n")
	currentSection := ""
	ruleNum := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Detect sections
		if strings.Contains(line, "Security Policy") || strings.Contains(line, "Policy Name:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				policy.PolicyName = strings.TrimSpace(parts[1])
			}
			currentSection = "policy"
			continue
		}

		if strings.Contains(line, "Global Properties") || strings.Contains(line, "Global Policy") {
			currentSection = "global"
			continue
		}

		if strings.Contains(line, "Access Rules") || strings.Contains(line, "Rule Base") {
			currentSection = "rules"
			continue
		}

		// Parse individual rules
		if currentSection == "rules" || currentSection == "global" {
			rule := parseCheckPointRule(line, &ruleNum)
			if rule != nil {
				if currentSection == "global" {
					policy.GlobalRules = append(policy.GlobalRules, *rule)
				} else {
					policy.Rules = append(policy.Rules, *rule)
				}
			}
		}

		// Parse metadata
		if strings.Contains(line, "Installed On:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				policy.Metadata["installed_on"] = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "Installation Targets:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				policy.Metadata["installation_targets"] = strings.TrimSpace(parts[1])
			}
		}
	}

	return policy, nil
}

func parseCheckPointRule(line string, ruleNum *int) *SecurityRule {
	// Check Point rule format examples:
	// #	Name	Source	Destination	Service	Action	Track	Install On	Time
	// 1	Allow HTTP	Internal	DMZ	http	accept	log	Policy Targets	Any

	re := regexp.MustCompile(`^(\d+)\s+(.+?)\s+(.+?)\s+(.+?)\s+(.+?)\s+(accept|drop|reject)\s+(.+?)\s+(.+?)\s+(.+)$`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 10 {
		*ruleNum++
		num, _ := strconv.Atoi(matches[1])
		rule := &SecurityRule{
			RuleNumber:      num,
			Name:            strings.TrimSpace(matches[2]),
			SourceAddresses: parseCheckPointList(matches[3]),
			DestAddresses:   parseCheckPointList(matches[4]),
			Services:        parseCheckPointList(matches[5]),
			Action:          matches[6],
			Logging:         strings.TrimSpace(matches[7]),
			Enabled:         true,
			Position:        *ruleNum,
			Comments:        strings.TrimSpace(matches[9]),
		}
		return rule
	}

	// Try alternative format with tabs
	re2 := regexp.MustCompile(`^(\d+)\t+(.+?)\t+(.+?)\t+(.+?)\t+(.+?)\t+(accept|drop|reject)\t+(.+?)\t+(.+?)\t+(.+)$`)
	matches2 := re2.FindStringSubmatch(line)
	if len(matches2) >= 10 {
		*ruleNum++
		num, _ := strconv.Atoi(matches2[1])
		rule := &SecurityRule{
			RuleNumber:      num,
			Name:            strings.TrimSpace(matches2[2]),
			SourceAddresses: parseCheckPointList(matches2[3]),
			DestAddresses:   parseCheckPointList(matches2[4]),
			Services:        parseCheckPointList(matches2[5]),
			Action:          matches2[6],
			Logging:         strings.TrimSpace(matches2[7]),
			Enabled:         true,
			Position:        *ruleNum,
			Comments:        strings.TrimSpace(matches2[9]),
		}
		return rule
	}

	return nil
}

func parseCheckPointList(list string) []string {
	items := []string{}
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item != "" && item != "Any" {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		items = append(items, "Any")
	}
	return items
}

// NormalizeToUnifiedPolicy converts vendor-specific policy to unified format
func NormalizeToUnifiedPolicy(policy *SecurityPolicy) *NormalizedPolicy {
	np := &NormalizedPolicy{
		Vendor:        policy.Vendor,
		DeviceName:    policy.DeviceName,
		PolicyType:    "Security Policy",
		RuleCount:     len(policy.Rules),
		Rules:         []NormalizedRule{},
		RuleOrder:     "sequential",
		DefaultAction: "drop",
		Metadata:      policy.Metadata,
	}

	for i, rule := range policy.Rules {
		nr := NormalizedRule{
			ID:              fmt.Sprintf("%s-%s-%d", policy.Vendor, policy.DeviceName, rule.RuleNumber),
			Name:            rule.Name,
			Sequence:        i + 1,
			SourceIPs:       rule.SourceAddresses,
			SourceZones:     rule.SourceZones,
			DestinationIPs:  rule.DestAddresses,
			DestinationZones: rule.DestZones,
			Ports:           extractPorts(rule.Services),
			Protocols:       extractProtocols(rule.Services),
			Applications:    rule.Applications,
			Action:          rule.Action,
			Logging:         rule.Logging,
			Enabled:         rule.Enabled,
			Description:     rule.Comments,
			OriginalVendor:  policy.Vendor,
			OriginalRuleNum: rule.RuleNumber,
		}
		np.Rules = append(np.Rules, nr)
	}

	return np
}

func extractPorts(services []string) []string {
	ports := []string{}
	re := regexp.MustCompile(`(\d+)(?:-(\d+))?`)
	for _, svc := range services {
		if matches := re.FindStringSubmatch(svc); len(matches) >= 2 {
			if matches[2] != "" {
				ports = append(ports, fmt.Sprintf("%s-%s", matches[1], matches[2]))
			} else {
				ports = append(ports, matches[1])
			}
		}
	}
	return ports
}

func extractProtocols(services []string) []string {
	protocols := []string{}
	for _, svc := range services {
		svcLower := strings.ToLower(svc)
		if strings.Contains(svcLower, "tcp") {
			protocols = append(protocols, "tcp")
		} else if strings.Contains(svcLower, "udp") {
			protocols = append(protocols, "udp")
		} else if strings.Contains(svcLower, "icmp") {
			protocols = append(protocols, "icmp")
		}
	}
	if len(protocols) == 0 {
		protocols = append(protocols, "any")
	}
	return protocols
}

// MergePolicies merges multiple vendor policies into unified format
func MergePolicies(policies []*SecurityPolicy) *NormalizedPolicy {
	merged := &NormalizedPolicy{
		Vendor:        "Multi-Vendor",
		DeviceName:    "Merged Policy",
		PolicyType:    "Merged Security Policy",
		Rules:         []NormalizedRule{},
		RuleOrder:     "sequential",
		DefaultAction: "drop",
		Metadata:      make(map[string]string),
	}

	ruleSeq := 0
	for _, policy := range policies {
		for _, rule := range policy.Rules {
			ruleSeq++
			nr := NormalizedRule{
				ID:              fmt.Sprintf("merged-%d", ruleSeq),
				Name:            rule.Name,
				Sequence:        ruleSeq,
				SourceIPs:       rule.SourceAddresses,
				SourceZones:     rule.SourceZones,
				DestinationIPs:  rule.DestAddresses,
				DestinationZones: rule.DestZones,
				Ports:           extractPorts(rule.Services),
				Protocols:       extractProtocols(rule.Services),
				Applications:    rule.Applications,
				Action:          rule.Action,
				Logging:         rule.Logging,
				Enabled:         rule.Enabled,
				Description:     rule.Comments,
				OriginalVendor:  policy.Vendor,
				OriginalRuleNum: rule.RuleNumber,
			}
			merged.Rules = append(merged.Rules, nr)
		}
	}

	merged.RuleCount = len(merged.Rules)
	merged.Metadata["merged_from"] = fmt.Sprintf("%d policies", len(policies))
	return merged
}
