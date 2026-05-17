package constants

const (
	SOURCE_TYPE_VENDOR = "vendor"

	VendorCisco    = "cisco"
	VendorFortinet = "fortinet"
	VendorPaloAlto = "paloalto"
	VendorJuniper  = "juniper"
	VendorArista   = "arista"
	VendorAruba    = "aruba"
	VendorDell     = "dell"

	DeviceTypeFirewall = "firewall"
	DeviceTypeSwitch   = "switch"
	DeviceTypeRouter   = "router"

	HAModeStandalone    = "standalone"
	HAModeActiveStandby = "active-standby"
	HAModeActiveActive  = "active-active"

	InterfaceTypePhysical  = "physical"
	InterfaceTypeVLAN      = "vlan"
	InterfaceTypeLoopback  = "loopback"
	InterfaceTypeTunnel    = "tunnel"
	InterfaceTypeAggregate = "aggregate"
	InterfaceTypeOther     = "other"

	PolicyActionAllow = "allow"
	PolicyActionDeny  = "deny"
	PolicyActionVPN   = "vpn"

	ProtocolTCP   = "tcp"
	ProtocolUDP   = "udp"
	ProtocolICMP  = "icmp"
	ProtocolICMPv6 = "icmpv6"
	ProtocolIP    = "ip"

	RoutingProtocolStatic    = "static"
	RoutingProtocolConnected = "connected"
	RoutingProtocolOSPF      = "ospf"
	RoutingProtocolBGP       = "bgp"
	RoutingProtocolRIP       = "rip"
	RoutingProtocolISIS      = "isis"

	NATTypeSource      = "source"
	NATTypeDestination = "destination"

	VPNTypeSiteToSite   = "site-to-site"
	VPNTypeRemoteAccess = "remote-access"
	VPNTypeDynamicDNS   = "dynamic-dns"

	VIPTypeStaticNAT  = "static-nat"
	VIPTypeLoadBalance = "load-balance"
	VIPTypeDNS        = "dns"

	AddressTypeHost      = "host"
	AddressTypeNetwork   = "network"
	AddressTypeRange     = "range"
	AddressTypeFQDN      = "fqdn"
	AddressTypeWildcard  = "wildcard"
	AddressTypeGeography = "geography"
)

var CommonProtocolPorts = map[string]int{
	"http":  80,
	"https": 443,
	"ssh":   22,
	"dns":   53,
}
