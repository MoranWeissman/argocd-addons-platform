import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Pause, CheckCircle2, Loader2, Clock, XCircle, SkipForward, ExternalLink, CheckCircle } from 'lucide-react'
import { api } from '@/services/api'
import type { Migration, MigrationStep } from '@/services/api'
import { StatusBadge } from '@/components/StatusBadge'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

function StepIcon({ status, number }: { status: MigrationStep['status']; number: number }) {
  const base = 'flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[9px] font-bold'
  switch (status) {
    case 'completed':
      return <div className={cn(base, 'bg-green-500 text-white')}><CheckCircle2 className="h-3 w-3" /></div>
    case 'running':
      return <div className={cn(base, 'bg-blue-500 text-white')}><Loader2 className="h-3 w-3 animate-spin" /></div>
    case 'waiting':
      return <div className={cn(base, 'bg-amber-500 text-white')}><Clock className="h-3 w-3" /></div>
    case 'failed':
      return <div className={cn(base, 'bg-red-500 text-white')}><XCircle className="h-3 w-3" /></div>
    case 'skipped':
      return <div className={cn(base, 'bg-gray-400 text-white')}><SkipForward className="h-2.5 w-2.5" /></div>
    default:
      return <div className={cn(base, 'bg-gray-600 text-gray-300')}>{number}</div>
  }
}

function stepDuration(step: MigrationStep): string | null {
  if (!step.started_at || !step.completed_at) return null
  const ms = new Date(step.completed_at).getTime() - new Date(step.started_at).getTime()
  if (ms < 1000) return '<1s'
  if (ms < 60000) return `${Math.round(ms / 1000)}s`
  return `${Math.round(ms / 60000)}m`
}

export default function MigrationDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [migration, setMigration] = useState<Migration | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedStep, setSelectedStep] = useState<number>(1)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const logEndRef = useRef<HTMLDivElement>(null)

  const fetchMigration = useCallback(async () => {
    if (!id) return
    try {
      setError(null)
      const data = await api.getMigration(id)
      setMigration(data)
      // Auto-select current active step
      if (data.current_step > 0) {
        setSelectedStep(prev => {
          // Only auto-advance if user hasn't manually selected a different step
          // or if the step they selected is now completed
          const prevStep = data.steps?.find(s => s.number === prev)
          if (!prevStep || prevStep.status === 'completed') {
            return data.current_step
          }
          return prev
        })
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load migration')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { void fetchMigration() }, [fetchMigration])

  // Polling
  useEffect(() => {
    if (!migration) return
    if (intervalRef.current) { clearInterval(intervalRef.current); intervalRef.current = null }
    const status = migration.status
    if (['completed', 'failed', 'cancelled'].includes(status)) return
    const pollMs = status === 'running' ? 2000 : 5000
    intervalRef.current = setInterval(() => { void fetchMigration() }, pollMs)
    return () => { if (intervalRef.current) clearInterval(intervalRef.current) }
  }, [migration, fetchMigration])

  // Auto-scroll log
  useEffect(() => {
    if (logEndRef.current) logEndRef.current.scrollTop = logEndRef.current.scrollHeight
  }, [migration?.logs?.length, selectedStep])

  const handleContinue = async () => {
    if (!id) return
    try { await api.continueMigration(id); void fetchMigration() } catch { /* poll */ }
  }
  const handleRetry = async () => {
    if (!id) return
    try { await api.retryMigration(id); void fetchMigration() } catch { /* poll */ }
  }
  const handleMergePR = async (step: number) => {
    if (!id) return
    try { await api.mergeMigrationPR(id, step); void fetchMigration() }
    catch (e: unknown) { alert(e instanceof Error ? e.message : 'Failed to merge PR') }
  }
  const handlePause = async () => {
    if (!id) return
    try { await api.pauseMigration(id); void fetchMigration() } catch { /* poll */ }
  }

  if (loading) return <LoadingState message="Loading migration details..." />
  if (error) return <ErrorState message={error} onRetry={fetchMigration} />
  if (!migration) return null

  const steps = migration.steps ?? []
  const allLogs = migration.logs ?? []
  const stepLogs = allLogs.filter(l => l.step === selectedStep)
  const activeStep = steps.find(s => s.number === selectedStep)
  const isRunning = migration.status === 'running'
  const isGated = migration.status === 'gated'

  return (
    <div className="space-y-3">
      {/* Back + Header */}
      <button onClick={() => navigate('/migration')}
        className="flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200">
        <ArrowLeft className="h-4 w-4" /> Back to Migrations
      </button>

      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">{migration.addon_name}</h1>
            <Badge variant={migration.mode === 'yolo' ? 'destructive' : 'secondary'}>
              {migration.mode === 'yolo' ? 'YOLO' : 'Gates'}
            </Badge>
            <StatusBadge status={migration.status} size="md" />
          </div>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Cluster: {migration.cluster_name} &middot; Step {migration.current_step} of {steps.length}
          </p>
        </div>
        {isRunning && (
          <Button variant="outline" size="sm" onClick={handlePause}
            className="border-amber-300 text-amber-600 hover:bg-amber-50 dark:border-amber-700 dark:text-amber-400">
            <Pause className="h-4 w-4" /> Pause
          </Button>
        )}
      </div>

      {migration.error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/30 dark:text-red-400">
          {migration.error}
        </div>
      )}

      {/* Azure Pipeline-style layout */}
      <div className="flex gap-0 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800" style={{ height: 'calc(100vh - 220px)' }}>

        {/* Left: Step list (like Azure DevOps jobs panel) */}
        <div className="w-72 shrink-0 overflow-y-auto border-r border-gray-200 dark:border-gray-700">
          {steps.map((step) => {
            const isSelected = step.number === selectedStep
            const duration = stepDuration(step)
            const stepLogCount = allLogs.filter(l => l.step === step.number).length

            return (
              <button
                key={step.number}
                onClick={() => setSelectedStep(step.number)}
                className={cn(
                  'flex w-full items-center gap-2.5 border-b border-gray-100 px-3 py-2.5 text-left transition-colors dark:border-gray-800',
                  isSelected
                    ? 'bg-blue-50 dark:bg-blue-900/20'
                    : 'hover:bg-gray-50 dark:hover:bg-gray-800/50',
                )}
              >
                <StepIcon status={step.status} number={step.number} />
                <div className="min-w-0 flex-1">
                  <div className={cn(
                    'truncate text-xs',
                    isSelected ? 'font-semibold text-gray-900 dark:text-gray-100' : 'text-gray-700 dark:text-gray-300',
                    step.status === 'completed' && !isSelected && 'text-gray-500 dark:text-gray-500',
                  )}>
                    {step.title}
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-1.5">
                  {stepLogCount > 0 && (
                    <span className="text-[9px] text-gray-400">{stepLogCount}</span>
                  )}
                  {duration && (
                    <span className="text-[10px] text-gray-400">{duration}</span>
                  )}
                  {step.status === 'running' && (
                    <span className="text-[10px] text-blue-400">...</span>
                  )}
                </div>
              </button>
            )
          })}
        </div>

        {/* Right: Log panel for selected step */}
        <div className="flex flex-1 flex-col overflow-hidden">
          {activeStep && (
            <>
              {/* Step header with description + actions */}
              <div className="shrink-0 border-b border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-700 dark:bg-gray-900/50">
                <div className="flex items-start justify-between">
                  <div>
                    <div className="flex items-center gap-2">
                      <StepIcon status={activeStep.status} number={activeStep.number} />
                      <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100">
                        {activeStep.title}
                      </h3>
                    </div>
                    {activeStep.description && (
                      <p className="mt-1 ml-7 text-xs text-gray-500 dark:text-gray-400">
                        {activeStep.description}
                      </p>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {activeStep.pr_url && (
                      <a href={activeStep.pr_url} target="_blank" rel="noopener noreferrer"
                        className="inline-flex items-center gap-1 rounded-md border border-gray-300 px-2 py-1 text-xs text-blue-600 hover:bg-blue-50 dark:border-gray-600 dark:text-blue-400">
                        <ExternalLink className="h-3 w-3" /> PR {activeStep.pr_status && `(${activeStep.pr_status})`}
                      </a>
                    )}
                  </div>
                </div>

                {/* Inline actions: approve, merge, retry */}
                {(isGated && activeStep.number === migration.current_step && activeStep.status === 'pending') && (
                  <div className="mt-2 ml-7 flex items-center gap-2 rounded-md border border-amber-400 bg-amber-50 px-3 py-2 dark:border-amber-600 dark:bg-amber-900/20">
                    <span className="flex-1 text-xs font-semibold text-amber-700 dark:text-amber-400">
                      Awaiting approval to continue
                    </span>
                    <Button size="sm" onClick={handleContinue} className="h-7 bg-amber-600 hover:bg-amber-700 text-xs">
                      <CheckCircle className="h-3 w-3" /> Approve
                    </Button>
                  </div>
                )}

                {activeStep.status === 'waiting' && (
                  <div className="mt-2 ml-7 flex items-center gap-2">
                    <span className="text-xs text-amber-600 dark:text-amber-400">Waiting for PR merge</span>
                    {activeStep.pr_number && (
                      <Button size="sm" variant="outline" onClick={() => handleMergePR(activeStep.number)}
                        className="h-6 px-2 text-xs bg-green-50 border-green-300 text-green-700 hover:bg-green-100 dark:bg-green-900/20 dark:border-green-700 dark:text-green-400">
                        Merge PR
                      </Button>
                    )}
                    <Button size="sm" variant="outline" onClick={handleContinue} className="h-6 px-2 text-xs">
                      PR Merged &rarr; Continue
                    </Button>
                  </div>
                )}

                {activeStep.status === 'failed' && activeStep.error && (
                  <div className="mt-2 ml-7 flex items-center gap-2">
                    <span className="text-xs text-red-600 dark:text-red-400">{activeStep.error}</span>
                    <Button size="sm" variant="destructive" onClick={handleRetry} className="h-6 px-2 text-xs">
                      Retry
                    </Button>
                  </div>
                )}
              </div>

              {/* Log entries */}
              <div ref={logEndRef} className="flex-1 overflow-y-auto bg-gray-950 p-3 font-mono text-xs">
                {stepLogs.length === 0 ? (
                  <p className="py-8 text-center text-gray-600">
                    {activeStep.status === 'pending' ? 'Step not started yet' : 'Waiting for activity...'}
                  </p>
                ) : (
                  stepLogs.map((log, i) => (
                    <div key={i} className="flex gap-2 py-0.5 leading-5">
                      <span className="shrink-0 select-none text-gray-600">
                        {new Date(log.timestamp).toLocaleTimeString()}
                      </span>
                      <span className={cn(
                        'shrink-0 font-bold uppercase',
                        log.repo === 'SYSTEM' ? 'text-cyan-400'
                          : log.repo.includes('NEW') ? 'text-green-400'
                            : log.repo.includes('OLD') ? 'text-orange-400'
                              : 'text-blue-400'
                      )}>
                        {log.repo}
                      </span>
                      <span className="text-gray-500">{log.action}</span>
                      <span className="text-gray-300">{log.detail}</span>
                    </div>
                  ))
                )}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
