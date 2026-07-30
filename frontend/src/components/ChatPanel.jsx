import { useEffect, useRef, useState } from 'react'
import {
  listConversations,
  createConversation,
  listMessages,
  sendMessage,
  renameConversation,
  deleteConversation,
} from '../api'
import Alert from './Alert.jsx'
import InlineEdit from './InlineEdit.jsx'
import ConfirmDialog from './ConfirmDialog.jsx'
import { PencilIcon, TrashIcon, PlusIcon } from './icons.jsx'

export default function ChatPanel({ project }) {
  const [conversations, setConversations] = useState([])
  const [activeId, setActiveId] = useState(null)
  const [messages, setMessages] = useState([])
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const [renamingId, setRenamingId] = useState(null)
  const [pendingDelete, setPendingDelete] = useState(null)
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

  async function handleRename(id, title) {
    setError('')
    try {
      const updated = await renameConversation(id, title)
      setConversations((prev) => prev.map((c) => (c.id === updated.id ? updated : c)))
      setRenamingId(null)
    } catch (err) {
      setError(err.message)
    }
  }

  async function handleDelete() {
    setError('')
    try {
      await deleteConversation(pendingDelete.id)
      const remaining = conversations.filter((c) => c.id !== pendingDelete.id)
      setConversations(remaining)
      // If the open chat was deleted, fall back to the next one.
      if (pendingDelete.id === activeId) {
        setActiveId(remaining[0]?.id ?? null)
        setMessages([])
      }
    } catch (err) {
      setError(err.message)
    } finally {
      setPendingDelete(null)
    }
  }

  const active = conversations.find((c) => c.id === activeId)

  return (
    <div className="chat">
      <div className="conv-bar">
        {renamingId ? (
          <InlineEdit
            value={active?.title || ''}
            onSave={(title) => handleRename(renamingId, title)}
            onCancel={() => setRenamingId(null)}
          />
        ) : (
          <>
            <select
              className="conv-select"
              value={activeId || ''}
              onChange={(e) => setActiveId(e.target.value)}
            >
              {conversations.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.title}
                </option>
              ))}
            </select>
            <div className="row-actions">
              <button
                className="icon-btn"
                title="Rename chat"
                disabled={!active}
                onClick={() => setRenamingId(activeId)}
              >
                <PencilIcon />
              </button>
              <button
                className="icon-btn danger"
                title="Delete chat"
                disabled={!active}
                onClick={() => setPendingDelete(active)}
              >
                <TrashIcon />
              </button>
              <button className="icon-btn" title="New chat" onClick={handleNewChat}>
                <PlusIcon />
              </button>
            </div>
          </>
        )}
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

      <ConfirmDialog
        open={Boolean(pendingDelete)}
        title={`Delete "${pendingDelete?.title}"?`}
        message="This chat and all of its messages will be permanently deleted."
        onConfirm={handleDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </div>
  )
}
