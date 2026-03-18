import { useState, useRef, useEffect } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import {
  LayoutDashboard,
  Server,
  Package,
  TableProperties,
  Activity,
  ArrowUpCircle,
  MessageSquare,
  BarChart3,
  BookOpen,
  Settings,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  Plug,
  Sun,
  Moon,
} from 'lucide-react'
import { useConnections } from '@/hooks/useConnections'
import { useTheme } from '@/hooks/useTheme'
import { DateTimeDisplay } from '@/components/DateTimeDisplay'

const navItems = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/clusters', label: 'Clusters', icon: Server },
  { to: '/addons', label: 'Addon Catalog', icon: Package },
  { to: '/version-matrix', label: 'Version Matrix', icon: TableProperties },
  { to: '/observability', label: 'Observability', icon: Activity },
  { to: '/upgrade', label: 'Upgrade Checker', icon: ArrowUpCircle },
  { to: '/assistant', label: 'AI Assistant', icon: MessageSquare },
  { to: '/dashboards', label: 'Dashboards', icon: BarChart3 },
  { to: '/docs', label: 'Docs', icon: BookOpen },
  { to: '/settings', label: 'Settings', icon: Settings },
]

export function Layout() {
  const navigate = useNavigate()
  const [collapsed, setCollapsed] = useState(false)
  const [dropdownOpen, setDropdownOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const { theme, toggleTheme } = useTheme()

  const { connections, activeConnection, setActiveConnection, loading } =
    useConnections()

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node)
      ) {
        setDropdownOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const handleConnectionSelect = async (name: string) => {
    await setActiveConnection(name)
    setDropdownOpen(false)
  }

  return (
    <div className="flex h-screen bg-gray-50 dark:bg-gray-950">
      {/* Sidebar — always dark like ArgoCD */}
      <aside
        className={`flex flex-col bg-slate-900 shadow-sm transition-[width] duration-200 ${
          collapsed ? 'w-16' : 'w-60'
        }`}
      >
        {/* Logo / title */}
        <div
          className="flex h-14 cursor-pointer items-center gap-2 border-b border-slate-700 px-4 transition-colors hover:bg-slate-800"
          onClick={() => navigate('/')}
        >
          <Package className="h-6 w-6 shrink-0 text-cyan-400" />
          {!collapsed && (
            <div className="flex flex-col leading-tight">
              <span className="text-sm font-bold text-white">AAP</span>
              <span className="text-[10px] text-slate-400">
                ArgoCD Addons Platform
              </span>
            </div>
          )}
        </div>

        {/* Navigation */}
        <nav className="flex-1 space-y-1 px-2 py-4">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                `flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
                  isActive
                    ? 'border-l-[3px] border-cyan-400 bg-slate-700 text-white'
                    : 'border-l-[3px] border-transparent text-slate-300 hover:bg-slate-800 hover:text-white'
                } ${collapsed ? 'justify-center px-0' : ''}`
              }
              title={collapsed ? item.label : undefined}
            >
              <item.icon className="h-5 w-5 shrink-0" />
              {!collapsed && <span>{item.label}</span>}
            </NavLink>
          ))}
        </nav>

        {/* Collapse toggle */}
        <div className="border-t border-slate-700 p-2">
          <button
            onClick={() => setCollapsed((c) => !c)}
            className="flex w-full items-center justify-center rounded-lg p-2 text-slate-400 hover:bg-slate-800 hover:text-white"
            aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            {collapsed ? (
              <ChevronRight className="h-5 w-5" />
            ) : (
              <ChevronLeft className="h-5 w-5" />
            )}
          </button>
        </div>
      </aside>

      {/* Right side: top bar + content */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Top bar */}
        <header className="flex h-14 shrink-0 items-center justify-between border-b border-gray-200 bg-white px-6 dark:border-gray-700 dark:bg-gray-900">
          <div>{/* Breadcrumb placeholder */}</div>

          <div className="flex items-center gap-4">
            {/* Dark mode toggle */}
            <button
              onClick={toggleTheme}
              className="rounded-lg p-2 text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200"
              aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
            >
              {theme === 'dark' ? (
                <Sun className="h-5 w-5" />
              ) : (
                <Moon className="h-5 w-5" />
              )}
            </button>

            {/* Connection switcher */}
            {!loading && (
              <div ref={dropdownRef} className="relative">
                <button
                  onClick={() => setDropdownOpen((o) => !o)}
                  className="flex items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
                  aria-haspopup="listbox"
                  aria-expanded={dropdownOpen}
                >
                  <Plug className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                  <span>{activeConnection ?? 'No connection'}</span>
                  <ChevronDown className="h-4 w-4 text-gray-400" />
                </button>

                {dropdownOpen && (
                  <div className="absolute right-0 z-50 mt-1 w-56 rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-600 dark:bg-gray-800">
                    {connections.map((conn) => (
                      <button
                        key={conn.name}
                        onClick={() => handleConnectionSelect(conn.name)}
                        className={`flex w-full items-center px-4 py-2 text-left text-sm ${
                          conn.name === activeConnection
                            ? 'bg-cyan-50 font-medium text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400'
                            : 'text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700'
                        }`}
                        role="option"
                        aria-selected={conn.name === activeConnection}
                      >
                        {conn.name}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}

            <DateTimeDisplay />
          </div>
        </header>

        {/* Content */}
        <main className="flex-1 overflow-auto">
          <div className="mx-auto max-w-7xl p-8">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
