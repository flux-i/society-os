import { test, expect } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { login, chooseOption } from './helpers'
import { ensureMaintenanceReviewer } from './maintenance-fixtures'
import { apiIssuedFine, apiFineReport, apiFineVerify, fineGet, finePost } from './fines-fixtures'
import { names, execute } from './native-webmcp-helpers'
import type { ContextDocument } from './native-webmcp-helpers'

test('native fine metadata matches the visible exact position and original receipt while preserving an ordinary human decision form', async ({ page, browser }) => {
  test.setTimeout(60000)
  await login(page); await ensureMaintenanceReviewer(page)
  const reviewContext = await browser.newContext({ baseURL: new URL(page.url()).origin })
  const ownerContext = await browser.newContext({ baseURL: new URL(page.url()).origin })
  const reviewer = await reviewContext.newPage(), owner = await ownerContext.newPage()
  await login(reviewer, 'Committee'); await login(owner, 'Owner')
  const id = await apiIssuedFine(page, reviewer, owner, 'NATIVE FINE Considered amount')
  const report = await apiFineReport(owner, id), paid = await apiFineVerify(reviewer, report)
  const fine = await fineGet(page, id)
  const appeal = (await finePost(owner, '/api/fine-appeals', { notice_id: fine.notice_id, body: 'PRIVATE native household account is omitted from metadata.' })).id
  const waiver = (await finePost(page, '/api/fine-waivers', { fine_id: id, charge_version: fine.charge_version, kind: 'WAIVER', amount: '75.25', policy_reference: 'PRIVATE supplied native correction authority', reason: 'PRIVATE native supplied amount reduction needs independent review.' })).id
  await page.goto('/#fines?fine=' + id)
  await expect(page.getByRole('dialog').locator('.fine-balance strong')).toHaveText('₹150.25')
  await chooseOption(page, 'Fine decision', 'Private treasury note')
  const marker = 'PRIVATE_NATIVE_FINE_FORM Must remain unchanged by metadata reads.'
  await page.getByLabel('Decision reason', { exact: true }).fill(marker)
  const mutations: string[] = [], listener = (request: import('@playwright/test').Request) => { if (!['GET', 'HEAD'].includes(request.method())) mutations.push(request.url()) }
  page.on('request', listener)
  const reads = [
    ['society_find_fines', { query: 'NATIVE FINE Considered amount' }], ['society_read_fine', { fine_id: id }],
    ['society_find_fine_notices', { query: 'NATIVE FINE Considered amount' }], ['society_read_fine_notice', { notice_id: fine.notice_id }],
    ['society_find_fine_payments', { fine_id: id }], ['society_read_fine_payment', { report_id: report }],
    ['society_find_fine_appeals', { fine_id: id }], ['society_read_fine_appeal', { appeal_id: appeal }],
    ['society_find_fine_corrections', { fine_id: id }], ['society_read_fine_correction', { correction_id: waiver }]
  ] as const
  for (const [name, args] of reads) {
    const data = JSON.parse(await execute(page, name, args))
    if ('items' in data) { expect(data.page_size).toBe(12); expect(data.total).toBe(1); expect(data.items).toHaveLength(1) }
    for (const forbidden of ['PRIVATE', 'incident_id', 'source_key', 'source_json', 'policy_reference', 'response_total', 'reason', 'author', 'reviewer', 'body', 'evidence_id', 'current_fine_version', 'events']) expect(JSON.stringify(data)).not.toContain(forbidden)
  }
  const current = JSON.parse(await execute(page, 'society_read_fine', { fine_id: id }))
  expect(current.active_paise).toBe(25025); expect(current.allocated_paise).toBe(10000); expect(current.outstanding_paise).toBe(15025)
  expect(JSON.parse(await execute(page, 'society_read_fine_payment', { report_id: report })).receipt).toBe(paid.receipt)
  await expect(page.getByRole('textbox', { name: 'Decision reason', exact: true })).toHaveValue(marker)
  await expect(page.getByRole('dialog').getByRole('checkbox')).not.toBeChecked()
  await expect(execute(page, 'society_open_workspace', { screen: 'overview' })).rejects.toThrow()
  expect(mutations).toEqual([]); page.off('request', listener)
  await reviewContext.close(); await ownerContext.close()
})

test('native fine tools reject unsupported input unbounded pages invalid identities and genuine Chrome cancellation', async ({ page }) => {
  await login(page); await expect.poll(() => names(page)).toContain('society_read_fine')
  const invalid = [
    ['society_find_fines', { page: 0 }], ['society_find_fines', { query: 'x'.repeat(101) }], ['society_find_fines', { state: 'AUTOMATIC' }],
    ['society_find_fine_notices', { page: 10001 }], ['society_read_fine_notice', { notice_id: '../private' }],
    ['society_read_fine', { fine_id: 'fixture', action: 'ISSUE' }], ['society_find_fine_payments', { state: 'CORRECTED' }],
    ['society_read_fine_payment', { report_id: 'fixture', evidence: true }], ['society_find_fine_appeals', { page: -1 }],
    ['society_read_fine_appeal', { appeal_id: '' }], ['society_find_fine_corrections', { page: 10001 }],
    ['society_read_fine_correction', { correction_id: 'fixture', history_page: 1 }]
  ] as const
  for (const [name, args] of invalid) await expect(execute(page, name, args)).rejects.toThrow()
  const cancelled = await page.evaluate(async () => {
    const context = (document as ContextDocument).modelContext, tool = (await context.getTools()).find(item => item.name === 'society_find_fines')!
    const controller = new AbortController(); controller.abort()
    const major = Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1]), args = { page: 1 }
    try { await context.executeTool(tool, major < 155 ? JSON.stringify(args) : args, { signal: controller.signal }); return false } catch { return true }
  })
  expect(cancelled).toBe(true)
})

test('native nonfinancial fine notices and own appeals expose frozen metadata without granting a tenant ledger access', async ({ page, browser }) => {
  test.setTimeout(60000); await login(page); await ensureMaintenanceReviewer(page)
  const reviewContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), reviewer = await reviewContext.newPage()
  const tenantContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), tenant = await tenantContext.newPage()
  await login(reviewer, 'Committee'); await login(tenant, 'Tenant')
  const id = await apiIssuedFine(page, reviewer, page, 'NATIVE FINE Nonfinancial household', 'demo-flat-A-103')
  const fine = await fineGet(page, id)
  await tenant.goto('/#fines?notice=' + fine.notice_id)
  await expect(tenant.getByRole('textbox', { name: 'Your response', exact: true })).toBeVisible()
  await tenant.getByRole('textbox', { name: 'Your response', exact: true }).fill('PRIVATE tenant response in progress must stay in this form.')
  const before = JSON.parse(await execute(tenant, 'society_read_fine_notice', { notice_id: fine.notice_id }))
  expect(before.home).toBe('A-103'); expect(before.state).toBe('ISSUED')
  expect(JSON.parse(await execute(tenant, 'society_find_fine_notices', { query: 'NATIVE FINE Nonfinancial household' })).total).toBe(1)
  expect(Object.keys(before).sort()).toEqual(['id', 'title', 'home', 'state', 'version', 'due_date', 'response_by'].sort())
  for (const key of ['amount_paise', 'body', 'PRIVATE', 'source', 'responses', 'incident', 'policy', 'actor']) expect(JSON.stringify(before)).not.toContain(key)
  await expect(tenant.getByRole('textbox', { name: 'Your response', exact: true })).toHaveValue('PRIVATE tenant response in progress must stay in this form.')
  await expect.poll(() => names(tenant)).not.toContain('society_read_fine'); await expect.poll(() => names(tenant)).not.toContain('society_read_fine_payment')
  await expect(execute(tenant, 'society_read_fine', { fine_id: id })).rejects.toThrow()
  expect((await tenant.request.get('/api/fines/' + id)).status()).toBe(403)
  const appeal = (await finePost(tenant, '/api/fine-appeals', { notice_id: fine.notice_id, body: 'PRIVATE household appeal through its permitted human submission.' })).id
  const data = JSON.parse(await execute(tenant, 'society_read_fine_appeal', { appeal_id: appeal }))
  expect(data.state).toBe('PENDING'); expect(data.home).toBe('A-103')
  expect(JSON.parse(await execute(tenant, 'society_find_fine_appeals', { fine_id: id })).total).toBe(1)
  expect(JSON.stringify(data)).not.toContain('PRIVATE'); expect(JSON.stringify(data)).not.toContain('amount_paise')
  await reviewContext.close(); await tenantContext.close()
})

test('native fine reads discard a captured financial result when the home flag changes and retain only current household tools', async ({ page, browser }) => {
  test.setTimeout(60000); await login(page); await ensureMaintenanceReviewer(page)
  const reviewContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), reviewer = await reviewContext.newPage()
  const ownerContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), owner = await ownerContext.newPage()
  await login(reviewer, 'Committee'); await login(owner, 'Owner')
  const id = await apiIssuedFine(page, reviewer, owner, 'NATIVE FINE Captured scope')
  const fine = await fineGet(page, id)
  await owner.goto('/#fines?fine=' + id)
  await expect(owner.getByRole('dialog').locator('.fine-balance strong')).toHaveText('₹250.25')
  const path = process.env.SOCIETY_BROWSER_DB
  if (!path || !path.includes('society-browser-')) throw new Error('A disposable database is required for native fine scope checks.')
  const financial = (allowed: boolean) => execFileSync('python3', ['-c', "import sqlite3,sys;db=sqlite3.connect(sys.argv[1]);r=db.execute(\"UPDATE flat_memberships SET can_view_finances=? WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'\",(int(sys.argv[2]),));assert r.rowcount==1;db.commit()", path, allowed ? '1' : '0'])
  let release!: () => void, captured!: () => void
  const hold = new Promise<void>(resolve => { release = resolve }), ready = new Promise<void>(resolve => { captured = resolve })
  await owner.route('**/api/fines/' + id, async route => { const response = await route.fetch(); captured(); await hold; await route.fulfill({ response }).catch(() => {}) })
  const reading = execute(owner, 'society_read_fine', { fine_id: id }); await ready
  try {
    financial(false); release(); await expect(reading).rejects.toThrow(); await owner.unroute('**/api/fines/' + id)
    await expect(owner.getByRole('dialog')).toHaveCount(0)
    expect((await (await owner.request.get('/api/auth/me')).json()).can_read_records).toBe(true)
    await expect.poll(() => names(owner)).toContain('society_read_fine_notice')
    await expect(execute(owner, 'society_read_fine', { fine_id: id })).rejects.toThrow()
    expect(JSON.parse(await execute(owner, 'society_read_fine_notice', { notice_id: fine.notice_id })).home).toBe('A-101')
  } finally { release(); financial(true); await owner.unroute('**/api/fines/' + id); await reviewContext.close(); await ownerContext.close() }
})
