package models

// CheckPointDevice represents a Check Point device
type CheckPointDevice struct {
	Hostname     string
	IPAddress    string
	Version      string
	Model        string
	SerialNumber string
	Platform     string // Gaia, SecurePlatform, etc.
	Management   string // Standalone, Provider-1, MDS
}

// SecurityPolicy represents a Check Point security policy
type SecurityPolicy struct {
	Name           string
	Version        int
	InstallTargets []string
	Rules          []Rule
	GlobalRules    []Rule
	Metadata       map[string]string
}

// Rule represents a firewall rule
type Rule struct {
	Number      int
	Name        string
	Source      []string
	Destination []string
	Service     []string
	Action      string
	Track       string
	InstallOn   string
	Time        string
	Comments    string
	Enabled     bool
	UID         string
}

// NATRule represents a NAT rule
type NATRule struct {
	Number       int
	Name         string
	OriginalSrc  string
	OriginalDst  string
	OriginalSvc  string
	TranslatedSrc string
	TranslatedDst string
	TranslatedSvc string
	Enabled      bool
}

// VPNCommunity represents a VPN community
type VPNCommunity struct {
	Name        string
	Type        string // Star, Mesh, etc.
	Participants []string
	Encryption   string
	Authentication string
}

// Cluster represents a Check Point cluster
type Cluster struct {
	Name           string
	Members        []ClusterMember
	ClusterIP      string
	State          string
	Version        string
}

// ClusterMember represents a cluster member
type ClusterMember struct {
	Name      string
	IPAddress string
	Priority  int
	Status    string
}
