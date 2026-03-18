# AI Assistant

The ArgoCD Addons Platform includes an AI assistant that can answer natural language questions about your clusters, addons, health status, and configurations using agentic tool calling.

---

## Table of Contents

- [Overview](#overview)
- [Supported Providers](#supported-providers)
- [Ollama Deployment in Kubernetes](#ollama-deployment-in-kubernetes)
- [Available Tools](#available-tools)
- [Agent Memory](#agent-memory)
- [Data Privacy](#data-privacy)

---

## Overview

The AI assistant is an agentic system that sits between you and the platform's data. When you ask a question, it:

1. Receives your message and conversation history
2. Decides which platform tools to call (e.g., list clusters, check health, fetch configs)
3. Executes tools and receives structured results
4. May call additional tools based on intermediate results (up to 5 iterations)
5. Synthesizes a final natural language response

This means you can ask complex questions like "Which clusters have unhealthy addons and what are the recent events?" and the agent will chain multiple tool calls to build a comprehensive answer.

### Enabling the AI Assistant

Set `ai.enabled: true` in your Helm values and configure a provider:

```yaml
ai:
  enabled: true
  provider: gemini          # or: ollama, openai
  cloudModel: gemini-2.5-flash  # required for cloud providers
  apiKey: ""                # set via --set or existingSecret
```

---

## Supported Providers

| Provider | Type | Privacy | Tool Calling | Configuration |
|----------|------|---------|--------------|---------------|
| **Ollama** | Local (in-cluster) | All data stays within your cluster | Depends on model (see below) | `ai.provider: ollama` |
| **OpenAI** | Cloud API | Data sent to OpenAI's API | Strong (GPT-4o, GPT-4-turbo) | `ai.provider: openai` + `ai.apiKey` |
| **Gemini** | Cloud API | Data sent to Google's API | Strong (Gemini 2.5 Flash/Pro) | `ai.provider: gemini` + `ai.apiKey` |

### Provider Details

#### Ollama (Local)

- **Privacy**: All data remains within your Kubernetes cluster. No external API calls.
- **Cost**: Free. You provide the compute.
- **Latency**: Depends on hardware. CPU-only inference is slower than GPU.
- **Tool calling**: Varies significantly by model. Smaller models may struggle with multi-step tool calling.

#### OpenAI

- **Privacy**: Cluster names, addon configurations, health status, and tool results are sent to OpenAI's API.
- **Cost**: Pay-per-token pricing via OpenAI API.
- **Models**: `gpt-4o` (recommended), `gpt-4-turbo`, `gpt-3.5-turbo`
- **Tool calling**: Excellent. Handles complex multi-step reasoning reliably.

#### Gemini

- **Privacy**: Same data as OpenAI is sent to Google's Gemini API.
- **Cost**: Pay-per-token pricing via Google AI API.
- **Models**: `gemini-2.5-flash` (recommended, fast and capable), `gemini-2.5-pro`
- **Tool calling**: Excellent. Fast response times with strong tool-calling capability.

---

## Ollama Deployment in Kubernetes

The Helm chart can deploy an Ollama pod alongside AAP automatically.

### Minimal Setup

```yaml
ai:
  enabled: true
  provider: ollama
  ollama:
    deploy: true
    model: llama3.2
    persistence: true
    storageSize: 10Gi
```

### Model Recommendations

| Model | Parameters | RAM Required | Tool Calling | Notes |
|-------|-----------|-------------|--------------|-------|
| `llama3.2` | 3B | 2-4 GB | Weak | Fast, good for simple Q&A. Default. |
| `qwen2.5` | 7B | 4-6 GB | Good | Best tool calling for its size |
| `mistral` | 7B | 4-6 GB | Moderate | Fast inference, decent reasoning |
| `llama3.1:8b` | 8B | 6-8 GB | Moderate | Solid general-purpose model |
| `llama3.1:70b` | 70B | 40+ GB | Strong | Needs GPU. Near cloud-quality tool calling |
| `mixtral` | 8x7B (MoE) | 26+ GB | Strong | Mixture of experts. Good reasoning |

### Resource Configuration

Adjust resources based on the model you choose:

```yaml
ai:
  ollama:
    deploy: true
    model: qwen2.5
    resources:
      requests:
        memory: "4Gi"
        cpu: "1000m"
      limits:
        memory: "6Gi"
        cpu: "4000m"
```

### Using a Separate Agent Model

For tool-calling tasks, you can use a larger model while keeping a smaller model for simple queries:

```yaml
ai:
  ollama:
    model: llama3.2        # For simple queries (fast, low memory)
    agentModel: qwen2.5    # For tool calling (better reasoning)
```

### Persistence (Strongly Recommended)

Without persistence, models are re-downloaded on every pod restart (multi-GB downloads). Enable persistence to avoid this:

```yaml
ai:
  ollama:
    persistence: true
    storageClassName: ""    # Uses cluster default; or set e.g. "gp3"
    storageSize: 10Gi      # 10 Gi fits 1-2 small models; 50 Gi+ for larger ones
```

### GPU Support

For larger models, GPU acceleration dramatically improves response times:

```yaml
ai:
  ollama:
    gpu: true
```

Requires the NVIDIA device plugin to be installed on your cluster nodes.

### External Ollama

If you already run Ollama elsewhere, point AAP to it:

```yaml
ai:
  enabled: true
  provider: ollama
  ollama:
    deploy: false
    url: "http://ollama.my-namespace.svc.cluster.local:11434"
    model: llama3.1:8b
```

---

## Available Tools

The AI agent has access to 24 platform-aware tools. All tools are **read-only** and cannot modify any resources.

### Cluster Tools

| Tool | Description |
|------|-------------|
| `list_clusters` | List all Kubernetes clusters with their connection status |
| `get_cluster_addons` | Get addons enabled on a specific cluster with health status |
| `get_cluster_values` | Get per-cluster configuration overrides |
| `get_cluster_status` | Get connection status of all clusters (Connected, Failed, Unknown) |

### Addon Tools

| Tool | Description |
|------|-------------|
| `list_addons` | List all addons in the catalog (what could be deployed) |
| `get_addon_values` | Get global default values for an addon |
| `find_addon_deployments` | Find which clusters have a specific addon installed |
| `get_addon_on_cluster` | Get detailed info about a specific addon on a specific cluster |
| `get_addon_config_on_cluster` | Get merged config (global defaults + cluster overrides) for an addon on a cluster |
| `search_addons` | Search addons by partial name match |
| `get_unhealthy_addons` | Find all addons with Degraded, Progressing, or Unknown health |

### ArgoCD Tools

| Tool | Description |
|------|-------------|
| `get_argocd_app_health` | Get deployed addons with health/sync status, optionally filtered by cluster |
| `get_recent_syncs` | Get recent sync/deployment activity across all applications |
| `get_app_resources` | Get Kubernetes resources managed by an ArgoCD app (never returns Secrets) |
| `get_app_events` | Get recent Kubernetes events for an ArgoCD application |
| `get_app_details` | Get detailed ArgoCD app info: source, sync policy, history, conditions |
| `get_pod_logs` | Get recent log lines from a pod via ArgoCD proxy |

### Helm Tools

| Tool | Description |
|------|-------------|
| `compare_chart_versions` | Diff values.yaml between two Helm chart versions |
| `list_chart_versions` | List available versions for a Helm chart |
| `get_release_notes` | Fetch release notes for a specific chart version from GitHub |

### Platform Tools

| Tool | Description |
|------|-------------|
| `get_platform_info` | Get platform summary: ArgoCD version, cluster/app counts, health overview |
| `web_search` | Search the internet for documentation, CVEs, best practices |

### Memory Tools

| Tool | Description |
|------|-------------|
| `save_memory` | Save an observation for future conversations |
| `recall_memories` | Search through saved memories from previous conversations |

---

## Agent Memory

The AI agent maintains a persistent memory file at `/tmp/aap-agent-memory.json` that stores learned observations across conversations.

- **Maximum entries**: 100
- **Categories**: `user_preference`, `platform_observation`, `addon_info`, `troubleshooting`, `faq`
- **Persistence**: Pod-local. Memory is lost on pod restart (non-critical data).
- **Security**: No secrets or tokens are stored. File permissions are `0600`.

The agent automatically saves and recalls relevant memories during conversations. For example, if it learns that a particular addon requires specific configuration on certain clusters, it will remember this for future questions.

---

## Data Privacy

### What Data Is Sent to Cloud Providers

When using OpenAI or Gemini, the following data is included in API requests:

- **Sent**: Cluster names, addon names and versions, health/sync status, configuration values (from Git), Kubernetes resource lists (names, kinds, status), pod events, pod logs (when requested), Helm chart metadata
- **Never sent**: Kubernetes Secrets, API tokens, passwords, authentication credentials

### What Stays Local

When using Ollama, all data remains within your Kubernetes cluster. No external API calls are made for AI features.

### AI Agent Security Boundaries

- The agent has **read-only** access to platform data
- It **cannot** modify ArgoCD applications, clusters, or any Kubernetes resources
- Kubernetes Secrets are explicitly blocked from tool results (the `get_app_resources` tool refuses to return Secret-kind resources)
- Agent sessions expire after 1 hour with a maximum of 100 concurrent sessions
- Each agent session is bound to an authenticated user session

For complete security details, see [SECURITY.md](SECURITY.md).
