import { test,expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { chmodSync,mkdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { login,navigate,chooseOption } from './helpers'
import { apiMaintenance,apiPublish,apiReceived,ensureMaintenanceReviewer,financialHeaders } from './maintenance-fixtures'

async function capture(page:Page,name:string) {
  if(!process.env.SOCIETY_CAPTURE_UI)return
  if(process.env.SOCIETY_CAPTURE_DETAIL_ONLY && !name.startsWith('period-review-actions-'))return
  await page.evaluate(()=>document.fonts.ready)
  await page.evaluate(async()=>{const finite=document.getAnimations().filter(animation=>animation.effect?.getComputedTiming().iterations!==Infinity);await Promise.allSettled(finite.map(animation=>animation.finished))})
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))
  if(await page.getByRole('dialog').count())await expect(page.getByRole('dialog').locator('.dialog-close')).toBeInViewport({ratio:1})
  const folder=resolve('../reports/local/maintenance-review');mkdirSync(folder,{recursive:true,mode:0o700})
  const path=resolve(folder,name+'.png')
  const scrollY=await page.evaluate(()=>window.scrollY)
  const modal=await page.getByRole('dialog').count()>0
  // Chromium captures a top-layer modal incorrectly over a scrolled document.
  // Stabilise only the QA background and restore it; the dialog's own scroll,
  // current focus and form data remain untouched.
  if(modal)await page.evaluate(()=>window.scrollTo({top:0,behavior:'instant'}))
  try{await page.screenshot({path,animations:'allow'});chmodSync(path,0o600)}
  finally{if(modal)await page.evaluate(y=>window.scrollTo({top:y,behavior:'instant'}),scrollY)}
}
async function preparePeriod(page:Page,title:string,two=true) {
  await page.getByRole('button',{name:'Prepare a period',exact:true}).click()
  await page.getByRole('textbox',{name:'Period title',exact:true}).fill(title)
  await page.getByLabel('Period starts',{exact:true}).fill('2026-01-01')
  await page.getByLabel('Period ends',{exact:true}).fill('2026-01-31')
  await page.getByLabel('Explicit due date',{exact:true}).fill('2026-01-10')
  await page.getByRole('textbox',{name:'Source / approval reference',exact:true}).fill('Fictional approved community register')
  await chooseOption(page,'Participating home 1','Home A-101')
  await page.getByRole('textbox',{name:'Amount for home 1',exact:true}).fill('1000.00')
  if(two){await page.getByRole('button',{name:'Add another home',exact:true}).click();await chooseOption(page,'Participating home 2','Home A-102');await page.getByRole('textbox',{name:'Amount for home 2',exact:true}).fill('750.25')}
  await page.getByRole('button',{name:'Review this period',exact:true}).click()
}
async function confirmDecision(page:Page,button:string) {
  await page.getByRole('textbox',{name:'Decision reason',exact:true}).fill('Separately checked the fictional supplied register and each participating home')
  await page.getByRole('dialog').getByRole('checkbox').check()
  await page.getByRole('button',{name:button,exact:true}).click()
}
async function openStatement(page:Page) {
  await chooseOption(page,'Statement home','Home A-101')
  await page.getByRole('button',{name:'Open statement',exact:true}).click()
  await expect(page.getByRole('dialog').getByRole('heading',{name:'Home A-101',exact:true})).toBeVisible()
  await expect(page.getByRole('dialog').getByText('Updating this statement…',{exact:true})).toHaveCount(0)
}

test('visible period preparation, separate treasury review and receipt allocation preserve the ledger and original receipt',async({page,browser})=>{
  await page.setViewportSize({width:1440,height:1000});await login(page);await ensureMaintenanceReviewer(page);await navigate(page,'Maintenance')
  const title='UI October maintenance '+Date.now();await preparePeriod(page,title)
  await expect(page.getByRole('dialog').getByText('₹1,750.25',{exact:true})).toBeVisible()
  await expect(page.getByRole('button',{name:'Submit for separate review',exact:true})).toBeDisabled()
  await capture(page,'period-preview-desktop')
  await page.getByRole('dialog').getByRole('checkbox').check()
  const submitted=page.waitForResponse(response=>response.url().endsWith('/api/maintenance')&&response.request().method()==='POST')
  await page.getByRole('button',{name:'Submit for separate review',exact:true}).click();const id=(await(await submitted).json()).id
  await expect(page.getByRole('dialog').getByText('A different treasury reviewer must decide this proposal.',{exact:true})).toBeVisible()
  await expect(page.getByRole('button',{name:'Approve & publish',exact:true})).toHaveCount(0)
  await capture(page,'pending-author-desktop')
  const context=await browser.newContext({baseURL:new URL(page.url()).origin});const reviewer=await context.newPage();await login(reviewer,'Committee');await navigate(reviewer,'Maintenance');await reviewer.getByRole('button',{name:'Open period '+title,exact:true}).click()
  await reviewer.getByRole('button',{name:'Approve & publish',exact:true}).click()
  await expect(reviewer.getByRole('button',{name:'Confirm publication',exact:true})).toBeDisabled()
  expect(await reviewer.getByRole('dialog').evaluate(element=>{const dialog=element.getBoundingClientRect(),close=element.querySelector('.dialog-close')!.getBoundingClientRect();return {closeInViewport:close.top>=0&&close.bottom<=innerHeight,dialog:{top:dialog.top,bottom:dialog.bottom,height:dialog.height},close:{top:close.top,bottom:close.bottom},scrollY,viewport:innerHeight}})).toMatchObject({closeInViewport:true});await capture(reviewer,'separate-publication-review')
  await confirmDecision(reviewer,'Confirm publication')
  await expect(reviewer.getByRole('dialog').getByText('MAINTENANCE · PUBLISHED',{exact:true})).toBeVisible()
  await capture(reviewer,'published-period-desktop')
  const data=await(await page.request.get('/api/maintenance/'+id)).json();expect(data.active_paise).toBe(175025);expect(data.allocated_paise).toBe(0);expect(data.outstanding_paise).toBe(175025)
  const ledger=await(await page.request.get('/api/entries?home=demo-flat-A-101&q='+encodeURIComponent('Maintenance · '+title))).json();expect(ledger.items).toHaveLength(1);expect(ledger.items[0].kind).toBe('CHARGE');expect(ledger.items[0].receipt_id).toBe('')
  await page.getByRole('button',{name:'Close period details',exact:true}).click()
  const receipt=await apiReceived(page,'400.00');await openStatement(page)
  const before=await(await page.request.get('/api/statements/demo-flat-A-101')).json()
  await page.getByRole('button',{name:'Allocate credit',exact:true}).click()
  await chooseOption(page,'Allocation credit source',receipt.receipt_number+' · ₹400.00 available')
  await chooseOption(page,'Allocation charge','Maintenance · '+title+' · ₹1,000.00 due')
  await page.getByRole('textbox',{name:'Amount to allocate',exact:true}).fill('400.00')
  await page.getByRole('textbox',{name:'Allocation reason',exact:true}).fill('Checked against the original fictional receipt')
  await expect(page.locator('.allocation-preview').getByText('₹600.00',{exact:true})).toBeVisible()
  await capture(page,'allocation-preview-desktop')
  await page.getByRole('dialog').getByRole('checkbox').check();await page.getByRole('button',{name:'Confirm allocation',exact:true}).click()
  await expect(page.getByRole('status').filter({hasText:'Allocation recorded.'})).toBeVisible()
  const after=await(await page.request.get('/api/statements/demo-flat-A-101')).json();expect(after.allocated_paise-before.allocated_paise).toBe(40000);expect(before.outstanding_paise-after.outstanding_paise).toBe(40000)
  const original=await(await page.request.get('/api/entries/'+receipt.id)).json();expect(original.receipt_id).toBe(receipt.receipt_id);expect(original.amount_paise).toBe(40000);expect(original.receipt_number).toBe(receipt.receipt_number)
  await capture(page,'allocation-recorded-desktop')
  await page.getByRole('button',{name:'Receipts & credit',exact:true}).click();await expect(page.getByRole('region',{name:'Home receipts and credits'}).getByText(receipt.receipt_number,{exact:true})).toBeVisible()
  await page.getByRole('button',{name:'Close home statement',exact:true}).click();await context.close()
})

test('period preparation and opened filters fit desktop tablet and small phones with keyboard focus and scrolling',async({page})=>{
  await login(page);await navigate(page,'Maintenance')
  for(const viewport of [{width:1440,height:900},{width:768,height:1024},{width:375,height:812},{width:320,height:568}]){
    await page.setViewportSize(viewport);await capture(page,'maintenance-'+viewport.width)
    const filter=page.getByRole('combobox',{name:'Filter maintenance by state',exact:true});await filter.click();await page.getByRole('option',{name:'Awaiting review',exact:true}).hover();await capture(page,'state-menu-'+viewport.width);await page.keyboard.press('Escape');await expect(filter).toBeFocused()
    await page.getByRole('button',{name:'Prepare a period',exact:true}).click()
    await page.getByRole('textbox',{name:'Period title',exact:true}).fill('Viewport fictional maintenance')
    await page.getByLabel('Explicit due date',{exact:true}).fill('2026-12-15')
    const home=page.getByRole('combobox',{name:'Participating home 1',exact:true});await home.click();await page.getByRole('option',{name:'Home A-101',exact:true}).hover();await capture(page,'participant-menu-'+viewport.width);await page.getByRole('option',{name:'Home A-101',exact:true}).click()
    await page.getByRole('textbox',{name:'Source / approval reference',exact:true}).fill('Supplied fictional approval source')
    await page.getByRole('textbox',{name:'Amount for home 1',exact:true}).fill('1000.00')
    await page.getByRole('button',{name:'Add another home',exact:true}).click();await chooseOption(page,'Participating home 2','Home A-102');await page.getByRole('textbox',{name:'Amount for home 2',exact:true}).fill('750.25')
    await page.getByRole('button',{name:'Remove participating home 2',exact:true}).click()
    await page.getByRole('textbox',{name:'Amount for home 1',exact:true}).fill('0.00');await page.getByRole('button',{name:'Review this period',exact:true}).click();await expect(page.getByRole('alert')).toContainText('positive amount')
    await page.getByRole('textbox',{name:'Amount for home 1',exact:true}).fill('1000.00');await page.getByRole('button',{name:'Review this period',exact:true}).click()
    await expect(page.locator('[data-review-title]')).toBeFocused();await capture(page,'period-review-'+viewport.width)
    if(viewport.width<=375){await page.getByRole('button',{name:'Edit before submitting',exact:true}).scrollIntoViewIfNeeded();await expect(page.getByRole('button',{name:'Submit for separate review',exact:true})).toBeInViewport({ratio:1});await expect(page.getByRole('button',{name:'Edit before submitting',exact:true})).toBeInViewport({ratio:1});await capture(page,'period-review-actions-'+viewport.width)}
    await page.getByRole('button',{name:'Edit before submitting',exact:true}).click();await expect(page.getByRole('textbox',{name:'Period title',exact:true})).toHaveValue('Viewport fictional maintenance')
    await page.getByRole('dialog').evaluate(element=>{const scroll=element.querySelector('.dialog-scroll')!;scroll.scrollTop=scroll.scrollHeight})
    await expect(page.getByRole('button',{name:'Review this period',exact:true})).toBeInViewport()
    expect(await page.evaluate(()=>({width:innerWidth,actual:document.documentElement.scrollWidth,overflow:[...document.querySelectorAll<HTMLElement>('body *')].filter(el=>el.getBoundingClientRect().right>innerWidth+1).map(el=>({tag:el.tagName,cls:el.className,right:el.getBoundingClientRect().right})).slice(0,15)}))).toEqual({width:viewport.width,actual:viewport.width,overflow:[]})
    expect(await page.getByRole('dialog').evaluate(element=>element.scrollWidth<=element.clientWidth)).toBe(true)
    await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);await expect(page.getByRole('button',{name:'Prepare a period',exact:true})).toBeFocused()
  }
})

test('a lost submission response retries a frozen identity and busy dialogs preserve work',async({page})=>{
  await login(page);await navigate(page,'Maintenance');const title='Lost maintenance submission '+Date.now();await preparePeriod(page,title,false)
  let calls=0,release!:()=>void;const keys:string[]=[]
  await page.route('**/api/maintenance',async route=>{if(route.request().method()!=='POST'){await route.continue();return}calls++;keys.push(route.request().postDataJSON().operation_key);if(calls===1){await new Promise<void>(resolve=>{release=resolve});await route.fetch();await route.abort('failed')}else await route.continue()})
  await page.getByRole('dialog').getByRole('checkbox').check();await page.getByRole('button',{name:'Submit for separate review',exact:true}).click()
  await expect(page.getByRole('button',{name:'Close period preparation',exact:true})).toBeDisabled();await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toBeVisible()
  await expect.poll(()=>typeof release).toBe('function');release();await expect(page.getByRole('button',{name:'Retry submitting period',exact:true})).toBeEnabled();await expect(page.getByRole('button',{name:'Edit before submitting',exact:true})).toBeDisabled();await page.getByRole('button',{name:'Retry submitting period',exact:true}).scrollIntoViewIfNeeded();await capture(page,'lost-submit-locked')
  await page.getByRole('button',{name:'Retry submitting period',exact:true}).click();await expect(page.getByRole('dialog').getByRole('heading',{name:title,exact:true})).toBeVisible();expect(keys).toHaveLength(2);expect(keys[0]).toBe(keys[1])
  const result=await(await page.request.get('/api/maintenance?q='+encodeURIComponent(title))).json();expect(result.total).toBe(1);expect(result.items[0].state).toBe('PENDING');expect(result.totals.active_paise).toBe(0)
})

test('decline withdrawal and stale decisions keep frozen history and do not publish twice',async({page,browser})=>{
  await login(page);await ensureMaintenanceReviewer(page);const title='Decision maintenance '+Date.now(),id=await apiMaintenance(page,title)
  await navigate(page,'Maintenance');await page.getByRole('button',{name:'Open period '+title,exact:true}).click();await page.getByRole('button',{name:'Withdraw proposal',exact:true}).click();await confirmDecision(page,'Confirm withdrawal');await expect(page.getByRole('dialog').getByText('MAINTENANCE · WITHDRAWN',{exact:true})).toBeVisible();await capture(page,'withdrawn-period');await page.getByRole('button',{name:'Close period details',exact:true}).click()
  const title2='Declined maintenance '+Date.now(),id2=await apiMaintenance(page,title2)
  const context=await browser.newContext({baseURL:new URL(page.url()).origin});const reviewer=await context.newPage();await login(reviewer,'Committee');await navigate(reviewer,'Maintenance');await reviewer.getByRole('button',{name:'Open period '+title2,exact:true}).click();await reviewer.getByRole('button',{name:'Decline period',exact:true}).click();await confirmDecision(reviewer,'Confirm decline');await expect(reviewer.getByText('MAINTENANCE · DECLINED',{exact:true})).toBeVisible();await capture(reviewer,'declined-period');expect((await(await page.request.get('/api/maintenance/'+id2)).json()).state).toBe('DECLINED');expect((await(await page.request.get('/api/maintenance/'+id)).json()).state).toBe('WITHDRAWN')
  await reviewer.getByRole('button',{name:'Close period details',exact:true}).click()
  const title3='Stale maintenance '+Date.now(),id3=await apiMaintenance(page,title3);await navigate(page,'Overview');await navigate(page,'Maintenance');await page.getByRole('button',{name:'Open period '+title3,exact:true}).click();await page.getByRole('button',{name:'Withdraw proposal',exact:true}).click()
  await apiPublish(reviewer,id3);await confirmDecision(page,'Confirm withdrawal');await expect(page.getByRole('alert')).toContainText('changed');await expect(page.getByRole('button',{name:'Reload period',exact:true})).toBeVisible();await capture(page,'stale-decision');await page.getByRole('button',{name:'Reload period',exact:true}).click();await expect(page.getByRole('dialog').getByText('MAINTENANCE · PUBLISHED',{exact:true})).toBeVisible();await context.close()
})

test('maintenance source failures and delayed filters hide old totals and retry restores current data',async({page})=>{
  await login(page);await page.route('**/api/maintenance?**',route=>route.fulfill({status:503,contentType:'application/json',body:'{}'}));await navigate(page,'Maintenance')
  await expect(page.getByRole('heading',{name:'Periods unavailable.',exact:true})).toBeVisible();await expect(page.locator('.maintenance-metrics strong')).toHaveText(['—','—','—','—']);await expect(page.getByRole('button',{name:'Prepare a period',exact:true})).toBeDisabled();await page.getByRole('heading',{name:'Periods unavailable.',exact:true}).scrollIntoViewIfNeeded();await capture(page,'period-list-error')
  await page.unroute('**/api/maintenance?**');await page.getByRole('button',{name:'Try again',exact:true}).click();await expect(page.getByRole('button',{name:'Prepare a period',exact:true})).toBeEnabled()
  let release!:()=>void;await page.route('**/api/maintenance?**',async route=>{await new Promise<void>(resolve=>{release=resolve});await route.continue()})
  await page.getByRole('searchbox',{name:'Search maintenance periods',exact:true}).fill('No matching fictional maintenance');await expect(page.locator('.maintenance-metrics strong')).toHaveText(['—','—','—','—']);await capture(page,'period-search-loading');await expect.poll(()=>typeof release).toBe('function');release();await expect(page.getByRole('heading',{name:'No periods match.',exact:true})).toBeVisible();await expect(page.locator('.maintenance-metrics strong')).toHaveText(['₹0.00','₹0.00','₹0.00','₹0.00']);await page.unroute('**/api/maintenance?**')
})

test('home statements and allocation menus fit every viewport and a linked correction preserves its original history',async({page,browser})=>{
  await login(page);await ensureMaintenanceReviewer(page)
  const title='Viewport allocation period '+Date.now(), id=await apiMaintenance(page,title,[{flat_id:'demo-flat-A-101',amount:'1000.00'}])
  const context=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await context.newPage();await login(reviewer,'Committee');await apiPublish(reviewer,id);await context.close()
  const receipt=await apiReceived(page,'75.25'), details=await(await page.request.get('/api/maintenance/'+id)).json()
  const allocation=await page.request.post('/api/allocations',{headers:await financialHeaders(page),data:{operation_key:crypto.randomUUID(),source_id:receipt.id,charge_id:details.lines[0].entry_id,amount:'10.00',reason:'Fictional viewport correction fixture',confirmed:true}});expect(allocation.status()).toBe(200);const allocationId=(await allocation.json()).id
  const available=await apiReceived(page,'75.25');await navigate(page,'Maintenance')
  for(const viewport of [{width:1440,height:900},{width:768,height:1024},{width:375,height:812},{width:320,height:568}]){
    await page.setViewportSize(viewport);await openStatement(page);await page.getByRole('button',{name:'Allocate credit',exact:true}).click()
    const source=page.getByRole('combobox',{name:'Allocation credit source',exact:true});await source.click();await page.getByRole('option',{name:available.receipt_number+' · ₹75.25 available',exact:true}).hover();await capture(page,'credit-menu-'+viewport.width);await page.keyboard.press('Escape');await expect(source).toBeFocused()
    const charge=page.getByRole('combobox',{name:'Allocation charge',exact:true});await charge.click();await capture(page,'charge-menu-'+viewport.width);await page.keyboard.press('Escape')
    await page.getByRole('button',{name:'Cancel',exact:true}).click();await page.getByRole('button',{name:'Receipts & credit',exact:true}).click();await capture(page,'home-credit-'+viewport.width);await page.getByRole('button',{name:'Allocation history',exact:true}).click();await capture(page,'allocation-history-'+viewport.width)
    expect(await page.getByRole('dialog').evaluate(element=>element.scrollWidth<=element.clientWidth)).toBe(true)
    await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);await expect(page.getByRole('button',{name:'Open statement',exact:true})).toBeFocused()
  }
  await page.setViewportSize({width:375,height:812});await openStatement(page);await page.getByRole('button',{name:'Allocation history',exact:true}).click();await page.getByRole('button',{name:'Correct allocation '+allocationId,exact:true}).click();await capture(page,'allocation-correction-phone')
  await page.getByRole('textbox',{name:'Correction reason',exact:true}).fill('Selected the wrong fictional period in this allocation');await page.getByRole('dialog').getByRole('checkbox').check();await page.getByRole('button',{name:'Confirm allocation correction',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'Allocation corrected.'})).toBeVisible();await expect(page.getByText('Corrected',{exact:true})).toBeVisible();await capture(page,'allocation-corrected-phone')
  const corrected=page.locator('.allocation-history > li').filter({has:page.getByText('Corrected',{exact:true})});await corrected.scrollIntoViewIfNeeded();await expect(corrected).toContainText('Selected the wrong fictional period in this allocation');await expect(corrected.getByRole('button')).toHaveCount(0);await capture(page,'allocation-corrected-history-phone')
})

test('residents receive published current-home periods without private proposal notes or financial write controls',async({page,browser})=>{
  await login(page);await ensureMaintenanceReviewer(page)
  const title='Resident maintenance '+Date.now(),id=await apiMaintenance(page,title,[{flat_id:'demo-flat-A-101',amount:'1000.00'},{flat_id:'demo-flat-B-101',amount:'750.25'}])
  const context=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await context.newPage();await login(reviewer,'Committee');await apiPublish(reviewer,id);await context.close();await page.getByRole('button',{name:'Sign out',exact:true}).click()
  await login(page,'Owner');await navigate(page,'Maintenance');await expect(page.getByRole('button',{name:'Prepare a period',exact:true})).toHaveCount(0)
  const periods=await(await page.request.get('/api/maintenance')).json();expect(periods.items.every((item:{state:string;source_reference?:string;note?:string})=>item.state==='PUBLISHED'&&!item.source_reference&&!item.note)).toBe(true)
  await page.getByRole('button',{name:'Open period '+title,exact:true}).click();await expect(page.getByRole('heading',{name:'Review history',exact:true})).toHaveCount(0);await expect(page.getByText('Private source reference',{exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'Approve & publish',exact:true})).toHaveCount(0);await capture(page,'resident-period')
  await page.getByRole('button',{name:'Open statement for A-101',exact:true}).click();await expect(page.getByRole('dialog').getByRole('heading',{name:'Home A-101',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'Allocate credit',exact:true})).toHaveCount(0);await capture(page,'resident-statement')
  await page.getByRole('button',{name:'Close home statement',exact:true}).click();expect((await page.request.get('/api/statements/demo-flat-B-101')).status()).toBe(404)
  await page.getByRole('button',{name:'Sign out',exact:true}).click();await login(page,'Tenant');await expect(page.getByRole('link',{name:'Maintenance',exact:true})).toHaveCount(0);await page.goto('/#maintenance');await expect(page.getByRole('heading',{name:'Maintenance needs financial access.',exact:true})).toBeVisible();expect((await page.request.get('/api/maintenance')).status()).toBe(403)
})

test('publication retries preserve one charge per home and a pending decision cannot close or change its reason',async({page,browser})=>{
  await login(page);await ensureMaintenanceReviewer(page);const title='Lost publication '+Date.now(),id=await apiMaintenance(page,title)
  const context=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await context.newPage();await login(reviewer,'Committee');await navigate(reviewer,'Maintenance');await reviewer.getByRole('button',{name:'Open period '+title,exact:true}).click();await reviewer.getByRole('button',{name:'Approve & publish',exact:true}).click()
  let release!:()=>void;const keys:string[]=[];let calls=0
  await reviewer.route('**/api/maintenance/*/decision',async route=>{keys.push(route.request().postDataJSON().operation_key);calls++;if(calls===1){await new Promise<void>(resolve=>{release=resolve});await route.fetch();await route.abort('failed')}else await route.continue()})
  await confirmDecision(reviewer,'Confirm publication');await expect(reviewer.getByRole('button',{name:'Close period details',exact:true})).toBeDisabled();await reviewer.keyboard.press('Escape');await expect(reviewer.getByRole('dialog')).toBeVisible();await expect.poll(()=>typeof release).toBe('function');release()
  await expect(reviewer.getByRole('button',{name:'Retry this decision',exact:true})).toBeEnabled();await expect(reviewer.getByRole('textbox',{name:'Decision reason',exact:true})).toBeDisabled();await capture(reviewer,'lost-publication-retry')
  await reviewer.getByRole('button',{name:'Retry this decision',exact:true}).click();await expect(reviewer.getByText('MAINTENANCE · PUBLISHED',{exact:true})).toBeVisible();expect(keys).toHaveLength(2);expect(keys[0]).toBe(keys[1])
  const detail=await(await page.request.get('/api/maintenance/'+id)).json();expect(detail.lines).toHaveLength(2);expect(new Set(detail.lines.map((line:{entry_id:string})=>line.entry_id)).size).toBe(2);expect(detail.event_total).toBe(2);expect(detail.active_paise).toBe(175025)
  const entries=await(await page.request.get('/api/entries?q='+encodeURIComponent('Maintenance · '+title))).json();expect(entries.total).toBe(2);expect(entries.items.every((entry:{receipt_id:string})=>entry.receipt_id==='')).toBe(true)
  await context.close()
})

test('a lost allocation response retries one link and a competing writer requires a fresh statement before reallocation',async({page,browser})=>{
  await login(page);await ensureMaintenanceReviewer(page);const title='Retry allocation period '+Date.now(),id=await apiMaintenance(page,title,[{flat_id:'demo-flat-A-101',amount:'1000.00'}])
  const context=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await context.newPage();await login(reviewer,'Committee');await apiPublish(reviewer,id);const details=await(await page.request.get('/api/maintenance/'+id)).json(),chargeId=details.lines[0].entry_id
  const receipt=await apiReceived(page,'500.00');await navigate(page,'Maintenance');await openStatement(page);await page.getByRole('button',{name:'Allocate credit',exact:true}).click();await chooseOption(page,'Allocation credit source',receipt.receipt_number+' · ₹500.00 available');await chooseOption(page,'Allocation charge','Maintenance · '+title+' · ₹1,000.00 due');await page.getByRole('textbox',{name:'Amount to allocate',exact:true}).fill('250.00');await page.getByRole('textbox',{name:'Allocation reason',exact:true}).fill('Confirmed original fictional source and target')
  const keys:string[]=[];let release!:()=>void,calls=0
  await page.route('**/api/allocations',async route=>{keys.push(route.request().postDataJSON().operation_key);calls++;if(calls===1){await new Promise<void>(resolve=>{release=resolve});await route.fetch();await route.abort('failed')}else await route.continue()})
  await page.getByRole('dialog').getByRole('checkbox').check();await page.getByRole('button',{name:'Confirm allocation',exact:true}).click();await expect(page.getByRole('button',{name:'Close home statement',exact:true})).toBeDisabled();await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toBeVisible();await expect.poll(()=>typeof release).toBe('function');release();await expect(page.getByRole('button',{name:'Retry this allocation',exact:true})).toBeEnabled();await expect(page.getByRole('textbox',{name:'Allocation reason',exact:true})).toBeDisabled();await capture(page,'lost-allocation-retry')
  await page.getByRole('button',{name:'Retry this allocation',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'Allocation recorded.'})).toBeVisible();expect(keys).toHaveLength(2);expect(keys[0]).toBe(keys[1]);await page.unroute('**/api/allocations')
  const allocated=await(await page.request.get('/api/maintenance/'+id)).json();expect(allocated.outstanding_paise).toBe(75000);expect(allocated.allocated_paise).toBe(25000)
  const original=await(await page.request.get('/api/entries/'+receipt.id)).json();expect(original.receipt_number).toBe(receipt.receipt_number);expect(original.amount_paise).toBe(50000)
  await page.getByRole('button',{name:'Allocate credit',exact:true}).click();await chooseOption(page,'Allocation credit source',receipt.receipt_number+' · ₹250.00 available');await chooseOption(page,'Allocation charge','Maintenance · '+title+' · ₹750.00 due');await page.getByRole('textbox',{name:'Amount to allocate',exact:true}).fill('250.00');await page.getByRole('textbox',{name:'Allocation reason',exact:true}).fill('UI amount becomes stale after a competing treasury allocation')
  const competitor=await reviewer.request.post('/api/allocations',{headers:await financialHeaders(reviewer),data:{operation_key:crypto.randomUUID(),source_id:receipt.id,charge_id:chargeId,amount:'10.00',reason:'Current concurrent fictional allocation',confirmed:true}});expect(competitor.status()).toBe(200)
  await page.getByRole('dialog').getByRole('checkbox').check();await page.getByRole('button',{name:'Confirm allocation',exact:true}).click();await expect(page.getByRole('alert')).toContainText('changed');await expect(page.getByRole('button',{name:'Reload statement',exact:true})).toBeVisible();await capture(page,'stale-allocation-statement');await page.getByRole('button',{name:'Reload statement',exact:true}).click();await expect(page.getByRole('alert')).toHaveCount(0)
  const current=await(await page.request.get('/api/maintenance/'+id)).json();expect(current.outstanding_paise).toBe(74000);expect(current.allocated_paise).toBe(26000);await context.close()
})

test('bulk supplied amounts freeze all listed homes with bounded participants and maintenance overview links and source retry',async({page,browser})=>{
  await page.setViewportSize({width:1440,height:1000});await login(page);await ensureMaintenanceReviewer(page);await navigate(page,'Maintenance')
  const title='Bulk supplied maintenance '+Date.now();await page.getByRole('button',{name:'Prepare a period',exact:true}).click();await page.getByRole('textbox',{name:'Period title',exact:true}).fill(title);await page.getByLabel('Period starts',{exact:true}).fill('2026-01-01');await page.getByLabel('Period ends',{exact:true}).fill('2026-01-31');await page.getByLabel('Explicit due date',{exact:true}).fill('2026-01-10');await page.getByRole('textbox',{name:'Source / approval reference',exact:true}).fill('Fictional approval for a supplied one rupee per listed home');await page.getByRole('button',{name:'Include all homes',exact:true}).click();await expect(page.getByRole('combobox',{name:/^Participating home /})).toHaveCount(118)
  await page.getByRole('textbox',{name:'Supplied amount for selected homes',exact:true}).fill('1.00');await page.getByRole('button',{name:'Apply supplied amount',exact:true}).click();await page.getByRole('button',{name:'Review this period',exact:true}).click();await expect(page.getByRole('dialog').getByText('₹118.00',{exact:true})).toBeVisible();await page.getByRole('dialog').getByRole('checkbox').check()
  const submitting=page.waitForResponse(response=>response.url().endsWith('/api/maintenance')&&response.request().method()==='POST');await page.getByRole('button',{name:'Submit for separate review',exact:true}).click();const id=(await(await submitting).json()).id
  await expect(page.getByRole('dialog').getByRole('heading',{name:title,exact:true})).toBeVisible();const detail=await(await page.request.get('/api/maintenance/'+id)).json();expect(detail.participants).toBe(118);expect(detail.lines).toHaveLength(20);expect(detail.requested_paise).toBe(11800)
  await page.getByRole('button',{name:'Next participating homes page',exact:true}).click();await expect(page.getByRole('dialog').getByText('Page 2 of 6',{exact:true})).toBeVisible();await capture(page,'bulk-participants-second-page');await page.getByRole('button',{name:'Close period details',exact:true}).click()
  const context=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await context.newPage();await login(reviewer,'Committee');await navigate(reviewer,'Overview');await expect(reviewer.getByRole('button',{name:'Refresh overview'})).toBeEnabled()
  const source=reviewer.getByRole('region',{name:'Maintenance, in view.',exact:true});const queue=await(await reviewer.request.get('/api/overview/maintenance')).json();expect(queue.counts.pending_review).toBeGreaterThanOrEqual(1);await expect(source.locator('.overview-maintenance-amounts > div').nth(2).locator('strong')).toHaveText(String(queue.counts.pending_review))
  await source.getByRole('link',{name:'Open maintenance',exact:true}).click();await reviewer.getByRole('button',{name:'Open period '+title,exact:true}).click();await reviewer.getByRole('button',{name:'Approve & publish',exact:true}).click();await confirmDecision(reviewer,'Confirm publication');await expect(reviewer.getByText('MAINTENANCE · PUBLISHED',{exact:true})).toBeVisible();await reviewer.getByRole('button',{name:'Close period details',exact:true}).click();await navigate(reviewer,'Overview')
  const overview=await(await reviewer.request.get('/api/overview/maintenance')).json();await expect(source.locator('.overview-maintenance-amounts > div').nth(0).locator('strong')).toHaveText(new Intl.NumberFormat('en-IN',{style:'currency',currency:'INR',minimumFractionDigits:2}).format(overview.counts.outstanding_paise/100));await source.scrollIntoViewIfNeeded();await capture(reviewer,'maintenance-overview-populated')
  await reviewer.route('**/api/overview/maintenance',route=>route.fulfill({status:503,contentType:'application/json',body:'{}'}));await reviewer.getByRole('button',{name:'Refresh overview'}).click();await expect(source.locator('strong').filter({hasText:'—'})).toHaveCount(3);await expect(source.getByRole('button',{name:'Retry maintenance',exact:true})).toBeVisible();await source.scrollIntoViewIfNeeded();await capture(reviewer,'maintenance-overview-error');await reviewer.unroute('**/api/overview/maintenance');await source.getByRole('button',{name:'Retry maintenance',exact:true}).click();await expect(source.getByRole('alert')).toHaveCount(0);await context.close()
})
