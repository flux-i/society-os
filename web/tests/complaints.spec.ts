import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { mkdirSync, chmodSync } from 'node:fs'
import { resolve } from 'node:path'
import { login, navigate, chooseOption } from './helpers'

async function capture(page: Page, name: string) {
  if (!process.env.SOCIETY_CAPTURE_UI) return
  await page.evaluate(() => document.fonts.ready)
  const folder = resolve('../reports/local/complaints-review'); mkdirSync(folder, { recursive: true, mode: 0o700 })
  const path = resolve(folder, name + '.png'); await page.screenshot({ path, animations: 'disabled' }); chmodSync(path, 0o600)
}
async function desk(page: Page) { await navigate(page, 'Help & repairs'); await expect(page.getByRole('heading', { name: 'Little things, well taken care of.' })).toBeVisible() }
async function close(page: Page) { await page.getByRole('button', { name: 'Close service request details', exact: true }).click() }
async function switchTo(page: Page, account: string) { if (await page.getByRole('button', { name: 'Close service request details', exact: true }).count()) await close(page); await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await login(page, account) }
async function report(page: Page, subject: string) {
  await desk(page); await page.getByRole('button', { name: 'Report an issue', exact: true }).click()
  await chooseOption(page, 'Service request home', 'Home A-101'); await chooseOption(page, 'Service category', 'Plumbing')
  await page.getByRole('textbox', { name: 'Subject', exact: true }).fill(subject)
  await page.getByRole('textbox', { name: 'Details', exact: true }).fill('A fictional slow leak in the kitchen needs a handler and a clear progress update.')
}
async function saveReport(page: Page) {
  const response = page.waitForResponse(response => response.url().endsWith('/api/complaints') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Save service request', exact: true }).click()
  const result = await response; expect(result.status()).toBe(200)
  const id = (await result.json()).id as string
  await expect(page.getByRole('dialog').locator('.case-status')).toHaveText('Open'); return id
}
async function openCase(page: Page, subject: string) { await desk(page); await page.getByRole('searchbox', { name: 'Search service requests' }).fill(subject); await page.getByRole('button', { name: 'Open case ' + subject, exact: true }).click(); await expect(page.getByRole('dialog').getByRole('heading', { name: subject, exact: true })).toBeVisible() }
async function saveUpdate(page: Page, message: string) {
  await page.getByRole('textbox', { name: 'Service update message' }).fill(message)
  const response = page.waitForResponse(response => /\/api\/complaints\/[^/]+\/updates$/.test(response.url()) && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Save update', exact: true }).click()
  expect((await response).status()).toBe(200)
  await expect(page.getByRole('region', { name: 'Service request conversation' }).getByText(message, { exact: true })).toBeVisible()
}
async function state(page: Page, value: string, message: string, staff = true) { await chooseOption(page, 'Service update type', staff ? 'Change the status' : 'Close or reopen after resolution'); await chooseOption(page, 'Next service status', value); await saveUpdate(page, message) }
async function fixture(page: Page, subject: string) {
  const me = await (await page.request.get('/api/auth/me')).json()
  const response = await page.request.post('/api/complaints', { headers: { Origin: new URL(page.url()).origin, 'X-CSRF-Token': me.csrf_token }, data: { operation_key: crypto.randomUUID(), flat_id: 'demo-flat-A-101', category: 'PLUMBING', subject, description: 'A synthetic service request used for a bounded browser scenario.', priority: 'NORMAL' } })
  expect(response.status()).toBe(200); return (await response.json()).id as string
}
async function apiUpdate(page: Page, id: string, version: number, message: string, visibility = 'RESIDENT_VISIBLE') {
  const me = await (await page.request.get('/api/auth/me')).json()
  const response = await page.request.post('/api/complaints/' + id + '/updates', { headers: { Origin: new URL(page.url()).origin, 'X-CSRF-Token': me.csrf_token }, data: { operation_key: crypto.randomUUID(), version, action: 'COMMENT', message, visibility, status: '', assigned_to: '', priority: '' } })
  expect(response.status()).toBe(200)
}

test('a resident reports, a handler assigns and resolves, and the resident closes and reopens through visible controls', async ({ page }) => {
  await login(page, 'Owner'); await report(page, 'CARE A leaking kitchen pipe'); const id = await saveReport(page)
  await expect(page.getByRole('combobox', { name: 'Update visibility' })).toHaveCount(0); await capture(page, 'owner-open-case')
  await switchTo(page, 'Registry officer'); await openCase(page, 'CARE A leaking kitchen pipe')
  await chooseOption(page, 'Service update type', 'Assign a handler'); await chooseOption(page, 'Service handler', 'Demo Committee Member'); await saveUpdate(page, 'Committee member will coordinate a fictional visit')
  await expect(page.getByRole('definition').filter({ hasText: 'Demo Committee Member' })).toBeVisible()
  await state(page, 'Acknowledged', 'We have read the report and will coordinate the repair')
  await state(page, 'In progress', 'A fictional maintenance visit is now in progress')
  await chooseOption(page, 'Service update type', 'Change the priority'); await chooseOption(page, 'Updated service priority', 'High'); await saveUpdate(page, 'The leak now needs a sooner maintenance visit')
  await state(page, 'Waiting', 'Waiting for a replacement fitting to arrive')
  await state(page, 'In progress', 'The fitting is here and the repair continues')
  await state(page, 'Resolved', 'The new fitting is installed and the leak has stopped'); await capture(page, 'handler-resolved-case')
  await switchTo(page, 'Owner'); await page.goto('/#help?case=' + id)
  await expect(page.getByRole('dialog').locator('.case-status')).toHaveText('Resolved')
  await state(page, 'Closed', 'I checked the repair and confirm this can close', false)
  await expect(page.getByRole('dialog').locator('.case-status')).toHaveText('Closed')
  await chooseOption(page, 'Next service status', 'Open'); await saveUpdate(page, 'The leak came back, so please reopen this case')
  await expect(page.getByRole('dialog').locator('.case-status')).toHaveText('Open'); await capture(page, 'owner-reopened-case')
  const item = await (await page.request.get('/api/complaints/' + id)).json(); expect(item.history_total).toBe(10); expect(item.version).toBe(10)
})

test('staff notes stay private in resident conversation, search, counts and unrelated-case URLs', async ({ page }) => {
  await login(page, 'Owner'); await report(page, 'CARE Private water inspection'); const id = await saveReport(page)
  await switchTo(page, 'Registry officer'); await openCase(page, 'CARE Private water inspection')
  await chooseOption(page, 'Update visibility', 'Handling team only'); await saveUpdate(page, 'STAFF_SECRET_Fictional private contractor coordination')
  await expect(page.locator('.staff-note-label')).toHaveText('Staff only')
  await page.locator('.staff-note').evaluate(element => { const scroll = element.closest<HTMLElement>('.dialog-scroll')!; const target = element.getBoundingClientRect(), area = scroll.getBoundingClientRect(); scroll.scrollTop += target.top - area.top - Math.max(0, (area.height - target.height) / 2) })
  await expect(page.locator('.staff-note')).toBeInViewport({ ratio: 1 }); await expect(page.getByRole('button', { name: 'Close service request details' })).toBeInViewport({ ratio: 1 }); await capture(page, 'handler-private-note')
  await saveUpdate(page, 'The next visit will be discussed with the resident')
  await switchTo(page, 'Owner'); await openCase(page, 'CARE Private water inspection')
  await expect(page.getByText('STAFF_SECRET_Fictional private contractor coordination')).toHaveCount(0)
  await expect(page.getByRole('region', { name: 'Service request conversation' }).locator('li')).toHaveCount(2); await capture(page, 'owner-public-conversation')
  const item = await (await page.request.get('/api/complaints/' + id)).json(); expect(item.history_total).toBe(2); expect(JSON.stringify(item)).not.toContain('STAFF_SECRET')
  await close(page); await page.getByRole('searchbox', { name: 'Search service requests' }).fill('STAFF_SECRET'); await expect(page.getByRole('status')).toHaveText('0 cases found')
  await switchTo(page, 'Tenant'); expect((await page.request.get('/api/complaints/' + id)).status()).toBe(404)
  await desk(page); await page.getByRole('searchbox', { name: 'Search service requests' }).fill('CARE Private water inspection'); await expect(page.getByRole('status')).toHaveText('0 cases found')
})

test('service forms, opened menus, required validation and long content fit desktop, tablet and small phones', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' }); await login(page, 'Owner')
  for (const viewport of [{ width: 1440, height: 900 }, { width: 768, height: 1024 }, { width: 375, height: 812 }, { width: 320, height: 568 }]) {
    await page.setViewportSize(viewport); await desk(page); await capture(page, `care-${viewport.width}-page`)
    await page.getByRole('combobox', { name: 'Filter service requests by status' }).click(); await expect(page.getByRole('listbox')).toBeInViewport({ ratio: 1 }); await capture(page, `care-${viewport.width}-status-menu`); await page.keyboard.press('Escape')
    await page.getByRole('button', { name: 'Report an issue', exact: true }).click()
    await page.getByRole('textbox', { name: 'Subject', exact: true }).fill('CARE A long request ' + viewport.width)
    await page.getByRole('textbox', { name: 'Details', exact: true }).fill('Fictional request details. '.repeat(100))
    await page.getByRole('button', { name: 'Save service request', exact: true }).click(); await expect(page.getByRole('combobox', { name: 'Service request home' })).toBeFocused()
    await page.keyboard.press('Enter'); await expect(page.getByRole('listbox')).toBeInViewport({ ratio: 1 }); await capture(page, `care-${viewport.width}-home-menu`)
    await page.getByRole('option', { name: 'Home A-102', exact: true }).click()
    await page.getByRole('combobox', { name: 'Service category' }).click(); await expect(page.getByRole('listbox')).toBeInViewport({ ratio: 1 }); await capture(page, `care-${viewport.width}-category-menu`)
    await page.keyboard.press('End'); await expect(page.getByRole('option', { name: 'Something else', exact: true })).toBeFocused(); await page.keyboard.press('Enter')
    await page.getByRole('combobox', { name: 'Service priority' }).click(); await expect(page.getByRole('listbox')).toBeInViewport({ ratio: 1 }); await capture(page, `care-${viewport.width}-priority-menu`)
    await page.getByRole('option', { name: 'Urgent', exact: true }).click(); await expect(page.getByText('Contact security or emergency services directly for urgent safety issues.', { exact: false })).toBeVisible()
    const widths = await page.locator('.records-fieldset .select-field').evaluateAll(elements => elements.map(el => ({ field: el.getBoundingClientRect().width, trigger: el.querySelector('button')!.getBoundingClientRect().width })))
    expect(widths.every(item => Math.abs(item.field - item.trigger) < 2)).toBe(true)
    expect(await page.getByRole('dialog').evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true)
    await page.keyboard.press('Escape'); await expect(page.getByRole('button', { name: 'Report an issue', exact: true })).toBeFocused()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  }
})

test('lost report and update responses preserve one identity and busy dialogs stay open', async ({ page }) => {
  await login(page, 'Owner'); await report(page, 'CARE Retried report identity')
  const reports: string[] = []
  await page.route('**/api/complaints', async route => { if (route.request().method() !== 'POST') return route.continue(); reports.push(route.request().postDataJSON().operation_key); await route.fetch(); await route.abort() })
  await page.getByRole('button', { name: 'Save service request', exact: true }).click(); await expect(page.getByRole('alert')).toBeVisible(); await expect(page.getByRole('button', { name: 'Close service request form', exact: true })).toBeInViewport({ ratio: 1 }); expect(await page.getByRole('dialog').evaluate(el => el.scrollTop)).toBe(0); await capture(page, 'care-lost-report-response')
  await page.unroute('**/api/complaints'); page.on('request', request => { if (request.method() === 'POST' && request.url().endsWith('/api/complaints')) reports.push(request.postDataJSON().operation_key) })
  await page.getByRole('button', { name: 'Retry this report', exact: true }).click(); await expect(page.getByRole('dialog').locator('.case-status')).toHaveText('Open'); expect(reports).toHaveLength(2); expect(reports[0]).toBe(reports[1])
  await page.getByRole('textbox', { name: 'Service update message' }).fill('Retried conversation update must appear exactly once')
  const updates: string[] = []; let release!: () => void; const hold = new Promise<void>(resolve => { release = resolve })
  await page.route('**/api/complaints/*/updates', async route => { updates.push(route.request().postDataJSON().operation_key); await route.fetch(); await hold; await route.abort() })
  await page.getByRole('button', { name: 'Save update', exact: true }).click(); await expect(page.getByRole('button', { name: 'Saving your update…' })).toBeDisabled(); await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toBeVisible()
  release(); await expect(page.getByRole('alert')).toBeVisible(); await page.unroute('**/api/complaints/*/updates')
  page.on('request', request => { if (request.method() === 'POST' && /\/api\/complaints\/[^/]+\/updates$/.test(request.url())) updates.push(request.postDataJSON().operation_key) })
  await page.getByRole('button', { name: 'Retry this update', exact: true }).click(); await expect(page.getByRole('region', { name: 'Service request conversation' }).locator('li')).toHaveCount(2); expect(updates).toHaveLength(2); expect(updates[0]).toBe(updates[1])
})

test('phone list/detail failures retry and stale updates reload instead of overwriting another handler', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 568 }); await login(page, 'Owner'); const id = await fixture(page, 'CARE A stale conversation')
  await page.route('**/api/complaints?**', route => route.fulfill({ status: 503, json: { error: 'unavailable' } }))
  await desk(page); await expect(page.getByRole('alert')).toBeInViewport({ ratio: 1 }); await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeInViewport({ ratio: 1 }); await capture(page, 'care-phone-list-error')
  await page.unroute('**/api/complaints?**'); await page.getByRole('button', { name: 'Try again', exact: true }).click()
  await page.route('**/api/complaints/' + id + '?**', route => route.fulfill({ status: 503, json: { error: 'unavailable' } }))
  await page.getByRole('button', { name: 'Open case CARE A stale conversation' }).click(); await expect(page.getByRole('button', { name: 'Reload service request', exact: true })).toBeVisible()
  await page.unroute('**/api/complaints/' + id + '?**'); await page.getByRole('button', { name: 'Reload service request', exact: true }).click()
  await page.getByRole('textbox', { name: 'Service update message' }).fill('A stale message from this older conversation view')
  await apiUpdate(page, id, 1, 'A completed update wins before the stale browser message')
  await page.getByRole('button', { name: 'Save update', exact: true }).click(); await expect(page.getByRole('alert')).toBeInViewport({ ratio: 1 }); await expect(page.getByRole('button', { name: 'Close service request details', exact: true })).toBeInViewport({ ratio: 1 }); expect(await page.getByRole('dialog').evaluate(el => el.scrollTop)).toBe(0); await capture(page, 'care-phone-stale-update')
  await page.getByRole('button', { name: 'Reload latest details', exact: true }).click(); await expect(page.getByRole('region', { name: 'Service request conversation' }).locator('li')).toHaveCount(2)
  await saveUpdate(page, 'A deliberate message sent after loading the newer conversation')
})

test('case pagination clamps after filters and conversation pages retain the older visible messages', async ({ page }) => {
  await login(page, 'Owner'); const ids: string[] = []
  for (let i = 0; i < 14; i++) ids.push(await fixture(page, 'CARE PAGE Fixture ' + i))
  await desk(page); await page.getByRole('searchbox', { name: 'Search service requests' }).fill('CARE PAGE Fixture'); await expect(page.getByRole('status')).toHaveText('14 cases found')
  const visited = new Set<string>(); for (const current of [1, 2]) {
    if (current === 2) { const response = page.waitForResponse(response => response.url().includes('/api/complaints?') && response.url().includes('page=2')); await page.getByRole('button', { name: 'Next service requests page' }).click(); await response }
    const buttons = page.getByRole('button', { name: /^Open case CARE PAGE Fixture/ }); for (let i = 0; i < await buttons.count(); i++) { const name = await buttons.nth(i).getAttribute('aria-label'); visited.add(name!); await buttons.nth(i).click(); await expect(page.getByRole('dialog')).toBeVisible(); await close(page) }
  }
  expect(visited.size).toBe(14)
  await page.getByRole('searchbox', { name: 'Search service requests' }).fill('CARE PAGE Fixture 13'); await expect(page.getByText('Page 1 of 1')).toBeVisible()
  const id = ids[13]; for (let version = 1; version <= 32; version++) await apiUpdate(page, id, version, 'Visible history message ' + version)
  await page.getByRole('button', { name: 'Open case CARE PAGE Fixture 13', exact: true }).click()
  await expect(page.getByRole('region', { name: 'Service request conversation' }).locator('li')).toHaveCount(30)
  await page.getByRole('button', { name: 'Older conversation updates' }).click(); await expect(page.getByRole('region', { name: 'Service request conversation' }).locator('li')).toHaveCount(3)
  await expect(page.getByText('Visible history message 1', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Newer conversation updates' }).click(); await expect(page.getByRole('region', { name: 'Service request conversation' }).locator('li')).toHaveCount(30)
})

test('handler decision controls and menus work in a narrow scrolling dialog', async ({ page }) => {
  await login(page, 'Owner'); const id = await fixture(page, 'CARE Handler controls on a phone')
  await switchTo(page, 'Registry officer'); await page.setViewportSize({ width: 320, height: 568 }); await page.goto('/#help?case=' + id)
  for (const [type, label, target] of [['Assign a handler', 'Service handler', 'Demo Committee Member'], ['Change the priority', 'Updated service priority', 'Urgent'], ['Change the status', 'Next service status', 'Acknowledged']]) {
    await page.getByRole('combobox', { name: 'Service update type' }).click(); await expect(page.getByRole('listbox')).toBeInViewport({ ratio: 1 }); await page.getByRole('option', { name: type, exact: true }).click()
    await page.getByRole('combobox', { name: label, exact: true }).click(); await expect(page.getByRole('listbox')).toBeInViewport({ ratio: 1 }); await capture(page, `care-phone-${label.replaceAll(' ', '-').toLowerCase()}`); await page.getByRole('option', { name: target, exact: true }).click()
    await saveUpdate(page, 'Phone handler decision: ' + type)
  }
  await page.getByRole('combobox', { name: 'Update visibility' }).click(); await expect(page.getByRole('listbox')).toBeInViewport({ ratio: 1 }); await capture(page, 'care-phone-note-visibility'); await page.keyboard.press('Escape')
  expect(await page.getByRole('dialog').evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true)
})
