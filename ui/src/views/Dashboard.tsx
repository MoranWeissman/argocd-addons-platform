import { useState, useEffect, useCallback } from 'react';
import {
  PieChart,
  Pie,
  Cell,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import { Server, AppWindow, Package, Rocket } from 'lucide-react';
import { api } from '@/services/api';
import type { DashboardStats } from '@/services/models';
import { StatCard } from '@/components/StatCard';
import { LoadingState } from '@/components/LoadingState';
import { ErrorState } from '@/components/ErrorState';

const HEALTH_COLORS: Record<string, string> = {
  Healthy: '#22c55e',
  Progressing: '#3b82f6',
  Degraded: '#ef4444',
  Unknown: '#9ca3af',
};

const CONNECTION_COLORS: Record<string, string> = {
  Connected: '#22c55e',
  Disconnected: '#ef4444',
};

export function Dashboard() {
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

  const healthData = [
    { name: 'Healthy', value: stats.applications.by_health_status.healthy },
    { name: 'Progressing', value: stats.applications.by_health_status.progressing },
    { name: 'Degraded', value: stats.applications.by_health_status.degraded },
    { name: 'Unknown', value: stats.applications.by_health_status.unknown },
  ].filter((d) => d.value > 0);

  const clusterData = [
    { name: 'Connected', value: stats.clusters.connected_to_argocd },
    { name: 'Disconnected', value: stats.clusters.disconnected_from_argocd },
  ].filter((d) => d.value > 0);

  return (
    <div className="space-y-8">
      {/* Hero Section */}
      <div className="rounded-2xl bg-gradient-to-r from-cyan-600 to-blue-700 px-8 py-10 text-white shadow-lg dark:from-cyan-800 dark:to-blue-900">
        <h1 className="text-3xl font-bold tracking-tight sm:text-4xl">
          ArgoCD Addons Platform
        </h1>
        <p className="mt-2 max-w-2xl text-lg text-cyan-100">
          Centralized visibility into addon deployments, health status, and
          configurations across all your Kubernetes clusters.
        </p>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total Clusters"
          value={stats.clusters.total}
          icon={<Server className="h-6 w-6" />}
          color="default"
        />
        <StatCard
          title="Applications"
          value={stats.applications.total}
          icon={<AppWindow className="h-6 w-6" />}
          color="success"
        />
        <StatCard
          title="Available Addons"
          value={stats.addons.total_available}
          icon={<Package className="h-6 w-6" />}
          color="default"
        />
        <StatCard
          title="Active Deployments"
          value={`${stats.addons.enabled_deployments} / ${stats.addons.total_deployments}`}
          icon={<Rocket className="h-6 w-6" />}
          color="warning"
        />
      </div>

      {/* Charts Section — Health + Cluster Connectivity */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Application Health Status */}
        <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <h3 className="mb-1 text-lg font-semibold text-gray-900 dark:text-gray-100">
            Application Health Status
          </h3>
          <p className="mb-4 text-sm text-gray-500 dark:text-gray-400">
            Operational health of deployed applications
          </p>
          <ResponsiveContainer width="100%" height={280}>
            <PieChart>
              <Pie
                data={healthData}
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={100}
                paddingAngle={2}
                dataKey="value"
                label
              >
                {healthData.map((entry) => (
                  <Cell
                    key={entry.name}
                    fill={HEALTH_COLORS[entry.name] ?? '#9ca3af'}
                  />
                ))}
              </Pie>
              <Tooltip />
              <Legend />
            </PieChart>
          </ResponsiveContainer>
        </div>

        {/* Cluster Connectivity */}
        <div className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <h3 className="mb-1 text-lg font-semibold text-gray-900 dark:text-gray-100">
            Cluster Connectivity
          </h3>
          <p className="mb-4 text-sm text-gray-500 dark:text-gray-400">
            Cluster connection status to ArgoCD
          </p>
          <ResponsiveContainer width="100%" height={280}>
            <PieChart>
              <Pie
                data={clusterData}
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={100}
                paddingAngle={2}
                dataKey="value"
                label
              >
                {clusterData.map((entry) => (
                  <Cell
                    key={entry.name}
                    fill={CONNECTION_COLORS[entry.name] ?? '#9ca3af'}
                  />
                ))}
              </Pie>
              <Tooltip />
              <Legend />
            </PieChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
}
