import { test, expect } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { chooseOption } from './helpers'
import { pageFits, dialogFits } from './registry-import-fixtures'
import { captureReview, loginReviewer, reviewDB, reviewHost } from './recovery-review-fixtures'
import { addUnknownGapItem, fillGap, fillGapDecision } from './recovery-gap-fixtures'

test.describe.configure({ mode: 'serial' })
test.setTimeout(120000)
test.use({ actionTimeout: 10000 })
let host: Awaited<ReturnType<typeof reviewHost>>, preparationID: string
const path = '/api/recovery/financial-gap/preparations'
test.beforeAll(async ({ browser }) => { host = await reviewHost(browser); await loginReviewer(host.one); await loginReviewer(host.two) })
test.afterAll(async () => { await host?.close() })
function originalsHeld() {
  expect(reviewDB(host.target, 'SELECT amount_paise FROM entries WHERE id IN(SELECT entry_id FROM receipts)')[0][0]).toBe(40000)
  expect(reviewDB(host.target, "SELECT SUM(CASE WHEN kind IN('OPENING_DEBIT','CHARGE') THEN amount_paise ELSE -amount_paise END) FROM entries WHERE state='POSTED' AND id NOT IN(SELECT entry_id FROM entry_reversals)")[0][0]).toBe(150001)
  expect(reviewDB(host.target, 'SELECT required FROM recovery_quarantine')[0][0]).toBe(1)
}

test('actual financial empty view and shared kind menu fit desktop tablet both small phones and a short dialog', async () => {
  const page = host.one.page
  await page.getByRole('button', { name: 'Financial comparison', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Room for a careful comparison.', exact: true })).toBeVisible()
  for (const [width, height] of [[1440, 1000], [820, 1050], [375, 812], [320, 812], [320, 440]]) {
    await page.setViewportSize({ width, height }); await pageFits(page)
    await page.getByRole('button', { name: 'Prepare financial assessment', exact: true }).scrollIntoViewIfNeeded()
    await captureReview(page, `gap-empty-${width}-${height}`)
    await page.getByRole('button', { name: 'Prepare financial assessment', exact: true }).click(); await dialogFits(page)
    const overflow = await page.locator('dialog .gap-item').evaluate(element => element.scrollWidth - element.clientWidth)
    expect(overflow, `Every supplied item must fit its dialog without clipping at ${width}px`).toBeLessThanOrEqual(1)
    const kind = page.getByRole('combobox', { name: 'Item 1 kind', exact: true })
    await kind.focus(); await page.keyboard.press('Enter')
    await expect(page.getByRole('option', { name: 'Evidence only', exact: true })).toBeVisible()
    await page.getByRole('option', { name: 'Evidence only', exact: true }).hover(); await captureReview(page, `gap-kind-menu-${width}-${height}`)
    await page.keyboard.press('Escape'); await expect(kind).toBeFocused(); await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0)
  }
})

test('exact independent comparison retains unknown evidence and resolves one lost reply with actual fresh identity', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 320, height: 440 })
  await page.getByRole('button', { name: 'Prepare financial assessment', exact: true }).click(); await fillGap(page); await addUnknownGapItem(page)
  await page.getByRole('button', { name: 'Add supplied item', exact: true }).click(); await expect(page.getByLabel('Item 3 book reference', { exact: true })).toBeFocused()
  await page.getByRole('button', { name: 'Remove item 3', exact: true }).click(); await expect(page.getByLabel('Item 2 book reference', { exact: true })).toBeFocused()
  const preview = page.getByRole('article', { name: 'A-101 comparison preview', exact: true })
  await expect(preview).toContainText('₹1,500.01'); await expect(preview).toContainText('-₹250.01'); await expect(preview).toContainText('₹1,250.00')
  await preview.scrollIntoViewIfNeeded(); await captureReview(page, 'gap-independent-comparison-short320')
  const keys: string[] = [], me = await (await page.request.get('/api/auth/me')).json()
  await page.route('**' + path, async route => { if (route.request().method() !== 'POST') { await route.continue(); return }; keys.push(route.request().postDataJSON().operation_key); if (keys.length === 1) { const response = await route.fetch(); expect(response.status(), await response.text()).toBe(200); preparationID = (await response.json()).id; await route.abort('failed') } else await route.continue() })
  await page.getByRole('button', { name: 'Retain financial assessment', exact: true }).click(); await expect(page.getByRole('alert').filter({ hasText: 'The reply is uncertain.' })).toBeVisible()
  await expect(page.getByLabel('Item 1 amount', { exact: true })).toHaveValue('250.01'); await expect(page.getByLabel('Item 1 amount', { exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Close financial preparation', exact: true })).toBeDisabled(); await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(1)
  reviewDB(host.target, "UPDATE sessions SET reauthenticated_at=unixepoch('now')-301 WHERE user_id=?", [me.id])
  const denied = page.waitForResponse(response => response.url().endsWith(path) && response.request().method() === 'POST' && response.status() === 403)
  await page.getByRole('button', { name: 'Retry same assessment', exact: true }).click(); await denied
  await expect(page.getByRole('button', { name: 'Confirm identity to continue', exact: true })).toBeEnabled(); await captureReview(page, 'gap-uncertain-fresh-confirmation-short320')
  await page.getByRole('button', { name: 'Confirm identity to continue', exact: true }).click()
  const confirm = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Confirm it’s you.', exact: true }) })
  await confirm.getByLabel('Current password', { exact: true }).fill(host.one.password); await confirm.getByRole('checkbox', { name: 'Use an unused recovery code.', exact: true }).check(); await confirm.getByLabel('Unused recovery code', { exact: true }).fill(host.one.recovery.shift()!)
  await confirm.getByRole('button', { name: 'Confirm identity', exact: true }).click(); await expect(page.getByRole('button', { name: 'Retry same assessment', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Retry same assessment', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Financial assessment retained for another reviewer.' })).toBeVisible()
  expect(keys).toHaveLength(3); expect(new Set(keys).size).toBe(1); expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_gap_preparations')[0][0]).toBe(1)
  await page.unroute('**' + path)
  const result = await (await page.request.get(path + '/' + preparationID)).json()
  expect(result.comparison.homes[0]).toMatchObject({ checkpoint_paise: 150001, difference_paise: -25001, comparison_paise: 125000 }); expect(result.comparison.unresolved_items).toBe(1); expect(result.comparison.known_items).toBe(1)
  originalsHeld(); await page.getByRole('button', { name: 'Open financial review', exact: true }).click()
  await expect(page.getByText('You prepared this assessment. Another current reviewer checks and decides the exact version.', { exact: true })).toBeVisible(); await expect(page.getByRole('button', { name: 'Retain financial decision', exact: true })).toHaveCount(0)
  await page.getByText('You prepared this assessment. Another current reviewer checks and decides the exact version.', { exact: true }).scrollIntoViewIfNeeded(); await captureReview(page, 'gap-frozen-author-short320'); await page.getByRole('button', { name: 'Close frozen financial review', exact: true }).click()
})

test('another real reviewer opens the decision menu and a saved decision keeps unresolved evidence and originals', async () => {
  const page = host.two.page; await page.setViewportSize({ width: 375, height: 812 })
  await page.getByRole('button', { name: 'Financial comparison', exact: true }).click(); await page.getByRole('button', { name: 'Open financial review', exact: true }).click(); await dialogFits(page)
  await chooseOption(page, 'Financial assessment decision', 'Decline supplied assessment'); await page.getByRole('combobox', { name: 'Financial assessment decision', exact: true }).click(); await page.getByRole('option', { name: 'Approve supplied assessment', exact: true }).hover(); await captureReview(page, 'gap-separate-decision-menu-375'); await page.getByRole('option', { name: 'Approve supplied assessment', exact: true }).click()
  await fillGapDecision(page); let writes = 0
  await page.route('**' + path + '/*/decision', async route => { if (++writes === 1) { const response = await route.fetch(); expect(response.status()).toBe(200); await route.abort('failed') } else await route.continue() })
  await page.getByRole('button', { name: 'Retain financial decision', exact: true }).click(); await expect(page.getByRole('alert').filter({ hasText: 'The reply is uncertain.' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Close frozen financial review', exact: true })).toBeDisabled(); await captureReview(page, 'gap-separate-decision-uncertain-375')
  await page.getByRole('button', { name: 'Check saved result', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Separate financial decision retained. Unresolved evidence stays visible.' })).toBeVisible()
  expect(writes).toBe(1); expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_gap_decisions')[0][0]).toBe(1); originalsHeld(); await page.unroute('**' + path + '/*/decision')
  await page.getByRole('button', { name: 'Open financial review', exact: true }).click(); await expect(page.getByRole('region', { name: 'Frozen financial comparison', exact: true })).toContainText('1 unresolved item(s)')
  const proof = JSON.parse(readFileSync(join(host.root, 'matching-recovery-proof.json'), 'utf8'))
  const pdfHash = reviewDB(host.target, 'SELECT file_hash FROM receipt_jobs WHERE receipt_id=?', [host.receiptID])[0][0]
  expect(pdfHash).toBe(proof.original_pdf_sha256)
  expect((await page.request.get('/api/receipts/' + host.receiptID + '/download')).status()).toBe(423)
  await page.getByText('Unverified · needs investigation', { exact: true }).scrollIntoViewIfNeeded(); await captureReview(page, 'gap-approved-keeps-unknown-375'); await page.getByRole('button', { name: 'Close frozen financial review', exact: true }).click()
})

test('linked financial assessment retains the earlier comparison and actual filter selection', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 820, height: 1050 })
  await page.getByRole('button', { name: 'Refresh financial assessments', exact: true }).click(); await page.getByRole('button', { name: 'Open financial review', exact: true }).click(); await page.getByRole('button', { name: 'Prepare linked assessment', exact: true }).click()
  await page.getByLabel('Item 1 amount', { exact: true }).fill('250.02'); await chooseOption(page, 'Item 2 evidence state', 'Supported by supplied evidence'); await expect(page.getByRole('region', { name: 'Known comparison previews', exact: true })).toContainText('1 unresolved supplied item(s)'); await page.getByRole('checkbox', { name: 'I checked the supplied coverage, exact amounts and unresolved evidence.', exact: true }).check()
  await page.getByRole('button', { name: 'Retain financial assessment', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Financial assessment retained' })).toBeVisible()
  await page.getByRole('combobox', { name: 'Financial assessment state', exact: true }).click(); await page.getByRole('option', { name: 'Earlier version', exact: true }).hover(); await captureReview(page, 'gap-state-filter-menu-tablet'); await page.getByRole('option', { name: 'Earlier version', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Open financial review', exact: true })).toHaveCount(1)
  await chooseOption(page, 'Financial assessment state', 'Awaiting another reviewer'); await page.getByRole('button', { name: 'Open financial review', exact: true }).click()
  await expect(page.getByText('Version 2 · Earlier version', { exact: true })).toBeVisible(); await expect(page.getByText('Version 3 · Awaiting another reviewer', { exact: true })).toBeVisible()
  await expect(page.getByRole('article', { name: 'A-101 frozen comparison', exact: true })).toContainText('₹1,249.99'); await expect(page.getByRole('region', { name: 'Frozen financial comparison', exact: true })).toContainText('1 unresolved item(s)'); originalsHeld(); await page.getByRole('heading', { name: 'Every assessment, retained.', exact: true }).scrollIntoViewIfNeeded(); await captureReview(page, 'gap-linked-history-tablet')
  await page.getByRole('button', { name: 'Close frozen financial review', exact: true }).click()
})

test('an actual empty declaration needs complete coverage and its explicit attestation', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 320, height: 812 })
  await page.getByRole('button', { name: 'Prepare financial assessment', exact: true }).click()
  await page.getByRole('button', { name: 'Remove item 1', exact: true }).click()
  await page.getByLabel('Coverage evidence', { exact: true }).fill('Independently supplied complete fictional book was checked and contains no missing items for this interval.')
  await page.getByLabel('Assessment reason', { exact: true }).fill('An explicit independently supplied complete interval attestation needs another reviewer.')
  await page.getByRole('checkbox', { name: 'I explicitly checked that the complete covered interval has no missing items.', exact: true }).check()
  await page.getByRole('checkbox', { name: 'I checked the supplied coverage, exact amounts and unresolved evidence.', exact: true }).check()
  await page.getByRole('button', { name: 'Retain financial assessment', exact: true }).click(); await expect(page.getByRole('alert').filter({ hasText: 'empty gap requires' })).toBeVisible()
  await page.getByRole('checkbox', { name: 'The supplied evidence covers this whole interval.', exact: true }).check(); await page.getByRole('checkbox', { name: 'I explicitly checked that the complete covered interval has no missing items.', exact: true }).scrollIntoViewIfNeeded(); await captureReview(page, 'gap-explicit-empty-attestation-320')
  await page.getByRole('button', { name: 'Retain financial assessment', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Financial assessment retained' })).toBeVisible()
  expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_gap_preparations WHERE known_count=0 AND unresolved_count=0')[0][0]).toBe(1); originalsHeld()
})

test('actual financial read errors retry and a bounded short-phone journal keeps aligned page controls', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 320, height: 440 })
  const me = await (await page.request.get('/api/auth/me')).json(), preparation = await (await page.request.get(path + '/' + preparationID)).json()
  const headers = { Origin: new URL(page.url()).origin, 'X-CSRF-Token': me.csrf_token }
  for (let i = 0; i < 12; i++) {
    const response = await page.request.post(path, { headers, data: { ...preparation.input, operation_key: 'bounded-financial-browser-' + String(i).padStart(3, '0'), predecessor_id: '', predecessor_version: 0, title: `Independent supplied financial assessment ${i + 1}`, items: preparation.input.items.map((item: { item_ref: string }) => ({ ...item, item_ref: item.item_ref + '-page-' + i })) } })
    expect(response.status(), await response.text()).toBe(200)
  }
  await page.getByRole('button', { name: 'Refresh financial assessments', exact: true }).click(); await expect(page.getByRole('button', { name: 'Open financial review', exact: true })).toHaveCount(12)
  const pager = page.getByRole('navigation', { name: 'Recovery result pages', exact: true })
  await pager.getByRole('button', { name: 'Next', exact: true }).click(); await expect(page.getByRole('button', { name: 'Open financial review', exact: true })).toHaveCount(3)
  await expect(pager).toContainText('Page 2 · 15 records'); await pager.scrollIntoViewIfNeeded(); await pageFits(page)
  const alignment = await pager.evaluate(nav => { const buttons = nav.querySelectorAll('button'); return Math.abs(buttons[0].getBoundingClientRect().y - buttons[1].getBoundingClientRect().y) }); expect(alignment).toBeLessThanOrEqual(1)
  await expect(pager.getByRole('button', { name: 'Next', exact: true })).toBeDisabled(); await captureReview(page, 'gap-bounded-page2-short320')
  await pager.getByRole('button', { name: 'Previous', exact: true }).click()
  let calls = 0
  await page.route('**' + path + '?**', async route => { if (++calls === 1) await route.fulfill({ status: 500, json: { error: 'controlled_financial_read_failure' } }); else await route.continue() })
  await page.getByRole('button', { name: 'Refresh financial assessments', exact: true }).click(); await expect(page.getByRole('button', { name: 'Try financial list again', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Try financial list again', exact: true }).scrollIntoViewIfNeeded(); await captureReview(page, 'gap-list-error-short320')
  await page.getByRole('button', { name: 'Try financial list again', exact: true }).click(); await expect(page.getByRole('button', { name: 'Open financial review', exact: true })).toHaveCount(12); await page.unroute('**' + path + '?**')
  const detailPattern = /\/api\/recovery\/financial-gap\/preparations\/[A-Za-z0-9_-]{43}$/
  let entered!: () => void, release!: () => void
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route(detailPattern, async route => { const response = await route.fetch(); entered(); await held; await route.fulfill({ response }).catch(() => {}) })
  try {
    await page.getByRole('button', { name: 'Open financial review', exact: true }).first().click(); await ready
    await expect(page.getByRole('heading', { name: 'Opening this assessment…', exact: true })).toBeVisible(); await captureReview(page, 'gap-detail-loading-short320')
    await page.getByRole('button', { name: 'Close frozen financial review', exact: true }).click(); release(); await expect(page.getByRole('dialog')).toHaveCount(0)
  } finally { release(); await page.unroute(detailPattern) }
  calls = 0
  await page.route('**' + path + '/*/history?**', async route => { if (++calls === 1) await route.fulfill({ status: 500, json: { error: 'controlled_financial_history_failure' } }); else await route.continue() })
  await page.getByRole('button', { name: 'Open financial review', exact: true }).first().click(); await expect(page.getByRole('button', { name: 'Try financial history again', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Try financial history again', exact: true }).scrollIntoViewIfNeeded(); await captureReview(page, 'gap-history-error-short320')
  await page.getByRole('button', { name: 'Try financial history again', exact: true }).click(); await expect(page.getByText('Version 1 · Awaiting another reviewer', { exact: true })).toBeVisible()
  await page.unroute('**' + path + '/*/history?**'); await page.getByRole('button', { name: 'Close frozen financial review', exact: true }).click(); originalsHeld()
})
