package decision

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

// L2DecisionEngine implements Layer 2 reachability determination logic.
// It produces Allowed/Conditional/Blocked outcomes with evidence-based reasoning
// per the reference L2 project requirements.
//
// Decision criteria:
// - Allowed: High confidence + authenticated + DHCP bound
// - Conditional: Medium confidence OR partial evidence
// - Blocked: Low confidence OR security violations OR isolation controls
type L2DecisionEngine struct {
	// Configuration
	EnableStrictMode      bool          // Require both auth + DHCP for Allowed
	ConditionalThreshold  int           // Minimum confidence for Conditional (default: 40)
	AllowedThreshold      int           // Minimum confidence for Allowed (default: 70)
	DecisionTTL           time.Duration // Cache TTL (default: 5 minutes)

	// Evidence sources
	DHCPBindings          map[string]DHCPBinding        // MAC → binding
	AuthSessions          map[string]AuthSession        // MAC → session
	IPSGBindings          map[string][]IPSGBinding      // Interface → bindings
	PortSecurityState     map[string]PortSecurityState  // Interface → state
	ARPEntries            map[string]ARPEntry           // MAC → ARP
	MACTable              map[string]MACTableEntry      // MAC → entry
	InterfaceSecurity     map[string]InterfaceSecurityState // Interface → security
	PVLANIsolation        map[int]PVLANIsolation        // VLAN → PVLAN info
}

// DHCPBinding represents a DHCP snooping binding.
type DHCPBinding struct {
	MAC         string
	IP          string
	VLAN        int
	Interface   string
	LeaseTime   int
	CollectedAt time.Time
}

// AuthSession represents an 802.1X/MAB authentication session.
type AuthSession struct {
	MAC            string
	IP             string
	VLAN           int
	Interface      string
	Method         string // "dot1x", "mab"
	Status         string // "Authc Success", "Authorized", "Authc Failed"
	CollectedAt    time.Time
}

// IPSGBinding represents an IP Source Guard binding.
type IPSGBinding struct {
	MAC         string
	IP          string
	VLAN        int
	Interface   string
	Filter      string // "ip", "ip-mac"
}

// PortSecurityState represents port security operational state.
type PortSecurityState struct {
	Interface        string
	Enabled          bool
	MaxMAC           int
	CurrentMAC       int
	ViolationMode    string
	ViolationCount   int
}

// ARPEntry represents an ARP table entry.
type ARPEntry struct {
	IP          string
	MAC         string
	Interface   string
	Age         int
}

// MACTableEntry represents a MAC address table entry.
type MACTableEntry struct {
	MAC         string
	VLAN        int
	Interface   string
	Type        string // "dynamic", "static", "secure"
	Age         int
}

// InterfaceSecurityState represents aggregated security controls per interface.
type InterfaceSecurityState struct {
	Interface            string
	DHCPSnoopingTrust    bool
	ARPInspectionTrust   bool
	IPSGEnabled          bool
	Dot1XEnabled         bool
	MABEnabled           bool
	PortSecurityEnabled  bool
	UnauthVLAN           int
	SecurityScore        int    // 0-100
	SecurityLevel        string // "high", "medium", "low", "none"
}

// PVLANIsolation represents Private VLAN isolation information.
type PVLANIsolation struct {
	PrimaryVLAN      int
	SecondaryVLANs   map[int]string // VLAN → type ("isolated", "community")
	IsolatedPorts    []string
	PromiscuousPorts []string
}

// L2ReachabilityQuery represents a query for L2 reachability.
type L2ReachabilityQuery struct {
	SourceMAC      string
	DestMAC        string
	VLANID         int
	SourceInterface string // Optional: if known
	DestInterface   string // Optional: if known
}

// L2Decision represents the outcome of L2 reachability analysis.
type L2Decision struct {
	// Query context
	SourceMAC       string    `json:"source_mac"`
	DestMAC         string    `json:"dest_mac"`
	VLANID          int       `json:"vlan_id"`
	SourceInterface string    `json:"source_interface,omitempty"`
	DestInterface   string    `json:"dest_interface,omitempty"`

	// Decision
	Outcome         string    `json:"outcome"` // "Allowed", "Conditional", "Blocked"
	ReasonCodes     []string  `json:"reason_codes"`
	ReasonSummary   string    `json:"reason_summary"`

	// Evidence
	SourceEvidence  Evidence  `json:"source_evidence"`
	DestEvidence    Evidence  `json:"dest_evidence"`

	// Security posture
	SourceSecurityScore int    `json:"source_security_score"`
	DestSecurityScore   int    `json:"dest_security_score"`
	ConfidenceScore     int    `json:"confidence_score"` // Overall decision confidence

	// L3 handoff
	HandoffToL3         bool   `json:"handoff_to_l3"`
	HandoffReason       string `json:"handoff_reason,omitempty"`

	// Metadata
	EvaluatedAt    time.Time `json:"evaluated_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	DecisionID     string    `json:"decision_id"`
}

// Evidence represents supporting evidence for a decision.
type Evidence struct {
	MACTableEntry      bool     `json:"mac_table_entry"`
	DHCPBound          bool     `json:"dhcp_bound"`
	Authenticated      bool     `json:"authenticated"`
	AuthMethod         string   `json:"auth_method,omitempty"`
	IPSGBound          bool     `json:"ipsg_bound"`
	PortSecurityValid  bool     `json:"port_security_valid"`
	ARPEntryExists     bool     `json:"arp_entry_exists"`
	EvidenceSources    []string `json:"evidence_sources"`
	ConfidenceScore    int      `json:"confidence_score"`
}

// ReasonCode constants for L2 decisions.
const (
	// Allowed reasons
	ReasonAuthenticatedAndBound     = "AUTHENTICATED_AND_DHCP_BOUND"
	ReasonAuthenticatedHighConf     = "AUTHENTICATED_HIGH_CONFIDENCE"
	ReasonStaticMAC                 = "STATIC_MAC_ENTRY"

	// Conditional reasons
	ReasonAuthenticatedNoDHCP       = "AUTHENTICATED_NO_DHCP_BINDING"
	ReasonDHCPBoundNotAuth          = "DHCP_BOUND_NOT_AUTHENTICATED"
	ReasonMediumConfidence          = "MEDIUM_CONFIDENCE_PARTIAL_EVIDENCE"
	ReasonFreshMACEntry             = "FRESH_MAC_ENTRY"

	// Blocked reasons
	ReasonLowConfidence             = "LOW_CONFIDENCE_INSUFFICIENT_EVIDENCE"
	ReasonPortSecurityViolation     = "PORT_SECURITY_VIOLATION"
	ReasonAuthenticationFailed      = "AUTHENTICATION_FAILED"
	ReasonPVLANIsolated             = "PVLAN_ISOLATED"
	ReasonDifferentVLAN             = "DIFFERENT_VLAN"
	ReasonNoMACEntry                = "NO_MAC_TABLE_ENTRY"
	ReasonStaleEvidence             = "STALE_EVIDENCE"
)

// NewL2DecisionEngine creates a new L2 decision engine with default configuration.
func NewL2DecisionEngine() *L2DecisionEngine {
	return &L2DecisionEngine{
		EnableStrictMode:     true,
		ConditionalThreshold: 40,
		AllowedThreshold:     70,
		DecisionTTL:          5 * time.Minute,
		DHCPBindings:         make(map[string]DHCPBinding),
		AuthSessions:         make(map[string]AuthSession),
		IPSGBindings:         make(map[string][]IPSGBinding),
		PortSecurityState:    make(map[string]PortSecurityState),
		ARPEntries:           make(map[string]ARPEntry),
		MACTable:             make(map[string]MACTableEntry),
		InterfaceSecurity:    make(map[string]InterfaceSecurityState),
		PVLANIsolation:       make(map[int]PVLANIsolation),
	}
}

// DetermineL2Reachability evaluates whether source MAC can reach destination MAC at Layer 2.
// This is the primary entry point for L2 decision logic.
func (e *L2DecisionEngine) DetermineL2Reachability(query L2ReachabilityQuery) L2Decision {
	now := time.Now()

	decision := L2Decision{
		SourceMAC:       query.SourceMAC,
		DestMAC:         query.DestMAC,
		VLANID:          query.VLANID,
		SourceInterface: query.SourceInterface,
		DestInterface:   query.DestInterface,
		EvaluatedAt:     now,
		ExpiresAt:       now.Add(e.DecisionTTL),
		DecisionID:      generateDecisionID(query),
		ReasonCodes:     make([]string, 0),
	}

	// Gather evidence for source and destination
	decision.SourceEvidence = e.gatherEvidence(query.SourceMAC, query.SourceInterface, query.VLANID)
	decision.DestEvidence = e.gatherEvidence(query.DestMAC, query.DestInterface, query.VLANID)

	// Calculate security scores
	decision.SourceSecurityScore = e.calculateSecurityScore(query.SourceInterface, decision.SourceEvidence)
	decision.DestSecurityScore = e.calculateSecurityScore(query.DestInterface, decision.DestEvidence)

	// Overall confidence is the minimum of source and dest
	decision.ConfidenceScore = min(decision.SourceEvidence.ConfidenceScore, decision.DestEvidence.ConfidenceScore)

	// Check basic reachability constraints
	if !decision.SourceEvidence.MACTableEntry {
		decision.Outcome = "Blocked"
		decision.ReasonCodes = append(decision.ReasonCodes, ReasonNoMACEntry)
		decision.ReasonSummary = fmt.Sprintf("Source MAC %s not found in MAC table", query.SourceMAC)
		return decision
	}

	if !decision.DestEvidence.MACTableEntry {
		decision.Outcome = "Blocked"
		decision.ReasonCodes = append(decision.ReasonCodes, ReasonNoMACEntry)
		decision.ReasonSummary = fmt.Sprintf("Destination MAC %s not found in MAC table", query.DestMAC)
		return decision
	}

	// Check VLAN consistency (both must be in same VLAN for L2 reachability)
	sourceMACEntry, srcExists := e.MACTable[strings.ToLower(query.SourceMAC)]
	destMACEntry, dstExists := e.MACTable[strings.ToLower(query.DestMAC)]

	if srcExists && dstExists && sourceMACEntry.VLAN != destMACEntry.VLAN {
		decision.Outcome = "Blocked"
		decision.ReasonCodes = append(decision.ReasonCodes, ReasonDifferentVLAN)
		decision.ReasonSummary = fmt.Sprintf("Source VLAN %d != Dest VLAN %d (L3 handoff required)",
			sourceMACEntry.VLAN, destMACEntry.VLAN)
		decision.HandoffToL3 = true
		decision.HandoffReason = "Different VLANs require Layer 3 routing"
		return decision
	}

	// Check PVLAN isolation
	if pvlan, exists := e.PVLANIsolation[query.VLANID]; exists {
		if e.checkPVLANIsolation(pvlan, query.SourceInterface, query.DestInterface) {
			decision.Outcome = "Blocked"
			decision.ReasonCodes = append(decision.ReasonCodes, ReasonPVLANIsolated)
			decision.ReasonSummary = "Private VLAN isolation prevents direct communication"
			return decision
		}
	}

	// Check port security violations
	if query.SourceInterface != "" {
		if portSec, exists := e.PortSecurityState[query.SourceInterface]; exists {
			if portSec.Enabled && portSec.ViolationCount > 0 {
				decision.Outcome = "Blocked"
				decision.ReasonCodes = append(decision.ReasonCodes, ReasonPortSecurityViolation)
				decision.ReasonSummary = fmt.Sprintf("Port security violations detected on %s (%d violations)",
					query.SourceInterface, portSec.ViolationCount)
				return decision
			}
		}
	}

	// Check authentication failures
	if decision.SourceEvidence.Authenticated {
		srcAuth := e.AuthSessions[strings.ToLower(query.SourceMAC)]
		if srcAuth.Status == "Authc Failed" || srcAuth.Status == "Failed" {
			decision.Outcome = "Blocked"
			decision.ReasonCodes = append(decision.ReasonCodes, ReasonAuthenticationFailed)
			decision.ReasonSummary = fmt.Sprintf("Source MAC authentication failed (status: %s)", srcAuth.Status)
			return decision
		}
	}

	// Apply decision logic based on evidence and confidence
	decision.Outcome, decision.ReasonCodes, decision.ReasonSummary = e.applyDecisionLogic(
		decision.SourceEvidence,
		decision.DestEvidence,
		decision.ConfidenceScore,
	)

	return decision
}

// gatherEvidence collects all available evidence for a given MAC address.
func (e *L2DecisionEngine) gatherEvidence(mac, iface string, vlan int) Evidence {
	macLower := strings.ToLower(mac)
	evidence := Evidence{
		EvidenceSources: make([]string, 0),
		ConfidenceScore: 0,
	}

	// Check MAC table
	if macEntry, exists := e.MACTable[macLower]; exists {
		evidence.MACTableEntry = true
		evidence.EvidenceSources = append(evidence.EvidenceSources, "mac-table")

		// Base confidence from MAC type
		switch macEntry.Type {
		case "static":
			evidence.ConfidenceScore += 10
		case "secure":
			evidence.PortSecurityValid = true
			evidence.ConfidenceScore += 20
			evidence.EvidenceSources = append(evidence.EvidenceSources, "port-security")
		case "dynamic":
			// Confidence depends on age
			if macEntry.Age <= 600 { // 10 minutes
				evidence.ConfidenceScore += 5
			}
		}
	}

	// Check DHCP snooping binding (PRIMARY evidence source)
	if dhcp, exists := e.DHCPBindings[macLower]; exists {
		evidence.DHCPBound = true
		evidence.ConfidenceScore += 40
		evidence.EvidenceSources = append(evidence.EvidenceSources, "dhcp-snooping")

		// Bonus for fresh binding
		if time.Since(dhcp.CollectedAt) <= 10*time.Minute {
			evidence.ConfidenceScore += 5
		}
	}

	// Check 802.1X/MAB authentication (CRITICAL for Allowed)
	if auth, exists := e.AuthSessions[macLower]; exists {
		evidence.Authenticated = true
		evidence.AuthMethod = auth.Method
		evidence.EvidenceSources = append(evidence.EvidenceSources, "dot1x")

		// High confidence boost for successful authentication
		if auth.Status == "Authc Success" || auth.Status == "Authorized" {
			evidence.ConfidenceScore += 35
		} else {
			evidence.ConfidenceScore += 10 // Auth enabled but not successful
		}
	}

	// Check IPSG binding
	if iface != "" {
		if ipsgList, exists := e.IPSGBindings[iface]; exists {
			for _, ipsg := range ipsgList {
				if strings.EqualFold(ipsg.MAC, mac) {
					evidence.IPSGBound = true
					evidence.ConfidenceScore += 15
					evidence.EvidenceSources = append(evidence.EvidenceSources, "ipsg")
					break
				}
			}
		}
	}

	// Check ARP table correlation
	if _, exists := e.ARPEntries[macLower]; exists {
		evidence.ARPEntryExists = true
		evidence.ConfidenceScore += 10
		evidence.EvidenceSources = append(evidence.EvidenceSources, "arp")
	}

	return evidence
}

// calculateSecurityScore computes a 0-100 security score for an interface.
func (e *L2DecisionEngine) calculateSecurityScore(iface string, evidence Evidence) int {
	if iface == "" {
		return evidence.ConfidenceScore
	}

	ifaceSec, exists := e.InterfaceSecurity[iface]
	if !exists {
		return evidence.ConfidenceScore
	}

	return ifaceSec.SecurityScore
}

// applyDecisionLogic implements the core decision algorithm.
func (e *L2DecisionEngine) applyDecisionLogic(srcEvidence, dstEvidence Evidence, overallConf int) (string, []string, string) {
	reasons := make([]string, 0)

	// ALLOWED: Authenticated + DHCP bound + high confidence
	if e.EnableStrictMode {
		if srcEvidence.Authenticated && srcEvidence.DHCPBound && overallConf >= e.AllowedThreshold {
			reasons = append(reasons, ReasonAuthenticatedAndBound)
			return "Allowed", reasons, "Authenticated (802.1X/MAB) + DHCP snooping bound + high confidence"
		}
	} else {
		// Less strict: Either authentication OR DHCP binding with high confidence
		if (srcEvidence.Authenticated || srcEvidence.DHCPBound) && overallConf >= e.AllowedThreshold {
			if srcEvidence.Authenticated {
				reasons = append(reasons, ReasonAuthenticatedHighConf)
				return "Allowed", reasons, fmt.Sprintf("Authenticated with high confidence (%d)", overallConf)
			} else {
				reasons = append(reasons, ReasonAuthenticatedAndBound)
				return "Allowed", reasons, fmt.Sprintf("DHCP bound with high confidence (%d)", overallConf)
			}
		}
	}

	// Static MAC entries get special treatment
	if srcEvidence.PortSecurityValid && srcEvidence.MACTableEntry {
		reasons = append(reasons, ReasonStaticMAC)
		return "Allowed", reasons, "Static/secure MAC entry (port security)"
	}

	// CONDITIONAL: Authenticated but missing DHCP binding
	if srcEvidence.Authenticated && !srcEvidence.DHCPBound && overallConf >= e.ConditionalThreshold {
		reasons = append(reasons, ReasonAuthenticatedNoDHCP)
		return "Conditional", reasons, "Authenticated but missing DHCP snooping binding"
	}

	// CONDITIONAL: DHCP bound but not authenticated
	if srcEvidence.DHCPBound && !srcEvidence.Authenticated && overallConf >= e.ConditionalThreshold {
		reasons = append(reasons, ReasonDHCPBoundNotAuth)
		return "Conditional", reasons, "DHCP snooping bound but not authenticated"
	}

	// CONDITIONAL: Medium confidence with partial evidence
	if overallConf >= e.ConditionalThreshold {
		reasons = append(reasons, ReasonMediumConfidence)
		return "Conditional", reasons, fmt.Sprintf("Medium confidence (%d) with partial evidence: %v",
			overallConf, srcEvidence.EvidenceSources)
	}

	// BLOCKED: Low confidence or insufficient evidence
	reasons = append(reasons, ReasonLowConfidence)
	return "Blocked", reasons, fmt.Sprintf("Low confidence (%d), insufficient evidence", overallConf)
}

// checkPVLANIsolation determines if PVLAN rules block communication between interfaces.
func (e *L2DecisionEngine) checkPVLANIsolation(pvlan PVLANIsolation, srcIface, dstIface string) bool {
	if srcIface == "" || dstIface == "" {
		return false
	}

	// Check if both are isolated ports (isolated ports cannot communicate with each other)
	srcIsolated := contains(pvlan.IsolatedPorts, srcIface)
	dstIsolated := contains(pvlan.IsolatedPorts, dstIface)

	if srcIsolated && dstIsolated {
		return true // Isolated ports cannot talk to each other
	}

	// Isolated ports can only talk to promiscuous ports
	if srcIsolated && !contains(pvlan.PromiscuousPorts, dstIface) {
		return true
	}
	if dstIsolated && !contains(pvlan.PromiscuousPorts, srcIface) {
		return true
	}

	return false
}

// Helper functions

func generateDecisionID(query L2ReachabilityQuery) string {
	return fmt.Sprintf("%s_%s_vlan%d_%d",
		strings.ReplaceAll(query.SourceMAC, ":", ""),
		strings.ReplaceAll(query.DestMAC, ":", ""),
		query.VLANID,
		time.Now().Unix(),
	)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// LoadDHCPBindings loads DHCP snooping bindings into the decision engine.
func (e *L2DecisionEngine) LoadDHCPBindings(bindings []DHCPBinding) {
	for _, binding := range bindings {
		e.DHCPBindings[strings.ToLower(binding.MAC)] = binding
	}
}

// LoadAuthSessions loads 802.1X/MAB authentication sessions into the decision engine.
func (e *L2DecisionEngine) LoadAuthSessions(sessions []AuthSession) {
	for _, session := range sessions {
		e.AuthSessions[strings.ToLower(session.MAC)] = session
	}
}

// LoadMACTable loads MAC address table entries into the decision engine.
func (e *L2DecisionEngine) LoadMACTable(entries []MACTableEntry) {
	for _, entry := range entries {
		e.MACTable[strings.ToLower(entry.MAC)] = entry
	}
}

// LoadInterfaceSecurity loads interface security state into the decision engine.
func (e *L2DecisionEngine) LoadInterfaceSecurity(states map[string]InterfaceSecurityState) {
	e.InterfaceSecurity = states
}

// LoadPVLANIsolation loads Private VLAN isolation rules into the decision engine.
func (e *L2DecisionEngine) LoadPVLANIsolation(isolation map[int]PVLANIsolation) {
	e.PVLANIsolation = isolation
}
