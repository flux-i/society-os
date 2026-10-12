import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { login, navigate, chooseOption } from './helpers'
import { db, pageFits, dialogFits } from './registry-import-fixtures'
import { captureOffsite, fixtureStatus, openProtection, receivedDraft, reconcileCurrent } from './offsite-helpers'

test.describe.configure({ mode: 'serial' })
let operator: Page
test.beforeAll(async ({ browser }) => { operator = await browser.newPage({ viewport: { width: 1440, height: 1000 } }); await login(operator) })
test.afterAll(async () => { await operator?.context().close() })

test('actual uncertain upload status, opened selection, focus, keyboard, history scrolling and dismissal fit five layouts', async () => {
  const page = operator
  for (const [width, height] of [[1440, 1000], [820, 1050], [375, 812], [320, 740], [320, 440]]) {
    await page.setViewportSize({ width, height }); await openProtection(page); await pageFits(page)
    await expect(page.locator('.backup-attempt .backup-status')).toHaveText('Needs confirmation')
    expect(await page.locator('.backups-heading h1').evaluate(element => getComputedStyle(element).fontFamily)).toContain('Instrument Serif')
    await captureOffsite(page, `uncertain-protection-${width}-${height}`)
    await page.locator('.backup-protection-title').scrollIntoViewIfNeeded()
    if(width<=375) {
      const title=(await page.locator('.backup-protection-title h2').boundingBox())!
      const refresh=(await page.getByRole('button',{name:'Refresh recovery protection',exact:true}).boundingBox())!
      expect(title.width).toBeGreaterThanOrEqual(160);expect(refresh.y).toBeGreaterThanOrEqual(title.y+title.height)
    }
    await captureOffsite(page,`protection-card-controls-${width}-${height}`)
    const select = page.getByRole('combobox', { name: 'Filter recovery attempts', exact: true })
    expect.soft((await select.boundingBox())!.height, 'Recovery filter must retain a usable touch height').toBeGreaterThanOrEqual(44)
    await select.focus(); await expect(select).toBeFocused(); await select.press('Enter')
    await expect(page.getByRole('option', { name: 'All attempts', exact: true })).toHaveAttribute('aria-selected', 'true')
    await page.getByRole('option', { name: 'Needs confirmation', exact: true }).hover()
    await captureOffsite(page, `recovery-menu-${width}-${height}`)
    await page.keyboard.press('End')
    await expect(page.getByRole('option',{name:'In progress',exact:true})).toBeInViewport()
    await captureOffsite(page,`recovery-menu-last-option-${width}-${height}`)
    await page.keyboard.press('Escape'); await expect(select).toBeFocused()
    await chooseOption(page, 'Filter recovery attempts', 'Confirmed copies')
    await expect(page.getByRole('heading', { name: 'No attempts in this view.' })).toBeVisible()
    await page.getByRole('button', { name: 'Show all attempts', exact: true }).click()
    await expect(page.locator('.backup-attempt')).toHaveCount(1)
    await page.locator('.backup-attempt').click()
    await expect(page.getByRole('heading', { name: 'A copy, accounted for.' })).toBeVisible()
    await expect(page.locator('.backup-timeline li')).toHaveCount(3)
    await dialogFits(page); await captureOffsite(page, `recovery-history-${width}-${height}`)
    await page.locator('.backup-dialog-body').evaluate(element => { element.scrollTop = element.scrollHeight })
    await page.locator('.backup-dialog-body').hover()
    await page.mouse.wheel(0, 1200)
    await expect.soft(page.locator('.backup-timeline li').last(), 'The oldest recovery event must be reachable through internal scrolling').toBeInViewport({ timeout: 1500 })
    await expect(page.getByRole('button', { name: 'Close recovery point', exact: true })).toBeInViewport()
    await captureOffsite(page, `recovery-history-scrolled-${width}-${height}`)
    await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0)
    await expect(page.locator('.backup-attempt')).toBeFocused()
  }
  const fixture = await fixtureStatus(page)
  expect(fixture.puts).toBe(1); expect(fixture.objects).toHaveLength(1)
  expect(db("SELECT state,acknowledged_at FROM offsite_backup_jobs")).toEqual([['UNCERTAIN', 0]])
})

test('loading, error, refresh, exact history retry and pending read dismissal preserve current input', async () => {
  const page = operator; await page.setViewportSize({ width: 1440, height: 1000 })
  let release!: () => void, entered!: () => void
  const held = new Promise<void>(resolve => release = resolve), ready = new Promise<void>(resolve => entered = resolve)
  await page.route('**/api/backups?**', async route => { entered(); await held; await route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":"unavailable"}' }) })
  await page.getByRole('button', { name: 'Refresh recovery protection', exact: true }).click(); await ready
  await expect(page.locator('.backup-protection-card')).toHaveAttribute('aria-busy', 'true')
  await expect.soft(page.locator('.backup-counts')).toHaveText('Checking counts…')
  await captureOffsite(page, 'protection-read-pending-desktop'); release()
  await expect(page.getByRole('heading', { name: 'Status unavailable' })).toBeVisible()
  await expect.soft(page.locator('.backup-counts')).toHaveText('Counts unavailable')
  await captureOffsite(page, 'protection-read-error-desktop')
  await page.unroute('**/api/backups?**'); await page.getByRole('button', { name: 'Retry recovery protection', exact: true }).click()
  await expect(page.locator('.backup-attempt')).toHaveCount(1)
  await expect(page.locator('.backup-counts')).toHaveText('0 confirmed1 need confirmation0 in progress')
  const point = String(db('SELECT id FROM offsite_backup_jobs')[0][0]), path = '**/api/backups/' + point + '?**'
  await page.route(path, async route => route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":"unavailable"}' }))
  await page.locator('.backup-attempt').click(); await expect(page.getByRole('heading', { name: 'This history is unavailable.' })).toBeVisible()
  await expect.soft(page.locator('.backup-dialog-heading p')).not.toContainText('Opening this recovery point')
  await captureOffsite(page, 'recovery-history-read-error-desktop'); await page.unroute(path)
  await page.getByRole('button', { name: 'Retry recovery history', exact: true }).click(); await expect(page.locator('.backup-timeline li')).toHaveCount(3)
  await page.getByRole('button', { name: 'Close recovery point', exact: true }).click()
  let releaseDetail!: () => void
  const detailHeld = new Promise<void>(resolve => releaseDetail = resolve)
  await page.route(path, async route => { await detailHeld; await route.continue().catch(() => {}) })
  await page.locator('.backup-attempt').click(); await expect(page.getByText('Loading recovery history…', { exact: true })).toBeVisible()
  await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0)
  releaseDetail(); await page.unroute(path)
})

test('actual pause retains a 40001-paise draft and its retry identity until independent custody verification restores protection', async () => {
  const page = operator; await page.setViewportSize({ width: 375, height: 812 })
  await receivedDraft(page, 'Supplied received record awaiting verified protection')
  const dialog = page.getByRole('dialog'); await dialog.getByRole('checkbox').check()
  const requests: unknown[] = [], paths: string[] = []
  page.on('request', request => { if (request.method() === 'POST' && /\/api\/entries\/[^/]+\/post$/.test(request.url())) { requests.push(request.postDataJSON()); paths.push(new URL(request.url()).pathname) } })
  await dialog.getByRole('button', { name: 'Confirm this entry', exact: true }).click()
  await expect(dialog.getByRole('alert')).toContainText('New financial changes are paused')
  await captureOffsite(page, 'actual-paused-human-entry-375')
  expect(db("SELECT state,amount_paise FROM entries")).toEqual([['DRAFT', 40001]])
  expect(db('SELECT COUNT(*) FROM receipts')[0][0]).toBe(0)
  await reconcileCurrent(page)
  await expect(dialog.getByRole('heading', { name: 'Review this draft.' })).toBeVisible()
  await dialog.getByRole('button', { name: 'Confirm this entry', exact: true }).click()
  await expect(dialog.getByRole('heading', { name: 'A clear record.' })).toBeVisible()
  expect(requests).toHaveLength(2); expect(requests[0]).toEqual(requests[1]); expect(paths[0]).toBe(paths[1])
  expect(db("SELECT state,amount_paise FROM entries")).toEqual([['POSTED', 40001]])
  expect(db('SELECT COUNT(*) FROM receipts')[0][0]).toBe(1)
  await expect(dialog.getByRole('button', { name: 'Download PDF', exact: true })).toBeEnabled()
  await captureOffsite(page, 'confirmed-original-receipt-after-custody-375')
  await page.getByRole('button', { name: 'Close entry details', exact: true }).click()
  await navigate(page, 'Backups & recovery'); await page.getByRole('button', { name: 'Refresh recovery protection', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Records protected' })).toBeVisible()
  await expect(page.locator('.backup-attempt .backup-status')).toHaveText('Confirmed')
  await captureOffsite(page, 'independently-confirmed-protection-375')
  await page.locator('.backup-attempt').click(); await expect(page.getByText('Custodian verified the recovered copy', { exact: true })).toBeVisible()
  await captureOffsite(page, 'independent-custody-history-375'); await page.getByRole('button', { name: 'Close recovery point', exact: true }).click()
})

test('warning, stale and clock error projections keep their independent age and pending-copy expectations', async () => {
  const page = operator
  const original = await (await page.request.get('/api/backups')).json()
  for (const [phase, age, paused] of [['WARNING', 2700, false], ['PAUSED', 3600, true], ['CLOCK_ERROR', null, true]] as const) {
    const capture=age===null?original.as_of+60:original.as_of-age
    await page.route('**/api/backups?**', async route => route.fulfill({ json: { ...original, phase, capture_at:capture, age_seconds: age, finance_paused: paused, status_key: 'controlled-' + phase } }))
    await page.getByRole('button', { name: 'Refresh recovery protection', exact: true }).click()
    await expect(page.locator('.backup-protection-card')).toHaveClass(new RegExp('backup-phase-' + phase.toLowerCase()))
    await captureOffsite(page, 'projected-' + phase.toLowerCase() + '-375')
    await page.unroute('**/api/backups?**')
  }
  await page.route('**/api/backups?**',async route=>route.fulfill({json:{...original,phase:'HEALTHY',age_seconds:0,finance_paused:false,staging_full:true,status_key:'controlled-capacity'}}))
  await navigate(page,'Overview')
  await expect(page.locator('.backup-protection-notice')).toContainText('Recovery staging needs attention')
  await page.setViewportSize({width:320,height:440})
  await page.locator('.backup-protection-notice').scrollIntoViewIfNeeded()
  const body=(await page.locator('.backup-protection-notice p').boundingBox())!
  const link=(await page.getByRole('link',{name:'Check protection',exact:false}).boundingBox())!
  expect(body.width).toBeGreaterThanOrEqual(180);expect(link.y).toBeGreaterThanOrEqual(body.y+body.height)
  await pageFits(page);await captureOffsite(page,'projected-capacity-overview-short320')
  await page.getByRole('link',{name:'Check protection',exact:false}).click()
  await expect(page.locator('.backup-protection-card [role="alert"]')).toContainText('Local recovery staging is full')
  await captureOffsite(page,'projected-capacity-protection-short320')
  await page.unroute('**/api/backups?**')
  await page.getByRole('button', { name: 'Refresh recovery protection', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Records protected' })).toBeVisible()
})

test('residents have no recovery navigation or global history and direct URLs preserve access denial', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 440 }); await login(page, 'Owner')
  await expect(page.getByRole('link', { name: 'Backups & recovery', exact: true })).toHaveCount(0)
  expect((await page.request.get('/api/backups')).status()).toBe(403)
  await page.goto('/#backups'); await expect(page.getByRole('heading', { name: 'Operator access required.' })).toBeVisible()
  await pageFits(page); await captureOffsite(page, 'resident-recovery-denied-short320')
})

test('retained attempts and history paginate with full counts while changing filters resets the page', async () => {
  const page=operator
  const binding=String(db('SELECT config_hash FROM offsite_policy')[0][0])
  const captured=Number(db('SELECT captured_at FROM offsite_backup_jobs')[0][0])-120
  const ids:string[]=[]
  for(let i=0;i<13;i++) {
    const id=('fictional-retained-'+String(i).padStart(2,'0')).padEnd(43,'x'); ids.push(id)
    db("INSERT INTO offsite_backup_jobs(id,object_key,config_hash,captured_at,state,attempts,available_at,lease_token,lease_until,error_code) VALUES(?,?,?,?,'FAILED',5,0,'',0,'CAPTURE_FAILED')",[id,'recovery/society/fictional-history/'+id,binding,captured-i])
  }
  for(let i=0;i<17;i++) db("INSERT INTO offsite_backup_events(job_id,action,actor_kind,note,occurred_at) VALUES(?,'FAILED','SYSTEM',?,?)",[ids[0],'Fictional retained event '+i,captured+i])
  await page.setViewportSize({width:320,height:440})
  await navigate(page,'Backups & recovery')
  await page.getByRole('button',{name:'Refresh recovery protection',exact:true}).click()
  await expect(page.locator('.backup-attempt')).toHaveCount(12)
  const response=await (await page.request.get('/api/backups?state=FAILED&page=2')).json()
  expect(response.total).toBe(13);expect(response.items).toHaveLength(1);expect(response.counts.FAILED).toBe(13);expect(response.counts.VERIFIED).toBe(1)
  await chooseOption(page,'Filter recovery attempts','Failed attempts')
  await expect(page.getByText('Page 1 of 2',{exact:true})).toBeVisible()
  await page.getByRole('button',{name:'Next recovery attempts page',exact:true}).click()
  await expect(page.getByText('Page 2 of 2',{exact:true})).toBeVisible();await expect(page.locator('.backup-attempt')).toHaveCount(1)
  await pageFits(page);await captureOffsite(page,'retained-attempts-page2-short320')
  await page.getByRole('button',{name:'Previous recovery attempts page',exact:true}).click()
  await expect(page.locator('.backup-attempt')).toHaveCount(12)
  await page.locator('.backup-attempt').first().click()
  await expect(page.locator('.backup-timeline li')).toHaveCount(12)
  await expect(page.getByText('History page 1 of 2',{exact:true})).toBeVisible()
  await page.getByRole('button',{name:'Next recovery history page',exact:true}).click()
  await expect(page.getByText('History page 2 of 2',{exact:true})).toBeVisible();await expect(page.locator('.backup-timeline li')).toHaveCount(5)
  await page.locator('.backup-dialog-body').evaluate(element=>{element.scrollTop=element.scrollHeight})
  await expect(page.getByText('Fictional retained event 0',{exact:true})).toBeInViewport()
  await expect(page.getByRole('button',{name:'Close recovery point',exact:true})).toBeInViewport()
  await dialogFits(page);await captureOffsite(page,'retained-history-page2-short320')
  await page.keyboard.press('Escape');await expect(page.locator('.backup-attempt').first()).toBeFocused()
  await chooseOption(page,'Filter recovery attempts','Confirmed copies')
  await expect(page.locator('.backup-attempt')).toHaveCount(1)
  await expect(page.getByRole('button',{name:'Next recovery attempts page',exact:true})).toHaveCount(0)
  await expect(page.getByText('1 confirmed',{exact:false})).toBeVisible()
})
