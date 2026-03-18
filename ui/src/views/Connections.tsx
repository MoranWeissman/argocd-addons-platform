import { useState, useEffect, useCallback } from 'react'
import type { FormEvent } from 'react'
import {
  GitBranch,
  Server,
  Shield,
  Loader2,
  Plus,
  Pencil,
  X,
  Activity,
  Monitor,
  Globe,
  Sparkles,
  BarChart2,
  CheckCircle,
  XCircle,
} from 'lucide-react'
import { useConnections } from '@/hooks/useConnections'
import { api } from '@/services/api'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { Badge } from '@/components/ui/badge'
import type { AIConfigResponse, AIProviderInfo, ConnectionResponse } from '@/services/models'

interface PlatformInfo {
  status: string
}

interface ConnectionFormData {
  name: string
  description: string
  git_provider: 'github' | 'azuredevops'
  // GitHub fields
  github_owner: string
  github_repo: string
  github_token: string
  // Azure DevOps fields
  azure_org: string
  azure_project: string
  azure_repo: string
  azure_pat: string
  // ArgoCD fields
  argocd_server_url: string
  argocd_token: string
  argocd_namespace: string
}

const emptyForm: ConnectionFormData = {
  name: '',
  description: '',
  git_provider: 'github',
  github_owner: '',
  github_repo: '',
  github_token: '',
  azure_org: '',
  azure_project: '',
  azure_repo: '',
  azure_pat: '',
  argocd_server_url: '',
  argocd_token: '',
  argocd_namespace: 'argocd',
}

function buildPayload(form: ConnectionFormData) {
  const base = {
    name: form.name,
    description: form.description || undefined,
    git_provider: form.git_provider,
    argocd_server_url: form.argocd_server_url,
    argocd_namespace: form.argocd_namespace,
  }

  if (form.git_provider === 'github') {
    return {
      ...base,
      git_repo_identifier: `${form.github_owner}/${form.github_repo}`,
      git_token: form.github_token || undefined,
      argocd_token: form.argocd_token || undefined,
    }
  }
  return {
    ...base,
    git_repo_identifier: `${form.azure_org}/${form.azure_project}/${form.azure_repo}`,
    git_token: form.azure_pat || undefined,
    argocd_token: form.argocd_token || undefined,
  }
}

function formFromConnection(conn: ConnectionResponse): ConnectionFormData {
  const parts = conn.git_repo_identifier.split('/')
  if (conn.git_provider === 'github') {
    return {
      name: conn.name,
      description: conn.description ?? '',
      git_provider: 'github',
      github_owner: parts[0] ?? '',
      github_repo: parts.slice(1).join('/'),
      github_token: '',
      azure_org: '',
      azure_project: '',
      azure_repo: '',
      azure_pat: '',
      argocd_server_url: conn.argocd_server_url,
      argocd_token: '',
      argocd_namespace: conn.argocd_namespace,
    }
  }
  return {
    name: conn.name,
    description: conn.description ?? '',
    git_provider: 'azuredevops',
    github_owner: '',
    github_repo: '',
    github_token: '',
    azure_org: parts[0] ?? '',
    azure_project: parts[1] ?? '',
    azure_repo: parts.slice(2).join('/'),
    azure_pat: '',
    argocd_server_url: conn.argocd_server_url,
    argocd_token: '',
    argocd_namespace: conn.argocd_namespace,
  }
}

/* ------------------------------------------------------------------ */
/*  Shared form fields                                                 */
/* ------------------------------------------------------------------ */

const labelCls =
  'block text-sm font-medium text-gray-700 dark:text-gray-300'
const inputCls =
  'mt-1 block w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 dark:placeholder:text-gray-500'
const selectCls =
  'mt-1 block w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100'

function ConnectionFormFields({
  form,
  onChange,
  isEdit,
}: {
  form: ConnectionFormData
  onChange: (patch: Partial<ConnectionFormData>) => void
  isEdit: boolean
}) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      {/* Name */}
      <div>
        <label className={labelCls}>Name</label>
        <input
          className={inputCls}
          value={form.name}
          onChange={(e) => onChange({ name: e.target.value })}
          placeholder="e.g. production"
          disabled={isEdit}
          required
        />
      </div>

      {/* Description */}
      <div>
        <label className={labelCls}>Description</label>
        <input
          className={inputCls}
          value={form.description}
          onChange={(e) => onChange({ description: e.target.value })}
          placeholder="Optional description"
        />
      </div>

      {/* Git Provider */}
      <div>
        <label className={labelCls}>Git Provider</label>
        <select
          className={selectCls}
          value={form.git_provider}
          onChange={(e) =>
            onChange({
              git_provider: e.target.value as 'github' | 'azuredevops',
            })
          }
        >
          <option value="github">GitHub</option>
          <option value="azuredevops">Azure DevOps</option>
        </select>
      </div>

      {/* Conditional git fields */}
      {form.git_provider === 'github' ? (
        <>
          <div>
            <label className={labelCls}>Owner</label>
            <input
              className={inputCls}
              value={form.github_owner}
              onChange={(e) => onChange({ github_owner: e.target.value })}
              placeholder="e.g. my-org"
              required
            />
          </div>
          <div>
            <label className={labelCls}>Repository</label>
            <input
              className={inputCls}
              value={form.github_repo}
              onChange={(e) => onChange({ github_repo: e.target.value })}
              placeholder="e.g. k8s-addons"
              required
            />
          </div>
          <div>
            <label className={labelCls}>Token</label>
            <input
              className={inputCls}
              type="password"
              value={form.github_token}
              onChange={(e) => onChange({ github_token: e.target.value })}
              placeholder={
                isEdit ? 'Leave blank to keep existing' : 'ghp_...'
              }
            />
          </div>
        </>
      ) : (
        <>
          <div>
            <label className={labelCls}>Organization</label>
            <input
              className={inputCls}
              value={form.azure_org}
              onChange={(e) => onChange({ azure_org: e.target.value })}
              placeholder="e.g. MyOrg"
              required
            />
          </div>
          <div>
            <label className={labelCls}>Project</label>
            <input
              className={inputCls}
              value={form.azure_project}
              onChange={(e) => onChange({ azure_project: e.target.value })}
              placeholder="e.g. MyProject"
              required
            />
          </div>
          <div>
            <label className={labelCls}>Repository</label>
            <input
              className={inputCls}
              value={form.azure_repo}
              onChange={(e) => onChange({ azure_repo: e.target.value })}
              placeholder="e.g. k8s-addons"
              required
            />
          </div>
          <div>
            <label className={labelCls}>PAT</label>
            <input
              className={inputCls}
              type="password"
              value={form.azure_pat}
              onChange={(e) => onChange({ azure_pat: e.target.value })}
              placeholder={
                isEdit ? 'Leave blank to keep existing' : 'Personal Access Token'
              }
            />
          </div>
        </>
      )}

      {/* ArgoCD fields */}
      <div>
        <label className={labelCls}>ArgoCD URL</label>
        <input
          className={inputCls}
          value={form.argocd_server_url}
          onChange={(e) => onChange({ argocd_server_url: e.target.value })}
          placeholder="https://argocd.example.com"
          required
        />
      </div>
      <div>
        <label className={labelCls}>ArgoCD Token</label>
        <input
          className={inputCls}
          type="password"
          value={form.argocd_token}
          onChange={(e) => onChange({ argocd_token: e.target.value })}
          placeholder={
            isEdit ? 'Leave blank to keep existing' : 'ArgoCD auth token'
          }
        />
      </div>
      <div>
        <label className={labelCls}>ArgoCD Namespace</label>
        <input
          className={inputCls}
          value={form.argocd_namespace}
          onChange={(e) => onChange({ argocd_namespace: e.target.value })}
          placeholder="argocd"
          required
        />
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Main component                                                     */
/* ------------------------------------------------------------------ */

export function Connections() {
  const { connections, loading, error, refreshConnections } =
    useConnections()
  const [switchingTo, setSwitchingTo] = useState<string | null>(null)
  const [platformInfo, setPlatformInfo] = useState<PlatformInfo | null>(null)
  const [healthLoading, setHealthLoading] = useState(true)

  // Add form state
  const [showAddForm, setShowAddForm] = useState(false)
  const [addForm, setAddForm] = useState<ConnectionFormData>({ ...emptyForm })
  const [addSaving, setAddSaving] = useState(false)
  const [addError, setAddError] = useState<string | null>(null)

  // Edit form state
  const [editingName, setEditingName] = useState<string | null>(null)
  const [editForm, setEditForm] = useState<ConnectionFormData>({ ...emptyForm })
  const [editSaving, setEditSaving] = useState(false)
  const [editError, setEditError] = useState<string | null>(null)

  const fetchHealth = useCallback(() => {
    setHealthLoading(true)
    api
      .health()
      .then((data) => setPlatformInfo(data))
      .catch(() => setPlatformInfo(null))
      .finally(() => setHealthLoading(false))
  }, [])

  useEffect(() => {
    fetchHealth()
  }, [fetchHealth])

  async function handleSwitch(name: string) {
    setSwitchingTo(name)
    try {
      await api.setActiveConnection(name)
      refreshConnections()
    } finally {
      setSwitchingTo(null)
    }
  }

  async function handleAddSubmit(e: FormEvent) {
    e.preventDefault()
    setAddSaving(true)
    setAddError(null)
    try {
      await api.createConnection(buildPayload(addForm))
      refreshConnections()
      setShowAddForm(false)
      setAddForm({ ...emptyForm })
    } catch (err) {
      setAddError(err instanceof Error ? err.message : 'Failed to create connection')
    } finally {
      setAddSaving(false)
    }
  }

  function handleEditStart(conn: ConnectionResponse) {
    setEditingName(conn.name)
    setEditForm(formFromConnection(conn))
    setEditError(null)
  }

  async function handleEditSubmit(e: FormEvent) {
    e.preventDefault()
    if (!editingName) return
    setEditSaving(true)
    setEditError(null)
    try {
      await api.updateConnection(editingName, buildPayload(editForm))
      refreshConnections()
      setEditingName(null)
    } catch (err) {
      setEditError(err instanceof Error ? err.message : 'Failed to update connection')
    } finally {
      setEditSaving(false)
    }
  }

  if (loading) {
    return <LoadingState message="Loading settings..." />
  }

  if (error) {
    return <ErrorState message={error} onRetry={refreshConnections} />
  }

  const activeConn = connections.find((c) => c.is_active) ?? null

  return (
    <div className="space-y-8">
      {/* Header */}
      <div>
        <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100">
          Settings
        </h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Manage connections and view platform information.
        </p>
      </div>

      {/* Active Connections */}
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
            Active Connections
          </h3>
          <button
            onClick={() => {
              setShowAddForm((prev) => !prev)
              setAddForm({ ...emptyForm })
              setAddError(null)
            }}
            className="inline-flex items-center gap-1.5 rounded-lg bg-cyan-600 px-3 py-1.5 text-sm font-medium text-white shadow-sm hover:bg-cyan-700 focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:ring-offset-2 dark:bg-cyan-700 dark:hover:bg-cyan-600 dark:focus:ring-offset-gray-900"
          >
            <Plus className="h-4 w-4" />
            Add Connection
          </button>
        </div>

        {/* Add Connection Form */}
        {showAddForm && (
          <form
            onSubmit={handleAddSubmit}
            className="rounded-xl border border-cyan-200 bg-cyan-50/50 p-6 shadow-sm dark:border-cyan-800 dark:bg-cyan-950/20"
          >
            <h4 className="mb-4 text-base font-semibold text-gray-900 dark:text-gray-100">
              New Connection
            </h4>
            <ConnectionFormFields
              form={addForm}
              onChange={(patch) =>
                setAddForm((prev) => ({ ...prev, ...patch }))
              }
              isEdit={false}
            />
            {addError && (
              <p className="mt-3 text-sm text-red-600 dark:text-red-400">
                {addError}
              </p>
            )}
            <div className="mt-4 flex items-center gap-3">
              <button
                type="submit"
                disabled={addSaving}
                className="inline-flex items-center gap-1.5 rounded-lg bg-cyan-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-cyan-700 disabled:opacity-50 dark:bg-cyan-700 dark:hover:bg-cyan-600"
              >
                {addSaving && (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                )}
                Save
              </button>
              <button
                type="button"
                onClick={() => setShowAddForm(false)}
                className="rounded-lg px-4 py-2 text-sm font-medium text-gray-600 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
              >
                Cancel
              </button>
            </div>
          </form>
        )}

        {connections.length === 0 ? (
          <p className="py-8 text-center text-gray-400 dark:text-gray-500">
            No connections configured.
          </p>
        ) : (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            {connections.map((conn) => (
              <div key={conn.name}>
                <div
                  className={`rounded-xl border bg-white p-6 shadow-sm dark:bg-gray-800 ${
                    conn.is_active
                      ? 'border-cyan-500 ring-2 ring-cyan-100 dark:ring-cyan-900/50'
                      : 'border-gray-200 dark:border-gray-700'
                  }`}
                >
                  {/* Name + badges */}
                  <div className="mb-4 flex items-center justify-between">
                    <div className="flex flex-wrap items-center gap-2">
                      <h4 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
                        {conn.name}
                      </h4>
                      {conn.is_default && (
                        <Badge variant="secondary" className="text-xs">
                          Default
                        </Badge>
                      )}
                      {conn.is_active && (
                        <Badge className="bg-cyan-100 text-xs text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400">
                          Active
                        </Badge>
                      )}
                    </div>

                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handleEditStart(conn)}
                        className="inline-flex items-center gap-1 text-xs font-medium text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                      >
                        <Pencil className="h-3 w-3" />
                        Edit
                      </button>
                      {!conn.is_active && (
                        <button
                          onClick={() => handleSwitch(conn.name)}
                          disabled={switchingTo === conn.name}
                          className="text-xs font-medium text-cyan-600 hover:text-cyan-700 disabled:opacity-50 dark:text-cyan-400 dark:hover:text-cyan-300"
                        >
                          {switchingTo === conn.name ? (
                            <Loader2 className="inline h-3 w-3 animate-spin" />
                          ) : (
                            'Switch'
                          )}
                        </button>
                      )}
                    </div>
                  </div>

                  {/* Details */}
                  <dl className="space-y-2 text-sm">
                    <div className="flex items-center justify-between">
                      <dt className="flex items-center gap-1.5 text-gray-500 dark:text-gray-400">
                        <GitBranch className="h-3.5 w-3.5" />
                        Git Provider
                      </dt>
                      <dd className="font-medium capitalize text-gray-900 dark:text-gray-100">
                        {conn.git_provider}
                      </dd>
                    </div>
                    <div className="flex items-center justify-between">
                      <dt className="flex items-center gap-1.5 text-gray-500 dark:text-gray-400">
                        <GitBranch className="h-3.5 w-3.5" />
                        Repository
                      </dt>
                      <dd className="font-mono text-xs text-gray-700 dark:text-gray-300">
                        {conn.git_repo_identifier}
                      </dd>
                    </div>
                    <div className="flex items-start justify-between gap-2">
                      <dt className="flex shrink-0 items-center gap-1.5 text-gray-500 dark:text-gray-400">
                        <Server className="h-3.5 w-3.5" />
                        ArgoCD URL
                      </dt>
                      <dd className="break-all text-right font-mono text-xs text-gray-700 dark:text-gray-300">
                        {conn.argocd_server_url}
                      </dd>
                    </div>
                    <div className="flex items-center justify-between">
                      <dt className="flex items-center gap-1.5 text-gray-500 dark:text-gray-400">
                        <Shield className="h-3.5 w-3.5" />
                        Namespace
                      </dt>
                      <dd className="font-mono text-xs text-gray-700 dark:text-gray-300">
                        {conn.argocd_namespace}
                      </dd>
                    </div>
                  </dl>
                </div>

                {/* Inline Edit Form */}
                {editingName === conn.name && (
                  <form
                    onSubmit={handleEditSubmit}
                    className="mt-2 rounded-xl border border-amber-200 bg-amber-50/50 p-6 shadow-sm dark:border-amber-800 dark:bg-amber-950/20"
                  >
                    <div className="mb-4 flex items-center justify-between">
                      <h4 className="text-base font-semibold text-gray-900 dark:text-gray-100">
                        Edit Connection
                      </h4>
                      <button
                        type="button"
                        onClick={() => setEditingName(null)}
                        className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                      >
                        <X className="h-4 w-4" />
                      </button>
                    </div>
                    <ConnectionFormFields
                      form={editForm}
                      onChange={(patch) =>
                        setEditForm((prev) => ({ ...prev, ...patch }))
                      }
                      isEdit={true}
                    />
                    {editError && (
                      <p className="mt-3 text-sm text-red-600 dark:text-red-400">
                        {editError}
                      </p>
                    )}
                    <div className="mt-4 flex items-center gap-3">
                      <button
                        type="submit"
                        disabled={editSaving}
                        className="inline-flex items-center gap-1.5 rounded-lg bg-amber-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-amber-700 disabled:opacity-50 dark:bg-amber-700 dark:hover:bg-amber-600"
                      >
                        {editSaving && (
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        )}
                        Update
                      </button>
                      <button
                        type="button"
                        onClick={() => setEditingName(null)}
                        className="rounded-lg px-4 py-2 text-sm font-medium text-gray-600 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
                      >
                        Cancel
                      </button>
                    </div>
                  </form>
                )}
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Platform Info */}
      <section className="space-y-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
          Platform Info
        </h3>
        <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <dl className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            {/* Deployment Mode */}
            <div>
              <dt className="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400">
                <Monitor className="h-3.5 w-3.5" />
                Deployment Mode
              </dt>
              <dd className="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">
                Local Development
              </dd>
            </div>

            {/* API Health */}
            <div>
              <dt className="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400">
                <Activity className="h-3.5 w-3.5" />
                API Health
              </dt>
              <dd className="mt-1 flex items-center gap-2 text-sm font-medium">
                {healthLoading ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin text-gray-400 dark:text-gray-500" />
                ) : platformInfo?.status === 'ok' || platformInfo?.status === 'healthy' ? (
                  <>
                    <span className="inline-block h-2.5 w-2.5 rounded-full bg-green-500" />
                    <span className="text-green-600 dark:text-green-400">Healthy</span>
                  </>
                ) : (
                  <>
                    <span className="inline-block h-2.5 w-2.5 rounded-full bg-red-500" />
                    <span className="text-red-600 dark:text-red-400">{platformInfo?.status ?? 'Unreachable'}</span>
                  </>
                )}
              </dd>
            </div>

            {/* Git Provider */}
            <div>
              <dt className="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400">
                <GitBranch className="h-3.5 w-3.5" />
                Git Provider
              </dt>
              <dd className="mt-1 text-sm font-medium capitalize text-gray-900 dark:text-gray-100">
                {activeConn?.git_provider ?? 'N/A'}
              </dd>
            </div>

            {/* ArgoCD Server */}
            <div>
              <dt className="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400">
                <Globe className="h-3.5 w-3.5" />
                ArgoCD Server
              </dt>
              <dd className="mt-1 break-all font-mono text-sm text-gray-900 dark:text-gray-100">
                {activeConn?.argocd_server_url ?? 'N/A'}
              </dd>
            </div>
          </dl>
        </div>
      </section>

      {/* AI Configuration */}
      <section className="space-y-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
          AI Configuration
        </h3>
        <AIConfigSection />
      </section>

      {/* Datadog Metrics */}
      <section className="space-y-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
          Datadog Metrics
        </h3>
        <DatadogConfigSection />
      </section>
    </div>
  )
}

function AIConfigSection() {
  const [aiConfig, setAiConfig] = useState<AIConfigResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [testResult, setTestResult] = useState<string | null>(null)
  const [testing, setTesting] = useState(false)
  const [switching, setSwitching] = useState<string | null>(null)

  const fetchConfig = useCallback(() => {
    setLoading(true)
    api.getAIConfig()
      .then(setAiConfig)
      .catch(() => setAiConfig(null))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    fetchConfig()
  }, [fetchConfig])

  const handleTest = async () => {
    setTesting(true)
    setTestResult(null)
    try {
      const res = await api.testAI()
      setTestResult(res.status === 'ok' ? 'AI is responding correctly' : 'AI returned unexpected response')
    } catch (err) {
      setTestResult(err instanceof Error ? err.message : 'Connection failed')
    } finally {
      setTesting(false)
    }
  }

  const handleSwitchProvider = async (providerId: string) => {
    setSwitching(providerId)
    setTestResult(null)
    try {
      await api.setAIProvider(providerId)
      fetchConfig()
    } catch (err) {
      setTestResult(err instanceof Error ? err.message : 'Failed to switch provider')
    } finally {
      setSwitching(null)
    }
  }

  const isEnabled = aiConfig?.current_provider && aiConfig.current_provider !== 'none' && aiConfig.current_provider !== ''
  const activeProvider = aiConfig?.available_providers.find((p: AIProviderInfo) => p.id === aiConfig.current_provider)

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${
            isEnabled
              ? 'bg-purple-100 dark:bg-purple-900/30'
              : 'bg-gray-100 dark:bg-gray-700'
          }`}>
            <Sparkles className={`h-5 w-5 ${isEnabled ? 'text-purple-600 dark:text-purple-400' : 'text-gray-400'}`} />
          </div>
          <div>
            <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100">
              AI Analysis
              {loading ? '' : isEnabled ? (
                <span className="ml-2 inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900/30 dark:text-green-400">
                  <span className="inline-block h-1.5 w-1.5 rounded-full bg-green-500" />
                  Active
                </span>
              ) : (
                <span className="ml-2 inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500 dark:bg-gray-700 dark:text-gray-400">
                  Disabled
                </span>
              )}
            </h4>
            <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              {isEnabled && activeProvider
                ? `Using ${activeProvider.name}${activeProvider.model ? ` — ${activeProvider.model}` : ''}`
                : 'AI-powered analysis for the Upgrade Impact Checker (Ollama, Claude, OpenAI, or Gemini)'
              }
            </p>
          </div>
        </div>
        {isEnabled && (
          <button
            onClick={handleTest}
            disabled={testing}
            className="rounded-lg border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
          >
            {testing ? 'Testing...' : 'Test Connection'}
          </button>
        )}
      </div>

      {testResult && (
        <div className={`mt-3 rounded-lg px-3 py-2 text-xs ${
          testResult.includes('correctly')
            ? 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
            : 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400'
        }`}>
          {testResult}
        </div>
      )}

      {/* Provider Selector Cards */}
      {!loading && aiConfig && (
        <div className="mt-5">
          <p className="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">
            Available Providers
          </p>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {aiConfig.available_providers.map((provider: AIProviderInfo) => {
              const isActive = provider.id === aiConfig.current_provider
              const canSwitch = provider.configured && !isActive

              const providerMeta: Record<string, { privacy: string; privacyColor: string; toolCalling: string; toolColor: string; note: string }> = {
                ollama: {
                  privacy: 'Local — no data leaves the cluster',
                  privacyColor: 'text-green-600 dark:text-green-400',
                  toolCalling: 'Depends on model (3B weak, 8B+ moderate, 70B+ good)',
                  toolColor: 'text-amber-600 dark:text-amber-400',
                  note: 'Requires persistent storage. Image ~3GB. Models 2-40+ GB RAM.',
                },
                claude: {
                  privacy: 'Data sent to Anthropic (external)',
                  privacyColor: 'text-amber-600 dark:text-amber-400',
                  toolCalling: 'Excellent — best reasoning and tool use',
                  toolColor: 'text-green-600 dark:text-green-400',
                  note: 'Cluster names, addon configs, and health data are sent to the API.',
                },
                openai: {
                  privacy: 'Data sent to OpenAI (external)',
                  privacyColor: 'text-amber-600 dark:text-amber-400',
                  toolCalling: 'Very good',
                  toolColor: 'text-green-600 dark:text-green-400',
                  note: 'Cluster names, addon configs, and health data are sent to the API.',
                },
                gemini: {
                  privacy: 'Data sent to Google (external)',
                  privacyColor: 'text-amber-600 dark:text-amber-400',
                  toolCalling: 'Good — free tier available',
                  toolColor: 'text-green-600 dark:text-green-400',
                  note: 'Cluster names, addon configs, and health data are sent to the API.',
                },
              }
              const meta = providerMeta[provider.id]

              return (
                <div
                  key={provider.id}
                  className={`relative rounded-lg border p-4 transition-all ${
                    isActive
                      ? 'border-purple-500 bg-purple-50 ring-2 ring-purple-200 dark:border-purple-400 dark:bg-purple-950/20 dark:ring-purple-900/50'
                      : provider.configured
                        ? 'border-gray-200 bg-white hover:border-gray-300 dark:border-gray-600 dark:bg-gray-800 dark:hover:border-gray-500'
                        : 'border-gray-300 bg-gray-50 opacity-60 dark:border-gray-700 dark:bg-gray-900'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className={`text-sm font-semibold ${
                      isActive
                        ? 'text-purple-700 dark:text-purple-300'
                        : 'text-gray-900 dark:text-gray-100'
                    }`}>
                      {provider.name}
                    </span>
                    {provider.configured ? (
                      <span className="inline-block h-2 w-2 rounded-full bg-green-500" title="Configured" />
                    ) : (
                      <span className="inline-block h-2 w-2 rounded-full bg-gray-300 dark:bg-gray-600" title="Not configured" />
                    )}
                  </div>

                  <p className="mt-1 font-mono text-xs text-gray-500 dark:text-gray-400">
                    {provider.configured && provider.model
                      ? provider.model
                      : 'Not configured'}
                  </p>

                  {meta && (
                    <div className="mt-2 space-y-1 border-t border-gray-100 pt-2 dark:border-gray-700">
                      <p className={`text-[10px] ${meta.privacyColor}`}>
                        {meta.privacy}
                      </p>
                      <p className={`text-[10px] ${meta.toolColor}`}>
                        Tool calling: {meta.toolCalling}
                      </p>
                      <p className="text-[10px] text-gray-400 dark:text-gray-500">
                        {meta.note}
                      </p>
                    </div>
                  )}

                  <div className="mt-3">
                    {isActive ? (
                      <span className="inline-flex items-center gap-1 text-xs font-medium text-purple-600 dark:text-purple-400">
                        <span className="inline-block h-1.5 w-1.5 rounded-full bg-purple-500" />
                        Active
                      </span>
                    ) : canSwitch ? (
                      <button
                        onClick={() => handleSwitchProvider(provider.id)}
                        disabled={switching === provider.id}
                        className="inline-flex items-center gap-1 rounded-md bg-purple-600 px-2.5 py-1 text-xs font-medium text-white transition-colors hover:bg-purple-700 disabled:opacity-50 dark:bg-purple-700 dark:hover:bg-purple-600"
                      >
                        {switching === provider.id ? (
                          <Loader2 className="h-3 w-3 animate-spin" />
                        ) : null}
                        Switch
                      </button>
                    ) : (
                      <span className="text-xs text-gray-400 dark:text-gray-500">
                        Setup required
                      </span>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Setup guide for unconfigured providers */}
      {!loading && aiConfig && aiConfig.available_providers.some((p: AIProviderInfo) => !p.configured) && (
        <div className="mt-5 rounded-lg bg-gray-50 p-4 dark:bg-gray-900">
          <p className="text-sm font-medium text-gray-700 dark:text-gray-300">How to enable additional providers</p>
          <p className="mt-2 text-xs text-gray-600 dark:text-gray-400">
            Add the configuration to{' '}
            <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">.env.secrets</code> and restart:
          </p>

          <div className="mt-3 space-y-3">
            {!aiConfig.available_providers.find((p: AIProviderInfo) => p.id === 'ollama')?.configured && (
              <div>
                <p className="text-xs font-semibold text-gray-700 dark:text-gray-300">Ollama (local, free)</p>
                <ol className="mt-1 space-y-1 text-xs text-gray-600 dark:text-gray-400">
                  <li className="flex gap-2">
                    <span className="font-medium text-gray-500">1.</span>
                    Install: <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">brew install ollama</code>
                  </li>
                  <li className="flex gap-2">
                    <span className="font-medium text-gray-500">2.</span>
                    Start: <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">ollama serve</code>
                  </li>
                  <li className="flex gap-2">
                    <span className="font-medium text-gray-500">3.</span>
                    Pull model: <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">ollama pull llama3.2</code>
                  </li>
                </ol>
                <pre className="mt-1 rounded-lg bg-gray-900 p-2 font-mono text-xs text-gray-300">
{`AI_PROVIDER=ollama
AI_OLLAMA_URL=http://localhost:11434
AI_OLLAMA_MODEL=llama3.2`}
                </pre>
              </div>
            )}

            {!aiConfig.available_providers.find((p: AIProviderInfo) => p.id === 'claude')?.configured && (
              <div>
                <p className="text-xs font-semibold text-gray-700 dark:text-gray-300">Claude (Anthropic)</p>
                <pre className="mt-1 rounded-lg bg-gray-900 p-2 font-mono text-xs text-gray-300">
{`AI_PROVIDER=claude
AI_API_KEY=sk-ant-...
AI_CLOUD_MODEL=claude-sonnet-4-20250514`}
                </pre>
              </div>
            )}

            {!aiConfig.available_providers.find((p: AIProviderInfo) => p.id === 'openai')?.configured && (
              <div>
                <p className="text-xs font-semibold text-gray-700 dark:text-gray-300">OpenAI</p>
                <pre className="mt-1 rounded-lg bg-gray-900 p-2 font-mono text-xs text-gray-300">
{`AI_PROVIDER=openai
AI_API_KEY=sk-...
AI_CLOUD_MODEL=gpt-4o`}
                </pre>
              </div>
            )}

            {!aiConfig.available_providers.find((p: AIProviderInfo) => p.id === 'gemini')?.configured && (
              <div>
                <p className="text-xs font-semibold text-gray-700 dark:text-gray-300">Google Gemini (free tier available)</p>
                <pre className="mt-1 rounded-lg bg-gray-900 p-2 font-mono text-xs text-gray-300">
{`AI_PROVIDER=gemini
AI_API_KEY=AIzaSy...
AI_CLOUD_MODEL=gemini-2.5-flash`}
                </pre>
              </div>
            )}
          </div>

          <p className="mt-3 text-xs text-gray-500 dark:text-gray-400">
            Then restart the platform with <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">make dev</code>
          </p>
        </div>
      )}
    </div>
  )
}

function DatadogConfigSection() {
  const [status, setStatus] = useState<{ enabled: boolean; site: string } | null>(null)
  const [loading, setLoading] = useState(true)
  const [testResult, setTestResult] = useState<string | null>(null)
  const [testing, setTesting] = useState(false)

  const fetchStatus = useCallback(() => {
    setLoading(true)
    api.getDatadogStatus()
      .then(setStatus)
      .catch(() => setStatus(null))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    fetchStatus()
  }, [fetchStatus])

  const handleTest = async () => {
    setTesting(true)
    setTestResult(null)
    try {
      // Test by fetching metrics for a known namespace
      await api.getDatadogNamespaceMetrics('default')
      setTestResult('Datadog connection successful')
    } catch (err) {
      setTestResult(err instanceof Error ? err.message : 'Connection failed')
    } finally {
      setTesting(false)
    }
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${
            status?.enabled
              ? 'bg-indigo-100 dark:bg-indigo-900/30'
              : 'bg-gray-100 dark:bg-gray-700'
          }`}>
            <BarChart2 className={`h-5 w-5 ${status?.enabled ? 'text-indigo-600 dark:text-indigo-400' : 'text-gray-400'}`} />
          </div>
          <div>
            <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100">
              Datadog Metrics
              {loading ? '' : status?.enabled ? (
                <span className="ml-2 inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900/30 dark:text-green-400">
                  <span className="inline-block h-1.5 w-1.5 rounded-full bg-green-500" />
                  Active
                </span>
              ) : (
                <span className="ml-2 inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500 dark:bg-gray-700 dark:text-gray-400">
                  Disabled
                </span>
              )}
            </h4>
            <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              {status?.enabled
                ? `Connected to ${status.site}`
                : 'Real-time K8s resource metrics from Datadog (CPU, memory, pods)'
              }
            </p>
          </div>
        </div>
        {status?.enabled && (
          <button
            onClick={handleTest}
            disabled={testing}
            className="rounded-lg border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
          >
            {testing ? 'Testing...' : 'Test Connection'}
          </button>
        )}
      </div>

      {testResult && (
        <div className={`mt-3 flex items-center gap-2 rounded-lg px-3 py-2 text-xs ${
          testResult.includes('successful')
            ? 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
            : 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400'
        }`}>
          {testResult.includes('successful') ? (
            <CheckCircle className="h-3.5 w-3.5" />
          ) : (
            <XCircle className="h-3.5 w-3.5" />
          )}
          {testResult}
        </div>
      )}

      {!loading && !status?.enabled && (
        <div className="mt-4 rounded-lg bg-gray-50 p-4 dark:bg-gray-900">
          <p className="text-sm font-medium text-gray-700 dark:text-gray-300">How to enable Datadog metrics</p>
          <p className="mt-2 text-xs text-gray-600 dark:text-gray-400">
            Add the following to{' '}
            <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">.env.secrets</code> and restart:
          </p>
          <pre className="mt-2 rounded-lg bg-gray-900 p-2 font-mono text-xs text-gray-300">
{`DATADOG_API_KEY=your-datadog-api-key
DATADOG_APP_KEY=your-datadog-app-key
DATADOG_SITE=datadoghq.com`}
          </pre>
          <p className="mt-3 text-xs text-gray-500 dark:text-gray-400">
            Then restart the platform with <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">make dev</code>
          </p>
        </div>
      )}
    </div>
  )
}
