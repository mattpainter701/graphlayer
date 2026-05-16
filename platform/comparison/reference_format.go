// Package comparison provides the reference comparison harness for GraphLayer.
// Sprint 10: Objective, automated comparison showing where GraphLayer beats reference.
//
// This file defines the reference-compatible interchange format and translation
// functions that convert GraphLayer topology records into the comparison format.
package comparison

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ====================================================================
// reference-Compatible Format — the canonical comparison interchange
// ====================================================================

// referenceTopology is the reference-compatible topology representation.
// Both GraphLayer and reference exports are normalized to this format
// before comparison, ensuring fair, apples-to-apples evaluation.
type referenceTopology struct {
	// Metadata about this topology snapshot.
	Metadata referenceMetadata `json:"metadata"`

	// Devices discovered in the topology.
	Devices []referenceDevice `json:"devices"`

	// Links between devices.
	Links []referenceLink `json:"links"`
}

// referenceMetadata carries provenance and generation info.
type referenceMetadata struct {
	// Source is the tool that produced this topology (e.g., "graphlayer", "reference").
	Source string `json:"source"`

	// Version is the tool version or release tag.
	Version string `json:"version"`

	// GeneratedAt is when this topology was exported.
	GeneratedAt time.Time `json:"generated_at"`

	// TotalDevices is the claimed device count.
	TotalDevices int `json:"total_devices"`

	// TotalLinks is the claimed link count.
	TotalLinks int `json:"total_links"`

	// InputFiles lists the raw input files/sources.
	InputFiles []string `json:"input_files,omitempty"`

	// Extra carries arbitrary key-value metadata.
	Extra map[string]string `json:"extra,omitempty"`
}

// referenceDevice represents a single network device in the comparison format.
type referenceDevice struct {
	// ID is a unique identifier within this topology (e.g., management IP or serial).
	ID string `json:"id"`

	// Hostname is the configured hostname.
	Hostname string `json:"hostname"`

	// Vendor is the device vendor (e.g., "cisco", "arista", "dell").
	Vendor string `json:"vendor"`

	// Family is the product family (e.g., "IOS", "EOS", "FortiGate").
	Family string `json:"family,omitempty"`

	// Model is the hardware model (e.g., "N9K-C93180YC-FX").
	Model string `json:"model,omitempty"`

	// Serial is the device serial number.
	Serial string `json:"serial,omitempty"`

	// ManagementIP is the primary management IP address.
	ManagementIP string `json:"management_ip"`

	// DeviceType is the role/type (e.g., "switch", "router", "firewall").
	DeviceType string `json:"device_type"`

	// OSVersion is the operating system version.
	OSVersion string `json:"os_version,omitempty"`

	// InterfaceCount is the number of interfaces on this device.
	InterfaceCount int `json:"interface_count"`

	// HasConfig indicates whether a configuration was parsed.
	HasConfig bool `json:"has_config"`

	// HasOperational indicates whether operational data was collected.
	HasOperational bool `json:"has_operational"`

	// EvidenceCount is the number of evidence sources for this device.
	EvidenceCount int `json:"evidence_count"`

	// EvidenceSources lists the evidence source types.
	EvidenceSources []string `json:"evidence_sources,omitempty"`

	// Confidence is the overall confidence score (0.0–1.0).
	Confidence float64 `json:"confidence"`

	// Extra carries vendor-specific or tool-specific fields.
	Extra map[string]string `json:"extra,omitempty"`
}

// referenceLink represents a connection between two devices.
type referenceLink struct {
	// ID is a unique identifier for this link.
	ID string `json:"id"`

	// FromDevice is the source device ID (match key).
	FromDevice string `json:"from_device"`

	// FromPort is the source interface/port.
	FromPort string `json:"from_port"`

	// ToDevice is the destination device ID.
	ToDevice string `json:"to_device"`

	// ToPort is the destination interface/port.
	ToPort string `json:"to_port"`

	// LinkType describes the link (e.g., "l2", "l3", "vlt", "mlag", "port-channel").
	LinkType string `json:"link_type"`

	// Protocol is the discovery protocol (e.g., "lldp", "cdp", "inferred").
	Protocol string `json:"protocol,omitempty"`

	// EvidenceCount is the number of evidence sources confirming this link.
	EvidenceCount int `json:"evidence_count"`

	// Confidence is the overall confidence score (0.0–1.0).
	Confidence float64 `json:"confidence"`

	// Extra carries vendor-specific fields.
	Extra map[string]string `json:"extra,omitempty"`
}

// ====================================================================
// Translation: GraphLayer → reference format
// ====================================================================

// TopoExport represents a complete GraphLayer topology export for translation.
type TopoExport struct {
	Assets       []models.AssetNode
	Links        []models.Link
	L2Links      []models.L2Link
	Interfaces   []models.Interface
	RoutingTable []models.RoutingEntry
}

// TranslateAssetsToreference converts GraphLayer AssetNodes into reference devices.
func TranslateAssetsToreference(assets []models.AssetNode) []referenceDevice {
	devices := make([]referenceDevice, 0, len(assets))
	for _, a := range assets {
		d := referenceDevice{
			ID:              deviceID(a),
			Hostname:        a.Hostname,
			Vendor:          a.Vendor,
			Family:          a.Family,
			Model:           a.Model,
			Serial:          a.Serial,
			ManagementIP:    a.IP,
			DeviceType:      a.DeviceType,
			OSVersion:       a.OSVersion,
			InterfaceCount:  len(a.PhysicalNetworkInterfaces),
			HasConfig:        a.ConfigurationParsed,
			HasOperational:   len(a.PhysicalNetworkInterfaces) > 0,
			EvidenceCount:    evidenceCountFromAsset(a),
			EvidenceSources:  evidenceSourcesFromAsset(a),
			Confidence:       0.0, // filled by scoring engine
		}
		devices = append(devices, d)
	}
	return devices
}

// TranslateLinksToreference converts GraphLayer Links + L2Links into reference links.
func TranslateLinksToreference(links []models.Link, l2Links []models.L2Link) []referenceLink {
	referenceLinks := make([]referenceLink, 0)

	// Convert L3/L4 links
	for _, l := range links {
		nl := referenceLink{
			ID:         linkID(l),
			FromDevice: l.FromID,
			FromPort:   l.Interface.Name,
			ToDevice:   l.ToID,
			ToPort:     l.DestInterface.Name,
			LinkType:   "l3",
			Protocol:   l.Protocol,
			Confidence: 0.0,
		}
		referenceLinks = append(referenceLinks, nl)
	}

	// Convert L2 links
	for _, l := range l2Links {
		linkType := l.LinkType
		if linkType == "" {
			linkType = "l2"
		}
		nl := referenceLink{
			ID:         l2LinkID(l),
			FromDevice: l.FromDevice,
			FromPort:   l.FromPort,
			ToDevice:   l.ToDevice,
			ToPort:     l.ToPort,
			LinkType:   linkType,
			Confidence: 0.0,
		}
		referenceLinks = append(referenceLinks, nl)
	}

	return referenceLinks
}

// ====================================================================
// Helpers
// ====================================================================

// deviceID derives a stable match key for a device.
// Priority: Serial > ManagementIP > Hostname > random hash.
func deviceID(a models.AssetNode) string {
	if a.Serial != "" {
		return "serial:" + a.Serial
	}
	if a.IP != "" {
		return "ip:" + a.IP
	}
	if a.Hostname != "" {
		return "hostname:" + a.Hostname
	}
	// Fallback: hash of available fields
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s", a.Vendor, a.Family, a.Model, a.Mac)
	return "hash:" + hex.EncodeToString(h.Sum(nil))[:16]
}

// linkID derives a stable match key for a link.
func linkID(l models.Link) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s", l.FromID, l.Interface.Name, l.ToID, l.DestInterface.Name)
	return "link:" + hex.EncodeToString(h.Sum(nil))[:16]
}

// l2LinkID derives a stable match key for an L2 link.
func l2LinkID(l models.L2Link) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s", l.FromDevice, l.FromPort, l.ToDevice, l.ToPort)
	return "l2link:" + hex.EncodeToString(h.Sum(nil))[:16]
}

// evidenceCountFromAsset counts evidence sources for an asset.
func evidenceCountFromAsset(a models.AssetNode) int {
	count := 0
	if a.Integration != "" {
		count++
	}
	if a.ConfigurationParsed {
		count++
	}
	if a.UpdatedBy != "" {
		count++
	}
	return count
}

// evidenceSourcesFromAsset lists evidence source types for an asset.
func evidenceSourcesFromAsset(a models.AssetNode) []string {
	sources := make([]string, 0)
	if a.Integration != "" {
		sources = append(sources, a.Integration)
	}
	if a.ConfigurationParsed {
		sources = append(sources, "config")
	}
	if len(a.PhysicalNetworkInterfaces) > 0 {
		sources = append(sources, "operational")
	}
	return sources
}

// ComputeConfidence derives a confidence score from the asset's metadata.
func ComputeConfidence(a models.AssetNode) float64 {
	// Base confidence from integration quality
	score := 0.5

	// Config-parsed devices get a big boost
	if a.ConfigurationParsed {
		score += 0.30
	}

	// Operational data adds confidence
	if len(a.PhysicalNetworkInterfaces) > 0 {
		score += 0.10
	}

	// Evidence from multiple sources
	switch {
	case evidenceCountFromAsset(a) >= 3:
		score += 0.10
	case evidenceCountFromAsset(a) >= 2:
		score += 0.05
	}

	// Vendor known
	if a.Vendor != "" {
		score += 0.05
	}

	if score > 1.0 {
		score = 1.0
	}
	return score
}
