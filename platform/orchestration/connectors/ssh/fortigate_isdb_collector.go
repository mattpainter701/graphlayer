package ssh

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/reference/appid"
)

/*
FortiGate ISDB Collector

Collects Internet Service Database (ISDB) from FortiGate via SSH CLI.

CLI Command: diagnose internet-service id {id}

Features:
- Uses existing SSH connector
- Chunks queries to avoid timeouts (1000 entries per batch)
- Parses FortiOS 6.x and 7.x output formats
- Detects partial fetches and handles errors gracefully
*/

// FortiGateISDBCollector collects ISDB data from FortiGate devices
type FortiGateISDBCollector struct {
	connector *SSHConnector
	deviceID  string
	workDir   string
	logger    appid.Logger
}

// NewFortiGateISDBCollector creates a new ISDB collector
func NewFortiGateISDBCollector(connector *SSHConnector, deviceID string, workDir string) *FortiGateISDBCollector {
	return &FortiGateISDBCollector{
		connector: connector,
		deviceID:  deviceID,
		workDir:   workDir,
		logger:    &defaultCollectorLogger{},
	}
}

// SetLogger sets a custom logger
func (c *FortiGateISDBCollector) SetLogger(logger appid.Logger) {
	c.logger = logger
}

// ========================================
// Main Collection Method
// ========================================

// CollectISDB collects the complete ISDB from FortiGate
func (c *FortiGateISDBCollector) CollectISDB(ctx context.Context) (*appid.CollectionResult, error) {
	result := appid.NewCollectionResult(c.deviceID, "fortios")
	result.Metadata.Source = "fortigate-ssh-cli"

	c.logger.Info("Starting ISDB collection for device: %s", c.deviceID)

	// Apply collection timeout
	ctx, cancel := context.WithTimeout(ctx, appid.CollectionTimeout)
	defer cancel()

	// Get FortiOS version first (for metadata and format detection)
	fortiosVersion, err := c.getFortiOSVersion(ctx)
	if err != nil {
		c.logger.Warn("Could not get FortiOS version: %v (continuing anyway)", err)
		fortiosVersion = "unknown"
	}
	result.Metadata.FirmwareVersion = fortiosVersion

	// Get ISDB version
	isdbVersion, err := c.getISDBVersion(ctx)
	if err != nil {
		c.logger.Warn("Could not get ISDB version: %v", err)
		isdbVersion = "unknown"
	}
	result.Metadata.ContentVersion = isdbVersion

	// Determine ISDB range (max ID)
	maxID, err := c.getISDBRange(ctx)
	if err != nil {
		result.AddError(err)
		result.Complete(appid.FetchStatusFailed)
		return result, fmt.Errorf("failed to determine ISDB range: %w", err)
	}

	c.logger.Info("ISDB range: 1 to %d", maxID)
	result.Metadata.ExpectedTotal = maxID

	// Create cache
	cache := appid.NewISDBCache()
	cache.Version = isdbVersion
	cache.Date = time.Now().Format(time.RFC3339)
	cache.Server = c.deviceID

	// Collect ISDB in chunks
	totalCollected := 0
	for start := 1; start <= maxID; start += appid.ISDBChunkSize {
		// Check context for cancellation
		if ctx.Err() != nil {
			c.logger.Warn("ISDB collection cancelled/timeout at ID %d", start)
			result.Metadata.ActualTotal = totalCollected
			result.Metadata.Completeness = float64(totalCollected) / float64(maxID)
			result.Complete(appid.FetchStatusPartialTimeout)
			result.ISDBCache = cache
			result.ServicesCollected = len(cache.Services)
			return result, ctx.Err()
		}

		end := start + appid.ISDBChunkSize - 1
		if end > maxID {
			end = maxID
		}

		c.logger.Debug("Fetching ISDB range: %d-%d", start, end)

		// Fetch chunk with retry
		chunkCache, err := c.fetchISDBRangeWithRetry(ctx, start, end)
		if err != nil {
			c.logger.Error("Failed to fetch ISDB range %d-%d: %v", start, end, err)
			result.AddError(fmt.Errorf("chunk %d-%d failed: %w", start, end, err))

			// Don't fail completely, continue with partial data
			result.Metadata.ActualTotal = totalCollected
			result.Metadata.Completeness = float64(totalCollected) / float64(maxID)
			result.Complete(appid.FetchStatusPartialIncomplete)
			result.ISDBCache = cache
			result.ServicesCollected = len(cache.Services)
			return result, nil
		}

		// Merge chunk into main cache
		for id, service := range chunkCache.Services {
			cache.Services[id] = service
			cache.ServicesByName[strings.ToLower(service.Name)] = service
			totalCollected++
		}

		c.logger.Debug("Collected %d services from chunk (total: %d)", len(chunkCache.Services), totalCollected)

		// Rate limiting between chunks
		time.Sleep(appid.SSHCommandDelay)
	}

	result.ISDBCache = cache
	result.ServicesCollected = len(cache.Services)
	result.Metadata.ActualTotal = totalCollected
	result.Metadata.Completeness = float64(totalCollected) / float64(maxID)

	// Determine final status
	if result.Metadata.Completeness >= appid.MinCompletionPercentage {
		result.Complete(appid.FetchStatusFullSuccess)
		c.logger.Info("ISDB collection successful: %d services (%.1f%% complete)",
			result.ServicesCollected, result.Metadata.Completeness*100)
	} else {
		result.Complete(appid.FetchStatusPartialIncomplete)
		c.logger.Warn("ISDB collection incomplete: %d services (%.1f%% complete)",
			result.ServicesCollected, result.Metadata.Completeness*100)
	}

	return result, nil
}

// ========================================
// SSH CLI Methods
// ========================================

// fetchISDBRangeWithRetry fetches an ISDB range with retry logic
func (c *FortiGateISDBCollector) fetchISDBRangeWithRetry(ctx context.Context, start, end int) (*appid.ISDBCache, error) {
	var lastErr error

	for attempt := 0; attempt < appid.ISDBMaxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Debug("Retry attempt %d for range %d-%d", attempt+1, start, end)
			time.Sleep(time.Second * time.Duration(attempt)) // Exponential backoff
		}

		cache, err := c.fetchISDBRange(ctx, start, end)
		if err == nil {
			return cache, nil
		}

		lastErr = err
		c.logger.Warn("Attempt %d failed for range %d-%d: %v", attempt+1, start, end, err)
	}

	return nil, fmt.Errorf("all %d retry attempts failed: %w", appid.ISDBMaxRetries, lastErr)
}

// fetchISDBRange fetches ISDB entries for a specific range
func (c *FortiGateISDBCollector) fetchISDBRange(ctx context.Context, start, end int) (*appid.ISDBCache, error) {
	cache := appid.NewISDBCache()

	// Query each ID in range individually
	// Note: FortiGate doesn't support range queries, must query one at a time
	for id := start; id <= end; id++ {
		// Check context
		if ctx.Err() != nil {
			return cache, ctx.Err()
		}

		output, err := c.executeCommand(ctx, fmt.Sprintf("diagnose internet-service id %d", id))
		if err != nil {
			return cache, fmt.Errorf("command failed for ID %d: %w", id, err)
		}

		// Parse output
		service := c.parseISDBEntry(id, output)
		if service != nil {
			cache.Services[service.ID] = service
			cache.ServicesByName[strings.ToLower(service.Name)] = service
		}

		// Small delay between commands
		time.Sleep(50 * time.Millisecond)
	}

	return cache, nil
}

// getISDBRange determines the maximum ISDB ID
func (c *FortiGateISDBCollector) getISDBRange(ctx context.Context) (int, error) {
	// Try to get ISDB database info
	output, err := c.executeCommand(ctx, "diagnose internet-service database info")
	if err != nil {
		// If command fails, use conservative default
		c.logger.Warn("Could not get ISDB range, using default of 10000")
		return 10000, nil
	}

	// Parse max ID from output
	// Format varies by version, look for patterns like "Total entries: 12345"
	re := regexp.MustCompile(`(?i)total.*?(\d+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		maxID, err := strconv.Atoi(matches[1])
		if err == nil && maxID > 0 {
			return maxID, nil
		}
	}

	// Default to 10000 if can't determine
	c.logger.Warn("Could not parse ISDB range from output, using default of 10000")
	return 10000, nil
}

// getFortiOSVersion gets the FortiOS version
func (c *FortiGateISDBCollector) getFortiOSVersion(ctx context.Context) (string, error) {
	output, err := c.executeCommand(ctx, "get system status")
	if err != nil {
		return "", err
	}

	// Parse version from output
	// Format: "Version: FortiGate-VM64 v7.2.3,build1234,..."
	re := regexp.MustCompile(`Version:.*?v?([\d.]+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "unknown", nil
}

// getISDBVersion gets the ISDB version
func (c *FortiGateISDBCollector) getISDBVersion(ctx context.Context) (string, error) {
	output, err := c.executeCommand(ctx, "diagnose internet-service database info")
	if err != nil {
		return "unknown", nil
	}

	// Parse ISDB version
	// Format varies, look for patterns like "Version: 12.00365" or "Database: 12.00365"
	re := regexp.MustCompile(`(?i)(?:version|database):\s*([\d.]+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "unknown", nil
}

// executeCommand executes an SSH command on the FortiGate
func (c *FortiGateISDBCollector) executeCommand(ctx context.Context, command string) (string, error) {
	// Apply command timeout
	ctx, cancel := context.WithTimeout(ctx, appid.ChunkTimeout)
	defer cancel()

	// Use SSH connector's ExecuteCommand method
	output, err := c.connector.ExecuteCommand(ctx, command)
	if err != nil {
		return "", err
	}

	return output, nil
}

// ========================================
// ISDB Parsing
// ========================================

// parseISDBEntry parses a single ISDB entry from CLI output
func (c *FortiGateISDBCollector) parseISDBEntry(id int, output string) *appid.ISDBServiceEntry {
	if strings.Contains(output, "not found") || strings.Contains(output, "invalid") {
		return nil // ID doesn't exist
	}

	entry := &appid.ISDBServiceEntry{
		ID:      fmt.Sprintf("%d", id),
		Content: output,
	}

	// Parse name
	// Format: "Name: Google" or "Service: Google"
	reName := regexp.MustCompile(`(?i)(?:name|service):\s*(.+)`)
	if matches := reName.FindStringSubmatch(output); len(matches) > 1 {
		entry.Name = strings.TrimSpace(matches[1])
	} else {
		entry.Name = fmt.Sprintf("Service_%d", id)
	}

	// Parse protocols
	// Format: "Protocol: TCP" or "Protocols: TCP, UDP"
	reProto := regexp.MustCompile(`(?i)protocols?:\s*(.+)`)
	if matches := reProto.FindStringSubmatch(output); len(matches) > 1 {
		protoStr := strings.TrimSpace(matches[1])
		// Split by comma and clean up
		for _, p := range strings.Split(protoStr, ",") {
			p = strings.TrimSpace(strings.ToUpper(p))
			if p != "" {
				entry.Protocols = append(entry.Protocols, p)
			}
		}
	}

	// Parse ports
	// Format: "Ports: 80,443" or "Port: 80-443"
	rePort := regexp.MustCompile(`(?i)ports?:\s*(.+)`)
	if matches := rePort.FindStringSubmatch(output); len(matches) > 1 {
		portStr := strings.TrimSpace(matches[1])
		// Split by comma
		for _, p := range strings.Split(portStr, ",") {
			p = strings.TrimSpace(p)
			if p != "" && p != "0" {
				entry.Ports = append(entry.Ports, p)
			}
		}
	}

	// Parse categories
	// Format: "Category: Search-Engines" or "Categories: Search-Engines, Social-Media"
	reCat := regexp.MustCompile(`(?i)categor(?:y|ies):\s*(.+)`)
	if matches := reCat.FindStringSubmatch(output); len(matches) > 1 {
		catStr := strings.TrimSpace(matches[1])
		for _, cat := range strings.Split(catStr, ",") {
			cat = strings.TrimSpace(cat)
			if cat != "" {
				entry.Categories = append(entry.Categories, cat)
			}
		}
	}

	// Store version and date from when this was collected
	entry.Version = "collected"
	entry.Date = time.Now().Format(time.RFC3339)

	return entry
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

// ValidateSSHAccess validates that the SSH connector can access FortiGate CLI
func (c *FortiGateISDBCollector) ValidateSSHAccess(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Try to get system status as a validation check
	_, err := c.getFortiOSVersion(ctx)
	if err != nil {
		return fmt.Errorf("SSH access validation failed: %w", err)
	}

	return nil
}

// EstimateCollectionTime estimates how long ISDB collection will take
func (c *FortiGateISDBCollector) EstimateCollectionTime(maxID int) time.Duration {
	// Each ID query takes ~100ms + 50ms delay = 150ms per entry
	// With chunking and retries, add 20% buffer
	estimatedSeconds := (maxID * 150 / 1000) * 120 / 100
	return time.Duration(estimatedSeconds) * time.Second
}
