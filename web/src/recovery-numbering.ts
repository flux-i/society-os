import type { RecoveryDecisionInput, RecoveryProposal } from './recovery-review'

export interface RecoveryNumberingSpec { prefix: string; digits: number; first: number; last: number; period: string; authority_ref: string; namespace_ref: string; verified_at: number }
export interface RecoveryNumberingInput { operation_key: string; scope_key: string; predecessor_id: string; predecessor_version: number; title: string; reason: string; policy: RecoveryNumberingSpec }
export interface RecoveryNumberingPolicy {
  id: string; title: string; state: 'PENDING' | 'APPROVED' | 'DECLINED' | 'STALE' | 'SUPERSEDED'; version: number; author_id: string; author: string; created_at: number; predecessor_id: string; first_example: string; last_example: string; input: RecoveryNumberingInput; decision: RecoveryProposal['decision']
  reservation: { operation_id: string; state: 'PREPARED' | 'HANDOFF' | 'UNKNOWN' | 'REJECTED' | 'VERIFIED' | 'STALE'; original_state: string; attempts: number; updated_at: number; checksum_verified: boolean } | null
}
export type RecoveryNumberingDecision = RecoveryDecisionInput
export interface RecoveryNumberingStatus { pending: number; approved: number; verified: number; unresolved: number; status_key: string; can_resume: false }
export const numberingState = { PENDING: 'Awaiting another reviewer', APPROVED: 'Approved preparation', DECLINED: 'Declined', STALE: 'Earlier appointment', SUPERSEDED: 'Earlier version' }
export const reservationState = { PREPARED: 'Original retained · awaiting custody handoff', HANDOFF: 'Handoff recorded · outcome needs checking', UNKNOWN: 'Outcome uncertain · independent check needed', REJECTED: 'Known rejection · original retained', VERIFIED: 'Original checksum and version verified', STALE: 'Earlier preparation · held for the record' }
