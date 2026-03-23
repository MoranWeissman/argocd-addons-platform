# AI Assistant

## Overview

The AI Assistant is a conversational agent that can answer questions about your clusters, addons, and configurations using natural language. It has access to 20+ read-only tools that query your Git repository, ArgoCD, and optionally Datadog.

## Setup

Configure an AI provider in **Settings > AI Configuration**:

| Provider | Requirements |
|----------|-------------|
| Claude (Anthropic) | API key |
| OpenAI | API key |
| Gemini (Google) | API key |
| Ollama | Local URL (no API key needed) |
| Custom OpenAI-compatible | Base URL + optional auth header |

Test the connection before saving. The assistant won't work without a configured provider.

## What It Can Do

**Cluster questions:**
- "What clusters are connected?"
- "What addons are on cluster-prod-1?"
- "Is cluster-dev-1 healthy?"

**Addon questions:**
- "Where is datadog deployed?"
- "What version of keda is running?"
- "Which addons have health issues?"

**Comparison and analysis:**
- "Compare datadog versions across clusters"
- "What's different about cluster-prod-1 vs cluster-dev-2?"
- "Should I upgrade istio-base?"

**Configuration:**
- "What are the global values for datadog?"
- "What overrides does cluster-prod-1 have?"

## How It Works

The assistant uses a multi-turn tool-calling loop:
1. You ask a question
2. The AI decides which tools to call (e.g., `list_clusters`, `get_addon_values`)
3. Tools execute and return data
4. The AI synthesizes the results into a human-readable answer
5. If more data is needed, it calls additional tools (up to 8 iterations)

## Memory

The assistant remembers facts across conversations. If you tell it "datadog is only deployed on dev clusters", it will remember this in future sessions.

## Upgrade Analysis

The **Upgrade Impact Checker** page provides AI-powered analysis of addon upgrades:
1. Select an addon and target version
2. The checker diffs `values.yaml` between versions
3. Click **AI Analysis** for a summary of:
   - Breaking changes
   - Risk assessment (Low/Medium/High)
   - Action items before upgrading
   - Impact on your specific configuration

## Safety

- The assistant uses **read-only tools** by default. It cannot modify your clusters or create PRs.
- Write tools (enable/disable addon, sync app) are only available if explicitly enabled via configuration.
- All queries go through your configured AI provider's API. No data is sent to third parties unless you've configured a cloud provider.
