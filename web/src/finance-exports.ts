import { APIError } from './api'

export type FinanceReport = 'LEDGER' | 'RECEIPTS' | 'MAINTENANCE' | 'FUNDS'
export type ExportFilter = { report: FinanceReport; scope: 'SOCIETY' | 'OWN' | 'HOME'; home_id: string; fund_id: string; from: string; to: string }
export type ExportSummary = { current_net_paise: string; current_received_paise: string; available_received_paise: string; opening_credit_paise: string; original_received_paise: string; reversed_received_paise: string; usable_received_paise: string; requested_paise: string; active_paise: string; allocated_paise: string; outstanding_paise: string; waived_paise: string; voluntary_paise: string; pending_reports: number; sources: number }
export type FinanceExport = ExportFilter & { id: string; authority: 'TREASURY' | 'PERSONAL'; homes: { id: string; label: string }[]; scope_label: string; date_basis: string; summary: ExportSummary; content_hash: string; sha256: string; rows: number; bytes: number; generated_at: string }
export type ExportChoices = { society: boolean; fresh: boolean; homes: { id: string; label: string }[]; own_homes: { id: string; label: string }[]; funds: { id: string; label: string }[] }
export type ExportPage = { items: FinanceExport[]; total: number; page: number; page_size: number }
export const exportReports: Record<FinanceReport, string> = { LEDGER: 'Confirmed ledger', RECEIPTS: 'Original receipt register', MAINTENANCE: 'Maintenance reconciliation', FUNDS: 'Fund reconciliation' }
export const exportBasis: Record<FinanceReport, string> = { LEDGER: 'Ledger entry date', RECEIPTS: 'Original receipt date', MAINTENANCE: 'Maintenance period start', FUNDS: 'Fund start date' }
export function exportMoney(paise: string) {
  const value = BigInt(paise), absolute = value < 0n ? -value : value
  return `${value < 0n ? '−' : ''}₹${new Intl.NumberFormat('en-IN').format(absolute / 100n)}.${(absolute % 100n).toString().padStart(2, '0')}`
}

export async function downloadFinanceExport(snapshot: FinanceExport) {
  let response: Response
  try { response = await fetch(`/api/finance-exports/${encodeURIComponent(snapshot.id)}/download`, { cache: 'no-store', credentials: 'same-origin' }) }
  catch { throw new Error('The connection was interrupted. This snapshot is ready; try the download again.') }
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as { error?: string; message?: string }
    if (response.status === 401) window.dispatchEvent(new Event('session-expired'))
    if (response.status === 403 && body.error === 'permission_required') window.dispatchEvent(new Event('session-recheck'))
    const message = body.error === 'reauthentication_required' ? 'Confirm your password and verification code in Account security, then try again.' : response.status === 403 || response.status === 404 ? 'This snapshot is unavailable for your current financial access.' : 'The CSV could not be downloaded. Try again.'
    throw new APIError(response.status, body.message ?? message, body.error)
  }
  const bytes = await response.arrayBuffer()
  const digest = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))).map(value => value.toString(16).padStart(2, '0')).join('')
  if (!response.headers.get('Content-Type')?.startsWith('text/csv') || bytes.byteLength !== snapshot.bytes || digest !== snapshot.sha256 || response.headers.get('X-Export-SHA256') !== snapshot.sha256) throw new Error('This file did not match the reviewed snapshot. No download was started; try again.')
  const url = URL.createObjectURL(new Blob([bytes], { type: 'text/csv;charset=utf-8' }))
  const link = document.createElement('a'); link.href = url; link.download = `society-${snapshot.report.toLowerCase()}-${snapshot.from}-${snapshot.to}.csv`; document.body.append(link); link.click(); link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 30000)
}
