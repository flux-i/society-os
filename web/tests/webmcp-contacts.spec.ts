import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { login } from './helpers'
import { financialHeaders } from './maintenance-fixtures'
import { names, execute } from './native-webmcp-helpers'
import type { ContextDocument } from './native-webmcp-helpers'

async function post(page: Page, path: string, data: Record<string, unknown>) {
  const response = await page.request.post(path, { headers: await financialHeaders(page), data: { operation_key: crypto.randomUUID(), confirmed: true, ...data } })
  expect(response.status(), await response.text()).toBe(200)
  return response.json()
}
async function proposal(owner: Page) {
  const current = await (await owner.request.get('/api/contacts/me')).json()
  return post(owner, '/api/contacts/me/register', { version: current.version, phone: '+919000000701', email: 'native-contact@example.test', preferred_channel: 'EMAIL', community_whatsapp: true, community_email: false, finance_whatsapp: false, finance_email: true, consent_source: 'PRIVATE_NATIVE_SOURCE actual person and deliberate permission reference', reason: 'PRIVATE_NATIVE_REASON supplied the fictional contact destinations for separate review.' })
}
function privateFixture(script: string) {
  const path = process.env.SOCIETY_BROWSER_DB
  if (!path || !path.includes('society-browser-')) throw new Error('A disposable native contact database is required.')
  execFileSync('python3', ['-c', 'import sqlite3,sys;db=sqlite3.connect(sys.argv[1]);db.execute("PRAGMA foreign_keys=ON");' + script + ';db.commit()', path])
}

test('native contact metadata matches exact preferences while excluding private destinations and preserving the ordinary human form', async ({ page, browser }) => {
  test.setTimeout(60000)
  await login(page)
  const ownerContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), owner = await ownerContext.newPage()
  try {
    await login(owner, 'Owner')
    const { id } = await proposal(owner)
    const pending = await (await page.request.get('/api/contacts/' + id)).json()
    await post(page, '/api/contacts/' + id + '/actions', { version: pending.version, action: 'VERIFIED', reason: 'PRIVATE_NATIVE_CHECK independently checked the supplied person and recorded choices.' })
    await owner.goto('/#contacts?person=me')
    await owner.getByRole('button', { name: 'Change contact choices', exact: true }).click()
    const marker = 'PRIVATE_NATIVE_FORM Human changes must stay in this unfinished form.'
    await owner.getByRole('textbox', { name: 'Registration reason', exact: true }).fill(marker)
    const writes: string[] = [], listener = (request: import('@playwright/test').Request) => { if (!['GET', 'HEAD'].includes(request.method())) writes.push(request.url()) }
    owner.on('request', listener)
    const metadata = JSON.parse(await execute(owner, 'society_read_contact', { contact_id: 'me' }))
    expect(Object.keys(metadata).sort()).toEqual(['id', 'name', 'state', 'current', 'version', 'preferred_channel', 'destinations_recorded', 'permissions', 'eligible'].sort())
    expect(metadata.state).toBe('VERIFIED'); expect(metadata.current).toBe(true)
    expect(metadata.destinations_recorded).toEqual({ whatsapp: true, email: true })
    const preferences = { community_whatsapp: true, community_email: false, finance_whatsapp: false, finance_email: true }
    expect(metadata.permissions).toEqual(preferences); expect(metadata.eligible).toEqual(preferences)
    const own = JSON.parse(await execute(owner, 'society_find_contacts', {}))
    expect(own.total).toBe(1); expect(own.items).toEqual([metadata]); expect(own.page_size).toBe(12)
    for (const forbidden of ['+919000000701', 'native-contact@example.test', 'PRIVATE_NATIVE', 'consent_source', 'decision_reason', 'submitted_by', 'reviewed_by', 'events', 'homes']) expect(JSON.stringify(own)).not.toContain(forbidden)
    await expect(owner.getByRole('textbox', { name: 'Registration reason', exact: true })).toHaveValue(marker)
    await expect(execute(owner, 'society_open_workspace', { screen: 'contacts' })).rejects.toThrow()
    expect(writes).toEqual([]); owner.off('request', listener)
    const owners = JSON.parse(await execute(page, 'society_find_contacts', { relationship: 'OWNER', wing: 'A' }))
    expect(owners.total).toBe(40); expect(owners.items).toHaveLength(12)
    const tenants = JSON.parse(await execute(page, 'society_find_contacts', { relationship: 'TENANT' }))
    expect(tenants.total).toBe(35); expect(tenants.items).toHaveLength(12)
    expect((await names(owner)).filter(name => /contact/.test(name)).sort()).toEqual(['society_find_contacts', 'society_read_contact'])
  } finally { await ownerContext.close() }
})

test('native contact reads enforce person privacy and discard held results after membership ends while preserving own opt-out metadata', async ({ page, browser }) => {
  test.setTimeout(60000); await login(page)
  const ownerContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), owner = await ownerContext.newPage()
  const tenantContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), tenant = await tenantContext.newPage()
  let release: () => void = () => {}
  try {
    await login(owner, 'Owner'); await proposal(owner)
    const x = await (await page.request.get('/api/contacts/demo-owner-A-101')).json()
    await post(page, '/api/contacts/' + x.id + '/actions', { version: x.version, action: 'VERIFIED', reason: 'PRIVATE_NATIVE_CURRENT independently verified this current contact and preferences.' })
    privateFixture("db.execute(\"INSERT INTO flat_memberships VALUES('native-contact-shared','demo-flat-A-101','demo-tenant-A-103','TENANT','2020-01-01',NULL,0,0)\")")
    await login(tenant, 'Tenant')
    await expect.poll(() => names(tenant)).toContain('society_read_contact')
    expect((await names(tenant)).includes('society_read_fine')).toBe(false)
    await expect(execute(tenant, 'society_read_contact', { contact_id: x.id })).rejects.toThrow()
    const ownTenant = JSON.parse(await execute(tenant, 'society_find_contacts', {}))
    expect(ownTenant.total).toBe(1); expect(ownTenant.items[0].id).toBe('demo-tenant-A-103')
    expect(JSON.stringify(ownTenant)).not.toContain('demo-owner-A-101')
    await owner.goto('/#contacts?person=me'); await expect(owner.getByRole('dialog')).toBeVisible()
    let captured!: () => void
    const held = new Promise<void>(resolve => { release = resolve }), ready = new Promise<void>(resolve => { captured = resolve })
    await owner.route('**/api/contacts/me', async route => { const response = await route.fetch(); captured(); await held; await route.fulfill({ response }).catch(() => {}) })
    const reading = execute(owner, 'society_read_contact', { contact_id: 'me' }); await ready
    privateFixture("r=db.execute(\"UPDATE flat_memberships SET end_date='2026-01-01' WHERE resident_id='demo-owner-A-101'\");assert r.rowcount==2")
    release(); await expect(reading).rejects.toThrow(); await owner.unroute('**/api/contacts/me')
    await expect(owner.getByRole('dialog')).toHaveCount(0)
    await expect.poll(() => names(owner)).toContain('society_read_contact')
    const former = JSON.parse(await execute(owner, 'society_read_contact', { contact_id: 'me' }))
    expect(former.current).toBe(false)
    expect(former.permissions).toEqual({ community_whatsapp: true, community_email: false, finance_whatsapp: false, finance_email: true })
    expect(former.eligible).toEqual({ community_whatsapp: false, community_email: false, finance_whatsapp: false, finance_email: false })
    expect(JSON.parse(await execute(owner, 'society_find_contacts', {})).total).toBe(1)
  } finally {
    release(); privateFixture("db.execute(\"UPDATE flat_memberships SET end_date=NULL WHERE resident_id='demo-owner-A-101'\");db.execute(\"DELETE FROM flat_memberships WHERE id='native-contact-shared'\")")
    await ownerContext.close(); await tenantContext.close()
  }
})

test('native contact tools reject extra arguments unbounded reads cancellation and held results from a revoked session', async ({ page }) => {
  await login(page); await expect.poll(() => names(page)).toContain('society_read_contact')
  const invalid = [
    ['society_find_contacts', { page: 0 }], ['society_find_contacts', { page: 10001 }], ['society_find_contacts', { query: 'x'.repeat(101) }],
    ['society_find_contacts', { relationship: 'JOINT' }], ['society_find_contacts', { wing: 1 }], ['society_find_contacts', { state: 'APPROVED' }],
    ['society_find_contacts', { email: 'private@example.test' }], ['society_read_contact', { contact_id: '' }],
    ['society_read_contact', { contact_id: 'x'.repeat(101) }], ['society_read_contact', { contact_id: 'me', action: 'VERIFIED' }], ['society_read_contact', { contact_id: 'me', event_page: 2 }]
  ] as const
  for (const [tool, args] of invalid) await expect(execute(page, tool, args)).rejects.toThrow()
  const cancelled = await page.evaluate(async () => {
    const context = (document as ContextDocument).modelContext, tool = (await context.getTools()).find(item => item.name === 'society_find_contacts')!
    const controller = new AbortController(); controller.abort()
    const major = Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1])
    try { await context.executeTool(tool, major < 155 ? '{}' : {}, { signal: controller.signal }); return false } catch { return true }
  })
  expect(cancelled).toBe(true)
  let release!: () => void, captured!: () => void
  const held = new Promise<void>(resolve => { release = resolve }), ready = new Promise<void>(resolve => { captured = resolve })
  await page.route('**/api/contacts?*', async route => { const response = await route.fetch(); captured(); await held; await route.fulfill({ response }).catch(() => {}) })
  const reading = execute(page, 'society_find_contacts', {}); await ready
  try {
    expect((await page.request.post('/api/auth/logout', { headers: await financialHeaders(page), data: {} })).status()).toBe(200)
    release(); await expect(reading).rejects.toThrow(); await expect.poll(() => names(page)).toEqual([])
    await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
  } finally { release(); await page.unroute('**/api/contacts?*') }
})
