import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { ConnectionProvider } from '@/hooks/useConnections'
import { ThemeProvider } from '@/hooks/useTheme'
import { Layout } from '@/components/Layout'
import { Dashboard } from '@/views/Dashboard'
import { ClustersOverview } from '@/views/ClustersOverview'
import { ClusterDetail } from '@/views/ClusterDetail'
import { AddonCatalog } from '@/views/AddonCatalog'
import { AddonDetail } from '@/views/AddonDetail'
import { Connections } from '@/views/Connections'
import { VersionMatrix } from '@/views/VersionMatrix'
import { Docs } from '@/views/Docs'
import { Observability } from '@/views/Observability'
import { Dashboards } from '@/views/Dashboards'
import { UpgradeChecker } from '@/views/UpgradeChecker'
import { AIAssistant } from '@/views/AIAssistant'

export default function App() {
  return (
    <BrowserRouter>
      <ThemeProvider>
        <ConnectionProvider>
          <Routes>
            <Route path="/" element={<Layout />}>
              <Route index element={<Navigate to="/dashboard" replace />} />
              <Route path="dashboard" element={<Dashboard />} />
              <Route path="clusters" element={<ClustersOverview />} />
              <Route path="clusters/:name" element={<ClusterDetail />} />
              <Route path="addons" element={<AddonCatalog />} />
              <Route path="addons/:name" element={<AddonDetail />} />
              <Route path="version-matrix" element={<VersionMatrix />} />
              <Route path="observability" element={<Observability />} />
              <Route path="upgrade" element={<UpgradeChecker />} />
              <Route path="assistant" element={<AIAssistant />} />
              <Route path="dashboards" element={<Dashboards />} />
              <Route path="docs" element={<Docs />} />
              <Route path="settings" element={<Connections />} />
            </Route>
          </Routes>
        </ConnectionProvider>
      </ThemeProvider>
    </BrowserRouter>
  )
}
