// Minimal client-side auth state.
// NOTE: temporary — there is no real session/token yet. The "current user" is
// whoever last signed up, persisted in localStorage. Replace once POST /auth/login
// and token-based sessions exist.

const KEY = 'agentic_rfp_user'

export function getUser() {
  const raw = localStorage.getItem(KEY)
  return raw ? JSON.parse(raw) : null
}

export function setUser(user) {
  localStorage.setItem(KEY, JSON.stringify(user))
}

export function clearUser() {
  localStorage.removeItem(KEY)
}
