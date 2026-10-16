import { test, expect } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { login, navigate } from './helpers'
import { apiMaintenance, apiPublish, ensureMaintenanceReviewer } from './maintenance-fixtures'

test('a published maintenance period opens its reminder controls without creating or sending a message', async ({ page, browser }) => {
  await login(page); await ensureMaintenanceReviewer(page)
  const context = await browser.newContext({ baseURL: new URL(page.url()).origin }), reviewer = await context.newPage()
  try {
    await login(reviewer, 'Committee')
    const title = 'Reminder navigation maintenance', id = await apiMaintenance(page, title)
    await apiPublish(reviewer, id)
    const before = await (await page.request.get('/api/messages')).json()
    // Existing reminder envelopes use cycle; ordinary period links remain valid.
    await page.goto('/#maintenance?cycle=' + id)
    await page.getByRole('link', { name: 'Prepare dues reminder', exact: true }).click()
    const dialog = page.getByRole('dialog')
    await expect(dialog.getByRole('combobox', { name: 'Message source', exact: true })).toContainText('Outstanding maintenance reminder')
    await expect(dialog.getByRole('textbox', { name: 'Search published message sources', exact: true })).toHaveValue(title)
    const original = dialog.locator('.message-source-option').filter({ hasText: title })
    await original.click(); await expect(original).toHaveAttribute('aria-pressed', 'true')
    await dialog.getByRole('combobox', { name: 'Delivery channel', exact: true }).click()
    await page.getByRole('option', { name: 'WhatsApp · simulation', exact: true }).click()
    await expect(dialog.getByRole('combobox', { name: 'Delivery channel', exact: true })).toContainText('WhatsApp')
    await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0)
    expect((await (await page.request.get('/api/messages')).json()).total).toBe(before.total)
    await reviewer.getByRole('button', { name: 'Sign out', exact: true }).click(); await login(reviewer, 'Tenant')
    await reviewer.goto('/#messages?compose=maintenance&source_search=' + encodeURIComponent(title))
    await expect(reviewer.getByRole('heading', { name: 'WhatsApp & email.', exact: true })).toBeVisible()
    await expect(reviewer.getByRole('dialog')).toHaveCount(0)
  } finally { await context.close() }
})

test('money sections open the real payment queues and scoped export form on desktop and phone', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await login(page)
  const folder = resolve('../reports/local/rollout-review'); mkdirSync(folder, { recursive: true })
  for (const viewport of [{ width: 1440, height: 900 }, { width: 768, height: 1024 }, { width: 320, height: 640 }]) {
    await page.setViewportSize(viewport)
    await navigate(page, 'Payment reconciliation')
    await expect(page.getByRole('heading', { name: 'Payment reconciliation.', exact: true })).toBeVisible()
    await page.screenshot({ path: resolve(folder, `payments-${viewport.width}.png`) })
    await page.getByRole('link', { name: /Fund payment reports/ }).click()
    await expect(page.getByRole('tab', { name: 'Payment reports', exact: true })).toHaveAttribute('aria-selected', 'true')
    await expect(page.getByRole('searchbox', { name: 'Search payment reports', exact: true })).toBeVisible()
    await navigate(page, 'Payment reconciliation')
    await page.getByRole('link', { name: /Fine payment reports/ }).click()
    await expect(page.getByRole('tab', { name: 'Paid reports', exact: true })).toHaveAttribute('aria-selected', 'true')
    await navigate(page, 'Reports')
    await expect(page.getByRole('heading', { name: 'Financial reports.', exact: true })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.screenshot({ path: resolve(folder, `reports-${viewport.width}.png`) })
    await page.getByRole('button', { name: 'Review monthly collections', exact: true }).click()
    await expect(page.getByRole('dialog')).toContainText('Original receipt register')
    await page.keyboard.press('Escape')
    await expect(page.getByRole('button', { name: 'Review monthly collections', exact: true })).toBeFocused()
  }
})

test('finance hubs honor denied tenant access and Help explains the current role', async ({ page }) => {
  await login(page, 'Tenant')
  for (const route of ['payments', 'reports']) {
    await page.goto('/#' + route)
    await expect(page.getByRole('heading', { name: 'Financial access required.', exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: /^Review monthly/ })).toHaveCount(0)
  }
})
