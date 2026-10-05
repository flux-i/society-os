import { useEffect, useRef, useState } from 'react'
import { APIError, mutate, request } from '../api'
import type { Account, User } from '../api'
import { FormSelect } from './FilterSelect'
import { Icon } from './Icon'
import { PortalDialog } from './PortalDialog'

export const roleLabel = (role: string) => (({ ADMINISTRATOR: 'Administrator', COMMITTEE: 'Committee', TREASURER: 'Treasurer', AUDITOR: 'Accountant / auditor' } as Record<string, string>)[role] ?? role)
export interface Appointment { id: string; role: string; state: string; valid_from: number; valid_until: number; granted_by: string; granted_name: string; revoked_at: number; revoked_by: string; revoked_name: string }
interface AccessEvent { id: number; action: string; actor: string; at: number; reason: string }
export interface AccountDetails { account: Account; version: number; grants: Appointment[]; grant_total: number; grant_page: number; events: AccessEvent[]; event_total: number; history_page: number; page_size: number }
type Action = { kind: 'grant' | 'suspend' | 'resume' } | { kind: 'revoke'; grant: Appointment }
const dateTime = (value: number) => new Date(value * 1000).toLocaleString('en-IN', { timeZone: 'Asia/Kolkata', day: 'numeric', month: 'short', year: 'numeric', hour: 'numeric', minute: '2-digit' }) + ' IST'
const powers: Record<string, string> = {
  ADMINISTRATOR: 'Manage accounts and the registry; review community requests, service cases and non-financial documents. Financial access and posting need their own entitlement.',
  COMMITTEE: 'Read the community registry and financial records; review requests, service cases and non-financial documents. No account administration or financial posting.',
  TREASURER: 'Read and post supplied financial records, make linked corrections and manage accounting documents. No account administration or committee review powers.',
  AUDITOR: 'Read financial records and permitted approved accounting documents. No financial posting, account administration or operational review.',
}
const eventLabel = (action: string) => (({ APPOINTMENT_GRANTED: 'Appointment added', APPOINTMENT_REVOKED: 'Appointment ended', ACCOUNT_SUSPENDED: 'Account suspended', ACCOUNT_RESUMED: 'Account resumed', INVITE_LINK_ISSUED: 'Invitation prepared', INVITE_COMPLETED: 'Invitation accepted', PASSWORD_RESET_LINK_ISSUED: 'Recovery prepared', PASSWORD_RESET_COMPLETED: 'Password recovered', PASSWORD_CHANGED: 'Password changed', OFFLINE_MFA_RECOVERY: 'Authenticator recovery recorded' } as Record<string, string>)[action] ?? action.toLowerCase().replaceAll('_', ' '))

export function AccountAdministration({ user, account, onClose, onUpdated }: { user: User; account: Account; onClose: () => void; onUpdated: () => void }) {
  const [data, setData] = useState<AccountDetails | null>(null)
  const [readError, setReadError] = useState('')
  const [error, setError] = useState('')
  const [conflict, setConflict] = useState(false)
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)
  const [reload, setReload] = useState(0)
  const [grantPage, setGrantPage] = useState(1)
  const [historyPage, setHistoryPage] = useState(1)
  const [tab, setTab] = useState<'grants' | 'history'>('grants')
  const [action, setAction] = useState<Action | null>(null)
  const [role, setRole] = useState('COMMITTEE')
  const [days, setDays] = useState('90')
  const [reason, setReason] = useState('')
  const [confirmed, setConfirmed] = useState(false)
  const [success, setSuccess] = useState('')
  const editorOpener = useRef('')
  const restoreEditor = useRef(false)
  const contentRef = useRef<HTMLDivElement>(null)
  const errorRef = useRef<HTMLDivElement>(null)
  const successRef = useRef<HTMLParagraphElement>(null)
  const focusSuccess = useRef(false)
  const path = '/api/admin/accounts/' + encodeURIComponent(account.id)
  const self = account.id === user.id

  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setReadError('')
    request<AccountDetails>(path + '?' + new URLSearchParams({ grant_page: String(grantPage), history_page: String(historyPage) }), controller.signal)
      .then(value => { if (!controller.signal.aborted) { setData(value); setLoading(false) } })
      .catch((err: Error) => { if (!controller.signal.aborted) { setData(null); setReadError(err.message); setLoading(false) } })
    return () => controller.abort()
  }, [path, grantPage, historyPage, reload])
  useEffect(() => { if (focusSuccess.current && !loading && data) { successRef.current?.focus({ preventScroll: true }); focusSuccess.current = false } }, [data, loading])
  useEffect(() => { if (restoreEditor.current && !action && !loading) { contentRef.current?.querySelector<HTMLElement>('[data-access-opener="' + CSS.escape(editorOpener.current) + '"]')?.focus({ preventScroll: true }); restoreEditor.current = false } }, [action, loading])
  useEffect(() => { if (error && !busy) errorRef.current?.focus({ preventScroll: true }) }, [error, busy])

  const openAction = (next: Action, opener: HTMLElement) => { editorOpener.current = opener.dataset.accessOpener ?? ''; setAction(next); setReason(''); setConfirmed(false); setError(''); setConflict(false); setSuccess('') }
  const cancel = () => { restoreEditor.current = true; setAction(null); setError(''); setConflict(false) }
  const refresh = () => { setAction(null); setError(''); setConflict(false); setSuccess(''); setReload(value => value + 1) }
  const submit = async (event: React.SubmitEvent<HTMLFormElement>) => {
    event.preventDefault(); if (!data || !action || busy || loading || conflict) return
    setBusy(true); setError(''); setSuccess('')
    try {
      const input = { version: data.version, confirmed, reason: reason.trim() }
      if (action.kind === 'grant') await mutate(path + '/roles', 'POST', { ...input, role, term_days: Number(days) })
      else if (action.kind === 'revoke') await mutate(path + '/roles/' + encodeURIComponent(action.grant.id) + '/revoke', 'POST', input)
      else await mutate(path + '/status', 'POST', { ...input, action: action.kind.toUpperCase() })
      setSuccess(action.kind === 'grant' ? 'Appointment saved. They’ll sign in again.' : action.kind === 'revoke' ? 'Appointment ended. They’ll sign in again under their remaining access.' : action.kind === 'suspend' ? 'Account suspended. Sessions, personal links and appointments have ended.' : 'Account resumed. Previous appointments stay ended; assign any new access explicitly.')
      setAction(null); setReason(''); setConfirmed(false); focusSuccess.current = true
      setReload(value => value + 1); onUpdated()
    } catch (err) { setError(err instanceof Error ? err.message : 'The change could not be saved.'); setConflict(err instanceof APIError && err.status === 409) }
    finally { setBusy(false) }
  }
  const actionTitle = !action ? '' : action.kind === 'grant' ? 'Add an appointment' : action.kind === 'revoke' ? 'End ' + roleLabel(action.grant.role) + ' access' : action.kind === 'suspend' ? 'Suspend this account' : 'Resume this account'

  return <PortalDialog titleId="account-access-title" closeLabel="Close account access" onClose={onClose} busy={busy} className="account-access-dialog">
    <header className="dialog-heading account-access-heading">
      <span className="eyebrow">ACCESS, WITH CARE</span><h2 id="account-access-title">{account.name}</h2><p>{account.email}</p>
      <div className="account-access-summary"><span className={'account-state state-' + (data?.account.state ?? account.state).toLowerCase()}><i />{data?.account.state === 'PENDING' ? 'Invitation pending' : (data?.account.state ?? account.state).toLowerCase().replace(/^./, value => value.toUpperCase())}</span><span>{data?.account.active_homes ?? account.active_homes} active homes</span><span>{data?.account.mfa_enrolled ?? account.mfa_enrolled ? 'Authenticator on' : 'Authenticator not set'}</span></div>
    </header>
    <div className="dialog-scroll account-access-content" ref={contentRef} aria-busy={loading}>
      {readError ? <div className="empty-state" role="alert"><h3>Account unavailable.</h3><p>{readError}</p><button className="button button-outline" onClick={refresh}>Retry account<Icon name="refresh" /></button></div> : !data ? <p className="form-help" role="status">Opening this account…</p> : <>
        {loading && <p className="form-help" role="status">Updating this account…</p>}
        {success && <p className="form-success" ref={successRef} tabIndex={-1} role="status">{success}</p>}
        {self && <p className="access-care-note"><Icon name="shield" />This is your account. Ask another administrator to change your access.</p>}
        {action ? <section className={'access-change-panel ' + (action.kind === 'suspend' || action.kind === 'revoke' ? 'access-change-caution' : '')}>
          <div className="section-heading"><div><span className="eyebrow">A VERIFIED DECISION</span><h3>{actionTitle}</h3></div><button className="text-link" type="button" disabled={busy} onClick={cancel}>Cancel<Icon name="close" /></button></div>
          <form id="account-access-form" className="portal-form" onSubmit={submit}>
            <fieldset className="access-fields" disabled={busy || loading || conflict}>
            {action.kind === 'grant' ? <><div className="form-pair"><label>Appointment<FormSelect label="Appointment" value={role} onChange={value => { setRole(value); setConfirmed(false) }} options={Object.keys(powers).map(value => ({ value, label: roleLabel(value) }))} /></label><label>Term (days)<input type="number" value={days} onChange={event => { setDays(event.target.value); setConfirmed(false) }} min={1} max={365} step={1} required /></label></div><p className="access-powers">{powers[role]}</p><p className="form-help">Starts when saved. A term lasts 1–365 days. They’ll sign in again and verify their authenticator before privileged access.</p></> : <p className="access-powers">{action.kind === 'revoke' ? 'This appointment will end immediately. Their other current appointments and home entitlements remain. They’ll sign in again.' : action.kind === 'suspend' ? 'This ends all sessions, outstanding personal links, recovery codes and appointments. Identity, confirmed authenticator and historical records are retained.' : 'Verify their identity before resuming. Old sessions, links and appointments stay ended. An unaccepted invitation needs a new personal handover.'}</p>}
            <label>Reason<textarea value={reason} onChange={event => setReason(event.target.value)} minLength={10} maxLength={300} rows={3} required placeholder="Record the approved decision and verification…" /></label>
            <label className="checkbox-label"><input type="checkbox" checked={confirmed} onChange={event => setConfirmed(event.target.checked)} required /><span>{action.kind === 'resume' ? 'I have verified this person’s identity and authority to resume access.' : 'I have verified the approved decision and authority for this change.'}</span></label>
            </fieldset>
            {error && <div className="access-change-error" ref={errorRef} tabIndex={-1} role="alert"><p className="form-error">{error}</p>{conflict && <button type="button" className="button button-outline" disabled={busy} onClick={refresh}>Reload account<Icon name="refresh" /></button>}</div>}
            <p className="form-help">Sensitive changes need recent identity confirmation. <a href="#security" aria-disabled={busy || undefined} onClick={event => { if (busy) event.preventDefault(); else onClose() }}>Open Account security</a></p>
          </form>
        </section> : <>
          <div className="detail-tabs" aria-label="Account access details"><button aria-pressed={tab === 'grants'} disabled={loading} onClick={() => setTab('grants')}>Appointments<span>{data.grant_total}</span></button><button aria-pressed={tab === 'history'} disabled={loading} onClick={() => setTab('history')}>Activity<span>{data.event_total}</span></button></div>
          {tab === 'grants' ? <>
            <div className="section-heading access-appointments-heading"><div><h3>Clear roles.<br /><em>Considered access.</em></h3><p>Appointments expire at the time shown. Home access follows current relationships separately.</p></div>{!self && data.account.state === 'ACTIVE' && <button className="button button-outline" disabled={loading} data-access-opener="add" onClick={event => openAction({ kind: 'grant' }, event.currentTarget)}>Add appointment<Icon name="plus" /></button>}</div>
            {data.grants.length === 0 ? <div className="access-appointments-empty"><Icon name="leaf" /><h4>No appointments yet.</h4><p>{data.account.state === 'PENDING' ? 'Let them accept their invitation before adding an appointment.' : data.account.state === 'SUSPENDED' ? 'Resume the account and verify the new decision before assigning access.' : 'This person’s access follows their current home relationships.'}</p></div> : <div className="appointment-list">{data.grants.map(grant => <article className={'appointment-card appointment-' + grant.state.toLowerCase()} key={grant.id}>
              <div className="appointment-top"><h4>{roleLabel(grant.role)}</h4><span className="appointment-state">{grant.state.toLowerCase().replace(/^./, value => value.toUpperCase())}</span></div>
              <p>{grant.state === 'REVOKED' ? 'Ended ' + dateTime(grant.revoked_at) : 'Expires ' + dateTime(grant.valid_until)}</p><small>Started {dateTime(grant.valid_from)} · Assigned by {grant.granted_name}{grant.revoked_name && ' · Ended by ' + grant.revoked_name}</small>
              {!self && ['ACTIVE', 'UPCOMING'].includes(grant.state) && <button className="text-link" disabled={loading} data-access-opener={grant.id} onClick={event => openAction({ kind: 'revoke', grant }, event.currentTarget)}>End {roleLabel(grant.role)} appointment<Icon name="arrow" /></button>}
            </article>)}</div>}
            <AccessPagination label="appointment" total={data.grant_total} page={grantPage} disabled={loading} onPage={setGrantPage} />
          </> : <>
            <div className="access-history-intro"><h3>A record of care.</h3><p>Verified decisions and account handovers, with their author and reason.</p></div>
            {data.events.length === 0 ? <p className="form-help">No recorded account handovers or changes yet.</p> : <ol className="access-history">{data.events.map(event => <li key={event.id}><span className="access-history-dot" /><div><strong>{eventLabel(event.action)}</strong><p>{event.reason}</p><small>{event.actor} · {dateTime(event.at)}</small></div></li>)}</ol>}
            <AccessPagination label="activity" total={data.event_total} page={historyPage} disabled={loading} onPage={setHistoryPage} />
          </>}
          {!self && <div className="account-status-care"><div><strong>{data.account.state === 'SUSPENDED' ? 'Ready for a new start?' : 'Need to pause access?'}</strong><p>{data.account.state === 'SUSPENDED' ? 'Resume after verification. Assign any appointments again explicitly.' : 'Suspend the identity while retaining its history. A successor administrator must be ready before an administrator leaves.'}</p></div><button className="button button-outline" disabled={loading} data-access-opener="status" onClick={event => openAction({ kind: data.account.state === 'SUSPENDED' ? 'resume' : 'suspend' }, event.currentTarget)}>{data.account.state === 'SUSPENDED' ? 'Resume account' : 'Suspend account'}<Icon name="shield" /></button></div>}
        </>}
      </>}
    </div>
    {action && data && !readError && <footer className="dialog-actions"><span className="form-help">A decision recorded with care.</span><button form="account-access-form" className="button button-dark" disabled={busy || loading || conflict || !confirmed}>{busy ? 'Saving…' : action.kind === 'grant' ? 'Save appointment' : action.kind === 'revoke' ? 'End appointment' : action.kind === 'suspend' ? 'Confirm suspension' : 'Confirm resumption'}<Icon name="arrow" /></button></footer>}
  </PortalDialog>
}

function AccessPagination({ label, total, page, disabled, onPage }: { label: string; total: number; page: number; disabled: boolean; onPage: (page: number) => void }) {
  if (total <= 20) return null
  return <div className="pagination"><span>{total} {label === 'activity' ? 'events' : 'appointments'} · Page {page} of {Math.ceil(total / 20)}</span><div><button aria-label={'Previous ' + label + ' page'} disabled={disabled || page === 1} onClick={() => onPage(page - 1)}><Icon name="left" /></button><button aria-label={'Next ' + label + ' page'} disabled={disabled || page * 20 >= total} onClick={() => onPage(page + 1)}><Icon name="chevron" /></button></div></div>
}
