package api

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/reference/appid"
)

/*
PAN-OS App-ID Collector

Collects App-ID database from PAN-OS firewall or Panorama via XML API.

API Endpoint: /api/?type=op&cmd=<show><predefined><application></application></predefined></show>

Features:
- Uses existing API connector authentication
- Handles large XML responses with streaming
- Detects partial fetches (timeout, API errors)
- Converts XML to AppIDCache format
*/

// PANOSAppIDCollector collects App-ID data from PAN-OS devices
type PANOSAppIDCollector struct {
	connector *APIConnector
	deviceID  string
	workDir   string
	logger    appid.Logger
}

// NewPANOSAppIDCollector creates a new App-ID collector
func NewPANOSAppIDCollector(connector *APIConnector, deviceID string, workDir string) *PANOSAppIDCollector {
	return &PANOSAppIDCollector{
		connector: connector,
		deviceID:  deviceID,
		workDir:   workDir,
		logger:    &defaultCollectorLogger{},
	}
}

// SetLogger sets a custom logger
func (c *PANOSAppIDCollector) SetLogger(logger appid.Logger) {
	c.logger = logger
}

// ========================================
// Main Collection Method
// ========================================

// CollectAppID collects the complete App-ID database from PAN-OS
func (c *PANOSAppIDCollector) CollectAppID(ctx context.Context) (*appid.CollectionResult, error) {
	result := appid.NewCollectionResult(c.deviceID, "panos")
	result.Metadata.Source = "panos-xml-api"

	c.logger.Info("Starting App-ID collection for device: %s", c.deviceID)

	// Apply collection timeout
	ctx, cancel := context.WithTimeout(ctx, appid.CollectionTimeout)
	defer cancel()

	// Get PAN-OS version first (for metadata)
	panosVersion, err := c.getPANOSVersion(ctx)
	if err != nil {
		c.logger.Warn("Could not get PAN-OS version: %v (continuing anyway)", err)
		panosVersion = "unknown"
	}
	result.Metadata.FirmwareVersion = panosVersion

	// Fetch predefined applications via XML API
	xmlData, err := c.fetchPredefinedApps(ctx)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			c.logger.Error("App-ID collection timed out after %v", appid.CollectionTimeout)
			result.Complete(appid.FetchStatusPartialTimeout)
			return result, fmt.Errorf("collection timeout: %w", err)
		}
		result.AddError(err)
		result.Complete(appid.FetchStatusFailed)
		return result, fmt.Errorf("failed to fetch App-ID data: %w", err)
	}

	// Parse XML response to AppIDCache
	cache, contentVersion, err := c.parseXMLResponse(xmlData)
	if err != nil {
		result.AddError(err)
		result.Complete(appid.FetchStatusFailed)
		return result, fmt.Errorf("failed to parse App-ID XML: %w", err)
	}

	result.AppIDCache = cache
	result.AppsCollected = len(cache.Applications)
	result.Metadata.ContentVersion = contentVersion
	result.Metadata.ActualTotal = len(cache.Applications)

	// Determine if this was a complete fetch
	if result.AppsCollected >= 1000 {
		// Reasonable App-ID count, consider it complete
		result.Metadata.Completeness = 1.0
		result.Complete(appid.FetchStatusFullSuccess)
		c.logger.Info("App-ID collection successful: %d applications", result.AppsCollected)
	} else {
		// Suspiciously low count, might be partial
		result.Metadata.Completeness = 0.5 // Unknown completeness
		result.Complete(appid.FetchStatusPartialIncomplete)
		c.logger.Warn("App-ID collection completed with only %d apps (expected >1000)", result.AppsCollected)
	}

	return result, nil
}

// ========================================
// XML API Methods
// ========================================

// fetchPredefinedApps fetches the predefined application list via XML API
func (c *PANOSAppIDCollector) fetchPredefinedApps(ctx context.Context) ([]byte, error) {
	// XML command to get predefined applications
	cmd := "<show><predefined><application></application></predefined></show>"

	// Build API request URL
	url := fmt.Sprintf("%s/api/?type=op&cmd=%s&key=%s",
		c.connector.baseURL,
		cmd,
		c.connector.config.APIKey)

	c.logger.Debug("Fetching App-ID from: %s", c.connector.baseURL)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := c.connector.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read response body
	xmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	c.logger.Debug("Received XML response: %d bytes", len(xmlData))

	return xmlData, nil
}

// getPANOSVersion gets the PAN-OS version via XML API
func (c *PANOSAppIDCollector) getPANOSVersion(ctx context.Context) (string, error) {
	cmd := "<show><system><info></info></system></show>"

	url := fmt.Sprintf("%s/api/?type=op&cmd=%s&key=%s",
		c.connector.baseURL,
		cmd,
		c.connector.config.APIKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.connector.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	xmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Parse version from XML
	type SystemInfo struct {
		Version string `xml:"result>system>sw-version"`
	}

	var info SystemInfo
	if err := xml.Unmarshal(xmlData, &info); err != nil {
		return "", err
	}

	return info.Version, nil
}

// ========================================
// XML Parsing
// ========================================

// parseXMLResponse parses PAN-OS XML response into AppIDCache
func (c *PANOSAppIDCollector) parseXMLResponse(xmlData []byte) (*appid.AppIDCache, string, error) {
	// PAN-OS XML structure for predefined applications
	type Entry struct {
		Name            string `xml:"name"`
		Category        string `xml:"category"`
		SubCategory     string `xml:"subcategory"`
		Technology      string `xml:"technology"`
		Description     string `xml:"description"`
		Risk            int    `xml:"risk"`
		DefaultIdent    string `xml:"default>ident-by-ip-protocol"`
		DefaultPort     string `xml:"default>port"`
		UseApplications string `xml:"use-applications>member"`
		Deprecated      string `xml:"deprecated"`
	}

	type Response struct {
		Status  string  `xml:"status,attr"`
		Version string  `xml:"result>application>version"`
		Entries []Entry `xml:"result>application>entry"`
	}

	var resp Response
	if err := xml.Unmarshal(xmlData, &resp); err != nil {
		return nil, "", fmt.Errorf("XML parse error: %w", err)
	}

	// Check API response status
	if resp.Status != "success" {
		return nil, "", fmt.Errorf("API returned status: %s", resp.Status)
	}

	c.logger.Debug("Parsed %d application entries from XML", len(resp.Entries))

	// Convert to AppIDCache
	cache := appid.NewAppIDCache()
	cache.Version = resp.Version
	cache.LastUpdated = time.Now().Format(time.RFC3339)
	cache.Source = "panos-xml-api"

	for _, entry := range resp.Entries {
		appEntry := &appid.AppIDEntry{
			Name:            entry.Name,
			Category:        entry.Category,
			SubCategory:     entry.SubCategory,
			Technology:      entry.Technology,
			Description:     entry.Description,
			Risk:            entry.Risk,
			Deprecated:      entry.Deprecated == "yes",
			ParentApp:       entry.UseApplications,
		}

		// Parse default protocol and ports
		protocol, ports := c.parseDefaultService(entry.DefaultIdent, entry.DefaultPort)
		appEntry.DefaultProtocol = protocol
		appEntry.DefaultPorts = ports

		// Add to cache
		cache.Applications[strings.ToLower(entry.Name)] = appEntry
	}

	c.logger.Info("Created App-ID cache with %d applications (version: %s)",
		len(cache.Applications), cache.Version)

	return cache, resp.Version, nil
}

// parseDefaultService extracts protocol and ports from PAN-OS default service fields
func (c *PANOSAppIDCollector) parseDefaultService(identByProto string, portField string) (string, []string) {
	var protocol string
	var ports []string

	// Parse protocol from ident-by-ip-protocol field
	switch strings.ToLower(identByProto) {
	case "tcp":
		protocol = "tcp"
	case "udp":
		protocol = "udp"
	case "icmp":
		protocol = "icmp"
	case "icmp6":
		protocol = "icmp6"
	default:
		protocol = "tcp" // Default to TCP if unknown
	}

	// Parse ports from port field
	// Format can be: "tcp/80", "udp/53", "tcp/80,443", etc.
	if portField != "" {
		// Split by comma for multiple ports
		portList := strings.Split(portField, ",")
		for _, p := range portList {
			// Remove protocol prefix if present
			p = strings.TrimSpace(p)
			if idx := strings.Index(p, "/"); idx != -1 {
				p = p[idx+1:]
			}
			if p != "" && p != "0" {
				ports = append(ports, p)
			}
		}
	}

	return protocol, ports
}

// ========================================
// Helper Types
// ========================================

// defaultCollectorLogger provides basic logging for collector
type defaultCollectorLogger struct{}

func (l *defaultCollectorLogger) Info(format string, args ...interface{})  { fmt.Printf("[INFO] "+format+"\n", args...) }
func (l *defaultCollectorLogger) Warn(format string, args ...interface{})  { fmt.Printf("[WARN] "+format+"\n", args...) }
func (l *defaultCollectorLogger) Error(format string, args ...interface{}) { fmt.Printf("[ERROR] "+format+"\n", args...) }
func (l *defaultCollectorLogger) Debug(format string, args ...interface{}) { fmt.Printf("[DEBUG] "+format+"\n", args...) }

// ========================================
// Validation and Health Checks
// ========================================

// ValidateAPIAccess validates that the API connector can access PAN-OS
func (c *PANOSAppIDCollector) ValidateAPIAccess(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Try to get system info as a validation check
	_, err := c.getPANOSVersion(ctx)
	if err != nil {
		return fmt.Errorf("API access validation failed: %w", err)
	}

	return nil
}

// EstimateCollectionTime estimates how long collection will take
func (c *PANOSAppIDCollector) EstimateCollectionTime() time.Duration {
	// App-ID collection typically takes 30 seconds to 2 minutes
	// depending on device model and number of applications
	return 2 * time.Minute
}
