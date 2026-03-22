# Addon Migration Guide: OLD ArgoCD → NEW ArgoCD

## What This Migration Does

Moves an addon from the OLD ArgoCD instance to the NEW ArgoCD instance with **zero downtime**. Resources (pods, services, configmaps) are **adopted, not recreated** — no pod restarts, no service interruption.

## How It Works

The OLD ArgoCD manages addons via one Git repo. The NEW ArgoCD manages addons via a different Git repo. Both can be GitHub or Azure DevOps.

The migration:
1. Enables the addon in the NEW repo (ArgoCD creates the app but doesn't touch existing resources due to `inMigration: true`)
2. Disables the addon in the OLD repo (ArgoCD deletes the app but leaves resources due to `preserveResourcesOnDeletion`)
3. Refreshes the NEW ArgoCD app so it adopts the orphaned resources
4. Verifies everything is healthy

## Repository Structures

### NEW Repo (GitHub)
```
configuration/
  addons-catalog.yaml          — addon definitions (name, chart, version, inMigration flag)
  cluster-addons.yaml          — cluster labels (addon-name: enabled/disabled)
  addons-global-values/
    <addon>.yaml               — default Helm values for each addon
  addons-clusters-values/
    <cluster>.yaml             — per-cluster value overrides
```

### OLD Repo (may be V1 or V2 structure)

**V2 structure** (same layout as NEW repo):
```
configuration/
  cluster-addons.yaml
  addons-global-values/<addon>.yaml
  addons-clusters-values/<cluster>.yaml
```

**V1 structure** (legacy):
```
values/
  clusters.yaml                — cluster definitions
  addons-config/
    defaults.yaml              — global addon defaults
    overrides/<cluster>/<addon>.yaml  — per-cluster overrides
```

**Important:** Always try V2 paths first. If not found, try V1. Never assume — read the repo to determine which structure it uses.

## The 10 Migration Steps

### Step 1: Verify addon in catalog
Read `addons-catalog.yaml` from the NEW repo. Verify the addon exists and has `inMigration: true`. If the addon is not found or `inMigration` is not set, the migration cannot proceed.

### Step 2: Compare values
Read the addon's global and cluster-specific values from BOTH repos. Compare them to identify differences. This is advisory — differences don't block the migration, but the user should know about them. Report what's different clearly.

### Step 3: Enable addon in NEW repo
Create a PR in the NEW repo to set the addon's label to `enabled` for the target cluster in `cluster-addons.yaml`. Wait for the PR to be merged (user approves or auto-merge).

### Step 4: Verify app created in NEW ArgoCD
Query the NEW ArgoCD API to check if the application was created (name pattern: `<addon>-<cluster>`). The app may show OutOfSync — that's expected and normal. Wait up to 30 seconds, retrying every 10 seconds.

### Step 5: Disable addon in OLD repo
Create a PR in the OLD repo to disable the addon label for the target cluster. The file location depends on the repo structure (V1 or V2 — read first to determine). Wait for PR merge.

### Step 6: Sync OLD ArgoCD
Trigger a sync on the clusters/bootstrap application in the OLD ArgoCD so it processes the removal. The application name may vary — check what exists before syncing.

### Step 7: Verify app removed from OLD ArgoCD
Confirm the application no longer exists in the OLD ArgoCD. Query the API — if the app is not found (404), that's the desired state. Wait up to 60 seconds.

### Step 8: Refresh NEW ArgoCD
Trigger a hard refresh on the application in the NEW ArgoCD. This makes ArgoCD re-scan the cluster and adopt any orphaned resources that were previously managed by the OLD ArgoCD.

### Step 9: Verify healthy
Confirm the application in the NEW ArgoCD is Healthy. OutOfSync is acceptable — it just means ArgoCD hasn't synced yet, which is normal. What matters is the health status. Check pods are running with no restarts.

### Step 10: Finalize migration
**Special logic:** Before disabling `inMigration`, check if ALL clusters that had this addon in the OLD repo have been migrated. If clusters remain unmigrated, do NOT finalize — report which clusters are left. Only finalize when the addon is fully migrated across all clusters.

## Safety Mechanisms

- `inMigration: true` — prevents the NEW ArgoCD from syncing (avoids conflicts during migration)
- `preserveResourcesOnDeletion` — when the OLD ArgoCD deletes the app, resources stay on the cluster
- `prune: false` — extra safety during migration to prevent accidental resource deletion

## Common Issues

### ArgoCD returns 403 on sync
The token doesn't have sync permission. The ArgoCD RBAC needs: `p, <account>, applications, sync, */*, allow`

### PR merge fails on Azure DevOps
Branch policies may block the merge. The migration tool attempts to bypass policies. If the PAT doesn't have bypass permission, merge the PR manually in Azure DevOps and click "I Merged It".

### Application shows Degraded after migration
Check the pod events. Common causes: image pull errors, resource quota exceeded, missing secrets. Use K8s tools (if available) to investigate pod status and events.

### Application is OutOfSync after step 9
This is normal. The app is healthy and running. ArgoCD will sync on the next cycle, or you can trigger a manual sync.
