package firewall

import (
	_ "embed"
	"encoding/base64"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/rustengine"
)

// Versions are persisted compatibility identifiers, preserved across the port.
const ParserVersion = "asa-config-v3"
const PANOSParserVersion = "panos-config-v1"
const FortiGateParserVersion = "fortios-config-v1"
const panosPathVersion = "panos-path-v1"
const fortiPathVersion = "fortios-path-v1"
const fortiMaxBytes = 2 << 20

//go:embed testdata/enterprise.asa.txt
var sampleASA []byte

//go:embed testdata/services.asa.txt
var sampleServicesASA []byte

//go:embed testdata/panos_sample.txt
var panosSample []byte

//go:embed testdata/fortigate_sample.txt
var fortiSample []byte

func SampleASA() []byte         { return append([]byte(nil), sampleASA...) }
func SampleServicesASA() []byte { return append([]byte(nil), sampleServicesASA...) }
func SamplePANOS() []byte       { return []byte(strings.ReplaceAll(string(panosSample), "\r\n", "\n")) }
func SampleFortiGate() []byte   { return []byte(strings.ReplaceAll(string(fortiSample), "\r\n", "\n")) }

func parse(operation string, input []byte) (*Config, error) {
	var result Config
	// Rust hashes the original bytes, including an empty or nil upload.
	if err := rustengine.Evaluate(operation, map[string]any{"config_base64": base64.StdEncoding.EncodeToString(input)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
func ParseASAConfig(input []byte) (*Config, error)       { return parse("parse_asa", input) }
func ParsePANOSConfig(input []byte) (*Config, error)     { return parse("parse_panos", input) }
func ParseFortiGateConfig(input []byte) (*Config, error) { return parse("parse_fortigate", input) }

// These compatibility helpers are used by the earlier forwarding-worker
// adapter; their source and identity decisions are also authoritative in Rust.
func sanitizeLine(text string) string {
	var result struct {
		Text string `json:"text"`
	}
	if rustengine.Evaluate("asa.sanitize_line", map[string]any{"text": text}, &result) != nil {
		return "[REDACTED SENSITIVE CONFIGURATION]"
	}
	return result.Text
}
func evidenceID(hash string, lines []int) string {
	var result struct {
		ID string `json:"id"`
	}
	if rustengine.Evaluate("asa.evidence_id", map[string]any{"hash": hash, "lines": lines}, &result) != nil {
		return ""
	}
	return result.ID
}
