import { useEffect, useRef, useState } from 'react'
import { APIError, mutate, request } from './api'
import type { FinancialHome, StatementEntry } from './maintenance'
export type FundTotals = { requested_paise:number; active_paise:number; allocated_paise:number; outstanding_paise:number; overdue_paise:number; waived_paise:number; voluntary_paise:number; pending_reports:number; paid_homes:number; partial_homes:number; unpaid_homes:number; exempt_homes:number }
export type Fund = FundTotals & { id:string; title:string; purpose:string; contribution_type:string; start_date:string; due_date:string; target_paise:number; source_reference?:string; note?:string; state:string; version:number; author_id?:string; author?:string; created_at:number; decided_by?:string; decided_at:number; decision_reason?:string; participants:number }
export type FundLine = { flat_id:string; home:string; requested_paise:number; active_paise:number; allocated_paise:number; outstanding_paise:number; waived_paise:number; voluntary_paise:number; pending_reports:number; entry_id:string; original_entry_id:string; version:number; status:string }
export type FundEvent = { action:string; actor:string; reason:string; version:number; at:number }
export type FundDetail = Fund & { lines:FundLine[]; line_page:number; page_size:number; events?:FundEvent[]; event_total?:number; event_page?:number }
export type FundPage = { items:Fund[]; homes:FinancialHome[]; totals:FundTotals; total:number; page:number; page_size:number }
export type PaymentReport = { id:string; campaign_id:string; campaign:string; flat_id:string; home:string; author_id:string; author:string; amount_paise:number; payment_date:string; payer:string; method:string; reference:string; comment:string; evidence_id:string; state:string; version:number; created_at:number; updated_at:number; entry_id:string; allocation_id:string; duplicate_report_id:string; reviewer:string; reviewed_at:number; decision_reason:string; receipt_id:string; receipt:string; current_state:string; events:FundEvent[]; event_total:number; event_page:number; page_size:number }
export type ReportPage = { items:PaymentReport[]; homes:FinancialHome[]; total:number; page:number; page_size:number; counts:Record<string,number> }
export type ConfirmOptions = { entries:(StatementEntry & {verification_source?:string;payment_identity?:string})[]; charge_id:string; outstanding_paise:number; contribution_type:string; campaign_state:string }
export type FundWaiver = { id:string; campaign_id:string; campaign:string; flat_id:string; home:string; participant_version:number; amount_paise:number; source_reference:string; reason:string; author_id:string; author:string; state:string; version:number; created_at:number; reviewer:string; reviewed_at:number; decision_reason:string; replacement_entry_id:string; events:FundEvent[]; event_total:number; event_page:number; page_size:number }
export type WaiverPage = { items:FundWaiver[]; total:number; page:number; page_size:number }
export type Contribution = { id:string; campaign_id:string; campaign:string; flat_id:string; home:string; source_id:string; receipt_id:string; receipt:string; amount_paise:number; state:string; actor?:string; reason?:string; created_at:number; correction_reason?:string; corrected_by?:string; corrected_at?:number }
export type ContributionPage = { items:Contribution[]; total:number; page:number; page_size:number }
export const fundStates:Record<string,string> = { PENDING:'Awaiting review', PUBLISHED:'Open', CLOSED:'Closed', DECLINED:'Declined', WITHDRAWN:'Withdrawn' }
export const reportStates:Record<string,string> = { PENDING:'Awaiting verification', NEEDS_INFO:'More information needed', REJECTED:'Not verified', WITHDRAWN:'Withdrawn', CONFIRMED:'Verified', DUPLICATE:'Already recorded', CORRECTED:'Linked record corrected' }
export const lineStates:Record<string,string> = { PROPOSED:'Proposed', UNPAID:'Unpaid', PART_PAID:'Part paid', PAID:'Paid', EXEMPT:'Exempt', CORRECTED:'Charge corrected', NO_CONTRIBUTION:'No confirmed contribution', CONFIRMED:'Confirmed contribution' }
export const fundActions:Record<string,string> = { PUBLISHED:'Publish fund', DECLINED:'Decline proposal', WITHDRAWN:'Withdraw', CLOSED:'Close new reports', REOPEN:'Reopen new reports', APPROVED:'Approve exemption', NEEDS_INFO:'Ask for more information', REJECTED:'Decline verification', CONFIRMED:'Verify received payment', REPORTED:'Payment reported', REVISED:'Report revised', SUBMITTED:'Proposal submitted', ATTRIBUTED:'Credit assigned to purpose', REVERSED:'Purpose assignment corrected' }
export const paymentMethods:Record<string,string> = { CASH:'Cash', BANK_TRANSFER:'Bank transfer', CHEQUE:'Cheque', UPI:'UPI' }
export const selectOptions = (labels:Record<string,string>) => Object.entries(labels).map(([value,label])=>({value,label}))
export const rupees = (paise:number) => (paise/100).toFixed(2)
export const fundLink = (key:string) => new URLSearchParams(window.location.hash.split('?')[1]??'').get(key)??''
export function useFundLoad<T>(path:string,enabled=true){
 const [data,setData]=useState<T|null>(null),[loading,setLoading]=useState(enabled),[error,setError]=useState(''),[revision,setRevision]=useState(0)
 useEffect(()=>{if(!enabled){setData(null);setLoading(false);return}const controller=new AbortController();setLoading(true);setError('');const timer=setTimeout(()=>request<T>(path,controller.signal).then(value=>{setData(value);setLoading(false)}).catch((err:Error)=>{if(!controller.signal.aborted){setError(err.message);setLoading(false)}}),0);return()=>{clearTimeout(timer);controller.abort()}},[path,enabled,revision])
 return {data,loading,error,reload:()=>setRevision(value=>value+1)}
}
export function useFundWrite(){
 const pending=useRef<{path:string;payload:Record<string,unknown>}|null>(null),inFlight=useRef(false),feedback=useRef<HTMLDivElement>(null)
 const [busy,setBusy]=useState(false),[locked,setLocked]=useState(false),[error,setError]=useState(''),[conflict,setConflict]=useState(false)
 useEffect(()=>{if(error)feedback.current?.scrollIntoView({block:'nearest'})},[error])
 const reset=()=>{pending.current=null;setLocked(false);setError('');setConflict(false)}
 const send=async(path:string,payload:Record<string,unknown>)=>{
  if(inFlight.current)return null;inFlight.current=true;setBusy(true);setError('')
  pending.current??={path,payload:{...payload,operation_key:crypto.randomUUID(),confirmed:true}}
  try{const result=await mutate<{id:string}>(pending.current.path,'POST',pending.current.payload);reset();return result}
  catch(err){const rejected=err instanceof APIError&&[400,403,404].includes(err.status);if(rejected)pending.current=null;setError((err as Error).message);setLocked(!rejected);setConflict(err instanceof APIError&&err.status===409);return null}
  finally{inFlight.current=false;setBusy(false)}
 }
 return {busy,locked,error,conflict,feedback,reset,send}
}
