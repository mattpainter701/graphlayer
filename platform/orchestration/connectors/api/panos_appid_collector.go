package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mattpainter701/graphlayer/platform/reference/appid"
	"github.com/mattpainter701/graphlayer/platform/rustengine"
)

type PANOSAppIDCollector struct {
	connector *APIConnector
	deviceID  string
	workDir   string
	logger    appid.Logger
}

func NewPANOSAppIDCollector(connector *APIConnector, deviceID, workDir string) *PANOSAppIDCollector {
	return &PANOSAppIDCollector{connector: connector, deviceID: deviceID, workDir: workDir, logger: &defaultCollectorLogger{}}
}
func (c *PANOSAppIDCollector) SetLogger(logger appid.Logger) { c.logger = logger }
func (c *PANOSAppIDCollector) ValidateAPIAccess(ctx context.Context) error {
	if c.connector == nil {
		return rustengine.ErrUnavailable
	}
	return c.connector.Ping(ctx, 10*time.Second)
}
func (c *PANOSAppIDCollector) EstimateCollectionTime() time.Duration { return 5 * time.Minute }

func (c *PANOSAppIDCollector) CollectAppID(ctx context.Context) (*appid.CollectionResult, error) {
	result := appid.NewCollectionResult(c.deviceID, "panos")
	result.Metadata.Source = "panos-xml-api"
	if c.connector == nil || c.connector.worker == nil {
		result.Complete(appid.FetchStatusFailed)
		return result, rustengine.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, appid.CollectionTimeout)
	defer cancel()
	var envelope map[string]json.RawMessage
	err := c.connector.worker.Call(ctx, "api.collect_appid", map[string]any{"session_id": c.connector.workerSessionID, "device_id": c.deviceID}, &envelope)
	if err != nil {
		result.AddError(err)
		result.Complete(appid.FetchStatusFailed)
		return result, err
	}
	if raw := envelope["status"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &result.Status)
	}
	if raw := envelope["apps_collected"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &result.AppsCollected)
	}
	if raw := envelope["errors"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &result.ErrorMessages)
	}
	if raw := envelope["metadata"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &result.Metadata); err != nil {
			return nil, fmt.Errorf("invalid Rust App-ID metadata: %w", err)
		}
	}
	if raw := envelope["cache"]; len(raw) > 0 {
		var data struct {
			Applications map[string]*appid.AppIDEntry `json:"applications"`
			Version      string                       `json:"version"`
			LastUpdated  string                       `json:"last_updated"`
			Source       string                       `json:"source"`
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("invalid Rust App-ID cache: %w", err)
		}
		cache := appid.NewAppIDCache()
		cache.Applications = data.Applications
		cache.Version = data.Version
		cache.LastUpdated = data.LastUpdated
		cache.Source = data.Source
		result.AppIDCache = cache
		if result.AppsCollected == 0 {
			result.AppsCollected = len(cache.Applications)
		}
	}
	result.Complete(result.Status)
	return result, nil
}

type defaultCollectorLogger struct{}

func (l *defaultCollectorLogger) Info(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}
func (l *defaultCollectorLogger) Warn(format string, args ...interface{}) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}
func (l *defaultCollectorLogger) Error(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}
func (l *defaultCollectorLogger) Debug(format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}
