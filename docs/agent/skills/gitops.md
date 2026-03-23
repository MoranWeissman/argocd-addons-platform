# GitOps Patterns Knowledge

## Core Principle
Git is the single source of truth. The cluster state is always derived from Git — never the reverse. An agent never modifies the cluster directly; it modifies Git, then ArgoCD reconciles the cluster.

## How ArgoCD Watches and Reconciles
1. ArgoCD polls the Git repo (or receives a webhook) at a configurable interval (default: 3 minutes).
2. It renders the manifests (Helm, Kustomize, raw YAML).
3. It diffs the rendered output against the live cluster state.
4. If `auto-sync` is enabled, it applies the diff automatically.
5. If `auto-sync` is disabled, a human or agent must trigger sync manually.

For time-sensitive migrations, trigger sync explicitly rather than waiting for the poll interval.

## ApplicationSet Generators

| Generator | Use Case |
|-----------|---------|
| `list` | Fixed set of clusters or environments |
| `cluster` | One Application per registered ArgoCD cluster |
| `git` (directory) | One Application per matching directory in repo |
| `git` (file) | One Application per matching JSON/YAML file |
| `matrix` | Combine two generators (e.g., clusters × addons) |
| `merge` | Merge values from multiple generators |

In this platform, the `git` file generator is common — `addons-catalog.yaml` is the source, and one Application is generated per addon entry.

## The `inMigration` Flag
`inMigration: true` on a catalog entry signals that the addon is mid-migration.

What it does:
- The ApplicationSet template may skip auto-sync for flagged addons.
- Prevents the new Application from syncing until the agent confirms readiness.
- Allows the old Application (in the source ArgoCD) to remain live without conflict.

Lifecycle:
1. Set `inMigration: true` → open PR → merge
2. New Application is created but does not sync
3. Agent validates values, resources, connectivity
4. Agent sets `inMigration: false` → open PR → merge
5. New Application syncs → addon is live on new ArgoCD
6. Agent deletes old Application from source ArgoCD

Never skip step 2–4. The flag is a gate, not a cosmetic field.

## Resource Adoption
When ArgoCD takes over a resource that already exists in the cluster (created by the old system):
- ArgoCD compares the live resource against its rendered manifest.
- If they match, it marks as `Synced` with no changes.
- If they differ, it marks as `OutOfSync` and will overwrite on sync (with prune rules applying).

To safely adopt: ensure the new Application's Helm values produce a manifest that matches the live state. Use `argocd app diff` to verify before syncing.

## `preserveResourcesOnDeletion`
```yaml
spec:
  syncPolicy:
    preserveResourcesOnDeletion: true
```
Set this on Applications (or the ApplicationSet template) before any deletion during migration. Without it, deleting an Application deletes all managed Kubernetes resources — including running workloads.

Standard migration sequence:
1. Confirm `preserveResourcesOnDeletion: true` on old Application.
2. Delete old Application object.
3. New Application adopts resources.
4. Remove `preserveResourcesOnDeletion` from new Application once migration is stable.

## Prune in Migration Context
- `prune: false` (default): removing a resource from Git does not delete it from the cluster. Safe during migration.
- `prune: true`: Git is authoritative — resources not in Git are deleted. Enable only after migration is fully complete and validated.

Do not enable pruning on a migrating addon. Stale resources can be cleaned up manually post-migration.
