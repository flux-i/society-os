import { test, expect } from '@playwright/test'
import type { ContextDocument } from './native-webmcp-helpers'
import { execute, names } from './native-webmcp-helpers'
import { captureReview, loginReviewer, nestedReviewHost, reviewDB, reviewHost } from './recovery-review-fixtures'
import { addUnknownGapItem, fillGap } from './recovery-gap-fixtures'

test.describe.configure({ mode: 'serial' })
test.setTimeout(120000)
test.use({ actionTimeout: 10000 })
let host: Awaited<ReturnType<typeof reviewHost>>
const toolName = 'society_read_recovery_gap_status', statusPath = '**/api/recovery/financial-gap/status'
test.beforeAll(async ({ browser }) => { host = await reviewHost(browser); await loginReviewer(host.one); await loginReviewer(host.two) })
test.afterAll(async () => { await host?.close() })

test('actual native financial read exposes only safe counts and keeps human items focus and scroll unchanged', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 375, height: 812 })
  await expect.poll(async () => (await names(page)).sort()).toEqual(['society_read_recovery_gap_status', 'society_read_recovery_numbering_status', 'society_read_recovery_review_status', 'society_read_recovery_state'])
  await page.getByRole('button', { name: 'Financial comparison', exact: true }).click(); await page.getByRole('button', { name: 'Prepare financial assessment', exact: true }).click(); await fillGap(page); await addUnknownGapItem(page)
  const notes = page.getByLabel('Assessment reason', { exact: true }); await notes.focus()
  const before = await page.evaluate(() => ({ hash: location.hash, scroll: scrollY, dialogScroll: document.querySelector('dialog .dialog-scroll')?.scrollTop }))
  const writes: string[] = []; page.on('request', request => { if (!['GET', 'HEAD'].includes(request.method())) writes.push(request.url()) })
  expect(JSON.parse(await execute(page, toolName, {}))).toEqual({ pending: 0, approved_assessments: 0, supplied_known_differences: 0, unresolved_evidence: 0, can_resume: false })
  expect(await page.evaluate(() => ({ hash: location.hash, scroll: scrollY, dialogScroll: document.querySelector('dialog .dialog-scroll')?.scrollTop }))).toEqual(before)
  await expect(notes).toBeFocused(); await expect(notes).toHaveValue('Compare the independently supplied account records while keeping originals and unresolved evidence intact.')
  await expect(page.getByLabel('Item 1 amount', { exact: true })).toHaveValue('250.01'); await expect(page.getByLabel('Item 2 evidence', { exact: true })).toHaveValue('A supplied voluntary fund attribution is unverified and remains outside the known comparison.')
  for (const [name, args] of [[toolName, { home_id: 'private-home' }], [toolName, { evidence: true }], ['society_approve_financial_gap', {}], ['society_post_recovery_money', {}], ['society_resume_recovery', {}]] as const) await expect(execute(page, name, args)).rejects.toThrow()
  expect(writes).toEqual([]); expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_gap_preparations')[0][0]).toBe(0)
  await captureReview(page, 'gap-native-stable-human-form-375')
})

test('actual native cancellation aborts before and during execution without changing the active assessment', async () => {
  const page = host.one.page
  const cancelled = await page.evaluate(async name => { const context = (document as ContextDocument).modelContext, tool = (await context.getTools()).find(tool => tool.name === name)!, controller = new AbortController(); controller.abort(); const major = Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1]); try { await context.executeTool(tool, major < 155 ? '{}' : {}, { signal: controller.signal }); return false } catch { return true } }, toolName)
  expect(cancelled).toBe(true)
  let entered!: () => void, release!: () => void, calls = 0
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route(statusPath, async route => { if (++calls !== 1) { await route.continue(); return }; const response = await route.fetch(); entered(); await held; await route.fulfill({ response }).catch(() => {}) })
  const aborting = page.evaluate(async name => { const context = (document as ContextDocument).modelContext, tool = (await context.getTools()).find(tool => tool.name === name)!, controller = new AbortController(); (window as Window & { gapAbort?: () => void }).gapAbort = () => controller.abort(); const major = Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1]); try { await context.executeTool(tool, major < 155 ? '{}' : {}, { signal: controller.signal }); return false } catch { return true } }, toolName)
  try { await ready; await page.evaluate(() => (window as Window & { gapAbort?: () => void }).gapAbort?.()); release(); expect(await aborting).toBe(true) } finally { release(); await page.unroute(statusPath) }
  await expect(page.getByLabel('Item 1 amount', { exact: true })).toHaveValue('250.01'); expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_gap_preparations')[0][0]).toBe(0)
})

test('captured financial counts are discarded after another real person changes the gap journal', async () => {
  const page = host.one.page, other = host.two.page
  let entered!: () => void, release!: () => void, calls = 0
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route(statusPath, async route => { const response = await route.fetch(); if (++calls === 1) { entered(); await held }; await route.fulfill({ response }) })
  const reading = execute(page, toolName, {}).then(value => ({ value, failed: false }), () => ({ value: '', failed: true }))
  try {
    await ready; await other.getByRole('button', { name: 'Financial comparison', exact: true }).click(); await other.getByRole('button', { name: 'Prepare financial assessment', exact: true }).click(); await fillGap(other); await addUnknownGapItem(other)
    await other.getByRole('button', { name: 'Retain financial assessment', exact: true }).click(); await expect(other.getByRole('status').filter({ hasText: 'Financial assessment retained for another reviewer.' })).toBeVisible(); release()
    expect((await reading).failed).toBe(true)
  } finally { release(); await page.unroute(statusPath) }
  expect(JSON.parse(await execute(page, toolName, {}))).toEqual({ pending: 1, approved_assessments: 0, supplied_known_differences: 1, unresolved_evidence: 1, can_resume: false })
  expect(JSON.parse(await execute(page, 'society_read_recovery_review_status', {})).pending).toBe(1)
  await expect(page.getByLabel('Item 1 amount', { exact: true })).toHaveValue('250.01'); expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_gap_decisions')[0][0]).toBe(0)
  await captureReview(page, 'gap-native-changed-journal-keeps-form-375'); await page.getByRole('button', { name: 'Close financial preparation', exact: true }).click()
})

test('a genuine nested restore retains supplied amounts and evidence but withholds copied review authority', async ({ browser }) => {
  const nested = await nestedReviewHost(host, browser, false, true)
  try {
    await expect.poll(() => names(nested.page)).toEqual(['society_read_recovery_state'])
    await expect(execute(nested.page, toolName, {})).rejects.toThrow(); await expect(execute(nested.page, 'society_approve_financial_gap', {})).rejects.toThrow()
    expect((await nested.page.request.get('/api/recovery/financial-gap/status')).status()).toBe(423)
    expect(reviewDB(nested.target, "SELECT json_extract(comparison_json,'$.homes[0].checkpoint_paise'),json_extract(comparison_json,'$.homes[0].comparison_paise'),unresolved_count FROM recovery_gap_preparations")[0]).toEqual([150001, 125000, 1])
    await captureReview(nested.page, 'gap-native-nested-copied-review-held-short320')
  } finally { await nested.close() }
})

test('actual offline revocation removes native financial tools and discards a held status result', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 320, height: 440 })
  let entered!: () => void, release!: () => void, calls = 0
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route(statusPath, async route => { const response = await route.fetch(); if (++calls === 1) { entered(); await held }; await route.fulfill({ response }) })
  const reading = execute(page, toolName, {}).then(value => ({ value, failed: false }), () => ({ value: '', failed: true }))
  try { await ready; expect(host.revoke().status).toBe('recovery_nomination_revoked'); release(); expect((await reading).failed).toBe(true) } finally { release(); await page.unroute(statusPath) }
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible(); await expect.poll(() => names(page)).toEqual([])
  await page.getByLabel('Email address', { exact: true }).fill(host.one.email); await page.getByLabel('Password', { exact: true }).fill(host.one.password); await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Your records. Kept together.', exact: true })).toBeVisible(); await expect.poll(() => names(page)).toEqual(['society_read_recovery_state'])
  await expect(execute(page, toolName, {})).rejects.toThrow(); expect((await page.request.get('/api/recovery/financial-gap/status')).status()).toBe(423)
  expect(reviewDB(host.target, 'SELECT amount_paise FROM entries WHERE id IN(SELECT entry_id FROM receipts)')[0][0]).toBe(40000)
  await captureReview(page, 'gap-native-revoked-copied-identity-short320')
})
