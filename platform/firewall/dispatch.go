package firewall

import (
	"encoding/base64"
	"github.com/mattpainter701/graphlayer/platform/rustengine"
)

// ParseConfig delegates format detection and normalization to Rust.
func ParseConfig(input []byte, vendor string) (*Config, error) {
	var result Config
	if err := rustengine.Evaluate("parse_config", map[string]any{"vendor": vendor, "config_base64": base64.StdEncoding.EncodeToString(input)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SupportedParser is the persisted wire-version compatibility table.
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
