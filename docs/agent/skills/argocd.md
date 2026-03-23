# ArgoCD Operational Knowledge

## Application Health States
| State | Meaning |
|-------|---------|
| `Synced` | Live cluster state matches Git |
| `OutOfSync` | Drift detected — Git differs from cluster |
| `Healthy` | All resources are healthy (pods running, etc.) |
| `Degraded` | A resource failed its health check |
| `Progressing` | Resources are rolling out (transient) |
| `Unknown` | Health check not defined for a resource type |
| `Missing` | Resource exists in Git but not in cluster |

An app can be `Synced` but `Degraded` — they are independent axes.

## Sync vs Refresh vs Hard Refresh
- **Refresh**: Re-reads Git, updates the diff view. Does not apply changes to cluster.
- **Sync**: Applies the current Git state to the cluster (respects sync options).
- **Hard Refresh**: Busts ArgoCD's manifest cache (forces re-render of Helm/Kustomize). Use when a values change isn't being picked up.

## ApplicationSets
ApplicationSets generate `Application` objects from a template + generator. Key generators:
- **List generator**: explicit list of clusters/values
- **Cluster generator**: one App per registered ArgoCD cluster
- **Git generator**: one App per directory or file match in a repo
- **Matrix generator**: cartesian product of two generators

When an ApplicationSet is deleted, by default it deletes all generated Applications (and their resources). The `preserveResourcesOnDeletion: true` sync policy prevents resource deletion.

## RBAC
- Accounts are defined in `argocd-cm` ConfigMap under `accounts.<name>`.
- Policies are defined in `argocd-rbac-cm` under `policy.csv`.
- Tokens are issued per account via `argocd account generate-token`.
- A 401 means the token is invalid or expired. A 403 means the token is valid but the account lacks permission.
- Default policy applies to all authenticated users — check `policy.default` in `argocd-rbac-cm`.

## Common Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `401 Unauthorized` | Token expired or wrong | Re-generate token; check `ARGOCD_TOKEN` env var |
| `403 Forbidden` | Account lacks RBAC policy | Add policy in `argocd-rbac-cm` |
| `ComparisonError` | ArgoCD can't reach repo or render templates | Check repo credentials; run hard refresh |
| `SyncFailed: hook failed` | A sync hook (Job/Pod) exited non-zero | Check hook pod logs |
| `Namespace not found` | App targets a namespace that doesn't exist | Pre-create namespace or add `CreateNamespace=true` sync option |
| `Resource already exists` | Resource owned by another tool | Use `kubectl annotate` to adopt, or delete and re-sync |

## preserveResourcesOnDeletion
Setting `spec.syncPolicy.preserveResourcesOnDeletion: true` on an Application or ApplicationSet means that deleting the Application object does NOT delete the Kubernetes resources it manages. Critical during migrations — set this before deleting the old Application.

## Prune Behavior
By default ArgoCD does not prune resources removed from Git. Enable with `spec.syncPolicy.automated.prune: true` or pass `--prune` flag on manual sync. During migrations, leave prune OFF until the new Application is confirmed healthy.
