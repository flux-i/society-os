import { test, expect } from '@playwright/test'
import type { APIResponse } from '@playwright/test'
import type { RecoveryInput, RecoveryProposal } from '../src/recovery-review'
import { chooseOption } from './helpers'
import { pageFits, dialogFits } from './registry-import-fixtures'
import { captureReview, fillAccessReview, loginReviewer, nestedReviewHost, reviewDB, reviewHost } from './recovery-review-fixtures'

test.describe.configure({ mode: 'serial' })
test.setTimeout(120000)
test.use({ actionTimeout: 10000 })
let host: Awaited<ReturnType<typeof reviewHost>>
test.beforeAll(async ({ browser }) => { host = await reviewHost(browser); await loginReviewer(host.one); await loginReviewer(host.two) })
test.afterAll(async () => { await host?.close() })

test('actual offline nomination and real personal MFA expose only the changing recovery desk', async () => {
  const page = host.one.page
  for (const path of ['/api/flats', '/api/entries', '/api/receipts/' + host.receiptID + '/download', '/api/documents', '/api/messages', '/api/admin/accounts']) expect((await page.request.get(path)).status(), path).toBe(423)
  const me = await (await page.request.get('/api/auth/me')).json(), headers = { Origin: new URL(page.url()).origin, 'X-CSRF-Token': me.csrf_token }
  expect((await page.request.post('/api/entries/' + host.draft + '/post', { headers, data: { operation_key: crypto.randomUUID(), confirmed: true, reason: '' } })).status()).toBe(423)
  expect(reviewDB(host.target, 'SELECT amount_paise,state FROM entries WHERE id=?', [host.draft])[0]).toEqual([40001, 'DRAFT'])
  expect(reviewDB(host.target, 'SELECT COUNT(*) FROM role_grants WHERE user_id IN(SELECT user_id FROM recovery_reviewers)')[0][0]).toBe(0)
  const summary = await (await page.request.get('/api/recovery/status')).json(); expect(summary.pending).toBe(0); expect(summary.approved).toBe(0); expect(summary.retained_work).toBe(1); expect(summary.can_resume).toBe(false)
  await page.setViewportSize({ width: 1440, height: 1000 }); await pageFits(page); await captureReview(page, 'current-review-desktop')
  await page.setViewportSize({ width: 820, height: 1050 }); await pageFits(page); await captureReview(page, 'current-review-tablet')
  await page.getByRole('button', { name: 'Review journal', exact: true }).click(); await expect(page.getByRole('heading', { name: 'Room for the first careful review.' })).toBeVisible(); await captureReview(page, 'review-journal-empty-tablet')
})

test('visible access preparation opens shared menus, freezes supplied evidence and prevents proposer approval', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 375, height: 812 })
  await page.getByRole('button', { name: 'Current access', exact: true }).click(); await page.getByRole('button', { name: 'Prepare new person', exact: true }).click()
  await expect(page.getByRole('dialog')).toBeVisible(); await dialogFits(page)
  const role = page.getByRole('combobox', { name: 'Proposed access', exact: true }); await role.focus(); await page.keyboard.press('Enter')
  await expect(page.getByRole('option', { name: 'Household member', exact: true })).toBeVisible(); await page.getByRole('option', { name: 'Household member', exact: true }).hover(); await captureReview(page, 'access-menu-hover-375')
  await page.keyboard.press('Escape'); await expect(role).toBeFocused(); await expect(page.getByRole('dialog')).toHaveCount(1)
  await fillAccessReview(page); await captureReview(page, 'access-preparation-scrolled-375')
  await page.getByRole('button', { name: 'Submit for separate review', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Preparation retained.' })).toBeVisible()
  expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_proposals')[0][0]).toBe(1)
  await page.getByRole('button', { name: 'Open review', exact: true }).click(); await expect(page.getByText('Another current reviewer must decide your preparation.', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Save separate decision', exact: true })).toHaveCount(0); await captureReview(page, 'proposer-separate-review-required-375')
  await page.getByRole('button', { name: 'Close frozen recovery review', exact: true }).click()
})

test('a second reviewer checks the frozen proposal and resolves a lost reply without another decision', async () => {
  const page = host.two.page; await page.setViewportSize({ width: 320, height: 440 })
  await page.getByRole('button', { name: 'Review journal', exact: true }).click(); await page.getByRole('button', { name: 'Open review', exact: true }).click(); await expect(page.getByRole('combobox', { name: 'Separate review decision', exact: true })).toBeVisible(); await dialogFits(page)
  await chooseOption(page, 'Separate review decision', 'Decline preparation'); await chooseOption(page, 'Separate review decision', 'Approve preparation')
  await page.getByRole('textbox', { name: 'Decision reason', exact: true }).fill('Independently checked the current supplied person, home and term against separate evidence.')
  await page.getByLabel('Independent reviewer attestation', { exact: true }).fill('I checked this exact frozen proposal independently of its author.')
  await page.getByRole('checkbox', { name: 'I independently checked this frozen version and its supplied evidence.' }).check()
  let writes = 0
  await page.route('**/api/recovery/proposals/*/decision', async route => { if (++writes === 1) { const response = await route.fetch(); expect(response.status()).toBe(200); await route.abort('failed') } else await route.continue() })
  await page.getByRole('button', { name: 'Save separate decision', exact: true }).click()
  await expect(page.getByRole('alert').filter({ hasText: 'The reply is uncertain.' })).toBeVisible(); await expect(page.getByRole('button', { name: 'Close frozen recovery review', exact: true })).toBeDisabled()
  await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(1)
  await expect(page.getByRole('textbox', { name: 'Decision reason', exact: true })).toHaveValue('Independently checked the current supplied person, home and term against separate evidence.')
  await captureReview(page, 'separate-decision-uncertain-short320')
  await page.getByRole('button', { name: 'Check saved result', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Separate decision retained.' })).toBeVisible()
  expect(writes).toBe(1); expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_decisions')[0][0]).toBe(1)
  await page.unroute('**/api/recovery/proposals/*/decision'); await page.getByRole('button', { name: 'Open review', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Approved preparation', exact: true })).toBeVisible(); await captureReview(page, 'approved-review-history-short320')
  await page.getByRole('button', { name: 'Close frozen recovery review', exact: true }).click()
})

test('unfinished work preserves its original identity and review filters, loading, failure and retry fit small phones', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 320, height: 740 })
  await page.getByRole('button', { name: 'Unfinished work', exact: true }).click(); await expect(page.getByRole('button', { name: 'Prepare work review', exact: true })).toBeVisible()
  await captureReview(page, 'held-original-work-320'); await page.getByRole('button', { name: 'Prepare work review', exact: true }).click(); await dialogFits(page)
  await expect(page.getByRole('combobox', { name: 'Supplied outcome', exact: true })).toContainText('Keep outcome uncertain')
  await page.getByRole('combobox', { name: 'Supplied outcome', exact: true }).click(); await page.getByRole('option', { name: 'Independent evidence recorded', exact: true }).hover(); await captureReview(page, 'work-outcome-menu-320')
  await page.getByRole('option', { name: 'Keep outcome uncertain', exact: true }).click()
  await page.getByLabel('Independent evidence reference', { exact: true }).fill('independent-custody-check-has-no-acknowledgement')
  await page.getByRole('textbox', { name: 'Review notes', exact: true }).fill('The independent review has no external acknowledgement; retain uncertainty and the original.')
  await page.getByRole('checkbox', { name: 'I checked this supplied evidence and understand this prepares a decision while recovery remains held.' }).check()
  await page.getByRole('button', { name: 'Submit for separate review', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Preparation retained.' })).toBeVisible()
  expect(reviewDB(host.target, 'SELECT state,attempts,object_version,acknowledged_at FROM offsite_backup_jobs WHERE id=?', [host.oldJob])[0]).toEqual(['FAILED', 5, '', 0])
  await chooseOption(page, 'Review state', 'Awaiting review'); await expect(page.getByRole('button', { name: 'Open review', exact: true })).toHaveCount(1)
  await page.getByRole('combobox', { name: 'Review kind', exact: true }).click(); await page.getByRole('option', { name: 'Unfinished work', exact: true }).hover(); await captureReview(page, 'journal-filter-menu-320'); await page.getByRole('option', { name: 'Unfinished work', exact: true }).click()
  let calls = 0
  await page.route('**/api/recovery/accounts?**', async route => { if (++calls === 1) await route.fulfill({ status: 500, json: { error: 'controlled_read_failure' } }); else await route.continue() })
  await page.getByRole('button', { name: 'Current access', exact: true }).click(); await expect(page.getByRole('alert')).toBeVisible(); await captureReview(page, 'accounts-error-retry-320')
  await page.getByRole('button', { name: 'Try again', exact: true }).click(); await expect(page.getByRole('button', { name: 'Prepare access', exact: true }).first()).toBeVisible(); await page.unroute('**/api/recovery/accounts?**')
  for (const [width, height] of [[375, 812], [320, 740], [320, 440]]) { await page.setViewportSize({ width, height }); await page.evaluate(() => window.scrollTo(0, 0)); await pageFits(page); await captureReview(page, 'review-progress-' + width + '-' + height); await page.getByRole('button', { name: 'Prepare new person', exact: true }).click(); await dialogFits(page); await captureReview(page, 'review-dialog-opening-' + width + '-' + height); await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0) }
  const current = await (await page.request.get('/api/recovery/status')).json(); expect(current.approved).toBe(1); expect(current.pending).toBe(1); expect(current.can_resume).toBe(false)
})

test('offline revocation removes current review access and preserves the original manual finance journal', async () => {
  const page = host.one.page; expect(host.revoke().status).toBe('recovery_nomination_revoked')
  expect((await page.request.get('/api/recovery/status')).status()).toBe(401)
  await page.evaluate(() => window.dispatchEvent(new Event('session-recheck'))); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
  await page.getByLabel('Email address', { exact: true }).fill(host.one.email); await page.getByLabel('Password', { exact: true }).fill(host.one.password); await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Your records. Kept together.' })).toBeVisible(); await expect(page.getByRole('button', { name: 'Prepare new person', exact: true })).toHaveCount(0)
  const me = await (await page.request.get('/api/auth/me')).json(); expect(me.recovery_reviewer).toBe(false); expect(me.can_review_recovery).toBe(false); expect(me.roles).toEqual([])
  expect(reviewDB(host.target, 'SELECT amount_paise FROM entries WHERE id IN(SELECT entry_id FROM receipts)')[0][0]).toBe(40000)
  expect(reviewDB(host.target, "SELECT SUM(CASE WHEN e.kind IN('OPENING_DEBIT','CHARGE') THEN e.amount_paise ELSE -e.amount_paise END) FROM entries e WHERE state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals WHERE entry_id=e.id)")[0][0]).toBe(150001)
  await captureReview(page, 'revoked-review-authority-withheld-320')
})

test('a factor verification reply received after sign-out cannot reopen the recovery desk', async () => {
  const page = host.one.page; expect(host.renominate().reviewers).toBe(2)
  await page.goto('/'); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeEnabled()
  await page.getByLabel('Email address', { exact: true }).fill(host.one.email); await page.getByLabel('Password', { exact: true }).fill(host.one.password); await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Use a recovery code', exact: true })).toBeVisible(); await page.getByRole('button', { name: 'Use a recovery code', exact: true }).click()
  const code = host.one.recovery.shift(); expect(code).toBeTruthy(); await page.getByLabel('Recovery code', { exact: true }).fill(code!)
  let entered!: () => void, release!: () => void
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route('**/api/auth/mfa/verify', async route => { const response = await route.fetch(); expect(response.status()).toBe(200); entered(); await held; await route.fulfill({ response }) })
  await page.getByRole('button', { name: 'Verify and continue', exact: true }).click(); await ready
  await page.getByRole('button', { name: 'Sign out and start again', exact: true }).click(); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
  await page.evaluate(() => { const value = window as Window & { reviewReopened?: boolean }; value.reviewReopened = false; new MutationObserver(() => { if (document.querySelector('.review-desk')) value.reviewReopened = true }).observe(document.body, { childList: true, subtree: true }) })
  const finished = page.waitForResponse(response => response.url().endsWith('/api/auth/mfa/verify')); release(); await finished
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
  expect(await page.evaluate(() => (window as Window & { reviewReopened?: boolean }).reviewReopened)).toBe(false)
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible(); expect((await page.request.get('/api/auth/me')).status()).toBe(401)
  await page.unroute('**/api/auth/mfa/verify'); await captureReview(page, 'late-factor-reply-keeps-signout-320')
})

test('an uncertain preparation retries the same identity and preserves the human inputs once', async () => {
  const page = host.one.page; await loginReviewer(host.one); await page.setViewportSize({ width: 320, height: 440 })
  await page.getByRole('button', { name: 'Prepare new person', exact: true }).click(); await fillAccessReview(page, 'Retain these supplied details while the proposal reply is uncertain.')
  const before = Number(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_proposals')[0][0]), keys: string[] = []
  await page.route('**/api/recovery/proposals', async route => { if (route.request().method() !== 'POST') { await route.continue(); return }; keys.push(route.request().postDataJSON().operation_key); if (keys.length === 1) { const response = await route.fetch(); expect(response.status()).toBe(200); await route.abort('failed') } else await route.continue() })
  await page.getByRole('button', { name: 'Submit for separate review', exact: true }).click(); await expect(page.getByRole('alert').filter({ hasText: 'The reply is uncertain.' })).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Review notes', exact: true })).toHaveValue('Retain these supplied details while the proposal reply is uncertain.')
  await expect(page.getByRole('button', { name: 'Close recovery preparation', exact: true })).toBeDisabled(); await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(1)
  await page.getByRole('button', { name: 'Retry same submission', exact: true }).scrollIntoViewIfNeeded(); await captureReview(page, 'uncertain-preparation-same-retry-short320')
  await page.getByRole('button', { name: 'Retry same submission', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Preparation retained.' })).toBeVisible()
  expect(keys).toHaveLength(2); expect(keys[0]).toBe(keys[1]); expect(Number(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_proposals')[0][0])).toBe(before + 1)
  await page.unroute('**/api/recovery/proposals')
})


test('replaced approvals are earlier appointments and a current linked family correction keeps their evidence', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 375, height: 812 })
  const old = reviewDB(host.target, "SELECT p.id,p.input_json FROM recovery_proposals p JOIN recovery_decisions d ON d.proposal_id=p.id WHERE d.decision='APPROVED'")[0]
  const before = reviewDB(host.target, 'SELECT * FROM recovery_decisions'), oldInput = JSON.parse(String(old[1]))
  const summary = await (await page.request.get('/api/recovery/status')).json(); expect(summary.approved).toBe(0); expect(summary.stale).toBe(2); expect(summary.pending).toBe(1)
  await page.getByRole('button', { name: 'Review journal', exact: true }).click(); await chooseOption(page, 'Review kind', 'Every review kind'); await chooseOption(page, 'Review state', 'Approved preparation')
  await expect(page.getByRole('heading', { name: 'Room for the first careful review.' })).toBeVisible()
  await chooseOption(page, 'Review state', 'Earlier appointment'); await expect(page.getByRole('button', { name: 'Open review', exact: true })).toHaveCount(2)
  let failed = true
  await page.route('**/api/recovery/proposals/*/history?**', async route => { if (failed) await route.fulfill({ status: 500, json: { error: 'controlled_history_failure' } }); else await route.continue() })
  await page.locator('.review-desk-list > article').filter({ has: page.getByRole('heading', { name: oldInput.title, exact: true }) }).getByRole('button', { name: 'Open review', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Approved preparation', exact: true })).toBeVisible(); await expect(page.getByRole('button', { name: 'Save separate decision', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Try history again', exact: true })).toBeVisible(); await captureReview(page, 'earlier-approval-history-error-375')
  failed = false; await page.getByRole('button', { name: 'Try history again', exact: true }).click(); await expect(page.getByText('Version 2 · Earlier appointment', { exact: true })).toBeVisible(); await captureReview(page, 'earlier-approval-history-retained-375'); await page.unroute('**/api/recovery/proposals/*/history?**')
  await page.getByRole('button', { name: 'Prepare linked correction', exact: true }).click(); await dialogFits(page)
  await expect(page.getByLabel('Independent person reference', { exact: true })).toHaveValue(oldInput.access.identity_ref)
  await chooseOption(page, 'Supplied relationship', 'Authorized occupant'); await page.getByRole('combobox', { name: 'Supplied relationship', exact: true }).click(); await page.getByRole('option', { name: 'Family member', exact: true }).hover(); await captureReview(page, 'current-family-correction-menu-375'); await page.getByRole('option', { name: 'Family member', exact: true }).click()
  await page.getByRole('textbox', { name: 'Review notes', exact: true }).fill('Current independent evidence confirms this supplied person as a family member after reappointment.')
  await page.getByRole('checkbox', { name: 'I checked this supplied evidence and understand this prepares a decision while recovery remains held.' }).check()
  await page.getByRole('button', { name: 'Submit for separate review', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Preparation retained.' })).toBeVisible()
  const linked = reviewDB(host.target, 'SELECT version,input_json FROM recovery_proposals WHERE predecessor=?', [old[0]])[0]; expect(linked[0]).toBe(3); expect(JSON.parse(String(linked[1])).access.relationship).toBe('FAMILY')
  expect(reviewDB(host.target, 'SELECT * FROM recovery_decisions')).toEqual(before); expect(reviewDB(host.target, 'SELECT input_json FROM recovery_proposals WHERE id=?', [old[0]])[0][0]).toBe(old[1])
  await captureReview(page, 'linked-current-correction-journal-375')
})

test('home loading and retry, independent external review and real reauthentication preserve the hold', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 320, height: 440 })
  await page.getByRole('button', { name: 'Current access', exact: true }).click()
  let release!: () => void, entered!: () => void
  const held = new Promise<void>(resolve => release = resolve), ready = new Promise<void>(resolve => entered = resolve)
  await page.route('**/api/recovery/homes', async route => { entered(); await held; await route.fulfill({ status: 500, json: { error: 'controlled_home_failure' } }) })
  await page.getByRole('button', { name: 'Prepare new person', exact: true }).click(); await ready
  await chooseOption(page, 'Proposed access', 'Household member'); await expect(page.getByRole('combobox', { name: 'Verified home', exact: true })).toBeDisabled(); await captureReview(page, 'home-loading-short320')
  release(); await expect(page.getByRole('button', { name: 'Try homes again', exact: true })).toBeVisible(); await captureReview(page, 'home-error-retry-short320')
  await page.unroute('**/api/recovery/homes'); await page.getByRole('button', { name: 'Try homes again', exact: true }).click(); await chooseOption(page, 'Verified home', 'A-101'); await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.getByRole('button', { name: 'Unfinished work', exact: true }).click(); await page.getByRole('button', { name: 'Prepare work review', exact: true }).click()
  await chooseOption(page, 'Supplied outcome', 'Close out without replay'); await page.getByLabel('Independent evidence reference', { exact: true }).fill('supplied-current-custody-check-no-provider-replay')
  await page.getByRole('textbox', { name: 'Review notes', exact: true }).fill('Current custodians supplied evidence to close review without replaying the retained failed action.')
  await page.getByRole('checkbox', { name: 'I checked this supplied evidence and understand this prepares a decision while recovery remains held.' }).check(); await page.getByRole('button', { name: 'Submit for separate review', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Preparation retained.' })).toBeVisible()
  await loginReviewer(host.two); const reviewer = host.two.page, me = await (await reviewer.request.get('/api/auth/me')).json()
  reviewDB(host.target, "UPDATE sessions SET reauthenticated_at=unixepoch('now')-301 WHERE user_id=?", [me.id])
  await reviewer.evaluate(() => window.dispatchEvent(new Event('session-recheck')))
  await expect.poll(async () => (await (await reviewer.request.get('/api/auth/me')).json()).fresh_authentication).toBe(false)
  await reviewer.setViewportSize({ width: 320, height: 440 }); await reviewer.getByRole('button', { name: 'Review journal', exact: true }).click(); await chooseOption(reviewer, 'Review kind', 'Unfinished work'); await chooseOption(reviewer, 'Review state', 'Awaiting review'); await reviewer.getByRole('button', { name: 'Open review', exact: true }).click()
  await expect(reviewer.getByRole('button', { name: 'Save separate decision', exact: true })).toBeDisabled(); await reviewer.getByRole('button', { name: 'Confirm identity to continue', exact: true }).click()
  const reauth = reviewer.getByRole('dialog').filter({ has: reviewer.getByRole('heading', { name: 'Confirm it’s you.' }) })
  await reauth.getByLabel('Current password', { exact: true }).fill('Wrong-supplied-password-2026!'); await reauth.getByRole('checkbox', { name: 'Use an unused recovery code.' }).check()
  const code = host.two.recovery.shift(); expect(code).toBeTruthy(); await reauth.getByLabel('Unused recovery code', { exact: true }).fill(code!)
  await reauth.getByRole('button', { name: 'Confirm identity', exact: true }).click(); await expect(reauth.getByRole('alert')).toBeVisible(); await captureReview(reviewer, 'identity-confirmation-error-short320')
  await reauth.getByLabel('Current password', { exact: true }).fill(host.two.password); await reauth.getByRole('button', { name: 'Confirm identity', exact: true }).click(); await expect(reauth).toHaveCount(0)
  const frozen = reviewer.getByRole('dialog'); await expect(frozen).toHaveCount(1); await dialogFits(reviewer)
  await frozen.getByRole('textbox', { name: 'Decision reason', exact: true }).fill('Independent evidence supports closing this preparation while the original external attempt stays held.')
  await frozen.getByLabel('Independent reviewer attestation', { exact: true }).fill('I separately checked the retained action and this supplied evidence.')
  await frozen.getByRole('checkbox', { name: 'I independently checked this frozen version and its supplied evidence.' }).check(); await captureReview(reviewer, 'independent-work-decision-short320')
  await frozen.getByRole('button', { name: 'Save separate decision', exact: true }).click(); await expect(reviewer.getByRole('status').filter({ hasText: 'Separate decision retained.' })).toBeVisible()
  expect(reviewDB(host.target, 'SELECT state,attempts,object_version,acknowledged_at FROM offsite_backup_jobs WHERE id=?', [host.oldJob])[0]).toEqual(['FAILED', 5, '', 0])
  expect((await (await reviewer.request.get('/api/recovery/status')).json()).can_resume).toBe(false)
})

test('bounded account, journal and linked-history pages render their actual controls and read loading states', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 820, height: 1050 }); await page.getByRole('button', { name: 'Current access', exact: true }).click()
  await expect(page.getByRole('navigation', { name: 'Recovery result pages' })).toBeVisible(); await page.getByRole('button', { name: 'Next', exact: true }).click(); await expect(page.getByText(/^Page 2 ·/)).toBeVisible(); await captureReview(page, 'retained-people-second-page-tablet'); await page.getByRole('button', { name: 'Previous', exact: true }).click()
  await page.getByRole('searchbox', { name: 'Find a retained person', exact: true }).fill('no-supplied-person-matches'); await expect(page.getByRole('heading', { name: 'No retained people match this search.' })).toBeVisible(); await captureReview(page, 'people-search-empty-tablet'); await page.getByRole('searchbox', { name: 'Find a retained person', exact: true }).fill('')
  const one = await (await page.request.get('/api/auth/me')).json(), two = await (await host.two.page.request.get('/api/auth/me')).json(), now = Math.floor(Date.now() / 1000)
  let previous: RecoveryProposal | null = null
  for (let index = 0; index < 13; index++) {
    const input: RecoveryInput = { operation_key: crypto.randomUUID(), scope_key: one.scope_key, kind: 'ACCESS', predecessor_id: previous?.id ?? '', predecessor_version: previous?.version ?? 0, title: 'Paged independent review ' + String(index).padStart(2, '0'), reason: 'Independent bounded history fixture with supplied current person evidence.', access: { account_id: '', email: 'history.person@example.test', name: 'Supplied History Person', identity_ref: 'independent-history-person', role: 'HOUSEHOLD', home_id: 'demo-flat-A-101', relationship: 'TENANT', finance: false, valid_from: now - 1, valid_until: now + 86400, verified_at: now - 1, evidence_ref: 'supplied-independent-history-evidence' }, work: null }
    const made: APIResponse = await page.request.post('/api/recovery/proposals', { headers: { Origin: new URL(page.url()).origin, 'X-CSRF-Token': one.csrf_token }, data: input }); expect(made.status()).toBe(200); previous = await made.json()
    if (index < 12) { const decided: APIResponse = await host.two.page.request.post('/api/recovery/proposals/' + previous!.id + '/decision', { headers: { Origin: new URL(host.two.page.url()).origin, 'X-CSRF-Token': two.csrf_token }, data: { operation_key: crypto.randomUUID(), scope_key: two.scope_key, version: previous!.version, decision: 'DECLINED', reason: 'Supplied current evidence needs this linked follow-up version.', attestation: 'I independently checked and decline this frozen preparation.', confirmed: true } }); expect(decided.status()).toBe(200); previous = await decided.json() }
  }
  await page.getByRole('button', { name: 'Refresh review', exact: true }).click(); await page.getByRole('button', { name: 'Review journal', exact: true }).click(); await chooseOption(page, 'Review kind', 'Every review kind'); await chooseOption(page, 'Review state', 'Every review state')
  await page.getByRole('button', { name: 'Next', exact: true }).click(); await expect(page.getByText(/^Page 2 ·/)).toBeVisible(); await captureReview(page, 'review-journal-second-page-tablet'); await page.getByRole('button', { name: 'Previous', exact: true }).click()
  let entered!: () => void, release!: () => void; const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route('**/api/recovery/proposals/*/history?**', async route => { const response = await route.fetch(); entered(); await held; await route.fulfill({ response }) })
  await page.locator('.review-desk-list > article').filter({ has: page.getByRole('heading', { name: 'Paged independent review 12', exact: true }) }).getByRole('button', { name: 'Open review', exact: true }).click(); await ready; await expect(page.getByRole('status').filter({ hasText: 'Opening review history…' })).toBeVisible(); await captureReview(page, 'review-history-loading-tablet'); release()
  await expect(page.getByRole('region', { name: 'Retained review history' }).getByRole('article')).toHaveCount(12); await page.unroute('**/api/recovery/proposals/*/history?**')
  await page.setViewportSize({ width: 320, height: 440 }); await dialogFits(page); const history = page.getByRole('region', { name: 'Retained review history' }); await history.getByRole('button', { name: 'Next', exact: true }).click(); await expect(history.getByRole('article')).toHaveCount(1)
  const pager = history.getByRole('navigation', { name: 'Recovery result pages' }); await pager.scrollIntoViewIfNeeded(); await captureReview(page, 'linked-history-second-page-short320')
  const previousBounds = await pager.getByRole('button', { name: 'Previous', exact: true }).boundingBox(), nextBounds = await pager.getByRole('button', { name: 'Next', exact: true }).boundingBox()
  expect(previousBounds).not.toBeNull(); expect(nextBounds).not.toBeNull(); expect(Math.abs(previousBounds!.y - nextBounds!.y)).toBeLessThanOrEqual(1)
  await history.getByRole('button', { name: 'Previous', exact: true }).click(); await expect(history.getByRole('article')).toHaveCount(12); await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0)
  expect(reviewDB(host.target, 'SELECT amount_paise FROM entries WHERE id IN(SELECT entry_id FROM receipts)')[0][0]).toBe(40000)
})


test('detail loading and failure retain a dismissible dialog and retry the exact selected review', async () => {
  const page = host.one.page; await page.setViewportSize({ width: 320, height: 440 })
  const detailURL = /\/api\/recovery\/proposals\/[A-Za-z0-9_-]{43}$/
  let entered!: () => void, release!: () => void; const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route(detailURL, async route => { entered(); await held; await route.fulfill({ status: 500, json: { error: 'controlled_detail_failure' } }) })
  await page.locator('.review-desk-list > article').filter({ has: page.getByRole('heading', { name: 'Paged independent review 12', exact: true }) }).getByRole('button', { name: 'Open review', exact: true }).click(); await ready
  await expect(page.getByRole('heading', { name: 'Opening this review…', exact: true })).toBeVisible(); await dialogFits(page); await captureReview(page, 'selected-review-loading-short320'); release()
  await expect(page.getByRole('heading', { name: 'A little pause.', exact: true })).toBeVisible(); await dialogFits(page); await captureReview(page, 'selected-review-error-short320')
  await page.unroute(detailURL); await page.getByRole('dialog').getByRole('button', { name: 'Try again', exact: true }).click(); await expect(page.getByRole('dialog').getByRole('heading', { name: 'Paged independent review 12', exact: true })).toBeVisible(); await dialogFits(page)
  await page.mouse.click(5, 5); await expect(page.getByRole('dialog')).toHaveCount(0)
})


test('a matching nested restore retains nominations and decisions while withholding their earlier authority', async ({ browser }) => {
  const nested = await nestedReviewHost(host, browser)
  try {
    expect(nested.proof.prior_decisions).toBeGreaterThan(0)
    await pageFits(nested.page); await captureReview(nested.page, 'nested-copied-nomination-withheld-short320')
    await expect(nested.page.getByRole('button', { name: 'Prepare new person', exact: true })).toHaveCount(0)
    expect(reviewDB(host.target, 'SELECT amount_paise FROM entries WHERE id IN(SELECT entry_id FROM receipts)')[0][0]).toBe(40000)
  } finally { await nested.close() }
})


test('expired fresh proof during an uncertain retry keeps the original operation locked until reauthentication', async () => {
  const page = host.one.page; await page.getByRole('button', { name: 'Current access', exact: true }).click(); await page.getByRole('button', { name: 'Prepare new person', exact: true }).click()
  await fillAccessReview(page, 'Preserve this exact supplied evidence across uncertain retry and fresh-identity expiry.')
  const before = Number(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_proposals')[0][0]), me = await (await page.request.get('/api/auth/me')).json(), keys: string[] = []
  await page.route('**/api/recovery/proposals', async route => { if (route.request().method() !== 'POST') { await route.continue(); return }; keys.push(route.request().postDataJSON().operation_key); if (keys.length === 1) { const response = await route.fetch(); expect(response.status()).toBe(200); await route.abort('failed') } else await route.continue() })
  await page.getByRole('button', { name: 'Submit for separate review', exact: true }).click(); await expect(page.getByRole('alert').filter({ hasText: 'The reply is uncertain.' })).toBeVisible()
  reviewDB(host.target, "UPDATE sessions SET reauthenticated_at=unixepoch('now')-301 WHERE user_id=?", [me.id])
  const denied = page.waitForResponse(response => response.url().endsWith('/api/recovery/proposals') && response.status() === 403)
  await page.getByRole('button', { name: 'Retry same submission', exact: true }).click(); expect((await denied).status()).toBe(403)
  await expect(page.getByRole('button', { name: 'Retry same submission', exact: true })).toBeEnabled()
  await expect(page.getByRole('button', { name: 'Close recovery preparation', exact: true })).toBeDisabled()
  await expect(page.getByRole('textbox', { name: 'Review notes', exact: true })).toBeDisabled(); await expect(page.getByRole('textbox', { name: 'Review notes', exact: true })).toHaveValue('Preserve this exact supplied evidence across uncertain retry and fresh-identity expiry.')
  await expect(page.getByRole('button', { name: 'Retry same submission', exact: true })).toBeVisible(); expect(keys).toHaveLength(2); expect(keys[1]).toBe(keys[0])
  await page.getByRole('button', { name: 'Confirm identity to continue', exact: true }).click()
  const reauth = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Confirm it’s you.' }) })
  await reauth.getByLabel('Current password', { exact: true }).fill(host.one.password); await reauth.getByRole('checkbox', { name: 'Use an unused recovery code.' }).check()
  const code = host.one.recovery.shift(); expect(code).toBeTruthy(); await reauth.getByLabel('Unused recovery code', { exact: true }).fill(code!); await reauth.getByRole('button', { name: 'Confirm identity', exact: true }).click(); await expect(reauth).toHaveCount(0)
  await page.setViewportSize({ width: 320, height: 440 }); await page.getByRole('button', { name: 'Retry same submission', exact: true }).scrollIntoViewIfNeeded(); await captureReview(page, 'uncertain-retry-after-real-reauth-short320')
  await page.getByRole('button', { name: 'Retry same submission', exact: true }).click(); await expect(page.getByRole('status').filter({ hasText: 'Preparation retained.' })).toBeVisible()
  expect(keys).toHaveLength(3); expect(new Set(keys).size).toBe(1); expect(Number(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_proposals')[0][0])).toBe(before + 1)
  await page.unroute('**/api/recovery/proposals'); expect(reviewDB(host.target, 'SELECT amount_paise FROM entries WHERE id IN(SELECT entry_id FROM receipts)')[0][0]).toBe(40000)
})
