package registry

import (
	"context"
	"fmt"

	"github.com/mattpainter701/graphlayer/platform/orchestration/connectors/api"
	"github.com/mattpainter701/graphlayer/platform/orchestration/connectors/base"
	"github.com/mattpainter701/graphlayer/platform/orchestration/connectors/ssh"
	"github.com/mattpainter701/graphlayer/platform/orchestration/types"
)

// ConnectorFactory creates connectors based on device type and connection configuration
type ConnectorFactory struct {
	// Registry of device type to connector type mappings
	deviceTypeRegistry map[types.DeviceType]string
}

// NewConnectorFactory creates a new connector factory
func NewConnectorFactory() *ConnectorFactory {
	factory := &ConnectorFactory{
		deviceTypeRegistry: make(map[types.DeviceType]string),
	}

	// Register device type mappings
	factory.registerDeviceTypes()

	return factory
}

// CreateConnector creates a connector instance based on the device type and configuration
func (cf *ConnectorFactory) CreateConnector(deviceType types.DeviceType, config *base.ConnectionConfig) (base.Connector, error) {
	connectorType, exists := cf.deviceTypeRegistry[deviceType]
	if !exists {
		return nil, fmt.Errorf("unsupported device type: %s", deviceType)
	}

	switch connectorType {
	case "ssh":
		return ssh.NewSSHConnector(config, deviceType)
	case "api":
		return api.NewAPIConnector(config, deviceType)
	default:
		return nil, fmt.Errorf("unsupported connector type: %s", connectorType)
	}
}

// GetSupportedDeviceTypes returns all supported device types
func (cf *ConnectorFactory) GetSupportedDeviceTypes() []types.DeviceType {
	var deviceTypes []types.DeviceType

	for deviceType := range cf.deviceTypeRegistry {
		deviceTypes = append(deviceTypes, deviceType)
	}

	return deviceTypes
}

// GetConnectorTypeForDevice returns the connector type for a given device type
func (cf *ConnectorFactory) GetConnectorTypeForDevice(deviceType types.DeviceType) (string, error) {
	connectorType, exists := cf.deviceTypeRegistry[deviceType]
	if !exists {
		return "", fmt.Errorf("unsupported device type: %s", deviceType)
	}

	return connectorType, nil
}

// registerDeviceTypes registers device type to connector type mappings
func (cf *ConnectorFactory) registerDeviceTypes() {
	// Direct SSH connections
	cf.deviceTypeRegistry[types.DeviceTypeCiscoIOS] = "ssh"
	cf.deviceTypeRegistry[types.DeviceTypeCiscoASA] = "ssh"
	cf.deviceTypeRegistry[types.DeviceTypeCiscoNXOS] = "ssh"
	cf.deviceTypeRegistry[types.DeviceTypePaloAltoPA] = "ssh"
	cf.deviceTypeRegistry[types.DeviceTypeJuniperJunos] = "ssh"
	cf.deviceTypeRegistry[types.DeviceTypeJuniperSRX] = "ssh"
	cf.deviceTypeRegistry[types.DeviceTypeFortinetFortiGate] = "ssh"
	cf.deviceTypeRegistry[types.DeviceTypeCheckPointGaia] = "ssh"
	cf.deviceTypeRegistry[types.DeviceTypeF5BigIP] = "ssh"
	cf.deviceTypeRegistry[types.DeviceTypeGenericSSH] = "ssh"

	// API-based management systems
	cf.deviceTypeRegistry[types.DeviceTypeCiscoFMC] = "api"
	cf.deviceTypeRegistry[types.DeviceTypePaloAltoPanorama] = "api"
	cf.deviceTypeRegistry[types.DeviceTypeFortinetFortiManager] = "api"
	cf.deviceTypeRegistry[types.DeviceTypeSolarWindsNCM] = "api"
	cf.deviceTypeRegistry[types.DeviceTypeGenericAPI] = "api"
}

// ConnectorManager manages multiple connectors
type ConnectorManager struct {
	factory    *ConnectorFactory
	connectors map[string]base.Connector // deviceID -> connector
}

// NewConnectorManager creates a new connector manager
func NewConnectorManager() *ConnectorManager {
	return &ConnectorManager{
		factory:    NewConnectorFactory(),
		connectors: make(map[string]base.Connector),
	}
}

// CreateAndRegisterConnector creates a new connector and registers it
func (cm *ConnectorManager) CreateAndRegisterConnector(deviceID string, deviceType types.DeviceType, config *base.ConnectionConfig) (base.Connector, error) {
	connector, err := cm.factory.CreateConnector(deviceType, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connector: %w", err)
	}

	cm.connectors[deviceID] = connector
	return connector, nil
}

// GetConnector retrieves a registered connector by device ID
func (cm *ConnectorManager) GetConnector(deviceID string) (base.Connector, error) {
	connector, exists := cm.connectors[deviceID]
	if !exists {
		return nil, fmt.Errorf("connector not found for device: %s", deviceID)
	}

	return connector, nil
}

// RemoveConnector removes a connector from the registry
func (cm *ConnectorManager) RemoveConnector(deviceID string) error {
	connector, exists := cm.connectors[deviceID]
	if !exists {
		return fmt.Errorf("connector not found for device: %s", deviceID)
	}

	// Disconnect before removing
	ctx := context.Background()
	connector.Disconnect(ctx)

	delete(cm.connectors, deviceID)
	return nil
}

// GetAllConnectors returns all registered connectors
func (cm *ConnectorManager) GetAllConnectors() map[string]base.Connector {
	connectors := make(map[string]base.Connector)
	for deviceID, connector := range cm.connectors {
		connectors[deviceID] = connector
	}

	return connectors
}

// GetFactory returns the underlying factory
func (cm *ConnectorManager) GetFactory() *ConnectorFactory {
	return cm.factory
}

// GetConnectorTypes returns supported connector types
func (cm *ConnectorManager) GetConnectorTypes() []string {
	return []string{"ssh", "api"}
}
