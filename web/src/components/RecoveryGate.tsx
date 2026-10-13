import { useCallback, useEffect, useRef, useState } from 'react'
import { request } from '../api'
import type { User } from '../api'
import { useSocietyTools } from '../webmcp'
import { Icon } from './Icon'

export function RecoveryGate({ user, onUser, onLogout }: { user: User; onUser: (user: User) => void; onLogout: () => void }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [checked, setChecked] = useState(false)
  const pending = useRef<AbortController | null>(null)
  useEffect(() => () => pending.current?.abort(), [])
  const noHome = useCallback(() => {}, [])
  useSocietyTools(user, noHome)
  const refresh = async () => {
    pending.current?.abort()
    const controller = new AbortController(); pending.current = controller
    setBusy(true); setError(''); setChecked(false)
    try {
      const current = await request<User>('/api/auth/me', controller.signal)
      if (!controller.signal.aborted) { onUser(current); setChecked(true) }
    }
    catch (err) { if (!controller.signal.aborted) setError((err as Error).message) }
    finally { if (!controller.signal.aborted) setBusy(false) }
  }
  return <main className="recovery-screen" id="main-content" tabIndex={-1}>
    <header className="recovery-top"><span className="footer-wordmark">society.</span><span className="recovery-status"><i className="dot dot-green" />Recovery review</span></header>
    <section className="recovery-intro" aria-labelledby="recovery-title">
      <span className="about-mark"><Icon name="leaf" /></span><span className="eyebrow">A CAREFUL NEW BEGINNING</span>
      <h1 id="recovery-title">Your records.<br /><em>Kept together.</em></h1>
      <p>This workspace has been recovered. The society’s records are retained while current access and unfinished actions are reviewed.</p>
    </section>
    <section className="recovery-pause" aria-label="What happens during review">
      <span className="eyebrow">UNTIL THE REVIEW IS COMPLETE</span>
      <div className="recovery-boundaries">
        <article><Icon name="shield" /><div><h2>Private access waits.</h2><p>Homes, documents and personal records open after access is verified.</p></div></article>
        <article><Icon name="receipt" /><div><h2>New changes stay paused.</h2><p>Original receipts and entries remain intact. New financial changes need reconciliation.</p></div></article>
        <article><Icon name="community" /><div><h2>Unfinished actions stay held.</h2><p>Previous uploads and message attempts are retained for review before anything proceeds.</p></div></article>
      </div>
      <p className="recovery-guidance">Your society’s recovery custodian will complete the review. Signing in again keeps this same protection in place.</p>
      {error && <p className="form-error" role="alert">{error}</p>}
      <p className="recovery-feedback" role="status">{busy ? 'Checking this workspace…' : checked ? 'Checked just now. Recovery review is still required.' : 'Your records stay retained throughout the review.'}</p>
      <div className="recovery-actions"><button type="button" className="button button-dark" disabled={busy} onClick={() => void refresh()}>{busy ? 'Checking…' : error ? 'Try again' : 'Check status'}<Icon name="refresh" /></button><button type="button" className="button button-light" onClick={onLogout}>Sign out<Icon name="arrow" /></button></div>
    </section>
    <footer className="recovery-footnote"><Icon name="leaf" />A little care, before community life continues.</footer>
  </main>
}
