import { useState } from 'react'
import type { FormEvent } from 'react'
import { mutate, setSession } from '../api'
import type { User } from '../api'
import { Icon } from './Icon'
import { Neighbourhood } from './Neighbourhood'

const demoPassword = 'Community-preview-2026!'
const accounts = [
  { id: 'admin', label: 'Registry officer', detail: 'Manage homes & people' },
  { id: 'committee', label: 'Committee', detail: 'View the community' },
  { id: 'owner', label: 'Owner', detail: 'View two own homes' },
  { id: 'tenant', label: 'Tenant', detail: 'View one rented home' },
]

export function Login({ onLogin, message = '' }: { onLogin: (user: User) => void; message?: string }) {
  const [login, setLogin] = useState('admin@demo.society')
  const [password, setPassword] = useState(demoPassword)
  const [showPassword, setShowPassword] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(message)
  const [help, setHelp] = useState(false)
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setBusy(true); setError('')
    try { const user = await mutate<User>('/api/auth/login', 'POST', { login, password }); setSession(user); onLogin(user) }
    catch (err) { setError((err as Error).message) }
    finally { setBusy(false) }
  }
  return <main className="login-shell">
    <section className="login-story" aria-labelledby="welcome-title">
      <a href="#overview" className="login-wordmark">society<span>.</span></a>
      <div className="login-story-copy"><span className="hero-eyebrow">A PLACE FOR EVERY HOME</span><h1 id="welcome-title">Good neighbours.<br /><em>Better together.</em></h1><p>Your homes, your people and the everyday details.<br />All in a place that feels like home.</p></div>
      <Neighbourhood className="login-illustration" />
      <span className="login-story-caption">118 HOMES · ONE COMMUNITY</span>
    </section>
    <section className="login-panel" aria-labelledby="login-title">
      <span className="eyebrow">MAKE YOURSELF AT HOME</span><h2 id="login-title">Welcome <em>back.</em></h2><p className="login-intro">Sign in to your community workspace.</p>
      <form onSubmit={submit} className="portal-form">
        <label>Email address<input type="email" name="email" autoComplete="username" value={login} onChange={e => setLogin(e.target.value)} required maxLength={254} /></label>
        <label><span id="password-label">Password</span><span className="password-control"><input aria-labelledby="password-label" type={showPassword ? 'text' : 'password'} name="password" autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)} required maxLength={256} /><button type="button" onClick={() => setShowPassword(v => !v)} aria-label={showPassword ? 'Hide password' : 'Show password'}>{showPassword ? 'Hide' : 'Show'}</button></span></label>
        {error && <p className="form-error" role="alert">{error}</p>}
        <button className="button button-dark login-submit" disabled={busy}>{busy ? 'Opening your workspace…' : 'Sign in'}<Icon name="arrow" /></button>
      </form>
      <button className="text-link login-help" onClick={() => setHelp(v => !v)} aria-expanded={help}>Need access or forgot your password?<Icon name="key" /></button>
      {help && <p className="form-help help-note">Contact your society’s registry officer. After verifying your identity and home relationship, they can give you a personal invitation or password recovery link.</p>}
      <div className="demo-accounts"><div className="demo-heading"><Icon name="spark" /><span>TAKE A LOOK AROUND</span><small>Local preview</small></div><p>Choose a fictional account to explore its access.</p><div className="demo-account-grid">{accounts.map(a => <button key={a.id} type="button" disabled={busy} aria-pressed={login === `${a.id}@demo.society`} onClick={() => { setLogin(`${a.id}@demo.society`); setPassword(demoPassword); setError('') }}><strong>{a.label}</strong><span>{a.detail}</span></button>)}</div></div>
      <p className="login-footnote"><Icon name="leaf" />Fictional people. A working community workspace.</p>
    </section>
  </main>
}
