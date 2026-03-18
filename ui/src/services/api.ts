import type {
  AddonCatalogResponse,
  AddonDetailResponse,
  AIConfigResponse,
  AvailableVersionsResponse,
  ClusterComparisonResponse,
  ClusterDetailResponse,
  ClusterMetricsData,
  ClustersResponse,
  ConfigDiffResponse,
  ConnectionsListResponse,
  DashboardStats,
  DatadogNamespaceMetrics,
  ObservabilityOverviewResponse,
  PullRequestsResponse,
  UpgradeCheckResponse,
  VersionMatrixResponse,
} from './models'

const BASE_URL = '/api/v1'
const TOKEN_KEY = 'aap-auth-token'

function authHeaders(): Record<string, string> {
  const token = sessionStorage.getItem(TOKEN_KEY)
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    headers: authHeaders(),
  })
  if (res.status === 401) {
    sessionStorage.removeItem(TOKEN_KEY)
    window.location.reload()
    throw new Error('Session expired')
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

async function postJSON<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: body ? JSON.stringify(body) : undefined,
  })
  if (res.status === 401) {
    sessionStorage.removeItem(TOKEN_KEY)
    window.location.reload()
    throw new Error('Session expired')
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

async function putJSON<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: body ? JSON.stringify(body) : undefined,
  })
  if (res.status === 401) {
    sessionStorage.removeItem(TOKEN_KEY)
    window.location.reload()
    throw new Error('Session expired')
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

async function deleteJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (res.status === 401) {
    sessionStorage.removeItem(TOKEN_KEY)
    window.location.reload()
    throw new Error('Session expired')
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

export const api = {
  // Health
  health: () => fetchJSON<{ status: string }>('/health'),

  // Clusters
  getClusters: () => fetchJSON<ClustersResponse>('/clusters'),
  getCluster: (name: string) => fetchJSON<ClusterDetailResponse>(`/clusters/${name}`),
  getClusterComparison: (name: string) => fetchJSON<ClusterComparisonResponse>(`/clusters/${name}/comparison`),
  getClusterValues: (name: string) => fetchJSON<{ cluster_name: string; values_yaml: string }>(`/clusters/${name}/values`),
  getConfigDiff: (name: string) => fetchJSON<ConfigDiffResponse>(`/clusters/${name}/config-diff`),

  // Addons
  getAddonCatalog: () => fetchJSON<AddonCatalogResponse>('/addons/catalog'),
  getAddonDetail: (name: string) => fetchJSON<AddonDetailResponse>(`/addons/${name}`),
  getAddonValues: (name: string) => fetchJSON<{ addon_name: string; values_yaml: string }>(`/addons/${name}/values`),
  getVersionMatrix: () => fetchJSON<VersionMatrixResponse>('/addons/version-matrix'),

  // Dashboard
  getDashboardStats: () => fetchJSON<DashboardStats>('/dashboard/stats'),
  getPullRequests: () => fetchJSON<PullRequestsResponse>('/dashboard/pull-requests'),

  // Connections
  getConnections: () => fetchJSON<ConnectionsListResponse>('/connections/'),
  createConnection: (data: unknown) => postJSON('/connections/', data),
  updateConnection: (name: string, data: unknown) => putJSON(`/connections/${name}`, data),
  deleteConnection: (name: string) => deleteJSON(`/connections/${name}`),
  setActiveConnection: (name: string) => postJSON('/connections/active', { connection_name: name }),
  testConnection: () => postJSON<{ git: { status: string }; argocd: { status: string } }>('/connections/test'),

  // Observability
  getObservability: () => fetchJSON<ObservabilityOverviewResponse>('/observability/overview'),

  // Upgrade
  getUpgradeVersions: (addonName: string) => fetchJSON<AvailableVersionsResponse>(`/upgrade/${addonName}/versions`),
  checkUpgrade: (addonName: string, targetVersion: string) => postJSON<UpgradeCheckResponse>('/upgrade/check', { addon_name: addonName, target_version: targetVersion }),

  // AI
  getAIStatus: () => fetchJSON<{ enabled: boolean }>('/upgrade/ai-status'),
  getAISummary: (addonName: string, targetVersion: string) => postJSON<{ summary: string }>('/upgrade/ai-summary', { addon_name: addonName, target_version: targetVersion }),
  getAIConfig: () => fetchJSON<AIConfigResponse>('/ai/config'),
  setAIProvider: (provider: string) => postJSON<{ status: string; provider: string }>('/ai/provider', { provider }),
  testAI: () => postJSON<{ status: string; response: string }>('/ai/test', {}),

  // Datadog
  getDatadogStatus: () => fetchJSON<{ enabled: boolean; site: string }>('/datadog/status'),
  getDatadogNamespaceMetrics: (namespace: string) => fetchJSON<DatadogNamespaceMetrics>(`/datadog/metrics/${namespace}`),
  getClusterMetrics: (clusterName: string) => fetchJSON<ClusterMetricsData>(`/datadog/cluster-metrics/${clusterName}`),

  // Agent Chat
  agentChat: (sessionId: string, message: string) => postJSON<{ session_id: string; response: string }>('/agent/chat', { session_id: sessionId, message }),
  agentReset: (sessionId: string) => postJSON<{ status: string }>('/agent/reset', { session_id: sessionId }),
}
