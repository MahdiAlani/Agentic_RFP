import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { listWorkspaces, createWorkspace } from '../api'
import { getUser, clearSession } from '../auth'
import ProjectsPanel from '../components/ProjectsPanel.jsx'
import DocumentsPanel from '../components/DocumentsPanel.jsx'
import AnswersPanel from '../components/AnswersPanel.jsx'

const TABS = [
  { key: 'documents', label: 'Documents' },
  { key: 'answers', label: 'Q&A' },
  { key: 'projects', label: 'Projects' },
]

export default function Dashboard() {
  const navigate = useNavigate()
  const user = getUser()

  const [workspaces, setWorkspaces] = useState([])
  const [selected, setSelected] = useState(null)
  const [tab, setTab] = useState('documents')

  useEffect(() => {
    listWorkspaces()
      .then((ws) => {
        const list = ws || []
        setWorkspaces(list)
        if (list.length) setSelected(list[0])
      })
      .catch((err) => toast.error(err.message))
  }, [])

  async function handleNewWorkspace() {
    const name = window.prompt('Workspace name')
    if (!name || !name.trim()) return
    try {
      const ws = await createWorkspace({ name: name.trim() })
      setWorkspaces((prev) => [...prev, ws])
      setSelected(ws)
      toast.success(`Created “${ws.name}”`)
    } catch (err) {
      toast.error(err.message)
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
          {!selected ? (
            <div className="placeholder">
              <p>Create a workspace to get started.</p>
            </div>
          ) : (
            <>
              <div className="main-head">
                <h2>{selected.name}</h2>
              </div>

              <nav className="tabs-row">
                {TABS.map((t) => (
                  <button
                    key={t.key}
                    className={tab === t.key ? 'tab-btn active' : 'tab-btn'}
                    onClick={() => setTab(t.key)}
                  >
                    {t.label}
                  </button>
                ))}
              </nav>

              {tab === 'documents' && <DocumentsPanel workspace={selected} />}
              {tab === 'answers' && <AnswersPanel workspace={selected} />}
              {tab === 'projects' && <ProjectsPanel workspace={selected} />}
            </>
          )}
        </main>
      </div>
    </div>
  )
}
