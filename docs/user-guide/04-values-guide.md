# Values Guide

## Understanding Config Overrides

Addon values follow a layered architecture. Each layer can override the one above it:

```
1. Helm Chart Defaults        (from chart repository)
       |  overridden by
2. global-values.yaml          (shared defaults for all clusters)
       |  overridden by
3. Cluster-specific values     (per-cluster overrides file)
       |  overridden by
4. ApplicationSet Parameters   (highest precedence)
```

## What the Diff Viewer Shows

The Config Overrides tab on each cluster detail page shows a side-by-side comparison for every addon. On the left you see the global default values, on the right the cluster-specific overrides. Addons marked "Uses defaults" have no per-cluster customization. Addons marked "Custom overrides" have cluster-specific values that differ from or extend the global configuration.

## Repository Structure

```
configuration/
  addons-catalog.yaml              # Addon definitions (name, version, chart, inMigration)
  cluster-addons.yaml              # Cluster labels (which addons are enabled)
  addons-global-values/
    datadog.yaml                   # Global defaults for datadog
    keda.yaml                      # Global defaults for keda
  addons-clusters-values/
    cluster-prod-1.yaml            # Per-cluster overrides for cluster-prod-1
    cluster-dev-1.yaml             # Per-cluster overrides for cluster-dev-1
```

## Per-Cluster Values

Each cluster has a single values file at `configuration/addons-clusters-values/<cluster>.yaml`. Each root key corresponds to an addon:

```yaml
clusterGlobalValues:
  env: &env dev
  clusterName: &clusterName my-cluster

datadog:
  clusterAgent:
    resources:
      limits:
        memory: 1Gi

external-secrets:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::12345:role/ESO-Role"
```

## YAML Anchors

Use YAML anchors in `clusterGlobalValues` to avoid duplication across addon sections:

```yaml
clusterGlobalValues:
  clusterName: &clusterName my-cluster
  region: &region eu-west-1

datadog:
  clusterName: *clusterName     # Resolves to: my-cluster

anodot:
  config:
    clusterRegion: *region      # Resolves to: eu-west-1
```
