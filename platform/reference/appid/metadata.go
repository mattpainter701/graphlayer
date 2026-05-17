package appid

import "time"

/*
Metadata structures for tracking cache state, fetch history, and health.

These structures are persisted alongside cache files to enable:
- Health assessment
- Merge decision-making
- Audit trails
- Staleness detection
*/

// CacheMetadata tracks the state and history of a cache
type CacheMetadata struct {
	// Device identification
	DeviceID string `json:"device_id"`

	// Firmware/content versions
	FirmwareVersion string `json:"firmware_version"` // e.g., "10.1.3" for PAN-OS, "7.2.3" for FortiOS
	ContentVersion  string `json:"content_version"`  // e.g., "8595-8012" for App-ID

	// Timestamps
	CreatedAt       string `json:"created_at"`        // When cache was first created
	LastFullFetch   string `json:"last_full_fetch"`   // Last successful full fetch
	LastPartialFetch string `json:"last_partial_fetch"` // Last partial/incomplete fetch
	LastUpdated     string `json:"last_updated"`      // Last modification (full or partial)

	// Size metrics
	TotalApplications int `json:"total_applications,omitempty"` // For App-ID caches
	TotalServices     int `json:"total_services,omitempty"`     // For ISDB caches

	// Health and staleness
	Health        string `json:"health"`         // Current health status
	StalnessDays  int    `json:"staleness_days"` // Days since last update

	// Fetch history (last N fetches)
	FetchHistory []FetchRecord `json:"fetch_history"`

	// Source information
	Source string `json:"source"` // "panos-xml-api", "fortigate-ssh-cli", "baseline", etc.

	// Additional metadata
	Metadata map[string]string `json:"metadata,omitempty"` // Custom key-value pairs
}

// FetchRecord represents a single fetch attempt
type FetchRecord struct {
	// Timing
	Timestamp string `json:"timestamp"` // ISO 8601 format
	Duration  int64  `json:"duration"`  // Duration in milliseconds

	// Status
	Status FetchStatus `json:"status"` // "full_success", "partial_success", etc.

	// Metrics
	EntriesFetched int `json:"entries_fetched"` // Number of entries fetched this attempt
	EntriesAdded   int `json:"entries_added"`   // Number of new entries added
	EntriesUpdated int `json:"entries_updated"` // Number of existing entries updated
	EntriesRemoved int `json:"entries_removed"` // Number of entries removed (full fetch only)
	EntriesUnchanged int `json:"entries_unchanged"` // Number of entries that didn't change

	// Error information
	Error        string `json:"error,omitempty"`         // Error message if failed
	ErrorContext string `json:"error_context,omitempty"` // Additional context about error

	// Source
	Source string `json:"source"` // Where data came from

	// Completeness (for partial fetches)
	ExpectedTotal int     `json:"expected_total,omitempty"` // Expected number of entries
	Completeness  float64 `json:"completeness,omitempty"`   // Percentage complete (0.0-1.0)
}

// MergeReport describes the outcome of a merge operation
type MergeReport struct {
	// Timestamp
	Timestamp string `json:"timestamp"`

	// Merge type
	FullReplace bool `json:"full_replace"` // true if existing cache was empty

	// Change counts
	Added       []string `json:"added"`        // Names of added entries
	Updated     []string `json:"updated"`      // Names of updated entries
	Removed     []string `json:"removed"`      // Names of removed entries
	Unchanged   int      `json:"unchanged"`    // Count of unchanged entries

	// Conflicts
	Conflicts []ConflictRecord `json:"conflicts,omitempty"` // Conflicts encountered

	// Metadata
	OldTotalCount int    `json:"old_total_count"` // Count before merge
	NewTotalCount int    `json:"new_total_count"` // Count after merge
	Source        string `json:"source"`          // Source of new data

	// Warnings
	Warnings []string `json:"warnings,omitempty"` // Non-fatal issues during merge
}

// ConflictRecord represents a conflict during merge
type ConflictRecord struct {
	EntryName    string `json:"entry_name"`    // Name of conflicting entry
	ConflictType string `json:"conflict_type"` // "data_mismatch", "version_conflict", etc.
	OldValue     string `json:"old_value"`     // Old value (abbreviated)
	NewValue     string `json:"new_value"`     // New value (abbreviated)
	Resolution   string `json:"resolution"`    // How conflict was resolved
}

// CollectionResult represents the outcome of a collection operation
type CollectionResult struct {
	// Status
	Status FetchStatus `json:"status"`

	// Timing
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  int64     `json:"duration_ms"` // Duration in milliseconds

	// Collection metrics
	AppsCollected     int `json:"apps_collected,omitempty"`     // For App-ID
	ServicesCollected int `json:"services_collected,omitempty"` // For ISDB

	// Cache data (populated on success)
	AppIDCache *AppIDCache `json:"-"` // Don't serialize (too large)
	ISDBCache  *ISDBCache  `json:"-"` // Don't serialize (too large)

	// Errors
	Errors []error `json:"-"` // Runtime errors
	ErrorMessages []string `json:"errors,omitempty"` // Serializable error messages

	// Metadata for storage
	Metadata FetchMetadata `json:"metadata"`
}

// FetchMetadata contains metadata about a fetch operation
type FetchMetadata struct {
	DeviceID        string      `json:"device_id"`
	DeviceType      string      `json:"device_type"` // "panos", "fortios", etc.
	FirmwareVersion string      `json:"firmware_version"`
	ContentVersion  string      `json:"content_version,omitempty"`
	Source          string      `json:"source"` // "panos-xml-api", "fortigate-ssh-cli"
	Status          FetchStatus `json:"status"`
	Timestamp       string      `json:"timestamp"`

	// Expected vs. actual
	ExpectedTotal int     `json:"expected_total,omitempty"`
	ActualTotal   int     `json:"actual_total"`
	Completeness  float64 `json:"completeness"`
}

// ========================================
// Helper Functions
// ========================================

// NewCacheMetadata creates a new metadata structure
func NewCacheMetadata(deviceID, firmwareVersion string) *CacheMetadata {
	now := time.Now().Format(time.RFC3339)
	return &CacheMetadata{
		DeviceID:        deviceID,
		FirmwareVersion: firmwareVersion,
		CreatedAt:       now,
		LastUpdated:     now,
		Health:          string(HealthStatusUnknown),
		FetchHistory:    []FetchRecord{},
		Metadata:        make(map[string]string),
	}
}

// AddFetchRecord adds a fetch record to the metadata
func (m *CacheMetadata) AddFetchRecord(record FetchRecord) {
	m.FetchHistory = append(m.FetchHistory, record)

	// Keep only last 10 fetch records to prevent unbounded growth
	if len(m.FetchHistory) > 10 {
		m.FetchHistory = m.FetchHistory[len(m.FetchHistory)-10:]
	}

	// Update timestamps based on fetch status
	if record.Status == FetchStatusFullSuccess {
		m.LastFullFetch = record.Timestamp
	} else if record.Status == FetchStatusPartialSuccess ||
	          record.Status == FetchStatusPartialIncomplete {
		m.LastPartialFetch = record.Timestamp
	}

	m.LastUpdated = record.Timestamp
}

// UpdateStaleness calculates and updates the staleness in days
func (m *CacheMetadata) UpdateStaleness() {
	if m.LastUpdated == "" {
		m.StalnessDays = 0
		return
	}

	lastUpdate, err := time.Parse(time.RFC3339, m.LastUpdated)
	if err != nil {
		// If parse fails, mark as very stale
		m.StalnessDays = 999
		return
	}

	m.StalnessDays = int(time.Since(lastUpdate).Hours() / 24)
}

// GetLastFetchStatus returns the status of the most recent fetch
func (m *CacheMetadata) GetLastFetchStatus() FetchStatus {
	if len(m.FetchHistory) == 0 {
		return FetchStatusUnknown
	}
	return m.FetchHistory[len(m.FetchHistory)-1].Status
}

// GetRecentErrors returns errors from the last N fetches
func (m *CacheMetadata) GetRecentErrors(n int) []string {
	if n > len(m.FetchHistory) {
		n = len(m.FetchHistory)
	}

	var errors []string
	for i := len(m.FetchHistory) - n; i < len(m.FetchHistory); i++ {
		if m.FetchHistory[i].Error != "" {
			errors = append(errors, m.FetchHistory[i].Error)
		}
	}

	return errors
}

// CountRecentErrors counts errors in the last N fetches
func (m *CacheMetadata) CountRecentErrors(n int) int {
	if n > len(m.FetchHistory) {
		n = len(m.FetchHistory)
	}

	errorCount := 0
	for i := len(m.FetchHistory) - n; i < len(m.FetchHistory); i++ {
		if m.FetchHistory[i].Status == FetchStatusFailed ||
		   m.FetchHistory[i].Error != "" {
			errorCount++
		}
	}

	return errorCount
}

// NewMergeReport creates a new merge report
func NewMergeReport(source string) *MergeReport {
	return &MergeReport{
		Timestamp: time.Now().Format(time.RFC3339),
		Source:    source,
		Added:     []string{},
		Updated:   []string{},
		Removed:   []string{},
		Conflicts: []ConflictRecord{},
		Warnings:  []string{},
	}
}

// AddConflict adds a conflict record to the merge report
func (r *MergeReport) AddConflict(entryName, conflictType, oldVal, newVal, resolution string) {
	r.Conflicts = append(r.Conflicts, ConflictRecord{
		EntryName:    entryName,
		ConflictType: conflictType,
		OldValue:     oldVal,
		NewValue:     newVal,
		Resolution:   resolution,
	})
}

// AddWarning adds a warning to the merge report
func (r *MergeReport) AddWarning(warning string) {
	r.Warnings = append(r.Warnings, warning)
}

// Summary returns a human-readable summary of the merge
func (r *MergeReport) Summary() string {
	if r.FullReplace {
		return fmt.Sprintf("Full replace: %d entries added", len(r.Added))
	}

	return fmt.Sprintf("Merge: +%d added, ~%d updated, -%d removed, =%d unchanged",
		len(r.Added), len(r.Updated), len(r.Removed), r.Unchanged)
}

// NewCollectionResult creates a new collection result
func NewCollectionResult(deviceID, deviceType string) *CollectionResult {
	return &CollectionResult{
		Status:        FetchStatusUnknown,
		StartTime:     time.Now(),
		Errors:        []error{},
		ErrorMessages: []string{},
		Metadata: FetchMetadata{
			DeviceID:   deviceID,
			DeviceType: deviceType,
			Timestamp:  time.Now().Format(time.RFC3339),
		},
	}
}

// Complete marks the collection as complete and calculates duration
func (r *CollectionResult) Complete(status FetchStatus) {
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime).Milliseconds()
	r.Status = status
	r.Metadata.Status = status

	// Populate error messages from errors
	for _, err := range r.Errors {
		r.ErrorMessages = append(r.ErrorMessages, err.Error())
	}
}

// AddError adds an error to the collection result
func (r *CollectionResult) AddError(err error) {
	r.Errors = append(r.Errors, err)
}

// IsSuccess returns true if collection was successful (full or partial)
func (r *CollectionResult) IsSuccess() bool {
	return r.Status == FetchStatusFullSuccess ||
	       r.Status == FetchStatusPartialSuccess
}

// ToFetchRecord converts collection result to a fetch record
func (r *CollectionResult) ToFetchRecord() FetchRecord {
	record := FetchRecord{
		Timestamp:      r.Metadata.Timestamp,
		Duration:       r.Duration,
		Status:         r.Status,
		Source:         r.Metadata.Source,
		ExpectedTotal:  r.Metadata.ExpectedTotal,
		Completeness:   r.Metadata.Completeness,
	}

	if r.AppIDCache != nil {
		record.EntriesFetched = len(r.AppIDCache.Applications)
	} else if r.ISDBCache != nil {
		record.EntriesFetched = len(r.ISDBCache.Services)
	}

	if len(r.ErrorMessages) > 0 {
		record.Error = r.ErrorMessages[0] // First error
		if len(r.ErrorMessages) > 1 {
			record.ErrorContext = fmt.Sprintf("%d total errors", len(r.ErrorMessages))
		}
	}

	return record
}

// Compile-time check to ensure we import fmt was here; removed stray import.
