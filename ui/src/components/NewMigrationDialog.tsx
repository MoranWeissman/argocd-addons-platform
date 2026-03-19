import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { api } from '@/services/api'
import type { Migration } from '@/services/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

interface NewMigrationDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onStarted: (migration: Migration) => void
}

export function NewMigrationDialog({ open, onOpenChange, onStarted }: NewMigrationDialogProps) {
  const [addonName, setAddonName] = useState('')
  const [clusterName, setClusterName] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleStart = async () => {
    if (!addonName.trim() || !clusterName.trim()) return
    setLoading(true)
    setError(null)
    try {
      const migration = await api.startMigration({
        addon_name: addonName.trim(),
        cluster_name: clusterName.trim(),
      })
      setAddonName('')
      setClusterName('')
      onStarted(migration)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to start migration')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New Migration</DialogTitle>
          <DialogDescription>
            Start a new addon migration from the old platform to the new one.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {error && (
            <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/30 dark:text-red-400">
              {error}
            </div>
          )}

          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              Addon Name
            </label>
            <Input
              value={addonName}
              onChange={(e) => setAddonName(e.target.value)}
              placeholder="e.g. cert-manager"
              disabled={loading}
            />
          </div>

          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              Cluster Name
            </label>
            <Input
              value={clusterName}
              onChange={(e) => setClusterName(e.target.value)}
              placeholder="e.g. prod-eu-west-1"
              disabled={loading}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>
            Cancel
          </Button>
          <Button onClick={handleStart} disabled={!addonName.trim() || !clusterName.trim() || loading}>
            {loading && <Loader2 className="h-4 w-4 animate-spin" />}
            Start Migration
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
