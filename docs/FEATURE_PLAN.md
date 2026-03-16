# AAP Feature Plan (Post-Rebuild)

## Current State
- Go backend + React/Tailwind frontend, working with real data
- GitHub + ArgoCD integration working
- Dashboard, Clusters, Addons, Connections views

## Priority 1: Clean Up Current Features

### Remove/Simplify
- [ ] Remove connection management UI (add/edit/test pages) — connections are config-file or K8s-secret only
- [ ] Keep a read-only "Settings" page showing current connection info
- [ ] Remove PR list from dashboard (or reduce to tiny "recent activity" widget)
- [ ] Remove "Quick Actions" and "System Status" panels from dashboard — redundant
- [ ] Rename "disabled_in_git" to something neutral (e.g., "Not Enabled" or "Opted Out")
- [ ] Clean up "untracked_in_argocd" — these are infra apps, filter them better

### Fix/Improve
- [ ] ArgoCD version column should show actual deployed chart version (not targetRevision)
- [ ] Cluster comparison: show only what matters — addon name, git version, argocd health, issues
- [ ] Addon catalog: simpler cards, less noise

## Priority 2: New High-Value Features

### Addon Version Matrix (killer feature)
- [ ] Single table: rows = addons, columns = clusters, cells = version + health badge
- [ ] Instantly see version drift across the entire fleet
- [ ] Filter by addon, by environment, by health status
- [ ] Highlight cells where version differs from catalog default
- [ ] This is the #1 thing Datadog or ArgoCD UI can't provide

### Config Diff Viewer
- [ ] When viewing a cluster, show what's different between cluster values and global defaults
- [ ] Side-by-side or inline diff of YAML
- [ ] Highlight overrides clearly
- [ ] Help DevOps engineers understand what's custom on their cluster

### Upgrade Impact Checker (AI-powered)
- [ ] Select an addon and a target version
- [ ] Fetch official chart values.yaml for current and target versions
- [ ] Diff the schema: new fields, removed fields, default changes
- [ ] Compare against global values and per-cluster overrides
- [ ] Flag conflicts: "cluster X overrides field Y which changed defaults in new version"
- [ ] AI summarization of release notes / breaking changes (optional LLM integration)
- [ ] Could use MCP for Helm chart research tool

## Priority 3: Observability

### Health History / Trends
- [ ] Poll ArgoCD periodically (or on page load) and store snapshots
- [ ] Show health timeline: "addon X was healthy until 2 days ago"
- [ ] Simple time-series visualization per addon/cluster
- [ ] Lightweight — could use SQLite or file-based storage for snapshots

### Datadog Integration
- [ ] Pull Datadog monitor status via API for addon-related monitors
- [ ] Show alongside ArgoCD health: "ArgoCD says Healthy, Datadog alerting"
- [ ] Bridge the gap between deployment health and operational health
- [ ] Requires Datadog API key + monitor tag convention

## Priority 4: Open Source Readiness

### For Open Source Release
- [ ] Connection management UI is actually useful for open source users (keep as optional)
- [ ] Documentation: README, setup guide, architecture docs
- [ ] Helm chart for easy deployment
- [ ] Support multiple Git providers (GitHub, GitLab, Bitbucket)
- [ ] Support multiple ArgoCD patterns (not just ApplicationSet with labels)
- [ ] Example configurations for common addon setups

### NOT for Open Source (org-specific)
- GetPort IDP integration
- Bootstrap pipeline tracker
- Cluster onboarding lifecycle (tied to your IDP)
- Internal Datadog monitor mappings

## Architecture Notes

### Connections
- For internal use: K8s Secrets, mounted at deploy time
- For open source: config file + optional K8s Secrets
- Two ArgoCD instances: nonprod and prod (separate connections)

### AI/LLM Integration
- Keep it optional — works without LLM, LLM enhances upgrade analysis
- API key provided via config/env var
- MCP server for Helm chart tools could be interesting
- Start simple: just diff values.yaml schemas + summarize changelog
