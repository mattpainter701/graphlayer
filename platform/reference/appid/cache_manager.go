package appid

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

/*
CacheManager provides CRUD operations for App-ID and ISDB caches.

It handles:
- Loading caches from device-specific or baseline locations
- Saving caches with atomic writes
- Creating and managing backups
- Metadata persistence
- Compression/decompression of baseline files
*/

// CacheManager manages cache file operations
type CacheManager struct {
	cacheDir    string // Base directory for device caches
	baselineDir string // Directory for baseline caches
	logger      Logger // Logger for operations
}

// Logger interface for cache manager (allows dependency injection)
type Logger interface {
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
	Debug(format string, args ...interface{})
}

// defaultLogger provides basic logging to stdout
type defaultLogger struct{}

func (l *defaultLogger) Info(format string, args ...interface{})  { fmt.Printf("[INFO] "+format+"\n", args...) }
func (l *defaultLogger) Warn(format string, args ...interface{})  { fmt.Printf("[WARN] "+format+"\n", args...) }
func (l *defaultLogger) Error(format string, args ...interface{}) { fmt.Printf("[ERROR] "+format+"\n", args...) }
func (l *defaultLogger) Debug(format string, args ...interface{}) { fmt.Printf("[DEBUG] "+format+"\n", args...) }

// NewCacheManager creates a new cache manager
func NewCacheManager(cacheDir, baselineDir string, logger Logger) *CacheManager {
	if logger == nil {
		logger = &defaultLogger{}
	}
	return &CacheManager{
		cacheDir:    cacheDir,
		baselineDir: baselineDir,
		logger:      logger,
	}
}

// NewDefaultCacheManager creates a cache manager with default directories
func NewDefaultCacheManager() *CacheManager {
	return NewCacheManager(DefaultCacheDir, DefaultBaselineDir, nil)
}

// ========================================
// App-ID Cache Operations
// ========================================

// LoadAppIDCache loads an App-ID cache from device-specific location
func (m *CacheManager) LoadAppIDCache(deviceID string) (*AppIDCache, error) {
	cachePath := filepath.Join(m.cacheDir, deviceID, CurrentCacheFileName)
	return m.loadAppIDFromFile(cachePath)
}

// SaveAppIDCache saves an App-ID cache with atomic write
func (m *CacheManager) SaveAppIDCache(deviceID string, cache *AppIDCache) error {
	cachePath := filepath.Join(m.cacheDir, deviceID, CurrentCacheFileName)
	return m.saveAppIDToFileAtomic(cache, cachePath)
}

// LoadAppIDCacheWithMetadata loads cache and its metadata
func (m *CacheManager) LoadAppIDCacheWithMetadata(deviceID string) (*AppIDCache, *CacheMetadata, error) {
	cache, err := m.LoadAppIDCache(deviceID)
	if err != nil {
		return nil, nil, err
	}

	metadata, err := m.LoadMetadata(deviceID)
	if err != nil {
		// Metadata missing - create default
		m.logger.Warn("Metadata not found for %s, creating default", deviceID)
		metadata = &CacheMetadata{
			DeviceID:         deviceID,
			TotalApplications: len(cache.Applications),
			Health:           string(HealthStatusUnknown),
		}
	}

	return cache, metadata, nil
}

// ========================================
// ISDB Cache Operations
// ========================================

// LoadISDBCache loads an ISDB cache from device-specific location
func (m *CacheManager) LoadISDBCache(deviceID string) (*ISDBCache, error) {
	cachePath := filepath.Join(m.cacheDir, deviceID, CurrentCacheFileName)
	return m.loadISDBFromFile(cachePath)
}

// SaveISDBCache saves an ISDB cache with atomic write
func (m *CacheManager) SaveISDBCache(deviceID string, cache *ISDBCache) error {
	cachePath := filepath.Join(m.cacheDir, deviceID, CurrentCacheFileName)
	return m.saveISDBToFileAtomic(cache, cachePath)
}

// LoadISDBCacheWithMetadata loads cache and its metadata
func (m *CacheManager) LoadISDBCacheWithMetadata(deviceID string) (*ISDBCache, *CacheMetadata, error) {
	cache, err := m.LoadISDBCache(deviceID)
	if err != nil {
		return nil, nil, err
	}

	metadata, err := m.LoadMetadata(deviceID)
	if err != nil {
		m.logger.Warn("Metadata not found for %s, creating default", deviceID)
		metadata = &CacheMetadata{
			DeviceID:      deviceID,
			TotalServices: len(cache.Services),
			Health:        string(HealthStatusUnknown),
		}
	}

	return cache, metadata, nil
}

// ========================================
// Metadata Operations
// ========================================

// LoadMetadata loads cache metadata
func (m *CacheManager) LoadMetadata(deviceID string) (*CacheMetadata, error) {
	metaPath := filepath.Join(m.cacheDir, deviceID, MetadataFileName)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata CacheMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &metadata, nil
}

// SaveMetadata saves cache metadata
func (m *CacheManager) SaveMetadata(deviceID string, metadata *CacheMetadata) error {
	metaPath := filepath.Join(m.cacheDir, deviceID, MetadataFileName)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(metaPath), 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Atomic write
	return m.atomicWrite(metaPath, data)
}

// ========================================
// Backup Operations
// ========================================

// BackupAppIDCache creates a timestamped backup of the current cache
func (m *CacheManager) BackupAppIDCache(deviceID string) error {
	currentPath := filepath.Join(m.cacheDir, deviceID, CurrentCacheFileName)

	// Check if current cache exists
	if _, err := os.Stat(currentPath); os.IsNotExist(err) {
		m.logger.Debug("No current cache to backup for %s", deviceID)
		return nil
	}

	// Create backup directory
	backupDir := filepath.Join(m.cacheDir, deviceID, BackupsDir)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Generate timestamped backup filename
	timestamp := time.Now().Format("2006-01-02_150405")
	backupPath := filepath.Join(backupDir, timestamp+".json")

	// Copy current cache to backup
	if err := m.copyFile(currentPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	m.logger.Info("Created backup for %s: %s", deviceID, backupPath)

	// Cleanup old backups
	return m.cleanupBackups(deviceID)
}

// RestoreBackup restores a specific backup (by timestamp) or latest if empty
func (m *CacheManager) RestoreBackup(deviceID string, timestamp string) error {
	backupDir := filepath.Join(m.cacheDir, deviceID, BackupsDir)

	var backupPath string
	if timestamp == "" {
		// Get latest backup
		latest, err := m.getLatestBackup(deviceID)
		if err != nil {
			return fmt.Errorf("failed to find latest backup: %w", err)
		}
		backupPath = latest
	} else {
		backupPath = filepath.Join(backupDir, timestamp+".json")
	}

	// Check backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", backupPath)
	}

	// Copy backup to current
	currentPath := filepath.Join(m.cacheDir, deviceID, CurrentCacheFileName)
	if err := m.copyFile(backupPath, currentPath); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	m.logger.Info("Restored backup for %s from %s", deviceID, backupPath)
	return nil
}

// ListBackups returns a list of available backups for a device
func (m *CacheManager) ListBackups(deviceID string) ([]string, error) {
	backupDir := filepath.Join(m.cacheDir, deviceID, BackupsDir)

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // No backups yet
		}
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			backups = append(backups, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}

	// Sort descending (newest first)
	sort.Sort(sort.Reverse(sort.StringSlice(backups)))
	return backups, nil
}

// cleanupBackups removes old backups, keeping only MaxBackups most recent
func (m *CacheManager) cleanupBackups(deviceID string) error {
	backupDir := filepath.Join(m.cacheDir, deviceID, BackupsDir)

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	// Get list of backup files
	var backups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			backups = append(backups, entry.Name())
		}
	}

	// If we have more than MaxBackups, delete oldest
	if len(backups) > MaxBackups {
		// Sort ascending (oldest first)
		sort.Strings(backups)

		// Delete oldest files
		toDelete := len(backups) - MaxBackups
		for i := 0; i < toDelete; i++ {
			oldPath := filepath.Join(backupDir, backups[i])
			if err := os.Remove(oldPath); err != nil {
				m.logger.Warn("Failed to delete old backup %s: %v", oldPath, err)
			} else {
				m.logger.Debug("Deleted old backup: %s", backups[i])
			}
		}
	}

	return nil
}

// getLatestBackup returns the path to the most recent backup
func (m *CacheManager) getLatestBackup(deviceID string) (string, error) {
	backups, err := m.ListBackups(deviceID)
	if err != nil {
		return "", err
	}
	if len(backups) == 0 {
		return "", fmt.Errorf("no backups available")
	}

	backupDir := filepath.Join(m.cacheDir, deviceID, BackupsDir)
	return filepath.Join(backupDir, backups[0]+".json"), nil
}

// ========================================
// Baseline Operations
// ========================================

// LoadBaseline loads a compressed baseline cache
func (m *CacheManager) LoadBaseline(relativePath string, isISDB bool) (interface{}, error) {
	baselinePath := filepath.Join(m.baselineDir, relativePath)

	// Try .json.gz first (compressed)
	if strings.HasSuffix(baselinePath, ".json") {
		gzPath := baselinePath + ".gz"
		if _, err := os.Stat(gzPath); err == nil {
			return m.loadCompressedBaseline(gzPath, isISDB)
		}
	}

	// Try uncompressed .json
	if _, err := os.Stat(baselinePath); err == nil {
		if isISDB {
			return m.loadISDBFromFile(baselinePath)
		}
		return m.loadAppIDFromFile(baselinePath)
	}

	return nil, fmt.Errorf("baseline not found: %s", baselinePath)
}

// loadCompressedBaseline loads a gzipped baseline file
func (m *CacheManager) loadCompressedBaseline(gzPath string, isISDB bool) (interface{}, error) {
	file, err := os.Open(gzPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open compressed baseline: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	data, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read compressed baseline: %w", err)
	}

	if isISDB {
		var cache ISDBCache
		if err := json.Unmarshal(data, &cache); err != nil {
			return nil, fmt.Errorf("failed to parse ISDB baseline: %w", err)
		}
		return &cache, nil
	}

	var cache AppIDCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse App-ID baseline: %w", err)
	}
	return &cache, nil
}

// ========================================
// Internal Helper Methods
// ========================================

// loadAppIDFromFile loads an App-ID cache from a JSON file
func (m *CacheManager) loadAppIDFromFile(path string) (*AppIDCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache AppIDCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse cache JSON: %w", err)
	}

	return &cache, nil
}

// loadISDBFromFile loads an ISDB cache from a JSON file
func (m *CacheManager) loadISDBFromFile(path string) (*ISDBCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	// Try new format first (ISDBCache struct)
	var cache ISDBCache
	if err := json.Unmarshal(data, &cache); err == nil {
		return &cache, nil
	}

	// Try old format (ISDBFileFormat from fortinet_isdb.go)
	var fileFormat ISDBFileFormat
	if err := json.Unmarshal(data, &fileFormat); err != nil {
		return nil, fmt.Errorf("failed to parse cache JSON: %w", err)
	}

	// Convert old format to new
	cache = ISDBCache{
		Services:       make(map[string]*ISDBServiceEntry),
		ServicesByName: make(map[string]*ISDBServiceEntry),
		Version:        fileFormat.Version,
		Date:           fileFormat.Date,
		Server:         fileFormat.Server,
	}

	// Parse each ISDB entry
	for id, entryData := range fileFormat.ISDB {
		if name, ok := entryData["name"]; ok {
			content := entryData["content"]
			entry := parseISDBContent(id, name, content)
			cache.Services[id] = entry
			cache.ServicesByName[strings.ToLower(name)] = entry
		}
	}

	return &cache, nil
}

// saveAppIDToFileAtomic saves an App-ID cache with atomic write
func (m *CacheManager) saveAppIDToFileAtomic(cache *AppIDCache, path string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	return m.atomicWrite(path, data)
}

// saveISDBToFileAtomic saves an ISDB cache with atomic write
func (m *CacheManager) saveISDBToFileAtomic(cache *ISDBCache, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	return m.atomicWrite(path, data)
}

// atomicWrite writes data to a file atomically using temp file + rename
func (m *CacheManager) atomicWrite(path string, data []byte) error {
	// Write to temporary file first
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Rename to final path (atomic operation)
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Cleanup temp file on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst
func (m *CacheManager) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// parseISDBContent parses ISDB content string (helper for old format)
func parseISDBContent(id, name, content string) *ISDBServiceEntry {
	// This is a simplified version - full implementation in fortinet_isdb.go
	entry := &ISDBServiceEntry{
		ID:      id,
		Name:    name,
		Content: content,
	}

	// Parse content for protocols, ports, categories
	// (Regex parsing logic from fortinet_isdb.go would go here)

	return entry
}
