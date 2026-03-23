# CI/CD Pipeline Knowledge

## ArgoCD's Role in CI/CD
ArgoCD is the CD layer only. It does not build, test, or publish artifacts — it deploys whatever is in Git.

Typical split:
- **CI** (GitHub Actions, Azure Pipelines, Jenkins): build → test → push image → update image tag in Git
- **CD** (ArgoCD): detect Git change → render manifests → sync to cluster

The agent operates in the CD layer. It modifies Git (values, catalog) and triggers ArgoCD syncs. It does not interact with CI pipelines directly unless checking build status.

## PR-Based Deployment Workflow
```
Agent modifies values/catalog
       ↓
Agent opens PR on GitHub / Azure DevOps
       ↓
CI runs checks (lint, dry-run, policy)
       ↓
PR merges (squash) → Git is updated
       ↓
ArgoCD detects change (poll or webhook)
       ↓
ArgoCD syncs Application → cluster updated
```

Each step is auditable. The PR is the deployment record — include the addon name, reason, and before/after summary in the PR description.

## Branch Policies and Automation Impact
- **Required status checks**: Agent must wait for CI to pass before merging. Poll `check-runs` API (GitHub) or `build status` API (Azure DevOps).
- **Required approvals**: Agent may be blocked. Options: (a) request a human reviewer, (b) use an admin token with bypass, (c) configure a bot as an auto-approver.
- **Linear history / squash-only**: Set `merge_method: squash` in the merge API call or `mergeStrategy: squash` in Azure DevOps.

Check branch protection policy before opening a PR — a PR that can't be merged blocks the migration step.

## Waiting for Sync After Merge
After a PR merges, ArgoCD may take up to 3 minutes to detect the change (poll interval). To avoid waiting:
1. Trigger a manual refresh: `argocd app get <name> --refresh`
2. Trigger sync: `argocd app sync <name>`
3. Or use the ArgoCD API: `POST /api/v1/applications/{name}/sync`

Poll application health after sync. A sync that completes but leaves the app `Degraded` or `Progressing` for >5 minutes needs investigation.

## Rollback Strategies in GitOps

| Strategy | Method | Use When |
|----------|--------|---------|
| Git revert | Open a revert PR | Preferred — keeps audit trail |
| ArgoCD rollback | `argocd app rollback <name> <revision>` | Fast recovery; reverts to previous ArgoCD revision without a new Git commit |
| Helm rollback | `helm rollback <release> <revision> -n <ns>` | Last resort; bypasses ArgoCD — re-sync will overwrite |

For migration rollbacks: revert the catalog entry change (set `inMigration: true` again, or remove the new entry) and re-enable the old Application. Keep `preserveResourcesOnDeletion: true` on both Applications during rollback to avoid accidental resource deletion.

## When to Sync vs Wait for Auto-Sync
- **Sync immediately** when: migration gate is cleared, time-sensitive cutover, you've verified the diff is safe.
- **Wait for auto-sync** when: low-risk config change, no urgency, auto-sync is enabled and you want to observe behavior.
- **Never sync** when: `inMigration: true` is still set, the PR has not merged yet, or a required health check is failing.

Auto-sync with `selfHeal: true` will revert any manual cluster changes — do not mix manual `kubectl apply` with a self-healing Application.
