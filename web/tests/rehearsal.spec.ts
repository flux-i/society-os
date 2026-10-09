import { test, expect } from '@playwright/test'
import { closeSync, copyFileSync, mkdirSync, openSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { createHash } from 'node:crypto'
import { execFileSync, spawn } from 'node:child_process'
import { createServer } from 'node:net'
import type { Page } from '@playwright/test'
import { chooseOption, navigate } from './helpers'
import { pageFits, dialogFits } from './registry-import-fixtures'
import { artifactFolder, capture, createRehearsal, entity, homeFinance, loginPerson, personalPassword, prepareVisibility, readDB, visibility } from './rehearsal-fixtures'
import type { RehearsalActors } from './rehearsal-fixtures'
import { plainPDF } from './document-fixtures'

test.describe.configure({ mode: 'serial' })
test.use({ actionTimeout: 10000 })
test.setTimeout(120000)
let actors: RehearsalActors
let receiptID = '', receivedID = '', originalSnapshot = '', documentID = '', reportID = ''

test.beforeAll(async ({ browser }) => { actors = await createRehearsal(browser) })
test.afterAll(async () => { await actors?.close() })

test('supplied twelve-home import and visible personal activation separate registry, Treasury, review and household access', async () => {
  expect(readDB('SELECT (SELECT COUNT(*) FROM flats),(SELECT COUNT(*) FROM residents),(SELECT COUNT(*) FROM flat_memberships),(SELECT COUNT(*) FROM users),(SELECT COUNT(*) FROM users WHERE is_demo=1),(SELECT COUNT(*) FROM flat_memberships WHERE can_view_finances=1),(SELECT COUNT(*) FROM entries),(SELECT COUNT(*) FROM receipts)')[0]).toEqual([12,17,18,6,0,0,0,0])
  const registry = await (await actors.registry.request.get('/api/auth/me')).json()
  expect(registry.can_manage_accounts).toBe(true); expect(registry.can_manage_records).toBe(false)
  for (const person of [actors.finance, actors.secondFinance]) {
    const me = await (await person.page.request.get('/api/auth/me')).json()
    expect(me.can_manage_records).toBe(true); expect(me.mfa_enrolled).toBe(true); expect(me.is_demo).toBe(false); expect(me.can_manage_accounts).toBe(false)
  }
  for (const person of [actors.owner, actors.tenant]) {
    expect((await person.page.request.get('/api/entries')).status()).toBe(403)
    expect((await person.page.request.get('/api/flats/'+entity('HOME','a-101')+'/finance-visibility')).status()).toBe(403)
  }
  await navigate(actors.registry,'Access & invitations'); const search=actors.registry.getByRole('searchbox',{name:'Search accounts',exact:true}); await search.focus()
  const focus=await search.evaluate(element=>{const field=element.closest('label')!,icon=field.querySelector('svg')!.getBoundingClientRect(),input=element.getBoundingClientRect();return {inputOutline:getComputedStyle(element).outlineStyle,frameShadow:getComputedStyle(field).boxShadow,inputLeft:input.left,iconRight:icon.right}})
  await capture(actors.registry,'account-search-keyboard-focus-desktop')
  expect(focus.inputLeft-focus.iconRight).toBeGreaterThanOrEqual(8); expect(focus.inputOutline).toBe('none'); expect(focus.frameShadow).not.toBe('none')
  await navigate(actors.registry,'Homes & people'); await actors.registry.getByRole('button',{name:'View home A-101',exact:true}).click()
  await expect(actors.registry.getByRole('button',{name:'Financial access',exact:true})).toHaveCount(0)
  await capture(actors.registry,'registry-separate-authority-desktop'); await actors.registry.getByRole('button',{name:'Close details',exact:true}).click()
  await navigate(actors.owner.page,'Your homes'); await expect(actors.owner.page.getByText('2 homes found',{exact:true})).toBeVisible()
  await navigate(actors.tenant.page,'Your homes'); await expect(actors.tenant.page.getByText('1 homes found',{exact:true})).toBeVisible()
})

test('household access menus, selection, focus, verified input, internal scrolling and dismissal fit all five layouts', async () => {
  const page=actors.finance.page
  for (const [width,height] of [[1440,1000],[820,1050],[375,812],[320,740],[320,440]]) {
    await page.setViewportSize({width,height}); await homeFinance(page); await dialogFits(page); await expect(page.getByRole('button',{name:'Close details',exact:true})).toBeInViewport({ratio:1})
    await expect(page.locator('.household-finance-person')).toHaveCount(2)
    await capture(page,`access-people-${width}-${height}`)
    await page.getByRole('button',{name:'Review financial access for Sample Owner A 101',exact:true}).click()
    const select=page.getByRole('combobox',{name:'Household financial visibility',exact:true})
    await select.scrollIntoViewIfNeeded(); await select.focus(); await page.keyboard.press('Space')
    await expect(page.getByRole('option',{name:'Allow household view',exact:true})).toHaveAttribute('aria-selected','true')
    await page.getByRole('option',{name:'Withhold household view',exact:true}).hover(); await capture(page,`access-menu-${width}-${height}`)
    await page.keyboard.press('End'); await expect(page.getByRole('option',{name:'Withhold household view',exact:true})).toBeFocused(); await page.keyboard.press('Enter')
    await expect(select).toContainText('Withhold household view'); await expect(select).toBeFocused()
    await chooseOption(page,'Household financial visibility','Allow household view')
    await page.getByLabel('Financial access verification note',{exact:true}).fill('Retained verified input for this responsive household review.')
    await page.getByRole('checkbox',{name:'I verified this person’s current household relationship and financial access.'}).check()
    await page.getByRole('button',{name:'Allow household view',exact:true}).evaluate(element=>{const body=element.closest('.dialog-scroll')!,button=element.getBoundingClientRect(),viewport=body.getBoundingClientRect();body.scrollBy({top:button.top+button.height/2-viewport.top-viewport.height/2,behavior:'instant'})}); await expect(page.getByRole('button',{name:'Allow household view',exact:true})).toBeInViewport({ratio:1})
    const check=await page.getByRole('checkbox',{name:'I verified this person’s current household relationship and financial access.'}).evaluate(input=>{const label=input.closest('label')!,node=[...label.childNodes].find(node=>node.nodeType===Node.TEXT_NODE&&node.textContent?.trim())!,range=document.createRange();range.selectNodeContents(node);const text=range.getBoundingClientRect(),box=input.getBoundingClientRect();return {inputRight:box.right,textLeft:text.left,inputTop:box.top,textTop:text.top,width:box.width,height:box.height}})
    expect(check.textLeft).toBeGreaterThanOrEqual(check.inputRight+8); expect(Math.abs(check.inputTop-check.textTop)).toBeLessThanOrEqual(6); expect(check.width).toBeGreaterThanOrEqual(18); expect(check.height).toBeGreaterThanOrEqual(18)
    await dialogFits(page); await expect(page.getByRole('button',{name:'Close details',exact:true})).toBeInViewport({ratio:1}); await capture(page,`access-review-${width}-${height}`)
    await page.keyboard.press('Escape'); await expect(page.getByRole('dialog')).toHaveCount(0)
    await expect(page.getByRole('button',{name:'View home A-101',exact:true})).toBeFocused(); await pageFits(page)
  }
  expect(readDB('SELECT COUNT(*) FROM household_finance_actions')[0][0]).toBe(0)
})

test('access reads recover from failure and empty search; real reauthentication and a lost accepted reply retain one exact decision', async () => {
  const page=actors.finance.page;await page.setViewportSize({width:1440,height:1000})
  const read=/\/api\/flats\/[^/]+\/finance-visibility\?.*$/
  let releaseRead!:()=>void;const heldRead=new Promise<void>(resolve=>releaseRead=resolve)
  await page.route(read,async route=>{await heldRead;await route.fulfill({status:503,contentType:'application/json',body:'{}'})})
  await navigate(page,'Homes & people');await page.getByRole('button',{name:'View home A-101',exact:true}).click();await page.getByRole('button',{name:'Financial access',exact:true}).click()
  await expect(page.getByText('Opening current financial access…',{exact:true})).toBeVisible();await capture(page,'access-loading-desktop');releaseRead()
  await expect(page.getByRole('alert')).toBeVisible();await capture(page,'access-read-error-desktop');await page.unroute(read);await page.getByRole('button',{name:'Try financial access again',exact:true}).click()
  await expect(page.locator('.household-finance-person')).toHaveCount(2)
  await page.getByRole('searchbox',{name:'Search financial access',exact:true}).fill('No current supplied person')
  await expect(page.getByRole('heading',{name:'No current people match.'})).toBeVisible();await capture(page,'access-empty-search-desktop');await page.getByRole('button',{name:'Clear financial search',exact:true}).click()
  await prepareVisibility(page,'Sample Owner A 101')
  const write=/\/api\/flats\/[^/]+\/finance-visibility$/;let calls=0;const requests:unknown[]=[]
  let release!:()=>void;const held=new Promise<void>(resolve=>release=resolve)
  await page.route(write,async route=>{
    calls++;requests.push(route.request().postDataJSON())
    if(calls===1){await route.fulfill({status:403,contentType:'application/json',body:'{"error":"reauthentication_required"}'});return}
    const result=await route.fetch();expect(result.status()).toBe(200);await held;await route.fulfill({response:result,body:'{'})
  })
  await page.getByRole('button',{name:'Allow household view',exact:true}).click();await expect(page.getByRole('heading',{name:'Confirm it’s you.'})).toBeVisible()
  expect(calls).toBe(1);await page.getByLabel('Current password',{exact:true}).fill(personalPassword)
  await page.getByRole('button',{name:'Use a recovery code',exact:true}).click()
  const recovery=actors.finance.recovery.shift();expect(recovery).toBeTruthy();await page.getByLabel('Recovery code',{exact:true}).fill(recovery!)
  await page.getByRole('button',{name:'Confirm my identity',exact:true}).click()
  await expect(page.getByText('Identity confirmed. Retry the retained request when you are ready.',{exact:true})).toBeVisible()
  expect(calls).toBe(1);await expect(page.getByLabel('Financial access verification note',{exact:true})).toHaveValue('Verified the supplied current household relationship and this deliberate financial view.')
  await capture(page,'access-retained-reauth-desktop');await page.getByRole('button',{name:'Retry the same access decision',exact:true}).click()
  await expect(page.getByRole('button',{name:'Recording this decision…',exact:true})).toBeDisabled();await expect(page.getByRole('button',{name:'Close details',exact:true})).toBeDisabled()
  await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toBeVisible();await expect(page.getByRole('button',{name:'People',exact:true})).toBeDisabled();await capture(page,'access-pending-desktop')
  await expect.poll(()=>readDB('SELECT COUNT(*) FROM household_finance_actions')[0][0]).toBe(1);release()
  await expect(page.getByRole('alert')).toContainText('connection was interrupted');await page.unroute(write)
  expect(requests[1]).toEqual(requests[0]);expect(calls).toBe(2)
  await page.getByRole('button',{name:'Check saved access decision',exact:true}).click()
  await expect(page.getByRole('status').filter({hasText:'is recorded. Current household access is refreshed below.'})).toBeVisible()
  expect(readDB("SELECT (SELECT COUNT(*) FROM household_finance_actions),(SELECT COUNT(*) FROM audit_events WHERE action='HOUSEHOLD_FINANCE_GRANT'),(SELECT COUNT(*) FROM entries),(SELECT COUNT(*) FROM receipts)")[0]).toEqual([1,1,0,0])
  await capture(page,'access-original-acknowledgement-desktop')
  await homeFinance(page,'A-102');await visibility(page,'Sample Owner A 101')
  await homeFinance(page,'A-103');await visibility(page,'Sample Tenant A 103');await page.getByRole('button',{name:'Close details',exact:true}).click()
  await actors.owner.page.reload();await actors.tenant.page.reload()
  expect((await actors.owner.page.request.get('/api/auth/me')).status()).toBe(200)
  expect((await (await actors.owner.page.request.get('/api/auth/me')).json()).can_read_records).toBe(true)
})

async function manualEntry(page:Page,kind:string,amount:string,description:string) {
  await navigate(page,'Entries');await page.getByRole('button',{name:'Add an entry',exact:true}).click()
  await chooseOption(page,'Entry home','Home A-101');await chooseOption(page,'Entry type',kind)
  await page.getByRole('textbox',{name:'Amount in rupees',exact:true}).fill(amount);await page.getByLabel('Entry date',{exact:true}).fill('2026-10-09')
  await page.getByRole('textbox',{name:'Description',exact:true}).fill(description)
  if(kind==='Money received'){await page.getByRole('textbox',{name:'Received from',exact:true}).fill('Sample Owner A 101');await chooseOption(page,'Received via','Bank transfer');await page.getByRole('textbox',{name:'Reference',exact:true}).fill('SUPPLIED-REHEARSAL-400')}
  const draft=page.waitForResponse(r=>r.url().endsWith('/api/entries')&&r.request().method()==='POST')
  await page.getByRole('button',{name:'Save draft for review',exact:true}).click();const id=(await(await draft).json()).id
  await expect(page.getByRole('heading',{name:'Review this draft.'})).toBeVisible()
  return id as string
}

test('manual records independently reach 110001 paise and produce one frozen society receipt; correction restores 150001 paise and preserves it',async()=>{
  const page=actors.finance.page
  for(const [kind,amount,description] of [['Opening amount due','1000.00','Supplied opening balance'],['Given charge','500.01','Supplied maintenance charge'],['Money received','400.00','Supplied money already received']]){
    const before=readDB("SELECT COALESCE(SUM(CASE WHEN kind='RECEIVED' THEN -amount_paise ELSE amount_paise END),0) FROM entries WHERE state='POSTED' AND id NOT IN (SELECT entry_id FROM entry_reversals)")[0][0]
    const id=await manualEntry(page,kind,amount,description)
    expect(readDB("SELECT COALESCE(SUM(CASE WHEN kind='RECEIVED' THEN -amount_paise ELSE amount_paise END),0) FROM entries WHERE state='POSTED' AND id NOT IN (SELECT entry_id FROM entry_reversals)")[0][0]).toBe(before)
    await page.getByRole('dialog').getByRole('checkbox').check();await page.getByRole('button',{name:'Confirm this entry',exact:true}).click()
    if(kind==='Money received'){
      receivedID=id;await expect(page.getByRole('button',{name:'Download PDF',exact:true})).toBeEnabled()
      receiptID=readDB('SELECT id FROM receipts WHERE entry_id=?',[id])[0][0] as string
      originalSnapshot=readDB('SELECT snapshot_json FROM receipts WHERE id=?',[receiptID])[0][0] as string
      expect(JSON.parse(originalSnapshot)).toMatchObject({amount_paise:40000,issuer:{format_version:1,name:'The Neighbourhood Rehearsal',mode:'FICTIONAL_REHEARSAL'}})
      await capture(page,'confirmed-receipt-desktop');const downloading=page.waitForEvent('download');await page.getByRole('button',{name:'Download PDF',exact:true}).click()
      const file=await downloading;mkdirSync(artifactFolder(),{recursive:true,mode:0o700});const path=join(artifactFolder(),'supplied-receipt.pdf');await file.saveAs(path)
      const text=execFileSync('pdftotext',['-layout',path,'-'],{encoding:'utf8'});expect(text).toContain('The Neighbourhood Rehearsal');expect(text).toContain('FICTIONAL REHEARSAL');expect(text).toContain('₹400.00')
    }
    await page.getByRole('button',{name:'Close entry details',exact:true}).click()
  }
  expect(readDB("SELECT (SELECT COUNT(*) FROM entries WHERE state='POSTED' AND id NOT IN (SELECT entry_id FROM entry_reversals)),(SELECT COUNT(*) FROM receipts),(SELECT SUM(CASE WHEN kind='RECEIVED' THEN -amount_paise ELSE amount_paise END) FROM entries WHERE state='POSTED' AND id NOT IN (SELECT entry_id FROM entry_reversals))")[0]).toEqual([3,1,110001])
  await navigate(actors.owner.page,'Entries');await expect(actors.owner.page.getByRole('button',{name:'Add an entry',exact:true})).toHaveCount(0);await capture(actors.owner.page,'household-ledger-desktop')
  expect((await actors.tenant.page.request.get('/api/receipts/'+receiptID+'/download')).status()).toBe(404)
  await page.goto('/#entries?entry='+receivedID)
  await expect(page.getByRole('button',{name:'Correct this entry',exact:true})).toBeVisible();await page.getByRole('button',{name:'Correct this entry',exact:true}).click()
  await page.getByRole('textbox',{name:'Reason for reversal',exact:true}).fill('Supplied reference correction preserves the original receipt and audit.')
  await page.getByRole('dialog').getByRole('checkbox').check();await page.getByRole('button',{name:'Confirm reversal',exact:true}).click()
  await expect(page.getByText('This entry has been reversed.',{exact:true})).toBeVisible();await capture(page,'retained-receipt-correction-desktop')
  expect(readDB("SELECT SUM(CASE WHEN kind='RECEIVED' THEN -amount_paise ELSE amount_paise END) FROM entries WHERE state='POSTED' AND id NOT IN (SELECT entry_id FROM entry_reversals)")[0][0]).toBe(150001)
  expect(readDB('SELECT snapshot_json FROM receipts WHERE id=?',[receiptID])[0][0]).toBe(originalSnapshot)
  const download=page.waitForEvent('download');await page.getByRole('button',{name:'Download original',exact:true}).click();expect(readFileSync((await(await download).path())!)).toEqual(readFileSync(join(artifactFolder(),'supplied-receipt.pdf')))
  await page.getByRole('button',{name:'Close entry details',exact:true}).click()
})

test('different-person expense and notice approvals, approved private originals and resident reports keep money and household boundaries',async()=>{
  const owner=actors.owner.page,reviewer=actors.reviewer.page,registry=actors.registry
  const before=readDB('SELECT (SELECT COUNT(*) FROM entries),(SELECT COUNT(*) FROM receipts)')[0]
  await navigate(owner,'Your requests');await owner.getByRole('button',{name:'Submit a request',exact:true}).click();await chooseOption(owner,'Request type','Expense proposal');await chooseOption(owner,'Request home','Home A-101')
  await owner.getByRole('textbox',{name:'Title',exact:true}).fill('Rehearsal garden expense');await owner.getByRole('textbox',{name:'Description',exact:true}).fill('Supplied fictional request for a different person to review.');await owner.getByRole('textbox',{name:'Estimated amount in rupees (optional)',exact:true}).fill('123.45')
  await owner.getByRole('button',{name:'Send for review',exact:true}).click();await expect(owner.getByRole('button',{name:'Approve request',exact:true})).toHaveCount(0);await owner.getByRole('button',{name:'Close request details',exact:true}).click()
  await navigate(reviewer,'Requests & approvals');await reviewer.getByRole('searchbox',{name:'Search requests',exact:true}).fill('Rehearsal garden expense');await reviewer.getByRole('button',{name:'Open Rehearsal garden expense',exact:true}).click()
  await chooseOption(reviewer,'Review decision','Approve for follow-up');await reviewer.getByRole('textbox',{name:'Reason for this decision',exact:true}).fill('Separately reviewed supplied expense; this decision posts no money.');await reviewer.getByRole('dialog').getByRole('checkbox').check();await reviewer.getByRole('button',{name:'Approve request',exact:true}).click()
  await expect(reviewer.getByRole('dialog').getByText('Approved',{exact:true}).first()).toBeVisible();await capture(reviewer,'separate-expense-approval-desktop');await reviewer.getByRole('button',{name:'Close request details',exact:true}).click()
  await navigate(registry,'Requests & approvals');await registry.getByRole('button',{name:'Submit a request',exact:true}).click();await chooseOption(registry,'Request type','Notice proposal');await chooseOption(registry,'Notice audience','Current owners')
  await registry.getByRole('textbox',{name:'Title',exact:true}).fill('Rehearsal owners notice');await registry.getByRole('textbox',{name:'Description',exact:true}).fill('A supplied fictional owners notice for separate review and current audience access.');await registry.getByRole('button',{name:'Send for review',exact:true}).click()
  await registry.getByRole('button',{name:'Close request details',exact:true}).click();await reviewer.getByRole('searchbox',{name:'Search requests',exact:true}).fill('Rehearsal owners notice');await reviewer.getByRole('button',{name:'Open Rehearsal owners notice',exact:true}).click();await chooseOption(reviewer,'Review decision','Approve & publish notice')
  await reviewer.getByRole('textbox',{name:'Reason for this decision',exact:true}).fill('Separately reviewed this exact fictional notice and owner audience.');await reviewer.getByRole('dialog').getByRole('checkbox').check();await reviewer.getByRole('button',{name:'Approve & publish',exact:true}).click();await reviewer.getByRole('button',{name:'Close request details',exact:true}).click()
  await navigate(owner,'Community');await owner.getByRole('searchbox',{name:'Search notices',exact:true}).fill('Rehearsal owners notice');await expect(owner.getByRole('status')).toHaveText('1 notices found');await capture(owner,'scoped-owners-notice-desktop')
  await navigate(actors.tenant.page,'Community');await actors.tenant.page.getByRole('searchbox',{name:'Search notices',exact:true}).fill('Rehearsal owners notice');await expect(actors.tenant.page.getByRole('status')).toHaveText('0 notices found')
  await navigate(owner,'Documents');await owner.getByRole('button',{name:'Add a document',exact:true}).click();await owner.getByLabel('Document file',{exact:true}).setInputFiles({name:'private-rehearsal.pdf',mimeType:'application/pdf',buffer:plainPDF()});await owner.getByRole('textbox',{name:'Title',exact:true}).fill('Rehearsal private household original')
  const uploaded=owner.waitForResponse(r=>/\/api\/documents\/[^/]+\/content$/.test(r.url())&&r.request().method()==='POST');await owner.getByRole('button',{name:'Upload for review',exact:true}).click();documentID=(await(await uploaded).json()).id;await expect(owner.getByRole('button',{name:'Download original',exact:true})).toBeEnabled();await owner.getByRole('button',{name:'Close document details',exact:true}).click()
  await navigate(reviewer,'Documents');await reviewer.getByRole('searchbox',{name:'Search documents',exact:true}).fill('Rehearsal private household original');await reviewer.getByRole('button',{name:'Open document Rehearsal private household original',exact:true}).click();await chooseOption(reviewer,'Document decision','Approve this version');await reviewer.getByRole('textbox',{name:'Document decision reason',exact:true}).fill('Separately reviewed this exact fictional private original and its audience.');await reviewer.getByRole('checkbox',{name:'I have checked this version, its audience and my decision.'}).check();await reviewer.getByRole('button',{name:'Save document decision',exact:true}).click();await expect(reviewer.getByRole('dialog').locator('.review-state')).toHaveText('Approved');await capture(reviewer,'approved-private-original-desktop');await reviewer.getByRole('button',{name:'Close document details',exact:true}).click()
  expect((await actors.tenant.page.request.get('/api/documents/'+documentID)).status()).toBe(404)
  await navigate(owner,'Help & repairs');await owner.getByRole('button',{name:'Report an issue',exact:true}).click();await chooseOption(owner,'Service request home','Home A-101');await chooseOption(owner,'Service category','Plumbing');await owner.getByRole('textbox',{name:'Subject',exact:true}).fill('Rehearsal private household report');await owner.getByRole('textbox',{name:'Details',exact:true}).fill('A supplied private fictional resident issue for the appropriate authorized handler.')
  const reported=owner.waitForResponse(r=>r.url().endsWith('/api/complaints')&&r.request().method()==='POST');await owner.getByRole('button',{name:'Save service request',exact:true}).click();reportID=(await(await reported).json()).id;await expect(owner.getByRole('dialog')).toContainText('Rehearsal private household report');await capture(owner,'private-resident-report-desktop');await owner.getByRole('button',{name:'Close service request details',exact:true}).click()
  expect((await actors.tenant.page.request.get('/api/complaints/'+reportID)).status()).toBe(404)
  expect(readDB('SELECT (SELECT COUNT(*) FROM entries),(SELECT COUNT(*) FROM receipts)')[0]).toEqual(before)
})

test('revocation invalidates a loaded home even while another home keeps financial access, and ended tenancy denies current home reads',async()=>{
  const finance=actors.finance.page,owner=actors.owner.page,registry=actors.registry,tenant=actors.tenant.page
  await owner.goto('/#entries?entry='+receivedID);await expect(owner.getByRole('button',{name:'Download original',exact:true})).toBeEnabled()
  const before=(await(await owner.request.get('/api/auth/me')).json()).scope_key
  await homeFinance(finance);await visibility(finance,'Sample Owner A 101','Withhold household view');await finance.getByRole('button',{name:'Close details',exact:true}).click()
  const after=await(await owner.request.get('/api/auth/me')).json();expect(after.can_read_records).toBe(true);expect(after.scope_key).not.toBe(before)
  await owner.evaluate(()=>window.dispatchEvent(new Event('session-recheck')))
  await expect(owner.getByRole('button',{name:'Download original',exact:true})).toHaveCount(0)
  expect((await owner.request.get('/api/receipts/'+receiptID+'/download')).status()).toBe(404)
  const firstKey=readDB("SELECT operation_key FROM household_finance_actions WHERE flat_id=? ORDER BY created_at,id LIMIT 1",[entity('HOME','a-101')])[0][0] as string
  const reply=await(await finance.request.get('/api/flats/'+entity('HOME','a-101')+'/finance-visibility/operations/'+firstKey)).json();expect(reply.visible).toBe(true)
  expect(readDB("SELECT can_view_finances FROM flat_memberships WHERE flat_id=? AND resident_id=?",[entity('HOME','a-101'),entity('PERSON','owner-a-101')])[0][0]).toBe(0)
  await navigate(registry,'Homes & people');await registry.getByRole('button',{name:'View home A-103',exact:true}).click();await registry.getByRole('button',{name:'Manage home',exact:true}).click();await registry.getByRole('button',{name:'End relationship',exact:true}).click()
  await chooseOption(registry,'Relationship to end','Sample Tenant A 103 · Tenant');await registry.getByLabel('End date',{exact:true}).fill(new Date().toLocaleDateString('en-CA',{timeZone:'Asia/Kolkata'}));await registry.getByLabel('Reason for this change',{exact:true}).fill('Verified supplied fictional tenancy ending on the current local date.');await registry.getByRole('button',{name:'End this relationship',exact:true}).click();await expect(registry.getByRole('status').filter({hasText:'relationship'})).toBeVisible();await registry.getByRole('button',{name:'Close details',exact:true}).click()
  expect((await tenant.request.get('/api/flats/'+entity('HOME','a-103'))).status()).toBe(404);expect((await tenant.request.get('/api/entries')).status()).toBe(403)
  await tenant.reload();await navigate(tenant,'Your homes');await expect(tenant.getByText('0 homes found',{exact:true})).toBeVisible();await tenant.setViewportSize({width:320,height:440});await capture(tenant,'ended-household-empty-320-440')
})

test('populated configured snapshot and restore preserve source, private originals, exact paise and current authority without reviving bearer sessions',async({browser})=>{
  const db=process.env.SOCIETY_BROWSER_DB!;expect(db).toContain('society-browser-')
  const folder=artifactFolder();mkdirSync(folder,{recursive:true,mode:0o700});const runtime=dirname(db),binary=join(runtime,'society-server'),snapshot=join(folder,'snapshot'),restored=join(folder,'restored','society.db')
  for(const name of ['mfa.key','messages.key']){const source=join(runtime,'keys',name==='messages.key'?'messages.key':'mfa.key');copyFileSync(source,join(folder,name))}
  const result=JSON.parse(execFileSync(binary,['snapshot','--db',db,'--out',snapshot],{encoding:'utf8'}))
  expect(result.schema_version).toBe(26)
  execFileSync(binary,['restore-check','--snapshot',snapshot,'--out',restored],{encoding:'utf8'})
  const verify=`import sqlite3,json,sys,hashlib
def state(path):
 c=sqlite3.connect(path)
 definitions=c.execute("SELECT type,name,sql FROM sqlite_schema WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY type,name").fetchall()
 tables=[x[1] for x in definitions if x[0]=='table' and x[1] not in ['sessions','account_tokens','mfa_pending','mfa_recovery_codes']]
 rows={t:sorted([list(r) for r in c.execute('SELECT * FROM "'+t+'"')],key=repr) for t in tables}
 return c,definitions,rows
a,ad,ar=state(sys.argv[1]);b,bd,br=state(sys.argv[2]);assert ad==bd and ar==br
assert b.execute('PRAGMA integrity_check').fetchone()[0]=='ok' and not b.execute('PRAGMA foreign_key_check').fetchall()
for t in ['sessions','account_tokens','mfa_pending','mfa_recovery_codes']:assert b.execute('SELECT COUNT(*) FROM '+t).fetchone()[0]==0
assert b.execute("SELECT SUM(CASE WHEN kind='RECEIVED' THEN -amount_paise ELSE amount_paise END) FROM entries WHERE state='POSTED' AND id NOT IN (SELECT entry_id FROM entry_reversals)").fetchone()[0]==150001
assert b.execute('SELECT COUNT(*) FROM receipts').fetchone()[0]==1
assert b.execute("SELECT COUNT(*) FROM flat_memberships WHERE flat_id=(SELECT entity_id FROM registry_import_entities WHERE entity_kind='HOME' AND source_id='a-101') AND can_view_finances=1").fetchone()[0]==0
print(json.dumps({'persistent_tables':len(ar)-1,'all_rows_and_definitions_preserved':True,'integrity_and_foreign_keys':True,'bearer_sessions_purged':True,'balance_paise':150001,'original_receipts':1}))`
  const proof=JSON.parse(execFileSync('python3',['-c',verify,db,restored],{encoding:'utf8'}));expect(proof.persistent_tables).toBe(100)
  expect(readDB('SELECT snapshot_json FROM receipts WHERE id=?',[receiptID])[0][0]).toBe(originalSnapshot)
  const issuer=JSON.parse(originalSnapshot).issuer;expect(issuer.name).toBe('The Neighbourhood Rehearsal')
  const hash=(path:string)=>createHash('sha256').update(readFileSync(path)).digest('hex')
  writeFileSync(join(folder,'recovery-proof.json'),JSON.stringify({proof,manifest:result,original_snapshot_sha256:createHash('sha256').update(originalSnapshot).digest('hex'),keys_preserved:['mfa.key','messages.key'].every(name=>hash(join(folder,name))===hash(join(runtime,'keys',name)))},null,2)+'\n',{mode:0o600})
  expect(JSON.parse(readFileSync(join(folder,'recovery-proof.json'),'utf8')).keys_preserved).toBe(true)
  const reservation=createServer();await new Promise<void>((resolve,reject)=>{reservation.once('error',reject);reservation.listen(0,'127.0.0.1',resolve)})
  const port=(reservation.address() as {port:number}).port;await new Promise<void>(resolve=>reservation.close(()=>resolve()))
  const origin='http://127.0.0.1:'+port,log=openSync(join(folder,'restored-server.log'),'w',0o600)
  const server=spawn(binary,['serve','--workspace','--db',restored,'--mfa-key-file',join(folder,'mfa.key'),'--message-key-file',join(folder,'messages.key'),'--addr','127.0.0.1:'+port,'--web-dir',join(runtime,'web')],{stdio:['ignore',log,log]})
  const contexts=[]
  try{
    await expect.poll(async()=>{try{return (await fetch(origin+'/ready')).status}catch{return 0}}).toBe(200)
    const cookies=await actors.owner.context.cookies(),previous=cookies.find(cookie=>cookie.name==='society_session')
    expect(previous).toBeTruthy();expect((await actors.owner.page.request.get(origin+'/api/auth/me',{headers:{Cookie:'society_session='+previous!.value}})).status()).toBe(401)
    for(const original of [actors.owner,actors.tenant]){
      const context=await browser.newContext({baseURL:origin,serviceWorkers:'block'});contexts.push(context)
      const page=await context.newPage();await loginPerson({page,context,name:original.name,email:original.email,recovery:[]})
      const identity=await(await page.request.get('/api/auth/me')).json();expect(identity.is_demo).toBe(false)
      expect(identity.can_manage_records).toBe(false);expect(identity.can_read_records).toBe(original===actors.owner)
      const homes=await(await page.request.get('/api/flats')).json();expect(homes.total).toBe(original===actors.owner?2:0)
      if(original===actors.owner){
        const records=await(await page.request.get('/api/entries')).json();expect(records.homes.map((home:{label:string})=>home.label)).toEqual(['A-102'])
        expect((await page.request.get('/api/receipts/'+receiptID+'/download')).status()).toBe(404)
        const content=await page.request.get('/api/documents/'+documentID+'/download');expect(content.status()).toBe(200);expect(await content.body()).toEqual(plainPDF())
        await navigate(page,'Entries');await capture(page,'restored-current-finance-home-desktop')
      }else{
        expect((await page.request.get('/api/complaints/'+reportID)).status()).toBe(404)
        expect((await page.request.get('/api/documents/'+documentID+'/download')).status()).toBe(404)
        await navigate(page,'Your homes');await page.setViewportSize({width:320,height:440});await capture(page,'restored-ended-household-320-440')
      }
    }
    const saved=JSON.parse(readFileSync(join(folder,'recovery-proof.json'),'utf8'));saved.restored_http={prior_bearer_rejected:true,personal_password_login:true,finance_home:'A-102',ended_tenant_homes:0,private_original_bytes_preserved:true};writeFileSync(join(folder,'recovery-proof.json'),JSON.stringify(saved,null,2)+'\n',{mode:0o600})
  }finally{
    await Promise.all(contexts.map(context=>context.close()))
    if(server.exitCode===null){const finished=new Promise<void>(resolve=>server.once('exit',()=>resolve()));server.kill('SIGTERM');const timer=setTimeout(()=>server.kill('SIGKILL'),6000);await finished;clearTimeout(timer)}
    closeSync(log)
  }
})
