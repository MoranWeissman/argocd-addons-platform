import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Pause, Terminal } from 'lucide-react'
import { api } from '@/services/api'
import type { Migration } from '@/services/api'
import { MigrationStepper } from '@/components/MigrationStepper'
import { StatusBadge } from '@/components/StatusBadge'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

export default function MigrationDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [migration, setMigration] = useState<Migration | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const logEndRef = useRef<HTMLDivElement>(null)

  const fetchMigration = useCallback(async () => {
    if (!id) return
    try {
      setError(null)
      const data = await api.getMigration(id)
      setMigration(data)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load migration')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    void fetchMigration()
  }, [fetchMigration])

  // Polling
  useEffect(() => {
    if (!migration) return
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }
    const status = migration.status
    if (['completed', 'failed', 'cancelled'].includes(status)) return

    const pollMs = status === 'running' ? 2000 : 5000
    intervalRef.current = setInterval(() => { void fetchMigration() }, pollMs)
    return () => { if (intervalRef.current) clearInterval(intervalRef.current) }
  }, [migration, fetchMigration])

  // Auto-scroll log
  useEffect(() => {
    if (logEndRef.current) {
      logEndRef.current.scrollTop = logEndRef.current.scrollHeight
    }
  }, [migration?.logs?.length])

  const handleContinue = async () => {
    if (!id) return
    try { await api.continueMigration(id); void fetchMigration() } catch { /* next poll */ }
  }
  const handleRetry = async () => {
    if (!id) return
    try { await api.retryMigration(id); void fetchMigration() } catch { /* next poll */ }
  }
  const handleMergePR = async (step: number) => {
    if (!id) return
    try {
      await api.mergeMigrationPR(id, step)
      void fetchMigration()
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : 'Failed to merge PR')
    }
  }
  const handlePause = async () => {
    if (!id) return
    try { await api.pauseMigration(id); void fetchMigration() } catch { /* next poll */ }
  }
  if (loading) return <LoadingState message="Loading migration details..." />
  if (error) return <ErrorState message={error} onRetry={fetchMigration} />
  if (!migration) return null

  const isRunning = migration.status === 'running'

  // Filter logs for selected step (or show all)
  const logs = migration.logs ?? []

  return (
    <div className="space-y-4">
      {/* Back button */}
      <button
        onClick={() => navigate('/migration')}
        className="flex items-center gap-1.5 text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to Migrations
      </button>

      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">
              {migration.addon_name}
            </h1>
            <Badge variant={migration.mode === 'yolo' ? 'destructive' : 'secondary'}>
              {migration.mode === 'yolo' ? 'YOLO' : 'Gates'}
            </Badge>
            <StatusBadge status={migration.status} size="md" />
          </div>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Cluster: {migration.cluster_name} &middot; Step {migration.current_step} of {migration.steps?.length ?? 10}
          </p>
        </div>

        <div className="flex items-center gap-2">
          {isRunning && (
            <Button variant="outline" size="sm" onClick={handlePause} className="border-amber-300 text-amber-600 hover:bg-amber-50 dark:border-amber-700 dark:text-amber-400">
              <Pause className="h-4 w-4" /> Pause
            </Button>
          )}
        </div>
      </div>

      {/* Error banner */}
      {migration.error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/30 dark:text-red-400">
          {migration.error}
        </div>
      )}

      {/* Pipeline layout: narrow stages on left, wide logs on right */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-7">
        {/* Left: Pipeline stages (narrow) */}
        <div className="lg:col-span-3">
          <div className="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800">
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
              Pipeline
            </h3>
            {migration.steps && migration.steps.length > 0 && (
              <MigrationStepper
                steps={migration.steps}
                currentStep={migration.current_step}
                migrationStatus={migration.status}
                migrationId={migration.id}
                onContinue={handleContinue}
                onRetry={handleRetry}
                onMergePR={handleMergePR}
              />
            )}
          </div>
        </div>

        {/* Right: Activity Log (wide) */}
        <div className="lg:col-span-4">
          <div className="sticky top-4 rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
            <div className="flex items-center gap-2 border-b border-gray-200 p-4 dark:border-gray-700">
              <Terminal className="h-4 w-4 text-gray-500" />
              <span className="text-sm font-semibold text-gray-700 dark:text-gray-300">Activity Log</span>
              <Badge variant="secondary" className="ml-auto">{logs.length}</Badge>
            </div>
            <div
              ref={logEndRef}
              className="h-[calc(100vh-280px)] overflow-auto p-3"
            >
              {logs.length === 0 ? (
                <p className="py-8 text-center text-xs text-gray-400">Waiting for activity...</p>
              ) : (
                logs.map((log, i) => (
                  <div key={i} className="flex items-start gap-1.5 border-b border-gray-100 py-1.5 dark:border-gray-800">
                    <span className="shrink-0 pt-0.5 font-mono text-[10px] text-gray-400">
                      {new Date(log.timestamp).toLocaleTimeString()}
                    </span>
                    <div className="min-w-0">
                      <div className="flex items-center gap-1.5">
                        <span
                          className={cn(
                            'inline-block shrink-0 rounded px-1 py-0.5 text-[9px] font-bold uppercase',
                            log.repo.includes('NEW')
                              ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400'
                              : log.repo.includes('OLD')
                                ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-400'
                                : 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400'
                          )}
                        >
                          {log.repo}
                        </span>
                        <span className="text-[10px] text-gray-500">{log.action}</span>
                      </div>
                      <p className="text-xs text-gray-700 dark:text-gray-300">{log.detail}</p>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
