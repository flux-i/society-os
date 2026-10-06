import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { APIError, mutate, request, uploadStatementOriginal } from '../api'
import type { StatementDetail, StatementFile } from '../statements'
import { selectOptions, statementKinds, statementSize } from '../statements'
import { displayDate } from '../maintenance'
import { PortalDialog } from './PortalDialog'
import { FormSelect } from './FilterSelect'
import { FundCheck } from './FundShared'
import { Icon } from './Icon'

type UploadSnapshot = { payload:Record<string,unknown>; file:File; id:string }
export function StatementUpload({ existing, onClose, onSaved }:{ existing?:StatementFile; onClose:()=>void; onSaved:(id:string)=>void }) {
  const [source,setSource]=useState(existing),[file,setFile]=useState<File|null>(null)
  const [title,setTitle]=useState(existing?.title??''),[kind,setKind]=useState(existing?.kind??'INCOME'),[start,setStart]=useState(existing?.period_start??''),[end,setEnd]=useState(existing?.period_end??''),[preparer,setPreparer]=useState(existing?.prepared_by??''),[reference,setReference]=useState(existing?.source??''),[reason,setReason]=useState('')
  const [preview,setPreview]=useState<UploadSnapshot|null>(null),[checked,setChecked]=useState(false),[busy,setBusy]=useState(false),[locked,setLocked]=useState(false),[conflict,setConflict]=useState(false),[reauth,setReauth]=useState(false),[error,setError]=useState(''),[stage,setStage]=useState('')
  const pending=useRef<UploadSnapshot|null>(null),flight=useRef(false),alive=useRef(true),feedback=useRef<HTMLDivElement>(null)
  useEffect(()=>{alive.current=true;return()=>{alive.current=false}},[])
  useEffect(()=>{if(error)feedback.current?.scrollIntoView({block:'nearest'})},[error])
  const resume=!!source?.can_upload,disabled=busy||locked
  const prepare=async()=>{
    if(!file||flight.current)return
    flight.current=true;setBusy(true);setError('');setStage('Preparing the original preview…')
    try {
      if(!/\.(pdf|xlsx|csv)$/i.test(file.name)||file.size<1||file.size>10*1024*1024||(/\.(pdf|csv)$/i.test(file.name)&&file.size>4*1024*1024))throw new Error('Choose a nonempty PDF/CSV up to 4 MB or plain XLSX up to 10 MB.')
      const sha=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',await file.arrayBuffer()))).map(x=>x.toString(16).padStart(2,'0')).join('')
      if(resume&&source&&(file.size!==source.size_bytes||sha!==source.sha256))throw new Error('Choose the unchanged original matching the existing reservation.')
      const payload={operation_key:crypto.randomUUID(),title:title.trim(),kind,period_start:start,period_end:end,prepared_by:preparer.trim(),source:reference.trim(),filename:file.name,size_bytes:file.size,sha256:sha,replaces_id:source&&!resume?source.id:'',version:source&&!resume?source.version:0,confirmed:true,reason:reason.trim()}
      if(alive.current){setPreview({payload,file,id:resume?source!.id:''});setChecked(false)}
    } catch(err){if(alive.current)setError((err as Error).message)}
    finally{flight.current=false;if(alive.current)setBusy(false)}
  }
  const submit=async(event:FormEvent)=>{
    event.preventDefault()
    if(!preview){await prepare();return}
    if(!checked||flight.current||conflict)return
    flight.current=true;setBusy(true);setError('');setReauth(false);pending.current??=preview
    try {
      if(!pending.current.id){setStage('Saving the reviewed original…');const out=await mutate<{id:string}>('/api/financial-statements','POST',pending.current.payload);pending.current.id=out.id}
      setStage('Uploading the unchanged original…');await uploadStatementOriginal(pending.current.id,pending.current.file)
      const id=pending.current.id;pending.current=null;setLocked(false);onSaved(id)
    } catch(err){
      const definite=err instanceof APIError&&[400,403,404,428].includes(err.status)
      if(definite)pending.current=null
      setError((err as Error).message);setLocked(!definite);setConflict(err instanceof APIError&&err.status===409);setReauth(err instanceof APIError&&err.code==='reauthentication_required')
    } finally{flight.current=false;if(alive.current)setBusy(false)}
  }
  const reload=async()=>{
    if(busy)return
    setBusy(true);setError('')
    try {
      if(source){const current=await request<StatementDetail>('/api/financial-statements/'+encodeURIComponent(source.id));const next=current.current?current:current.versions.find(x=>x.current&&x.state==='APPROVED');if(!next)throw new Error('Open the current approved version before preparing its replacement.');setSource(next)}
      pending.current=null;setLocked(false);setConflict(false);setPreview(null);setChecked(false)
    }catch(err){setError((err as Error).message)}finally{setBusy(false)}
  }
  return <PortalDialog titleId="statement-upload-title" closeLabel="Close statement upload" onClose={onClose} busy={disabled} className="fine-dialog statement-dialog">
    <div className="dialog-scroll statement-upload"><span className="eyebrow">THE ORIGINAL. THE CONTEXT. A SECOND REVIEW.</span><h2 id="statement-upload-title">{preview?'One original, clearly recorded.':resume?'Finish the original upload.':source?'A carefully kept replacement.':'Bring the accounts together.'}</h2><p className="form-help">Prepared outside this portal. Uploading creates no ledger entry or receipt.</p>
      <form className="portal-form statement-form" onSubmit={event=>{void submit(event)}}>
        {preview?<><section className="statement-paper" aria-label="Original statement preview"><span className="eyebrow">{statementKinds[String(preview.payload.kind)]}</span><h3>{String(preview.payload.title)}</h3><dl><div><dt>Period</dt><dd>{displayDate(String(preview.payload.period_start))} – {displayDate(String(preview.payload.period_end))}</dd></div><div><dt>Prepared by</dt><dd>{String(preview.payload.prepared_by)}</dd></div><div><dt>Source</dt><dd>{String(preview.payload.source)}</dd></div><div><dt>Unchanged original</dt><dd>{preview.file.name} · {statementSize(preview.file.size)}</dd></div></dl></section><p className="preserve-lines form-help">Reason: {String(preview.payload.reason)}</p><FundCheck checked={checked} onChange={setChecked} disabled={disabled}>I checked this exact original, period, preparer and source. It stays private until separately reviewed and deliberately published.</FundCheck></>:<fieldset className="records-fieldset" disabled={disabled}>
          <label>Original file<span className="document-upload-target"><input type="file" className="document-file-input" aria-label="Statement original" accept=".pdf,.xlsx,.csv" required onChange={event=>{setFile(event.target.files?.[0]??null);setError('')}}/><Icon name="document"/><strong>{file?.name??'Choose the prepared original'}</strong><span>{file?statementSize(file.size):'Plain PDF, XLSX or UTF-8 CSV'}</span></span></label>
          <label>Statement title<input required minLength={5} maxLength={120} disabled={resume} value={title} onChange={event=>setTitle(event.target.value)} placeholder="A clear name for these accounts"/></label>
          <label>Statement type<FormSelect label="Statement type" value={kind} disabled={!!source} options={selectOptions(statementKinds)} onChange={setKind}/></label>
          <div className="statement-field-grid"><label>Period from<input type="date" required disabled={!!source} value={start} onChange={event=>setStart(event.target.value)}/></label><label>Period to<input type="date" required min={start||undefined} disabled={!!source} value={end} onChange={event=>setEnd(event.target.value)}/></label></div>
          <label>Prepared by<input required minLength={2} maxLength={120} disabled={resume} value={preparer} onChange={event=>setPreparer(event.target.value)} placeholder="Accountant or source author"/></label>
          <label>Source reference<textarea required minLength={5} maxLength={300} disabled={resume} value={reference} onChange={event=>setReference(event.target.value)}/></label>
          <label>Upload reason<textarea required minLength={5} maxLength={300} value={reason} onChange={event=>setReason(event.target.value)}/></label>
        </fieldset>}
        <p className="form-help">PDF/CSV up to 4 MB; plain worksheet XLSX up to 10 MB. Originals and earlier versions are retained.</p>
        {error&&<div className="form-error" role="alert" ref={feedback}><p>{error}</p>{conflict&&<button type="button" className="text-link" onClick={()=>{void reload()}}>Reload current version<Icon name="refresh"/></button>}</div>}{locked&&!conflict&&<p className="form-help">Retry keeps this exact original and submission identity.</p>}{reauth&&<a className="text-link" href="#security">Confirm your identity in Account security<Icon name="arrow"/></a>}
        <div className="form-actions message-form-actions"><button type="button" className="button button-light" disabled={disabled} onClick={()=>preview?(setPreview(null),setChecked(false)):onClose()}>{preview?'Edit original details':'Cancel'}</button><button className="button button-dark" disabled={busy||conflict||!file||(!!preview&&!checked)}>{busy?stage:locked?'Retry this upload':preview?'Upload for separate review':'Preview original'}<Icon name="arrow"/></button></div>
      </form>
    </div>
  </PortalDialog>
}
