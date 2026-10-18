// graphlayer-policy replays a policy bundle against one explicit flow.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mattpainter701/graphlayer/platform/policy"
)

const maxInputBytes = 8 << 20

func main() {
	bundlePath := flag.String("bundle", "", "path to policy bundle JSON")
	flowPath := flag.String("flow", "", "path to flow JSON")
	flag.Parse()
	if *bundlePath == "" || *flowPath == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: graphlayer-policy --bundle PATH --flow PATH")
		os.Exit(2)
	}
	var b policy.Bundle
	var f policy.Flow
	if err := readStrict(*bundlePath, &b); err != nil {
		fmt.Fprintf(os.Stderr, "invalid bundle: %v\n", err)
		os.Exit(2)
	}
	if err := readStrict(*flowPath, &f); err != nil {
		fmt.Fprintf(os.Stderr, "invalid flow: %v\n", err)
		os.Exit(2)
	}
	r, err := policy.Evaluate(b, f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid input: %v\n", err)
		os.Exit(2)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		fmt.Fprintln(os.Stderr, "could not encode result")
		os.Exit(1)
	}
}

func readStrict(path string, dst any) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.New("cannot open input file")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxInputBytes+1))
	if err != nil {
		return errors.New("cannot read input file")
	}
	if len(data) > maxInputBytes {
		return errors.New("input exceeds 8 MiB limit")
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("JSON object required")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON structure")
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	if _, ok := dst.(*policy.Bundle); ok {
		if err := validateBundleJSONPresence(data); err != nil {
			return err
		}
	}
	return nil
}

// validateBundleJSONPresence checks fields whose zero values are meaningful
// to policy evaluation. Keep DisallowUnknownFields above as the shape guard.
func validateBundleJSONPresence(data []byte) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return errors.New("invalid bundle JSON object")
	}
	complete, ok := object["complete"]
	if !ok || bytes.Equal(bytes.TrimSpace(complete), []byte("null")) {
		return errors.New("bundle complete field is required")
	}
	rulesRaw, ok := object["rules"]
	if !ok || len(bytes.TrimSpace(rulesRaw)) == 0 || bytes.TrimSpace(rulesRaw)[0] != '[' {
		return errors.New("bundle rules array is required")
	}
	var rules []json.RawMessage
	if err := json.Unmarshal(rulesRaw, &rules); err != nil {
		return errors.New("invalid bundle rules array")
	}
	for i, raw := range rules {
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 || trimmed[0] != '{' {
			return fmt.Errorf("bundle rule %d must be an object", i)
		}
		var rule map[string]json.RawMessage
		if err := json.Unmarshal(raw, &rule); err != nil || rule == nil {
			return fmt.Errorf("bundle rule %d must be an object", i)
		}
		for _, field := range []string{"enabled", "position"} {
			value, exists := rule[field]
			if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("bundle rule %d %s field is required", i, field)
			}
		}
	}
	return nil
}
