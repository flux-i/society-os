import { useEffect, useRef, useState } from 'react'
import type { User } from './api'
import { APIError, mutate } from './api'
export { useFundLoad as useBudgetLoad } from './collections'

export const canReadBudgets = (user:User) => !user.mfa_pending&&(user.can_manage_records||user.roles.includes('AUDITOR'))
export type BudgetSnapshot = { version:number; plan_version:number; action:string; title:string; period_start:string; period_end:string; source_reference:string; collections_paise:number; expenses_paise:number; submitted_by:string; submitted_at:number; reason:string }
export type BudgetResource = { id:string; version:number; decision:string; state:string; snapshot:BudgetSnapshot; approved?:BudgetSnapshot; can_revise:boolean; can_decide:boolean; can_cancel:boolean; can_close:boolean; can_add_expense:boolean }
export type BudgetEvent = { version:number; proposal_version:number; action:string; actor:string; reason:string; at:number; snapshot:BudgetSnapshot }
export type BudgetDetail = BudgetResource & { events:BudgetEvent[]; event_total:number; event_page:number; page_size:number; current_key:string }
export type BudgetPage = { items:BudgetResource[]; total:number; page:number; page_size:number; counts:Record<string,number>; can_prepare:boolean; current_key:string }
export type BudgetComparison = { available:boolean; period_start:string; period_end:string; plan_version:number; planned_collections_paise:number; planned_expenses_paise:number; original_collections_paise:number; reversed_collections_paise:number; collections_paise:number; recorded_expenses_paise:number; collection_variance_paise:number; expense_headroom_paise:number; difference_paise:number; receipt_count:number; expense_count:number; pending_expenses:number; current_key:string; checked_at:number }
export type PaidExpenseSnapshot = { version:number; action:string; budget_id:string; budget_version:number; previous_version:number; amount_paise:number; paid_date:string; payee:string; category:string; method:string; reference:string; source_note:string; submitted_by:string; submitted_at:number; reason:string }
export type PaidExpenseResource = { id:string; budget_id:string; version:number; decision:string; state:string; snapshot:PaidExpenseSnapshot; approved?:PaidExpenseSnapshot; can_revise:boolean; can_decide:boolean; can_cancel:boolean; can_correct:boolean; can_void:boolean }
export type PaidExpenseEvent = { version:number; proposal_version:number; action:string; actor:string; reason:string; at:number; snapshot:PaidExpenseSnapshot }
export type PaidExpenseDetail = PaidExpenseResource & { budget_title:string; events:PaidExpenseEvent[]; event_total:number; event_page:number; page_size:number; current_key:string }
export type PaidExpensePage = { items:PaidExpenseResource[]; total:number; page:number; page_size:number; counts:Record<string,number>; current_key:string }
export const budgetDecisions:Record<string,string> = { PENDING:'Awaiting separate review',APPROVED:'Approved',DECLINED:'Declined',CANCELLED:'Proposal withdrawn' }
export const budgetActions:Record<string,string> = { PLAN:'Operating plan',CLOSE:'Close the period',PAID:'Already-paid expense',CORRECTION:'Linked correction',VOID:'Remove this record’s effect',PROPOSED:'Proposed',REVISED:'Proposal revised',APPROVED:'Separately approved',DECLINED:'Declined',CANCELLED:'Proposal withdrawn' }
export const expenseMethods:Record<string,string> = { CASH:'Cash',BANK_TRANSFER:'Bank transfer',CHEQUE:'Cheque',UPI:'UPI' }
export const budgetLink = (key:string) => new URLSearchParams(window.location.hash.split('?')[1]??'').get(key)??''
export const moneyInput = (paise:number) => Math.trunc(paise/100)+'.'+String(paise%100).padStart(2,'0')
export const previewPaise = (text:string,zero=true):number|null => {
 if(!/^(0|[1-9][0-9]{0,7})(\.[0-9]{1,2})?$/.test(text))return null
 const [whole,fraction='']=text.split('.'),value=Number(whole)*100+Number(fraction.padEnd(2,'0'))
 return value<=1000000000&&(zero?value>=0:value>0)?value:null
}
export const todayInSociety = () => new Intl.DateTimeFormat('en-CA',{timeZone:'Asia/Kolkata',year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date())

// A duplicate identifier is a definite rejection. Uncertain writes keep their
// frozen operation until the same request succeeds or a stale record is reloaded.
export function useBudgetWrite(){
 const pending=useRef<{path:string;payload:Record<string,unknown>}|null>(null),inFlight=useRef(false),feedback=useRef<HTMLDivElement>(null)
 const [busy,setBusy]=useState(false),[locked,setLocked]=useState(false),[error,setError]=useState(''),[conflict,setConflict]=useState(false),[reauth,setReauth]=useState(false)
 useEffect(()=>{if(error)feedback.current?.scrollIntoView({block:'nearest'})},[error])
 const reset=()=>{pending.current=null;setLocked(false);setError('');setConflict(false);setReauth(false)}
 const send=async(path:string,payload:Record<string,unknown>)=>{
  if(inFlight.current)return null
  inFlight.current=true;setBusy(true);setError('');setReauth(false)
  pending.current??={path,payload:{...payload,operation_key:crypto.randomUUID(),confirmed:true}}
  try{const result=await mutate<{id:string}>(pending.current.path,'POST',pending.current.payload);reset();return result}
  catch(err){const duplicate=err instanceof APIError&&err.code==='duplicate_paid_expense',rejected=err instanceof APIError&&([400,403,404,428].includes(err.status)||duplicate);if(rejected)pending.current=null;setError((err as Error).message);setLocked(!rejected);setConflict(err instanceof APIError&&err.status===409&&!duplicate);setReauth(err instanceof APIError&&err.code==='reauthentication_required');return null}
  finally{inFlight.current=false;setBusy(false)}
 }
 return {busy,locked,error,conflict,reauth,feedback,reset,send}
}

export type BudgetDetailView = { expense_page:number; event_page:number; query:string; state:string }
