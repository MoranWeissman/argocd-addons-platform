import { useState } from 'react'
import {
  CheckCircle2,
  Loader2,
  Clock,
  XCircle,
  ExternalLink,
  ChevronDown,
  ChevronRight,
  SkipForward,
} from 'lucide-react'
import type { MigrationStep as MigrationStepType } from '@/services/api'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface MigrationStepProps {
  step: MigrationStepType
  isActive: boolean
  isLast: boolean
  onContinue?: () => void
  onRetry?: () => void
}

function StepIcon({ status, number }: { status: MigrationStepType['status']; number: number }) {
  const base = 'flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm font-bold'

  switch (status) {
    case 'completed':
      return (
        <div className={cn(base, 'bg-green-500 text-white')}>
          <CheckCircle2 className="h-5 w-5" />
        </div>
      )
    case 'running':
      return (
        <div className={cn(base, 'bg-blue-500 text-white')}>
          <Loader2 className="h-5 w-5 animate-spin" />
        </div>
      )
    case 'waiting':
      return (
        <div className={cn(base, 'bg-amber-500 text-white')}>
          <Clock className="h-5 w-5" />
        </div>
      )
    case 'failed':
      return (
        <div className={cn(base, 'bg-red-500 text-white')}>
          <XCircle className="h-5 w-5" />
        </div>
      )
    case 'skipped':
      return (
        <div className={cn(base, 'bg-gray-400 text-white')}>
          <SkipForward className="h-4 w-4" />
        </div>
      )
    default:
      return (
        <div className={cn(base, 'bg-gray-300 text-gray-600 dark:bg-gray-600 dark:text-gray-300')}>
          {number}
        </div>
      )
  }
}

export function MigrationStepCard({ step, isActive, isLast, onContinue, onRetry }: MigrationStepProps) {
  const [expanded, setExpanded] = useState(isActive)

  const isCompleted = step.status === 'completed'
  const isFailed = step.status === 'failed'
  const isWaiting = step.status === 'waiting'
  const hasDetails = step.message || step.pr_url || step.error

  return (
    <div className="flex gap-4">
      {/* Left: icon + connecting line */}
      <div className="flex flex-col items-center">
        <StepIcon status={step.status} number={step.number} />
        {!isLast && (
          <div
            className={cn(
              'mt-1 w-0.5 flex-1',
              isCompleted ? 'bg-green-400' : 'border-l-2 border-dashed border-gray-300 dark:border-gray-600'
            )}
          />
        )}
      </div>

      {/* Right: content */}
      <div
        className={cn(
          'mb-6 flex-1 rounded-lg border p-4',
          isActive && !isFailed && 'border-blue-300 bg-blue-50/50 dark:border-blue-700 dark:bg-blue-900/20',
          isFailed && 'border-red-300 bg-red-50/50 dark:border-red-700 dark:bg-red-900/20',
          !isActive && !isFailed && 'border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800',
          isCompleted && 'opacity-60'
        )}
      >
        <div className="flex items-start justify-between">
          <div>
            <h4 className="font-semibold text-gray-900 dark:text-gray-100">
              {step.title}
            </h4>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              {step.description}
            </p>
          </div>
          {isCompleted && hasDetails && (
            <button
              onClick={() => setExpanded((e) => !e)}
              className="ml-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
            >
              {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
            </button>
          )}
        </div>

        {(isActive || isFailed || expanded) && (
          <div className="mt-3 space-y-2">
            {/* AI message */}
            {step.message && (
              <div className="rounded-md bg-gray-100 p-3 text-sm text-gray-700 dark:bg-gray-700 dark:text-gray-300">
                {step.message}
              </div>
            )}

            {/* PR link */}
            {step.pr_url && (
              <a
                href={step.pr_url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 text-sm font-medium text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                View Pull Request
                {step.pr_status && <span className="text-gray-500">({step.pr_status})</span>}
              </a>
            )}

            {/* Waiting banner */}
            {isWaiting && (
              <div className="flex items-center gap-2 rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">
                <Clock className="h-4 w-4" />
                Waiting for PR merge...
              </div>
            )}

            {/* Continue button */}
            {isWaiting && onContinue && (
              <Button size="sm" onClick={onContinue}>
                Continue
              </Button>
            )}

            {/* Error message */}
            {step.error && (
              <div className="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-900/30 dark:text-red-400">
                {step.error}
              </div>
            )}

            {/* Retry button */}
            {isFailed && onRetry && (
              <Button size="sm" variant="destructive" onClick={onRetry}>
                Retry
              </Button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
