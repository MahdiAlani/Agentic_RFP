import { useEffect, useRef, useState } from 'react'
import {
  listDocuments,
  listProjectDocuments,
  createDocument,
  uploadToStorage,
  confirmDocument,
  getDownloadURL,
} from '../api'
import Alert from './Alert.jsx'

const DOC_TYPES = ['rfp', 'reference', 'proposal', 'other']

// If `project` is provided, documents are scoped to that project; otherwise to
// the workspace (the product knowledge base).
export default function DocumentsPanel({ workspace, project }) {
  const [documents, setDocuments] = useState([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [docType, setDocType] = useState(DOC_TYPES[0])
  const [error, setError] = useState('')
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

  return (
    <section>
      <form className="upload-bar" onSubmit={handleUpload}>
        <input ref={fileRef} type="file" className="file-input" />
        <select value={docType} onChange={(e) => setDocType(e.target.value)}>
          {DOC_TYPES.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
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
              <th>Type</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {documents.map((d) => (
              <tr key={d.id}>
                <td>{d.file_name}</td>
                <td className="muted">{d.document_type}</td>
                <td>
                  <span className={`badge ${d.status}`}>{d.status}</span>
                </td>
                <td className="right">
                  <button className="link" onClick={() => handleDownload(d.id)}>
                    Download
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}
