package orchestrator

import (
	"math"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/orchestration/connectors/base"
)

// RetrySettings configures exponential backoff retry behavior for connector jobs.
type RetrySettings struct {
	// MaxRetries is the maximum number of retry attempts (0 means no retries).
	MaxRetries int `json:"max_retries"`

	// InitialBackoff is the delay before the first retry.
	InitialBackoff time.Duration `json:"initial_backoff"`

	// MaxBackoff caps the exponential backoff delay.
	MaxBackoff time.Duration `json:"max_backoff"`

	// BackoffMultiplier controls the exponential factor (2.0 = standard doubling).
	BackoffMultiplier float64 `json:"backoff_multiplier"`

	// Jitter adds randomness to backoff (±50%) to avoid thundering herd.
	Jitter bool `json:"jitter"`

	// RetryOn specifies which errors are retryable. If empty, all errors are retryable.
	RetryOn []RetryableError `json:"retry_on,omitempty"`
}

// RetryableError categorizes errors for retry decisions.
type RetryableError string

const (
	RetryableConnectionError RetryableError = "connection_error"
	RetryableTimeout         RetryableError = "timeout"
	RetryableNetworkError    RetryableError = "network_error"
	RetryableSSHError        RetryableError = "ssh_error"
	RetryableAPIError        RetryableError = "api_error"
	RetryableDeviceError     RetryableError = "device_error"
	RetryableAll             RetryableError = "all"
)

// DefaultRetrySettings returns sensible default retry settings.
// 3 retries with exponential backoff starting at 5s, capping at 2m.
func DefaultRetrySettings() *RetrySettings {
	return &RetrySettings{
		MaxRetries:        3,
		InitialBackoff:    5 * time.Second,
		MaxBackoff:        2 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            true,
		RetryOn:           []RetryableError{RetryableAll},
	}
}

// NoRetry returns settings that disable retry entirely.
func NoRetry() *RetrySettings {
	return &RetrySettings{MaxRetries: 0}
}

// RetryAttempt records a single retry attempt outcome.
type RetryAttempt struct {
	Attempt  int              `json:"attempt"`
	Success  bool             `json:"success"`
	Error    string           `json:"error,omitempty"`
	Duration time.Duration    `json:"duration"`
	Result   *CollectionResult `json:"result,omitempty"`
}

// backoffDuration calculates the backoff duration for the given attempt number
// using the formula: initial * multiplier^(attempt-1), capped at maxBackoff.
func (rs *RetrySettings) backoffDuration(attempt int) time.Duration {
	if attempt <= 0 {
		return rs.InitialBackoff
	}

	multiplier := math.Pow(rs.BackoffMultiplier, float64(attempt-1))
	duration := time.Duration(float64(rs.InitialBackoff) * multiplier)

	if duration > rs.MaxBackoff {
		duration = rs.MaxBackoff
	}

	if rs.Jitter {
		// Add ±50% jitter
		jitterRange := duration / 2
		jitter := time.Duration(int64(jitterRange) * int64(attempt%3-1))
		duration += jitter
		if duration < 0 {
			duration = rs.InitialBackoff
		}
	}

	return duration
}

// isRetryable determines if an error/result qualifies for retry.
func (rs *RetrySettings) isRetryable(err error, result *CollectionResult) bool {
	// If no retry filters set, everything is retryable
	if len(rs.RetryOn) == 0 {
		return true
	}

	for _, category := range rs.RetryOn {
		if category == RetryableAll {
			return true
		}
	}

	if err != nil {
		errStr := strings.ToLower(err.Error())
		for _, category := range rs.RetryOn {
			if matchesRetryCategory(errStr, category) {
				return true
			}
		}
	}

	if result != nil {
		for _, e := range result.Errors {
			errStr := strings.ToLower(e)
			for _, category := range rs.RetryOn {
				if matchesRetryCategory(errStr, category) {
					return true
				}
			}
		}
	}

	return false
}

// matchesRetryCategory checks if an error string matches a retry category.
func matchesRetryCategory(errStr string, category RetryableError) bool {
	switch category {
	case RetryableConnectionError:
		return strings.Contains(errStr, "connection") ||
			strings.Contains(errStr, "connect") ||
			strings.Contains(errStr, "dial")
	case RetryableTimeout:
		return strings.Contains(errStr, "timeout") ||
			strings.Contains(errStr, "deadline exceeded") ||
			strings.Contains(errStr, "context deadline")
	case RetryableNetworkError:
		return strings.Contains(errStr, "network") ||
			strings.Contains(errStr, "unreachable") ||
			strings.Contains(errStr, "reset") ||
			strings.Contains(errStr, "refused") ||
			strings.Contains(errStr, "broken pipe")
	case RetryableSSHError:
		return strings.Contains(errStr, "ssh") ||
			strings.Contains(errStr, "handshake") ||
			strings.Contains(errStr, "authentication")
	case RetryableAPIError:
		return strings.Contains(errStr, "api") ||
			strings.Contains(errStr, "http") ||
			strings.Contains(errStr, "500") ||
			strings.Contains(errStr, "502") ||
			strings.Contains(errStr, "503") ||
			strings.Contains(errStr, "504")
	case RetryableDeviceError:
		return strings.Contains(errStr, "device")
	default:
		return false
	}
}

// ExecutePullWithRetry wraps ExecutePull with exponential backoff retry.
// This is a convenience method for single-job execution with retry.
func (co *ConnectorOrchestrator) ExecutePullWithRetry(job *ConnectorJob, rs *RetrySettings) (*CollectionResult, error) {
	if rs == nil {
		rs = DefaultRetrySettings()
	}

	var lastResult *CollectionResult
	var lastErr error

	attempt := 0
	for attempt <= rs.MaxRetries {
		attempt++

		connResult, connErr := co.ExecutePull(nil, job) // nil context — caller's responsibility

		if connErr == nil && connResult.Success {
			return connResult, nil
		}

		lastResult = connResult
		lastErr = connErr

		if attempt > rs.MaxRetries {
			break
		}

		if !rs.isRetryable(connErr, connResult) {
			break
		}

		backoff := rs.backoffDuration(attempt)
		time.Sleep(backoff)
	}

	if lastErr != nil {
		return lastResult, lastErr
	}
	return lastResult, nil
}

// ExecutePullWithConfig executes a pull with full RunConfig (retry, timeout, etc.)
// for a single job, not parallel.
func (co *ConnectorOrchestrator) ExecutePullWithConfig(job *ConnectorJob, cfg *RunConfig) *JobRunResult {
	if cfg == nil {
		cfg = DefaultRunConfig()
	}
	return co.runSingleJob(nil, job, cfg, "standalone")
}

// calculateBackoff returns the backoff duration for display/planning purposes.
func calculateBackoff(attempt int, rs *RetrySettings) time.Duration {
	if rs == nil {
		return 0
	}
	return rs.backoffDuration(attempt)
}

// RetryPolicy configures retry policies for specific connector types.
type RetryPolicy struct {
	ConnectorType string         `json:"connector_type"`
	DeviceType    string         `json:"device_type,omitempty"`
	Settings      *RetrySettings `json:"settings"`
}

// MatchConnector checks if this policy applies to a given connector type and device.
func (rp *RetryPolicy) MatchConnector(connectorType, deviceType string) bool {
	if rp.ConnectorType != "" && rp.ConnectorType != connectorType {
		return false
	}
	if rp.DeviceType != "" && rp.DeviceType != deviceType {
		return false
	}
	return true
}

// RetryPolicyRegistry manages per-connector-type retry policies.
type RetryPolicyRegistry struct {
	Policies []*RetryPolicy        `json:"policies"`
	Defaults  *RetrySettings       `json:"defaults"`
}

// NewRetryPolicyRegistry creates a new registry with default retry settings.
func NewRetryPolicyRegistry() *RetryPolicyRegistry {
	return &RetryPolicyRegistry{
		Policies: make([]*RetryPolicy, 0),
		Defaults: DefaultRetrySettings(),
	}
}

// Register adds a retry policy for a specific connector/device type.
func (rpr *RetryPolicyRegistry) Register(policy *RetryPolicy) {
	rpr.Policies = append(rpr.Policies, policy)
}

// GetPolicy returns the retry settings that match the given connector type.
func (rpr *RetryPolicyRegistry) GetPolicy(connectorType, deviceType string) *RetrySettings {
	for _, policy := range rpr.Policies {
		if policy.MatchConnector(connectorType, deviceType) {
			return policy.Settings
		}
	}
	return rpr.Defaults
}

// Helper to create connector-specific retry.
func APIQuickRetry() *RetrySettings {
	return &RetrySettings{
		MaxRetries:        5,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            true,
		RetryOn:           []RetryableError{RetryableAPIError, RetryableNetworkError, RetryableTimeout},
	}
}

func SSHConservativeRetry() *RetrySettings {
	return &RetrySettings{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Second,
		MaxBackoff:        5 * time.Minute,
		BackoffMultiplier: 2.0,
		Jitter:            true,
		RetryOn:           []RetryableError{RetryableConnectionError, RetryableSSHError, RetryableNetworkError, RetryableTimeout},
	}
}

// Helper for ResetConnector — used with base.Connector interface
type ResetConnector interface {
	base.Connector
	Reset() error
}
