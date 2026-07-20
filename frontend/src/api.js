// Thin API client. All calls go through the Vite dev proxy (/api -> :8080).

async function request(method, path, body) {
  const opts = { method, headers: {} }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }

  const res = await fetch(`/api${path}`, opts)
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text.trim() || `request failed (${res.status})`)
  }
  const text = await res.text()
  return text ? JSON.parse(text) : null
}

// --- auth ---

// Signup -> POST /users (creates a user with a hashed password).
export function signUp({ email, name, password }) {
  return request('POST', '/users', { email, name, password })
}

// Signin -> POST /auth/login (NOTE: backend endpoint not built yet; will 404).
export function signIn({ email, password }) {
  return request('POST', '/auth/login', { email, password })
}

// --- workspaces ---

export function listWorkspaces(userID) {
  return request('GET', `/users/${userID}/workspaces`)
}

export function createWorkspace({ userID, name }) {
  return request('POST', '/workspaces', { user_id: userID, name })
}

// --- projects ---

export function listProjects(workspaceID) {
  return request('GET', `/workspaces/${workspaceID}/projects`)
}

export function createProject({ workspaceID, name }) {
  return request('POST', '/projects', { workspace_id: workspaceID, name })
}
