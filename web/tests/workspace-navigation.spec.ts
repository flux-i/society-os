import { test, expect } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { login, navigate } from './helpers'

const captures = resolve('../reports/local/rollout-review')
mkdirSync(captures, { recursive: true })

test('grouped navigation keeps every permitted workspace reachable with personal homes and plain-language help', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await login(page)
  const main = page.getByRole('navigation', { name: 'Main navigation', exact: true })
  await expect(main.getByRole('link')).toHaveText(['Today', 'People & homes', 'Money', 'Operations', 'Community', 'Administration'])
  await page.getByRole('link', { name: 'My home', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'My home.', exact: true })).toBeVisible()
  await expect(page.getByText('0 homes found', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: /^View home/ })).toHaveCount(0)
  await navigate(page, 'Homes & people')
  await expect(page.getByRole('button', { name: 'View home A-101', exact: true })).toBeVisible()
  await expect(page.getByRole('navigation', { name: 'People & homes sections' }).getByRole('link', { name: 'Homes & people', exact: true })).toHaveAttribute('aria-current', 'page')
  for (const viewport of [{ width: 1440, height: 900 }, { width: 768, height: 1024 }, { width: 390, height: 844 }, { width: 320, height: 568 }, { width: 568, height: 320 }]) {
    await page.setViewportSize(viewport)
    await navigate(page, 'Homes & people')
    await expect(page.getByRole('button', { name: 'View home A-101', exact: true })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.screenshot({ path: resolve(captures, `homes-${viewport.width}.png`) })
    await page.getByRole('button', { name: 'View home A-101', exact: true }).focus()
    await page.keyboard.press('Enter')
    await expect(page.getByRole('heading', { name: 'Home 101', exact: true })).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.getByRole('button', { name: 'View home A-101', exact: true })).toBeFocused()
  }
  await page.setViewportSize({ width: 390, height: 844 })
  for (const name of ['Help & repairs', 'Your requests', 'Upkeep', 'Rules & conduct', 'Fines & appeals', 'Documents', 'Messages', 'Statements', 'Access & invitations', 'Backups & recovery', 'Account security']) {
    await navigate(page, name === 'Your requests' ? 'Requests & approvals' : name)
    await expect(page.locator('main h1')).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  }
  await navigate(page, 'Help & permissions')
  const admin = page.locator('.role-guide summary').filter({ hasText: /^Administrator$/ })
  await admin.focus(); await page.keyboard.press('Enter')
  await expect(page.getByText('Finance posting needs a separate Treasurer appointment.', { exact: false })).toBeVisible()
  await page.screenshot({ path: resolve(captures, 'help-phone.png'), fullPage: true })
  await page.getByRole('button', { name: 'Menu', exact: true }).click()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('button', { name: 'Menu', exact: true })).toBeFocused()
  await expect(page.getByRole('button', { name: 'Menu', exact: true })).toHaveAttribute('aria-expanded', 'false')
})

test('personal homes retain real scope, handle failed reads and hide administration from a resident', async ({ page }) => {
  await login(page, 'Owner')
  await navigate(page, 'My home')
  await expect(page.getByRole('button', { name: /^View home/ })).toHaveCount(2)
  await expect(page.getByRole('button', { name: 'View home A-101', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'View home A-102', exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Administration', exact: true })).toHaveCount(0)
  let failed = true
  await page.route('**/api/flats?*', async route => { if (failed) await route.fulfill({ status: 503, json: { error: 'unavailable' } }); else await route.continue() })
  await page.reload()
  await expect(page.getByRole('alert').filter({ hasText: 'Let’s try that again.' })).toBeVisible()
  failed = false
  await page.getByRole('button', { name: 'Try again', exact: true }).click()
  await expect(page.getByRole('button', { name: /^View home/ })).toHaveCount(2)
  await navigate(page, 'Help & permissions')
  await expect(page.locator('.access-summary > div').filter({ hasText: 'Manage users and roles' })).toContainText('Not assigned')
  await expect(page.getByRole('link', { name: 'Manage users & roles', exact: true })).toHaveCount(0)
})
