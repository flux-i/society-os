import type { MeetingSnapshot } from '../meetings'
import { meetingActions, meetingStates } from '../meetings'
import { communityTime } from '../community'

export function MeetingPaper({snapshot:x,state='',privateReview=false}:{snapshot:MeetingSnapshot;state?:string;privateReview?:boolean}){
 return <section className="community-paper meeting-paper" aria-label={privateReview?'Exact meeting proposal':'Approved meeting record'}>
  <div className="community-paper-kicker"><span className="eyebrow">{meetingActions[x.action]}</span>{state&&<span className={'community-state meeting-state-'+state.toLowerCase()}>{meetingStates[state]??state}</span>}</div>
  <h3>{x.title}</h3><p className="preserve-lines">{x.body}</p>
  <dl><div><dt>Scheduled start</dt><dd>{communityTime(x.start_at)}</dd></div><div><dt>Scheduled end</dt><dd>{x.end_at?communityTime(x.end_at):'Not supplied'}</dd></div><div><dt>Location</dt><dd>{x.location}</dd></div><div><dt>Approved area</dt><dd>{x.scope==='ALL'?'All homes at proposal time':x.scope==='WING'?'Wing '+x.building_code:'Selected homes'} · {x.homes.length} {x.homes.length===1?'home':'homes'}</dd></div></dl>
  {x.action==='MINUTES'&&<div className="community-restoration"><strong>Supplied minutes</strong><p className="meeting-held">Meeting held: {communityTime(x.held_at)}</p><p className="preserve-lines">{x.minutes}</p></div>}
  {x.action==='CANCEL'&&<div className="community-restoration"><strong>Public cancellation update</strong><p className="preserve-lines">{x.update_text}</p></div>}
  {x.action==='WITHDRAW'&&<p className="form-help">Approval removes current portal access. The original meeting, decisions and personal acknowledgements remain retained.</p>}
  {(x.action==='AGENDA'||x.action==='MINUTES')&&<p className="meeting-ack-policy">{x.ack_required?'Personal acknowledgement requested.'+(x.ack_deadline?' Supplied deadline: '+communityTime(x.ack_deadline)+'.':' No deadline supplied.'):'No personal acknowledgement requested for this version.'}</p>}
  {state==='PAST'&&<p className="form-help">The scheduled time has passed. Minutes have not been published for this version.</p>}
  <details className="community-area"><summary>View the exact meeting homes</summary><div>{x.homes.map(h=><span key={h.id}>{h.label}</span>)}</div></details>
  {privateReview&&x.reason&&<div className="community-private-note"><strong>Private proposal reason</strong><p className="preserve-lines">{x.reason}</p></div>}
 </section>
}
