# Migration Guide

## Overview

The Migration Wizard moves addons from an OLD ArgoCD instance to the NEW ArgoCD Addons Platform. It uses a 10-step pipeline that creates PRs, verifies ArgoCD state, and ensures zero downtime through resource adoption.

## Prerequisites

1. **Migration Settings** -- Configure the OLD Git repo and OLD ArgoCD connection in Settings > Migration.
2. **Addon catalog** -- The addon must exist in `addons-catalog.yaml` with `inMigration: true`.
3. **Safety flags** -- `preserveResourcesOnDeletion` must be set in the OLD ArgoCD app to prevent resource deletion during handover.

## Migration Modes

- **Gates** -- Pauses after each step for your approval. Recommended for first migrations.
- **YOLO** -- Runs all steps automatically, auto-merges PRs. Use only after you trust the pipeline.

## The 10 Steps

| Step | What It Does |
|------|-------------|
| 1. Verify catalog | Checks addon exists in NEW repo with `inMigration: true` |
| 2. Compare values | Reads values from both repos, logs differences (advisory) |
| 3. Enable in NEW | Creates PR to set `addon: enabled` in NEW repo's `cluster-addons.yaml` |
| 4. Verify app created | Waits for ArgoCD to create the application in NEW instance |
| 5. Disable in OLD | Creates PR to set `addon: disabled` in OLD repo |
| 6. Sync OLD ArgoCD | Triggers sync so OLD ArgoCD processes the removal |
| 7. Verify removal | Confirms app is gone from OLD ArgoCD |
| 8. Refresh NEW | Hard refresh in NEW ArgoCD to adopt orphaned resources |
| 9. Verify healthy | Confirms app is Healthy in NEW ArgoCD (OutOfSync is OK) |
| 10. Finalize | Checks all clusters migrated, then sets `inMigration: false` |

## Batch Migration

When migrating an entire cluster, addons run one at a time in sequence. The batch progress view shows:
- Which addon is currently migrating
- Queue of pending and completed addons
- Overall progress

Only one migration runs at a time to avoid PR conflicts on shared files.

## Rollback

If a migration fails mid-way, click the **Rollback** button. It reverses completed PR steps:
- Step 3 (enabled in NEW) -> creates PR to disable
- Step 5 (disabled in OLD) -> creates PR to re-enable

## Smart Detection

- **Already migrated** -- If the addon is already enabled in NEW repo and running in NEW ArgoCD, the migration completes immediately without creating any PRs.
- **Already set** -- Steps 3 and 5 skip PR creation if the addon label already has the target value.

## Common Issues

See the [Troubleshooting](05-troubleshooting.md) page for migration-specific error messages and fixes.
