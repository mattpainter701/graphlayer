package integration

import (
	"context"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/orchestration/orchestration"
)

func TestParserBridgeRoutesFilesToConfiguredParsers(t *testing.T) {
	t.Parallel()

	var (
		configFiles      []string
		operationalFiles []string
	)

	bridge := NewParserBridge(nil)
	bridge.configParser = func(ctx context.Context, path string) error {
		configFiles = append(configFiles, path)
		return nil
	}
	bridge.operationalParser = func(ctx context.Context, path string) error {
		operationalFiles = append(operationalFiles, path)
		return nil
	}

	result := &orchestration.CollectionResult{
		ConfigFiles: []string{
			"/tmp/device_running_config.txt",
			"/tmp/device_show_interfaces.txt",
		},
	}

	if err := bridge.ParseCollectionResult(context.Background(), result); err != nil {
		t.Fatalf("ParseCollectionResult returned error: %v", err)
	}

	if len(configFiles) != 1 || configFiles[0] != "/tmp/device_running_config.txt" {
		t.Fatalf("expected config parser to receive running config, got %#v", configFiles)
	}

	if len(operationalFiles) != 1 || operationalFiles[0] != "/tmp/device_show_interfaces.txt" {
		t.Fatalf("expected operational parser to receive show command output, got %#v", operationalFiles)
	}
}

func TestParserBridgeParseFilesRejectsUnknownFiles(t *testing.T) {
	t.Parallel()

	bridge := NewParserBridge(nil)

	err := bridge.ParseFiles(context.Background(), []string{"unclassified.log"})
	if err == nil {
		t.Fatalf("expected error for unknown file type")
	}
}
