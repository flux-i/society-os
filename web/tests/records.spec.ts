import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { chmodSync, mkdirSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { execFileSync } from 'node:child_process'
import { login, navigate, chooseOption } from './helpers'

async function capture(page: Page, name: string) {
  if (!process.env.SOCIETY_CAPTURE_UI) return
  await page.evaluate(() => document.fonts.ready)
  const folder = resolve('../reports/local/records-review'); mkdirSync(folder, { recursive: true, mode: 0o700 })
  const path = resolve(folder, name + '.png'); await page.screenshot({ path, animations: 'disabled' }); chmodSync(path, 0o600)
}
async function apiEntry(page: Page, description: string, kind = 'RECEIVED', amount = '400.00', home = 'demo-flat-A-101', post = true) {
  const user = await (await page.request.get('/api/auth/me')).json()
  const headers = { Origin: new URL(page.url()).origin, 'X-CSRF-Token': user.csrf_token }
  const response = await page.request.post('/api/entries', { headers, data: { operation_key: crypto.randomUUID(), flat_id: home, kind, amount, date: '2026-01-01', description, payer: kind === 'RECEIVED' ? 'Demo Owner A-101' : '', method: kind === 'RECEIVED' ? 'BANK_TRANSFER' : '', reference: kind === 'RECEIVED' ? 'DEMO-REF-101' : '' } })
  expect(response.status(), await response.text()).toBe(200)
  const result = await response.json()
  if (post) expect((await page.request.post(`/api/entries/${result.id}/post`, { headers, data: { operation_key: crypto.randomUUID(), confirmed: true, reason: '' } })).status()).toBe(200)
  return result.id as string
}
async function fillDraft(page: Page, note: string, kind = 'Money received') {
  await page.getByRole('button', { name: 'Add an entry', exact: true }).click()
  await chooseOption(page, 'Entry home', 'Home A-101')
  await chooseOption(page, 'Entry type', kind)
  await page.getByRole('textbox', { name: 'Amount in rupees', exact: true }).fill('400.01')
  await page.getByLabel('Entry date', { exact: true }).fill('2026-01-01')
  await page.getByRole('textbox', { name: 'Description', exact: true }).fill(note)
  await page.getByRole('textbox', { name: 'Source / evidence note (optional)', exact: true }).fill('Supplied fictional community record')
  if (kind === 'Money received') {
    await page.getByRole('textbox', { name: 'Received from', exact: true }).fill('Demo Owner A-101')
    await chooseOption(page, 'Received via', 'UPI')
    await page.getByRole('textbox', { name: 'Reference', exact: true }).fill('DEMO-UPI-101')
  }
}

test('manual draft review, confirmation, PDF download and linked reversal work through visible controls', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 }); await login(page); await navigate(page, 'Entries')
  await expect(page.getByRole('heading', { name: 'The entry book' })).toBeVisible()
  await capture(page, 'entries-desktop')
  const note = `Browser reviewed receipt ${Date.now()}`
  await fillDraft(page, note)
  await capture(page, 'new-received-desktop')
  await page.getByRole('button', { name: 'Save draft for review', exact: true }).click()
  const dialog = page.getByRole('dialog'); await expect(dialog.getByRole('heading', { name: 'Review this draft.' })).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Confirm this entry', exact: true })).toBeDisabled()
  await expect(dialog.getByText('₹400.01', { exact: true })).toBeVisible()
  await capture(page, 'draft-review-desktop')
  await dialog.getByRole('checkbox').check(); await dialog.getByRole('button', { name: 'Confirm this entry', exact: true }).click()
  await expect(dialog.getByRole('button', { name: 'Download PDF', exact: true })).toBeEnabled({ timeout: 10000 })
  await capture(page, 'receipt-ready-desktop')
  const downloading = page.waitForEvent('download'); await dialog.getByRole('button', { name: 'Download PDF', exact: true }).click()
  const download = await downloading; expect(download.suggestedFilename()).toMatch(/^SOS-\d{4}-\d{6}\.pdf$/)
  const path = await download.path(); expect(path).not.toBeNull(); expect(readFileSync(path!).subarray(0, 5).toString()).toBe('%PDF-')
  if (process.env.SOCIETY_CAPTURE_UI) { const folder = resolve('../reports/local/records-review'); const dest = resolve(folder, 'reviewed-receipt.pdf'); await download.saveAs(dest); chmodSync(dest, 0o600) }
  await dialog.getByRole('button', { name: 'Correct this entry', exact: true }).click()
  await dialog.getByRole('textbox', { name: 'Reason for reversal', exact: true }).fill('Fictional supplied reference was wrong')
  await dialog.getByRole('button', { name: 'Keep entry', exact: true }).click()
  await expect(dialog.getByText('This entry has been reversed.', { exact: true })).toHaveCount(0)
  await dialog.getByRole('button', { name: 'Correct this entry', exact: true }).click()
  await dialog.getByRole('checkbox').check(); await dialog.getByRole('button', { name: 'Confirm reversal', exact: true }).click()
  await expect(dialog.getByText('This entry has been reversed.', { exact: true })).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Download original', exact: true })).toBeEnabled()
  await capture(page, 'reversal-desktop')
  await page.getByRole('button', { name: 'Close entry details', exact: true }).click()
  await navigate(page, 'Receipts'); await page.getByRole('searchbox', { name: 'Search records', exact: true }).fill(note)
  await expect(page.getByRole('button', { name: /^Open SOS-/ })).toHaveCount(1)
  await chooseOption(page, 'Filter records by status', 'Reversed')
  await capture(page, 'receipt-collection-reversed')
})

test('lost create and confirm responses retry with their original identities; pending dialogs cannot close', async ({ page }) => {
  await login(page); await navigate(page, 'Entries')
  const note = `Lost response record ${Date.now()}`; await fillDraft(page, note, 'Given charge')
  let createCalls = 0; const keys: string[] = []
  await page.route('**/api/entries', async route => {
    if (route.request().method() !== 'POST') { await route.continue(); return }
    createCalls++; keys.push(route.request().postDataJSON().operation_key)
    if (createCalls === 1) { await route.fetch(); await route.abort('failed') } else await route.continue()
  })
  await page.getByRole('button', { name: 'Save draft for review', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Retry saving this draft', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Retry saving this draft', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Review this draft.' })).toBeVisible()
  expect(createCalls).toBe(2); expect(keys[0]).toBe(keys[1])
  let postCalls = 0; const postKeys: string[] = []; let release!: () => void
  await page.route('**/api/entries/*/post', async route => {
    postCalls++; postKeys.push(route.request().postDataJSON().operation_key)
    if (postCalls === 1) { await new Promise<void>(resolve => { release = resolve }); await route.fetch(); await route.abort('failed') } else await route.continue()
  })
  await page.getByRole('dialog').getByRole('checkbox').check(); await page.getByRole('button', { name: 'Confirm this entry', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Close entry details', exact: true })).toBeDisabled()
  await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toBeVisible()
  await expect.poll(() => typeof release).toBe('function'); release()
  await expect(page.getByRole('alert')).toBeVisible()
  await page.getByRole('button', { name: 'Confirm this entry', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'A clear record.' })).toBeVisible()
  expect(postCalls).toBe(2); expect(postKeys[0]).toBe(postKeys[1])
  await page.getByRole('button', { name: 'Close entry details', exact: true }).click()
  await page.getByRole('searchbox', { name: 'Search records', exact: true }).fill(note)
  await expect(page.getByRole('button', { name: new RegExp('^Open ' + note) })).toHaveCount(1)
})

test('new record controls, opened menus, keyboard, validation and scrolling fit desktop, tablet and small phones', async ({ page }) => {
  await login(page); await navigate(page, 'Entries')
  for (const viewport of [{ width: 1440, height: 900 }, { width: 768, height: 1024 }, { width: 375, height: 812 }, { width: 320, height: 568 }]) {
    await page.setViewportSize(viewport); await capture(page, `entries-${viewport.width}`)
    const home = page.getByRole('combobox', { name: 'Filter records by home', exact: true }); await home.click()
    await expect(page.getByRole('option', { name: 'Home A-101', exact: true })).toBeVisible()
    await capture(page, `home-filter-${viewport.width}`); await page.keyboard.press('Escape'); await expect(home).toBeFocused()
    await chooseOption(page, 'Filter records by status', 'Draft'); await expect(page.getByRole('combobox', { name: 'Filter records by status', exact: true })).toHaveText('Draft')
    await page.getByRole('button', { name: 'Clear', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Add an entry', exact: true })).toBeEnabled()
    await page.getByRole('button', { name: 'Add an entry', exact: true }).click()
    await page.getByRole('button', { name: 'Save draft for review', exact: true }).click()
    await expect(page.getByRole('combobox', { name: 'Entry home', exact: true })).toBeFocused()
    await chooseOption(page, 'Entry home', 'Home C-1002')
    const homeTrigger = page.getByRole('combobox', { name: 'Entry home', exact: true }); await homeTrigger.click()
    await expect(page.getByRole('option', { name: 'Home C-1002', exact: true })).toHaveAttribute('aria-selected', 'true')
    await capture(page, `entry-home-menu-${viewport.width}`); await page.keyboard.press('Escape')
    await chooseOption(page, 'Entry type', 'Opening credit'); await expect(page.getByRole('textbox', { name: 'Received from', exact: true })).toHaveCount(0)
    await chooseOption(page, 'Entry type', 'Opening amount due'); await chooseOption(page, 'Entry type', 'Given charge'); await chooseOption(page, 'Entry type', 'Money received')
    const method = page.getByRole('combobox', { name: 'Received via', exact: true }); await method.click()
    await page.getByRole('option', { name: 'Cash', exact: true }).hover(); await capture(page, `entry-method-menu-${viewport.width}`)
    await page.getByRole('option', { name: 'Cash', exact: true }).click(); await expect(page.getByRole('textbox', { name: 'Reference (optional)', exact: true })).toBeVisible()
    await chooseOption(page, 'Received via', 'Cheque')
    await expect(page.getByRole('textbox', { name: 'Reference', exact: true })).toHaveAttribute('required', '')
    await page.getByRole('dialog').evaluate(element => { const scroller = element.querySelector('.dialog-scroll')!; scroller.scrollTop = scroller.scrollHeight })
    await expect(page.getByRole('button', { name: 'Save draft for review', exact: true })).toBeInViewport()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Add an entry', exact: true })).toBeFocused()
  }
})

test('record list failures retry, download failures recover, and zero amounts can be corrected', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 }); await login(page)
  let failed = false
  await page.route('**/api/entries?*', async route => { if (!failed) { failed = true; await route.fulfill({ status: 503, contentType: 'application/json', body: '{}' }) } else await route.continue() })
  await navigate(page, 'Entries'); await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeVisible()
  await capture(page, 'list-error-phone'); await page.getByRole('button', { name: 'Try again', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Add an entry', exact: true })).toBeEnabled()
  await fillDraft(page, `Corrected validation ${Date.now()}`)
  await page.getByRole('textbox', { name: 'Amount in rupees', exact: true }).fill('0.00')
  await page.getByRole('button', { name: 'Save draft for review', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('above zero')
  await expect(page.getByRole('textbox', { name: 'Amount in rupees', exact: true })).toBeEnabled()
  await page.getByRole('textbox', { name: 'Amount in rupees', exact: true }).fill('1500.50')
  await page.getByRole('button', { name: 'Save draft for review', exact: true }).click()
  await page.getByRole('dialog').getByRole('checkbox').check(); await page.getByRole('button', { name: 'Confirm this entry', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Download PDF', exact: true })).toBeEnabled({ timeout: 10000 })
  await capture(page, 'receipt-ready-phone')
  await page.route('**/api/receipts/*/download', route => route.fulfill({ status: 503 }))
  await page.getByRole('button', { name: 'Download PDF', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('could not be downloaded')
  await expect(page.getByRole('alert')).toBeInViewport()
  await capture(page, 'download-error-phone')
  await page.unroute('**/api/receipts/*/download')
  const downloaded = page.waitForEvent('download'); await page.getByRole('button', { name: 'Download PDF', exact: true }).click(); await downloaded
  await expect(page.getByRole('alert')).toHaveCount(0)
})

test('residents see only financially permitted confirmed records; tenants cannot open entries or PDFs', async ({ page }) => {
  await login(page)
  const id = await apiEntry(page, `Resident scope ${Date.now()}`, 'RECEIVED', '10.02', 'demo-flat-A-102')
  const alien = await apiEntry(page, `Alien record ${Date.now()}`, 'RECEIVED', '99.00', 'demo-flat-B-101')
  await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await login(page, 'Owner')
  await navigate(page, 'Entries'); await expect(page.getByRole('button', { name: 'Add an entry', exact: true })).toHaveCount(0)
  const detail = await page.request.get(`/api/entries/${id}`); expect(detail.status()).toBe(200)
  const entry = await detail.json(); expect((await page.request.get(`/api/entries/${alien}`)).status()).toBe(404)
  await page.getByRole('button', { name: /^Open Resident scope/ }).click()
  await expect(page.getByRole('button', { name: 'Correct this entry', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Download PDF', exact: true })).toBeEnabled({ timeout: 10000 })
  await capture(page, 'resident-receipt')
  await page.getByRole('button', { name: 'Close entry details', exact: true }).click()
  await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await login(page, 'Tenant')
  await expect(page.getByRole('link', { name: 'Entries', exact: true })).toHaveCount(0)
  expect((await page.request.get('/api/entries')).status()).toBe(403)
  expect((await page.request.get(`/api/receipts/${entry.receipt_id}/download`)).status()).toBe(403)
  await page.goto('/#entries'); await expect(page.getByRole('heading', { name: 'These records need access.' })).toBeVisible()
  await page.getByRole('link', { name: 'Return to your homes', exact: true }).click(); await expect(page.getByRole('heading', { name: 'Your homes', exact: true })).toBeVisible()
})

test('record pagination opens every matched draft and clamps when a filter reduces the page count', async ({ page }) => {
  await login(page)
  const marker = 'Paging ' + Date.now()
  for (let i = 0; i < 13; i++) await apiEntry(page, marker + ' ' + i, 'CHARGE', '0.01', 'demo-flat-C-1001', false)
  await navigate(page, 'Entries'); await page.getByRole('searchbox', { name: 'Search records', exact: true }).fill(marker)
  await expect(page.getByRole('status')).toHaveText('13 entries found')
  for (let p = 1; p <= 2; p++) {
    const rows = page.getByRole('button', { name: new RegExp('^Open ' + marker) }); await expect(rows).toHaveCount(p === 1 ? 12 : 1); const count = await rows.count()
    expect(count).toBe(p === 1 ? 12 : 1)
    for (let index = 0; index < count; index++) { await rows.nth(index).click(); await expect(page.getByRole('heading', { name: 'Review this draft.' })).toBeVisible(); await page.getByRole('button', { name: 'Close entry details', exact: true }).click() }
    if (p === 1) await page.getByRole('button', { name: 'Next records page', exact: true }).click()
  }
  await expect(page.getByRole('button', { name: 'Next records page', exact: true })).toBeDisabled()
  await chooseOption(page, 'Filter records by status', 'Confirmed'); await expect(page.getByRole('status')).toHaveText('0 entries found')
  await page.getByRole('button', { name: 'Clear filters', exact: true }).click(); await expect(page.getByRole('status')).not.toHaveText('0 entries found')
})

test('failed PDF retry keeps the saved entry and receipt identity and regenerates through the worker', async ({ page }) => {
  await login(page)
  const note = `Retry receipt ${Date.now()}`
  const id = await apiEntry(page, note)
  await expect.poll(async () => (await (await page.request.get(`/api/entries/${id}`)).json()).pdf_state).toBe('READY')
  const before = await (await page.request.get(`/api/entries/${id}`)).json()
  const isolated = process.env.SOCIETY_BROWSER_DB
  expect(isolated).toContain('society-browser-')
  // An explicit synthetic worker-failure fixture, never the user's preview database.
  execFileSync('python3', ['-c', 'import sqlite3,sys; c=sqlite3.connect(sys.argv[1]); c.execute("UPDATE receipt_jobs SET state=\'FAILED\',attempts=5 WHERE receipt_id=?",(sys.argv[2],)); c.commit(); c.close()', isolated!, before.receipt_id])
  await navigate(page, 'Receipts'); await page.getByRole('searchbox', { name: 'Search records', exact: true }).fill(note)
  await page.getByRole('button', { name: /^Open SOS-/ }).click()
  await expect(page.getByRole('button', { name: 'Retry PDF', exact: true })).toBeEnabled()
  await capture(page, 'pdf-failed-desktop')
  await page.getByRole('button', { name: 'Retry PDF', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Download PDF', exact: true })).toBeEnabled({ timeout: 10000 })
  const after = await (await page.request.get(`/api/entries/${id}`)).json()
  expect(after.receipt_id).toBe(before.receipt_id); expect(after.receipt_number).toBe(before.receipt_number); expect(after.amount_paise).toBe(40000)
  await capture(page, 'pdf-retried-desktop')
})

test('a reviewed draft can be discarded without changing balances or issuing a receipt', async ({ page }) => {
  await login(page); await navigate(page, 'Entries')
  const before = await (await page.request.get('/api/entries')).json()
  const note = 'Discard reviewed draft ' + Date.now()
  await fillDraft(page, note)
  await page.getByRole('button', { name: 'Save draft for review', exact: true }).click()
  await page.getByRole('button', { name: 'Discard this draft', exact: true }).click()
  await page.getByRole('textbox', { name: 'Reason for discarding draft', exact: true }).fill('Fictional amount supplied incorrectly')
  await page.getByRole('dialog').getByRole('checkbox').check()
  await page.getByRole('button', { name: 'Confirm discard', exact: true }).click()
  await expect(page.getByText('This draft has been discarded.', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Confirm this entry', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Download PDF', exact: true })).toHaveCount(0)
  const after = await (await page.request.get('/api/entries')).json(); expect(after.balance_paise).toBe(before.balance_paise); expect(after.drafts).toBe(before.drafts)
  await capture(page, 'discarded-draft-desktop')
  await page.getByRole('button', { name: 'Close entry details', exact: true }).click()
  await page.getByRole('searchbox', { name: 'Search records', exact: true }).fill(note)
  await chooseOption(page, 'Filter records by status', 'Discarded')
  await expect(page.getByRole('status')).toHaveText('1 entries found')
})
