package firewall

import (
	"fmt"
	"strings"
)

// ParseConfig selects only a positively identified native configuration format.
func ParseConfig(input []byte, vendor string) (*Config, error) {
	if len(input) > 2<<20 {
		return nil, fmt.Errorf("configuration exceeds input budget")
	}
	vendor = strings.ToLower(strings.TrimSpace(vendor))
	if vendor == "" || vendor == "auto" {
		pan, forti, asa := false, false, false
		for _, line := range strings.Split(strings.ReplaceAll(string(input), "\r\n", "\n"), "\n") {
			line = strings.TrimSpace(line)
			pan = pan || strings.HasPrefix(line, "set network ") || strings.HasPrefix(line, "set rulebase ") || strings.HasPrefix(line, "set deviceconfig ")
			forti = forti || strings.HasPrefix(line, "config system interface") || strings.HasPrefix(line, "config firewall ")
			asa = asa || strings.HasPrefix(line, "access-list ") || strings.HasPrefix(line, "access-group ")
		}
		n := 0
		for _, v := range []bool{pan, forti, asa} {
			if v {
				n++
			}
		}
		if n != 1 {
			return nil, fmt.Errorf("configuration format is absent or ambiguous")
		}
		switch {
		case pan:
			vendor = "paloalto-panos"
		case forti:
			vendor = "fortinet-fortigate"
		case asa:
			vendor = "cisco-asa"
		}
	}
	switch vendor {
	case "cisco-asa":
		return ParseASAConfig(input)
	case "paloalto-panos":
		return ParsePANOSConfig(input)
	case "fortinet-fortigate":
		return ParseFortiGateConfig(input)
	default:
		return nil, fmt.Errorf("unsupported configuration format")
	}
}

func SupportedParser(vendor, parser string) bool {
	switch vendor {
	case "cisco-asa":
		return parser == "asa-config-v1" || parser == "asa-config-v2" || parser == ParserVersion
	case "paloalto-panos":
		return parser == PANOSParserVersion
	case "fortinet-fortigate":
		return parser == FortiGateParserVersion
	}
	return false
}
