package solarwinds

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NCMClient represents a SolarWinds NCM (Network Configuration Manager) client
// Uses SWIS (SolarWinds Information Service) for API access
type NCMClient struct {
	BaseURL    string       // e.g., "https://solarwinds.example.com:17778"
	Username   string
	Password   string
	HTTPClient *http.Client
}

// NCMConfig represents configuration for creating an NCM client
type NCMConfig struct {
	BaseURL           string
	Username          string
	Password          string
	InsecureSkipVerify bool  // Skip TLS verification (for self-signed certs)
	Timeout           time.Duration
}

// NodeConfigData represents a configuration from NCM
type NodeConfigData struct {
	NodeID           int       `json:"NodeID"`
	NodeName         string    `json:"NodeName"`
	NodeIPAddress    string    `json:"NodeIPAddress"`
	NodeVendor       string    `json:"NodeVendor"`
	NodeMachineType  string    `json:"NodeMachineType"`
	ConfigID         int       `json:"ConfigID"`
	ConfigText       string    `json:"ConfigText"`
	ConfigType       string    `json:"ConfigType"`       // "Running", "Startup"
	DownloadTime     time.Time `json:"DownloadTime"`
	ConfigSize       int       `json:"ConfigSize"`
	TransferProtocol string    `json:"TransferProtocol"` // "SSH", "TFTP", "SCP"
}

// SWISQueryRequest represents a SWIS query request
type SWISQueryRequest struct {
	Query      string                 `json:"query"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// SWISQueryResponse represents a SWIS query response
type SWISQueryResponse struct {
	Results []map[string]interface{} `json:"results"`
	Errors  []SWISError              `json:"errors,omitempty"`
}

// SWISError represents an error from SWIS
type SWISError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// NewNCMClient creates a new SolarWinds NCM client
func NewNCMClient(config NCMConfig) *NCMClient {
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

	return &NCMClient{
		BaseURL:    config.BaseURL,
		Username:   config.Username,
		Password:   config.Password,
		HTTPClient: httpClient,
	}
}

// QueryConfigs retrieves configurations from NCM using SWIS query
func (c *NCMClient) QueryConfigs(filter ConfigFilter) ([]NodeConfigData, error) {
	query := c.buildConfigQuery(filter)

	response, err := c.executeSWISQuery(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query NCM configs: %w", err)
	}

	// Check for SWIS errors
	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("SWIS query error: %s (code: %d)", response.Errors[0].Message, response.Errors[0].Code)
	}

	// Parse results into NodeConfigData
	configs := make([]NodeConfigData, 0, len(response.Results))
	for _, result := range response.Results {
		config, err := c.parseConfigResult(result)
		if err != nil {
			// Log warning but continue processing other results
			fmt.Printf("Warning: failed to parse config result: %v\n", err)
			continue
		}
		configs = append(configs, config)
	}

	return configs, nil
}

// ConfigFilter defines filters for querying configurations
type ConfigFilter struct {
	NodeIDs       []int     // Filter by specific node IDs
	NodeNames     []string  // Filter by node names (partial match)
	Vendors       []string  // Filter by vendor (e.g., "Cisco", "Palo Alto", "Fortinet")
	ConfigType    string    // "Running", "Startup", or empty for both
	SinceTime     *time.Time // Only configs downloaded after this time
	Limit         int       // Limit number of results (0 = no limit)
}

// buildConfigQuery constructs a SWQL query based on filter criteria
func (c *NCMClient) buildConfigQuery(filter ConfigFilter) string {
	query := `
		SELECT
			n.NodeID,
			n.Caption AS NodeName,
			n.IP_Address AS NodeIPAddress,
			n.Vendor AS NodeVendor,
			n.MachineType AS NodeMachineType,
			cfg.ConfigID,
			cfg.Config AS ConfigText,
			cfg.ConfigType,
			cfg.DownloadTime,
			cfg.ConfigSize,
			cfg.TransferProtocol
		FROM NCM.NodeConfigData cfg
		INNER JOIN Orion.Nodes n ON cfg.NodeID = n.NodeID
		WHERE 1=1
	`

	// Add filters
	if len(filter.NodeIDs) > 0 {
		query += fmt.Sprintf(" AND n.NodeID IN (%v)", formatIntList(filter.NodeIDs))
	}

	if len(filter.NodeNames) > 0 {
		query += " AND ("
		for i, name := range filter.NodeNames {
			if i > 0 {
				query += " OR "
			}
			query += fmt.Sprintf("n.Caption LIKE '%%%s%%'", name)
		}
		query += ")"
	}

	if len(filter.Vendors) > 0 {
		query += " AND ("
		for i, vendor := range filter.Vendors {
			if i > 0 {
				query += " OR "
			}
			query += fmt.Sprintf("n.Vendor LIKE '%%%s%%'", vendor)
		}
		query += ")"
	}

	if filter.ConfigType != "" {
		query += fmt.Sprintf(" AND cfg.ConfigType = '%s'", filter.ConfigType)
	}

	if filter.SinceTime != nil {
		query += fmt.Sprintf(" AND cfg.DownloadTime > '%s'", filter.SinceTime.Format("2006-01-02T15:04:05"))
	}

	// Order by most recent first
	query += " ORDER BY cfg.DownloadTime DESC"

	// Apply limit
	if filter.Limit > 0 {
		query += fmt.Sprintf(" TOP %d", filter.Limit)
	}

	return query
}

// executeSWISQuery executes a SWQL query against the SWIS API
func (c *NCMClient) executeSWISQuery(query string) (*SWISQueryResponse, error) {
	url := fmt.Sprintf("%s/SolarWinds/InformationService/v3/Json/Query", c.BaseURL)

	requestBody := SWISQueryRequest{
		Query: query,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.Username, c.Password)

	// Execute request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute SWIS query: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SWIS API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response SWISQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode SWIS response: %w", err)
	}

	return &response, nil
}

// parseConfigResult parses a SWIS query result into NodeConfigData
func (c *NCMClient) parseConfigResult(result map[string]interface{}) (NodeConfigData, error) {
	config := NodeConfigData{}

	// Helper function to safely extract values
	getString := func(key string) string {
		if val, ok := result[key]; ok && val != nil {
			return fmt.Sprintf("%v", val)
		}
		return ""
	}

	getInt := func(key string) int {
		if val, ok := result[key]; ok && val != nil {
			if intVal, ok := val.(float64); ok {
				return int(intVal)
			}
		}
		return 0
	}

	getTime := func(key string) time.Time {
		if val, ok := result[key]; ok && val != nil {
			if timeStr, ok := val.(string); ok {
				t, err := time.Parse(time.RFC3339, timeStr)
				if err == nil {
					return t
				}
			}
		}
		return time.Time{}
	}

	// Parse fields
	config.NodeID = getInt("NodeID")
	config.NodeName = getString("NodeName")
	config.NodeIPAddress = getString("NodeIPAddress")
	config.NodeVendor = getString("NodeVendor")
	config.NodeMachineType = getString("NodeMachineType")
	config.ConfigID = getInt("ConfigID")
	config.ConfigText = getString("ConfigText")
	config.ConfigType = getString("ConfigType")
	config.DownloadTime = getTime("DownloadTime")
	config.ConfigSize = getInt("ConfigSize")
	config.TransferProtocol = getString("TransferProtocol")

	return config, nil
}

// GetConfigByNodeID retrieves the latest configuration for a specific node
func (c *NCMClient) GetConfigByNodeID(nodeID int) (*NodeConfigData, error) {
	configs, err := c.QueryConfigs(ConfigFilter{
		NodeIDs: []int{nodeID},
		Limit:   1,
	})
	if err != nil {
		return nil, err
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no configuration found for node ID %d", nodeID)
	}

	return &configs[0], nil
}

// GetConfigsByVendor retrieves configurations for all devices from a specific vendor
func (c *NCMClient) GetConfigsByVendor(vendor string, limit int) ([]NodeConfigData, error) {
	return c.QueryConfigs(ConfigFilter{
		Vendors: []string{vendor},
		Limit:   limit,
	})
}

// TestConnection tests the connection to SolarWinds NCM
func (c *NCMClient) TestConnection() error {
	query := "SELECT TOP 1 NodeID FROM Orion.Nodes"
	_, err := c.executeSWISQuery(query)
	if err != nil {
		return fmt.Errorf("NCM connection test failed: %w", err)
	}
	return nil
}

// Helper function to format int list for SQL query
func formatIntList(ids []int) string {
	if len(ids) == 0 {
		return ""
	}

	result := ""
	for i, id := range ids {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%d", id)
	}
	return result
}
