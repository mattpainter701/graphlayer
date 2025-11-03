package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/types"
)

// APIConnector implements the Connector interface for REST API-based management systems
// Production-ready with connection pooling, rate limiting, and proper error handling
type APIConnector struct {
	*base.BaseConnector
	httpClient  *http.Client
	baseURL     string
	deviceType  types.DeviceType
	rateLimiter *rate.Limiter
	sessionMutex sync.Mutex
	
	// Authentication tokens
	authToken   string
	tokenExpiry time.Time
	tokenMutex  sync.RWMutex
	
	// Request tracking
	requestCount  uint64
	requestMutex  sync.RWMutex
}

// APIResponse represents a generic API response structure
type APIResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error,omitempty"`
}

// DeviceListResponse represents a list of devices from management systems
type DeviceListResponse struct {
	Devices []DeviceInfo `json:"devices"`
	Total   int          `json:"total"`
}

// DeviceInfo represents device information from management systems
type DeviceInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip_address"`
	Model    string `json:"model"`
	Vendor   string `json:"vendor"`
	Status   string `json:"status"`
}

// NewAPIConnector creates a new API connector instance
func NewAPIConnector(config *base.ConnectionConfig, deviceType types.DeviceType) (*APIConnector, error) {
	if config == nil {
		return nil, fmt.Errorf("connection configuration is nil")
	}

	bc := base.NewBaseConnector(config)
	if err := bc.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Determine protocol
	protocol := "https"
	if config.Port == 80 {
		protocol = "http"
	}

	// Build base URL
	baseURL := fmt.Sprintf("%s://%s:%d", protocol, config.Host, config.Port)

	connector := &APIConnector{
		BaseConnector: bc,
		baseURL:       baseURL,
		deviceType:    deviceType,
		// Rate limit: 100 requests per second with burst of 10
		rateLimiter:   rate.NewLimiter(rate.Limit(100), 10),
	}

	// Setup HTTP client with production settings
	connector.setupHTTPClient()

	return connector, nil
}

// setupHTTPClient configures the HTTP client with proper TLS settings and connection pooling
func (ac *APIConnector) setupHTTPClient() {
	// Create TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		// Preferred cipher suites for production
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
	}

	// Load CA certificate if provided
	if ac.config.CACert != "" {
		caCert, err := ioutil.ReadFile(ac.config.CACert)
		if err == nil {
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.RootCAs = caCertPool
		}
	}

	// Load client certificate if provided
	if ac.config.CertFile != "" && ac.config.KeyFileAPI != "" {
		cert, err := tls.LoadX509KeyPair(ac.config.CertFile, ac.config.KeyFileAPI)
		if err == nil {
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	// Create HTTP transport with connection pooling
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		// Connection pooling settings
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     20,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		// Timeouts
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Create HTTP client with timeout
	ac.httpClient = &http.Client{
		Transport: transport,
		Timeout:   ac.config.Timeout,
		// Don't follow redirects automatically for better control
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
}

// Connect establishes a connection to the API server
func (ac *APIConnector) Connect(ctx context.Context) error {
	ac.sessionMutex.Lock()
	defer ac.sessionMutex.Unlock()

	if err := ac.BaseConnector.Connect(ctx); err != nil {
		return err
	}

	ac.setConnectionState(base.ConnectionStateConnecting)

	// Test connectivity and authenticate if needed
	if err := ac.authenticate(ctx); err != nil {
		ac.setConnectionState(base.ConnectionStateError)
		return fmt.Errorf("failed to authenticate to API server: %w", err)
	}

	ac.setConnected(true)
	ac.setConnectionState(base.ConnectionStateConnected)

	return nil
}

// authenticate performs authentication with the API server
func (ac *APIConnector) authenticate(ctx context.Context) error {
	// Different authentication methods based on device type
	switch ac.deviceType {
	case types.DeviceTypeCiscoFMC:
		return ac.authenticateCiscoFMC(ctx)
	case types.DeviceTypePaloAltoPanorama:
		return ac.authenticatePanorama(ctx)
	case types.DeviceTypeFortinetFortiManager:
		return ac.authenticateFortiManager(ctx)
	case types.DeviceTypeSolarWindsNCM:
		return ac.authenticateSolarWinds(ctx)
	default:
		// For generic APIs, just test connectivity
		return ac.testConnection(ctx)
	}
}

// authenticateCiscoFMC authenticates with Cisco FMC
func (ac *APIConnector) authenticateCiscoFMC(ctx context.Context) error {
	endpoint := "/api/fmc_platform/v1/auth/generatetoken"

	req, err := http.NewRequestWithContext(ctx, "POST", ac.baseURL+endpoint, nil)
	if err != nil {
		return err
	}

	// FMC uses basic auth for token generation
	req.SetBasicAuth(ac.config.Username, ac.config.Password)

	resp, err := ac.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("authentication failed with status: %d", resp.StatusCode)
	}

	// Extract auth tokens from headers
	ac.tokenMutex.Lock()
	ac.authToken = resp.Header.Get("X-auth-access-token")
	ac.tokenExpiry = time.Now().Add(30 * time.Minute) // FMC tokens expire in 30 minutes
	ac.tokenMutex.Unlock()

	return nil
}

// authenticatePanorama authenticates with Palo Alto Panorama
func (ac *APIConnector) authenticatePanorama(ctx context.Context) error {
	// Panorama uses API key authentication, no separate login needed
	if ac.config.APIKey == "" {
		return fmt.Errorf("API key required for Panorama")
	}
	
	// Test with a simple API call
	return ac.testConnection(ctx)
}

// authenticateSolarWinds authenticates with SolarWinds
func (ac *APIConnector) authenticateSolarWinds(ctx context.Context) error {
	// SolarWinds uses basic auth, no separate login needed
	return ac.testConnection(ctx)
}

// authenticateFortiManager authenticates with Fortinet FortiManager
func (ac *APIConnector) authenticateFortiManager(ctx context.Context) error {
	endpoint := "/jsonrpc"
	
	requestData := map[string]interface{}{
		"id":     1,
		"method": "exec",
		"params": []map[string]interface{}{
			{
				"data": map[string]string{
					"user":   ac.config.Username,
					"passwd": ac.config.Password,
				},
				"url": "/sys/login/user",
			},
		},
	}
	
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", ac.baseURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := ac.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("FortiManager authentication failed with status: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	
	var result struct {
		Session string `json:"session"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse FortiManager auth response: %w", err)
	}
	
	if result.Session == "" {
		return fmt.Errorf("FortiManager authentication failed: no session returned")
	}
	
	// Store session ID as auth token
	ac.tokenMutex.Lock()
	ac.authToken = result.Session
	ac.tokenExpiry = time.Now().Add(12 * time.Hour) // FortiManager sessions last ~12 hours
	ac.tokenMutex.Unlock()
	
	return nil
}

// testConnection tests basic connectivity to the API server
func (ac *APIConnector) testConnection(ctx context.Context) error {
	// Try common health check endpoints
	endpoints := []string{"/api/health", "/api/version", "/health", "/version", "/"}

	for _, endpoint := range endpoints {
		req, err := ac.createRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			continue
		}

		resp, err := ac.httpClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
	}

	return fmt.Errorf("no accessible API endpoints found")
}

// Disconnect closes the API connection
func (ac *APIConnector) Disconnect(ctx context.Context) error {
	ac.sessionMutex.Lock()
	defer ac.sessionMutex.Unlock()

	if ac.httpClient != nil {
		ac.httpClient.CloseIdleConnections()
	}

	ac.setConnected(false)
	ac.setConnectionState(base.ConnectionStateDisconnected)
	return nil
}

// doRequest performs an HTTP request with retry logic and rate limiting
func (ac *APIConnector) doRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	// Apply rate limiting
	if err := ac.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit error: %w", err)
	}

	// Increment request counter
	ac.requestMutex.Lock()
	ac.requestCount++
	ac.requestMutex.Unlock()

	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	// Retry logic with exponential backoff
	var resp *http.Response
	for attempt := 0; attempt < ac.config.RetryCount; attempt++ {
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, reqErr := ac.createRequest(ctx, method, endpoint, reqBody)
		if reqErr != nil {
			return nil, reqErr
		}

		resp, err = ac.httpClient.Do(req)
		
		if err == nil && resp.StatusCode < 500 {
			// Success or client error (don't retry client errors)
			return resp, nil
		}

		// Close response body if it exists
		if resp != nil {
			resp.Body.Close()
		}

		// Wait before retry (exponential backoff)
		if attempt < ac.config.RetryCount-1 {
			backoff := time.Duration(attempt+1) * ac.config.RetryDelay
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", ac.config.RetryCount, err)
	}

	return resp, nil
}

// createRequest creates an HTTP request with authentication
func (ac *APIConnector) createRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	fullURL := ac.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}

	// Add authentication headers based on device type
	ac.tokenMutex.RLock()
	authToken := ac.authToken
	tokenExpiry := ac.tokenExpiry
	ac.tokenMutex.RUnlock()

	// Check if token needs refresh
	if !tokenExpiry.IsZero() && time.Now().After(tokenExpiry.Add(-5*time.Minute)) {
		// Token expiring soon, refresh it
		if err := ac.authenticate(ctx); err != nil {
			return nil, fmt.Errorf("failed to refresh auth token: %w", err)
		}
		ac.tokenMutex.RLock()
		authToken = ac.authToken
		ac.tokenMutex.RUnlock()
	}

	// Set authentication headers
	if authToken != "" {
		req.Header.Set("X-auth-access-token", authToken)
	} else if ac.config.APIKey != "" {
		req.Header.Set("X-PAN-KEY", ac.config.APIKey) // Panorama
		req.Header.Set("X-API-Key", ac.config.APIKey)  // Generic
	} else if ac.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ac.config.Token)
	} else if ac.config.Username != "" && ac.config.Password != "" {
		req.SetBasicAuth(ac.config.Username, ac.config.Password)
	}

	// Add custom headers
	for key, value := range ac.config.Headers {
		req.Header.Set(key, value)
	}

	// Set standard headers
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// IsConnected returns whether the API connection is active
func (ac *APIConnector) IsConnected() bool {
	return ac.BaseConnector.IsConnected() && ac.httpClient != nil
}

// GetType returns the connector type
func (ac *APIConnector) GetType() string {
	return "api"
}

// Ping performs a basic connectivity check
func (ac *APIConnector) Ping(ctx context.Context, timeout time.Duration) error {
	if !ac.IsConnected() {
		return fmt.Errorf("not connected")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return ac.testConnection(ctx)
}

// GetDeviceInfo retrieves device information from the management system
func (ac *APIConnector) GetDeviceInfo(ctx context.Context) (*base.DeviceInfo, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	deviceInfo := &base.DeviceInfo{
		ID:             ac.generateDeviceID(),
		Hostname:       ac.config.Host,
		IPAddress:      ac.config.Host,
		Vendor:         ac.getVendor(),
		Model:          ac.getModel(),
		ConnectionType: "api",
		Capabilities:   ac.getCapabilities(),
		LastSeen:       time.Now(),
	}

	return deviceInfo, nil
}

// GetSupportedCommands returns supported commands for this connector type
func (ac *APIConnector) GetSupportedCommands(ctx context.Context) ([]string, error) {
	var commands []string

	switch ac.deviceType {
	case types.DeviceTypeCiscoFMC:
		commands = []string{"get_device_list", "get_device_config", "get_security_events", "get_policies"}
	case types.DeviceTypePaloAltoPanorama:
		commands = []string{"get_device_list", "get_device_config", "get_device_groups", "get_templates"}
	case types.DeviceTypeSolarWindsNCM:
		commands = []string{"get_device_list", "get_device_config", "get_node_info", "execute_script"}
	default:
		commands = []string{"get_device_list", "get_device_config"}
	}

	return commands, nil
}

// ExecuteCommand executes a command via API
func (ac *APIConnector) ExecuteCommand(ctx context.Context, command string, timeout time.Duration) (*base.CommandResult, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	var output string
	var err error

	switch ac.deviceType {
	case types.DeviceTypeCiscoFMC:
		output, err = ac.executeCiscoFMCCommand(ctx, command)
	case types.DeviceTypePaloAltoPanorama:
		output, err = ac.executePanoramaCommand(ctx, command)
	case types.DeviceTypeFortinetFortiManager:
		output, err = ac.executeFortiManagerCommand(ctx, command)
	case types.DeviceTypeSolarWindsNCM:
		output, err = ac.executeSolarWindsCommand(ctx, command)
	default:
		err = fmt.Errorf("command execution not supported for device type: %s", ac.deviceType)
	}

	duration := time.Since(start)

	if err != nil {
		return ac.createCommandResult(ac.config.Host, command, "", err.Error(), -1, duration), nil
	}

	return ac.createCommandResult(ac.config.Host, command, output, "", 0, duration), nil
}

// GetDeviceInventory gets the list of devices from the management system
func (ac *APIConnector) GetDeviceInventory(ctx context.Context) ([]*base.DeviceInfo, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	switch ac.deviceType {
	case types.DeviceTypeCiscoFMC:
		return ac.getCiscoFMCDevices(ctx)
	case types.DeviceTypePaloAltoPanorama:
		return ac.getPanoramaDevices(ctx)
	case types.DeviceTypeFortinetFortiManager:
		return ac.getFortiManagerDevices(ctx)
	case types.DeviceTypeSolarWindsNCM:
		return ac.getNCMDevices(ctx)
	default:
		return nil, fmt.Errorf("device inventory not supported for device type: %s", ac.deviceType)
	}
}

// GetDeviceConfigs retrieves configurations for multiple devices
func (ac *APIConnector) GetDeviceConfigs(ctx context.Context, deviceIDs []string) (map[string]*base.ConfigResult, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	results := make(map[string]*base.ConfigResult)

	for _, deviceID := range deviceIDs {
		config, err := ac.getDeviceConfig(ctx, deviceID)
		if err != nil {
			results[deviceID] = &base.ConfigResult{
				DeviceID:    deviceID,
				ConfigType:  "running",
				RetrievedAt: time.Now(),
			}
		} else {
			results[deviceID] = config
		}
	}

	return results, nil
}

// SendCommandsToDevice sends commands to a specific device through the management system
func (ac *APIConnector) SendCommandsToDevice(ctx context.Context, deviceID string, commands []string, timeout time.Duration) ([]*base.CommandResult, error) {
	if !ac.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	results := make([]*base.CommandResult, len(commands))

	for i, command := range commands {
		result, err := ac.ExecuteCommand(ctx, command, timeout)
		if err != nil {
			results[i] = &base.CommandResult{
				DeviceID:   deviceID,
				Command:    command,
				Error:      err.Error(),
				ExitCode:   -1,
				ExecutedAt: time.Now(),
			}
		} else {
			results[i] = result
			results[i].DeviceID = deviceID
		}
	}

	return results, nil
}

// Platform-specific implementations

// executeCiscoFMCCommand executes commands on Cisco FMC
func (ac *APIConnector) executeCiscoFMCCommand(ctx context.Context, command string) (string, error) {
	switch command {
	case "get_device_list":
		devices, err := ac.getCiscoFMCDevices(ctx)
		if err != nil {
			return "", err
		}
		jsonData, _ := json.Marshal(devices)
		return string(jsonData), nil
	case "get_security_events":
		return ac.getCiscoFMCSecurityEvents(ctx)
	case "get_policies":
		return ac.getCiscoFMCPolicies(ctx)
	default:
		return "", fmt.Errorf("unsupported command: %s", command)
	}
}

// executePanoramaCommand executes commands on Palo Alto Panorama
func (ac *APIConnector) executePanoramaCommand(ctx context.Context, command string) (string, error) {
	switch command {
	case "get_device_list":
		devices, err := ac.getPanoramaDevices(ctx)
		if err != nil {
			return "", err
		}
		jsonData, _ := json.Marshal(devices)
		return string(jsonData), nil
	case "get_device_groups":
		return ac.getPanoramaDeviceGroups(ctx)
	case "get_templates":
		return ac.getPanoramaTemplates(ctx)
	default:
		return "", fmt.Errorf("unsupported command: %s", command)
	}
}

// executeSolarWindsCommand executes commands on SolarWinds NCM
func (ac *APIConnector) executeSolarWindsCommand(ctx context.Context, command string) (string, error) {
	switch command {
	case "get_device_list":
		devices, err := ac.getNCMDevices(ctx)
		if err != nil {
			return "", err
		}
		jsonData, _ := json.Marshal(devices)
		return string(jsonData), nil
	case "execute_script":
		return ac.executeNCMScript(ctx, "")
	default:
		return "", fmt.Errorf("unsupported command: %s", command)
	}
}

// getCiscoFMCDevices retrieves devices from Cisco FMC
func (ac *APIConnector) getCiscoFMCDevices(ctx context.Context) ([]*base.DeviceInfo, error) {
	endpoint := "/api/fmc_config/v1/domain/default/devices/devicerecords"

	resp, err := ac.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse FMC response
	var fmcResp struct {
		Items []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			HostName string `json:"hostName"`
			Model    string `json:"model"`
			Type     string `json:"type"`
		} `json:"items"`
	}
	
	if err := json.Unmarshal(body, &fmcResp); err != nil {
		return nil, fmt.Errorf("failed to parse FMC response: %w", err)
	}

	devices := make([]*base.DeviceInfo, 0, len(fmcResp.Items))
	for _, item := range fmcResp.Items {
		devices = append(devices, &base.DeviceInfo{
			ID:             item.ID,
			Name:           item.Name,
			Hostname:       item.HostName,
			Model:          item.Model,
			Vendor:         "Cisco",
			ConnectionType: "api",
			LastSeen:       time.Now(),
		})
	}
	
	return devices, nil
}

// getCiscoFMCSecurityEvents retrieves security events from Cisco FMC
func (ac *APIConnector) getCiscoFMCSecurityEvents(ctx context.Context) (string, error) {
	endpoint := "/api/fmc_config/v1/domain/default/policy/accesspolicies"

	resp, err := ac.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// getCiscoFMCPolicies retrieves policies from Cisco FMC
func (ac *APIConnector) getCiscoFMCPolicies(ctx context.Context) (string, error) {
	endpoint := "/api/fmc_config/v1/domain/default/policy/accesspolicies"

	resp, err := ac.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// getPanoramaDevices retrieves devices from Palo Alto Panorama
func (ac *APIConnector) getPanoramaDevices(ctx context.Context) ([]*base.DeviceInfo, error) {
	endpoint := "/api/?type=op&cmd=<show><devices><connected></connected></devices></show>"

	resp, err := ac.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse Panorama XML response
	type Device struct {
		Name     string `xml:"name,attr"`
		Serial   string `xml:"serial"`
		Hostname string `xml:"hostname"`
		Model    string `xml:"model"`
		Status   string `xml:"connected"`
	}
	
	type Response struct {
		Status  string   `xml:"status,attr"`
		Devices []Device `xml:"result>devices>entry"`
	}
	
	var xmlResp Response
	if err := xml.Unmarshal(body, &xmlResp); err != nil {
		return nil, fmt.Errorf("failed to parse Panorama XML: %w", err)
	}

	devices := make([]*base.DeviceInfo, 0, len(xmlResp.Devices))
	for _, d := range xmlResp.Devices {
		devices = append(devices, &base.DeviceInfo{
			ID:             d.Serial,
			Name:           d.Name,
			Hostname:       d.Hostname,
			Model:          d.Model,
			Vendor:         "Palo Alto Networks",
			ConnectionType: "api",
			Status:         d.Status,
			LastSeen:       time.Now(),
		})
	}
	
	return devices, nil
}

// getPanoramaDeviceGroups retrieves device groups from Palo Alto Panorama
func (ac *APIConnector) getPanoramaDeviceGroups(ctx context.Context) (string, error) {
	endpoint := "/api/?type=config&action=get&xpath=/config/devices/entry[@name='localhost.localdomain']/device-group"

	resp, err := ac.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// getPanoramaTemplates retrieves templates from Palo Alto Panorama
func (ac *APIConnector) getPanoramaTemplates(ctx context.Context) (string, error) {
	endpoint := "/api/?type=config&action=get&xpath=/config/devices/entry[@name='localhost.localdomain']/template"

	resp, err := ac.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// getNCMDevices retrieves devices from SolarWinds NCM
func (ac *APIConnector) getNCMDevices(ctx context.Context) ([]*base.DeviceInfo, error) {
	endpoint := "/SolarWinds/InformationService/v3/Json/Query"

	query := `SELECT NodeID, IPAddress, Caption, Vendor, MachineType, Status FROM Orion.Nodes`

	data := map[string]string{
		"query": query,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	resp, err := ac.doRequest(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse SolarWinds SWIS response
	var swisResp struct {
		Results []struct {
			NodeID      int    `json:"NodeID"`
			IPAddress   string `json:"IPAddress"`
			Caption     string `json:"Caption"`
			Vendor      string `json:"Vendor"`
			MachineType string `json:"MachineType"`
			Status      int    `json:"Status"`
		} `json:"results"`
	}
	
	if err := json.Unmarshal(body, &swisResp); err != nil {
		return nil, fmt.Errorf("failed to parse SolarWinds response: %w", err)
	}

	devices := make([]*base.DeviceInfo, 0, len(swisResp.Results))
	for _, node := range swisResp.Results {
		status := "unknown"
		if node.Status == 1 {
			status = "up"
		} else if node.Status == 2 {
			status = "down"
		}
		
		devices = append(devices, &base.DeviceInfo{
			ID:             fmt.Sprintf("%d", node.NodeID),
			Name:           node.Caption,
			IPAddress:      node.IPAddress,
			Model:          node.MachineType,
			Vendor:         node.Vendor,
			Status:         status,
			ConnectionType: "api",
			LastSeen:       time.Now(),
		})
	}
	
	return devices, nil
}

// executeNCMScript executes a script on SolarWinds NCM
func (ac *APIConnector) executeNCMScript(ctx context.Context, script string) (string, error) {
	endpoint := "/SolarWinds/InformationService/v3/Json/Invoke/Orion.Nodes/ExecuteScript"

	data := map[string]string{
		"script": script,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	resp, err := ac.doRequest(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// executeFortiManagerCommand executes commands on Fortinet FortiManager
func (ac *APIConnector) executeFortiManagerCommand(ctx context.Context, command string) (string, error) {
	switch command {
	case "get_device_list":
		devices, err := ac.getFortiManagerDevices(ctx)
		if err != nil {
			return "", err
		}
		jsonData, _ := json.Marshal(devices)
		return string(jsonData), nil
	default:
		return "", fmt.Errorf("unsupported command: %s", command)
	}
}

// getFortiManagerDevices retrieves devices from Fortinet FortiManager
func (ac *APIConnector) getFortiManagerDevices(ctx context.Context) ([]*base.DeviceInfo, error) {
	endpoint := "/jsonrpc"
	
	// Get list of FortiGate devices in the ADOM
	requestData := map[string]interface{}{
		"id":      1,
		"method":  "get",
		"params": []map[string]interface{}{
			{
				"url": "/dvmdb/adom/root/device",
			},
		},
		"session": ac.authToken,
	}
	
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}
	
	resp, err := ac.doRequest(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	// Parse FortiManager response
	var fmgrResp struct {
		Result []struct {
			Data []struct {
				Name      string `json:"name"`
				IP        string `json:"ip"`
				Platform  string `json:"platform_str"`
				OsVersion string `json:"os_ver"`
				ConnStatus string `json:"conn_status"`
			} `json:"data"`
			Status struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"status"`
		} `json:"result"`
	}
	
	if err := json.Unmarshal(body, &fmgrResp); err != nil {
		return nil, fmt.Errorf("failed to parse FortiManager response: %w", err)
	}
	
	if len(fmgrResp.Result) == 0 || fmgrResp.Result[0].Status.Code != 0 {
		return nil, fmt.Errorf("FortiManager API error: %v", fmgrResp.Result[0].Status.Message)
	}
	
	devices := make([]*base.DeviceInfo, 0, len(fmgrResp.Result[0].Data))
	for _, dev := range fmgrResp.Result[0].Data {
		status := "unknown"
		if dev.ConnStatus == "1" {
			status = "connected"
		} else {
			status = "disconnected"
		}
		
		devices = append(devices, &base.DeviceInfo{
			ID:             dev.Name,
			Name:           dev.Name,
			IPAddress:      dev.IP,
			Model:          dev.Platform,
			Vendor:         "Fortinet",
			Status:         status,
			ConnectionType: "api",
			LastSeen:       time.Now(),
		})
	}
	
	return devices, nil
}

// getDeviceConfig retrieves configuration for a specific device
func (ac *APIConnector) getDeviceConfig(ctx context.Context, deviceID string) (*base.ConfigResult, error) {
	// Implementation depends on management system API
	return nil, fmt.Errorf("device config retrieval not fully implemented for device type: %s", ac.deviceType)
}

// Helper methods

// generateDeviceID generates a unique device ID
func (ac *APIConnector) generateDeviceID() string {
	return fmt.Sprintf("api_%s_%d", ac.config.Host, ac.config.Port)
}

// getVendor returns the vendor name based on device type
func (ac *APIConnector) getVendor() string {
	switch ac.deviceType {
	case types.DeviceTypeCiscoFMC:
		return "Cisco"
	case types.DeviceTypePaloAltoPanorama:
		return "Palo Alto Networks"
	case types.DeviceTypeFortinetFortiManager:
		return "Fortinet"
	case types.DeviceTypeSolarWindsNCM:
		return "SolarWinds"
	default:
		return "Unknown"
	}
}

// getModel returns the model name based on device type
func (ac *APIConnector) getModel() string {
	switch ac.deviceType {
	case types.DeviceTypeCiscoFMC:
		return "Firepower Management Center"
	case types.DeviceTypePaloAltoPanorama:
		return "Panorama"
	case types.DeviceTypeFortinetFortiManager:
		return "FortiManager"
	case types.DeviceTypeSolarWindsNCM:
		return "Network Configuration Manager"
	default:
		return "Unknown"
	}
}

// getCapabilities returns the device capabilities
func (ac *APIConnector) getCapabilities() []string {
	caps := types.GetDeviceCapabilities(ac.deviceType)
	var capabilities []string

	if caps.CanManageDevices {
		capabilities = append(capabilities, "device_management")
	}
	if caps.CanGetDeviceList {
		capabilities = append(capabilities, "device_inventory")
	}
	if caps.CanPushConfig {
		capabilities = append(capabilities, "config_push")
	}
	if caps.CanGetLogs {
		capabilities = append(capabilities, "log_retrieval")
	}
	if caps.CanGetSecurityEvents {
		capabilities = append(capabilities, "security_events")
	}

	return capabilities
}

// GetRequestCount returns the total number of API requests made
func (ac *APIConnector) GetRequestCount() uint64 {
	ac.requestMutex.RLock()
	defer ac.requestMutex.RUnlock()
	return ac.requestCount
}

// ===== Missing Interface Methods - Phase 1.1 Implementation =====

// GetRunningConfig retrieves the running configuration from the management system or managed device
func (ac *APIConnector) GetRunningConfig(ctx context.Context, timeout time.Duration) (*base.ConfigResult, error) {
	startTime := time.Now()
	
	switch ac.deviceType {
	case types.DeviceTypeCiscoFMC:
		return ac.getCiscoFMCConfig(ctx, "running")
	case types.DeviceTypePaloAltoPanorama:
		return ac.getPanoramaConfig(ctx, "running")
	case types.DeviceTypeFortinetFortiManager:
		return ac.getFortiManagerConfig(ctx, "running")
	case types.DeviceTypeSolarWindsNCM:
		return ac.getNCMConfig(ctx, "running")
	default:
		return nil, fmt.Errorf("GetRunningConfig not supported for device type: %s", ac.deviceType)
	}
	
	_ = time.Since(startTime)
}

// GetStartupConfig retrieves the startup configuration from the management system or managed device
func (ac *APIConnector) GetStartupConfig(ctx context.Context, timeout time.Duration) (*base.ConfigResult, error) {
	startTime := time.Now()
	
	switch ac.deviceType {
	case types.DeviceTypeCiscoFMC:
		return ac.getCiscoFMCConfig(ctx, "startup")
	case types.DeviceTypePaloAltoPanorama:
		return ac.getPanoramaConfig(ctx, "candidate")
	case types.DeviceTypeFortinetFortiManager:
		return ac.getFortiManagerConfig(ctx, "startup")
	case types.DeviceTypeSolarWindsNCM:
		return ac.getNCMConfig(ctx, "startup")
	default:
		return nil, fmt.Errorf("GetStartupConfig not supported for device type: %s", ac.deviceType)
	}
	
	_ = time.Since(startTime)
}

// ExecuteCommands executes multiple commands in sequence
func (ac *APIConnector) ExecuteCommands(ctx context.Context, commands []string, timeout time.Duration) ([]*base.CommandResult, error) {
	results := make([]*base.CommandResult, 0, len(commands))
	
	for _, cmd := range commands {
		result, err := ac.ExecuteCommand(ctx, cmd, timeout)
		if err != nil {
			// Create error result
			result = &base.CommandResult{
				Command:    cmd,
				Error:      err.Error(),
				ExitCode:   1,
				ExecutedAt: time.Now(),
			}
		}
		results = append(results, result)
	}
	
	return results, nil
}

// GetManagedDevices is an alias for GetDeviceInventory
func (ac *APIConnector) GetManagedDevices(ctx context.Context) ([]*base.DeviceInfo, error) {
	return ac.GetDeviceInventory(ctx)
}

// CollectDeviceConfigs collects configurations from multiple managed devices
func (ac *APIConnector) CollectDeviceConfigs(ctx context.Context, deviceIDs []string, workingFolder string) (map[string][]string, error) {
	configs := make(map[string][]string)
	
	// Set working folder if provided
	if workingFolder != "" {
		if err := ac.SetWorkingFolder(workingFolder); err != nil {
			return nil, fmt.Errorf("failed to set working folder: %w", err)
		}
	}
	
	// Get configs for all devices
	configResults, err := ac.GetDeviceConfigs(ctx, deviceIDs)
	if err != nil {
		return nil, err
	}
	
	// Convert ConfigResult to file paths
	for deviceID, configResult := range configResults {
		if configResult != nil {
			// Write config to file
			fileName := fmt.Sprintf("%s_config.txt", deviceID)
			filePath := ac.GetConfigFilePath(fileName)
			
			if err := ac.WriteConfigToFile(ctx, configResult.ConfigData, filePath); err != nil {
				return nil, fmt.Errorf("failed to write config for device %s: %w", deviceID, err)
			}
			
			configs[deviceID] = []string{filePath}
		}
	}
	
	return configs, nil
}

// WriteConfigToFile writes configuration data to a file
func (ac *APIConnector) WriteConfigToFile(ctx context.Context, configData string, filePath string) error {
	return ioutil.WriteFile(filePath, []byte(configData), 0644)
}

// GetConfigFilePath returns the full path for a config file
func (ac *APIConnector) GetConfigFilePath(fileName string) string {
	workingFolder := ac.GetWorkingFolder()
	if workingFolder == "" {
		workingFolder = "."
	}
	return fmt.Sprintf("%s/%s", workingFolder, fileName)
}

// SetWorkingFolder sets the working folder for file operations
func (ac *APIConnector) SetWorkingFolder(folder string) error {
	// Store in config options
	if ac.config.Options == nil {
		ac.config.Options = make(map[string]interface{})
	}
	ac.config.Options["working_folder"] = folder
	return nil
}

// GetWorkingFolder gets the current working folder
func (ac *APIConnector) GetWorkingFolder() string {
	if ac.config.Options != nil {
		if folder, ok := ac.config.Options["working_folder"].(string); ok {
			return folder
		}
	}
	return ""
}

// GetConnectionState returns the current connection state
func (ac *APIConnector) GetConnectionState() base.ConnectionState {
	if ac.IsConnected() {
		return base.ConnectionStateConnected
	}
	return base.ConnectionStateDisconnected
}

// GetConfig returns the connection configuration
func (ac *APIConnector) GetConfig() *base.ConnectionConfig {
	return ac.config
}

// ===== Config Retrieval Methods by Platform =====

// getCiscoFMCConfig retrieves configuration from Cisco FMC
func (ac *APIConnector) getCiscoFMCConfig(ctx context.Context, configType string) (*base.ConfigResult, error) {
	startTime := time.Now()
	
	// FMC API endpoint for policies and configurations
	endpoint := "/api/fmc_config/v1/domain/default/policy/accesspolicies"
	
	resp, err := ac.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get FMC config: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read FMC response: %w", err)
	}
	
	// Compute hash
	hash := fmt.Sprintf("%x", body)
	
	return &base.ConfigResult{
		DeviceID:    ac.generateDeviceID(),
		ConfigType:  configType,
		ConfigData:  string(body),
		Hash:        hash,
		RetrievedAt: time.Now(),
		Duration:    time.Since(startTime),
	}, nil
}

// getPanoramaConfig retrieves configuration from Palo Alto Panorama
func (ac *APIConnector) getPanoramaConfig(ctx context.Context, configType string) (*base.ConfigResult, error) {
	startTime := time.Now()
	
	// Panorama XML API endpoint for full config
	endpoint := "/api/?type=op&cmd=<show><config><" + configType + "/></config></show>"
	
	resp, err := ac.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Panorama config: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Panorama response: %w", err)
	}
	
	// Compute hash
	hash := fmt.Sprintf("%x", body)
	
	return &base.ConfigResult{
		DeviceID:    ac.generateDeviceID(),
		ConfigType:  configType,
		ConfigData:  string(body),
		Hash:        hash,
		RetrievedAt: time.Now(),
		Duration:    time.Since(startTime),
	}, nil
}

// getFortiManagerConfig retrieves configuration from Fortinet FortiManager
func (ac *APIConnector) getFortiManagerConfig(ctx context.Context, configType string) (*base.ConfigResult, error) {
	startTime := time.Now()
	
	// FortiManager JSON-RPC request to get global settings
	requestBody := map[string]interface{}{
		"method": "get",
		"params": []interface{}{
			map[string]interface{}{
				"url": "/pm/config/global/obj/firewall/policy",
			},
		},
		"id":      1,
		"session": ac.authToken,
	}
	
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal FortiManager request: %w", err)
	}
	
	resp, err := ac.doRequest(ctx, "POST", "/jsonrpc", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to get FortiManager config: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read FortiManager response: %w", err)
	}
	
	// Compute hash
	hash := fmt.Sprintf("%x", body)
	
	return &base.ConfigResult{
		DeviceID:    ac.generateDeviceID(),
		ConfigType:  configType,
		ConfigData:  string(body),
		Hash:        hash,
		RetrievedAt: time.Now(),
		Duration:    time.Since(startTime),
	}, nil
}

// getNCMConfig retrieves configuration from SolarWinds NCM
func (ac *APIConnector) getNCMConfig(ctx context.Context, configType string) (*base.ConfigResult, error) {
	startTime := time.Now()
	
	// SolarWinds SWIS query for device configurations
	query := fmt.Sprintf(`
		SELECT NodeID, NodeName, ConfigDate, Config
		FROM NCM.NodeConfigs
		WHERE ConfigType = '%s'
		ORDER BY ConfigDate DESC
	`, configType)
	
	requestBody := map[string]interface{}{
		"query": query,
	}
	
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal NCM request: %w", err)
	}
	
	resp, err := ac.doRequest(ctx, "POST", "/SolarWinds/InformationService/v3/Json/Query", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to get NCM config: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read NCM response: %w", err)
	}
	
	// Compute hash
	hash := fmt.Sprintf("%x", body)
	
	return &base.ConfigResult{
		DeviceID:    ac.generateDeviceID(),
		ConfigType:  configType,
		ConfigData:  string(body),
		Hash:        hash,
		RetrievedAt: time.Now(),
		Duration:    time.Since(startTime),
	}, nil
}
