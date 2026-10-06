import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import type { User } from '../api'
import { contactActions, contactStates, preferenceFields, useContactLoad, useContactWrite } from '../contacts'
import type { Contact, ContactDetail, ContactPage, ContactPreferences } from '../contacts'
import { careTime } from '../upkeep'
import { PortalDialog } from './PortalDialog'
import { FilterSelect, FormSelect } from './FilterSelect'
import { FundCheck, FundUnavailable } from './FundShared'
import { FineFeedback } from './FineShared'
import { PageControls } from './Maintenance'
import { Icon } from './Icon'

function ConnectionArt() {
  return <svg className="contact-art" aria-hidden="true" viewBox="0 0 240 200" fill="none">
    <ellipse cx="122" cy="174" rx="106" ry="13" fill="#e2e8d7" />
    <rect x="39" y="52" width="158" height="100" rx="13" fill="#fdfbf4" stroke="#a4b48f" strokeWidth="2" />
    <path d="m42 58 76 55 76-55M42 147l55-42m97 42-55-42" stroke="#b6c5a5" strokeWidth="2" />
    <rect x="142" y="13" width="65" height="76" rx="17" fill="#dce5ce" />
    <path d="m158 48 11 11 21-24" stroke="#738966" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />
    <circle cx="35" cy="120" r="25" fill="#eee4d5" /><path d="M22 120h26m-13-13v26" stroke="#a39175" strokeWidth="2" />
    <path d="M219 108c-17 8-21 24-13 43" stroke="#a8ba96" strokeWidth="2" /><ellipse cx="221" cy="112" rx="6" ry="13" transform="rotate(42 221 112)" fill="#c1cfb0" />
  </svg>
}
function ContactFacts({ x, preview = false }: { x: Pick<Contact, 'phone' | 'email' | 'preferred_channel'> & ContactPreferences; preview?: boolean }) {
  return <section className="contact-paper" aria-label={preview ? 'Proposed contact choices' : 'Registered contact choices'}>
    <span className="eyebrow">{preview ? 'YOUR CONTACT PREVIEW' : 'DESTINATIONS & PERMISSION'}</span>
    <dl className="contact-destinations"><div><dt>WhatsApp number</dt><dd>{x.phone || 'Not supplied'}</dd></div><div><dt>Email address</dt><dd>{x.email || 'Not supplied'}</dd></div><div><dt>Preferred channel</dt><dd>{x.preferred_channel === 'WHATSAPP' ? 'WhatsApp' : x.preferred_channel === 'EMAIL' ? 'Email' : 'No preference'}</dd></div></dl>
    <ul className="contact-permissions">{preferenceFields.map(([key, label]) => <li key={key}><Icon name={x[key] ? 'check' : 'close'} /><span>{label}<small>{x[key] ? 'Permission recorded' : 'Not permitted'}</small></span></li>)}</ul>
  </section>
}
type Editor = ContactPreferences & { phone: string; email: string; preferred_channel: string; consent_source: string; reason: string }
const editorFor = (x: Contact): Editor => ({ phone: x.phone, email: x.email, preferred_channel: x.preferred_channel, consent_source: x.consent_source, reason: '', community_whatsapp: x.community_whatsapp, community_email: x.community_email, finance_whatsapp: x.finance_whatsapp, finance_email: x.finance_email })
function ContactRegistration({ x, form, setForm, preview, setPreview, writer, onSaved, onReload, onBack }: { x: ContactDetail; form: Editor; setForm: (form: Editor) => void; preview: boolean; setPreview: (preview: boolean) => void; writer: ReturnType<typeof useContactWrite>; onSaved: () => void; onReload: () => void; onBack: () => void }) {
  const [checked, setChecked] = useState(false)
  useEffect(() => { setChecked(false) }, [x.version])
  const disabled = writer.busy || writer.locked
  const change = <K extends keyof Editor>(key: K, value: Editor[K]) => { setForm({ ...form, [key]: value }); setChecked(false) }
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!preview) { setPreview(true); return }
    if (!checked || writer.conflict) return
    if (await writer.send('/api/contacts/' + encodeURIComponent(x.id) + '/register', { ...form, version: x.version })) onSaved()
  }
  return <form className="portal-form contact-form" onSubmit={event => { void submit(event) }}>
    <h3>{preview ? 'Review these choices.' : 'A good way to reach you.'}</h3>
    <p className="form-help">The person and permission source are reviewed independently. Changed destinations remain unavailable for delivery until verification.</p>
    {preview ? <><ContactFacts x={form} preview /><p className="preserve-lines">Permission source: {form.consent_source}</p><p className="preserve-lines">Reason: {form.reason}</p><FundCheck checked={checked} onChange={setChecked} disabled={disabled}>I reviewed the destination and recorded permission for these choices.</FundCheck></> : <fieldset className="records-fieldset" disabled={disabled}>
      <div className="contact-field-grid"><label>International WhatsApp number<input type="tel" autoComplete="tel" maxLength={40} placeholder="+ country code and number" value={form.phone} onChange={event => change('phone', event.target.value)} /></label><label>Email address<input type="email" autoComplete="email" maxLength={254} placeholder="name@example.com" value={form.email} onChange={event => change('email', event.target.value)} /></label></div>
      <label>Preferred channel<FormSelect label="Preferred channel" value={form.preferred_channel} options={[{ value: 'NONE', label: 'No preference' }, { value: 'WHATSAPP', label: 'WhatsApp' }, { value: 'EMAIL', label: 'Email' }]} onChange={value => change('preferred_channel', value)} /></label>
      <section className="contact-choice-fields" aria-label="Communication permission"><h4>You choose what reaches you.</h4><p className="form-help">Every permission is separate. Financial messages still require access to the underlying record.</p>{preferenceFields.map(([key, label]) => <FundCheck key={key} checked={form[key]} onChange={value => change(key, value)}>{label}</FundCheck>)}</section>
      <label>Identity and permission source<input required minLength={5} maxLength={300} value={form.consent_source} onChange={event => change('consent_source', event.target.value)} /></label>
      <label>Registration reason<textarea required minLength={10} maxLength={1000} value={form.reason} onChange={event => change('reason', event.target.value)} /></label>
    </fieldset>}
    <FineFeedback writer={writer} onReload={onReload} />
    <div className="form-actions contact-form-actions"><button type="button" className="button button-light" disabled={disabled} onClick={() => { if (preview) { setPreview(false); setChecked(false) } else onBack() }}>{preview ? 'Edit choices' : 'Back to contact'}</button><button className="button button-dark" disabled={writer.busy || writer.conflict || (preview && !checked)}>{writer.busy ? 'Saving…' : writer.locked ? 'Retry this registration' : preview ? 'Submit for verification' : 'Review contact'}<Icon name="arrow" /></button></div>
  </form>
}
type DecisionDraft = { action: string; reason: string; channel: string; purpose: string }
const emptyDecision = (): DecisionDraft => ({ action: 'VERIFIED', reason: '', channel: 'ALL', purpose: 'ALL' })
function ContactDecision({ x, kind, draft, setDraft, writer, onSaved, onReload, onBack }: { x: ContactDetail; kind: 'REVIEW' | 'STOP' | 'WITHDRAW'; draft: DecisionDraft; setDraft: (draft: DecisionDraft) => void; writer: ReturnType<typeof useContactWrite>; onSaved: () => void; onReload: () => void; onBack: () => void }) {
  const { action, reason, channel, purpose } = draft
  const [checked, setChecked] = useState(false)
  const setAction = (action: string) => setDraft({ ...draft, action }), setReason = (reason: string) => setDraft({ ...draft, reason }), setChannel = (channel: string) => setDraft({ ...draft, channel }), setPurpose = (purpose: string) => setDraft({ ...draft, purpose })
  useEffect(() => { setChecked(false) }, [x.version])
  const disabled = writer.busy || writer.locked, actual = kind === 'STOP' ? 'OPTED_OUT' : kind === 'WITHDRAW' ? 'WITHDRAWN' : action
  const label = kind === 'STOP' ? 'Stop selected messages' : kind === 'WITHDRAW' ? 'Withdraw registration' : action === 'VERIFIED' ? 'Verify contact and permission' : 'Decline verification'
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!checked || writer.conflict || (kind === 'REVIEW' && !x.can_verify)) return
    if (await writer.send('/api/contacts/' + encodeURIComponent(x.id) + '/actions', { version: x.version, action: actual, reason, ...(kind === 'STOP' ? { channel, purpose } : {}) })) onSaved()
  }
  return <form className="portal-form contact-form" onSubmit={event => { void submit(event) }}>
    <h3>{kind === 'STOP' ? 'Your choice, effective now.' : kind === 'WITHDRAW' ? 'Keep the history. Stop using this registration.' : 'A separate pair of eyes.'}</h3>
    <ContactFacts x={x} />
    {kind === 'REVIEW' && <p className="form-help preserve-lines">Supplied identity and permission source: {x.consent_source}. Verification records your offline attestation; it does not send a code or a message.</p>}
    <fieldset className="records-fieldset" disabled={disabled}>
      {kind === 'REVIEW' ? <label>Verification decision<FormSelect label="Verification decision" value={action} options={[{ value: 'VERIFIED', label: 'Verify contact and permission' }, { value: 'DECLINED', label: 'Decline verification' }]} onChange={value => { setAction(value); setChecked(false) }} /></label> : kind === 'STOP' ? <div className="contact-field-grid"><label>Channel to stop<FormSelect label="Channel to stop" value={channel} options={[{ value: 'ALL', label: 'All channels' }, { value: 'WHATSAPP', label: 'WhatsApp' }, { value: 'EMAIL', label: 'Email' }]} onChange={value => { setChannel(value); setChecked(false) }} /></label><label>Messages to stop<FormSelect label="Messages to stop" value={purpose} options={[{ value: 'ALL', label: 'All messages' }, { value: 'COMMUNITY', label: 'Community notices' }, { value: 'FINANCE', label: 'Financial messages' }]} onChange={value => { setPurpose(value); setChecked(false) }} /></label></div> : null}
      <label>{kind === 'REVIEW' ? 'Verification reason' : 'Change reason'}<textarea required minLength={10} maxLength={1000} value={reason} onChange={event => { setReason(event.target.value); setChecked(false) }} /></label>
      <FundCheck checked={checked} onChange={setChecked}>I reviewed this person, destination, permission and this decision’s effect.</FundCheck>
    </fieldset>
    <p className="form-help">{kind === 'STOP' ? 'Selected permissions stop immediately. Other choices and the original verification remain recorded. Restoring permission requires a new registration and review.' : kind === 'WITHDRAW' ? 'Withdrawal prevents new delivery through this profile and preserves its previous contact decisions.' : 'Only the selected recorded permissions become eligible. Current home and record access will be checked again before delivery.'}</p>
    <FineFeedback writer={writer} onReload={onReload} />
    <div className="form-actions contact-form-actions"><button type="button" className="button button-light" disabled={disabled} onClick={onBack}>Back to contact</button><button className="button button-dark" disabled={writer.busy || writer.conflict || !checked || (kind === 'REVIEW' && !x.can_verify)}>{writer.busy ? 'Saving…' : writer.locked ? 'Retry this decision' : label}<Icon name="check" /></button></div>
  </form>
}
function ContactDialog({ id, onClose, onChanged }: { id: string; onClose: () => void; onChanged: () => void }) {
  const [eventPage, setEventPage] = useState(1), [mode, setMode] = useState<'VIEW' | 'REGISTER' | 'REVIEW' | 'STOP' | 'WITHDRAW'>('VIEW'), [success, setSuccess] = useState('')
  // Human drafts survive transient loading and explicit stale reloads. Current
  // authority/data still comes from the new response and attestation resets.
  const [registrationDraft, setRegistrationDraft] = useState<Editor | null>(null), [registrationPreview, setRegistrationPreview] = useState(false), [decisionDraft, setDecisionDraft] = useState<DecisionDraft>(emptyDecision)
  const load = useContactLoad<ContactDetail>('/api/contacts/' + encodeURIComponent(id) + '?event_page=' + eventPage), writer = useContactWrite(), x = load.data
  const start = (next: typeof mode) => { writer.reset(); setRegistrationDraft(null); setRegistrationPreview(false); setDecisionDraft(emptyDecision()); setMode(next) }
  const reload = () => { writer.reset(); load.reload() }, saved = () => { setSuccess(mode === 'REGISTER' ? 'Contact choices saved for independent verification.' : mode === 'STOP' ? 'Selected messages stopped.' : mode === 'WITHDRAW' ? 'Registration withdrawn; history retained.' : 'Verification decision saved.'); setMode('VIEW'); load.reload(); onChanged() }
  return <PortalDialog titleId="contact-title" closeLabel="Close contact" className="fine-dialog contact-dialog" onClose={onClose} busy={writer.busy || writer.locked}>
    <div className="dialog-scroll contact-detail"><span className="eyebrow">PRIVATE CONTACT & COMMUNICATION CHOICES</span><h2 id="contact-title">{x?.name ?? 'Your contact'}</h2><FundUnavailable error={load.error} loading={load.loading} onRetry={load.reload}>{x && <>
      <span className={'contact-state contact-state-' + x.state.toLowerCase()}>{contactStates[x.state]}</span>
      {success && <p className="form-success" role="status">{success}</p>}
      {!x.current && <p className="form-help">This person has no current home relationship. Their history remains available; new delivery is unavailable.</p>}
      {mode === 'REGISTER' ? <ContactRegistration x={x} form={registrationDraft ?? editorFor(x)} setForm={setRegistrationDraft} preview={registrationPreview} setPreview={setRegistrationPreview} writer={writer} onSaved={saved} onReload={reload} onBack={() => setMode('VIEW')} /> : mode !== 'VIEW' ? <ContactDecision x={x} kind={mode} draft={decisionDraft} setDraft={setDecisionDraft} writer={writer} onSaved={saved} onReload={reload} onBack={() => setMode('VIEW')} /> : <>
        {x.state === 'NONE' ? <div className="contact-empty"><Icon name="community" /><h3>A place for your preferences.</h3><p>Register a destination and choose which messages you want to receive.</p></div> : <><ContactFacts x={x} /><p className="form-help">{x.state === 'VERIFIED' && x.current ? 'The selected choices are eligible for permitted messages when delivery is configured.' : 'These destinations are unavailable for delivery until a current registration is independently verified.'}</p>{x.decision_reason && <p className="form-help preserve-lines">Recorded decision: {x.decision_reason}</p>}</>}
        <div className="contact-detail-actions">{x.can_register && <button className="button button-dark" onClick={() => start('REGISTER')}>{x.state === 'NONE' ? 'Register contact' : 'Change contact choices'}<Icon name="arrow" /></button>}{x.can_verify && <button className="button button-dark" onClick={() => start('REVIEW')}>Review verification<Icon name="check" /></button>}{x.can_opt_out && <button className="button button-light" onClick={() => start('STOP')}>Stop messages<Icon name="close" /></button>}{x.can_withdraw && <button className="text-link" onClick={() => start('WITHDRAW')}>Withdraw registration</button>}</div>
        {x.event_total > 0 && <section className="review-history contact-history" aria-label="Contact activity"><h3>Each choice, kept together.</h3><ol>{x.events.map(event => <li key={event.version}><strong>{contactActions[event.action]}</strong><p>{event.reason}</p><small>{event.actor} · {careTime(event.at)}</small><details><summary>Contact choices at this step</summary><ContactFacts x={event.snapshot} /></details></li>)}</ol>{x.event_total > 20 && <PageControls label="contact activity" page={eventPage} total={x.event_total} size={20} onPage={setEventPage} />}</section>}
      </>}
    </>}</FundUnavailable></div>
  </PortalDialog>
}
export function Contacts({ user }: { user: User }) {
  const initial = new URLSearchParams(window.location.hash.split('?')[1] ?? '').get('person'), [selected, setSelected] = useState<string | null>(initial), [search, setSearch] = useState(''), [relationship, setRelationship] = useState(''), [building, setBuilding] = useState(''), [state, setState] = useState(''), [page, setPage] = useState(1), [revision, setRevision] = useState(0)
  const query = new URLSearchParams({ q: search, relationship, building, state, page: String(page), refresh: String(revision) }), load = useContactLoad<ContactPage>('/api/contacts?' + query, user.can_read_contacts), visible = !load.loading && !load.error ? load.data : null
  const open = (id: string) => { setSelected(id); window.history.replaceState(null, '', '#contacts?person=' + encodeURIComponent(id)) }, close = () => { setSelected(null); window.history.replaceState(null, '', '#contacts') }
  useEffect(() => { const change = () => setSelected(new URLSearchParams(window.location.hash.split('?')[1] ?? '').get('person')); window.addEventListener('hashchange', change); return () => window.removeEventListener('hashchange', change) }, [])
  if (!user.can_read_contacts) return <div className="empty-state"><h2>Contact preferences are unavailable.</h2><p>This account needs a linked resident or current community review appointment.</p></div>
  return <div className="page-enter contacts-page"><section className="maintenance-hero contact-hero"><div><span className="eyebrow">A COMMUNITY, IN CONVERSATION</span><h1>Good neighbours.<br /><em>Better connected.</em></h1><p>{user.can_manage_contacts ? 'Keep destinations verified, choices respected, and the right people in the conversation.' : 'Choose how your community reaches you. Every preference stays in your hands.'}</p></div><ConnectionArt /></section>
    <section className="contact-register"><div className="section-heading"><div><h2>{user.can_manage_contacts ? 'People & preferences' : 'Your communication choices'}</h2><p>{user.can_manage_contacts ? 'Open a person to register, independently verify or stop messages.' : 'Your destinations are private. Register a change or stop selected messages whenever you need.'}</p></div><span className="result-count">{visible ? visible.total + (visible.total === 1 ? ' person in this view' : ' people in this view') : 'Checking current people…'}</span></div>
      {user.can_manage_contacts && <div className="contact-filters"><label className="search-control"><Icon name="search" /><input aria-label="Search contacts by person" placeholder="Find a person…" maxLength={100} value={search} onChange={event => { setSearch(event.target.value); setPage(1) }} /></label><FilterSelect label="Filter contacts by relationship" value={relationship} options={[{ value: '', label: 'All people' }, { value: 'OWNER', label: 'Owners' }, { value: 'TENANT', label: 'Tenants' }]} onChange={value => { setRelationship(value); setPage(1) }} /><FilterSelect label="Filter contacts by wing" value={building} options={[{ value: '', label: 'All wings' }, ...['A', 'B', 'C'].map(value => ({ value, label: 'Wing ' + value }))]} onChange={value => { setBuilding(value); setPage(1) }} /><FilterSelect label="Filter contacts by verification" value={state} options={[{ value: '', label: 'All registrations' }, ...Object.entries(contactStates).map(([value, label]) => ({ value, label }))]} onChange={value => { setState(value); setPage(1) }} />{(search || relationship || building || state) && <button className="clear-button" onClick={() => { setSearch(''); setRelationship(''); setBuilding(''); setState(''); setPage(1) }}>Clear<Icon name="close" /></button>}</div>}
      {load.loading ? <p className="empty-state" role="status">Opening current communication choices…</p> : load.error ? <div className="empty-state" role="alert"><h3>Contacts could not be opened.</h3><p>{load.error}</p><button className="button button-dark" onClick={load.reload}>Try again<Icon name="refresh" /></button></div> : visible && <>{visible.items.length ? <div className="contact-cards">{visible.items.map(x => <button key={x.id} className="contact-card" onClick={() => open(x.id)} aria-label={'Open contact ' + x.name}><span className="contact-card-top"><span className="contact-avatar">{x.name.split(' ').map(v => v[0]).slice(-2).join('')}</span><Icon name="arrow" /></span><h3>{x.name}</h3><span className="contact-homes">{x.homes.map(h => h.label).join(' · ') || 'Former home relationship'}</span><span className={'contact-state contact-state-' + x.state.toLowerCase()}>{contactStates[x.state]}</span><span className="contact-card-destinations"><span>{x.phone || 'No WhatsApp number'}</span><span>{x.email || 'No email supplied'}</span></span></button>)}</div> : <div className="empty-state"><h3>No people match these choices.</h3><p>{user.can_manage_contacts ? 'Clear a filter or search for another current resident.' : 'Your account has no linked contact profile.'}</p></div>}{visible.total > 12 && <PageControls label="people and preferences" page={page} total={visible.total} size={12} onPage={setPage} />}</>}
    </section>{selected && <ContactDialog key={selected} id={selected} onClose={close} onChanged={() => setRevision(value => value + 1)} />}
  </div>
}
