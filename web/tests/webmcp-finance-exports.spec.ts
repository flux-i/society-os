import { test, expect } from '@playwright/test'
import { login } from './helpers'
import { names, execute } from './native-webmcp-helpers'
import type { ContextDocument } from './native-webmcp-helpers'
import { apiExport, exportEntry, openExport } from './finance-exports-fixtures'
import { messagePrivateFixture } from './messages-fixtures'

test.setTimeout(90000)
test('actual native export tools expose bounded status only preserve human forms and reject mutations and invalid schemas', async ({ page, browser }) => {
  await login(page); await exportEntry(page, 'RECEIVED', '432.19')
  const context = await browser.newContext({ baseURL: new URL(page.url()).origin }), owner = await context.newPage()
  try {
    await login(owner, 'Owner'); const snapshot = await apiExport(owner); await expect.poll(() => names(owner)).toContain('society_read_finance_export'); await openExport(owner); await owner.getByLabel('Export from date', { exact: true }).fill('2026-01-02')
    const writes: string[] = []; const observe = (r: { method: () => string; url: () => string }) => { if (!['GET', 'HEAD'].includes(r.method()) || r.url().endsWith('/download')) writes.push(r.url()) }; owner.on('request', observe)
    const metadata = JSON.parse(await execute(owner, 'society_read_finance_export', { export_id: snapshot.id })); expect(Object.keys(metadata).sort()).toEqual(['id', 'report', 'scope', 'date_basis', 'from', 'to', 'generated_at', 'rows', 'bytes', 'homes_count'].sort()); expect(metadata).toMatchObject({ id: snapshot.id, report: 'LEDGER', scope: 'OWN', rows: 2, homes_count: 2 }); for (const key of ['summary', 'current_net_paise', 'amount_paise', 'sha256', 'homes', 'home_id', 'description', 'payer', 'reference', 'csv_bytes', 'link']) expect(metadata).not.toHaveProperty(key)
    const found = JSON.parse(await execute(owner, 'society_find_finance_exports', {})); expect(found.items).toEqual([metadata]); expect(found.page_size).toBe(12); expect(writes).toEqual([]); await expect(owner.getByLabel('Export from date', { exact: true })).toHaveValue('2026-01-02'); await expect(execute(owner, 'society_open_workspace', { screen: 'entries' })).rejects.toThrow()
    for (const [tool, args] of [['society_find_finance_exports', { page: 0 }], ['society_find_finance_exports', { page: 10001 }], ['society_find_finance_exports', { report: 'LEDGER' }], ['society_read_finance_export', { export_id: '' }], ['society_read_finance_export', { export_id: 'x'.repeat(101) }], ['society_read_finance_export', { export_id: snapshot.id, action: 'DOWNLOAD' }]] as const) await expect(execute(owner, tool, args)).rejects.toThrow()
    owner.off('request', observe); await owner.keyboard.press('Escape'); for (let i = 0; i < 12; i++) await apiExport(owner); const bounded = JSON.parse(await execute(owner, 'society_find_finance_exports', {})); expect(bounded.total).toBe(13); expect(bounded.items).toHaveLength(12); expect(JSON.parse(await execute(owner, 'society_find_finance_exports', { page: 2 })).items).toHaveLength(1); expect((await names(owner)).filter(x => x.includes('finance_export')).sort()).toEqual(['society_find_finance_exports', 'society_read_finance_export'])
  } finally { await context.close() }
})

test('native export reads deny tenant and other actor scopes cancel held results after ended home access and honor abort', async ({ page, browser }) => {
  await login(page); const privileged = await apiExport(page, 'LEDGER', 'SOCIETY')
  const ownerContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), owner = await ownerContext.newPage(), tenantContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), tenant = await tenantContext.newPage(); let release = () => {}
  try {
    await login(owner, 'Owner'); await login(tenant, 'Tenant'); expect((await names(tenant)).filter(name => name.includes('finance_export'))).toEqual([]); await expect(execute(owner, 'society_read_finance_export', { export_id: privileged.id })).rejects.toThrow()
    const cancelled = await owner.evaluate(async () => { const c = (document as ContextDocument).modelContext, tool = (await c.getTools()).find(t => t.name === 'society_find_finance_exports')!, controller = new AbortController(); controller.abort(); const major = Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1]); try { await c.executeTool(tool, major < 155 ? '{}' : {}, { signal: controller.signal }); return false } catch { return true } }); expect(cancelled).toBe(true)
    const own = await apiExport(owner); await openExport(owner); await owner.getByLabel('Export from date', { exact: true }).fill('2026-01-03')
    let ready!: () => void; const held = new Promise<void>(r => { release = r }), captured = new Promise<void>(r => { ready = r }); await owner.route('**/api/finance-exports/' + own.id, async route => { const response = await route.fetch(); ready(); await held; await route.fulfill({ response }).catch(() => {}) }); const reading = execute(owner, 'society_read_finance_export', { export_id: own.id }); await captured
    messagePrivateFixture("db.execute(\"UPDATE flat_memberships SET end_date=date('now') WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101' AND end_date IS NULL\")"); release(); await expect(reading).rejects.toThrow(); await owner.unroute('**/api/finance-exports/' + own.id); await expect(owner.getByRole('dialog')).toHaveCount(0); await expect(execute(owner, 'society_read_finance_export', { export_id: own.id })).rejects.toThrow(); expect(JSON.parse(await execute(owner, 'society_find_finance_exports', {})).total).toBe(0)
  } finally { release(); messagePrivateFixture("db.execute(\"UPDATE flat_memberships SET end_date=NULL WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'\")"); await ownerContext.close(); await tenantContext.close() }
})
