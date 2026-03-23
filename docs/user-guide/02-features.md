# Features

A quick tour of every page in the platform.

## Dashboard

The landing page shows a high-level summary: total clusters, addon health distribution, and recent issues. Use it as a starting point to spot problems at a glance.

## Clusters

Lists every cluster with its connection status and Kubernetes version. Click a cluster to see a detailed comparison of Git-configured addons versus live ArgoCD state. The detail page also includes a Config Overrides tab showing per-cluster value differences.

## Addon Catalog

A searchable, filterable catalog of all addons defined in your repository. Each card shows deployment count, health breakdown, and version. Click an addon to see per-cluster deployment details.

## Version Matrix

A matrix view with addons as rows and clusters as columns. Each cell shows the deployed version and a health indicator dot. Cells with version drift (different from the catalog version) are highlighted in amber.

## Upgrade Impact Checker

Analyze the impact of addon upgrades before applying them. The checker compares `values.yaml` between chart versions, detects conflicts with your configured overrides, fetches release notes from GitHub, and provides AI-powered analysis summarizing breaking changes and recommended actions.

## AI Assistant

A conversational agent accessible from the sidebar. It has 20+ specialized tools for querying cluster status, addon health, version information, configuration data, and more. Ask questions in natural language and get structured, actionable answers.

Example prompts:
- "What addons are deployed on cluster-prod-1?"
- "Is everything healthy?"
- "Compare datadog versions across clusters"
- "Should I upgrade istio-base?"

## Observability

Track ArgoCD sync operations over time with the sync timeline view. Addon health cards provide at-a-glance status for each deployment. When Datadog is configured, CPU and memory usage metrics are shown per addon per cluster.

## External Dashboards

Embed external monitoring dashboards (Datadog, Grafana, or custom URLs) directly into the platform. Configure dashboard links per addon or cluster for one-click access to relevant observability data.
