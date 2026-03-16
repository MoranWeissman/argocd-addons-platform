import { useState, useCallback } from 'react';

export interface EmbeddedDashboard {
  id: string;
  name: string;
  url: string;
  provider: 'datadog' | 'grafana' | 'custom';
}

const STORAGE_KEY = 'aap-dashboards';

function loadDashboards(): EmbeddedDashboard[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    return JSON.parse(raw) as EmbeddedDashboard[];
  } catch {
    return [];
  }
}

function saveDashboards(dashboards: EmbeddedDashboard[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(dashboards));
}

/** Extract the src URL from an iframe snippet, or return the input as-is. */
export function extractUrlFromIframe(input: string): string {
  const match = input.match(/src=["']([^"']+)["']/);
  return match ? match[1] : input;
}

export function useDashboards() {
  const [dashboards, setDashboards] = useState<EmbeddedDashboard[]>(loadDashboards);

  const addDashboard = useCallback(
    (dashboard: Omit<EmbeddedDashboard, 'id'>) => {
      const newDashboard: EmbeddedDashboard = {
        ...dashboard,
        id: crypto.randomUUID?.() ?? Date.now().toString(),
      };
      setDashboards((prev) => {
        const next = [...prev, newDashboard];
        saveDashboards(next);
        return next;
      });
    },
    [],
  );

  const updateDashboard = useCallback(
    (id: string, updates: Partial<Omit<EmbeddedDashboard, 'id'>>) => {
      setDashboards((prev) => {
        const next = prev.map((d) => (d.id === id ? { ...d, ...updates } : d));
        saveDashboards(next);
        return next;
      });
    },
    [],
  );

  const removeDashboard = useCallback((id: string) => {
    setDashboards((prev) => {
      const next = prev.filter((d) => d.id !== id);
      saveDashboards(next);
      return next;
    });
  }, []);

  return { dashboards, addDashboard, updateDashboard, removeDashboard };
}
