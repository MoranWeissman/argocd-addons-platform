import { useState, useEffect, useCallback, useMemo } from 'react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from 'recharts';
import {
  Activity,
  Clock,
  Server,
  CheckCircle,
  AlertTriangle,
  RefreshCw,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';
import { api } from '@/services/api';
import type {
  ObservabilityOverviewResponse,
  AddonHealthDetail,
  SyncActivityEntry,
} from '@/services/models';
import { LoadingState } from '@/components/LoadingState';
import { ErrorState } from '@/components/ErrorState';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffSecs = Math.max(0, Math.floor((now - then) / 1000));

  if (diffSecs < 60) return `${diffSecs}s ago`;
  const mins = Math.floor(diffSecs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function healthColor(health: string): string {
  const h = health.toLowerCase();
  if (h === 'healthy') return 'text-green-500';
  if (h === 'degraded') return 'text-red-500';
  if (h === 'progressing') return 'text-blue-500';
  return 'text-gray-400';
}

function healthBg(health: string): string {
  const h = health.toLowerCase();
  if (h === 'healthy') return 'bg-green-500';
  if (h === 'degraded') return 'bg-red-500';
  if (h === 'progressing') return 'bg-blue-500';
  return 'bg-gray-400';
}

function statusIcon(status: string) {
  const s = status.toLowerCase();
  if (s === 'succeeded' || s === 'synced' || s === 'healthy')
    return <CheckCircle className="h-4 w-4 text-green-500" />;
  if (s === 'failed' || s === 'degraded')
    return <AlertTriangle className="h-4 w-4 text-red-500" />;
  return <RefreshCw className="h-4 w-4 text-blue-400" />;
}

type SortMode = 'issues' | 'alpha' | 'deployed';

// ---------------------------------------------------------------------------
// Section 1: Control Plane Health
// ---------------------------------------------------------------------------

function ControlPlaneSection({
  data,
}: {
  data: ObservabilityOverviewResponse['control_plane'];
}) {
  const healthData = useMemo(
    () =>
      Object.entries(data.health_summary).map(([name, value]) => ({
        name,
        value,
      })),
    [data.health_summary],
  );

  const COLORS: Record<string, string> = {
    Healthy: '#22c55e',
    Degraded: '#ef4444',
    Progressing: '#3b82f6',
    Missing: '#f59e0b',
    Unknown: '#9ca3af',
  };

  const total = healthData.reduce((sum, d) => sum + d.value, 0);

  return (
    <section className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-900">
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
          Control Plane
        </h2>
        <span className="rounded-full bg-cyan-100 px-2.5 py-0.5 text-xs font-medium text-cyan-800 dark:bg-cyan-900/40 dark:text-cyan-300">
          ArgoCD {data.argocd_version}
        </span>
        <span className="rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-400">
          Helm {data.helm_version}
        </span>
        <span className="rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-400">
          kubectl {data.kubectl_version}
        </span>
      </div>

      <div className="mb-5 grid grid-cols-3 gap-4">
        <StatBlock label="Total Apps" value={data.total_apps} />
        <StatBlock label="Total Clusters" value={data.total_clusters} />
        <StatBlock
          label="Connected"
          value={data.connected_clusters}
          sub={`/ ${data.total_clusters}`}
        />
      </div>

      {/* Health bar */}
      <div>
        <p className="mb-2 text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
          Health Summary
        </p>
        <div className="flex h-4 overflow-hidden rounded-full">
          {healthData.map((d) => (
            <div
              key={d.name}
              style={{
                width: total > 0 ? `${(d.value / total) * 100}%` : '0%',
                backgroundColor: COLORS[d.name] ?? '#9ca3af',
              }}
              title={`${d.name}: ${d.value}`}
            />
          ))}
        </div>
        <div className="mt-2 flex flex-wrap gap-4">
          {healthData.map((d) => (
            <span
              key={d.name}
              className="flex items-center gap-1.5 text-xs text-gray-600 dark:text-gray-400"
            >
              <span
                className="inline-block h-2.5 w-2.5 rounded-full"
                style={{ backgroundColor: COLORS[d.name] ?? '#9ca3af' }}
              />
              {d.name}: {d.value}
            </span>
          ))}
        </div>
      </div>
    </section>
  );
}

function StatBlock({
  label,
  value,
  sub,
}: {
  label: string;
  value: number;
  sub?: string;
}) {
  return (
    <div className="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-800">
      <p className="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
        {label}
      </p>
      <p className="mt-1 text-2xl font-bold text-gray-900 dark:text-gray-100">
        {value}
        {sub && (
          <span className="text-base font-normal text-gray-400"> {sub}</span>
        )}
      </p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Section 2: Sync Activity Timeline
// ---------------------------------------------------------------------------

function SyncActivitySection({
  syncs,
}: {
  syncs: SyncActivityEntry[];
}) {
  const [addonFilter, setAddonFilter] = useState('');
  const [clusterFilter, setClusterFilter] = useState('');

  const addonNames = useMemo(
    () => [...new Set(syncs.map((s) => s.addon_name))].sort(),
    [syncs],
  );
  const clusterNames = useMemo(
    () => [...new Set(syncs.map((s) => s.cluster_name))].sort(),
    [syncs],
  );

  const filtered = useMemo(
    () =>
      syncs.filter(
        (s) =>
          (!addonFilter || s.addon_name === addonFilter) &&
          (!clusterFilter || s.cluster_name === clusterFilter),
      ),
    [syncs, addonFilter, clusterFilter],
  );

  // Bar chart: syncs per hour over the last 24h
  const hourlyData = useMemo(() => {
    const now = Date.now();
    const buckets: Record<number, number> = {};
    for (let i = 0; i < 24; i++) buckets[i] = 0;
    for (const s of syncs) {
      const hoursAgo = Math.floor((now - new Date(s.timestamp).getTime()) / 3600000);
      if (hoursAgo >= 0 && hoursAgo < 24) {
        buckets[hoursAgo]++;
      }
    }
    return Array.from({ length: 24 }, (_, i) => ({
      label: i === 0 ? 'now' : `${i}h`,
      count: buckets[i],
    })).reverse();
  }, [syncs]);

  return (
    <section className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-900">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-lg font-semibold text-gray-900 dark:text-gray-100">
          <Activity className="h-5 w-5 text-cyan-500" />
          Sync Activity
        </h2>
        <div className="flex gap-2">
          <select
            value={addonFilter}
            onChange={(e) => setAddonFilter(e.target.value)}
            className="rounded-md border border-gray-200 bg-white px-2 py-1 text-xs text-gray-700 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300"
            aria-label="Filter by addon"
          >
            <option value="">All Addons</option>
            {addonNames.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
          <select
            value={clusterFilter}
            onChange={(e) => setClusterFilter(e.target.value)}
            className="rounded-md border border-gray-200 bg-white px-2 py-1 text-xs text-gray-700 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300"
            aria-label="Filter by cluster"
          >
            <option value="">All Clusters</option>
            {clusterNames.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Sync frequency chart */}
      {syncs.length > 0 && (
        <div className="mb-5 h-32" style={{ minWidth: 0, minHeight: 0 }}>
          <ResponsiveContainer width="100%" height={128} minWidth={100}>
            <BarChart data={hourlyData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" opacity={0.3} />
              <XAxis dataKey="label" tick={{ fontSize: 10 }} interval={3} />
              <YAxis allowDecimals={false} tick={{ fontSize: 10 }} width={30} />
              <Tooltip />
              <Bar dataKey="count" fill="#06b6d4" radius={[2, 2, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Timeline feed */}
      <div className="max-h-80 space-y-1 overflow-y-auto">
        {filtered.length === 0 && (
          <p className="py-4 text-center text-sm text-gray-500">
            No sync activity found.
          </p>
        )}
        {filtered.map((s, idx) => (
          <div
            key={`${s.timestamp}-${s.app_name}-${idx}`}
            className="flex items-center gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-gray-50 dark:hover:bg-gray-800"
          >
            {statusIcon(s.status)}
            <span className="w-16 shrink-0 text-xs text-gray-400">
              {timeAgo(s.timestamp)}
            </span>
            <span className="min-w-0 flex-1 truncate text-sm font-medium text-gray-900 dark:text-gray-100">
              {s.addon_name}
            </span>
            <span className="hidden truncate text-xs text-gray-500 sm:inline dark:text-gray-400">
              {s.cluster_name}
            </span>
            <span className="flex items-center gap-1 text-xs text-gray-400">
              <Clock className="h-3 w-3" />
              {s.duration}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}

// ---------------------------------------------------------------------------
// Section 3: Addon Health Overview
// ---------------------------------------------------------------------------

function AddonHealthSection({
  addons,
}: {
  addons: AddonHealthDetail[];
}) {
  const [sortMode, setSortMode] = useState<SortMode>('issues');
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const sorted = useMemo(() => {
    const copy = [...addons];
    if (sortMode === 'issues') {
      copy.sort((a, b) => b.degraded_clusters - a.degraded_clusters);
    } else if (sortMode === 'alpha') {
      copy.sort((a, b) => a.addon_name.localeCompare(b.addon_name));
    } else {
      copy.sort((a, b) => {
        const ta = a.last_deploy_time ? new Date(a.last_deploy_time).getTime() : 0;
        const tb = b.last_deploy_time ? new Date(b.last_deploy_time).getTime() : 0;
        return tb - ta;
      });
    }
    return copy;
  }, [addons, sortMode]);

  const toggle = (name: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  };

  return (
    <section>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-lg font-semibold text-gray-900 dark:text-gray-100">
          <Server className="h-5 w-5 text-cyan-500" />
          Addon Health
        </h2>
        <div className="flex gap-1">
          {(['issues', 'alpha', 'deployed'] as SortMode[]).map((m) => (
            <button
              key={m}
              onClick={() => setSortMode(m)}
              className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
                sortMode === m
                  ? 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900/40 dark:text-cyan-300'
                  : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800'
              }`}
            >
              {m === 'issues' ? 'Most Issues' : m === 'alpha' ? 'A-Z' : 'Last Deployed'}
            </button>
          ))}
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {sorted.map((addon) => {
          const isExpanded = expanded.has(addon.addon_name);
          const pct =
            addon.total_clusters > 0
              ? (addon.healthy_clusters / addon.total_clusters) * 100
              : 0;

          return (
            <div
              key={addon.addon_name}
              className="rounded-xl border border-gray-200 bg-white shadow-sm transition-shadow hover:shadow-md dark:border-gray-700 dark:bg-gray-900"
            >
              <button
                onClick={() => toggle(addon.addon_name)}
                className="flex w-full items-center justify-between p-4 text-left"
                aria-expanded={isExpanded}
              >
                <span className="text-sm font-semibold text-gray-900 dark:text-gray-100">
                  {addon.addon_name}
                </span>
                {isExpanded ? (
                  <ChevronUp className="h-4 w-4 text-gray-400" />
                ) : (
                  <ChevronDown className="h-4 w-4 text-gray-400" />
                )}
              </button>

              <div className="px-4 pb-4">
                {/* Health bar */}
                <div className="mb-3">
                  <div className="mb-1 flex items-center justify-between text-xs">
                    <span className="text-gray-500 dark:text-gray-400">
                      {addon.healthy_clusters}/{addon.total_clusters} healthy
                    </span>
                    <span className="font-medium text-gray-700 dark:text-gray-300">
                      {Math.round(pct)}%
                    </span>
                  </div>
                  <div className="h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
                    <div
                      className={`h-full rounded-full transition-all ${
                        pct === 100 ? 'bg-green-500' : pct > 50 ? 'bg-yellow-500' : 'bg-red-500'
                      }`}
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                </div>

                <div className="flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
                  {addon.last_deploy_time && (
                    <span className="flex items-center gap-1">
                      <Clock className="h-3 w-3" />
                      {timeAgo(addon.last_deploy_time)}
                    </span>
                  )}
                  {addon.avg_sync_duration && (
                    <span className="flex items-center gap-1">
                      <RefreshCw className="h-3 w-3" />
                      {addon.avg_sync_duration}
                    </span>
                  )}
                </div>

                {/* Expanded cluster details */}
                {isExpanded && addon.clusters.length > 0 && (
                  <div className="mt-3 space-y-2 border-t border-gray-100 pt-3 dark:border-gray-700">
                    {addon.clusters.map((cl) => (
                      <div
                        key={cl.cluster_name}
                        className="flex flex-wrap items-center gap-2 rounded-md bg-gray-50 px-3 py-2 text-xs dark:bg-gray-800"
                      >
                        <span className="font-medium text-gray-800 dark:text-gray-200">
                          {cl.cluster_name}
                        </span>
                        <span
                          className={`rounded-full px-1.5 py-0.5 text-[10px] font-semibold uppercase ${healthColor(
                            cl.health,
                          )} ${healthBg(cl.health)} bg-opacity-10`}
                        >
                          {cl.health}
                        </span>
                        {cl.health_since && (
                          <span className="text-gray-400">
                            since {timeAgo(cl.health_since)}
                          </span>
                        )}
                        <span className="ml-auto text-gray-400">
                          {cl.healthy_resources}/{cl.resource_count} resources
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

// ---------------------------------------------------------------------------
// Main View
// ---------------------------------------------------------------------------

export function Observability() {
  const [data, setData] = useState<ObservabilityOverviewResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const result = await api.getObservability();
      setData(result);
    } catch (e: unknown) {
      setError(
        e instanceof Error ? e.message : 'Failed to load observability data',
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchData();
  }, [fetchData]);

  if (loading) return <LoadingState message="Loading observability data..." />;
  if (error) return <ErrorState message={error} onRetry={fetchData} />;
  if (!data) return null;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">
        Observability
      </h1>
      <ControlPlaneSection data={data.control_plane} />
      <SyncActivitySection syncs={data.recent_syncs} />
      <AddonHealthSection addons={data.addon_health} />
    </div>
  );
}
