import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getProject } from '../api'
import ChatPanel from '../components/ChatPanel.jsx'
import DocumentsPanel from '../components/DocumentsPanel.jsx'
import RfpEditor from '../components/RfpEditor.jsx'
import Alert from '../components/Alert.jsx'

export default function ProjectView() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [project, setProject] = useState(null)
  const [error, setError] = useState('')
  const [chatOpen, setChatOpen] = useState(true)
  const [docsOpen, setDocsOpen] = useState(false)

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
        <div className="topbar-right">
          <button className="ghost" onClick={() => setChatOpen((v) => !v)}>
            {chatOpen ? 'Hide chat' : 'Show chat'}
          </button>
        </div>
      </header>

      <div className={chatOpen ? 'project-body with-chat' : 'project-body'}>
        <main className="project-main">
          <Alert message={error} />
          {!project ? (
            error ? null : <p className="muted">Loading…</p>
          ) : (
            <>
              <RfpEditor project={project} />

              <section className="section">
                <button
                  className="disclosure"
                  onClick={() => setDocsOpen((v) => !v)}
                  aria-expanded={docsOpen}
                >
                  <span className="disclosure-caret">{docsOpen ? '▾' : '▸'}</span>
                  RFP &amp; supporting documents
                </button>
                {docsOpen && (
                  <>
                    <p className="muted section-sub">
                      Upload the RFP itself as type <strong>RFP</strong>; supporting
                      material (requirements, references) as <strong>Other</strong>.
                    </p>
                    <DocumentsPanel
                      workspace={{ id: project.workspace_id }}
                      project={project}
                    />
                  </>
                )}
              </section>
            </>
          )}
        </main>

        {chatOpen && project && (
          <aside className="project-chat">
            <div className="project-chat-head">
              <h3>Chat</h3>
              <p className="muted small">
                Ask anything - grounded in your workspace knowledge base.
              </p>
            </div>
            <ChatPanel project={project} />
          </aside>
        )}
      </div>
    </div>
  )
}
