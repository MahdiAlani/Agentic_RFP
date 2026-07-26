import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { listProjects, createProject } from '../api'

export default function ProjectsPanel({ workspace }) {
  const [projects, setProjects] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    listProjects(workspace.id)
      .then((ps) => setProjects(ps || []))
      .catch((err) => toast.error(err.message))
      .finally(() => setLoading(false))
  }, [workspace.id])

  async function handleNew() {
    const name = window.prompt('Project name')
    if (!name || !name.trim()) return
    try {
      const p = await createProject({ workspaceID: workspace.id, name: name.trim() })
      setProjects((prev) => [...prev, p])
      toast.success(`Created “${p.name}”`)
    } catch (err) {
      toast.error(err.message)
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

      {loading ? (
        <p className="muted">Loading…</p>
      ) : projects.length === 0 ? (
        <div className="placeholder">
          <p>No projects yet.</p>
        </div>
      ) : (
        <div className="grid">
          {projects.map((p) => (
            <div key={p.id} className="card">
              <h4>{p.name}</h4>
              <span className="muted small">
                {new Date(p.created_at).toLocaleDateString()}
              </span>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}
