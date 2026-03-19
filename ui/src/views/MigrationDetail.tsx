import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Pause, XCircle, Terminal, ChevronUp, ChevronDown, CheckCircle } from 'lucide-react'
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
  const [logsExpanded, setLogsExpanded] = useState(false)
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

  // Initial fetch
  useEffect(() => {
    void fetchMigration()
  }, [fetchMigration])

  // Polling
  useEffect(() => {
    if (!migration) return

    // Clear any existing interval
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }

    const status = migration.status
    if (status === 'completed' || status === 'failed' || status === 'cancelled') {
      return
    }

    const pollMs = status === 'running' ? 3000 : 10000
    intervalRef.current = setInterval(() => {
      void fetchMigration()
    }, pollMs)

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
    }
  }, [migration, fetchMigration])

  // Auto-scroll logs to bottom
  useEffect(() => {
    if (logEndRef.current && logsExpanded) {
      logEndRef.current.scrollTop = logEndRef.current.scrollHeight
    }
  }, [migration?.logs?.length, logsExpanded])

  const handleContinue = async () => {
    if (!id) return
    try {
      await api.continueMigration(id)
      void fetchMigration()
    } catch {
      // handled by next poll
    }
  }

  const handleRetry = async () => {
    if (!id) return
    try {
      await api.retryMigration(id)
      void fetchMigration()
    } catch {
      // handled by next poll
    }
  }

  const handlePause = async () => {
    if (!id) return
    try {
      await api.pauseMigration(id)
      void fetchMigration()
    } catch {
      // handled by next poll
    }
  }

  const handleCancel = async () => {
    if (!id) return
    try {
      await api.cancelMigration(id)
      void fetchMigration()
    } catch {
      // handled by next poll
    }
  }

  if (loading) {
    return <LoadingState message="Loading migration details..." />
  }

  if (error) {
    return <ErrorState message={error} onRetry={fetchMigration} />
  }

  if (!migration) return null

  const isTerminal = ['completed', 'failed', 'cancelled'].includes(migration.status)
  const isRunning = migration.status === 'running'
  const isGated = migration.status === 'gated'

  return (
    <div className="space-y-6">
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
            {migration.mode && (
              <Badge variant={migration.mode === 'yolo' ? 'destructive' : 'secondary'}>
                {migration.mode === 'yolo' ? 'YOLO' : 'Gates'}
              </Badge>
            )}
          </div>
          <p className="mt-1 text-gray-500 dark:text-gray-400">
            Cluster: {migration.cluster_name}
          </p>
          <div className="mt-2">
            <StatusBadge status={migration.status} size="md" />
          </div>
        </div>

        {/* Control buttons */}
        <div className="flex items-center gap-2">
          {isRunning && (
            <Button variant="outline" onClick={handlePause} className="border-amber-300 text-amber-600 hover:bg-amber-50 dark:border-amber-700 dark:text-amber-400 dark:hover:bg-amber-900/30">
              <Pause className="h-4 w-4" />
              Pause
            </Button>
          )}
          {!isTerminal && (
            <Button variant="outline" onClick={handleCancel} className="border-red-300 text-red-600 hover:bg-red-50 dark:border-red-700 dark:text-red-400 dark:hover:bg-red-900/30">
              <XCircle className="h-4 w-4" />
              Cancel
            </Button>
          )}
        </div>
      </div>

      {/* Error banner */}
      {migration.error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/30 dark:text-red-400">
          {migration.error}
        </div>
      )}

      {/* Stepper */}
      {migration.steps && migration.steps.length > 0 && (
        <MigrationStepper
          steps={migration.steps}
          currentStep={migration.current_step}
          onContinue={handleContinue}
          onRetry={handleRetry}
        />
      )}

      {/* Action buttons for gated / waiting statuses */}
      <div className="flex items-center gap-2">
        {isGated && (
          <Button onClick={handleContinue}>
            <CheckCircle className="h-4 w-4" />
            Approve &amp; Continue
          </Button>
        )}
        {migration.status === 'waiting' && (
          <Button onClick={handleContinue}>
            Continue (PR Merged)
          </Button>
        )}
      </div>

      {/* Live Activity Log */}
      <div className="mt-6 rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <button
          type="button"
          onClick={() => setLogsExpanded(!logsExpanded)}
          className="flex w-full items-center justify-between p-4"
        >
          <div className="flex items-center gap-2">
            <Terminal className="h-4 w-4" />
            <span className="text-sm font-semibold">Activity Log</span>
            <Badge variant="secondary">{migration.logs?.length ?? 0}</Badge>
          </div>
          {logsExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
        </button>

        {logsExpanded && (
          <div
            ref={logEndRef}
            className="max-h-64 overflow-auto border-t border-gray-200 p-4 dark:border-gray-700"
          >
            {migration.logs?.map((log, i) => (
              <div key={i} className="flex items-start gap-2 py-1 font-mono text-xs">
                <span className="shrink-0 text-gray-400">
                  {new Date(log.timestamp).toLocaleTimeString()}
                </span>
                <span
                  className={cn(
                    'shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold',
                    log.repo.startsWith('NEW')
                      ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                      : log.repo.startsWith('OLD')
                        ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
                        : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
                  )}
                >
                  {log.repo}
                </span>
                <span className="text-gray-500">[{log.action}]</span>
                <span className="text-gray-700 dark:text-gray-300">{log.detail}</span>
              </div>
            ))}
            {(!migration.logs || migration.logs.length === 0) && (
              <p className="text-xs text-gray-400">No activity yet...</p>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
