package cisco

import (
	"fmt"
	"strconv"
	"strings"
)

// ServiceObject and ServiceGroup retain the common TCP/UDP service grammar.
// Unsupported members are represented only by a bounded static label.
type ServiceObject struct {
	Name                        string
	Protocol                    string
	SourcePort, DestinationPort string
	MemberLine                  int
	Lines                       []int
	UnsupportedLines            []int
}
type ServiceMember struct {
	Kind, Protocol, SourcePort, DestinationPort, Reference string
	ReferenceKind                                          string
	Line                                                   int
}
type ServiceGroup struct {
	Name             string
	Protocol         string
	Members          []ServiceMember
	Lines            []int
	UnsupportedLines []int
}

func parseServiceDefinitions(c *ASAConfig) {
	var currentKind, currentName, groupProtocol string
	for _, l := range c.Lines {
		f := strings.Fields(strings.TrimSpace(l.Text))
		if len(f) == 0 {
			continue
		}
		if len(f) >= 3 && f[0] == "object" && f[1] == "service" {
			currentKind, currentName, groupProtocol = "object", f[2], ""
			o := c.ServiceObjects[currentName]
			o.Name = currentName
			o.Lines = append(o.Lines, l.Number)
			c.ServiceObjects[currentName] = o
			continue
		}
		if len(f) >= 3 && f[0] == "object-group" && f[1] == "service" {
			currentKind, currentName = "group", f[2]
			groupProtocol = ""
			if len(f) > 3 {
				groupProtocol = strings.ToLower(f[3])
			}
			g := c.ServiceGroups[currentName]
			g.Name = currentName
			g.Protocol = groupProtocol
			g.Lines = append(g.Lines, l.Number)
			c.ServiceGroups[currentName] = g
			continue
		}
		if !strings.HasPrefix(l.Text, " ") && !strings.HasPrefix(l.Text, "\t") {
			currentKind, currentName = "", ""
			continue
		}
		if currentName == "" {
			continue
		}
		switch currentKind {
		case "object":
			o := c.ServiceObjects[currentName]
			o.Lines = append(o.Lines, l.Number)
			if len(f) >= 2 && f[0] == "service" {
				p, sp, dp, ok := parseServiceSpec(f[1:], false)
				if ok {
					o.Protocol, o.SourcePort, o.DestinationPort = p, sp, dp
					o.MemberLine = l.Number
				} else {
					o.UnsupportedLines = append(o.UnsupportedLines, l.Number)
				}
			} else if !isHarmlessObjectMetadata(f) {
				o.UnsupportedLines = append(o.UnsupportedLines, l.Number)
			}
			c.ServiceObjects[currentName] = o
		case "group":
			g := c.ServiceGroups[currentName]
			g.Lines = append(g.Lines, l.Number)
			m := ServiceMember{Line: l.Number}
			extraMembers := []ServiceMember{}
			switch {
			case len(f) >= 2 && f[0] == "group-object":
				m.Kind, m.Reference, m.ReferenceKind = "reference", f[1], "service_group"
			case len(f) >= 2 && f[0] == "service-object":
				if g.Protocol != "" {
					m.Kind = "unsupported"
				} else if strings.ToLower(f[1]) == "object" && len(f) >= 3 {
					m.Kind, m.Reference, m.ReferenceKind = "reference", f[2], "service_object"
				} else if strings.ToLower(f[1]) == "tcp" || strings.ToLower(f[1]) == "udp" || strings.ToLower(f[1]) == "tcp-udp" {
					p, sp, dp, ok := parseServiceSpec(f[1:], true)
					if ok {
						m.Kind, m.Protocol, m.SourcePort, m.DestinationPort = "service", p, sp, dp
					} else {
						m.Kind = "unsupported"
					}
				} else {
					m.Kind = "unsupported"
				}
			case len(f) >= 3 && f[0] == "port-object":
				ports, _, ok := parsePortObjectAlternatives(f[1:], g.Protocol)
				if ok && (g.Protocol == "tcp" || g.Protocol == "udp" || g.Protocol == "tcp-udp") && len(ports) > 0 {
					m.Kind, m.Protocol, m.DestinationPort = "service", g.Protocol, ports[0]
					for _, p := range ports[1:] {
						extraMembers = append(extraMembers, ServiceMember{Kind: "service", Protocol: g.Protocol, DestinationPort: p, Line: l.Number})
					}
				} else {
					m.Kind = "unsupported"
				}
			default:
				if !isHarmlessObjectMetadata(f) {
					m.Kind = "unsupported"
				}
			}
			if m.Kind == "unsupported" {
				g.UnsupportedLines = append(g.UnsupportedLines, l.Number)
			}
			if m.Kind != "" {
				g.Members = append(g.Members, m)
			}
			g.Members = append(g.Members, extraMembers...)
			c.ServiceGroups[currentName] = g
		}
	}
}

func parseServiceSpec(f []string, allowTCPUDP bool) (protocol, source, destination string, ok bool) {
	if len(f) == 0 {
		return "", "", "", false
	}
	protocol = strings.ToLower(f[0])
	if protocol == "tcp-udp" {
		if !allowTCPUDP {
			return "", "", "", false
		}
	} else if protocol != "tcp" && protocol != "udp" {
		return "", "", "", false
	}
	i := 1
	for i < len(f) {
		which := strings.ToLower(f[i])
		if which != "source" && which != "destination" {
			return "", "", "", false
		}
		i++
		v, n, good := parsePortOperatorFor(f[i:], protocol)
		if !good {
			return "", "", "", false
		}
		i += n
		if which == "source" {
			if source != "" {
				return "", "", "", false
			}
			source = v
		} else {
			if destination != "" {
				return "", "", "", false
			}
			destination = v
		}
	}
	return protocol, source, destination, true
}

func parsePortObjectAlternatives(f []string, protocol string) ([]string, int, bool) {
	if len(f) == 0 || (strings.ToLower(f[0]) != "eq" && strings.ToLower(f[0]) != "range") {
		return nil, 0, false
	}
	return parsePortAlternativesFor(f, protocol)
}
func parsePortOperator(f []string) (string, int, bool) {
	return parsePortOperatorFor(f, "")
}
func parsePortOperatorFor(f []string, protocol string) (string, int, bool) {
	ports, n, ok := parsePortAlternativesFor(f, protocol)
	if !ok {
		return "", n, false
	}
	return strings.Join(ports, "|"), n, true
}
func parsePortAlternativesFor(f []string, protocol string) ([]string, int, bool) {
	if len(f) < 2 {
		return nil, 0, false
	}
	op := strings.ToLower(f[0])
	switch op {
	case "eq":
		p, ok := servicePortFor(protocol, f[1])
		if !ok {
			return nil, 2, false
		}
		return []string{p}, 2, true
	case "range":
		if len(f) < 3 {
			return nil, 0, false
		}
		a, oa := servicePortFor(protocol, f[1])
		b, ob := servicePortFor(protocol, f[2])
		if !oa || !ob {
			return nil, 0, false
		}
		ai, _ := strconv.Atoi(a)
		bi, _ := strconv.Atoi(b)
		if ai > bi {
			return nil, 0, false
		}
		return []string{fmt.Sprintf("%s-%s", a, b)}, 3, true
	case "lt", "gt", "neq":
		p, ok := servicePortFor(protocol, f[1])
		if !ok {
			return nil, 2, false
		}
		v, _ := strconv.Atoi(p)
		out := []string{}
		if op == "lt" || op == "neq" {
			if v > 1 {
				out = append(out, fmt.Sprintf("1-%d", v-1))
			}
		}
		if op == "gt" || op == "neq" {
			if v < 65535 {
				out = append(out, fmt.Sprintf("%d-65535", v+1))
			}
		}
		return out, 2, len(out) > 0
	default:
		return nil, 0, false
	}
}

func servicePortFor(protocol, name string) (string, bool) {
	p, ok := servicePort(name)
	if !ok || protocol == "" {
		return p, ok
	}
	if _, err := strconv.Atoi(name); err == nil {
		return p, true
	}
	n := strings.ToLower(name)
	tcpOnly := map[string]bool{"https": true, "ssh": true, "smtp": true, "ftp": true, "ftp-data": true, "telnet": true, "ldap": true, "ldaps": true, "imap4": true, "pop3": true, "bgp": true, "sqlnet": true}
	udpOnly := map[string]bool{"snmp": true, "snmptrap": true, "tftp": true, "radius": true, "radius-acct": true, "ntp": true, "dns": true, "syslog": true}
	dual := map[string]bool{"http": true, "www": true, "domain": true, "kerberos": true, "sip": true, "tacacs": true}
	if protocol == "tcp-udp" {
		if !dual[n] {
			return "", false
		}
		return p, true
	}
	if protocol == "tcp" && (udpOnly[n] || !(tcpOnly[n] || dual[n])) {
		return "", false
	}
	if protocol == "udp" && (tcpOnly[n] || !(udpOnly[n] || dual[n])) {
		return "", false
	}
	return p, true
}
