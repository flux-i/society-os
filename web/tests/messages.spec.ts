import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { mkdirSync, chmodSync } from 'node:fs'
import { resolve } from 'node:path'
import { login,navigate,chooseOption } from './helpers'
import { financialHeaders,ensureMaintenanceReviewer,apiReceived } from './maintenance-fixtures'
import { messageActors,messageSources,messageProposal,messageApprove,messagePost,openMessage } from './messages-fixtures'

test.setTimeout(90000)
const reviewCheck='I reviewed the current content, recipients, permissions and this decision’s effect.'
const previewCheck='I reviewed this exact content, eligible recipients, unique destinations and omissions.'
async function capture(page:Page,name:string) {
  if(!process.env.SOCIETY_CAPTURE_UI)return
  await page.evaluate(()=>document.fonts.ready)
  await page.waitForFunction(()=>{const root=document.querySelector('.messages-page');return !root||Number(getComputedStyle(root).opacity)>=.99})
  const root=resolve(process.env.SOCIETY_MESSAGE_CAPTURE_ROOT??'../reports/local/messages-review');mkdirSync(root,{recursive:true,mode:0o700})
  const path=resolve(root,name+'.png');await page.screenshot({path,animations:'disabled'});chmodSync(path,0o600)
}
async function within(page:Page) {
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
  const dialog=page.getByRole('dialog')
  if(await dialog.count()) {
    await expect(dialog.locator('.dialog-close')).toBeInViewport({ratio:1})
    const geometry=await dialog.evaluate(element=>({width:element.scrollWidth,client:element.clientWidth,gap:element.querySelector('.dialog-scroll')!.getBoundingClientRect().top-element.querySelector('.dialog-close')!.getBoundingClientRect().bottom}))
    expect(geometry.width).toBeLessThanOrEqual(geometry.client);expect(geometry.gap).toBeGreaterThanOrEqual(8)
  }
}
async function actionUI(page:Page,button:string,outcome='') {
  await page.getByRole('button',{name:button,exact:true}).click()
  if(outcome)await chooseOption(page,'Simulation outcome',outcome)
  await page.getByRole('textbox',{name:'Decision reason',exact:true}).fill('PRIVATE_UI_REASON checked the current content, audience and this deliberate decision.')
  await page.getByRole('checkbox',{name:reviewCheck,exact:true}).check()
}
async function proposeUI(page:Page,title:string) {
  await navigate(page,'Messages');await page.getByRole('button',{name:'Prepare a message',exact:true}).click()
  await page.getByRole('textbox',{name:'Search published message sources',exact:true}).fill(title)
  await page.getByRole('button',{name:new RegExp('^'+title)}).click()
  await page.getByRole('textbox',{name:'Proposal reason',exact:true}).fill('PRIVATE_UI_PROPOSAL reviewed the exact content, intended audience and destination choices.')
}

test('communication register empty loading error retry menus typography and four widths are actually rendered',async({page})=>{
  await login(page)
  for(const width of [1440,768,375,320]) {
    await page.setViewportSize({width,height:width<=375?640:1000});await navigate(page,'Messages')
    await expect(page.getByRole('heading',{name:'A conversation starts here.'})).toBeVisible();await capture(page,'register-empty-'+width);await within(page)
    await page.getByRole('combobox',{name:'Filter message state',exact:true}).click();await expect(page.getByRole('option',{name:'All decisions',exact:true})).toHaveAttribute('aria-selected','true')
    await page.getByRole('option',{name:'Awaiting separate review',exact:true}).hover();await capture(page,'state-menu-hover-'+width)
    await page.keyboard.press('End');await expect(page.getByRole('option',{name:'Unsent delivery cancelled',exact:true})).toBeFocused();await page.keyboard.press('Enter');await expect(page.getByRole('combobox',{name:'Filter message state',exact:true})).toContainText('Unsent delivery cancelled')
    await page.getByRole('button',{name:'Clear',exact:true}).click();await chooseOption(page,'Filter message source','Private receipts');await expect(page.locator('.result-count')).toHaveText('0 messages in this view')
    await page.getByRole('button',{name:'Clear',exact:true}).click()
  }
  await page.getByRole('button',{name:'Prepare a message',exact:true}).click()
  const search=page.getByRole('textbox',{name:'Search published message sources',exact:true})
  expect(await search.evaluate(element=>getComputedStyle(element).borderLeftWidth)).toBe('0px')
  await chooseOption(page,'Delivery channel','WhatsApp · simulation');await capture(page,'compose-empty-source-320');await within(page)
  await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);await expect(page.getByRole('button',{name:'Prepare a message',exact:true})).toBeFocused()
  let release!:()=>void,ready!:()=>void;const held=new Promise<void>(resolve=>{release=resolve}),started=new Promise<void>(resolve=>{ready=resolve})
  await page.route('**/api/messages?*',async route=>{ready();await held;await route.fulfill({status:503,contentType:'application/json',body:'{"error":"temporarily_unavailable"}'})})
  const updating=page.getByRole('textbox',{name:'Search message register',exact:true}).fill('missing');await started;await updating
  await expect(page.getByText('Opening current messages…',{exact:true})).toBeVisible();await page.getByText('Opening current messages…',{exact:true}).scrollIntoViewIfNeeded();await capture(page,'list-loading-320');release()
  await expect(page.getByRole('heading',{name:'Messages could not be opened.'})).toBeVisible();await page.getByRole('heading',{name:'Messages could not be opened.'}).scrollIntoViewIfNeeded();await capture(page,'list-error-320')
  await page.unroute('**/api/messages?*');await page.getByRole('button',{name:'Try again',exact:true}).click();await expect(page.getByRole('heading',{name:'No messages match these choices.'})).toBeVisible()
  await page.route('**/api/messages/summary?*',route=>route.fulfill({status:503,contentType:'application/json',body:'{"error":"temporarily_unavailable"}'}))
  await navigate(page,'Overview');await navigate(page,'Messages');await expect(page.getByText('Delivery counts are unavailable. The register remains usable.')).toBeVisible();await expect(page.locator('.message-attention strong')).toHaveText(['—','—','—','—'])
  await page.unroute('**/api/messages/summary?*');await page.getByRole('button',{name:'Retry counts',exact:true}).click();await expect(page.locator('.message-attention strong')).toHaveText(['0','0','0','0'])
  await page.route('**/api/messages/config',route=>route.fulfill({status:503,contentType:'application/json',body:'{"error":"provider_unavailable"}'}));await navigate(page,'Overview');await navigate(page,'Messages')
  await expect(page.getByRole('button',{name:'Prepare a message',exact:true})).toBeDisabled();await expect(page.getByRole('button',{name:'Retry provider status',exact:true})).toBeVisible()
  await page.unroute('**/api/messages/config');await page.getByRole('button',{name:'Retry provider status',exact:true}).click();await expect(page.getByRole('button',{name:'Prepare a message',exact:true})).toBeEnabled()
})

test('all owner tenant wing home and selected-person audiences show independent exact counts and separate approval',async({page,browser})=>{
  const a=await messageActors(page,browser)
  try {
    const title='MESSAGE UI considered water update';await messageSources(page,a.reviewer,title)
    await page.setViewportSize({width:375,height:640});await proposeUI(page,title)
    await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click()
    await expect(page.getByRole('region',{name:'Exact recipient counts'}).locator('strong')).toHaveText(['153','153','3','3','2','150'])
    for(const width of [1440,768,375,320]){await page.setViewportSize({width,height:width<=375?640:1000});await page.getByRole('heading',{name:'The right words. The right people.'}).scrollIntoViewIfNeeded();await capture(page,'recipient-preview-'+width);await page.getByRole('region',{name:'Exact recipient counts'}).scrollIntoViewIfNeeded();await capture(page,'recipient-counts-scrolled-'+width);await within(page)
      const facts=await page.getByRole('region',{name:'Exact recipient counts'}).locator('div').evaluateAll(elements=>elements.map(element=>({row:element.getBoundingClientRect().top,valueTop:element.querySelector('strong')!.getBoundingClientRect().top})))
      for(const fact of facts)for(const peer of facts.filter(x=>Math.abs(x.row-fact.row)<1))expect(Math.abs(peer.valueTop-fact.valueTop),'count values align when labels wrap at '+width+'px').toBeLessThanOrEqual(1)
    }
    for(const [group,counts] of [['Owners',['118','118','2','2','1','116']],['Tenants',['35','35','1','1','1','34']],['One wing',['52','52','3','3','2','49']]] as const){await page.getByRole('button',{name:'Edit proposal',exact:true}).click();await chooseOption(page,'Recipient group',group);await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await expect(page.getByRole('region',{name:'Exact recipient counts'}).locator('strong')).toHaveText([...counts])}
    await page.getByRole('button',{name:'Edit proposal',exact:true}).click();await chooseOption(page,'Recipient group','Selected people')
    await page.getByRole('textbox',{name:'Search message recipients',exact:true}).fill('Demo Owner A-101');await page.getByRole('checkbox',{name:'Demo Owner A-101',exact:true}).check()
    await page.getByRole('textbox',{name:'Search message recipients',exact:true}).fill('Demo Tenant A-103');await page.getByRole('checkbox',{name:'Demo Tenant A-103',exact:true}).check()
    await expect(page.getByRole('button',{name:'Remove Demo Owner A-101',exact:true})).toBeVisible();await capture(page,'selected-people-320')
    await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await expect(page.getByRole('region',{name:'Exact recipient counts'}).locator('strong')).toHaveText(['2','2','2','2','2','0'])
    await page.getByRole('button',{name:'Edit proposal',exact:true}).click();await chooseOption(page,'Recipient group','Selected homes')
    await page.getByRole('textbox',{name:'Search message recipients',exact:true}).fill('A-101');await page.getByRole('checkbox',{name:'A-101',exact:true}).check()
    const check=page.getByRole('checkbox',{name:'A-101',exact:true});expect(await check.evaluate(element=>element.parentElement!.getBoundingClientRect().height)).toBeGreaterThanOrEqual(44)
    await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await expect(page.getByRole('region',{name:'Exact recipient counts'}).locator('strong')).toHaveText(['2','2','2','2','1','0'])
    await expect(page.getByRole('button',{name:'Propose for separate review',exact:true})).toBeDisabled();await page.getByRole('checkbox',{name:previewCheck,exact:true}).check();await page.getByRole('button',{name:'Propose for separate review',exact:true}).click()
    await expect(page.getByRole('dialog').locator('.contact-state').first()).toHaveText('Awaiting separate review');await expect(page.getByRole('button',{name:'Review for approval',exact:true})).toHaveCount(0)
    const id=new URLSearchParams(page.url().split('?')[1]).get('message')!;const pending=await(await page.request.get('/api/messages/summary')).json();expect(pending.outcomes.QUEUED??0).toBe(0)
	await expect(page.getByRole('region',{name:'Reported delivery outcomes',exact:true})).toHaveCount(0);await expect(page.getByRole('region',{name:'Envelope outcomes',exact:true})).toHaveCount(0)
	await expect(page.getByRole('region',{name:'Recipient decisions',exact:true})).not.toContainText('Queued');await expect(page.getByRole('region',{name:'Recipient decisions',exact:true})).toContainText('Eligible when checked')
	await expect(page.locator('.message-card').filter({hasText:'Awaiting separate review'}).locator('.message-card-outcomes')).toBeEmpty()
    await capture(page,'proposer-awaiting-review-320');await openMessage(a.reviewer,id);await a.reviewer.setViewportSize({width:375,height:640});await actionUI(a.reviewer,'Review for approval')
    await capture(a.reviewer,'separate-approval-form-375');await a.reviewer.getByRole('button',{name:'Approve frozen proposal',exact:true}).click();await expect(a.reviewer.getByRole('dialog').locator('.contact-state').first()).toHaveText('Approved for delivery')
    const accepted=await(await page.request.get('/api/messages/'+id)).json();expect(accepted.counts.eligible_people).toBe(2);expect(accepted.counts.destinations).toBe(1);expect(accepted.outcomes.QUEUED).toBe(1)
    const money=await(await page.request.get('/api/entries')).json();expect(money.total).toBe(0)
  }finally{await a.close()}
})

test('unknown provider outcome is reconciled through actual controls without a second handoff and updates overview',async({page,browser})=>{
  const a=await messageActors(page,browser)
  try {
    const source=await messageSources(page,a.reviewer,'MESSAGE UI unknown handoff');const id=await messageProposal(page,source,{kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']});await messageApprove(a.reviewer,id)
    await openMessage(page,id);await page.setViewportSize({width:375,height:640});await actionUI(page,'Run simulated delivery','Lost response · reconcile before retry');await capture(page,'unknown-outcome-form-375')
    await page.getByRole('button',{name:'Run simulated delivery',exact:true}).click();await expect(page.getByRole('button',{name:'Reconcile handoff',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'Run simulated delivery',exact:true})).toHaveCount(0)
    let detail=await(await page.request.get('/api/messages/'+id)).json();expect(detail.outcomes.UNKNOWN).toBe(1);expect(detail.deliveries[0].attempts).toBe(1);expect(detail.deliveries[0].accepted_at).toBe(0);expect(detail.deliveries[0].provider_id).toBe('')
    await page.getByRole('button',{name:'Reconcile handoff',exact:true}).scrollIntoViewIfNeeded();await capture(page,'unknown-reconcile-375')
    await page.getByRole('button',{name:'Close message',exact:true}).click();await navigate(page,'Overview');await expect(page.locator('.overview-messages').getByText('Needs reconciliation',{exact:true})).toBeVisible()
    const attention=await(await page.request.get('/api/overview/messages')).json();expect(attention.counts.UNKNOWN).toBe(1);expect(attention.items.some((x:{id:string})=>x.id===id)).toBe(true)
    await page.getByRole('link',{name:'Open messages',exact:true}).click();await openMessage(page,id);await actionUI(page,'Reconcile handoff');await page.getByRole('button',{name:'Reconcile this handoff',exact:true}).click()
    await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Provider accepted');detail=await(await page.request.get('/api/messages/'+id)).json();expect(detail.outcomes.ACCEPTED).toBe(1);expect(detail.deliveries[0].attempts).toBe(1);expect(detail.deliveries[0].delivered_at).toBe(0);expect(detail.deliveries[0].read_at).toBe(0)
    await page.getByRole('region',{name:'Envelope outcomes'}).scrollIntoViewIfNeeded();await capture(page,'reconciled-accepted-375');await within(page)
  }finally{await a.close()}
})

test('a lost proposal response locks dismissal edits and resubmits the same operation without another proposal',async({page,browser})=>{
  const a=await messageActors(page,browser)
  try {
    const title='MESSAGE UI lost proposal response';await messageSources(page,a.reviewer,title);await page.setViewportSize({width:320,height:480});await proposeUI(page,title)
    await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await page.getByRole('checkbox',{name:previewCheck,exact:true}).check()
    const bodies:string[]=[];let lost=true
    await page.route('**/api/messages',async route=>{if(route.request().method()!=='POST'){await route.continue();return}bodies.push(route.request().postData()!);const response=await route.fetch();if(lost){lost=false;await route.abort('failed')}else await route.fulfill({response})})
    await page.getByRole('button',{name:'Propose for separate review',exact:true}).click();await expect(page.getByRole('button',{name:'Retry this proposal',exact:true})).toBeEnabled();await expect(page.getByRole('button',{name:'Close message proposal',exact:true})).toBeDisabled();await expect(page.getByRole('button',{name:'Edit proposal',exact:true})).toBeDisabled()
    await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toBeVisible();await capture(page,'unknown-client-write-320-short')
    await page.getByRole('button',{name:'Retry this proposal',exact:true}).click();await expect(page.getByRole('dialog').locator('.contact-state').first()).toHaveText('Awaiting separate review');expect(bodies).toHaveLength(2);expect(bodies[0]).toBe(bodies[1]);await page.unroute('**/api/messages')
    const matches=await(await page.request.get('/api/messages?q='+encodeURIComponent(title))).json();expect(matches.total).toBe(1)
  }finally{await a.close()}
})

test('stale review preserves the human reason clears attestation and requires a refreshed separately approved snapshot',async({page,browser})=>{
  const a=await messageActors(page,browser)
  try {
    const source=await messageSources(page,a.reviewer,'MESSAGE UI stale audience');const id=await messageProposal(page,source,{kind:'OWNERS',wing:'',ids:[]});await openMessage(a.reviewer,id);await a.reviewer.setViewportSize({width:375,height:480});await actionUI(a.reviewer,'Review for approval')
    const reason=await a.reviewer.getByRole('textbox',{name:'Decision reason',exact:true}).inputValue()
    const contact=await(await page.request.get('/api/contacts/demo-owner-A-101')).json();await messagePost(page,'/api/contacts/demo-owner-A-101/actions',{version:contact.version,action:'OPTED_OUT',channel:'EMAIL',purpose:'COMMUNITY',reason:'Please stop only these selected fictional community messages.'})
    await a.reviewer.getByRole('button',{name:'Approve frozen proposal',exact:true}).click();await expect(a.reviewer.getByRole('button',{name:'Reload current record',exact:true})).toBeVisible();await a.reviewer.getByRole('button',{name:'Reload current record',exact:true}).click()
    await expect(a.reviewer.getByRole('textbox',{name:'Decision reason',exact:true})).toHaveValue(reason);await expect(a.reviewer.getByRole('checkbox',{name:reviewCheck,exact:true})).not.toBeChecked();await expect(a.reviewer.getByRole('button',{name:'Approve frozen proposal',exact:true})).toBeDisabled();await capture(a.reviewer,'stale-reviewed-reload-375-short')
    await openMessage(page,id);await actionUI(page,'Refresh recipient snapshot');await page.getByRole('button',{name:'Refresh recipient snapshot',exact:true}).click()
    const refreshed=await(await page.request.get('/api/messages/'+id)).json();expect(refreshed.snapshot_version).toBe(2);expect(refreshed.counts.eligible_people).toBe(1);expect(refreshed.counts.destinations).toBe(1);expect(refreshed.counts.reasons.OPTED_OUT).toBe(1)
    await a.reviewer.getByRole('button',{name:'Back to message',exact:true}).click();await a.reviewer.reload();await actionUI(a.reviewer,'Review for approval');await a.reviewer.getByRole('button',{name:'Approve frozen proposal',exact:true}).click();await expect(a.reviewer.getByRole('dialog').locator('.contact-state').first()).toHaveText('Approved for delivery')
    await capture(a.reviewer,'refreshed-approval-375-short');await within(a.reviewer)
  }finally{await a.close()}
})

test('definite failures stop at three attempts explicit read reports stay distinct and unsent cancellation keeps history',async({page,browser})=>{
  const a=await messageActors(page,browser)
  try {
    const source=await messageSources(page,a.reviewer,'MESSAGE UI bounded failure');const target={kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']};const id=await messageProposal(page,source,target);await messageApprove(a.reviewer,id);await openMessage(page,id)
    for(let i=1;i<=3;i++){await actionUI(page,'Run simulated delivery','Definite rejection · retry allowed');await page.getByRole('button',{name:'Run simulated delivery',exact:true}).click();await expect(page.locator('.message-detail .form-success')).toContainText('Simulation recorded.');await expect(page.getByRole('region',{name:'Envelope outcomes'})).toBeVisible();await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Definitively failed');const detail=await(await page.request.get('/api/messages/'+id)).json();expect(detail.deliveries[0].attempts).toBe(i)}
    await expect(page.getByRole('button',{name:'Run simulated delivery',exact:true})).toHaveCount(0);await page.getByRole('region',{name:'Envelope outcomes'}).scrollIntoViewIfNeeded();await capture(page,'failure-three-attempts-1440')
    await actionUI(page,'Cancel unsent messages');await page.getByRole('button',{name:'Cancel unsent messages',exact:true}).click();await expect(page.getByRole('dialog').locator('.contact-state').first()).toHaveText('Unsent delivery cancelled')
    const read=await messageProposal(page,source,target);await messageApprove(a.reviewer,read);await openMessage(page,read);await actionUI(page,'Run simulated delivery','Explicit delivery and read reports');await page.getByRole('button',{name:'Run simulated delivery',exact:true}).click()
    await expect(page.locator('.message-detail .form-success')).toContainText('Simulation recorded.');await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Read reported')
    const detail=await(await page.request.get('/api/messages/'+read)).json();expect(detail.outcomes.READ).toBe(1);expect(detail.deliveries[0].delivered_at).toBeGreaterThan(0);expect(detail.deliveries[0].read_at).toBeGreaterThan(0);await page.getByRole('region',{name:'Envelope outcomes'}).scrollIntoViewIfNeeded();await capture(page,'explicit-read-proof-1440')
    const declined=await messageProposal(page,source,target);await openMessage(a.reviewer,declined);await actionUI(a.reviewer,'Decline proposal');await a.reviewer.getByRole('button',{name:'Decline proposal',exact:true}).click();await expect(a.reviewer.getByRole('dialog').locator('.contact-state').first()).toHaveText('Declined')
    const withdrawn=await messageProposal(page,source,target);await openMessage(page,withdrawn);await actionUI(page,'Withdraw proposal');await page.getByRole('button',{name:'Withdraw proposal',exact:true}).click();await expect(page.getByRole('dialog').locator('.contact-state').first()).toHaveText('Withdrawn')
  }finally{await a.close()}
})

test('receipt composition uses only a generic private link exact source audience and independent treasury sharing',async({page,browser})=>{
  const a=await messageActors(page,browser)
  try {
    await messageSources(page,a.reviewer,'MESSAGE UI private receipt source');await ensureMaintenanceReviewer(page);await login(a.reviewer,'Committee');const received=await apiReceived(page,'432.19')
    await page.setViewportSize({width:375,height:640});await navigate(page,'Messages');await page.getByRole('button',{name:'Prepare a message',exact:true}).click();await chooseOption(page,'Message source','Original received-money receipt')
    await page.getByRole('textbox',{name:'Search published message sources',exact:true}).fill(received.receipt_number);await page.getByRole('button',{name:new RegExp(received.receipt_number)}).click();await page.getByRole('textbox',{name:'Proposal reason',exact:true}).fill('Reviewed this original receipt and deliberate private authenticated sharing.')
    await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await expect(page.getByRole('region',{name:'Exact recipient counts'}).locator('strong')).toHaveText(['153','2','2','2','1','151'])
    const wording=await page.getByRole('region',{name:'Exact message preview'}).innerText();expect(wording).toContain('A receipt is available in your society portal.');expect(wording).not.toContain('432');expect(wording).not.toContain('A-101');expect(wording).not.toContain('Demo Owner');await page.getByRole('region',{name:'Exact message preview'}).scrollIntoViewIfNeeded();await capture(page,'private-receipt-preview-375')
    await page.getByRole('checkbox',{name:previewCheck,exact:true}).check();await page.getByRole('button',{name:'Propose for separate review',exact:true}).click();await expect(page.getByRole('dialog').locator('.contact-state').first()).toHaveText('Awaiting separate review');const id=new URLSearchParams(page.url().split('?')[1]).get('message')!;await messageApprove(a.reviewer,id)
    const ownerContext=await browser.newContext({baseURL:new URL(page.url()).origin,viewport:{width:1440,height:1000}});try {const owner=await ownerContext.newPage();await login(owner,'Owner');await openMessage(owner,id);await expect(owner.getByRole('button',{name:'Run simulated delivery',exact:true})).toHaveCount(0);await expect(owner.getByRole('region',{name:'Recipient decisions'}).locator('li')).toHaveCount(1);await capture(owner,'own-receipt-message-1440');const forbidden=await owner.request.post('/api/messages/preview',{headers:await financialHeaders(owner),data:{source_kind:'RECEIPT',source_id:received.receipt_id,channel:'EMAIL',target:{kind:'ALL',wing:'',ids:[]}}});expect(forbidden.status()).toBe(403)}finally{await ownerContext.close()}
    const unchanged=await(await page.request.get('/api/entries/'+received.id)).json();expect(unchanged.amount_paise).toBe(43219);expect(unchanged.receipt_id).toBe(received.receipt_id)
  }finally{await a.close()}
})

test('source recipient register and immutable activity pagination retain choices and expose bounded current records',async({page,browser})=>{
  const a=await messageActors(page,browser)
  try {
    const prefix='MESSAGE UI pagination';const first=await messageSources(page,a.reviewer,prefix+' 00');const sources=[first]
    for(let i=1;i<13;i++) {
      const notice=await messagePost(page,'/api/reviews',{kind:'NOTICE',title:prefix+' '+String(i).padStart(2,'0'),body:'An independently approved fictional notice for bounded source selection.',audience:'ALL_RESIDENTS',building_code:''})
      await messagePost(a.reviewer,'/api/reviews/'+notice.id+'/decision',{version:1,decision:'APPROVED',reason:'Reviewed the immutable fictional source and intended resident audience.'});sources.push(notice.id)
    }
    await page.setViewportSize({width:320,height:480});await navigate(page,'Messages');await page.getByRole('button',{name:'Prepare a message',exact:true}).click();await page.getByRole('textbox',{name:'Search published message sources',exact:true}).fill(prefix)
    const picker=page.getByRole('region',{name:'Published sources'});await expect(picker.locator('.message-source-option')).toHaveCount(12)
    await page.getByRole('button',{name:'Next message sources page',exact:true}).click();await expect(picker.locator('.message-source-option')).toHaveCount(1);await expect(page.getByRole('button',{name:'Next message sources page',exact:true})).toBeDisabled();await picker.locator('.message-source-option').click()
    const sourcePageTwo=await(await page.request.get('/api/messages/sources?'+new URLSearchParams({kind:'NOTICE',q:prefix,page:'2'}))).json();expect(sourcePageTwo.items).toHaveLength(1)
    const targetPageTwo=await(await page.request.get('/api/messages/targets?'+new URLSearchParams({source_kind:'NOTICE',source_id:sourcePageTwo.items[0].id,kind:'PEOPLE',page:'2'}))).json();expect(targetPageTwo.items).toHaveLength(12)
    await chooseOption(page,'Recipient group','Selected people');const choices=page.getByRole('region',{name:'Select message recipients'});await expect(choices.getByRole('checkbox')).toHaveCount(12);await choices.getByRole('checkbox').first().check()
    await page.getByRole('button',{name:'Next recipient choices page',exact:true}).click();await expect(choices.getByText('Page 2 of 13',{exact:true})).toBeVisible();await choices.getByRole('checkbox',{name:targetPageTwo.items[0].label,exact:true}).check();await expect(page.locator('.message-selected-people')).toContainText('2 selected')
    await page.getByRole('button',{name:'Previous recipient choices page',exact:true}).click();await expect(choices.getByRole('checkbox',{name:'Demo Joint Owner A-101',exact:true})).toBeChecked();await page.getByRole('button',{name:'Remove Demo Joint Owner A-101',exact:true}).click();await expect(choices.getByRole('checkbox',{name:'Demo Joint Owner A-101',exact:true})).not.toBeChecked();await expect(page.locator('.message-selected-people')).toContainText('1 selected')
    await chooseOption(page,'Recipient group','All current people');await page.getByRole('textbox',{name:'Proposal reason',exact:true}).fill('Reviewed paginated current sources, selections and the exact recipient intersection.');await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click()
    const people=page.getByRole('region',{name:'Recipient decisions'});await expect(people.locator('li')).toHaveCount(20);const pageOne=await people.locator('li strong').allTextContents();await page.getByRole('button',{name:'Next message recipients page',exact:true}).click();await expect(people.getByText('Page 2 of 8',{exact:true})).toBeVisible();await expect(people.locator('li strong').first()).not.toHaveText(pageOne[0]);expect(await people.locator('li strong').allTextContents()).not.toEqual(pageOne)
    await page.getByRole('button',{name:'Previous message recipients page',exact:true}).click();await expect(people.getByText('Page 1 of 8',{exact:true})).toBeVisible();expect(await people.locator('li strong').allTextContents()).toEqual(pageOne);await capture(page,'paginated-recipient-review-320-short');await within(page);await page.getByRole('button',{name:'Close message proposal',exact:true}).click()
    const ids=[];for(const source of sources)ids.push(await messageProposal(page,source,{kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']}))
    await navigate(page,'Overview');await navigate(page,'Messages');await page.getByRole('textbox',{name:'Search message register',exact:true}).fill(prefix);await expect(page.locator('.message-card')).toHaveCount(12);await page.getByRole('button',{name:'Next messages page',exact:true}).click();await expect(page.locator('.message-card')).toHaveCount(1);await expect(page.getByRole('button',{name:'Next messages page',exact:true})).toBeDisabled();await page.locator('.message-card').click();await expect(page.getByRole('dialog').locator('.contact-state').first()).toHaveText('Awaiting separate review');await page.getByRole('button',{name:'Close message',exact:true}).click()
    const id=ids[0];for(let i=0;i<21;i++){const current=await(await page.request.get('/api/messages/'+id)).json();await messagePost(page,'/api/messages/'+id+'/actions',{version:current.version,action:'REFRESH',reason:'Deliberately refresh this current fictional recipient snapshot, keeping each earlier decision.'})}
    await openMessage(page,id);const history=page.getByRole('region',{name:'Message activity'});await expect(history.locator('ol > li')).toHaveCount(20);await page.getByRole('button',{name:'Next message activity page',exact:true}).click();await expect(history.locator('ol > li')).toHaveCount(2);await expect(history.getByText('Page 2 of 2',{exact:true})).toBeVisible();await history.locator('summary').last().click();await expect(history.getByRole('region',{name:'Exact recipient counts'})).toBeVisible();await history.locator('summary').last().scrollIntoViewIfNeeded();await capture(page,'frozen-history-page-two-320-short');await within(page)
  }finally{await a.close()}
})
