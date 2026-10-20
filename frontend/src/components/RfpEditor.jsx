import { useEffect, useRef, useState } from 'react'
import {
  listProjectAnswers,
  listProjectDocuments,
  createAnswer,
  updateAnswer,
  regenerateAnswer,
  approveAnswer,
  deleteAnswer,
  fillProjectAnswers,
  extractQuestions,
  exportProject,
} from '../api'
import Alert from './Alert.jsx'
import InlineEdit from './InlineEdit.jsx'
import ConfirmDialog from './ConfirmDialog.jsx'
import { TrashIcon, PlusIcon } from './icons.jsx'

// The working area for one RFP: every extracted question, its draft answer,
// and the citations that grounded it.
export default function RfpEditor({ project }) {
  const [answers, setAnswers] = useState([])
  const [documents, setDocuments] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [adding, setAdding] = useState(false)
  const [pendingDelete, setPendingDelete] = useState(null)

  function loadAnswers() {
    return listProjectAnswers(project.id)
      .then((as) => setAnswers(as || []))
      .catch((err) => setError(err.message))
  }

  useEffect(() => {
    setLoading(true)
    setError('')
    Promise.all([
      loadAnswers(),
      listProjectDocuments(project.id)
        .then((ds) => setDocuments(ds || []))
        .catch(() => {}),
    ]).finally(() => setLoading(false))
  }, [project.id])

  // Poll while any question is being answered.
  useEffect(() => {
    if (!answers.some((a) => a.status === 'pending')) return
    const t = setInterval(loadAnswers, 3000)
    return () => clearInterval(t)
  }, [answers, project.id])

  const rfpDocs = documents.filter((d) => d.document_type === 'rfp')
  const answered = answers.filter((a) => a.status === 'done' || a.status === 'approved').length
  const approved = answers.filter((a) => a.status === 'approved').length

  async function handleExtract(documentID) {
    setError('')
    setNotice('')
    setBusy(true)
    try {
      await extractQuestions({
        projectID: project.id,
        workspaceID: project.workspace_id,
        documentID,
      })
      setNotice('Reading the RFP - questions will appear here shortly.')
      // Extraction happens in a worker; poll a few times for the results.
      let tries = 0
      const t = setInterval(async () => {
        await loadAnswers()
        if (++tries >= 10) clearInterval(t)
      }, 3000)
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  async function handleFillAll() {
    setError('')
    setNotice('')
    setBusy(true)
    try {
      const { queued } = await fillProjectAnswers(project.id)
      setNotice(
        queued === 0
          ? 'Nothing to fill - every question already has an answer.'
          : `Drafting ${queued} answer${queued === 1 ? '' : 's'}...`,
      )
      await loadAnswers()
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  async function handleAddQuestion(question) {
    setError('')
    try {
      await createAnswer({
        workspaceID: project.workspace_id,
        projectID: project.id,
        question,
      })
      setAdding(false)
      await loadAnswers()
    } catch (err) {
      setError(err.message)
    }
  }

  // Runs an answer-changing API call and swaps the returned row into the list.
  async function updateRow(call) {
    setError('')
    try {
      const updated = await call()
      setAnswers((prev) => prev.map((a) => (a.id === updated.id ? { ...a, ...updated } : a)))
    } catch (err) {
      setError(err.message)
    }
  }

  async function handleDelete() {
    setError('')
    try {
      await deleteAnswer(pendingDelete.id)
      setAnswers((prev) => prev.filter((a) => a.id !== pendingDelete.id))
    } catch (err) {
      setError(err.message)
    } finally {
      setPendingDelete(null)
    }
  }

  async function handleExport() {
    setError('')
    try {
      await exportProject(project.id)
    } catch (err) {
      setError(err.message)
    }
  }

  if (loading) return <p className="muted">Loading…</p>

  return (
    <section className="editor">
      <div className="editor-bar">
        <div className="editor-stats">
          {answers.length > 0 && (
            <span className="muted small">
              {answered} of {answers.length} answered · {approved} approved
            </span>
          )}
        </div>
        <div className="editor-actions">
          {rfpDocs.length > 0 && (
            <select
              className="extract-select"
              defaultValue=""
              disabled={busy}
              onChange={(e) => {
                if (e.target.value) handleExtract(e.target.value)
                e.target.value = ''
              }}
            >
              <option value="">Extract questions from…</option>
              {rfpDocs.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.file_name}
                </option>
              ))}
            </select>
          )}
          {answers.length > 0 && (
            <>
              <button className="ghost" onClick={handleFillAll} disabled={busy}>
                Draft all with AI
              </button>
              <button className="ghost" onClick={handleExport}>
                Export to Word
              </button>
            </>
          )}
        </div>
      </div>

      <Alert message={error} />
      <Alert message={notice} type="success" />

      {answers.length === 0 ? (
        <div className="placeholder">
          {rfpDocs.length === 0 ? (
            <p>
              Upload the RFP below with type <strong>RFP</strong>, then extract its
              questions here.
            </p>
          ) : (
            <p>
              No questions yet. Use <strong>Extract questions from…</strong> to read
              the RFP, or add one by hand.
            </p>
          )}
        </div>
      ) : (
        <ol className="qa-list">
          {answers.map((a) => (
            <QuestionRow
              key={a.id}
              answer={a}
              onSave={(text) => updateRow(() => updateAnswer(a.id, text))}
              onRegenerate={() => updateRow(() => regenerateAnswer(a.id))}
              onApprove={() => updateRow(() => approveAnswer(a.id))}
              onDelete={() => setPendingDelete(a)}
            />
          ))}
        </ol>
      )}

      {adding ? (
        <InlineEdit
          placeholder="Question text"
          onSave={handleAddQuestion}
          onCancel={() => setAdding(false)}
        />
      ) : (
        <button className="ghost with-icon add-question" onClick={() => setAdding(true)}>
          <PlusIcon />
          Add question
        </button>
      )}

      <ConfirmDialog
        open={Boolean(pendingDelete)}
        title="Delete this question?"
        message={pendingDelete?.question}
        onConfirm={handleDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </section>
  )
}

// A finished answer with no sources wasn't covered by the knowledge base, so a
// person has to supply it.
function statusOf(answer) {
  if (answer.status === 'done' && !answer.citations?.length) return 'needs-input'
  return answer.status
}

const STATUS_LABELS = { 'needs-input': 'needs input' }

// One question: the prompt, an editable answer, citations and per-row actions.
function QuestionRow({ answer, onSave, onRegenerate, onApprove, onDelete }) {
  const [draft, setDraft] = useState(answer.answer_text || '')
  const [dirty, setDirty] = useState(false)
  const [showSources, setShowSources] = useState(false)
  const taRef = useRef(null)

  // Adopt worker-generated text, unless the user is mid-edit.
  useEffect(() => {
    if (!dirty) setDraft(answer.answer_text || '')
  }, [answer.answer_text, dirty])

  // Grow the textarea to fit its content.
  useEffect(() => {
    const ta = taRef.current
    if (!ta) return
    ta.style.height = 'auto'
    ta.style.height = `${ta.scrollHeight}px`
  }, [draft])

  const pending = answer.status === 'pending'
  const status = statusOf(answer)
  const citations = answer.citations || []

  function save() {
    if (!dirty || !draft.trim()) return
    onSave(draft)
    setDirty(false)
  }

  return (
    <li className="qa-item">
      <div className="qa-question">
        <p>{answer.question}</p>
        <div className="row-actions">
          <span className={`badge ${status}`}>{STATUS_LABELS[status] || status}</span>
          <button className="icon-btn danger" title="Delete question" onClick={onDelete}>
            <TrashIcon />
          </button>
        </div>
      </div>

      {pending ? (
        <p className="muted qa-thinking">Drafting…</p>
      ) : (
        <textarea
          ref={taRef}
          className="qa-answer"
          value={draft}
          placeholder="No answer yet - draft it with AI or write your own."
          onChange={(e) => {
            setDraft(e.target.value)
            setDirty(true)
          }}
          onBlur={save}
        />
      )}

      <div className="qa-foot">
        {citations.length > 0 && (
          <button className="link" onClick={() => setShowSources((v) => !v)}>
            {showSources ? 'Hide' : 'Show'} {citations.length} source
            {citations.length === 1 ? '' : 's'}
          </button>
        )}
        <div className="qa-foot-actions">
          {dirty && (
            <button className="link" onClick={save}>
              Save
            </button>
          )}
          <button className="link" onClick={onRegenerate} disabled={pending}>
            {answer.answer_text ? 'Redraft with AI' : 'Draft with AI'}
          </button>
          {answer.status === 'done' && !dirty && (
            <button className="link" onClick={onApprove}>
              Approve
            </button>
          )}
        </div>
      </div>

      {showSources && (
        <ol className="qa-sources">
          {citations.map((c) => (
            <li key={c.position}>
              <span className="qa-source-ref">
                [{c.position}] {c.file_name}
              </span>
              <p>{c.content || 'Passage no longer available (document re-indexed).'}</p>
            </li>
          ))}
        </ol>
      )}
    </li>
  )
}
