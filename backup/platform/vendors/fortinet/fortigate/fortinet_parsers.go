package fortinet

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ========================================
// FortiGate System Parsers
// ========================================

// FortiGateSystemStatus represents FortiGate system information
type FortiGateSystemStatus struct {
	Hostname      string
	Version       string
	Build         int
	SerialNumber  string
	VDOMMode      string // "split" or "multi-vdom"
	Model         string
	CurrentVDOM   string
	AdminUsers    int
	OperationMode string // "nat" or "transparent"
	HAMode        string // "standalone", "active-passive", "active-active"
	HAGroupID     int
	HAGroupName   string
}

// ParseFortiGateSystemStatus parses system status from FortiGate REST API
func ParseFortiGateSystemStatus(jsonData string) (*FortiGateSystemStatus, error) {
	var response struct {
		Results struct {
			Hostname      string `json:"hostname"`
			Version       string `json:"version"`
			Build         int    `json:"build"`
			Serial        string `json:"serial"`
			VDOMMode      string `json:"vdom-mode"`
			Model         string `json:"model"`
			CurrentVDOM   string `json:"current-vdom"`
			AdminUsers    int    `json:"admin-users"`
			OperationMode string `json:"operation-mode"`
			HAMode        string `json:"ha-mode"`
			HAGroupID     int    `json:"ha-group-id"`
			HAGroupName   string `json:"ha-group-name"`
		} `json:"results"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate system status: %w", err)
	}

	return &FortiGateSystemStatus{
		Hostname:      response.Results.Hostname,
		Version:       response.Results.Version,
		Build:         response.Results.Build,
		SerialNumber:  response.Results.Serial,
		VDOMMode:      response.Results.VDOMMode,
		Model:         response.Results.Model,
		CurrentVDOM:   response.Results.CurrentVDOM,
		AdminUsers:    response.Results.AdminUsers,
		OperationMode: response.Results.OperationMode,
		HAMode:        response.Results.HAMode,
		HAGroupID:     response.Results.HAGroupID,
		HAGroupName:   response.Results.HAGroupName,
	}, nil
}

// FortiGateVDOM represents a Virtual Domain
type FortiGateVDOM struct {
	Name       string
	Comment    string
	Enabled    string // "enable" or "disable"
	VClusterID int
}

// ParseFortiGateVDOMList parses VDOM list from FortiGate REST API
func ParseFortiGateVDOMList(jsonData string) ([]FortiGateVDOM, error) {
	var response struct {
		Results []struct {
			Name       string `json:"name"`
			Comment    string `json:"comment"`
			Enabled    string `json:"enabled"`
			VClusterID int    `json:"vcluster-id"`
		} `json:"results"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate VDOM list: %w", err)
	}

	vdoms := make([]FortiGateVDOM, 0, len(response.Results))
	for _, v := range response.Results {
		vdoms = append(vdoms, FortiGateVDOM{
			Name:       v.Name,
			Comment:    v.Comment,
			Enabled:    v.Enabled,
			VClusterID: v.VClusterID,
		})
	}

	return vdoms, nil
}

// ========================================
// FortiGate Network Configuration Parsers
// ========================================

// FortiGateInterface represents a FortiGate interface
type FortiGateInterface struct {
	Name            string
	VDOM            string
	IP              string
	Netmask         string
	Type            string // "physical", "vlan", "loopback", "tunnel", "aggregate"
	Alias           string
	Description     string
	Status          string // "up" or "down"
	Role            string // "lan", "wan", "dmz", "undefined"
	Mode            string // "static", "dhcp"
	AllowAccess     []string
	VlanID          int
	Interface       string // Parent interface for VLANs
	Members         []string // For aggregate/redundant interfaces
	Speed           string
	Duplex          string
	MTU             int
	MTUOverride     string // "enable" or "disable"
}

// ParseFortiGateInterfaces parses interface configuration
func ParseFortiGateInterfaces(jsonData string) ([]FortiGateInterface, error) {
	var response struct {
		Results []struct {
			Name        string `json:"name"`
			VDOM        string `json:"vdom"`
			IP          string `json:"ip"`
			Type        string `json:"type"`
			Alias       string `json:"alias"`
			Description string `json:"description"`
			Status      string `json:"status"`
			Role        string `json:"role"`
			Mode        string `json:"mode"`
			AllowAccess string `json:"allowaccess"`
			VlanID      int    `json:"vlanid"`
			Interface   string `json:"interface"`
			Member      []struct {
				InterfaceName string `json:"interface-name"`
			} `json:"member"`
			Speed       string `json:"speed"`
			Duplex      string `json:"duplex"`
			MTU         int    `json:"mtu"`
			MTUOverride string `json:"mtu-override"`
		} `json:"results"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate interfaces: %w", err)
	}

	interfaces := make([]FortiGateInterface, 0, len(response.Results))
	for _, iface := range response.Results {
		// Parse IP/Netmask
		ip := iface.IP
		netmask := ""
		if strings.Contains(ip, " ") {
			parts := strings.Fields(ip)
			if len(parts) >= 2 {
				ip = parts[0]
				netmask = parts[1]
			}
		}

		// Parse allow access services
		allowAccess := []string{}
		if iface.AllowAccess != "" {
			allowAccess = strings.Split(iface.AllowAccess, " ")
		}

		// Parse member interfaces
		members := make([]string, 0, len(iface.Member))
		for _, m := range iface.Member {
			members = append(members, m.InterfaceName)
		}

		interfaces = append(interfaces, FortiGateInterface{
			Name:        iface.Name,
			VDOM:        iface.VDOM,
			IP:          ip,
			Netmask:     netmask,
			Type:        iface.Type,
			Alias:       iface.Alias,
			Description: iface.Description,
			Status:      iface.Status,
			Role:        iface.Role,
			Mode:        iface.Mode,
			AllowAccess: allowAccess,
			VlanID:      iface.VlanID,
			Interface:   iface.Interface,
			Members:     members,
			Speed:       iface.Speed,
			Duplex:      iface.Duplex,
			MTU:         iface.MTU,
			MTUOverride: iface.MTUOverride,
		})
	}

	return interfaces, nil
}

// FortiGateZone represents a FortiGate zone
type FortiGateZone struct {
	Name        string
	VDOM        string
	Interfaces  []string
	Description string
	Intrazone   string // "allow", "deny"
}

// ParseFortiGateZones parses zone configuration
func ParseFortiGateZones(jsonData string) ([]FortiGateZone, error) {
	var response struct {
		Results []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Intrazone   string `json:"intrazone"`
			Interface   []struct {
				InterfaceName string `json:"interface-name"`
			} `json:"interface"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate zones: %w", err)
	}

	zones := make([]FortiGateZone, 0, len(response.Results))
	for _, z := range response.Results {
		interfaces := make([]string, 0, len(z.Interface))
		for _, iface := range z.Interface {
			interfaces = append(interfaces, iface.InterfaceName)
		}

		zones = append(zones, FortiGateZone{
			Name:        z.Name,
			VDOM:        response.VDOM,
			Interfaces:  interfaces,
			Description: z.Description,
			Intrazone:   z.Intrazone,
		})
	}

	return zones, nil
}

// FortiGateStaticRoute represents a static route
type FortiGateStaticRoute struct {
	SeqNum      int
	Status      string // "enable" or "disable"
	Destination string // CIDR notation
	Gateway     string
	Device      string
	Distance    int
	Weight      int
	Priority    int
	Comment     string
	VDOM        string
}

// ParseFortiGateStaticRoutes parses static route configuration
func ParseFortiGateStaticRoutes(jsonData string) ([]FortiGateStaticRoute, error) {
	var response struct {
		Results []struct {
			SeqNum  int    `json:"seq-num"`
			Status  string `json:"status"`
			Dst     string `json:"dst"`
			Gateway string `json:"gateway"`
			Device  string `json:"device"`
			Distance int   `json:"distance"`
			Weight  int    `json:"weight"`
			Priority int   `json:"priority"`
			Comment string `json:"comment"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate static routes: %w", err)
	}

	routes := make([]FortiGateStaticRoute, 0, len(response.Results))
	for _, r := range response.Results {
		routes = append(routes, FortiGateStaticRoute{
			SeqNum:      r.SeqNum,
			Status:      r.Status,
			Destination: r.Dst,
			Gateway:     r.Gateway,
			Device:      r.Device,
			Distance:    r.Distance,
			Weight:      r.Weight,
			Priority:    r.Priority,
			Comment:     r.Comment,
			VDOM:        response.VDOM,
		})
	}

	return routes, nil
}

// FortiGateBGPConfig represents BGP configuration
type FortiGateBGPConfig struct {
	AS                  int
	RouterID            string
	VDOM                string
	Neighbors           []FortiGateBGPNeighbor
	Networks            []FortiGateBGPNetwork
	AggregateAddresses  []string
	RedistributeConnected string
	RedistributeStatic  string
	RedistributeOSPF    string
}

// FortiGateBGPNeighbor represents a BGP neighbor
type FortiGateBGPNeighbor struct {
	IP                 string
	RemoteAS           int
	Description        string
	UpdateSource       string
	NextHopSelf        string // "enable" or "disable"
	RouteMapIn         string
	RouteMapOut        string
	PrefixListIn       string
	PrefixListOut      string
	MaximumPrefix      int
	MaximumPrefixThreshold int
}

// FortiGateBGPNetwork represents a BGP network advertisement
type FortiGateBGPNetwork struct {
	ID           int
	Prefix       string
	BackdoorRoute string
	RouteMap     string
}

// ParseFortiGateBGP parses BGP configuration
func ParseFortiGateBGP(jsonData string) (*FortiGateBGPConfig, error) {
	var response struct {
		Results struct {
			AS       int    `json:"as"`
			RouterID string `json:"router-id"`
			Neighbor []struct {
				IP                     string `json:"ip"`
				RemoteAS               int    `json:"remote-as"`
				Description            string `json:"description"`
				UpdateSource           string `json:"update-source"`
				NextHopSelf            string `json:"next-hop-self"`
				RouteMapIn             string `json:"route-map-in"`
				RouteMapOut            string `json:"route-map-out"`
				PrefixListIn           string `json:"prefix-list-in"`
				PrefixListOut          string `json:"prefix-list-out"`
				MaximumPrefix          int    `json:"maximum-prefix"`
				MaximumPrefixThreshold int    `json:"maximum-prefix-threshold"`
			} `json:"neighbor"`
			Network []struct {
				ID            int    `json:"id"`
				Prefix        string `json:"prefix"`
				BackdoorRoute string `json:"backdoor"`
				RouteMap      string `json:"route-map"`
			} `json:"network"`
			AggregateAddress []struct {
				Prefix string `json:"prefix"`
			} `json:"aggregate-address"`
			Redistribute []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"redistribute"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate BGP config: %w", err)
	}

	// Parse neighbors
	neighbors := make([]FortiGateBGPNeighbor, 0, len(response.Results.Neighbor))
	for _, n := range response.Results.Neighbor {
		neighbors = append(neighbors, FortiGateBGPNeighbor{
			IP:                     n.IP,
			RemoteAS:               n.RemoteAS,
			Description:            n.Description,
			UpdateSource:           n.UpdateSource,
			NextHopSelf:            n.NextHopSelf,
			RouteMapIn:             n.RouteMapIn,
			RouteMapOut:            n.RouteMapOut,
			PrefixListIn:           n.PrefixListIn,
			PrefixListOut:          n.PrefixListOut,
			MaximumPrefix:          n.MaximumPrefix,
			MaximumPrefixThreshold: n.MaximumPrefixThreshold,
		})
	}

	// Parse networks
	networks := make([]FortiGateBGPNetwork, 0, len(response.Results.Network))
	for _, net := range response.Results.Network {
		networks = append(networks, FortiGateBGPNetwork{
			ID:            net.ID,
			Prefix:        net.Prefix,
			BackdoorRoute: net.BackdoorRoute,
			RouteMap:      net.RouteMap,
		})
	}

	// Parse aggregate addresses
	aggregates := make([]string, 0, len(response.Results.AggregateAddress))
	for _, agg := range response.Results.AggregateAddress {
		aggregates = append(aggregates, agg.Prefix)
	}

	// Parse redistribution
	redistMap := make(map[string]string)
	for _, redist := range response.Results.Redistribute {
		redistMap[redist.Name] = redist.Status
	}

	return &FortiGateBGPConfig{
		AS:                    response.Results.AS,
		RouterID:              response.Results.RouterID,
		VDOM:                  response.VDOM,
		Neighbors:             neighbors,
		Networks:              networks,
		AggregateAddresses:    aggregates,
		RedistributeConnected: redistMap["connected"],
		RedistributeStatic:    redistMap["static"],
		RedistributeOSPF:      redistMap["ospf"],
	}, nil
}

// ========================================
// FortiGate Security Policy Parsers
// ========================================

// FortiGateFirewallPolicy represents an IPv4 firewall policy
type FortiGateFirewallPolicy struct {
	PolicyID       int
	Name           string
	UUID           string
	VDOM           string
	SourceIntf     []string
	DestIntf       []string
	SourceAddr     []string
	DestAddr       []string
	Service        []string
	Action         string // "accept", "deny", "ipsec"
	Status         string // "enable" or "disable"
	Schedule       string
	NATEnabled     string // "enable" or "disable"
	NATType        string // "ippool", "source-nat"
	Poolname       []string
	LogTraffic     string // "all", "utm", "disable"
	Comments       string
	AVProfile      string
	WebfilterProfile string
	IPSProfile     string
	ApplicationList string
	VoIPProfile    string
	DNSFilterProfile string
	SSHFilterProfile string
}

// ParseFortiGateIPv4Policies parses IPv4 firewall policies
func ParseFortiGateIPv4Policies(jsonData string) ([]FortiGateFirewallPolicy, error) {
	var response struct {
		Results []struct {
			PolicyID int    `json:"policyid"`
			Name     string `json:"name"`
			UUID     string `json:"uuid"`
			Srcintf  []struct {
				Name string `json:"name"`
			} `json:"srcintf"`
			Dstintf []struct {
				Name string `json:"name"`
			} `json:"dstintf"`
			Srcaddr []struct {
				Name string `json:"name"`
			} `json:"srcaddr"`
			Dstaddr []struct {
				Name string `json:"name"`
			} `json:"dstaddr"`
			Service []struct {
				Name string `json:"name"`
			} `json:"service"`
			Action         string `json:"action"`
			Status         string `json:"status"`
			Schedule       string `json:"schedule"`
			NAT            string `json:"nat"`
			Poolname       []struct {
				Name string `json:"name"`
			} `json:"poolname"`
			Logtraffic           string `json:"logtraffic"`
			Comments             string `json:"comments"`
			AVProfile            string `json:"av-profile"`
			WebfilterProfile     string `json:"webfilter-profile"`
			IPSProfile           string `json:"ips-sensor"`
			ApplicationList      string `json:"application-list"`
			VoIPProfile          string `json:"voip-profile"`
			DNSFilterProfile     string `json:"dnsfilter-profile"`
			SSHFilterProfile     string `json:"ssh-filter-profile"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate IPv4 policies: %w", err)
	}

	policies := make([]FortiGateFirewallPolicy, 0, len(response.Results))
	for _, p := range response.Results {
		// Parse source interfaces
		srcIntf := make([]string, 0, len(p.Srcintf))
		for _, intf := range p.Srcintf {
			srcIntf = append(srcIntf, intf.Name)
		}

		// Parse destination interfaces
		dstIntf := make([]string, 0, len(p.Dstintf))
		for _, intf := range p.Dstintf {
			dstIntf = append(dstIntf, intf.Name)
		}

		// Parse source addresses
		srcAddr := make([]string, 0, len(p.Srcaddr))
		for _, addr := range p.Srcaddr {
			srcAddr = append(srcAddr, addr.Name)
		}

		// Parse destination addresses
		dstAddr := make([]string, 0, len(p.Dstaddr))
		for _, addr := range p.Dstaddr {
			dstAddr = append(dstAddr, addr.Name)
		}

		// Parse services
		services := make([]string, 0, len(p.Service))
		for _, svc := range p.Service {
			services = append(services, svc.Name)
		}

		// Parse poolname
		poolname := make([]string, 0, len(p.Poolname))
		for _, pool := range p.Poolname {
			poolname = append(poolname, pool.Name)
		}

		policies = append(policies, FortiGateFirewallPolicy{
			PolicyID:         p.PolicyID,
			Name:             p.Name,
			UUID:             p.UUID,
			VDOM:             response.VDOM,
			SourceIntf:       srcIntf,
			DestIntf:         dstIntf,
			SourceAddr:       srcAddr,
			DestAddr:         dstAddr,
			Service:          services,
			Action:           p.Action,
			Status:           p.Status,
			Schedule:         p.Schedule,
			NATEnabled:       p.NAT,
			Poolname:         poolname,
			LogTraffic:       p.Logtraffic,
			Comments:         p.Comments,
			AVProfile:        p.AVProfile,
			WebfilterProfile: p.WebfilterProfile,
			IPSProfile:       p.IPSProfile,
			ApplicationList:  p.ApplicationList,
			VoIPProfile:      p.VoIPProfile,
			DNSFilterProfile: p.DNSFilterProfile,
			SSHFilterProfile: p.SSHFilterProfile,
		})
	}

	return policies, nil
}

// FortiGateIPv6Policy represents an IPv6 firewall policy
type FortiGateIPv6Policy struct {
	PolicyID   int
	Name       string
	UUID       string
	VDOM       string
	SourceIntf []string
	DestIntf   []string
	SourceAddr []string
	DestAddr   []string
	Service    []string
	Action     string
	Status     string
	Schedule   string
	LogTraffic string
	Comments   string
}

// ParseFortiGateIPv6Policies parses IPv6 firewall policies
func ParseFortiGateIPv6Policies(jsonData string) ([]FortiGateIPv6Policy, error) {
	var response struct {
		Results []struct {
			PolicyID int    `json:"policyid"`
			Name     string `json:"name"`
			UUID     string `json:"uuid"`
			Srcintf  []struct {
				Name string `json:"name"`
			} `json:"srcintf"`
			Dstintf []struct {
				Name string `json:"name"`
			} `json:"dstintf"`
			Srcaddr []struct {
				Name string `json:"name"`
			} `json:"srcaddr"`
			Dstaddr []struct {
				Name string `json:"name"`
			} `json:"dstaddr"`
			Service []struct {
				Name string `json:"name"`
			} `json:"service"`
			Action     string `json:"action"`
			Status     string `json:"status"`
			Schedule   string `json:"schedule"`
			Logtraffic string `json:"logtraffic"`
			Comments   string `json:"comments"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate IPv6 policies: %w", err)
	}

	policies := make([]FortiGateIPv6Policy, 0, len(response.Results))
	for _, p := range response.Results {
		srcIntf := make([]string, 0, len(p.Srcintf))
		for _, intf := range p.Srcintf {
			srcIntf = append(srcIntf, intf.Name)
		}

		dstIntf := make([]string, 0, len(p.Dstintf))
		for _, intf := range p.Dstintf {
			dstIntf = append(dstIntf, intf.Name)
		}

		srcAddr := make([]string, 0, len(p.Srcaddr))
		for _, addr := range p.Srcaddr {
			srcAddr = append(srcAddr, addr.Name)
		}

		dstAddr := make([]string, 0, len(p.Dstaddr))
		for _, addr := range p.Dstaddr {
			dstAddr = append(dstAddr, addr.Name)
		}

		services := make([]string, 0, len(p.Service))
		for _, svc := range p.Service {
			services = append(services, svc.Name)
		}

		policies = append(policies, FortiGateIPv6Policy{
			PolicyID:   p.PolicyID,
			Name:       p.Name,
			UUID:       p.UUID,
			VDOM:       response.VDOM,
			SourceIntf: srcIntf,
			DestIntf:   dstIntf,
			SourceAddr: srcAddr,
			DestAddr:   dstAddr,
			Service:    services,
			Action:     p.Action,
			Status:     p.Status,
			Schedule:   p.Schedule,
			LogTraffic: p.Logtraffic,
			Comments:   p.Comments,
		})
	}

	return policies, nil
}

// FortiGateCentralSNAT represents a Central SNAT policy
type FortiGateCentralSNAT struct {
	PolicyID     int
	UUID         string
	VDOM         string
	SourceIntf   []string
	DestIntf     []string
	OrigAddr     []string
	DestAddr     []string
	NATType      string // "ippool", "dynamic-ip-pool"
	Poolname     []string
	NATIPPool    []string
	Status       string
	Comments     string
}

// ParseFortiGateCentralSNAT parses Central SNAT policies
func ParseFortiGateCentralSNAT(jsonData string) ([]FortiGateCentralSNAT, error) {
	var response struct {
		Results []struct {
			PolicyID int    `json:"policyid"`
			UUID     string `json:"uuid"`
			Srcintf  []struct {
				Name string `json:"name"`
			} `json:"srcintf"`
			Dstintf []struct {
				Name string `json:"name"`
			} `json:"dstintf"`
			OrigAddr []struct {
				Name string `json:"name"`
			} `json:"orig-addr"`
			DstAddr []struct {
				Name string `json:"name"`
			} `json:"dst-addr"`
			NATType string `json:"nat"`
			Poolname []struct {
				Name string `json:"name"`
			} `json:"poolname"`
			IPPool []struct {
				Name string `json:"name"`
			} `json:"ip-pool"`
			Status   string `json:"status"`
			Comments string `json:"comments"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate Central SNAT: %w", err)
	}

	policies := make([]FortiGateCentralSNAT, 0, len(response.Results))
	for _, p := range response.Results {
		srcIntf := make([]string, 0, len(p.Srcintf))
		for _, intf := range p.Srcintf {
			srcIntf = append(srcIntf, intf.Name)
		}

		dstIntf := make([]string, 0, len(p.Dstintf))
		for _, intf := range p.Dstintf {
			dstIntf = append(dstIntf, intf.Name)
		}

		origAddr := make([]string, 0, len(p.OrigAddr))
		for _, addr := range p.OrigAddr {
			origAddr = append(origAddr, addr.Name)
		}

		dstAddr := make([]string, 0, len(p.DstAddr))
		for _, addr := range p.DstAddr {
			dstAddr = append(dstAddr, addr.Name)
		}

		poolname := make([]string, 0, len(p.Poolname))
		for _, pool := range p.Poolname {
			poolname = append(poolname, pool.Name)
		}

		ipPool := make([]string, 0, len(p.IPPool))
		for _, pool := range p.IPPool {
			ipPool = append(ipPool, pool.Name)
		}

		policies = append(policies, FortiGateCentralSNAT{
			PolicyID:   p.PolicyID,
			UUID:       p.UUID,
			VDOM:       response.VDOM,
			SourceIntf: srcIntf,
			DestIntf:   dstIntf,
			OrigAddr:   origAddr,
			DestAddr:   dstAddr,
			NATType:    p.NATType,
			Poolname:   poolname,
			NATIPPool:  ipPool,
			Status:     p.Status,
			Comments:   p.Comments,
		})
	}

	return policies, nil
}

// FortiGateVIP represents a Virtual IP (DNAT)
type FortiGateVIP struct {
	Name       string
	UUID       string
	VDOM       string
	Type       string // "static-nat", "server-load-balance", "dns-translation"
	ExtIP      string
	ExtIntf    string
	MappedIP   []string
	Protocol   string // "tcp", "udp", "sctp", "icmp"
	ExtPort    string
	MappedPort string
	Comment    string
}

// ParseFortiGateVIPs parses Virtual IPs (DNAT configuration)
func ParseFortiGateVIPs(jsonData string) ([]FortiGateVIP, error) {
	var response struct {
		Results []struct {
			Name     string `json:"name"`
			UUID     string `json:"uuid"`
			Type     string `json:"type"`
			ExtIP    string `json:"extip"`
			ExtIntf  string `json:"extintf"`
			Mappedip []struct {
				Range string `json:"range"`
			} `json:"mappedip"`
			Protocol   string `json:"protocol"`
			ExtPort    string `json:"extport"`
			MappedPort string `json:"mappedport"`
			Comment    string `json:"comment"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate VIPs: %w", err)
	}

	vips := make([]FortiGateVIP, 0, len(response.Results))
	for _, v := range response.Results {
		mappedIP := make([]string, 0, len(v.Mappedip))
		for _, ip := range v.Mappedip {
			mappedIP = append(mappedIP, ip.Range)
		}

		vips = append(vips, FortiGateVIP{
			Name:       v.Name,
			UUID:       v.UUID,
			VDOM:       response.VDOM,
			Type:       v.Type,
			ExtIP:      v.ExtIP,
			ExtIntf:    v.ExtIntf,
			MappedIP:   mappedIP,
			Protocol:   v.Protocol,
			ExtPort:    v.ExtPort,
			MappedPort: v.MappedPort,
			Comment:    v.Comment,
		})
	}

	return vips, nil
}

// FortiGateIPPool represents an IP pool for NAT
type FortiGateIPPool struct {
	Name     string
	UUID     string
	VDOM     string
	Type     string // "overload", "one-to-one", "fixed-port-range"
	StartIP  string
	EndIP    string
	Comments string
}

// ParseFortiGateIPPools parses IP pools
func ParseFortiGateIPPools(jsonData string) ([]FortiGateIPPool, error) {
	var response struct {
		Results []struct {
			Name     string `json:"name"`
			UUID     string `json:"uuid"`
			Type     string `json:"type"`
			Startip  string `json:"startip"`
			Endip    string `json:"endip"`
			Comments string `json:"comments"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate IP pools: %w", err)
	}

	pools := make([]FortiGateIPPool, 0, len(response.Results))
	for _, pool := range response.Results {
		pools = append(pools, FortiGateIPPool{
			Name:     pool.Name,
			UUID:     pool.UUID,
			VDOM:     response.VDOM,
			Type:     pool.Type,
			StartIP:  pool.Startip,
			EndIP:    pool.Endip,
			Comments: pool.Comments,
		})
	}

	return pools, nil
}

// ========================================
// FortiGate Object Parsers
// ========================================

// FortiGateAddress represents an address object
type FortiGateAddress struct {
	Name       string
	UUID       string
	VDOM       string
	Type       string // "ipmask", "iprange", "fqdn", "geography", "wildcard"
	Subnet     string // For type=ipmask
	StartIP    string // For type=iprange
	EndIP      string // For type=iprange
	FQDN       string // For type=fqdn
	Country    string // For type=geography
	Wildcard   string // For type=wildcard
	Comment    string
	Associated string // Associated interface
}

// ParseFortiGateAddresses parses address objects
func ParseFortiGateAddresses(jsonData string) ([]FortiGateAddress, error) {
	var response struct {
		Results []struct {
			Name       string `json:"name"`
			UUID       string `json:"uuid"`
			Type       string `json:"type"`
			Subnet     string `json:"subnet"`
			StartIP    string `json:"start-ip"`
			EndIP      string `json:"end-ip"`
			FQDN       string `json:"fqdn"`
			Country    string `json:"country"`
			Wildcard   string `json:"wildcard"`
			Comment    string `json:"comment"`
			Associated string `json:"associated-interface"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate addresses: %w", err)
	}

	addresses := make([]FortiGateAddress, 0, len(response.Results))
	for _, addr := range response.Results {
		addresses = append(addresses, FortiGateAddress{
			Name:       addr.Name,
			UUID:       addr.UUID,
			VDOM:       response.VDOM,
			Type:       addr.Type,
			Subnet:     addr.Subnet,
			StartIP:    addr.StartIP,
			EndIP:      addr.EndIP,
			FQDN:       addr.FQDN,
			Country:    addr.Country,
			Wildcard:   addr.Wildcard,
			Comment:    addr.Comment,
			Associated: addr.Associated,
		})
	}

	return addresses, nil
}

// FortiGateAddressGroup represents an address group
type FortiGateAddressGroup struct {
	Name    string
	UUID    string
	VDOM    string
	Members []string
	Comment string
}

// ParseFortiGateAddressGroups parses address groups
func ParseFortiGateAddressGroups(jsonData string) ([]FortiGateAddressGroup, error) {
	var response struct {
		Results []struct {
			Name    string `json:"name"`
			UUID    string `json:"uuid"`
			Member  []struct {
				Name string `json:"name"`
			} `json:"member"`
			Comment string `json:"comment"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate address groups: %w", err)
	}

	groups := make([]FortiGateAddressGroup, 0, len(response.Results))
	for _, grp := range response.Results {
		members := make([]string, 0, len(grp.Member))
		for _, m := range grp.Member {
			members = append(members, m.Name)
		}

		groups = append(groups, FortiGateAddressGroup{
			Name:    grp.Name,
			UUID:    grp.UUID,
			VDOM:    response.VDOM,
			Members: members,
			Comment: grp.Comment,
		})
	}

	return groups, nil
}

// FortiGateService represents a service object
type FortiGateService struct {
	Name         string
	UUID         string
	VDOM         string
	Category     string // "General", "Web Access", "File Access", etc.
	Protocol     string // "TCP/UDP/SCTP", "ICMP", "ICMP6", "IP"
	TCPPortRange []string
	UDPPortRange []string
	SCTPPortRange []string
	ICMPType     int
	ICMPCode     int
	Protocol_Number int
	Comment      string
}

// ParseFortiGateServices parses service objects
func ParseFortiGateServices(jsonData string) ([]FortiGateService, error) {
	var response struct {
		Results []struct {
			Name     string `json:"name"`
			UUID     string `json:"uuid"`
			Category string `json:"category"`
			Protocol string `json:"protocol"`
			TCPPortRange []struct {
				SrcPort string `json:"src-port-range"`
				DstPort string `json:"dst-port-range"`
			} `json:"tcp-portrange"`
			UDPPortRange []struct {
				SrcPort string `json:"src-port-range"`
				DstPort string `json:"dst-port-range"`
			} `json:"udp-portrange"`
			SCTPPortRange []struct {
				SrcPort string `json:"src-port-range"`
				DstPort string `json:"dst-port-range"`
			} `json:"sctp-portrange"`
			ICMPType        int    `json:"icmptype"`
			ICMPCode        int    `json:"icmpcode"`
			ProtocolNumber  int    `json:"protocol-number"`
			Comment         string `json:"comment"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate services: %w", err)
	}

	services := make([]FortiGateService, 0, len(response.Results))
	for _, svc := range response.Results {
		tcpPorts := make([]string, 0, len(svc.TCPPortRange))
		for _, port := range svc.TCPPortRange {
			tcpPorts = append(tcpPorts, fmt.Sprintf("%s:%s", port.SrcPort, port.DstPort))
		}

		udpPorts := make([]string, 0, len(svc.UDPPortRange))
		for _, port := range svc.UDPPortRange {
			udpPorts = append(udpPorts, fmt.Sprintf("%s:%s", port.SrcPort, port.DstPort))
		}

		sctpPorts := make([]string, 0, len(svc.SCTPPortRange))
		for _, port := range svc.SCTPPortRange {
			sctpPorts = append(sctpPorts, fmt.Sprintf("%s:%s", port.SrcPort, port.DstPort))
		}

		services = append(services, FortiGateService{
			Name:            svc.Name,
			UUID:            svc.UUID,
			VDOM:            response.VDOM,
			Category:        svc.Category,
			Protocol:        svc.Protocol,
			TCPPortRange:    tcpPorts,
			UDPPortRange:    udpPorts,
			SCTPPortRange:   sctpPorts,
			ICMPType:        svc.ICMPType,
			ICMPCode:        svc.ICMPCode,
			Protocol_Number: svc.ProtocolNumber,
			Comment:         svc.Comment,
		})
	}

	return services, nil
}

// FortiGateServiceGroup represents a service group
type FortiGateServiceGroup struct {
	Name    string
	UUID    string
	VDOM    string
	Members []string
	Comment string
}

// ParseFortiGateServiceGroups parses service groups
func ParseFortiGateServiceGroups(jsonData string) ([]FortiGateServiceGroup, error) {
	var response struct {
		Results []struct {
			Name    string `json:"name"`
			UUID    string `json:"uuid"`
			Member  []struct {
				Name string `json:"name"`
			} `json:"member"`
			Comment string `json:"comment"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate service groups: %w", err)
	}

	groups := make([]FortiGateServiceGroup, 0, len(response.Results))
	for _, grp := range response.Results {
		members := make([]string, 0, len(grp.Member))
		for _, m := range grp.Member {
			members = append(members, m.Name)
		}

		groups = append(groups, FortiGateServiceGroup{
			Name:    grp.Name,
			UUID:    grp.UUID,
			VDOM:    response.VDOM,
			Members: members,
			Comment: grp.Comment,
		})
	}

	return groups, nil
}

// ========================================
// FortiGate SD-WAN Parsers
// ========================================

// FortiGateSDWAN represents SD-WAN configuration
type FortiGateSDWAN struct {
	Status      string // "enable" or "disable"
	LoadBalance string // "source-ip-based", "weight-based", "usage-based", "source-dest-ip-based"
	Members     []FortiGateSDWANMember
	Zones       []FortiGateSDWANZone
	HealthCheck []FortiGateSDWANHealthCheck
	Service     []FortiGateSDWANService
}

// FortiGateSDWANMember represents an SD-WAN member interface
type FortiGateSDWANMember struct {
	SeqNum    int
	Interface string
	Zone      string
	Gateway   string
	Source    string
	Gateway6  string
	Source6   string
	Cost      int
	Weight    int
	Priority  int
	Status    string
	Comment   string
}

// FortiGateSDWANZone represents an SD-WAN zone
type FortiGateSDWANZone struct {
	Name string
}

// FortiGateSDWANHealthCheck represents an SD-WAN health check
type FortiGateSDWANHealthCheck struct {
	Name           string
	ProbePackets   string // "disable", "enable"
	AddrMode       string // "ipv4", "ipv6"
	SystemDNS      string // "enable", "disable"
	Server         string
	Protocol       string // "ping", "tcp-echo", "udp-echo", "http", "https", "twamp", "dns"
	Port           int
	Interval       int
	ProbeTimeout   int
	Failtime       int
	Recoverytime   int
	ProbeCount     int
	SecurityMode   string
	Members        []string
}

// FortiGateSDWANService represents an SD-WAN service rule
type FortiGateSDWANService struct {
	ID               int
	Name             string
	Mode             string // "manual", "priority", "sla", "load-balance"
	MinimumSLAMeetMembers int
	DstAddr          []string
	SrcAddr          []string
	Users            []string
	Groups           []string
	Protocol         int
	StartPort        int
	EndPort          int
	PriorityMembers  []string
	Status           string
	Comment          string
}

// ParseFortiGateSDWAN parses SD-WAN configuration
func ParseFortiGateSDWAN(jsonData string) (*FortiGateSDWAN, error) {
	var response struct {
		Results struct {
			Status      string `json:"status"`
			LoadBalance string `json:"load-balance-mode"`
			Members     []struct {
				SeqNum    int    `json:"seq-num"`
				Interface string `json:"interface"`
				Zone      string `json:"zone"`
				Gateway   string `json:"gateway"`
				Source    string `json:"source"`
				Gateway6  string `json:"gateway6"`
				Source6   string `json:"source6"`
				Cost      int    `json:"cost"`
				Weight    int    `json:"weight"`
				Priority  int    `json:"priority"`
				Status    string `json:"status"`
				Comment   string `json:"comment"`
			} `json:"members"`
			Zone []struct {
				Name string `json:"name"`
			} `json:"zone"`
			HealthCheck []struct {
				Name         string `json:"name"`
				ProbePackets string `json:"probe-packets"`
				AddrMode     string `json:"addr-mode"`
				SystemDNS    string `json:"system-dns"`
				Server       string `json:"server"`
				Protocol     string `json:"protocol"`
				Port         int    `json:"port"`
				Interval     int    `json:"interval"`
				ProbeTimeout int    `json:"probe-timeout"`
				Failtime     int    `json:"failtime"`
				Recoverytime int    `json:"recoverytime"`
				ProbeCount   int    `json:"probe-count"`
				SecurityMode string `json:"security-mode"`
				Members      []struct {
					SeqNum int `json:"seq-num"`
				} `json:"members"`
			} `json:"health-check"`
			Service []struct {
				ID                    int    `json:"id"`
				Name                  string `json:"name"`
				Mode                  string `json:"mode"`
				MinimumSLAMeetMembers int    `json:"minimum-sla-meet-members"`
				Dst                   []struct {
					Name string `json:"name"`
				} `json:"dst"`
				Src []struct {
					Name string `json:"name"`
				} `json:"src"`
				Protocol  int `json:"protocol"`
				StartPort int `json:"start-port"`
				EndPort   int `json:"end-port"`
				Priority  []struct {
					SeqNum int `json:"seq-num"`
				} `json:"priority-members"`
				Status  string `json:"status"`
				Comment string `json:"comment"`
			} `json:"service"`
		} `json:"results"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate SD-WAN: %w", err)
	}

	// Parse members
	members := make([]FortiGateSDWANMember, 0, len(response.Results.Members))
	for _, m := range response.Results.Members {
		members = append(members, FortiGateSDWANMember{
			SeqNum:    m.SeqNum,
			Interface: m.Interface,
			Zone:      m.Zone,
			Gateway:   m.Gateway,
			Source:    m.Source,
			Gateway6:  m.Gateway6,
			Source6:   m.Source6,
			Cost:      m.Cost,
			Weight:    m.Weight,
			Priority:  m.Priority,
			Status:    m.Status,
			Comment:   m.Comment,
		})
	}

	// Parse zones
	zones := make([]FortiGateSDWANZone, 0, len(response.Results.Zone))
	for _, z := range response.Results.Zone {
		zones = append(zones, FortiGateSDWANZone{
			Name: z.Name,
		})
	}

	// Parse health checks
	healthChecks := make([]FortiGateSDWANHealthCheck, 0, len(response.Results.HealthCheck))
	for _, hc := range response.Results.HealthCheck {
		memberSeqNums := make([]string, 0, len(hc.Members))
		for _, m := range hc.Members {
			memberSeqNums = append(memberSeqNums, strconv.Itoa(m.SeqNum))
		}

		healthChecks = append(healthChecks, FortiGateSDWANHealthCheck{
			Name:         hc.Name,
			ProbePackets: hc.ProbePackets,
			AddrMode:     hc.AddrMode,
			SystemDNS:    hc.SystemDNS,
			Server:       hc.Server,
			Protocol:     hc.Protocol,
			Port:         hc.Port,
			Interval:     hc.Interval,
			ProbeTimeout: hc.ProbeTimeout,
			Failtime:     hc.Failtime,
			Recoverytime: hc.Recoverytime,
			ProbeCount:   hc.ProbeCount,
			SecurityMode: hc.SecurityMode,
			Members:      memberSeqNums,
		})
	}

	// Parse services
	services := make([]FortiGateSDWANService, 0, len(response.Results.Service))
	for _, svc := range response.Results.Service {
		dstAddr := make([]string, 0, len(svc.Dst))
		for _, dst := range svc.Dst {
			dstAddr = append(dstAddr, dst.Name)
		}

		srcAddr := make([]string, 0, len(svc.Src))
		for _, src := range svc.Src {
			srcAddr = append(srcAddr, src.Name)
		}

		priorityMembers := make([]string, 0, len(svc.Priority))
		for _, p := range svc.Priority {
			priorityMembers = append(priorityMembers, strconv.Itoa(p.SeqNum))
		}

		services = append(services, FortiGateSDWANService{
			ID:                    svc.ID,
			Name:                  svc.Name,
			Mode:                  svc.Mode,
			MinimumSLAMeetMembers: svc.MinimumSLAMeetMembers,
			DstAddr:               dstAddr,
			SrcAddr:               srcAddr,
			Protocol:              svc.Protocol,
			StartPort:             svc.StartPort,
			EndPort:               svc.EndPort,
			PriorityMembers:       priorityMembers,
			Status:                svc.Status,
			Comment:               svc.Comment,
		})
	}

	return &FortiGateSDWAN{
		Status:      response.Results.Status,
		LoadBalance: response.Results.LoadBalance,
		Members:     members,
		Zones:       zones,
		HealthCheck: healthChecks,
		Service:     services,
	}, nil
}

// FortiGateSDWANHealthCheckStatus represents SD-WAN health check operational status
type FortiGateSDWANHealthCheckStatus struct {
	Name    string
	Members []FortiGateSDWANHealthMemberStatus
}

// FortiGateSDWANHealthMemberStatus represents health status of an SD-WAN member
type FortiGateSDWANHealthMemberStatus struct {
	SeqNum       int
	Interface    string
	State        string // "alive", "dead"
	PacketSent   int
	PacketLost   int
	PacketRecv   int
	Latency      float64
	Jitter       float64
	PacketLoss   float64
	SLAUptime    float64
}

// ParseFortiGateSDWANHealthCheck parses SD-WAN health check status
func ParseFortiGateSDWANHealthCheck(jsonData string) ([]FortiGateSDWANHealthCheckStatus, error) {
	var response struct {
		Results []struct {
			Name    string `json:"name"`
			Members []struct {
				SeqNum     int     `json:"seq-num"`
				Interface  string  `json:"interface"`
				State      string  `json:"state"`
				PacketSent int     `json:"packet-sent"`
				PacketLost int     `json:"packet-lost"`
				PacketRecv int     `json:"packet-recv"`
				Latency    float64 `json:"latency"`
				Jitter     float64 `json:"jitter"`
				PacketLoss float64 `json:"packet-loss"`
				SLAUptime  float64 `json:"sla-uptime"`
			} `json:"members"`
		} `json:"results"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate SD-WAN health check: %w", err)
	}

	healthChecks := make([]FortiGateSDWANHealthCheckStatus, 0, len(response.Results))
	for _, hc := range response.Results {
		members := make([]FortiGateSDWANHealthMemberStatus, 0, len(hc.Members))
		for _, m := range hc.Members {
			members = append(members, FortiGateSDWANHealthMemberStatus{
				SeqNum:     m.SeqNum,
				Interface:  m.Interface,
				State:      m.State,
				PacketSent: m.PacketSent,
				PacketLost: m.PacketLost,
				PacketRecv: m.PacketRecv,
				Latency:    m.Latency,
				Jitter:     m.Jitter,
				PacketLoss: m.PacketLoss,
				SLAUptime:  m.SLAUptime,
			})
		}

		healthChecks = append(healthChecks, FortiGateSDWANHealthCheckStatus{
			Name:    hc.Name,
			Members: members,
		})
	}

	return healthChecks, nil
}

// ========================================
// FortiGate VPN Parsers
// ========================================

// FortiGateIPsecPhase1 represents IPsec Phase 1 configuration
type FortiGateIPsecPhase1 struct {
	Name              string
	Type              string // "static", "dynamic", "ddns"
	Interface         string
	LocalGW           string
	RemoteGW          string
	PeerType          string // "any", "one", "dialup", "peer", "peergrp"
	Proposal          []string
	DHGroup           []int
	AuthMethod        string // "psk", "signature"
	PSK               string // Hashed/masked
	Certificate       []string
	IKEVersion        int
	Mode              string // "main", "aggressive"
	Peergrp           string
	NetDevice         string // "enable" or "disable"
	AddRoute          string // "enable" or "disable"
	NATTraversal      string // "enable", "disable", "forced"
	DPD               string // "disable", "on-idle", "on-demand"
	DPDRetryCount     int
	DPDRetryInterval  int
	Comments          string
	VDOM              string
}

// ParseFortiGateIPsecPhase1 parses IPsec Phase 1 configuration
func ParseFortiGateIPsecPhase1(jsonData string) ([]FortiGateIPsecPhase1, error) {
	var response struct {
		Results []struct {
			Name       string `json:"name"`
			Type       string `json:"type"`
			Interface  string `json:"interface"`
			LocalGW    string `json:"local-gw"`
			RemoteGW   string `json:"remote-gw"`
			PeerType   string `json:"peertype"`
			Proposal   string `json:"proposal"`
			DHGroup    string `json:"dhgrp"`
			AuthMethod string `json:"authmethod"`
			PSK        string `json:"psksecret"`
			Certificate []struct {
				Name string `json:"name"`
			} `json:"certificate"`
			IKEVersion       int    `json:"ike-version"`
			Mode             string `json:"mode"`
			Peergrp          string `json:"peergrp"`
			NetDevice        string `json:"net-device"`
			AddRoute         string `json:"add-route"`
			NATTraversal     string `json:"nattraversal"`
			DPD              string `json:"dpd"`
			DPDRetryCount    int    `json:"dpd-retrycount"`
			DPDRetryInterval int    `json:"dpd-retryinterval"`
			Comments         string `json:"comments"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate IPsec Phase 1: %w", err)
	}

	phase1List := make([]FortiGateIPsecPhase1, 0, len(response.Results))
	for _, p1 := range response.Results {
		// Parse proposal list
		proposals := []string{}
		if p1.Proposal != "" {
			proposals = strings.Split(p1.Proposal, " ")
		}

		// Parse DH groups
		dhGroups := []int{}
		if p1.DHGroup != "" {
			for _, dhStr := range strings.Split(p1.DHGroup, " ") {
				if dh, err := strconv.Atoi(dhStr); err == nil {
					dhGroups = append(dhGroups, dh)
				}
			}
		}

		// Parse certificates
		certificates := make([]string, 0, len(p1.Certificate))
		for _, cert := range p1.Certificate {
			certificates = append(certificates, cert.Name)
		}

		phase1List = append(phase1List, FortiGateIPsecPhase1{
			Name:             p1.Name,
			Type:             p1.Type,
			Interface:        p1.Interface,
			LocalGW:          p1.LocalGW,
			RemoteGW:         p1.RemoteGW,
			PeerType:         p1.PeerType,
			Proposal:         proposals,
			DHGroup:          dhGroups,
			AuthMethod:       p1.AuthMethod,
			PSK:              p1.PSK,
			Certificate:      certificates,
			IKEVersion:       p1.IKEVersion,
			Mode:             p1.Mode,
			Peergrp:          p1.Peergrp,
			NetDevice:        p1.NetDevice,
			AddRoute:         p1.AddRoute,
			NATTraversal:     p1.NATTraversal,
			DPD:              p1.DPD,
			DPDRetryCount:    p1.DPDRetryCount,
			DPDRetryInterval: p1.DPDRetryInterval,
			Comments:         p1.Comments,
			VDOM:             response.VDOM,
		})
	}

	return phase1List, nil
}

// FortiGateIPsecPhase2 represents IPsec Phase 2 configuration
type FortiGateIPsecPhase2 struct {
	Name           string
	Phase1Name     string
	Proposal       []string
	DHGroup        []int
	ReplayDetection string // "enable" or "disable"
	PFS            string // "enable" or "disable"
	Keylifeseconds int
	Keylifekbs     int
	SrcAddrType    string // "subnet", "range", "ip", "name"
	DstAddrType    string // "subnet", "range", "ip", "name"
	SrcSubnet      []string
	DstSubnet      []string
	Protocol       int
	SrcPort        int
	DstPort        int
	Comments       string
	VDOM           string
}

// ParseFortiGateIPsecPhase2 parses IPsec Phase 2 configuration
func ParseFortiGateIPsecPhase2(jsonData string) ([]FortiGateIPsecPhase2, error) {
	var response struct {
		Results []struct {
			Name            string `json:"name"`
			Phase1Name      string `json:"phase1name"`
			Proposal        string `json:"proposal"`
			DHGroup         string `json:"dhgrp"`
			ReplayDetection string `json:"replay"`
			PFS             string `json:"pfs"`
			Keylifeseconds  int    `json:"keylifeseconds"`
			Keylifekbs      int    `json:"keylifekbs"`
			SrcAddrType     string `json:"src-addr-type"`
			DstAddrType     string `json:"dst-addr-type"`
			SrcSubnet       []struct {
				Subnet string `json:"subnet"`
			} `json:"src-subnet"`
			DstSubnet []struct {
				Subnet string `json:"subnet"`
			} `json:"dst-subnet"`
			Protocol int    `json:"protocol"`
			SrcPort  int    `json:"src-port"`
			DstPort  int    `json:"dst-port"`
			Comments string `json:"comments"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate IPsec Phase 2: %w", err)
	}

	phase2List := make([]FortiGateIPsecPhase2, 0, len(response.Results))
	for _, p2 := range response.Results {
		// Parse proposals
		proposals := []string{}
		if p2.Proposal != "" {
			proposals = strings.Split(p2.Proposal, " ")
		}

		// Parse DH groups
		dhGroups := []int{}
		if p2.DHGroup != "" {
			for _, dhStr := range strings.Split(p2.DHGroup, " ") {
				if dh, err := strconv.Atoi(dhStr); err == nil {
					dhGroups = append(dhGroups, dh)
				}
			}
		}

		// Parse source subnets
		srcSubnets := make([]string, 0, len(p2.SrcSubnet))
		for _, subnet := range p2.SrcSubnet {
			srcSubnets = append(srcSubnets, subnet.Subnet)
		}

		// Parse destination subnets
		dstSubnets := make([]string, 0, len(p2.DstSubnet))
		for _, subnet := range p2.DstSubnet {
			dstSubnets = append(dstSubnets, subnet.Subnet)
		}

		phase2List = append(phase2List, FortiGateIPsecPhase2{
			Name:            p2.Name,
			Phase1Name:      p2.Phase1Name,
			Proposal:        proposals,
			DHGroup:         dhGroups,
			ReplayDetection: p2.ReplayDetection,
			PFS:             p2.PFS,
			Keylifeseconds:  p2.Keylifeseconds,
			Keylifekbs:      p2.Keylifekbs,
			SrcAddrType:     p2.SrcAddrType,
			DstAddrType:     p2.DstAddrType,
			SrcSubnet:       srcSubnets,
			DstSubnet:       dstSubnets,
			Protocol:        p2.Protocol,
			SrcPort:         p2.SrcPort,
			DstPort:         p2.DstPort,
			Comments:        p2.Comments,
			VDOM:            response.VDOM,
		})
	}

	return phase2List, nil
}

// ========================================
// FortiGate Operational Status Parsers
// ========================================

// FortiGateInterfaceStatus represents operational interface status
type FortiGateInterfaceStatus struct {
	Name       string
	Status     string // "up" or "down"
	Link       string // "up" or "down"
	Speed      int    // Mbps
	Duplex     string // "full" or "half"
	TxPackets  int64
	RxPackets  int64
	TxBytes    int64
	RxBytes    int64
	TxErrors   int64
	RxErrors   int64
	TxDropped  int64
	RxDropped  int64
	Interface  string
	VDOM       string
}

// ParseFortiGateInterfaceStatus parses interface operational status
func ParseFortiGateInterfaceStatus(jsonData string) ([]FortiGateInterfaceStatus, error) {
	var response struct {
		Results []struct {
			Name      string `json:"name"`
			Status    string `json:"status"`
			Link      string `json:"link"`
			Speed     int    `json:"speed"`
			Duplex    string `json:"duplex"`
			TxPackets int64  `json:"tx_packets"`
			RxPackets int64  `json:"rx_packets"`
			TxBytes   int64  `json:"tx_bytes"`
			RxBytes   int64  `json:"rx_bytes"`
			TxErrors  int64  `json:"tx_errors"`
			RxErrors  int64  `json:"rx_errors"`
			TxDropped int64  `json:"tx_dropped"`
			RxDropped int64  `json:"rx_dropped"`
			Interface string `json:"interface"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate interface status: %w", err)
	}

	statuses := make([]FortiGateInterfaceStatus, 0, len(response.Results))
	for _, status := range response.Results {
		statuses = append(statuses, FortiGateInterfaceStatus{
			Name:      status.Name,
			Status:    status.Status,
			Link:      status.Link,
			Speed:     status.Speed,
			Duplex:    status.Duplex,
			TxPackets: status.TxPackets,
			RxPackets: status.RxPackets,
			TxBytes:   status.TxBytes,
			RxBytes:   status.RxBytes,
			TxErrors:  status.TxErrors,
			RxErrors:  status.RxErrors,
			TxDropped: status.TxDropped,
			RxDropped: status.RxDropped,
			Interface: status.Interface,
			VDOM:      response.VDOM,
		})
	}

	return statuses, nil
}

// FortiGateRoute represents a routing table entry
type FortiGateRoute struct {
	Type        string // "connected", "static", "rip", "ospf", "bgp", "isis"
	Destination string // CIDR
	Gateway     string
	Distance    int
	Metric      int
	Age         int
	Interface   string
	VDOM        string
}

// ParseFortiGateRoutingTable parses the routing table
func ParseFortiGateRoutingTable(jsonData string) ([]FortiGateRoute, error) {
	var response struct {
		Results []struct {
			Type        string `json:"type"`
			Destination string `json:"ip_mask"`
			Gateway     string `json:"gateway"`
			Distance    int    `json:"distance"`
			Metric      int    `json:"metric"`
			Age         int    `json:"age"`
			Interface   string `json:"interface"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate routing table: %w", err)
	}

	routes := make([]FortiGateRoute, 0, len(response.Results))
	for _, route := range response.Results {
		routes = append(routes, FortiGateRoute{
			Type:        route.Type,
			Destination: route.Destination,
			Gateway:     route.Gateway,
			Distance:    route.Distance,
			Metric:      route.Metric,
			Age:         route.Age,
			Interface:   route.Interface,
			VDOM:        response.VDOM,
		})
	}

	return routes, nil
}

// FortiGateBGPNeighborStatus represents BGP neighbor operational status
type FortiGateBGPNeighborStatus struct {
	NeighborIP       string
	RemoteAS         int
	State            string // "Established", "Active", "Idle", etc.
	UptimeSeconds    int
	PrefixReceived   int
	PrefixAccepted   int
	PrefixAdvertised int
	LocalIP          string
	LocalAS          int
	UpdateSource     string
	VDOM             string
}

// ParseFortiGateBGPNeighbors parses BGP neighbor status
func ParseFortiGateBGPNeighbors(jsonData string) ([]FortiGateBGPNeighborStatus, error) {
	var response struct {
		Results []struct {
			NeighborIP       string `json:"neighbor_ip"`
			RemoteAS         int    `json:"remote_as"`
			State            string `json:"state"`
			UptimeSeconds    int    `json:"uptime"`
			PrefixReceived   int    `json:"received_prefix_count"`
			PrefixAccepted   int    `json:"accepted_prefix_count"`
			PrefixAdvertised int    `json:"announced_prefix_count"`
			LocalIP          string `json:"local_ip"`
			LocalAS          int    `json:"local_as"`
			UpdateSource     string `json:"update_source"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate BGP neighbors: %w", err)
	}

	neighbors := make([]FortiGateBGPNeighborStatus, 0, len(response.Results))
	for _, neighbor := range response.Results {
		neighbors = append(neighbors, FortiGateBGPNeighborStatus{
			NeighborIP:       neighbor.NeighborIP,
			RemoteAS:         neighbor.RemoteAS,
			State:            neighbor.State,
			UptimeSeconds:    neighbor.UptimeSeconds,
			PrefixReceived:   neighbor.PrefixReceived,
			PrefixAccepted:   neighbor.PrefixAccepted,
			PrefixAdvertised: neighbor.PrefixAdvertised,
			LocalIP:          neighbor.LocalIP,
			LocalAS:          neighbor.LocalAS,
			UpdateSource:     neighbor.UpdateSource,
			VDOM:             response.VDOM,
		})
	}

	return neighbors, nil
}

// FortiGateVPNStatus represents IPsec VPN tunnel status
type FortiGateVPNStatus struct {
	Name           string
	Type           string // "dialup", "static"
	RemoteGW       string
	LocalGW        string
	Status         string // "up", "down"
	Established    time.Time
	TxBytes        int64
	RxBytes        int64
	TxPackets      int64
	RxPackets      int64
	IncomingSPI    string
	OutgoingSPI    string
	VDOM           string
}

// ParseFortiGateVPNStatus parses VPN tunnel status
func ParseFortiGateVPNStatus(jsonData string) ([]FortiGateVPNStatus, error) {
	var response struct {
		Results []struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			RemoteGW    string `json:"remote_gw"`
			LocalGW     string `json:"local_gw"`
			Status      string `json:"status"`
			Established int64  `json:"established"`
			TxBytes     int64  `json:"tx_bytes"`
			RxBytes     int64  `json:"rx_bytes"`
			TxPackets   int64  `json:"tx_packets"`
			RxPackets   int64  `json:"rx_packets"`
			IncomingSPI string `json:"incoming_spi"`
			OutgoingSPI string `json:"outgoing_spi"`
		} `json:"results"`
		VDOM string `json:"vdom"`
	}

	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		return nil, fmt.Errorf("failed to parse FortiGate VPN status: %w", err)
	}

	vpnStatuses := make([]FortiGateVPNStatus, 0, len(response.Results))
	for _, vpn := range response.Results {
		established := time.Time{}
		if vpn.Established > 0 {
			established = time.Unix(vpn.Established, 0)
		}

		vpnStatuses = append(vpnStatuses, FortiGateVPNStatus{
			Name:        vpn.Name,
			Type:        vpn.Type,
			RemoteGW:    vpn.RemoteGW,
			LocalGW:     vpn.LocalGW,
			Status:      vpn.Status,
			Established: established,
			TxBytes:     vpn.TxBytes,
			RxBytes:     vpn.RxBytes,
			TxPackets:   vpn.TxPackets,
			RxPackets:   vpn.RxPackets,
			IncomingSPI: vpn.IncomingSPI,
			OutgoingSPI: vpn.OutgoingSPI,
			VDOM:        response.VDOM,
		})
	}

	return vpnStatuses, nil
}
