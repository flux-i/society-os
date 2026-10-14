import { test, expect } from '@playwright/test'
import { names, execute } from './native-webmcp-helpers'
import { captureReview, fillAccessReview, loginReviewer, nestedReviewHost, reviewDB, reviewHost } from './recovery-review-fixtures'

test.describe.configure({ mode: 'serial' })
test.setTimeout(120000)
test.use({ actionTimeout: 10000 })
let host: Awaited<ReturnType<typeof reviewHost>>
test.beforeAll(async ({ browser }) => { host = await reviewHost(browser); await loginReviewer(host.one); await loginReviewer(host.two) })
test.afterAll(async () => { await host?.close() })

test('actual native discovery exposes two bounded reads and preserves a human access preparation', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 375, height: 812 })
  await expect.poll(async () => (await names(page)).sort()).toEqual(['society_read_recovery_review_status', 'society_read_recovery_state'])
  await page.getByRole('button', { name: 'Prepare new person', exact: true }).click(); await fillAccessReview(page, 'Keep this supplied evidence while native status reads.')
  const notes = page.getByRole('textbox', { name: 'Review notes', exact: true }); await notes.focus()
  const before = await page.evaluate(() => ({ hash: location.hash, scroll: scrollY, dialogScroll: document.querySelector('dialog .dialog-scroll')?.scrollTop }))
  const writes: string[] = []; page.on('request', request => { if (!['GET', 'HEAD'].includes(request.method())) writes.push(request.url()) })
  expect(JSON.parse(await execute(page, 'society_read_recovery_review_status', {}))).toEqual({ pending: 0, approved_preparation: 0, declined: 0, earlier_appointment: 0, unfinished_actions_held: 1, can_resume: false })
  expect(await page.evaluate(() => ({ hash: location.hash, scroll: scrollY, dialogScroll: document.querySelector('dialog .dialog-scroll')?.scrollTop }))).toEqual(before)
  await expect(notes).toBeFocused(); await expect(notes).toHaveValue('Keep this supplied evidence while native status reads.')
  await expect(page.getByRole('checkbox', { name: 'I checked this supplied evidence and understand this prepares a decision while recovery remains held.' })).toBeChecked()
  for (const [tool, args] of [['society_read_recovery_review_status', { approve: true }], ['society_nominate_recovery', {}], ['society_approve_recovery', {}], ['society_resume_recovery', {}]] as const) await expect(execute(page, tool, args)).rejects.toThrow()
  expect(writes).toEqual([]); expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_proposals')[0][0]).toBe(0)
  await captureReview(page, 'native-review-stable-human-form-375'); await page.getByRole('button', { name: 'Close recovery preparation', exact: true }).click()
})

test('actual native held status rejects a changed review journal without returning stale counts or approving anything', async () => {
  const page = host.one.page, other = host.two.page; await page.setViewportSize({ width: 320, height: 440 })
  let entered!: () => void, release!: () => void, calls = 0
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route('**/api/recovery/status', async route => { const response = await route.fetch(); if (++calls === 1) { entered(); await held }; await route.fulfill({ response }) })
  const reading = execute(page, 'society_read_recovery_review_status', {}).then(value => ({ value, failed: false }), () => ({ value: '', failed: true }))
  await ready; await other.getByRole('button', { name: 'Prepare new person', exact: true }).click(); await fillAccessReview(other, 'Separate current reviewer supplied this preparation while a status read waited.')
  await other.getByRole('button', { name: 'Submit for separate review', exact: true }).click(); await expect(other.getByRole('status').filter({ hasText: 'Preparation retained.' })).toBeVisible(); release()
  expect((await reading).failed).toBe(true); await page.unroute('**/api/recovery/status')
  expect(JSON.parse(await execute(page, 'society_read_recovery_review_status', {}))).toEqual({ pending: 1, approved_preparation: 0, declined: 0, earlier_appointment: 0, unfinished_actions_held: 1, can_resume: false })
  expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_decisions')[0][0]).toBe(0)
  await captureReview(page, 'native-current-review-status-short320')
})

test('a matching nested restore exposes only bounded held status for copied current nominees', async ({ browser }) => {
  const nested = await nestedReviewHost(host, browser)
  try {
    await expect.poll(() => names(nested.page)).toEqual(['society_read_recovery_state'])
    expect(JSON.parse(await execute(nested.page, 'society_read_recovery_state', {}))).toEqual({ recovery_required: true, private_access: 'WITHHELD', financial_changes: 'WITHHELD', external_handoffs: 'WITHHELD', can_resume: false })
    await expect(execute(nested.page, 'society_read_recovery_review_status', {})).rejects.toThrow()
    await expect(execute(nested.page, 'society_resume_recovery', {})).rejects.toThrow()
    await captureReview(nested.page, 'native-nested-copied-nomination-withheld-short320')
  } finally { await nested.close() }
})

test('explicit offline revocation discards held native results and older credentials retain only withheld status', async () => {
  const page = host.one.page; let entered!: () => void, release!: () => void, calls = 0
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route('**/api/recovery/status', async route => { const response = await route.fetch(); if (++calls === 1) { entered(); await held }; await route.fulfill({ response }) })
  const reading = execute(page, 'society_read_recovery_review_status', {}).then(value => ({ value, failed: false }), () => ({ value: '', failed: true }))
  await ready; expect(host.revoke().status).toBe('recovery_nomination_revoked'); release(); expect((await reading).failed).toBe(true)
  await page.unroute('**/api/recovery/status'); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible(); await expect.poll(() => names(page)).toEqual([])
  await page.getByLabel('Email address', { exact: true }).fill(host.one.email); await page.getByLabel('Password', { exact: true }).fill(host.one.password); await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Your records. Kept together.' })).toBeVisible(); await expect.poll(() => names(page)).toEqual(['society_read_recovery_state'])
  expect(JSON.parse(await execute(page, 'society_read_recovery_state', {}))).toEqual({ recovery_required: true, private_access: 'WITHHELD', financial_changes: 'WITHHELD', external_handoffs: 'WITHHELD', can_resume: false })
  await expect(execute(page, 'society_read_recovery_review_status', {})).rejects.toThrow()
  expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_decisions')[0][0]).toBe(0)
  expect(reviewDB(host.target, 'SELECT state,object_version,acknowledged_at FROM offsite_backup_jobs WHERE id=?', [host.oldJob])[0]).toEqual(['FAILED', '', 0])
  await captureReview(page, 'native-revoked-review-withheld-short320')
})
