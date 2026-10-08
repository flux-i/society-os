import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { chmodSync, mkdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

export async function pwaFixture(page: Page, values: Record<string, unknown>) { const response = await page.request.post('/__pwa-fixture', { data: values }); expect(response.status()).toBe(200) }
export async function pwaStats(page: Page) { return (await (await page.request.get('/__pwa-fixture')).json()) as { state: Record<string, unknown>; requests: { method: string; path: string; status: number; bytes: number }[] } }
export function pwaRecord(name: string, record: unknown) {
  if (process.env.SOCIETY_CAPTURE_UI !== '1') return
  const root = resolve(process.env.SOCIETY_PWA_CAPTURE_ROOT ?? '../reports/local/pwa-ui'); mkdirSync(root, { recursive: true, mode: 0o700 })
  writeFileSync(resolve(root, name + '.json'), JSON.stringify(record, null, 2) + '\n', { mode: 0o600, flag: 'wx' })
}
export async function readyShell(page: Page) {
  await page.goto('/')
  await page.evaluate(async () => { await navigator.serviceWorker.ready })
  await page.reload()
  await page.waitForFunction(() => !!navigator.serviceWorker.controller)
}
export async function publicCache(page: Page) {
  return page.evaluate(async () => {
    const result = []
    for (const name of await caches.keys()) for (const request of await (await caches.open(name)).keys()) result.push({ cache: name, path: new URL(request.url).pathname, search: new URL(request.url).search, body: await (await (await caches.open(name)).match(request))!.text() })
    return result
  })
}
export async function pwaCapture(page: Page, name: string) {
  if (process.env.SOCIETY_CAPTURE_UI !== '1') return
  await page.evaluate(() => document.fonts.ready)
  const root = resolve(process.env.SOCIETY_PWA_CAPTURE_ROOT ?? '../reports/local/pwa-ui'); mkdirSync(root, { recursive: true, mode: 0o700 })
  const path = resolve(root, name + '.png'); await page.screenshot({ path, animations: 'disabled' }); chmodSync(path, 0o600)
}
export async function pwaWithin(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
  if (await page.getByRole('dialog').count()) { const box = (await page.getByRole('dialog').boundingBox())!; expect(box.x).toBeGreaterThanOrEqual(0); expect(box.y).toBeGreaterThanOrEqual(0); expect(box.y + box.height).toBeLessThanOrEqual(page.viewportSize()!.height + 1); await expect(page.locator('dialog .dialog-close')).toBeInViewport({ ratio: 1 }) }
  const about = page.locator('.about-dialog')
  if (await about.count()) { const body = (await about.locator('.dialog-scroll').boundingBox())!, close = (await about.locator('.dialog-close').boundingBox())!; expect(body.y - (close.y + close.height)).toBeGreaterThanOrEqual(0); const summary = about.locator('summary'); if (await summary.evaluate(element => element === document.activeElement)) expect(await summary.evaluate(element => Number.parseFloat(getComputedStyle(element).outlineOffset))).toBeLessThanOrEqual(0) }
  const checklist = page.locator('.checklist-dialog .dialog-scroll')
  if (await checklist.count()) expect(await checklist.evaluate(element => Number.parseFloat(getComputedStyle(element).paddingLeft))).toBeGreaterThanOrEqual(20)
}
export async function prepareUpdate(page: Page, revision: string, failedInstall = false) {
  await pwaFixture(page, { revision, failedInstall })
  await page.evaluate(async ({ failedInstall }) => {
    const registration = (await navigator.serviceWorker.getRegistration())!
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('The observed update did not finish installing.')), 15000)
      const found = () => {
        const worker = registration.installing
        if (!worker) return
        const changed = () => {
          if (worker.state === (failedInstall ? 'redundant' : 'installed')) { clearTimeout(timer); registration.removeEventListener('updatefound', found); resolve() }
        }
        worker.addEventListener('statechange', changed); changed()
      }
      registration.addEventListener('updatefound', found)
      void registration.update().catch(error => { clearTimeout(timer); registration.removeEventListener('updatefound', found); reject(error) })
    })
  }, { failedInstall })
  if (!failedInstall) await page.waitForFunction(async () => !!(await navigator.serviceWorker.getRegistration())?.waiting)
}
