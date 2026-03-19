import type { MigrationStep, Migration } from '@/services/api'
import { MigrationStepCard } from '@/components/MigrationStep'
import { CheckCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface MigrationStepperProps {
  steps: MigrationStep[]
  currentStep: number
  migrationStatus: Migration['status']
  onContinue: () => void
  onRetry: () => void
}

export function MigrationStepper({ steps, currentStep, migrationStatus, onContinue, onRetry }: MigrationStepperProps) {
  const isGated = migrationStatus === 'gated'

  return (
    <div className="space-y-0">
      {steps.map((step, index) => {
        const isActive = step.number === currentStep
        const isLast = index === steps.length - 1
        // Show gate between this step and the next when:
        // - migration is gated
        // - this step is completed
        // - next step is pending (the gate is before the next step)
        const showGateAfter = isGated && step.status === 'completed' && step.number === currentStep - 1

        return (
          <div key={step.number}>
            <MigrationStepCard
              step={step}
              isActive={isActive}
              isLast={isLast && !showGateAfter}
              onContinue={onContinue}
              onRetry={onRetry}
            />

            {/* Gate approval button between steps */}
            {showGateAfter && (
              <div className="flex gap-4">
                {/* Align with the connecting line */}
                <div className="flex flex-col items-center">
                  <div className="w-0.5 flex-1 border-l-2 border-dashed border-amber-400" />
                </div>
                <div className="my-2 flex flex-1 items-center gap-3 rounded-lg border-2 border-amber-400 bg-amber-50 px-4 py-3 dark:border-amber-600 dark:bg-amber-900/20">
                  <div className="flex-1">
                    <p className="text-sm font-semibold text-amber-700 dark:text-amber-400">
                      Gate — Awaiting Approval
                    </p>
                    <p className="text-xs text-amber-600 dark:text-amber-500">
                      Step {step.number} completed. Review the results above before continuing.
                    </p>
                  </div>
                  <Button size="sm" onClick={onContinue} className="bg-amber-600 hover:bg-amber-700">
                    <CheckCircle className="h-4 w-4" />
                    Approve
                  </Button>
                </div>
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
