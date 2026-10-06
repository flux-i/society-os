export { useFundLoad as useStatementLoad, selectOptions } from './collections'
export { useFineWrite as useStatementWrite } from './fines'
import type { MessageTarget } from './messages'

export type StatementFile = {
  id:string; replaces_id:string; title:string; kind:string; period_start:string; period_end:string;
  prepared_by:string; source:string; filename:string; size_bytes:number; sha256:string; content_type:string;
  uploader_id:string; created_at:number; expires_at:number; uploaded_at:number; available_at:number;
  validation:string; validation_code:string; state:string; reviewer_id:string; reviewed_at:number;
  revision:number; version:number; current:boolean; publication_id:string; can_upload:boolean;
  can_review:boolean; can_withdraw:boolean; can_retry:boolean; can_replace:boolean; can_publish:boolean; can_download:boolean;
}
export type StatementEvent = { action:string; actor:string; reason:string; at:number; version:number }
export type StatementPublication = {
  id:string; file_id:string; target:MessageTarget; target_people:number; preview_hash:string; state:string;
  proposer_id:string; proposed_at:number; reviewer_id:string; reviewed_at:number; version:number; current:boolean;
  can_approve:boolean; can_decline:boolean; can_withdraw:boolean; can_revoke:boolean; problem:string; events:StatementEvent[]; file:StatementFile;
}
export type StatementDetail = StatementFile & {
  staff:boolean; events:StatementEvent[]; event_total:number; versions:StatementFile[]; version_total:number;
  publications:StatementPublication[]; publication_total:number; event_page:number; version_page:number; publication_page:number; page_size:number;
}
export type StatementPage = { items:StatementFile[]; total:number; page:number; page_size:number; can_prepare:boolean }
export type PublicationPreview = { file:StatementFile; target:MessageTarget; target_people:number; preview_hash:string }
export const statementKinds:Record<string,string> = { INCOME:'Income statement', BALANCE:'Balance sheet', BUDGET:'Budget', AUDIT:'Audit report' }
export const statementDecisions:Record<string,string> = { APPROVED:'Approve internally', DECLINED:'Decline this version', WITHDRAWN:'Withdraw this original', RETRY_VALIDATION:'Retry unavailable checks', PUBLISHED:'Approve publication', REVOKED:'Remove this publication' }
export const publicationStates:Record<string,string> = { PENDING:'Publication awaiting review', PUBLISHED:'Published', DECLINED:'Publication declined', WITHDRAWN:'Publication withdrawn', REVOKED:'Publication removed', SUPERSEDED:'Earlier publication' }
export const statementEvents:Record<string,string> = { RESERVED:'Original reserved', UPLOADED:'Original received', VALIDATING:'Original checks started', VALIDATED:'Original checks passed', REJECTED:'Original needs attention', ABANDONED:'Upload reservation expired', APPROVED:'Version approved internally', DECLINED:'Version declined', WITHDRAWN:'Original withdrawn', RETRY_VALIDATION:'Original checks retried' }
export const statementSize = (bytes:number) => bytes<1024*1024?Math.max(1,Math.round(bytes/1024))+' KB':(bytes/1024/1024).toFixed(1)+' MB'
export const statementState = (x:StatementFile) => x.state==='APPROVED'?(x.publication_id?'Shared with its audience':'Approved internally'):x.state==='DECLINED'?'Declined':x.state==='WITHDRAWN'?'Withdrawn':x.validation==='REJECTED'?'Original needs attention':x.validation==='ABANDONED'?'Upload expired':x.uploaded_at===0?'Upload unfinished':x.validation==='AVAILABLE'?'Awaiting separate review':'Checking original'
export const statementCheckErrors:Record<string,string> = {
  INVALID_PDF:'Choose a fresh plain PDF that opens correctly.', ENCRYPTED_PDF:'Choose an unencrypted PDF.',
  UNSUPPORTED_PDF_CONTENT:'Use a plain PDF without active forms, scripts or embedded attachments.',
  PDF_CHECK_LIMIT:'Export a smaller, simpler PDF.', CHECKS_UNAVAILABLE:'Original checks are unavailable. Retry after the checking service is restored.',
  INVALID_WORKBOOK:'This workbook package could not be verified. Export a fresh plain XLSX or PDF.',
  UNSUPPORTED_WORKBOOK_PART:'Use a plain worksheet workbook without macros, charts, embedded objects or external links.',
  ACTIVE_SPREADSHEET_CONTENT:'This original contains unsupported active formulas or links. Use plain worksheets with ordinary calculations, or export a PDF.',
  SPREADSHEET_LIMIT:'Use a smaller workbook or CSV with fewer rows, columns and long fields.',
  INVALID_CSV:'Use a consistent UTF-8 CSV without control characters or broken quotes.',
  CHECKSUM_MISMATCH:'The original no longer matches its reserved checksum.', FILE_TOO_LARGE:'Choose an original within the displayed size limit.',
  CHECK_CANCELLED:'Original checking was interrupted. Retry the current record.', UNSUPPORTED_TYPE:'Choose a plain PDF, XLSX or UTF-8 CSV.',
}

export type StatementSummary = { counts:Record<string,number>; items:{id:string;title:string;kind:string;state:string}[] }
