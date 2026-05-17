package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/orchestration/types"
)

// ========================================
// Retry Tests
// ========================================

func TestRetrySettings_BackoffDuration(t *testing.T) {
	rs := DefaultRetrySettings()
	rs.Jitter = false // Disable jitter for deterministic testing

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 5 * time.Second},
		{1, 5 * time.Second},   // attempt 1: 5s * 2^0 = 5s
		{2, 10 * time.Second},  // attempt 2: 5s * 2^1 = 10s
		{3, 20 * time.Second},  // attempt 3: 5s * 2^2 = 20s
		{4, 40 * time.Second},  // attempt 4: 5s * 2^3 = 40s
	}

	for _, tt := range tests {
		got := rs.backoffDuration(tt.attempt)
		if got != tt.expected {
			t.Errorf("backoffDuration(%d) = %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

func TestRetrySettings_BackoffCapped(t *testing.T) {
	rs := DefaultRetrySettings()
	rs.Jitter = false
	rs.MaxBackoff = 30 * time.Second

	// At attempt 20: 5s * 2^19 would be huge, but should be capped at 30s
	got := rs.backoffDuration(20)
	if got != 30*time.Second {
		t.Errorf("backoff should be capped at 30s, got %v", got)
	}
}

func TestRetrySettings_IsRetryable(t *testing.T) {
	rs := &RetrySettings{
		MaxRetries: 3,
		RetryOn:    []RetryableError{RetryableConnectionError, RetryableTimeout},
	}

	tests := []struct {
		name      string
		errStr    string
		expected  bool
	}{
		{"connection refused", "connection refused", true},
		{"timeout", "context deadline exceeded", true},
		{"dial tcp timeout", "dial tcp: i/o timeout", true},
		{"permission denied", "permission denied", false},
		{"invalid syntax", "invalid syntax error", false},
	}

	for _, tt := range tests {
		err := &testError{msg: tt.errStr}
		got := rs.isRetryable(err, nil)
		if got != tt.expected {
			t.Errorf("isRetryable(%q) = %v, want %v", tt.name, got, tt.expected)
		}
	}
}

func TestRetrySettings_RetryAll(t *testing.T) {
	rs := &RetrySettings{
		MaxRetries: 3,
		RetryOn:    []RetryableError{RetryableAll},
	}

	// Everything should be retryable with RetryableAll
	err := &testError{msg: "anything at all"}
	if !rs.isRetryable(err, nil) {
		t.Error("RetryableAll should match any error")
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestNoRetry(t *testing.T) {
	rs := NoRetry()
	if rs.MaxRetries != 0 {
		t.Error("NoRetry should have MaxRetries=0")
	}
}

func TestAPIQuickRetry(t *testing.T) {
	rs := APIQuickRetry()
	if rs.MaxRetries != 5 {
		t.Errorf("APIQuickRetry MaxRetries = %d, want 5", rs.MaxRetries)
	}
	if rs.InitialBackoff != 1*time.Second {
		t.Errorf("APIQuickRetry InitialBackoff = %v, want 1s", rs.InitialBackoff)
	}
}

func TestSSHConservativeRetry(t *testing.T) {
	rs := SSHConservativeRetry()
	if rs.MaxRetries != 3 {
		t.Errorf("SSHConservativeRetry MaxRetries = %d, want 3", rs.MaxRetries)
	}
	if rs.InitialBackoff != 10*time.Second {
		t.Errorf("SSHConservativeRetry InitialBackoff = %v, want 10s", rs.InitialBackoff)
	}
}

// ========================================
// Health Monitor Tests
// ========================================

func TestHealthMonitor_RecordJobResult(t *testing.T) {
	hm := NewHealthMonitor()

	successResult := &JobRunResult{
		Job: &ConnectorJob{
			GUID:       "test-guid-1",
			DeviceType: types.DeviceTypeCiscoIOS,
			Config:     &base.ConnectionConfig{Host: "10.0.0.1"},
		},
		Success:  true,
		Duration: 5 * time.Second,
	}

	failResult := &JobRunResult{
		Job: &ConnectorJob{
			GUID:       "test-guid-1",
			DeviceType: types.DeviceTypeCiscoIOS,
			Config:     &base.ConnectionConfig{Host: "10.0.0.1"},
		},
		Success:  false,
		Error:    "connection refused",
		Duration: 2 * time.Second,
	}

	hm.RecordJobResult(successResult)
	hm.RecordJobResult(failResult)

	health := hm.GetConnectorHealth("test-guid-1")
	if health == nil {
		t.Fatal("expected health record for test-guid-1")
	}

	if health.TotalRuns != 2 {
		t.Errorf("TotalRuns = %d, want 2", health.TotalRuns)
	}
	if health.SuccessfulRuns != 1 {
		t.Errorf("SuccessfulRuns = %d, want 1", health.SuccessfulRuns)
	}
	if health.FailedRuns != 1 {
		t.Errorf("FailedRuns = %d, want 1", health.FailedRuns)
	}
	if health.SuccessRate != 0.5 {
		t.Errorf("SuccessRate = %f, want 0.5", health.SuccessRate)
	}
}

func TestHealthMonitor_GenerateReport(t *testing.T) {
	hm := NewHealthMonitor()

	// Add 3 healthy connectors, 1 degraded, 1 failed
	for i := 0; i < 3; i++ {
		for j := 0; j < 10; j++ {
			hm.RecordJobResult(&JobRunResult{
				Job:      &ConnectorJob{GUID: "healthy-" + string(rune('a'+i)), Config: &base.ConnectionConfig{Host: "10.0.0.1"}},
				Success:  true,
				Duration: time.Second,
			})
		}
	}

	for j := 0; j < 10; j++ {
		success := j >= 3 // 7 failures, 3 successes = 0.3 rate
		hm.RecordJobResult(&JobRunResult{
			Job:      &ConnectorJob{GUID: "degraded", Config: &base.ConnectionConfig{Host: "10.0.0.2"}},
			Success:  success,
			Duration: time.Second,
		})
	}

	for j := 0; j < 10; j++ {
		hm.RecordJobResult(&JobRunResult{
			Job:      &ConnectorJob{GUID: "failed", Config: &base.ConnectionConfig{Host: "10.0.0.3"}},
			Success:  false,
			Error:    "device unreachable",
			Duration: time.Second,
		})
	}

	report := hm.GenerateReport()
	if report.TotalConnectors != 5 {
		t.Errorf("TotalConnectors = %d, want 5", report.TotalConnectors)
	}
	if report.HealthyCount != 3 {
		t.Errorf("HealthyCount = %d, want 3", report.HealthyCount)
	}
	if report.DegradedCount != 1 {
		t.Errorf("DegradedCount = %d, want 1", report.DegradedCount)
	}
	if report.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", report.FailedCount)
	}
}

// ========================================
// Pipeline Visualization Tests
// ========================================

func TestPipelineRunLog_Visualize(t *testing.T) {
	now := time.Now()
	log := &PipelineRunLog{
		RunID:       "test-run-001",
		StartedAt:   now,
		CompletedAt: now.Add(30 * time.Second),
		Duration:    30 * time.Second,
		TotalJobs:   3,
		Successful:  2,
		Failed:      1,
		JobEntries: []*PipelineJobEntry{
			{JobGUID: "j1", DeviceType: "cisco_ios", DeviceID: "10.0.0.1", Status: PipelineStatusSuccess, StartedAt: now, EndedAt: now.Add(5 * time.Second), Duration: 5 * time.Second, Attempts: 1},
			{JobGUID: "j2", DeviceType: "palo_alto", DeviceID: "10.0.0.2", Status: PipelineStatusFailed, StartedAt: now, EndedAt: now.Add(10 * time.Second), Duration: 10 * time.Second, Attempts: 3, Error: "connection refused"},
			{JobGUID: "j3", DeviceType: "fortinet", DeviceID: "10.0.0.3", Status: PipelineStatusSkipped, StartedAt: now, EndedAt: now, Duration: 0, Skipped: true, SkipReason: "no changes since last collection"},
		},
	}

	output := log.Visualize()
	if output == "" {
		t.Error("Visualize() returned empty string")
	}

	// Verify key strings are present
	checks := []string{
		"test-run-001",
		"10.0.0.1",
		"10.0.0.2",
		"10.0.0.3",
		"success",
		"failed",
		"skipped",
	}
	for _, check := range checks {
		if !containsStr(output, check) {
			t.Errorf("Visualize() output missing %q", check)
		}
	}
}

func TestPipelineRunLog_VisualizeJSON(t *testing.T) {
	log := &PipelineRunLog{
		RunID:      "test-run-json",
		TotalJobs:  1,
		Successful: 1,
		JobEntries: []*PipelineJobEntry{
			{JobGUID: "j1", DeviceType: "cisco_ios", DeviceID: "10.0.0.1", Status: PipelineStatusSuccess},
		},
	}

	jsonStr, err := log.VisualizeJSON()
	if err != nil {
		t.Fatalf("VisualizeJSON() error: %v", err)
	}
	if !containsStr(jsonStr, "test-run-json") {
		t.Error("JSON output missing run ID")
	}
}

func TestPipelineRunLog_Filters(t *testing.T) {
	log := &PipelineRunLog{
		JobEntries: []*PipelineJobEntry{
			{JobGUID: "j1", Status: PipelineStatusSuccess},
			{JobGUID: "j2", Status: PipelineStatusFailed, Error: "timeout"},
			{JobGUID: "j3", Status: PipelineStatusSkipped},
			{JobGUID: "j4", Status: PipelineStatusSuccess},
		},
	}

	failed := log.FailedJobs()
	if len(failed) != 1 || failed[0].JobGUID != "j2" {
		t.Errorf("FailedJobs() = %d, want 1 (j2)", len(failed))
	}

	successful := log.SuccessfulJobs()
	if len(successful) != 2 {
		t.Errorf("SuccessfulJobs() = %d, want 2", len(successful))
	}

	skipped := log.SkippedJobs()
	if len(skipped) != 1 || skipped[0].JobGUID != "j3" {
		t.Errorf("SkippedJobs() = %d, want 1 (j3)", len(skipped))
	}
}

// ========================================
// Artifact Storage Tests
// ========================================

func TestMemoryArtifactStore(t *testing.T) {
	store := NewMemoryArtifactStore()

	artifact := &Artifact{
		ID:          "art-1",
		RunID:       "run-1",
		ConnectorID: "conn-1",
		DeviceID:    "10.0.0.1",
		FilePath:    "/tmp/test.txt",
		Success:     true,
	}

	ctx := context.Background()

	// Store
	if err := store.StoreArtifact(ctx, artifact); err != nil {
		t.Fatalf("StoreArtifact() error: %v", err)
	}

	// Get
	retrieved, err := store.GetArtifact(ctx, "art-1")
	if err != nil {
		t.Fatalf("GetArtifact() error: %v", err)
	}
	if retrieved.RunID != "run-1" {
		t.Errorf("retrieved.RunID = %s, want run-1", retrieved.RunID)
	}

	// List
	artifacts, err := store.ListArtifacts(ctx, "run-1", "", 10, 0)
	if err != nil {
		t.Fatalf("ListArtifacts() error: %v", err)
	}
	if len(artifacts) != 1 {
		t.Errorf("ListArtifacts() = %d, want 1", len(artifacts))
	}

	// Delete
	if err := store.DeleteArtifact(ctx, "art-1"); err != nil {
		t.Fatalf("DeleteArtifact() error: %v", err)
	}

	_, err = store.GetArtifact(ctx, "art-1")
	if err == nil {
		t.Error("GetArtifact() should error after delete")
	}
}

func TestMemoryArtifactStore_Prune(t *testing.T) {
	store := NewMemoryArtifactStore()
	ctx := context.Background()

	oldArtifact := &Artifact{ID: "old", RunID: "run-1", ConnectorID: "c1", DeviceID: "d1", FilePath: "/tmp/old.txt"}
	oldArtifact.StoredAt = time.Now().Add(-48 * time.Hour)
	store.StoreArtifact(ctx, oldArtifact)

	newArtifact := &Artifact{ID: "new", RunID: "run-2", ConnectorID: "c1", DeviceID: "d1", FilePath: "/tmp/new.txt"}
	store.StoreArtifact(ctx, newArtifact)

	pruned, err := store.PruneArtifacts(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("PruneArtifacts() error: %v", err)
	}
	if pruned != 1 {
		t.Errorf("PruneArtifacts() = %d, want 1", pruned)
	}

	// Old should be gone
	_, err = store.GetArtifact(ctx, "old")
	if err == nil {
		t.Error("old artifact should have been pruned")
	}

	// New should remain
	_, err = store.GetArtifact(ctx, "new")
	if err != nil {
		t.Error("new artifact should remain after prune")
	}
}

func TestFileArtifactStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "artifact-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file to store
	testFilePath := filepath.Join(tmpDir, "test_config.txt")
	if err := os.WriteFile(testFilePath, []byte("hostname test-device\ninterface Gi0/1\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store, err := NewFileArtifactStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileArtifactStore() error: %v", err)
	}

	artifact := &Artifact{
		RunID:       "run-file-1",
		ConnectorID: "conn-file-1",
		DeviceID:    "10.0.0.1",
		FilePath:    testFilePath,
		Success:     true,
	}

	ctx := context.Background()
	if err := store.StoreArtifact(ctx, artifact); err != nil {
		t.Fatalf("StoreArtifact() error: %v", err)
	}

	if artifact.ID == "" {
		t.Error("artifact ID should be set after store")
	}

	// Verify the artifact file exists at the stored path
	if _, err := os.Stat(artifact.FilePath); os.IsNotExist(err) {
		t.Errorf("stored artifact file does not exist: %s", artifact.FilePath)
	}
}

// ========================================
// Notification Tests
// ========================================

func TestLogNotifier(t *testing.T) {
	notifier := NewLogNotifier()

	event := &NotificationEvent{
		RunID:      "test-run",
		JobGUID:    "test-job",
		DeviceID:   "10.0.0.1",
		DeviceType: "cisco_ios",
		Success:    false,
		Error:      "connection timeout",
		Duration:   5 * time.Second,
	}

	ctx := context.Background()
	if err := notifier.Notify(ctx, event); err != nil {
		t.Fatalf("Notify() error: %v", err)
	}

	if len(notifier.Events) != 1 {
		t.Errorf("Events length = %d, want 1", len(notifier.Events))
	}
	if notifier.Events[0].DeviceID != "10.0.0.1" {
		t.Errorf("DeviceID = %s, want 10.0.0.1", notifier.Events[0].DeviceID)
	}
}

func TestMultiNotifier(t *testing.T) {
	n1 := NewLogNotifier()
	n2 := NewLogNotifier()
	multi := NewMultiNotifier(n1, n2)

	event := &NotificationEvent{RunID: "multi-test"}
	ctx := context.Background()

	if err := multi.Notify(ctx, event); err != nil {
		t.Fatalf("MultiNotifier.Notify() error: %v", err)
	}

	if len(n1.Events) != 1 || len(n2.Events) != 1 {
		t.Errorf("Both notifiers should have received event: n1=%d, n2=%d", len(n1.Events), len(n2.Events))
	}
}

func TestFilteredNotifier(t *testing.T) {
	log := NewLogNotifier()
	failureNotifier := NewFailureNotifier(log)

	ctx := context.Background()

	// Success event — should be filtered out
	successEvent := &NotificationEvent{RunID: "success", Success: true}
	failureNotifier.Notify(ctx, successEvent)
	if len(log.Events) != 0 {
		t.Error("success event should be filtered out")
	}

	// Failure event — should pass through
	failEvent := &NotificationEvent{RunID: "failure", Success: false, Error: "timeout"}
	failureNotifier.Notify(ctx, failEvent)
	if len(log.Events) != 1 {
		t.Error("failure event should pass through filter")
	}
}

// ========================================
// Scheduler Tests
// ========================================

func TestScheduler_AddAndListEntries(t *testing.T) {
	// Create a minimal orchestrator for testing
	co := &ConnectorOrchestrator{}
	scheduler := NewScheduler(co, nil)

	entry := &SchedulerEntry{
		Name:    "daily-collection",
		Enabled: true,
		Schedule: &Schedule{
			Minute: "0",
			Hour:   "6",
		},
		Jobs: []*ConnectorJob{
			{GUID: "j1", Enabled: true, Config: &base.ConnectionConfig{Host: "10.0.0.1"}},
		},
	}

	if err := scheduler.AddEntry(entry); err != nil {
		t.Fatalf("AddEntry() error: %v", err)
	}

	entries := scheduler.ListEntries()
	if len(entries) != 1 {
		t.Errorf("ListEntries() = %d, want 1", len(entries))
	}

	retrieved, err := scheduler.GetEntry("daily-collection")
	if err != nil {
		t.Fatalf("GetEntry() error: %v", err)
	}
	if retrieved.Name != "daily-collection" {
		t.Errorf("retrieved.Name = %s, want daily-collection", retrieved.Name)
	}
}

func TestScheduler_RemoveEntry(t *testing.T) {
	co := &ConnectorOrchestrator{}
	scheduler := NewScheduler(co, nil)

	scheduler.AddEntry(&SchedulerEntry{Name: "temp", Enabled: true})
	if err := scheduler.RemoveEntry("temp"); err != nil {
		t.Fatalf("RemoveEntry() error: %v", err)
	}

	entries := scheduler.ListEntries()
	if len(entries) != 0 {
		t.Errorf("entries after remove = %d, want 0", len(entries))
	}
}

func TestSchedule_IsRecurring(t *testing.T) {
	tests := []struct {
		name     string
		schedule *Schedule
		expected bool
	}{
		{"nil schedule", nil, false},
		{"all fields empty", &Schedule{}, false},
		{"minute set", &Schedule{Minute: "0"}, true},
		{"all wildcards", &Schedule{Minute: "*", Hour: "*", DayOfMonth: "*", Month: "*", DayOfWeek: "*"}, true},
	}

	for _, tt := range tests {
		got := tt.schedule.IsRecurring()
		if got != tt.expected {
			t.Errorf("%s: IsRecurring() = %v, want %v", tt.name, got, tt.expected)
		}
	}
}

// ========================================
// Helpers
// (containsStr and findSubstring are defined in error_handling_test.go)
// ========================================
