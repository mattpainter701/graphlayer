package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/orchestration/types"
)

type mockTaskService struct{}
func (m *mockTaskService) GetTask(taskGUID string) (*Task, error) {
	return &Task{GUID: taskGUID, Scheduled: false}, nil
}
func (m *mockTaskService) UpdateTaskStatus(taskGUID, status string) error { return nil }

type mockCredentialService struct{}
func (m *mockCredentialService) GetCredential(key string, decrypt bool) (*Credential, error) {
	return &Credential{Payload: map[string]interface{}{}}, nil
}

func makeTestOrchestrator(t *testing.T) *ConnectorOrchestrator {
	t.Helper()
	dir := t.TempDir()
	co, err := NewConnectorOrchestrator(dir, &mockCredentialService{}, &mockTaskService{})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	return co
}

func makeJob(guid string, enabled bool) *ConnectorJob {
	return &ConnectorJob{
		GUID:          guid,
		Name:          "test-" + guid,
		ConnectorType: "ssh",
		DeviceType:    types.DeviceTypeCiscoIOS,
		Config:        &base.ConnectionConfig{Host: guid + ".example.com"},
		Enabled:       enabled,
	}
}

func TestRunAll_BasicExecution(t *testing.T) {
	co := makeTestOrchestrator(t)
	jobs := []*ConnectorJob{
		makeJob("dev-1", true),
		makeJob("dev-2", true),
		makeJob("dev-3", false),
		makeJob("dev-4", true),
	}
	cfg := DefaultRunConfig()
	cfg.Incremental = false
	cfg.RetrySettings = nil
	cfg.PerConnectorTimeout = 2 * time.Second

	result, err := co.RunAll(context.Background(), jobs, cfg)
	if err != nil {
		t.Fatalf("RunAll error: %v", err)
	}
	if result.TotalJobs != 4 {
		t.Errorf("TotalJobs = %d, want 4", result.TotalJobs)
	}
	if len(result.Results) != 3 {
		t.Errorf("Results = %d, want 3", len(result.Results))
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestRunAll_DisabledJobsSkipped(t *testing.T) {
	co := makeTestOrchestrator(t)
	jobs := []*ConnectorJob{
		makeJob("off-1", false),
		makeJob("off-2", false),
	}
	cfg := DefaultRunConfig()
	cfg.Incremental = false

	result, err := co.RunAll(context.Background(), jobs, cfg)
	if err != nil {
		t.Fatalf("RunAll error: %v", err)
	}
	if len(result.Results) != 0 {
		t.Errorf("Results = %d, want 0", len(result.Results))
	}
}

func TestRunAll_EmptyJobs(t *testing.T) {
	co := makeTestOrchestrator(t)
	cfg := DefaultRunConfig()
	result, err := co.RunAll(context.Background(), []*ConnectorJob{}, cfg)
	if err != nil {
		t.Fatalf("RunAll error: %v", err)
	}
	if result.TotalJobs != 0 {
		t.Errorf("TotalJobs = %d, want 0", result.TotalJobs)
	}
}

func TestRunAll_PipelineLog(t *testing.T) {
	co := makeTestOrchestrator(t)
	jobs := []*ConnectorJob{
		makeJob("dev-a", true),
		makeJob("dev-b", true),
	}
	cfg := DefaultRunConfig()
	cfg.Incremental = false
	cfg.RetrySettings = nil

	result, err := co.RunAll(context.Background(), jobs, cfg)
	if err != nil {
		t.Fatalf("RunAll error: %v", err)
	}
	if result.PipelineLog == nil {
		t.Fatal("PipelineLog should not be nil")
	}
	if len(result.PipelineLog.JobEntries) != 2 {
		t.Errorf("JobEntries = %d, want 2", len(result.PipelineLog.JobEntries))
	}
}

func TestRunAll_RunIDUniqueness(t *testing.T) {
	co := makeTestOrchestrator(t)
	ids := make(map[string]bool)
	for i := 0; i < 5; i++ {
		jobs := []*ConnectorJob{makeJob(fmt.Sprintf("dev-%d", i), true)}
		cfg := DefaultRunConfig()
		cfg.Incremental = false
		cfg.RetrySettings = nil
		result, err := co.RunAll(context.Background(), jobs, cfg)
		if err != nil {
			t.Fatalf("RunAll error: %v", err)
		}
		if ids[result.RunID] {
			t.Errorf("Duplicate RunID: %s", result.RunID)
		}
		ids[result.RunID] = true
	}
}

func TestRunAll_ConcurrentSafety(t *testing.T) {
	co := makeTestOrchestrator(t)
	var wg sync.WaitGroup
	for round := 0; round < 3; round++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			jobs := make([]*ConnectorJob, 4)
			for i := range jobs {
				jobs[i] = makeJob(fmt.Sprintf("r%d-dev%d", r, i), true)
			}
			cfg := DefaultRunConfig()
			cfg.MaxConcurrency = 2
			cfg.Incremental = false
			cfg.RetrySettings = nil
			cfg.PerConnectorTimeout = 1 * time.Second
			_, _ = co.RunAll(context.Background(), jobs, cfg)
		}(round)
	}
	wg.Wait()
}

func TestDefaultRunConfig(t *testing.T) {
	cfg := DefaultRunConfig()
	if cfg.MaxConcurrency != 5 {
		t.Errorf("MaxConcurrency = %d, want 5", cfg.MaxConcurrency)
	}
	if cfg.PerConnectorTimeout != 5*time.Minute {
		t.Errorf("PerConnectorTimeout = %v, want 5m", cfg.PerConnectorTimeout)
	}
	if !cfg.Incremental {
		t.Error("Incremental should be true")
	}
}

func TestJobRunResult_Fields(t *testing.T) {
	co := makeTestOrchestrator(t)
	jobs := []*ConnectorJob{makeJob("meta-dev", true)}
	cfg := DefaultRunConfig()
	cfg.Incremental = false
	cfg.RetrySettings = nil

	result, err := co.RunAll(context.Background(), jobs, cfg)
	if err != nil {
		t.Fatalf("RunAll error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(result.Results))
	}
	r := result.Results[0]
	if r.Job.GUID != "meta-dev" {
		t.Errorf("Job.GUID = %q, want meta-dev", r.Job.GUID)
	}
	if r.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}
	if r.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}
}