package models

const (
	CISCO_VIRTUAL_CONTEXT    = "cisco_context"
	FORTINET_VIRTUAL_CONTEXT = "fortinet_vdom"
	PALOALTO_VIRTUAL_CONTEXT = "paloalto_vsys"
)

// VirtualContext defines a vendor-specific logical firewall/routing context.
type VirtualContext struct {
	ID           string       `json:"id" bson:"_id,omitempty"`
	Name         string       `json:"name" bson:"name"`                                   // Human-readable name
	Description  string       `json:"description,omitempty" bson:"description,omitempty"` // Optional notes
	Type         string       `json:"type" bson:"type"`                                   // fortinet_vdom, paloalto_vsys, cisco_context
	ParentDevice string       `json:"parent_device" bson:"parent_device"`                 // Device this context belongs to
	Interfaces   []Interface  `json:"interfaces,omitempty" bson:"interfaces,omitempty"`   // Optional list of interface references
	Metadata     NodeMetadata `json:"metadata,omitempty" bson:"metadata,omitempty"`
}

// Interface defines a physical or virtual entry/exit point on the network.
type Interface struct {
	ID              string   `json:"id" bson:"_id,omitempty"`
	Name            string   `json:"name" bson:"name"`                                             // Interface name (e.g., port1, GigabitEthernet0/1)
	Addresses       []string `json:"addresses" bson:"addresses"`                                   // One or more IP addresses (IPv4 or IPv6)
	VlanID          int      `json:"vlan_id,omitempty" bson:"vlan_id,omitempty"`                   // VLAN tag if a subinterface
	ParentInterface string   `json:"parent_interface,omitempty" bson:"parent_interface,omitempty"` // Parent interface (e.g., for subinterfaces)
	Role            string   `json:"role" bson:"role"`                                             // lan, wan, dmz, etc.
	Type            string   `json:"type" bson:"type"`                                             // physical, vlan, tunnel, loopback, aggregated
	Status          string   `json:"status" bson:"status"`                                         // up / down / administratively-down
	LinkStatus      string   `json:"link_status" bson:"link_status"`                               // up / down
	Zones           []string `json:"zones" bson:"zones"`                                           // Logical zone assignment
	VirtualFirewall string   `json:"virtual_firewall" bson:"virtual_firewall"`                     // Routing context (e.g., VRF, VDOM, ASA Context)
	PublicFacing    *bool    `json:"public_facing,omitempty" bson:"public_facing,omitempty"`       // Whether interface is externally accessible
	DNSName         string   `json:"dns_name,omitempty" bson:"dns_name,omitempty"`                 // Public DNS name if assigned
	Description     string   `json:"description,omitempty" bson:"description,omitempty"`           // Optional description
}

// Zone groups interfaces into logical security segments.
type Zone struct {
	ID             string   `json:"id" bson:"_id,omitempty"`
	Name           string   `json:"name" bson:"name"`
	VirtualContext string   `json:"virtual_context" bson:"virtual_context"`
	Interfaces     []string `json:"interfaces" bson:"interfaces"`
	Type           string   `json:"type" bson:"type"` // layer3, layer2, etc.
	Description    string   `json:"description" bson:"description"`
	SecurityLevel  *int     `json:"security_level,omitempty" bson:"security_level,omitempty"`
	HasACLs        bool     `json:"has_acls,omitempty" bson:"has_acls,omitempty"`
}

// Address represents a specific host, subnet, or range.
type Address struct {
	ID             string   `json:"id" bson:"_id,omitempty"`
	Name           string   `json:"name" bson:"name"`
	VirtualContext string   `json:"virtual_context" bson:"virtual_context"`
	Value          string   `json:"value" bson:"value"`
	Type           string   `json:"type" bson:"type"`
	Description    string   `json:"description" bson:"description"`
	PublicFacing   *bool    `json:"public_facing,omitempty" bson:"public_facing,omitempty"`
	DNSName        string   `json:"dns_name,omitempty" bson:"dns_name,omitempty"`
	Group          []string `json:"group,omitempty" bson:"group,omitempty"`
	Interface      string   `json:"interface,omitempty" bson:"interface,omitempty"`
}

// AddressGroup defines a logical grouping of multiple address objects.
type AddressGroup struct {
	ID                    string   `json:"id" bson:"_id,omitempty"`
	Name                  string   `json:"name" bson:"name"`
	VirtualContext        string   `json:"virtual_context" bson:"virtual_context"`
	Members               []string `json:"members" bson:"members"`
	DynamicFilter         string   `json:"dynamic_filter,omitempty" bson:"dynamic_filter,omitempty"`
	Description           string   `json:"description,omitempty" bson:"description,omitempty"`
	ColorTag              string   `json:"color_tag,omitempty" bson:"color_tag,omitempty"`
	IsDynamic             bool     `json:"is_dynamic" bson:"is_dynamic"`
	ContainsExternalHosts bool     `json:"contains_external_hosts" bson:"contains_external_hosts"`
}
