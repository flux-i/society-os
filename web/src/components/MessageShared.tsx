import type { MessageCounts, MessageRecipient, MessageProvider, MessageReminder } from '../messages'
import { deliveryStates, messageReasons } from '../messages'
import { displayDate, money } from '../maintenance'
import { careTime } from '../upkeep'
import { PageControls } from './Maintenance'
import { Icon } from './Icon'

export function MessageEnvelope({ envelope }: { envelope:string }) {
  return <section className="message-envelope" aria-label="Exact message preview"><span className="eyebrow">WHAT LEAVES THE PORTAL</span><p className="preserve-lines">{envelope}</p><small><Icon name="shield" />The linked record still requires a permitted portal account.</small></section>
}
export function MessageProviderFacts({provider}:{provider?:MessageProvider}) {
  if(!provider)return null
  return <section className="message-template" aria-label="Reviewed WhatsApp template"><span className="eyebrow">THE APPROVED TEMPLATE</span><dl><div><dt>Template</dt><dd>{provider.name}</dd></div><div><dt>Language</dt><dd>{provider.language}</dd></div><div><dt>Provider category</dt><dd>{provider.category==='UTILITY'?'Utility':'Marketing'}</dd></div></dl><p className="form-help">The wording and provider category are frozen with this proposal. Changed templates require a fresh review.</p></section>
}
export function MessageCountFacts({ counts }: { counts:MessageCounts }) {
  return <section className="message-counts" aria-label="Exact recipient counts">{[
    ['Targeted people',counts.target_people],['Within source audience',counts.source_people],['Verified permission',counts.consented_people],['Eligible people',counts.eligible_people],['Unique destinations',counts.destinations],['Omitted people',counts.omitted_people],
  ].map(([label,value])=><div key={label}><span>{label}</span><strong>{value}</strong></div>)}</section>
}
export function MessageReminderFacts({reminder}:{reminder?:MessageReminder}) {
  if(!reminder)return null
  return <section className="message-reminder-facts" aria-label="Reviewed reminder criterion"><span className="eyebrow">A CURRENT RECORD. A CONSIDERED REMINDER.</span><h3>{reminder.basis==='DEADLINE_PASSED'?'After the supplied deadline':'Still outstanding when checked'}</h3><p>{reminder.publication_version?`Personal acknowledgement of publication ${reminder.publication_version}.`:'Exact active charges and confirmed allocations in the selected permitted homes.'} {reminder.due_date?`Supplied due date: ${displayDate(reminder.due_date)}. Dates use Asia/Kolkata.`:reminder.deadline_at?`Supplied acknowledgement deadline: ${careTime(reminder.deadline_at)}.`:'No deadline was supplied.'}</p><small>A changed amount, completed acknowledgement or ended eligibility stops a new handoff. The shared envelope contains only a private portal link.</small></section>
}
export function MessageRecipientList({items,total,page,onPage,disabled=false}:{items:MessageRecipient[];total:number;page:number;onPage:(page:number)=>void;disabled?:boolean}) {
  return <section className="message-people" aria-label="Recipient decisions"><h3>Every person accounted for.</h3><ul>{items.map((x,index)=><li key={x.id||index}><span><strong>{x.name||'Outside source audience'}</strong>{x.destination&&<small>{x.destination}</small>}{x.reminder&&<small className="message-binding">{x.reminder.obligations.length?`${money(x.reminder.outstanding_paise)} outstanding across ${x.reminder.obligations.length} ${x.reminder.obligations.length===1?'home':'homes'}`:'Exact personal acknowledgement outstanding'}</small>}{x.reminder?.obligations.map(line=><small key={line.charge_id}>{line.home} · {money(line.outstanding_paise)} remaining</small>)}</span><span className="message-person-decision">{messageReasons[x.reason]??(x.reason||deliveryStates[x.state??'']||'Eligible when checked')}</span></li>)}</ul>{total>20&&<PageControls label="message recipients" page={page} total={total} size={20} onPage={onPage} disabled={disabled}/>}</section>
}
