#!/usr/bin/env bash
# One-line installer for ArgoCD Addons Platform.
# Reads secrets from environment variables — nothing stored in values files.
#
# Required env vars:
#   GITHUB_TOKEN        — GitHub PAT for reading the addons repo
#
# Optional env vars:
#   AI_PROVIDER         — ollama | claude | openai | gemini
#   AI_API_KEY          — API key for claude/openai/gemini
#   AI_CLOUD_MODEL      — e.g. claude-sonnet-4-20250514
#   DATADOG_API_KEY     — Datadog API key
#   DATADOG_APP_KEY     — Datadog application key
#   DATADOG_SITE        — datadoghq.com or datadoghq.eu
#
# Usage:
#   export GITHUB_TOKEN=ghp_xxxxx
#   ./scripts/helm-install.sh
#
# Or one-liner:
#   GITHUB_TOKEN=ghp_xxx ./scripts/helm-install.sh

set -euo pipefail

CHART_DIR="$(cd "$(dirname "$0")/../charts/argocd-addons-platform" && pwd)"
NAMESPACE="argocd-addons-platform"
RELEASE="aap"

# --- Validate required vars ---
if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  echo "ERROR: GITHUB_TOKEN is required. Export it before running this script."
  exit 1
fi

# --- Build --set args for secrets ---
SECRET_ARGS=(
  --set "secrets.GITHUB_TOKEN=${GITHUB_TOKEN}"
)

# AI provider (optional)
if [[ -n "${AI_PROVIDER:-}" ]]; then
  SECRET_ARGS+=(
    --set "ai.enabled=true"
    --set "ai.provider=${AI_PROVIDER}"
  )
  if [[ -n "${AI_API_KEY:-}" ]]; then
    SECRET_ARGS+=(--set "ai.apiKey=${AI_API_KEY}")
  fi
  if [[ -n "${AI_CLOUD_MODEL:-}" ]]; then
    SECRET_ARGS+=(--set "ai.cloudModel=${AI_CLOUD_MODEL}")
  fi
fi

# Datadog (optional)
if [[ -n "${DATADOG_API_KEY:-}" ]]; then
  SECRET_ARGS+=(
    --set "datadog.enabled=true"
    --set "datadog.apiKey=${DATADOG_API_KEY}"
  )
  if [[ -n "${DATADOG_APP_KEY:-}" ]]; then
    SECRET_ARGS+=(--set "datadog.appKey=${DATADOG_APP_KEY}")
  fi
  if [[ -n "${DATADOG_SITE:-}" ]]; then
    SECRET_ARGS+=(--set "datadog.site=${DATADOG_SITE}")
  fi
fi

echo "Installing ${RELEASE} into namespace ${NAMESPACE}..."
echo "  Chart:  ${CHART_DIR}"
echo "  AI:     ${AI_PROVIDER:-disabled}"
echo "  Datadog: ${DATADOG_API_KEY:+enabled}${DATADOG_API_KEY:-disabled}"

helm upgrade --install "${RELEASE}" "${CHART_DIR}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  -f "${CHART_DIR}/values-production.yaml" \
  "${SECRET_ARGS[@]}"

echo ""
echo "Done. Check status:"
echo "  kubectl -n ${NAMESPACE} get pods"
echo "  kubectl -n ${NAMESPACE} logs -f deploy/${RELEASE}-argocd-addons-platform"
