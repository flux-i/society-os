import { expect } from '@playwright/test'
import type { Page, Browser } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { login, navigate } from './helpers'
import { financialHeaders, apiMaintenance, apiPublish } from './maintenance-fixtures'
import { apiFund, apiFundAction, apiFundReport } from './collections-fixtures'
import { statementActors } from './statements-fixtures'
import { messagePost } from './messages-fixtures'

export const exportCheck = 'I confirm the report, date range and financial scope shown above.'
export async function exportEntry(page: Page, kind: string, amount: string, home = 'demo-flat-A-101') {
  const response = await page.request.post('/api/entries', { headers: await financialHeaders(page), data: { operation_key: crypto.randomUUID(), flat_id: home, kind, amount, date: '2026-01-01', description: 'Fictional export source ' + kind, payer: kind === 'RECEIVED' ? 'Fictional owner' : '', method: kind === 'RECEIVED' ? 'CASH' : '', reference: '' } })
  expect(response.status(), await response.text()).toBe(200); const out = await response.json()
  await messagePost(page, '/api/entries/' + out.id + '/post', { reason: '' }); return out.id as string
}
export async function exactExportSources(page: Page, browser: Browser) {
  const actors = await statementActors(page, browser), period = await apiMaintenance(page, 'Export exact supplied maintenance', [{ flat_id: 'demo-flat-A-101', amount: '200.00' }]); await apiPublish(actors.reviewer, period)
  const maintenance = await (await page.request.get('/api/maintenance/' + period)).json()
  const fund = await apiFund(page, 'Export exact supplied fund', { lines: [{ flat_id: 'demo-flat-A-101', amount: '432.19' }] }); const f = await apiFundAction(actors.reviewer, fund, 'PUBLISHED')
  const manual = await exportEntry(page, 'CHARGE', '0.01'), opening = await exportEntry(page, 'OPENING_CREDIT', '100.00'), received = await exportEntry(page, 'RECEIVED', '500.00'), reversed = await exportEntry(page, 'RECEIVED', '1.01')
  await messagePost(page, '/api/entries/' + reversed + '/reverse', { reason: 'Wrong fictional reference preserved with a linked correction.' })
  const allocate = async (source: string, charge: string, amount: string) => (await messagePost(page, '/api/allocations', { source_id: source, charge_id: charge, amount, reason: 'PRIVATE export purpose assignment deliberately checked against originals.' })).id as string
  await allocate(opening, maintenance.lines[0].entry_id, '100.00'); await allocate(received, maintenance.lines[0].entry_id, '100.00')
  const old = await allocate(received, f.lines[0].entry_id, '100.00'); await messagePost(page, '/api/allocations/' + old + '/reverse', { reason: 'PRIVATE linked correction before the reviewed new allocation.' }); const live = await allocate(received, f.lines[0].entry_id, '60.00')
  const context = await browser.newContext({ baseURL: new URL(page.url()).origin }), owner = await context.newPage(); await login(owner, 'Owner'); await apiFundReport(owner, fund, '0.01', 'PRIVATE export pending claim')
  return { ...actors, owner, manual, opening, received, reversed, fund, period, old, live, close: async () => { await context.close(); await actors.close() } }
}
export async function openExport(page: Page, workspace = 'Entries') { await navigate(page, workspace); await page.getByRole('button', { name: 'Export records', exact: true }).click(); await expect(page.getByRole('combobox', { name: 'Financial export report', exact: true })).toBeVisible() }
export async function exportDates(page: Page) { await page.getByLabel('Export from date', { exact: true }).fill('2026-01-01'); await page.getByLabel('Export to date', { exact: true }).fill('2026-01-31') }
export async function previewExport(page: Page) { await exportDates(page); await page.getByRole('button', { name: 'Preview export', exact: true }).click(); await expect(page.getByRole('region', { name: 'Financial export preview' })).toBeVisible() }
export async function createExportUI(page: Page) { await page.getByRole('checkbox', { name: exportCheck, exact: true }).check(); const response = page.waitForResponse(r => new URL(r.url()).pathname === '/api/finance-exports' && r.request().method() === 'POST'); await page.getByRole('button', { name: 'Create reviewed snapshot', exact: true }).click(); const result = await response; expect(result.status(), await result.text()).toBe(200); const snapshot = await result.json(); await expect(page.getByRole('button', { name: 'Download reviewed CSV', exact: true })).toBeVisible(); return snapshot }
export async function downloadedCSV(page: Page) { const downloaded = page.waitForEvent('download'); await page.getByRole('button', { name: 'Download reviewed CSV', exact: true }).click(); const file = await downloaded; const path = await file.path(); expect(path).toBeTruthy(); return { bytes: readFileSync(path!), filename: file.suggestedFilename() } }
export async function apiExport(page: Page, report = 'LEDGER', scope = 'OWN') { const input = { report, scope, home_id: '', fund_id: '', from: '2026-01-01', to: '2026-01-31' }; const preview = await page.request.post('/api/finance-exports/preview', { headers: await financialHeaders(page), data: input }); expect(preview.status(), await preview.text()).toBe(200); const p = await preview.json(); return messagePost(page, '/api/finance-exports', { ...input, preview_hash: p.content_hash }) }
