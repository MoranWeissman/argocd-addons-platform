#!/usr/bin/env bash
# Installer for ArgoCD Addons Platform.
# Sources secrets from .env.secrets and passes them via --set (never in values files).
#
# Usage:
#   ./scripts/helm-install.sh              # Sources .env.secrets from project root
#   ./scripts/helm-install.sh /path/to/.env.secrets  # Custom secrets file

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHART_DIR="${PROJECT_ROOT}/charts/argocd-addons-platform"
NAMESPACE="argocd-addons-platform"
RELEASE="aap"

# --- Source secrets ---
SECRETS_FILE="${1:-${PROJECT_ROOT}/.env.secrets}"
if [[ ! -f "${SECRETS_FILE}" ]]; then
  echo "ERROR: Secrets file not found: ${SECRETS_FILE}"
  echo "Usage: $0 [path-to-.env.secrets]"
  exit 1
fi

# Source the file (skip comments and empty lines)
set -a
# shellcheck source=/dev/null
source <(grep -v '^\s*#' "${SECRETS_FILE}" | grep -v '^\s*$')
set +a

# --- Validate required vars ---
if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  echo "ERROR: GITHUB_TOKEN is required in ${SECRETS_FILE}"
  exit 1
fi

# --- Build --set args for secrets ---
SECRET_ARGS=(
  --set "secrets.GITHUB_TOKEN=${GITHUB_TOKEN}"
)

# ArgoCD tokens
[[ -n "${ARGOCD_TOKEN:-}" ]] && SECRET_ARGS+=(--set "secrets.ARGOCD_TOKEN=${ARGOCD_TOKEN}")
[[ -n "${ARGOCD_NONPROD_SERVER_URL:-}" ]] && SECRET_ARGS+=(--set "secrets.ARGOCD_NONPROD_SERVER_URL=${ARGOCD_NONPROD_SERVER_URL}")
[[ -n "${ARGOCD_NONPROD_TOKEN:-}" ]] && SECRET_ARGS+=(--set "secrets.ARGOCD_NONPROD_TOKEN=${ARGOCD_NONPROD_TOKEN}")

# AI
if [[ -n "${AI_API_KEY:-}" ]]; then
  SECRET_ARGS+=(--set "ai.apiKey=${AI_API_KEY}")
fi

# Datadog
if [[ -n "${DATADOG_API_KEY:-}" ]]; then
  SECRET_ARGS+=(
    --set "datadog.apiKey=${DATADOG_API_KEY}"
    --set "datadog.appKey=${DATADOG_APP_KEY:-}"
  )
fi

echo "=== ArgoCD Addons Platform Installer ==="
echo "  Namespace: ${NAMESPACE}"
echo "  Chart:     ${CHART_DIR}"
echo "  Secrets:   ${SECRETS_FILE}"
echo "  AI:        ${AI_PROVIDER:-disabled} ${AI_CLOUD_MODEL:+(${AI_CLOUD_MODEL})}"
echo "  Datadog:   ${DATADOG_API_KEY:+enabled}${DATADOG_API_KEY:-disabled}"
echo ""

helm upgrade --install "${RELEASE}" "${CHART_DIR}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  -f "${CHART_DIR}/values-production.yaml" \
  "${SECRET_ARGS[@]}"

echo ""
echo "=== Installed successfully ==="
echo "  kubectl -n ${NAMESPACE} get pods"
echo "  kubectl -n ${NAMESPACE} logs -f deploy/${RELEASE}-argocd-addons-platform"
echo "  kubectl -n ${NAMESPACE} port-forward svc/${RELEASE}-argocd-addons-platform 8080:8080"
