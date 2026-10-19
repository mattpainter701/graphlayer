package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mattpainter701/graphlayer/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/orchestration/types"
	"github.com/mattpainter701/graphlayer/platform/rustengine"
)

type SSHConnector struct {
	*base.BaseConnector
	deviceType      types.DeviceType
	worker          *rustengine.Session
	workerSessionID string
	workerConfig    map[string]any
	sessionMutex    sync.Mutex
	shell           *InteractiveShell
}

type InteractiveShell struct {
	worker          *rustengine.Session
	workerSessionID string
	mutex           sync.Mutex
}

const maxSSHPrivateKeyBytes int64 = 1 << 20

func NewSSHConnector(config *base.ConnectionConfig, deviceType types.DeviceType) (*SSHConnector, error) {
	if config == nil {
		return nil, fmt.Errorf("connection configuration is nil")
	}
	bc := base.NewBaseConnector(config)
	if err := bc.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return &SSHConnector{BaseConnector: bc, deviceType: deviceType}, nil
}

func (sc *SSHConnector) Connect(ctx context.Context) error {
	sc.sessionMutex.Lock()
	defer sc.sessionMutex.Unlock()
	if sc.IsConnected() {
		return nil
	}
	if err := sc.BaseConnector.Connect(ctx); err != nil {
		return err
	}
	sc.SetConnectionState(base.ConnectionStateConnecting)
	privateKey := ""
	if sc.Config.KeyFile != "" {
		data, err := readPrivateKey(sc.Config.KeyFile)
		if err != nil {
			sc.SetConnectionState(base.ConnectionStateError)
			return err
		}
		privateKey = string(data)
	}
	knownHosts := ""
	if sc.Config.Options != nil {
		knownHosts, _ = sc.Config.Options["known_hosts_path"].(string)
	}
	config := map[string]any{"host": sc.Config.Host, "port": sc.Config.Port, "username": sc.Config.Username, "password": sc.Config.Password, "private_key": privateKey, "private_key_passphrase": sc.Config.KeyPassword, "known_hosts_path": knownHosts, "timeout_seconds": sc.Config.Timeout.Seconds()}
	worker, err := rustengine.NewSession()
	if err != nil {
		sc.SetConnectionState(base.ConnectionStateError)
		return err
	}
	sid := sc.generateDeviceID()
	var result struct {
		SessionID string `json:"session_id"`
		Connected bool   `json:"connected"`
	}
	if err = worker.Call(ctx, "ssh.connect", map[string]any{"session_id": sid, "config": config}, &result); err != nil {
		_ = worker.Close()
		sc.SetConnectionState(base.ConnectionStateError)
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}
	if !result.Connected {
		_ = worker.Close()
		sc.SetConnectionState(base.ConnectionStateError)
		return fmt.Errorf("SSH worker did not establish a session")
	}
	sc.worker = worker
	sc.workerSessionID = sid
	if result.SessionID != "" {
		sc.workerSessionID = result.SessionID
	}
	sc.workerConfig = config
	sc.SetConnected(true)
	sc.SetConnectionState(base.ConnectionStateConnected)
	return nil
}

func readPrivateKey(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH private key")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSSHPrivateKeyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH private key")
	}
	if int64(len(data)) > maxSSHPrivateKeyBytes {
		return nil, fmt.Errorf("SSH private key exceeds size limit")
	}
	return data, nil
}

func (sc *SSHConnector) InvokeShell(ctx context.Context) (*InteractiveShell, error) {
	if !sc.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	var result struct {
		Connected bool `json:"connected"`
	}
	if err := sc.call(ctx, "ssh.open_shell", map[string]any{"config": sc.workerConfig}, &result); err != nil {
		return nil, err
	}
	if !result.Connected {
		return nil, fmt.Errorf("SSH shell did not open")
	}
	shell := &InteractiveShell{worker: sc.worker, workerSessionID: sc.workerSessionID}
	sc.shell = shell
	return shell, nil
}

func (shell *InteractiveShell) SendCommand(command string, timeout time.Duration) (string, error) {
	if shell == nil || shell.worker == nil {
		return "", rustengine.ErrUnavailable
	}
	shell.mutex.Lock()
	defer shell.mutex.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var raw map[string]json.RawMessage
	if err := shell.worker.Call(ctx, "ssh.shell_execute", map[string]any{"session_id": shell.workerSessionID, "command": command, "timeout_seconds": timeout.Seconds()}, &raw); err != nil {
		return "", err
	}
	var stdout, stderr string
	var status int
	_ = json.Unmarshal(raw["stdout"], &stdout)
	_ = json.Unmarshal(raw["stderr"], &stderr)
	_ = json.Unmarshal(raw["exit_status"], &status)
	if status != 0 {
		return stdout + stderr, fmt.Errorf("command exited with status %d", status)
	}
	return stdout + stderr, nil
}
func (shell *InteractiveShell) Close() error {
	if shell == nil || shell.worker == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out map[string]json.RawMessage
	return shell.worker.Call(ctx, "ssh.close_shell", map[string]any{"session_id": shell.workerSessionID}, &out)
}

func (sc *SSHConnector) call(ctx context.Context, operation string, input map[string]any, out any) error {
	if sc.worker == nil {
		return rustengine.ErrUnavailable
	}
	input["session_id"] = sc.workerSessionID
	return sc.worker.Call(ctx, operation, input, out)
}
func (sc *SSHConnector) Disconnect(ctx context.Context) error {
	sc.sessionMutex.Lock()
	defer sc.sessionMutex.Unlock()
	var disconnectErr error
	if sc.worker != nil {
		var out map[string]json.RawMessage
		disconnectErr = sc.call(ctx, "ssh.disconnect", map[string]any{}, &out)
		if closeErr := sc.worker.Close(); disconnectErr == nil {
			disconnectErr = closeErr
		}
	}
	sc.worker = nil
	sc.workerSessionID = ""
	sc.workerConfig = nil
	sc.shell = nil
	sc.SetConnected(false)
	sc.SetConnectionState(base.ConnectionStateDisconnected)
	return disconnectErr
}
func (sc *SSHConnector) IsConnected() bool { return sc.BaseConnector.IsConnected() && sc.worker != nil }
func (sc *SSHConnector) GetType() string   { return "ssh" }

func (sc *SSHConnector) ExecuteCommand(ctx context.Context, command string, timeout time.Duration) (*base.CommandResult, error) {
	if !sc.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	op := "ssh.execute"
	if sc.shell != nil {
		op = "ssh.shell_execute"
	}
	var raw map[string]json.RawMessage
	if err := sc.call(ctx, op, map[string]any{"command": command, "timeout_seconds": timeout.Seconds()}, &raw); err != nil {
		return sc.CreateCommandResult(sc.Config.Host, command, "", err.Error(), -1, time.Since(start)), nil
	}
	var stdout, stderr string
	var status int
	_ = json.Unmarshal(raw["stdout"], &stdout)
	_ = json.Unmarshal(raw["stderr"], &stderr)
	_ = json.Unmarshal(raw["exit_status"], &status)
	output := stdout + stderr
	var parsed string
	if err := rustengine.Evaluate("ssh.parse_output", map[string]any{"output": output}, &parsed); err != nil {
		return sc.CreateCommandResult(sc.Config.Host, command, "", err.Error(), -1, time.Since(start)), nil
	}
	output = parsed
	errorText := ""
	exit := status
	if status != 0 {
		errorText = fmt.Sprintf("command exited with status %d", status)
		exit = -1
	}
	return sc.CreateCommandResult(sc.Config.Host, command, output, errorText, exit, time.Since(start)), nil
}

func (sc *SSHConnector) GetDeviceInfo(ctx context.Context) (*base.DeviceInfo, error) {
	if !sc.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	var raw map[string]json.RawMessage
	if err := sc.call(ctx, "ssh.device_info", map[string]any{"device_type": string(sc.deviceType)}, &raw); err != nil {
		return nil, err
	}
	data, _ := json.Marshal(raw)
	var info base.DeviceInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
func (sc *SSHConnector) GetSupportedCommands(ctx context.Context) ([]string, error) {
	var commands []string
	if err := rustengine.Evaluate("ssh.supported_commands", map[string]any{"device_type": string(sc.deviceType)}, &commands); err != nil {
		return nil, err
	}
	return commands, nil
}
func (sc *SSHConnector) Ping(ctx context.Context, timeout time.Duration) error {
	if !sc.IsConnected() {
		return fmt.Errorf("not connected")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var raw map[string]json.RawMessage
	err := sc.call(ctx, "ssh.execute", map[string]any{"command": "show version", "timeout_seconds": timeout.Seconds()}, &raw)
	if err != nil {
		return err
	}
	var status int
	_ = json.Unmarshal(raw["exit_status"], &status)
	if status != 0 {
		return fmt.Errorf("command exited with status %d", status)
	}
	return nil
}

func (sc *SSHConnector) getConfig(ctx context.Context, kind string, timeout time.Duration) (*base.ConfigResult, error) {
	if !sc.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	var raw map[string]json.RawMessage
	if err := sc.call(ctx, "ssh.get_config", map[string]any{"device_type": string(sc.deviceType), "config_type": kind, "timeout_seconds": timeout.Seconds()}, &raw); err != nil {
		return nil, err
	}
	var data string
	_ = json.Unmarshal(raw["config"], &data)
	var status int
	_ = json.Unmarshal(raw["exit_status"], &status)
	if status != 0 {
		return nil, fmt.Errorf("configuration command exited with status %d", status)
	}
	return sc.CreateConfigResult(sc.Config.Host, kind, data, time.Since(start)), nil
}
func (sc *SSHConnector) GetRunningConfig(ctx context.Context, timeout time.Duration) (*base.ConfigResult, error) {
	return sc.getConfig(ctx, "running", timeout)
}
func (sc *SSHConnector) GetStartupConfig(ctx context.Context, timeout time.Duration) (*base.ConfigResult, error) {
	return sc.getConfig(ctx, "startup", timeout)
}
func (sc *SSHConnector) WriteConfigToFile(ctx context.Context, configType, filePath string) error {
	var c *base.ConfigResult
	var err error
	switch configType {
	case "running":
		c, err = sc.GetRunningConfig(ctx, 60*time.Second)
	case "startup":
		c, err = sc.GetStartupConfig(ctx, 60*time.Second)
	default:
		return fmt.Errorf("unsupported config type: %s", configType)
	}
	if err != nil {
		return err
	}
	_, err = writeConfigFile(ctx, filepath.Dir(filePath), filepath.Base(filePath), c.ConfigData)
	return err
}
func (sc *SSHConnector) GetConfigFilePath(configType string) string {
	folder := sc.GetWorkingFolder()
	if folder == "" {
		return ""
	}
	return filepath.Join(folder, fmt.Sprintf("%s_%s.txt", sc.Config.Host, configType))
}
func (sc *SSHConnector) GetManagedDevices(ctx context.Context) ([]*base.DeviceInfo, error) {
	device, err := sc.GetDeviceInfo(ctx)
	if err != nil {
		return nil, err
	}
	return []*base.DeviceInfo{device}, nil
}
func (sc *SSHConnector) CollectDeviceConfigs(ctx context.Context, deviceIDs []string, workingFolder string) (map[string][]string, error) {
	if workingFolder != "" {
		if err := sc.SetWorkingFolder(workingFolder); err != nil {
			return nil, err
		}
	}
	device, err := sc.GetDeviceInfo(ctx)
	if err != nil {
		return nil, err
	}
	running, err := sc.GetRunningConfig(ctx, 60*time.Second)
	if err != nil {
		return map[string][]string{}, fmt.Errorf("failed to get running config: %w", err)
	}
	folder := sc.GetWorkingFolder()
	if folder == "" {
		return nil, fmt.Errorf("working folder is required")
	}
	runPath, err := writeConfigFile(ctx, folder, device.Hostname+"_running_config.txt", running.ConfigData)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{device.ID: {runPath}}
	if startup, e := sc.GetStartupConfig(ctx, 60*time.Second); e == nil {
		path, writeErr := writeConfigFile(ctx, folder, device.Hostname+"_startup_config.txt", startup.ConfigData)
		if writeErr == nil {
			out[device.ID] = append(out[device.ID], path)
		}
	}
	return out, nil
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

func (sc *SSHConnector) generateDeviceID() string {
	return fmt.Sprintf("ssh_%s_%d", sc.Config.Host, sc.Config.Port)
}
func (sc *SSHConnector) parseOutput(output, command string) string {
	var parsed string
	if rustengine.Evaluate("ssh.parse_output", map[string]any{"output": output}, &parsed) != nil {
		return ""
	}
	return parsed
}
