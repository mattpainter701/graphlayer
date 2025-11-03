package storage

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WorkspaceManager handles file storage and change tracking
type WorkspaceManager struct {
	baseDir     string
	trackerFile string
}

// FileMetadata tracks file changes and commits
type FileMetadata struct {
	Filename    string    `json:"filename"`
	Hash        string    `json:"hash"`
	DeviceID    string    `json:"device_id"`
	Workspace   string    `json:"workspace"`
	CommittedAt time.Time `json:"committed_at"`
	ConfigType  string    `json:"config_type"`
}

// WorkspaceTracker tracks committed files per workspace
type WorkspaceTracker struct {
	Workspaces  map[string]*WorkspaceData `json:"workspaces"`
	LastUpdated time.Time                 `json:"last_updated"`
}

// WorkspaceData holds workspace-specific data
type WorkspaceData struct {
	ImportedDevices []string                `json:"imported_devices"`
	Files           map[string]FileMetadata `json:"files"`
}

// NewWorkspaceManager creates a new workspace manager
func NewWorkspaceManager(baseDir string) (*WorkspaceManager, error) {
	if baseDir == "" {
		baseDir = "/mount/connector"
	}

	trackerFile := filepath.Join(baseDir, "uploaded_files.json")

	// Create base directory if not exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &WorkspaceManager{
		baseDir:     baseDir,
		trackerFile: trackerFile,
	}, nil
}

// GetWorkingFolder returns the working folder for a connector
func (wm *WorkspaceManager) GetWorkingFolder(connectorGUID string) string {
	return filepath.Join(wm.baseDir, connectorGUID)
}

// CreateWorkingFolder creates and cleans a working folder
func (wm *WorkspaceManager) CreateWorkingFolder(connectorGUID string) (string, error) {
	folder := wm.GetWorkingFolder(connectorGUID)

	// Remove existing folder
	if err := os.RemoveAll(folder); err != nil {
		return "", fmt.Errorf("failed to remove existing folder: %w", err)
	}

	// Create fresh folder
	if err := os.MkdirAll(folder, 0755); err != nil {
		return "", fmt.Errorf("failed to create working folder: %w", err)
	}

	return folder, nil
}

// CleanupWorkingFolder removes the working folder
func (wm *WorkspaceManager) CleanupWorkingFolder(connectorGUID string) error {
	folder := wm.GetWorkingFolder(connectorGUID)
	return os.RemoveAll(folder)
}

// WriteConfigFile writes configuration data to a file
func (wm *WorkspaceManager) WriteConfigFile(connectorGUID, deviceID, configType, data string) (string, string, error) {
	folder := wm.GetWorkingFolder(connectorGUID)

	// Sanitize device ID for filename
	safeDeviceID := sanitizeFilename(deviceID)
	filename := fmt.Sprintf("%s_%s.txt", safeDeviceID, configType)
	filePath := filepath.Join(folder, filename)

	// Write file
	if err := ioutil.WriteFile(filePath, []byte(data), 0644); err != nil {
		return "", "", fmt.Errorf("failed to write file: %w", err)
	}

	// Calculate hash
	hash := calculateMD5Hash(data)

	return filePath, hash, nil
}

// GetDownloadedFiles returns all files in the working folder
func (wm *WorkspaceManager) GetDownloadedFiles(connectorGUID string) ([]string, error) {
	folder := wm.GetWorkingFolder(connectorGUID)

	var files []string
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}

// ShouldCommitFile checks if a file has changed and should be committed
func (wm *WorkspaceManager) ShouldCommitFile(filePath, workspace string, deviceIDs []string) (bool, error) {
	tracker, err := wm.loadTracker()
	if err != nil {
		// If tracker doesn't exist, commit everything
		return true, nil
	}

	wsData, exists := tracker.Workspaces[workspace]
	if !exists {
		return true, nil
	}

	// Read file and calculate hash
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to read file: %w", err)
	}

	currentHash := calculateMD5Hash(string(data))
	filename := filepath.Base(filePath)

	// Check if file hash has changed
	if fileData, exists := wsData.Files[filename]; exists {
		if fileData.Hash == currentHash {
			// Check if new devices are being imported
			for _, deviceID := range deviceIDs {
				if !contains(wsData.ImportedDevices, deviceID) {
					return true, nil
				}
			}
			return false, nil
		}
	}

	return true, nil
}

// TrackCommittedFile records a file as committed
func (wm *WorkspaceManager) TrackCommittedFile(filePath, workspace, deviceID, configType string) error {
	tracker, err := wm.loadTracker()
	if err != nil {
		tracker = &WorkspaceTracker{
			Workspaces: make(map[string]*WorkspaceData),
		}
	}

	// Read file and calculate hash
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	hash := calculateMD5Hash(string(data))
	filename := filepath.Base(filePath)

	// Initialize workspace if needed
	if _, exists := tracker.Workspaces[workspace]; !exists {
		tracker.Workspaces[workspace] = &WorkspaceData{
			ImportedDevices: []string{},
			Files:           make(map[string]FileMetadata),
		}
	}

	wsData := tracker.Workspaces[workspace]

	// Add device to imported devices
	if !contains(wsData.ImportedDevices, deviceID) {
		wsData.ImportedDevices = append(wsData.ImportedDevices, deviceID)
	}

	// Track file metadata
	wsData.Files[filename] = FileMetadata{
		Filename:    filename,
		Hash:        hash,
		DeviceID:    deviceID,
		Workspace:   workspace,
		CommittedAt: time.Now(),
		ConfigType:  configType,
	}

	tracker.LastUpdated = time.Now()

	return wm.saveTracker(tracker)
}

// loadTracker loads the workspace tracker from disk
func (wm *WorkspaceManager) loadTracker() (*WorkspaceTracker, error) {
	data, err := ioutil.ReadFile(wm.trackerFile)
	if err != nil {
		return nil, err
	}

	var tracker WorkspaceTracker
	if err := json.Unmarshal(data, &tracker); err != nil {
		return nil, err
	}

	return &tracker, nil
}

// saveTracker saves the workspace tracker to disk
func (wm *WorkspaceManager) saveTracker(tracker *WorkspaceTracker) error {
	data, err := json.MarshalIndent(tracker, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tracker: %w", err)
	}

	if err := ioutil.WriteFile(wm.trackerFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write tracker file: %w", err)
	}

	return nil
}

// Helper functions

// calculateMD5Hash calculates MD5 hash of data
func calculateMD5Hash(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// sanitizeFilename replaces invalid characters in filename
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(name)
}

// contains checks if a string slice contains an item
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

