package appid

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

/*
Fortinet ISDB (Internet Service Database) Cache System

FortiGate firewalls use the ISDB to identify applications and services.
This package provides caching and lookup for ISDB entries.

ISDB provides:
- Service ID → Application name mapping
- Protocols and ports for each service
- Categories for grouping

Data Source:
- FortiGate via "diagnose internet-service id" command
- Downloaded using fortinet_isdb_exporter.py script
- Cached as JSON for offline lookups

Usage:
	loader := NewISDBCache()
	service, found := loader.GetService("1") // Get service ID 1
	ports := loader.GetPorts("Google")
*/

// ISDBServiceEntry represents a single ISDB service entry
type ISDBServiceEntry struct {
	ID         string   `json:"id"`          // Service ID
	Name       string   `json:"name"`        // Service name (e.g., "Google", "Facebook")
	Protocols  []string `json:"protocols"`   // Protocols (e.g., ["TCP"], ["UDP"], ["TCP", "UDP"])
	Ports      []string `json:"ports"`       // Port numbers
	Categories []string `json:"categories"`  // Categories (e.g., ["Search-Engines"])
	Content    string   `json:"content"`     // Raw output from FortiGate
	Date       string   `json:"date"`        // Date retrieved
	Version    string   `json:"version"`     // ISDB version
}

// ISDBCache represents the full ISDB database cache
type ISDBCache struct {
	Services    map[string]*ISDBServiceEntry // Map of service ID to entry
	ServicesByName map[string]*ISDBServiceEntry // Map of service name to entry
	Version     string                        // ISDB version
	Date        string                        // Last update date
	Server      string                        // Source FortiGate server

	mu sync.RWMutex // Protects concurrent access
}

// ISDBFileFormat represents the JSON file structure from fortinet_isdb_exporter.py
type ISDBFileFormat struct {
	Version string                       `json:"version"`
	Date    string                       `json:"date"`
	Server  string                       `json:"server"`
	ISDB    map[string]map[string]string `json:"isdb"` // ID -> {"name": "...", "content": "..."}
}

// NewISDBCache creates an empty ISDB cache
func NewISDBCache() *ISDBCache {
	return &ISDBCache{
		Services:       make(map[string]*ISDBServiceEntry),
		ServicesByName: make(map[string]*ISDBServiceEntry),
		Version:        "unknown",
		Date:           "",
		Server:         "unknown",
	}
}

// LoadFromFile loads ISDB cache from a JSON file (fortinet_isdb_exporter.py output)
func (c *ISDBCache) LoadFromFile(filePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read ISDB cache file: %w", err)
	}

	var fileData ISDBFileFormat
	if err := json.Unmarshal(data, &fileData); err != nil {
		return fmt.Errorf("failed to parse ISDB cache JSON: %w", err)
	}

	c.Version = fileData.Version
	c.Date = fileData.Date
	c.Server = fileData.Server

	// Parse each ISDB entry
	for id, entryData := range fileData.ISDB {
		if name, ok := entryData["name"]; ok {
			content := entryData["content"]
			entry := c.parseISDBContent(id, name, content)
			entry.Date = fileData.Date
			entry.Version = fileData.Version

			c.Services[id] = entry
			c.ServicesByName[strings.ToLower(name)] = entry
		}
	}

	return nil
}

// parseISDBContent parses the raw ISDB "content" field from FortiGate output
// Example content:
// ID: 1
// Name: Google
// Protocols: TCP
// Ports: 80,443
// Categories: Search-Engines
func (c *ISDBCache) parseISDBContent(id, name, content string) *ISDBServiceEntry {
	entry := &ISDBServiceEntry{
		ID:         id,
		Name:       name,
		Protocols:  []string{},
		Ports:      []string{},
		Categories: []string{},
		Content:    content,
	}

	// Parse protocols
	if match := regexp.MustCompile(`Protocols?:\s*([^\n]+)`).FindStringSubmatch(content); len(match) > 1 {
		protoStr := strings.TrimSpace(match[1])
		for _, proto := range strings.Split(protoStr, ",") {
			proto = strings.ToLower(strings.TrimSpace(proto))
			if proto != "" {
				entry.Protocols = append(entry.Protocols, proto)
			}
		}
	}

	// Parse ports
	if match := regexp.MustCompile(`Ports?:\s*([^\n]+)`).FindStringSubmatch(content); len(match) > 1 {
		portStr := strings.TrimSpace(match[1])
		for _, port := range strings.Split(portStr, ",") {
			port = strings.TrimSpace(port)
			if port != "" && port != "-" {
				entry.Ports = append(entry.Ports, port)
			}
		}
	}

	// Parse categories
	if match := regexp.MustCompile(`Categories?:\s*([^\n]+)`).FindStringSubmatch(content); len(match) > 1 {
		catStr := strings.TrimSpace(match[1])
		for _, cat := range strings.Split(catStr, ",") {
			cat = strings.TrimSpace(cat)
			if cat != "" {
				entry.Categories = append(entry.Categories, cat)
			}
		}
	}

	return entry
}

// SaveToFile saves the ISDB cache to a JSON file
func (c *ISDBCache) SaveToFile(filePath string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Convert to file format
	fileData := ISDBFileFormat{
		Version: c.Version,
		Date:    c.Date,
		Server:  c.Server,
		ISDB:    make(map[string]map[string]string),
	}

	for id, entry := range c.Services {
		fileData.ISDB[id] = map[string]string{
			"name":    entry.Name,
			"content": entry.Content,
			"date":    entry.Date,
			"version": entry.Version,
		}
	}

	data, err := json.MarshalIndent(fileData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ISDB cache: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write ISDB cache file: %w", err)
	}

	return nil
}

// GetService retrieves an ISDB service by ID
func (c *ISDBCache) GetService(id string) (*ISDBServiceEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.Services[id]
	return entry, ok
}

// GetServiceByName retrieves an ISDB service by name (case-insensitive)
func (c *ISDBCache) GetServiceByName(name string) (*ISDBServiceEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.ServicesByName[strings.ToLower(name)]
	return entry, ok
}

// GetPorts returns the ports for a service (by name or ID)
func (c *ISDBCache) GetPorts(nameOrID string) []string {
	// Try by ID first
	if entry, ok := c.GetService(nameOrID); ok {
		return entry.Ports
	}

	// Try by name
	if entry, ok := c.GetServiceByName(nameOrID); ok {
		return entry.Ports
	}

	return nil
}

// GetProtocols returns the protocols for a service (by name or ID)
func (c *ISDBCache) GetProtocols(nameOrID string) []string {
	// Try by ID first
	if entry, ok := c.GetService(nameOrID); ok {
		return entry.Protocols
	}

	// Try by name
	if entry, ok := c.GetServiceByName(nameOrID); ok {
		return entry.Protocols
	}

	return nil
}

// MapToService maps an ISDB service to protocol and ports
// This is useful for converting FortiGate policies to traditional ACLs
func (c *ISDBCache) MapToService(nameOrID string) (protocols []string, ports []string, ok bool) {
	var entry *ISDBServiceEntry

	// Try by ID first
	if e, found := c.GetService(nameOrID); found {
		entry = e
	} else if e, found := c.GetServiceByName(nameOrID); found {
		entry = e
	} else {
		return nil, nil, false
	}

	return entry.Protocols, entry.Ports, true
}

// GetServicesByCategory returns all services in a given category
func (c *ISDBCache) GetServicesByCategory(category string) []*ISDBServiceEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []*ISDBServiceEntry

	for _, service := range c.Services {
		for _, cat := range service.Categories {
			if strings.EqualFold(cat, category) {
				results = append(results, service)
				break
			}
		}
	}

	return results
}

// GetAllServices returns a list of all service names
func (c *ISDBCache) GetAllServices() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	services := make([]string, 0, len(c.Services))
	for _, entry := range c.Services {
		services = append(services, entry.Name)
	}

	return services
}

// Count returns the total number of services in the cache
func (c *ISDBCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.Services)
}

// AddService adds or updates a service in the cache
func (c *ISDBCache) AddService(entry *ISDBServiceEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Services[entry.ID] = entry
	c.ServicesByName[strings.ToLower(entry.Name)] = entry
}

// Singleton instance for global access
var (
	globalISDBCache *ISDBCache
	isdbOnce        sync.Once
)

// GetGlobalISDBCache returns the singleton ISDB cache instance
// Lazily loads from default location if not already loaded
func GetGlobalISDBCache() *ISDBCache {
	isdbOnce.Do(func() {
		globalISDBCache = NewISDBCache()

		// Try to load from default location
		defaultPath := filepath.Join("platform", "reference", "data", "fortinet_isdb.json")
		if err := globalISDBCache.LoadFromFile(defaultPath); err != nil {
			// Silently fail - cache will be empty but functional
		}
	})

	return globalISDBCache
}

// InitializeGlobalISDBCache explicitly initializes the global cache from a file
func InitializeGlobalISDBCache(filePath string) error {
	cache := NewISDBCache()
	if err := cache.LoadFromFile(filePath); err != nil {
		return err
	}

	globalISDBCache = cache
	return nil
}
