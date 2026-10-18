package cisco

// This file parses the firewall-relevant subset of an ASA running-config. It is
// intentionally conservative: a configuration construct that may affect an
// ACE is retained as an unresolved rule instead of being discarded.
import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
)

const (
	MaxConfigBytes     = 2 << 20
	MaxConfigLines     = 50000
	MaxConfigLineBytes = 8192
	MaxObjectDepth     = 16
	MaxACEExpansion    = 256
	MaxConfigRules     = 10000
)

type ConfigLine struct {
	Number int
	Text   string
}
type AddressObject struct {
	Name             string
	Members          []string
	Lines            []int
	UnsupportedLines []int
	Group            bool
}
type ConfigACE struct {
	ACLName                     string
	Line                        int
	Order                       int
	ExplicitOrder               bool
	Raw                         string
	Action                      string
	Protocol                    string
	Source, Destination         string
	SourcePort, DestinationPort string
	Disabled                    bool
	Enabled                     bool
	Unsupported                 []string
	SourceLines                 []int
}
type ACLBinding struct {
	ACLName, Interface, Direction string
	Line                          int
	Supported                     bool
}
type ASAConfig struct {
	Hostname       string
	Context        string
	Lines          []ConfigLine
	ACLs           map[string][]ConfigACE
	Bindings       []ACLBinding
	Objects        map[string]AddressObject
	Warnings       []ConfigWarning
	ArtifactSHA256 string
}
type ConfigWarning struct {
	Code, Message string
	Line          int
	ACLName       string
}

// ParseRunningConfig parses sanitized-independent source text; callers must
// sanitize before persisting or returning any line content.
func ParseRunningConfig(input []byte) (*ASAConfig, error) {
	if len(input) > MaxConfigBytes {
		return nil, fmt.Errorf("ASA config exceeds %d bytes", MaxConfigBytes)
	}
	text := strings.ReplaceAll(string(input), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > MaxConfigLines {
		return nil, fmt.Errorf("ASA config exceeds %d lines", MaxConfigLines)
	}
	c := &ASAConfig{Context: "default", ACLs: map[string][]ConfigACE{}, Objects: map[string]AddressObject{}, ArtifactSHA256: sha256Hex(input)}
	contextCount, multiContext := 0, false
	for i, line := range lines {
		if len(line) > MaxConfigLineBytes {
			return nil, fmt.Errorf("ASA config line %d exceeds %d bytes", i+1, MaxConfigLineBytes)
		}
		c.Lines = append(c.Lines, ConfigLine{i + 1, line})
	}
	for _, l := range c.Lines {
		f := strings.Fields(strings.TrimSpace(l.Text))
		if len(f) == 0 {
			continue
		}
		switch {
		case len(f) >= 2 && f[0] == "hostname":
			c.Hostname = f[1]
		case len(f) >= 2 && f[0] == "context":
			c.Context = f[1]
			contextCount++
		case len(f) >= 2 && f[0] == "mode" && f[1] == "multiple":
			multiContext = true
		case len(f) >= 2 && f[0] == "access-list":
			if len(f) >= 3 && f[2] == "remark" {
				continue
			}
			acl := f[1]
			ace, ok := parseConfigACE(acl, l)
			if !ok {
				ace = ConfigACE{ACLName: acl, Line: l.Number, Raw: l.Text, Action: "deny", Protocol: "ip", Source: "any", Destination: "any", Unsupported: []string{"vendor_default"}, SourceLines: []int{l.Number}}
				c.Warnings = append(c.Warnings, ConfigWarning{Code: "unsupported_ace", Message: "ACE retained as unresolved", Line: l.Number, ACLName: acl})
			}
			if !ace.ExplicitOrder {
				ace.Order = 1
				for _, prior := range c.ACLs[acl] {
					if prior.Order >= ace.Order {
						ace.Order = prior.Order + 1
					}
				}
			}
			c.ACLs[acl] = append(c.ACLs[acl], ace)
		case len(f) >= 1 && f[0] == "access-group":
			if len(f) < 2 {
				c.Warnings = append(c.Warnings, ConfigWarning{Code: "unsupported_binding", Message: "Malformed access-group directive", Line: l.Number})
				continue
			}
			if len(f) == 5 && (f[2] == "in" || f[2] == "out") && f[3] == "interface" {
				c.Bindings = append(c.Bindings, ACLBinding{ACLName: f[1], Interface: f[4], Direction: f[2], Line: l.Number, Supported: true})
			} else {
				iface, direction := "", "unsupported"
				if len(f) > 2 {
					direction = strings.ToLower(f[2])
				}
				if len(f) > 4 && f[3] == "interface" {
					iface = f[4]
				}
				c.Bindings = append(c.Bindings, ACLBinding{ACLName: f[1], Interface: iface, Direction: direction, Line: l.Number})
				c.Warnings = append(c.Warnings, ConfigWarning{Code: "unsupported_binding", Message: "Unsupported access-group binding retained", Line: l.Number, ACLName: f[1]})
			}
		case len(f) >= 3 && f[0] == "object" && f[1] == "network":
			o := c.Objects[f[2]]
			o.Name = f[2]
			o.Lines = append(o.Lines, l.Number)
			c.Objects[o.Name] = o
		case len(f) >= 3 && f[0] == "object-group" && f[1] == "network":
			o := c.Objects[f[2]]
			o.Name = f[2]
			o.Group = true
			o.Lines = append(o.Lines, l.Number)
			c.Objects[o.Name] = o
		case len(f) >= 2 && (f[0] == "network-object" || f[0] == "group-object" || f[0] == "host" || f[0] == "subnet"):
			// Object body association is tracked using indentation scope below.
		}
	}
	// Resolve indented object bodies, including nested network groups.
	current := ""
	for _, l := range c.Lines {
		trim := strings.TrimSpace(l.Text)
		f := strings.Fields(trim)
		if len(f) == 0 {
			continue
		}
		if len(f) >= 3 && f[0] == "object" && f[1] == "network" {
			current = f[2]
			continue
		}
		if len(f) >= 3 && f[0] == "object-group" && f[1] == "network" {
			current = f[2]
			continue
		}
		if !strings.HasPrefix(l.Text, " ") && !strings.HasPrefix(l.Text, "\t") {
			current = ""
			continue
		}
		if current == "" {
			continue
		}
		o := c.Objects[current]
		var member string
		switch {
		case len(f) >= 2 && f[0] == "host":
			member = f[1]
		case len(f) >= 3 && f[0] == "subnet":
			member = maskToPrefix(f[1], f[2])
		case len(f) >= 3 && f[0] == "network-object" && f[1] == "host":
			member = f[2]
		case len(f) >= 3 && f[0] == "network-object" && f[1] == "object":
			member = "@" + f[2]
		case len(f) >= 3 && f[0] == "network-object" && f[1] == "object-group":
			member = "@" + f[2]
		case len(f) >= 3 && f[0] == "network-object":
			member = maskToPrefix(f[1], f[2])
		case len(f) >= 2 && f[0] == "group-object":
			member = "@" + f[1]
		}
		if member != "" {
			o.Members = append(o.Members, member)
			o.Lines = append(o.Lines, l.Number)
			c.Objects[current] = o
		} else if !isHarmlessObjectMetadata(f) {
			o.UnsupportedLines = append(o.UnsupportedLines, l.Number)
			o.Lines = append(o.Lines, l.Number)
			c.Objects[current] = o
		}
	}
	aclNames := make([]string, 0, len(c.ACLs))
	for name := range c.ACLs {
		aclNames = append(aclNames, name)
	}
	sort.Strings(aclNames)
	for _, aclName := range aclNames {
		aces := c.ACLs[aclName]
		explicit, implicit := 0, 0
		positions := map[int]bool{}
		for _, ace := range aces {
			if ace.ExplicitOrder {
				explicit++
				if positions[ace.Order] {
					c.Warnings = append(c.Warnings, ConfigWarning{Code: "ambiguous_acl_order", Message: "Duplicate explicit ACE sequence numbers; order is ambiguous", Line: ace.Line, ACLName: aclName})
				}
				positions[ace.Order] = true
			} else {
				implicit++
			}
		}
		if explicit > 0 {
			line := aces[0].Line
			message := "Explicit line sequence directives are not replayed; order cannot be established safely"
			if implicit > 0 {
				message = "ACL mixes explicit sequence numbers and unnumbered ACEs; order cannot be established safely"
			}
			c.Warnings = append(c.Warnings, ConfigWarning{Code: "ambiguous_acl_order", Message: message, Line: line, ACLName: aclName})
		}
	}
	if multiContext || contextCount > 1 {
		c.Warnings = append(c.Warnings, ConfigWarning{Code: "ambiguous_context", Message: "Multiple ASA security contexts appear in the export; ACL chains cannot be attributed safely", Line: 0})
	}
	if c.Hostname == "" {
		c.Hostname = "unnamed-asa"
		c.Warnings = append(c.Warnings, ConfigWarning{Code: "missing_hostname", Message: "No hostname was configured; using a local snapshot label", Line: 0})
	}
	return c, nil
}

func parseConfigACE(acl string, l ConfigLine) (ConfigACE, bool) {
	f := strings.Fields(strings.TrimSpace(l.Text))
	if len(f) < 6 || f[0] != "access-list" {
		return ConfigACE{}, false
	}
	base, order := 2, 0
	if f[base] == "line" {
		if len(f) <= base+1 {
			return ConfigACE{}, false
		}
		var err error
		order, err = strconv.Atoi(f[base+1])
		if err != nil || order < 1 {
			return ConfigACE{}, false
		}
		base += 2
	}
	if len(f) <= base+4 || f[base] != "extended" {
		return ConfigACE{}, false
	}
	a := ConfigACE{ACLName: acl, Line: l.Number, Raw: l.Text, Order: order, ExplicitOrder: base > 2, Action: f[base+1], Protocol: strings.ToLower(f[base+2]), Enabled: true, SourceLines: []int{l.Number}}
	if a.Action != "permit" && a.Action != "deny" {
		return ConfigACE{}, false
	}
	if a.Protocol != "tcp" && a.Protocol != "udp" && a.Protocol != "icmp" && a.Protocol != "ip" {
		return unsupportedACE(acl, l), false
	}
	for _, token := range f {
		if strings.EqualFold(token, "not") {
			a.Unsupported = append(a.Unsupported, "negation")
			return a, false
		}
	}
	for _, x := range f {
		if x == "inactive" {
			a.Disabled = true
		}
	}
	i := base + 3
	var ok bool
	a.Source, i, ok = parseAddressExpr(f, i)
	if !ok {
		return unsupportedACE(acl, l), false
	}
	if i < len(f) {
		a.SourcePort, i, ok = parsePortExpr(f, i)
		if !ok {
			return unsupportedACE(acl, l), false
		}
	}
	a.Destination, i, ok = parseAddressExpr(f, i)
	if !ok {
		return unsupportedACE(acl, l), false
	}
	if i < len(f) {
		a.DestinationPort, i, ok = parsePortExpr(f, i)
		if !ok {
			return unsupportedACE(acl, l), false
		}
	}
	for ; i < len(f); i++ {
		switch f[i] {
		case "inactive":
			a.Disabled = true
		case "time-range":
			a.Unsupported = append(a.Unsupported, "schedule")
			i++
		case "log", "log-input":
			// Logging options do not alter the packet match.
		case "established", "fragments", "object-group-search":
			a.Unsupported = append(a.Unsupported, "stateful")
		default:
			return unsupportedACE(acl, l), false
		}
	}
	return a, true
}
func unsupportedACE(acl string, l ConfigLine) ConfigACE {
	return ConfigACE{ACLName: acl, Line: l.Number, Raw: l.Text, Action: "deny", Protocol: "ip", Source: "any", Destination: "any", Unsupported: []string{"vendor_default"}, SourceLines: []int{l.Number}}
}

func isHarmlessObjectMetadata(fields []string) bool {
	if len(fields) == 0 {
		return true
	}
	switch strings.ToLower(fields[0]) {
	case "description", "comment", "tag", "color":
		return true
	default:
		return false
	}
}
func parseAddressExpr(f []string, i int) (string, int, bool) {
	if i >= len(f) {
		return "", i, false
	}
	switch f[i] {
	case "any":
		return "any", i + 1, true
	case "any4", "any6":
		return f[i], i + 1, true
	case "host":
		if i+1 < len(f) {
			return f[i+1], i + 2, true
		}
		return "", i, false
	case "object", "object-group":
		if i+1 < len(f) {
			return "@" + f[i+1], i + 2, true
		}
		return "", i, false
	default:
		if p, e := netip.ParsePrefix(f[i]); e == nil {
			return p.Masked().String(), i + 1, true
		}
		if i+1 < len(f) {
			if p := maskToPrefix(f[i], f[i+1]); p != "" {
				return p, i + 2, true
			}
		}
		if _, e := netip.ParseAddr(f[i]); e == nil {
			return f[i], i + 1, true
		}
		return "@" + f[i], i + 1, true
	}
}
func parsePortExpr(f []string, i int) (string, int, bool) {
	if i >= len(f) {
		return "", i, true
	}
	switch f[i] {
	case "eq":
		if i+1 < len(f) {
			p, ok := servicePort(f[i+1])
			return p, i + 2, ok
		}
		return "", i, false
	case "range":
		if i+2 < len(f) {
			a, oa := servicePort(f[i+1])
			b, ob := servicePort(f[i+2])
			if oa && ob {
				return a + "-" + b, i + 3, true
			}
		}
		return "", i, false
	default:
		return "", i, true
	}
}
func servicePort(s string) (string, bool) {
	if n, e := strconv.Atoi(s); e == nil && n > 0 && n <= 65535 {
		return strconv.Itoa(n), true
	}
	ports := map[string]string{
		"http": "80", "www": "80", "https": "443", "ssh": "22", "domain": "53", "dns": "53",
		"smtp": "25", "ftp": "21", "ftp-data": "20", "telnet": "23", "snmp": "161", "snmptrap": "162",
		"ntp": "123", "tftp": "69", "mysql": "3306", "rdp": "3389", "ldap": "389", "ldaps": "636",
		"imap": "143", "imaps": "993", "pop3": "110", "pop3s": "995", "bgp": "179", "syslog": "514",
		"tacacs": "49", "radius": "1812", "radius-acct": "1813", "sip": "5060", "kerberos": "88",
	}
	p, ok := ports[strings.ToLower(s)]
	return p, ok
}
func maskToPrefix(ip, mask string) string {
	a, e := netip.ParseAddr(ip)
	if e != nil || !a.Is4() {
		return ""
	}
	m, e := netip.ParseAddr(mask)
	if e != nil || !m.Is4() {
		return ""
	}
	v := m.As4()
	ones := 0
	zero := false
	for _, b := range v {
		for bit := 7; bit >= 0; bit-- {
			one := b&(1<<uint(bit)) != 0
			if one && zero {
				return ""
			}
			if one {
				ones++
			} else {
				zero = true
			}
		}
	}
	p, e := netip.ParsePrefix(fmt.Sprintf("%s/%d", ip, ones))
	if e != nil {
		return ""
	}
	return p.Masked().String()
}
func sha256Hex(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// ExpandAddress resolves address objects and nested network groups, preserving
// declaration order and rejecting cycles/depth/expansion overflow.
func (c *ASAConfig) ExpandAddress(expr string) ([]string, []int, error) {
	if !strings.HasPrefix(expr, "@") {
		return []string{expr}, nil, nil
	}
	out := []string{}
	lines := []int{}
	var visit func(string, map[string]bool, int) error
	visit = func(n string, stack map[string]bool, d int) error {
		if d > MaxObjectDepth {
			return fmt.Errorf("network object nesting limit")
		}
		if stack[n] {
			return fmt.Errorf("cyclic network object")
		}
		o, ok := c.Objects[n]
		if !ok {
			return fmt.Errorf("unresolved network object")
		}
		lines = append(lines, o.Lines...)
		if len(o.UnsupportedLines) > 0 {
			return fmt.Errorf("network object contains unsupported members")
		}
		if len(o.Members) == 0 {
			return fmt.Errorf("unresolved network object")
		}
		stack[n] = true
		for _, m := range o.Members {
			if strings.HasPrefix(m, "@") {
				if e := visit(strings.TrimPrefix(m, "@"), stack, d+1); e != nil {
					return e
				}
			} else {
				out = append(out, m)
				if len(out) > MaxACEExpansion {
					return fmt.Errorf("ACE expansion limit")
				}
			}
		}
		delete(stack, n)
		return nil
	}
	if e := visit(strings.TrimPrefix(expr, "@"), map[string]bool{}, 0); e != nil {
		return nil, lines, e
	}
	return out, uniqueInts(lines), nil
}
func uniqueInts(in []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, n := range in {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}
