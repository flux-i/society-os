import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { APIError, mutate, request } from '../api'
import type { User } from '../api'
import { dateToday, displayDate } from '../maintenance'
import { downloadFinanceExport, exportBasis, exportMoney, exportReports } from '../finance-exports'
import type { ExportChoices, ExportFilter, ExportPage, FinanceExport, FinanceReport } from '../finance-exports'
import { FormSelect } from './FilterSelect'
import { Icon } from './Icon'
import { PortalDialog } from './PortalDialog'
import { PageControls } from './Maintenance'

export function FinanceExportAction({ user, report, home = '' }: { user: User; report: FinanceReport; home?: string }) {
  const [open, setOpen] = useState(false)
  if (!user.can_export_finance) return null
  return <><button className="button button-outline" onClick={() => setOpen(true)}>Export records<Icon name="document" /></button>{open && <FinanceExportDialog initialReport={report} initialHome={home} onClose={() => setOpen(false)} />}</>
}

function ExportAmounts({ snapshot }: { snapshot: FinanceExport }) {
  const s = snapshot.summary, purpose = snapshot.report === 'MAINTENANCE' || snapshot.report === 'FUNDS'
  const amounts = purpose ? [['Selected charges', s.active_paise, 'Current approved charge amounts'], ['Purpose outstanding', s.outstanding_paise, 'Selected charges less current allocations'], ['Current net balance', s.current_net_paise, 'All current records for the scoped homes'], ['Available received credit', s.available_received_paise, 'Cash received, less current purpose assignments']] : [['Original receipts', s.original_received_paise, 'Received sources in the chosen date range'], ['Usable selected receipts', s.usable_received_paise, 'Originals less linked source reversals'], ['Current net balance', s.current_net_paise, 'All current records for the scoped homes'], ['Available received credit', s.available_received_paise, 'Cash received, less current purpose assignments']]
  return <><div className="export-amounts" aria-label="Reviewed export amounts">{amounts.map(([label, value, hint]) => <div key={label}><span>{label}</span><strong>{exportMoney(value)}</strong><small>{hint}</small></div>)}</div><p className="form-help">Opening credit {exportMoney(s.opening_credit_paise)} is separate from cash received. {purpose ? <>Selected allocations {exportMoney(s.allocated_paise)} settle this purpose.</> : <>Reversed original receipts {exportMoney(s.reversed_received_paise)} remain in the register.</>}{snapshot.report === 'FUNDS' && <> {s.pending_reports} scoped payment {s.pending_reports === 1 ? 'report awaits' : 'reports await'} verification; reports add no cash or receipts. Voluntary attributions {exportMoney(s.voluntary_paise)} are separate from fixed charges.</>}</p></>
}

export function FinanceExportDialog({ initialReport, initialHome, onClose }: { initialReport: FinanceReport; initialHome: string; onClose: () => void }) {
  const [choices, setChoices] = useState<ExportChoices | null>(null), [loading, setLoading] = useState(true), [loadError, setLoadError] = useState(''), [revision, setRevision] = useState(0)
  const [filter, setFilter] = useState<ExportFilter>({ report: initialReport, scope: 'OWN', home_id: '', fund_id: '', from: dateToday().slice(0, 8) + '01', to: dateToday() })
  const [preview, setPreview] = useState<FinanceExport | null>(null), [ready, setReady] = useState<FinanceExport | null>(null), [confirmed, setConfirmed] = useState(false)
  const [busy, setBusy] = useState<'' | 'preview' | 'create' | 'download'>(''), [locked, setLocked] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState(''), [tab, setTab] = useState('NEW')
  const [history, setHistory] = useState<ExportPage | null>(null), [historyPage, setHistoryPage] = useState(1), [historyError, setHistoryError] = useState(''), [historyLoading, setHistoryLoading] = useState(false), [historyRevision, setHistoryRevision] = useState(0)
  const pending = useRef<Record<string, unknown> | null>(null), feedback = useRef<HTMLParagraphElement>(null), scroll = useRef<HTMLDivElement>(null)
  useEffect(() => { if (error) feedback.current?.scrollIntoView({ block: 'nearest' }) }, [error])
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setLoadError('')
    request<ExportChoices>('/api/finance-exports/choices', controller.signal).then(value => {
      setChoices(value); setFilter(current => ({ ...current, scope: initialHome && value.homes.some(h => h.id === initialHome) ? 'HOME' : value.society ? 'SOCIETY' : 'OWN', home_id: initialHome && value.homes.some(h => h.id === initialHome) ? initialHome : '' })); setLoading(false)
    }).catch((err: Error) => { if (!controller.signal.aborted) { setLoadError(err.message); setLoading(false) } })
    return () => controller.abort()
  }, [initialHome, revision])
  useEffect(() => {
    if (tab !== 'SAVED') return
    const controller = new AbortController(); setHistoryLoading(true); setHistoryError('')
    request<ExportPage>('/api/finance-exports?page=' + historyPage, controller.signal).then(value => { setHistory(value); setHistoryLoading(false) }).catch((err: Error) => { if (!controller.signal.aborted) { setHistoryError(err.message); setHistoryLoading(false) } })
    return () => controller.abort()
  }, [tab, historyPage, historyRevision])
  const change = (next: Partial<ExportFilter>) => { setFilter(current => ({ ...current, ...next })); setPreview(null); setConfirmed(false); setError(''); setNotice('') }
  const newExport = () => { setPreview(null); setReady(null); setConfirmed(false); setLocked(false); setError(''); setNotice(''); pending.current = null; setTab('NEW'); scroll.current?.scrollTo(0, 0) }
  const prepare = async (event: FormEvent) => {
    event.preventDefault(); if (busy || locked) return; setBusy('preview'); setPreview(null); setConfirmed(false); setError(''); setNotice('')
    try { setPreview(await mutate<FinanceExport>('/api/finance-exports/preview', 'POST', filter)) }
    catch (err) { setError((err as Error).message) }
    finally { setBusy(''); }
  }
  const create = async () => {
    if (busy || !preview || !confirmed) return; setBusy('create'); setError(''); setNotice('')
    pending.current ??= { ...previewFilter(preview), preview_hash: preview.content_hash, operation_key: crypto.randomUUID(), confirmed: true }
    try { const result = await mutate<FinanceExport>('/api/finance-exports', 'POST', pending.current); setReady(result); setLocked(false); pending.current = null; setHistoryRevision(n => n + 1); scroll.current?.scrollTo(0, 0) }
    catch (err) {
      if (err instanceof APIError && [400, 403, 404, 409].includes(err.status)) { pending.current = null; setLocked(false); setPreview(null); setConfirmed(false); setError(err.status === 409 ? 'The records or selected homes changed. Reload the preview and review the current scope before creating this export.' : err.message) }
      else { setLocked(true); setError((err as Error).message) }
    } finally { setBusy('') }
  }
  const download = async () => {
    if (!ready || busy) return; setBusy('download'); setError(''); setNotice('')
    try { await downloadFinanceExport(ready); setNotice('The verified CSV download was started. Check your browser’s downloads.') }
    catch (err) { setError((err as Error).message) }
    finally { setBusy('') }
  }
  const openSaved = async (id: string) => {
    if (busy) return; setBusy('preview'); setError(''); setNotice('')
    try { const saved = await request<FinanceExport>('/api/finance-exports/' + encodeURIComponent(id)); setReady(saved); setTab('NEW'); scroll.current?.scrollTo(0, 0) }
    catch (err) { setError((err as Error).message) }
    finally { setBusy('') }
  }
  return <PortalDialog className="finance-export-dialog" titleId="export-title" closeLabel="Close financial export" busy={!!busy} onClose={onClose}>
    <div className="dialog-heading records-dialog-heading"><span className="eyebrow">A CLEAR COPY, WITH EVERY LINK</span><h2 id="export-title">Your records, <br /><em>ready to keep.</em></h2><p>Choose what you need. Review the scope. Keep an exact CSV snapshot.</p></div>
    <div className="dialog-scroll detail-body" ref={scroll}>
      {loading ? <div className="export-loading" role="status">Opening your permitted export choices…<div className="skeleton home-skeleton" /></div> : loadError ? <div className="empty-state" role="alert"><h3>Export access unavailable.</h3><p>{loadError}</p><button className="button button-dark" onClick={() => setRevision(n => n + 1)}>Retry export access<Icon name="refresh" /></button></div> : choices && <>
        {!ready && <div className="upkeep-tabs export-tabs" role="tablist" aria-label="Export workspaces" onKeyDown={event => { if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key) || busy || locked) return; const tabs = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="tab"]')); event.preventDefault(); const current = tabs.indexOf(document.activeElement as HTMLButtonElement), index = event.key === 'Home' ? 0 : event.key === 'End' ? 1 : (current + 1) % 2; tabs[index]?.click(); tabs[index]?.focus() }}>{[['NEW', 'New export'], ['SAVED', 'Saved snapshots']].map(([value, label]) => <button key={value} type="button" role="tab" aria-selected={tab === value} aria-controls="export-panel" tabIndex={tab === value ? 0 : -1} disabled={!!busy || locked} onClick={() => { setTab(value); setError(''); setNotice('') }}>{label}</button>)}</div>}
        <div id="export-panel" role="tabpanel">
          {ready ? <div className="export-ready"><span className="export-ready-mark"><Icon name="check" /></span><span className="eyebrow">SNAPSHOT READY</span><h3>{exportReports[ready.report]}</h3><p>{ready.scope_label} · {ready.homes.length} financial {ready.homes.length === 1 ? 'home' : 'homes'}<br />{displayDate(ready.from)} – {displayDate(ready.to)} · {ready.date_basis}</p><div className="export-file-facts"><span>{ready.rows.toLocaleString()} output {ready.rows === 1 ? 'row' : 'rows'}</span><span>{(ready.bytes / 1024).toFixed(1)} KiB · UTF-8 CSV</span><span>{new Date(ready.generated_at).toLocaleString('en-IN')}</span></div><ExportAmounts snapshot={ready} /><p className="form-help">This accepted snapshot keeps its original bytes. It includes current corrections at the recorded time; a new export captures later changes. Financial access is checked again for every download.</p></div> : tab === 'SAVED' ? <div aria-busy={historyLoading}>{historyLoading ? <p role="status">Opening your saved snapshots…</p> : historyError ? <div role="alert" className="export-history-error"><p>{historyError}</p><button className="button button-dark" onClick={() => setHistoryRevision(n => n + 1)}>Retry saved snapshots<Icon name="refresh" /></button></div> : !history?.items.length ? <div className="records-empty"><span className="chapter-icon"><Icon name="document" /></span><h3>A clear copy starts here.</h3><p>Your accepted snapshots appear while their original financial scope remains accessible.</p><button className="text-link" onClick={() => setTab('NEW')}>Prepare an export<Icon name="arrow" /></button></div> : <><div className="export-history">{history.items.map(item => <button className="export-history-row" key={item.id} disabled={!!busy} onClick={() => void openSaved(item.id)} aria-label={'Open saved ' + exportReports[item.report] + ' ' + item.scope_label}><span><strong>{exportReports[item.report]}</strong><small>{item.scope_label} · {displayDate(item.from)} – {displayDate(item.to)}</small><small>{item.rows} {item.rows === 1 ? 'row' : 'rows'} · {new Date(item.generated_at).toLocaleString('en-IN')}</small></span><Icon name="arrow" /></button>)}</div><PageControls label="export snapshots" page={history.page} size={history.page_size} total={history.total} onPage={setHistoryPage} disabled={!!busy} /></>}</div> : <form className="portal-form" onSubmit={prepare}>
            <fieldset className="records-fieldset" disabled={!!busy || locked}><label>Report<FormSelect label="Financial export report" value={filter.report} onChange={value => change({ report: value as FinanceReport, fund_id: '' })} options={Object.entries(exportReports).map(([value, label]) => ({ value, label }))} disabled={!!busy || locked} /></label>
              <div className="form-pair"><label>From<input aria-label="Export from date" type="date" required min="1900-01-01" max={dateToday()} value={filter.from} onChange={event => change({ from: event.target.value })} /></label><label>To<input aria-label="Export to date" type="date" required min={filter.from || '1900-01-01'} max={dateToday()} value={filter.to} onChange={event => change({ to: event.target.value })} /></label></div>
              <label>Financial scope<FormSelect label="Financial export scope" value={filter.scope} onChange={value => change({ scope: value as ExportFilter['scope'], home_id: '' })} options={[...(choices.society ? [{ value: 'SOCIETY', label: 'Whole society · Treasury access' }] : []), ...(choices.own_homes.length ? [{ value: 'OWN', label: 'All your financial homes' }] : []), { value: 'HOME', label: 'One permitted home' }]} disabled={!!busy || locked} /></label>
              {filter.scope === 'HOME' && <label>Home<FormSelect label="Financial export home" required value={filter.home_id} onChange={value => change({ home_id: value })} options={[{ value: '', label: 'Choose a financial home…' }, ...choices.homes.map(h => ({ value: h.id, label: 'Home ' + h.label }))]} disabled={!!busy || locked} /></label>}
              {filter.report === 'FUNDS' && <label>Published fund<FormSelect label="Financial export fund" value={filter.fund_id} onChange={value => change({ fund_id: value })} options={[{ value: '', label: 'All published funds in the range' }, ...choices.funds.map(f => ({ value: f.id, label: f.label }))]} disabled={!!busy || locked} /></label>}
            </fieldset><div className="export-scope-note"><Icon name="shield" /><p><strong>{exportBasis[filter.report]}</strong> selects original sources. Linked corrections and allocations reflect their current state. Your home balance includes all current records, beyond this date range.</p></div>
            {choices.society && !choices.fresh && filter.scope !== 'OWN' && <p className="form-help">Society and Treasury home exports require recent password and two-step verification in <a href="#security">Account security</a>.</p>}
            {!locked && <button className="button button-dark" type="submit" disabled={!!busy || (filter.scope === 'HOME' && !filter.home_id)}>{busy === 'preview' ? 'Preparing your preview…' : preview ? 'Reload current preview' : 'Preview export'}<Icon name="arrow" /></button>}
          </form>}
          {!ready && tab === 'NEW' && preview && <section className="export-review" aria-label="Financial export preview"><div className="export-review-heading"><span className="eyebrow">REVIEW BEFORE CREATING</span><h3>{preview.scope_label}</h3><p>{preview.homes.length} financial {preview.homes.length === 1 ? 'home' : 'homes'} · {preview.summary.sources} selected {preview.summary.sources === 1 ? 'source' : 'sources'} · {preview.rows} output {preview.rows === 1 ? 'row' : 'rows'}</p></div><ExportAmounts snapshot={preview} /><p className="form-help">UTF-8 CSV · {(preview.bytes / 1024).toFixed(1)} KiB · original identities and linked corrections retained. This is an operational snapshot of supplied records; externally prepared financial statements remain separate.</p><label className="export-confirm"><input type="checkbox" checked={confirmed} disabled={!!busy || locked} onChange={event => setConfirmed(event.target.checked)} /><span>I confirm the report, date range and financial scope shown above.</span></label>{locked && <p className="form-help">The response was interrupted. Retry keeps the exact operation identity and returns the same accepted snapshot if it was already created.</p>}<button className="button button-dark" type="button" disabled={!!busy || !confirmed} onClick={() => void create()}>{busy === 'create' ? 'Creating your snapshot…' : locked ? 'Retry this snapshot' : 'Create reviewed snapshot'}<Icon name="check" /></button></section>}
        </div>
        {error && <p ref={feedback} className="form-error" role="alert">{error}</p>}{notice && <p className="form-success" role="status">{notice}</p>}
        {ready && <div className="export-ready-actions"><button className="button button-dark" disabled={!!busy} onClick={() => void download()}>{busy === 'download' ? 'Checking your CSV…' : 'Download reviewed CSV'}<Icon name="document" /></button><button className="text-link" disabled={!!busy} onClick={newExport}>Prepare a new export<Icon name="refresh" /></button></div>}
      </>}
    </div>
  </PortalDialog>
}
function previewFilter(snapshot: FinanceExport): ExportFilter { return { report: snapshot.report, scope: snapshot.scope, home_id: snapshot.home_id, fund_id: snapshot.fund_id, from: snapshot.from, to: snapshot.to } }
