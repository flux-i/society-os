import type { RecoveryDecisionInput, RecoveryProposal } from './recovery-review'

export interface RecoveryGapItem {
  book_ref: string; item_ref: string; flat_id: string; kind: 'DEBIT' | 'RECEIVED' | 'EVIDENCE'; amount: string; date: string; entry_ref: string; receipt_number: string; evidence: string; evidence_state: 'SUPPORTED' | 'UNKNOWN' | 'CONTRADICTORY'
}
export interface RecoveryGapInput {
  operation_key: string; scope_key: string; predecessor_id: string; predecessor_version: number; title: string; reason: string; coverage_from: number; coverage_through: number; verified_at: number; coverage_evidence: string; coverage_complete: boolean; declared_empty: boolean; items: RecoveryGapItem[]
}
export interface RecoveryGapHome {
  flat_id: string; home: string; debit_paise: number; credit_paise: number; voluntary_paise: number; checkpoint_paise: number; difference_paise: number; comparison_paise: number
}
export interface RecoveryGapComparison {
  homes: RecoveryGapHome[]; matches: { item: number; entry_ref: string; entry_kind: string; linked_reversal: boolean; receipt_matched: boolean }[]; known_items: number; unresolved_items: number; coverage_unresolved: boolean; prepared_only: true
}
export interface RecoveryGapPreparation {
  id: string; title: string; state: 'PENDING' | 'APPROVED' | 'DECLINED' | 'STALE' | 'SUPERSEDED'; version: number; author_id: string; author: string; created_at: number; predecessor_id: string; input: RecoveryGapInput; comparison: RecoveryGapComparison; decision: RecoveryProposal['decision']
}
export interface RecoveryGapStatus { pending: number; approved: number; known: number; unresolved: number; status_key: string; can_resume: false }
export type RecoveryGapDecision = RecoveryDecisionInput
export const gapState = { PENDING: 'Awaiting another reviewer', APPROVED: 'Approved assessment', DECLINED: 'Declined', STALE: 'Earlier appointment', SUPERSEDED: 'Earlier version' }
export const gapKind = { DEBIT: 'Supplied account debit', RECEIVED: 'Account money already received', EVIDENCE: 'Evidence only' }
export const gapEvidence = { SUPPORTED: 'Supported by supplied evidence', UNKNOWN: 'Unverified · needs investigation', CONTRADICTORY: 'Contradictory · needs investigation' }

// Decimal strings are converted in whole paise; invalid input stays invalid.
export function gapPaise(amount: string): number | null {
  if (!/^\d{1,8}(\.\d{1,2})?$/.test(amount)) return null
  const [whole, fraction = ''] = amount.split('.')
  const paise = Number(whole) * 100 + Number(fraction.padEnd(2, '0'))
  return Number.isSafeInteger(paise) && paise > 0 && paise <= 1000000000 ? paise : null
}
