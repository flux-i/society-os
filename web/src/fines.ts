import { useEffect, useRef, useState } from 'react'
import { APIError, mutate } from './api'
import type { FundEvent } from './collections'
import type { IncidentResponse, Rule } from './incidents'
import type { StatementEntry } from './maintenance'
export { useFundLoad as useFineLoad, selectOptions } from './collections'
export type FineSource = {
  incident_id: string; flat_id: string; home: string; rule: Rule; incident_date: string;
  substantiated_at: number; outcome_event_id: number; material_key: string; source_key: string;
  notice?: { id: string; title: string; body: string; response_by: string; responses: IncidentResponse[] };
  response_total: number; response_page: number; page_size: number;
}
export type FineSourcePage = { items: FineSource[]; total: number; page: number; page_size: number }
export type Fine = {
  id: string; incident_id?: string; replaces_id?: string; flat_id: string; home: string; rule_id?: string;
  material_key?: string; source_key?: string; source?: FineSource; current_source?: FineSource;
  title: string; amount_paise: number; policy_reference?: string; reason?: string; due_date: string;
  response_by: string; notice_body?: string; state: string; author_id?: string; created_at: number;
  updated_at: number; version: number; notice_id?: string; original_entry_id?: string; current_entry_id?: string;
  charge_version?: number; waived_paise: number; active_paise: number; allocated_paise: number;
  outstanding_paise: number; resolution?: string; resolution_key?: string; early_issue_reference?: string;
  resolved_by?: string; resolved_at?: number; issued_by?: string; issued_at?: number; pause_until?: string;
  pause_appeal_id?: string; source_current: boolean; resolution_current: boolean; needs_review: boolean;
  response_total?: number; current_source_key?: string; can_decide: boolean;
}
export type FineTotals = { active_paise: number; allocated_paise: number; outstanding_paise: number; overdue_paise: number; pending: number; notified: number; paused: number; needs_review: number }
export type FinePage = { items: Fine[]; total: number; page: number; page_size: number; totals: FineTotals }
export type FineDetail = Fine & { events?: FundEvent[]; event_total?: number; event_page?: number; page_size: number; notice?: FineNotice }
export type FineNotice = {
  id: string; fine_id: string; flat_id: string; home: string; title: string; body: string; amount_paise: number;
  policy_reference: string; due_date: string; response_by: string; created_at: number; version: number; state: string;
  can_respond: boolean; can_appeal: boolean; can_read_finance: boolean; responses: IncidentResponse[];
  response_total: number; response_page: number; page_size: number;
}
export type FineNoticePage = { items: FineNotice[]; total: number; page: number; page_size: number }
export type FineReport = {
  id: string; fine_id: string; fine: string; flat_id: string; home: string; author_id: string; author: string;
  amount_paise: number; payment_date: string; payer: string; method: string; reference: string; comment: string;
  evidence_id: string; state: string; version: number; created_at: number; updated_at: number; entry_id: string;
  allocation_id: string; duplicate_report_id: string; reviewer: string; reviewed_at: number; decision_reason: string;
  receipt_id: string; receipt: string; current_state: string; can_decide: boolean; events?: FundEvent[];
  event_total?: number; event_page?: number; page_size: number;
}
export type FineReportPage = { items: FineReport[]; total: number; page: number; page_size: number; counts: Record<string, number> }
export type FineReceipt = StatementEntry & { verification_source?: string; payment_identity?: string }
export type FineConfirmOptions = { entries: FineReceipt[]; outstanding_paise: number; charge_id: string; total: number; page: number; page_size: number }
export type FineEvidence = { id: string; title: string; filename: string }
export type FineEvidencePage = { items: FineEvidence[]; total: number; page: number; page_size: number }
export type FineAppeal = {
  id: string; fine_id: string; fine: string; home: string; author_id?: string; author?: string; body: string;
  state: string; version: number; created_at: number; updated_at: number; reviewer?: string; reviewed_at?: number;
  decision_reason: string; policy_reference: string; pause_until: string; can_decide: boolean; can_withdraw: boolean;
  current_fine_version?: number; events?: FundEvent[]; event_total?: number; event_page?: number; page_size: number;
}
export type FineWaiver = {
  id: string; fine_id: string; fine: string; home: string; charge_version: number; amount_paise: number;
  kind: string; policy_reference: string; reason: string; author_id: string; author: string; state: string;
  version: number; created_at: number; reviewer: string; reviewed_at: number; decision_reason: string;
  replacement_entry_id: string; can_decide: boolean; current_fine_version: number; events: FundEvent[];
  event_total: number; event_page: number; page_size: number;
}
export type FineAppealPage = { items: FineAppeal[]; total: number; page: number; page_size: number }
export type FineWaiverPage = { items: FineWaiver[]; total: number; page: number; page_size: number }
export const fineStates: Record<string, string> = { PENDING: 'Awaiting independent review', NOTIFIED: 'Household notice approved', ISSUED: 'Issued', WAIVED: 'Charge removed', DECLINED: 'Declined', WITHDRAWN: 'Withdrawn' }
export const fineActions: Record<string, string> = { PROPOSED: 'Fine proposed', NOTE: 'Private treasury note', NOTIFY: 'Approve household notice', RESOLVE: 'Resolve the current responses', ISSUE: 'Issue reviewed charge', DECLINED: 'Decline proposal', WITHDRAWN: 'Withdraw proposal', RESPONSE: 'Household response retained', ISSUED: 'Charge issued', NOTIFIED: 'Household notice approved', RESOLVED: 'Response review recorded', CORRECTION_PROPOSED: 'Correction proposed', CORRECTION_APPROVED: 'Linked correction approved', CORRECTION_DECLINED: 'Correction declined', APPEAL_SUBMITTED: 'Appeal submitted', PAUSED: 'Collection paused', APPROVED: 'Correction approved', CONFIRMED: 'Already-paid money verified', DUPLICATE: 'Original receipt linked', NEEDS_INFO: 'More information requested', REJECTED: 'Verification declined' }
export const appealStates: Record<string, string> = { PENDING: 'Awaiting review', PAUSED: 'Collection paused', RESOLVED: 'Resolved', DECLINED: 'Declined', WITHDRAWN: 'Withdrawn' }
export const correctionStates: Record<string, string> = { PENDING: 'Awaiting review', APPROVED: 'Approved', DECLINED: 'Declined', WITHDRAWN: 'Withdrawn' }
export const fineLink = (key: string) => new URLSearchParams(window.location.hash.split('?')[1] ?? '').get(key) ?? ''
export function useFineWrite() {
  const pending = useRef<{ path: string; payload: Record<string, unknown> } | null>(null), inFlight = useRef(false), feedback = useRef<HTMLDivElement>(null)
  const [busy, setBusy] = useState(false), [locked, setLocked] = useState(false), [error, setError] = useState(''), [conflict, setConflict] = useState(false), [reauth, setReauth] = useState(false)
  useEffect(() => { if (error) feedback.current?.scrollIntoView({ block: 'nearest' }) }, [error])
  const reset = () => { pending.current = null; setLocked(false); setError(''); setConflict(false); setReauth(false) }
  const send = async (path: string, payload: Record<string, unknown>) => {
    if (inFlight.current) return null
    inFlight.current = true; setBusy(true); setError(''); setReauth(false)
    pending.current ??= { path, payload: { ...payload, operation_key: crypto.randomUUID(), confirmed: true } }
    try { const result = await mutate<{ id: string }>(pending.current.path, 'POST', pending.current.payload); reset(); return result }
    catch (err) {
      const rejected = err instanceof APIError && [400, 403, 404, 428].includes(err.status)
      if (rejected) pending.current = null
      setError((err as Error).message); setLocked(!rejected); setConflict(err instanceof APIError && err.status === 409); setReauth(err instanceof APIError && err.code === 'reauthentication_required'); return null
    } finally { inFlight.current = false; setBusy(false) }
  }
  return { busy, locked, error, conflict, reauth, feedback, reset, send }
}
