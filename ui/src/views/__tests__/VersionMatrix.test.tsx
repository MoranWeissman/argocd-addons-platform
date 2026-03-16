import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { VersionMatrix } from '@/views/VersionMatrix'

vi.mock('@/services/api', () => ({
  api: {
    getVersionMatrix: () =>
      Promise.resolve({
        clusters: ['cluster-prod-1', 'cluster-prod-2', 'cluster-dev-1'],
        addons: [
          {
            addon_name: 'ingress-nginx',
            catalog_version: '4.8.0',
            chart: 'ingress-nginx',
            cells: {
              'cluster-prod-1': { version: '4.8.0', health: 'Healthy', drift_from_catalog: false },
              'cluster-prod-2': { version: '4.7.1', health: 'Healthy', drift_from_catalog: true },
              'cluster-dev-1': { version: '4.8.0', health: 'Degraded', drift_from_catalog: false },
            },
          },
          {
            addon_name: 'cert-manager',
            catalog_version: '1.13.0',
            chart: 'cert-manager',
            cells: {
              'cluster-prod-1': { version: '1.13.0', health: 'Healthy', drift_from_catalog: false },
            },
          },
        ],
      }),
  },
}))

function renderMatrix() {
  return render(
    <MemoryRouter>
      <VersionMatrix />
    </MemoryRouter>,
  )
}

describe('VersionMatrix', () => {
  beforeEach(() => { vi.clearAllMocks() })

  it('renders loading state initially', () => {
    renderMatrix()
    expect(screen.getByText('Loading version matrix...')).toBeInTheDocument()
  })

  it('renders addon rows after loading', async () => {
    renderMatrix()
    await waitFor(() => {
      expect(screen.getByText('Addon Version Matrix')).toBeInTheDocument()
    })
    expect(screen.getByText('ingress-nginx')).toBeInTheDocument()
    expect(screen.getByText('cert-manager')).toBeInTheDocument()
  })

  it('renders cluster chips within addon rows', async () => {
    renderMatrix()
    await waitFor(() => {
      expect(screen.getByText('ingress-nginx')).toBeInTheDocument()
    })
    // Cluster names appear as chips (with -eks stripped)
    expect(screen.getAllByText('cluster-prod-1').length).toBeGreaterThanOrEqual(1)
  })

  it('renders search input', async () => {
    renderMatrix()
    await waitFor(() => {
      expect(screen.getByText('Addon Version Matrix')).toBeInTheDocument()
    })
    expect(screen.getByPlaceholderText('Search addon by name...')).toBeInTheDocument()
  })

  it('shows stats summary', async () => {
    renderMatrix()
    await waitFor(() => {
      expect(screen.getByText('2 addons')).toBeInTheDocument()
    })
    expect(screen.getAllByText('3 clusters').length).toBeGreaterThanOrEqual(1)
  })
})
