import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { mutate } from '../api'
import type { User } from '../api'
import { useMessageLoad, useMessageWrite } from '../messages'
import type { MessagePreview, MessageSource, MessageSourcePage, MessageTarget, MessageTargetPage } from '../messages'
import { PortalDialog } from './PortalDialog'
import { FormSelect } from './FilterSelect'
import { FundCheck, FundUnavailable } from './FundShared'
import { FineFeedback } from './FineShared'
import { MessageCountFacts, MessageEnvelope, MessageRecipientList } from './MessageShared'
import { PageControls } from './Maintenance'
import { Icon } from './Icon'

export function MessageCompose({user,onClose,onSaved}:{user:User;onClose:()=>void;onSaved:(id:string)=>void}) {
  const [kind,setKind]=useState(user.can_review_requests?'NOTICE':'RECEIPT'),[source,setSource]=useState<MessageSource|null>(null),[channel,setChannel]=useState('EMAIL'),[target,setTarget]=useState<MessageTarget>({kind:'ALL',wing:'',ids:[]}),[selected,setSelected]=useState<Record<string,string>>({}),[reason,setReason]=useState(''),[sourceSearch,setSourceSearch]=useState(''),[sourcePage,setSourcePage]=useState(1),[targetSearch,setTargetSearch]=useState(''),[targetPage,setTargetPage]=useState(1)
  const [preview,setPreview]=useState<MessagePreview|null>(null),[previewLoading,setPreviewLoading]=useState(false),[previewError,setPreviewError]=useState(''),[checked,setChecked]=useState(false)
  const controller=useRef<AbortController|null>(null),writer=useMessageWrite()
  useEffect(()=>()=>controller.current?.abort(),[])
  const disabled=writer.busy||writer.locked,waiting=disabled||previewLoading
  const sources=useMessageLoad<MessageSourcePage>('/api/messages/sources?'+new URLSearchParams({kind,q:sourceSearch,page:String(sourcePage)}),!preview)
  const targets=useMessageLoad<MessageTargetPage>('/api/messages/targets?'+new URLSearchParams({source_kind:kind,source_id:source?.id??'',kind:target.kind,q:targetSearch,page:String(targetPage)}),!!source&&['PEOPLE','HOMES'].includes(target.kind)&&!preview)
  const resetAudience=()=>{setSelected({});setTargetSearch('');setTargetPage(1);setChecked(false)}
  const input=()=>({source_kind:kind,source_id:source?.id??'',channel,target})
  const loadPreview=async(page=1)=>{
    controller.current?.abort();const next=new AbortController();controller.current=next
    setPreviewLoading(true);setPreviewError('');setChecked(false)
    try {const out=await mutate<MessagePreview>('/api/messages/preview?page='+page,'POST',input(),next.signal);if(!next.signal.aborted)setPreview(out)}
    catch(err){if(!next.signal.aborted)setPreviewError((err as Error).message)}
    finally {if(!next.signal.aborted)setPreviewLoading(false)}
  }
  const submit=async(event:FormEvent)=>{
    event.preventDefault()
    if(!preview){await loadPreview();return}
    if(!checked||previewLoading||previewError||writer.conflict||preview.counts.destinations===0)return
    const result=await writer.send('/api/messages',{...input(),preview_hash:preview.preview_hash,reason})
    if(result)onSaved(result.id)
  }
  const reload=()=>{writer.reset();void loadPreview(1)}
  return <PortalDialog titleId="message-compose-title" closeLabel="Close message proposal" onClose={onClose} busy={disabled} className="fine-dialog message-dialog">
    <div className="dialog-scroll message-compose"><span className="eyebrow">A CONSIDERED CONVERSATION</span><h2 id="message-compose-title">{preview?'The right words. The right people.':kind==='STATEMENT'?'Share a financial statement.':'Prepare a community message.'}</h2><p className="message-simulation"><Icon name="leaf"/>Local simulation · No real WhatsApp or email will be sent.</p>
      <form className="portal-form message-form" onSubmit={event=>{void submit(event)}}>
        {preview?<>
          <MessageEnvelope envelope={preview.envelope}/><MessageCountFacts counts={preview.counts}/>
          <p className="form-help">Shared destinations count once; each person’s source access, account and permission is checked separately. Permission is checked again before handoff. Published statement delivery also requires finance-message consent.</p>
          {Object.keys(preview.counts.reasons).length>0&&<p className="message-omission-note">{preview.counts.omitted_people} people omitted. Their decisions appear below.</p>}
          {preview.counts.destinations===0&&<p className="form-error" role="alert">No eligible destination. Change the audience or verify registered contacts before proposing delivery.</p>}
          <p className="preserve-lines form-help">Proposal reason: {reason}</p>
          {!previewLoading&&!previewError&&<FundCheck checked={checked} onChange={setChecked} disabled={disabled}>I reviewed this exact content, eligible recipients, unique destinations and omissions.</FundCheck>}
          <FundUnavailable error={previewError} loading={previewLoading} onRetry={()=>{void loadPreview(preview.page)}}><MessageRecipientList items={preview.recipients} total={preview.counts.target_people} page={preview.page} onPage={page=>{void loadPreview(page)}} disabled={waiting}/></FundUnavailable>
        </>:<fieldset className="records-fieldset" disabled={waiting}>
          <div className="message-field-grid"><label>Message source<FormSelect label="Message source" value={kind} options={[...(user.can_review_requests?[{value:'NOTICE',label:'Published community notice'}]:[]),...(user.can_manage_records?[{value:'RECEIPT',label:'Original received-money receipt'},{value:'STATEMENT',label:'Published financial statement'}]:[])]} onChange={value=>{setKind(value);setSource(null);setSourcePage(1);setSourceSearch('');resetAudience();setTarget({kind:'ALL',wing:'',ids:[]})}}/></label><label>Delivery channel<FormSelect label="Delivery channel" value={channel} options={[{value:'EMAIL',label:'Email · simulation'},{value:'WHATSAPP',label:'WhatsApp · simulation'}]} onChange={setChannel}/></label></div>
          <section className="message-source-picker" aria-label="Published sources"><h3>Choose the original.</h3><label className="search-control"><Icon name="search"/><input aria-label="Search published message sources" maxLength={100} placeholder={kind==='NOTICE'?'Find a published notice…':kind==='STATEMENT'?'Find a published statement…':'Find a receipt number…'} value={sourceSearch} onChange={event=>{setSourceSearch(event.target.value);setSourcePage(1)}}/></label>
            <FundUnavailable error={sources.error} loading={sources.loading} onRetry={sources.reload}>{sources.data&&<>{sources.data.items.length?<div className="message-source-options">{sources.data.items.map(x=><button key={x.id} type="button" className={'message-source-option '+(source?.id===x.id?'message-source-selected':'')} aria-pressed={source?.id===x.id} onClick={()=>{setSource(x);resetAudience();setTarget({...target,ids:[]})}}><span><strong>{x.title}</strong><small>{x.kind==='NOTICE'?'Approved notice · '+x.audience.toLowerCase().replaceAll('_',' '):x.kind==='STATEMENT'?'Published statement · exact original and audience':'Original receipt · authenticated private link'}</small></span><Icon name={source?.id===x.id?'check':'arrow'}/></button>)}</div>:<p className="form-help">No published source matches. {kind==='NOTICE'?'Publish an independently reviewed notice in Community.':kind==='STATEMENT'?'An approved original must be deliberately published in Financial statements.':'A confirmed received entry must have its original receipt.'}</p>}{sources.data.total>12&&<PageControls label="message sources" page={sourcePage} total={sources.data.total} size={12} onPage={setSourcePage} disabled={waiting}/>}</>}</FundUnavailable>
            {source&&<p className="message-selection-summary">Selected: <strong>{source.title}</strong></p>}
          </section>
          <div className="message-field-grid"><label>Recipient group<FormSelect label="Recipient group" value={target.kind} options={[{value:'ALL',label:'All current people'},{value:'OWNERS',label:'Owners'},{value:'TENANTS',label:'Tenants'},{value:'WING',label:'One wing'},{value:'HOMES',label:'Selected homes'},{value:'PEOPLE',label:'Selected people'}]} onChange={value=>{resetAudience();setTarget({kind:value,wing:value==='WING'?'A':'',ids:[]})}}/></label>{target.kind==='WING'&&<label>Recipient wing<FormSelect label="Recipient wing" value={target.wing} options={['A','B','C'].map(value=>({value,label:'Wing '+value}))} onChange={value=>setTarget({...target,wing:value})}/></label>}</div>
          {['HOMES','PEOPLE'].includes(target.kind)&&<section className="message-target-picker" aria-label="Select message recipients"><h3>{target.kind==='HOMES'?'Choose the homes.':'Choose the people.'}</h3><p className="form-help">Choices are limited to this source’s audience. Up to 200 may be selected.</p>{source?<><label className="search-control"><Icon name="search"/><input aria-label="Search message recipients" placeholder="Find a permitted recipient…" maxLength={100} value={targetSearch} onChange={event=>{setTargetSearch(event.target.value);setTargetPage(1)}}/></label><FundUnavailable error={targets.error} loading={targets.loading} onRetry={targets.reload}>{targets.data&&<><div className="message-target-options">{targets.data.items.map(x=><FundCheck key={x.id} checked={!!selected[x.id]} disabled={!selected[x.id]&&target.ids.length>=200} onChange={value=>{const next={...selected};if(value)next[x.id]=x.label;else delete next[x.id];setSelected(next);setTarget({...target,ids:Object.keys(next).sort()})}}>{x.label}</FundCheck>)}</div>{!targets.data.items.length&&<p className="form-help">No recipients match this search.</p>}{targets.data.total>12&&<PageControls label="recipient choices" page={targetPage} total={targets.data.total} size={12} onPage={setTargetPage} disabled={waiting}/>}</>}</FundUnavailable>{target.ids.length>0&&<div className="message-selected-people"><span>{target.ids.length} selected</span>{Object.entries(selected).map(([id,label])=><button type="button" key={id} aria-label={'Remove '+label} onClick={()=>{const next={...selected};delete next[id];setSelected(next);setTarget({...target,ids:Object.keys(next).sort()})}}>{label}<Icon name="close"/></button>)}</div>}</>:<p className="form-help">Choose a published source first.</p>}</section>}
          <label>Proposal reason<textarea required minLength={10} maxLength={300} value={reason} onChange={event=>setReason(event.target.value)}/></label>
        </fieldset>}
        {!preview&&(previewLoading||previewError)&&<FundUnavailable error={previewError} loading={previewLoading} onRetry={()=>{void loadPreview()}}><span/></FundUnavailable>}
        <FineFeedback writer={writer} onReload={reload}/>
        <div className="form-actions message-form-actions">{preview?<button type="button" className="button button-light" disabled={waiting} onClick={()=>{setPreview(null);setChecked(false);setPreviewError('')}}>Edit proposal</button>:<button type="button" className="button button-light" disabled={disabled} onClick={onClose}>Cancel</button>}<button className="button button-dark" disabled={writer.busy||writer.conflict||previewLoading||!!previewError||(preview?(!checked||preview.counts.destinations===0):(!source||(['HOMES','PEOPLE'].includes(target.kind)&&target.ids.length===0)))}>{writer.busy?'Saving…':writer.locked?'Retry this proposal':preview?'Propose for separate review':'Preview exact recipients'}<Icon name="arrow"/></button></div>
      </form>
    </div>
  </PortalDialog>
}
