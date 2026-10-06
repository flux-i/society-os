import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { fineActions, useFineLoad, useFineWrite } from '../fines'
import type { FineNotice, FineSource } from '../fines'
import { careTime } from '../upkeep'
import { displayDate, money } from '../maintenance'
import type { FundEvent } from '../collections'
import { IncidentFeedback } from './IncidentShared'
import { FundCheck, FundUnavailable } from './FundShared'
import { PageControls } from './Maintenance'
import { RuleText } from './IncidentRules'
import { Icon } from './Icon'
export { FundCheck as FineCheck, FundUnavailable as FineUnavailable }
export function FineFeedback({ writer, onReload }: { writer: ReturnType<typeof useFineWrite>; onReload?: () => void }) {
  return <><IncidentFeedback writer={writer} onReload={onReload} />{writer.reauth && <a className="text-link" href="#security">Confirm your identity in Account security<Icon name="arrow" /></a>}</>
}
export function FineFacts({ children }: { children: ReactNode }) { return <div className="incident-review-facts fine-facts">{children}</div> }
export function FineHistory({ events, total, page, onPage, disabled = false }: { events: FundEvent[]; total: number; page: number; onPage: (page: number) => void; disabled?: boolean }) {
  return <section className="review-history upkeep-history" aria-label="Fine activity"><h3>Each decision, kept together.</h3><ol>{events.map(event => <li key={event.version}><strong>{fineActions[event.action] ?? event.action}</strong><p>{event.reason}</p><small>{event.actor} · {careTime(event.at)}</small></li>)}</ol>{total > 20 && <PageControls label="fine activity" page={page} total={total} size={20} onPage={onPage} disabled={disabled} />}</section>
}
export function FineNoticePaper({ notice, proposed = false }: { notice: Pick<FineNotice, 'home' | 'title' | 'body' | 'amount_paise' | 'policy_reference' | 'due_date' | 'response_by'>; proposed?: boolean }) {
  return <section className="fine-notice-paper" aria-label={proposed ? 'Proposed household notice' : 'Approved household notice'}><span className="eyebrow">{proposed ? 'HOUSEHOLD PREVIEW' : 'APPROVED HOUSEHOLD WORDING'} · HOME {notice.home}</span><h3>{notice.title}</h3><p className="preserve-lines">{notice.body}</p><dl><div><dt>{proposed ? 'Proposed amount' : 'Originally notified amount'}</dt><dd>{money(notice.amount_paise)}</dd></div><div><dt>Response date</dt><dd>{displayDate(notice.response_by)}</dd></div><div><dt>Supplied due date</dt><dd>{displayDate(notice.due_date)}</dd></div><div><dt>Supplied authority</dt><dd>{notice.policy_reference}</dd></div></dl><p className="form-help">Approving this wording creates no charge. Current balances follow a separate issuance and any reviewed corrections.</p></section>
}
export function FineSourcePaper({ source, disabled = false, onContextChange, onReviewState }: { source: FineSource; disabled?: boolean; onContextChange?: () => void; onReviewState?: (blocked: boolean) => void }) {
  const [page, setPage] = useState(1), load = useFineLoad<FineSource>('/api/fine-sources/' + encodeURIComponent(source.incident_id) + '?response_page=' + page, page > 1)
  const [invalidated, setInvalidated] = useState(false)
  const x = page === 1 ? source : load.data
  useEffect(() => { setPage(1); setInvalidated(false) }, [source.source_key])
  const changed = page > 1 && x && x.source_key !== source.source_key
  useEffect(() => { if (changed) setInvalidated(true) }, [changed])
  const blocked = invalidated || !!changed || (page > 1 && (load.loading || !!load.error || !x || x.response_page !== page))
  useEffect(() => { onReviewState?.(blocked) }, [blocked, onReviewState])
  return <section className="fine-source-paper" aria-label="Supplied incident decision"><FineFacts><span>Tagged home<strong>{source.home}</strong></span><span>Incident date<strong>{displayDate(source.incident_date)}</strong></span></FineFacts><RuleText rule={source.rule} /><p className="form-help">The original operational case retains its private observations and reporter. This financial review uses the supplied rule, substantiated decision and deliberately shared responses.</p>{page > 1 && (load.error || load.loading) ? <FundUnavailable error={load.error} loading={load.loading} onRetry={load.reload}><span /></FundUnavailable> : (changed || invalidated) ? <div className="form-error" role="alert"><p>The source has changed. Reload this fine before reviewing the new responses.</p>{onContextChange && <button type="button" className="text-link" disabled={disabled} onClick={onContextChange}>Reload current context<Icon name="refresh" /></button>}</div> : x?.notice && <><section className="fine-source-notice"><h3>{x.notice.title}</h3><p className="preserve-lines">{x.notice.body}</p><small>Operational response date · {displayDate(x.notice.response_by)}</small></section><ResponseList responses={x.notice.responses} label="Responses to the operational notice" /></>}{source.response_total > 20 && <PageControls label="source responses" page={page} total={source.response_total} size={20} onPage={value => { onReviewState?.(true); setPage(value) }} disabled={disabled} />}</section>
}
export function ResponseList({ responses, label }: { responses: FineNotice['responses']; label: string }) {
  return <section aria-label={label} className="fine-responses"><h3>{label}</h3>{responses?.length ? <ol>{responses.map(item => <li key={item.id}><p className="preserve-lines">{item.body}</p><small>{item.actor ? item.actor + ' · ' : ''}{careTime(item.created_at)}</small></li>)}</ol> : <p className="form-help">No retained responses in this view yet.</p>}</section>
}
