package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mattpainter701/graphlayer/platform/reference/appid"
	"github.com/mattpainter701/graphlayer/platform/rustengine"
)

type FortiGateISDBCollector struct {
	connector *SSHConnector
	deviceID  string
	workDir   string
	logger    appid.Logger
}

func NewFortiGateISDBCollector(connector *SSHConnector, deviceID, workDir string) *FortiGateISDBCollector {
	return &FortiGateISDBCollector{connector: connector, deviceID: deviceID, workDir: workDir, logger: &defaultCollectorLogger{}}
}
func (c *FortiGateISDBCollector) SetLogger(logger appid.Logger) { c.logger = logger }
func (c *FortiGateISDBCollector) ValidateSSHAccess(ctx context.Context) error {
	if c.connector == nil {
		return rustengine.ErrUnavailable
	}
	return c.connector.Ping(ctx, 10*time.Second)
}
func (c *FortiGateISDBCollector) EstimateCollectionTime(maxID int) time.Duration {
	if maxID < 0 {
		maxID = 0
	}
	return time.Duration(maxID/1000+1) * time.Minute
}

func (c *FortiGateISDBCollector) CollectISDB(ctx context.Context) (*appid.CollectionResult, error) {
	result := appid.NewCollectionResult(c.deviceID, "fortios")
	result.Metadata.Source = "fortigate-ssh-cli"
	if c.connector == nil || c.connector.worker == nil {
		result.Complete(appid.FetchStatusFailed)
		return result, rustengine.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, appid.CollectionTimeout)
	defer cancel()
	var envelope map[string]json.RawMessage
	err := c.connector.worker.Call(ctx, "ssh.collect_isdb", map[string]any{"session_id": c.connector.workerSessionID, "device_id": c.deviceID}, &envelope)
	if err != nil {
		result.AddError(err)
		result.Complete(appid.FetchStatusFailed)
		return result, err
	}
	if raw := envelope["status"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &result.Status)
	}
	if raw := envelope["services_collected"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &result.ServicesCollected)
	}
	if raw := envelope["errors"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &result.ErrorMessages)
	}
	if raw := envelope["metadata"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &result.Metadata); err != nil {
			return nil, fmt.Errorf("invalid Rust ISDB metadata: %w", err)
		}
	}
	if raw := envelope["cache"]; len(raw) > 0 {
		var data struct {
			Services       map[string]*appid.ISDBServiceEntry `json:"services"`
			ServicesByName map[string]*appid.ISDBServiceEntry `json:"services_by_name"`
			Version        string                             `json:"version"`
			Date           string                             `json:"date"`
			Server         string                             `json:"server"`
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("invalid Rust ISDB cache: %w", err)
		}
		cache := appid.NewISDBCache()
		cache.Services = data.Services
		cache.ServicesByName = data.ServicesByName
		cache.Version = data.Version
		cache.Date = data.Date
		cache.Server = data.Server
		result.ISDBCache = cache
		if result.ServicesCollected == 0 {
			result.ServicesCollected = len(cache.Services)
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
