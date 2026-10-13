import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { login } from './helpers'
import { names, execute } from './native-webmcp-helpers'
import { db } from './registry-import-fixtures'
import { receivedDraft } from './offsite-helpers'
import { captureRecovery, insertControlledQuarantine, recoveredHost } from './recovery-helpers'

test.describe.configure({ mode: 'serial' })
test.setTimeout(120000)
let source: Page, host: Awaited<ReturnType<typeof recoveredHost>>
test.beforeAll(async ({ browser }) => { test.setTimeout(120000); source = await browser.newPage(); await login(source); host = await recoveredHost(source, browser, 'actual-native-host'); await login(host.page) })
test.afterAll(async () => { await host?.close(); await source?.context().close() })

test('actual native recovered-host discovery exposes one bounded status read without old private tools or release mutations', async () => {
  const page = host.page; await page.setViewportSize({ width: 320, height: 440 })
  await expect.poll(() => names(page)).toEqual(['society_read_recovery_state'])
  const writes: string[] = []; page.on('request', request => { if (!['GET', 'HEAD'].includes(request.method())) writes.push(request.url()) })
  const button = page.getByRole('button', { name: 'Check status', exact: true }); await button.scrollIntoViewIfNeeded(); await button.focus()
  const before = await page.evaluate(() => ({ hash: location.hash, scroll: scrollY }))
  expect(JSON.parse(await execute(page, 'society_read_recovery_state', {}))).toEqual({ recovery_required: true, private_access: 'WITHHELD', financial_changes: 'WITHHELD', external_handoffs: 'WITHHELD', can_resume: false })
  expect(await page.evaluate(() => ({ hash: location.hash, scroll: scrollY }))).toEqual(before)
  await expect(button).toBeFocused()
  await expect(execute(page, 'society_read_recovery_state', { release: true })).rejects.toThrow()
  await expect(execute(page, 'society_resume_recovery', {})).rejects.toThrow()
  expect(writes).toEqual([])
  await captureRecovery(page, 'native-quarantine-status-short320')
})

test('actual native stable reads preserve the human draft while a later quarantine rejects held results and clears old private scope', async () => {
  const page = source; await page.setViewportSize({ width: 375, height: 812 })
  await expect.poll(() => names(page)).toContain('society_read_recovery_protection')
  await receivedDraft(page, 'Supplied manual amount retained while current recovery authority changes')
  const identityBefore = await (await page.request.get('/api/auth/me')).json()
  const stable = JSON.parse(await execute(page, 'society_read_recovery_protection', {}))
  expect(stable.finance_paused).toBe(false)
  await expect(page.getByRole('heading', { name: 'Review this draft.' })).toBeVisible()
  await expect(page.getByRole('dialog').getByRole('checkbox')).not.toBeChecked()
  await captureRecovery(page, 'native-stable-read-human-draft-375')
  let entered!: () => void, release!: () => void, calls = 0
  const ready = new Promise<void>(resolve => entered = resolve), held = new Promise<void>(resolve => release = resolve)
  const writes: string[] = []; page.on('request', request => { if (!['GET', 'HEAD'].includes(request.method())) writes.push(request.url()) })
  await page.route('**/api/backups', async route => {
    const response = await route.fetch()
    if (++calls === 1) { expect(response.status()).toBe(200); entered(); await held }
    await route.fulfill({ response }).catch(() => {})
  })
  const reading = execute(page, 'society_read_recovery_protection', {})
  await ready
  try {
    insertControlledQuarantine(); release()
    await expect(reading).rejects.toThrow(/UnknownError|Recovery|changed|aborted/)
    await expect(page.getByRole('heading', { name: 'Your records. Kept together.' })).toBeVisible()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await expect.poll(() => names(page)).toEqual(['society_read_recovery_state'])
    const identity = await (await page.request.get('/api/auth/me')).json()
    expect(identity.recovery_required).toBe(true); expect(identity.scope_key).not.toBe(identityBefore.scope_key); expect(identity.roles).toEqual([])
    expect(db('SELECT state,amount_paise FROM entries')).toEqual([['DRAFT', 40001]])
    expect(db('SELECT COUNT(*) FROM receipts')[0][0]).toBe(0)
    expect((await page.request.get('/api/entries')).status()).toBe(423)
    expect(writes).toEqual([])
    await captureRecovery(page, 'native-held-scope-withheld-375')
  } finally { release(); await page.unroute('**/api/backups') }
})
