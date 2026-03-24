import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Server, AppWindow, Package, Rocket, AlertTriangle, CheckCircle2 } from 'lucide-react';
import { api } from '@/services/api';
import type { DashboardStats } from '@/services/models';
import { StatCard } from '@/components/StatCard';
import { LoadingState } from '@/components/LoadingState';
import { ErrorState } from '@/components/ErrorState';

interface HealthBarProps {
  title: string;
  subtitle: string;
  segments: { label: string; value: number; color: string }[];
}

function HealthBar({ title, subtitle, segments }: HealthBarProps) {
  const total = segments.reduce((sum, s) => sum + s.value, 0);
  if (total === 0) return null;

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100">{title}</h3>
      <p className="mb-3 text-xs text-gray-500 dark:text-gray-400">{subtitle}</p>

      {/* Stacked bar */}
      <div className="mb-3 flex h-3 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
        {segments.filter(s => s.value > 0).map((seg) => (
          <div
            key={seg.label}
            className="transition-all duration-500"
            style={{ width: `${(seg.value / total) * 100}%`, backgroundColor: seg.color }}
            title={`${seg.label}: ${seg.value}`}
          />
        ))}
      </div>

      {/* Legend */}
      <div className="flex flex-wrap gap-x-4 gap-y-1">
        {segments.filter(s => s.value > 0).map((seg) => (
          <div key={seg.label} className="flex items-center gap-1.5 text-xs">
            <div className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: seg.color }} />
            <span className="font-medium text-gray-700 dark:text-gray-300">{seg.value}</span>
            <span className="text-gray-500 dark:text-gray-400">{seg.label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function Dashboard() {
  const navigate = useNavigate();
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const statsData = await api.getDashboardStats();
      setStats(statsData);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load dashboard');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchData();
  }, [fetchData]);

  if (loading) {
    return <LoadingState message="Loading dashboard statistics..." />;
  }

  if (error) {
    return <ErrorState message={error} onRetry={fetchData} />;
  }

  if (!stats) return null;

  const degradedCount = stats.applications.by_health_status.degraded;
  const disconnectedCount = stats.clusters.disconnected_from_argocd;
  const hasIssues = degradedCount > 0 || disconnectedCount > 0;

  return (
    <div className="space-y-6">
      {/* Hero Section */}
      <div className="rounded-2xl bg-gradient-to-r from-cyan-600 to-blue-700 px-8 py-10 text-white shadow-lg dark:from-cyan-800 dark:to-blue-900">
        <h1 className="text-3xl font-bold tracking-tight sm:text-4xl">
          ArgoCD Addons Platform
        </h1>
        <p className="mt-2 max-w-2xl text-lg text-cyan-100">
          Centralized visibility into add-on deployments, health status, and
          configurations across all your Kubernetes clusters.
        </p>
      </div>

      {/* Needs Attention */}
      {hasIssues ? (
        <div className="rounded-xl border-2 border-amber-300 bg-amber-50 p-5 dark:border-amber-700 dark:bg-amber-900/20">
          <div className="mb-3 flex items-center gap-2 text-amber-700 dark:text-amber-400">
            <AlertTriangle className="h-5 w-5" />
            <h3 className="text-sm font-semibold">Needs Attention</h3>
          </div>
          <div className="flex flex-wrap gap-3">
            {degradedCount > 0 && (
              <button
                onClick={() => navigate('/addons?filter=unhealthy')}
                className="flex items-center gap-2 rounded-lg border border-red-200 bg-white px-4 py-2 text-sm text-red-700 transition-colors hover:bg-red-50 dark:border-red-800 dark:bg-gray-800 dark:text-red-400 dark:hover:bg-red-900/20"
              >
                <div className="h-2 w-2 rounded-full bg-red-500" />
                {degradedCount} degraded application{degradedCount !== 1 ? 's' : ''}
              </button>
            )}
            {disconnectedCount > 0 && (
              <button
                onClick={() => navigate('/clusters?status=disconnected')}
                className="flex items-center gap-2 rounded-lg border border-red-200 bg-white px-4 py-2 text-sm text-red-700 transition-colors hover:bg-red-50 dark:border-red-800 dark:bg-gray-800 dark:text-red-400 dark:hover:bg-red-900/20"
              >
                <div className="h-2 w-2 rounded-full bg-red-500" />
                {disconnectedCount} disconnected cluster{disconnectedCount !== 1 ? 's' : ''}
              </button>
            )}
          </div>
        </div>
      ) : (
        <div className="flex items-center gap-2 rounded-xl border border-green-200 bg-green-50 px-5 py-4 dark:border-green-800 dark:bg-green-900/20">
          <CheckCircle2 className="h-5 w-5 text-green-600 dark:text-green-400" />
          <span className="text-sm font-medium text-green-700 dark:text-green-400">All systems operational</span>
        </div>
      )}

      {/* Stats Cards — clickable */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total Clusters"
          value={stats.clusters.total}
          icon={<Server className="h-6 w-6" />}
          color="default"
          onClick={() => navigate('/clusters')}
        />
        <StatCard
          title="Applications"
          value={stats.applications.total}
          icon={<AppWindow className="h-6 w-6" />}
          color="success"
          onClick={() => navigate('/addons')}
        />
        <StatCard
          title="Available Add-ons"
          value={stats.addons.total_available}
          icon={<Package className="h-6 w-6" />}
          color="default"
          onClick={() => navigate('/addons')}
        />
        <StatCard
          title="Active Deployments"
          value={`${stats.addons.enabled_deployments} / ${stats.addons.total_deployments}`}
          icon={<Rocket className="h-6 w-6" />}
          color="warning"
          onClick={() => navigate('/version-matrix')}
        />
      </div>

      {/* Health Bars — replace pie charts */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <HealthBar
          title="Application Health"
          subtitle="Operational health of deployed applications"
          segments={[
            { label: 'Healthy', value: stats.applications.by_health_status.healthy, color: '#22c55e' },
            { label: 'Progressing', value: stats.applications.by_health_status.progressing, color: '#3b82f6' },
            { label: 'Degraded', value: stats.applications.by_health_status.degraded, color: '#ef4444' },
            { label: 'Unknown', value: stats.applications.by_health_status.unknown, color: '#9ca3af' },
          ]}
        />
        <HealthBar
          title="Cluster Connectivity"
          subtitle="Cluster connection status to ArgoCD"
          segments={[
            { label: 'Connected', value: stats.clusters.connected_to_argocd, color: '#22c55e' },
            { label: 'Disconnected', value: stats.clusters.disconnected_from_argocd, color: '#ef4444' },
          ]}
        />
      </div>
    </div>
  );
}
