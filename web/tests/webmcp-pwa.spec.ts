import { test, expect } from '@playwright/test'
import { login } from './helpers'
import { names, execute } from './native-webmcp-helpers'
import { checklistDesk } from './checklists-fixtures'
import { publicCache, pwaCapture, pwaFixture, pwaStats, prepareUpdate, readyShell } from './pwa-fixtures'

test.use({ serviceWorkers: 'allow', viewport: { width: 1440, height: 1000 } })
test.setTimeout(90000)
test.beforeEach(async ({ page }) => { await pwaFixture(page, { revision: '', failedInstall: false, dropPost: '', failAPI: false, reset: true }) })

test('actual native tools disappear offline and reconnect keeps the human form without queued writes or private caching', async ({ page, context }) => {
  await page.setViewportSize({ width: 375, height: 640 }); await readyShell(page); await login(page); await expect.poll(() => names(page)).toContain('society_find_homes'); expect(JSON.parse(await execute(page, 'society_find_homes', {})).total).toBe(118)
  await checklistDesk(page); await page.getByRole('button', { name: 'Prepare a checklist', exact: true }).click(); const input = page.getByRole('textbox', { name: 'Checklist explanation', exact: true }); await input.fill('PRIVATE_PWA_NATIVE retain this supplied human input.'); await pwaFixture(page, { reset: true }); await context.setOffline(true)
  await expect.poll(() => names(page)).toEqual([]); await expect(execute(page, 'society_find_homes', {})).rejects.toThrow(); await expect(input).toBeDisabled(); await expect(input).toHaveValue('PRIVATE_PWA_NATIVE retain this supplied human input.'); await pwaCapture(page, 'native-offline-form-375-640')
  await context.setOffline(false); await expect(input).toBeEnabled(); await expect.poll(() => names(page)).toContain('society_find_move_checklists'); expect(JSON.parse(await execute(page, 'society_find_move_checklists', {})).total).toBe(0); await expect(input).toHaveValue('PRIVATE_PWA_NATIVE retain this supplied human input.'); expect((await pwaStats(page)).requests.filter(request => request.method === 'POST' && request.path.startsWith('/api/'))).toEqual([]); expect((await publicCache(page)).some(item => item.path.startsWith('/api/') || item.body.includes('PRIVATE_PWA_NATIVE'))).toBe(false)
})

test('offline signout immediately removes native authority and later login discovers only the new household', async ({ page, context }) => {
  await readyShell(page); await login(page); await expect.poll(() => names(page)).toContain('society_find_accounts'); await context.setOffline(true); await expect.poll(() => names(page)).toEqual([]); await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await expect(page.getByRole('heading', { name: 'Your workspace is closed.', exact: true })).toBeVisible(); await page.reload(); await expect.poll(() => names(page)).toEqual([])
  await context.setOffline(false); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible(); expect((await page.request.get('/api/auth/me')).status()).toBe(401); await login(page, 'Tenant'); await expect.poll(() => names(page)).toContain('society_find_homes'); expect(await names(page)).not.toContain('society_find_accounts'); expect(JSON.parse(await execute(page, 'society_find_homes', {})).items.map((item: { id: string }) => item.id)).toEqual(['demo-flat-A-103']); await expect(execute(page, 'society_find_accounts', {})).rejects.toThrow()
})

test('waiting worker update preserves actual native human-form protection and rediscovery after deliberate reload', async ({ page }) => {
  await readyShell(page); await login(page, 'Tenant'); await expect.poll(() => names(page)).toContain('society_find_move_checklists'); await checklistDesk(page); await page.getByRole('button', { name: 'Prepare a checklist', exact: true }).click(); const input = page.getByRole('textbox', { name: 'Checklist explanation', exact: true }); await input.fill('PRIVATE_PWA_NATIVE keep this form while an update waits.'); await prepareUpdate(page, 'native-waiting')
  await expect(page.getByRole('dialog').getByText('An app update is ready.', { exact: true })).toBeVisible(); await expect(execute(page, 'society_open_workspace', { screen: 'entries' })).rejects.toThrow(); expect(JSON.parse(await execute(page, 'society_find_homes', {})).items.map((item: { id: string }) => item.id)).toEqual(['demo-flat-A-103']); await expect(input).toHaveValue('PRIVATE_PWA_NATIVE keep this form while an update waits.'); await page.keyboard.press('Escape'); await pwaFixture(page, { reset: true })
  await Promise.all([page.waitForEvent('load'), page.getByRole('button', { name: 'Update app', exact: true }).click()]); await expect.poll(() => names(page)).toContain('society_find_homes'); expect(await names(page)).not.toContain('society_find_accounts'); expect(JSON.parse(await execute(page, 'society_find_homes', {})).items.map((item: { id: string }) => item.id)).toEqual(['demo-flat-A-103']); expect((await pwaStats(page)).requests.filter(request => request.method === 'POST')).toEqual([]); await pwaCapture(page, 'native-updated-current-household-desktop')
})
