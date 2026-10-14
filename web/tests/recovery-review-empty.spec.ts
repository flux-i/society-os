import { test, expect } from '@playwright/test'
import { pageFits, dialogFits } from './registry-import-fixtures'
import { captureReview, loginReviewer, reviewDB, reviewHost } from './recovery-review-fixtures'

test.setTimeout(120000)
test('an actually empty retained-work checkpoint renders its summary failure and retry without inventing actions', async ({ browser }) => {
  const host = await reviewHost(browser, false)
  try {
    await loginReviewer(host.one)
    const page = host.one.page
    await page.setViewportSize({ width: 375, height: 812 })
    await page.getByRole('button', { name: 'Unfinished work', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'No unfinished actions in this checkpoint.', exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Prepare work review', exact: true })).toHaveCount(0)
    expect(reviewDB(host.target, 'SELECT COUNT(*) FROM recovery_external_jobs')[0][0]).toBe(0)
    const summary = await (await page.request.get('/api/recovery/status')).json()
    expect(summary.retained_work).toBe(0); expect(summary.pending).toBe(0); expect(summary.approved).toBe(0); expect(summary.can_resume).toBe(false)
    await pageFits(page); await captureReview(page, 'empty-retained-work-375')
    await page.route('**/api/recovery/status', route => route.fulfill({ status: 500, json: { error: 'controlled_summary_failure' } }))
    await page.getByRole('button', { name: 'Refresh review', exact: true }).click(); await expect(page.getByRole('alert')).toBeVisible()
    await page.setViewportSize({ width: 320, height: 440 }); await captureReview(page, 'current-review-summary-error-short320')
    await page.unroute('**/api/recovery/status'); await page.getByRole('button', { name: 'Try again', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Refresh review', exact: true })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'No unfinished actions in this checkpoint.', exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Current access', exact: true }).click(); await page.getByRole('button', { name: 'Prepare new person', exact: true }).click()
    await expect(page.getByRole('dialog')).toBeVisible(); await dialogFits(page)
    await page.keyboard.press('Tab'); expect(await page.getByRole('dialog').evaluate(element => element.contains(document.activeElement))).toBe(true)
    await page.getByRole('button', { name: 'Close recovery preparation', exact: true }).focus(); await page.keyboard.press('Shift+Tab')
    expect(await page.getByRole('dialog').evaluate(element => element.contains(document.activeElement))).toBe(true)
    await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0)
    expect(reviewDB(host.target, 'SELECT amount_paise FROM entries WHERE id IN(SELECT entry_id FROM receipts)')[0][0]).toBe(40000)
  } finally { await host.close() }
})
