# Managing Addons

## Reading Addon Status

Each addon can have one of these statuses:

- **Healthy** -- The addon is deployed and ArgoCD reports it as healthy and synced.
- **Degraded** -- The addon is deployed but ArgoCD reports health issues (pods crashing, resources unavailable).
- **Not Deployed** -- The addon is enabled in Git but no matching ArgoCD Application exists. This may indicate a sync issue.
- **Unmanaged** -- An ArgoCD Application exists but the addon is not defined in the Git repository.
- **Not Enabled** -- The addon exists in the catalog but is not enabled for this cluster.

## Understanding Health

Health information comes from ArgoCD, which checks the actual Kubernetes resources. A green dot means all resources are running. A red dot means at least one resource is unhealthy. The Version Matrix page is the fastest way to see health across all clusters at once.

## Using the Version Matrix

The matrix highlights version drift in amber. Filter by health status or drift to focus on clusters that need attention. The catalog version shown under each addon name is the default version from the addon catalog -- clusters running a different version will be flagged.

## Version Overrides

Override the default chart version for a specific cluster by adding a version label in `cluster-addons.yaml`:

```yaml
clusters:
  - name: my-cluster
    labels:
      datadog: enabled
      datadog-version: "3.70.7"   # Override default version
```

## Enabling/Disabling Addons

Addons are controlled via labels in `cluster-addons.yaml`:

```yaml
clusters:
  - name: my-cluster
    labels:
      datadog: enabled      # Deploy datadog on this cluster
      keda: disabled         # Don't deploy keda
      # anodot: enabled      # Commented out = not deployed
```

Changes are made via Git PRs and applied by ArgoCD automatically.
