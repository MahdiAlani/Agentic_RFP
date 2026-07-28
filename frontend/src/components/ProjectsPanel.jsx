import { useEffect, useState } from 'react'
import { listProjects, createProject } from '../api'
import Alert from './Alert.jsx'

export default function ProjectsPanel({ workspace, onOpen }) {
  const [projects, setProjects] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoading(true)
    setError('')
    listProjects(workspace.id)
      .then((ps) => setProjects(ps || []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [workspace.id])

  async function handleNew() {
    const name = window.prompt('Project name')
    if (!name || !name.trim()) return
    setError('')
    try {
      const p = await createProject({ workspaceID: workspace.id, name: name.trim() })
      setProjects((prev) => [...prev, p])
    } catch (err) {
      setError(err.message)
    }
  }

  return (
    <section>
      <div className="panel-bar">
        <p className="muted">Group related documents and questions.</p>
        <button className="primary" onClick={handleNew}>
          New project
        </button>
      </div>

      <Alert message={error} />

      {loading ? (
        <p className="muted">Loading…</p>
      ) : projects.length === 0 ? (
        <div className="placeholder">
          <p>No projects yet.</p>
        </div>
      ) : (
        <div className="grid">
          {projects.map((p) => (
            <button
              key={p.id}
              className="card card-btn"
              onClick={() => onOpen?.(p)}
            >
              <h4>{p.name}</h4>
              <span className="muted small">
                {new Date(p.created_at).toLocaleDateString()}
              </span>
            </button>
          ))}
        </div>
      )}
    </section>
  )
}
