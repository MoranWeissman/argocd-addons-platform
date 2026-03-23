# Kubernetes Operational Knowledge

## Pod Lifecycle States
| State | Meaning | Common Cause |
|-------|---------|--------------|
| `Pending` | Scheduled but not started | No node capacity, PVC not bound, missing secret |
| `Running` | Container(s) started | Normal |
| `CrashLoopBackOff` | Container keeps crashing and restarting | App error, bad config, missing env var |
| `ImagePullBackOff` | Can't pull container image | Wrong image name, missing imagePullSecret, registry unreachable |
| `OOMKilled` | Container exceeded memory limit | Increase `resources.limits.memory` |
| `Evicted` | Node ran out of resources | Check node pressure; set resource requests |
| `Terminating` (stuck) | Pod has finalizers blocking deletion | `kubectl patch pod <name> -p '{"metadata":{"finalizers":[]}}' --type=merge` |
| `Completed` | Container exited 0 (expected for Jobs) | Normal for batch workloads |

## Diagnosing Pod Issues
```bash
kubectl describe pod <name> -n <ns>   # Events section — most useful for startup issues
kubectl logs <name> -n <ns>           # Stdout/stderr from the main container
kubectl logs <name> -n <ns> --previous  # Logs from the previous crashed container
kubectl get events -n <ns> --sort-by=.lastTimestamp  # All namespace events
```
Check `Events:` in `describe` output first — it tells you why the pod failed to schedule or start before looking at logs.

## Resource Types Relevant to Addon Migration

| Type | Role |
|------|------|
| `Application` (ArgoCD CRD) | Represents one deployed addon |
| `ApplicationSet` (ArgoCD CRD) | Generates Applications from a template |
| `Deployment` | Stateless addon workloads |
| `StatefulSet` | Stateful addons (Prometheus, etc.) |
| `ConfigMap` | Addon configuration |
| `Secret` | Addon credentials (often synced from Vault/ESO) |
| `CRD` | Addon extends the API — must exist before dependent resources |
| `ServiceAccount` + `ClusterRoleBinding` | RBAC for addon pods |

## Namespace Conventions
- Addons typically get their own namespace: `monitoring`, `logging`, `cert-manager`, etc.
- ArgoCD itself lives in `argocd` namespace.
- Cross-namespace resource references (e.g., secrets) require explicit namespace in the reference.
- When migrating, the namespace usually stays the same — the new Application points to the same namespace.

## Common Migration Issues

### Orphaned Resources
Resources that were created by the old Application but not yet owned by the new one. ArgoCD will show them as `OutOfSync` unless they're adopted.

To adopt: add ArgoCD tracking annotations:
```bash
kubectl annotate <resource> <name> -n <ns> \
  argocd.argoproj.io/app-name=<new-app-name> \
  argocd.argoproj.io/app-namespace=argocd
```

### Ownership Conflicts
Two Applications managing the same resource → sync errors. Ensure the old Application is deleted (or has `preserveResourcesOnDeletion: true`) before the new one syncs.

### CRD Timing
If an addon's CRDs are installed by the same Application that uses them, the first sync may fail because CRDs aren't registered yet. ArgoCD retries automatically — or split CRD installation into a separate wave using `argocd.argoproj.io/sync-wave`.

### Immutable Fields
Some fields (e.g., `spec.selector` on Deployments, `claimRef` on PVs) are immutable. If the new Helm chart changes them, you must delete and recreate the resource. Plan for a brief downtime window or use a blue/green approach.
