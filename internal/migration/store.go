package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const settingsFile = "settings.json"

// Store provides JSON file-based persistence for migrations and settings.
// Each migration is stored as {dataDir}/{id}.json. Settings are stored at
// {dataDir}/settings.json.
type Store struct {
	dataDir string
}

// NewStore creates a new Store backed by the given directory. The directory
// is created if it does not already exist.
func NewStore(dataDir string) *Store {
	_ = os.MkdirAll(dataDir, 0o750)
	return &Store{dataDir: dataDir}
}

// SaveMigration persists a migration to disk as a JSON file.
func (s *Store) SaveMigration(m *Migration) error {
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

// GetMigration reads a single migration by ID from disk.
func (s *Store) GetMigration(id string) (*Migration, error) {
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

// ListMigrations returns all migrations stored on disk, excluding the
// settings file.
func (s *Store) ListMigrations() ([]*Migration, error) {
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
			continue // skip corrupt files
		}
		migrations = append(migrations, m)
	}
	return migrations, nil
}

// SaveSettings persists the migration settings to disk.
func (s *Store) SaveSettings(settings *MigrationSettings) error {
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

// GetSettings reads the migration settings from disk. If no settings file
// exists, a zero-value MigrationSettings is returned (Configured = false).
func (s *Store) GetSettings() (*MigrationSettings, error) {
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
