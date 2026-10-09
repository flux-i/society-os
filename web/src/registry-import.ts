import type { User } from './api'
export const canImportRegistry=(user:User)=>user.can_manage_registry && ['FICTIONAL_REHEARSAL','LOCAL_WORKSPACE'].includes(user.workspace_mode)
export interface ImportCounts { buildings:number; homes:number; people:number; relationships:number; occupied:number; vacant:number; owners:number; tenants:number }
export interface ImportIssue { section:string; row:number; field:string; message:string }
export interface ImportMember { source_id:string; name:string; relationship:string; start_date:string; end_date:string; primary_contact:boolean }
export interface ImportHome { source_id:string; label:string; floor:number; occupancy:'OWNER_OCCUPIED'|'RENTED'|'VACANT'; members:ImportMember[] }
export interface ImportResult { id:string; source_key:string; digest:string; effective_date:string; counts:ImportCounts; created_at:number }
export interface ImportPreview { digest:string; base_digest:string; effective_date:string; source_key:string; counts:ImportCounts; errors:ImportIssue[]; error_count:number; warnings:ImportIssue[]; homes:ImportHome[]; can_apply:boolean; already_applied?:ImportResult }
export interface ImportStatus { initial_import_available:boolean; registry_counts:{buildings:number;flats:number;residents:number;memberships:number}; applied?:ImportResult; mode:string; society_key:string }
