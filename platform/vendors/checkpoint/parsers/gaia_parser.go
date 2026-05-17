package parsers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// GaiaConfig represents parsed Gaia configuration
type GaiaConfig struct {
	Hostname       string
	Interfaces     []GaiaInterface
	Routes         []GaiaRoute
	DNS            GaiaDNS
	NTP            []string
	SNMP           GaiaSNMP
	Users          []GaiaUser
	Services       []GaiaService
	SecurityPolicy GaiaSecurityPolicy
}

// GaiaInterface represents a network interface
type GaiaInterface struct {
	Name       string
	IPAddress  string
	Netmask    string
	Status     string
	MTU        int
	Comments   string
}

// GaiaRoute represents a routing table entry
type GaiaRoute struct {
	Destination string
	Netmask     string
	Gateway     string
	Interface   string
	Metric      int
}

// GaiaDNS represents DNS configuration
type GaiaDNS struct {
	Primary   string
	Secondary string
	Suffixes  []string
}

// GaiaSNMP represents SNMP configuration
type GaiaSNMP struct {
	Enabled    bool
	Community  string
	Location   string
	Contact    string
}

// GaiaUser represents a local user
type GaiaUser struct {
	Username string
	UID      int
	Groups   []string
	Shell    string
}

// GaiaService represents a system service
type GaiaService struct {
	Name    string
	Enabled bool
	Port    int
}

// GaiaSecurityPolicy represents security policy rules
type GaiaSecurityPolicy struct {
	Rules []GaiaRule
}

// GaiaRule represents a single security rule
type GaiaRule struct {
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
}

// ParseGaiaConfig parses Gaia CLISH show configuration output
func ParseGaiaConfig(config string) (*GaiaConfig, error) {
	gc := &GaiaConfig{
		Interfaces: []GaiaInterface{},
		Routes:     []GaiaRoute{},
		NTP:        []string{},
		Users:      []GaiaUser{},
		Services:   []GaiaService{},
	}

	lines := strings.Split(config, "\n")
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Detect sections
		if strings.HasPrefix(line, "hostname") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				gc.Hostname = parts[1]
			}
			continue
		}

		if strings.HasPrefix(line, "interface") {
			currentSection = "interface"
			parseGaiaInterface(line, gc)
			continue
		}

		if strings.HasPrefix(line, "ip route") {
			currentSection = "route"
			parseGaiaRoute(line, gc)
			continue
		}

		if strings.HasPrefix(line, "dns") {
			currentSection = "dns"
			parseGaiaDNS(line, gc)
			continue
		}

		if strings.HasPrefix(line, "ntp") {
			currentSection = "ntp"
			parseGaiaNTP(line, gc)
			continue
		}

		if strings.HasPrefix(line, "snmp") {
			currentSection = "snmp"
			parseGaiaSNMP(line, gc)
			continue
		}

		if strings.HasPrefix(line, "user") {
			currentSection = "user"
			parseGaiaUser(line, gc)
			continue
		}

		if strings.HasPrefix(line, "service") {
			currentSection = "service"
			parseGaiaService(line, gc)
			continue
		}
	}

	return gc, nil
}

func parseGaiaInterface(line string, gc *GaiaConfig) {
	re := regexp.MustCompile(`interface\s+(\S+)\s+ip\s+address\s+(\S+)\s+netmask\s+(\S+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 4 {
		iface := GaiaInterface{
			Name:      matches[1],
			IPAddress: matches[2],
			Netmask:   matches[3],
			Status:    "up",
			MTU:       1500,
		}
		gc.Interfaces = append(gc.Interfaces, iface)
	}
}

func parseGaiaRoute(line string, gc *GaiaConfig) {
	re := regexp.MustCompile(`ip\s+route\s+(\S+)\s+(\S+)\s+(\S+)\s*(\S+)?`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 4 {
		route := GaiaRoute{
			Destination: matches[1],
			Netmask:     matches[2],
			Gateway:     matches[3],
			Metric:      1,
		}
		if len(matches) >= 5 {
			route.Interface = matches[4]
		}
		gc.Routes = append(gc.Routes, route)
	}
}

func parseGaiaDNS(line string, gc *GaiaConfig) {
	re := regexp.MustCompile(`dns\s+(primary|secondary)\s+(\S+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 3 {
		if matches[1] == "primary" {
			gc.DNS.Primary = matches[2]
		} else if matches[1] == "secondary" {
			gc.DNS.Secondary = matches[2]
		}
	}
}

func parseGaiaNTP(line string, gc *GaiaConfig) {
	re := regexp.MustCompile(`ntp\s+server\s+(\S+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 2 {
		gc.NTP = append(gc.NTP, matches[1])
	}
}

func parseGaiaSNMP(line string, gc *GaiaConfig) {
	re := regexp.MustCompile(`snmp\s+(enable|disable|community|location|contact)\s*(.*)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 2 {
		switch matches[1] {
		case "enable":
			gc.SNMP.Enabled = true
		case "disable":
			gc.SNMP.Enabled = false
		case "community":
			gc.SNMP.Community = strings.TrimSpace(matches[2])
		case "location":
			gc.SNMP.Location = strings.TrimSpace(matches[2])
		case "contact":
			gc.SNMP.Contact = strings.TrimSpace(matches[2])
		}
	}
}

func parseGaiaUser(line string, gc *GaiaConfig) {
	re := regexp.MustCompile(`user\s+(\S+)\s+uid\s+(\d+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 3 {
		uid, _ := strconv.Atoi(matches[2])
		user := GaiaUser{
			Username: matches[1],
			UID:      uid,
			Groups:   []string{},
			Shell:    "/bin/bash",
		}
		gc.Users = append(gc.Users, user)
	}
}

func parseGaiaService(line string, gc *GaiaConfig) {
	re := regexp.MustCompile(`service\s+(\S+)\s+(enable|disable)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 3 {
		service := GaiaService{
			Name:    matches[1],
			Enabled: matches[2] == "enable",
			Port:    0,
		}
		gc.Services = append(gc.Services, service)
	}
}

// ParseGaiaCLISH parses CLISH command output
func ParseGaiaCLISH(output string) (*GaiaConfig, error) {
	return ParseGaiaConfig(output)
}

// ParseGaiaShowConfig parses 'show configuration' output
func ParseGaiaShowConfig(output string) (*GaiaConfig, error) {
	return ParseGaiaConfig(output)
}
