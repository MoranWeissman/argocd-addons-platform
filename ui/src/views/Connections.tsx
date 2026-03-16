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
} from 'lucide-react'
import { useConnections } from '@/hooks/useConnections'
import { api } from '@/services/api'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { Badge } from '@/components/ui/badge'
import type { ConnectionResponse } from '@/services/models'

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
    </div>
  )
}

function AIConfigSection() {
  const [aiStatus, setAiStatus] = useState<{ enabled: boolean } | null>(null)
  const [loading, setLoading] = useState(true)
  const [testResult, setTestResult] = useState<string | null>(null)
  const [testing, setTesting] = useState(false)

  useEffect(() => {
    api.getAIStatus()
      .then(setAiStatus)
      .catch(() => setAiStatus({ enabled: false }))
      .finally(() => setLoading(false))
  }, [])

  const handleTest = async () => {
    setTesting(true)
    setTestResult(null)
    try {
      const res = await api.getAISummary('test', '0.0.0')
      setTestResult(res.summary ? 'AI is responding correctly' : 'AI returned empty response')
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
            aiStatus?.enabled
              ? 'bg-purple-100 dark:bg-purple-900/30'
              : 'bg-gray-100 dark:bg-gray-700'
          }`}>
            <Sparkles className={`h-5 w-5 ${aiStatus?.enabled ? 'text-purple-600 dark:text-purple-400' : 'text-gray-400'}`} />
          </div>
          <div>
            <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100">
              AI Analysis
              {loading ? '' : aiStatus?.enabled ? (
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
              {aiStatus?.enabled
                ? 'Ollama is connected and providing AI-powered upgrade analysis'
                : 'AI-powered analysis for the Upgrade Impact Checker'
              }
            </p>
          </div>
        </div>
        {aiStatus?.enabled && (
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

      {!aiStatus?.enabled && !loading && (
        <div className="mt-4 rounded-lg bg-gray-50 p-4 dark:bg-gray-900">
          <p className="text-sm font-medium text-gray-700 dark:text-gray-300">How to enable</p>
          <ol className="mt-2 space-y-1.5 text-xs text-gray-600 dark:text-gray-400">
            <li className="flex gap-2">
              <span className="font-medium text-gray-500">1.</span>
              Install Ollama: <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">brew install ollama</code>
            </li>
            <li className="flex gap-2">
              <span className="font-medium text-gray-500">2.</span>
              Start Ollama: <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">ollama serve</code>
            </li>
            <li className="flex gap-2">
              <span className="font-medium text-gray-500">3.</span>
              Pull a model: <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">ollama pull llama3.2</code>
            </li>
            <li className="flex gap-2">
              <span className="font-medium text-gray-500">4.</span>
              Add to <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">.env.secrets</code>:
            </li>
          </ol>
          <pre className="mt-2 rounded-lg bg-gray-900 p-3 font-mono text-xs text-gray-300">
{`AI_PROVIDER=ollama
AI_OLLAMA_URL=http://localhost:11434
AI_OLLAMA_MODEL=llama3.2`}
          </pre>
          <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
            Then restart the platform with <code className="rounded bg-gray-200 px-1 dark:bg-gray-700">make dev</code>
          </p>
        </div>
      )}
    </div>
  )
}
