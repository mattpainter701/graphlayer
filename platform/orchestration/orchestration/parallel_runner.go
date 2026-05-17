package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mattpainter701/graphlayer/platform/orchestration/types"
)

// RunConfig configures a parallel collection run with concurrency control,
// retry policy, incremental collection, and notification hooks.
type RunConfig struct {
	// MaxConcurrency limits the number of connectors running simultaneously.
	// 0 or negative means unlimited (but bounded by len(jobs)).
	MaxConcurrency int `json:"max_concurrency"`

	// Timeout per-connector collection attempt (excluding retries).
	PerConnectorTimeout time.Duration `json:"per_connector_timeout"`

	// RetrySettings configures exponential backoff retries. If nil, no retries.
	RetrySettings *RetrySettings `json:"retry_settings,omitempty"`

	// Incremental enables hash-based change detection to skip unchanged devices.
	Incremental bool `json:"incremental"`

	// NotificationHooks are called on collection events (success, failure, completion).
	NotificationHooks []NotificationHook `json:"notification_hooks,omitempty"`

	// ArtifactStore persists raw collection data for reprocessing.
	ArtifactStore ArtifactStore `json:"artifact_store,omitempty"`
}

// DefaultRunConfig returns sane defaults for a parallel collection run.
func DefaultRunConfig() *RunConfig {
	return &RunConfig{
		MaxConcurrency:      5,
		PerConnectorTimeout: 5 * time.Minute,
		RetrySettings:       DefaultRetrySettings(),
		Incremental:         true,
	}
}

// RunResult aggregates results from running multiple connector jobs in parallel.
type RunResult struct {
	RunID        string                    `json:"run_id"`
	StartedAt    time.Time                 `json:"started_at"`
	CompletedAt  time.Time                 `json:"completed_at"`
	Duration     time.Duration             `json:"duration"`
	TotalJobs    int                       `json:"total_jobs"`
	Successful   int                       `json:"successful"`
	Failed       int                       `json:"failed"`
	Skipped      int                       `json:"skipped"`
	Results      []*JobRunResult           `json:"results"`
	HealthReport *HealthReport             `json:"health_report,omitempty"`
	PipelineLog  *PipelineRunLog           `json:"pipeline_log,omitempty"`
}

// JobRunResult captures the outcome of a single connector job execution.
type JobRunResult struct {
	Job             *ConnectorJob      `json:"job"`
	Success         bool               `json:"success"`
	Result          *CollectionResult  `json:"result,omitempty"`
	Error           string             `json:"error,omitempty"`
	Attempts        int                `json:"attempts"`
	StartedAt       time.Time          `json:"started_at"`
	CompletedAt     time.Time          `json:"completed_at"`
	Duration        time.Duration      `json:"duration"`
	RetryAttempts   []*RetryAttempt    `json:"retry_attempts,omitempty"`
	Skipped         bool               `json:"skipped"`
	SkipReason      string             `json:"skip_reason,omitempty"`
}

// RunAll executes a collection of connector jobs in parallel with configurable
// concurrency, retries, incremental collection, artifact storage, and notifications.
// This is the main entry point for the Orchestration Engine v2.
func (co *ConnectorOrchestrator) RunAll(ctx context.Context, jobs []*ConnectorJob, cfg *RunConfig) (*RunResult, error) {
	if cfg == nil {
		cfg = DefaultRunConfig()
	}

	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	startedAt := time.Now()

	runResult := &RunResult{
		RunID:     runID,
		StartedAt: startedAt,
		TotalJobs: len(jobs),
		Results:   make([]*JobRunResult, 0, len(jobs)),
		PipelineLog: &PipelineRunLog{
			RunID:      runID,
			StartedAt:  startedAt,
			JobEntries: make([]*PipelineJobEntry, 0, len(jobs)),
		},
	}

	// Deduplicate and filter: skip disabled jobs
	activeJobs := make([]*ConnectorJob, 0, len(jobs))
	for _, j := range jobs {
		if j.Enabled {
			activeJobs = append(activeJobs, j)
		}
	}

	// Concurrency control with semaphore pattern
	maxConcurrency := cfg.MaxConcurrency
	if maxConcurrency <= 0 || maxConcurrency > len(activeJobs) {
		maxConcurrency = len(activeJobs)
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, job := range activeJobs {
		wg.Add(1)
		go func(j *ConnectorJob) {
			defer wg.Done()

			// Acquire semaphore slot
			sem <- struct{}{}
			defer func() { <-sem }()

			jobResult := co.runSingleJob(ctx, j, cfg, runID)

			mu.Lock()
			runResult.Results = append(runResult.Results, jobResult)
			if jobResult.Skipped {
				runResult.Skipped++
			} else if jobResult.Success {
				runResult.Successful++
			} else {
				runResult.Failed++
			}

			// Append to pipeline log
			runResult.PipelineLog.JobEntries = append(runResult.PipelineLog.JobEntries,
				jobResultToPipelineEntry(jobResult))
			mu.Unlock()

			// Fire notification hooks
			co.fireNotifications(ctx, cfg.NotificationHooks, jobResult, runID)
		}(job)
	}

	wg.Wait()

	runResult.CompletedAt = time.Now()
	runResult.Duration = runResult.CompletedAt.Sub(runResult.StartedAt)
	runResult.PipelineLog.CompletedAt = runResult.CompletedAt
	runResult.PipelineLog.Duration = runResult.Duration
	runResult.PipelineLog.TotalJobs = runResult.TotalJobs
	runResult.PipelineLog.Successful = runResult.Successful
	runResult.PipelineLog.Failed = runResult.Failed
	runResult.PipelineLog.Skipped = runResult.Skipped

	return runResult, nil
}

// runSingleJob executes one connector job with optional retry and incremental checks.
func (co *ConnectorOrchestrator) runSingleJob(ctx context.Context, job *ConnectorJob, cfg *RunConfig, runID string) *JobRunResult {
	startedAt := time.Now()
	result := &JobRunResult{
		Job:       job,
		StartedAt: startedAt,
	}

	// Incremental collection check: skip if device data hasn't changed
	if cfg.Incremental {
		changed, reason, err := co.hasDeviceChanged(ctx, job)
		if err == nil && !changed {
			result.Skipped = true
			result.SkipReason = reason
			result.CompletedAt = time.Now()
			result.Duration = result.CompletedAt.Sub(startedAt)
			return result
		}
		// On error checking, proceed with collection (fail open)
	}

	// Determine per-connector timeout
	connectorTimeout := cfg.PerConnectorTimeout
	if connectorTimeout <= 0 {
		connectorTimeout = 5 * time.Minute
	}

	// Execute with retry if configured
	if cfg.RetrySettings != nil && cfg.RetrySettings.MaxRetries > 0 {
		return co.runWithRetry(ctx, job, cfg, startedAt)
	}

	// Single attempt
	connCtx, cancel := context.WithTimeout(ctx, connectorTimeout)
	defer cancel()

	collectionResult, err := co.ExecutePull(connCtx, job)
	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(startedAt)
	result.Attempts = 1

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.Result = collectionResult
	} else {
		result.Success = collectionResult.Success
		result.Result = collectionResult
		if !collectionResult.Success && len(collectionResult.Errors) > 0 {
			result.Error = fmt.Sprintf("collection errors: %v", collectionResult.Errors)
		}
	}

	// Store raw artifacts
	if cfg.ArtifactStore != nil {
		co.storeArtifacts(ctx, cfg.ArtifactStore, job, collectionResult, runID)
	}

	return result
}

// runWithRetry executes a job with exponential backoff retry logic.
func (co *ConnectorOrchestrator) runWithRetry(ctx context.Context, job *ConnectorJob, cfg *RunConfig, startedAt time.Time) *JobRunResult {
	rs := cfg.RetrySettings
	result := &JobRunResult{
		Job:           job,
		StartedAt:     startedAt,
		RetryAttempts: make([]*RetryAttempt, 0),
	}

	attempt := 0
	for attempt <= rs.MaxRetries {
		attempt++
		attemptStart := time.Now()

		connCtx, cancel := context.WithTimeout(ctx, cfg.PerConnectorTimeout)
		collectionResult, err := co.ExecutePull(connCtx, job)
		cancel()

		attemptDuration := time.Since(attemptStart)
		retryAttempt := &RetryAttempt{
			Attempt:  attempt,
			Success:  err == nil && collectionResult.Success,
			Error:    "",
			Duration: attemptDuration,
			Result:   collectionResult,
		}
		if err != nil {
			retryAttempt.Error = err.Error()
		} else if !collectionResult.Success && len(collectionResult.Errors) > 0 {
			retryAttempt.Error = fmt.Sprintf("collection errors: %v", collectionResult.Errors)
		}
		result.RetryAttempts = append(result.RetryAttempts, retryAttempt)

		if retryAttempt.Success {
			result.Success = true
			result.Result = collectionResult
			break
		}

		// Last attempt — give up
		if attempt > rs.MaxRetries {
			result.Success = false
			result.Error = retryAttempt.Error
			result.Result = collectionResult
			break
		}

		// Determine if this error is retryable
		if !rs.isRetryable(err, collectionResult) {
			result.Success = false
			result.Error = retryAttempt.Error
			result.Result = collectionResult
			break
		}

		// Apply exponential backoff
		backoff := rs.backoffDuration(attempt)
		select {
		case <-ctx.Done():
			result.Success = false
			result.Error = "context cancelled during backoff"
			result.Result = collectionResult
			result.CompletedAt = time.Now()
			result.Duration = result.CompletedAt.Sub(startedAt)
			result.Attempts = attempt
			return result
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(startedAt)
	result.Attempts = attempt
	return result
}

// hasDeviceChanged checks whether a device's data has changed since last collection.
// Returns (changed, reason, error). Fail-open: on error, assume changed = true.
func (co *ConnectorOrchestrator) hasDeviceChanged(_ context.Context, job *ConnectorJob) (bool, string, error) {
	// Check if we have tracked files for this device
	files, err := co.storage.GetDownloadedFiles(job.GUID)
	if err != nil || len(files) == 0 {
		// No previous data or error — treat as changed
		return true, "no previous collection data", nil
	}

	// For scheduled tasks, check file hashes against tracked commits
	if job.TaskGUID != "" {
		task, err := co.taskService.GetTask(job.TaskGUID)
		if err == nil && task.Scheduled {
			// Check each workspace
			allUnchanged := true
			for _, ws := range job.Workspaces {
				for _, file := range files {
					shouldCommit, err := co.storage.ShouldCommitFile(file, ws, job.Devices)
					if err != nil || shouldCommit {
						allUnchanged = false
						break
					}
				}
				if !allUnchanged {
					break
				}
			}
			if allUnchanged && len(files) > 0 {
				return false, fmt.Sprintf("no changes detected for device %s (tracked files unchanged)", job.Config.Host), nil
			}
		}
	}

	return true, "", nil
}

// storeArtifacts persists raw collection data for later reprocessing.
func (co *ConnectorOrchestrator) storeArtifacts(ctx context.Context, store ArtifactStore, job *ConnectorJob, result *CollectionResult, runID string) {
	if store == nil || result == nil {
		return
	}

	for _, file := range result.ConfigFiles {
		artifact := &Artifact{
			RunID:       runID,
			ConnectorID: job.GUID,
			DeviceID:    job.Config.Host,
			FilePath:    file,
			CollectedAt: time.Now(),
			Success:     result.Success,
		}
		if err := store.StoreArtifact(ctx, artifact); err != nil {
			// Artifact storage is best-effort — log but don't fail
			fmt.Printf("[WARN] Failed to store artifact %s: %v\n", file, err)
		}
	}
}

// fireNotifications calls notification hooks for a job result.
func (co *ConnectorOrchestrator) fireNotifications(ctx context.Context, hooks []NotificationHook, result *JobRunResult, runID string) {
	if len(hooks) == 0 {
		return
	}

	event := &NotificationEvent{
		RunID:      runID,
		JobGUID:    result.Job.GUID,
		DeviceID:   result.Job.Config.Host,
		DeviceType: string(result.Job.DeviceType),
		Success:    result.Success,
		Skipped:    result.Skipped,
		Error:      result.Error,
		Duration:   result.Duration,
		Timestamp:  time.Now(),
	}

	for _, hook := range hooks {
		// Fire hooks in background goroutines so they don't block
		go func(h NotificationHook) {
			if err := h.Notify(ctx, event); err != nil {
				fmt.Printf("[WARN] Notification hook failed: %v\n", err)
			}
		}(hook)
	}
}

// jobResultToPipelineEntry converts a JobRunResult into a PipelineJobEntry for visualization.
func jobResultToPipelineEntry(jr *JobRunResult) *PipelineJobEntry {
	status := PipelineStatusSuccess
	if jr.Skipped {
		status = PipelineStatusSkipped
	} else if !jr.Success {
		status = PipelineStatusFailed
	}

	return &PipelineJobEntry{
		JobGUID:    jr.Job.GUID,
		DeviceType: string(jr.Job.DeviceType),
		DeviceID:   jr.Job.Config.Host,
		Status:     status,
		StartedAt:  jr.StartedAt,
		EndedAt:    jr.CompletedAt,
		Duration:   jr.Duration,
		Attempts:   jr.Attempts,
		Error:      jr.Error,
		Skipped:    jr.Skipped,
		SkipReason: jr.SkipReason,
	}
}

// JobConfig is a convenience type for building ConnectorJobs.
type JobConfig struct {
	GUID                 string
	Name                 string
	ConnectorType        string
	DeviceType           types.DeviceType
	Host                 string
	Port                 int
	Username             string
	Password             string
	TaskGUID             string
	Enabled              bool
	AuxDataEnabled       bool
	CollectAppID         bool
	CollectISDB          bool
	Devices              []string
	Workspaces           []string
	ConditionalExecution bool
	CapabilityDetection  bool
}
