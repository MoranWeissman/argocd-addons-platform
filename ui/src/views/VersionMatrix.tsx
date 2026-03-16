import { useState, useEffect, useMemo } from 'react'
import { Search, ChevronDown, ChevronRight } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '@/services/api'
import type { VersionMatrixResponse, VersionMatrixRow, VersionMatrixCell } from '@/services/models'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'

type HealthFilter = 'all' | 'healthy' | 'issues' | 'not_deployed'

function healthColor(health: string): string {
  switch (health.toLowerCase()) {
    case 'healthy': return 'bg-green-500'
    case 'degraded': return 'bg-red-500'
    case 'missing': case 'progressing': return 'bg-amber-500'
    default: return 'bg-gray-400'
  }
}

function healthRing(health: string): string {
  switch (health.toLowerCase()) {
    case 'healthy': return 'ring-green-200 dark:ring-green-800'
    case 'degraded': return 'ring-red-200 dark:ring-red-800'
    case 'missing': case 'progressing': return 'ring-amber-200 dark:ring-amber-800'
    default: return 'ring-gray-200 dark:ring-gray-700'
  }
}

function healthLabel(health: string): string {
  switch (health.toLowerCase()) {
    case 'healthy': return 'Healthy'
    case 'degraded': return 'Degraded'
    case 'missing': return 'Not Deployed'
    case 'progressing': return 'Progressing'
    case 'not_enabled': return 'Disabled'
    default: return 'Unknown'
  }
}

function matchesHealth(row: VersionMatrixRow, filter: HealthFilter): boolean {
  if (filter === 'all') return true
  const activeCells = Object.values(row.cells).filter(c => c.health !== 'not_enabled')
  if (filter === 'healthy') return activeCells.length > 0 && activeCells.every(c => c.health.toLowerCase() === 'healthy')
  if (filter === 'issues') return activeCells.some(c => ['degraded', 'unknown'].includes(c.health.toLowerCase()))
  if (filter === 'not_deployed') return activeCells.some(c => c.health.toLowerCase() === 'missing')
  return true
}

function ClusterChip({ cluster, cell, addonName }: { cluster: string; cell: VersionMatrixCell; addonName: string }) {
  const navigate = useNavigate()
  const isDrift = cell.drift_from_catalog

  return (
    <button
      type="button"
      onClick={() => navigate(`/clusters/${cluster}`)}
      title={`${addonName} on ${cluster}\nVersion: ${cell.version}\nHealth: ${healthLabel(cell.health)}${isDrift ? '\nVersion drift from catalog' : ''}`}
      className={`group inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-xs transition-all hover:shadow-md
        ${isDrift
          ? 'border-amber-300 bg-amber-50 dark:border-amber-700 dark:bg-amber-900/20'
          : 'border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800'
        }
        hover:ring-2 ${healthRing(cell.health)}
      `}
    >
      <span className={`inline-block h-2.5 w-2.5 shrink-0 rounded-full ${healthColor(cell.health)}`} />
      <span className="max-w-[140px] truncate font-medium text-gray-700 dark:text-gray-300 group-hover:text-gray-900 dark:group-hover:text-white">
        {cluster.replace(/-eks$/, '')}
      </span>
      <span className={`font-mono text-[10px] ${isDrift ? 'font-bold text-amber-600 dark:text-amber-400' : 'text-gray-400 dark:text-gray-500'}`}>
        {cell.version}
      </span>
    </button>
  )
}

function AddonRow({ row, clusters }: { row: VersionMatrixRow; clusters: string[] }) {
  const [expanded, setExpanded] = useState(true)

  const activeCells = useMemo(() => {
    const entries: { cluster: string; cell: VersionMatrixCell }[] = []
    for (const cluster of clusters) {
      const cell = row.cells[cluster]
      if (cell && cell.health !== 'not_enabled') {
        entries.push({ cluster, cell })
      }
    }
    return entries
  }, [row, clusters])

  const healthyCount = activeCells.filter(e => e.cell.health.toLowerCase() === 'healthy').length
  const issueCount = activeCells.filter(e => !['healthy', 'not_enabled'].includes(e.cell.health.toLowerCase())).length
  const driftCount = activeCells.filter(e => e.cell.drift_from_catalog).length

  if (activeCells.length === 0) return null

  return (
    <div className="rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800/50">
      {/* Header */}
      <button
        type="button"
        onClick={() => setExpanded(v => !v)}
        className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-gray-800"
      >
        {expanded
          ? <ChevronDown className="h-4 w-4 shrink-0 text-gray-400" />
          : <ChevronRight className="h-4 w-4 shrink-0 text-gray-400" />
        }
        <div className="min-w-0 flex-1">
          <span className="text-sm font-semibold text-gray-900 dark:text-white">{row.addon_name}</span>
          <span className="ml-2 text-xs text-gray-400 dark:text-gray-500">v{row.catalog_version}</span>
        </div>
        <div className="flex items-center gap-3 text-xs">
          <span className="flex items-center gap-1 text-gray-500 dark:text-gray-400">
            <span className="inline-block h-2 w-2 rounded-full bg-green-500" />
            {healthyCount}
          </span>
          {issueCount > 0 && (
            <span className="flex items-center gap-1 text-red-600 dark:text-red-400">
              <span className="inline-block h-2 w-2 rounded-full bg-red-500" />
              {issueCount}
            </span>
          )}
          {driftCount > 0 && (
            <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">
              {driftCount} drift
            </span>
          )}
          <span className="text-gray-400 dark:text-gray-500">
            {activeCells.length} cluster{activeCells.length !== 1 ? 's' : ''}
          </span>
        </div>
      </button>

      {/* Cluster chips */}
      {expanded && (
        <div className="flex flex-wrap gap-2 border-t border-gray-100 px-4 py-3 dark:border-gray-700">
          {activeCells.map(({ cluster, cell }) => (
            <ClusterChip key={cluster} cluster={cluster} cell={cell} addonName={row.addon_name} />
          ))}
        </div>
      )}
    </div>
  )
}

export function VersionMatrix() {
  const [data, setData] = useState<VersionMatrixResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [healthFilter, setHealthFilter] = useState<HealthFilter>('all')
  const [showDriftOnly, setShowDriftOnly] = useState(false)

  const fetchData = () => {
    setLoading(true)
    setError(null)
    api.getVersionMatrix()
      .then(setData)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchData() }, [])

  const filteredAddons = useMemo(() => {
    if (!data) return []
    return data.addons.filter(row => {
      // Must have at least one active cell
      const hasActive = Object.values(row.cells).some(c => c.health !== 'not_enabled')
      if (!hasActive) return false
      if (search && !row.addon_name.toLowerCase().includes(search.toLowerCase())) return false
      if (!matchesHealth(row, healthFilter)) return false
      if (showDriftOnly && !Object.values(row.cells).some(c => c.drift_from_catalog)) return false
      return true
    })
  }, [data, search, healthFilter, showDriftOnly])

  const totalDeployed = useMemo(() => {
    if (!data) return 0
    return data.addons.reduce((sum, row) =>
      sum + Object.values(row.cells).filter(c => c.health !== 'not_enabled' && c.health.toLowerCase() !== 'missing').length, 0)
  }, [data])

  const totalDrifts = useMemo(() => {
    if (!data) return 0
    return data.addons.reduce((sum, row) =>
      sum + Object.values(row.cells).filter(c => c.drift_from_catalog).length, 0)
  }, [data])

  if (loading) return <LoadingState message="Loading version matrix..." />
  if (error) return <ErrorState message={error} onRetry={fetchData} />
  if (!data) return null

  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Addon Version Matrix</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Version and health status of every addon across all clusters
        </p>
      </div>

      {/* Stats */}
      <div className="flex flex-wrap gap-3 text-sm">
        <span className="rounded-full bg-gray-100 px-3 py-1 font-medium text-gray-700 dark:bg-gray-800 dark:text-gray-300">
          {filteredAddons.length} addons
        </span>
        <span className="rounded-full bg-gray-100 px-3 py-1 font-medium text-gray-700 dark:bg-gray-800 dark:text-gray-300">
          {data.clusters.length} clusters
        </span>
        <span className="rounded-full bg-cyan-50 px-3 py-1 font-medium text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400">
          {totalDeployed} deployed
        </span>
        {totalDrifts > 0 && (
          <span className="rounded-full bg-amber-50 px-3 py-1 font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">
            {totalDrifts} version {totalDrifts === 1 ? 'drift' : 'drifts'}
          </span>
        )}
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
          <input
            type="text"
            placeholder="Search addon by name..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="w-56 rounded-lg border border-gray-300 py-2 pl-10 pr-3 text-sm focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
          />
        </div>
        <select value={healthFilter} onChange={e => setHealthFilter(e.target.value as HealthFilter)}
          className="rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200">
          <option value="all">All Health</option>
          <option value="healthy">Healthy Only</option>
          <option value="issues">Has Issues</option>
          <option value="not_deployed">Not Deployed</option>
        </select>
        <label className="inline-flex cursor-pointer items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
          <input type="checkbox" checked={showDriftOnly} onChange={e => setShowDriftOnly(e.target.checked)}
            className="rounded border-gray-300 dark:border-gray-600" />
          Version drift only
        </label>
      </div>

      {/* Addon rows */}
      <div className="space-y-3">
        {filteredAddons.length === 0 ? (
          <div className="rounded-lg border border-gray-200 bg-white p-8 text-center text-sm text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400">
            No addons match the current filters.
          </div>
        ) : (
          filteredAddons.map(row => (
            <AddonRow key={row.addon_name} row={row} clusters={data.clusters} />
          ))
        )}
      </div>

      {/* Legend */}
      <div className="flex flex-wrap items-center gap-x-5 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
        <span className="font-medium">Legend:</span>
        <span className="flex items-center gap-1.5"><span className="inline-block h-2.5 w-2.5 rounded-full bg-green-500" /> Healthy</span>
        <span className="flex items-center gap-1.5"><span className="inline-block h-2.5 w-2.5 rounded-full bg-red-500" /> Degraded</span>
        <span className="flex items-center gap-1.5"><span className="inline-block h-2.5 w-2.5 rounded-full bg-amber-500" /> Not Deployed</span>
        <span className="flex items-center gap-1.5"><span className="inline-block h-2.5 w-2.5 rounded-full bg-gray-400" /> Unknown</span>
        <span className="flex items-center gap-1.5">
          <span className="rounded border border-amber-300 bg-amber-50 px-1 text-[10px] dark:border-amber-700 dark:bg-amber-900/20">amber border</span>
          = version drift
        </span>
      </div>
    </div>
  )
}
