import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { login, navigate } from './helpers'
import { execute, names } from './native-webmcp-helpers'
import { captureOffsite, openProtection, receivedDraft, reconcileCurrent } from './offsite-helpers'
import { db } from './registry-import-fixtures'

test.describe.configure({ mode: 'serial' })
let operator: Page
test.beforeAll(async ({ browser }) => { operator = await browser.newPage({ viewport: { width: 1440, height: 1000 } }); await login(operator) })
test.afterAll(async () => { await operator?.context().close() })

test('actual native recovery status is bounded, private and preserves an active human financial form', async () => {
  const page = operator
  await openProtection(page)
  await expect.poll(() => names(page)).toContain('society_read_recovery_protection')
  await receivedDraft(page, 'Native protection read preserves this human draft')
  const writes: string[] = []
  const observer = (request: { method: () => string; url: () => string }) => { if (!['GET', 'HEAD'].includes(request.method())) writes.push(request.url()) }
  page.on('request', observer)
  const result = JSON.parse(await execute(page, 'society_read_recovery_protection', {}))
  expect(result.phase).toBe('PAUSED'); expect(result.finance_paused).toBe(true); expect(result.counts.UNCERTAIN).toBe(1)
  for (const secret of ['object_key', 'object_version', 'encrypted_sha256', 'private_recipients', 'lease_token', 'config_hash', 'fixture-version', 'actor_kind', 'Source / evidence', 'custody_note']) expect(JSON.stringify(result)).not.toContain(secret)
  await expect(execute(page, 'society_read_recovery_protection', { action: 'UPLOAD' })).rejects.toThrow()
  await expect(execute(page, 'society_upload_backup', {})).rejects.toThrow()
  await expect(execute(page, 'society_override_backup_protection', {})).rejects.toThrow()
  await expect(execute(page, 'society_open_workspace', { screen: 'backups' })).rejects.toThrow()
  expect(writes).toEqual([]); page.off('request', observer)
  await expect(page.getByRole('heading', { name: 'Review this draft.' })).toBeVisible()
  await expect(page.getByRole('dialog').getByRole('checkbox')).not.toBeChecked()
  await page.setViewportSize({ width: 320, height: 440 }); await captureOffsite(page, 'native-human-review-preserved-short320')
})

test('a held actual native protection result is discarded after independent custody changes the current recovery point', async () => {
  const page = operator
  let release!: () => void, entered!: () => void, calls = 0
  const observed: { status_key: string; phase: string; counts: Record<string, number> }[] = []
  const held = new Promise<void>(resolve => release = resolve), ready = new Promise<void>(resolve => entered = resolve)
  await page.route('**/api/backups', async route => {
    const call = ++calls
    const result = await route.fetch(); expect(result.status()).toBe(200); observed.push(await result.json())
    if (call === 1) { entered(); await held }
    await route.fulfill({ response: result }).catch(() => {})
  })
  const reading = execute(page, 'society_read_recovery_protection', {})
  await ready
  try {
    expect(observed[0].phase).toBe('PAUSED'); expect(observed[0].counts.UNCERTAIN).toBe(1)
    await reconcileCurrent(page, 'native-independent-custody')
    // Native Chrome masks callback exceptions behind UnknownError. Prove the
    // two successful reads changed before asserting the real invocation failed.
    release(); await expect(reading).rejects.toThrow(/UnknownError|Recovery protection changed/)
    expect(calls).toBe(2); expect(observed[1].phase).toBe('HEALTHY'); expect(observed[1].counts.VERIFIED).toBe(1)
    expect(observed[1].status_key).not.toBe(observed[0].status_key)
    await page.unroute('**/api/backups')
    const current = JSON.parse(await execute(page, 'society_read_recovery_protection', {}))
    expect(current.phase).toBe('HEALTHY'); expect(current.finance_paused).toBe(false); expect(current.counts.VERIFIED).toBe(1)
    await expect(page.getByRole('heading', { name: 'Review this draft.' })).toBeVisible()
    await expect(page.getByRole('dialog').getByRole('checkbox')).not.toBeChecked()
    expect(db('SELECT state,amount_paise FROM entries')).toEqual([['DRAFT', 40001]])
    expect(db('SELECT COUNT(*) FROM receipts')[0][0]).toBe(0)
    await page.getByRole('button', { name: 'Close entry details', exact: true }).click()
    await navigate(page, 'Backups & recovery'); await expect(page.getByRole('heading', { name: 'Records protected' })).toBeVisible()
    await captureOffsite(page, 'native-current-verified-protection-short320')
  } finally { release(); await page.unroute('**/api/backups') }
})

test('actual native discovery and direct API reads exclude household-only accounts from global recovery information', async ({ page }) => {
  await login(page, 'Owner')
  await expect.poll(() => names(page)).toContain('society_find_homes')
  expect(await names(page)).not.toContain('society_read_recovery_protection')
  await expect(execute(page, 'society_read_recovery_protection', {})).rejects.toThrow()
  await expect(execute(page, 'society_open_workspace', { screen: 'backups' })).rejects.toThrow()
  expect((await page.request.get('/api/backups')).status()).toBe(403)
  expect(db('SELECT COUNT(*) FROM receipts')[0][0]).toBe(0)
})
