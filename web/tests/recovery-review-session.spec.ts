import { test, expect } from '@playwright/test'
import { pageFits } from './registry-import-fixtures'
import { captureReview, loginReviewer, reviewDB, reviewHost } from './recovery-review-fixtures'

test.setTimeout(120000)
test('an earlier successful identity reply cannot reopen recovery review after offline revocation', async ({ browser }) => {
  const host = await reviewHost(browser)
  let release: (() => void) | undefined
  try {
    await loginReviewer(host.one)
    const page = host.one.page
    await page.setViewportSize({ width: 320, height: 440 })
    await page.getByRole('button', { name: 'Confirm identity', exact: true }).click()
    await page.getByLabel('Current password', { exact: true }).fill(host.one.password)
    await page.getByRole('checkbox', { name: 'Use an unused recovery code.' }).check()
    const code = host.one.recovery.shift(); expect(code).toBeTruthy()
    await page.getByLabel('Unused recovery code', { exact: true }).fill(code!)
    let entered!: () => void
    const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
    await page.route('**/api/auth/reauthenticate', async route => {
      const response = await route.fetch(); expect(response.status()).toBe(200)
      entered(); await held; await route.fulfill({ response })
    })
    await page.getByRole('dialog').getByRole('button', { name: 'Confirm identity', exact: true }).click()
    await ready
    expect(host.revoke().status).toBe('recovery_nomination_revoked')
    expect((await page.request.get('/api/auth/me')).status()).toBe(401)
    await page.evaluate(() => window.dispatchEvent(new Event('session-recheck')))
    await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    // Observe even a transient mount before a subsequent protected read fails.
    await page.evaluate(() => {
      const value = window as Window & { reviewReopened?: boolean }; value.reviewReopened = false
      new MutationObserver(() => { if (document.querySelector('.review-desk')) value.reviewReopened = true }).observe(document.body, { childList: true, subtree: true })
    })
    const completed = page.waitForResponse(response => response.url().endsWith('/api/auth/reauthenticate'))
    release!(); await completed
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
    expect(await page.evaluate(() => (window as Window & { reviewReopened?: boolean }).reviewReopened)).toBe(false)
    await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
    expect((await page.request.get('/api/auth/me')).status()).toBe(401)
    await page.unroute('**/api/auth/reauthenticate')
    await pageFits(page); await captureReview(page, 'late-identity-reply-keeps-revocation-short320')
    await page.getByLabel('Email address', { exact: true }).fill(host.one.email)
    await page.getByLabel('Password', { exact: true }).fill(host.one.password)
    await page.getByRole('button', { name: 'Sign in', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Your records. Kept together.' })).toBeVisible()
    const me = await (await page.request.get('/api/auth/me')).json()
    expect(me.recovery_required).toBe(true); expect(me.recovery_reviewer).toBe(false); expect(me.can_review_recovery).toBe(false); expect(me.roles).toEqual([])
    for (const key of Object.keys(me).filter(key => key.startsWith('can_'))) expect(me[key], key).toBe(false)
    expect((await page.request.get('/api/receipts/' + host.receiptID + '/download')).status()).toBe(423)
    expect((await page.request.get('/api/recovery/status')).status()).toBe(423)
    expect(reviewDB(host.target, 'SELECT amount_paise FROM entries WHERE id IN(SELECT entry_id FROM receipts)')[0][0]).toBe(40000)
    expect(reviewDB(host.target, "SELECT SUM(CASE WHEN e.kind IN('OPENING_DEBIT','CHARGE') THEN e.amount_paise ELSE -e.amount_paise END) FROM entries e WHERE state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals WHERE entry_id=e.id)")[0][0]).toBe(150001)
    await pageFits(page); await captureReview(page, 'late-identity-reply-revoked-basic-gate-short320')
  } finally { release?.(); await host.close() }
})
