package parser

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================
// PANORAMA PROCESSOR
// ========================================

// PanoramaProcessor handles Panorama-specific mapping to vendor-agnostic models
type PanoramaProcessor struct {
	// Input: Parsed Panorama data
	systemInfo         PANOSSystemInfo
	deviceGroups       []PanoramaDeviceGroup
	templates          []PanoramaTemplate
	managedDevices     []PanoramaManagedDevice
	connectedDevices   []PanoramaConnectedDevice
	deviceHealth       []PanoramaDeviceHealth
	sharedAddresses    []PANOSAddressObject
	sharedAddrGroups   []PANOSAddressGroup
	sharedServices     []PANOSServiceObject
	sharedSvcGroups    []PANOSServiceGroup

	// Output: Vendor-agnostic topology models
	TopologyDevices              map[string]models.Device
	TopologyDeviceGroups         map[string]models.DeviceGroup
	TopologyAddresses            map[string]models.Address
	TopologyAddressGroups        map[string]models.AddressGroup
	TopologyServices             map[string]models.Service
	TopologyServiceGroups        map[string]models.ServiceGroup
	TopologyIntegrationMessages  []models.IntegrationMessage
}

// ========================================
// DEVICE GROUP MODEL (NEW)
// ========================================

// Note: This requires adding models.DeviceGroup to the common models package
// If models.DeviceGroup doesn't exist, it should be defined as:
/*
package models

type DeviceGroup struct {
	ID          string
	Name        string
	Description string
	Devices     []string // Serial numbers or hostnames
	ParentGroup string
	Tags        []string
	Metadata    map[string]string
}
*/

// ========================================
// PANORAMA MAPPERS
// ========================================

// buildPanoramaDeviceModel maps Panorama system info to models.Device
func (p *PanoramaProcessor) buildPanoramaDeviceModel() error {
	hostname := p.systemInfo.Hostname
	if hostname == "" {
		hostname = "panorama-device"
	}

	device := models.Device{
		ID:           uuid.New().String(),
		Hostname:     hostname,
		ManagementIP: p.systemInfo.IPAddress,
		Model:        p.systemInfo.Model,
		Vendor:       "palo_alto",
		OSVersion:    p.systemInfo.SWVersion,
		SerialNumber: p.systemInfo.Serial,
		DeviceType:   "panorama", // Key difference: type is "panorama" not "firewall"
		LastSeen:     time.Now().Format(time.RFC3339),
		Metadata: map[string]string{
			"uptime":               p.systemInfo.Uptime,
			"panorama_mode":        "true",
			"managed_device_count": fmt.Sprintf("%d", len(p.managedDevices)),
		},
	}

	p.TopologyDevices[hostname] = device
	p.addIntegrationMessage("info", fmt.Sprintf("Mapped Panorama device: %s (Model: %s, Version: %s)", hostname, device.Model, device.OSVersion))

	return nil
}

// buildPanoramaDeviceGroupModel maps device groups to models.DeviceGroup
func (p *PanoramaProcessor) buildPanoramaDeviceGroupModel() error {
	for _, dg := range p.deviceGroups {
		deviceGroupID := uuid.New().String()

		// Extract device serial numbers from device group
		var deviceSerials []string
		for _, dev := range dg.Devices {
			deviceSerials = append(deviceSerials, dev.Name)
		}

		deviceGroup := models.DeviceGroup{
			ID:          deviceGroupID,
			Name:        dg.Name,
			Description: dg.Description,
			Devices:     deviceSerials,
			ParentGroup: dg.ParentGroup,
			Tags:        dg.Tags,
			Metadata: map[string]string{
				"device_count": fmt.Sprintf("%d", len(deviceSerials)),
			},
		}

		p.TopologyDeviceGroups[dg.Name] = deviceGroup
		p.addIntegrationMessage("info", fmt.Sprintf("Mapped device group: %s (Devices: %d, Parent: %s)",
			dg.Name, len(deviceSerials), dg.ParentGroup))
	}

	return nil
}

// buildPanoramaManagedDeviceModel maps managed devices to models.Device
func (p *PanoramaProcessor) buildPanoramaManagedDeviceModel() error {
	for _, managedDev := range p.managedDevices {
		deviceID := uuid.New().String()
		hostname := managedDev.Hostname
		if hostname == "" {
			hostname = managedDev.Name
		}

		// Find operational status from connected devices
		var connected bool
		var configStatus string
		var haState string
		var lastConnected string

		for _, connectedDev := range p.connectedDevices {
			if connectedDev.SerialNumber == managedDev.SerialNumber {
				connected = connectedDev.Connected == "yes"
				configStatus = connectedDev.ConfigStatus
				haState = connectedDev.HAState
				lastConnected = connectedDev.LastConnected
				break
			}
		}

		// Find health metrics from device health
		var uptime string
		var cpuUsage string
		var memoryUsage string
		var sessionCount string

		for _, healthDev := range p.deviceHealth {
			if healthDev.SerialNumber == managedDev.SerialNumber {
				uptime = healthDev.Uptime
				cpuUsage = healthDev.CPUUsage
				memoryUsage = healthDev.MemoryUsage
				sessionCount = healthDev.SessionCount
				break
			}
		}

		device := models.Device{
			ID:           deviceID,
			Hostname:     hostname,
			ManagementIP: managedDev.IPAddress,
			Model:        managedDev.Model,
			Vendor:       "palo_alto",
			OSVersion:    managedDev.SWVersion,
			SerialNumber: managedDev.SerialNumber,
			DeviceType:   "firewall",
			LastSeen:     time.Now().Format(time.RFC3339),
			Metadata: map[string]string{
				"panorama_managed":      "true",
				"connected":             fmt.Sprintf("%t", connected),
				"config_status":         configStatus,
				"ha_state":              haState,
				"last_connected":        lastConnected,
				"device_group":          managedDev.DeviceGroup,
				"template":              managedDev.Template,
				"uptime":                uptime,
				"cpu_usage":             cpuUsage,
				"memory_usage":          memoryUsage,
				"session_count":         sessionCount,
			},
		}

		p.TopologyDevices[hostname] = device
		p.addIntegrationMessage("info", fmt.Sprintf("Mapped managed device: %s (Serial: %s, Connected: %t, Config: %s)",
			hostname, managedDev.SerialNumber, connected, configStatus))
	}

	return nil
}

// buildPanoramaSharedObjectModel maps shared address and service objects
func (p *PanoramaProcessor) buildPanoramaSharedObjectModel() error {
	// Map shared address objects
	for _, addr := range p.sharedAddresses {
		addressID := uuid.New().String()

		addressValue := ""
		if addr.IPNetmask != "" {
			addressValue = addr.IPNetmask
		} else if addr.IPRange != "" {
			addressValue = addr.IPRange
		} else if addr.FQDN != "" {
			addressValue = addr.FQDN
		}

		addressModel := models.Address{
			ID:          addressID,
			Name:        addr.Name,
			Value:       addressValue,
			Description: addr.Description,
			Tags:        addr.Tags,
			Metadata: map[string]string{
				"shared":         "true",
				"panorama_scope": "global",
			},
		}

		if addr.FQDN != "" {
			addressModel.Metadata["address_type"] = "fqdn"
		}

		p.TopologyAddresses[addr.Name] = addressModel
	}

	// Map shared address groups
	for _, grp := range p.sharedAddrGroups {
		addressGroupID := uuid.New().String()

		addressGroupModel := models.AddressGroup{
			ID:          addressGroupID,
			Name:        grp.Name,
			Members:     grp.Static,
			Description: grp.Description,
			Tags:        grp.Tags,
			Metadata: map[string]string{
				"shared":         "true",
				"panorama_scope": "global",
			},
		}

		if grp.Dynamic != "" {
			addressGroupModel.Metadata["dynamic_filter"] = grp.Dynamic
		}

		p.TopologyAddressGroups[grp.Name] = addressGroupModel
	}

	// Map shared service objects
	for _, svc := range p.sharedServices {
		serviceID := uuid.New().String()

		serviceModel := models.Service{
			ID:              serviceID,
			Name:            svc.Name,
			Protocol:        svc.Protocol,
			DestinationPort: parsePortRange(svc.Port),
			Description:     svc.Description,
			Tags:            svc.Tags,
			Metadata: map[string]string{
				"shared":         "true",
				"panorama_scope": "global",
			},
		}

		p.TopologyServices[svc.Name] = serviceModel
	}

	// Map shared service groups
	for _, grp := range p.sharedSvcGroups {
		serviceGroupID := uuid.New().String()

		serviceGroupModel := models.ServiceGroup{
			ID:          serviceGroupID,
			Name:        grp.Name,
			Members:     grp.Members,
			Description: grp.Description,
			Tags:        grp.Tags,
			Metadata: map[string]string{
				"shared":         "true",
				"panorama_scope": "global",
			},
		}

		p.TopologyServiceGroups[grp.Name] = serviceGroupModel
	}

	p.addIntegrationMessage("info", fmt.Sprintf("Mapped shared objects: %d addresses, %d address groups, %d services, %d service groups",
		len(p.sharedAddresses), len(p.sharedAddrGroups), len(p.sharedServices), len(p.sharedSvcGroups)))

	return nil
}

// ========================================
// ORCHESTRATOR
// ========================================

// BuildPanoramaTopologyModels orchestrates all Panorama mapper functions
func (p *PanoramaProcessor) BuildPanoramaTopologyModels() error {
	// Initialize maps if nil
	if p.TopologyDevices == nil {
		p.TopologyDevices = make(map[string]models.Device)
	}
	if p.TopologyDeviceGroups == nil {
		p.TopologyDeviceGroups = make(map[string]models.DeviceGroup)
	}
	if p.TopologyAddresses == nil {
		p.TopologyAddresses = make(map[string]models.Address)
	}
	if p.TopologyAddressGroups == nil {
		p.TopologyAddressGroups = make(map[string]models.AddressGroup)
	}
	if p.TopologyServices == nil {
		p.TopologyServices = make(map[string]models.Service)
	}
	if p.TopologyServiceGroups == nil {
		p.TopologyServiceGroups = make(map[string]models.ServiceGroup)
	}

	// Build Panorama device model
	if err := p.buildPanoramaDeviceModel(); err != nil {
		return fmt.Errorf("failed to build Panorama device model: %w", err)
	}

	// Build device group models
	if err := p.buildPanoramaDeviceGroupModel(); err != nil {
		return fmt.Errorf("failed to build device group models: %w", err)
	}

	// Build managed device models
	if err := p.buildPanoramaManagedDeviceModel(); err != nil {
		return fmt.Errorf("failed to build managed device models: %w", err)
	}

	// Build shared object models
	if err := p.buildPanoramaSharedObjectModel(); err != nil {
		return fmt.Errorf("failed to build shared object models: %w", err)
	}

	return nil
}

// ========================================
// HELPER FUNCTIONS
// ========================================

// addIntegrationMessage adds a message to the integration message list
func (p *PanoramaProcessor) addIntegrationMessage(level, message string) {
	integrationMessage := models.IntegrationMessage{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Message:   message,
	}
	p.TopologyIntegrationMessages = append(p.TopologyIntegrationMessages, integrationMessage)
}

// ========================================
// FINAL MAPPER FUNCTION
// ========================================

// PanoramaMapper is the main entry point for Panorama topology mapping
func PanoramaMapper(
	systemInfo PANOSSystemInfo,
	deviceGroups []PanoramaDeviceGroup,
	templates []PanoramaTemplate,
	managedDevices []PanoramaManagedDevice,
	connectedDevices []PanoramaConnectedDevice,
	deviceHealth []PanoramaDeviceHealth,
	sharedAddresses []PANOSAddressObject,
	sharedAddrGroups []PANOSAddressGroup,
	sharedServices []PANOSServiceObject,
	sharedSvcGroups []PANOSServiceGroup,
) (*PanoramaProcessor, error) {
	processor := &PanoramaProcessor{
		systemInfo:         systemInfo,
		deviceGroups:       deviceGroups,
		templates:          templates,
		managedDevices:     managedDevices,
		connectedDevices:   connectedDevices,
		deviceHealth:       deviceHealth,
		sharedAddresses:    sharedAddresses,
		sharedAddrGroups:   sharedAddrGroups,
		sharedServices:     sharedServices,
		sharedSvcGroups:    sharedSvcGroups,
		TopologyDevices:              make(map[string]models.Device),
		TopologyDeviceGroups:         make(map[string]models.DeviceGroup),
		TopologyAddresses:            make(map[string]models.Address),
		TopologyAddressGroups:        make(map[string]models.AddressGroup),
		TopologyServices:             make(map[string]models.Service),
		TopologyServiceGroups:        make(map[string]models.ServiceGroup),
		TopologyIntegrationMessages:  []models.IntegrationMessage{},
	}

	err := processor.BuildPanoramaTopologyModels()
	if err != nil {
		return nil, fmt.Errorf("failed to build Panorama topology models: %w", err)
	}

	return processor, nil
}
