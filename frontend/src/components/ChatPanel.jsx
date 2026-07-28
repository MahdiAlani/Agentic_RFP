import { useEffect, useRef, useState } from 'react'
import {
  listConversations,
  createConversation,
  listMessages,
  sendMessage,
} from '../api'
import Alert from './Alert.jsx'

export default function ChatPanel({ project }) {
  const [conversations, setConversations] = useState([])
  const [activeId, setActiveId] = useState(null)
  const [messages, setMessages] = useState([])
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const bottomRef = useRef(null)

  // Load (or create) conversations for this project.
  useEffect(() => {
    listConversations(project.id)
      .then(async (cs) => {
        let list = cs || []
        if (list.length === 0) {
          list = [await createConversation(project.id, 'New chat')]
        }
        setConversations(list)
        setActiveId(list[0].id)
      })
      .catch((err) => setError(err.message))
  }, [project.id])

  function loadMessages(id) {
    listMessages(id)
      .then((ms) => setMessages(ms || []))
      .catch((err) => setError(err.message))
  }

  useEffect(() => {
    if (activeId) loadMessages(activeId)
  }, [activeId])

  // Poll while the assistant reply is still being generated.
  useEffect(() => {
    if (!activeId) return
    const pending = messages.some((m) => m.status === 'pending')
    if (!pending) return
    const t = setInterval(() => loadMessages(activeId), 3000)
    return () => clearInterval(t)
  }, [messages, activeId])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  async function handleSend(e) {
    e.preventDefault()
    const content = input.trim()
    if (!content || !activeId) return
    setError('')
    setSending(true)
    try {
      const { user, assistant } = await sendMessage(activeId, content)
      setMessages((prev) => [...prev, user, assistant])
      setInput('')
    } catch (err) {
      setError(err.message)
    } finally {
      setSending(false)
    }
  }

  async function handleNewChat() {
    setError('')
    try {
      const c = await createConversation(project.id, 'New chat')
      setConversations((prev) => [c, ...prev])
      setActiveId(c.id)
      setMessages([])
    } catch (err) {
      setError(err.message)
    }
  }

  return (
    <div className="chat">
      <div className="chat-sidebar">
        <button className="primary full" onClick={handleNewChat}>
          + New chat
        </button>
        <ul className="conv-list">
          {conversations.map((c) => (
            <li key={c.id}>
              <button
                className={activeId === c.id ? 'conv active' : 'conv'}
                onClick={() => setActiveId(c.id)}
              >
                {c.title}
              </button>
            </li>
          ))}
        </ul>
      </div>

      <div className="chat-main">
        <Alert message={error} />
        <div className="messages">
          {messages.length === 0 ? (
            <p className="muted chat-empty">
              Ask anything about this RFP — grounded in your workspace knowledge base.
            </p>
          ) : (
            messages.map((m) => (
              <div key={m.id} className={`msg ${m.role}`}>
                {m.role === 'assistant' && m.status === 'pending' ? (
                  <span className="muted">Thinking…</span>
                ) : (
                  m.content
                )}
              </div>
            ))
          )}
          <div ref={bottomRef} />
        </div>

        <form className="chat-input" onSubmit={handleSend}>
          <input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Message…"
          />
          <button className="primary" type="submit" disabled={sending}>
            {sending ? '…' : 'Send'}
          </button>
        </form>
      </div>
    </div>
  )
}
