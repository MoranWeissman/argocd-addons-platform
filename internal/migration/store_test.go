package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewMigration(t *testing.T) {
	m := NewMigration("cert-manager", "prod-eu", "gates")

	if m.AddonName != "cert-manager" {
		t.Fatalf("expected addon_name cert-manager, got %s", m.AddonName)
	}
	if m.ClusterName != "prod-eu" {
		t.Fatalf("expected cluster_name prod-eu, got %s", m.ClusterName)
	}
	if m.Status != StatusPending {
		t.Fatalf("expected status pending, got %s", m.Status)
	}
	if m.CurrentStep != 1 {
		t.Fatalf("expected current_step 1, got %d", m.CurrentStep)
	}
	if len(m.Steps) != 10 {
		t.Fatalf("expected 10 steps, got %d", len(m.Steps))
	}
	for i, step := range m.Steps {
		if step.Number != i+1 {
			t.Errorf("step %d: expected number %d, got %d", i, i+1, step.Number)
		}
		if step.Status != StepPending {
			t.Errorf("step %d: expected status pending, got %s", i, step.Status)
		}
	}
}

func TestSaveAndGetMigration(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	m := NewMigration("external-dns", "staging-us", "gates")
	m.Status = StatusRunning
	m.Steps[0].Status = StepCompleted

	if err := store.SaveMigration(m); err != nil {
		t.Fatalf("SaveMigration: %v", err)
	}

	// Verify file exists on disk.
	if _, err := os.Stat(filepath.Join(dir, m.ID+".json")); err != nil {
		t.Fatalf("expected file on disk: %v", err)
	}

	got, err := store.GetMigration(m.ID)
	if err != nil {
		t.Fatalf("GetMigration: %v", err)
	}

	if got.ID != m.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, m.ID)
	}
	if got.AddonName != "external-dns" {
		t.Errorf("AddonName: got %s, want external-dns", got.AddonName)
	}
	if got.ClusterName != "staging-us" {
		t.Errorf("ClusterName: got %s, want staging-us", got.ClusterName)
	}
	if got.Status != StatusRunning {
		t.Errorf("Status: got %s, want running", got.Status)
	}
	if got.Steps[0].Status != StepCompleted {
		t.Errorf("Step 0 status: got %s, want completed", got.Steps[0].Status)
	}
}

func TestListMigrations(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	m1 := NewMigration("addon-a", "cluster-1", "gates")
	m2 := NewMigration("addon-b", "cluster-2", "yolo")
	m3 := NewMigration("addon-c", "cluster-3", "gates")

	for _, m := range []*Migration{m1, m2, m3} {
		if err := store.SaveMigration(m); err != nil {
			t.Fatalf("SaveMigration: %v", err)
		}
	}

	list, err := store.ListMigrations()
	if err != nil {
		t.Fatalf("ListMigrations: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(list))
	}
}

func TestListMigrationsExcludesSettings(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	m := NewMigration("addon-x", "cluster-y", "gates")
	if err := store.SaveMigration(m); err != nil {
		t.Fatalf("SaveMigration: %v", err)
	}
	if err := store.SaveSettings(&MigrationSettings{Configured: true}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	list, err := store.ListMigrations()
	if err != nil {
		t.Fatalf("ListMigrations: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 migration (settings excluded), got %d", len(list))
	}
}

func TestGetMigrationNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, err := store.GetMigration("does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing migration")
	}
}

func TestSaveAndGetSettings(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	settings := &MigrationSettings{
		OldGit: OldGitConfig{
			Provider: "github",
			Owner:    "my-org",
			Repo:     "old-addons",
			Token:    "ghp_secret",
		},
		OldArgocd: OldArgocdConfig{
			ServerURL: "https://argocd.old.example.com",
			Token:     "argo-token",
			Namespace: "argocd",
			Insecure:  true,
		},
		Configured: true,
	}

	if err := store.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	got, err := store.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}

	if !got.Configured {
		t.Error("expected Configured=true")
	}
	if got.OldGit.Provider != "github" {
		t.Errorf("Provider: got %s, want github", got.OldGit.Provider)
	}
	if got.OldGit.Owner != "my-org" {
		t.Errorf("Owner: got %s, want my-org", got.OldGit.Owner)
	}
	if got.OldArgocd.ServerURL != "https://argocd.old.example.com" {
		t.Errorf("ServerURL: got %s, want https://argocd.old.example.com", got.OldArgocd.ServerURL)
	}
	if !got.OldArgocd.Insecure {
		t.Error("expected Insecure=true")
	}
}

func TestGetSettingsDefault(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	got, err := store.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.Configured {
		t.Error("expected Configured=false for default settings")
	}
}
