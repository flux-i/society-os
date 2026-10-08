import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import type { CommunityOptions } from '../community'
import { communityInstant, communityLocal } from '../community'
import type { MeetingDetail, MeetingSnapshot } from '../meetings'
import { useMeetingLoad, useMeetingWrite } from '../meetings'
import { PortalDialog } from './PortalDialog'
import { MeetingPaper } from './MeetingShared'
import { FormSelect } from './FilterSelect'
import { FineFeedback } from './FineShared'
import { FundCheck, FundUnavailable } from './FundShared'
import { Icon } from './Icon'

export type MeetingDraft={title:string;body:string;location:string;scope:string;building_code:string;home_ids:string[];start_local:string;end_local:string;held_local:string;minutes:string;update_text:string;ack_required:boolean;deadline_local:string;reason:string}
function draftFor(x:MeetingSnapshot|undefined,action:string):MeetingDraft{
 return {title:x?.title??'',body:x?.body??'',location:x?.location??'',scope:x?.scope??'ALL',building_code:x?.building_code??'',home_ids:x?.scope==='HOMES'?x.homes.map(h=>h.id):[],start_local:communityLocal(x?.start_at||Math.floor(Date.now()/1000)+3600),end_local:x?.end_at?communityLocal(x.end_at):'',held_local:communityLocal(x?.held_at||Math.floor(Date.now()/1000)),minutes:x?.minutes??'',update_text:action==='CANCEL'?x?.update_text??'':'',ack_required:(action==='AGENDA'||x?.action==='MINUTES')?x?.ack_required??false:false,deadline_local:x?.ack_deadline?communityLocal(x.ack_deadline):'',reason:''}
}
export function MeetingProposal({onClose,onSaved}:{onClose:()=>void;onSaved:(id:string)=>void}){
 const writer=useMeetingWrite(),[draft,setDraft]=useState<MeetingDraft|null>(null)
 return <PortalDialog titleId="meeting-proposal-title" closeLabel="Close meeting proposal" className="community-dialog meeting-dialog" busy={writer.busy||writer.locked} onClose={onClose}><div className="dialog-heading community-dialog-heading"><span className="eyebrow">MAKE ROOM FOR A CONVERSATION</span><h2 id="meeting-proposal-title">A little time.{' '}<br/><em>A shared purpose.</em></h2></div><div className="dialog-scroll community-detail"><p className="detail-intro">Prepare the supplied agenda and exact homes. A different reviewer decides publication.</p><MeetingProposalForm action="AGENDA" draft={draft} setDraft={setDraft} writer={writer} onSaved={onSaved} onReload={writer.reset} onBack={onClose}/></div></PortalDialog>
}
export function MeetingProposalForm({resource,action,draft,setDraft,writer,onSaved,onReload,onBack}:{resource?:MeetingDetail;action:string;draft:MeetingDraft|null;setDraft:(draft:MeetingDraft|null)=>void;writer:ReturnType<typeof useMeetingWrite>;onSaved:(id:string)=>void;onReload:()=>void;onBack:()=>void}){
 const derived=action!=='AGENDA',load=useMeetingLoad<CommunityOptions>('/api/meetings/options',!derived),[preview,setPreview]=useState(false),[checked,setChecked]=useState(false),[error,setError]=useState(''),feedback=useRef<HTMLParagraphElement>(null)
 const source=resource?.published??resource?.snapshot,editable=resource?.state==='PENDING'&&resource.snapshot.action===action?resource.snapshot:derived?source:resource?.snapshot
 const form=draft??draftFor(editable,action),options=load.data,disabled=writer.busy||writer.locked,asks=(action==='AGENDA'||action==='MINUTES')&&form.ack_required
 useEffect(()=>{if(!draft)setDraft(form)},[draft,setDraft,form])
 useEffect(()=>{setChecked(false);setPreview(false)},[resource?.version])
 useEffect(()=>{if(error)feedback.current?.scrollIntoView({block:'nearest'})},[error])
 const change=(next:Partial<MeetingDraft>)=>{setDraft({...form,...next});setChecked(false);setError('')}
 const area=derived?source?.homes??[]:options?.homes.filter(h=>form.scope==='ALL'||(form.scope==='WING'&&h.label.startsWith(form.building_code+'-'))||(form.scope==='HOMES'&&form.home_ids.includes(h.id)))??[]
 const snapshot:MeetingSnapshot=derived&&source?{...source,action,held_at:action==='MINUTES'?communityInstant(form.held_local):source.held_at,minutes:action==='MINUTES'?form.minutes:source.minutes,update_text:action==='CANCEL'?form.update_text:'',ack_required:asks,ack_deadline:asks?communityInstant(form.deadline_local):0,reason:undefined}:{version:0,action,title:form.title.trim(),body:form.body.trim(),location:form.location.trim(),scope:form.scope,building_code:form.building_code,homes:area,start_at:communityInstant(form.start_local),end_at:communityInstant(form.end_local),held_at:0,ack_required:asks,ack_deadline:asks?communityInstant(form.deadline_local):0}
 const submit=async(event:FormEvent)=>{
  event.preventDefault();if(writer.busy||writer.conflict)return;setError('')
  if(!preview){
   if(!area.length){setError('Choose at least one current home for this meeting.');return}
   if(!Number.isSafeInteger(snapshot.start_at)||(snapshot.end_at&&snapshot.end_at<snapshot.start_at)){setError('The supplied end must be at or after the start.');return}
   if(action==='MINUTES'&&(!Number.isSafeInteger(snapshot.held_at)||snapshot.held_at<snapshot.start_at||snapshot.held_at>Math.floor(Date.now()/1000))){setError('The meeting-held time must be at or after the approved start and no later than now.');return}
   if(asks&&form.deadline_local&&!Number.isSafeInteger(snapshot.ack_deadline)){setError('Supply a valid acknowledgement deadline or leave it empty.');return}
   setPreview(true);setChecked(false);return
  }
  if(!checked)return
  const common={version:resource?.version??0,action,reason:form.reason},acknowledgement={ack_required:asks,ack_deadline:snapshot.ack_deadline}
  const payload=derived?{...common,...(action==='MINUTES'?{held_at:snapshot.held_at,minutes:form.minutes,...acknowledgement}:action==='CANCEL'?{update_text:form.update_text}:{})}:{...common,title:form.title,body:form.body,location:form.location,scope:form.scope,building_code:form.scope==='WING'?form.building_code:'',home_ids:form.scope==='HOMES'?form.home_ids:[],area_key:options?.area_key??'',start_at:snapshot.start_at,end_at:snapshot.end_at,...acknowledgement}
  const result=await writer.send('/api/meetings'+(resource?'/'+encodeURIComponent(resource.id)+'/proposals':''),payload);if(result)onSaved(result.id)
 }
 const reload=()=>{setPreview(false);setChecked(false);writer.reset();if(!derived)load.reload();onReload()}
 return <FundUnavailable error={derived?'':load.error} loading={!derived&&load.loading} onRetry={load.reload}><form className="portal-form community-proposal-form meeting-proposal-form" onSubmit={event=>{void submit(event)}}><h3>{action==='MINUTES'?'The minutes, as supplied.':action==='CANCEL'?'An explicit cancellation.':action==='WITHDRAW'?'Withdraw access, retain the meeting.':resource?'Prepare the next exact agenda.':'What brings us together?'}</h3>
 {preview?<><MeetingPaper snapshot={snapshot} privateReview/><p className="form-help preserve-lines">Private proposal reason: {form.reason}</p><FundCheck checked={checked} onChange={setChecked} disabled={disabled}>I checked this exact agenda, timing, homes and acknowledgement choice for separate review.</FundCheck></>:<fieldset className="records-fieldset" disabled={disabled}>
 {derived?<><MeetingPaper snapshot={source!} privateReview/>{action==='MINUTES'&&<><label>Supplied meeting-held time · IST<input type="datetime-local" required min={communityLocal(source!.start_at)} max={communityLocal(Math.floor(Date.now()/1000))} value={form.held_local} onChange={event=>change({held_local:event.target.value})}/></label><label>Supplied minutes<textarea required minLength={10} maxLength={8000} value={form.minutes} onChange={event=>change({minutes:event.target.value})}/></label><p className="form-help">Minutes retain the original agenda and schedule. Financial entries and other actions use their own approval workflows.</p></>}{action==='CANCEL'&&<label>Public cancellation update<textarea required minLength={10} maxLength={2000} value={form.update_text} onChange={event=>change({update_text:event.target.value})}/></label>}</>:<MeetingAgendaFields form={form} change={change} options={options} areaCount={area.length}/>}
 {(action==='AGENDA'||action==='MINUTES')&&<div className="meeting-ack-choice"><FundCheck checked={form.ack_required} onChange={ack_required=>change({ack_required,deadline_local:ack_required?form.deadline_local:''})}>Request each eligible member's personal acknowledgement of this version</FundCheck>{asks&&<><label>Supplied acknowledgement deadline · IST<input type="datetime-local" aria-describedby="meeting-deadline-help" min="2000-01-01T05:30" max="2100-01-01T05:29" value={form.deadline_local} onChange={event=>change({deadline_local:event.target.value})}/></label><p className="form-help" id="meeting-deadline-help">Optional. Acknowledgement records a personal portal action; it does not prove attendance, voting or consent.</p></>}</div>}
 <label>Private meeting proposal reason<textarea required minLength={10} maxLength={800} value={form.reason} onChange={event=>change({reason:event.target.value})}/></label></fieldset>}
 {error&&<p className="form-error" ref={feedback} role="alert">{error}</p>}<FineFeedback writer={writer} onReload={reload}/><div className="form-actions community-form-actions"><button type="button" className="button button-light" disabled={disabled} onClick={()=>{if(preview){setPreview(false);setChecked(false)}else onBack()}}>{preview?'Edit meeting proposal':'Back'}</button><button className="button button-dark" disabled={writer.busy||writer.conflict||(preview&&!checked)}>{writer.busy?'Saving meeting proposal…':writer.locked?'Retry this meeting proposal':preview?'Submit meeting for separate review':'Review exact meeting proposal'}<Icon name="arrow"/></button></div>
 </form></FundUnavailable>
}

function MeetingAgendaFields({form,change,options,areaCount}:{form:MeetingDraft;change:(next:Partial<MeetingDraft>)=>void;options:CommunityOptions|null;areaCount:number}){
 return <><label>Meeting title<input required minLength={5} maxLength={120} value={form.title} onChange={event=>change({title:event.target.value})}/></label><label>Supplied meeting agenda<textarea required minLength={10} maxLength={4000} value={form.body} onChange={event=>change({body:event.target.value})}/></label><label>Meeting location<input required minLength={3} maxLength={200} value={form.location} onChange={event=>change({location:event.target.value})}/></label>
 <div className="community-field-grid"><label>Scheduled start · IST<input type="datetime-local" required min="2000-01-01T05:30" max="2100-01-01T05:29" value={form.start_local} onChange={event=>change({start_local:event.target.value})}/></label><label>Scheduled end · IST<input type="datetime-local" min={form.start_local} max="2100-01-01T05:29" value={form.end_local} onChange={event=>change({end_local:event.target.value})}/></label></div><p className="form-help">The end can be left unknown. A passed schedule does not confirm that a meeting took place.</p>
 <label>Meeting publication area<FormSelect label="Meeting publication area" value={form.scope} options={[{value:'ALL',label:'All current homes'},{value:'WING',label:'One wing'},{value:'HOMES',label:'Selected homes'}]} onChange={scope=>change({scope,building_code:'',home_ids:[]})}/></label>
 {form.scope==='WING'&&<label>Meeting publication wing<FormSelect label="Meeting publication wing" value={form.building_code} required options={[{value:'',label:'Choose a wing'},...(options?.buildings??[]).map(h=>({value:h.id,label:h.label}))]} onChange={building_code=>change({building_code})}/></label>}
 {form.scope==='HOMES'&&<fieldset className="community-home-choices"><legend>Choose the exact meeting homes</legend><div>{options?.homes.map(h=><FundCheck key={h.id} checked={form.home_ids.includes(h.id)} onChange={value=>change({home_ids:value?[...form.home_ids,h.id]:form.home_ids.filter(id=>id!==h.id)})}>{h.label}</FundCheck>)}</div></fieldset>}
 <p className="form-help">{areaCount} homes in this reviewed area. Changes to the home options require a fresh review.</p></>
}
