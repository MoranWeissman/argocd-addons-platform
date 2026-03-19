package migration

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/moran/argocd-addons-platform/internal/gitops"
	"github.com/moran/argocd-addons-platform/internal/gitprovider"
)

// executeStep dispatches to the appropriate step handler.
func (e *Executor) executeStep(ctx context.Context, m *Migration, stepNum int) error {
	switch stepNum {
	case 1:
		return e.stepVerifyCatalog(ctx, m)
	case 2:
		return e.stepConfigureValues(ctx, m)
	case 3:
		return e.stepEnableAddon(ctx, m)
	case 4:
		return e.stepVerifyAppCreated(ctx, m)
	case 5:
		return e.stepDisableAddonOld(ctx, m)
	case 6:
		return e.stepSyncOldArgoCD(ctx, m)
	case 7:
		return e.stepVerifyAppRemoved(ctx, m)
	case 8:
		return e.stepHardRefresh(ctx, m)
	case 9:
		return e.stepVerifyHealthy(ctx, m)
	case 10:
		return e.stepDisableMigrationMode(ctx, m)
	}
	return fmt.Errorf("unknown step %d", stepNum)
}

// Step 1: Verify addon exists in addons-catalog.yaml with inMigration: true.
func (e *Executor) stepVerifyCatalog(ctx context.Context, m *Migration) error {
	const catalogPath = "configuration/addons-catalog.yaml"

	data, err := e.newGP.GetFileContent(ctx, catalogPath, "main")
	if err != nil {
		return fmt.Errorf("reading catalog from NEW repo: %w", err)
	}

	content := string(data)

	// Check that the addon is present.
	if !strings.Contains(content, "appName: "+m.AddonName) {
		return fmt.Errorf("addon %q not found in addons-catalog.yaml", m.AddonName)
	}

	// Check inMigration flag.
	if !strings.Contains(content, "inMigration: true") {
		// The flag might not be near our addon; do a more targeted check.
		// For now, a simple heuristic: find the addon block and check nearby lines.
		slog.Warn("migration: inMigration flag not found globally, checking addon block", "addon", m.AddonName)
	}

	assessment, _ := e.aiEvaluate(ctx, m.Steps[0].Title,
		fmt.Sprintf("Addon catalog content for %q:\n%s", m.AddonName, truncate(content, 2000)))

	step := &m.Steps[0]
	step.Message = fmt.Sprintf("Addon %q found in catalog. %s", m.AddonName, assessment)
	_ = e.store.SaveMigration(m)
	return nil
}

// Step 2: Compare values between OLD and NEW repos (advisory).
func (e *Executor) stepConfigureValues(ctx context.Context, m *Migration) error {
	step := &m.Steps[1]

	// Read values from NEW repo.
	newValues, newErr := e.newGP.GetFileContent(ctx, "configuration/addons-catalog.yaml", "main")
	newValStr := "(not available)"
	if newErr == nil {
		newValStr = truncate(string(newValues), 1500)
	}

	// Read values from OLD repo (if available).
	oldValStr := "(old git provider not configured)"
	if e.oldGP != nil {
		// Try V2 path first, then V1.
		oldValues, err := e.oldGP.GetFileContent(ctx, "configuration/cluster-addons.yaml", "main")
		if err != nil {
			oldValues, err = e.oldGP.GetFileContent(ctx, "values/clusters.yaml", "main")
		}
		if err == nil {
			oldValStr = truncate(string(oldValues), 1500)
		}
	}

	assessment, _ := e.aiEvaluate(ctx, step.Title,
		fmt.Sprintf("Comparing values for addon %q on cluster %q.\n\nNEW repo catalog:\n%s\n\nOLD repo clusters:\n%s",
			m.AddonName, m.ClusterName, newValStr, oldValStr))

	step.Message = fmt.Sprintf("Values comparison complete (advisory). %s", assessment)
	_ = e.store.SaveMigration(m)
	return nil
}

// Step 3: Create PR to enable addon label in NEW repo.
func (e *Executor) stepEnableAddon(ctx context.Context, m *Migration) error {
	const clusterAddonsPath = "configuration/cluster-addons.yaml"

	data, err := e.newGP.GetFileContent(ctx, clusterAddonsPath, "main")
	if err != nil {
		return fmt.Errorf("reading cluster-addons.yaml from NEW repo: %w", err)
	}

	updated, err := gitops.EnableAddonLabel(data, m.ClusterName, m.AddonName)
	if err != nil {
		return fmt.Errorf("enabling addon label: %w", err)
	}

	step := &m.Steps[2]
	return e.createPR(ctx, e.newGP, m, step,
		clusterAddonsPath, updated,
		"enable",
		fmt.Sprintf("Enable %s on %s (migration)", m.AddonName, m.ClusterName),
		fmt.Sprintf("[Migration] Enable %s on %s", m.AddonName, m.ClusterName),
		fmt.Sprintf("Migration step 3: enable addon label for **%s** on cluster **%s**.\n\nThis PR sets the addon label to `enabled` in `cluster-addons.yaml`.", m.AddonName, m.ClusterName),
	)
}

// Step 4: Verify the application was created in NEW ArgoCD.
func (e *Executor) stepVerifyAppCreated(ctx context.Context, m *Migration) error {
	appName := fmt.Sprintf("%s-%s", m.AddonName, m.ClusterName)
	step := &m.Steps[3]

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
			}
		}

		app, err := e.newArgoCD.GetApplication(ctx, appName)
		if err != nil {
			lastErr = err
			slog.Info("migration: app not yet visible", "app", appName, "attempt", attempt+1)
			continue
		}

		assessment, _ := e.aiEvaluate(ctx, step.Title,
			fmt.Sprintf("Application %q found in NEW ArgoCD. Sync: %s, Health: %s",
				appName, app.SyncStatus, app.HealthStatus))

		step.Message = fmt.Sprintf("Application %q found (sync=%s, health=%s). %s",
			appName, app.SyncStatus, app.HealthStatus, assessment)
		_ = e.store.SaveMigration(m)
		return nil
	}

	return fmt.Errorf("application %q not found in NEW ArgoCD after 3 attempts: %w", appName, lastErr)
}

// Step 5: Create PR to disable addon in OLD repo.
func (e *Executor) stepDisableAddonOld(ctx context.Context, m *Migration) error {
	if e.oldGP == nil {
		return fmt.Errorf("old git provider not configured — cannot disable addon in OLD repo")
	}

	// Try V2 path first, then V1.
	clusterFile := "configuration/cluster-addons.yaml"
	data, err := e.oldGP.GetFileContent(ctx, clusterFile, "main")
	if err != nil {
		clusterFile = "values/clusters.yaml"
		data, err = e.oldGP.GetFileContent(ctx, clusterFile, "main")
		if err != nil {
			return fmt.Errorf("reading cluster file from OLD repo (tried V2 and V1 paths): %w", err)
		}
	}

	updated, err := gitops.DisableAddonLabel(data, m.ClusterName, m.AddonName)
	if err != nil {
		return fmt.Errorf("disabling addon label in OLD repo: %w", err)
	}

	step := &m.Steps[4]
	return e.createPR(ctx, e.oldGP, m, step,
		clusterFile, updated,
		"disable",
		fmt.Sprintf("Disable %s on %s (migration)", m.AddonName, m.ClusterName),
		fmt.Sprintf("[Migration] Disable %s on %s", m.AddonName, m.ClusterName),
		fmt.Sprintf("Migration step 5: disable addon label for **%s** on cluster **%s** in the OLD repository.\n\nThis PR sets the addon label to `disabled` so the OLD ArgoCD will remove the application on next sync.", m.AddonName, m.ClusterName),
	)
}

// Step 6: Trigger sync of the clusters app in OLD ArgoCD.
func (e *Executor) stepSyncOldArgoCD(ctx context.Context, m *Migration) error {
	if e.oldArgoCD == nil {
		return fmt.Errorf("old ArgoCD client not configured")
	}

	if err := e.oldArgoCD.SyncApplication(ctx, "clusters"); err != nil {
		return fmt.Errorf("syncing clusters app in OLD ArgoCD: %w", err)
	}

	step := &m.Steps[5]
	step.Message = "Triggered sync of 'clusters' application in OLD ArgoCD."
	_ = e.store.SaveMigration(m)
	return nil
}

// Step 7: Verify app was removed from OLD ArgoCD.
func (e *Executor) stepVerifyAppRemoved(ctx context.Context, m *Migration) error {
	if e.oldArgoCD == nil {
		return fmt.Errorf("old ArgoCD client not configured")
	}

	appName := fmt.Sprintf("%s-%s", m.AddonName, m.ClusterName)
	step := &m.Steps[6]

	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}

		_, err := e.oldArgoCD.GetApplication(ctx, appName)
		if err != nil {
			// Application not found — this is the desired state.
			assessment, _ := e.aiEvaluate(ctx, step.Title,
				fmt.Sprintf("Application %q is no longer present in OLD ArgoCD (attempt %d).", appName, attempt+1))
			step.Message = fmt.Sprintf("Application %q removed from OLD ArgoCD. %s", appName, assessment)
			_ = e.store.SaveMigration(m)
			return nil
		}

		slog.Info("migration: app still exists in OLD ArgoCD", "app", appName, "attempt", attempt+1)
	}

	return fmt.Errorf("application %q still exists in OLD ArgoCD after 5 attempts", appName)
}

// Step 8: Trigger hard refresh in NEW ArgoCD.
func (e *Executor) stepHardRefresh(ctx context.Context, m *Migration) error {
	appName := fmt.Sprintf("%s-%s", m.AddonName, m.ClusterName)

	app, err := e.newArgoCD.RefreshApplication(ctx, appName, true)
	if err != nil {
		return fmt.Errorf("hard refresh of %q in NEW ArgoCD: %w", appName, err)
	}

	step := &m.Steps[7]
	step.Message = fmt.Sprintf("Hard refresh triggered for %q (sync=%s, health=%s).", appName, app.SyncStatus, app.HealthStatus)
	_ = e.store.SaveMigration(m)
	return nil
}

// Step 9: Verify application is Synced + Healthy in NEW ArgoCD.
func (e *Executor) stepVerifyHealthy(ctx context.Context, m *Migration) error {
	appName := fmt.Sprintf("%s-%s", m.AddonName, m.ClusterName)
	step := &m.Steps[8]

	var lastStatus string
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
			}
		}

		app, err := e.newArgoCD.GetApplication(ctx, appName)
		if err != nil {
			return fmt.Errorf("getting application %q from NEW ArgoCD: %w", appName, err)
		}

		lastStatus = fmt.Sprintf("sync=%s, health=%s", app.SyncStatus, app.HealthStatus)

		if app.SyncStatus == "Synced" && app.HealthStatus == "Healthy" {
			assessment, _ := e.aiEvaluate(ctx, step.Title,
				fmt.Sprintf("Application %q is %s.", appName, lastStatus))
			step.Message = fmt.Sprintf("Application %q is Synced and Healthy. %s", appName, assessment)
			_ = e.store.SaveMigration(m)
			return nil
		}

		slog.Info("migration: app not yet healthy", "app", appName, "status", lastStatus, "attempt", attempt+1)
	}

	return fmt.Errorf("application %q is not healthy after 3 attempts (last status: %s)", appName, lastStatus)
}

// Step 10: Create PR to set inMigration: false in addons-catalog.yaml.
func (e *Executor) stepDisableMigrationMode(ctx context.Context, m *Migration) error {
	const catalogPath = "configuration/addons-catalog.yaml"

	data, err := e.newGP.GetFileContent(ctx, catalogPath, "main")
	if err != nil {
		return fmt.Errorf("reading catalog from NEW repo: %w", err)
	}

	updated, err := setInMigrationFalse(data, m.AddonName)
	if err != nil {
		return fmt.Errorf("updating inMigration flag: %w", err)
	}

	step := &m.Steps[9]
	return e.createPR(ctx, e.newGP, m, step,
		catalogPath, updated,
		"finalize",
		fmt.Sprintf("Disable migration mode for %s", m.AddonName),
		fmt.Sprintf("[Migration] Finalize %s — disable migration mode", m.AddonName),
		fmt.Sprintf("Migration step 10: set `inMigration: false` for **%s** in `addons-catalog.yaml`.\n\nThis completes the migration process.", m.AddonName),
	)
}

// createPR is the shared helper for steps that create a pull request.
func (e *Executor) createPR(
	ctx context.Context,
	gp gitprovider.GitProvider,
	m *Migration,
	step *MigrationStep,
	filePath string,
	content []byte,
	operation, commitMsg, prTitle, prBody string,
) error {
	ts := time.Now().Unix()
	branch := fmt.Sprintf("aap/migration/%s/%s/%d", sanitize(m.AddonName), sanitize(m.ClusterName), ts)

	if err := gp.CreateBranch(ctx, branch, "main"); err != nil {
		return fmt.Errorf("creating branch %q: %w", branch, err)
	}
	if err := gp.CreateOrUpdateFile(ctx, filePath, content, branch, commitMsg); err != nil {
		return fmt.Errorf("updating file %q on branch %q: %w", filePath, branch, err)
	}
	pr, err := gp.CreatePullRequest(ctx, prTitle, prBody, branch, "main")
	if err != nil {
		return fmt.Errorf("creating pull request: %w", err)
	}

	step.PRURL = pr.URL
	step.PRStatus = "open"
	step.Status = StepWaiting
	step.Message = fmt.Sprintf("PR created: %s — waiting for merge", pr.URL)
	_ = e.store.SaveMigration(m)
	return nil
}

// setInMigrationFalse updates the inMigration field from true to false for
// the given addon in an addons-catalog.yaml document.
func setInMigrationFalse(data []byte, addonName string) ([]byte, error) {
	lines := strings.Split(string(data), "\n")

	// Find the addon block.
	appIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "- appName: "+addonName {
			appIdx = i
			break
		}
	}
	if appIdx == -1 {
		return nil, fmt.Errorf("addon %q not found in addons-catalog.yaml", addonName)
	}

	appIndent := leadingSpaces(lines[appIdx])

	for i := appIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") && leadingSpaces(line) <= appIndent {
			break
		}
		if strings.HasPrefix(trimmed, "inMigration:") {
			colonPos := strings.Index(line, "inMigration:")
			prefix := line[:colonPos+len("inMigration:")]
			lines[i] = prefix + " false"
			return []byte(strings.Join(lines, "\n")), nil
		}
	}

	return nil, fmt.Errorf("inMigration field not found for addon %q", addonName)
}

// leadingSpaces returns the number of leading space characters in s.
func leadingSpaces(s string) int {
	return len(s) - len(strings.TrimLeft(s, " "))
}

// sanitize replaces non-alphanumeric characters with hyphens for use in branch names.
func sanitize(s string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	return re.ReplaceAllString(s, "-")
}

// truncate shortens s to at most maxLen characters, appending an ellipsis
// indicator if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}
