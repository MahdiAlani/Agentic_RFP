import { useEffect, useRef, useState } from 'react'
import {
  listDocuments,
  listProjectDocuments,
  createDocument,
  uploadToStorage,
  confirmDocument,
  getDownloadURL,
  renameDocument,
  deleteDocument,
} from '../api'
import Alert from './Alert.jsx'
import InlineEdit from './InlineEdit.jsx'
import ConfirmDialog from './ConfirmDialog.jsx'
import { PencilIcon, TrashIcon, DownloadIcon } from './icons.jsx'

// Only the RFP/not-RFP distinction is meaningful: an RFP is what questions are
// extracted from, and it is excluded from answer retrieval. Everything else is
// just supporting material.
const DOC_TYPES = ['rfp', 'other']

// 'rfp' is an acronym, so it reads as RFP rather than Rfp.
function typeLabel(type) {
  if (type === 'rfp') return 'RFP'
  return type.charAt(0).toUpperCase() + type.slice(1)
}

// If `project` is provided, documents are scoped to that project; otherwise to
// the workspace (the product knowledge base).
export default function DocumentsPanel({ workspace, project }) {
  const [documents, setDocuments] = useState([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  // Workspace documents are always supporting material; only a project needs to
  // distinguish the RFP itself.
  const [docType, setDocType] = useState(project ? 'rfp' : 'other')
  const [error, setError] = useState('')
  const [renamingId, setRenamingId] = useState(null)
  const [pendingDelete, setPendingDelete] = useState(null)
  const fileRef = useRef(null)

  function refresh() {
    setLoading(true)
    setError('')
    const load = project
      ? listProjectDocuments(project.id)
      : listDocuments(workspace.id)
    load
      .then((ds) => setDocuments(ds || []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(refresh, [workspace.id, project?.id])

  async function handleUpload(e) {
    e.preventDefault()
    setError('')
    const file = fileRef.current?.files?.[0]
    if (!file) {
      setError('Choose a file first.')
      return
    }
    setUploading(true)
    try {
      // 1. create metadata + get presigned PUT url
      const doc = await createDocument({
        workspaceID: workspace.id,
        projectID: project?.id,
        fileName: file.name,
        documentType: docType,
      })
      // 2. upload bytes directly to storage
      await uploadToStorage(doc.upload_url, file)
      // 3. confirm -> backend verifies + queues embedding
      await confirmDocument(doc.id)

      if (fileRef.current) fileRef.current.value = ''
      refresh()
    } catch (err) {
      setError(err.message)
    } finally {
      setUploading(false)
    }
  }

  async function handleDownload(id) {
    setError('')
    try {
      const { download_url } = await getDownloadURL(id)
      window.open(download_url, '_blank')
    } catch (err) {
      setError(err.message)
    }
  }

  async function handleRename(id, fileName) {
    setError('')
    try {
      const updated = await renameDocument(id, fileName)
      setDocuments((prev) => prev.map((d) => (d.id === updated.id ? updated : d)))
      setRenamingId(null)
    } catch (err) {
      setError(err.message)
    }
  }

  async function handleDelete() {
    setError('')
    try {
      await deleteDocument(pendingDelete.id)
      setDocuments((prev) => prev.filter((d) => d.id !== pendingDelete.id))
    } catch (err) {
      setError(err.message)
    } finally {
      setPendingDelete(null)
    }
  }

  return (
    <section>
      <form className="upload-bar" onSubmit={handleUpload}>
        <input ref={fileRef} type="file" className="file-input" />
        {project && (
          <select value={docType} onChange={(e) => setDocType(e.target.value)}>
            {DOC_TYPES.map((t) => (
              <option key={t} value={t}>
                {typeLabel(t)}
              </option>
            ))}
          </select>
        )}
        <button className="primary" type="submit" disabled={uploading}>
          {uploading ? 'Uploading…' : 'Upload'}
        </button>
      </form>

      <Alert message={error} />

      {loading ? (
        <p className="muted">Loading…</p>
      ) : documents.length === 0 ? (
        <div className="placeholder">
          <p>No documents yet. Upload one to get started.</p>
        </div>
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>File</th>
              {project && <th>Type</th>}
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {documents.map((d) => (
              <tr key={d.id}>
                <td>
                  {renamingId === d.id ? (
                    <InlineEdit
                      value={d.file_name}
                      onSave={(name) => handleRename(d.id, name)}
                      onCancel={() => setRenamingId(null)}
                    />
                  ) : (
                    d.file_name
                  )}
                </td>
                {project && <td className="muted">{typeLabel(d.document_type)}</td>}
                <td>
                  <span className={`badge ${d.status}`}>{d.status}</span>
                </td>
                <td className="right">
                  <div className="row-actions end">
                    <button
                      className="icon-btn"
                      title="Download"
                      onClick={() => handleDownload(d.id)}
                    >
                      <DownloadIcon />
                    </button>
                    <button
                      className="icon-btn"
                      title="Rename"
                      onClick={() => setRenamingId(d.id)}
                    >
                      <PencilIcon />
                    </button>
                    <button
                      className="icon-btn danger"
                      title="Delete"
                      onClick={() => setPendingDelete(d)}
                    >
                      <TrashIcon />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <ConfirmDialog
        open={Boolean(pendingDelete)}
        title={`Delete "${pendingDelete?.file_name}"?`}
        message="The file and its embedded chunks will be permanently deleted."
        onConfirm={handleDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </section>
  )
}
