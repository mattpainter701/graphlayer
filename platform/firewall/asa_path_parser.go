package firewall

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	asa "github.com/mattpainter701/graphlayer/platform/vendors/cisco/asa"
)

const asaPathVersion = "asa-path-v1"

const (
	asaEngineProtocolVersion = "graphlayer-engine-v1"
	asaEngineOperation       = "parse_asa_path"
	asaEngineMaxOutput       = 8 << 20
	maxPathInterfaces        = 1024
	maxPathRoutes            = 4096
	maxPathNATRules          = 4096
)

var (
	asaEngineTimeout  = 10 * time.Second
	asaEngineTestArgs []string
	// ErrAnalysisEngine identifies failures to run or validate the required Rust
	// analysis worker. Callers can distinguish these from invalid ASA input.
	ErrAnalysisEngine = errors.New("required Rust analysis engine is unavailable")
)

//go:embed testdata/asa_path_sample.txt
var samplePathASA []byte

// SamplePathASA returns a copy of the sanitized path-investigation example.
func SamplePathASA() []byte { return append([]byte(nil), samplePathASA...) }

type asaPathEngineLine struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

type asaPathEngineNetworkObject struct {
	Group            bool     `json:"group"`
	Members          []string `json:"members"`
	UnsupportedLines []int    `json:"unsupported_lines"`
}

type asaPathEngineInput struct {
	ArtifactSHA256 string                                `json:"artifact_sha256"`
	Lines          []asaPathEngineLine                   `json:"lines"`
	NetworkObjects map[string]asaPathEngineNetworkObject `json:"network_objects"`
}

type asaPathEngineRequest struct {
	ProtocolVersion string             `json:"protocol_version"`
	Operation       string             `json:"operation"`
	Input           asaPathEngineInput `json:"input"`
}

type asaPathEngineResult struct {
	Path              json.RawMessage `json:"path"`
	Evidence          []Evidence      `json:"evidence"`
	SourceLineUpdates []SourceLine    `json:"source_line_updates"`
}

type asaPathEngineResponse struct {
	ProtocolVersion string          `json:"protocol_version"`
	Operation       string          `json:"operation"`
	Result          json.RawMessage `json:"result"`
	Error           string          `json:"error,omitempty"`
}

// addASAPathInventory delegates ASA forwarding projection to the required Rust
// engine. The Go ASA parser remains responsible for ACL normalization and the
// network object summaries passed across the process boundary.
func addASAPathInventory(c *Config, parsed *asa.ASAConfig) error {
	if c == nil || parsed == nil {
		return errors.New("ASA path engine input is unavailable")
	}
	request := asaPathEngineRequest{
		ProtocolVersion: asaEngineProtocolVersion,
		Operation:       asaEngineOperation,
		Input: asaPathEngineInput{
			ArtifactSHA256: parsed.ArtifactSHA256,
			Lines:          make([]asaPathEngineLine, 0, len(parsed.Lines)),
			NetworkObjects: make(map[string]asaPathEngineNetworkObject, len(parsed.Objects)),
		},
	}
	for _, line := range parsed.Lines {
		request.Input.Lines = append(request.Input.Lines, asaPathEngineLine{Number: line.Number, Text: line.Text})
	}
	for name, object := range parsed.Objects {
		request.Input.NetworkObjects[name] = asaPathEngineNetworkObject{
			Group:            object.Group,
			Members:          append([]string{}, object.Members...),
			UnsupportedLines: append([]int{}, object.UnsupportedLines...),
		}
	}

	response, err := callASAPathEngine(request)
	if err != nil {
		return wrapAnalysisEngineError(err)
	}
	if len(response.Result) == 0 || response.Error != "" {
		return wrapAnalysisEngineError(errors.New("worker returned an invalid response"))
	}
	var result asaPathEngineResult
	if err := decodeStrictJSON(response.Result, &result); err != nil {
		return wrapAnalysisEngineError(errors.New("worker returned an invalid response"))
	}
	if len(result.Path) == 0 {
		return wrapAnalysisEngineError(errors.New("worker returned an invalid response"))
	}
	var path PathConfig
	if err := decodeStrictJSON(result.Path, &path); err != nil || path.Version != asaPathVersion {
		return wrapAnalysisEngineError(errors.New("worker returned an invalid path model"))
	}
	if path.Interfaces == nil || path.Routes == nil || path.NATRules == nil || path.Warnings == nil || path.EvidenceRefs == nil || result.Evidence == nil || result.SourceLineUpdates == nil {
		return wrapAnalysisEngineError(errors.New("worker returned an incomplete path model"))
	}
	if len(path.Interfaces) > maxPathInterfaces || len(path.Routes) > maxPathRoutes || len(path.NATRules) > maxPathNATRules {
		return wrapAnalysisEngineError(errors.New("worker returned an oversized path model"))
	}
	if err := mergeASAPathEngineOutput(c, parsed, path, result); err != nil {
		return wrapAnalysisEngineError(err)
	}
	return nil
}

func wrapAnalysisEngineError(err error) error {
	return fmt.Errorf("%w: %s", ErrAnalysisEngine, err.Error())
}

func mergeASAPathEngineOutput(c *Config, parsed *asa.ASAConfig, path PathConfig, result asaPathEngineResult) error {
	evidenceIDs := make(map[string]bool, len(c.Evidence)+len(result.Evidence))
	for _, item := range c.Evidence {
		evidenceIDs[item.ID] = true
	}
	for _, item := range result.Evidence {
		if item.StartLine < 1 || item.StartLine > len(parsed.Lines) || item.EndLine != item.StartLine || item.ID != evidenceID(parsed.ArtifactSHA256, []int{item.StartLine}) {
			return errors.New("ASA path engine returned invalid evidence")
		}
		if !evidenceIDs[item.ID] {
			c.Evidence = append(c.Evidence, item)
			evidenceIDs[item.ID] = true
		}
	}
	lastLine := 0
	for _, update := range result.SourceLineUpdates {
		if update.Number <= lastLine || update.Number < 1 || update.Number > len(c.SourceLines) {
			return errors.New("ASA path engine returned invalid source line updates")
		}
		lastLine = update.Number
		if c.SourceLines[update.Number-1].Number != update.Number {
			return errors.New("ASA path engine returned invalid source line updates")
		}
		if update.Text != sanitizeLine(parsed.Lines[update.Number-1].Text) && update.Text != "[OMITTED UNSUPPORTED PATH CONFIGURATION]" {
			return errors.New("ASA path engine returned invalid source line updates")
		}
		c.SourceLines[update.Number-1].Text = update.Text
	}
	validateRefs := func(refs []string) bool {
		for _, ref := range refs {
			if !evidenceIDs[ref] {
				return false
			}
		}
		return true
	}
	if !validateRefs(path.EvidenceRefs) {
		return errors.New("ASA path engine returned unresolved evidence references")
	}
	for _, item := range path.Interfaces {
		if !validateRefs(item.EvidenceRefs) {
			return errors.New("ASA path engine returned unresolved evidence references")
		}
	}
	for _, item := range path.Routes {
		if !validateRefs(item.EvidenceRefs) {
			return errors.New("ASA path engine returned unresolved evidence references")
		}
	}
	for _, item := range path.NATRules {
		if !validateRefs(item.EvidenceRefs) {
			return errors.New("ASA path engine returned unresolved evidence references")
		}
	}
	c.Path = &path
	return nil
}

func callASAPathEngine(request asaPathEngineRequest) (asaPathEngineResponse, error) {
	commandPath, err := asaPathEnginePath()
	if err != nil {
		return asaPathEngineResponse{}, err
	}
	input, err := json.Marshal(request)
	if err != nil {
		return asaPathEngineResponse{}, errors.New("ASA path engine request could not be encoded")
	}
	ctx, cancel := context.WithTimeout(context.Background(), asaEngineTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, commandPath, asaEngineTestArgs...)
	cmd.Stdin = bytes.NewReader(input)
	stdout := &asaEngineOutputBuffer{limit: asaEngineMaxOutput}
	cmd.Stdout = stdout
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if stdout.overflow {
			return asaPathEngineResponse{}, errors.New("ASA path engine output exceeds the size limit")
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return asaPathEngineResponse{}, errors.New("ASA path engine timed out")
		}
		return asaPathEngineResponse{}, errors.New("ASA path engine failed")
	}
	if stdout.overflow {
		return asaPathEngineResponse{}, errors.New("ASA path engine output exceeds the size limit")
	}
	var response asaPathEngineResponse
	if err := ensureASAPathProtocol(stdout.Bytes()); err != nil || decodeStrictJSON(stdout.Bytes(), &response) != nil {
		return asaPathEngineResponse{}, errors.New("ASA path engine returned malformed output")
	}
	if response.ProtocolVersion != asaEngineProtocolVersion || response.Operation != asaEngineOperation {
		return asaPathEngineResponse{}, errors.New("ASA path engine protocol mismatch")
	}
	return response, nil
}

func asaPathEnginePath() (string, error) {
	if configured := os.Getenv("GRAPHLAYER_ENGINE"); configured != "" {
		if !filepath.IsAbs(configured) {
			return "", errors.New("ASA path engine is not configured with an absolute path")
		}
		info, err := os.Stat(configured)
		if err != nil || !isRunnableEngineFile(info) {
			return "", errors.New("ASA path engine is unavailable")
		}
		return configured, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", errors.New("ASA path engine is unavailable")
	}
	name := "graphlayer-engine"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	candidate := filepath.Join(filepath.Dir(executable), name)
	info, err := os.Stat(candidate)
	if err != nil || !isRunnableEngineFile(info) {
		return "", errors.New("ASA path engine is unavailable")
	}
	return candidate, nil
}

func isRunnableEngineFile(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() {
		return false
	}
	return runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0
}

type asaEngineOutputBuffer struct {
	buffer   bytes.Buffer
	limit    int
	overflow bool
}

func (b *asaEngineOutputBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *asaEngineOutputBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.overflow = true
		return 0, errors.New("output limit")
	}
	if len(p) > remaining {
		_, _ = b.buffer.Write(p[:remaining])
		b.overflow = true
		return remaining, errors.New("output limit")
	}
	return b.buffer.Write(p)
}

func decodeStrictJSON(data []byte, target any) error {
	if err := rejectDuplicateJSONFields(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("%w: trailing JSON value", err)
	}
	return nil
}

func rejectDuplicateJSONFields(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	first, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := first.(json.Delim); !ok || delimiter != '{' {
		return errors.New("JSON value must be an object")
	}
	if err := consumeJSONContainer(decoder, first, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("JSON contains trailing data")
	}
	return nil
}

func consumeJSONContainer(decoder *json.Decoder, opening json.Token, depth int) error {
	if depth > 128 {
		return errors.New("JSON nesting limit exceeded")
	}
	delimiter, ok := opening.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key is invalid")
			}
			if _, duplicate := seen[key]; duplicate {
				return errors.New("JSON contains duplicate object fields")
			}
			seen[key] = struct{}{}
			value, err := decoder.Token()
			if err != nil {
				return err
			}
			if err := consumeJSONValue(decoder, value, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return errors.New("JSON object is not closed")
		}
	case '[':
		for decoder.More() {
			value, err := decoder.Token()
			if err != nil {
				return err
			}
			if err := consumeJSONValue(decoder, value, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return errors.New("JSON array is not closed")
		}
	default:
		return errors.New("JSON container is invalid")
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder, value json.Token, depth int) error {
	if delimiter, ok := value.(json.Delim); ok {
		return consumeJSONContainer(decoder, delimiter, depth)
	}
	return nil
}

// ensureASAPathProtocol checks byte-level constraints not covered by the
// standard decoder: the subprocess must return one JSON object, then EOF.
func ensureASAPathProtocol(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("invalid JSON object")
	}
	return nil
}
