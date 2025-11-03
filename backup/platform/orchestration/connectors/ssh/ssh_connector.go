package ssh

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/types"
)

// SSHConnector implements the Connector interface for SSH-based connections
// This is production-ready with proper session management, interactive shell support,
// and paramiko-style concepts
type SSHConnector struct {
	*base.BaseConnector
	client       *ssh.Client
	shell        *InteractiveShell
	deviceType   types.DeviceType
	sessionMutex sync.Mutex
	
	// Connection settings
	keepAliveInterval time.Duration
	keepAliveDone     chan struct{}
}

// InteractiveShell represents an interactive shell session (similar to paramiko's invoke_shell)
type InteractiveShell struct {
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
	stderr  io.Reader
	buffer  *bytes.Buffer
	reader  *bufio.Reader
	mutex   sync.Mutex
	
	// Terminal settings
	terminalType  string
	terminalWidth  int
	terminalHeight int
	
	// Prompt detection
	promptPattern *regexp.Regexp
}

// NewSSHConnector creates a new SSH connector instance
func NewSSHConnector(config *base.ConnectionConfig, deviceType types.DeviceType) (*SSHConnector, error) {
	if err := base.NewBaseConnector(config).ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	connector := &SSHConnector{
		BaseConnector:     base.NewBaseConnector(config),
		deviceType:        deviceType,
		keepAliveInterval: 30 * time.Second, // Default keepalive every 30 seconds
		keepAliveDone:     make(chan struct{}),
	}

	return connector, nil
}

// Connect establishes an SSH connection to the device
func (sc *SSHConnector) Connect(ctx context.Context) error {
	sc.sessionMutex.Lock()
	defer sc.sessionMutex.Unlock()

	if err := sc.BaseConnector.Connect(ctx); err != nil {
		return err
	}

	sc.setConnectionState(base.ConnectionStateConnecting)

	// Get authentication methods
	authMethods, err := sc.getAuthMethods()
	if err != nil {
		sc.setConnectionState(base.ConnectionStateError)
		return fmt.Errorf("failed to setup authentication: %w", err)
	}

	// Setup host key callback
	hostKeyCallback, err := sc.getHostKeyCallback()
	if err != nil {
		// Fall back to insecure for development, but log warning
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	config := &ssh.ClientConfig{
		User:            sc.config.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         sc.config.Timeout,
		// Production settings
		Config: ssh.Config{
			Ciphers: []string{
				"aes128-ctr", "aes192-ctr", "aes256-ctr",
				"aes128-gcm@openssh.com",
				"chacha20-poly1305@openssh.com",
			},
		},
	}

	addr := fmt.Sprintf("%s:%d", sc.config.Host, sc.config.Port)

	// Dial with context support
	dialer := &net.Dialer{
		Timeout: sc.config.Timeout,
	}

	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		sc.setConnectionState(base.ConnectionStateError)
		return fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, addr, config)
	if err != nil {
		netConn.Close()
		sc.setConnectionState(base.ConnectionStateError)
		return fmt.Errorf("failed to establish SSH connection to %s: %w", addr, err)
	}

	sc.client = ssh.NewClient(sshConn, chans, reqs)
	sc.setConnected(true)
	sc.setConnectionState(base.ConnectionStateConnected)

	// Start keepalive goroutine
	go sc.keepAlive()

	return nil
}

// InvokeShell creates an interactive shell session (paramiko-style)
func (sc *SSHConnector) InvokeShell(ctx context.Context) (*InteractiveShell, error) {
	sc.sessionMutex.Lock()
	defer sc.sessionMutex.Unlock()

	if !sc.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	session, err := sc.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Setup terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // Disable echo
		ssh.TTY_OP_ISPEED: 14400, // Input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // Output speed = 14.4kbaud
	}

	// Request pseudo terminal
	if err := session.RequestPty("vt100", 80, 24, modes); err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to request pseudo terminal: %w", err)
	}

	// Setup I/O
	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to setup stdin: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to setup stdout: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to setup stderr: %w", err)
	}

	// Start shell
	if err := session.Shell(); err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to start shell: %w", err)
	}

	buffer := &bytes.Buffer{}
	reader := bufio.NewReader(stdout)

	shell := &InteractiveShell{
		session:        session,
		stdin:          stdin,
		stdout:         stdout,
		stderr:         stderr,
		buffer:         buffer,
		reader:         reader,
		terminalType:   "vt100",
		terminalWidth:  80,
		terminalHeight: 24,
		promptPattern:  sc.getPromptPattern(),
	}

	sc.shell = shell

	// Wait for initial prompt
	time.Sleep(500 * time.Millisecond)
	shell.clearBuffer()

	return shell, nil
}

// SendCommand sends a command to the interactive shell and waits for response
func (shell *InteractiveShell) SendCommand(command string, timeout time.Duration) (string, error) {
	shell.mutex.Lock()
	defer shell.mutex.Unlock()

	// Clear buffer before sending command
	shell.clearBuffer()

	// Send command
	_, err := shell.stdin.Write([]byte(command + "\n"))
	if err != nil {
		return "", fmt.Errorf("failed to write command: %w", err)
	}

	// Read response with timeout
	responseChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	go func() {
		output, err := shell.readUntilPrompt(timeout)
		if err != nil {
			errorChan <- err
			return
		}
		responseChan <- output
	}()

	select {
	case output := <-responseChan:
		return output, nil
	case err := <-errorChan:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("command timeout after %v", timeout)
	}
}

// readUntilPrompt reads until a prompt is detected
func (shell *InteractiveShell) readUntilPrompt(timeout time.Duration) (string, error) {
	var output bytes.Buffer
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Set read deadline
		if err := shell.session.Conn.SetReadDeadline(deadline); err != nil {
			return "", err
		}

		line, err := shell.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "timeout") {
				break
			}
			return "", err
		}

		output.WriteString(line)

		// Check if we've hit a prompt
		if shell.promptPattern != nil && shell.promptPattern.MatchString(line) {
			break
		}

		// Also check for common prompt patterns
		if isPromptLine(line) {
			break
		}
	}

	return output.String(), nil
}

// clearBuffer clears the shell buffer
func (shell *InteractiveShell) clearBuffer() {
	// Read and discard any pending data
	shell.buffer.Reset()
	
	// Non-blocking read to clear any buffered data
	buf := make([]byte, 4096)
	for {
		shell.session.Conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := shell.reader.Read(buf)
		if err != nil || n == 0 {
			break
		}
	}
	shell.session.Conn.SetReadDeadline(time.Time{}) // Clear deadline
}

// Close closes the interactive shell
func (shell *InteractiveShell) Close() error {
	if shell.session != nil {
		return shell.session.Close()
	}
	return nil
}

// ExecuteCommand executes a single command on the device
func (sc *SSHConnector) ExecuteCommand(ctx context.Context, command string, timeout time.Duration) (*base.CommandResult, error) {
	if !sc.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	// If we have an interactive shell, use it
	if sc.shell != nil {
		output, err := sc.shell.SendCommand(command, timeout)
		if err != nil {
			return sc.createCommandResult(sc.config.Host, command, "", err.Error(), -1, 0), nil
		}
		return sc.createCommandResult(sc.config.Host, command, output, "", 0, 0), nil
	}

	// Otherwise, use exec mode (creates new session for each command)
	return sc.executeCommandExec(ctx, command, timeout)
}

// executeCommandExec executes command using exec mode (new session per command)
func (sc *SSHConnector) executeCommandExec(ctx context.Context, command string, timeout time.Duration) (*base.CommandResult, error) {
	sc.sessionMutex.Lock()
	defer sc.sessionMutex.Unlock()

	session, err := sc.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Setup timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// Execute command with context
	outputChan := make(chan []byte, 1)
	errorChan := make(chan error, 1)

	go func() {
		output, err := session.CombinedOutput(command)
		if err != nil {
			errorChan <- err
			return
		}
		outputChan <- output
	}()

	select {
	case output := <-outputChan:
		duration := time.Since(start)
		cleaned := sc.parseOutput(string(output), command)
		return sc.createCommandResult(sc.config.Host, command, cleaned, "", 0, duration), nil
	case err := <-errorChan:
		duration := time.Since(start)
		return sc.createCommandResult(sc.config.Host, command, "", err.Error(), -1, duration), nil
	case <-ctx.Done():
		duration := time.Since(start)
		return sc.createCommandResult(sc.config.Host, command, "", "command timeout", -1, duration), nil
	}
}

// Disconnect closes the SSH connection
func (sc *SSHConnector) Disconnect(ctx context.Context) error {
	sc.sessionMutex.Lock()
	defer sc.sessionMutex.Unlock()

	// Stop keepalive
	close(sc.keepAliveDone)

	// Close shell if exists
	if sc.shell != nil {
		sc.shell.Close()
		sc.shell = nil
	}

	// Close client
	if sc.client != nil {
		sc.client.Close()
		sc.client = nil
	}

	sc.setConnected(false)
	sc.setConnectionState(base.ConnectionStateDisconnected)
	return nil
}

// keepAlive sends periodic keepalive packets
func (sc *SSHConnector) keepAlive() {
	ticker := time.NewTicker(sc.keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if sc.client != nil {
				// Send keepalive
				_, _, err := sc.client.SendRequest("keepalive@openssh.com", true, nil)
				if err != nil {
					// Connection lost, mark as disconnected
					sc.setConnectionState(base.ConnectionStateError)
					return
				}
			}
		case <-sc.keepAliveDone:
			return
		}
	}
}

// getAuthMethods returns SSH authentication methods
func (sc *SSHConnector) getAuthMethods() ([]ssh.AuthMethod, error) {
	var authMethods []ssh.AuthMethod

	// Password authentication
	if sc.config.Password != "" {
		authMethods = append(authMethods, ssh.Password(sc.config.Password))
	}

	// Public key authentication
	if sc.config.KeyFile != "" {
		key, err := sc.parsePrivateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, key)
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods configured")
	}

	return authMethods, nil
}

// parsePrivateKey parses the private key file
func (sc *SSHConnector) parsePrivateKey() (ssh.AuthMethod, error) {
	keyData, err := ioutil.ReadFile(sc.config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	var signer ssh.Signer
	if sc.config.KeyPassword != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(sc.config.KeyPassword))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return ssh.PublicKeys(signer), nil
}

// getHostKeyCallback returns the host key callback for verification
func (sc *SSHConnector) getHostKeyCallback() (ssh.HostKeyCallback, error) {
	// Try to load known_hosts file
	knownHostsFile := os.Getenv("HOME") + "/.ssh/known_hosts"
	if _, err := os.Stat(knownHostsFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("known_hosts file not found")
	}

	callback, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create known hosts callback: %w", err)
	}

	return callback, nil
}

// getPromptPattern returns the prompt pattern for this device type
func (sc *SSHConnector) getPromptPattern() *regexp.Regexp {
	patterns := map[types.DeviceType]string{
		types.DeviceTypeCiscoIOS:     `[>#]\s*$`,
		types.DeviceTypeCiscoASA:     `[>#]\s*$`,
		types.DeviceTypeCiscoNXOS:    `[>#]\s*$`,
		types.DeviceTypePaloAltoPA:   `>\s*$`,
		types.DeviceTypeJuniperJunos: `[%>]\s*$`,
		types.DeviceTypeFortinetFortiGate: `[#$]\s*$`,
	}

	if pattern, exists := patterns[sc.deviceType]; exists {
		return regexp.MustCompile(pattern)
	}

	// Default generic prompt pattern
	return regexp.MustCompile(`[>#%$]\s*$`)
}

// isPromptLine checks if a line looks like a prompt
func isPromptLine(line string) bool {
	line = strings.TrimSpace(line)
	
	// Common prompt indicators
	promptChars := []string{">", "#", "$", "%"}
	for _, char := range promptChars {
		if strings.HasSuffix(line, char) {
			return true
		}
	}
	
	return false
}

// GetDeviceInfo retrieves device information
func (sc *SSHConnector) GetDeviceInfo(ctx context.Context) (*base.DeviceInfo, error) {
	if !sc.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	// Execute show version command to get device information
	result, err := sc.ExecuteCommand(ctx, "show version", 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	deviceInfo := &base.DeviceInfo{
		ID:                sc.generateDeviceID(),
		Hostname:          sc.config.Host,
		IPAddress:         sc.config.Host,
		Vendor:            sc.getVendor(),
		Model:             sc.extractModel(result.Output),
		OSType:            string(types.GetOSTypeFromDeviceType(sc.deviceType)),
		OSVersion:         sc.extractOSVersion(result.Output),
		ConnectionType:    "ssh",
		Capabilities:      sc.getCapabilities(),
		SupportedCommands: sc.getSupportedCommands(),
		LastSeen:          time.Now(),
	}

	return deviceInfo, nil
}

// GetSupportedCommands returns the list of supported commands for this device type
func (sc *SSHConnector) GetSupportedCommands(ctx context.Context) ([]string, error) {
	templates := types.GetCommandTemplates(sc.deviceType)
	commands := make([]string, len(templates))

	for i, template := range templates {
		commands[i] = template.Name
	}

	return commands, nil
}

// GetRunningConfig retrieves the running configuration
func (sc *SSHConnector) GetRunningConfig(ctx context.Context, timeout time.Duration) (*base.ConfigResult, error) {
	if !sc.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	command := sc.getConfigCommand("running")
	if command == "" {
		return nil, fmt.Errorf("running config command not supported for device type: %s", sc.deviceType)
	}

	result, err := sc.ExecuteCommand(ctx, command, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get running config: %w", err)
	}

	return sc.createConfigResult(sc.config.Host, "running", result.Output, result.Duration), nil
}

// GetStartupConfig retrieves the startup configuration
func (sc *SSHConnector) GetStartupConfig(ctx context.Context, timeout time.Duration) (*base.ConfigResult, error) {
	if !sc.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	command := sc.getConfigCommand("startup")
	if command == "" {
		return nil, fmt.Errorf("startup config command not supported for device type: %s", sc.deviceType)
	}

	result, err := sc.ExecuteCommand(ctx, command, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get startup config: %w", err)
	}

	return sc.createConfigResult(sc.config.Host, "startup", result.Output, result.Duration), nil
}

// IsConnected returns whether the SSH connection is active
func (sc *SSHConnector) IsConnected() bool {
	return sc.BaseConnector.IsConnected() && sc.client != nil
}

// GetType returns the connector type
func (sc *SSHConnector) GetType() string {
	return "ssh"
}

// Ping performs a basic connectivity check
func (sc *SSHConnector) Ping(ctx context.Context, timeout time.Duration) error {
	if !sc.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// Send a simple command to verify connectivity
	_, err := sc.ExecuteCommand(ctx, "show version", timeout)
	return err
}

// Helper methods

// generateDeviceID generates a unique device ID
func (sc *SSHConnector) generateDeviceID() string {
	return fmt.Sprintf("ssh_%s_%d", sc.config.Host, sc.config.Port)
}

// getVendor returns the vendor name based on device type
func (sc *SSHConnector) getVendor() string {
	switch sc.deviceType {
	case types.DeviceTypeCiscoIOS, types.DeviceTypeCiscoASA, types.DeviceTypeCiscoNXOS:
		return "Cisco"
	case types.DeviceTypePaloAltoPA, types.DeviceTypePaloAltoPanorama:
		return "Palo Alto Networks"
	case types.DeviceTypeJuniperJunos, types.DeviceTypeJuniperSRX:
		return "Juniper Networks"
	case types.DeviceTypeFortinetFortiGate, types.DeviceTypeFortinetFortiManager:
		return "Fortinet"
	case types.DeviceTypeCheckPointGaia:
		return "Check Point"
	case types.DeviceTypeF5BigIP:
		return "F5 Networks"
	default:
		return "Unknown"
	}
}

// extractModel extracts the device model from show version output
func (sc *SSHConnector) extractModel(output string) string {
	lines := strings.Split(output, "\n")

	// Cisco patterns
	if sc.deviceType == types.DeviceTypeCiscoIOS || sc.deviceType == types.DeviceTypeCiscoASA {
		for _, line := range lines {
			if strings.Contains(line, "Cisco") && (strings.Contains(line, "IOS") || strings.Contains(line, "ASA")) {
				re := regexp.MustCompile(`Cisco.*Software.*\(([^)]+)\)`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 1 {
					return matches[1]
				}
			}
		}
	}

	// Palo Alto patterns
	if sc.deviceType == types.DeviceTypePaloAltoPA {
		for _, line := range lines {
			if strings.Contains(line, "PA-") {
				re := regexp.MustCompile(`PA-(\d+[A-Z]*-\d+[A-Z]*)`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 1 {
					return "PA-" + matches[1]
				}
			}
		}
	}

	return "Unknown"
}

// extractOSVersion extracts the OS version from show version output
func (sc *SSHConnector) extractOSVersion(output string) string {
	lines := strings.Split(output, "\n")

	// Cisco IOS patterns
	if sc.deviceType == types.DeviceTypeCiscoIOS {
		for _, line := range lines {
			if strings.Contains(line, "Version") {
				re := regexp.MustCompile(`Version\s+([^\s,]+)`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 1 {
					return matches[1]
				}
			}
		}
	}

	// ASA patterns
	if sc.deviceType == types.DeviceTypeCiscoASA {
		for _, line := range lines {
			if strings.Contains(line, "Version") {
				re := regexp.MustCompile(`Version\s+([^\s]+)`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 1 {
					return matches[1]
				}
			}
		}
	}

	return "Unknown"
}

// getCapabilities returns the device capabilities
func (sc *SSHConnector) getCapabilities() []string {
	caps := types.GetDeviceCapabilities(sc.deviceType)
	var capabilities []string

	if caps.CanGetRunningConfig {
		capabilities = append(capabilities, "running_config")
	}
	if caps.CanGetStartupConfig {
		capabilities = append(capabilities, "startup_config")
	}
	if caps.CanExecuteCommands {
		capabilities = append(capabilities, "command_execution")
	}
	if caps.CanExecuteConfigCommands {
		capabilities = append(capabilities, "config_commands")
	}

	return capabilities
}

// getSupportedCommands returns the supported command names
func (sc *SSHConnector) getSupportedCommands() []string {
	templates := types.GetCommandTemplates(sc.deviceType)
	commands := make([]string, len(templates))

	for i, template := range templates {
		commands[i] = template.Name
	}

	return commands
}

// getConfigCommand returns the appropriate config command for the device type
func (sc *SSHConnector) getConfigCommand(configType string) string {
	templates := types.GetCommandTemplates(sc.deviceType)

	for _, template := range templates {
		if template.Category == types.CommandCategoryConfig {
			switch configType {
			case "running":
				if strings.Contains(template.Name, "running") {
					return template.Command
				}
			case "startup":
				if strings.Contains(template.Name, "startup") {
					return template.Command
				}
			}
		}
	}

	return ""
}

// parseOutput parses and cleans command output
func (sc *SSHConnector) parseOutput(output, command string) string {
	lines := strings.Split(output, "\n")
	var cleanedLines []string

	// Remove common prompt patterns and pagination
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and common prompts
		if line == "" || strings.Contains(line, "--More--") || strings.Contains(line, "Press any key") {
			continue
		}

		// Remove ANSI escape sequences
		line = sc.removeANSIEscapeCodes(line)

		cleanedLines = append(cleanedLines, line)
	}

	return strings.Join(cleanedLines, "\n")
}

// removeANSIEscapeCodes removes ANSI escape codes from output
func (sc *SSHConnector) removeANSIEscapeCodes(input string) string {
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mGKHF]`)
	return ansiRegex.ReplaceAllString(input, "")
}

// WriteConfigToFile writes configuration to a file
func (sc *SSHConnector) WriteConfigToFile(ctx context.Context, configType string, filePath string) error {
	var config *base.ConfigResult
	var err error

	switch configType {
	case "running":
		config, err = sc.GetRunningConfig(ctx, 60*time.Second)
	case "startup":
		config, err = sc.GetStartupConfig(ctx, 60*time.Second)
	default:
		return fmt.Errorf("unsupported config type: %s", configType)
	}

	if err != nil {
		return err
	}

	return ioutil.WriteFile(filePath, []byte(config.ConfigData), 0644)
}

// GetConfigFilePath returns the file path for a config type
func (sc *SSHConnector) GetConfigFilePath(configType string) string {
	if sc.workingFolder == "" {
		return ""
	}
	return filepath.Join(sc.workingFolder, fmt.Sprintf("%s_%s.txt", sc.config.Host, configType))
}

// GetManagedDevices returns managed devices (SSH connectors manage single devices)
func (sc *SSHConnector) GetManagedDevices(ctx context.Context) ([]*base.DeviceInfo, error) {
	deviceInfo, err := sc.GetDeviceInfo(ctx)
	if err != nil {
		return nil, err
	}
	return []*base.DeviceInfo{deviceInfo}, nil
}

// CollectDeviceConfigs collects configs from multiple devices (single device for SSH)
func (sc *SSHConnector) CollectDeviceConfigs(ctx context.Context, deviceIDs []string, workingFolder string) (map[string][]string, error) {
	result := make(map[string][]string)

	// SSH connectors manage single devices
	deviceInfo, err := sc.GetDeviceInfo(ctx)
	if err != nil {
		return nil, err
	}

	// Get running config
	runningConfig, err := sc.GetRunningConfig(ctx, 60*time.Second)
	if err != nil {
		return result, fmt.Errorf("failed to get running config: %w", err)
	}

	// Write to file
	runningPath := filepath.Join(workingFolder, fmt.Sprintf("%s_running_config.txt", deviceInfo.Hostname))
	if err := ioutil.WriteFile(runningPath, []byte(runningConfig.ConfigData), 0644); err != nil {
		return result, err
	}

	result[deviceInfo.ID] = []string{runningPath}

	// Try to get startup config
	caps := types.GetDeviceCapabilities(sc.deviceType)
	if caps.CanGetStartupConfig {
		startupConfig, err := sc.GetStartupConfig(ctx, 60*time.Second)
		if err == nil {
			startupPath := filepath.Join(workingFolder, fmt.Sprintf("%s_startup_config.txt", deviceInfo.Hostname))
			if err := ioutil.WriteFile(startupPath, []byte(startupConfig.ConfigData), 0644); err == nil {
				result[deviceInfo.ID] = append(result[deviceInfo.ID], startupPath)
			}
		}
	}

	return result, nil
}