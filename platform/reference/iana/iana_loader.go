package iana

import (
	"bufio"
	"embed"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:embed ../data/*.txt ../data/*.csv
var embeddedData embed.FS

// IANALoader provides access to IANA protocol, service, and EtherType mappings
type IANALoader struct {
	ProtocolMap       ProtocolMap
	ServiceMap        ServiceMap
	EtherTypeMap      EtherTypeMap
	ReverseServiceMap ReverseServiceMap

	mu sync.RWMutex // Protects concurrent access
}

// NewIANALoader creates and initializes an IANA loader
// It loads data from embedded files or external files if specified
func NewIANALoader() *IANALoader {
	loader := &IANALoader{
		ProtocolMap:       make(ProtocolMap),
		ServiceMap:        make(ServiceMap),
		EtherTypeMap:      EtherTypeMap{},
		ReverseServiceMap: make(ReverseServiceMap),
	}

	// Initialize with base protocol map
	for k, v := range BaseProtocolMap {
		loader.ProtocolMap[k] = v
	}

	// Load IANA data
	loader.loadProtocols()
	loader.loadServices()
	loader.loadEtherTypes()

	return loader
}

// NewIANALoaderFromFiles creates a loader using external files instead of embedded data
func NewIANALoaderFromFiles(dataDir string) (*IANALoader, error) {
	loader := &IANALoader{
		ProtocolMap:       make(ProtocolMap),
		ServiceMap:        make(ServiceMap),
		EtherTypeMap:      make(EtherTypeMap),
		ReverseServiceMap: make(ReverseServiceMap),
	}

	// Initialize with base protocol map
	for k, v := range BaseProtocolMap {
		loader.ProtocolMap[k] = v
	}

	// Load from external files
	if err := loader.loadProtocolsFromFile(filepath.Join(dataDir, "iana_protocol_numbers.txt")); err != nil {
		return nil, fmt.Errorf("failed to load protocols: %w", err)
	}

	if err := loader.loadServicesFromFile(filepath.Join(dataDir, "iana_service_names_port_numbers.txt")); err != nil {
		return nil, fmt.Errorf("failed to load services: %w", err)
	}

	if err := loader.loadEtherTypesFromFile(filepath.Join(dataDir, "ieee-802-numbers.csv")); err != nil {
		return nil, fmt.Errorf("failed to load EtherTypes: %w", err)
	}

	return loader, nil
}

// loadProtocols loads IANA protocol numbers from embedded data
func (l *IANALoader) loadProtocols() {
	data, err := embeddedData.ReadFile("../data/iana_protocol_numbers.txt")
	if err != nil {
		// Silently fail - base protocol map is already loaded
		return
	}

	l.parseProtocolData(string(data))
}

// loadProtocolsFromFile loads IANA protocol numbers from external file
func (l *IANALoader) loadProtocolsFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	l.parseProtocolData(string(data))
	return nil
}

// parseProtocolData parses IANA protocol number data
// Format: NUMBER    KEYWORD    PROTOCOL
// Example: 6        TCP        Transmission Control
func (l *IANALoader) parseProtocolData(data string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Regex: NUMBER followed by PROTOCOL_NAME
	// Example: "6        TCP        Transmission Control"
	protoRegex := regexp.MustCompile(`^(\d+)(?:-\d*)?\s+([+.\/a-zA-Z0-9_-]+)`)

	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := protoRegex.FindStringSubmatch(line)
		if len(matches) >= 3 {
			protoNum := matches[1]
			protoID := strings.ToLower(strings.TrimSpace(matches[2]))

			// Skip reserved/unassigned
			if protoID == "" || strings.Contains(protoID, "unassigned") ||
				strings.Contains(protoID, "reserved") {
				continue
			}

			// Handle special cases from original Python code
			switch protoNum {
			case "61":
				protoID = "any_host"
			case "63":
				protoID = "any_local_network"
			case "68":
				protoID = "any_dist_file_sys"
			case "99":
				protoID = "any_pvt_encryption"
			case "114":
				protoID = "any_0_hop"
			}

			l.ProtocolMap[protoID] = protoNum
		}
	}
}

// loadServices loads IANA service name and port number mappings
func (l *IANALoader) loadServices() {
	data, err := embeddedData.ReadFile("../data/iana_service_names_port_numbers.txt")
	if err != nil {
		// Fall back to Cisco default services only
		l.mu.Lock()
		for svcName, svcMap := range CiscoDefaultServices {
			l.ServiceMap[svcName] = svcMap
		}
		l.mu.Unlock()
		l.buildReverseServiceMap()
		return
	}

	l.parseServiceData(string(data))
}

// loadServicesFromFile loads services from external file
func (l *IANALoader) loadServicesFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	l.parseServiceData(string(data))
	return nil
}

// parseServiceData parses IANA service name and port number data
// Format (multi-line):
// service-name    PORT    PROTOCOL
// Optional continuation lines with just PORT and PROTOCOL
func (l *IANALoader) parseServiceData(data string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Start with Cisco default services
	for svcName, svcMap := range CiscoDefaultServices {
		l.ServiceMap[svcName] = copyServicePorts(svcMap)
	}

	validProtocols := map[string]bool{"tcp": true, "udp": true, "sctp": true, "dccp": true}

	// Regex patterns
	serviceLineRegex := regexp.MustCompile(`^([a-zA-Z0-9_-]+)\s+(\d+)(?:-\d*)?\s+(tcp|udp|sctp|dccp)`)
	portOnlyRegex := regexp.MustCompile(`^(\d+)(?:-\d*)?\s+(tcp|udp|sctp|dccp)`)

	var currentService string

	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Empty line resets current service
		if line == "" {
			currentService = ""
			continue
		}

		// Skip reserved/unassigned lines
		if strings.Contains(strings.ToLower(line), "reserved") ||
			strings.Contains(strings.ToLower(line), "unassigned") {
			currentService = ""
			continue
		}

		// Try to match full service line (name + port + protocol)
		if matches := serviceLineRegex.FindStringSubmatch(line); len(matches) == 4 {
			serviceName := matches[1]
			port := matches[2]
			protocol := matches[3]

			if !validProtocols[protocol] {
				continue
			}

			currentService = serviceName

			if _, exists := l.ServiceMap[serviceName]; !exists {
				l.ServiceMap[serviceName] = make(map[string][]string)
			}

			if _, exists := l.ServiceMap[serviceName][protocol]; !exists {
				l.ServiceMap[serviceName][protocol] = []string{}
			}

			// Add port if not already present
			if !contains(l.ServiceMap[serviceName][protocol], port) {
				l.ServiceMap[serviceName][protocol] = append(l.ServiceMap[serviceName][protocol], port)
			}

			continue
		}

		// Try to match continuation line (port + protocol only)
		if currentService != "" {
			if matches := portOnlyRegex.FindStringSubmatch(line); len(matches) == 3 {
				port := matches[1]
				protocol := matches[2]

				if !validProtocols[protocol] {
					continue
				}

				if _, exists := l.ServiceMap[currentService][protocol]; !exists {
					l.ServiceMap[currentService][protocol] = []string{}
				}

				// Add port if not already present
				if !contains(l.ServiceMap[currentService][protocol], port) {
					l.ServiceMap[currentService][protocol] = append(l.ServiceMap[currentService][protocol], port)
				}
			}
		}
	}

	// Build reverse lookup map
	l.buildReverseServiceMap()
}

// buildReverseServiceMap creates a reverse mapping from (protocol, port) to service name
func (l *IANALoader) buildReverseServiceMap() {
	// Note: mu should already be locked by caller
	for serviceName, protocols := range l.ServiceMap {
		for protocol, ports := range protocols {
			for _, port := range ports {
				key := ProtocolPort{Protocol: protocol, Port: port}
				// Last definition wins (matches Python behavior)
				l.ReverseServiceMap[key] = serviceName
			}
		}
	}
}

// loadEtherTypes loads IEEE 802 EtherType mappings
func (l *IANALoader) loadEtherTypes() {
	data, err := embeddedData.ReadFile("../data/ieee-802-numbers.csv")
	if err != nil {
		return
	}

	l.parseEtherTypeData(string(data))
}

// loadEtherTypesFromFile loads EtherTypes from external file
func (l *IANALoader) loadEtherTypesFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	l.parseEtherTypeData(string(data))
	return nil
}

// parseEtherTypeData parses IEEE 802 EtherType CSV data
// Expected header: Ethertype (decimal),Ethertype (hex),Exp. Ethernet (decimal),Exp. Ethernet (octal),Description,Reference
func (l *IANALoader) parseEtherTypeData(data string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	reader := csv.NewReader(strings.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return
	}

	for i, record := range records {
		// Skip header
		if i == 0 && strings.Contains(record[0], "Ethertype (decimal)") {
			continue
		}

		if len(record) < 6 {
			continue
		}

		decimal := strings.TrimSpace(record[0])
		description := strings.TrimSpace(record[4])

		if decimal == "" || description == "" || description == "-" {
			continue
		}

		// Handle ranges (e.g., "1536-1791")
		if strings.Contains(decimal, "-") {
			// For simplicity, just map the first value in the range
			parts := strings.Split(decimal, "-")
			if len(parts) >= 1 {
				l.EtherTypeMap[strings.TrimSpace(parts[0])] = description
			}
		} else {
			l.EtherTypeMap[decimal] = description
		}
	}
}

// GetProtocolNumber returns the protocol number for a given protocol name
func (l *IANALoader) GetProtocolNumber(protocolName string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	num, ok := l.ProtocolMap[strings.ToLower(protocolName)]
	return num, ok
}

// GetProtocolName returns the protocol name for a given protocol number
func (l *IANALoader) GetProtocolName(protocolNumber string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for name, num := range l.ProtocolMap {
		if num == protocolNumber {
			return name, true
		}
	}

	return "", false
}

// GetServiceFromPort returns the service name for a given port and protocol
func (l *IANALoader) GetServiceFromPort(port string, protocol string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	key := ProtocolPort{Protocol: strings.ToLower(protocol), Port: port}
	serviceName, ok := l.ReverseServiceMap[key]
	return serviceName, ok
}

// GetPortsForService returns all ports for a given service name and protocol
func (l *IANALoader) GetPortsForService(serviceName string, protocol string) ([]string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if protocols, exists := l.ServiceMap[strings.ToLower(serviceName)]; exists {
		if ports, exists := protocols[strings.ToLower(protocol)]; exists {
			return ports, true
		}
	}

	return nil, false
}

// GetAllProtocolsForService returns all protocols and ports for a service
func (l *IANALoader) GetAllProtocolsForService(serviceName string) (map[string][]string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	protocols, exists := l.ServiceMap[strings.ToLower(serviceName)]
	return protocols, exists
}

// GetEtherTypeName returns the description for a given EtherType decimal value
func (l *IANALoader) GetEtherTypeName(etherType string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	name, ok := l.EtherTypeMap[etherType]
	return name, ok
}

// GetAllProtocols returns a sorted list of all known protocol names
func (l *IANALoader) GetAllProtocols() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	protocols := make([]string, 0, len(l.ProtocolMap))
	for name := range l.ProtocolMap {
		protocols = append(protocols, name)
	}

	sort.Strings(protocols)
	return protocols
}

// GetAllServices returns a sorted list of all known service names
func (l *IANALoader) GetAllServices() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	services := make([]string, 0, len(l.ServiceMap))
	for name := range l.ServiceMap {
		services = append(services, name)
	}

	sort.Strings(services)
	return services
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func copyServicePorts(src map[string][]string) map[string][]string {
	dst := make(map[string][]string)
	for proto, ports := range src {
		dst[proto] = make([]string, len(ports))
		copy(dst[proto], ports)
	}
	return dst
}
