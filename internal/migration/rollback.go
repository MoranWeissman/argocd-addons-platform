package migration

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moran/argocd-addons-platform/internal/gitops"
)

// RollbackMigration reverses completed PR steps of a failed/cancelled migration.
// Step 3 (enable in NEW) → creates PR to disable in NEW.
// Step 5 (disable in OLD) → creates PR to re-enable in OLD.
func (e *Executor) RollbackMigration(ctx context.Context, migrationID string) error {
	m, err := e.store.GetMigration(migrationID)
	if err != nil {
		return fmt.Errorf("reading migration: %w", err)
	}

	if m.Status != StatusFailed && m.Status != StatusCancelled {
		return fmt.Errorf("can only rollback failed or cancelled migrations (current: %s)", m.Status)
	}

	slog.Info("migration rollback started", "id", migrationID, "addon", m.AddonName, "cluster", m.ClusterName)
	e.addLog(m, 0, "SYSTEM", "rollback", "Starting rollback...")

	rollbackErrors := 0

	// Reverse step 5 if completed: re-enable addon in OLD repo
	if m.Steps[4].Status == StepCompleted && e.oldGP != nil {
		e.addLog(m, 5, "SYSTEM", "rollback", fmt.Sprintf("Reverting step 5: re-enabling %s in OLD repo...", m.AddonName))

		clusterFile := "configuration/cluster-addons.yaml"
		data, err := e.oldGP.GetFileContent(ctx, clusterFile, "main")
		if err != nil {
			// Try V1 path
			clusterFile = "values/clusters.yaml"
			data, err = e.oldGP.GetFileContent(ctx, clusterFile, "main")
		}
		if err != nil {
			e.addLog(m, 5, "SYSTEM", "error", fmt.Sprintf("Failed to read cluster file from OLD repo: %s", err))
			rollbackErrors++
		} else {
			updated, err := gitops.EnableAddonLabel(data, m.ClusterName, m.AddonName)
			if err != nil {
				e.addLog(m, 5, "SYSTEM", "error", fmt.Sprintf("Failed to enable addon label: %s", err))
				rollbackErrors++
			} else {
				ts := time.Now().Unix()
				branch := fmt.Sprintf("aap/rollback/%s/%s/%d", sanitize(m.AddonName), sanitize(m.ClusterName), ts)
				step := &MigrationStep{} // dummy step for logging
				err := e.createPRWithLog(ctx, e.oldGP, m, step, 5,
					clusterFile, updated, branch,
					fmt.Sprintf("Rollback: re-enable %s on %s", m.AddonName, m.ClusterName),
					fmt.Sprintf("[Rollback] Re-enable %s on %s", m.AddonName, m.ClusterName),
					fmt.Sprintf("**Rollback** of migration: re-enabling addon **%s** on cluster **%s** in OLD repo.", m.AddonName, m.ClusterName),
				)
				if err != nil {
					e.addLog(m, 5, "SYSTEM", "error", fmt.Sprintf("Failed to create rollback PR: %s", err))
					rollbackErrors++
				} else {
					e.addLog(m, 5, "SYSTEM", "completed", "Rollback PR created for step 5 (re-enable in OLD)")
				}
			}
		}
	}

	// Reverse step 3 if completed: disable addon in NEW repo
	if m.Steps[2].Status == StepCompleted && e.newGP != nil {
		e.addLog(m, 3, "SYSTEM", "rollback", fmt.Sprintf("Reverting step 3: disabling %s in NEW repo...", m.AddonName))

		const clusterAddonsPath = "configuration/cluster-addons.yaml"
		data, err := e.newGP.GetFileContent(ctx, clusterAddonsPath, "main")
		if err != nil {
			e.addLog(m, 3, "SYSTEM", "error", fmt.Sprintf("Failed to read cluster-addons.yaml from NEW repo: %s", err))
			rollbackErrors++
		} else {
			updated, err := gitops.DisableAddonLabel(data, m.ClusterName, m.AddonName)
			if err != nil {
				e.addLog(m, 3, "SYSTEM", "error", fmt.Sprintf("Failed to disable addon label: %s", err))
				rollbackErrors++
			} else {
				ts := time.Now().Unix()
				branch := fmt.Sprintf("aap/rollback/%s/%s/%d", sanitize(m.AddonName), sanitize(m.ClusterName), ts)
				step := &MigrationStep{} // dummy step for logging
				err := e.createPRWithLog(ctx, e.newGP, m, step, 3,
					clusterAddonsPath, updated, branch,
					fmt.Sprintf("Rollback: disable %s on %s", m.AddonName, m.ClusterName),
					fmt.Sprintf("[Rollback] Disable %s on %s", m.AddonName, m.ClusterName),
					fmt.Sprintf("**Rollback** of migration: disabling addon **%s** on cluster **%s** in NEW repo.", m.AddonName, m.ClusterName),
				)
				if err != nil {
					e.addLog(m, 3, "SYSTEM", "error", fmt.Sprintf("Failed to create rollback PR: %s", err))
					rollbackErrors++
				} else {
					e.addLog(m, 3, "SYSTEM", "completed", "Rollback PR created for step 3 (disable in NEW)")
				}
			}
		}
	}

	// Update migration status
	if rollbackErrors > 0 {
		m.Status = StatusFailed
		m.Error = fmt.Sprintf("Rollback completed with %d errors — check logs", rollbackErrors)
	} else {
		m.Status = "rolled_back"
		m.Error = ""
	}
	m.UpdatedAt = now()
	e.addLog(m, 0, "SYSTEM", "rollback", fmt.Sprintf("Rollback finished (%d errors)", rollbackErrors))
	_ = e.store.SaveMigration(m)

	return nil
}
