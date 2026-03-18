import { useState } from 'react'
import { useAuth } from '@/hooks/useAuth'

export function Login() {
  const { login, error: authError } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!username || !password) {
      setError('Username and password are required')
      return
    }
    setError('')
    setLoading(true)
    try {
      await login(username, password)
    } catch {
      setError(authError || 'Invalid credentials')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      className="relative flex min-h-screen items-center justify-center bg-cover bg-center"
      style={{ backgroundImage: 'url(/login-bg.png)' }}
    >
      {/* Overlay for readability on small screens */}
      <div className="absolute inset-0 bg-gray-900/40 backdrop-blur-sm lg:hidden" />

      {/* Desktop: side-by-side layout */}
      <div className="relative flex w-full min-h-screen">
        {/* Left side — background shows through (desktop only) */}
        <div className="hidden flex-1 lg:block" />

        {/* Right side — login form */}
        <div className="flex w-full flex-col items-center justify-center px-6 py-12 lg:w-[420px] lg:min-w-[420px] lg:bg-white lg:px-8 lg:dark:bg-gray-900">
          <div className="w-full max-w-sm rounded-2xl bg-white/95 p-8 shadow-xl backdrop-blur-md dark:bg-gray-900/95 lg:rounded-none lg:bg-transparent lg:p-0 lg:shadow-none lg:backdrop-blur-none lg:dark:bg-transparent">
            {/* Logo */}
            <div className="mb-8 text-center">
              <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
                AAP
              </h1>
              <p className="mt-1 text-sm font-medium text-gray-700 dark:text-gray-300">
                ArgoCD Addons Platform
              </p>
              <p className="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
                Control plane for Kubernetes add-ons
              </p>
            </div>

            <form onSubmit={handleSubmit} className="space-y-5">
              <div>
                <label
                  htmlFor="username"
                  className="block text-sm font-medium text-gray-700 dark:text-gray-300"
                >
                  Username
                </label>
                <input
                  id="username"
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoComplete="username"
                  autoFocus
                  className="mt-1 block w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm text-gray-900 placeholder-gray-400 focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white dark:placeholder-gray-500 dark:focus:border-cyan-500"
                  placeholder="admin"
                />
              </div>

              <div>
                <label
                  htmlFor="password"
                  className="block text-sm font-medium text-gray-700 dark:text-gray-300"
                >
                  Password
                </label>
                <input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="current-password"
                  className="mt-1 block w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm text-gray-900 placeholder-gray-400 focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-600 dark:bg-gray-800 dark:text-white dark:placeholder-gray-500 dark:focus:border-cyan-500"
                  placeholder="Password"
                />
              </div>

              {error && (
                <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
              )}

              <button
                type="submit"
                disabled={loading}
                className="w-full rounded-lg bg-cyan-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-cyan-700 focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:ring-offset-2 disabled:opacity-50 dark:bg-cyan-700 dark:hover:bg-cyan-600 dark:focus:ring-offset-gray-900"
              >
                {loading ? 'Signing in...' : 'SIGN IN'}
              </button>
            </form>

            {/* Footer */}
            <p className="mt-12 text-center text-[10px] text-gray-400 dark:text-gray-600">
              AAP
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
