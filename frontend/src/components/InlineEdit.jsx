import { useEffect, useRef, useState } from 'react'
import { CheckIcon, CloseIcon } from './icons.jsx'

// A single-field inline form used for both renaming and creating.
// Enter saves, Escape cancels; an empty (or unchanged) value just cancels.
export default function InlineEdit({
  value = '',
  placeholder = '',
  onSave,
  onCancel,
  className = '',
}) {
  const [draft, setDraft] = useState(value)
  const inputRef = useRef(null)

  useEffect(() => {
    inputRef.current?.focus()
    inputRef.current?.select()
  }, [])

  function submit(e) {
    e?.preventDefault()
    e?.stopPropagation()
    const next = draft.trim()
    if (!next || next === value.trim()) {
      onCancel?.()
      return
    }
    onSave(next)
  }

  return (
    <form className={`inline-edit ${className}`} onSubmit={submit}>
      <input
        ref={inputRef}
        value={draft}
        placeholder={placeholder}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Escape') {
            e.preventDefault()
            onCancel?.()
          }
        }}
      />
      <button className="icon-btn" type="submit" title="Save">
        <CheckIcon />
      </button>
      <button
        className="icon-btn"
        type="button"
        title="Cancel"
        onClick={() => onCancel?.()}
      >
        <CloseIcon />
      </button>
    </form>
  )
}
