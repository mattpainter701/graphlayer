package paloalto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParsePANOSSystemInfo tests system information parsing
func TestParsePANOSSystemInfo(t *testing.T) {
	tests := []struct {
		name          string
		xmlData       string
		expectedError bool
		validate      func(t *testing.T, info PANOSSystemInfo)
	}{
		{
			name: "Valid system info",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <system>
      <hostname>fw-prod-01</hostname>
      <ip-address>192.168.1.1</ip-address>
      <model>PA-3020</model>
      <serial>015351000012345</serial>
      <sw-version>10.2.3</sw-version>
      <uptime>45 days, 3:22:15</uptime>
    </system>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, info PANOSSystemInfo) {
				assert.Equal(t, "fw-prod-01", info.Hostname)
				assert.Equal(t, "192.168.1.1", info.IPAddress)
				assert.Equal(t, "PA-3020", info.Model)
				assert.Equal(t, "015351000012345", info.Serial)
				assert.Equal(t, "10.2.3", info.SWVersion)
				assert.Equal(t, "45 days, 3:22:15", info.Uptime)
			},
		},
		{
			name: "PA-220 firewall",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <system>
      <hostname>branch-fw-01</hostname>
      <ip-address>10.50.1.1</ip-address>
      <model>PA-220</model>
      <serial>015351000067890</serial>
      <sw-version>9.1.12</sw-version>
      <uptime>120 days, 5:10:30</uptime>
    </system>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, info PANOSSystemInfo) {
				assert.Equal(t, "branch-fw-01", info.Hostname)
				assert.Equal(t, "PA-220", info.Model)
				assert.Equal(t, "9.1.12", info.SWVersion)
			},
		},
		{
			name: "Error response",
			xmlData: `<?xml version="1.0"?>
<response status="error">
  <result>
    <msg>Invalid API key</msg>
  </result>
</response>`,
			expectedError: true,
		},
		{
			name:          "Invalid XML",
			xmlData:       `<invalid><xml>`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParsePANOSSystemInfo(tt.xmlData)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, info)
			}
		})
	}
}

// TestParsePANOSVirtualSystems tests vsys parsing
func TestParsePANOSVirtualSystems(t *testing.T) {
	tests := []struct {
		name          string
		xmlData       string
		expectedError bool
		validate      func(t *testing.T, vsys []PANOSVirtualSystem)
	}{
		{
			name: "Single vsys",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <vsys>
      <entry name="vsys1">
        <display-name>Production</display-name>
        <description>Production environment</description>
      </entry>
    </vsys>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, vsys []PANOSVirtualSystem) {
				require.Len(t, vsys, 1)
				assert.Equal(t, "vsys1", vsys[0].Name)
				assert.Equal(t, "Production", vsys[0].DisplayName)
				assert.Equal(t, "Production environment", vsys[0].Description)
			},
		},
		{
			name: "Multiple vsys",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <vsys>
      <entry name="vsys1">
        <display-name>Production</display-name>
        <description>Production environment</description>
      </entry>
      <entry name="vsys2">
        <display-name>Development</display-name>
        <description>Dev environment</description>
      </entry>
      <entry name="vsys3">
        <display-name>DMZ</display-name>
        <description>DMZ environment</description>
      </entry>
    </vsys>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, vsys []PANOSVirtualSystem) {
				require.Len(t, vsys, 3)
				assert.Equal(t, "vsys1", vsys[0].Name)
				assert.Equal(t, "vsys2", vsys[1].Name)
				assert.Equal(t, "vsys3", vsys[2].Name)
				assert.Equal(t, "Development", vsys[1].DisplayName)
			},
		},
		{
			name: "Empty vsys list",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <vsys></vsys>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, vsys []PANOSVirtualSystem) {
				assert.Len(t, vsys, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vsys, err := ParsePANOSVirtualSystems(tt.xmlData)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, vsys)
			}
		})
	}
}

// TestParsePANOSZones tests security zone parsing
func TestParsePANOSZones(t *testing.T) {
	tests := []struct {
		name          string
		xmlData       string
		expectedError bool
		validate      func(t *testing.T, zones []PANOSZone)
	}{
		{
			name: "Layer 3 zones",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <zone>
      <entry name="trust">
        <network>
          <layer3>
            <member>ethernet1/1</member>
            <member>ethernet1/2</member>
          </layer3>
        </network>
      </entry>
      <entry name="untrust">
        <network>
          <layer3>
            <member>ethernet1/3</member>
          </layer3>
        </network>
      </entry>
      <entry name="dmz">
        <network>
          <layer3>
            <member>ethernet1/4</member>
            <member>ethernet1/5</member>
          </layer3>
        </network>
      </entry>
    </zone>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, zones []PANOSZone) {
				require.Len(t, zones, 3)

				// Check trust zone
				assert.Equal(t, "trust", zones[0].Name)
				assert.Len(t, zones[0].Network.Layer3, 2)
				assert.Contains(t, zones[0].Network.Layer3, "ethernet1/1")
				assert.Contains(t, zones[0].Network.Layer3, "ethernet1/2")

				// Check untrust zone
				assert.Equal(t, "untrust", zones[1].Name)
				assert.Len(t, zones[1].Network.Layer3, 1)
				assert.Contains(t, zones[1].Network.Layer3, "ethernet1/3")

				// Check dmz zone
				assert.Equal(t, "dmz", zones[2].Name)
				assert.Len(t, zones[2].Network.Layer3, 2)
			},
		},
		{
			name: "Layer 2 zone",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <zone>
      <entry name="vwire-zone">
        <network>
          <layer2>
            <member>ethernet1/6</member>
            <member>ethernet1/7</member>
          </layer2>
        </network>
      </entry>
    </zone>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, zones []PANOSZone) {
				require.Len(t, zones, 1)
				assert.Equal(t, "vwire-zone", zones[0].Name)
				assert.Len(t, zones[0].Network.Layer2, 2)
				assert.Contains(t, zones[0].Network.Layer2, "ethernet1/6")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zones, err := ParsePANOSZones(tt.xmlData)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, zones)
			}
		})
	}
}

// TestParsePANOSInterfaces tests interface parsing
func TestParsePANOSInterfaces(t *testing.T) {
	tests := []struct {
		name          string
		xmlData       string
		expectedError bool
		validate      func(t *testing.T, interfaces []PANOSInterface)
	}{
		{
			name: "Layer 3 ethernet interfaces",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <ethernet>
      <entry name="ethernet1/1">
        <layer3>
          <ip>
            <entry name="10.1.1.1/24"/>
            <entry name="10.1.2.1/24"/>
          </ip>
          <mtu>1500</mtu>
        </layer3>
      </entry>
      <entry name="ethernet1/2">
        <layer3>
          <ip>
            <entry name="192.168.1.1/24"/>
          </ip>
          <mtu>9000</mtu>
        </layer3>
      </entry>
    </ethernet>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, interfaces []PANOSInterface) {
				require.Len(t, interfaces, 2)

				// Check ethernet1/1
				assert.Equal(t, "ethernet1/1", interfaces[0].Name)
				require.NotNil(t, interfaces[0].Layer3)
				assert.Len(t, interfaces[0].Layer3.IP, 2)
				assert.Equal(t, "10.1.1.1/24", interfaces[0].Layer3.IP[0].Address)
				assert.Equal(t, "10.1.2.1/24", interfaces[0].Layer3.IP[1].Address)
				assert.Equal(t, "1500", interfaces[0].Layer3.MTU)

				// Check ethernet1/2
				assert.Equal(t, "ethernet1/2", interfaces[1].Name)
				require.NotNil(t, interfaces[1].Layer3)
				assert.Len(t, interfaces[1].Layer3.IP, 1)
				assert.Equal(t, "9000", interfaces[1].Layer3.MTU)
			},
		},
		{
			name: "Loopback interface",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <loopback>
      <units>
        <entry name="loopback.1">
          <ip>
            <entry name="1.1.1.1/32"/>
          </ip>
        </entry>
      </units>
    </loopback>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, interfaces []PANOSInterface) {
				require.Len(t, interfaces, 1)
				assert.Equal(t, "loopback.1", interfaces[0].Name)
				require.NotNil(t, interfaces[0].Loopback)
				assert.Len(t, interfaces[0].Loopback.IP, 1)
				assert.Equal(t, "1.1.1.1/32", interfaces[0].Loopback.IP[0].Address)
			},
		},
		{
			name: "Tunnel interface",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <tunnel>
      <units>
        <entry name="tunnel.1">
          <ip>
            <entry name="172.16.1.1/30"/>
          </ip>
          <mtu>1400</mtu>
        </entry>
      </units>
    </tunnel>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, interfaces []PANOSInterface) {
				require.Len(t, interfaces, 1)
				assert.Equal(t, "tunnel.1", interfaces[0].Name)
				require.NotNil(t, interfaces[0].Tunnel)
				assert.Len(t, interfaces[0].Tunnel.IP, 1)
				assert.Equal(t, "172.16.1.1/30", interfaces[0].Tunnel.IP[0].Address)
				assert.Equal(t, "1400", interfaces[0].Tunnel.MTU)
			},
		},
		{
			name: "VLAN interface",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <vlan>
      <units>
        <entry name="vlan.100">
          <ip>
            <entry name="10.100.1.1/24"/>
          </ip>
        </entry>
      </units>
    </vlan>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, interfaces []PANOSInterface) {
				require.Len(t, interfaces, 1)
				assert.Equal(t, "vlan.100", interfaces[0].Name)
				require.NotNil(t, interfaces[0].VLAN)
				assert.Len(t, interfaces[0].VLAN.IP, 1)
				assert.Equal(t, "10.100.1.1/24", interfaces[0].VLAN.IP[0].Address)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interfaces, err := ParsePANOSInterfaces(tt.xmlData)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, interfaces)
			}
		})
	}
}

// TestParsePANOSAddressObjects tests address object parsing
func TestParsePANOSAddressObjects(t *testing.T) {
	tests := []struct {
		name          string
		xmlData       string
		expectedError bool
		validate      func(t *testing.T, objects []PANOSAddressObject, groups []PANOSAddressGroup)
	}{
		{
			name: "IP and FQDN addresses",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <address>
      <entry name="webserver-1">
        <ip-netmask>10.1.1.10/32</ip-netmask>
        <description>Production web server</description>
        <tag>
          <member>production</member>
          <member>web</member>
        </tag>
      </entry>
      <entry name="api-server">
        <fqdn>api.example.com</fqdn>
        <description>API endpoint</description>
      </entry>
      <entry name="dhcp-range">
        <ip-range>192.168.1.100-192.168.1.200</ip-range>
        <description>DHCP pool</description>
      </entry>
    </address>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, objects []PANOSAddressObject, groups []PANOSAddressGroup) {
				require.Len(t, objects, 3)

				// Check webserver-1
				assert.Equal(t, "webserver-1", objects[0].Name)
				assert.Equal(t, "10.1.1.10/32", objects[0].IPNetmask)
				assert.Equal(t, "Production web server", objects[0].Description)
				assert.Len(t, objects[0].Tags, 2)
				assert.Contains(t, objects[0].Tags, "production")
				assert.Contains(t, objects[0].Tags, "web")

				// Check api-server
				assert.Equal(t, "api-server", objects[1].Name)
				assert.Equal(t, "api.example.com", objects[1].FQDN)

				// Check dhcp-range
				assert.Equal(t, "dhcp-range", objects[2].Name)
				assert.Equal(t, "192.168.1.100-192.168.1.200", objects[2].IPRange)
			},
		},
		{
			name: "Address groups",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <address>
      <entry name="webserver-1">
        <ip-netmask>10.1.1.10/32</ip-netmask>
      </entry>
      <entry name="webserver-2">
        <ip-netmask>10.1.1.11/32</ip-netmask>
      </entry>
    </address>
    <address-group>
      <entry name="web-servers">
        <static>
          <member>webserver-1</member>
          <member>webserver-2</member>
        </static>
        <description>All web servers</description>
        <tag>
          <member>web</member>
        </tag>
      </entry>
    </address-group>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, objects []PANOSAddressObject, groups []PANOSAddressGroup) {
				require.Len(t, objects, 2)
				require.Len(t, groups, 1)

				// Check address group
				assert.Equal(t, "web-servers", groups[0].Name)
				assert.Len(t, groups[0].Static, 2)
				assert.Contains(t, groups[0].Static, "webserver-1")
				assert.Contains(t, groups[0].Static, "webserver-2")
				assert.Equal(t, "All web servers", groups[0].Description)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects, groups, err := ParsePANOSAddressObjects(tt.xmlData)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, objects, groups)
			}
		})
	}
}

// TestParsePANOSServiceObjects tests service object parsing
func TestParsePANOSServiceObjects(t *testing.T) {
	tests := []struct {
		name          string
		xmlData       string
		expectedError bool
		validate      func(t *testing.T, objects []PANOSServiceObject, groups []PANOSServiceGroup)
	}{
		{
			name: "TCP and UDP services",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <service>
      <entry name="web-alt">
        <protocol>
          <tcp>
            <port>8080</port>
          </tcp>
        </protocol>
        <description>Alternative HTTP port</description>
        <tag>
          <member>web</member>
        </tag>
      </entry>
      <entry name="syslog">
        <protocol>
          <udp>
            <port>514</port>
          </udp>
        </protocol>
      </entry>
      <entry name="ephemeral">
        <protocol>
          <tcp>
            <port>1024-65535</port>
          </tcp>
        </protocol>
        <description>Ephemeral port range</description>
      </entry>
    </service>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, objects []PANOSServiceObject, groups []PANOSServiceGroup) {
				require.Len(t, objects, 3)

				// Check web-alt
				assert.Equal(t, "web-alt", objects[0].Name)
				assert.Equal(t, "tcp", objects[0].Protocol)
				assert.Equal(t, "8080", objects[0].Port)
				assert.Equal(t, "Alternative HTTP port", objects[0].Description)

				// Check syslog
				assert.Equal(t, "syslog", objects[1].Name)
				assert.Equal(t, "udp", objects[1].Protocol)
				assert.Equal(t, "514", objects[1].Port)

				// Check ephemeral
				assert.Equal(t, "ephemeral", objects[2].Name)
				assert.Equal(t, "1024-65535", objects[2].Port)
			},
		},
		{
			name: "Service groups",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <service>
      <entry name="http">
        <protocol>
          <tcp>
            <port>80</port>
          </tcp>
        </protocol>
      </entry>
      <entry name="https">
        <protocol>
          <tcp>
            <port>443</port>
          </tcp>
        </protocol>
      </entry>
    </service>
    <service-group>
      <entry name="web-services">
        <members>
          <member>http</member>
          <member>https</member>
        </members>
        <description>Web protocols</description>
        <tag>
          <member>web</member>
        </tag>
      </entry>
    </service-group>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, objects []PANOSServiceObject, groups []PANOSServiceGroup) {
				require.Len(t, objects, 2)
				require.Len(t, groups, 1)

				// Check service group
				assert.Equal(t, "web-services", groups[0].Name)
				assert.Len(t, groups[0].Members, 2)
				assert.Contains(t, groups[0].Members, "http")
				assert.Contains(t, groups[0].Members, "https")
				assert.Equal(t, "Web protocols", groups[0].Description)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects, groups, err := ParsePANOSServiceObjects(tt.xmlData)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, objects, groups)
			}
		})
	}
}

// TestParsePANOSSecurityPolicies tests security policy parsing
func TestParsePANOSSecurityPolicies(t *testing.T) {
	tests := []struct {
		name          string
		xmlData       string
		expectedError bool
		validate      func(t *testing.T, policies []PANOSSecurityPolicy)
	}{
		{
			name: "Allow policy with App-ID",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <security>
      <rules>
        <entry name="allow-web-traffic" uuid="uuid-12345">
          <from>
            <member>trust</member>
          </from>
          <to>
            <member>untrust</member>
          </to>
          <source>
            <member>10.1.1.0/24</member>
          </source>
          <source-user>
            <member>any</member>
          </source-user>
          <destination>
            <member>any</member>
          </destination>
          <application>
            <member>web-browsing</member>
            <member>ssl</member>
          </application>
          <service>
            <member>application-default</member>
          </service>
          <action>allow</action>
          <log-end>yes</log-end>
          <description>Allow web browsing</description>
          <disabled>no</disabled>
        </entry>
      </rules>
    </security>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, policies []PANOSSecurityPolicy) {
				require.Len(t, policies, 1)
				policy := policies[0]

				assert.Equal(t, "allow-web-traffic", policy.Name)
				assert.Equal(t, "uuid-12345", policy.UUID)
				assert.Contains(t, policy.FromZones, "trust")
				assert.Contains(t, policy.ToZones, "untrust")
				assert.Contains(t, policy.SourceAddresses, "10.1.1.0/24")
				assert.Contains(t, policy.DestinationAddresses, "any")
				assert.Contains(t, policy.Applications, "web-browsing")
				assert.Contains(t, policy.Applications, "ssl")
				assert.Contains(t, policy.Services, "application-default")
				assert.Equal(t, "allow", policy.Action)
				assert.Equal(t, "yes", policy.LogEnd)
				assert.Equal(t, "Allow web browsing", policy.Description)
				assert.Equal(t, "no", policy.Disabled)
			},
		},
		{
			name: "Deny policy with User-ID",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <security>
      <rules>
        <entry name="block-contractors">
          <from>
            <member>trust</member>
          </from>
          <to>
            <member>untrust</member>
          </to>
          <source>
            <member>any</member>
          </source>
          <source-user>
            <member>domain\contractors</member>
          </source-user>
          <destination>
            <member>any</member>
          </destination>
          <application>
            <member>any</member>
          </application>
          <service>
            <member>any</member>
          </service>
          <action>deny</action>
          <log-start>yes</log-start>
        </entry>
      </rules>
    </security>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, policies []PANOSSecurityPolicy) {
				require.Len(t, policies, 1)
				policy := policies[0]

				assert.Equal(t, "block-contractors", policy.Name)
				assert.Contains(t, policy.SourceUsers, "domain\\contractors")
				assert.Equal(t, "deny", policy.Action)
				assert.Equal(t, "yes", policy.LogStart)
			},
		},
		{
			name: "Multiple policies",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <security>
      <rules>
        <entry name="rule-1">
          <from><member>trust</member></from>
          <to><member>untrust</member></to>
          <source><member>any</member></source>
          <source-user><member>any</member></source-user>
          <destination><member>any</member></destination>
          <application><member>any</member></application>
          <service><member>any</member></service>
          <action>allow</action>
        </entry>
        <entry name="rule-2">
          <from><member>dmz</member></from>
          <to><member>trust</member></to>
          <source><member>any</member></source>
          <source-user><member>any</member></source-user>
          <destination><member>any</member></destination>
          <application><member>any</member></application>
          <service><member>any</member></service>
          <action>deny</action>
        </entry>
      </rules>
    </security>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, policies []PANOSSecurityPolicy) {
				require.Len(t, policies, 2)
				assert.Equal(t, "rule-1", policies[0].Name)
				assert.Equal(t, "rule-2", policies[1].Name)
				assert.Equal(t, "allow", policies[0].Action)
				assert.Equal(t, "deny", policies[1].Action)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policies, err := ParsePANOSSecurityPolicies(tt.xmlData)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, policies)
			}
		})
	}
}

// TestParsePANOSNATPolicies tests NAT policy parsing
func TestParsePANOSNATPolicies(t *testing.T) {
	tests := []struct {
		name          string
		xmlData       string
		expectedError bool
		validate      func(t *testing.T, policies []PANOSNATPolicy)
	}{
		{
			name: "Source NAT with dynamic IP",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <nat>
      <rules>
        <entry name="outbound-nat">
          <from>
            <member>trust</member>
          </from>
          <to>
            <member>untrust</member>
          </to>
          <source>
            <member>10.1.1.0/24</member>
          </source>
          <destination>
            <member>any</member>
          </destination>
          <service>any</service>
          <source-translation>
            <dynamic-ip-and-port>
              <translated-address>
                <member>203.0.113.100</member>
              </translated-address>
            </dynamic-ip-and-port>
          </source-translation>
        </entry>
      </rules>
    </nat>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, policies []PANOSNATPolicy) {
				require.Len(t, policies, 1)
				policy := policies[0]

				assert.Equal(t, "outbound-nat", policy.Name)
				assert.Contains(t, policy.FromZones, "trust")
				assert.Contains(t, policy.ToZones, "untrust")
				assert.Contains(t, policy.SourceAddresses, "10.1.1.0/24")
				assert.Contains(t, policy.DestinationAddresses, "any")
				require.NotNil(t, policy.SourceTranslation)
				assert.Equal(t, "dynamic-ip-and-port", policy.SourceTranslation.Type)
				require.NotNil(t, policy.SourceTranslation.DynamicIPAndPort)
				assert.Contains(t, policy.SourceTranslation.DynamicIPAndPort.TranslatedAddress, "203.0.113.100")
			},
		},
		{
			name: "Destination NAT",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <nat>
      <rules>
        <entry name="web-server-dnat">
          <from>
            <member>untrust</member>
          </from>
          <to>
            <member>dmz</member>
          </to>
          <source>
            <member>any</member>
          </source>
          <destination>
            <member>203.0.113.50</member>
          </destination>
          <service>tcp_80</service>
          <destination-translation>
            <translated-address>10.2.2.10</translated-address>
            <translated-port>8080</translated-port>
          </destination-translation>
        </entry>
      </rules>
    </nat>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, policies []PANOSNATPolicy) {
				require.Len(t, policies, 1)
				policy := policies[0]

				assert.Equal(t, "web-server-dnat", policy.Name)
				assert.Contains(t, policy.FromZones, "untrust")
				assert.Contains(t, policy.ToZones, "dmz")
				assert.Contains(t, policy.DestinationAddresses, "203.0.113.50")
				assert.Equal(t, "tcp_80", policy.Service)
				require.NotNil(t, policy.DestinationTranslation)
				assert.Equal(t, "10.2.2.10", policy.DestinationTranslation.TranslatedAddress)
				assert.Equal(t, "8080", policy.DestinationTranslation.TranslatedPort)
			},
		},
		{
			name: "Static IP source NAT",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <nat>
      <rules>
        <entry name="static-nat">
          <from>
            <member>trust</member>
          </from>
          <to>
            <member>untrust</member>
          </to>
          <source>
            <member>10.1.1.100</member>
          </source>
          <destination>
            <member>any</member>
          </destination>
          <service>any</service>
          <source-translation>
            <static-ip>
              <translated-address>203.0.113.200</translated-address>
            </static-ip>
          </source-translation>
        </entry>
      </rules>
    </nat>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, policies []PANOSNATPolicy) {
				require.Len(t, policies, 1)
				policy := policies[0]

				assert.Equal(t, "static-nat", policy.Name)
				require.NotNil(t, policy.SourceTranslation)
				assert.Equal(t, "static-ip", policy.SourceTranslation.Type)
				require.NotNil(t, policy.SourceTranslation.StaticIP)
				assert.Equal(t, "203.0.113.200", policy.SourceTranslation.StaticIP.TranslatedAddress)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policies, err := ParsePANOSNATPolicies(tt.xmlData)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, policies)
			}
		})
	}
}

// TestParsePANOSVirtualRouters tests virtual router parsing
func TestParsePANOSVirtualRouters(t *testing.T) {
	tests := []struct {
		name          string
		xmlData       string
		expectedError bool
		validate      func(t *testing.T, vrs []PANOSVirtualRouter)
	}{
		{
			name: "Virtual router with static routes",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <virtual-router>
      <entry name="default">
        <routing-table>
          <ip>
            <static-route>
              <entry name="default-route">
                <destination>0.0.0.0/0</destination>
                <nexthop>
                  <ip-address>192.168.1.1</ip-address>
                </nexthop>
                <interface>ethernet1/1</interface>
                <metric>10</metric>
              </entry>
              <entry name="internal-route">
                <destination>10.0.0.0/8</destination>
                <nexthop>
                  <ip-address>10.1.1.1</ip-address>
                </nexthop>
                <interface>ethernet1/2</interface>
                <metric>5</metric>
              </entry>
            </static-route>
          </ip>
        </routing-table>
      </entry>
    </virtual-router>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, vrs []PANOSVirtualRouter) {
				require.Len(t, vrs, 1)
				vr := vrs[0]

				assert.Equal(t, "default", vr.Name)
				require.Len(t, vr.StaticRoute, 2)

				// Check default route
				route1 := vr.StaticRoute[0]
				assert.Equal(t, "default-route", route1.Name)
				assert.Equal(t, "0.0.0.0/0", route1.Destination)
				assert.Equal(t, "192.168.1.1", route1.NextHop.IPAddress)
				assert.Equal(t, "ethernet1/1", route1.Interface)
				assert.Equal(t, "10", route1.Metric)

				// Check internal route
				route2 := vr.StaticRoute[1]
				assert.Equal(t, "internal-route", route2.Name)
				assert.Equal(t, "10.0.0.0/8", route2.Destination)
				assert.Equal(t, "10.1.1.1", route2.NextHop.IPAddress)
			},
		},
		{
			name: "Multiple virtual routers",
			xmlData: `<?xml version="1.0"?>
<response status="success">
  <result>
    <virtual-router>
      <entry name="vr-prod">
        <routing-table>
          <ip>
            <static-route>
              <entry name="prod-default">
                <destination>0.0.0.0/0</destination>
                <nexthop>
                  <ip-address>192.168.10.1</ip-address>
                </nexthop>
              </entry>
            </static-route>
          </ip>
        </routing-table>
      </entry>
      <entry name="vr-dev">
        <routing-table>
          <ip>
            <static-route>
              <entry name="dev-default">
                <destination>0.0.0.0/0</destination>
                <nexthop>
                  <ip-address>192.168.20.1</ip-address>
                </nexthop>
              </entry>
            </static-route>
          </ip>
        </routing-table>
      </entry>
    </virtual-router>
  </result>
</response>`,
			expectedError: false,
			validate: func(t *testing.T, vrs []PANOSVirtualRouter) {
				require.Len(t, vrs, 2)
				assert.Equal(t, "vr-prod", vrs[0].Name)
				assert.Equal(t, "vr-dev", vrs[1].Name)
				assert.Equal(t, "192.168.10.1", vrs[0].StaticRoute[0].NextHop.IPAddress)
				assert.Equal(t, "192.168.20.1", vrs[1].StaticRoute[0].NextHop.IPAddress)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vrs, err := ParsePANOSVirtualRouters(tt.xmlData)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, vrs)
			}
		})
	}
}

// TestParsePANOSInterfaceStatus tests interface status parsing
func TestParsePANOSInterfaceStatus(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
  <result>
    <hw>
      <entry>
        <name>ethernet1/1</name>
        <state>up</state>
        <mac>00:1a:2b:3c:4d:5e</mac>
        <speed>1000</speed>
        <duplex>full</duplex>
      </entry>
      <entry>
        <name>ethernet1/2</name>
        <state>down</state>
        <mac>00:1a:2b:3c:4d:5f</mac>
        <speed>auto</speed>
        <duplex>auto</duplex>
      </entry>
    </hw>
  </result>
</response>`

	statuses, err := ParsePANOSInterfaceStatus(xmlData)
	require.NoError(t, err)
	require.Len(t, statuses, 2)

	// Check ethernet1/1
	assert.Equal(t, "ethernet1/1", statuses[0].Name)
	assert.Equal(t, "up", statuses[0].State)
	assert.Equal(t, "00:1a:2b:3c:4d:5e", statuses[0].MAC)
	assert.Equal(t, "1000", statuses[0].Speed)
	assert.Equal(t, "full", statuses[0].Duplex)

	// Check ethernet1/2
	assert.Equal(t, "ethernet1/2", statuses[1].Name)
	assert.Equal(t, "down", statuses[1].State)
}

// TestParsePANOSARPTable tests ARP table parsing
func TestParsePANOSARPTable(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
  <result>
    <entries>
      <entry>
        <interface>ethernet1/1</interface>
        <ip>10.1.1.10</ip>
        <mac>aa:bb:cc:dd:ee:01</mac>
        <status>c</status>
      </entry>
      <entry>
        <interface>ethernet1/1</interface>
        <ip>10.1.1.20</ip>
        <mac>aa:bb:cc:dd:ee:02</mac>
        <status>c</status>
      </entry>
    </entries>
  </result>
</response>`

	arpTable, err := ParsePANOSARPTable(xmlData)
	require.NoError(t, err)
	require.Len(t, arpTable, 2)

	// Check first entry
	assert.Equal(t, "ethernet1/1", arpTable[0].Interface)
	assert.Equal(t, "10.1.1.10", arpTable[0].IP)
	assert.Equal(t, "aa:bb:cc:dd:ee:01", arpTable[0].MAC)
	assert.Equal(t, "c", arpTable[0].Status)

	// Check second entry
	assert.Equal(t, "10.1.1.20", arpTable[1].IP)
	assert.Equal(t, "aa:bb:cc:dd:ee:02", arpTable[1].MAC)
}

// TestParsePANOSRoutingTable tests routing table parsing
func TestParsePANOSRoutingTable(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
  <result>
    <entry>
      <virtual-router>default</virtual-router>
      <destination>0.0.0.0/0</destination>
      <nexthop>192.168.1.1</nexthop>
      <metric>10</metric>
      <flags>A S</flags>
      <interface>ethernet1/1</interface>
      <route-table>unicast</route-table>
    </entry>
    <entry>
      <virtual-router>default</virtual-router>
      <destination>10.1.1.0/24</destination>
      <nexthop>0.0.0.0</nexthop>
      <metric>0</metric>
      <flags>A C</flags>
      <interface>ethernet1/2</interface>
      <route-table>unicast</route-table>
    </entry>
  </result>
</response>`

	routes, err := ParsePANOSRoutingTable(xmlData)
	require.NoError(t, err)
	require.Len(t, routes, 2)

	// Check default route
	assert.Equal(t, "default", routes[0].VirtualRouter)
	assert.Equal(t, "0.0.0.0/0", routes[0].Destination)
	assert.Equal(t, "192.168.1.1", routes[0].NextHop)
	assert.Equal(t, "10", routes[0].Metric)
	assert.Equal(t, "ethernet1/1", routes[0].Interface)

	// Check connected route
	assert.Equal(t, "10.1.1.0/24", routes[1].Destination)
	assert.Equal(t, "0.0.0.0", routes[1].NextHop)
	assert.Equal(t, "0", routes[1].Metric)
}

// TestParsePANOSHAStatus tests HA status parsing
func TestParsePANOSHAStatus(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<response status="success">
  <result>
    <enabled>yes</enabled>
    <group>
      <mode>Active-Passive</mode>
      <local-info>
        <state>active</state>
        <mgmt-ip>192.168.1.10</mgmt-ip>
      </local-info>
      <peer-info>
        <state>passive</state>
        <mgmt-ip>192.168.1.11</mgmt-ip>
      </peer-info>
    </group>
  </result>
</response>`

	haStatus, err := ParsePANOSHAStatus(xmlData)
	require.NoError(t, err)

	assert.Equal(t, "yes", haStatus.Enabled)
	assert.Equal(t, "Active-Passive", haStatus.Group.Mode)
	assert.Equal(t, "active", haStatus.Group.LocalInfo.State)
	assert.Equal(t, "192.168.1.10", haStatus.Group.LocalInfo.MgmtIP)
	assert.Equal(t, "passive", haStatus.Group.PeerInfo.State)
	assert.Equal(t, "192.168.1.11", haStatus.Group.PeerInfo.MgmtIP)
}
