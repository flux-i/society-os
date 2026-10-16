import { test, expect } from '@playwright/test'
import { execute, names } from './native-webmcp-helpers'
import { captureReview, loginReviewer, reviewDB, reviewHost } from './recovery-review-fixtures'
import { fillNumbering } from './recovery-numbering-fixtures'

test.describe.configure({ mode: 'serial' })
test.setTimeout(120000)
test.use({ actionTimeout: 10000 })
let host: Awaited<ReturnType<typeof reviewHost>>
test.beforeAll(async ({ browser }) => { host = await reviewHost(browser); await loginReviewer(host.one); await loginReviewer(host.two) })
test.afterAll(async () => { await host?.close() })

test('actual native bounded numbering read preserves the human format fields focus and internal scroll', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 375, height: 812 })
  await expect.poll(async () => (await names(page)).sort()).toEqual(['society_read_recovery_gap_status', 'society_read_recovery_numbering_status', 'society_read_recovery_review_status', 'society_read_recovery_state'])
  await page.getByRole('button', { name: 'Receipt numbering', exact: true }).click(); await page.getByRole('button', { name: 'Prepare numbering', exact: true }).click(); await fillNumbering(page, 'TRV-R2026B')
  const notes = page.getByRole('textbox', { name: 'Preparation reason', exact: true }); await notes.focus()
  const before = await page.evaluate(() => ({ hash: location.hash, scroll: scrollY, dialogScroll: document.querySelector('dialog .dialog-scroll')?.scrollTop }))
  const writes: string[] = []; page.on('request', request => { if (!['GET', 'HEAD'].includes(request.method())) writes.push(request.url()) })
  expect(JSON.parse(await execute(page, 'society_read_recovery_numbering_status', {}))).toEqual({ pending: 0, approved_preparation: 0, independent_originals_verified: 0, unresolved_handoffs: 0, can_resume: false })
  expect(await page.evaluate(() => ({ hash: location.hash, scroll: scrollY, dialogScroll: document.querySelector('dialog .dialog-scroll')?.scrollTop }))).toEqual(before)
  await expect(notes).toBeFocused(); await expect(notes).toHaveValue('The current supplied numbering and independent custody records were checked.'); await expect(page.getByLabel('Receipt prefix', { exact: true })).toHaveValue('TRV-R2026B')
  await expect(page.getByRole('checkbox', { name: 'I checked the supplied numbering bounds and independent custody evidence.' })).toBeChecked()
  for (const [tool, input] of [['society_read_recovery_numbering_status', { policy_id: 'private-policy' }], ['society_approve_numbering', {}], ['society_reserve_receipt_series', {}], ['society_reconcile_receipt_series', {}], ['society_resume_recovery', {}]] as const) await expect(execute(page, tool, input)).rejects.toThrow()
  expect(writes).toEqual([]); expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_numbering_policies')[0][0]).toBe(0)
  await captureReview(page, 'numbering-native-human-form-375')
})

test('actual captured numbering status is rejected after another current person changes the journal', async () => {
  const page = host.one.page, other = host.two.page
  let entered!: () => void, release!: () => void, calls = 0
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route('**/api/recovery/numbering/status', async route => { const response = await route.fetch(); if (++calls === 1) { entered(); await held }; await route.fulfill({ response }) })
  const reading = execute(page, 'society_read_recovery_numbering_status', {}).then(value => ({ value, failed: false }), () => ({ value: '', failed: true }))
  await ready
  await other.getByRole('button', { name: 'Receipt numbering', exact: true }).click(); await other.getByRole('button', { name: 'Prepare numbering', exact: true }).click(); await fillNumbering(other)
  await other.getByRole('button', { name: 'Retain numbering preparation', exact: true }).click(); await expect(other.getByRole('status').filter({ hasText: 'Numbering preparation retained for another reviewer.' })).toBeVisible(); release()
  expect((await reading).failed).toBe(true); await page.unroute('**/api/recovery/numbering/status')
  expect(JSON.parse(await execute(page, 'society_read_recovery_numbering_status', {}))).toEqual({ pending: 1, approved_preparation: 0, independent_originals_verified: 0, unresolved_handoffs: 0, can_resume: false })
  expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_numbering_decisions')[0][0]).toBe(0)
  await expect(page.getByLabel('Receipt prefix', { exact: true })).toHaveValue('TRV-R2026B')
  await captureReview(page, 'numbering-native-changed-journal-keeps-form-375'); await page.getByRole('button', { name: 'Close numbering preparation', exact: true }).click()
})

test('actual offline revocation discards captured numbering counts and copied identities keep only held status', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 320, height: 440 })
  let entered!: () => void, release!: () => void, calls = 0
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route('**/api/recovery/numbering/status', async route => { const response = await route.fetch(); if (++calls === 1) { entered(); await held }; await route.fulfill({ response }) })
  const reading = execute(page, 'society_read_recovery_numbering_status', {}).then(value => ({ value, failed: false }), () => ({ value: '', failed: true }))
  await ready; expect(host.revoke().status).toBe('recovery_nomination_revoked'); release(); expect((await reading).failed).toBe(true)
  await page.unroute('**/api/recovery/numbering/status'); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible(); await expect.poll(() => names(page)).toEqual([])
  await page.getByLabel('Email address', { exact: true }).fill(host.one.email); await page.getByLabel('Password', { exact: true }).fill(host.one.password); await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Your records. Kept together.' })).toBeVisible(); await expect.poll(() => names(page)).toEqual(['society_read_recovery_state'])
  await expect(execute(page, 'society_read_recovery_numbering_status', {})).rejects.toThrow(); await expect(execute(page, 'society_reserve_receipt_series', {})).rejects.toThrow()
  expect((await page.request.get('/api/recovery/numbering/status')).status()).toBe(423)
  expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_series_intents')[0][0]).toBe(0); expect(reviewDB(host.target, 'SELECT amount_paise FROM entries WHERE id IN(SELECT entry_id FROM receipts)')[0][0]).toBe(40000)
  await captureReview(page, 'numbering-native-revoked-identity-short320')
})
