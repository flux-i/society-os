import { useEffect, useState } from 'react'
import { useBudgetLoad, useBudgetWrite } from '../budgets'
import type { BudgetDetail, PaidExpenseDetail } from '../budgets'
import { BudgetPlanCard, PaidExpenseOriginal, BudgetReadError, BudgetWriteFeedback } from './BudgetShared'
import { PortalDialog } from './PortalDialog'
import { FilterSelect } from './FilterSelect'
import { Icon } from './Icon'

export function BudgetDecision({kind,id,withdraw=false,onClose,onSaved}:{kind:'budget'|'expense';id:string;withdraw?:boolean;onClose:()=>void;onSaved:()=>void}){
 const base=kind==='budget'?'/api/budgets/':'/api/paid-expenses/',load=useBudgetLoad<BudgetDetail|PaidExpenseDetail>(base+encodeURIComponent(id)),write=useBudgetWrite()
 const [action,setAction]=useState(withdraw?'CANCELLED':'APPROVED'),[reason,setReason]=useState(''),[checked,setChecked]=useState(false)
 const visible=!load.loading&&!load.error?load.data:null,canAct=!!visible&&(withdraw?visible.can_cancel:visible.can_decide)
 useEffect(()=>{if(write.reauth)setChecked(false)},[write.reauth])
 const reload=()=>{write.reset();setChecked(false);load.reload()}
 const submit=async()=>{if(!visible||!checked||(!canAct&&!write.locked))return;const result=await write.send(base+encodeURIComponent(id)+'/decisions',{version:visible.version,action,reason});if(result)onSaved()}
 const approveLabel=kind==='budget'?(visible?.snapshot.action==='CLOSE'?'Approve period closure':'Approve supplied plan'):'Confirm paid expense',label=withdraw?'Withdraw proposal':action==='DECLINED'?'Decline proposal':approveLabel
 return <PortalDialog titleId="budget-decision-heading" closeLabel={kind==='budget'?'Close budget decision':'Close expense decision'} onClose={onClose} busy={write.busy||(write.locked&&!write.conflict)} className="budget-dialog"><div className="dialog-scroll"><span className="eyebrow">A SEPARATE PAIR OF EYES</span><h2 id="budget-decision-heading">{withdraw?'Keep the record. Withdraw the proposal.':kind==='budget'?'A considered plan. A clear decision.':'Review the money already paid.'}</h2>
 {load.loading?<p className="empty-state" role="status">Opening the current proposal…</p>:load.error?<BudgetReadError title="This proposal could not be opened." error={load.error} onRetry={load.reload}/>:visible&&<>
 {kind==='budget'?<BudgetPlanCard plan={(visible as BudgetDetail).snapshot} label="Exact proposed plan"/>:<PaidExpenseOriginal expense={(visible as PaidExpenseDetail).snapshot} label="Exact proposed paid record"/>}
 {visible.approved&&visible.snapshot.version!==visible.approved.version&&<p className="budget-retained-note"><Icon name="shield"/>The accepted predecessor remains effective until a separate approval.</p>}
 <p className="form-help">Proposal version {visible.snapshot.version} · Current record version {visible.version}. {withdraw?'Withdrawal preserves the accepted predecessor and retained history.':kind==='expense'?'The reviewed original or linked correction becomes the effective paid record.':'Approval makes this exact supplied plan effective.'}</p>
 {!canAct&&!write.locked&&<div className="form-error" role="status"><p>This proposal is no longer available for this decision. Return to the current record.</p></div>}
 <form onSubmit={event=>{event.preventDefault();void submit()}}><fieldset disabled={write.busy||write.locked||!canAct}>
 {!withdraw&&<label>Decision<FilterSelect label="Budget decision" value={action} options={[{value:'APPROVED',label:approveLabel},{value:'DECLINED',label:'Decline proposal'}]} onChange={value=>{setAction(value);setChecked(false)}}/></label>}
 <label>Decision reason<textarea aria-label="Decision reason" value={reason} onChange={event=>{setReason(event.target.value);setChecked(false)}} required minLength={10} maxLength={800} rows={4}/></label>
 <label className="confirmation"><input type="checkbox" checked={checked} onChange={event=>setChecked(event.target.checked)}/>I reviewed this exact proposal, source and decision’s effect.</label>
 </fieldset><BudgetWriteFeedback write={write} onReload={reload}/><div className="form-actions"><button type="button" className="button button-quiet" disabled={write.busy||(write.locked&&!write.conflict)} onClick={onClose}>Back to record</button><button type="submit" className="button button-dark" disabled={write.busy||write.conflict||!checked||(!canAct&&!write.locked)}>{write.busy?'Saving…':write.locked?'Retry this decision':label}<Icon name="check"/></button></div></form>
 </>}
 </div></PortalDialog>
}
