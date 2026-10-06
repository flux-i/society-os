import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { login, navigate } from './helpers'
import { ensureMaintenanceReviewer } from './maintenance-fixtures'
import { apiFineSource, apiFine, apiFineAction, apiFineReport, apiFineVerify, fineGet } from './fines-fixtures'

async function overview(page: Page) {
  if (await page.getByRole('dialog').count()) {
    await page.keyboard.press('Escape')
    await expect(page.getByRole('dialog')).toHaveCount(0)
  }
  await navigate(page, 'Overview')
  await page.getByRole('button', { name: 'Refresh overview', exact: true }).click()
  await expect(page.getByRole('region', { name: 'Fines, with fairness.', exact: true }).locator('.overview-maintenance-amounts strong').first()).not.toHaveText('—')
}
async function capture(page: Page, name: string) {
  if (!process.env.SOCIETY_CAPTURE_UI) return
  const root = resolve(process.env.SOCIETY_FINE_CAPTURE_ROOT ?? '../reports/local/fines-review')
  mkdirSync(root, { recursive: true, mode: 0o700 })
  await page.evaluate(() => document.fonts.ready)
  await page.screenshot({ path: resolve(root, name + '.png'), fullPage: true, animations: 'disabled' })
}
test('fine attention follows an independent notice decision household response and exact verified money without surfacing private notes', async ({ page, browser }) => {
  test.setTimeout(60000); await login(page); await ensureMaintenanceReviewer(page)
  const reviewContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), reviewer = await reviewContext.newPage()
  const ownerContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), owner = await ownerContext.newPage()
  await login(reviewer, 'Committee'); await login(owner, 'Owner')
  const title = 'DAY FINE Current supplied review'
  const source = await apiFineSource(page, reviewer, owner, title), id = await apiFine(page, source, title)
  await overview(page)
  const panel = page.getByRole('region', { name: 'Fines, with fairness.', exact: true })
  await expect(panel.locator('.overview-maintenance-amounts strong')).toHaveText(['₹0.00', '0', '0'])
  await overview(reviewer)
  await expect(reviewer.getByRole('region', { name: 'Fines, with fairness.', exact: true }).locator('.overview-maintenance-amounts strong')).toHaveText(['₹0.00', '1', '0'])
  await expect(reviewer.locator('.overview-action-list a').filter({ hasText: title })).toHaveCount(1)
  await reviewer.locator('.overview-action-list a').filter({ hasText: title }).click()
  await expect(reviewer.getByRole('dialog').getByRole('heading', { name: title, exact: true, level: 2 })).toBeVisible()
  await apiFineAction(reviewer, id, 'NOTIFY')
  const fine = await fineGet(page, id)
  await overview(owner)
  const ownPanel = owner.getByRole('region', { name: 'Fines, with fairness.', exact: true })
  await expect(ownPanel.locator('.overview-maintenance-amounts strong')).toHaveText(['₹0.00', '1'])
  await expect(owner.locator('.overview-action-list')).not.toContainText('PRIVATE')
  await owner.locator('.overview-action-list a[href="#fines?notice=' + fine.notice_id + '"]').click()
  await expect(owner.getByLabel('Your response', { exact: true })).toBeVisible()
  await owner.getByLabel('Your response', { exact: true }).fill('The household supplied its account through the visible overview action.')
  await owner.getByRole('dialog').getByRole('checkbox').check()
  await owner.getByRole('button', { name: 'Send household response', exact: true }).click()
  await expect(owner.getByRole('dialog').getByText('The household supplied its account through the visible overview action.', { exact: true })).toBeVisible()
  await owner.getByRole('button', { name: 'Close fine household notice', exact: true }).click()
  await overview(owner); await expect(ownPanel.locator('.overview-maintenance-amounts strong')).toHaveText(['₹0.00', '0'])
  await apiFineAction(reviewer, id, 'RESOLVE'); await apiFineAction(reviewer, id, 'ISSUE')
  const report = await apiFineReport(owner, id)
  await overview(reviewer)
  await expect(reviewer.getByRole('region', { name: 'Fines, with fairness.', exact: true }).locator('.overview-maintenance-amounts strong')).toHaveText(['₹250.25', '1', '0'])
  await apiFineVerify(reviewer, report)
  await overview(owner); await expect(ownPanel.locator('.overview-maintenance-amounts strong')).toHaveText(['₹150.25', '0'])
  for (const width of [1440, 768, 375, 320]) {
    await owner.setViewportSize({ width, height: width <= 375 ? 640 : 1000 })
    expect(await owner.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await capture(owner, 'current-fine-overview-' + width)
  }
  await reviewContext.close(); await ownerContext.close()
})

test('unavailable fine totals stay unknown until their own retry restores the current exact result', async ({ page }) => {
  await login(page)
  await overview(page)
  const panel = page.getByRole('region', { name: 'Fines, with fairness.', exact: true })
  const retained = await panel.locator('.overview-maintenance-amounts strong').allTextContents()
  await page.route('**/api/overview/fines', route => route.fulfill({ status: 503, contentType: 'application/json', body: '{}' }))
  await navigate(page, 'Overview'); await page.getByRole('button', { name: 'Refresh overview', exact: true }).click()
  await expect(panel.locator('.overview-maintenance-amounts strong')).toHaveText(['—', '—', '—'])
  await expect(page.getByText('Fines & appeals unavailable', { exact: true })).toHaveCount(2)
  await capture(page, 'fine-overview-source-error')
  await page.unroute('**/api/overview/fines'); await panel.getByRole('button', { name: 'Retry fines & appeals', exact: true }).click()
  await expect(panel.locator('.overview-maintenance-amounts strong')).toHaveText(retained)
  await expect(page.getByText('Fines & appeals unavailable', { exact: true })).toHaveCount(0)
})
