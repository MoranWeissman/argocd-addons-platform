import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Pause, XCircle } from 'lucide-react'
import { api } from '@/services/api'
import type { Migration } from '@/services/api'
import { MigrationStepper } from '@/components/MigrationStepper'
import { StatusBadge } from '@/components/StatusBadge'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { Button } from '@/components/ui/button'

export default function MigrationDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [migration, setMigration] = useState<Migration | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

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
          <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">
            {migration.addon_name}
          </h1>
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
    </div>
  )
}
