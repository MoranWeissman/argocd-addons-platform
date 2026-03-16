# TODO — Next Development Session

## Priority Fixes

### 1. Datadog Metrics — Fix and Restructure
Current issues:
- Pod count is wrong (counting namespace-wide, not addon-specific)
- Metrics should be at **cluster level** in observability, not addon level
- Need to show **actual usage vs requests vs limits** (not just usage)

Fix approach:
- Query with more specific tags: `kube_namespace:{ns},kube_deployment:{addon}*`
- Show in observability per cluster, then per addon within cluster
- Add queries for: `kubernetes.cpu.requests`, `kubernetes.cpu.limits`, `kubernetes.memory.requests`, `kubernetes.memory.limits`
- Show as: "CPU: 0.17 cores / 0.5 requested / 1.0 limit" with a visual bar

### 2. UI/UX Fixes Still Pending
- Version Matrix design needs more work (cluster chip approach works but could be better)
- Some dark mode inconsistencies remain
- Observability page needs cluster-level grouping option

### 3. AI Agent Improvements
- Streaming responses (show text as it generates)
- Real tool call indicators (show which tool is being called)
- Conversation memory across page navigations (localStorage)

### 4. Deployment Testing
- Docker build not tested since rebuild
- Minikube deployment not tested
- Need to verify `make build` and `make deploy` work

### 5. Open Source Readiness
- README needs full rewrite for the new architecture
- Helm chart for easy deployment
- GitLab/Bitbucket Git provider support
- Example configurations
- Contributing guide

## Current Version: v1.4.0
## Total Tests: 107 (frontend) + Go tests
## Git Tags: v1.0.0 → v1.4.0
