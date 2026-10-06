import type { StatementEvent, StatementFile } from '../statements'
import { statementEvents, statementKinds, statementSize } from '../statements'
import { displayDate } from '../maintenance'
import { careTime } from '../upkeep'
import { PageControls } from './Maintenance'

export function StatementPaper({file}:{file:StatementFile}) {
 return <section className="statement-paper" aria-label="Prepared original statement"><span className="eyebrow">{statementKinds[file.kind]}{file.revision>0?' · Revision '+file.revision:''}</span><h3>{file.title}</h3><dl><div><dt>Period</dt><dd>{displayDate(file.period_start)} – {displayDate(file.period_end)}</dd></div><div><dt>Prepared by</dt><dd>{file.prepared_by}</dd></div><div><dt>Source reference</dt><dd className="preserve-lines">{file.source}</dd></div><div><dt>Original file</dt><dd>{file.filename} · {statementSize(file.size_bytes)}</dd></div></dl><p className="form-help">Figures belong to this supplied original. Uploading, approving or publishing it creates no ledger entry or receipt.</p></section>
}
export function StatementHistory({events,total=events.length,page=1,onPage,disabled=false,publication=false}:{events:StatementEvent[];total?:number;page?:number;onPage?:(page:number)=>void;disabled?:boolean;publication?:boolean}) {
 const actions:Record<string,string>={PROPOSED:'Publication proposed',PUBLISHED:'Publication approved',DECLINED:'Publication declined',WITHDRAWN:'Publication withdrawn',REVOKED:'Publication removed',SUPERSEDED:'Replaced by a separately approved publication'}
 return <section className="review-history statement-history" aria-label={publication?'Publication decisions':'Statement activity'}><h3>{publication?'The publication decisions.':'Each step, kept together.'}</h3><ol>{events.map(e=><li key={e.version}><strong>{(publication?actions:statementEvents)[e.action]??e.action}</strong><p className="preserve-lines">{e.reason}</p><small>{e.actor} · {careTime(e.at)}</small></li>)}</ol>{total>20&&onPage&&<PageControls label="statement activity" page={page} total={total} size={20} onPage={onPage} disabled={disabled}/>}</section>
}
