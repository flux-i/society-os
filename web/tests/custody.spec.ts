import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { login, navigate, chooseOption } from './helpers'
import { db, pageFits, dialogFits } from './registry-import-fixtures'
import { archiveCustodyCandidate, captureCustody, exhaustCustodyCandidate } from './custody-helpers'
import { fixtureStatus, receivedDraft } from './offsite-helpers'

test.describe.configure({ mode: 'serial' })
test.setTimeout(120000)
let operator: Page
let candidate: Awaited<ReturnType<typeof exhaustCustodyCandidate>>
test.beforeAll(async ({ browser }) => {
  // Four real 10-second worker ticks exceed the default 30-second hook budget.
  // Keep production scheduling and retry limits intact.
  test.setTimeout(120000)
  operator = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
  await login(operator); candidate = await exhaustCustodyCandidate(operator)
})
test.afterAll(async () => { await operator?.context().close() })

test('a completed independent archive survives a real journal failure and deliberate resume releases one slot without changing the failed original', async () => {
  const page = operator
  await navigate(page, 'Backups & recovery')
  await expect(page.locator('.backup-protection-card [role="alert"]')).toContainText('staging is full')
  const before = db('SELECT action,actor_kind,note,occurred_at FROM offsite_backup_events WHERE job_id=? ORDER BY id', [candidate.id])
  db("CREATE TRIGGER fail_browser_custody BEFORE INSERT ON offsite_custody_dispositions BEGIN SELECT RAISE(ABORT,'independent journal failure'); END")
  expect(() => archiveCustodyCandidate(candidate.id, candidate.stage, 'actual-retained-copy')).toThrow()
  expect(db('SELECT COUNT(*) FROM offsite_custody_dispositions')[0][0]).toBe(0)
  await page.getByRole('button', { name: 'Refresh recovery protection', exact: true }).click()
  await expect(page.locator('.backup-counts')).not.toContainText('held in custody')
  await captureCustody(page, 'complete-copy-unacknowledged-desktop')
  db('DROP TRIGGER fail_browser_custody')
  const saved = archiveCustodyCandidate(candidate.id, candidate.stage, 'actual-retained-copy', true)
  const again = archiveCustodyCandidate(candidate.id, candidate.stage, 'actual-retained-copy', true)
  expect(again).toEqual(saved)
  expect(db('SELECT COUNT(*) FROM offsite_custody_dispositions')[0][0]).toBe(1)
  expect(db('SELECT id,object_key,config_hash,captured_at,state,attempts,available_at,lease_token,lease_until,encrypted_sha256,encrypted_bytes,object_version,acknowledged_at,error_code FROM offsite_backup_jobs WHERE id=?', [candidate.id])[0]).toEqual(candidate.original)
  expect(db('SELECT action,actor_kind,note,occurred_at FROM offsite_backup_events WHERE job_id=? ORDER BY id', [candidate.id])).toEqual(before)
  expect(db('SELECT COUNT(*) FROM receipts')[0][0]).toBe(0)
  await page.getByRole('button', { name: 'Refresh recovery protection', exact: true }).click()
  await expect(page.locator('.backup-counts')).toContainText('1 held in custody')
  await expect(page.getByRole('heading', { name: 'Financial changes paused' })).toBeVisible()
  await expect(page.locator('.backup-protection-card [role="alert"]')).toHaveCount(0)
  const source = await (await page.request.get('/api/backups')).json()
  expect(source.archived_count).toBe(1); expect(source.staging_full).toBe(false)
  expect(source.finance_paused).toBe(true); expect(source.capture_at).toBe(0); expect(source.acknowledged_at).toBe(0)
  expect(source.counts.FAILED).toBe(4); expect(source.counts.VERIFIED ?? 0).toBe(0)
  expect((await fixtureStatus(page)).puts).toBe(5)
  await receivedDraft(page, 'Custody disposition does not authorise this supplied received record')
  await page.getByRole('dialog').getByRole('checkbox').check()
  await page.getByRole('button', { name:'Confirm this entry',exact:true }).click()
  await expect(page.getByRole('dialog').getByRole('alert')).toContainText('New financial changes are paused')
  expect(db('SELECT state,amount_paise FROM entries')).toEqual([['DRAFT',40001]])
  expect(db('SELECT COUNT(*) FROM receipts')[0][0]).toBe(0)
  await captureCustody(page, 'archived-copy-finance-still-paused-desktop')
  await page.getByRole('button', { name:'Close entry details',exact:true }).click()
  await navigate(page, 'Backups & recovery')
})

test('read-only custody facts, failed-upload history and opened filters retain readable geometry and scrolling across five layouts', async () => {
  const page = operator
  for (const [width, height] of [[1440,1000],[820,1050],[375,812],[320,740],[320,440]]) {
    await page.setViewportSize({ width, height }); await pageFits(page)
    const select = page.getByRole('combobox', { name: 'Filter recovery attempts', exact: true })
    // Settle the responsive scroll before opening the positioned menu. The
    // previous layout may have left this control beneath the viewport.
    await select.scrollIntoViewIfNeeded()
    await expect(select).toBeInViewport()
    await select.focus(); await select.press('Enter')
    await expect(page.getByRole('option', { name: 'All attempts', exact: true })).toHaveAttribute('aria-selected','true')
    await page.getByRole('option', { name: 'Failed attempts', exact: true }).hover()
    await captureCustody(page, `custody-filter-${width}-${height}`)
    await page.keyboard.press('End'); await expect(page.getByRole('option', { name: 'In progress', exact: true })).toBeInViewport()
    await page.keyboard.press('Escape'); await expect(select).toBeFocused()
    await chooseOption(page, 'Filter recovery attempts', 'Failed attempts')
    const archived = page.locator('.backup-attempt').filter({ hasText: 'Independently retained' })
    await expect(archived).toHaveCount(1); await archived.click()
    await expect(page.getByRole('heading', { name: 'The original stays with us.' })).toBeVisible()
    await expect(page.getByRole('complementary', { name: 'Independent custody verification' })).toContainText('Independent fictional custodian')
    await expect(page.locator('.backup-detail-facts')).toContainText('No confirmed copy yet')
    await expect(page.getByRole('dialog').getByText('Failed attempt', { exact: true })).toBeVisible()
    await page.locator('.backup-custody-summary').scrollIntoViewIfNeeded()
    const text = (await page.locator('.backup-custody-summary > div').boundingBox())!
    expect(text.width).toBeGreaterThanOrEqual(180)
    await dialogFits(page); await captureCustody(page, `custody-verification-${width}-${height}`)
    await page.locator('.backup-dialog-body').evaluate(element => { element.scrollTop = element.scrollHeight })
    await page.locator('.backup-dialog-body').hover(); await page.mouse.wheel(0,1200)
    await expect(page.locator('.backup-timeline li').last()).toBeInViewport()
    await expect(page.getByRole('button', { name: 'Close recovery point', exact: true })).toBeInViewport()
    await captureCustody(page, `custody-original-history-${width}-${height}`)
    await page.keyboard.press('Escape'); await expect(archived).toBeFocused()
    await chooseOption(page, 'Filter recovery attempts', 'All attempts')
  }
})

test('failed custody status reads do not turn the retained count into zero and household-only accounts cannot read the new metadata', async ({ page }) => {
  await operator.setViewportSize({ width: 375, height: 812 })
  await operator.route('**/api/backups?**', async route => route.fulfill({ status:503,contentType:'application/json',body:'{"error":"unavailable"}' }))
  await operator.getByRole('button', { name:'Refresh recovery protection',exact:true }).click()
  await expect(operator.locator('.backup-counts')).toHaveText('Counts unavailable')
  await captureCustody(operator, 'custody-unavailable-375')
  await operator.unroute('**/api/backups?**')
  await operator.getByRole('button', { name:'Retry recovery protection',exact:true }).click()
  await expect(operator.locator('.backup-counts')).toContainText('1 held in custody')
  await page.setViewportSize({width:320,height:440}); await login(page,'Owner')
  expect((await page.request.get('/api/backups')).status()).toBe(403)
  expect((await page.request.get('/api/backups/'+candidate.id)).status()).toBe(403)
  await page.goto('/#backups'); await expect(page.getByRole('heading',{name:'Operator access required.'})).toBeVisible()
  await pageFits(page); await captureCustody(page,'custody-household-denied-short320')
})
