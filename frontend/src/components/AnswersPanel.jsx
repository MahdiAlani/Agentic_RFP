import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { listAnswers, createAnswer } from '../api'

export default function AnswersPanel({ workspace }) {
  const [answers, setAnswers] = useState([])
  const [loading, setLoading] = useState(true)
  const [question, setQuestion] = useState('')
  const [asking, setAsking] = useState(false)

  function refresh() {
    setLoading(true)
    listAnswers(workspace.id)
      .then((as) => setAnswers(as || []))
      .catch((err) => toast.error(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(refresh, [workspace.id])

  // Poll while anything is still pending so answers update once workers finish.
  useEffect(() => {
    const anyPending = answers.some((a) => a.status === 'pending')
    if (!anyPending) return
    const t = setInterval(refresh, 4000)
    return () => clearInterval(t)
  }, [answers])

  async function handleAsk(e) {
    e.preventDefault()
    const q = question.trim()
    if (!q) return
    setAsking(true)
    try {
      const a = await createAnswer({ workspaceID: workspace.id, question: q })
      setAnswers((prev) => [a, ...prev])
      setQuestion('')
      toast.success('Question submitted')
    } catch (err) {
      toast.error(err.message)
    } finally {
      setAsking(false)
    }
  }

  return (
    <section>
      <form className="ask-bar" onSubmit={handleAsk}>
        <input
          type="text"
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          placeholder="Ask a question about this workspace's documents…"
        />
        <button className="primary" type="submit" disabled={asking}>
          {asking ? 'Asking…' : 'Ask'}
        </button>
      </form>

      {loading ? (
        <p className="muted">Loading…</p>
      ) : answers.length === 0 ? (
        <div className="placeholder">
          <p>No questions yet.</p>
        </div>
      ) : (
        <ul className="answer-list">
          {answers.map((a) => (
            <li key={a.id} className="answer">
              <div className="answer-q">
                <span>{a.question}</span>
                <span className={`badge ${a.status}`}>{a.status}</span>
              </div>
              {a.answer_text ? (
                <p className="answer-a">{a.answer_text}</p>
              ) : (
                <p className="muted small">
                  {a.status === 'pending' ? 'Generating…' : 'No answer yet.'}
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
