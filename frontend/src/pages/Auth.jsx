import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { login, register } from '../api'
import { setSession } from '../auth'
import Alert from '../components/Alert.jsx'

const EMAIL_RE = /^\S+@\S+\.\S+$/

export default function Auth() {
  const navigate = useNavigate()
  const [mode, setMode] = useState('signin') // 'signin' | 'signup'
  const [form, setForm] = useState({ name: '', email: '', password: '' })
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const isSignup = mode === 'signup'

  const update = (field) => (e) => {
    setForm({ ...form, [field]: e.target.value })
    if (error) setError('')
  }

  function switchMode() {
    setMode(isSignup ? 'signin' : 'signup')
    setError('')
  }

  // Returns the first validation problem, or '' if valid.
  function validate() {
    if (isSignup && !form.name.trim()) return 'Enter your name.'
    if (!form.email.trim()) return 'Enter your email.'
    if (!EMAIL_RE.test(form.email)) return 'Enter a valid email address.'
    if (!form.password) return 'Enter your password.'
    if (isSignup && form.password.length < 8)
      return 'Password must be at least 8 characters.'
    return ''
  }

  async function handleSubmit(e) {
    e.preventDefault()
    const problem = validate()
    if (problem) {
      setError(problem)
      return
    }
    setError('')
    setLoading(true)
    try {
      const { token, user } = isSignup ? await register(form) : await login(form)
      setSession({ token, user })
      navigate('/')
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="shell">
      <aside className="panel">
        <div className="panel-top">
          <span className="mark">Agentic&nbsp;RFP</span>
        </div>
        <div className="panel-body">
          <h2>
            Answer every RFP<br />in a fraction of the time.
          </h2>
          <p>
            Upload your source documents once. Let the agent draft grounded,
            citable answers to each question.
          </p>
        </div>
        <div className="panel-foot">© {new Date().getFullYear()} Agentic RFP</div>
      </aside>

      <main className="content">
        <div className="form-wrap">
          <header className="form-head">
            <h1>{isSignup ? 'Create an account' : 'Sign in'}</h1>
            <p>
              {isSignup ? 'Already have one? ' : 'New here? '}
              <button type="button" className="link" onClick={switchMode}>
                {isSignup ? 'Sign in' : 'Create an account'}
              </button>
            </p>
          </header>

          <form onSubmit={handleSubmit} className="form" noValidate>
            {isSignup && (
              <label className="field">
                <span>Name</span>
                <input type="text" value={form.name} onChange={update('name')} />
              </label>
            )}

            <label className="field">
              <span>Email</span>
              <input type="email" value={form.email} onChange={update('email')} />
            </label>

            <label className="field">
              <span>Password</span>
              <input type="password" value={form.password} onChange={update('password')} />
            </label>

            <Alert message={error} />

            <button className="submit" type="submit" disabled={loading || Boolean(error)}>
              {loading ? 'Please wait…' : isSignup ? 'Create account' : 'Sign in'}
            </button>
          </form>
        </div>
      </main>
    </div>
  )
}
