# Troubleshooting

## Dashboard Shows No Data

- Check the Settings page to verify your connection is active and the API Health indicator is green.
- Make sure the Git repository and ArgoCD server URLs are correct in your active connection.
- If the API Health shows "Unreachable", the backend server may not be running.

## Addon Shows "Not Deployed"

- The addon is enabled in Git but no ArgoCD Application was found. Check that ArgoCD has synced the ApplicationSet.
- Verify the cluster has the correct label (e.g., `datadog: enabled`) in `cluster-addons.yaml`.
- Ensure the cluster values file exists at `configuration/addons-clusters-values/<cluster>.yaml`.

## Version Drift Detected

- The Version Matrix highlights cells in amber when the deployed version differs from the catalog version.
- Check whether the cluster has a version override label (e.g., `datadog-version: "3.70.7"`). Intentional overrides are expected drift.
- If the drift is unintentional, check the ArgoCD Application sync status for the affected cluster.

## Cluster Shows "Failed" Connection

- This status comes from ArgoCD. The cluster may be unreachable due to network issues or expired credentials.
- Check the ArgoCD UI or CLI for more details on the connection failure.

## Config Overrides Not Loading

- The Config Overrides tab loads data separately. If it shows an error, the backend may not have access to the Git repository.
- Check that the Git token in your connection settings has read access to the configuration directory.

## Migration Errors

- **"addon not found in catalog"** -- The addon must exist in `addons-catalog.yaml` with `inMigration: true`.
- **"already migrated"** -- The addon is already enabled in the NEW repo and running in NEW ArgoCD. No action needed.
- **PR merge failed** -- Check the PR URL in the logs. The PR may have merge conflicts or branch protection rules blocking it.
- **ArgoCD app not found after 3 attempts** -- The ApplicationSet may take time to generate the app. Wait and retry.

## AI Assistant Not Working

- Check Settings > AI Configuration. An AI provider must be configured and tested.
- If using Claude/OpenAI, verify the API key is valid and has not expired.
- Rate limit errors (429) mean you're sending too many requests. Wait a minute and try again.
