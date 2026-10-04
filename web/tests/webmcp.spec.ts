import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { login, navigate, completePreviewMFA, chooseOption } from './helpers'

type ContextDocument = Document & { modelContext: {
  getTools: () => Promise<{ name: string }[]>
  executeTool: (tool: { name: string }, args: unknown, options?: { signal: AbortSignal }) => Promise<string>
} }
async function names(page: Page) {
  return page.evaluate(async () => (await (document as ContextDocument).modelContext.getTools()).map(tool => tool.name))
}
async function execute(page: Page, name: string, args: unknown) {
  return page.evaluate(async ({ name, args }) => {
    const context = (document as ContextDocument).modelContext
    const tool = (await context.getTools()).find(item => item.name === name)
    if (!tool) throw new Error('Expected registered native tool: ' + name)
    // Chrome 154 accepts JSON strings; Chrome 155 changed this to objects.
    const major = Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1])
    return context.executeTool(tool, major < 155 ? JSON.stringify(args) : args)
  }, { name, args })
}

test('native WebMCP discovers permitted tools after MFA and executes bounded searches', async ({ page }) => {
  await page.goto('/')
  expect(await page.evaluate(() => !!(document as ContextDocument).modelContext)).toBe(true)
  expect(await names(page)).toEqual([])
  await login(page)
  await expect.poll(() => names(page)).toEqual(['society_find_complaints', 'society_find_homes', 'society_find_notices', 'society_find_records', 'society_find_requests', 'society_open_home', 'society_open_workspace', 'society_read_complaint'])
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
  await expect.poll(() => names(page)).toEqual(['society_find_complaints', 'society_find_homes', 'society_find_notices', 'society_find_requests', 'society_open_home', 'society_open_workspace', 'society_read_complaint'])
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
  expect((await names(page)).some(name => /approve|close|update|assign/.test(name))).toBe(false)
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
