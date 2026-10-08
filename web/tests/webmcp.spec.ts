import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { login, navigate, completePreviewMFA, chooseOption } from './helpers'
import { plainPDF } from './document-fixtures'
import { execFileSync } from 'node:child_process'
import { apiMaintenance, apiReceived, ensureMaintenanceReviewer } from './maintenance-fixtures'
import { apiCareRecord, apiWork, apiWorkAction, careDate } from './upkeep-fixtures'
import { apiFund, apiFundAction, apiFundReport, apiFundVerify } from './collections-fixtures'
import { financialHeaders } from './maintenance-fixtures'
import { apiRule,apiRuleAction,apiIncident,apiIncidentAction,apiPicture } from './incidents-fixtures'

import { names, execute } from './native-webmcp-helpers'
import type { ContextDocument } from './native-webmcp-helpers'

async function nativeManageAccount(page: Page, name: string) {
  await navigate(page, 'Access & invitations')
  await page.getByRole('button', { name: 'Manage access for ' + name, exact: true }).click()
  await expect(page.getByRole('dialog').getByRole('heading', { name, exact: true })).toBeVisible()
  await expect(page.getByRole('dialog').getByText('Opening this account…', { exact: true })).toHaveCount(0)
}
async function nativeAccessDecision(page: Page, button: string) {
  await page.getByRole('textbox', { name: 'Reason', exact: true }).fill('Verified fictional appointment against the approved society register for native checks.')
  await page.getByRole('dialog').getByRole('checkbox').check()
  await page.getByRole('button', { name: button, exact: true }).click()
  await expect(page.getByRole('dialog').locator('.form-success')).toBeVisible()
  await expect(page.getByRole('dialog').getByText('Updating this account…', { exact: true })).toHaveCount(0)
}

test('native account metadata matches a visible appointment and preserves its human form without security or activity content', async ({ page }) => {
  await login(page); await expect.poll(() => names(page)).toContain('society_read_account')
  const before = JSON.parse(await execute(page, 'society_read_account', { account_id: 'demo-user-owner' })); expect(before.version).toBe(1); expect(before.account.roles).toEqual([])
  await nativeManageAccount(page, 'Demo Owner A-101'); await page.getByRole('button', { name: 'Add appointment', exact: true }).click(); await chooseOption(page, 'Appointment', 'Accountant / auditor'); await page.getByRole('textbox', { name: 'Reason', exact: true }).fill('A private fictional verification note that must stay out of native metadata.')
  const mutations: string[] = []; const listener = (request: import('@playwright/test').Request) => { if (!['GET', 'HEAD'].includes(request.method())) mutations.push(request.url()) }; page.on('request', listener)
  const results = JSON.parse(await execute(page, 'society_find_accounts', { query: 'owner@demo.society' })); expect(results.total).toBe(1); expect(results.items[0].name).toBe('Demo Owner A-101'); expect(Object.keys(results.items[0]).sort()).toEqual(['active_homes', 'id', 'mfa_enrolled', 'name', 'roles', 'state'])
  await execute(page, 'society_read_account', { account_id: 'demo-user-owner' }); await expect(page.getByRole('textbox', { name: 'Reason', exact: true })).toHaveValue('A private fictional verification note that must stay out of native metadata.'); await expect(execute(page, 'society_open_workspace', { screen: 'overview' })).rejects.toThrow(); await expect(page.getByRole('dialog')).toBeVisible(); expect(mutations).toEqual([]); page.off('request', listener)
  await nativeAccessDecision(page, 'Save appointment'); const after = JSON.parse(await execute(page, 'society_read_account', { account_id: 'demo-user-owner' })); expect(after.version).toBe(2); expect(after.account.roles).toEqual(['AUDITOR']); expect(after.grants).toHaveLength(1); expect(after.grants[0].state).toBe('ACTIVE'); expect(after.grants[0].valid_until - after.grants[0].valid_from).toBe(90 * 86400)
  for (const forbidden of ['email', 'events', 'reason', 'csrf_token', 'password_hash', 'secret_ciphertext', 'verification note', 'download_url']) expect(JSON.stringify(after)).not.toContain(forbidden)
  await page.getByRole('button', { name: 'End Accountant / auditor appointment', exact: true }).click(); await nativeAccessDecision(page, 'End appointment'); const ended = JSON.parse(await execute(page, 'society_read_account', { account_id: 'demo-user-owner' })); expect(ended.version).toBe(3); expect(ended.account.roles).toEqual([]); expect(ended.grants[0].state).toBe('REVOKED'); await page.getByRole('button', { name: 'Close account access', exact: true }).click()
})

test('native account discovery denies residents and rejects unsupported unbounded and cancelled metadata requests', async ({ page }) => {
  await login(page); await expect.poll(() => names(page)).toContain('society_find_accounts')
  for (const [tool, input] of [['society_find_accounts', { page: 0 }], ['society_find_accounts', { query: 'x'.repeat(101) }], ['society_read_account', { account_id: 'demo-user-owner', grant_page: 10001 }], ['society_read_account', { account_id: 'demo-user-owner', history_page: 1 }], ['society_read_account', { account_id: 'demo-user-owner', action: 'SUSPEND' }]] as const) await expect(execute(page, tool, input)).rejects.toThrow()
  const cancelled = await page.evaluate(async () => {
    const context = (document as ContextDocument).modelContext; const tool = (await context.getTools()).find(item => item.name === 'society_read_account')!; const controller = new AbortController(); controller.abort(); const args = { account_id: 'demo-user-owner' }; const major = Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1])
    try { await context.executeTool(tool, major < 155 ? JSON.stringify(args) : args, { signal: controller.signal }); return false } catch { return true }
  }); expect(cancelled).toBe(true)
  await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await login(page, 'Tenant'); await expect.poll(() => names(page)).not.toContain('society_read_account'); await expect.poll(() => names(page)).not.toContain('society_find_accounts'); await expect(execute(page, 'society_open_workspace', { screen: 'access' })).rejects.toThrow(); expect((await page.request.get('/api/admin/accounts/demo-user-admin')).status()).toBe(403)
  await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await login(page); await expect.poll(() => names(page)).toContain('society_read_account')
  const path = process.env.SOCIETY_BROWSER_DB; if (!path || !path.includes('society-browser-')) throw new Error('An isolated synthetic database is required for the factor fixture.')
  execFileSync('python3', ['-c', "import sqlite3,sys;db=sqlite3.connect(sys.argv[1]);db.execute(\"DELETE FROM mfa_factors WHERE user_id='demo-user-admin'\");db.commit()", path])
  await expect(execute(page, 'society_read_account', { account_id: 'demo-user-owner' })).rejects.toThrow(); await expect.poll(() => names(page)).toEqual([]); await expect(page.getByRole('button', { name: 'Use a preview code', exact: true })).toBeVisible(); await completePreviewMFA(page); await expect.poll(() => names(page)).toContain('society_read_account')
})

test('an actual successor ends an administrator appointment and invalidates the prior native tools and session', async ({ page, browser }) => {
  await login(page); await nativeManageAccount(page, 'Demo Committee Member'); await page.getByRole('button', { name: 'Add appointment', exact: true }).click(); await chooseOption(page, 'Appointment', 'Administrator'); await nativeAccessDecision(page, 'Save appointment'); await page.getByRole('button', { name: 'Close account access', exact: true }).click()
  const context = await browser.newContext({ baseURL: new URL(page.url()).origin }); const successor = await context.newPage(); await login(successor, 'Committee'); await nativeManageAccount(successor, 'Demo Registry Officer'); await successor.getByRole('button', { name: 'End Administrator appointment', exact: true }).click(); await nativeAccessDecision(successor, 'End appointment')
  await expect(execute(page, 'society_read_account', { account_id: 'demo-user-owner' })).rejects.toThrow(); await expect.poll(() => names(page)).toEqual([]); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible(); await expect(page.getByRole('dialog')).toHaveCount(0)
  await successor.getByRole('button', { name: 'Add appointment', exact: true }).click(); await chooseOption(successor, 'Appointment', 'Administrator'); await nativeAccessDecision(successor, 'Save appointment'); await successor.getByRole('button', { name: 'Close account access', exact: true }).click()
  await login(page); await expect.poll(() => names(page)).toContain('society_read_account'); await nativeManageAccount(page, 'Demo Committee Member'); await page.getByRole('button', { name: 'End Administrator appointment', exact: true }).click(); await nativeAccessDecision(page, 'End appointment'); await page.getByRole('button', { name: 'Close account access', exact: true }).click(); await context.close()
})

test('native tools rediscover current personal scope after an appointment expires without signing the resident out', async ({ page, browser }) => {
  await login(page); await nativeManageAccount(page, 'Demo Owner A-101'); await page.getByRole('button', { name: 'Add appointment', exact: true }).click(); await chooseOption(page, 'Appointment', 'Accountant / auditor'); await nativeAccessDecision(page, 'Save appointment'); await page.getByRole('button', { name: 'Close account access', exact: true }).click()
  const me = await (await page.request.get('/api/auth/me')).json(); const headers = { Origin: new URL(page.url()).origin, 'X-CSRF-Token': me.csrf_token }
  const date = new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit', timeZone: 'Asia/Kolkata' }).format(new Date())
  const draft = await page.request.post('/api/entries', { headers, data: { operation_key: crypto.randomUUID(), flat_id: 'demo-flat-A-103', kind: 'CHARGE', amount: '37.25', date, description: 'NATIVE Supplied charge outside the auditor resident homes', payer: '', method: '', reference: '' } }); expect(draft.status()).toBe(200); const entryId = (await draft.json()).id
  expect((await page.request.post('/api/entries/' + entryId + '/post', { headers, data: { operation_key: crypto.randomUUID(), confirmed: true, reason: '' } })).status()).toBe(200)
  const context = await browser.newContext({ baseURL: new URL(page.url()).origin }); const resident = await context.newPage(); await login(resident, 'Owner'); await expect.poll(() => names(resident)).toContain('society_find_records')
  const before = await (await resident.request.get('/api/auth/me')).json(); expect(before.can_read_all_records).toBe(true)
  await expect(resident.locator('.overview-finance-balance strong')).toHaveText('₹37.25')
  const wide = JSON.parse(await execute(resident, 'society_find_records', { home_id: 'demo-flat-A-103' })); expect(wide.total).toBe(1); expect(wide.debit_paise).toBe(3725)
  const path = process.env.SOCIETY_BROWSER_DB; if (!path || !path.includes('society-browser-')) throw new Error('An isolated synthetic database is required for the expiry fixture.')
  execFileSync('python3', ['-c', "import sqlite3,sys,time;db=sqlite3.connect(sys.argv[1]);now=int(time.time());db.execute(\"UPDATE role_grants SET valid_from=?,valid_until=? WHERE user_id='demo-user-owner' AND role='AUDITOR' AND revoked_at IS NULL\",(now-172800,now-1));db.commit()", path])
  await expect(execute(resident, 'society_find_homes', {})).rejects.toThrow()
  await expect.poll(async () => {
    try { const homes = JSON.parse(await execute(resident, 'society_find_homes', {})); return homes.items.map((home: { id: string }) => home.id).sort() } catch { return [] }
  }).toEqual(['demo-flat-A-101', 'demo-flat-A-102'])
  const current = await (await resident.request.get('/api/auth/me')).json(); expect(current.id).toBe(before.id); expect(current.roles).toEqual([]); expect(current.can_read_all_records).toBe(false); expect(current.can_read_records).toBe(true)
  await expect(resident.locator('.overview-finance-balance strong')).toHaveText('₹0.00')
  await expect.poll(() => names(resident)).toEqual([
    'society_find_meetings', 'society_read_meeting', 'society_find_community_updates', 'society_read_community_update', 'society_find_financial_statements', 'society_read_financial_statement', 'society_find_messages', 'society_read_message', 'society_find_contacts', 'society_read_contact',
    'society_find_complaints', 'society_read_complaint', 'society_find_documents', 'society_read_document',
    'society_find_homes', 'society_open_home', 'society_open_workspace', 'society_read_overview',
    'society_find_notices', 'society_find_requests', 'society_find_upkeep', 'society_read_upkeep',
    'society_find_rules', 'society_read_rule', 'society_find_incidents', 'society_read_incident',
    'society_find_incident_notices', 'society_read_incident_notice',
    'society_find_fine_notices', 'society_read_fine_notice', 'society_find_fine_appeals', 'society_read_fine_appeal',
    'society_find_finance_exports', 'society_read_finance_export', 'society_find_records', 'society_read_home_statement', 'society_find_maintenance', 'society_read_maintenance',
    'society_find_collections', 'society_read_collection', 'society_find_payment_reports', 'society_read_payment_report',
    'society_find_fund_contributions', 'society_find_fines', 'society_read_fine',
    'society_find_fine_payments', 'society_read_fine_payment',
  ].sort())
  const hidden = JSON.parse(await execute(resident, 'society_find_records', { home_id: 'demo-flat-A-103' })); expect(hidden.items).toEqual([]); expect(hidden.total).toBe(0); expect(hidden.debit_paise).toBe(0); expect(hidden.homes.map((home: { id: string }) => home.id).sort()).toEqual(['demo-flat-A-101', 'demo-flat-A-102']); expect((await resident.request.get('/api/entries/' + entryId)).status()).toBe(404)
  await execute(resident, 'society_find_records', { home_id: 'demo-flat-A-101' }); await expect(resident.getByRole('button', { name: 'Sign out', exact: true })).toBeVisible(); await context.close()
})

test('native reads reject a response captured before a home entitlement ends and clear its old visible details', async ({ page }) => {
  await login(page, 'Owner'); await navigate(page, 'Your homes'); await expect(page.locator('.home-card')).toHaveCount(2); await execute(page, 'society_open_home', { home_id: 'demo-flat-A-102' }); await expect(page.getByRole('dialog').getByRole('heading', { name: 'Home 102', exact: true })).toBeVisible()
  const path = process.env.SOCIETY_BROWSER_DB; if (!path || !path.includes('society-browser-')) throw new Error('An isolated synthetic database is required for the membership fixture.')
  const date = new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit', timeZone: 'Asia/Kolkata' }).format(new Date())
  const membership = (restore: boolean) => execFileSync('python3', ['-c', "import sqlite3,sys;db=sqlite3.connect(sys.argv[1]);result=db.execute(\"UPDATE flat_memberships SET end_date=? WHERE flat_id='demo-flat-A-102' AND resident_id=(SELECT resident_id FROM users WHERE id='demo-user-owner') AND end_date IS \"+('NULL' if sys.argv[3]=='end' else '?'),([sys.argv[2]] if sys.argv[3]=='end' else [None,sys.argv[2]]));assert result.rowcount==1;db.commit()", path, date, restore ? 'restore' : 'end'])
  let ready: () => void = () => {}; const captured = new Promise<void>(resolve => { ready = resolve }); let release: () => void = () => {}; const pending = new Promise<void>(resolve => { release = resolve })
  await page.route('**/api/flats?**', async route => { const response = await route.fetch(); ready(); await pending; await route.fulfill({ response }).catch(() => {}) })
  const reading = execute(page, 'society_find_homes', {}); await captured
  membership(false)
  try {
    release(); await expect(reading).rejects.toThrow(); await page.unroute('**/api/flats?**')
    await expect(page.getByRole('dialog')).toHaveCount(0); await expect(page.locator('.home-card')).toHaveCount(1)
    await expect.poll(async () => { try { const homes = JSON.parse(await execute(page, 'society_find_homes', {})); return homes.items.map((home: { id: string }) => home.id) } catch { return [] } }).toEqual(['demo-flat-A-101'])
    await expect(execute(page, 'society_open_home', { home_id: 'demo-flat-A-102' })).rejects.toThrow(); await execute(page, 'society_open_home', { home_id: 'demo-flat-A-101' }); await expect(page.getByRole('dialog').getByRole('heading', { name: 'Home 101', exact: true })).toBeVisible()
  } finally { membership(true); release(); await page.unroute('**/api/flats?**') }
})

test('native WebMCP reads the same personal overview after an actual UI report and preserves its dialog', async ({ page }) => {
  await login(page, 'Owner'); await expect.poll(() => names(page)).toContain('society_read_overview')
  const before = JSON.parse(await execute(page, 'society_read_overview', { section: 'service' }))
  await navigate(page, 'Help & repairs'); await page.getByRole('button', { name: 'Report an issue', exact: true }).click(); await chooseOption(page, 'Service request home', 'Home A-101'); await chooseOption(page, 'Service category', 'Plumbing'); await page.getByRole('textbox', { name: 'Subject', exact: true }).fill('NATIVE Overview kitchen leak'); await page.getByRole('textbox', { name: 'Details', exact: true }).fill('A fictional leak for the actual native overview contract and current personal scope.'); await page.getByRole('button', { name: 'Save service request', exact: true }).click(); await expect(page.getByRole('dialog').locator('.case-status')).toHaveText('Open')
  const mutations: string[] = []; page.on('request', request => { if (!['GET', 'HEAD'].includes(request.method())) mutations.push(request.url()) })
  const result = JSON.parse(await execute(page, 'society_read_overview', { section: 'service' })); expect(result.counts.active).toBe(before.counts.active + 1); expect(result.items.length).toBeLessThanOrEqual(4); expect(result.calendar).toBe('Asia/Kolkata'); expect(result.items.every((item: Record<string, unknown>) => !('description' in item) && !('updates' in item))).toBe(true)
  await expect(execute(page, 'society_open_workspace', { screen: 'overview' })).rejects.toThrow(); await expect(page.getByRole('dialog')).toBeVisible(); await page.getByRole('button', { name: 'Close service request details', exact: true }).click(); await execute(page, 'society_open_workspace', { screen: 'overview' }); await expect(page.getByRole('heading', { name: 'Good things, in order.' })).toBeVisible(); await expect(page.getByRole('region', { name: 'Your active service requests', exact: true }).locator('strong')).toHaveText(String(result.counts.active)); expect(mutations).toEqual([])
})

test('native WebMCP denies unentitled overview finance unsupported sections and revoked sessions', async ({ page }) => {
  await login(page, 'Tenant'); await expect.poll(() => names(page)).toContain('society_read_overview')
  await expect(execute(page, 'society_read_overview', { section: 'finance' })).rejects.toThrow(); await expect(execute(page, 'society_read_overview', { section: '../entries' })).rejects.toThrow(); await expect(execute(page, 'society_read_overview', { section: 'notices', page: 2 })).rejects.toThrow()
  const docs = JSON.parse(await execute(page, 'society_read_overview', { section: 'documents' })); expect(docs.counts.ready_review).toBe(0); expect(docs.counts.upload_attention).toBe(0); expect(docs.items.every((item: Record<string, unknown>) => ['EXPIRED', 'EXPIRING'].includes(String(item.state)) && !('events' in item) && !('download_url' in item) && !('filename' in item))).toBe(true)
  const me = await (await page.request.get('/api/auth/me')).json(); expect((await page.request.post('/api/auth/logout', { headers: { Origin: new URL(page.url()).origin, 'X-CSRF-Token': me.csrf_token }, data: {} })).status()).toBe(200); await expect(execute(page, 'society_read_overview', { section: 'service' })).rejects.toThrow(); await expect.poll(() => names(page)).toEqual([])
})

test('native WebMCP discovers permitted tools after MFA and executes bounded searches', async ({ page }) => {
  await page.goto('/')
  expect(await page.evaluate(() => !!(document as ContextDocument).modelContext)).toBe(true)
  expect(await names(page)).toEqual([])
  await login(page)
  await expect.poll(() => names(page)).toEqual(['society_find_meetings', 'society_read_meeting', 'society_find_community_updates', 'society_read_community_update', 'society_find_financial_statements', 'society_read_financial_statement', 'society_find_delivery_exceptions', 'society_find_messages', 'society_read_message', 'society_find_contacts', 'society_read_contact', 'society_find_accounts', 'society_find_complaints', 'society_find_documents', 'society_find_homes', 'society_find_maintenance', 'society_find_notices', 'society_find_finance_exports', 'society_read_finance_export', 'society_find_records', 'society_find_requests', 'society_find_upkeep', 'society_find_upkeep_register', 'society_open_home', 'society_open_workspace', 'society_read_account', 'society_read_complaint', 'society_read_document', 'society_read_home_statement', 'society_read_maintenance', 'society_read_overview', 'society_read_upkeep', 'society_read_upkeep_register', 'society_find_collections', 'society_read_collection', 'society_find_payment_reports', 'society_read_payment_report', 'society_find_fund_contributions', 'society_find_fund_exemptions', 'society_read_fund_exemption', 'society_find_rules', 'society_read_rule', 'society_find_incidents', 'society_read_incident', 'society_find_incident_notices', 'society_read_incident_notice', 'society_find_fine_notices', 'society_read_fine_notice', 'society_find_fine_appeals', 'society_read_fine_appeal', 'society_find_fines', 'society_read_fine', 'society_find_fine_payments', 'society_read_fine_payment', 'society_find_fine_corrections', 'society_read_fine_correction'].sort())
  const mutations: string[] = []
  page.on('request', request => { if (!['GET', 'HEAD'].includes(request.method())) mutations.push(request.url()) })
  const homes = JSON.parse(await execute(page, 'society_find_homes', { wing: 'B', occupancy: 'RENTED' }))
  expect(homes.total).toBe(12)
  expect(homes.items).toHaveLength(12)
  expect(homes.items.every((home: { building_code: string; status: string }) => home.building_code === 'B' && home.status === 'RENTED')).toBe(true)
  const records = JSON.parse(await execute(page, 'society_find_records', { home_id: 'demo-flat-C-1002', receipts_only: true }))
  expect(records.total).toBe(0)
  await execute(page, 'society_open_workspace', { screen: 'homes' })
  await expect(page.getByRole('heading', { name: 'Homes & people', exact: true })).toBeVisible()
  await execute(page, 'society_open_home', { home_id: 'demo-flat-B-301' })
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Home 301', exact: true })).toBeVisible()
  expect(mutations).toEqual([])
})

test('native WebMCP preserves review dialogs, rejects invalid input and unregisters on logout', async ({ page }) => {
  await login(page)
  await expect.poll(() => names(page)).toContain('society_open_home')
  await expect(execute(page, 'society_find_homes', { page: -1 })).rejects.toThrow()
  await expect(execute(page, 'society_open_workspace', { screen: 'https://outside.invalid' })).rejects.toThrow()
  await execute(page, 'society_open_home', { home_id: 'demo-flat-A-101' })
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(execute(page, 'society_open_workspace', { screen: 'entries' })).rejects.toThrow()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.getByRole('button', { name: 'Close details', exact: true }).click()
  await page.getByRole('button', { name: 'Sign out', exact: true }).click()
  await expect.poll(() => names(page)).toEqual([])
})

test('native WebMCP follows resident scope and omits financial tools for an unentitled tenant', async ({ page }) => {
  await login(page, 'Owner')
  await expect.poll(() => names(page)).toContain('society_find_records')
  const homes = JSON.parse(await execute(page, 'society_find_homes', {}))
  expect(homes.items.map((home: { id: string }) => home.id).sort()).toEqual(['demo-flat-A-101', 'demo-flat-A-102'])
  await expect(execute(page, 'society_open_home', { home_id: 'demo-flat-A-103' })).rejects.toThrow()
  expect(await page.getByRole('dialog').count()).toBe(0)
  await page.getByRole('button', { name: 'Sign out', exact: true }).click()
  await login(page, 'Tenant')
  await expect.poll(() => names(page)).toEqual(['society_find_meetings', 'society_read_meeting', 'society_find_community_updates', 'society_read_community_update', 'society_find_financial_statements', 'society_read_financial_statement', 'society_find_messages', 'society_read_message', 'society_find_contacts', 'society_read_contact', 'society_find_complaints', 'society_find_documents', 'society_find_homes', 'society_find_notices', 'society_find_requests', 'society_find_upkeep', 'society_open_home', 'society_open_workspace', 'society_read_complaint', 'society_read_document', 'society_read_overview', 'society_read_upkeep', 'society_find_rules', 'society_read_rule', 'society_find_incidents', 'society_read_incident', 'society_find_incident_notices', 'society_read_incident_notice', 'society_find_fine_notices', 'society_read_fine_notice', 'society_find_fine_appeals', 'society_read_fine_appeal'].sort())
  await expect(execute(page, 'society_open_workspace', { screen: 'entries' })).rejects.toThrow()
  const tenantHomes = JSON.parse(await execute(page, 'society_find_homes', {}))
  expect(tenantHomes.items.map((home: { id: string }) => home.id)).toEqual(['demo-flat-A-103'])
})

test('native WebMCP stops returning personal data after a session is revoked', async ({ page }) => {
  await login(page, 'Owner')
  await expect.poll(() => names(page)).toContain('society_find_homes')
  const me = await (await page.request.get('/api/auth/me')).json()
  const base = new URL(page.url()).origin
  expect((await page.request.post('/api/auth/logout', { headers: { Origin: base, 'X-CSRF-Token': me.csrf_token }, data: {} })).ok()).toBe(true)
  await expect(execute(page, 'society_find_homes', {})).rejects.toThrow()
  await expect.poll(() => names(page)).toEqual([])
})

test('native WebMCP supports cancellation and does not register while verification is pending', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: /^Registry officer/ }).click()
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Use a preview code', exact: true })).toBeVisible()
  expect(await names(page)).toEqual([])
  await completePreviewMFA(page)
  await expect.poll(() => names(page)).toContain('society_find_homes')
  await page.route('**/api/flats?**', route => route.abort())
  await navigate(page, 'Account security')
  const cancelled = await page.evaluate(async () => {
    const context = (document as ContextDocument).modelContext
    const tool = (await context.getTools()).find(item => item.name === 'society_find_homes')!
    const controller = new AbortController()
    controller.abort()
    try { await context.executeTool(tool, '{}', { signal: controller.signal }); return false } catch { return true }
  })
  expect(cancelled).toBe(true)
  await expect(page.getByRole('button', { name: 'Sign out', exact: true })).toBeVisible()
})

test('native WebMCP reads real submission and publication states while approval uses the visible review form', async ({ page }) => {
  await login(page, 'Owner')
  await navigate(page, 'Your requests')
  await page.getByRole('button', { name: 'Submit a request', exact: true }).click()
  await chooseOption(page, 'Request type', 'Notice proposal')
  await page.getByRole('textbox', { name: 'Title', exact: true }).fill('NATIVE Notice approval journey')
  await page.getByRole('textbox', { name: 'Description', exact: true }).fill('A fictional notice submitted by one person and approved by another.')
  await chooseOption(page, 'Notice audience', 'All current residents')
  await page.getByRole('button', { name: 'Send for review', exact: true }).click()
  await expect(page.getByRole('dialog').getByText('Awaiting review', { exact: true })).toBeVisible()
  const pending = JSON.parse(await execute(page, 'society_find_requests', { query: 'NATIVE Notice approval journey' }))
  expect(pending.items).toHaveLength(1); expect(pending.items[0].state).toBe('PENDING')
  expect(JSON.parse(await execute(page, 'society_find_notices', { query: 'NATIVE Notice approval journey' })).total).toBe(0)
  await page.getByRole('button', { name: 'Close request details', exact: true }).click()
  await page.getByRole('button', { name: 'Sign out', exact: true }).click()
  await login(page)
  await expect.poll(() => names(page)).toContain('society_find_requests')
  expect((await names(page)).some(name => /approve|publish|post|delete/.test(name))).toBe(false)
  await execute(page, 'society_open_workspace', { screen: 'reviews' })
  await page.getByRole('searchbox', { name: 'Search requests' }).fill('NATIVE Notice approval journey')
  await page.getByRole('button', { name: 'Open NATIVE Notice approval journey' }).click()
  await page.getByRole('textbox', { name: 'Reason for this decision' }).fill('Reviewed the fictional content and selected resident audience')
  await page.getByRole('checkbox').check()
  await page.getByRole('button', { name: 'Approve & publish', exact: true }).click()
  await expect(page.getByRole('dialog').locator('.review-history li')).toHaveCount(2)
  const published = JSON.parse(await execute(page, 'society_find_notices', { query: 'NATIVE Notice approval journey' }))
  expect(published.items).toHaveLength(1); expect(published.items[0].id).toBe(pending.items[0].id)
  expect(published.items[0].state).toBe('APPROVED'); expect(published.items[0].events).toBeUndefined()
})

test('native WebMCP scopes complaint search and detail and excludes private staff messages for the reporter', async ({ page }) => {
  await login(page, 'Owner'); await navigate(page, 'Help & repairs')
  await page.getByRole('button', { name: 'Report an issue', exact: true }).click()
  await chooseOption(page, 'Service request home', 'Home A-101'); await chooseOption(page, 'Service category', 'Water')
  await page.getByRole('textbox', { name: 'Subject', exact: true }).fill('NATIVE A fictional water inspection')
  await page.getByRole('textbox', { name: 'Details', exact: true }).fill('The fictional water pressure needs an inspection from the handling team.')
  const response = page.waitForResponse(response => response.url().endsWith('/api/complaints') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Save service request', exact: true }).click(); const result = await response; expect(result.status()).toBe(200); const id = (await result.json()).id
  await page.getByRole('button', { name: 'Close service request details' }).click(); await page.getByRole('button', { name: 'Sign out', exact: true }).click()
  await login(page); await page.goto('/#help?case=' + id)
  await chooseOption(page, 'Update visibility', 'Handling team only')
  await page.getByRole('textbox', { name: 'Service update message' }).fill('NATIVE_STAFF_SECRET_A private inspection coordination note')
  await page.getByRole('button', { name: 'Save update', exact: true }).click(); await expect(page.getByText('NATIVE_STAFF_SECRET_A private inspection coordination note')).toBeVisible()
  await expect.poll(() => names(page)).toContain('society_read_complaint')
  const staff = JSON.parse(await execute(page, 'society_read_complaint', { case_id: id })); expect(staff.history_total).toBe(2); expect(JSON.stringify(staff)).toContain('NATIVE_STAFF_SECRET')
  await page.getByRole('button', { name: 'Close service request details' }).click(); await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await login(page, 'Owner')
  await expect.poll(() => names(page)).toContain('society_read_complaint')
  const reporter = JSON.parse(await execute(page, 'society_read_complaint', { case_id: id, history_page: 99 })); expect(reporter.history_total).toBe(1); expect(reporter.version).toBe(1); expect(reporter.history_page).toBe(1); expect(JSON.stringify(reporter)).not.toContain('NATIVE_STAFF_SECRET')
  const found = JSON.parse(await execute(page, 'society_find_complaints', { query: 'NATIVE A fictional water inspection', status: 'OPEN' })); expect(found.items.map((item: { id: string }) => item.id)).toEqual([id])
  const hiddenSearch = JSON.parse(await execute(page, 'society_find_complaints', { query: 'NATIVE_STAFF_SECRET' })); expect(hiddenSearch.total).toBe(0)
  await execute(page, 'society_open_workspace', { screen: 'help' }); await expect(page.getByRole('heading', { name: 'Your service requests' })).toBeVisible()
  await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await login(page, 'Tenant'); await expect.poll(() => names(page)).toContain('society_read_complaint')
  await expect(execute(page, 'society_read_complaint', { case_id: id })).rejects.toThrow(); expect(JSON.parse(await execute(page, 'society_find_complaints', { query: 'NATIVE A fictional water inspection' })).total).toBe(0)
  await expect(execute(page, 'society_find_complaints', { status: 'APPROVED' })).rejects.toThrow()
  expect((await names(page)).some(name => /^society_(?:approve|close|update|assign)(?:_|$)/.test(name))).toBe(false)
})

test('native complaint reads reject cancellation and cease returning data after session revocation', async ({ page }) => {
  await login(page, 'Owner'); await expect.poll(() => names(page)).toContain('society_read_complaint')
  const cancelled = await page.evaluate(async () => {
    const context = (document as ContextDocument).modelContext; const tool = (await context.getTools()).find(item => item.name === 'society_find_complaints')!
    const controller = new AbortController(); controller.abort()
    const major = Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1])
    try { await context.executeTool(tool, major < 155 ? '{}' : {}, { signal: controller.signal }); return false } catch { return true }
  })
  expect(cancelled).toBe(true)
  const me = await (await page.request.get('/api/auth/me')).json(); expect((await page.request.post('/api/auth/logout', { headers: { Origin: new URL(page.url()).origin, 'X-CSRF-Token': me.csrf_token }, data: {} })).status()).toBe(200)
  await expect(execute(page, 'society_find_complaints', {})).rejects.toThrow(); await expect.poll(() => names(page)).toEqual([])
})

test('native document tools scope a real original before and after visible separate approval without exposing bytes or private review history', async ({ page }) => {
 await login(page, 'Owner'); await expect.poll(() => names(page)).toContain('society_find_documents'); await execute(page, 'society_open_workspace', { screen: 'documents' }); await page.getByRole('button', { name: 'Add a document', exact: true }).click(); await page.getByLabel('Document file', { exact: true }).setInputFiles({ name: 'native-circular.pdf', mimeType: 'application/pdf', buffer: plainPDF() }); await page.getByRole('textbox', { name: 'Title', exact: true }).fill('NATIVE Document circulation'); await chooseOption(page, 'Document category', 'Circular'); await chooseOption(page, 'Document audience', 'Current society residents')
 await expect(execute(page, 'society_open_workspace', { screen: 'overview' })).rejects.toThrow(); await expect(page.getByRole('dialog')).toBeVisible(); const saved = page.waitForResponse(response => /\/api\/documents\/[^/]+\/content$/.test(response.url()) && response.request().method() === 'POST'); await page.getByRole('button', { name: 'Upload for review', exact: true }).click(); const response = await saved; expect(response.status()).toBe(200); const id = (await response.json()).id; await expect(page.getByRole('button', { name: 'Download original', exact: true })).toBeEnabled()
 const own = JSON.parse(await execute(page, 'society_read_document', { document_id: id, history_page: 99 })); expect(own.review_state).toBe('PENDING'); expect(own.history_page).toBe(1); expect(own.history_total).toBe(1); expect(own.events.length).toBeGreaterThan(0)
 await page.getByRole('button', { name: 'Close document details' }).click(); await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await login(page, 'Tenant'); await expect.poll(() => names(page)).toContain('society_read_document'); expect(JSON.parse(await execute(page, 'society_find_documents', { query: 'NATIVE Document circulation' })).total).toBe(0); await expect(execute(page, 'society_read_document', { document_id: id })).rejects.toThrow()
 await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await login(page); await page.goto('/#documents?file=' + id); await chooseOption(page, 'Document decision', 'Approve this version'); await page.getByRole('textbox', { name: 'Document decision reason' }).fill('NATIVE_REVIEW_SECRET Only staff and the uploader can see this review reason'); await page.getByRole('checkbox', { name: 'I have checked this version, its audience and my decision.' }).check(); await page.getByRole('button', { name: 'Save document decision', exact: true }).click(); await expect(page.getByRole('dialog').locator('.review-state')).toHaveText('Approved'); await page.getByRole('button', { name: 'Close document details' }).click(); await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await login(page, 'Tenant'); await expect.poll(() => names(page)).toContain('society_read_document')
 const approved = JSON.parse(await execute(page, 'society_read_document', { document_id: id })); expect(approved.review_state).toBe('APPROVED'); expect(approved.revision).toBe(1); expect(approved.events).toBeUndefined(); expect(JSON.stringify(approved)).not.toContain('NATIVE_REVIEW_SECRET'); expect(approved.original_bytes).toBeUndefined(); expect(approved.download_url).toBeUndefined(); const found = JSON.parse(await execute(page, 'society_find_documents', { query: 'NATIVE Document circulation' })); expect(found.items.map((item: { id: string }) => item.id)).toEqual([id]); await expect(execute(page, 'society_find_documents', { category: 'HTML', page: -1 })).rejects.toThrow(); expect((await names(page)).some(name => /approve|upload|archive|delete/.test(name))).toBe(false)
})

test('native document cancellation and bounded arguments fail safely and tools unregister after session revocation', async ({ page }) => {
 await login(page, 'Owner'); await expect.poll(() => names(page)).toContain('society_find_documents'); await expect(execute(page, 'society_find_documents', { query: 'x'.repeat(101) })).rejects.toThrow(); await expect(execute(page, 'society_read_document', { document_id: 'x'.repeat(101), history_page: -1 })).rejects.toThrow()
 const cancelled = await page.evaluate(async () => { const context = (document as ContextDocument).modelContext; const tool = (await context.getTools()).find(item => item.name === 'society_find_documents')!; const controller = new AbortController(); controller.abort(); const major = Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1]); try { await context.executeTool(tool, major < 155 ? '{}' : {}, { signal: controller.signal }); return false } catch { return true } }); expect(cancelled).toBe(true)
 const me = await (await page.request.get('/api/auth/me')).json(); expect((await page.request.post('/api/auth/logout', { headers: { Origin: new URL(page.url()).origin, 'X-CSRF-Token': me.csrf_token }, data: {} })).status()).toBe(200); await expect(execute(page, 'society_find_documents', {})).rejects.toThrow(); await expect.poll(() => names(page)).toEqual([])
})

test('actual native maintenance metadata matches separately published visible charges and an explicit receipt allocation',async({page,browser})=>{
  await login(page);await ensureMaintenanceReviewer(page);await expect.poll(()=>names(page)).toContain('society_read_maintenance')
  const title='NATIVE maintenance '+Date.now(),id=await apiMaintenance(page,title)
  const pending=JSON.parse(await execute(page,'society_find_maintenance',{query:title}));expect(pending.total).toBe(1);expect(pending.items[0].requested_paise).toBe(175025);expect(pending.totals.active_paise).toBe(0)
  const context=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await context.newPage();await login(reviewer,'Committee');await navigate(reviewer,'Maintenance');await reviewer.getByRole('button',{name:'Open period '+title,exact:true}).click();await reviewer.getByRole('button',{name:'Approve & publish',exact:true}).click();await reviewer.getByRole('textbox',{name:'Decision reason',exact:true}).fill('PRIVATE native review of every supplied maintenance amount')
  const metadata=JSON.parse(await execute(reviewer,'society_read_maintenance',{period_id:id}))
  expect(metadata.lines).toHaveLength(2);expect(metadata.requested_paise).toBe(175025)
  for(const field of ['source_reference','note','author','decided_by','decision_reason','events'])expect(metadata).not.toHaveProperty(field)
  await expect(reviewer.getByRole('textbox',{name:'Decision reason',exact:true})).toHaveValue('PRIVATE native review of every supplied maintenance amount')
  await expect(execute(reviewer,'society_open_workspace',{screen:'overview'})).rejects.toThrow()
  await reviewer.getByRole('dialog').getByRole('checkbox').check();await reviewer.getByRole('button',{name:'Confirm publication',exact:true}).click();await expect(reviewer.getByText('MAINTENANCE · PUBLISHED',{exact:true})).toBeVisible();await context.close()
  const receipt=await apiReceived(page,'400.00')
  await execute(page,'society_open_workspace',{screen:'maintenance'});await expect(page.getByRole('heading',{name:'The maintenance calendar',exact:true})).toBeVisible()
  await chooseOption(page,'Statement home','Home A-101');await page.getByRole('button',{name:'Open statement',exact:true}).click();await page.getByRole('button',{name:'Allocate credit',exact:true}).click()
  await chooseOption(page,'Allocation credit source',receipt.receipt_number+' · ₹400.00 available');await chooseOption(page,'Allocation charge','Maintenance · '+title+' · ₹1,000.00 due');await page.getByRole('textbox',{name:'Amount to allocate',exact:true}).fill('400.00');await page.getByRole('textbox',{name:'Allocation reason',exact:true}).fill('PRIVATE native allocation reason checked against original receipt')
  const mutations:string[]=[];const listener=(request:import('@playwright/test').Request)=>{if(!['GET','HEAD'].includes(request.method()))mutations.push(request.url())};page.on('request',listener)
  const before=JSON.parse(await execute(page,'society_read_home_statement',{home_id:'demo-flat-A-101'}));expect(before.charges.length).toBeLessThanOrEqual(20);expect(before.credits.length).toBeLessThanOrEqual(20);expect(before.allocations.length).toBeLessThanOrEqual(20);expect(mutations).toEqual([])
  await expect(page.getByRole('textbox',{name:'Allocation reason',exact:true})).toHaveValue('PRIVATE native allocation reason checked against original receipt');await expect(execute(page,'society_open_workspace',{screen:'maintenance'})).rejects.toThrow();page.off('request',listener)
  await page.getByRole('dialog').getByRole('checkbox').check();await page.getByRole('button',{name:'Confirm allocation',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'Allocation recorded.'})).toBeVisible()
  const after=JSON.parse(await execute(page,'society_read_home_statement',{home_id:'demo-flat-A-101'}));expect(after.allocated_paise-before.allocated_paise).toBe(40000);expect(before.outstanding_paise-after.outstanding_paise).toBe(40000)
  for(const allocation of after.allocations){expect(allocation).not.toHaveProperty('reason');expect(allocation).not.toHaveProperty('actor');expect(allocation).not.toHaveProperty('correction_reason')}
  const period=JSON.parse(await execute(page,'society_read_maintenance',{period_id:id}));expect(period.active_paise).toBe(175025);expect(period.allocated_paise).toBe(40000);expect(period.outstanding_paise).toBe(135025)
  await page.getByRole('button',{name:'Close home statement',exact:true}).click()
})

test('native maintenance tools reject unsupported writes invalid pages and cancellation and are omitted for an unentitled tenant',async({page})=>{
  await login(page);await expect.poll(()=>names(page)).toContain('society_read_home_statement')
  const invalid:[string,unknown][]=[
    ['society_find_maintenance',{state:'APPROVED'}],['society_find_maintenance',{query:'x'.repeat(101)}],['society_find_maintenance',{page:0}],
    ['society_find_maintenance',{confirmed:true}],['society_read_maintenance',{period_id:'unknown',event_page:1}],['society_read_maintenance',{period_id:'unknown',line_page:10001}],
    ['society_read_home_statement',{home_id:'demo-flat-A-101',amount:'10.00'}],['society_read_home_statement',{home_id:'demo-flat-A-101',credit_page:-1}],
  ]
  for(const [tool,input]of invalid)await expect(execute(page,tool,input)).rejects.toThrow()
  const cancelled=await page.evaluate(async()=>{
    const context=(document as ContextDocument).modelContext,tool=(await context.getTools()).find(item=>item.name==='society_read_home_statement')!,controller=new AbortController();controller.abort()
    const input={home_id:'demo-flat-A-101'},major=Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1])
    try{await context.executeTool(tool,major<155?JSON.stringify(input):input,{signal:controller.signal});return false}catch{return true}
  });expect(cancelled).toBe(true)
  await page.getByRole('button',{name:'Sign out',exact:true}).click();await login(page,'Tenant')
  for(const tool of ['society_find_maintenance','society_read_maintenance','society_read_home_statement'])await expect.poll(()=>names(page)).not.toContain(tool)
  await expect(execute(page,'society_open_workspace',{screen:'maintenance'})).rejects.toThrow()
  await expect(execute(page,'society_read_overview',{section:'maintenance'})).rejects.toThrow()
  expect((await page.request.get('/api/statements/demo-flat-A-101')).status()).toBe(403)
})

test('native captured home statements are discarded when one financial entitlement ends and the other remains active',async({page})=>{
  await login(page,'Owner');await expect.poll(()=>names(page)).toContain('society_read_home_statement');await execute(page,'society_open_workspace',{screen:'maintenance'})
  await chooseOption(page,'Statement home','Home A-102');await page.getByRole('button',{name:'Open statement',exact:true}).click();await expect(page.getByRole('heading',{name:'Home A-102',exact:true})).toBeVisible();await expect(page.getByText('Updating this statement…',{exact:true})).toHaveCount(0)
  const path=process.env.SOCIETY_BROWSER_DB;if(!path||!path.includes('society-browser-'))throw new Error('A disposable synthetic database is required for this current-entitlement fixture.')
  const entitlement=(enabled:boolean)=>execFileSync('python3',['-c',"import sqlite3,sys;db=sqlite3.connect(sys.argv[1]);r=db.execute(\"UPDATE flat_memberships SET can_view_finances=? WHERE flat_id='demo-flat-A-102' AND resident_id=(SELECT resident_id FROM users WHERE id='demo-user-owner') AND end_date IS NULL\",(int(sys.argv[2]),));assert r.rowcount==1;db.commit()",path,enabled?'1':'0'])
  let captured!:()=>void,release!:()=>void;const ready=new Promise<void>(resolve=>{captured=resolve}),pending=new Promise<void>(resolve=>{release=resolve})
  await page.route('**/api/statements/demo-flat-A-102?**',async route=>{const response=await route.fetch();captured();await pending;await route.fulfill({response}).catch(()=>{})})
  const reading=execute(page,'society_read_home_statement',{home_id:'demo-flat-A-102'});await ready;entitlement(false)
  try{
    release();await expect(reading).rejects.toThrow();await page.unroute('**/api/statements/demo-flat-A-102?**')
    await expect(page.getByRole('dialog')).toHaveCount(0);await expect.poll(()=>names(page)).toContain('society_read_home_statement')
    await expect(execute(page,'society_read_home_statement',{home_id:'demo-flat-A-102'})).rejects.toThrow()
    const allowed=JSON.parse(await execute(page,'society_read_home_statement',{home_id:'demo-flat-A-101'}));expect(allowed.flat_id).toBe('demo-flat-A-101')
    await page.getByRole('combobox',{name:'Statement home',exact:true}).click();await expect(page.getByRole('option',{name:'Home A-102',exact:true})).toHaveCount(0);await expect(page.getByRole('option',{name:'Home A-101',exact:true})).toBeVisible();await page.keyboard.press('Escape')
  }finally{entitlement(true);release();await page.unroute('**/api/statements/demo-flat-A-102?**')}
})

test('actual native upkeep metadata preserves the human form and excludes private work and register details',async({page})=>{
 await login(page);await expect.poll(()=>names(page)).toContain('society_read_upkeep_register')
 const title='NATIVE private upkeep '+Date.now(),id=await apiWork(page,title),vendor=await apiCareRecord(page,'VENDOR','NATIVE fictional care partner')
 await execute(page,'society_open_workspace',{screen:'upkeep'});await page.getByRole('button',{name:'Plan work',exact:true}).click();await page.getByRole('textbox',{name:'Work title',exact:true}).fill('PRIVATE unsaved work title');await page.getByRole('textbox',{name:'Private work details',exact:true}).fill('PRIVATE human instructions must remain in this form')
 const mutations:string[]=[];const listener=(request:import('@playwright/test').Request)=>{if(!['GET','HEAD'].includes(request.method()))mutations.push(request.url())};page.on('request',listener)
 const found=JSON.parse(await execute(page,'society_find_upkeep',{query:title}));expect(found.total).toBe(1);expect(found.items).toHaveLength(1);expect(found.items[0].state).toBe('PLANNED');expect(found.items.length).toBeLessThanOrEqual(12)
 const work=JSON.parse(await execute(page,'society_read_upkeep',{task_id:id}));const directory=JSON.parse(await execute(page,'society_find_upkeep_register',{kind:'VENDOR',query:'NATIVE fictional care partner'}));const record=JSON.parse(await execute(page,'society_read_upkeep_register',{kind:'VENDOR',record_id:vendor}));expect(directory.total).toBe(1);expect(record.state).toBe('ACTIVE')
 for(const data of [work,record,directory.items[0]])for(const field of ['body','events','contact','phone','email','source_reference','location','assigned_to','ready_by','complaint_id'])expect(data).not.toHaveProperty(field)
 await expect(page.getByRole('textbox',{name:'Work title',exact:true})).toHaveValue('PRIVATE unsaved work title');await expect(page.getByRole('textbox',{name:'Private work details',exact:true})).toHaveValue('PRIVATE human instructions must remain in this form');await expect(execute(page,'society_open_workspace',{screen:'overview'})).rejects.toThrow();expect(mutations).toEqual([]);page.off('request',listener);await page.getByRole('button',{name:'Close work preparation',exact:true}).click()
 await page.goto('/#upkeep?task='+id);await expect(page.getByRole('dialog').getByRole('heading',{name:title,exact:true})).toBeVisible();const overview=JSON.parse(await execute(page,'society_read_overview',{section:'upkeep'}));expect(overview.items.length).toBeLessThanOrEqual(4);expect(overview.counts.open).toBeGreaterThanOrEqual(1)
})

test('native upkeep publication stays frozen across private activity and residents cannot discover operational registers',async({page,browser})=>{
 await login(page);const id=await apiWork(page,'NATIVE internal pump work',{due_date:careDate(-1)}),hidden=await apiWork(page,'NATIVE hidden internal work')
 await apiWorkAction(page,id,'PUBLISH',{audience:'BUILDING',building_code:'A',public_title:'NATIVE shared care update',public_body:'A deliberately supplied fictional resident update for Wing A.'})
 const context=await browser.newContext({baseURL:new URL(page.url()).origin}),owner=await context.newPage();await login(owner,'Owner');await expect.poll(()=>names(owner)).toContain('society_read_upkeep');await expect.poll(()=>names(owner)).not.toContain('society_find_upkeep_register')
 const before=JSON.parse(await execute(owner,'society_read_upkeep',{task_id:id}));expect(before.title).toBe('NATIVE shared care update');expect(before.state).toBe('PLANNED');expect(before.version).toBe(1)
 await expect(execute(owner,'society_read_upkeep',{task_id:hidden})).rejects.toThrow();expect((await owner.request.get('/api/upkeep/options')).status()).toBe(403)
 await apiWorkAction(page,id,'COMMENT');await apiWorkAction(page,id,'START');expect(JSON.parse(await execute(owner,'society_read_upkeep',{task_id:id}))).toEqual(before)
 const found=JSON.parse(await execute(owner,'society_find_upkeep',{query:'NATIVE shared care update',state:'PLANNED'}));expect(found.total).toBe(1);expect(found.counts.overdue).toBe(1);expect(found.counts.unassigned).toBe(0)
 await execute(owner,'society_open_workspace',{screen:'upkeep'});await owner.getByRole('button',{name:'Open work NATIVE shared care update',exact:true}).click();await expect(owner.getByRole('dialog').getByText('Planned',{exact:true})).toBeVisible();await expect(owner.getByRole('heading',{name:'Private activity',exact:true})).toHaveCount(0)
 await apiWorkAction(page,id,'UNPUBLISH');await expect(execute(owner,'society_read_upkeep',{task_id:id})).rejects.toThrow();await context.close()
})

test('native upkeep rejects unsupported writes unbounded input cancellation and captured results after audience membership ends',async({page,browser})=>{
 await login(page);const id=await apiWork(page,'NATIVE scope changing work');await apiWorkAction(page,id,'PUBLISH',{audience:'ALL_RESIDENTS',public_title:'NATIVE current homes care',public_body:'This fictional update is only for current residents.'})
 const invalid:[string,unknown][]=[['society_find_upkeep',{query:'x'.repeat(101)}],['society_find_upkeep',{state:'APPROVED'}],['society_find_upkeep',{page:10001}],['society_read_upkeep',{task_id:id,action:'CONFIRM_DONE'}],['society_read_upkeep',{task_id:'x'.repeat(101)}],['society_find_upkeep_register',{kind:'OTHER'}],['society_read_upkeep_register',{kind:'VENDOR',record_id:'missing',source_reference:'write'}]]
 for(const [name,input]of invalid)await expect(execute(page,name,input)).rejects.toThrow()
 const cancelled=await page.evaluate(async()=>{const context=(document as ContextDocument).modelContext,tool=(await context.getTools()).find(t=>t.name==='society_find_upkeep')!,controller=new AbortController();controller.abort();const major=Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1]);try{await context.executeTool(tool,major<155?'{}':{},{signal:controller.signal});return false}catch{return true}});expect(cancelled).toBe(true)
 const context=await browser.newContext({baseURL:new URL(page.url()).origin}),owner=await context.newPage();await login(owner,'Owner');await owner.goto('/#upkeep?task='+id);await expect(owner.getByRole('dialog').getByRole('heading',{name:'NATIVE current homes care',exact:true})).toBeVisible()
 const path=process.env.SOCIETY_BROWSER_DB;if(!path||!path.includes('society-browser-'))throw new Error('A disposable synthetic database is required for audience checks.')
 const membership=(ended:boolean)=>execFileSync('python3',['-c',"import sqlite3,sys;db=sqlite3.connect(sys.argv[1]);r=db.execute(\"UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101' AND flat_id IN ('demo-flat-A-101','demo-flat-A-102')\",(sys.argv[2] if sys.argv[3]=='end' else None,));assert r.rowcount==2;db.commit()",path,careDate(),ended?'end':'restore'])
 let captured!:()=>void,release!:()=>void;const ready=new Promise<void>(resolve=>{captured=resolve}),pending=new Promise<void>(resolve=>{release=resolve})
 await owner.route('**/api/upkeep/tasks/'+id,async route=>{const response=await route.fetch();captured();await pending;await route.fulfill({response}).catch(()=>{})})
 const reading=execute(owner,'society_read_upkeep',{task_id:id});await ready;membership(true)
 try{release();await expect(reading).rejects.toThrow();await owner.unroute('**/api/upkeep/tasks/'+id);await expect(owner.getByRole('dialog')).toHaveCount(0);await expect(execute(owner,'society_read_upkeep',{task_id:id})).rejects.toThrow();expect(JSON.parse(await execute(owner,'society_find_upkeep',{})).items).toEqual([])}finally{release();membership(false);await owner.unroute('**/api/upkeep/tasks/'+id);await context.close()}
})

test('native collection and report metadata preserves unsaved forms and tracks separate verification without private source or evidence fields',async({page,browser})=>{
 test.setTimeout(60000);await login(page);await ensureMaintenanceReviewer(page);await expect.poll(()=>names(page)).toContain('society_read_collection');const title='NATIVE shared fund '+Date.now(),id=await apiFund(page,title);await execute(page,'society_open_workspace',{screen:'collections'});await page.getByRole('button',{name:'Prepare a fund',exact:true}).click();await page.getByRole('textbox',{name:'Fund title',exact:true}).fill('PRIVATE human proposal is still being prepared');await page.getByRole('textbox',{name:'Private supporting note (optional)',exact:true}).fill('PRIVATE unsaved source note');const writes:string[]=[];const listener=(request:import('@playwright/test').Request)=>{if(!['GET','HEAD'].includes(request.method()))writes.push(request.url())};page.on('request',listener)
 const pending=JSON.parse(await execute(page,'society_find_collections',{query:title}));expect(pending.total).toBe(1);expect(pending.items[0].requested_paise).toBe(150025);expect(pending.totals.outstanding_paise).toBe(0);const frozen=JSON.parse(await execute(page,'society_read_collection',{campaign_id:id}));expect(frozen.lines.length).toBe(2);expect(JSON.stringify(frozen)).not.toContain('PRIVATE');for(const field of ['purpose','source_reference','note','events','author_id','decided_by','homes'])expect(frozen).not.toHaveProperty(field);await expect(page.getByRole('textbox',{name:'Fund title',exact:true})).toHaveValue('PRIVATE human proposal is still being prepared');await expect(page.getByRole('textbox',{name:'Private supporting note (optional)',exact:true})).toHaveValue('PRIVATE unsaved source note');await expect(execute(page,'society_open_workspace',{screen:'overview'})).rejects.toThrow();expect(writes).toEqual([]);page.off('request',listener);await page.getByRole('button',{name:'Close fund preparation',exact:true}).click()
 const context=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await context.newPage();await login(reviewer,'Committee');await apiFundAction(reviewer,id,'PUBLISHED');const ownerContext=await browser.newContext({baseURL:new URL(page.url()).origin}),owner=await ownerContext.newPage();await login(owner,'Owner');const claim=await apiFundReport(owner,id,'400.00','PRIVATE native external reference '+id);await page.goto('/#collections?report='+claim);await chooseOption(page,'Payment report action','Verify payment');await page.getByRole('textbox',{name:'Private verification source',exact:true}).fill('PRIVATE native checked bank statement');await page.getByRole('textbox',{name:'Private payment identity',exact:true}).fill('PRIVATE native bank row '+id);await page.getByRole('textbox',{name:'Amount to allocate to this fund',exact:true}).fill('400.00');await page.getByRole('textbox',{name:'Decision reason',exact:true}).fill('PRIVATE checked original statement before native comparison');page.on('request',listener)
 const found=JSON.parse(await execute(page,'society_find_payment_reports',{campaign_id:id}));expect(found.total).toBe(1);expect(found.items[0].amount_paise).toBe(40000);const read=JSON.parse(await execute(page,'society_read_payment_report',{report_id:claim}));expect(read.receipt).toBe('');expect(read.state).toBe('PENDING');for(const field of ['PRIVATE','payer','reference','evidence','comment','verification_source','payment_identity','reason','events','author'])expect(JSON.stringify(read)).not.toContain(field);await expect(page.getByRole('textbox',{name:'Decision reason',exact:true})).toHaveValue('PRIVATE checked original statement before native comparison');expect(writes).toEqual([]);page.off('request',listener);await page.getByRole('dialog').getByRole('checkbox').check();await page.getByRole('button',{name:'Verify received payment',exact:true}).click();await expect(page.getByRole('dialog').getByText('Verified',{exact:true})).toBeVisible();const verified=JSON.parse(await execute(owner,'society_read_payment_report',{report_id:claim}));expect(verified.state).toBe('CONFIRMED');expect(verified.receipt).not.toBe('');const fund=JSON.parse(await execute(owner,'society_read_collection',{campaign_id:id}));expect(fund.allocated_paise).toBe(40000);expect(fund.outstanding_paise).toBe(110025);const overview=JSON.parse(await execute(owner,'society_read_overview',{section:'collections'}));expect(overview.counts.confirmed_paise).toBeGreaterThanOrEqual(40000);expect(overview.items.length).toBeLessThanOrEqual(4);await context.close();await ownerContext.close()
})

test('native voluntary and private exemption metadata stay bounded and resident tools omit private exemption access',async({page,browser})=>{
 await login(page);await ensureMaintenanceReviewer(page);const context=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await context.newPage();await login(reviewer,'Committee');const fixed=await apiFund(page,'NATIVE exemption source '+Date.now());await apiFundAction(reviewer,fixed,'PUBLISHED');const report=await apiFundReport(reviewer,fixed,'600.00','NATIVE-CREDIT-'+fixed),received=await apiFundVerify(page,report,'NATIVE-CREDIT-'+fixed,'0.00');const voluntary=await apiFund(page,'NATIVE voluntary purpose '+Date.now(),{contribution_type:'VOLUNTARY',lines:[{flat_id:'demo-flat-A-101',amount:''}]});await apiFundAction(reviewer,voluntary,'PUBLISHED');const headers=await financialHeaders(page);const saved=await page.request.post('/api/fund-contributions',{headers,data:{operation_key:crypto.randomUUID(),campaign_id:voluntary,flat_id:'demo-flat-A-101',source_id:received.entry_id,amount:'500.00',reason:'PRIVATE native purpose attribution reason',confirmed:true}});expect(saved.status()).toBe(200);const waiverSaved=await page.request.post('/api/fund-waivers',{headers,data:{operation_key:crypto.randomUUID(),campaign_id:fixed,flat_id:'demo-flat-A-101',participant_version:1,amount:'300.00',source_reference:'PRIVATE native exemption source',reason:'PRIVATE supplied exception before a separate reviewer',confirmed:true}});expect(waiverSaved.status()).toBe(200);const waiver=(await waiverSaved.json()).id
 const contributions=JSON.parse(await execute(page,'society_find_fund_contributions',{campaign_id:voluntary}));expect(contributions.total).toBe(1);expect(contributions.items[0].amount_paise).toBe(50000);expect(contributions.items[0].receipt).toBe(received.receipt);const exemptions=JSON.parse(await execute(page,'society_find_fund_exemptions',{campaign_id:fixed}));expect(exemptions.total).toBe(1);const exception=JSON.parse(await execute(page,'society_read_fund_exemption',{waiver_id:waiver}));expect(exception.amount_paise).toBe(30000);for(const field of ['PRIVATE','source_reference','reason','events','reviewer','author']){expect(JSON.stringify(exception)).not.toContain(field);expect(JSON.stringify(contributions)).not.toContain(field)}
 const ownerContext=await browser.newContext({baseURL:new URL(page.url()).origin}),owner=await ownerContext.newPage();await login(owner,'Owner');await expect.poll(()=>names(owner)).not.toContain('society_find_fund_exemptions');await expect.poll(()=>names(owner)).not.toContain('society_read_fund_exemption');expect(JSON.parse(await execute(owner,'society_find_payment_reports',{campaign_id:fixed})).total).toBe(0);const scoped=JSON.parse(await execute(owner,'society_find_fund_contributions',{campaign_id:voluntary}));expect(scoped.total).toBe(1);expect(scoped.items[0].amount_paise).toBe(50000);expect((await owner.request.get('/api/fund-waivers/'+waiver)).status()).toBe(403);await context.close();await ownerContext.close()
})

test('native collections reject unsupported writes cancellation and stale captured home results then rediscover permitted scope',async({page,browser})=>{
 test.setTimeout(60000);await login(page);await ensureMaintenanceReviewer(page);const reviewContext=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await reviewContext.newPage();await login(reviewer,'Committee');const id=await apiFund(page,'NATIVE current collection scope '+Date.now());await apiFundAction(reviewer,id,'PUBLISHED');const invalid:[string,unknown][]=[['society_find_collections',{state:'APPROVED'}],['society_find_collections',{query:'x'.repeat(101)}],['society_find_collections',{page:10001}],['society_read_collection',{campaign_id:id,action:'PUBLISHED'}],['society_find_payment_reports',{evidence_id:'private'}],['society_read_payment_report',{report_id:'x'.repeat(101)}],['society_find_fund_contributions',{home_id:1}],['society_find_fund_exemptions',{state:'UNKNOWN'}],['society_read_fund_exemption',{waiver_id:'missing',reason:'write'}]];for(const [tool,args]of invalid)await expect(execute(page,tool,args)).rejects.toThrow();const cancelled=await page.evaluate(async()=>{const context=(document as ContextDocument).modelContext,tool=(await context.getTools()).find(item=>item.name==='society_find_collections')!,controller=new AbortController();controller.abort();const major=Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1]);try{await context.executeTool(tool,major<155?'{}':{},{signal:controller.signal});return false}catch{return true}});expect(cancelled).toBe(true)
 const context=await browser.newContext({baseURL:new URL(page.url()).origin}),owner=await context.newPage();await login(owner,'Owner');const report=await apiFundReport(owner,id,'20.00','NATIVE-SCOPE-'+id);await owner.goto('/#collections?report='+report);await expect(owner.getByRole('dialog').getByText('Awaiting verification',{exact:true})).toBeVisible();const path=process.env.SOCIETY_BROWSER_DB;if(!path||!path.includes('society-browser-'))throw new Error('Isolated collection scope fixture required');const scope=(allowed:boolean)=>execFileSync('python3',['-c',"import sqlite3,sys;db=sqlite3.connect(sys.argv[1]);db.execute(\"UPDATE flat_memberships SET can_view_finances=? WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'\",(int(sys.argv[2]),));db.commit()",path,allowed?'1':'0']);let release!:()=>void,captured!:()=>void;const pending=new Promise<void>(resolve=>{release=resolve}),ready=new Promise<void>(resolve=>{captured=resolve});await owner.route('**/api/payment-reports/'+report,async route=>{const response=await route.fetch();captured();await pending;await route.fulfill({response}).catch(()=>{})});const reading=execute(owner,'society_read_payment_report',{report_id:report});await ready
 try{scope(false);release();await expect(reading).rejects.toThrow();await owner.unroute('**/api/payment-reports/'+report);await expect(owner.getByRole('dialog')).toHaveCount(0);await expect(execute(owner,'society_read_payment_report',{report_id:report})).rejects.toThrow();const current=JSON.parse(await execute(owner,'society_read_collection',{campaign_id:id}));expect(current.participants).toBe(1);expect(current.outstanding_paise).toBe(50025)}finally{release();scope(true);await owner.unroute('**/api/payment-reports/'+report);await context.close()}
 const tenantContext=await browser.newContext({baseURL:new URL(page.url()).origin}),tenant=await tenantContext.newPage();await login(tenant,'Tenant');for(const tool of ['society_find_collections','society_read_collection','society_find_payment_reports','society_read_payment_report','society_find_fund_contributions','society_find_fund_exemptions','society_read_fund_exemption'])await expect.poll(()=>names(tenant)).not.toContain(tool);await expect(execute(tenant,'society_open_workspace',{screen:'collections'})).rejects.toThrow();await expect(execute(tenant,'society_read_overview',{section:'collections'})).rejects.toThrow();await tenantContext.close();await reviewContext.close()
})



test('native rule and incident metadata remain bounded private and read only while a human report is prepared',async({page,browser})=>{
  test.setTimeout(60000);await login(page);await expect.poll(()=>names(page)).toContain('society_read_incident')
  const reviewContext=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await reviewContext.newPage();await login(reviewer,'Committee')
  const title='Native private rule '+Date.now(),rule=await apiRule(page,title);await apiRuleAction(reviewer,rule,'PUBLISHED')
  const pending=await apiRule(page,'Native unpublished policy '+Date.now())
  const residentContext=await browser.newContext({baseURL:new URL(page.url()).origin}),tenant=await residentContext.newPage();await login(tenant,'Tenant');const own=await apiIncident(tenant,rule)
  const other=await apiIncident(page,rule)
  for(let i=0;i<11;i++)await apiIncident(page,rule)
  await navigate(page,'Rules & conduct');await page.getByRole('button',{name:'Report an observation',exact:true}).click()
  const prepared='PRIVATE human observation retained while a native reader checks metadata.';await page.getByRole('textbox',{name:'Your observation',exact:true}).fill(prepared)
  const mutations:string[]=[],listener=(request:import('@playwright/test').Request)=>{if(!['GET','HEAD'].includes(request.method()))mutations.push(request.url())};page.on('request',listener)
  const metadata=JSON.parse(await execute(page,'society_read_rule',{rule_id:rule}));expect(Object.keys(metadata).sort()).toEqual(['effective_from','effective_until','fine_permitted','id','state','title','version']);expect(metadata.state).toBe('PUBLISHED')
  const found=JSON.parse(await execute(page,'society_find_incidents',{query:title}));expect(found.total).toBe(13);expect(found.items).toHaveLength(12)
  expect(JSON.parse(await execute(page,'society_find_incidents',{query:title,page:2})).items).toHaveLength(1)
  const report=JSON.parse(await execute(page,'society_read_incident',{case_id:own}));expect(Object.keys(report).sort()).toEqual(['home','id','incident_date','rule_id','rule_title','state','version'])
  for(const forbidden of ['comment','reporter_id','picture_id','notice_id','events','policy_reference','PRIVATE fictional observation'])expect(JSON.stringify({metadata,found,report})).not.toContain(forbidden)
  await expect(page.getByRole('textbox',{name:'Your observation',exact:true})).toHaveValue(prepared);await expect(page.getByRole('dialog')).toBeVisible();expect(mutations).toEqual([]);page.off('request',listener)
  for(const [tool,args] of [['society_find_rules',{page:10001}],['society_find_incidents',{query:'x'.repeat(101)}],['society_find_incidents',{state:'ISSUED'}],['society_read_rule',{rule_id:rule,action:'PUBLISHED'}],['society_read_incident',{case_id:own,picture:true}],['society_read_incident_notice',{notice_id:'../private'}]] as const)await expect(execute(page,tool,args)).rejects.toThrow()
  await expect.poll(()=>names(tenant)).toContain('society_find_incidents');expect(JSON.parse(await execute(tenant,'society_find_incidents',{query:title})).total).toBe(1)
  expect(JSON.parse(await execute(tenant,'society_read_incident',{case_id:own})).id).toBe(own);await expect(execute(tenant,'society_read_incident',{case_id:other})).rejects.toThrow();await expect(execute(tenant,'society_read_rule',{rule_id:pending})).rejects.toThrow()
  expect(JSON.parse(await execute(tenant,'society_find_rules',{query:title})).items[0].id).toBe(rule)
  await apiIncidentAction(page,own,'NEEDS_INFO')
  const staffAttention=JSON.parse(await execute(page,'society_read_overview',{section:'incidents'})),ownAttention=JSON.parse(await execute(tenant,'society_read_overview',{section:'incidents'}))
  expect(staffAttention.items.find((item:{id:string})=>item.id===own).kind).toBe('INCIDENT')
  expect(ownAttention.items.find((item:{id:string})=>item.id===own).kind).toBe('INCIDENT_CLARIFICATION')
  expect(ownAttention.counts.information_needed).toBe(1)
  expect(JSON.stringify({staffAttention,ownAttention})).not.toContain('PRIVATE')
  await expect(page.getByRole('textbox',{name:'Your observation',exact:true})).toHaveValue(prepared)
  await residentContext.close();await reviewContext.close()
})


test('native current-home notices expose frozen metadata and personal response attention without private case activity',async({page,browser})=>{
  test.setTimeout(60000);await login(page);const reviewContext=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await reviewContext.newPage();await login(reviewer,'Committee')
  const title='Native frozen notice rule '+Date.now(),rule=await apiRule(page,title);await apiRuleAction(reviewer,rule,'PUBLISHED')
  const residentContext=await browser.newContext({baseURL:new URL(page.url()).origin}),tenant=await residentContext.newPage();await login(tenant,'Tenant');const picture=await apiPicture(tenant),id=await apiIncident(tenant,rule,{picture_id:picture})
  const notice=await apiIncidentAction(page,id,'ISSUE_NOTICE',{title:'Native deliberate response notice',body:'PRIVATE wording deliberately issued only for this fictional household.',response_by:careDate(4)})
  const context=await browser.newContext({baseURL:new URL(page.url()).origin}),owner=await context.newPage();await login(owner,'Owner');await expect.poll(()=>names(owner)).toContain('society_read_incident_notice')
  const before=JSON.parse(await execute(owner,'society_read_incident_notice',{notice_id:notice.notice_id}));expect(Object.keys(before).sort()).toEqual(['active','home','id','incident_date','response_by','title','version']);expect(before.home).toBe('A-101')
  await apiIncidentAction(page,id,'NOTE');expect(JSON.parse(await execute(owner,'society_read_incident_notice',{notice_id:notice.notice_id}))).toEqual(before)
  expect(JSON.parse(await execute(owner,'society_find_incident_notices',{query:'Native deliberate response notice'})).total).toBe(1)
  await expect(execute(owner,'society_read_incident',{case_id:id})).rejects.toThrow();await expect(execute(tenant,'society_read_incident_notice',{notice_id:notice.notice_id})).rejects.toThrow();expect((await owner.request.get('/api/incident-pictures/'+picture+'/original')).status()).toBe(404)
  const attention=JSON.parse(await execute(owner,'society_read_overview',{section:'incidents'}));expect(attention.counts.responses_needed).toBe(1);expect(attention.counts.needs_review).toBe(0)
  await owner.goto('/#conduct?notice='+notice.notice_id);await owner.getByRole('textbox',{name:'Your household response',exact:true}).fill('PRIVATE prepared household response should stay outside metadata.');await expect(execute(owner,'society_open_workspace',{screen:'conduct'})).rejects.toThrow()
  const text=await execute(owner,'society_read_incident_notice',{notice_id:notice.notice_id});for(const forbidden of ['PRIVATE','body','reporter','picture','responses','updated_at','incident_id'])expect(text).not.toContain(forbidden)
  await expect(owner.getByRole('textbox',{name:'Your household response',exact:true})).toHaveValue('PRIVATE prepared household response should stay outside metadata.')
  await owner.getByRole('checkbox',{name:'I reviewed my response to this notice.',exact:true}).check();await owner.getByRole('button',{name:'Submit household response',exact:true}).click();await expect(owner.getByRole('region',{name:'Household responses',exact:true})).toContainText('PRIVATE prepared household response')
  expect(JSON.parse(await execute(owner,'society_read_overview',{section:'incidents'})).counts.responses_needed).toBe(0);expect(JSON.parse(await execute(owner,'society_read_incident_notice',{notice_id:notice.notice_id}))).toEqual(before)
  await apiIncidentAction(page,id,'REMOVE_NOTICE');await expect(execute(owner,'society_read_incident_notice',{notice_id:notice.notice_id})).rejects.toThrow();expect(JSON.parse(await execute(owner,'society_find_incident_notices',{query:'Native deliberate response notice'})).total).toBe(0)
  expect(JSON.parse(await execute(page,'society_read_incident_notice',{notice_id:notice.notice_id})).active).toBe(false)
  await context.close();await residentContext.close();await reviewContext.close()
})


test('native incident reads discard a held notice after only that home ends and honour cancellation with current rediscovery',async({page,browser})=>{
  test.setTimeout(60000);await login(page);const reviewContext=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await reviewContext.newPage();await login(reviewer,'Committee')
  const title='Native notice scope rule '+Date.now(),rule=await apiRule(page,title);await apiRuleAction(reviewer,rule,'PUBLISHED');const id=await apiIncident(reviewer,rule)
  const notice=await apiIncidentAction(page,id,'ISSUE_NOTICE',{title:'A current-home native scope fixture',body:'PRIVATE deliberate wording must not survive the current-home scope boundary.',response_by:careDate(4)})
  const context=await browser.newContext({baseURL:new URL(page.url()).origin}),owner=await context.newPage();await login(owner,'Owner');await owner.goto('/#conduct?notice='+notice.notice_id);await expect(owner.getByRole('textbox',{name:'Your household response',exact:true})).toBeVisible()
  const path=process.env.SOCIETY_BROWSER_DB;if(!path||!path.includes('society-browser-'))throw new Error('Disposable native incident scope fixture required')
  const scope=(ended:boolean)=>execFileSync('python3',['-c',"import sqlite3,sys;db=sqlite3.connect(sys.argv[1]);r=db.execute(\"UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'\",(sys.argv[2] if sys.argv[3]=='end' else None,));assert r.rowcount==1;db.commit()",path,careDate(),ended?'end':'restore'])
  let release!:()=>void,captured!:()=>void;const pending=new Promise<void>(resolve=>{release=resolve}),ready=new Promise<void>(resolve=>{captured=resolve})
  await owner.route('**/api/incident-notices/'+notice.notice_id,async route=>{const response=await route.fetch();captured();await pending;await route.fulfill({response}).catch(()=>{})})
  const reading=execute(owner,'society_read_incident_notice',{notice_id:notice.notice_id});await ready
  try{
    scope(true);release();await expect(reading).rejects.toThrow();await owner.unroute('**/api/incident-notices/'+notice.notice_id);await expect(owner.getByRole('dialog')).toHaveCount(0)
    expect((await(await owner.request.get('/api/auth/me')).json()).can_read_records).toBe(true);await expect.poll(()=>names(owner)).toContain('society_read_incident_notice');await expect(execute(owner,'society_read_incident_notice',{notice_id:notice.notice_id})).rejects.toThrow()
    expect(JSON.parse(await execute(owner,'society_read_overview',{section:'incidents'})).counts.responses_needed).toBe(0)
  }finally{release();scope(false);await owner.unroute('**/api/incident-notices/'+notice.notice_id);await context.close()}
  const cancelled=await page.evaluate(async(rule)=>{
    const context=(document as ContextDocument).modelContext,tool=(await context.getTools()).find(x=>x.name==='society_read_rule')!,controller=new AbortController();controller.abort();const args={rule_id:rule},major=Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1])
    try{await context.executeTool(tool,major<155?JSON.stringify(args):args,{signal:controller.signal});return false}catch{return true}
  },rule);expect(cancelled).toBe(true);await reviewContext.close()
})
