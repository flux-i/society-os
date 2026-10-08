import { useState } from 'react'
import { budgetActions, budgetDecisions, useBudgetLoad } from '../budgets'
import type { BudgetResource, PaidExpenseDetail as Detail, PaidExpenseResource } from '../budgets'
import { money } from '../maintenance'
import { careTime } from '../upkeep'
import { PaidExpenseOriginal, BudgetReadError } from './BudgetShared'
import { BudgetDecision } from './BudgetDecision'
import { PortalDialog } from './PortalDialog'
import { PageControls } from './Maintenance'
import { Icon } from './Icon'

export function PaidExpenseDetail({id,eventPage,onEventPage,onClose,onChanged,onEdit}:{id:string;eventPage:number;onEventPage:(page:number)=>void;onClose:()=>void;onChanged:()=>void;onEdit:(budget:BudgetResource,expense:PaidExpenseResource,action:'PAID'|'CORRECTION'|'VOID')=>void}){
 const [revision,setRevision]=useState(0),[decision,setDecision]=useState<'review'|'withdraw'|null>(null)
 const load=useBudgetLoad<Detail>('/api/paid-expenses/'+encodeURIComponent(id)+'?event_page='+eventPage+'&refresh='+revision),data=!load.loading&&!load.error?load.data:null
 const budget=useBudgetLoad<BudgetResource>('/api/budgets/'+encodeURIComponent(data?.budget_id??''),!!data),parent=!budget.loading&&!budget.error?budget.data:null
 const refresh=()=>{setRevision(v=>v+1);onChanged()}
 if(decision)return <BudgetDecision kind="expense" id={id} withdraw={decision==='withdraw'} onClose={()=>setDecision(null)} onSaved={()=>{setDecision(null);refresh()}}/>
 return <PortalDialog titleId="paid-expense-detail-heading" closeLabel="Close paid expense details" onClose={onClose} className="budget-dialog"><div className="dialog-scroll"><span className="eyebrow">AN ORIGINAL, WITH ITS CLEAR TRAIL</span><h2 id="paid-expense-detail-heading">{data?(data.approved??data.snapshot).payee:'Opening the paid record…'}</h2>
 {load.loading?<p className="empty-state" role="status">Opening the current paid record…</p>:load.error?<BudgetReadError title="This paid record could not be opened." error={load.error} onRetry={load.reload}/>:data&&<>
 <p className="dialog-description">{data.budget_title} · original budget version {data.snapshot.budget_version}</p><div className="budget-record-status"><span className="contact-state">{data.state==='CONFIRMED'?'Effective paid record':data.state==='VOID'?'Recorded effect removed':'No accepted paid record yet'}</span>{data.decision!=='APPROVED'&&<span>{budgetDecisions[data.decision]}</span>}</div>
 {data.approved&&<PaidExpenseOriginal expense={data.approved} label="Current accepted effect"/>}{(!data.approved||data.approved.version!==data.snapshot.version)&&<><PaidExpenseOriginal expense={data.snapshot} label={data.decision==='PENDING'?'Proposal awaiting separate review':'Latest retained proposal'}/><p className="budget-proposal-reason"><span>Proposal reason</span>{data.snapshot.reason}</p></>}
 <div className="budget-record-actions">{data.can_decide&&<button type="button" className="button button-dark" onClick={()=>setDecision('review')}>Review paid expense<Icon name="check"/></button>}{data.can_cancel&&<button type="button" className="button button-quiet" onClick={()=>setDecision('withdraw')}>Withdraw proposal</button>}{data.can_revise&&<button type="button" className="button button-quiet" disabled={!parent} onClick={()=>parent&&onEdit(parent,data,data.snapshot.action as 'PAID'|'CORRECTION'|'VOID')}>Revise expense proposal</button>}{data.can_correct&&<button type="button" className="button button-quiet" disabled={!parent} onClick={()=>parent&&onEdit(parent,data,'CORRECTION')}>Prepare linked correction</button>}{data.can_void&&<button type="button" className="text-link" disabled={!parent} onClick={()=>parent&&onEdit(parent,data,'VOID')}>Remove this record’s effect<Icon name="arrow"/></button>}</div>
 {budget.error&&<div className="connection-error" role="alert"><p>The original budget context is unavailable. Paid-record history remains open.</p><button type="button" onClick={budget.reload}>Retry original budget<Icon name="refresh"/></button></div>}
 <button type="button" className="text-link" onClick={onClose}>Return to budget period<Icon name="arrow"/></button><section className="budget-history" aria-label="Paid expense decision history"><h3>Every decision stays on record.</h3>{data.events.map(event=><article key={event.version}><div><span>{budgetActions[event.action]} · record version {event.version}</span><strong>{money(event.snapshot.amount_paise)}</strong></div><p>{event.reason}</p><small>{event.actor} · {careTime(event.at)} · proposal version {event.proposal_version}</small><details><summary>Read retained paid snapshot</summary><PaidExpenseOriginal expense={event.snapshot} label={'Retained version '+event.proposal_version}/></details></article>)}{data.event_total>12&&<PageControls label="paid expense history" page={eventPage} total={data.event_total} size={12} onPage={onEventPage}/>}</section>
 </>}
 </div></PortalDialog>
}
