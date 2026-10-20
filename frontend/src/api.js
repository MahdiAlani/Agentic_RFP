// Thin API client. Backend calls go through the Vite dev proxy (/api -> :8080).
import { getToken, clearSession } from './auth'

// Sends an authenticated request and returns the Response, throwing on errors.
async function send(method, path, body) {
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
  return res
}

async function request(method, path, body) {
  const text = await (await send(method, path, body)).text()
  return text ? JSON.parse(text) : null
}

// Fetches a file the API serves (e.g. an export) and saves it in the browser.
async function download(path, fallbackName) {
  const res = await send('GET', path)
  const disposition = res.headers.get('Content-Disposition') || ''
  const name = /filename="([^"]+)"/.exec(disposition)?.[1] || fallbackName

  const url = URL.createObjectURL(await res.blob())
  const link = document.createElement('a')
  link.href = url
  link.download = name
  link.click()
  URL.revokeObjectURL(url)
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

export function renameWorkspace(id, name) {
  return request('PATCH', `/workspaces/${id}`, { name })
}

export function deleteWorkspace(id) {
  return request('DELETE', `/workspaces/${id}`)
}

// --- projects ---

export function listProjects(workspaceID) {
  return request('GET', `/workspaces/${workspaceID}/projects`)
}

export function createProject({ workspaceID, name }) {
  return request('POST', '/projects', { workspace_id: workspaceID, name })
}

export function renameProject(id, name) {
  return request('PATCH', `/projects/${id}`, { name })
}

export function deleteProject(id) {
  return request('DELETE', `/projects/${id}`)
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

export function renameDocument(id, fileName) {
  return request('PATCH', `/documents/${id}`, { file_name: fileName })
}

export function deleteDocument(id) {
  return request('DELETE', `/documents/${id}`)
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

export function renameConversation(id, title) {
  return request('PATCH', `/conversations/${id}`, { title })
}

export function deleteConversation(id) {
  return request('DELETE', `/conversations/${id}`)
}

export function listMessages(conversationID) {
  return request('GET', `/conversations/${conversationID}/messages`)
}

export function sendMessage(conversationID, content) {
  return request('POST', `/conversations/${conversationID}/messages`, { content })
}

// --- answers (the RFP question/answer editor) ---

export function listAnswers(workspaceID) {
  return request('GET', `/workspaces/${workspaceID}/answers`)
}

export function listProjectAnswers(projectID) {
  return request('GET', `/projects/${projectID}/answers`)
}

export function createAnswer({ workspaceID, projectID, documentID, question }) {
  return request('POST', '/answers', {
    workspace_id: workspaceID,
    project_id: projectID || null,
    document_id: documentID || null,
    question,
  })
}

export function updateAnswer(id, answerText) {
  return request('PATCH', `/answers/${id}`, { answer_text: answerText })
}

export function regenerateAnswer(id) {
  return request('POST', `/answers/${id}/regenerate`)
}

// Sign off a drafted answer. Editing it afterwards returns it to review.
export function approveAnswer(id) {
  return request('POST', `/answers/${id}/approve`)
}

export function deleteAnswer(id) {
  return request('DELETE', `/answers/${id}`)
}

// Download the response as a Word document.
export function exportProject(projectID) {
  return download(`/projects/${projectID}/answers/export`, 'rfp-response.docx')
}

// Queue every unanswered question in the project. Resolves to { queued }.
export function fillProjectAnswers(projectID) {
  return request('POST', `/projects/${projectID}/answers/fill`)
}

// Ask a worker to read an RFP document and create a question row per question.
export function extractQuestions({ projectID, workspaceID, documentID }) {
  return request('POST', `/projects/${projectID}/answers/extract`, {
    workspace_id: workspaceID,
    document_id: documentID,
  })
}
