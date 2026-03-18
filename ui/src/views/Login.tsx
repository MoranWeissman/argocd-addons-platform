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
    <div className="flex min-h-screen">
      {/* Left side — background image */}
      <div
        className="hidden flex-1 bg-cover bg-center lg:block"
        style={{ backgroundImage: 'url(/login-bg.png)' }}
      />

      {/* Right side — login form */}
      <div className="flex w-full flex-col items-center justify-center bg-white px-8 lg:w-[420px] lg:min-w-[420px] dark:bg-gray-900">
        <div className="w-full max-w-sm">
          {/* Logo */}
          <div className="mb-8 text-center">
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
              argocd
            </h1>
            <p className="mt-1 text-sm tracking-widest text-gray-500 dark:text-gray-400">
              ADDONS PLATFORM
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
            argocd addons platform
          </p>
        </div>
      </div>
    </div>
  )
}
