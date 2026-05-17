package decision

import (
	"fmt"
	"github.com/mattpainter701/graphlayer/platform/models"
	asaparsers "github.com/mattpainter701/graphlayer/platform/vendors/cisco/asa"
	fortigateparsers "github.com/mattpainter701/graphlayer/platform/vendors/fortinet/fortigate"
	paloaltoparsers "github.com/mattpainter701/graphlayer/platform/vendors/paloalto/firewall"
	"log/slog"
	"net"
	"strconv"
	"strings"
)

// L4FirewallDecisionEngine provides unified firewall policy decisions across multiple vendors.
// Supported vendors: Cisco ASA, Fortinet FortiGate, Palo Alto PAN-OS
//
// The engine evaluates traffic flows against vendor-specific firewall policies and
// provides normalized permit/deny decisions with contextual metadata.
//
// Architecture:
//   - Vendor-agnostic interface for policy evaluation
//   - Normalized traffic flow representation
//   - Unified decision output format
//   - Integration with L3 routing decisions for next-hop awareness
type L4FirewallDecisionEngine struct {
	logger *slog.Logger

	// Vendor-specific policy stores
	ciscoASAPolicies     []asaparsers.ASAACLEntry
	fortigatePolicies    []fortigateparsers.FortiGateFirewallPolicy
	paloAltoPolicies     []paloaltoparsers.PANOSSecurityPolicy

	// L3 routing tables for next-hop resolution
	routingTables map[string]*models.L3RoutingTable // deviceID -> routing table

	// Decision metadata
	enableLogging   bool
	enableExplain   bool // Include detailed explanation in decision
	strictMode      bool // Fail closed on ambiguous policies
}

// TrafficFlow represents a network traffic flow for firewall evaluation
type TrafficFlow struct {
	// Source information
	SourceIP        string
	SourcePort      uint16
	SourceZone      string    // Security zone (if applicable)
	SourceInterface string

	// Destination information
	DestIP        string
	DestPort      uint16
	DestZone      string    // Security zone (if applicable)
	DestInterface string

	// Protocol information
	Protocol      string    // tcp, udp, icmp, etc.
	ICMPType      *uint8    // ICMP type (if protocol is icmp)
	ICMPCode      *uint8    // ICMP code (if protocol is icmp)

	// Application information (for next-gen firewalls)
	Application   string    // HTTP, SSH, SSL, etc.
	User          string    // Authenticated user (if available)

	// Flow metadata
	DeviceID      string    // Which firewall to evaluate against
	Direction     string    // inbound, outbound, lateral
}

// FirewallDecision represents the result of a firewall policy evaluation
type FirewallDecision struct {
	// Decision outcome
	Action        string    // permit, deny, reject, drop
	Confidence    string    // high, medium, low (based on rule specificity)

	// Matching policy information
	MatchedRule   string    // Policy/ACL name or number
	RuleIndex     int       // Position in policy list
	Vendor        string    // cisco_asa, fortinet, paloalto

	// Decision metadata
	Reason        string    // Human-readable explanation
	NextHop       string    // Next hop IP (if permit + routing available)
	EgressIface   string    // Egress interface (if permit + routing available)
	NATApplied    bool      // Whether NAT would be applied
	NATTranslation string   // NAT translation details

	// Logging and auditing
	LogEnabled    bool      // Whether this flow would be logged
	ThreatProfile string    // Applied security profiles (IPS, AV, etc.)
	QoSClass      string    // QoS/traffic shaping class

	// Explanation (if enableExplain is true)
	Explanation   []string  // Step-by-step decision process
}

// NewL4FirewallDecisionEngine creates a new unified firewall decision engine
func NewL4FirewallDecisionEngine(enableLogging, enableExplain, strictMode bool) *L4FirewallDecisionEngine {
	return &L4FirewallDecisionEngine{
		logger:          slog.Default().With("component", "l4_firewall_decision_engine"),
		routingTables:   make(map[string]*models.L3RoutingTable),
		enableLogging:   enableLogging,
		enableExplain:   enableExplain,
		strictMode:      strictMode,
	}
}

// LoadCiscoASAPolicies loads Cisco ASA ACL policies
func (e *L4FirewallDecisionEngine) LoadCiscoASAPolicies(policies []asaparsers.ASAACLEntry) {
	e.ciscoASAPolicies = policies
	e.logger.Info("Loaded Cisco ASA policies", "count", len(policies))
}

// LoadFortiGatePolicies loads FortiGate firewall policies
func (e *L4FirewallDecisionEngine) LoadFortiGatePolicies(policies []fortigateparsers.FortiGateFirewallPolicy) {
	e.fortigatePolicies = policies
	e.logger.Info("Loaded FortiGate policies", "count", len(policies))
}

// LoadPaloAltoPolicies loads Palo Alto security policies
func (e *L4FirewallDecisionEngine) LoadPaloAltoPolicies(policies []paloaltoparsers.PANOSSecurityPolicy) {
	e.paloAltoPolicies = policies
	e.logger.Info("Loaded Palo Alto policies", "count", len(policies))
}

// LoadRoutingTable loads L3 routing table for a device (for next-hop resolution)
func (e *L4FirewallDecisionEngine) LoadRoutingTable(deviceID string, table *models.L3RoutingTable) {
	e.routingTables[deviceID] = table
	e.logger.Info("Loaded routing table", "device_id", deviceID, "routes", len(table.Routes))
}

// EvaluateFlow evaluates a traffic flow against the appropriate firewall policies
func (e *L4FirewallDecisionEngine) EvaluateFlow(flow TrafficFlow) (*FirewallDecision, error) {
	if flow.DeviceID == "" {
		return nil, fmt.Errorf("device_id is required for flow evaluation")
	}

	var decision *FirewallDecision
	var err error

	// Determine vendor and evaluate accordingly
	// First try to determine vendor from loaded policies
	if len(e.ciscoASAPolicies) > 0 {
		decision, err = e.evaluateCiscoASAFlow(flow)
	} else if len(e.fortigatePolicies) > 0 {
		decision, err = e.evaluateFortiGateFlow(flow)
	} else if len(e.paloAltoPolicies) > 0 {
		decision, err = e.evaluatePaloAltoFlow(flow)
	} else {
		return nil, fmt.Errorf("no firewall policies loaded for device %s", flow.DeviceID)
	}

	if err != nil {
		return nil, err
	}

	// If permitted and routing table available, resolve next hop
	if decision.Action == "permit" || decision.Action == "allow" {
		if routingTable, exists := e.routingTables[flow.DeviceID]; exists {
			nextHop, egressIface := e.resolveNextHop(flow.DestIP, routingTable)
			decision.NextHop = nextHop
			decision.EgressIface = egressIface
		}
	}

	return decision, nil
}

// evaluateCiscoASAFlow evaluates traffic against Cisco ASA ACLs
func (e *L4FirewallDecisionEngine) evaluateCiscoASAFlow(flow TrafficFlow) (*FirewallDecision, error) {
	decision := &FirewallDecision{
		Action:      "deny", // Default deny (implicit deny at end of ASA ACL)
		Confidence:  "high",
		Vendor:      "cisco_asa",
		Explanation: []string{},
	}

	if e.enableExplain {
		decision.Explanation = append(decision.Explanation, "Evaluating against Cisco ASA ACLs")
		decision.Explanation = append(decision.Explanation, fmt.Sprintf("Flow: %s:%d -> %s:%d (%s)",
			flow.SourceIP, flow.SourcePort, flow.DestIP, flow.DestPort, flow.Protocol))
	}

	// Evaluate ACEs in order (first match wins)
	for idx, ace := range e.ciscoASAPolicies {
		if e.matchesASAACE(flow, ace) {
			decision.Action = strings.ToLower(ace.Action)
			decision.MatchedRule = ace.ACLName
			decision.RuleIndex = idx
			decision.Reason = fmt.Sprintf("Matched ACE %d in ACL %s", ace.LineNumber, ace.ACLName)
			decision.LogEnabled = ace.LogEnabled

			if e.enableExplain {
				decision.Explanation = append(decision.Explanation, fmt.Sprintf("Matched ACE: %s %s", ace.Action, ace.Description))
				decision.Explanation = append(decision.Explanation, fmt.Sprintf("Source: %s, Dest: %s, Protocol: %s",
					ace.SourceAddress, ace.DestAddress, ace.Protocol))
			}

			// ASA specific: Check for NAT (placeholder - would need NAT config)
			decision.NATApplied = false

			return decision, nil
		}
	}

	// No match - implicit deny
	decision.Reason = "Implicit deny (no matching ACE)"
	if e.enableExplain {
		decision.Explanation = append(decision.Explanation, "No matching ACE found - implicit deny")
	}

	return decision, nil
}

// evaluateFortiGateFlow evaluates traffic against FortiGate firewall policies
func (e *L4FirewallDecisionEngine) evaluateFortiGateFlow(flow TrafficFlow) (*FirewallDecision, error) {
	decision := &FirewallDecision{
		Action:      "deny", // Default deny (implicit deny in FortiGate)
		Confidence:  "high",
		Vendor:      "fortinet",
		Explanation: []string{},
	}

	if e.enableExplain {
		decision.Explanation = append(decision.Explanation, "Evaluating against FortiGate firewall policies")
		decision.Explanation = append(decision.Explanation, fmt.Sprintf("Flow: %s:%d -> %s:%d (%s)",
			flow.SourceIP, flow.SourcePort, flow.DestIP, flow.DestPort, flow.Protocol))
	}

	// Evaluate policies in order (first match wins)
	for idx, policy := range e.fortigatePolicies {
		if e.matchesFortiGatePolicy(flow, policy) {
			decision.Action = strings.ToLower(policy.Action)
			decision.MatchedRule = policy.Name
			decision.RuleIndex = idx
			decision.Reason = fmt.Sprintf("Matched policy %d: %s", policy.PolicyID, policy.Name)
			decision.LogEnabled = (policy.LogTraffic != "disable")

			if e.enableExplain {
				decision.Explanation = append(decision.Explanation, fmt.Sprintf("Matched policy: %s (ID: %d)",
					policy.Name, policy.PolicyID))
				decision.Explanation = append(decision.Explanation, fmt.Sprintf("Action: %s, Status: %s",
					policy.Action, policy.Status))
			}

			// FortiGate specific features
			decision.NATApplied = policy.NAT
			if policy.NAT {
				decision.NATTranslation = "Dynamic NAT (SNAT)"
			}

			// Security profiles
			if policy.AVProfile != "" || policy.IPSProfile != "" || policy.WebFilterProfile != "" {
				decision.ThreatProfile = fmt.Sprintf("AV:%s IPS:%s WebFilter:%s",
					policy.AVProfile, policy.IPSProfile, policy.WebFilterProfile)
			}

			// Traffic shaping
			if policy.TrafficShaper != "" {
				decision.QoSClass = policy.TrafficShaper
			}

			return decision, nil
		}
	}

	// No match - implicit deny
	decision.Reason = "Implicit deny (no matching policy)"
	if e.enableExplain {
		decision.Explanation = append(decision.Explanation, "No matching policy found - implicit deny")
	}

	return decision, nil
}

// evaluatePaloAltoFlow evaluates traffic against Palo Alto security policies
func (e *L4FirewallDecisionEngine) evaluatePaloAltoFlow(flow TrafficFlow) (*FirewallDecision, error) {
	decision := &FirewallDecision{
		Action:      "deny", // Default deny (implicit deny in PAN-OS)
		Confidence:  "high",
		Vendor:      "paloalto",
		Explanation: []string{},
	}

	if e.enableExplain {
		decision.Explanation = append(decision.Explanation, "Evaluating against Palo Alto security policies")
		decision.Explanation = append(decision.Explanation, fmt.Sprintf("Flow: %s:%d -> %s:%d (%s)",
			flow.SourceIP, flow.SourcePort, flow.DestIP, flow.DestPort, flow.Protocol))
	}

	// Evaluate policies in order (first match wins)
	for idx, policy := range e.paloAltoPolicies {
		if e.matchesPANOSPolicy(flow, policy) {
			decision.Action = strings.ToLower(policy.Action)
			decision.MatchedRule = policy.Name
			decision.RuleIndex = idx
			decision.Reason = fmt.Sprintf("Matched security rule: %s", policy.Name)
			decision.LogEnabled = (policy.LogStart || policy.LogEnd)

			if e.enableExplain {
				decision.Explanation = append(decision.Explanation, fmt.Sprintf("Matched policy: %s", policy.Name))
				decision.Explanation = append(decision.Explanation, fmt.Sprintf("Action: %s, Type: %s",
					policy.Action, policy.RuleType))
				decision.Explanation = append(decision.Explanation, fmt.Sprintf("Source Zone: %v, Dest Zone: %v",
					policy.SourceZones, policy.DestZones))
			}

			// Security profiles
			if policy.ProfileType == "group" && policy.GroupProfile != "" {
				decision.ThreatProfile = fmt.Sprintf("Security Profile Group: %s", policy.GroupProfile)
			} else if len(policy.Profiles) > 0 {
				decision.ThreatProfile = fmt.Sprintf("Profiles: %v", policy.Profiles)
			}

			// QoS
			if policy.QoSMarking != "" {
				decision.QoSClass = policy.QoSMarking
			}

			return decision, nil
		}
	}

	// No match - implicit deny
	decision.Reason = "Implicit deny (no matching security rule)"
	if e.enableExplain {
		decision.Explanation = append(decision.Explanation, "No matching security rule found - implicit deny")
	}

	return decision, nil
}

// matchesASAACE checks if a flow matches a Cisco ASA ACE
func (e *L4FirewallDecisionEngine) matchesASAACE(flow TrafficFlow, ace asaparsers.ASAACLEntry) bool {
	// Check protocol
	if !matchesProtocol(flow.Protocol, ace.Protocol) {
		return false
	}

	// Check source IP
	if !matchesIPAddress(flow.SourceIP, ace.SourceAddress, ace.SourceMask) {
		return false
	}

	// Check destination IP
	if !matchesIPAddress(flow.DestIP, ace.DestAddress, ace.DestMask) {
		return false
	}

	// Check source port
	if !matchesPort(flow.SourcePort, ace.SourcePort, ace.SourcePortOperator) {
		return false
	}

	// Check destination port
	if !matchesPort(flow.DestPort, ace.DestPort, ace.DestPortOperator) {
		return false
	}

	return true
}

// matchesFortiGatePolicy checks if a flow matches a FortiGate policy
func (e *L4FirewallDecisionEngine) matchesFortiGatePolicy(flow TrafficFlow, policy fortigateparsers.FortiGateFirewallPolicy) bool {
	// Check if policy is enabled
	if policy.Status != "enable" {
		return false
	}

	// Check source interface/zone
	if flow.SourceInterface != "" && !containsString(policy.SourceInterfaces, flow.SourceInterface) && !containsString(policy.SourceInterfaces, "any") {
		return false
	}

	// Check destination interface/zone
	if flow.DestInterface != "" && !containsString(policy.DestInterfaces, flow.DestInterface) && !containsString(policy.DestInterfaces, "any") {
		return false
	}

	// Check source addresses (simplified - would need address object resolution)
	if !containsString(policy.SourceAddresses, "all") {
		// Would need to resolve address objects here
		// For now, simplified check
	}

	// Check destination addresses
	if !containsString(policy.DestAddresses, "all") {
		// Would need to resolve address objects here
	}

	// Check services (simplified - would need service object resolution)
	if !containsString(policy.Services, "ALL") {
		// Would need to resolve service objects here
	}

	return true
}

// matchesPANOSPolicy checks if a flow matches a Palo Alto policy
func (e *L4FirewallDecisionEngine) matchesPANOSPolicy(flow TrafficFlow, policy paloaltoparsers.PANOSSecurityPolicy) bool {
	// Check if policy is disabled
	if policy.Disabled {
		return false
	}

	// Check source zone
	if flow.SourceZone != "" && !containsString(policy.SourceZones, flow.SourceZone) && !containsString(policy.SourceZones, "any") {
		return false
	}

	// Check destination zone
	if flow.DestZone != "" && !containsString(policy.DestZones, flow.DestZone) && !containsString(policy.DestZones, "any") {
		return false
	}

	// Check application (if specified)
	if flow.Application != "" && !containsString(policy.Applications, flow.Application) && !containsString(policy.Applications, "any") {
		return false
	}

	// Check source addresses (simplified - would need address object resolution)
	if !containsString(policy.SourceAddresses, "any") {
		// Would need to resolve address objects here
	}

	// Check destination addresses
	if !containsString(policy.DestAddresses, "any") {
		// Would need to resolve address objects here
	}

	return true
}

// resolveNextHop uses L3 routing table to find next hop for a destination
func (e *L4FirewallDecisionEngine) resolveNextHop(destIP string, routingTable *models.L3RoutingTable) (string, string) {
	ip := net.ParseIP(destIP)
	if ip == nil {
		return "", ""
	}

	var bestMatch *models.L3Route
	var longestPrefix int

	// Find longest prefix match
	for _, route := range routingTable.Routes {
		_, ipnet, err := net.ParseCIDR(route.Prefix)
		if err != nil {
			continue
		}

		if ipnet.Contains(ip) {
			prefixLen, _ := ipnet.Mask.Size()
			if prefixLen > longestPrefix {
				longestPrefix = prefixLen
				routeCopy := route
				bestMatch = &routeCopy
			}
		}
	}

	if bestMatch != nil {
		return bestMatch.NextHop, bestMatch.Interface
	}

	return "", ""
}

// Helper functions

func matchesProtocol(flowProto, ruleProto string) bool {
	flowProto = strings.ToLower(flowProto)
	ruleProto = strings.ToLower(ruleProto)

	if ruleProto == "ip" || ruleProto == "any" {
		return true
	}

	return flowProto == ruleProto
}

func matchesIPAddress(flowIP, ruleIP, ruleMask string) bool {
	if ruleIP == "any" || ruleIP == "0.0.0.0" {
		return true
	}

	// Parse flow IP
	ip := net.ParseIP(flowIP)
	if ip == nil {
		return false
	}

	// Check for host address (no mask)
	if ruleMask == "" || ruleMask == "255.255.255.255" {
		return flowIP == ruleIP
	}

	// Parse network and mask
	networkIP := net.ParseIP(ruleIP)
	mask := net.IPMask(net.ParseIP(ruleMask).To4())

	if networkIP == nil || mask == nil {
		return false
	}

	network := &net.IPNet{IP: networkIP, Mask: mask}
	return network.Contains(ip)
}

func matchesPort(flowPort uint16, rulePort string, operator string) bool {
	if rulePort == "" || rulePort == "any" {
		return true
	}

	// Parse rule port
	port, err := strconv.ParseUint(rulePort, 10, 16)
	if err != nil {
		return false
	}

	switch operator {
	case "eq", "":
		return flowPort == uint16(port)
	case "gt":
		return flowPort > uint16(port)
	case "lt":
		return flowPort < uint16(port)
	case "neq":
		return flowPort != uint16(port)
	default:
		return false
	}
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
