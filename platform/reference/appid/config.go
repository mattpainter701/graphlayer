package appid

import "time"

/*
Configuration Constants for App-ID/ISDB Collection

These constants control the behavior of App-ID and ISDB cache collection,
health assessment, and management. They are intentionally set as constants
for easy modification and tuning.
*/

const (
	// ========================================
	// Health Assessment Thresholds
	// ========================================

	// MinHealthyAppCount is the minimum number of applications required
	// for a PAN-OS App-ID cache to be considered "healthy"
	// Set conservatively low to avoid false negatives
	MinHealthyAppCount = 500

	// MinHealthyServiceCount is the minimum number of services required
	// for a FortiGate ISDB cache to be considered "healthy"
	// Set conservatively low to avoid false negatives
	MinHealthyServiceCount = 100

	// MinCompletionPercentage is the minimum percentage of expected entries
	// that must be present for a cache to be considered complete
	// 80% threshold allows for some variance while catching incomplete fetches
	MinCompletionPercentage = 0.80 // 80%

	// ========================================
	// Staleness Configuration
	// ========================================

	// StaleDays defines how many days old a cache can be before it's
	// considered "stale". Stale caches are still usable but trigger warnings.
	// App-ID/ISDB don't change frequently, so 90 days is reasonable.
	StaleDays = 90

	// CriticallyStaleDays defines when a cache is critically outdated
	// and should trigger stronger warnings or force refresh
	CriticallyStaleDays = 180

	// ========================================
	// Backup Configuration
	// ========================================

	// MaxBackups is the maximum number of backup files to retain
	// Keeping 2 backups allows rollback to previous version if needed
	// while limiting storage usage
	MaxBackups = 2

	// ========================================
	// Collection Timeouts
	// ========================================

	// CollectionTimeout is the maximum time allowed for a complete
	// App-ID or ISDB collection operation
	CollectionTimeout = 10 * time.Minute

	// ChunkTimeout is the maximum time allowed for collecting a single
	// chunk/batch of data (used for FortiGate ISDB chunked queries)
	ChunkTimeout = 2 * time.Minute

	// APIRequestTimeout is the timeout for individual API requests
	APIRequestTimeout = 30 * time.Second

	// ========================================
	// Chunking Configuration (FortiGate ISDB)
	// ========================================

	// ISDBChunkSize is the number of ISDB entries to query per CLI command
	// FortiGate ISDB can have 10000+ entries, so we chunk to avoid timeouts
	ISDBChunkSize = 1000

	// ISDBMaxRetries is the maximum number of retry attempts for failed chunks
	ISDBMaxRetries = 3

	// ========================================
	// Rate Limiting
	// ========================================

	// APIRateLimit is the maximum number of API requests per second
	// to avoid overwhelming the device
	APIRateLimit = 10

	// SSHCommandDelay is the delay between SSH commands to avoid
	// overwhelming the CLI
	SSHCommandDelay = 100 * time.Millisecond

	// ========================================
	// File Paths
	// ========================================

	// DefaultCacheDir is the default directory for device-specific caches
	DefaultCacheDir = "cache/appid"

	// DefaultBaselineDir is the default directory for baseline caches
	DefaultBaselineDir = "cache/appid/baselines"

	// DefaultISDBCacheDir is the default directory for ISDB caches
	DefaultISDBCacheDir = "cache/isdb"

	// DefaultISDBBaselineDir is the default directory for ISDB baselines
	DefaultISDBBaselineDir = "cache/isdb/baselines"

	// ========================================
	// File Names
	// ========================================

	// CurrentCacheFileName is the filename for the active cache
	CurrentCacheFileName = "current.json"

	// MetadataFileName is the filename for cache metadata
	MetadataFileName = "current.json.meta"

	// FetchLogFileName is the filename for fetch attempt history
	FetchLogFileName = "fetch_log.json"

	// BackupsDir is the subdirectory name for backups
	BackupsDir = "backups"

	// BaselineIndexFileName is the filename for the baseline version index
	BaselineIndexFileName = "baseline_index.json"
)

// HealthStatus represents the health state of a cache
type HealthStatus string

const (
	HealthStatusHealthy         HealthStatus = "healthy"          // Cache is fresh and complete
	HealthStatusStale           HealthStatus = "stale"            // Cache is old but usable
	HealthStatusCriticallyStale HealthStatus = "critically_stale" // Cache is very old
	HealthStatusIncomplete      HealthStatus = "incomplete"       // Cache has incomplete data
	HealthStatusUnhealthy       HealthStatus = "unhealthy"        // Cache has errors or corruption
	HealthStatusEmpty           HealthStatus = "empty"            // No cache exists
	HealthStatusUnknown         HealthStatus = "unknown"          // Health has not been assessed
)

// FetchStatus represents the outcome of a collection attempt
type FetchStatus string

const (
	FetchStatusFullSuccess       FetchStatus = "full_success"       // Complete fetch with no errors
	FetchStatusPartialSuccess    FetchStatus = "partial_success"    // Partial fetch due to timeout
	FetchStatusPartialIncomplete FetchStatus = "partial_incomplete" // Incomplete data returned
	FetchStatusPartialTimeout    FetchStatus = "partial_timeout"    // Timed out mid-fetch
	FetchStatusFailed            FetchStatus = "failed"             // Complete failure
	FetchStatusUnknown           FetchStatus = "unknown"            // Status not determined
)

// CollectionMode represents how data is collected
type CollectionMode string

const (
	CollectionModeFull    CollectionMode = "full"    // Complete refresh
	CollectionModePartial CollectionMode = "partial" // Incremental update
	CollectionModeDelta   CollectionMode = "delta"   // Only changes since last fetch
)

// LoadSource represents where cache data was loaded from
type LoadSource string

const (
	LoadSourceDeviceCache       LoadSource = "device_cache"       // Device-specific cache
	LoadSourceVersionedBaseline LoadSource = "versioned_baseline" // Version-matched baseline
	LoadSourceLatestBaseline    LoadSource = "latest_baseline"    // Latest available baseline
	LoadSourceEmbeddedSample    LoadSource = "embedded_sample"    // Built-in sample data
	LoadSourceUnknown           LoadSource = "unknown"            // Source not determined
)

// GetCacheDirectory returns the cache directory for a given device type
func GetCacheDirectory(deviceType string) string {
	switch deviceType {
	case "panos", "panorama":
		return DefaultCacheDir
	case "fortios", "fortigate":
		return DefaultISDBCacheDir
	default:
		return DefaultCacheDir
	}
}

// GetBaselineDirectory returns the baseline directory for a given device type
func GetBaselineDirectory(deviceType string) string {
	switch deviceType {
	case "panos", "panorama":
		return DefaultBaselineDir
	case "fortios", "fortigate":
		return DefaultISDBBaselineDir
	default:
		return DefaultBaselineDir
	}
}
