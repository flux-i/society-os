import { test, expect } from '@playwright/test'
import { login, navigate } from './helpers'
import { financialHeaders } from './maintenance-fixtures'
import { budgetEntry, budgetGet } from './budgets-fixtures'
import { checklistDesk } from './checklists-fixtures'
import { performanceWithin } from './performance-fixtures'
import { publicCache, pwaCapture, pwaFixture, pwaStats, pwaWithin, pwaRecord, prepareUpdate, readyShell } from './pwa-fixtures'

test.use({ serviceWorkers: 'allow', viewport: { width: 1440, height: 1000 } })
test.setTimeout(90000)
test.beforeEach(async ({ page }) => { await pwaFixture(page, { revision: '', failedInstall: false, dropPost: '', failAPI: false, reset: true }) })

test('actual worker caches only the public initial shell without downloading unvisited operational code', async ({ page }) => {
  await readyShell(page)
  const manifest = await (await page.request.get('/manifest.webmanifest')).json()
  expect(manifest).toMatchObject({ scope: '/', start_url: '/', display: 'standalone' })
  expect(manifest.icons.map((icon: { sizes: string }) => icon.sizes)).toEqual(expect.arrayContaining(['192x192', '512x512']))
  const before = await pwaStats(page), initialJS = before.requests.filter(request => request.path.endsWith('.js') && request.path !== '/sw.js')
  for (const request of initialJS) expect(request.path).not.toMatch(/\/(Records|Statements|Fines|MoveChecklists|webmcp-register)-/)
  expect(initialJS.reduce((sum, request) => sum + request.bytes, 0)).toBeLessThanOrEqual(879715)
  pwaRecord('actual-initial-worker-network', { initialJS, totalJavaScriptServerBytes: initialJS.reduce((sum, request) => sum + request.bytes, 0), allPublicWorkerAndPageRequests: before.requests })
  const saved = await publicCache(page); expect(saved.some(item => item.path === '/')).toBe(true); expect(saved.some(item => item.path.endsWith('.woff2'))).toBe(true)
  expect(saved.every(item => !item.search && !item.path.startsWith('/api/') && (item.path === '/' || item.path.startsWith('/assets/') || /^\/(icon(?:-192|-512)?\.(svg|png)|manifest.webmanifest)$/.test(item.path)))).toBe(true)
})

test('private reads original receipt downloads and deliberate offline reload never enter public storage', async ({ page, context }) => {
  await readyShell(page); await login(page); const id = await budgetEntry(page, '43.21', '2026-10-01'), original = await budgetGet<{ receipt_id: string; amount_paise: number }>(page, '/api/entries/' + id)
  await page.goto('/#entries?entry=' + id); await expect(page.getByRole('dialog')).toBeVisible(); await expect(page.locator('.record-detail-amount strong')).toHaveText('₹43.21')
  const download = page.waitForEvent('download'); await page.getByRole('button', { name: 'Download PDF', exact: true }).click(); await (await download).delete()
  const saved = await publicCache(page); expect(saved.every(item => !item.path.startsWith('/api/'))).toBe(true); expect(saved.some(item => item.body.includes('PRIVATE_BDG supplied original RECEIVED'))).toBe(false)
  await context.setOffline(true); await page.reload(); await expect(page.getByRole('heading', { name: 'We’ll be here when you reconnect.', exact: true })).toBeVisible(); await expect(page.getByRole('dialog')).toHaveCount(0); await expect(page.locator('body')).not.toContainText('PRIVATE_BDG')
  for (const [width, height] of [[1440, 1000], [768, 1000], [375, 640], [320, 440]]) { await page.setViewportSize({ width, height }); await pwaCapture(page, 'offline-reload-' + width + '-' + height); await pwaWithin(page) }
  await context.setOffline(false); await expect(page.getByRole('dialog')).toBeVisible(); expect(await budgetGet(page, '/api/entries/' + id)).toMatchObject({ amount_paise: 4321, receipt_id: original.receipt_id }); expect((await budgetGet<{ total: number }>(page, '/api/entries?receipts=true')).total).toBe(1)
})

test('offline human input stays visible but disabled and reconnect does not submit it', async ({ page, context }) => {
  await readyShell(page); await login(page); await checklistDesk(page); await page.getByRole('button', { name: 'Prepare a checklist', exact: true }).click(); const input = page.getByRole('textbox', { name: 'Checklist explanation', exact: true }), marker = 'PRIVATE_PWA the supplied human explanation stays in memory only.'; await input.fill(marker)
  await pwaFixture(page, { reset: true }); await context.setOffline(true); await expect(input).toBeDisabled(); await expect(input).toHaveValue(marker); await expect(page.getByRole('dialog').getByText('Your connection is paused.', { exact: true })).toBeVisible()
  for (const [width, height] of [[1440, 1000], [768, 1000], [375, 640], [320, 440]]) { await page.setViewportSize({ width, height }); await input.scrollIntoViewIfNeeded(); await pwaCapture(page, 'offline-open-input-' + width + '-' + height); await pwaWithin(page) }
  await context.setOffline(false); await expect(input).toBeEnabled(); await expect(input).toHaveValue(marker); expect((await pwaStats(page)).requests.filter(request => request.method === 'POST' && request.path.startsWith('/api/'))).toEqual([])
  await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0)
})

test('offline signout clears protected screens immediately and revokes before any account can return', async ({ page, context }) => {
  await readyShell(page); await login(page); await navigate(page, 'Homes & people'); await context.setOffline(true); await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await expect(page.getByRole('heading', { name: 'Your workspace is closed.', exact: true })).toBeVisible(); await expect(page.getByRole('link', { name: 'Homes & people', exact: true })).toHaveCount(0)
  expect(await page.evaluate(() => localStorage.getItem('society.pending-signout'))).toBe('1'); await page.reload(); await expect(page.getByRole('heading', { name: 'Your workspace is closed.', exact: true })).toBeVisible(); await pwaCapture(page, 'offline-signed-out-desktop')
  await context.setOffline(false); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible(); expect((await page.request.get('/api/auth/me')).status()).toBe(401); expect(await page.evaluate(() => localStorage.getItem('society.pending-signout'))).toBeNull()
  await login(page, 'Tenant'); await navigate(page, 'Your homes'); await expect(page.locator('.home-card')).toHaveCount(1); await expect(page.locator('body')).not.toContainText('Demo Owner A-101')
})

test('waiting update preserves an open form and other tabs then applies deliberately without changing saved money', async ({ page, context }) => {
  await readyShell(page); await login(page); const previous = (await budgetGet<{ total: number }>(page, '/api/entries?receipts=true')).total, id = await budgetEntry(page, '43.21', '2026-10-01'), original = await budgetGet<{ receipt_id: string }>(page, '/api/entries/' + id)
  await checklistDesk(page); await page.getByRole('button', { name: 'Prepare a checklist', exact: true }).click(); const input = page.getByRole('textbox', { name: 'Checklist explanation', exact: true }); await input.fill('PRIVATE_PWA keep this human form during a waiting update.'); await prepareUpdate(page, 'deliberate')
  await expect(page.getByRole('dialog').getByText('An app update is ready.', { exact: true })).toBeVisible(); await expect(input).toHaveValue('PRIVATE_PWA keep this human form during a waiting update.'); await pwaCapture(page, 'waiting-update-open-form-desktop'); await page.keyboard.press('Escape')
  const other = await context.newPage(); await other.goto('/'); await expect(other.getByRole('button', { name: 'Sign out', exact: true })).toBeVisible(); await page.getByRole('button', { name: 'Update app', exact: true }).click(); await expect(page.getByRole('alert').filter({ hasText: 'Close other Society tabs' })).toBeVisible(); await expect(page.getByRole('button', { name: 'Sign out', exact: true })).toBeVisible(); await pwaCapture(page, 'waiting-update-multiple-tabs-desktop'); await other.close()
  await page.setViewportSize({ width: 375, height: 640 }); await pwaCapture(page, 'waiting-update-mobile'); const location = page.url(), controller = await page.evaluate(() => navigator.serviceWorker.controller!.scriptURL); await Promise.all([page.waitForEvent('load'), page.getByRole('button', { name: 'Update app', exact: true }).click()]); await expect(page.getByRole('button', { name: 'Update app', exact: true })).toHaveCount(0); await expect(page.getByRole('heading', { name: 'The checklist desk.', exact: true })).toBeVisible(); expect(page.url()).toBe(location); expect(await page.evaluate(() => navigator.serviceWorker.controller!.scriptURL)).toBe(controller)
  expect(await budgetGet(page, '/api/entries/' + id)).toMatchObject({ amount_paise: 4321, receipt_id: original.receipt_id }); expect((await budgetGet<{ total: number }>(page, '/api/entries?receipts=true')).total).toBe(previous + 1); await performanceWithin(page)
})

test('actual lost confirmation response is reconciled and only an explicit retry can repeat the same operation', async ({ page }) => {
  await readyShell(page); await login(page); const before = (await budgetGet<{ total: number }>(page, '/api/entries?receipts=true')).total
  const created = await page.request.post('/api/entries', { headers: await financialHeaders(page), data: { operation_key: crypto.randomUUID(), flat_id: 'demo-flat-A-101', kind: 'RECEIVED', amount: '43.21', date: '2026-10-01', description: 'PRIVATE_PWA actual externally received original', payer: 'Fictional PWA owner', method: 'CASH', reference: '' } }); expect(created.status()).toBe(200); const id = (await created.json()).id as string
  await page.goto('/#entries?entry=' + id); await expect(page.getByRole('heading', { name: 'Review this draft.', exact: true })).toBeVisible(); await page.getByRole('dialog').getByRole('checkbox').check(); await pwaFixture(page, { reset: true, dropPost: '/api/entries/' + id + '/post' }); await page.getByRole('button', { name: 'Confirm this entry', exact: true }).click()
  await expect(page.getByRole('dialog').getByRole('alert')).toBeVisible(); await expect(page.getByRole('button', { name: 'Confirm this entry', exact: true })).toBeEnabled(); const original = await budgetGet<{ amount_paise: number; receipt_id: string; state: string }>(page, '/api/entries/' + id); expect(original).toMatchObject({ amount_paise: 4321, state: 'POSTED' }); expect(original.receipt_id).not.toBe(''); expect((await pwaStats(page)).requests.filter(request => request.method === 'POST' && request.path.startsWith('/api/'))).toHaveLength(1); await pwaCapture(page, 'lost-accepted-result-before-explicit-retry-desktop')
  await page.getByRole('button', { name: 'Confirm this entry', exact: true }).click(); await expect(page.locator('.record-detail-amount strong')).toHaveText('₹43.21'); await expect(page.getByRole('button', { name: 'Download PDF', exact: true })).toBeEnabled(); expect(await budgetGet(page, '/api/entries/' + id)).toMatchObject({ amount_paise: 4321, receipt_id: original.receipt_id, state: 'POSTED' }); expect((await budgetGet<{ total: number }>(page, '/api/entries?receipts=true')).total).toBe(before + 1); expect((await pwaStats(page)).requests.filter(request => request.method === 'POST' && request.path.startsWith('/api/'))).toHaveLength(2)
})

test('request failure while the browser reports online stays retryable and never queues a change', async ({ page }) => {
  await readyShell(page); await login(page); await pwaFixture(page, { failAPI: true, reset: true }); await navigate(page, 'Homes & people'); await expect(page.locator('.pwa-strip').getByText('Your connection is paused.', { exact: true })).toBeVisible(); expect(await page.evaluate(() => navigator.onLine)).toBe(true); await expect(page.locator('.pwa-strip').getByRole('button', { name: 'Check connection', exact: true })).toBeEnabled(); await pwaCapture(page, 'unreachable-server-online-signal-desktop')
  await pwaFixture(page, { failAPI: false }); await page.locator('.pwa-strip').getByRole('button', { name: 'Check connection', exact: true }).click(); await expect(page.locator('.pwa-strip')).toHaveCount(0); await page.getByRole('button', { name: 'Reconnect', exact: true }).click(); await page.getByRole('button', { name: 'Try again', exact: true }).click(); await expect(page.locator('.home-card')).toHaveCount(12); expect((await pwaStats(page)).requests.filter(request => request.method === 'POST' && request.path.startsWith('/api/'))).toEqual([])
})

test('offline signout remains cleared after reload when browser key-value storage is unavailable', async ({ page, context }) => {
  await page.addInitScript(() => { for (const key of ['getItem', 'setItem', 'removeItem']) Object.defineProperty(Storage.prototype, key, { value: () => { throw new DOMException('Synthetic blocked storage', 'SecurityError') } }) })
  await readyShell(page); await login(page); await context.setOffline(true); await page.getByRole('button', { name: 'Sign out', exact: true }).click(); expect(page.url()).toContain('#signed-out'); await page.reload(); await expect(page.getByRole('heading', { name: 'Your workspace is closed.', exact: true })).toBeVisible(); await page.setViewportSize({ width: 320, height: 440 }); await pwaCapture(page, 'blocked-storage-signed-out-320-440'); await pwaWithin(page)
  await context.setOffline(false); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible(); expect((await page.request.get('/api/auth/me')).status()).toBe(401)
})

test('failed initial installation and failed updates preserve online use and deliberate recovery bounds only app caches', async ({ page }) => {
  await pwaFixture(page, { revision: 'initial-broken', failedInstall: true }); await login(page); await page.getByRole('button', { name: 'About this preview', exact: true }).click(); const help = page.getByText('Install Society on your device', { exact: true }); await help.focus(); await page.keyboard.press('Enter'); await expect(page.getByRole('dialog').getByRole('alert')).toBeVisible(); await expect(page.getByRole('button', { name: 'Make yourself at home', exact: true })).toBeEnabled(); for (const [width, height] of [[1440, 1000], [768, 1000], [375, 640], [320, 440]]) { await page.setViewportSize({ width, height }); await page.getByRole('button', { name: 'Check for app updates', exact: true }).scrollIntoViewIfNeeded(); await pwaCapture(page, 'failed-install-help-' + width + '-' + height); await pwaWithin(page) }
  await pwaFixture(page, { revision: 'recovered', failedInstall: false }); await page.getByRole('button', { name: 'Check for app updates', exact: true }).click(); await page.evaluate(async () => { await navigator.serviceWorker.ready }); await page.keyboard.press('Escape'); await page.reload(); await page.waitForFunction(() => !!navigator.serviceWorker.controller); await expect(page.getByRole('button', { name: 'Sign out', exact: true })).toBeVisible()
  await page.evaluate(async () => { await (await caches.open('unrelated-public-fixture')).put('/unrelated-public-fixture', new Response('Fictional unrelated app cache')) }); await prepareUpdate(page, 'bad-update', true); await page.waitForFunction(async () => !(await navigator.serviceWorker.getRegistration())?.installing); expect((await page.evaluate(() => caches.keys())).some(name => name.includes('bad-update'))).toBe(false); await expect(page.getByRole('button', { name: 'Update app', exact: true })).toHaveCount(0)
  for (const revision of ['one', 'two', 'three']) { await prepareUpdate(page, revision); await Promise.all([page.waitForEvent('load'), page.getByRole('button', { name: 'Update app', exact: true }).click()]); await expect(page.getByRole('button', { name: 'Sign out', exact: true })).toBeVisible() }
  const cachesNow = await page.evaluate(() => caches.keys()); expect(cachesNow.filter(name => name.startsWith('society-public-'))).toHaveLength(2); expect(cachesNow).toContain('unrelated-public-fixture'); expect((await publicCache(page)).filter(item => item.cache.startsWith('society-public-')).some(item => item.path.startsWith('/api/'))).toBe(false)
})
