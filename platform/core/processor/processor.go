package processor

type ParserType string

const (
	ParserTypeCiscoIOS  ParserType = "cisco_ios"
	ParserTypeCiscoNXOS ParserType = "cisco_nxos"
	ParserTypePaloAlto  ParserType = "palo_alto"
)

func BuildCiscoL2Config() interface{} {
	return nil
}

func BuildEOSL2Config() interface{} {
	return nil
}

func BuildPanoramaTopologyModels() interface{} {
	return nil
}

func IsAddressInSubnet(ip, subnet string) bool {
	return false
}

func MapAddressesToNetworkNodes(addresses []string) interface{} {
	return nil
}

func ParseConfig(content string) (interface{}, error) {
	return nil, nil
}

func ParseEOSConfig(content string) (interface{}, error) {
	return nil, nil
}

func ProcessIOSFile(path string) error {
	return nil
}

func ProcessEOSFile(path string) error {
	return nil
}

func ProcessOS10File(path string) error {
	return nil
}

func BuildPANOSDeviceModel() interface{}  { return nil }
func BuildPANOSVirtualContextModel() interface{} { return nil }
func BuildPANOSZoneModel() interface{} { return nil }
func BuildPANOSInterfaceModel() interface{} { return nil }
func BuildPANOSRoutingModel() interface{} { return nil }
func BuildPANOSSecurityPolicyModel() interface{} { return nil }
func BuildPANOSNATModel() interface{} { return nil }
func BuildPANOSAddressModel() interface{} { return nil }
func BuildPANOSServiceModel() interface{} { return nil }
