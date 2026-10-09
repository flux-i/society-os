import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { APIError, mutate, request, statuses } from '../api'
import type { User } from '../api'
import { canImportRegistry } from '../registry-import'
import type { ImportCounts, ImportHome, ImportIssue, ImportPreview, ImportResult, ImportStatus } from '../registry-import'
import { Icon } from './Icon'
import { FilterSelect } from './FilterSelect'
import { PortalDialog } from './PortalDialog'
import { relationshipLabel } from './RegistryEditor'

const maxBytes=2*1024*1024
function Counts({counts}:{counts:ImportCounts}) {
 return <dl className="import-counts" aria-label="Supplied register totals">{[[counts.homes,'homes'],[counts.occupied,'occupied'],[counts.vacant,'vacant'],[counts.owners,'owners'],[counts.tenants,'tenants']].map(([value,label])=><div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
}
function Issues({items,label,total=items.length}:{items:ImportIssue[];label:string;total?:number}) {
 return <section className={'import-issues '+(label==='Check these rows'?'import-errors':'')} aria-label={label}><h3>{label}<span>{total}</span></h3><ul>{items.map((item,index)=><li key={index}><strong>{item.section}{item.row>0?' · row '+item.row:''} · {item.field}</strong><p>{item.message}</p></li>)}</ul>{total>items.length&&<p className="form-help">Showing the first {items.length} issues. Resolve these and validate again.</p>}</section>
}
function Members({home}:{home:ImportHome}) {
 return <div className="import-members">{home.members.map(member=><article key={member.source_id}><span className="avatar">{member.name.split(' ').map(x=>x[0]).slice(-2).join('')}</span><div><h3>{member.name}</h3><p>{relationshipLabel(member.relationship)}{member.primary_contact&&!member.end_date?' · Primary contact':''}{member.end_date?' · Former relationship':''}</p><small>{member.start_date}{member.end_date?' to '+member.end_date:' onwards'}</small><span className="import-source">Source identity · {member.source_id}</span></div></article>)}</div>
}
function ApplyDialog({preview,inputText,operationKey,onClose,onApplied}:{preview:ImportPreview;inputText:string;operationKey:string;onClose:()=>void;onApplied:(result:ImportResult)=>void}) {
 const [note,setNote]=useState(''),[confirmed,setConfirmed]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState(''),[checking,setChecking]=useState(false),[checked,setChecked]=useState('')
 const inFlight=useRef(false)
 const submit=async(event:FormEvent)=>{
  event.preventDefault();if(inFlight.current)return;inFlight.current=true;setBusy(true);setError('');setChecked('')
  try{onApplied(await mutate<ImportResult>('/api/registry/import/apply','POST',{input_text:inputText,digest:preview.digest,base_digest:preview.base_digest,effective_date:preview.effective_date,operation_key:operationKey,confirmed,note}))}
  catch(err){setError(err instanceof APIError&&err.status===409?'The register changed after this preview. Close this review and validate the file again.':(err as Error).message)}
  finally{inFlight.current=false;setBusy(false)}
 }
 const check=async()=>{setChecking(true);setChecked('');try{const status=await request<ImportStatus>('/api/registry/import');if(status.applied?.digest===preview.digest)onApplied(status.applied);else setChecked('This exact file has not been recorded. You can deliberately retry when your connection is ready.')}catch(err){setError((err as Error).message)}finally{setChecking(false)}}
 return <PortalDialog titleId="import-apply-title" closeLabel="Close import confirmation" onClose={onClose} busy={busy||checking} className="import-dialog">
  <div className="dialog-scroll import-dialog-body"><span className="eyebrow">ONE REVIEWED BEGINNING</span><h2 id="import-apply-title">Bring these<br/><em>homes in.</em></h2><p>Apply the register you reviewed. Existing financial records, accounts and contact permissions are separate.</p><Counts counts={preview.counts}/><dl className="import-provenance"><div><dt>Supplied source</dt><dd>{preview.source_key}</dd></div><div><dt>Exact file fingerprint</dt><dd><code>{preview.digest}</code></dd></div><div><dt>Relationships checked for</dt><dd>{preview.effective_date}</dd></div></dl>
  <form className="portal-form" onSubmit={submit}><label>Verification note<textarea aria-label="Verification note" value={note} onChange={e=>setNote(e.target.value)} minLength={10} maxLength={300} required rows={3} placeholder="How did you verify this supplied register?" disabled={busy||checking}/></label><label className="checkbox-label"><input type="checkbox" checked={confirmed} onChange={e=>setConfirmed(e.target.checked)} required disabled={busy||checking}/>I checked this register and its source identities.</label>{error&&<div className="form-error" role="alert"><p>{error}</p><button type="button" className="text-link" disabled={busy||checking} onClick={()=>void check()}>Check saved result<Icon name="refresh"/></button></div>}{checked&&<p className="form-help" role="status">{checked}</p>}<button type="submit" className="button button-dark" disabled={busy||checking||!confirmed}>{busy?'Adding the register…':checking?'Checking the saved result…':'Apply this register'}<Icon name="check"/></button><button type="button" className="text-link" onClick={onClose} disabled={busy||checking}>Back to the preview<Icon name="left"/></button></form></div>
 </PortalDialog>
}

export function RegistryImport({user}:{user:User}) {
 const permitted=canImportRegistry(user)
 const [status,setStatus]=useState<ImportStatus|null>(null),[loading,setLoading]=useState(true),[error,setError]=useState(''),[refresh,setRefresh]=useState(0)
 const [inputText,setInputText]=useState(''),[filename,setFilename]=useState(''),[fileKey,setFileKey]=useState(0),[operationKey,setOperationKey]=useState(''),[busy,setBusy]=useState(false)
 const [preview,setPreview]=useState<ImportPreview|null>(null),[applied,setApplied]=useState<ImportResult|null>(null),[query,setQuery]=useState(''),[occupancy,setOccupancy]=useState(''),[page,setPage]=useState(1),[selected,setSelected]=useState<ImportHome|null>(null),[confirm,setConfirm]=useState(false)
 const fileGeneration=useRef(0)
 useEffect(()=>{
  if(!permitted)return
  const controller=new AbortController();setLoading(true);setError('')
  void request<ImportStatus>('/api/registry/import',controller.signal).then(setStatus).catch(err=>{if(!controller.signal.aborted)setError((err as Error).message)}).finally(()=>{if(!controller.signal.aborted)setLoading(false)})
  return()=>controller.abort()
 },[permitted,refresh])
 useEffect(()=>()=>{fileGeneration.current++},[])
 const clear=()=>{fileGeneration.current++;setInputText('');setFilename('');setPreview(null);setApplied(null);setError('');setFileKey(n=>n+1);setQuery('');setOccupancy('');setPage(1);setSelected(null);setConfirm(false)}
 const readFile=async(file?:File)=>{
  clear();if(!file)return;const generation=fileGeneration.current;setBusy(true)
  try{if(file.size>maxBytes||file.size===0)throw new Error('Choose a non-empty JSON register of at most 2 MiB.');const text=new TextDecoder('utf-8',{fatal:true}).decode(await file.arrayBuffer());if(generation!==fileGeneration.current)return;setInputText(text);setFilename(file.name);setOperationKey(crypto.randomUUID())}
  catch(err){if(generation===fileGeneration.current)setError(err instanceof TypeError?'This file is not valid UTF-8 text. Save the register as UTF-8 JSON and choose it again.':(err as Error).message)}
  finally{if(generation===fileGeneration.current)setBusy(false)}
 }
 const validate=async(event:FormEvent)=>{event.preventDefault();setBusy(true);setError('');setPreview(null);setQuery('');setOccupancy('');setPage(1);try{setPreview(await mutate<ImportPreview>('/api/registry/import/preview','POST',{input_text:inputText}))}catch(err){setError((err as Error).message)}finally{setBusy(false)}}
 const template=()=>{
  const sample={format_version:1,society_key:status?.society_key,source_key:'initial-register-v1',buildings:[{source_id:'wing-a',code:'A',name:'Wing A'}],homes:[{source_id:'a-101',building:'wing-a',number:'101',floor:1,occupancy:'OWNER_OCCUPIED'}],people:[{source_id:'owner-101',name:'Fictional Owner'}],relationships:[{source_id:'owner-a-101',home:'a-101',person:'owner-101',relationship:'OWNER',start_date:'2020-01-01',end_date:'',primary_contact:true}]}
  const url=URL.createObjectURL(new Blob([JSON.stringify(sample,null,2)+'\n'],{type:'application/json'})),a=document.createElement('a');a.href=url;a.download='society-register-template.json';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)
 }
 const onApplied=(result:ImportResult)=>{setApplied(result);setConfirm(false);setInputText('');setFilename('');setPreview(null);setRefresh(n=>n+1)}
 const filtered=(preview?.homes??[]).filter(home=>(!occupancy||home.occupancy===occupancy)&&(!query||(home.label+' '+home.members.map(m=>m.name).join(' ')).toLowerCase().includes(query.toLowerCase())))
 const saved=applied??preview?.already_applied??status?.applied
 if(!permitted)return <section className="empty-state"><Icon name="shield"/><h1>Registry access required.</h1><p>The current registry administrator prepares a workspace’s initial register.</p></section>
 return <section className="import-workspace" aria-labelledby="import-title">
  <div className="import-heading"><div><span className="eyebrow">A PLACE FOR EVERY HOME</span><h1 id="import-title">A considered<br/><em>beginning.</em></h1><p>Bring your supplied register together. Check every home before it becomes part of your workspace.</p></div><span className="import-seal"><Icon name="homes"/><span>YOUR COMMUNITY,<br/>CAREFULLY RECORDED.</span></span></div>
  <ol className="import-steps"><li><span>01</span><div><strong>Choose your register</strong><small>A supplied, verified source</small></div></li><li><span>02</span><div><strong>Review the details</strong><small>Homes, people & relationships</small></div></li><li><span>03</span><div><strong>Give it a beginning</strong><small>One deliberate confirmation</small></div></li></ol>
  {loading&&<p role="status" className="import-loading">Opening your register…</p>}
  {error&&<div className="connection-error" role="alert"><p>{error}</p>{!status&&<button onClick={()=>setRefresh(n=>n+1)}>Try again<Icon name="refresh"/></button>}</div>}
  {!loading&&status&&<>
   {saved?<article className="import-saved"><span className="import-saved-mark"><Icon name="check"/></span><span className="eyebrow">A BEGINNING, PRESERVED</span><h2>Your register<br/><em>is here.</em></h2><p>This exact source is retained. Later changes follow each home’s reviewed registry controls.</p><Counts counts={saved.counts}/><dl className="import-provenance"><div><dt>Source</dt><dd>{saved.source_key}</dd></div><div><dt>Exact file fingerprint</dt><dd><code>{saved.digest}</code></dd></div><div><dt>Import reference</dt><dd>{saved.id}</dd></div></dl><a href="#homes" className="button button-dark">Explore your homes<Icon name="arrow"/></a></article>:!status.initial_import_available?<div className="empty-state"><h2>Your register has already begun.</h2><p>Existing homes and people are preserved. Open a home to make a reviewed change.</p><a href="#homes" className="button button-dark">Open homes<Icon name="arrow"/></a></div>:<>
    <div className="import-file-card"><div><span className="eyebrow">THE REGISTER YOU TRUST</span><h2>Every detail,<br/><em>from your source.</em></h2><p>Start with your prepared JSON register. Nothing is added while you validate or review it.</p><button type="button" className="text-link" onClick={template} disabled={busy}>Download a register template<Icon name="document"/></button><p className="form-help">Homes and relationships only. Account invitations, finance access and contact preferences are managed separately.</p></div><form className="portal-form import-file-form" data-protect-navigation={inputText?'true':undefined} onSubmit={validate}><label className="import-file-label">Supplied register<input key={fileKey} aria-label="Supplied register" type="file" accept=".json,application/json" disabled={busy} onChange={e=>void readFile(e.target.files?.[0])}/><span><Icon name="document"/>{filename||'Choose your JSON register'}</span><small>UTF-8 JSON · up to 2 MiB</small></label><button className="button button-dark" disabled={!inputText||busy}>{busy?'Checking your register…':preview?'Validate this file again':'Validate & preview'}<Icon name="arrow"/></button>{inputText&&<button className="text-link" type="button" disabled={busy} onClick={clear}>Clear this file<Icon name="close"/></button>}</form></div>
    {preview&&<section className="import-preview" aria-labelledby="import-preview-title"><div className="import-preview-heading"><div><span className="eyebrow">YOUR SUPPLIED REGISTER</span><h2 id="import-preview-title">Look a little <em>closer.</em></h2><p>{preview.error_count?'Resolve the listed rows, choose your corrected file and validate again.':'Review the proposed homes and open their people and relationships.'}</p></div>{preview.can_apply&&<button type="button" className="button button-dark" onClick={()=>setConfirm(true)}>Review & apply<Icon name="check"/></button>}</div><Counts counts={preview.counts}/>{preview.error_count>0?<Issues items={preview.errors} label="Check these rows" total={preview.error_count}/>:<><details className="import-file-proof"><summary>Exact source & file fingerprint</summary><dl className="import-provenance"><div><dt>Source</dt><dd>{preview.source_key}</dd></div><div><dt>SHA-256</dt><dd><code>{preview.digest}</code></dd></div></dl></details>{preview.warnings.length>0&&<Issues items={preview.warnings} label="Details to review"/>}<div className="import-filters"><label className="search-control"><Icon name="search"/><input aria-label="Search proposed homes or people" placeholder="Find a home or a person…" value={query} maxLength={100} onChange={e=>{setQuery(e.target.value);setPage(1)}}/></label><FilterSelect label="Proposed occupancy" value={occupancy} onChange={value=>{setOccupancy(value);setPage(1)}} options={[{value:'',label:'All occupancy'},...Object.entries(statuses).map(([value,label])=>({value,label}))]}/><span role="status">{filtered.length} proposed homes</span></div>{filtered.length===0?<div className="empty-state"><h3>No proposed homes match.</h3><button type="button" className="text-link" onClick={()=>{setQuery('');setOccupancy('');setPage(1)}}>Clear preview filters<Icon name="close"/></button></div>:<div className="import-homes">{filtered.slice((page-1)*12,page*12).map(home=><button type="button" className="import-home" key={home.source_id} onClick={()=>setSelected(home)} aria-label={'Review proposed home '+home.label}><span className="import-home-top"><span>{statuses[home.occupancy]}</span><Icon name="arrow"/></span><strong>{home.label}</strong><span>{home.members.length} supplied relationships · Floor {String(home.floor).padStart(2,'0')}</span></button>)}</div>}<div className="pagination"><span>Page {page} of {Math.max(1,Math.ceil(filtered.length/12))}</span><div><button aria-label="Previous proposed homes" disabled={page===1} onClick={()=>setPage(n=>n-1)}><Icon name="left"/></button><button aria-label="Next proposed homes" disabled={page*12>=filtered.length} onClick={()=>setPage(n=>n+1)}><Icon name="chevron"/></button></div></div></>}</section>}
   </>}
  </>}
  {selected&&<PortalDialog titleId="import-home-title" closeLabel="Close proposed home" onClose={()=>setSelected(null)} className="import-dialog"><div className="dialog-scroll import-dialog-body"><span className="eyebrow">PROPOSED HOME · {statuses[selected.occupancy]}</span><h2 id="import-home-title">{selected.label}</h2><p>Supplied people and dates. This preview has not changed the register.</p><Members home={selected}/><p className="form-help">Source identity · {selected.source_id}</p></div></PortalDialog>}
  {confirm&&preview&&<ApplyDialog preview={preview} inputText={inputText} operationKey={operationKey} onClose={()=>setConfirm(false)} onApplied={onApplied}/>} 
 </section>
}
