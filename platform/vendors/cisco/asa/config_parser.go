// Package cisco preserves the historical ASA API over the required Rust parser.
package cisco

import (
	"encoding/base64"
	"errors"
	"github.com/mattpainter701/graphlayer/platform/rustengine"
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
	MemberLines      []int
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
	ServiceGroup                string
	ServiceObject               string
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
	ServiceObjects map[string]ServiceObject
	ServiceGroups  map[string]ServiceGroup
	Warnings       []ConfigWarning
	ArtifactSHA256 string
}
type ConfigWarning struct {
	Code, Message string
	Line          int
	ACLName       string
}

// ParseRunningConfig performs native parsing in Rust; source remains transient.
func ParseRunningConfig(input []byte) (*ASAConfig, error) {
	var result ASAConfig
	if err := rustengine.Evaluate("parse_asa_raw", map[string]any{"config_base64": base64.StdEncoding.EncodeToString(input)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *ASAConfig) ExpandAddress(expression string) ([]string, []int, error) {
	var result struct {
		Addresses []string `json:"addresses"`
		Lines     []int    `json:"lines"`
		Error     *string  `json:"error"`
	}
	if err := rustengine.Evaluate("asa.expand_address", map[string]any{"config": c, "expression": expression}, &result); err != nil {
		return nil, nil, err
	}
	if result.Error != nil {
		return nil, result.Lines, errors.New(*result.Error)
	}
	return result.Addresses, result.Lines, nil
}
