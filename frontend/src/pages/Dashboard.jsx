import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { listWorkspaces, createWorkspace } from '../api'
import { getUser, clearSession } from '../auth'
import DocumentsPanel from '../components/DocumentsPanel.jsx'
import ProjectsPanel from '../components/ProjectsPanel.jsx'
import Alert from '../components/Alert.jsx'

export default function Dashboard() {
  const navigate = useNavigate()
  const user = getUser()

  const [workspaces, setWorkspaces] = useState([])
  const [selected, setSelected] = useState(null)
  const [error, setError] = useState('')

  useEffect(() => {
    listWorkspaces()
      .then((ws) => {
        const list = ws || []
        setWorkspaces(list)
        if (list.length) setSelected(list[0])
      })
      .catch((err) => setError(err.message))
  }, [])

  async function handleNewWorkspace() {
    const name = window.prompt('Workspace name (your product / company)')
    if (!name || !name.trim()) return
    setError('')
    try {
      const ws = await createWorkspace({ name: name.trim() })
      setWorkspaces((prev) => [...prev, ws])
      setSelected(ws)
    } catch (err) {
      setError(err.message)
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
          <Alert message={error} />
          {!selected ? (
            <div className="placeholder">
              <p>Create a workspace (your product) to get started.</p>
            </div>
          ) : (
            <>
              <div className="main-head">
                <h2>{selected.name}</h2>
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
    </div>
  )
}
