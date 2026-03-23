package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const settingsFile = "settings.json"

// Store provides JSON file-based persistence for migrations and K8s Secret
// storage for sensitive settings. Migration state files are stored at
// {dataDir}/{id}.json. Settings (credentials) are stored in a K8s Secret
// when running in-cluster, or in a local file for development.
type Store struct {
	dataDir     string
	secretStore *SecretStore
	cmStore     *ConfigMapStore
}

// NewStore creates a new Store backed by the given directory. The directory
// is created if it does not already exist.
func NewStore(dataDir string) *Store {
	_ = os.MkdirAll(dataDir, 0o750)
	return &Store{
		dataDir:     dataDir,
		secretStore: NewSecretStore(),    // nil if not in K8s
		cmStore:     NewConfigMapStore(), // nil if not in K8s
	}
}

// SaveMigration persists migration state. Uses K8s ConfigMap in-cluster, file locally.
func (s *Store) SaveMigration(m *Migration) error {
	if s.cmStore != nil {
		return s.cmStore.SaveMigration(context.Background(), m)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling migration %s: %w", m.ID, err)
	}
	path := filepath.Join(s.dataDir, m.ID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing migration %s: %w", m.ID, err)
	}
	return nil
}

// GetMigration reads migration state. Uses K8s ConfigMap in-cluster, file locally.
func (s *Store) GetMigration(id string) (*Migration, error) {
	if s.cmStore != nil {
		return s.cmStore.GetMigration(context.Background(), id)
	}
	path := filepath.Join(s.dataDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading migration %s: %w", id, err)
	}
	var m Migration
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("decoding migration %s: %w", id, err)
	}
	return &m, nil
}

// ListMigrations returns all migrations. Uses K8s ConfigMap in-cluster, file locally.
func (s *Store) ListMigrations() ([]*Migration, error) {
	if s.cmStore != nil {
		return s.cmStore.ListMigrations(context.Background())
	}
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading migration directory: %w", err)
	}

	var migrations []*Migration
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || name == settingsFile {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		m, err := s.GetMigration(id)
		if err != nil {
			continue
		}
		migrations = append(migrations, m)
	}
	return migrations, nil
}

// DeleteMigration removes a migration. Uses K8s ConfigMap in-cluster, file locally.
func (s *Store) DeleteMigration(id string) error {
	if s.cmStore != nil {
		return s.cmStore.DeleteMigration(context.Background(), id)
	}
	path := filepath.Join(s.dataDir, id+".json")
	return os.Remove(path)
}

// HasActiveMigration checks whether any migration is currently in an active
// state (running, waiting, paused, or gated). Returns the ID of the first
// active migration found.
func (s *Store) HasActiveMigration() (bool, string, error) {
	migrations, err := s.ListMigrations()
	if err != nil {
		return false, "", err
	}
	for _, m := range migrations {
		if m.Status == StatusRunning || m.Status == StatusWaiting || m.Status == StatusPaused || m.Status == StatusGated {
			return true, m.ID, nil
		}
	}
	return false, "", nil
}

// SaveBatch persists a migration batch.
func (s *Store) SaveBatch(b *MigrationBatch) error {
	if s.cmStore != nil {
		return s.cmStore.SaveBatch(context.Background(), b)
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dataDir, "batch-"+b.ID+".json"), data, 0o644)
}

// GetBatch loads a batch by ID.
func (s *Store) GetBatch(id string) (*MigrationBatch, error) {
	if s.cmStore != nil {
		return s.cmStore.GetBatch(context.Background(), id)
	}
	data, err := os.ReadFile(filepath.Join(s.dataDir, "batch-"+id+".json"))
	if err != nil {
		return nil, err
	}
	var b MigrationBatch
	return &b, json.Unmarshal(data, &b)
}

// GetActiveBatch returns the currently running batch, if any.
func (s *Store) GetActiveBatch() (*MigrationBatch, error) {
	// Simple: scan all batches for one that's running
	if s.cmStore != nil {
		return s.cmStore.GetActiveBatch(context.Background())
	}
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, nil
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "batch-") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dataDir, e.Name()))
		if err != nil {
			continue
		}
		var b MigrationBatch
		if json.Unmarshal(data, &b) == nil && b.Status == "running" {
			return &b, nil
		}
	}
	return nil, nil
}

// SaveSettings persists credentials. Uses K8s Secret in-cluster, file locally.
func (s *Store) SaveSettings(settings *MigrationSettings) error {
	if s.secretStore != nil {
		return s.secretStore.SaveSettings(context.Background(), settings)
	}
	// File fallback for local dev
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	path := filepath.Join(s.dataDir, settingsFile)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}
	return nil
}

// GetSettings reads credentials. Uses K8s Secret in-cluster, file locally.
func (s *Store) GetSettings() (*MigrationSettings, error) {
	if s.secretStore != nil {
		return s.secretStore.GetSettings(context.Background())
	}
	// File fallback for local dev
	path := filepath.Join(s.dataDir, settingsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &MigrationSettings{}, nil
		}
		return nil, fmt.Errorf("reading settings: %w", err)
	}
	var settings MigrationSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("decoding settings: %w", err)
	}
	return &settings, nil
}

// DeleteSettings removes credentials. Uses K8s Secret in-cluster, file locally.
func (s *Store) DeleteSettings() error {
	if s.secretStore != nil {
		return s.secretStore.DeleteSettings(context.Background())
	}
	path := filepath.Join(s.dataDir, settingsFile)
	return os.Remove(path)
}
