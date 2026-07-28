// Reusable inline alert. Renders nothing when there's no message.
export default function Alert({ message, type = 'error' }) {
  if (!message) return null
  return <div className={`alert-inline ${type}`}>{message}</div>
}
