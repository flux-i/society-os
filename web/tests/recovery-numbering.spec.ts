import { test, expect } from '@playwright/test'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { createHash } from 'node:crypto'
import { readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { createOffsiteFixture } from './offsite-fixture.mjs'
import { chooseOption } from './helpers'
import { pageFits, dialogFits } from './registry-import-fixtures'
import { captureReview, loginReviewer, reviewDB, reviewHost } from './recovery-review-fixtures'
import { nestedReviewHost } from './recovery-review-fixtures'
import { fillNumbering, fillNumberingDecision } from './recovery-numbering-fixtures'

test.describe.configure({ mode: 'serial' })
test.setTimeout(120000)
test.use({ actionTimeout: 10000 })
let host: Awaited<ReturnType<typeof reviewHost>>, peer: Awaited<ReturnType<typeof createOffsiteFixture>>
let policyID: string
const runPrivateCLI = promisify(execFile)
test.beforeAll(async ({ browser }) => { host = await reviewHost(browser); await loginReviewer(host.one); await loginReviewer(host.two); peer = await createOffsiteFixture(host.root, process.env.SOCIETY_BROWSER_BINARY!, 'LOST_REPLY') })
test.afterAll(async () => { await peer?.close(); await host?.close() })

test('actual supplied-numbering empty view and shared digit menu fit desktop tablet and short phones', async () => {
  const page = host.one.page
  await page.getByRole('button', { name: 'Receipt numbering', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'A fresh beginning, carefully numbered.' })).toBeVisible()
  for (const [width, height] of [[1440, 1000], [820, 1050], [375, 812], [320, 440]]) {
    await page.setViewportSize({ width, height }); await pageFits(page)
    await page.getByRole('button', { name: 'Prepare numbering', exact: true }).scrollIntoViewIfNeeded(); await captureReview(page, `numbering-empty-${width}-${height}`)
    const refreshAlignment = await page.getByRole('button', { name: 'Refresh numbering', exact: true }).evaluate(button => {
      const text = [...button.childNodes].find(node => node.nodeType === Node.TEXT_NODE)!
      const range = document.createRange(); range.selectNode(text)
      const label = range.getBoundingClientRect(), icon = button.querySelector('svg')!.getBoundingClientRect()
      return Math.abs(label.y + label.height / 2 - icon.y - icon.height / 2)
    })
    expect(refreshAlignment, `Refresh icon and label must share one centered row at ${width}px`).toBeLessThanOrEqual(1)
    await page.getByRole('button', { name: 'Prepare numbering', exact: true }).click(); await dialogFits(page)
    await page.getByRole('combobox', { name: 'Sequence digits', exact: true }).focus(); await page.keyboard.press('Enter')
    await expect(page.getByRole('option', { name: '7 digits', exact: true })).toBeVisible(); await page.getByRole('option', { name: '7 digits', exact: true }).hover(); await captureReview(page, `numbering-digit-menu-${width}-${height}`)
    await page.keyboard.press('Escape'); await expect(page.getByRole('combobox', { name: 'Sequence digits', exact: true })).toBeFocused()
    await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0)
  }
})

test('supplied format is independently previewed and actual policy fields survive an uncertain retry and fresh confirmation', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 320, height: 440 })
  await page.getByRole('button', { name: 'Prepare numbering', exact: true }).click(); await fillNumbering(page)
  await chooseOption(page, 'Sequence digits', '7 digits'); await page.getByLabel('First sequence', { exact: true }).fill('42'); await page.getByLabel('Last sequence', { exact: true }).fill('5000')
  await expect(page.getByLabel('Supplied receipt format preview')).toContainText('TRV-R2026A-0000042'); await expect(page.getByLabel('Supplied receipt format preview')).toContainText('4,959 available identities')
  await chooseOption(page, 'Sequence digits', '6 digits'); await page.getByLabel('First sequence', { exact: true }).fill('1'); await page.getByLabel('Last sequence', { exact: true }).fill('999999')
  await expect(page.getByLabel('Supplied receipt format preview')).toContainText('TRV-R2026A-000001')
  const keys: string[] = [], scope = await (await page.request.get('/api/auth/me')).json()
  await page.route('**/api/recovery/numbering/policies', async route => {
    if (route.request().method() !== 'POST') { await route.continue(); return }
    keys.push(route.request().postDataJSON().operation_key)
    if (keys.length === 1) { const response = await route.fetch(); expect(response.status()).toBe(200); policyID = (await response.json()).id; await route.abort('failed') } else await route.continue()
  })
  await page.getByRole('button', { name: 'Retain numbering preparation', exact: true }).click(); await expect(page.getByRole('alert').filter({ hasText: 'The reply is uncertain.' })).toBeVisible()
  await expect(page.getByLabel('Receipt prefix', { exact: true })).toHaveValue('TRV-R2026A'); await expect(page.getByLabel('Receipt prefix', { exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Close numbering preparation', exact: true })).toBeDisabled(); await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(1)
  reviewDB(host.target, "UPDATE sessions SET reauthenticated_at=unixepoch('now')-301 WHERE user_id=?", [scope.id])
  const denied = page.waitForResponse(response => response.url().endsWith('/api/recovery/numbering/policies') && response.request().method() === 'POST' && response.status() === 403)
  await page.getByRole('button', { name: 'Retry same preparation', exact: true }).click(); await denied
  await expect(page.getByRole('button', { name: 'Confirm identity to continue', exact: true })).toBeEnabled(); await captureReview(page, 'numbering-uncertain-fresh-confirmation-short320')
  await page.getByRole('button', { name: 'Confirm identity to continue', exact: true }).click()
  const confirm = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Confirm it’s you.', exact: true }) })
  await confirm.getByLabel('Current password', { exact: true }).fill(host.one.password); await confirm.getByRole('checkbox', { name: 'Use an unused recovery code.' }).check(); await confirm.getByLabel('Unused recovery code', { exact: true }).fill(host.one.recovery.shift()!)
  await confirm.getByRole('button', { name: 'Confirm identity', exact: true }).click(); await expect(page.getByRole('button', { name: 'Retry same preparation', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Retry same preparation', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Numbering preparation retained for another reviewer.' })).toBeVisible()
  expect(keys).toHaveLength(3); expect(new Set(keys).size).toBe(1); expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_numbering_policies')[0][0]).toBe(1)
  await page.unroute('**/api/recovery/numbering/policies'); await page.getByRole('button', { name: 'Open numbering review', exact: true }).click()
  await expect(page.getByText('You prepared this policy. Another current reviewer checks and decides the exact version.', { exact: true })).toBeVisible(); await expect(page.getByRole('button', { name: 'Retain separate decision', exact: true })).toHaveCount(0)
  await captureReview(page, 'numbering-author-separate-person-short320'); await page.getByRole('button', { name: 'Close frozen numbering review', exact: true }).click()
})

test('another current person opens the decision menu and resolves a lost decision reply once', async () => {
  const page = host.two.page; await page.setViewportSize({ width: 375, height: 812 }); await page.getByRole('button', { name: 'Receipt numbering', exact: true }).click(); await page.getByRole('button', { name: 'Open numbering review', exact: true }).click(); await dialogFits(page)
  await chooseOption(page, 'Numbering decision', 'Decline supplied preparation'); await page.getByRole('combobox', { name: 'Numbering decision', exact: true }).click(); await page.getByRole('option', { name: 'Approve supplied preparation', exact: true }).hover(); await captureReview(page, 'numbering-separate-decision-menu-375'); await page.getByRole('option', { name: 'Approve supplied preparation', exact: true }).click()
  await fillNumberingDecision(page); let writes = 0
  await page.route('**/api/recovery/numbering/policies/*/decision', async route => { if (++writes === 1) { const response = await route.fetch(); expect(response.status()).toBe(200); await route.abort('failed') } else await route.continue() })
  await page.getByRole('button', { name: 'Retain separate decision', exact: true }).click(); await expect(page.getByRole('alert').filter({ hasText: 'The reply is uncertain.' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Close frozen numbering review', exact: true })).toBeDisabled(); await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(1)
  await captureReview(page, 'numbering-separate-decision-uncertain-375'); await page.getByRole('button', { name: 'Check saved result', exact: true }).click()
  await expect(page.getByRole('status').filter({ hasText: 'Separate numbering decision retained for independent custody.' })).toBeVisible(); expect(writes).toBe(1); expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_numbering_decisions')[0][0]).toBe(1)
  await page.unroute('**/api/recovery/numbering/policies/*/decision')
})

test('real private CLI lost storage reply remains held until independent exact-version decryption and appears in the portal', async () => {
  const page = host.one.page, binary = process.env.SOCIETY_BROWSER_BINARY!, file = join(host.root, 'private-numbering-reservation.json')
  const policy = await (await page.request.get('/api/recovery/numbering/policies/' + policyID)).json()
  const request = { format_version: 1, operation_id: 'browser-series-reservation-0001', attempt_key: 'browser-series-initial-handoff-0001', epoch_id: host.origin.id, source_sha256: host.origin.source_sha256, nomination_id: host.nomination.operation_id, policy_id: policyID, policy_version: policy.version, custodians: host.nomination.custodians }
  writeFileSync(file, JSON.stringify(request), { mode: 0o600 })
  const invoke = async () => JSON.parse((await runPrivateCLI(binary, ['recover-series-reserve', '--db', host.target, '--series-config-file', peer.config, '--series-fixture', '--series-request-file', file], { encoding: 'utf8' })).stdout)
  expect(await invoke()).toEqual({ operation_id: request.operation_id, state: 'UNKNOWN', attempts: 1, prepared_only: true, can_resume: false })
  const before = reviewDB(host.target, 'SELECT cipher_sha256,hex(ciphertext) FROM recovery_series_intents')[0]
  expect((await invoke()).state).toBe('UNKNOWN'); expect(reviewDB(host.target, 'SELECT cipher_sha256,hex(ciphertext) FROM recovery_series_intents')[0]).toEqual(before)
  await page.getByRole('button', { name: 'Refresh numbering', exact: true }).click(); await page.getByRole('button', { name: 'Open numbering review', exact: true }).click()
  await expect(page.getByRole('region', { name: 'Independent reservation progress' })).toContainText('Outcome uncertain · independent check needed'); await captureReview(page, 'numbering-real-uncertain-custody-short320')
  await page.getByRole('button', { name: 'Close frozen numbering review', exact: true }).click()
  const independent = await (await page.request.get(peer.origin + '/_fixture/status')).json(); expect(independent.puts).toBe(1); expect(independent.objects).toHaveLength(1)
  const object = independent.objects[0]
  const digest = (value: string) => createHash('sha256').update(value).digest('hex')
  expect(object.key).toBe('recovery/society/receipt-series/' + digest('fictional-demo-housing_demo_v1') + '/' + digest('TRV-R2026A') + '.age')
  const result = JSON.parse((await runPrivateCLI(binary, ['recover-series-reconcile', '--db', host.target, '--series-config-file', peer.reader, '--series-fixture', '--series-id', request.operation_id, '--object-version', object.version, '--custody-identities-file', peer.identities[1]], { encoding: 'utf8' })).stdout)
  expect(result.state).toBe('VERIFIED'); expect(result.can_resume).toBe(false)
  await page.getByRole('button', { name: 'Refresh numbering', exact: true }).click(); await page.getByRole('button', { name: 'Open numbering review', exact: true }).click()
  await expect(page.getByRole('region', { name: 'Independent reservation progress' })).toContainText('Original checksum and version verified'); await captureReview(page, 'numbering-independent-original-verified-short320')
  expect((await page.request.get('/api/receipts/' + host.receiptID + '/download')).status()).toBe(423)
  expect(reviewDB(host.target, 'SELECT amount_paise FROM entries WHERE id IN(SELECT entry_id FROM receipts)')[0][0]).toBe(40000)
  expect(reviewDB(host.target, "SELECT SUM(CASE WHEN kind IN('OPENING_DEBIT','CHARGE') THEN amount_paise ELSE -amount_paise END) FROM entries WHERE state='POSTED' AND id NOT IN(SELECT entry_id FROM entry_reversals)")[0][0]).toBe(150001)
  const proof = { request, result, independent, original_cipher_sha256: before[0], original_pdf_sha256: host.receiptID && JSON.parse(readFileSync(join(host.root, 'matching-recovery-proof.json'), 'utf8')).original_pdf_sha256, amount_paise: 40000, balance_paise: 150001, limits: 'Actual private CLI, signed numeric-loopback storage, independent version and real age custody decrypt on a disposable current restored host; no live bucket, receipt issuance or cutover.' }
  writeFileSync(join(host.root, 'numbering-reservation-proof.json'), JSON.stringify(proof, null, 2), { mode: 0o600 })
  await page.getByRole('button', { name: 'Close frozen numbering review', exact: true }).click()
})

test('linked fresh policy keeps the attempted original and opened state filter while read errors retry visibly', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 820, height: 1050 })
  await page.getByRole('button', { name: 'Open numbering review', exact: true }).click(); await page.getByRole('button', { name: 'Prepare linked policy', exact: true }).click(); await fillNumbering(page, 'TRV-R2026B'); await page.getByLabel('Preparation title', { exact: true }).fill('Linked fresh recovery receipt series')
  await page.getByRole('button', { name: 'Retain numbering preparation', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Numbering preparation retained' })).toBeVisible()
  await page.getByRole('combobox', { name: 'Numbering policy state', exact: true }).click(); await page.getByRole('option', { name: 'Earlier version', exact: true }).hover(); await captureReview(page, 'numbering-state-filter-menu-tablet'); await page.getByRole('option', { name: 'Earlier version', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Open numbering review', exact: true })).toHaveCount(1)
  await chooseOption(page, 'Numbering policy state', 'Awaiting another reviewer'); await page.getByRole('button', { name: 'Open numbering review', exact: true }).click()
  await expect(page.getByText('Version 2 · Earlier version', { exact: true })).toBeVisible(); await expect(page.getByText('Version 3 · Awaiting another reviewer', { exact: true })).toBeVisible(); await captureReview(page, 'numbering-linked-history-tablet')
  await page.getByRole('button', { name: 'Close frozen numbering review', exact: true }).click()
  let calls = 0
  await page.route('**/api/recovery/numbering/policies?**', async route => { if (++calls === 1) await route.fulfill({ status: 500, json: { error: 'controlled_numbering_read_failure' } }); else await route.continue() })
  await page.getByRole('button', { name: 'Refresh numbering', exact: true }).click(); await expect(page.getByRole('button', { name: 'Try numbering list again', exact: true })).toBeVisible(); await captureReview(page, 'numbering-list-error-tablet')
  await page.getByRole('button', { name: 'Try numbering list again', exact: true }).click(); await expect(page.getByRole('button', { name: 'Open numbering review', exact: true })).toHaveCount(1); await page.unroute('**/api/recovery/numbering/policies?**')
  await chooseOption(page, 'Numbering policy state', 'Declined'); await expect(page.getByRole('heading', { name: 'No numbering preparations in this view.' })).toBeVisible(); await captureReview(page, 'numbering-filtered-empty-tablet')
})

test('actual numbering loading detail history errors and bounded pages retain usable short-phone controls', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 320, height: 440 })
  await chooseOption(page, 'Numbering policy state', 'Every numbering preparation')
  const me = await (await page.request.get('/api/auth/me')).json(), input = await (await page.request.get('/api/recovery/numbering/policies/' + policyID)).json()
  for (let i = 0; i < 12; i++) {
    const response = await page.request.post('/api/recovery/numbering/policies', { headers: { Origin: new URL(page.url()).origin, 'X-CSRF-Token': me.csrf_token }, data: { operation_key: 'numbering-browser-bounded-page-' + String(i).padStart(3, '0'), scope_key: me.scope_key, title: `Independent supplied numbering ${i + 1}`, reason: input.input.reason, policy: { ...input.input.policy, prefix: 'TRV-PAGE-' + String(i).padStart(3, '0') } } })
    expect(response.status()).toBe(200)
  }
  await page.getByRole('button', { name: 'Refresh numbering', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Open numbering review', exact: true })).toHaveCount(12)
  const pager = page.getByRole('navigation', { name: 'Recovery result pages', exact: true })
  await pager.getByRole('button', { name: 'Next', exact: true }).click(); await expect(page.getByRole('button', { name: 'Open numbering review', exact: true })).toHaveCount(2)
  await expect(pager).toContainText('Page 2 · 14 records'); await pager.scrollIntoViewIfNeeded(); await pageFits(page); await captureReview(page, 'numbering-bounded-page2-short320')
  const aligned = await pager.evaluate(nav => { const buttons = nav.querySelectorAll('button'); return Math.abs(buttons[0].getBoundingClientRect().y - buttons[1].getBoundingClientRect().y) })
  expect(aligned).toBeLessThanOrEqual(1); await expect(pager.getByRole('button', { name: 'Next', exact: true })).toBeDisabled()
  await pager.getByRole('button', { name: 'Previous', exact: true }).click(); await expect(page.getByRole('button', { name: 'Open numbering review', exact: true })).toHaveCount(12)

  const detailPattern = /\/api\/recovery\/numbering\/policies\/[A-Za-z0-9_-]{43}$/
  let enter!: () => void, release!: () => void, settled!: () => void, dismissed = false
  const entered = new Promise<void>(resolve => enter = resolve), held = new Promise<void>(resolve => release = resolve), finished = new Promise<void>(resolve => settled = resolve)
  await page.route(detailPattern, async route => {
    const response = await route.fetch(); enter(); await held
    try { await route.fulfill({ response }) } catch (error) {
      if (!dismissed || !(error instanceof Error) || !error.message.includes('Route is already handled')) throw error
    } finally { settled() }
  })
  await page.getByRole('button', { name: 'Open numbering review', exact: true }).first().click(); await entered
  await expect(page.getByRole('heading', { name: 'Opening this preparation…', exact: true })).toBeVisible(); await dialogFits(page); await captureReview(page, 'numbering-detail-loading-short320')
  dismissed = true; await page.getByRole('button', { name: 'Close frozen numbering review', exact: true }).click(); release(); await finished; await page.unroute(detailPattern)
  await expect(page.getByRole('dialog')).toHaveCount(0)
  let reads = 0
  await page.route(detailPattern, async route => { if (++reads === 1) await route.fulfill({ status: 500, json: { error: 'controlled_detail_failure' } }); else await route.continue() })
  await page.getByRole('button', { name: 'Open numbering review', exact: true }).first().click(); await expect(page.getByRole('button', { name: 'Try numbering review again', exact: true })).toBeVisible(); await dialogFits(page); await captureReview(page, 'numbering-detail-error-short320')
  await page.getByRole('button', { name: 'Try numbering review again', exact: true }).click(); await expect(page.getByRole('dialog').getByRole('heading', { name: 'Independent supplied numbering 12', exact: true })).toBeVisible(); await page.unroute(detailPattern)
  await page.getByRole('button', { name: 'Close frozen numbering review', exact: true }).click()
  let historyCalls = 0
  await page.route('**/api/recovery/numbering/policies/*/history?**', async route => { if (++historyCalls === 1) await route.fulfill({ status: 500, json: { error: 'controlled_history_failure' } }); else await route.continue() })
  await page.getByRole('button', { name: 'Open numbering review', exact: true }).first().click(); await expect(page.getByRole('button', { name: 'Try numbering history again', exact: true })).toBeVisible(); await page.getByRole('button', { name: 'Try numbering history again', exact: true }).scrollIntoViewIfNeeded(); await captureReview(page, 'numbering-history-error-short320')
  await page.getByRole('button', { name: 'Try numbering history again', exact: true }).click(); await expect(page.getByText('Version 1 · Awaiting another reviewer', { exact: true })).toBeVisible(); await page.unroute('**/api/recovery/numbering/policies/*/history?**')
  await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0)
  let statusCalls = 0
  await page.route('**/api/recovery/numbering/status', async route => { if (++statusCalls === 1) await route.fulfill({ status: 500, json: { error: 'controlled_status_failure' } }); else await route.continue() })
  await page.getByRole('button', { name: 'Refresh numbering', exact: true }).click(); await expect(page.getByRole('button', { name: 'Try numbering again', exact: true })).toBeVisible(); await expect(page.getByRole('button', { name: 'Prepare numbering', exact: true })).toBeDisabled(); await page.getByRole('button', { name: 'Try numbering again', exact: true }).scrollIntoViewIfNeeded(); await captureReview(page, 'numbering-status-error-short320')
  await page.getByRole('button', { name: 'Try numbering again', exact: true }).click(); await expect(page.getByRole('button', { name: 'Prepare numbering', exact: true })).toBeEnabled(); await page.unroute('**/api/recovery/numbering/status')
})

test('matching nested restore preserves the numbering journal encrypted original and held copied reviewer identities', async ({ browser }) => {
  const nested = await nestedReviewHost(host, browser, true)
  try {
    const path = join(nested.root, 'restored', 'society.db')
    expect(reviewDB(path, 'SELECT COUNT(*) FROM recovery_numbering_policies')[0][0]).toBe(14)
    expect(reviewDB(path, 'SELECT COUNT(*) FROM recovery_numbering_decisions')[0][0]).toBe(1)
    expect(reviewDB(path, 'SELECT cipher_sha256,hex(ciphertext) FROM recovery_series_intents')).toEqual(reviewDB(host.target, 'SELECT cipher_sha256,hex(ciphertext) FROM recovery_series_intents'))
    expect(reviewDB(path, 'SELECT state,attempt,attempt_key,object_version,reason_code FROM recovery_series_events ORDER BY sequence')).toEqual(reviewDB(host.target, 'SELECT state,attempt,attempt_key,object_version,reason_code FROM recovery_series_events ORDER BY sequence'))
    expect((await nested.page.request.get('/api/recovery/numbering/policies')).status()).toBe(423)
    await captureReview(nested.page, 'numbering-copied-authority-held-after-nested-restore-short320')
  } finally { await nested.close() }
})
