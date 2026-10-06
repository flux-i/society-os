export { useFundLoad as useContactLoad } from './collections'
export { useFineWrite as useContactWrite } from './fines'
export type ContactPreferences = { community_whatsapp: boolean; community_email: boolean; finance_whatsapp: boolean; finance_email: boolean }
export type Contact = ContactPreferences & {
  id: string; name: string; phone: string; email: string; preferred_channel: string;
  consent_source: string; state: string; submitted_by: string; submitted_at: number;
  reviewed_by: string; reviewed_at: number; decision_reason: string; created_at: number;
  updated_at: number; version: number; current: boolean; can_register: boolean;
  can_verify: boolean; can_withdraw: boolean; can_opt_out: boolean;
  eligible: ContactPreferences; homes: { id: string; label: string }[]; relationships: string[];
}
export type ContactDetail = Contact & { events: { version: number; action: string; actor: string; at: number; reason: string; snapshot: Contact }[]; event_total: number; event_page: number; page_size: number }
export type ContactPage = { items: Contact[]; total: number; page: number; page_size: number }
export const contactStates: Record<string, string> = { NONE: 'Not registered', PENDING: 'Awaiting verification', VERIFIED: 'Independently verified', DECLINED: 'Verification declined', WITHDRAWN: 'Registration withdrawn' }
export const contactActions: Record<string, string> = { REGISTERED: 'Contact choices supplied', VERIFIED: 'Identity and permission verified', DECLINED: 'Verification declined', WITHDRAWN: 'Registration withdrawn', OPTED_OUT: 'Messages stopped' }
export const preferenceFields = [
  ['community_whatsapp', 'Community notices on WhatsApp'], ['community_email', 'Community notices by email'],
  ['finance_whatsapp', 'Permitted financial messages on WhatsApp'], ['finance_email', 'Permitted financial messages by email'],
] as const
