import { useLayoutEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import type { User } from '../api'
import { money } from '../maintenance'
import type { RecoveryPage } from '../recovery-review'
import { recoveryDate } from '../recovery-review'
import type { RecoveryGapComparison, RecoveryGapDecision, RecoveryGapHome, RecoveryGapInput, RecoveryGapItem, RecoveryGapPreparation, RecoveryGapStatus } from '../recovery-gap'
import { gapEvidence, gapKind, gapPaise, gapState } from '../recovery-gap'
import { FilterSelect, FormSelect } from './FilterSelect'
import { Icon } from './Icon'
import { PortalDialog } from './PortalDialog'
import { Pager, useRecoveryRead, useRetainedWrite } from './RecoveryControls'

const base = '/api/recovery/financial-gap', preparationsPath = base + '/preparations', operationsPath = base + '/operations/'
const localDate = (seconds: number) => { const date = new Date(seconds * 1000); return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16) }
const seconds = (date: string) => Math.floor(new Date(date).getTime() / 1000)
const blankItem = (): RecoveryGapItem => ({ book_ref: '', item_ref: '', flat_id: '', kind: 'RECEIVED', amount: '', date: new Date().toLocaleDateString('en-CA'), entry_ref: '', receipt_number: '', evidence: '', evidence_state: 'SUPPORTED' })
type Row = { key: string; item: RecoveryGapItem }
type Home = { id: string; label: string }
type Callbacks = { user: User; confirm: () => void; saved: (value: RecoveryGapPreparation) => void }

function BalanceCard({ home, preview = false }: { home: RecoveryGapHome; preview?: boolean }) {
  return <article className="gap-balance" aria-label={`${home.home} ${preview ? 'comparison preview' : 'frozen comparison'}`}>
    <h3>{home.home}</h3><dl><div><dt>Saved checkpoint balance</dt><dd>{money(home.checkpoint_paise)}</dd></div><div><dt>Supplied known difference</dt><dd>{money(home.difference_paise)}</dd></div><div><dt>Known comparison only</dt><dd>{money(home.comparison_paise)}</dd></div></dl>
    <p className="form-help">Includes saved voluntary allocations. Unresolved items remain outside this comparison. {preview ? 'The retained version is checked again when submitted.' : 'This assessment has not changed the saved balance.'}</p>
  </article>
}

function BalancePreview({ homeID, items }: { homeID: string; items: RecoveryGapItem[] }) {
  const [revision, setRevision] = useState(0)
  const balance = useRecoveryRead<RecoveryGapHome>(`${base}/balance/${encodeURIComponent(homeID)}`, revision)
  if (balance.busy) return <p role="status">Opening the saved balance…</p>
  if (balance.error) return <div role="alert"><p className="form-error">{balance.error}</p><button type="button" className="button button-light" onClick={() => setRevision(value => value + 1)}>Try saved balance again</button></div>
  if (!balance.data) return null
  let difference = 0
  for (const item of items.filter(item => item.flat_id === homeID && item.kind !== 'EVIDENCE')) {
    const amount = gapPaise(item.amount)
    if (amount === null || item.evidence_state !== 'SUPPORTED') return <p className="form-help">Enter a valid exact amount and supported evidence to preview this home’s known difference.</p>
    difference += item.kind === 'RECEIVED' ? -amount : amount
  }
  const compared = balance.data.checkpoint_paise + difference
  if (!Number.isSafeInteger(compared)) return <p className="form-error" role="alert">This comparison exceeds exact supported monetary bounds.</p>
  return <BalanceCard home={{ ...balance.data, difference_paise: difference, comparison_paise: compared }} preview />
}

function RetainedActions({ action, user, confirm, label, retry }: { action: Pick<ReturnType<typeof useRetainedWrite<RecoveryGapInput | RecoveryGapDecision, RecoveryGapPreparation>>, 'error' | 'uncertain' | 'busy' | 'check' | 'retry'>; user: User; confirm: () => void; label: string; retry: string }) {
  return <>{action.error && <p role="alert" className="form-error">{action.error}</p>}{action.uncertain ? <div className="review-desk-actions"><button type="button" className="button button-light" disabled={action.busy} onClick={() => void action.check()}>Check saved result</button><button type="button" className="button button-dark" disabled={action.busy || !user.fresh_authentication} onClick={() => void action.retry()}>{retry}</button></div> : <button type="submit" className="button button-dark" disabled={action.busy || !user.fresh_authentication}>{action.busy ? 'Retaining…' : label}<Icon name="check" /></button>}{!user.fresh_authentication && <button type="button" className="button button-light" disabled={action.busy} onClick={confirm}>Confirm identity to continue</button>}</>
}

function GapPreparation({ user, predecessor, sourceCreatedAt, close, saved, confirm }: Callbacks & { predecessor: RecoveryGapPreparation | null; sourceCreatedAt: number; close: () => void }) {
  const source = predecessor?.input, now = Math.floor(Date.now() / 1000)
  const [title, setTitle] = useState(source?.title ?? 'Supplied checkpoint-to-return assessment'), [reason, setReason] = useState(source?.reason ?? '')
  const [from, setFrom] = useState(localDate(source?.coverage_from ?? sourceCreatedAt)), [through, setThrough] = useState(localDate(source?.coverage_through ?? now)), [verified, setVerified] = useState(localDate(now))
  const [coverage, setCoverage] = useState(source?.coverage_evidence ?? ''), [complete, setComplete] = useState(source?.coverage_complete ?? false), [empty, setEmpty] = useState(source?.declared_empty ?? false), [confirmed, setConfirmed] = useState(false)
  const [rows, setRows] = useState<Row[]>(() => (source?.items ?? [blankItem()]).map(item => ({ key: crypto.randomUUID(), item })))
  const [homesRevision, setHomesRevision] = useState(0), [focusKey, setFocusKey] = useState('')
  const formRef = useRef<HTMLFormElement>(null)
  useLayoutEffect(() => { if (focusKey) formRef.current?.querySelector<HTMLInputElement>(`[data-row="${focusKey}"] input`)?.focus() }, [focusKey])
  const homes = useRecoveryRead<{ items: Home[]; limit: number }>('/api/recovery/homes', homesRevision)
  const action = useRetainedWrite<RecoveryGapInput, RecoveryGapPreparation>(preparationsPath, saved, undefined, operationsPath)
  const change = (key: string, value: Partial<RecoveryGapItem>) => setRows(current => current.map(row => row.key === key ? { ...row, item: { ...row.item, ...value } } : row))
  const add = () => { const key = crypto.randomUUID(); setRows(current => [...current, { key, item: blankItem() }]); setEmpty(false); setFocusKey(key) }
  const remove = (key: string) => { const remaining = rows.filter(row => row.key !== key); setRows(remaining); setFocusKey(remaining.at(-1)?.key ?? ''); if (!remaining.length) formRef.current?.querySelector<HTMLButtonElement>('[data-add-item]')?.focus() }
  const items = rows.map(row => row.item), homeIDs = [...new Set(items.map(item => item.flat_id).filter(Boolean))]
  const submit = (event: FormEvent) => { event.preventDefault(); if (action.held || !confirmed || !homes.data) return
    void action.run({ operation_key: crypto.randomUUID(), scope_key: user.scope_key, predecessor_id: predecessor?.id ?? '', predecessor_version: predecessor?.version ?? 0, title, reason, coverage_from: seconds(from), coverage_through: seconds(through), verified_at: seconds(verified), coverage_evidence: coverage, coverage_complete: complete, declared_empty: empty, items })
  }
  return <PortalDialog titleId="gap-prepare-title" closeLabel="Close financial preparation" onClose={close} busy={action.held} className="review-desk-dialog gap-dialog">
    <div className="dialog-heading review-desk-dialog-top"><span className="eyebrow">THE RECORDS BETWEEN</span></div><div className="dialog-scroll">
      <h2 id="gap-prepare-title">Every difference.<br /><em>Carefully understood.</em></h2><p className="review-desk-note">Compare independently supplied records with the saved checkpoint. Another person reviews the frozen assessment before the later return to ordinary work.</p>
      <form ref={formRef} className="portal-form" onSubmit={submit}><fieldset disabled={action.held}>
        <label>Assessment title<input required minLength={5} maxLength={120} value={title} onChange={event => setTitle(event.target.value)} /></label>
        <div className="form-row"><label>Coverage starts<input type="datetime-local" required value={from} onChange={event => setFrom(event.target.value)} /></label><label>Coverage through<input type="datetime-local" required min={from} value={through} onChange={event => setThrough(event.target.value)} /></label></div>
        <label>Evidence verified on<input type="datetime-local" required min={through} value={verified} onChange={event => setVerified(event.target.value)} /></label>
        <label>Coverage evidence<textarea required minLength={10} maxLength={1000} aria-label="Coverage evidence" aria-describedby="gap-coverage-help" value={coverage} onChange={event => setCoverage(event.target.value)} /><span className="form-help" id="gap-coverage-help">Describe the independently checked records and the interval they cover. Entering a book reference does not retrieve or verify it.</span></label>
        <label className="checkbox-label"><input type="checkbox" checked={complete} onChange={event => setComplete(event.target.checked)} />The supplied evidence covers this whole interval.</label>
        {homes.busy ? <p role="status">Opening existing homes…</p> : homes.error ? <div role="alert"><p className="form-error">{homes.error}</p><button type="button" className="button button-light" onClick={() => setHomesRevision(value => value + 1)}>Try existing homes again</button></div> : null}
        <section className="gap-items" aria-label="Supplied item inventory"><div className="gap-items-heading"><h3>Supplied items</h3><span>{rows.length} of 24</span></div>
          {rows.map(({ key, item }, index) => { const n = index + 1; return <article className="gap-item" key={key} data-row={key} aria-label={`Supplied item ${n}`}>
            <div className="gap-items-heading"><h4>Item {n}</h4><button type="button" className="text-button" aria-label={`Remove item ${n}`} onClick={() => remove(key)}>Remove<Icon name="close" /></button></div>
            <div className="form-row"><label>Book reference<input aria-label={`Item ${n} book reference`} required maxLength={80} pattern="[A-Za-z0-9][A-Za-z0-9._-]{0,79}" value={item.book_ref} onChange={event => change(key, { book_ref: event.target.value })} /></label><label>Item reference<input aria-label={`Item ${n} item reference`} required maxLength={80} pattern="[A-Za-z0-9][A-Za-z0-9._-]{0,79}" value={item.item_ref} onChange={event => change(key, { item_ref: event.target.value })} /></label></div>
            <label>Item kind<FormSelect label={`Item ${n} kind`} value={item.kind} onChange={value => change(key, { kind: value as RecoveryGapItem['kind'], entry_ref: '', receipt_number: '', evidence_state: value === 'EVIDENCE' ? item.evidence_state : 'SUPPORTED' })} options={Object.entries(gapKind).map(([value, label]) => ({ value, label }))} /></label>
            <label>Existing home<FormSelect label={`Item ${n} home`} required={item.kind !== 'EVIDENCE' || Boolean(item.entry_ref)} disabled={!homes.data} value={item.flat_id} onChange={value => change(key, { flat_id: value })} options={[{ value: '', label: item.kind === 'EVIDENCE' ? 'No home identified' : 'Choose an existing home' }, ...(homes.data?.items ?? []).map(home => ({ value: home.id, label: home.label }))]} /></label>
            <div className="form-row"><label>Supplied amount (₹)<input aria-label={`Item ${n} amount`} required={item.kind !== 'EVIDENCE' || Boolean(item.entry_ref)} inputMode="decimal" maxLength={12} pattern="[0-9]{1,8}(\.[0-9]{1,2})?" value={item.amount} onChange={event => change(key, { amount: event.target.value })} placeholder={item.kind === 'EVIDENCE' ? 'Optional unless matching an entry' : 'Exact amount, e.g. 250.01'} /></label><label>Supplied calendar date<input aria-label={`Item ${n} date`} type="date" required={item.kind !== 'EVIDENCE'} value={item.date} onChange={event => change(key, { date: event.target.value })} /></label></div>
            {item.kind === 'EVIDENCE' && <><label>Existing entry reference<input aria-label={`Item ${n} existing entry reference`} maxLength={43} minLength={43} value={item.entry_ref} onChange={event => change(key, { entry_ref: event.target.value })} /><span className="form-help">Optional. A matching saved entry contributes no additional money. Unmatched evidence stays unresolved, even when its supporting book is checked.</span></label><label>Evidence state<FormSelect label={`Item ${n} evidence state`} value={item.evidence_state} onChange={value => change(key, { evidence_state: value as RecoveryGapItem['evidence_state'] })} options={Object.entries(gapEvidence).map(([value, label]) => ({ value, label }))} /></label></>}
            {item.kind !== 'DEBIT' && <label>Supplied printed receipt number<input aria-label={`Item ${n} printed receipt number`} maxLength={80} value={item.receipt_number} onChange={event => change(key, { receipt_number: event.target.value })} /><span className="form-help">Optional. Preserve the supplied number; this assessment issues no receipt.</span></label>}
            <label>Independent item evidence<textarea aria-label={`Item ${n} evidence`} required minLength={10} maxLength={300} value={item.evidence} onChange={event => change(key, { evidence: event.target.value })} /></label>
          </article> })}
          <button data-add-item type="button" className="button button-light" disabled={rows.length >= 24} onClick={add}>Add supplied item<Icon name="plus" /></button>
          {!rows.length && <label className="checkbox-label"><input type="checkbox" required checked={empty} onChange={event => setEmpty(event.target.checked)} />I explicitly checked that the complete covered interval has no missing items.</label>}
        </section>
        <section className="gap-preview" aria-label="Known comparison previews"><h3>Saved and supplied. Side by side.</h3>{homeIDs.map(home => <BalancePreview key={home} homeID={home} items={items} />)}<p className="form-help">{complete ? 'Whole-interval evidence supplied.' : 'Coverage is incomplete and remains unresolved.'} {items.filter(item => item.evidence_state !== 'SUPPORTED' || (item.kind === 'EVIDENCE' && !item.entry_ref)).length} unresolved supplied item(s). Empty records alone do not establish an empty gap.</p></section>
        <label>Assessment reason<textarea aria-label="Assessment reason" required minLength={10} maxLength={1000} value={reason} onChange={event => setReason(event.target.value)} /></label>
        <label className="checkbox-label"><input type="checkbox" required checked={confirmed} onChange={event => setConfirmed(event.target.checked)} />I checked the supplied coverage, exact amounts and unresolved evidence.</label>
      </fieldset><RetainedActions action={action} user={user} confirm={confirm} label="Retain financial assessment" retry="Retry same assessment" /></form>
    </div>
  </PortalDialog>
}

function FrozenComparison({ comparison }: { comparison: RecoveryGapComparison }) {
  return <section className="gap-preview" aria-label="Frozen financial comparison"><h3>Saved and supplied. Kept apart.</h3><p className="review-desk-note">{comparison.known_items} supplied known difference(s) · {comparison.unresolved_items} unresolved item(s), including incomplete coverage. Approval retains this assessment; unresolved evidence still needs investigation.</p>{comparison.homes.map(home => <BalanceCard key={home.flat_id} home={home} />)}{comparison.matches.map(match => <p className="form-help" key={match.item}>Item {match.item + 1}: saved {match.entry_kind.toLowerCase()} matched. No additional money included.{match.linked_reversal && ' Its linked correction remains preserved.'}{match.receipt_matched && ' Its original receipt number matches.'}</p>)}</section>
}

function GapReview({ user, preparation, close, saved, correction, confirm, refresh }: Callbacks & { preparation: RecoveryGapPreparation; close: () => void; correction: () => void; refresh: () => void }) {
  const [decision, setDecision] = useState('APPROVED'), [reason, setReason] = useState(''), [attestation, setAttestation] = useState(''), [confirmed, setConfirmed] = useState(false), [page, setPage] = useState(1), [revision, setRevision] = useState(0)
  const history = useRecoveryRead<RecoveryPage<RecoveryGapPreparation>>(`${preparationsPath}/${preparation.id}/history?page=${page}`, revision)
  const action = useRetainedWrite<RecoveryGapDecision, RecoveryGapPreparation>(`${preparationsPath}/${preparation.id}/decision`, saved, refresh, operationsPath)
  const submit = (event: FormEvent) => { event.preventDefault(); if (action.held || !confirmed) return; void action.run({ operation_key: crypto.randomUUID(), scope_key: user.scope_key, version: preparation.version, decision: decision as 'APPROVED' | 'DECLINED', reason, attestation, confirmed }) }
  const input = preparation.input
  return <PortalDialog titleId="gap-review-title" closeLabel="Close frozen financial review" onClose={close} busy={action.held} className="review-desk-dialog gap-dialog"><div className="dialog-heading review-desk-dialog-top"><span className="eyebrow">{gapState[preparation.state]}</span></div><div className="dialog-scroll">
    <h2 id="gap-review-title">{preparation.title}</h2><p className="form-help">Prepared by {preparation.author} · version {preparation.version} · {recoveryDate(preparation.created_at)}</p>
    <FrozenComparison comparison={preparation.comparison} />
    <dl className="review-desk-frozen"><div><dt>Supplied coverage</dt><dd>{recoveryDate(input.coverage_from)} through {recoveryDate(input.coverage_through)}</dd></div><div><dt>Coverage evidence</dt><dd>{input.coverage_evidence}</dd></div><div><dt>Evidence verified</dt><dd>{recoveryDate(input.verified_at)}</dd></div><div><dt>Assessment reason</dt><dd>{input.reason}</dd></div></dl>
    <section className="gap-items" aria-label="Frozen supplied inventory">{input.declared_empty && <p className="review-desk-note">A complete covered interval with no missing items was explicitly supplied.</p>}{input.items.map((item, index) => <article className="gap-item" key={index}><h4>Item {index + 1} · {gapKind[item.kind]}</h4><dl className="review-desk-frozen"><div><dt>Book / item</dt><dd>{item.book_ref} / {item.item_ref}</dd></div><div><dt>Supplied home</dt><dd>{preparation.comparison.homes.find(home => home.flat_id === item.flat_id)?.home ?? 'No home identified'}</dd></div><div><dt>Supplied amount</dt><dd>{item.amount ? `₹${item.amount}` : 'Unspecified evidence-only amount'}</dd></div><div><dt>Supplied calendar date</dt><dd>{item.date || 'Not supplied'}</dd></div>{item.entry_ref && <div><dt>Saved entry reference</dt><dd>{item.entry_ref}</dd></div>}{item.receipt_number && <div><dt>Supplied printed receipt</dt><dd>{item.receipt_number}</dd></div>}<div><dt>Evidence status</dt><dd>{gapEvidence[item.evidence_state]}</dd></div><div><dt>Independent evidence</dt><dd>{item.evidence}</dd></div></dl></article>)}</section>
    {preparation.decision && <div className="review-desk-decision"><span className="eyebrow">SEPARATE DECISION RETAINED</span><h3>{preparation.decision.actor} · {preparation.decision.decision.toLowerCase()}</h3><p>{preparation.decision.reason}</p><p>{preparation.decision.attestation}</p></div>}
    {preparation.state === 'PENDING' ? preparation.author_id === user.id ? <p className="review-desk-note">You prepared this assessment. Another current reviewer checks and decides the exact version.</p> : <form className="portal-form" onSubmit={submit}><fieldset disabled={action.held}>
      <label>Financial assessment decision<FormSelect label="Financial assessment decision" value={decision} onChange={setDecision} options={[{ value: 'APPROVED', label: 'Approve supplied assessment' }, { value: 'DECLINED', label: 'Decline supplied assessment' }]} /></label><label>Decision reason<textarea aria-label="Decision reason" required minLength={10} maxLength={1000} value={reason} onChange={event => setReason(event.target.value)} /></label><label>Independent review attestation<textarea aria-label="Independent review attestation" required minLength={10} maxLength={300} value={attestation} onChange={event => setAttestation(event.target.value)} /></label><label className="checkbox-label"><input type="checkbox" required checked={confirmed} onChange={event => setConfirmed(event.target.checked)} />I independently checked this frozen assessment and retained unresolved evidence.</label>
    </fieldset><RetainedActions action={action} user={user} confirm={confirm} label="Retain financial decision" retry="Retry same decision" /></form> : preparation.state !== 'SUPERSEDED' && <button type="button" className="button button-light" onClick={correction}>Prepare linked assessment<Icon name="arrow" /></button>}
    <section className="review-desk-history"><h3>Every assessment, retained.</h3>{history.busy ? <p role="status">Opening financial history…</p> : history.error ? <div role="alert"><p className="form-error">{history.error}</p><button type="button" className="button button-light" onClick={() => setRevision(value => value + 1)}>Try financial history again</button></div> : <>{history.data?.items.map(item => <article key={item.id}><strong>Version {item.version} · {gapState[item.state]}</strong><p>{item.comparison.known_items} known difference(s) · {item.comparison.unresolved_items} unresolved item(s)</p><span>{item.author} · {recoveryDate(item.created_at)}</span>{item.decision && <p>{item.decision.actor}: {item.decision.reason}</p>}</article>)}<Pager page={page} total={history.data?.total ?? 0} onPage={setPage} /></>}</section>
  </div></PortalDialog>
}

export function RecoveryGap({ user, sourceCreatedAt, confirm, changed }: { user: User; sourceCreatedAt: number; confirm: () => void; changed: () => void }) {
  const [page, setPage] = useState(1), [state, setState] = useState(''), [revision, setRevision] = useState(0), [selected, setSelected] = useState(''), [preparing, setPreparing] = useState(false), [predecessor, setPredecessor] = useState<RecoveryGapPreparation | null>(null), [feedback, setFeedback] = useState('')
  const feedbackRef = useRef<HTMLParagraphElement>(null)
  useLayoutEffect(() => { if (feedback) feedbackRef.current?.focus() }, [feedback])
  const status = useRecoveryRead<RecoveryGapStatus>(base + '/status', revision)
  const results = useRecoveryRead<RecoveryPage<RecoveryGapPreparation>>(`${preparationsPath}?${new URLSearchParams({ page: String(page), state })}`, revision)
  const detail = useRecoveryRead<RecoveryGapPreparation>(selected ? `${preparationsPath}/${selected}` : '', revision)
  const refresh = () => setRevision(value => value + 1)
  const saved = (value: RecoveryGapPreparation) => { setPreparing(false); setSelected(''); setPredecessor(null); setState(''); setPage(1); setFeedback(value.state === 'PENDING' ? 'Financial assessment retained for another reviewer.' : 'Separate financial decision retained. Unresolved evidence stays visible.'); refresh(); changed() }
  return <section className="review-desk-register" aria-label="Supplied financial recovery assessments">
    <div className="review-desk-heading"><div><span className="eyebrow">BETWEEN THE CHECKPOINT AND TODAY</span><h2>Every difference has a story.</h2><p>Compare independently supplied records. Preserve the saved money, original receipts and explicit uncertainty.</p></div><button type="button" className="button button-dark" disabled={!status.data || !sourceCreatedAt} onClick={() => { setPredecessor(null); setPreparing(true) }}>Prepare financial assessment<Icon name="plus" /></button></div>
    {status.busy ? <p role="status">Checking financial assessments…</p> : status.error ? <div className="review-desk-error" role="alert"><p>{status.error}</p><button type="button" className="button button-light" onClick={refresh}>Try financial status again</button></div> : status.data && <div className="numbering-summary" aria-label="Current financial review progress"><article><strong>{status.data.pending}</strong><span>awaiting another reviewer</span></article><article><strong>{status.data.known}</strong><span>supplied known differences</span></article><article><strong>{status.data.unresolved}</strong><span>items and coverage to investigate</span></article></div>}
    {feedback && <p className="review-desk-feedback" role="status" tabIndex={-1} ref={feedbackRef}>{feedback}</p>}
    <div className="review-desk-filters"><FilterSelect label="Financial assessment state" value={state} onChange={value => { setState(value); setPage(1) }} options={[{ value: '', label: 'Every financial assessment' }, ...Object.entries(gapState).map(([value, label]) => ({ value, label }))]} /><button type="button" className="text-button" onClick={refresh}>Refresh financial assessments<Icon name="refresh" /></button></div>
    {results.busy ? <p className="review-desk-empty" role="status">Opening financial assessments…</p> : results.error ? <div className="review-desk-error" role="alert"><p>{results.error}</p><button type="button" className="button button-light" onClick={refresh}>Try financial list again</button></div> : results.data?.items.length === 0 ? <div className="review-desk-empty"><Icon name="leaf" /><h3>{state ? 'No financial assessments in this view.' : 'Room for a careful comparison.'}</h3><p>{state ? 'Choose another state to see retained assessments.' : 'Supply the covered interval and records. No submitted items does not establish that nothing was lost.'}</p></div> : <div className="review-desk-list">{results.data?.items.map(item => <article key={item.id}><span className="review-desk-symbol"><Icon name="records" /></span><div><span className="eyebrow">{gapState[item.state]}</span><h3>{item.title}</h3><p>{item.comparison.known_items} known difference(s) · {item.comparison.unresolved_items} unresolved item(s)</p><span>{item.author} · version {item.version}</span></div><button type="button" className="button button-light" onClick={() => setSelected(item.id)}>Open financial review<Icon name="arrow" /></button></article>)}</div>}
    <Pager page={page} total={results.data?.total ?? 0} onPage={setPage} />
    {preparing && <GapPreparation user={user} sourceCreatedAt={sourceCreatedAt} predecessor={predecessor} close={() => { setPreparing(false); setPredecessor(null) }} saved={saved} confirm={confirm} />}
    {selected && (detail.busy || detail.error ? <PortalDialog titleId="gap-opening-title" closeLabel="Close frozen financial review" onClose={() => setSelected('')} className="review-desk-dialog"><div className="dialog-heading review-desk-dialog-top"><span className="eyebrow">CURRENT FINANCIAL REVIEW</span></div><div className="dialog-scroll"><h2 id="gap-opening-title">{detail.busy ? 'Opening this assessment…' : 'A little pause.'}</h2>{detail.busy ? <p role="status">Checking the exact retained version.</p> : <><p className="form-error" role="alert">{detail.error}</p><button type="button" className="button button-dark" onClick={refresh}>Try financial review again</button></>}</div></PortalDialog> : detail.data && <GapReview key={`${detail.data.id}:${detail.data.version}`} user={user} preparation={detail.data} close={() => setSelected('')} saved={saved} correction={() => { setPredecessor(detail.data!); setPreparing(true); setSelected('') }} confirm={confirm} refresh={refresh} />)}
  </section>
}
