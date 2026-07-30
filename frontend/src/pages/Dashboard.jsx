import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  listWorkspaces,
  createWorkspace,
  renameWorkspace,
  deleteWorkspace,
} from '../api'
import { getUser, clearSession } from '../auth'
import DocumentsPanel from '../components/DocumentsPanel.jsx'
import ProjectsPanel from '../components/ProjectsPanel.jsx'
import Alert from '../components/Alert.jsx'
import InlineEdit from '../components/InlineEdit.jsx'
import ConfirmDialog from '../components/ConfirmDialog.jsx'
import { PencilIcon, TrashIcon, PlusIcon } from '../components/icons.jsx'

export default function Dashboard() {
  const navigate = useNavigate()
  const user = getUser()

  const [workspaces, setWorkspaces] = useState([])
  const [selected, setSelected] = useState(null)
  const [error, setError] = useState('')
  const [creating, setCreating] = useState(false)
  const [renaming, setRenaming] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    listWorkspaces()
      .then((ws) => {
        const list = ws || []
        setWorkspaces(list)
        if (list.length) setSelected(list[0])
      })
      .catch((err) => setError(err.message))
  }, [])

  async function handleCreate(name) {
    setError('')
    try {
      const ws = await createWorkspace({ name })
      setWorkspaces((prev) => [...prev, ws])
      setSelected(ws)
      setCreating(false)
    } catch (err) {
      setError(err.message)
    }
  }

  async function handleRename(name) {
    setError('')
    try {
      const ws = await renameWorkspace(selected.id, name)
      setWorkspaces((prev) => prev.map((w) => (w.id === ws.id ? ws : w)))
      setSelected(ws)
      setRenaming(false)
    } catch (err) {
      setError(err.message)
    }
  }

  async function handleDelete() {
    setError('')
    try {
      await deleteWorkspace(selected.id)
      const remaining = workspaces.filter((w) => w.id !== selected.id)
      setWorkspaces(remaining)
      setSelected(remaining[0] || null)
      setConfirmDelete(false)
    } catch (err) {
      setError(err.message)
      setConfirmDelete(false)
    }
  }

  function logout() {
    clearSession()
    navigate('/login')
  }

  return (
    <div className="dash">
      <header className="topbar">
        <span className="mark">Agentic&nbsp;RFP</span>
        <div className="topbar-right">
          <span className="who">{user.name || user.email}</span>
          <button className="ghost" onClick={logout}>
            Sign out
          </button>
        </div>
      </header>

      <div className="dash-body">
        <aside className="sidebar">
          <div className="sidebar-head">
            <h3>Workspaces</h3>
            <button
              className="icon-btn"
              onClick={() => setCreating(true)}
              title="New workspace"
            >
              <PlusIcon />
            </button>
          </div>

          {creating && (
            <InlineEdit
              placeholder="Product or company name"
              onSave={handleCreate}
              onCancel={() => setCreating(false)}
            />
          )}

          {workspaces.length === 0 && !creating ? (
            <p className="empty">No workspaces yet.</p>
          ) : (
            <ul className="ws-list">
              {workspaces.map((ws) => (
                <li key={ws.id}>
                  <button
                    className={selected?.id === ws.id ? 'ws active' : 'ws'}
                    onClick={() => setSelected(ws)}
                  >
                    {ws.name}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </aside>

        <main className="main">
          <Alert message={error} />
          {!selected ? (
            <div className="placeholder">
              <p>Create a workspace (your product) to get started.</p>
            </div>
          ) : (
            <>
              <div className="main-head">
                <div className="main-head-row">
                  {renaming ? (
                    <InlineEdit
                      value={selected.name}
                      className="lg"
                      onSave={handleRename}
                      onCancel={() => setRenaming(false)}
                    />
                  ) : (
                    <>
                      <h2>{selected.name}</h2>
                      <div className="row-actions">
                        <button
                          className="icon-btn"
                          title="Rename workspace"
                          onClick={() => setRenaming(true)}
                        >
                          <PencilIcon />
                        </button>
                        <button
                          className="icon-btn danger"
                          title="Delete workspace"
                          onClick={() => setConfirmDelete(true)}
                        >
                          <TrashIcon />
                        </button>
                      </div>
                    </>
                  )}
                </div>
                <p className="muted">Product knowledge base &amp; RFP projects</p>
              </div>

              <section className="section">
                <h3 className="section-title">Knowledge base</h3>
                <p className="muted section-sub">
                  Upload product documentation. This is what answers are grounded in.
                </p>
                <DocumentsPanel workspace={selected} />
              </section>

              <section className="section">
                <h3 className="section-title">RFP projects</h3>
                <p className="muted section-sub">
                  Each project is an RFP you're responding to. Open one to work on it.
                </p>
                <ProjectsPanel
                  workspace={selected}
                  onOpen={(p) => navigate(`/projects/${p.id}`)}
                />
              </section>
            </>
          )}
        </main>
      </div>

      <ConfirmDialog
        open={confirmDelete}
        title={`Delete "${selected?.name}"?`}
        message="Its documents, projects and chats will be permanently deleted."
        onConfirm={handleDelete}
        onCancel={() => setConfirmDelete(false)}
      />
    </div>
  )
}
