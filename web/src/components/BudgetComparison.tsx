import { useBudgetLoad } from '../budgets'
import type { BudgetComparison as Comparison } from '../budgets'
import { displayDate, money } from '../maintenance'
import { careTime } from '../upkeep'
import { Icon } from './Icon'

function ComparisonTrack({label,planned,recorded,spending=false}:{label:string;planned:number;recorded:number;spending?:boolean}){
 const percent=planned>0?Math.min(100,Math.max(0,recorded/planned*100)):recorded>0?100:0
 return <div className={'budget-comparison-track '+(spending?'budget-track-spending':'')}><span className="eyebrow">{label}</span><strong>{money(recorded)}</strong><div className="budget-track"><span style={{width:percent+'%'}}/></div><div className="budget-track-caption"><span>Recorded</span><span>Plan {money(planned)}</span></div></div>
}
export function BudgetComparison({id,version,revision,onReloadRecord}:{id:string;version:number;revision:number;onReloadRecord:()=>void}){
 const load=useBudgetLoad<Comparison>('/api/budgets/'+encodeURIComponent(id)+'/comparison?version='+version+'&refresh='+revision),data=!load.loading&&!load.error?load.data:null
 return <section className="budget-comparison" aria-label="Current budget comparison"><div className="section-heading"><div><span className="eyebrow">THE PLAN, MEETING REAL LIFE</span><h3>A clearer view of the period.</h3></div><button type="button" className="text-link" onClick={()=>{load.reload();onReloadRecord()}} disabled={load.loading}>Refresh comparison<Icon name="refresh"/></button></div>
 {load.error?<div className="connection-error" role="alert"><p>The comparison is unavailable. The plan and retained history remain open.</p><p>{load.error}</p><button type="button" onClick={()=>{load.reload();onReloadRecord()}}>Retry budget comparison<Icon name="refresh"/></button></div>:load.loading?<p className="empty-state" role="status">Checking recorded collections and spending…</p>:data&&!data.available?<p className="empty-state">A separately approved plan opens this comparison.</p>:data&&<>
 <p className="budget-comparison-context">{displayDate(data.period_start)} – {displayDate(data.period_end)} · Approved plan version {data.plan_version}</p><div className="budget-comparison-pair"><ComparisonTrack label="Confirmed collections" planned={data.planned_collections_paise} recorded={data.collections_paise}/><ComparisonTrack label="Recorded paid expenses" planned={data.planned_expenses_paise} recorded={data.recorded_expenses_paise} spending/></div>
 <div className="budget-variance-row"><p><span>Collection variance</span><strong>{money(data.collection_variance_paise)}</strong><small>Recorded minus planned collections</small></p><p><span>Expense headroom</span><strong>{money(data.expense_headroom_paise)}</strong><small>Plan minus recorded spending</small></p></div><div className={'budget-period-difference '+(data.difference_paise<0?'budget-difference-negative':'')}><div><span>Collections minus recorded expenses</span><strong>{money(data.difference_paise)}</strong></div><Icon name="leaf"/><p>Opening cash, transfers and unrecorded activity are outside this difference.</p></div>
 <details className="budget-collection-basis"><summary>See the collection basis<Icon name="chevron"/></summary><dl><div><dt>Original received entries</dt><dd>{money(data.original_collections_paise)}</dd></div><div><dt>Current linked reversals</dt><dd>{money(data.reversed_collections_paise)}</dd></div><div><dt>Original received records</dt><dd>{data.receipt_count}</dd></div><div><dt>Effective paid-expense records</dt><dd>{data.expense_count}</dd></div><div><dt>Expense proposals awaiting review</dt><dd>{data.pending_expenses}</dd></div></dl><p>Each posted received entry counts once by its supplied date. Opening credit, unpaid charges, reported payments and allocations add no extra collections.</p></details><p className="budget-comparison-time">Current operational view · {careTime(data.checked_at)}</p>
 </>}
 </section>
}
