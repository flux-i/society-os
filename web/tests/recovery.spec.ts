import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { login } from './helpers'
import { pageFits } from './registry-import-fixtures'
import { captureRecovery, recoveredHost } from './recovery-helpers'

test.describe.configure({ mode: 'serial' })
test.setTimeout(120000)
let host: Awaited<ReturnType<typeof recoveredHost>>, source: Page
test.beforeAll(async ({ browser }) => { test.setTimeout(120000); source = await browser.newPage(); await login(source); host = await recoveredHost(source, browser, 'actual-ordinary-host'); await login(host.page) })
test.afterAll(async () => { await host?.close(); await source?.context().close() })

test('authenticated recovered host withholds private fragments and keeps real controls readable at five layouts', async () => {
  const page = host.page
  const identity = await (await page.request.get('/api/auth/me')).json()
  expect(identity.recovery_required).toBe(true); expect(identity.roles).toEqual([])
  expect(identity.can_manage_registry).toBe(false); expect(identity.can_read_records).toBe(false)
  for (const [width, height] of [[1440, 1000], [820, 1050], [375, 812], [320, 740], [320, 440]]) {
    await page.setViewportSize({ width, height }); await page.goto('/#homes?private=held')
    await expect(page.getByRole('heading', { name: 'Your records. Kept together.' })).toBeVisible()
    await expect(page.getByRole('navigation', { name: 'Main navigation' })).toHaveCount(0)
    await page.locator('.recovery-screen').focus()
    await page.evaluate(() => window.scrollTo({ top: 0, behavior: 'instant' }))
    await pageFits(page)
    expect(await page.locator('.recovery-intro h1').evaluate(element => getComputedStyle(element).fontFamily)).toContain('Instrument Serif')
    await captureRecovery(page, `quarantine-original-${width}-${height}`)
    const refresh = page.getByRole('button', { name: 'Check status', exact: true })
    await refresh.scrollIntoViewIfNeeded(); await expect(refresh).toBeInViewport()
    await page.locator('.recovery-screen').evaluate(element => (element as HTMLElement).focus({ preventScroll: true }))
    await page.keyboard.press('Tab'); await expect(refresh).toBeFocused()
    if (width <= 375) await page.mouse.wheel(0, 400)
    await expect(refresh).toBeInViewport({ ratio: 1 })
    expect((await refresh.boundingBox())!.height).toBeGreaterThanOrEqual(44)
    await captureRecovery(page, `quarantine-actions-focused-${width}-${height}`)
    await page.keyboard.press('Enter')
    await expect(page.locator('.recovery-feedback')).toHaveText('Checked just now. Recovery review is still required.')
    await page.keyboard.press('Tab'); await expect(page.getByRole('button', { name: 'Sign out', exact: true })).toBeFocused()
    await pageFits(page)
  }
  for (const path of ['/api/flats', '/api/entries', '/api/documents', '/api/messages', '/api/backups']) {
    const response = await page.request.get(path); expect(response.status()).toBe(423); expect((await response.json()).error).toBe('recovery_review_required')
  }
})

test('status pending, actual error and deliberate retry retain the recovery boundary and permit sign-out', async () => {
  const page = host.page; await page.setViewportSize({ width: 375, height: 812 })
  let entered!: () => void, release!: () => void
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route('**/api/auth/me', async route => { entered(); await held; await route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":"unavailable"}' }).catch(() => {}) })
  await page.getByRole('button', { name: 'Check status', exact: true }).click(); await ready
  await expect(page.getByRole('button', { name: 'Checking…', exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Sign out', exact: true })).toBeEnabled()
  await captureRecovery(page, 'quarantine-pending-read-375'); release()
  await expect(page.locator('.recovery-pause').getByRole('alert')).toBeVisible(); await captureRecovery(page, 'quarantine-read-error-375')
  await page.unroute('**/api/auth/me'); await page.getByRole('button', { name: 'Try again', exact: true }).click()
  await expect(page.locator('.recovery-feedback')).toHaveText('Checked just now. Recovery review is still required.')
  await expect(page.locator('.recovery-pause').getByRole('alert')).toHaveCount(0)
  await captureRecovery(page, 'quarantine-retried-read-375')
})

test('sign-out during a held status read clears the session and never revives a late response', async () => {
  const page = host.page; await page.setViewportSize({ width: 320, height: 440 })
  let entered!: () => void, release!: () => void
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  await page.route('**/api/auth/me', async route => { const response = await route.fetch(); entered(); await held; await route.fulfill({ response }).catch(() => {}) })
  await page.getByRole('button', { name: 'Check status', exact: true }).click(); await ready
  await page.getByRole('button', { name: 'Sign out', exact: true }).click(); release()
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
  await expect(page.locator('.recovery-screen')).toHaveCount(0)
  await page.unroute('**/api/auth/me')
  expect((await page.request.get('/api/auth/me')).status()).toBe(401)
  await captureRecovery(page, 'quarantine-signed-out-short320')
})
