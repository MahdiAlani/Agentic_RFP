import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import {
  listDocuments,
  createDocument,
  uploadToStorage,
  confirmDocument,
  getDownloadURL,
} from '../api'

const DOC_TYPES = ['rfp', 'reference', 'proposal', 'other']

export default function DocumentsPanel({ workspace }) {
  const [documents, setDocuments] = useState([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [docType, setDocType] = useState(DOC_TYPES[0])
  const fileRef = useRef(null)

  function refresh() {
    setLoading(true)
    listDocuments(workspace.id)
      .then((ds) => setDocuments(ds || []))
      .catch((err) => toast.error(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(refresh, [workspace.id])

  async function handleUpload(e) {
    e.preventDefault()
    const file = fileRef.current?.files?.[0]
    if (!file) {
      toast.error('Choose a file first')
      return
    }
    setUploading(true)
    try {
      // 1. create metadata + get presigned PUT url
      const doc = await createDocument({
        workspaceID: workspace.id,
        fileName: file.name,
        documentType: docType,
      })
      // 2. upload bytes directly to storage
      await uploadToStorage(doc.upload_url, file)
      // 3. confirm -> backend verifies + queues embedding
      await confirmDocument(doc.id)

      toast.success(`Uploaded ${file.name}`)
      if (fileRef.current) fileRef.current.value = ''
      refresh()
    } catch (err) {
      toast.error(err.message)
    } finally {
      setUploading(false)
    }
  }

  async function handleDownload(id) {
    try {
      const { download_url } = await getDownloadURL(id)
      window.open(download_url, '_blank')
    } catch (err) {
      toast.error(err.message)
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
