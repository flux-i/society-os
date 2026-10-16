import { useEffect, useState } from 'react'
import { request } from '../api'
import type { StatementFile } from '../statements'
import { FormSelect } from './FilterSelect'
import { Icon } from './Icon'

type Table = { sheets: string[]; sheet: number; page: number; page_size: number; total_rows: number; columns: number; rows: string[][] }
function columnName(index: number): string { let result = ''; for (let n = index + 1; n; n = Math.floor((n - 1) / 26)) result = String.fromCharCode(65 + (n - 1) % 26) + result; return result }
export function StatementPreview({ file, onClose }: { file: StatementFile; onClose: () => void }) {
  const [sheet, setSheet] = useState(1), [page, setPage] = useState(1), [revision, setRevision] = useState(0)
  const [data, setData] = useState<Table | null>(null), [error, setError] = useState(''), [loading, setLoading] = useState(true)
  const pdf = file.content_type === 'application/pdf', url = '/api/financial-statements/' + encodeURIComponent(file.id) + '/preview'
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setError(''); setData(null)
    if (pdf) {
      fetch(url, { method: 'HEAD', credentials: 'same-origin', cache: 'no-store', signal: controller.signal }).then(response => { if (!response.ok) { if (response.status === 401) window.dispatchEvent(new Event('session-expired')); throw new Error('This PDF could not be opened with your current access. Try again.'); } setLoading(false) }).catch((err: Error) => { if (!controller.signal.aborted) { setError(err.message); setLoading(false) } })
      return () => controller.abort()
    }
    request<Table>(url + '?' + new URLSearchParams({ sheet: String(sheet), page: String(page) }), controller.signal).then(value => { setData(value); setLoading(false) }).catch((err: Error) => { if (!controller.signal.aborted) { setError(err.message); setLoading(false) } })
    return () => controller.abort()
  }, [pdf, url, sheet, page, revision])
  return <section className="statement-preview" aria-label="Original statement preview">
    <div className="statement-preview-heading"><h3>Original preview</h3><button type="button" className="text-link" onClick={onClose}>Close preview<Icon name="close"/></button></div>
    {loading ? <p role="status">Opening the original…</p> : error ? <div role="alert"><p>{error}</p><button type="button" className="button button-outline" onClick={() => setRevision(n => n + 1)}>Retry preview<Icon name="refresh"/></button></div> : pdf ? <><p className="form-help">Use the PDF controls to zoom or change pages. If your browser cannot display it here, use Download original.</p><iframe title="Original PDF preview" src={url} className="statement-pdf"/></> : <>
      <p className="form-help">Stored cell values only. Formulas are not recalculated; dates and formatting may appear as raw values. The original file remains authoritative.</p>
      {data && <>
        <FormSelect label="Preview worksheet" value={String(sheet)} onChange={value => { setSheet(Number(value)); setPage(1) }} options={data.sheets.map((name, index) => ({ value: String(index + 1), label: name }))}/>
        <div className="statement-table-scroll" tabIndex={0} role="region" aria-label="Worksheet cells, scroll horizontally for more columns"><table><thead><tr><th scope="col">Row</th>{Array.from({ length: data.columns }, (_, index) => <th key={index} scope="col">{columnName(index)}</th>)}</tr></thead><tbody>{data.rows.map((row, index) => <tr key={index}><th scope="row">{(page - 1) * data.page_size + index + 1}</th>{row.map((cell, column) => <td key={column}>{cell}</td>)}</tr>)}</tbody></table></div>
        {!data.rows.length && <p>No cells on this page.</p>}
        <div className="pagination"><span>{data.total_rows} rows · Page {page} of {Math.max(1, Math.ceil(data.total_rows / data.page_size))}</span><div><button type="button" aria-label="Previous worksheet page" disabled={page === 1} onClick={() => setPage(n => n - 1)}><Icon name="left"/></button><button type="button" aria-label="Next worksheet page" disabled={page * data.page_size >= data.total_rows} onClick={() => setPage(n => n + 1)}><Icon name="chevron"/></button></div></div>
      </>}
    </>}
  </section>
}
