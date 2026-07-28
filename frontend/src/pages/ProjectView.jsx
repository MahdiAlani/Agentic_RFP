import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getProject } from '../api'
import ChatPanel from '../components/ChatPanel.jsx'
import DocumentsPanel from '../components/DocumentsPanel.jsx'
import Alert from '../components/Alert.jsx'

const TABS = [
  { key: 'chat', label: 'Chat' },
  { key: 'documents', label: 'RFP & documents' },
]

export default function ProjectView() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [project, setProject] = useState(null)
  const [tab, setTab] = useState('chat')
  const [error, setError] = useState('')

  useEffect(() => {
    getProject(id)
      .then(setProject)
      .catch((err) => setError(err.message))
  }, [id])

  return (
    <div className="dash">
      <header className="topbar">
        <div className="topbar-left">
          <button className="ghost" onClick={() => navigate('/')}>
            ← Workspace
          </button>
          <span className="mark">{project ? project.name : 'Loading…'}</span>
        </div>
      </header>

      <main className="main">
        <Alert message={error} />
        {!project ? (
          error ? null : <p className="muted">Loading…</p>
        ) : (
          <>
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

            {tab === 'chat' && <ChatPanel project={project} />}
            {tab === 'documents' && (
              <DocumentsPanel
                workspace={{ id: project.workspace_id }}
                project={project}
              />
            )}
          </>
        )}
      </main>
    </div>
  )
}
