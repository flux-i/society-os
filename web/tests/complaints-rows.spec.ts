import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { login, navigate, chooseOption } from './helpers'

async function capture(page: Page, name: string) {
  if (!process.env.SOCIETY_CAPTURE_UI) return
  const directory = resolve('../reports/local/complaints-rows-review')
  mkdirSync(directory, { recursive: true, mode: 0o700 })
  await page.evaluate(() => document.fonts.ready)
  await page.screenshot({ path: resolve(directory, name + '.png'), animations: 'disabled' })
}

async function post(page: Page, path: string, data: Record<string, unknown>) {
  const me = await (await page.request.get('/api/auth/me')).json()
  const response = await page.request.post(path, { headers: { Origin: new URL(page.url()).origin, 'X-CSRF-Token': me.csrf_token }, data: { ...(path.endsWith('/updates') ? { visibility: 'RESIDENT_VISIBLE' } : {}), ...data, operation_key: crypto.randomUUID() } })
  expect(response.status()).toBe(200)
  return response.json()
}

test('live request rows retain status, priority, handler, full details and scoped filtering across layouts', async ({ page }) => {
  test.setTimeout(90000)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await login(page, 'Owner')
  const subjects = ['ROW Kitchen pipe leaking', 'ROW Water pressure is low', 'ROW Repaired tap needs confirmation', 'ROW A resolved plumbing request with a longer subject that remains readable on a small phone']
  const ids: string[] = []
  for (let index = 0; index < subjects.length; index++) {
    const result = await post(page, '/api/complaints', { flat_id: 'demo-flat-A-101', category: 'PLUMBING', subject: subjects[index], description: 'Full fictional details remain in the conversation, including the reported location beside the kitchen sink.', priority: index === 0 ? 'URGENT' : 'NORMAL' })
    ids.push(result.id)
  }
  await page.getByRole('button', { name: 'Sign out', exact: true }).click()
  await login(page, 'Registry officer')
  const detail = await (await page.request.get('/api/complaints/' + ids[0])).json()
  const handler = detail.handlers.find((person: { name: string }) => person.name === 'Demo Committee Member')
  expect(handler).toBeTruthy()
  await post(page, `/api/complaints/${ids[0]}/updates`, { version: 1, action: 'ASSIGN', assigned_to: handler.id, message: 'Assigned to coordinate the fictional repair.' })
  await post(page, `/api/complaints/${ids[0]}/updates`, { version: 2, action: 'STATUS', status: 'IN_PROGRESS', message: 'The supplied repair work has begun.' })
  for (const index of [2, 3]) {
    await post(page, `/api/complaints/${ids[index]}/updates`, { version: 1, action: 'STATUS', status: 'IN_PROGRESS', message: 'The repair work has begun.' })
    await post(page, `/api/complaints/${ids[index]}/updates`, { version: 2, action: 'STATUS', status: 'RESOLVED', message: 'The repair is ready for confirmation.' })
  }
  await post(page, `/api/complaints/${ids[3]}/updates`, { version: 3, action: 'STATUS', status: 'CLOSED', message: 'The completed repair has been checked.' })
  await navigate(page, 'Help & repairs')
  await page.getByRole('searchbox', { name: 'Search service requests' }).fill('ROW ')
  await expect(page.locator('.service-desk').getByRole('status')).toHaveText('4 cases found')
  const active = page.getByRole('button', { name: 'Open case ' + subjects[0], exact: true })
  await expect(active).toContainText('A-101 · Plumbing')
  await expect(active).toContainText('Urgent priority')
  await expect(active).toContainText('In progress')
  await expect(active).toContainText('With Demo Committee Member')
  await expect(active).toHaveAccessibleDescription(/A-101.*Urgent priority.*In progress.*Demo Committee Member/)
  await expect(page.getByRole('button', { name: 'Open case ' + subjects[3], exact: true })).toContainText('No handler recorded')
  for (const viewport of [{ width: 1440, height: 900 }, { width: 768, height: 1024 }, { width: 390, height: 844 }, { width: 320, height: 740 }, { width: 568, height: 320 }]) {
    await page.setViewportSize(viewport)
    await page.evaluate(() => window.scrollTo({ top: 0, behavior: 'instant' }))
    await capture(page, `rows-${viewport.width}`)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    expect(await page.locator('.service-row').evaluateAll(rows => rows.every(row => row.scrollWidth <= row.clientWidth + 1))).toBe(true)
    if (viewport.width === 1440) for (const subject of subjects) await expect(page.getByRole('button', { name: 'Open case ' + subject, exact: true })).toBeInViewport({ ratio: 1 })
    await active.focus()
    await page.keyboard.press('Enter')
    await expect(page.getByRole('dialog')).toContainText('including the reported location beside the kitchen sink')
    await page.keyboard.press('Escape')
    await expect(active).toBeFocused()
    if (viewport.width <= 390) {
      await page.locator('.service-rows').evaluate(element => window.scrollTo({ top: scrollY + element.getBoundingClientRect().top - 82, behavior: 'instant' }))
      await capture(page, `rows-${viewport.width}-scrolled`)
    }
  }
  await page.setViewportSize({ width: 1440, height: 900 })
  await chooseOption(page, 'Filter service requests by status', 'Resolved')
  await expect(page.locator('.service-desk').getByRole('status')).toHaveText('1 cases found')
  await expect(page.getByRole('button', { name: 'Open case ' + subjects[2], exact: true })).toContainText('Review resolution')
  await page.getByRole('searchbox', { name: 'Search service requests' }).fill('no-such-row-123')
  await expect(page.getByRole('heading', { name: 'Nothing matches just yet.' })).toBeVisible()
  await page.getByRole('button', { name: 'Clear filters', exact: true }).click()
  await expect(page.locator('.service-desk').getByRole('status')).toHaveText('4 cases found')
  await active.hover()
  expect(await active.evaluate(element => getComputedStyle(element).backgroundColor)).toBe('rgb(237, 241, 229)')
  await active.focus(); await page.keyboard.press('Tab'); await page.keyboard.press('Shift+Tab')
  expect(await active.evaluate(element => getComputedStyle(element).outlineStyle)).toBe('solid')
  await capture(page, 'rows-keyboard-focus')
  await page.setViewportSize({ width: 390, height: 844 })
  await page.locator('.complaints-page').evaluate(root => {
    const elements = Array.from(root.querySelectorAll<HTMLElement>('*'))
    const sizes = elements.map(element => Number.parseFloat(getComputedStyle(element).fontSize))
    elements.forEach((element, index) => { element.style.fontSize = sizes[index] * 2 + 'px' })
  })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  expect(await page.locator('.service-row').evaluateAll(rows => rows.every(row => row.scrollWidth <= row.clientWidth + 1))).toBe(true)
  await page.locator('.service-rows').evaluate(element => window.scrollTo({ top: scrollY + element.getBoundingClientRect().top - 82, behavior: 'instant' }))
  await capture(page, 'rows-390-enlarged-text')
})

test('loading, empty and error states use real scoped requests and an explicit retry', async ({ page }) => {
  await login(page, 'Tenant')
  let release!: () => void
  const held = new Promise<void>(resolve => { release = resolve })
  await page.route('**/api/complaints?**', async route => { await held; await route.continue() })
  await navigate(page, 'Help & repairs')
  await expect(page.locator('.service-desk').getByRole('status')).toHaveText('Opening your requests…')
  await expect(page.getByRole('button', { name: 'Report an issue', exact: true })).toBeDisabled()
  release()
  await expect(page.getByRole('heading', { name: 'A little calm, for now.' })).toBeVisible()
  await page.unroute('**/api/complaints?**')
  await page.route('**/api/complaints?**', route => route.fulfill({ status: 503, json: { error: 'unavailable' } }))
  await page.getByRole('searchbox', { name: 'Search service requests' }).fill('ROW')
  await expect(page.locator('.service-desk').getByRole('alert')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Report an issue', exact: true })).toBeDisabled()
  await capture(page, 'rows-request-error')
  await page.unroute('**/api/complaints?**')
  await page.getByRole('button', { name: 'Try again', exact: true }).click()
  await expect(page.locator('.service-desk').getByRole('status')).toHaveText('0 cases found')
})

test('Escape dismisses a form dropdown while preserving the unsaved service request', async ({ page }) => {
  await login(page, 'Owner')
  await page.setViewportSize({ width: 320, height: 740 })
  await navigate(page, 'Help & repairs')
  await page.getByRole('button', { name: 'Report an issue', exact: true }).click()
  await page.getByRole('textbox', { name: 'Subject', exact: true }).fill('Keep this unsaved service request')
  const category = page.getByRole('combobox', { name: 'Service category', exact: true })
  await category.click()
  await page.getByRole('option', { name: 'Lift', exact: true }).hover()
  await capture(page, 'rows-form-dropdown')
  await page.keyboard.press('Escape')
  await expect(page.getByRole('listbox')).toHaveCount(0)
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Subject', exact: true })).toHaveValue('Keep this unsaved service request')
  await expect(category).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Report an issue', exact: true })).toBeFocused()
})
