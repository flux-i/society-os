import type { User } from '../api'
import type { FinanceReport } from '../finance-exports'
import { canReadBudgets } from '../budgets'
import { FinanceExportAction } from './FinanceExports'
import { Icon } from './Icon'

export function FinanceWorkspaces({ user, payments = false }: { user: User; payments?: boolean }) {
  if (!user.can_read_records) return <section className="empty-state"><Icon name="shield"/><h1>Financial access required.</h1><p>Your current role or household permission determines which records you can open.</p><a className="text-link" href="#guide">Understand your permissions<Icon name="arrow"/></a></section>
  const reports: [FinanceReport, string, string][] = [
    ['RECEIPTS', 'Monthly collections', 'Choose a date range to review original receipts, usable receipts and linked reversals.'],
    ['MAINTENANCE', 'Maintenance and outstanding dues', 'Published charges, allocations and outstanding amounts for the selected homes.'],
    ['FUNDS', 'Fund collections and reconciliation', 'Fixed charges, voluntary contributions and payment reports awaiting verification.'],
    ['LEDGER', 'Confirmed ledger', 'Supplied charges, money received, opening balances and linked corrections.'],
  ]
  return <div className="page-enter finance-hub">
    <div className="registry-heading"><div><span className="eyebrow">MONEY</span><h1>{payments ? <>Payment <em>reconciliation.</em></> : <>Financial <em>reports.</em></>}</h1><p>{payments ? 'Check money already paid outside the portal. Keep proof, verification and the original receipt together.' : 'Review the period and scope before creating a report. Each export keeps its reviewed snapshot.'}</p></div></div>
    {payments ? <>
      <ol className="workflow-steps" aria-label="Payment verification steps"><li><span>1</span>Record payment & proof</li><li><span>2</span>Verify external record</li><li><span>3</span>Confirm once & retain receipt</li></ol>
      <div className="help-task-links"><a href="#collections?panel=reports"><Icon name="receipt"/><span><strong>Fund payment reports</strong><small>UPI and bank transfer claims, payment proof, verification and duplicate checks.</small></span><Icon name="chevron"/></a><a href="#fines?panel=reports"><Icon name="shield"/><span><strong>Fine payment reports</strong><small>Review reported payments against the issued fine and its current balance.</small></span><Icon name="chevron"/></a><a href="#entries"><Icon name="records"/><span><strong>{user.can_manage_records ? 'Record received money' : 'View money records'}</strong><small>Manual entries for money already received. Review the source before confirming a receipt.</small></span><Icon name="chevron"/></a><a href="#receipts"><Icon name="document"/><span><strong>Confirmed receipts</strong><small>Find the original receipt and follow any linked correction.</small></span><Icon name="chevron"/></a></div>
      <p className="workflow-note">Bank connection: not connected. Payments and bank checks happen outside this portal.</p>
    </> : <>
      <div className="finance-report-rows">{reports.map(([report, title, description]) => <section key={report}><Icon name="document"/><div><h2>{title}</h2><p>{description}</p></div>{user.can_export_finance ? <FinanceExportAction user={user} report={report} label={'Review ' + title.toLowerCase()}/> : <span className="form-help">Export permission not assigned</span>}</section>)}</div>
      <p className="workflow-note">Current report downloads are CSV files that open in Excel. Prepared PDF and spreadsheet statements are available in Statements.</p>
      <div className="help-task-links">{canReadBudgets(user) && <a href="#statements?panel=budgets"><Icon name="records"/><span><strong>Budget & category-wise expenses</strong><small>Approved budgets, separately reviewed paid expenses and supporting records.</small></span><Icon name="chevron"/></a>}<a href="#statements"><Icon name="document"/><span><strong>Income statements & balance sheets</strong><small>Open the prepared originals available to your current audience.</small></span><Icon name="chevron"/></a></div>
    </>}
  </div>
}
