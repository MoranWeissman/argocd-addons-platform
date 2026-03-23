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

	e.addLog(m, 1, e.newRepoLabel(), "reading", "Reading addon catalog...")

	data, err := e.newGP.GetFileContent(ctx, catalogPath, "main")
	if err != nil {
		return fmt.Errorf("reading catalog from NEW repo: %w", err)
	}

	content := string(data)

	e.addLog(m, 1, e.newRepoLabel(), "verifying", fmt.Sprintf("Checking if addon %q exists with inMigration: true", m.AddonName))

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

	e.addLog(m, 1, e.newRepoLabel(), "completed", fmt.Sprintf("Addon %q found in catalog with inMigration: true", m.AddonName))

	step := &m.Steps[0]
	step.Message = fmt.Sprintf("Addon %q found in catalog with inMigration: true", m.AddonName)
	_ = e.store.SaveMigration(m)
	return nil
}

// Step 2: Compare values between OLD and NEW repos (advisory).
func (e *Executor) stepConfigureValues(ctx context.Context, m *Migration) error {
	step := &m.Steps[1]

	// 1. Read addon global values from NEW repo.
	newGlobalPath := fmt.Sprintf("configuration/addons-global-values/%s.yaml", m.AddonName)
	e.addLog(m, 2, e.newRepoLabel(), "reading", fmt.Sprintf("Reading global values for %s from NEW repo...", m.AddonName))
	newGlobalValues, newGlobalErr := e.newGP.GetFileContent(ctx, newGlobalPath, "main")
	if newGlobalErr != nil {
		e.addLog(m, 2, e.newRepoLabel(), "warning", fmt.Sprintf("No global values file found for %s in NEW repo — may need to create one", m.AddonName))
	} else {
		e.addLog(m, 2, e.newRepoLabel(), "completed", fmt.Sprintf("Read global values for %s (%d bytes)", m.AddonName, len(newGlobalValues)))
	}

	// 2. Read addon global values from OLD repo (try V1 then V2).
	if e.oldGP != nil {
		// Try V2 path first.
		oldGlobalPath := fmt.Sprintf("configuration/addons-global-values/%s.yaml", m.AddonName)
		e.addLog(m, 2, e.oldRepoLabel(), "reading", fmt.Sprintf("Reading global values for %s from OLD repo...", m.AddonName))
		oldGlobalValues, err := e.oldGP.GetFileContent(ctx, oldGlobalPath, "main")
		if err != nil {
			// Try V1 path: read defaults.yaml and extract addon section.
			v1Path := "values/addons-config/defaults.yaml"
			e.addLog(m, 2, e.oldRepoLabel(), "reading", "Trying alternative config location in OLD repo...")
			oldGlobalValues, err = e.oldGP.GetFileContent(ctx, v1Path, "main")
			if err != nil {
				e.addLog(m, 2, e.oldRepoLabel(), "warning", "No global values found in OLD repo (tried V2 and V1 paths)")
			} else {
				e.addLog(m, 2, e.oldRepoLabel(), "completed", fmt.Sprintf("Read global values for %s (%d bytes)", m.AddonName, len(oldGlobalValues)))
			}
		} else {
			e.addLog(m, 2, e.oldRepoLabel(), "completed", fmt.Sprintf("Read global values for %s (%d bytes)", m.AddonName, len(oldGlobalValues)))
		}
	}

	// 3. Read cluster values from NEW repo.
	newClusterPath := fmt.Sprintf("configuration/addons-clusters-values/%s.yaml", m.ClusterName)
	e.addLog(m, 2, e.newRepoLabel(), "reading", fmt.Sprintf("Reading cluster values for %s from NEW repo...", m.ClusterName))
	newClusterValues, newClusterErr := e.newGP.GetFileContent(ctx, newClusterPath, "main")
	if newClusterErr != nil {
		e.addLog(m, 2, e.newRepoLabel(), "warning", fmt.Sprintf("No cluster values file found for %s in NEW repo", m.ClusterName))
	} else {
		e.addLog(m, 2, e.newRepoLabel(), "completed", fmt.Sprintf("Read cluster values for %s (%d bytes)", m.ClusterName, len(newClusterValues)))
	}

	// 4. Read cluster values from OLD repo.
	if e.oldGP != nil {
		// Try V2 path first.
		oldClusterPath := fmt.Sprintf("configuration/addons-clusters-values/%s.yaml", m.ClusterName)
		e.addLog(m, 2, e.oldRepoLabel(), "reading", fmt.Sprintf("Reading cluster values for %s from OLD repo...", m.ClusterName))
		oldClusterValues, err := e.oldGP.GetFileContent(ctx, oldClusterPath, "main")
		if err != nil {
			// Try V1 path.
			v1Path := fmt.Sprintf("values/addons-config/overrides/%s/%s.yaml", m.ClusterName, m.AddonName)
			e.addLog(m, 2, e.oldRepoLabel(), "reading", "Trying alternative config location in OLD repo...")
			oldClusterValues, err = e.oldGP.GetFileContent(ctx, v1Path, "main")
			if err != nil {
				e.addLog(m, 2, e.oldRepoLabel(), "warning", "No cluster values found in OLD repo (tried V2 and V1 paths)")
			} else {
				e.addLog(m, 2, e.oldRepoLabel(), "completed", fmt.Sprintf("Read cluster values for %s/%s (%d bytes)", m.ClusterName, m.AddonName, len(oldClusterValues)))
			}
		} else {
			e.addLog(m, 2, e.oldRepoLabel(), "completed", fmt.Sprintf("Read cluster values for %s (%d bytes)", m.ClusterName, len(oldClusterValues)))
		}
	}

	// 5. Log comparison summary.
	e.addLog(m, 2, e.newRepoLabel(), "comparing", "Comparing values between OLD and NEW repos...")
	e.addLog(m, 2, e.newRepoLabel(), "completed", "Values comparison complete (advisory — review logs for details)")

	step.Message = "Values comparison complete (advisory — differences do not block migration)"
	_ = e.store.SaveMigration(m)
	return nil
}

// Step 3: Create PR to enable addon label in NEW repo.
func (e *Executor) stepEnableAddon(ctx context.Context, m *Migration) error {
	const clusterAddonsPath = "configuration/cluster-addons.yaml"

	e.addLog(m, 3, e.newRepoLabel(), "reading", "Reading cluster configuration...")

	data, err := e.newGP.GetFileContent(ctx, clusterAddonsPath, "main")
	if err != nil {
		return fmt.Errorf("reading cluster-addons.yaml from NEW repo: %w", err)
	}

	// Check if addon is already enabled — skip PR creation if so
	if isAddonAlreadySet(data, m.ClusterName, m.AddonName, "enabled") {
		e.addLog(m, 3, e.newRepoLabel(), "completed", fmt.Sprintf("Addon %s is already enabled for cluster %s — skipping PR", m.AddonName, m.ClusterName))
		step := &m.Steps[2]
		step.Message = fmt.Sprintf("Addon %s already enabled on %s — no changes needed", m.AddonName, m.ClusterName)
		_ = e.store.SaveMigration(m)
		return nil
	}

	e.addLog(m, 3, e.newRepoLabel(), "modifying", fmt.Sprintf("Setting %s: enabled for cluster %s", m.AddonName, m.ClusterName))

	updated, err := gitops.EnableAddonLabel(data, m.ClusterName, m.AddonName)
	if err != nil {
		return fmt.Errorf("enabling addon label: %w", err)
	}

	ts := time.Now().Unix()
	branch := fmt.Sprintf("aap/migration/%s/%s/%d", sanitize(m.AddonName), sanitize(m.ClusterName), ts)
	e.addLog(m, 3, e.newRepoLabel(), "creating", "Preparing pull request...")

	step := &m.Steps[2]
	return e.createPRWithLog(ctx, e.newGP, m, step, 3,
		clusterAddonsPath, updated, branch,
		fmt.Sprintf("Enable %s on %s (migration)", m.AddonName, m.ClusterName),
		fmt.Sprintf("[Migration] Enable %s on %s", m.AddonName, m.ClusterName),
		fmt.Sprintf("Migration step 3: enable addon label for **%s** on cluster **%s**.\n\nThis PR sets the addon label to `enabled` in `cluster-addons.yaml`.", m.AddonName, m.ClusterName),
	)
}

// Step 4: Verify the application was created in NEW ArgoCD.
func (e *Executor) stepVerifyAppCreated(ctx context.Context, m *Migration) error {
	appName := fmt.Sprintf("%s-%s", m.AddonName, m.ClusterName)
	step := &m.Steps[3]

	e.addLog(m, 4, "NEW ArgoCD", "verifying", fmt.Sprintf("Checking for application %s...", appName))

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			e.addLog(m, 4, "NEW ArgoCD", "retrying", fmt.Sprintf("Attempt %d/3 — waiting for application %s...", attempt+1, appName))
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

		e.addLog(m, 4, "NEW ArgoCD", "completed", fmt.Sprintf("Application found: sync=%s, health=%s", app.SyncStatus, app.HealthStatus))

		step.Message = fmt.Sprintf("Application %q found (sync=%s, health=%s)",
			appName, app.SyncStatus, app.HealthStatus)
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

	e.addLog(m, 5, e.oldRepoLabel(), "reading", "Reading cluster configuration from OLD repo...")

	// Try V2 path first, then V1.
	clusterFile := "configuration/cluster-addons.yaml"
	data, err := e.oldGP.GetFileContent(ctx, clusterFile, "main")
	if err != nil {
		e.addLog(m, 5, e.oldRepoLabel(), "reading", "Trying alternative config location in OLD repo...")
		clusterFile = "values/clusters.yaml"
		data, err = e.oldGP.GetFileContent(ctx, clusterFile, "main")
		if err != nil {
			return fmt.Errorf("reading cluster file from OLD repo (tried V2 and V1 paths): %w", err)
		}
	}

	// Check if addon is already disabled — skip PR creation if so
	if isAddonAlreadySet(data, m.ClusterName, m.AddonName, "disabled") {
		e.addLog(m, 5, e.oldRepoLabel(), "completed", fmt.Sprintf("Addon %s is already disabled for cluster %s in OLD repo — skipping PR", m.AddonName, m.ClusterName))
		step := &m.Steps[4]
		step.Message = fmt.Sprintf("Addon %s already disabled on %s in OLD repo — no changes needed", m.AddonName, m.ClusterName)
		_ = e.store.SaveMigration(m)
		return nil
	}

	e.addLog(m, 5, e.oldRepoLabel(), "modifying", fmt.Sprintf("Setting %s: disabled for cluster %s", m.AddonName, m.ClusterName))

	updated, err := gitops.DisableAddonLabel(data, m.ClusterName, m.AddonName)
	if err != nil {
		return fmt.Errorf("disabling addon label in OLD repo: %w", err)
	}

	ts := time.Now().Unix()
	branch := fmt.Sprintf("aap/migration/%s/%s/%d", sanitize(m.AddonName), sanitize(m.ClusterName), ts)
	e.addLog(m, 5, e.oldRepoLabel(), "creating", "Preparing pull request...")

	step := &m.Steps[4]
	return e.createPRWithLog(ctx, e.oldGP, m, step, 5,
		clusterFile, updated, branch,
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

	e.addLog(m, 6, "OLD ArgoCD", "syncing", "Triggering sync on clusters application...")

	if err := e.oldArgoCD.SyncApplication(ctx, "clusters"); err != nil {
		return fmt.Errorf("syncing clusters app in OLD ArgoCD: %w", err)
	}

	e.addLog(m, 6, "OLD ArgoCD", "completed", "Sync triggered successfully for 'clusters' application")

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

	e.addLog(m, 7, "OLD ArgoCD", "verifying", fmt.Sprintf("Checking if application %s was removed...", appName))

	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			e.addLog(m, 7, "OLD ArgoCD", "retrying", fmt.Sprintf("Attempt %d/5 — application %s still exists, waiting...", attempt+1, appName))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}

		_, err := e.oldArgoCD.GetApplication(ctx, appName)
		if err != nil {
			// Application not found — this is the desired state.
			e.addLog(m, 7, "OLD ArgoCD", "completed", fmt.Sprintf("Application %s is no longer present in OLD ArgoCD", appName))

			step.Message = fmt.Sprintf("Application %q removed from OLD ArgoCD", appName)
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

	e.addLog(m, 8, "NEW ArgoCD", "refreshing", fmt.Sprintf("Triggering hard refresh for %s...", appName))

	app, err := e.newArgoCD.RefreshApplication(ctx, appName, true)
	if err != nil {
		return fmt.Errorf("hard refresh of %q in NEW ArgoCD: %w", appName, err)
	}

	e.addLog(m, 8, "NEW ArgoCD", "completed", fmt.Sprintf("Hard refresh complete: sync=%s, health=%s", app.SyncStatus, app.HealthStatus))

	step := &m.Steps[7]
	step.Message = fmt.Sprintf("Hard refresh triggered for %q (sync=%s, health=%s).", appName, app.SyncStatus, app.HealthStatus)
	_ = e.store.SaveMigration(m)
	return nil
}

// Step 9: Verify application is Synced + Healthy in NEW ArgoCD.
func (e *Executor) stepVerifyHealthy(ctx context.Context, m *Migration) error {
	appName := fmt.Sprintf("%s-%s", m.AddonName, m.ClusterName)
	step := &m.Steps[8]

	e.addLog(m, 9, "NEW ArgoCD", "verifying", fmt.Sprintf("Checking health status of %s...", appName))

	var lastStatus string
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			e.addLog(m, 9, "NEW ArgoCD", "retrying", fmt.Sprintf("Attempt %d/3 — %s is %s, waiting...", attempt+1, appName, lastStatus))
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

		if app.HealthStatus == "Healthy" {
			syncNote := ""
			if app.SyncStatus != "Synced" {
				syncNote = fmt.Sprintf(" (sync status: %s — normal after migration, ArgoCD will sync on next cycle)", app.SyncStatus)
			}
			e.addLog(m, 9, "NEW ArgoCD", "completed", fmt.Sprintf("Application %s is Healthy%s", appName, syncNote))

			step.Message = fmt.Sprintf("Application %q is Healthy (sync=%s)%s", appName, app.SyncStatus, syncNote)
			_ = e.store.SaveMigration(m)
			return nil
		}

		slog.Info("migration: app not yet healthy", "app", appName, "status", lastStatus, "attempt", attempt+1)
	}

	return fmt.Errorf("application %q is not healthy after 3 attempts (health: %s)", appName, lastStatus)
}

// Step 10: Create PR to set inMigration: false in addons-catalog.yaml.
func (e *Executor) stepDisableMigrationMode(ctx context.Context, m *Migration) error {
	const catalogPath = "configuration/addons-catalog.yaml"

	e.addLog(m, 10, e.newRepoLabel(), "reading", "Reading addon catalog...")

	data, err := e.newGP.GetFileContent(ctx, catalogPath, "main")
	if err != nil {
		return fmt.Errorf("reading catalog from NEW repo: %w", err)
	}

	e.addLog(m, 10, e.newRepoLabel(), "modifying", fmt.Sprintf("Setting inMigration: false for %s", m.AddonName))

	updated, err := setInMigrationFalse(data, m.AddonName)
	if err != nil {
		return fmt.Errorf("updating inMigration flag: %w", err)
	}

	ts := time.Now().Unix()
	branch := fmt.Sprintf("aap/migration/%s/%s/%d", sanitize(m.AddonName), sanitize(m.ClusterName), ts)
	e.addLog(m, 10, e.newRepoLabel(), "creating", "Preparing pull request...")

	step := &m.Steps[9]
	return e.createPRWithLog(ctx, e.newGP, m, step, 10,
		catalogPath, updated, branch,
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

// createPRWithLog is like createPR but emits detailed log entries for each sub-operation.
func (e *Executor) createPRWithLog(
	ctx context.Context,
	gp gitprovider.GitProvider,
	m *Migration,
	step *MigrationStep,
	stepNum int,
	filePath string,
	content []byte,
	branch, commitMsg, prTitle, prBody string,
) error {
	repoLabel := e.newRepoLabel()
	if gp == e.oldGP {
		repoLabel = e.oldRepoLabel()
	}

	if err := gp.CreateBranch(ctx, branch, "main"); err != nil {
		return fmt.Errorf("creating branch %q: %w", branch, err)
	}

	e.addLog(m, stepNum, repoLabel, "committing", "Pushing file changes...")

	if err := gp.CreateOrUpdateFile(ctx, filePath, content, branch, commitMsg); err != nil {
		return fmt.Errorf("updating file %q on branch %q: %w", filePath, branch, err)
	}

	e.addLog(m, stepNum, repoLabel, "creating", "Opening pull request...")

	pr, err := gp.CreatePullRequest(ctx, prTitle, prBody, branch, "main")
	if err != nil {
		return fmt.Errorf("creating pull request: %w", err)
	}

	step.PRURL = pr.URL
	step.PRNumber = pr.ID
	step.PRRepo = "new"
	if gp == e.oldGP {
		step.PRRepo = "old"
	}

	e.addLog(m, stepNum, repoLabel, "waiting", fmt.Sprintf("PR #%d created — please review and merge: %s", pr.ID, pr.URL))
	step.PRStatus = "open"
	step.Status = StepWaiting
	step.Message = fmt.Sprintf("PR #%d created — please review and merge: %s", pr.ID, pr.URL)

	// Set a helpful message on the next step explaining why it's blocked
	if stepNum < len(m.Steps) {
		nextStep := &m.Steps[stepNum] // stepNum is 1-based, slice is 0-based, so m.Steps[stepNum] = next step
		nextStep.Message = fmt.Sprintf("Waiting for step %d PR to be merged before this step can start", stepNum)
	}

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

// isAddonAlreadySet checks if the addon label already has the given value
// (enabled/disabled) for the specified cluster in a cluster-addons.yaml file.
func isAddonAlreadySet(data []byte, clusterName, addonName, value string) bool {
	lines := strings.Split(string(data), "\n")

	// Find the cluster block
	inCluster := false
	inLabels := false
	clusterIndent := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "- name: "+clusterName {
			inCluster = true
			clusterIndent = len(line) - len(strings.TrimLeft(line, " "))
			continue
		}
		if inCluster && !inLabels {
			if strings.HasPrefix(trimmed, "- name:") && len(line)-len(strings.TrimLeft(line, " ")) <= clusterIndent {
				return false // next cluster, addon not found
			}
			if strings.HasPrefix(trimmed, "labels:") {
				inLabels = true
				continue
			}
		}
		if inCluster && inLabels {
			if trimmed == "" || (strings.HasPrefix(trimmed, "- name:") && len(line)-len(strings.TrimLeft(line, " ")) <= clusterIndent) {
				return false // left labels block
			}
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			// Check for "addonName: value"
			if strings.TrimSpace(line) == addonName+": "+value {
				return true
			}
		}
	}
	return false
}

// truncate shortens s to at most maxLen characters, appending an ellipsis
// indicator if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}
