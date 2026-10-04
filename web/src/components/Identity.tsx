import { FormSelect } from './FilterSelect'
import { useEffect, useLayoutEffect, useState } from 'react'
import type { FormEvent, ReactNode } from 'react'
import QRCode from 'qrcode'
import { APIError, mutate, request } from '../api'
import type { Account, AccountPage, IssuedLink, LinkDetails, MFAResult, Person, User } from '../api'
import { Icon } from './Icon'
import { Neighbourhood } from './Neighbourhood'
import { PortalDialog } from './PortalDialog'

function IdentityShell({ children }: { children: ReactNode }) {
  return <main className="login-shell identity-shell"><section className="login-story">
    <a href="#overview" className="login-wordmark">society<span>.</span></a>
    <div className="login-story-copy"><span className="hero-eyebrow">A LITTLE PEACE OF MIND</span><h1>Your place.<br /><em>In safe hands.</em></h1><p>A personal welcome. A little extra care.<br />A community you can count on.</p></div>
    <Neighbourhood className="login-illustration" /><span className="login-story-caption">YOUR COMMUNITY · YOUR CONNECTION</span>
  </section><section className="login-panel identity-panel">{children}</section></main>
}

function RecoveryCodes({ codes, onDone, doneLabel = 'Continue to workspace' }: { codes: string[]; onDone: () => void; doneLabel?: string }) {
  const [saved, setSaved] = useState(false)
  const download = () => {
    const url = URL.createObjectURL(new Blob(['Society OS recovery codes\nKeep these private. Each code works once.\n\n' + codes.join('\n') + '\n'], { type: 'text/plain' }))
    const link = document.createElement('a'); link.href = url; link.download = 'society-recovery-codes.txt'; link.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }
  return <div className="recovery-sheet"><span className="security-symbol"><Icon name="key" /></span><span className="eyebrow">YOUR SPARE KEYS</span><h2>Keep these <em>close.</em></h2><p className="form-help">Save these somewhere private, away from this device. Each code can replace an authenticator code once. They will not be shown again.</p>
    <div className="recovery-grid" aria-label="Recovery codes">{codes.map(code => <code key={code}>{code}</code>)}</div>
    <button className="button button-outline" onClick={download}>Download codes<Icon name="arrow" /></button>
    <label className="checkbox-label"><input type="checkbox" checked={saved} onChange={e => setSaved(e.target.checked)} />I have saved my recovery codes privately.</label>
    <button className="button button-dark" disabled={!saved} onClick={onDone}>{doneLabel}<Icon name="arrow" /></button>
  </div>
}

export function MFAGate({ user, onVerified, onLogout, embedded = false }: { user: User; onVerified: (user: User) => void; onLogout: () => void; embedded?: boolean }) {
  const [setup, setSetup] = useState<{ secret: string; uri: string } | null>(null)
  const [qr, setQR] = useState('')
  const [code, setCode] = useState('')
  const [recovery, setRecovery] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [retry, setRetry] = useState(0)
  const [result, setResult] = useState<MFAResult | null>(null)
  useEffect(() => {
    if (user.mfa_enrolled) return
    let active = true; setError(''); setSetup(null); setQR('')
    mutate<{ secret: string; uri: string }>('/api/auth/mfa/setup', 'POST', {}).then(async value => {
      if (!active) return
      setSetup(value)
      try { const data = await QRCode.toDataURL(value.uri, { width: 204, margin: 4, color: { dark: '#183d32', light: '#ffffff' } }); if (active) setQR(data) }
      catch { /* The manual setup key is always available. */ }
    }).catch((err: Error) => { if (active) setError(err.message) })
    return () => { active = false }
  }, [user.id, user.mfa_enrolled, retry])
  const preview = async () => {
    setBusy(true); setError('')
    try { const value = await mutate<{ code: string; recovery: boolean }>('/api/auth/mfa/demo-code', 'POST', {}); setCode(value.code); setRecovery(value.recovery) }
    catch (err) { setError((err as Error).message) } finally { setBusy(false) }
  }
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setBusy(true); setError('')
    try {
      if (user.mfa_enrolled) onVerified(await mutate<User>('/api/auth/mfa/verify', 'POST', { code, recovery }))
      else setResult(await mutate<MFAResult>('/api/auth/mfa/confirm', 'POST', { code }))
    } catch (err) { setError((err as Error).message) } finally { setBusy(false) }
  }
  useLayoutEffect(() => { window.scrollTo({ top: 0, behavior: 'instant' }) }, [Boolean(result)])
  const content = result ? <RecoveryCodes codes={result.recovery_codes ?? []} doneLabel={embedded ? 'Return to account security' : 'Continue to workspace'} onDone={() => onVerified(result.user)} /> : <>
    <span className="security-symbol"><Icon name="shield" /></span><span className="eyebrow">ONE EXTRA MOMENT OF CARE</span>
    <h2>{user.mfa_enrolled ? <>Just one <em>more step.</em></> : <>Your own <em>safe entry.</em></>}</h2>
    <p className="login-intro">{user.mfa_enrolled ? 'Enter a code from your authenticator app to open your workspace.' : 'Add Society OS to your authenticator app, then enter its six-digit code.'}</p>
    {!user.mfa_enrolled && setup && <div className="authenticator-setup">{qr && <img src={qr} width="204" height="204" alt="Scan this code with your authenticator app" />}<details><summary>Enter a setup key instead</summary><code className="setup-key">{setup.secret}</code><p className="form-help">Society OS · time-based · 6 digits · 30 seconds</p></details></div>}
    <form className="portal-form" onSubmit={submit}>
      <label>{recovery ? 'Recovery code' : 'Authenticator code'}<input autoComplete="one-time-code" inputMode={recovery ? 'text' : 'numeric'} value={code} onChange={e => setCode(e.target.value)} required maxLength={64} placeholder={recovery ? 'Your unused recovery code' : '000000'} /></label>
      {error && <p className="form-error" role="alert">{error}</p>}
      <button className="button button-dark" disabled={busy || (!user.mfa_enrolled && !setup)}>{busy ? 'Verifying…' : 'Verify and continue'}<Icon name="arrow" /></button>
      {user.mfa_enrolled && <button type="button" className="text-link" disabled={busy} onClick={() => { setRecovery(v => !v); setCode(''); setError('') }}>{recovery ? 'Use my authenticator app' : 'Use a recovery code'}</button>}
    </form>
    {user.is_demo && <div className="preview-verification"><Icon name="spark" /><div><strong>Exploring the local preview?</strong><p>This fictional account can use a preview code.</p><button className="text-link" disabled={busy || (!user.mfa_enrolled && !setup)} onClick={() => void preview()}>Use a preview code<Icon name="arrow" /></button></div></div>}
    {!user.mfa_enrolled && error && <button className="text-link" onClick={() => setRetry(v => v + 1)}>Restart setup</button>}
    <button className="text-link identity-exit" onClick={onLogout}>{embedded ? 'Back to account security' : 'Sign out and start again'}</button>
  </>
  return embedded ? <div className="security-card enrollment-card">{content}</div> : <IdentityShell>{content}</IdentityShell>
}

export function AccountLink({ token, onDone }: { token: string; onDone: () => void }) {
  const [details, setDetails] = useState<LinkDetails | null>(null)
  const [error, setError] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)
  const [saved, setSaved] = useState(false)
  useEffect(() => {
    let active = true
    mutate<LinkDetails>('/api/auth/link', 'POST', { token }).then(value => { if (active) setDetails(value) }).catch((err: Error) => { if (active) setError(err.message) })
    return () => { active = false }
  }, [token])
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setError('')
    if (password !== confirm) { setError('The two passwords do not match.'); return }
    setBusy(true)
    try { await mutate('/api/auth/link/complete', 'POST', { token, password }); setPassword(''); setConfirm(''); setSaved(true) }
    catch (err) { setError((err as Error).message) } finally { setBusy(false) }
  }
  return <IdentityShell><span className="security-symbol"><Icon name={saved ? 'check' : 'key'} /></span><span className="eyebrow">{saved ? 'ALL SET' : 'A PERSONAL WELCOME'}</span><h2>{saved ? <>Make yourself <em>at home.</em></> : details?.purpose === 'PASSWORD_RESET' ? <>A fresh <em>start.</em></> : <>Your place <em>is ready.</em></>}</h2>
    {saved ? <><p className="login-intro">Your password is saved. Sign in with your email and your new password. If you use an authenticator, it still protects your account.</p><button className="button button-dark" onClick={onDone}>Continue to sign in<Icon name="arrow" /></button></> : details ? <><p className="login-intro">Welcome, {details.name}. Set a password for <strong>{details.email}</strong>.</p><form className="portal-form" onSubmit={submit}><label>New password<input type="password" autoComplete="new-password" value={password} onChange={e => setPassword(e.target.value)} minLength={12} maxLength={256} required /></label><p className="form-help">Use at least 12 characters. A memorable, longer phrase works well.</p><label>Confirm new password<input type="password" autoComplete="new-password" value={confirm} onChange={e => setConfirm(e.target.value)} minLength={12} maxLength={256} required /></label>{error && <p className="form-error" role="alert">{error}</p>}<button className="button button-dark" disabled={busy}>{busy ? 'Saving…' : 'Save my password'}<Icon name="arrow" /></button></form></> : error ? <><p className="form-error" role="alert">{error}</p><button className="button button-dark" onClick={onDone}>Return to sign in<Icon name="arrow" /></button></> : <p role="status">Checking your personal link…</p>}
  </IdentityShell>
}

export function AccountSecurity({ user, onUser, onLogout }: { user: User; onUser: (user: User) => void; onLogout: () => void }) {
  const [enrolling, setEnrolling] = useState(false)
  const [reauthPassword, setReauthPassword] = useState('')
  const [currentPassword, setCurrentPassword] = useState('')
  const [code, setCode] = useState('')
  const [recovery, setRecovery] = useState(false)
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [codes, setCodes] = useState<string[]>([])
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [feedbackSection, setFeedbackSection] = useState<'authenticator' | 'identity' | 'password'>('authenticator')
  const [busy, setBusy] = useState(false)
  const run = async (action: () => Promise<void>, section: typeof feedbackSection = 'authenticator') => { setFeedbackSection(section); setBusy(true); setError(''); setMessage(''); try { await action() } catch (err) { setError((err as Error).message) } finally { setBusy(false) } }
  const feedback = (section: typeof feedbackSection) => feedbackSection === section && <>{error && <p className="form-error" role="alert">{error}</p>}{message && <p className="form-success" role="status">{message}</p>}</>
  useLayoutEffect(() => { window.scrollTo({ top: 0, behavior: 'instant' }) }, [enrolling, Boolean(codes.length)])
  if (enrolling) return <MFAGate user={user} embedded onLogout={() => setEnrolling(false)} onVerified={value => { onUser(value); setEnrolling(false) }} />
  return <div className="page-enter identity-page"><div className="section-heading"><div><span className="eyebrow">A LITTLE PEACE OF MIND</span><h1>Account <em>security.</em></h1><p className="page-description">Your access, with an extra layer of care.</p></div><span className="security-symbol"><Icon name="shield" /></span></div>
    {codes.length > 0 ? <div className="security-card"><RecoveryCodes codes={codes} doneLabel="Done, codes saved" onDone={() => setCodes([])} /></div> : <div className="security-grid">
      <section className="security-card"><span className="eyebrow">YOUR AUTHENTICATOR</span><h2>A second <em>key.</em></h2><div className="security-status"><i className={'dot ' + (user.mfa_enrolled ? 'dot-green' : '')} />{user.mfa_enrolled ? 'Two-step verification is enabled' : 'Two-step verification is not enabled'}</div><p>{user.mfa_required ? 'Required for privileged access. ' : 'Add protection to your personal account. '}Use your authenticator app each time you sign in.</p>
        {!user.mfa_enrolled ? <button className="button button-dark" onClick={() => setEnrolling(true)}>Set up authenticator<Icon name="arrow" /></button> : <button className="button button-outline" disabled={busy} onClick={() => void run(async () => { const value = await mutate<{ recovery_codes: string[] }>('/api/auth/recovery-codes', 'POST', {}); setCodes(value.recovery_codes) })}>Replace recovery codes<Icon name="key" /></button>}
        {feedback('authenticator')}
        <p className="form-help">Lost both your authenticator and your recovery codes? Contact the society’s designated custodians for assisted recovery.</p>
      </section>
      <section className="security-card"><span className="eyebrow">FOR SENSITIVE CHANGES</span><h2>Confirm <em>it’s you.</em></h2><p>Reconfirm before issuing access links, changing your password or replacing recovery codes. Confirmation lasts five minutes.</p>
        <form className="portal-form" onSubmit={e => { e.preventDefault(); void run(async () => { const value = await mutate<User>('/api/auth/reauthenticate', 'POST', { password: reauthPassword, code, recovery }); onUser(value); setReauthPassword(''); setCode(''); setMessage('Identity confirmed. You can make sensitive changes for five minutes.') }, 'identity') }}>
          <label>Current password<input type="password" autoComplete="current-password" value={reauthPassword} onChange={e => setReauthPassword(e.target.value)} maxLength={256} required /></label>
          {user.mfa_enrolled && <><label>{recovery ? 'Recovery code' : 'Authenticator code'}<input value={code} onChange={e => setCode(e.target.value)} autoComplete="one-time-code" inputMode={recovery ? 'text' : 'numeric'} maxLength={64} required /></label><button type="button" className="text-link" onClick={() => { setRecovery(v => !v); setCode('') }}>{recovery ? 'Use my authenticator app' : 'Use a recovery code'}</button>{user.is_demo && <button className="text-link" type="button" disabled={busy} onClick={() => void run(async () => { const value = await mutate<{ code: string; recovery: boolean }>('/api/auth/mfa/demo-code', 'POST', {}); setCode(value.code); setRecovery(value.recovery) }, 'identity')}>Use a preview code<Icon name="spark" /></button>}</>}
          {feedback('identity')}
          <button className="button button-dark" disabled={busy}>Confirm my identity<Icon name="arrow" /></button>
        </form>
      </section>
      <section className="security-card password-card"><div><span className="eyebrow">A FRESH START</span><h2>Change your <em>password.</em></h2><p>This signs you out of every device. Sign in again with your new password and authenticator.</p></div><form className="portal-form" onSubmit={e => { e.preventDefault(); void run(async () => { if (newPassword !== confirm) throw new Error('The new passwords do not match.'); await mutate('/api/auth/change-password', 'POST', { current_password: currentPassword, new_password: newPassword }); onLogout() }, 'password') }}><label>Current password<input type="password" autoComplete="current-password" value={currentPassword} onChange={e => setCurrentPassword(e.target.value)} maxLength={256} required /></label><label>New password<input type="password" autoComplete="new-password" value={newPassword} onChange={e => setNewPassword(e.target.value)} minLength={12} maxLength={256} required /></label><label>Confirm new password<input type="password" autoComplete="new-password" value={confirm} onChange={e => setConfirm(e.target.value)} minLength={12} maxLength={256} required /></label>{feedback('password')}<button className="button button-outline" disabled={busy}>Save new password<Icon name="arrow" /></button></form></section>
    </div>}
  </div>
}

export function Access({ user }: { user: User }) {
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [data, setData] = useState<AccountPage | null>(null)
  const [error, setError] = useState('')
  const [refresh, setRefresh] = useState(0)
  const [action, setAction] = useState<Account | 'invite' | null>(null)
  useEffect(() => {
    const controller = new AbortController(); setError(''); setData(null)
    const timer = setTimeout(() => request<AccountPage>('/api/admin/accounts?' + new URLSearchParams({ q: query, page: String(page), page_size: '12' }), controller.signal).then(setData).catch((err: Error) => { if (!controller.signal.aborted) setError(err.message) }), 200)
    return () => { clearTimeout(timer); controller.abort() }
  }, [query, page, refresh])
  return <div className="page-enter identity-page"><div className="section-heading"><div><span className="eyebrow">THE RIGHT PEOPLE, THE RIGHT ACCESS</span><h1>A personal <em>welcome.</em></h1><p className="page-description">Invite your neighbours. Keep every account thoughtfully in order.</p></div><button className="button button-dark" onClick={() => setAction('invite')}>Invite a person<Icon name="arrow" /></button></div>
    <div className="access-intro"><span className="access-intro-icon"><Icon name="community" /></span><div><strong>Every invitation starts with a familiar face.</strong><p>Choose someone in the registry, verify their identity and hand them their personal link through a trusted channel.</p></div><span className="chapter-tag">By invitation</span></div>
    <label className="search-field access-search"><Icon name="search" /><input type="search" aria-label="Search accounts" value={query} onChange={e => { setQuery(e.target.value); setPage(1) }} placeholder="Find a person or email…" maxLength={100} /></label>
    {error ? <div className="empty-state" role="alert"><h2>Let’s try that again.</h2><p>{error}</p><button className="button button-dark" onClick={() => setRefresh(v => v + 1)}>Try again<Icon name="refresh" /></button></div> : !data ? <p role="status">Opening the account book…</p> : <><div className="account-list">{data.items.map(account => <article className="account-row" key={account.id}><span className="avatar account-avatar">{account.name.split(' ').map(n => n[0]).slice(-2).join('')}</span><div className="account-person"><h3>{account.name}</h3><span>{account.email}</span><div className="account-meta"><span>{account.roles.length ? account.roles.map(r => r === 'ADMINISTRATOR' ? 'Administrator' : 'Committee').join(' · ') : 'Resident'}</span><span>{account.active_homes} active {account.active_homes === 1 ? 'home' : 'homes'}</span><span>{account.mfa_enrolled ? 'Authenticator on' : 'Authenticator not set'}</span></div></div><div className="account-actions"><span className={'account-state state-' + account.state.toLowerCase()}><i />{account.state === 'PENDING' ? 'Invitation pending' : account.state === 'ACTIVE' ? 'Active' : 'Disabled'}</span>{account.state !== 'DISABLED' && <button className="text-link" onClick={() => setAction(account)}>{account.state === 'PENDING' ? 'New invitation link' : 'Password recovery'}<Icon name="arrow" /></button>}</div></article>)}</div>{data.total === 0 && <div className="empty-state"><h2>No accounts found.</h2><p>Try another name or email.</p></div>}<div className="pagination"><span>{data.total} accounts · Page {page} of {Math.max(1, Math.ceil(data.total / 12))}</span><div><button aria-label="Previous account page" disabled={page === 1} onClick={() => setPage(p => p - 1)}><Icon name="left" /></button><button aria-label="Next account page" disabled={page * 12 >= data.total} onClick={() => setPage(p => p + 1)}><Icon name="chevron" /></button></div></div></>}
    <p className="access-footnote"><Icon name="shield" />Administrator access requires an authenticator. Links expire and work once.</p>
    {action && <InvitationDialog user={user} account={action} onClose={() => { setAction(null); setRefresh(v => v + 1) }} />}
  </div>
}

function InvitationDialog({ account, user, onClose }: { account: Account | 'invite'; user: User; onClose: () => void }) {
  const [query, setQuery] = useState('')
  const [people, setPeople] = useState<Person[]>([])
  const [lookupError, setLookupError] = useState('')
  const [person, setPerson] = useState('')
  const [email, setEmail] = useState('')
  const [role, setRole] = useState('RESIDENT')
  const [term, setTerm] = useState(90)
  const [verified, setVerified] = useState(false)
  const [note, setNote] = useState('')
  const [error, setError] = useState('')
  const [needsReauthentication, setNeedsReauthentication] = useState(false)
  const [busy, setBusy] = useState(false)
  const [link, setLink] = useState<IssuedLink | null>(null)
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    if (account !== 'invite') return
    setLookupError('')
    const controller = new AbortController()
    const timer = setTimeout(() => request<Person[]>('/api/people?q=' + encodeURIComponent(query), controller.signal).then(value => { if (!controller.signal.aborted) setPeople(value) }).catch((err: Error) => { if (!controller.signal.aborted) setLookupError(err.message) }), 200)
    return () => { clearTimeout(timer); controller.abort() }
  }, [account, query])
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setBusy(true); setError(''); setNeedsReauthentication(false)
    try {
      const input = { identity_verified: verified, note }
      const result = account === 'invite' ? await mutate<IssuedLink>('/api/admin/invitations', 'POST', { ...input, resident_id: person, email, role, term_days: term }) : await mutate<IssuedLink>('/api/admin/accounts/' + encodeURIComponent(account.id) + '/link', 'POST', { ...input, purpose: account.state === 'PENDING' ? 'INVITE' : 'PASSWORD_RESET' })
      setLink(result)
    } catch (err) { setError((err as Error).message); setNeedsReauthentication(err instanceof APIError && err.code === 'reauthentication_required') } finally { setBusy(false) }
  }
  const url = link ? window.location.origin + '/#' + (link.purpose === 'INVITE' ? 'activate=' : 'reset=') + link.token : ''
  return <PortalDialog titleId="invite-title" closeLabel="Close invitation" onClose={onClose} busy={busy} className="invitation-dialog"><header className="dialog-heading invitation-heading"><span className="eyebrow">{link ? 'A PERSONAL HANDOVER' : 'A THOUGHTFUL FIRST STEP'}</span><h2 id="invite-title">{link ? <>Their door <em>is open.</em></> : account === 'invite' ? <>Welcome <em>someone in.</em></> : <>A fresh <em>way in.</em></>}</h2></header><div className="dialog-scroll invitation-content">
    {link ? <><p>A {link.purpose === 'INVITE' ? 'welcome' : 'password recovery'} link for <strong>{link.name}</strong>. Give it to them through your verified channel.</p><div className="link-handover"><label>Personal link<input value={url} readOnly onFocus={e => e.target.select()} /></label><button className="button button-dark" onClick={async () => { try { await navigator.clipboard.writeText(url); setCopied(true) } catch { setError('Select and copy the link above.') } }}>{copied ? 'Link copied' : 'Copy personal link'}<Icon name={copied ? 'check' : 'arrow'} /></button><p className="form-help">Expires {new Date(link.expires_at * 1000).toLocaleTimeString('en-IN', { hour: 'numeric', minute: '2-digit' })}. Works once. This link is shown only here.</p></div><button className="text-link" onClick={onClose}>I’ve finished the handover<Icon name="check" /></button></> : <><p className="form-help">{account === 'invite' ? 'Use the person’s verified email and current home relationship.' : 'Verify ' + account.name + ' through a trusted channel before issuing a new link. Previous links will stop working.'}</p><form id="invitation-form" className="portal-form" onSubmit={submit}>
      {account === 'invite' && <><label>Find a person<input type="search" value={query} onChange={e => { setQuery(e.target.value); setPeople([]); setPerson(''); setVerified(false) }} placeholder="Search the registry by name" maxLength={100} /></label>{lookupError && <p className="form-error" role="alert">{lookupError}</p>}<label>Person in the registry<FormSelect label="Person in the registry" value={person} onChange={value => { setPerson(value); setVerified(false) }} required options={[{ value: '', label: 'Choose a person…' }, ...people.map(p => ({ value: p.id, label: p.name }))]} /></label><label>Email address<input type="email" autoComplete="off" value={email} onChange={e => setEmail(e.target.value)} maxLength={254} required /></label><div className="form-pair"><label>Access<FormSelect label="Access" value={role} onChange={setRole} options={[{ value: 'RESIDENT', label: 'Resident · their current homes' }, { value: 'COMMITTEE', label: 'Committee · read the registry' }, { value: 'ADMINISTRATOR', label: 'Administrator · manage registry' }]} /></label>{role !== 'RESIDENT' && <label>Access term (days)<input type="number" value={term} onChange={e => setTerm(Number(e.target.value))} min={1} max={365} required /></label>}</div></>}
      <label className="checkbox-label"><input type="checkbox" checked={verified} onChange={e => setVerified(e.target.checked)} required />I verified this person’s identity, email and home relationship.</label><label>Verification note<textarea value={note} onChange={e => setNote(e.target.value)} minLength={10} maxLength={300} required placeholder="How and when did you verify their identity?" rows={3} /></label>
      {error && <p className="form-error" role="alert">{error}</p>}{!user.fresh_authentication && <p className="form-help">You may need to confirm your identity in Account security first.</p>}
    </form>{needsReauthentication && <a className="text-link" href="#security" onClick={onClose}>Open Account security<Icon name="shield" /></a>}</>}
    {link && error && <p className="form-error" role="alert">{error}</p>}
  </div>{!link && <footer className="dialog-actions"><span className="form-help">A personal link, shared with care.</span><button form="invitation-form" className="button button-dark" disabled={busy}>{busy ? 'Preparing…' : 'Create personal link'}<Icon name="arrow" /></button></footer>}</PortalDialog>
}
