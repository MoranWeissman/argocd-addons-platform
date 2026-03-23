# Addon Migration Guide

## Goal
Move addon from OLD ArgoCD → NEW ArgoCD with zero downtime. Resources are adopted, not recreated.

## Repo Structures

**NEW repo (GitHub):**
- `configuration/addons-catalog.yaml` — addon definitions, inMigration flag
- `configuration/cluster-addons.yaml` — cluster labels (addon: enabled/disabled)
- `configuration/addons-global-values/<addon>.yaml` — default values
- `configuration/addons-clusters-values/<cluster>.yaml` — cluster overrides

**OLD repo (V2 or V1):**
- V2: same as NEW. V1: `values/clusters.yaml`, `values/addons-config/defaults.yaml`, `values/addons-config/overrides/<cluster>/<addon>.yaml`
- Always try V2 first. If 404, try V1. Never assume — read first.

## Steps

1. **Verify catalog** — read addons-catalog.yaml from NEW, confirm addon exists with inMigration: true
2. **Compare values** — read values from both repos, log differences. Advisory only.
3. **Enable addon** — create PR in NEW repo: set addon label to enabled in cluster-addons.yaml. Merge it.
4. **Verify app created** — check NEW ArgoCD for app `<addon>-<cluster>`. Retry 3x with 10s waits.
5. **Disable addon in OLD** — create PR in OLD repo: set addon to disabled. Merge it.
6. **Sync OLD ArgoCD** — trigger sync on the clusters/bootstrap app in OLD ArgoCD.
7. **Verify removal** — confirm app no longer in OLD ArgoCD. 404 = success. Retry 3x.
8. **Refresh NEW** — hard refresh app in NEW ArgoCD to adopt orphaned resources.
9. **Verify healthy** — check app is Healthy in NEW ArgoCD. OutOfSync is OK.
10. **Finalize** — check ALL clusters migrated before setting inMigration: false.

## Safety
- `inMigration: true` prevents NEW ArgoCD from syncing during migration
- `preserveResourcesOnDeletion` keeps resources when OLD removes the app
- All changes via PRs, never direct edits

## Common Errors
- **ArgoCD 401**: invalid/expired token
- **ArgoCD 403**: token lacks permission (need sync access)
- **Azure DevOps HTML response**: PAT expired
- **App OutOfSync after step 9**: normal, ArgoCD will sync next cycle
