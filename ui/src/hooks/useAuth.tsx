import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react'

interface AuthContextType {
  token: string | null
  login: (username: string, password: string) => Promise<void>
  logout: () => void
  isAuthenticated: boolean
  loading: boolean
  error: string | null
}

const AuthContext = createContext<AuthContextType | null>(null)

const TOKEN_KEY = 'aap-auth-token'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => sessionStorage.getItem(TOKEN_KEY))
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Verify existing token on mount
  useEffect(() => {
    if (!token) {
      setLoading(false)
      return
    }
    // Quick check — try health endpoint with token
    fetch('/api/v1/health', {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => {
        if (!r.ok) {
          setToken(null)
          sessionStorage.removeItem(TOKEN_KEY)
        }
      })
      .catch(() => {
        // If health doesn't require auth, token might still be valid
      })
      .finally(() => setLoading(false))
  }, [token])

  const login = useCallback(async (username: string, password: string) => {
    setError(null)
    const res = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({ error: 'Login failed' }))
      setError(data.error || 'Invalid credentials')
      throw new Error(data.error || 'Login failed')
    }
    const data = await res.json()
    setToken(data.token)
    sessionStorage.setItem(TOKEN_KEY, data.token)
  }, [])

  const logout = useCallback(() => {
    setToken(null)
    sessionStorage.removeItem(TOKEN_KEY)
  }, [])

  return (
    <AuthContext.Provider value={{ token, login, logout, isAuthenticated: !!token, loading, error }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
