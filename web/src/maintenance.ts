export type FinancialHome = { id: string; label: string }
export type MaintenanceTotals = { requested_paise: number; active_paise: number; allocated_paise: number; outstanding_paise: number; overdue_paise: number; reversed_paise: number }
export type MaintenanceCycle = MaintenanceTotals & { id: string; title: string; period_start: string; period_end: string; due_date: string; source_reference?: string; note?: string; state: string; version: number; author_id?: string; author?: string; submitted_at: number; decided_by?: string; decided_at: number; decision_reason?: string; participants: number }
export type MaintenancePage = { items: MaintenanceCycle[]; total: number; page: number; page_size: number; homes: FinancialHome[]; totals: MaintenanceTotals }
export type MaintenanceLine = { flat_id: string; home: string; entry_id: string; state: string; amount_paise: number; allocated_paise: number; outstanding_paise: number }
export type MaintenanceDetail = MaintenanceCycle & { lines: MaintenanceLine[]; line_page: number; page_size: number; events?: { action: string; version: number; actor: string; reason: string; at: number }[]; event_total?: number; event_page?: number }
export type StatementEntry = { id: string; kind: string; description: string; date: string; amount_paise: number; allocated_paise: number; remaining_paise: number; receipt: string; cycle_id: string; due_date: string; pause_until?: string }
export type Allocation = { id: string; source_id: string; charge_id: string; amount_paise: number; receipt: string; source_kind: string; description: string; state: string; reason?: string; actor?: string; created_at: number; correction_reason?: string; corrected_by?: string; corrected_at: number }
export type HomeStatement = { flat_id: string; home: string; debit_paise: number; credit_paise: number; allocated_paise: number; outstanding_paise: number; unallocated_paise: number; voluntary_paise: number; overdue_paise: number; charges: StatementEntry[]; credits: StatementEntry[]; allocations: Allocation[]; charge_total: number; credit_total: number; allocation_total: number; charge_page: number; credit_page: number; allocation_page: number; page_size: number }
export const money = (paise: number) => new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(paise / 100)
export const displayDate = (value: string) => new Intl.DateTimeFormat('en-IN', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Asia/Kolkata' }).format(new Date(value + 'T00:00:00+05:30'))
export const dateToday = () => new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit', timeZone: 'Asia/Kolkata' }).format(new Date())
export const cycleState = (value: string) => ({ PENDING: 'Awaiting review', PUBLISHED: 'Published', DECLINED: 'Declined', WITHDRAWN: 'Withdrawn' })[value] ?? value
export function exactPaise(value: string): number | null {
  if (!/^(0|[1-9][0-9]{0,7})(\.[0-9]{1,2})?$/.test(value)) return null
  const [rupees, fraction = ''] = value.split('.')
  const result = Number(rupees) * 100 + Number(fraction.padEnd(2, '0'))
  return result > 0 && result <= 1000000000 ? result : null
}
