export interface Building {
  id: string
  code: string
  name: string
  flats: number
  owner_occupied: number
  rented: number
  vacant: number
  owners: number
  tenants: number
}

export interface Summary {
  counts: { buildings: number; flats: number; residents: number; memberships: number }
  community: { owners: number; tenants: number; occupied: number; vacant: number; owner_occupied: number; rented: number }
  buildings: Building[]
  data_kind: string
  fixture_version: string
}

export interface Flat {
  id: string
  building_code: string
  number: string
  floor: number
  status: 'OWNER_OCCUPIED' | 'RENTED' | 'VACANT'
  primary_contact: string
  version: number
}

export interface FlatPage {
  items: Flat[]
  total: number
  page: number
  page_size: number
}

export interface FlatDetail extends Flat {
  members: { id: string; name: string; relationship: string; start_date: string; end_date: string | null; membership_id: string; primary_contact: boolean; active: boolean }[]
}

export interface User { id: string; name: string; roles: string[]; scope_key: string; can_read_registry: boolean; can_manage_registry: boolean; can_manage_accounts: boolean; can_manage_records: boolean; can_read_all_records: boolean; can_read_records: boolean; can_export_finance: boolean; can_review_requests: boolean; can_handle_complaints: boolean; can_manage_documents: boolean; can_read_contacts: boolean; can_manage_contacts: boolean; csrf_token: string; mfa_required: boolean; mfa_enrolled: boolean; mfa_pending: boolean; is_demo: boolean; fresh_authentication: boolean }
const permissionKeys = ['can_manage_accounts', 'can_manage_registry', 'can_read_registry', 'can_manage_records', 'can_read_all_records', 'can_read_records', 'can_export_finance', 'can_review_requests', 'can_handle_complaints', 'can_manage_documents', 'can_read_contacts', 'can_manage_contacts'] as const
// Normal identity refreshes preserve forms; a changed authority or home scope
// discards data loaded under the earlier scope, including when flags stay true.
export const userAccessScope = (user: User) => JSON.stringify([user.id, user.scope_key, user.mfa_pending, user.roles, ...permissionKeys.map(key => user[key])])
export interface MFAResult { user: User; recovery_codes?: string[] }
export interface Account { id: string; name: string; email: string; resident_name: string; state: string; roles: string[]; mfa_enrolled: boolean; active_homes: number }
export interface AccountPage { items: Account[]; total: number; page: number; page_size: number }
export interface IssuedLink { token: string; purpose: string; name: string; expires_at: number }
export interface LinkDetails { name: string; email: string; purpose: string; expires_at: number }
export interface Person { id: string; name: string }
export interface AuditEvent { id: number; actor: string; action: string; occurred_at: number; reason: string; before: Record<string, unknown>; after: Record<string, unknown> }
let csrf = ''
export const setSession = (user: User | null) => { csrf = user?.csrf_token ?? '' }
export class APIError extends Error { constructor(public status: number, message: string, public code = '') { super(message) } }

export const statuses: Record<Flat['status'], string> = {
  OWNER_OCCUPIED: 'Owner occupied', RENTED: 'Rented', VACANT: 'Vacant',
}

export async function request<T>(path: string, signal?: AbortSignal, init?: RequestInit): Promise<T> {
  let response: Response
  try { response = await fetch(path, { ...init, signal, cache: 'no-store', credentials: 'same-origin' }) }
  catch (err) {
    if (signal?.aborted || (err instanceof DOMException && err.name === 'AbortError')) throw err
    throw new Error('The connection was interrupted. Please try again.')
  }
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as { error?: string; message?: string }
    if (response.status === 401 && !path.startsWith('/api/auth/')) window.dispatchEvent(new Event('session-expired'))
    const messages: Record<number, string> = {
      401: path === '/api/auth/login' ? 'That email and password didn’t match. Please try again.' : 'Please sign in again to continue.',
      403: 'Your account does not have permission for this action.',
      404: 'This record is unavailable for your current account.',
      409: ['/api/entries', '/api/receipts', '/api/reviews', '/api/complaints', '/api/documents', '/api/admin/accounts', '/api/maintenance', '/api/allocations', '/api/upkeep', '/api/collections', '/api/payment-reports', '/api/fund-waivers', '/api/fund-contributions', '/api/rules', '/api/incidents', '/api/incident-notices', '/api/incident-pictures', '/api/fines', '/api/fine-', '/api/contacts', '/api/messages', '/api/statements', '/api/financial-statements', '/api/finance-exports', '/api/community'].some(prefix => path.startsWith(prefix)) ? 'This record has changed or this retry has different details. Reload the record before continuing.' : 'Someone has updated this home. Reload the details before saving.',
      429: 'Please wait a little before trying to sign in again.',
    }
    const codes: Record<string, string> = {
      invalid_verification: 'That verification code didn’t work. Use a fresh authenticator code or an unused recovery code.',
      reauthentication_required: 'Confirm your password and verification code in Account security, then try again.',
      mfa_required: 'Complete two-step verification to open your workspace.',
      verification_rate_limited: 'Too many attempts. Wait 15 minutes before trying again.',
    }
    if (body.error === 'mfa_required' && !path.startsWith('/api/auth/')) window.dispatchEvent(new Event('session-recheck'))
    if (body.error === 'permission_required' && !path.startsWith('/api/auth/')) window.dispatchEvent(new Event('session-recheck'))
    throw new APIError(response.status, body.message ?? codes[body.error ?? ''] ?? messages[response.status] ?? 'The workspace could not be reached. Please try again.', body.error)
  }
  return response.json() as Promise<T>
}

export const mutate = <T,>(path: string, method: string, data: unknown, signal?: AbortSignal): Promise<T> => request<T>(path, signal, {
  method, headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf }, body: JSON.stringify(data),
})

export const uploadOriginal = (id: string, file: File): Promise<{ id: string }> => request(`/api/documents/${encodeURIComponent(id)}/content`, undefined, {
  method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-CSRF-Token': csrf }, body: file,
})

export const uploadStatementOriginal = (id: string, file: File): Promise<{ id: string }> => request(`/api/financial-statements/${encodeURIComponent(id)}/content`, undefined, {
  method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-CSRF-Token': csrf }, body: file,
})

export const uploadIncidentPicture = <T,>(file: File, key: string): Promise<T> => request('/api/incident-pictures', undefined, {
  method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-CSRF-Token': csrf, 'X-Operation-Key': key, 'X-Picture-Filename': encodeURIComponent(file.name) }, body: file,
})
