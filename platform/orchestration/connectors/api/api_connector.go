package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/mattpainter701/graphlayer/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/orchestration/types"
	"github.com/mattpainter701/graphlayer/platform/rustengine"
)

const maxAPIRequestBodyBytes int64 = 16 << 20

type APIConnector struct {
	*base.BaseConnector
	deviceType      types.DeviceType
	worker          *rustengine.Session
	workerSessionID string
	sessionMutex    sync.Mutex
	workerMutex     sync.RWMutex
	callGate        sync.RWMutex
	requestCount    uint64
	requestMutex    sync.RWMutex
}

type APIResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error,omitempty"`
}
type DeviceListResponse struct {
	Devices []DeviceInfo `json:"devices"`
	Total   int          `json:"total"`
}
type DeviceInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip_address"`
	Model    string `json:"model"`
	Vendor   string `json:"vendor"`
	Status   string `json:"status"`
}

func NewAPIConnector(config *base.ConnectionConfig, deviceType types.DeviceType) (*APIConnector, error) {
	if config == nil {
		return nil, fmt.Errorf("connection configuration is nil")
	}
	bc := base.NewBaseConnector(config)
	if err := bc.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return &APIConnector{BaseConnector: bc, deviceType: deviceType}, nil
}

func (ac *APIConnector) Connect(ctx context.Context) error {
	ac.sessionMutex.Lock()
	defer ac.sessionMutex.Unlock()
	if ac.session() != nil {
		return nil
	}
	if err := ac.BaseConnector.Connect(ctx); err != nil {
		return err
	}
	ac.SetConnectionState(base.ConnectionStateConnecting)
	worker, err := rustengine.NewSession()
	if err != nil {
		ac.SetConnectionState(base.ConnectionStateError)
		return err
	}
	sid := ac.generateDeviceID()
	var result struct {
		SessionID string `json:"session_id"`
		Connected bool   `json:"connected"`
	}
	input := map[string]any{"session_id": sid, "device_type": string(ac.deviceType), "config": map[string]any{"host": ac.Config.Host, "port": ac.Config.Port, "username": ac.Config.Username, "password": ac.Config.Password, "api_key": ac.Config.APIKey, "token": ac.Config.Token, "headers": ac.Config.Headers, "timeout_seconds": ac.Config.Timeout.Seconds(), "retry_count": ac.Config.RetryCount, "retry_delay_ms": ac.Config.RetryDelay.Milliseconds(), "ca_cert": ac.Config.CACert, "cert_file": ac.Config.CertFile, "key_file_api": ac.Config.KeyFileAPI}}
	if err = worker.Call(ctx, "api.connect", input, &result); err != nil {
		_ = worker.Close()
		ac.SetConnectionState(base.ConnectionStateError)
		return fmt.Errorf("failed to authenticate to API server: %w", err)
	}
	if !result.Connected {
		_ = worker.Close()
		ac.SetConnectionState(base.ConnectionStateError)
		return fmt.Errorf("API worker did not establish a session")
	}
	workerID := sid
	if result.SessionID != "" {
		workerID = result.SessionID
	}
	ac.workerMutex.Lock()
	ac.worker = worker
	ac.workerSessionID = workerID
	ac.workerMutex.Unlock()
	ac.SetConnected(true)
	ac.SetConnectionState(base.ConnectionStateConnected)
	return nil
}

func (ac *APIConnector) Disconnect(ctx context.Context) error {
	ac.sessionMutex.Lock()
	defer ac.sessionMutex.Unlock()
	ac.callGate.Lock()
	defer ac.callGate.Unlock()
	ac.workerMutex.Lock()
	worker, sid := ac.worker, ac.workerSessionID
	ac.worker = nil
	ac.workerSessionID = ""
	ac.workerMutex.Unlock()
	var disconnectErr error
	if worker != nil {
		var result map[string]json.RawMessage
		disconnectErr = callSession(ctx, worker, sid, "api.disconnect", map[string]any{}, &result)
		if closeErr := worker.Close(); disconnectErr == nil {
			disconnectErr = closeErr
		}
	}
	ac.SetConnected(false)
	ac.SetConnectionState(base.ConnectionStateDisconnected)
	return disconnectErr
}

func (ac *APIConnector) call(ctx context.Context, operation string, input map[string]any, out any) error {
	ac.callGate.RLock()
	defer ac.callGate.RUnlock()
	ac.workerMutex.RLock()
	worker, sid := ac.worker, ac.workerSessionID
	ac.workerMutex.RUnlock()
	if worker == nil {
		return rustengine.ErrUnavailable
	}
	return callSession(ctx, worker, sid, operation, input, out)
}

func callSession(ctx context.Context, worker *rustengine.Session, sid, operation string, input map[string]any, out any) error {
	input["session_id"] = sid
	return worker.Call(ctx, operation, input, out)
}

func (ac *APIConnector) session() *rustengine.Session {
	ac.workerMutex.RLock()
	defer ac.workerMutex.RUnlock()
	return ac.worker
}

func (ac *APIConnector) IsConnected() bool {
	return ac.BaseConnector.IsConnected() && ac.session() != nil
}
func (ac *APIConnector) GetType() string { return "api" }
func (ac *APIConnector) Ping(ctx context.Context, timeout time.Duration) error {
	if !ac.IsConnected() {
		return fmt.Errorf("not connected")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var out map[string]json.RawMessage
	return ac.call(ctx, "api.health", map[string]any{"device_type": string(ac.deviceType)}, &out)
}
func (ac *APIConnector) GetDeviceInfo(ctx context.Context) (*base.DeviceInfo, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	var raw map[string]json.RawMessage
	if err := ac.call(ctx, "api.device_info", map[string]any{"device_type": string(ac.deviceType)}, &raw); err != nil {
		return nil, err
	}
	var info base.DeviceInfo
	if err := json.Unmarshal(mustJSON(raw), &info); err != nil {
		return nil, err
	}
	return &info, nil
}
func (ac *APIConnector) GetSupportedCommands(ctx context.Context) ([]string, error) {
	var commands []string
	if err := ac.call(ctx, "api.supported_commands", map[string]any{"device_type": string(ac.deviceType)}, &commands); err != nil {
		return nil, err
	}
	return commands, nil
}

func (ac *APIConnector) ExecuteCommand(ctx context.Context, command string, timeout time.Duration) (*base.CommandResult, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	var output string
	var err error
	if command == "get_device_config" {
		var c base.ConfigResult
		c, err = ac.config(ctx, ac.Config.Host, "running")
		output = c.ConfigData
	} else {
		op, script := command, ""
		switch command {
		case "get_device_list", "get_security_events", "get_policies", "get_device_groups", "get_templates", "get_node_info", "get_device_info":
		default:
			op = "execute_script"
			script = command
		}
		var raw map[string]json.RawMessage
		err = ac.call(ctx, "api.vendor_operation", map[string]any{"device_type": string(ac.deviceType), "operation": op, "script": script}, &raw)
		if err == nil {
			_ = json.Unmarshal(raw["output"], &output)
			if output == "" && len(raw["data"]) > 0 {
				var text string
				if json.Unmarshal(raw["data"], &text) == nil {
					output = text
				} else {
					output = string(raw["data"])
				}
			}
		}
	}
	if err != nil {
		return ac.CreateCommandResult(ac.Config.Host, command, "", err.Error(), -1, time.Since(start)), nil
	}
	return ac.CreateCommandResult(ac.Config.Host, command, output, "", 0, time.Since(start)), nil
}

func (ac *APIConnector) GetDeviceInventory(ctx context.Context) ([]*base.DeviceInfo, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	var raw []map[string]json.RawMessage
	if err := ac.call(ctx, "api.inventory", map[string]any{"device_type": string(ac.deviceType)}, &raw); err != nil {
		return nil, err
	}
	out := make([]*base.DeviceInfo, 0, len(raw))
	for _, item := range raw {
		var info base.DeviceInfo
		if err := json.Unmarshal(mustJSON(item), &info); err != nil {
			return nil, err
		}
		out = append(out, &info)
	}
	return out, nil
}
func (ac *APIConnector) config(ctx context.Context, deviceID, configType string) (base.ConfigResult, error) {
	var c base.ConfigResult
	err := ac.call(ctx, "api.config", map[string]any{"device_type": string(ac.deviceType), "device_id": deviceID, "config_type": configType}, &c)
	return c, err
}
func (ac *APIConnector) GetDeviceConfigs(ctx context.Context, deviceIDs []string) (map[string]*base.ConfigResult, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	out := make(map[string]*base.ConfigResult, len(deviceIDs))
	for _, id := range deviceIDs {
		c, err := ac.config(ctx, id, "running")
		if err != nil {
			out[id] = &base.ConfigResult{DeviceID: id, ConfigType: "running", RetrievedAt: time.Now()}
			continue
		}
		out[id] = &c
	}
	return out, nil
}
func (ac *APIConnector) SendCommandsToDevice(ctx context.Context, deviceID string, commands []string, timeout time.Duration) ([]*base.CommandResult, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	out := make([]*base.CommandResult, len(commands))
	for i, command := range commands {
		r, err := ac.ExecuteCommand(ctx, command, timeout)
		if err != nil {
			out[i] = ac.CreateCommandResult(deviceID, command, "", err.Error(), -1, 0)
			continue
		}
		r.DeviceID = deviceID
		out[i] = r
	}
	return out, nil
}
func (ac *APIConnector) GetRunningConfig(ctx context.Context, timeout time.Duration) (*base.ConfigResult, error) {
	return ac.getConfig(ctx, "running", timeout)
}
func (ac *APIConnector) GetStartupConfig(ctx context.Context, timeout time.Duration) (*base.ConfigResult, error) {
	t := "startup"
	if ac.deviceType == types.DeviceTypePaloAltoPanorama {
		t = "candidate"
	}
	return ac.getConfig(ctx, t, timeout)
}
func (ac *APIConnector) getConfig(ctx context.Context, kind string, timeout time.Duration) (*base.ConfigResult, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c, err := ac.config(ctx, ac.Config.Host, kind)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
func (ac *APIConnector) ExecuteCommands(ctx context.Context, commands []string, timeout time.Duration) ([]*base.CommandResult, error) {
	out := make([]*base.CommandResult, 0, len(commands))
	for _, command := range commands {
		r, err := ac.ExecuteCommand(ctx, command, timeout)
		if err != nil {
			r = ac.CreateCommandResult(ac.Config.Host, command, "", err.Error(), -1, 0)
		}
		out = append(out, r)
	}
	return out, nil
}
func (ac *APIConnector) GetManagedDevices(ctx context.Context) ([]*base.DeviceInfo, error) {
	return ac.GetDeviceInventory(ctx)
}
func (ac *APIConnector) CollectDeviceConfigs(ctx context.Context, deviceIDs []string, workingFolder string) (map[string][]string, error) {
	if workingFolder != "" {
		if err := ac.SetWorkingFolder(workingFolder); err != nil {
			return nil, err
		}
	}
	out := map[string][]string{}
	for _, id := range deviceIDs {
		c, err := ac.config(ctx, id, "running")
		if err != nil {
			return out, fmt.Errorf("failed to get config for %s: %w", id, err)
		}
		path, err := writeConfigFile(ctx, ac.GetWorkingFolder(), filepath.Base(id)+"_running_config.txt", c.ConfigData)
		if err != nil {
			return out, err
		}
		out[id] = []string{path}
	}
	return out, nil
}
func (ac *APIConnector) WriteConfigToFile(ctx context.Context, configData, filePath string) error {
	_, err := writeConfigFile(ctx, filepath.Dir(filePath), filepath.Base(filePath), configData)
	return err
}
func writeConfigFile(ctx context.Context, folder, fileName, content string) (string, error) {
	var result struct {
		Path string `json:"path"`
	}
	err := rustengine.Call(ctx, "connector.write_config", map[string]any{
		"working_folder": folder,
		"file_name":      fileName,
		"content":        content,
	}, &result)
	if err != nil {
		return "", err
	}
	if result.Path == "" {
		return "", rustengine.ErrUnavailable
	}
	return result.Path, nil
}
func (ac *APIConnector) GetConfigFilePath(fileName string) string {
	folder := ac.GetWorkingFolder()
	if folder == "" {
		return ""
	}
	return filepath.Join(folder, fileName)
}
func (ac *APIConnector) SetWorkingFolder(folder string) error {
	return ac.BaseConnector.SetWorkingFolder(folder)
}
func (ac *APIConnector) GetWorkingFolder() string { return ac.BaseConnector.GetWorkingFolder() }
func (ac *APIConnector) GetRequestCount() uint64 {
	ac.requestMutex.RLock()
	defer ac.requestMutex.RUnlock()
	return ac.requestCount
}

func (ac *APIConnector) doRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	query := map[string]any{}
	for k, v := range parsed.Query() {
		if len(v) == 1 {
			query[k] = v[0]
		} else {
			query[k] = v
		}
	}
	var payload *string
	if body != nil {
		b, e := readRequestBody(body)
		if e != nil {
			return nil, e
		}
		encoded := base64.StdEncoding.EncodeToString(b)
		payload = &encoded
	}
	var result struct {
		Status     int                 `json:"status"`
		Headers    map[string][]string `json:"headers"`
		BodyBase64 *string             `json:"body_base64"`
	}
	ac.requestMutex.Lock()
	ac.requestCount++
	ac.requestMutex.Unlock()
	if err := ac.call(ctx, "api.request", map[string]any{"method": method, "path": parsed.EscapedPath(), "query": query, "body_base64": payload, "raw_response": true}, &result); err != nil {
		return nil, err
	}
	if result.Status < 100 || result.Status > 599 || result.BodyBase64 == nil {
		return nil, rustengine.ErrUnavailable
	}
	responseBody, err := base64.StdEncoding.DecodeString(*result.BodyBase64)
	if err != nil || int64(len(responseBody)) > maxAPIRequestBodyBytes {
		return nil, rustengine.ErrUnavailable
	}
	header := make(http.Header)
	for k, values := range result.Headers {
		for _, value := range values {
			header.Add(k, value)
		}
	}
	return &http.Response{StatusCode: result.Status, Header: header, Body: io.NopCloser(bytes.NewReader(responseBody))}, nil
}

func readRequestBody(body io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(body, maxAPIRequestBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("request body read failed")
	}
	if int64(len(b)) > maxAPIRequestBodyBytes {
		return nil, fmt.Errorf("request body exceeds size limit")
	}
	return b, nil
}
func (ac *APIConnector) generateDeviceID() string {
	return fmt.Sprintf("api_%s_%d", ac.Config.Host, ac.Config.Port)
}
func mustJSON(value any) []byte { raw, _ := json.Marshal(value); return raw }
