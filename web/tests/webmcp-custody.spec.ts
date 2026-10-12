import { test, expect } from '@playwright/test'
import { login, navigate } from './helpers'
import { execute, names } from './native-webmcp-helpers'
import { db } from './registry-import-fixtures'
import { receivedDraft } from './offsite-helpers'
import { archiveCustodyCandidate, captureCustody, exhaustCustodyCandidate } from './custody-helpers'

test.setTimeout(120000)
test('actual native custody metadata rejects a held pre-disposition result and preserves the open human draft without granting financial protection', async ({ page }) => {
  await page.setViewportSize({width:375,height:812}); await login(page)
  const candidate = await exhaustCustodyCandidate(page)
  await expect.poll(() => names(page)).toContain('society_read_recovery_protection')
  await receivedDraft(page,'Native custody review preserves this human draft')
  let release!:()=>void, entered!:()=>void, calls=0
  const held=new Promise<void>(resolve=>release=resolve), ready=new Promise<void>(resolve=>entered=resolve)
  const results:{archived_count:number;staging_full:boolean;finance_paused:boolean;status_key:string}[]=[]
  const writes:string[]=[]
  page.on('request',request=>{if(!['GET','HEAD'].includes(request.method()))writes.push(request.url())})
  await page.route('**/api/backups',async route=>{
    const index=++calls, response=await route.fetch();expect(response.status()).toBe(200)
    results.push(await response.json())
    if(index===1){entered();await held}
    await route.fulfill({response}).catch(()=>{})
  })
  const reading=execute(page,'society_read_recovery_protection',{})
  await ready
  try {
    expect(results[0].archived_count).toBe(0);expect(results[0].staging_full).toBe(true)
    archiveCustodyCandidate(candidate.id,candidate.stage,'native-held-original')
    release();await expect(reading).rejects.toThrow(/UnknownError|Recovery protection changed/)
    expect(calls).toBe(2);expect(results[1].archived_count).toBe(1);expect(results[1].staging_full).toBe(false)
    expect(results[1].finance_paused).toBe(true);expect(results[1].status_key).not.toBe(results[0].status_key)
    await page.unroute('**/api/backups')
    const current=JSON.parse(await execute(page,'society_read_recovery_protection',{}))
    expect(current.archived_count).toBe(1);expect(current.finance_paused).toBe(true);expect(current.capture_at).toBe(0);expect(current.acknowledged_at).toBe(0)
    for(const secret of ['custody_note','archive_id','archive_manifest_sha256','ciphertext_sha256','object_key','config_hash','AGE-SECRET-KEY','snapshot.tar.gz.age'])expect(JSON.stringify(current)).not.toContain(secret)
    expect(await names(page)).not.toContain('society_archive_backup')
    await expect(execute(page,'society_archive_backup',{})).rejects.toThrow()
    expect(writes).toEqual([])
    await expect(page.getByRole('heading',{name:'Review this draft.'})).toBeVisible()
    await expect(page.getByRole('dialog').getByRole('checkbox')).not.toBeChecked()
    expect(db('SELECT state,amount_paise FROM entries')).toEqual([['DRAFT',40001]])
    expect(db('SELECT COUNT(*) FROM receipts')[0][0]).toBe(0)
    await page.setViewportSize({width:320,height:440});await captureCustody(page,'native-custody-human-review-short320')
    await page.getByRole('button',{name:'Close entry details',exact:true}).click()
    await navigate(page,'Backups & recovery')
    await page.locator('.backup-attempt').filter({hasText:'Independently retained'}).click()
    await expect(page.getByRole('heading',{name:'The original stays with us.'})).toBeVisible()
    await page.locator('.backup-custody-summary').scrollIntoViewIfNeeded()
    await captureCustody(page,'native-custody-current-history-short320')
  } finally {release();await page.unroute('**/api/backups')}
})
