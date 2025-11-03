package appid

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

/*
Palo Alto App-ID Cache System

This package provides a caching mechanism for Palo Alto Networks App-ID
(Application Identification) database. App-ID is PAN-OS's Layer 7 application
identification technology that classifies traffic by application instead of
just port/protocol.

App-ID provides:
- Application → Ports/Protocols mapping
- Application risk levels
- Application categories
- Application behaviors

Data Source:
- Palo Alto Applipedia (https://applipedia.paloaltonetworks.com/)
- Downloaded and cached as JSON for offline lookups

Usage:
	loader := NewAppIDLoader()
	app, found := loader.GetApplication("ssl")
	ports := loader.GetDefaultPorts("web-browsing")
*/

// AppIDEntry represents a single App-ID application definition
type AppIDEntry struct {
	Name            string   `json:"name"`             // Application name (e.g., "ssl", "web-browsing")
	Category        string   `json:"category"`         // Category (e.g., "networking", "business-systems")
	SubCategory     string   `json:"subcategory"`      // Subcategory
	Technology      string   `json:"technology"`       // Technology (e.g., "client-server", "peer-to-peer")
	Description     string   `json:"description"`      // Human-readable description
	Risk            int      `json:"risk"`             // Risk level (1-5, 5=highest)
	DefaultPorts    []string `json:"default_ports"`    // Default TCP/UDP ports
	DefaultProtocol string   `json:"default_protocol"` // Default IP protocol (tcp/udp/both)
	UsedByMalware   bool     `json:"used_by_malware"`  // Whether commonly used by malware
	FileTypeIdent   bool     `json:"file_type_ident"`  // Can identify file types
	VirusIdent      bool     `json:"virus_ident"`      // Can identify viruses
	DataIdent       bool     `json:"data_ident"`       // Can identify data patterns
	Deprecated      bool     `json:"deprecated"`       // Application is deprecated
	ParentApp       string   `json:"parent_app"`       // Parent application (for dependent apps)
}

// AppIDCache represents the full App-ID database cache
type AppIDCache struct {
	Applications map[string]*AppIDEntry // Map of app name to entry
	Version      string                 // App-ID database version
	LastUpdated  string                 // Last update timestamp
	Source       string                 // Source (e.g., "Applipedia")

	mu sync.RWMutex // Protects concurrent access
}

// NewAppIDCache creates an empty App-ID cache
func NewAppIDCache() *AppIDCache {
	return &AppIDCache{
		Applications: make(map[string]*AppIDEntry),
		Version:      "unknown",
		LastUpdated:  "",
		Source:       "Palo Alto Applipedia",
	}
}

// LoadFromFile loads App-ID cache from a JSON file
func (c *AppIDCache) LoadFromFile(filePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read App-ID cache file: %w", err)
	}

	if err := json.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to parse App-ID cache JSON: %w", err)
	}

	return nil
}

// SaveToFile saves the App-ID cache to a JSON file
func (c *AppIDCache) SaveToFile(filePath string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal App-ID cache: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write App-ID cache file: %w", err)
	}

	return nil
}

// GetApplication retrieves an App-ID entry by name
func (c *AppIDCache) GetApplication(name string) (*AppIDEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.Applications[strings.ToLower(name)]
	return entry, ok
}

// GetDefaultPorts returns the default ports for an application
func (c *AppIDCache) GetDefaultPorts(appName string) []string {
	entry, ok := c.GetApplication(appName)
	if !ok {
		return nil
	}

	return entry.DefaultPorts
}

// GetDefaultProtocol returns the default protocol for an application
func (c *AppIDCache) GetDefaultProtocol(appName string) string {
	entry, ok := c.GetApplication(appName)
	if !ok {
		return ""
	}

	return entry.DefaultProtocol
}

// MapToService maps an App-ID application to a traditional service name
// This is useful for converting PAN-OS policies to port-based ACLs
func (c *AppIDCache) MapToService(appName string) (protocol string, ports []string, ok bool) {
	entry, found := c.GetApplication(appName)
	if !found {
		return "", nil, false
	}

	return entry.DefaultProtocol, entry.DefaultPorts, true
}

// GetApplicationsByCategory returns all applications in a given category
func (c *AppIDCache) GetApplicationsByCategory(category string) []*AppIDEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []*AppIDEntry

	for _, app := range c.Applications {
		if strings.EqualFold(app.Category, category) {
			results = append(results, app)
		}
	}

	return results
}

// GetHighRiskApplications returns all applications with risk level >= threshold
func (c *AppIDCache) GetHighRiskApplications(riskThreshold int) []*AppIDEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []*AppIDEntry

	for _, app := range c.Applications {
		if app.Risk >= riskThreshold {
			results = append(results, app)
		}
	}

	return results
}

// GetAllApplications returns a list of all application names
func (c *AppIDCache) GetAllApplications() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	apps := make([]string, 0, len(c.Applications))
	for name := range c.Applications {
		apps = append(apps, name)
	}

	return apps
}

// Count returns the total number of applications in the cache
func (c *AppIDCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.Applications)
}

// AddApplication adds or updates an application in the cache
func (c *AppIDCache) AddApplication(entry *AppIDEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Applications[strings.ToLower(entry.Name)] = entry
}

// Singleton instance for global access
var (
	globalAppIDCache *AppIDCache
	once             sync.Once
)

// GetGlobalAppIDCache returns the singleton App-ID cache instance
// Lazily loads from default location if not already loaded
func GetGlobalAppIDCache() *AppIDCache {
	once.Do(func() {
		globalAppIDCache = NewAppIDCache()

		// Try to load from default location
		defaultPath := filepath.Join("platform", "reference", "data", "palo_alto_app_id.json")
		if err := globalAppIDCache.LoadFromFile(defaultPath); err != nil {
			// Silently fail - cache will be empty but functional
			// Applications can still be added programmatically
		}
	})

	return globalAppIDCache
}

// InitializeGlobalAppIDCache explicitly initializes the global cache from a file
func InitializeGlobalAppIDCache(filePath string) error {
	cache := NewAppIDCache()
	if err := cache.LoadFromFile(filePath); err != nil {
		return err
	}

	globalAppIDCache = cache
	return nil
}
