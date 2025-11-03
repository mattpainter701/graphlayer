package fortimanager

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FortiManagerClient represents a FortiManager API client
// Uses JSON-RPC 2.0 API for centralized management
type FortiManagerClient struct {
	BaseURL    string       // e.g., "https://fortimanager.example.com"
	Username   string
	Password   string
	SessionID  string       // Session token after authentication
	HTTPClient *http.Client
}

// FortiManagerConfig represents configuration for creating a FortiManager client
type FortiManagerConfig struct {
	BaseURL            string
	Username           string
	Password           string
	InsecureSkipVerify bool
	Timeout            time.Duration
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Session string      `json:"session,omitempty"`
	Version string      `json:"jsonrpc"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	ID     int                    `json:"id"`
	Result map[string]interface{} `json:"result"`
	Error  *JSONRPCError          `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// ADOMInfo represents Administrative Domain information
type ADOMInfo struct {
	Name        string `json:"name"`
	Description string `json:"desc"`
	Mode        string `json:"mode"`
	OSVersion   string `json:"os_ver"`
	MRVersion   int    `json:"mr"`
	Status      int    `json:"state"` // 1 = enabled
}

// ManagedDevice represents a FortiGate managed by FortiManager
type ManagedDevice struct {
	Name           string    `json:"name"`
	SerialNumber   string    `json:"sn"`
	IPAddress      string    `json:"ip"`
	Model          string    `json:"platform_str"`
	Version        string    `json:"os_ver"`
	Connected      int       `json:"conn_status"` // 1 = connected
	HAMode         string    `json:"ha_mode"`
	VDOMMode       string    `json:"vdom_mode"`
	ADOM           string    `json:"adom"`
	DeviceGroups   []string  `json:"dev_grp,omitempty"`
	InstallStatus  int       `json:"conf_status"` // 1 = installed, 2 = modified
	LastConnection time.Time `json:"-"`
}

// PolicyPackage represents a policy package in FortiManager
type PolicyPackage struct {
	Name        string             `json:"name"`
	Type        string             `json:"type"` // "pkg" or "folder"
	Description string             `json:"desc"`
	ADOM        string             `json:"adom"`
	Scope       []PolicyPackageScope `json:"scope"`
}

// PolicyPackageScope represents the scope of a policy package
type PolicyPackageScope struct {
	Name string `json:"name"`
	VDOM string `json:"vdom"`
}

// NewFortiManagerClient creates a new FortiManager API client
func NewFortiManagerClient(config FortiManagerConfig) *FortiManagerClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.InsecureSkipVerify,
			},
		},
	}

	return &FortiManagerClient{
		BaseURL:    config.BaseURL,
		Username:   config.Username,
		Password:   config.Password,
		HTTPClient: httpClient,
	}
}

// Login authenticates with FortiManager and obtains session ID
func (c *FortiManagerClient) Login() error {
	params := map[string]interface{}{
		"url": "/sys/login/user",
		"data": map[string]string{
			"user":   c.Username,
			"passwd": c.Password,
		},
	}

	response, err := c.executeRequest("exec", params)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Extract session ID from response
	if session, ok := response.Result["session"].(string); ok {
		c.SessionID = session
		return nil
	}

	return fmt.Errorf("no session ID in login response")
}

// Logout terminates the FortiManager session
func (c *FortiManagerClient) Logout() error {
	params := map[string]interface{}{
		"url": "/sys/logout",
	}

	_, err := c.executeRequest("exec", params)
	if err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}

	c.SessionID = ""
	return nil
}

// GetADOMs retrieves list of Administrative Domains
func (c *FortiManagerClient) GetADOMs() ([]ADOMInfo, error) {
	params := map[string]interface{}{
		"url": "/dvmdb/adom",
	}

	response, err := c.executeRequest("get", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get ADOMs: %w", err)
	}

	// Parse ADOM data
	data, ok := response.Result["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected ADOM data format")
	}

	adoms := make([]ADOMInfo, 0, len(data))
	for _, item := range data {
		adomData, _ := item.(map[string]interface{})

		adom := ADOMInfo{
			Name:        getStringField(adomData, "name"),
			Description: getStringField(adomData, "desc"),
			Mode:        getStringField(adomData, "mode"),
			OSVersion:   getStringField(adomData, "os_ver"),
			MRVersion:   getIntField(adomData, "mr"),
			Status:      getIntField(adomData, "state"),
		}
		adoms = append(adoms, adom)
	}

	return adoms, nil
}

// GetManagedDevices retrieves list of managed FortiGate devices for a specific ADOM
func (c *FortiManagerClient) GetManagedDevices(adomName string) ([]ManagedDevice, error) {
	params := map[string]interface{}{
		"url": fmt.Sprintf("/dvmdb/adom/%s/device", adomName),
	}

	response, err := c.executeRequest("get", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get managed devices: %w", err)
	}

	// Parse device data
	data, ok := response.Result["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected device data format")
	}

	devices := make([]ManagedDevice, 0, len(data))
	for _, item := range data {
		deviceData, _ := item.(map[string]interface{})

		device := ManagedDevice{
			Name:          getStringField(deviceData, "name"),
			SerialNumber:  getStringField(deviceData, "sn"),
			IPAddress:     getStringField(deviceData, "ip"),
			Model:         getStringField(deviceData, "platform_str"),
			Version:       getStringField(deviceData, "os_ver"),
			Connected:     getIntField(deviceData, "conn_status"),
			HAMode:        getStringField(deviceData, "ha_mode"),
			VDOMMode:      getStringField(deviceData, "vdom_mode"),
			ADOM:          adomName,
			InstallStatus: getIntField(deviceData, "conf_status"),
		}

		// Parse device groups if present
		if devGrps, ok := deviceData["dev_grp"].([]interface{}); ok {
			device.DeviceGroups = make([]string, 0, len(devGrps))
			for _, grp := range devGrps {
				if grpStr, ok := grp.(string); ok {
					device.DeviceGroups = append(device.DeviceGroups, grpStr)
				}
			}
		}

		devices = append(devices, device)
	}

	return devices, nil
}

// GetPolicyPackages retrieves list of policy packages for a specific ADOM
func (c *FortiManagerClient) GetPolicyPackages(adomName string) ([]PolicyPackage, error) {
	params := map[string]interface{}{
		"url": fmt.Sprintf("/pm/pkg/adom/%s", adomName),
	}

	response, err := c.executeRequest("get", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy packages: %w", err)
	}

	// Parse policy package data
	data, ok := response.Result["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected policy package data format")
	}

	packages := make([]PolicyPackage, 0, len(data))
	for _, item := range data {
		pkgData, _ := item.(map[string]interface{})

		pkg := PolicyPackage{
			Name:        getStringField(pkgData, "name"),
			Type:        getStringField(pkgData, "type"),
			Description: getStringField(pkgData, "desc"),
			ADOM:        adomName,
		}

		// Parse scope if present
		if scopeData, ok := pkgData["scope"].([]interface{}); ok {
			pkg.Scope = make([]PolicyPackageScope, 0, len(scopeData))
			for _, scopeItem := range scopeData {
				if scopeMap, ok := scopeItem.(map[string]interface{}); ok {
					scope := PolicyPackageScope{
						Name: getStringField(scopeMap, "name"),
						VDOM: getStringField(scopeMap, "vdom"),
					}
					pkg.Scope = append(pkg.Scope, scope)
				}
			}
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

// GetDeviceConfig retrieves configuration for a specific managed device
func (c *FortiManagerClient) GetDeviceConfig(adomName, deviceName string) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"url": fmt.Sprintf("/pm/config/adom/%s/obj/device/%s", adomName, deviceName),
	}

	response, err := c.executeRequest("get", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get device config: %w", err)
	}

	return response.Result, nil
}

// GetGlobalObjects retrieves global objects (addresses, services) for an ADOM
func (c *FortiManagerClient) GetGlobalObjects(adomName, objectType string) ([]interface{}, error) {
	// objectType can be: "firewall/address", "firewall/addrgrp", "firewall/service/custom", etc.
	params := map[string]interface{}{
		"url": fmt.Sprintf("/pm/config/adom/%s/obj/%s", adomName, objectType),
	}

	response, err := c.executeRequest("get", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get global objects: %w", err)
	}

	// Parse object data
	data, ok := response.Result["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected object data format")
	}

	return data, nil
}

// InstallPolicyPackage installs a policy package to target devices
func (c *FortiManagerClient) InstallPolicyPackage(adomName, packageName string, targets []string) error {
	params := map[string]interface{}{
		"url": "/securityconsole/install/package",
		"data": map[string]interface{}{
			"adom":  adomName,
			"pkg":   packageName,
			"scope": targets,
		},
	}

	_, err := c.executeRequest("exec", params)
	if err != nil {
		return fmt.Errorf("failed to install policy package: %w", err)
	}

	return nil
}

// executeRequest executes a JSON-RPC request to FortiManager
func (c *FortiManagerClient) executeRequest(method string, params interface{}) (*JSONRPCResponse, error) {
	url := fmt.Sprintf("%s/jsonrpc", c.BaseURL)

	request := JSONRPCRequest{
		ID:      1,
		Method:  method,
		Params:  []interface{}{params},
		Session: c.SessionID,
		Version: "2.0",
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("FortiManager API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for JSON-RPC error
	if response.Error != nil {
		return nil, fmt.Errorf("FortiManager API error: %s (code: %d)", response.Error.Message, response.Error.Code)
	}

	// Check result status
	if status, ok := response.Result["status"].(map[string]interface{}); ok {
		if code, ok := status["code"].(float64); ok && code != 0 {
			message, _ := status["message"].(string)
			return nil, fmt.Errorf("FortiManager operation failed: %s (code: %.0f)", message, code)
		}
	}

	return &response, nil
}

// Helper functions to safely extract fields from interface{} maps

func getStringField(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok && val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntField(data map[string]interface{}, key string) int {
	if val, ok := data[key]; ok && val != nil {
		if floatVal, ok := val.(float64); ok {
			return int(floatVal)
		}
		if intVal, ok := val.(int); ok {
			return intVal
		}
	}
	return 0
}

func getBoolField(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok && val != nil {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
		// FortiManager often uses 0/1 for booleans
		if intVal, ok := val.(float64); ok {
			return intVal != 0
		}
	}
	return false
}
