import type { User } from './api'
export { useFundLoad as useChecklistLoad } from './collections'
export { useBudgetWrite as useChecklistWrite, todayInSociety } from './budgets'
export const canReadChecklists=(user:User)=>!user.mfa_pending&&user.can_read_checklists
export const checklistLink=(key:string)=>new URLSearchParams(window.location.hash.split('?')[1]??'').get(key)??''
export const checklistKinds:Record<string,string>={MOVE_IN:'Move-in review',MOVE_OUT:'Move-out review',CONTACT_REVIEW:'Contact review'}
export const checklistPhases:Record<string,string>={CHECKING:'Awaiting checks',READY:'Ready for separate review',INFO:'Information requested',COMPLETED:'Review completed',DECLINED:'Proposal declined',CANCELLED:'Proposal withdrawn'}
export const checklistActions:Record<string,string>={SUBMIT:'Original submitted',REVISE:'Submission revised',CHECK:'Detail checked',READY:'Submitted for separate review',RETURN:'Returned to checks',INFO:'Information requested',APPROVED:'Review completed separately',DECLINED:'Proposal declined',CANCELLED:'Proposal withdrawn',CORRECTION:'Linked correction proposed'}
export const checkLabels:Record<string,string>={IDENTITY:'Identity & supplied source',REGISTRY:'Registry & relationship',CONTACT:'Contact preferences',DOCUMENTS:'Documents & access handover',HANDOVER:'Practical handover'}
export const checkHints:Record<string,string>={IDENTITY:'Review the named person and the supplied identity reference.',REGISTRY:'Review the recorded relationship and occupancy in the registry.',CONTACT:'Review the current registered profile and choices. Contact verification stays in Contacts.',DOCUMENTS:'Record the relevant document or account handover reference, or explain why it does not apply.',HANDOVER:'Record the supplied handover reference, or explain why it does not apply.'}
export const contactStateLabels:Record<string,string>={NONE:'No registered profile',PENDING:'Awaiting contact verification',VERIFIED:'Profile verified',DECLINED:'Verification declined',WITHDRAWN:'Profile withdrawn'}
export type ChecklistSource={registry_version:number;contact_version:number;contact_state:string;current:boolean;person_name:string;home_label:string;membership_key:string;key:string}
export type ChecklistCheck={kind:string;state:string;reference:string;checked_by:string;checked_at:number;source_key:string}
export type ChecklistSnapshot={version:number;action:string;phase:string;flat_id:string;resident_id:string;kind:string;author_id:string;proposed_by:string;ready_by:string;effective_date:string;note:string;person_name:string;home_label:string;checks:ChecklistCheck[];source_key:string;source:ChecklistSource;previous_approved_version:number;actor_id:string;occurred_at:number;reason:string}
export type ChecklistRecord={id:string;version:number;phase:string;state:string;pending:boolean;snapshot:ChecklistSnapshot;approved?:ChecklistSnapshot;source:ChecklistSource;checked:number;not_applicable:number;stale_checks:number;can_revise:boolean;can_check:boolean;can_ready:boolean;can_return:boolean;can_decide:boolean;can_complete:boolean;can_cancel:boolean;can_correct:boolean;can_open_home:boolean;can_open_contact:boolean}
export type ChecklistEvent={version:number;action:string;actor:string;at:number;reason:string;snapshot:ChecklistSnapshot}
export type ChecklistDetail=ChecklistRecord&{events:ChecklistEvent[];event_total:number;event_page:number;page_size:number;current_key:string}
export type ChecklistPage={items:ChecklistRecord[];total:number;page:number;page_size:number;counts:Record<string,number>;can_prepare:boolean;current_key:string}
export type ChecklistOptions={homes:{id:string;label:string}[];people:{id:string;name:string}[];total:number;page:number;page_size:number;can_prepare:boolean}
