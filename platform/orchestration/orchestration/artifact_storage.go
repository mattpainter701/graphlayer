package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// Artifact represents a stored collection artifact (raw data preserved for reprocessing).
type Artifact struct {
	ID          string    `json:"id"`
	RunID       string    `json:"run_id"`
	ConnectorID string    `json:"connector_id"`
	DeviceID    string    `json:"device_id"`
	FilePath    string    `json:"file_path"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	FileHash    string    `json:"file_hash"`
	ContentType string    `json:"content_type"`
	CollectedAt time.Time `json:"collected_at"`
	StoredAt    time.Time `json:"stored_at"`
	Success     bool      `json:"success"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// ArtifactStore defines the interface for persisting raw collection data.
type ArtifactStore interface {
	// StoreArtifact persists a single artifact for later reprocessing.
	StoreArtifact(ctx context.Context, artifact *Artifact) error

	// GetArtifact retrieves an artifact by ID.
	GetArtifact(ctx context.Context, id string) (*Artifact, error)

	// ListArtifacts returns artifacts matching optional filters.
	ListArtifacts(ctx context.Context, runID, connectorID string, limit, offset int) ([]*Artifact, error)

	// DeleteArtifact removes an artifact by ID.
	DeleteArtifact(ctx context.Context, id string) error

	// PruneArtifacts removes artifacts older than the specified age.
	PruneArtifacts(ctx context.Context, olderThan time.Duration) (int, error)
}

// FileArtifactStore implements ArtifactStore using the local filesystem.
type FileArtifactStore struct {
	baseDir    string
	indexPath  string
}

// artifactIndex is the on-disk index of stored artifacts.
type artifactIndex struct {
	Artifacts map[string]*Artifact `json:"artifacts"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// NewFileArtifactStore creates a filesystem-backed artifact store.
func NewFileArtifactStore(baseDir string) (*FileArtifactStore, error) {
	if baseDir == "" {
		baseDir = "/mount/artifacts"
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create artifact base directory: %w", err)
	}

	store := &FileArtifactStore{
		baseDir:   baseDir,
		indexPath: filepath.Join(baseDir, "artifact_index.json"),
	}

	// Initialize index if it doesn't exist
	if _, err := os.Stat(store.indexPath); os.IsNotExist(err) {
		index := &artifactIndex{
			Artifacts: make(map[string]*Artifact),
			UpdatedAt: time.Now(),
		}
		if err := store.saveIndex(index); err != nil {
			return nil, fmt.Errorf("failed to create artifact index: %w", err)
		}
	}

	return store, nil
}

// StoreArtifact persists a collection artifact to disk and updates the index.
func (fas *FileArtifactStore) StoreArtifact(ctx context.Context, artifact *Artifact) error {
	if artifact == nil {
		return fmt.Errorf("artifact is nil")
	}

	// Read the source file
	data, err := ioutil.ReadFile(artifact.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact source: %w", err)
	}

	// Compute hash
	hash := sha256.Sum256(data)
	artifact.FileHash = hex.EncodeToString(hash[:])
	artifact.FileSize = int64(len(data))
	artifact.StoredAt = time.Now()
	artifact.ContentType = "text/plain"

	// Generate artifact ID if not set
	if artifact.ID == "" {
		artifact.ID = fmt.Sprintf("artifact-%s-%s", artifact.RunID, artifact.FileHash[:12])
	}

	// Determine storage path: baseDir/runID/connectorID/filename
	storageDir := filepath.Join(fas.baseDir, artifact.RunID, artifact.ConnectorID)
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return fmt.Errorf("failed to create artifact storage dir: %w", err)
	}

	artifact.FileName = filepath.Base(artifact.FilePath)
	storagePath := filepath.Join(storageDir, artifact.FileName)

	// Copy data to storage
	if err := ioutil.WriteFile(storagePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write artifact: %w", err)
	}

	artifact.FilePath = storagePath

	// Update index
	index, err := fas.loadIndex()
	if err != nil {
		return fmt.Errorf("failed to load artifact index: %w", err)
	}

	index.Artifacts[artifact.ID] = artifact
	index.UpdatedAt = time.Now()

	if err := fas.saveIndex(index); err != nil {
		return fmt.Errorf("failed to save artifact index: %w", err)
	}

	return nil
}

// GetArtifact retrieves an artifact by its ID.
func (fas *FileArtifactStore) GetArtifact(ctx context.Context, id string) (*Artifact, error) {
	index, err := fas.loadIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to load artifact index: %w", err)
	}

	artifact, exists := index.Artifacts[id]
	if !exists {
		return nil, fmt.Errorf("artifact %q not found", id)
	}

	// Verify the file still exists
	if _, err := os.Stat(artifact.FilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("artifact file missing: %s", artifact.FilePath)
	}

	return artifact, nil
}

// ListArtifacts returns artifacts matching the given filters.
func (fas *FileArtifactStore) ListArtifacts(ctx context.Context, runID, connectorID string, limit, offset int) ([]*Artifact, error) {
	index, err := fas.loadIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to load artifact index: %w", err)
	}

	var results []*Artifact
	for _, artifact := range index.Artifacts {
		if runID != "" && artifact.RunID != runID {
			continue
		}
		if connectorID != "" && artifact.ConnectorID != connectorID {
			continue
		}
		results = append(results, artifact)
	}

	// Apply pagination
	start := offset
	if start > len(results) {
		return []*Artifact{}, nil
	}
	end := start + limit
	if limit <= 0 || end > len(results) {
		end = len(results)
	}

	return results[start:end], nil
}

// DeleteArtifact removes an artifact from disk and the index.
func (fas *FileArtifactStore) DeleteArtifact(ctx context.Context, id string) error {
	index, err := fas.loadIndex()
	if err != nil {
		return fmt.Errorf("failed to load artifact index: %w", err)
	}

	artifact, exists := index.Artifacts[id]
	if !exists {
		return fmt.Errorf("artifact %q not found", id)
	}

	// Remove file
	if err := os.Remove(artifact.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove artifact file: %w", err)
	}

	// Remove from index
	delete(index.Artifacts, id)
	index.UpdatedAt = time.Now()

	return fas.saveIndex(index)
}

// PruneArtifacts removes artifacts older than the specified duration.
func (fas *FileArtifactStore) PruneArtifacts(ctx context.Context, olderThan time.Duration) (int, error) {
	index, err := fas.loadIndex()
	if err != nil {
		return 0, fmt.Errorf("failed to load artifact index: %w", err)
	}

	cutoff := time.Now().Add(-olderThan)
	pruned := 0

	for id, artifact := range index.Artifacts {
		if artifact.StoredAt.Before(cutoff) {
			// Remove file
			os.Remove(artifact.FilePath) // best effort
			delete(index.Artifacts, id)
			pruned++
		}
	}

	if pruned > 0 {
		index.UpdatedAt = time.Now()
		if err := fas.saveIndex(index); err != nil {
			return pruned, fmt.Errorf("failed to save index after pruning: %w", err)
		}
	}

	return pruned, nil
}

// loadIndex loads the artifact index from disk.
func (fas *FileArtifactStore) loadIndex() (*artifactIndex, error) {
	data, err := ioutil.ReadFile(fas.indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &artifactIndex{Artifacts: make(map[string]*Artifact)}, nil
		}
		return nil, err
	}

	var index artifactIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse artifact index: %w", err)
	}

	if index.Artifacts == nil {
		index.Artifacts = make(map[string]*Artifact)
	}

	return &index, nil
}

// saveIndex persists the artifact index to disk.
func (fas *FileArtifactStore) saveIndex(index *artifactIndex) error {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal artifact index: %w", err)
	}
	return ioutil.WriteFile(fas.indexPath, data, 0644)
}

// MemoryArtifactStore implements ArtifactStore in-memory (for testing).
type MemoryArtifactStore struct {
	artifacts map[string]*Artifact
}

// NewMemoryArtifactStore creates an in-memory artifact store.
func NewMemoryArtifactStore() *MemoryArtifactStore {
	return &MemoryArtifactStore{
		artifacts: make(map[string]*Artifact),
	}
}

func (mas *MemoryArtifactStore) StoreArtifact(ctx context.Context, artifact *Artifact) error {
	if artifact.ID == "" {
		artifact.ID = fmt.Sprintf("mem-%d", len(mas.artifacts))
	}
	if artifact.StoredAt.IsZero() {
		artifact.StoredAt = time.Now()
	}
	mas.artifacts[artifact.ID] = artifact
	return nil
}

func (mas *MemoryArtifactStore) GetArtifact(ctx context.Context, id string) (*Artifact, error) {
	a, exists := mas.artifacts[id]
	if !exists {
		return nil, fmt.Errorf("artifact %q not found", id)
	}
	return a, nil
}

func (mas *MemoryArtifactStore) ListArtifacts(ctx context.Context, runID, connectorID string, limit, offset int) ([]*Artifact, error) {
	var results []*Artifact
	for _, a := range mas.artifacts {
		if runID != "" && a.RunID != runID {
			continue
		}
		if connectorID != "" && a.ConnectorID != connectorID {
			continue
		}
		results = append(results, a)
	}
	start := offset
	if start > len(results) {
		return []*Artifact{}, nil
	}
	end := start + limit
	if limit <= 0 || end > len(results) {
		end = len(results)
	}
	return results[start:end], nil
}

func (mas *MemoryArtifactStore) DeleteArtifact(ctx context.Context, id string) error {
	delete(mas.artifacts, id)
	return nil
}

func (mas *MemoryArtifactStore) PruneArtifacts(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	pruned := 0
	for id, a := range mas.artifacts {
		if a.StoredAt.Before(cutoff) {
			delete(mas.artifacts, id)
			pruned++
		}
	}
	return pruned, nil
}
