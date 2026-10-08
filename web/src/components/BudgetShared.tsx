import type { BudgetSnapshot, PaidExpenseSnapshot } from '../budgets'
import { budgetActions, expenseMethods } from '../budgets'
import type { useBudgetWrite } from '../budgets'
import { displayDate, money } from '../maintenance'
import { Icon } from './Icon'

export function BudgetPlanCard({plan,label='Supplied operating plan'}:{plan:BudgetSnapshot;label?:string}){
 return <section className="budget-plan-original" aria-label={label}><span className="eyebrow">{label}</span><h3>{plan.title}</h3><p>{displayDate(plan.period_start)} – {displayDate(plan.period_end)}</p><div className="budget-original-money"><div><span>Planned collections</span><strong>{money(plan.collections_paise)}</strong></div><div><span>Planned expenses</span><strong>{money(plan.expenses_paise)}</strong></div></div><p className="budget-source-reference"><span>Supplied source</span>{plan.source_reference}</p>{plan.action==='CLOSE'&&<span className="contact-state">Period closure</span>}</section>
}
export function PaidExpenseOriginal({expense,label='Supplied paid record'}:{expense:PaidExpenseSnapshot;label?:string}){
 return <section className="budget-expense-original" aria-label={label}><div className="budget-expense-original-top"><span className="eyebrow">{label}</span><span className="contact-state">{budgetActions[expense.action]}</span></div><h3>{expense.payee}</h3><strong className="budget-expense-amount">{money(expense.amount_paise)}</strong><dl className="budget-original-details"><div><dt>Paid on</dt><dd>{displayDate(expense.paid_date)}</dd></div><div><dt>Category</dt><dd>{expense.category}</dd></div><div><dt>Method</dt><dd>{expenseMethods[expense.method]}</dd></div><div><dt>Payment / voucher reference</dt><dd>{expense.reference}</dd></div></dl><p className="budget-source-reference"><span>Supplied source</span>{expense.source_note}</p>{expense.previous_version>0&&<p className="form-help">Linked to accepted expense version {expense.previous_version}. Its original remains on record.</p>}</section>
}
export function BudgetWriteFeedback({write,onReload}:{write:ReturnType<typeof useBudgetWrite>;onReload?:()=>void}){
 return <>{write.error&&<div className="form-error" role="alert" ref={write.feedback}><p>{write.error}</p>{write.conflict&&onReload&&<button type="button" className="text-link" onClick={onReload}>Reload current record<Icon name="refresh"/></button>}</div>}{write.locked&&!write.conflict&&<p className="form-help">Retry checks this same reviewed submission.</p>}{write.reauth&&<a className="text-link" href="#security" target="_blank" rel="noopener">Verify your identity in a new tab, then return to this form<Icon name="arrow"/></a>}</>
}
export function BudgetReadError({title,error,onRetry}:{title:string;error:string;onRetry:()=>void}){
 return <div className="empty-state" role="alert"><h3>{title}</h3><p>{error}</p><button type="button" className="button button-dark" onClick={onRetry}>Try again<Icon name="refresh"/></button></div>
}
