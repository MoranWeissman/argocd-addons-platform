# Overview

The ArgoCD Addons Platform (AAP) is a web interface for monitoring and managing Kubernetes add-ons across your cluster fleet. It provides a centralized view of addon health, version status, and configuration differences between clusters.

## What It Shows

- **Cluster health** -- connection status and addon deployment state for every cluster in your fleet.
- **Addon catalog** -- all available addons, their versions, and how many clusters have them deployed.
- **Version matrix** -- a cross-cluster view showing which version of each addon is running where, highlighting version drift.
- **Configuration differences** -- per-cluster overrides compared against global defaults.

## AI-Powered Operations

The platform includes an AI assistant that can answer natural-language questions about your clusters, addons, and configurations. It has access to 20+ specialized tools for querying cluster data, analyzing upgrade impacts, and providing actionable recommendations -- all without leaving the UI.

## Who It's For

Platform engineers and SREs who manage Kubernetes add-ons at scale. The UI gives you a read-only operational view -- all changes are made through Git (GitOps workflow) and applied by ArgoCD.

## Architecture

```
Git Repository (source of truth)
    |
    v
ArgoCD (applies changes to clusters)
    |
    v
AAP (reads from Git + ArgoCD, displays status)
```

AAP never modifies your clusters directly. It reads from:
1. **Git** -- addon catalog, cluster labels, values files
2. **ArgoCD** -- application health, sync status, cluster connectivity
3. **Datadog** (optional) -- CPU/memory metrics per addon
