import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { signIn, signUp } from '../api'
import { setUser } from '../auth'

export default function Auth() {
  const navigate = useNavigate()
  const [mode, setMode] = useState('signin') // 'signin' | 'signup'
  const [form, setForm] = useState({ name: '', email: '', password: '' })
  const [loading, setLoading] = useState(false)

  const isSignup = mode === 'signup'
  const update = (field) => (e) => setForm({ ...form, [field]: e.target.value })

  async function handleSubmit(e) {
    e.preventDefault()
    setLoading(true)
    try {
      const user = isSignup ? await signUp(form) : await signIn(form)
      setUser(user)
      toast.success(isSignup ? 'Account created' : `Welcome back, ${user.name || user.email}`)
      navigate('/')
    } catch (err) {
      toast.error(err.message)
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
              <button
                type="button"
                className="link"
                onClick={() => setMode(isSignup ? 'signin' : 'signup')}
              >
                {isSignup ? 'Sign in' : 'Create an account'}
              </button>
            </p>
          </header>

          <form onSubmit={handleSubmit} className="form">
            {isSignup && (
              <label className="field">
                <span>Name</span>
                <input type="text" value={form.name} onChange={update('name')} required />
              </label>
            )}

            <label className="field">
              <span>Email</span>
              <input type="email" value={form.email} onChange={update('email')} required />
            </label>

            <label className="field">
              <span>Password</span>
              <input
                type="password"
                value={form.password}
                onChange={update('password')}
                minLength={isSignup ? 8 : undefined}
                required
              />
            </label>

            <button className="submit" type="submit" disabled={loading}>
              {loading ? 'Please wait…' : isSignup ? 'Create account' : 'Sign in'}
            </button>
          </form>
        </div>
      </main>
    </div>
  )
}
