import { useEffect, useState } from 'react'
import { listProjects, createProject, renameProject, deleteProject } from '../api'
import Alert from './Alert.jsx'
import InlineEdit from './InlineEdit.jsx'
import ConfirmDialog from './ConfirmDialog.jsx'
import { PencilIcon, TrashIcon, PlusIcon } from './icons.jsx'

export default function ProjectsPanel({ workspace, onOpen }) {
  const [projects, setProjects] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [creating, setCreating] = useState(false)
  const [renamingId, setRenamingId] = useState(null)
  const [pendingDelete, setPendingDelete] = useState(null)

  useEffect(() => {
    setLoading(true)
    setError('')
    listProjects(workspace.id)
      .then((ps) => setProjects(ps || []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [workspace.id])

  async function handleCreate(name) {
    setError('')
    try {
      const p = await createProject({ workspaceID: workspace.id, name })
      setProjects((prev) => [...prev, p])
      setCreating(false)
    } catch (err) {
      setError(err.message)
    }
  }

  async function handleRename(id, name) {
    setError('')
    try {
      const updated = await renameProject(id, name)
      setProjects((prev) => prev.map((p) => (p.id === updated.id ? updated : p)))
      setRenamingId(null)
    } catch (err) {
      setError(err.message)
    }
  }

  async function handleDelete() {
    setError('')
    try {
      await deleteProject(pendingDelete.id)
      setProjects((prev) => prev.filter((p) => p.id !== pendingDelete.id))
    } catch (err) {
      setError(err.message)
    } finally {
      setPendingDelete(null)
    }
  }

  return (
    <section>
      <div className="panel-bar">
        {creating ? (
          <InlineEdit
            placeholder="RFP name"
            onSave={handleCreate}
            onCancel={() => setCreating(false)}
          />
        ) : (
          <button className="primary with-icon" onClick={() => setCreating(true)}>
            <PlusIcon />
            New project
          </button>
        )}
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
            <div key={p.id} className="card">
              {renamingId === p.id ? (
                <InlineEdit
                  value={p.name}
                  onSave={(name) => handleRename(p.id, name)}
                  onCancel={() => setRenamingId(null)}
                />
              ) : (
                <>
                  <button className="card-open" onClick={() => onOpen?.(p)}>
                    <h4>{p.name}</h4>
                    <span className="muted small">
                      {new Date(p.created_at).toLocaleDateString()}
                    </span>
                  </button>
                  <div className="row-actions">
                    <button
                      className="icon-btn"
                      title="Rename project"
                      onClick={() => setRenamingId(p.id)}
                    >
                      <PencilIcon />
                    </button>
                    <button
                      className="icon-btn danger"
                      title="Delete project"
                      onClick={() => setPendingDelete(p)}
                    >
                      <TrashIcon />
                    </button>
                  </div>
                </>
              )}
            </div>
          ))}
        </div>
      )}

      <ConfirmDialog
        open={Boolean(pendingDelete)}
        title={`Delete "${pendingDelete?.name}"?`}
        message="Its chats and documents will be permanently deleted."
        onConfirm={handleDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </section>
  )
}
