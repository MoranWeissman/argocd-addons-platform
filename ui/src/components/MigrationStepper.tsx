import type { MigrationStep } from '@/services/api'
import { MigrationStepCard } from '@/components/MigrationStep'

interface MigrationStepperProps {
  steps: MigrationStep[]
  currentStep: number
  onContinue: () => void
  onRetry: () => void
}

export function MigrationStepper({ steps, currentStep, onContinue, onRetry }: MigrationStepperProps) {
  return (
    <div className="space-y-0">
      {steps.map((step, index) => (
        <MigrationStepCard
          key={step.number}
          step={step}
          isActive={step.number === currentStep}
          isLast={index === steps.length - 1}
          onContinue={onContinue}
          onRetry={onRetry}
        />
      ))}
    </div>
  )
}
