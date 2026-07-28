// Thin API client. Backend calls go through the Vite dev proxy (/api -> :8080).
import { getToken, clearSession } from './auth'

async function request(method, path, body) {
  const opts = { method, headers: {} }

  const token = getToken()
  if (token) opts.headers['Authorization'] = `Bearer ${token}`

  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }

  const res = await fetch(`/api${path}`, opts)

  // Expired/invalid token: drop the stale session and return to login.
  // Skip for the auth endpoints themselves (a bad login shouldn't redirect).
  if (res.status === 401 && !path.startsWith('/auth/')) {
    clearSession()
    if (window.location.pathname !== '/login') {
      window.location.href = '/login'
    }
    throw new Error('session expired, please sign in again')
  }

  if (!res.ok) {
    const text = await res.text()
    throw new Error(text.trim() || `request failed (${res.status})`)
  }
  const text = await res.text()
  return text ? JSON.parse(text) : null
}

// --- auth (returns { token, user }) ---

export function register({ email, name, password }) {
  return request('POST', '/auth/register', { email, name, password })
}

export function login({ email, password }) {
  return request('POST', '/auth/login', { email, password })
}

// --- workspaces (scoped to the authenticated user via token) ---

export function listWorkspaces() {
  return request('GET', '/workspaces')
}

export function createWorkspace({ name }) {
  return request('POST', '/workspaces', { name })
}

// --- projects ---

export function listProjects(workspaceID) {
  return request('GET', `/workspaces/${workspaceID}/projects`)
}

export function createProject({ workspaceID, name }) {
  return request('POST', '/projects', { workspace_id: workspaceID, name })
}

// --- documents (presigned upload flow) ---

export function listDocuments(workspaceID) {
  return request('GET', `/workspaces/${workspaceID}/documents`)
}

// Step 1: create the metadata row and get a presigned PUT URL.
export function createDocument({ workspaceID, projectID, fileName, documentType }) {
  return request('POST', '/documents', {
    workspace_id: workspaceID,
    project_id: projectID || null,
    file_name: fileName,
    document_type: documentType,
  })
}

// Step 2: PUT the file bytes straight to MinIO (not through the API/proxy).
export async function uploadToStorage(uploadURL, file) {
  const res = await fetch(uploadURL, { method: 'PUT', body: file })
  if (!res.ok) throw new Error(`upload failed (${res.status})`)
}

// Step 3: confirm the upload -> backend verifies + enqueues embedding.
export function confirmDocument(id) {
  return request('POST', `/documents/${id}/confirm`)
}

export function getDownloadURL(id) {
  return request('GET', `/documents/${id}/download`)
}

export function listProjectDocuments(projectID) {
  return request('GET', `/projects/${projectID}/documents`)
}

// --- projects ---

export function getProject(id) {
  return request('GET', `/projects/${id}`)
}

// --- chat (multi-turn, project-scoped) ---

export function listConversations(projectID) {
  return request('GET', `/projects/${projectID}/conversations`)
}

export function createConversation(projectID, title) {
  return request('POST', `/projects/${projectID}/conversations`, { title: title || '' })
}

export function listMessages(conversationID) {
  return request('GET', `/conversations/${conversationID}/messages`)
}

export function sendMessage(conversationID, content) {
  return request('POST', `/conversations/${conversationID}/messages`, { content })
}

// --- answers (Q&A) ---

export function listAnswers(workspaceID) {
  return request('GET', `/workspaces/${workspaceID}/answers`)
}

export function createAnswer({ workspaceID, projectID, documentID, question }) {
  return request('POST', '/answers', {
    workspace_id: workspaceID,
    project_id: projectID || null,
    document_id: documentID || null,
    question,
  })
}
