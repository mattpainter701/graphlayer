package iana

/*
IANA Reference Constants

This package provides protocol numbers, service port mappings, and EtherType
definitions from IANA (Internet Assigned Numbers Authority) and IEEE standards.

Data sources:
- IANA Protocol Numbers Registry
- IANA Service Name and Port Number Registry
- IEEE 802 Numbers (EtherTypes)
- Cisco Default Services

Usage:
	loader := NewIANALoader()
	protocolNum, ok := loader.GetProtocolNumber("tcp")
	serviceName := loader.GetServiceFromPort(80, "tcp") // returns "http"
	etherType := loader.GetEtherTypeName("2048") // returns "IPv4"
*/

// ProtocolMap maps protocol names to protocol numbers
// Sourced from IANA Protocol Numbers Registry + custom mappings
type ProtocolMap map[string]string

// ServiceMap maps service names to protocol:port combinations
// Format: service_name -> protocol -> [ports]
// Example: "http" -> "tcp" -> ["80", "8080"]
type ServiceMap map[string]map[string][]string

// EtherTypeMap maps EtherType decimal values to protocol names
// Sourced from IEEE 802 Numbers Registry
type EtherTypeMap map[string]string

// ReverseServiceMap maps (protocol, port) tuples to service names
// Format: (protocol, port) -> service_name
// Example: ("tcp", "80") -> "http"
type ReverseServiceMap map[ProtocolPort]string

// ProtocolPort represents a protocol and port number tuple
type ProtocolPort struct {
	Protocol string
	Port     string
}

// Well-known protocol numbers for quick reference
const (
	ProtocolIP      = "4"   // IPv4
	ProtocolTCP     = "6"   // TCP
	ProtocolUDP     = "17"  // UDP
	ProtocolICMP    = "1"   // ICMP
	ProtocolICMPv6  = "58"  // ICMPv6
	ProtocolIPv6    = "41"  // IPv6
	ProtocolESP     = "50"  // Encapsulating Security Payload
	ProtocolAH      = "51"  // Authentication Header
	ProtocolGRE     = "47"  // Generic Routing Encapsulation
	ProtocolOSPF    = "89"  // OSPF
	ProtocolEIGRP   = "88"  // EIGRP
	ProtocolSCTP    = "132" // SCTP
	ProtocolAny     = "998" // Special: Any protocol
	ProtocolTCPUDP  = "999" // Special: TCP or UDP
)

// Well-known service names for quick reference
const (
	ServiceHTTP    = "http"
	ServiceHTTPS   = "https"
	ServiceSSH     = "ssh"
	ServiceTelnet  = "telnet"
	ServiceFTP     = "ftp"
	ServiceSMTP    = "smtp"
	ServiceDNS     = "dns"
	ServiceDHCP    = "dhcp"
	ServiceSNMP    = "snmp"
	ServiceLDAP    = "ldap"
	ServiceSyslog  = "syslog"
	ServiceRadius  = "radius"
	ServiceTFTP    = "tftp"
	ServiceNTP     = "ntp"
	ServiceBGP     = "bgp"
	ServiceRIP     = "rip"
	ServiceOSPF    = "ospf" // Note: OSPF uses protocol 89, not a port
	ServiceNetBIOS = "netbios-ns"
)

// Cisco-specific service mappings
// These are default services recognized by Cisco devices
var CiscoDefaultServices = map[string]map[string][]string{
	"aol":                {"tcp": {"5190"}, "udp": {"5190"}},
	"bgp":                {"tcp": {"179"}, "udp": {"179"}},
	"biff":               {"udp": {"512"}},
	"bootpc":             {"udp": {"68"}, "tcp": {"68"}},
	"bootps":             {"udp": {"67"}, "tcp": {"67"}},
	"chargen":            {"tcp": {"19"}, "udp": {"19"}},
	"citrix-ica":         {"tcp": {"1494"}},
	"cmd":                {"tcp": {"514"}},
	"ctiqbe":             {"tcp": {"2748"}},
	"daytime":            {"tcp": {"13"}, "udp": {"13"}},
	"discard":            {"tcp": {"9"}, "udp": {"9"}},
	"dns":                {"udp": {"53"}},
	"domain":             {"tcp": {"53"}, "udp": {"53"}},
	"dnsix":              {"udp": {"195", "90"}, "tcp": {"90"}},
	"echo":               {"tcp": {"7"}, "udp": {"7"}},
	"exec":               {"tcp": {"512"}},
	"finger":             {"tcp": {"79"}, "udp": {"79"}},
	"ftp":                {"tcp": {"21"}, "udp": {"21"}},
	"ftp-data":           {"tcp": {"20"}, "udp": {"20"}},
	"gopher":             {"tcp": {"70"}, "udp": {"70"}},
	"http":               {"tcp": {"80"}, "udp": {"80"}},
	"https":              {"tcp": {"443"}, "udp": {"443"}},
	"h323":               {"tcp": {"1720"}},
	"hostname":           {"tcp": {"101"}, "udp": {"101"}},
	"ident":              {"tcp": {"113"}},
	"imap4":              {"tcp": {"143"}},
	"irc":                {"tcp": {"194"}, "udp": {"194"}},
	"isakmp":             {"udp": {"500"}, "tcp": {"500"}},
	"kerberos":           {"tcp": {"750", "88"}, "udp": {"750", "88"}},
	"klogin":             {"tcp": {"543"}, "udp": {"543"}},
	"kshell":             {"tcp": {"544"}, "udp": {"544"}},
	"ldap":               {"tcp": {"389"}, "udp": {"389"}},
	"ldaps":              {"tcp": {"636"}, "udp": {"636"}},
	"lpd":                {"tcp": {"515"}},
	"login":              {"tcp": {"513"}},
	"lotusnotes":         {"tcp": {"1352"}},
	"mobile-ip":          {"udp": {"434"}},
	"nameserver":         {"udp": {"42"}, "tcp": {"42"}},
	"netbios-ns":         {"udp": {"137"}, "tcp": {"137"}},
	"netbios-dgm":        {"udp": {"138"}, "tcp": {"138"}},
	"netbios-ssn":        {"tcp": {"139"}, "udp": {"139"}},
	"nntp":               {"tcp": {"119"}, "udp": {"119"}},
	"ntp":                {"udp": {"123"}, "tcp": {"123"}},
	"pcanywhere-status":  {"udp": {"5632"}},
	"pcanywhere-data":    {"tcp": {"5631"}},
	"pim-auto-rp":        {"tcp": {"496"}, "udp": {"496"}},
	"pop2":               {"tcp": {"109"}, "udp": {"109"}},
	"pop3":               {"tcp": {"110"}, "udp": {"110"}},
	"pptp":               {"tcp": {"1723"}, "udp": {"1723"}},
	"radius":             {"udp": {"1645", "1812"}, "tcp": {"1812"}},
	"radius-acct":        {"udp": {"1646", "1813"}, "tcp": {"1813"}},
	"rip":                {"udp": {"520"}},
	"rsh":                {"tcp": {"514"}},
	"secureid-udp":       {"udp": {"5510"}},
	"sftp":               {"tcp": {"115"}, "udp": {"115"}},
	"smtp":               {"tcp": {"25"}, "udp": {"25"}},
	"snmp":               {"udp": {"161"}, "tcp": {"161"}},
	"snmptrap":           {"udp": {"162"}, "tcp": {"162"}},
	"sqlnet":             {"tcp": {"1521"}},
	"ssh":                {"tcp": {"22"}, "udp": {"22"}},
	"sunrpc":             {"tcp": {"111"}, "udp": {"111"}},
	"syslog":             {"udp": {"514"}},
	"tacacs":             {"tcp": {"49"}, "udp": {"49"}},
	"talk":               {"tcp": {"517"}, "udp": {"517"}},
	"tacacs-ds":          {"tcp": {"65"}},
	"telnet":             {"tcp": {"23", "24"}, "udp": {"23", "24"}},
	"tftp":               {"udp": {"69"}, "tcp": {"69"}},
	"time":               {"udp": {"37"}, "tcp": {"37"}},
	"uucp":               {"tcp": {"540"}, "udp": {"540"}},
	"wbem-http":          {"tcp": {"5988"}, "udp": {"5988"}},
	"wbem-https":         {"tcp": {"5989"}, "udp": {"5989"}},
	"who":                {"udp": {"513"}},
	"whois":              {"tcp": {"43"}},
	"www":                {"tcp": {"80"}, "udp": {"80"}},
	"xdmcp":              {"udp": {"177"}, "tcp": {"177"}},
}

// BaseProtocolMap provides common protocol name to number mappings
var BaseProtocolMap = ProtocolMap{
	"ip":       ProtocolIP,
	"tcp":      ProtocolTCP,
	"udp":      ProtocolUDP,
	"icmp":     ProtocolICMP,
	"icmp6":    ProtocolICMPv6,
	"icmpv6":   ProtocolICMPv6,
	"ipv6":     ProtocolIPv6,
	"esp":      ProtocolESP,
	"ah":       ProtocolAH,
	"gre":      ProtocolGRE,
	"ospf":     ProtocolOSPF,
	"ospfigp":  ProtocolOSPF,
	"eigrp":    ProtocolEIGRP,
	"sctp":     ProtocolSCTP,
	"any":      ProtocolAny,
	"tcp-udp":  ProtocolTCPUDP,
	"hopopt":   "0",
	"igmp":     "2",
	"ggp":      "3",
	"ipv4":     "4",
	"st":       "5",
	"cbt":      "7",
	"egp":      "8",
	"igp":      "9",
	"pup":      "12",
	"xnet":     "15",
	"chaos":    "16",
	"mux":      "18",
	"hmp":      "20",
	"rdp":      "27",
	"irtp":     "28",
	"iso-tp4":  "29",
	"netblt":   "30",
	"mfe-nsp":  "31",
	"merit-inp": "32",
	"dccp":     "33",
	"3pc":      "34",
	"idpr":     "35",
	"xtp":      "36",
	"ddp":      "37",
	"ipv6-route": "43",
	"ipv6-frag": "44",
	"idrp":     "45",
	"rsvp":     "46",
	"mobile":   "55",
	"ipv6-icmp": "58",
	"ipv6-nonxt": "59",
	"ipv6-opts": "60",
	"vrrp":     "112",
	"pgm":      "113",
	"l2tp":     "115",
	"isis":     "124",
	"fc":       "133",
	"mobility": "135",
	"udplite":  "136",
	"mpls-in-ip": "137",
	"manet":    "138",
	"hip":      "139",
	"shim6":    "140",
	"wesp":     "141",
	"rohc":     "142",
	"ethernet": "143",
}
