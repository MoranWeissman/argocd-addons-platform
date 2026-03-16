import { useState } from 'react'
import {
  BookOpen,
  Package,
  FileText,
  Bug,
  Info,
  Monitor,
} from 'lucide-react'
import type { ReactNode } from 'react'

interface DocPage {
  id: string
  title: string
  icon: typeof BookOpen
  content: ReactNode
}

/* ------------------------------------------------------------------ */
/*  Doc page content components                                        */
/* ------------------------------------------------------------------ */

function OverviewContent() {
  return (
    <div className="doc-content">
      <h1 className="mb-6 text-3xl font-bold text-gray-900 dark:text-white">
        Overview
      </h1>

      <p className="mb-4 text-gray-700 dark:text-gray-300">
        The ArgoCD Addons Platform (AAP) is a web interface for monitoring and
        managing Kubernetes add-ons across your cluster fleet. It provides a
        centralized view of addon health, version status, and configuration
        differences between clusters.
      </p>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        What It Shows
      </h2>
      <ul className="mb-4 list-disc space-y-2 pl-6 text-gray-700 dark:text-gray-300">
        <li>
          <strong>Cluster health</strong> -- connection status and addon
          deployment state for every cluster in your fleet.
        </li>
        <li>
          <strong>Addon catalog</strong> -- all available addons, their versions,
          and how many clusters have them deployed.
        </li>
        <li>
          <strong>Version matrix</strong> -- a cross-cluster view showing which
          version of each addon is running where, highlighting version drift.
        </li>
        <li>
          <strong>Configuration differences</strong> -- per-cluster overrides
          compared against global defaults.
        </li>
      </ul>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Who It's For
      </h2>
      <p className="mb-4 text-gray-700 dark:text-gray-300">
        Platform engineers and SREs who manage Kubernetes add-ons at scale.
        The UI gives you a read-only operational view -- all changes are made
        through Git (GitOps workflow) and applied by ArgoCD.
      </p>
    </div>
  )
}

function FeaturesContent() {
  return (
    <div className="doc-content">
      <h1 className="mb-6 text-3xl font-bold text-gray-900 dark:text-white">
        Features
      </h1>

      <p className="mb-4 text-gray-700 dark:text-gray-300">
        A quick tour of every page in the platform.
      </p>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Dashboard
      </h2>
      <p className="mb-4 text-gray-700 dark:text-gray-300">
        The landing page shows a high-level summary: total clusters, addon
        health distribution, and recent issues. Use it as a starting point to
        spot problems at a glance.
      </p>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Clusters
      </h2>
      <p className="mb-4 text-gray-700 dark:text-gray-300">
        Lists every cluster with its connection status and Kubernetes version.
        Click a cluster to see a detailed comparison of Git-configured addons
        versus live ArgoCD state. The detail page also includes a Config
        Overrides tab showing per-cluster value differences.
      </p>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Addon Catalog
      </h2>
      <p className="mb-4 text-gray-700 dark:text-gray-300">
        A searchable, filterable catalog of all addons defined in your
        repository. Each card shows deployment count, health breakdown, and
        version. Click an addon to see per-cluster deployment details.
      </p>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Version Matrix
      </h2>
      <p className="mb-4 text-gray-700 dark:text-gray-300">
        A matrix view with addons as rows and clusters as columns. Each cell
        shows the deployed version and a health indicator dot. Cells with
        version drift (different from the catalog version) are highlighted in
        amber. By default, addons with no deployments are hidden -- use the
        "Show all addons" toggle to reveal them.
      </p>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Config Diff
      </h2>
      <p className="mb-4 text-gray-700 dark:text-gray-300">
        Available on each cluster detail page under the "Config Overrides" tab.
        Shows a side-by-side comparison of global default values versus
        cluster-specific overrides for each addon.
      </p>
    </div>
  )
}

function ManagingAddonsContent() {
  return (
    <div className="doc-content">
      <h1 className="mb-6 text-3xl font-bold text-gray-900 dark:text-white">
        Managing Addons
      </h1>

      <h2 className="mb-3 mt-6 text-2xl font-semibold text-gray-900 dark:text-white">
        Reading Addon Status
      </h2>
      <p className="mb-4 text-gray-700 dark:text-gray-300">
        Each addon can have one of these statuses:
      </p>
      <ul className="mb-4 list-disc space-y-2 pl-6 text-gray-700 dark:text-gray-300">
        <li>
          <strong>Healthy</strong> -- The addon is deployed and ArgoCD reports
          it as healthy and synced.
        </li>
        <li>
          <strong>Degraded</strong> -- The addon is deployed but ArgoCD reports
          health issues (pods crashing, resources unavailable).
        </li>
        <li>
          <strong>Not Deployed</strong> -- The addon is enabled in Git but no
          matching ArgoCD Application exists. This may indicate a sync issue.
        </li>
        <li>
          <strong>Unmanaged</strong> -- An ArgoCD Application exists but the
          addon is not defined in the Git repository.
        </li>
        <li>
          <strong>Not Enabled</strong> -- The addon exists in the catalog but is
          not enabled for this cluster.
        </li>
      </ul>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Understanding Health
      </h2>
      <p className="mb-4 text-gray-700 dark:text-gray-300">
        Health information comes from ArgoCD, which checks the actual Kubernetes
        resources. A green dot means all resources are running. A red dot means
        at least one resource is unhealthy. The Version Matrix page is the
        fastest way to see health across all clusters at once.
      </p>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Using the Version Matrix
      </h2>
      <p className="mb-4 text-gray-700 dark:text-gray-300">
        The matrix highlights version drift in amber. Filter by health status
        or drift to focus on clusters that need attention. The catalog version
        shown under each addon name is the default version from the addon
        catalog -- clusters running a different version will be flagged.
      </p>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Version Overrides
      </h2>
      <p className="mb-2 text-gray-700 dark:text-gray-300">
        Override the default chart version for a specific cluster by adding a
        version label in{' '}
        <code className="rounded bg-gray-100 px-1.5 py-0.5 text-sm dark:bg-gray-800">
          cluster-addons.yaml
        </code>:
      </p>
      <pre className="mb-6 overflow-x-auto rounded-lg bg-gray-900 p-4 text-sm text-gray-100">
        <code>{`clusters:
  - name: my-cluster
    labels:
      datadog: enabled
      datadog-version: "3.70.7"   # Override default version`}</code>
      </pre>
    </div>
  )
}

function ValuesGuideContent() {
  return (
    <div className="doc-content">
      <h1 className="mb-6 text-3xl font-bold text-gray-900 dark:text-white">
        Values Guide
      </h1>

      <h2 className="mb-3 mt-6 text-2xl font-semibold text-gray-900 dark:text-white">
        Understanding Config Overrides
      </h2>
      <p className="mb-4 text-gray-700 dark:text-gray-300">
        Addon values follow a layered architecture. Each layer can override
        the one above it:
      </p>
      <pre className="mb-6 overflow-x-auto rounded-lg bg-gray-900 p-4 text-sm text-gray-100">
        <code>{`1. Helm Chart Defaults        (from chart repository)
       |  overridden by
2. global-values.yaml          (shared defaults for all clusters)
       |  overridden by
3. Cluster-specific values     (per-cluster overrides file)
       |  overridden by
4. ApplicationSet Parameters   (highest precedence)`}</code>
      </pre>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        What the Diff Viewer Shows
      </h2>
      <p className="mb-4 text-gray-700 dark:text-gray-300">
        The Config Overrides tab on each cluster detail page shows a
        side-by-side comparison for every addon. On the left you see the
        global default values, on the right the cluster-specific overrides.
        Addons marked "Uses defaults" have no per-cluster customization.
        Addons marked "Custom overrides" have cluster-specific values that
        differ from or extend the global configuration.
      </p>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Per-Cluster Values
      </h2>
      <p className="mb-2 text-gray-700 dark:text-gray-300">
        Each cluster has a single values file at{' '}
        <code className="rounded bg-gray-100 px-1.5 py-0.5 text-sm dark:bg-gray-800">
          configuration/addons-clusters-values/&lt;cluster&gt;.yaml
        </code>
        . Each root key corresponds to an addon:
      </p>
      <pre className="mb-6 overflow-x-auto rounded-lg bg-gray-900 p-4 text-sm text-gray-100">
        <code>{`clusterGlobalValues:
  env: &env dev
  clusterName: &clusterName my-cluster

datadog:
  clusterAgent:
    resources:
      limits:
        memory: 1Gi

external-secrets:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::12345:role/ESO-Role"`}</code>
      </pre>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        YAML Anchors
      </h2>
      <p className="mb-2 text-gray-700 dark:text-gray-300">
        Use YAML anchors in{' '}
        <code className="rounded bg-gray-100 px-1.5 py-0.5 text-sm dark:bg-gray-800">
          clusterGlobalValues
        </code>{' '}
        to avoid duplication across addon sections:
      </p>
      <pre className="mb-6 overflow-x-auto rounded-lg bg-gray-900 p-4 text-sm text-gray-100">
        <code>{`clusterGlobalValues:
  clusterName: &clusterName my-cluster
  region: &region eu-west-1

datadog:
  clusterName: *clusterName     # Resolves to: my-cluster

anodot:
  config:
    clusterRegion: *region      # Resolves to: eu-west-1`}</code>
      </pre>
    </div>
  )
}

function TroubleshootingContent() {
  return (
    <div className="doc-content">
      <h1 className="mb-6 text-3xl font-bold text-gray-900 dark:text-white">
        Troubleshooting
      </h1>

      <h2 className="mb-3 mt-6 text-2xl font-semibold text-gray-900 dark:text-white">
        Dashboard Shows No Data
      </h2>
      <ul className="mb-4 list-disc space-y-2 pl-6 text-gray-700 dark:text-gray-300">
        <li>
          Check the Settings page to verify your connection is active and the
          API Health indicator is green.
        </li>
        <li>
          Make sure the Git repository and ArgoCD server URLs are correct in
          your active connection.
        </li>
        <li>
          If the API Health shows "Unreachable", the backend server may not be
          running.
        </li>
      </ul>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Addon Shows "Not Deployed"
      </h2>
      <ul className="mb-4 list-disc space-y-2 pl-6 text-gray-700 dark:text-gray-300">
        <li>
          The addon is enabled in Git but no ArgoCD Application was found.
          Check that ArgoCD has synced the ApplicationSet.
        </li>
        <li>
          Verify the cluster has the correct label (e.g.,{' '}
          <code className="rounded bg-gray-100 px-1.5 py-0.5 text-sm dark:bg-gray-800">
            datadog: enabled
          </code>
          ) in{' '}
          <code className="rounded bg-gray-100 px-1.5 py-0.5 text-sm dark:bg-gray-800">
            cluster-addons.yaml
          </code>.
        </li>
        <li>
          Ensure the cluster values file exists at{' '}
          <code className="rounded bg-gray-100 px-1.5 py-0.5 text-sm dark:bg-gray-800">
            configuration/addons-clusters-values/&lt;cluster&gt;.yaml
          </code>.
        </li>
      </ul>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Version Drift Detected
      </h2>
      <ul className="mb-4 list-disc space-y-2 pl-6 text-gray-700 dark:text-gray-300">
        <li>
          The Version Matrix highlights cells in amber when the deployed version
          differs from the catalog version.
        </li>
        <li>
          Check whether the cluster has a version override label (e.g.,{' '}
          <code className="rounded bg-gray-100 px-1.5 py-0.5 text-sm dark:bg-gray-800">
            datadog-version: "3.70.7"
          </code>
          ). Intentional overrides are expected drift.
        </li>
        <li>
          If the drift is unintentional, check the ArgoCD Application sync
          status for the affected cluster.
        </li>
      </ul>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Cluster Shows "Failed" Connection
      </h2>
      <ul className="mb-4 list-disc space-y-2 pl-6 text-gray-700 dark:text-gray-300">
        <li>
          This status comes from ArgoCD. The cluster may be unreachable due to
          network issues or expired credentials.
        </li>
        <li>
          Check the ArgoCD UI or CLI for more details on the connection failure.
        </li>
      </ul>

      <h2 className="mb-3 mt-8 text-2xl font-semibold text-gray-900 dark:text-white">
        Config Overrides Not Loading
      </h2>
      <ul className="mb-4 list-disc space-y-2 pl-6 text-gray-700 dark:text-gray-300">
        <li>
          The Config Overrides tab loads data separately. If it shows an error,
          the backend may not have access to the Git repository.
        </li>
        <li>
          Check that the Git token in your connection settings has read access
          to the configuration directory.
        </li>
      </ul>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Doc pages registry                                                 */
/* ------------------------------------------------------------------ */

const docPages: DocPage[] = [
  { id: 'overview', title: 'Overview', icon: Info, content: <OverviewContent /> },
  { id: 'features', title: 'Features', icon: Monitor, content: <FeaturesContent /> },
  { id: 'managing-addons', title: 'Managing Addons', icon: Package, content: <ManagingAddonsContent /> },
  { id: 'values-guide', title: 'Values Guide', icon: FileText, content: <ValuesGuideContent /> },
  { id: 'troubleshooting', title: 'Troubleshooting', icon: Bug, content: <TroubleshootingContent /> },
]

/* ------------------------------------------------------------------ */
/*  Main Docs view                                                     */
/* ------------------------------------------------------------------ */

export function Docs() {
  const [activePageId, setActivePageId] = useState(docPages[0].id)
  const activePage = docPages.find((p) => p.id === activePageId) ?? docPages[0]

  return (
    <div>
      <div className="mb-6 flex items-center gap-3">
        <BookOpen className="h-7 w-7 text-cyan-600 dark:text-cyan-400" />
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
          Documentation
        </h1>
      </div>

      <div className="flex gap-6">
        {/* Left sidebar navigation */}
        <nav className="w-56 shrink-0">
          <ul className="space-y-1">
            {docPages.map((page) => {
              const isActive = page.id === activePageId
              return (
                <li key={page.id}>
                  <button
                    onClick={() => setActivePageId(page.id)}
                    className={`flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors ${
                      isActive
                        ? 'bg-cyan-50 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400'
                        : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200'
                    }`}
                    data-testid={`doc-nav-${page.id}`}
                  >
                    <page.icon className="h-4 w-4 shrink-0" />
                    <span>{page.title}</span>
                  </button>
                </li>
              )
            })}
          </ul>
        </nav>

        {/* Right content area */}
        <div className="min-w-0 flex-1 rounded-xl border border-gray-200 bg-white p-8 shadow-sm dark:border-gray-700 dark:bg-gray-900">
          <div className="max-w-none">{activePage.content}</div>
        </div>
      </div>
    </div>
  )
}
