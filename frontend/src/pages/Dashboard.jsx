import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import {
  listWorkspaces,
  createWorkspace,
  listProjects,
  createProject,
} from '../api'
import { getUser, clearUser } from '../auth'

export default function Dashboard() {
  const navigate = useNavigate()
  const user = getUser()

  const [workspaces, setWorkspaces] = useState([])
  const [selected, setSelected] = useState(null)
  const [projects, setProjects] = useState([])
  const [loadingProjects, setLoadingProjects] = useState(false)

  // Load workspaces on mount.
  useEffect(() => {
    listWorkspaces(user.id)
      .then((ws) => {
        const list = ws || []
        setWorkspaces(list)
        if (list.length) setSelected(list[0])
      })
      .catch((err) => toast.error(err.message))
  }, [user.id])

  // Load projects whenever the selected workspace changes.
  useEffect(() => {
    if (!selected) {
      setProjects([])
      return
    }
    setLoadingProjects(true)
    listProjects(selected.id)
      .then((ps) => setProjects(ps || []))
      .catch((err) => toast.error(err.message))
      .finally(() => setLoadingProjects(false))
  }, [selected])

  async function handleNewWorkspace() {
    const name = window.prompt('Workspace name')
    if (!name || !name.trim()) return
    try {
      const ws = await createWorkspace({ userID: user.id, name: name.trim() })
      setWorkspaces((prev) => [...prev, ws])
      setSelected(ws)
      toast.success(`Created “${ws.name}”`)
    } catch (err) {
      toast.error(err.message)
    }
  }

  async function handleNewProject() {
    if (!selected) return
    const name = window.prompt('Project name')
    if (!name || !name.trim()) return
    try {
      const p = await createProject({ workspaceID: selected.id, name: name.trim() })
      setProjects((prev) => [...prev, p])
      toast.success(`Created “${p.name}”`)
    } catch (err) {
      toast.error(err.message)
    }
  }

  function logout() {
    clearUser()
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
            <button className="add" onClick={handleNewWorkspace} title="New workspace">
              +
            </button>
          </div>
          {workspaces.length === 0 ? (
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
          {!selected ? (
            <div className="placeholder">
              <p>Create a workspace to get started.</p>
            </div>
          ) : (
            <>
              <div className="main-head">
                <div>
                  <h2>{selected.name}</h2>
                  <p className="muted">Projects</p>
                </div>
                <button className="primary" onClick={handleNewProject}>
                  New project
                </button>
              </div>

              {loadingProjects ? (
                <p className="muted">Loading…</p>
              ) : projects.length === 0 ? (
                <div className="placeholder">
                  <p>No projects in this workspace yet.</p>
                </div>
              ) : (
                <div className="grid">
                  {projects.map((p) => (
                    <div key={p.id} className="proj-card">
                      <h4>{p.name}</h4>
                      <span className="muted small">
                        {new Date(p.created_at).toLocaleDateString()}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </>
          )}
        </main>
      </div>
    </div>
  )
}
