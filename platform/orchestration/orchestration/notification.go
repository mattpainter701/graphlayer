package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// NotificationEvent represents a collection event for notification hooks.
type NotificationEvent struct {
	RunID      string    `json:"run_id"`
	JobGUID    string    `json:"job_guid"`
	DeviceID   string    `json:"device_id"`
	DeviceType string    `json:"device_type"`
	Success    bool      `json:"success"`
	Skipped    bool      `json:"skipped"`
	Error      string    `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	Timestamp  time.Time `json:"timestamp"`
}

// NotificationHook is called when collection events occur (success, failure, completion).
type NotificationHook interface {
	// Notify is called with the event. Must not block indefinitely.
	Notify(ctx context.Context, event *NotificationEvent) error

	// Name returns a human-readable hook name for logging.
	Name() string
}

// WebhookNotifier sends HTTP POST notifications to a configurable URL.
type WebhookNotifier struct {
	URL         string
	Method      string
	Headers     map[string]string
	Timeout     time.Duration
	client      *http.Client
}

// NewWebhookNotifier creates a webhook-based notification hook.
func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{
		URL:     url,
		Method:  "POST",
		Headers: map[string]string{"Content-Type": "application/json"},
		Timeout: 10 * time.Second,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Notify sends the event as JSON to the configured webhook URL.
func (wn *WebhookNotifier) Notify(ctx context.Context, event *NotificationEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, wn.Method, wn.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	for k, v := range wn.Headers {
		req.Header.Set(k, v)
	}

	resp, err := wn.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// Name returns the hook name.
func (wn *WebhookNotifier) Name() string {
	return fmt.Sprintf("webhook:%s", wn.URL)
}

// SlackNotifier sends notifications to a Slack webhook URL.
type SlackNotifier struct {
	WebhookURL string
	Channel    string
	Username   string
	Timeout    time.Duration
	client     *http.Client
}

// slackMessage is the Slack webhook message payload.
type slackMessage struct {
	Channel   string        `json:"channel,omitempty"`
	Username  string        `json:"username,omitempty"`
	Text      string        `json:"text"`
	Mrkdwn    bool          `json:"mrkdwn"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type slackAttachment struct {
	Color    string `json:"color"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Footer   string `json:"footer"`
	Ts       int64  `json:"ts"`
}

// NewSlackNotifier creates a Slack webhook notification hook.
func NewSlackNotifier(webhookURL, channel string) *SlackNotifier {
	return &SlackNotifier{
		WebhookURL: webhookURL,
		Channel:    channel,
		Username:   "GraphLayer Orchestrator",
		Timeout:    10 * time.Second,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify sends a formatted Slack message for the collection event.
func (sn *SlackNotifier) Notify(ctx context.Context, event *NotificationEvent) error {
	// Only notify on failures (reduce noise)
	if event.Success {
		return nil
	}

	color := "good"
	status := "✅ SUCCESS"
	if !event.Success {
		color = "danger"
		status = "❌ FAILED"
	}
	if event.Skipped {
		color = "warning"
		status = "⏭️ SKIPPED"
	}

	msg := slackMessage{
		Channel:  sn.Channel,
		Username: sn.Username,
		Text:     fmt.Sprintf("*Collection %s*: %s (%s)", status, event.DeviceID, event.DeviceType),
		Mrkdwn:   true,
		Attachments: []slackAttachment{
			{
				Color: color,
				Title: fmt.Sprintf("Run %s — %s", event.RunID, event.DeviceID),
				Text:  fmt.Sprintf("Device: %s\nType: %s\nDuration: %s\nError: %s",
					event.DeviceID, event.DeviceType, event.Duration.Round(time.Millisecond), event.Error),
				Footer: "GraphLayer Orchestration Engine v2",
				Ts:     event.Timestamp.Unix(),
			},
		},
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", sn.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create Slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := sn.client.Do(req)
	if err != nil {
		return fmt.Errorf("Slack request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Slack returned status %d", resp.StatusCode)
	}

	return nil
}

// Name returns the hook name.
func (sn *SlackNotifier) Name() string {
	return fmt.Sprintf("slack:%s", sn.Channel)
}

// LogNotifier writes notification events to a log (for debugging/testing).
type LogNotifier struct {
	Events []*NotificationEvent
}

// NewLogNotifier creates a log-based notification hook.
func NewLogNotifier() *LogNotifier {
	return &LogNotifier{
		Events: make([]*NotificationEvent, 0),
	}
}

// Notify records the event in the log.
func (ln *LogNotifier) Notify(ctx context.Context, event *NotificationEvent) error {
	ln.Events = append(ln.Events, event)
	return nil
}

// Name returns the hook name.
func (ln *LogNotifier) Name() string {
	return "log"
}

// MultiNotifier fans out notifications to multiple hooks.
type MultiNotifier struct {
	Hooks []NotificationHook
}

// NewMultiNotifier creates a composite notification hook.
func NewMultiNotifier(hooks ...NotificationHook) *MultiNotifier {
	return &MultiNotifier{Hooks: hooks}
}

// Notify fans out to all registered hooks.
func (mn *MultiNotifier) Notify(ctx context.Context, event *NotificationEvent) error {
	var lastErr error
	for _, hook := range mn.Hooks {
		if err := hook.Notify(ctx, event); err != nil {
			lastErr = err
			fmt.Printf("[WARN] Notification hook %q failed: %v\n", hook.Name(), err)
		}
	}
	return lastErr
}

// Name returns the hook name.
func (mn *MultiNotifier) Name() string {
	return "multi"
}

// FilteredNotifier only notifies for specific event types (e.g., only failures).
type FilteredNotifier struct {
	Hook     NotificationHook
	OnlyFailures bool
	OnlySuccess  bool
	OnlySkipped  bool
}

// NewFailureNotifier creates a notifier that only fires on failures.
func NewFailureNotifier(hook NotificationHook) *FilteredNotifier {
	return &FilteredNotifier{
		Hook:         hook,
		OnlyFailures: true,
	}
}

// Notify conditionally forwards the event to the wrapped hook.
func (fn *FilteredNotifier) Notify(ctx context.Context, event *NotificationEvent) error {
	if fn.OnlyFailures && event.Success {
		return nil
	}
	if fn.OnlySuccess && !event.Success {
		return nil
	}
	if fn.OnlySkipped && !event.Skipped {
		return nil
	}
	return fn.Hook.Notify(ctx, event)
}

// Name returns the hook name.
func (fn *FilteredNotifier) Name() string {
	return fmt.Sprintf("filtered:%s", fn.Hook.Name())
}
