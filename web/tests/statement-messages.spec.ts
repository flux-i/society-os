import { test,expect } from '@playwright/test'
import type { Page,Browser } from '@playwright/test'
import { mkdirSync,chmodSync } from 'node:fs'
import { resolve } from 'node:path'
import { login,navigate,chooseOption } from './helpers'
import { financialHeaders } from './maintenance-fixtures'
import { messageSources,messageProposal,messageApprove,messagePost,openMessage,messagePrivateFixture } from './messages-fixtures'
import { statementActors,statementOriginal,statementApprove,statementPublish,originalCSV } from './statements-fixtures'

test.setTimeout(90000)
const previewCheck='I reviewed this exact content, eligible recipients, unique destinations and omissions.'
const reviewCheck='I reviewed the current content, recipients, permissions and this decision’s effect.'
async function capture(page:Page,name:string) {
 if(!process.env.SOCIETY_CAPTURE_UI)return
 await page.evaluate(()=>document.fonts.ready)
 await page.waitForFunction(()=>{const root=document.querySelector('.messages-page');return !root||root.getAnimations().every(a=>['finished','idle'].includes(a.playState))})
 await page.evaluate(()=>new Promise<void>(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve()))))
 const root=resolve(process.env.SOCIETY_STATEMENT_MESSAGE_CAPTURE_ROOT??'../reports/local/statement-messages-review');mkdirSync(root,{recursive:true,mode:0o700});const file=resolve(root,name+'.png');await page.screenshot({path:file,animations:'disabled'});chmodSync(file,0o600)
}
async function within(page:Page) {
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 const dialog=page.getByRole('dialog');if(await dialog.count()) {await expect(dialog.locator('.dialog-close')).toBeInViewport({ratio:1});expect(await dialog.evaluate(e=>e.scrollWidth<=e.clientWidth)).toBe(true)}
}
async function fixture(page:Page,browser:Browser,title:string,audience='TENANTS') {
 const a=await statementActors(page,browser);await messageSources(page,a.reviewer,title+' contact fixture')
 const file=await statementOriginal(page,title);await statementApprove(a.reviewer,file);const pub=await statementPublish(page,a.reviewer,file,audience)
 return {...a,file,pub}
}
async function compose(page:Page,title:string) {
 await navigate(page,'Messages');await page.getByRole('button',{name:'Prepare a message',exact:true}).click();await chooseOption(page,'Message source','Published financial statement')
 await page.getByRole('textbox',{name:'Search published message sources',exact:true}).fill(title)
 await page.getByRole('button',{name:new RegExp('^'+title)}).click();await page.getByRole('textbox',{name:'Proposal reason',exact:true}).fill('PRIVATE_STMSG reviewed this exact published original, recipients and finance permission.')
}
async function decision(page:Page,button:string,outcome='') {
 await page.getByRole('button',{name:button,exact:true}).click();if(outcome)await chooseOption(page,'Simulation outcome',outcome)
 await page.getByRole('textbox',{name:'Decision reason',exact:true}).fill('PRIVATE_STMSG independently checked the exact original, audience and current finance authority.')
 await page.getByRole('checkbox',{name:reviewCheck,exact:true}).check()
}
async function refreshOverview(page:Page) {
 const button=page.getByRole('button',{name:'Refresh overview',exact:true});await expect(button).toBeEnabled()
 const response=page.waitForResponse(r=>r.request().method()==='GET'&&new URL(r.url()).pathname==='/api/overview/messages')
 await button.click();expect((await response).status()).toBe(200);await expect(button).toBeEnabled()
}

test('statement source menus empty source errors retries keyboard and four viewports are rendered',async({page})=>{
 await login(page)
 for(const width of [1440,768,375,320]) {
  await page.setViewportSize({width,height:width<=375?640:1000});await navigate(page,'Messages');await page.evaluate(()=>scrollTo({top:0,behavior:'instant'}))
  await page.getByRole('combobox',{name:'Filter message source',exact:true}).click();await expect(page.getByRole('option',{name:'All sources',exact:true})).toHaveAttribute('aria-selected','true');await page.getByRole('option',{name:'Financial statements',exact:true}).hover();await capture(page,'source-filter-open-'+width)
  await expect(page.getByRole('option',{name:'Financial statements',exact:true})).toBeFocused();await page.keyboard.press('Home');await expect(page.getByRole('option',{name:'All sources',exact:true})).toBeFocused();for(const label of ['Community notices','Private receipts','Financial statements']){await page.keyboard.press('ArrowDown');await expect(page.getByRole('option',{name:label,exact:true})).toBeFocused()}await page.keyboard.press('Enter');await expect(page.locator('.result-count')).toHaveText('0 messages in this view')
  if(width===1440) {const tops=await page.locator('.message-filters').locator('.search-control,button[role=combobox]').evaluateAll(xs=>xs.map(x=>x.getBoundingClientRect().top));expect(tops).toHaveLength(3);expect(Math.max(...tops)-Math.min(...tops)).toBeLessThanOrEqual(1)}
  await page.getByRole('button',{name:'Prepare a message',exact:true}).click();await page.getByRole('combobox',{name:'Message source',exact:true}).click();await page.getByRole('option',{name:'Published financial statement',exact:true}).hover();await capture(page,'source-menu-open-'+width)
  await expect(page.getByRole('option',{name:'Published financial statement',exact:true})).toBeFocused();await page.keyboard.press('Home');await expect(page.getByRole('option',{name:'Published community notice',exact:true})).toBeFocused();for(const label of ['Requested meeting acknowledgement','Original received-money receipt','Published financial statement']){await page.keyboard.press('ArrowDown');await expect(page.getByRole('option',{name:label,exact:true})).toBeFocused()}await page.keyboard.press('Enter');await expect(page.getByRole('heading',{name:'Share a financial statement.',exact:true})).toBeVisible();await expect(page.getByText('An approved original must be deliberately published in Financial statements.')).toBeVisible();await expect(page.getByRole('button',{name:'Preview exact recipients',exact:true})).toBeDisabled();await capture(page,'empty-source-'+width);await within(page)
  await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);await expect(page.getByRole('button',{name:'Prepare a message',exact:true})).toBeFocused()
  await page.getByRole('combobox',{name:'Filter message source',exact:true}).click();await expect(page.getByRole('option',{name:'Financial statements',exact:true})).toHaveAttribute('aria-selected','true');await page.keyboard.press('Escape');await page.getByRole('button',{name:'Clear',exact:true}).click()
 }
 await page.route('**/api/messages/sources?*',route=>route.fulfill({status:503,contentType:'application/json',body:'{"error":"temporarily_unavailable"}'}))
 await page.getByRole('button',{name:'Prepare a message',exact:true}).click();await chooseOption(page,'Message source','Published financial statement');await expect(page.getByRole('button',{name:'Try again',exact:true})).toBeVisible();await capture(page,'source-error-320')
 await page.unroute('**/api/messages/sources?*');await page.getByRole('button',{name:'Try again',exact:true}).click();await expect(page.getByText('An approved original must be deliberately published in Financial statements.')).toBeVisible();await page.keyboard.press('Escape')
})

test('a tenant statement is separately approved delivered through actual controls and links to the unchanged original',async({page,browser})=>{
 const title='STMSG UI exact tenant statement',a=await fixture(page,browser,title),tenantContext=await browser.newContext({baseURL:new URL(page.url()).origin}),tenant=await tenantContext.newPage()
 try {
  await compose(page,title);await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await expect(page.getByRole('region',{name:'Exact recipient counts'}).locator('strong')).toHaveText(['153','35','1','1','1','152'])
  await expect(page.getByRole('region',{name:'Exact message preview'})).not.toContainText(title);await expect(page.getByRole('region',{name:'Exact message preview'})).not.toContainText('432.19')
  for(const width of [1440,768,375,320]) {await page.setViewportSize({width,height:width<=375?640:1000});await page.getByRole('region',{name:'Exact recipient counts'}).scrollIntoViewIfNeeded();await capture(page,'tenant-preview-'+width);await within(page)}
  await page.getByRole('checkbox',{name:previewCheck,exact:true}).check();await page.getByRole('button',{name:'Propose for separate review',exact:true}).click();await expect(page.getByRole('dialog').locator('.contact-state').first()).toHaveText('Awaiting separate review');await expect(page.getByRole('button',{name:'Review for approval',exact:true})).toHaveCount(0)
  const id=new URLSearchParams(page.url().split('?')[1]).get('message')!;await page.keyboard.press('Escape');await navigate(page,'Overview');await refreshOverview(page);const pending=page.getByRole('link',{name:'Message awaiting separate review: Financial statement message',exact:true});await expect(pending).toBeVisible();await expect(page.locator('.overview-attention .overview-count')).toHaveText('1 item');await expect(page.locator('.overview-attention-foot')).toContainText('Showing 1 of 1 item');await pending.scrollIntoViewIfNeeded();await capture(page,'pending-message-attention-320');await pending.click();await expect(page).toHaveURL(new RegExp('message='+id))
  await openMessage(a.reviewer,id);await a.reviewer.setViewportSize({width:375,height:640});await decision(a.reviewer,'Review for approval');await capture(a.reviewer,'separate-treasury-review-375');await a.reviewer.getByRole('button',{name:'Approve frozen proposal',exact:true}).click()
  await openMessage(page,id);await decision(page,'Run simulated delivery','Lost response · reconcile before retry');await page.getByRole('button',{name:'Run simulated delivery',exact:true}).click();await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Needs reconciliation');await capture(page,'unknown-handoff-320')
  await page.keyboard.press('Escape');await navigate(page,'Overview');await refreshOverview(page);const uncertain=page.getByRole('link',{name:'Inspect delivery outcomes: Financial statement message',exact:true});await expect(uncertain).toBeVisible();await expect(page.locator('.overview-attention .overview-count')).toHaveText('1 item');await uncertain.scrollIntoViewIfNeeded();await capture(page,'uncertain-message-attention-320');await uncertain.click();await expect(page).toHaveURL(new RegExp('message='+id))
  await decision(page,'Reconcile handoff');await page.getByRole('button',{name:'Reconcile this handoff',exact:true}).click();await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Provider accepted');await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).not.toContainText('Delivery reported');await capture(page,'reconciled-original-handoff-320')
  const current=await(await page.request.get('/api/messages/'+id)).json();expect(current.purpose).toBe('FINANCE');expect(current.source.id).toBe(a.pub);expect(current.source.link).toBe('/#statements?statement='+a.file);expect(current.deliveries[0].attempts).toBe(1)
  await login(tenant,'Tenant');await tenant.setViewportSize({width:320,height:640});await openMessage(tenant,id);await expect(tenant.getByRole('link',{name:'Open shared statement',exact:true})).toBeVisible();await expect(tenant.getByRole('button',{name:'Run simulated delivery',exact:true})).toHaveCount(0);await capture(tenant,'tenant-own-message-320')
  await tenant.getByRole('link',{name:'Open shared statement',exact:true}).click();await expect(tenant).toHaveURL(new RegExp('statement='+a.file));await expect(tenant.getByRole('heading',{name:title,exact:true}).first()).toBeVisible();await capture(tenant,'tenant-exact-original-320')
  const download=await tenant.request.get('/api/financial-statements/'+a.file+'/download');expect(download.status()).toBe(200);expect(await download.body()).toEqual(originalCSV)
  expect((await tenant.request.get('/api/statements/demo-flat-A-103')).status()).toBe(403)
 }finally {await tenantContext.close();await a.close()}
})

test('revoked and superseded statement sources cannot substitute another original or a delivery',async({page,browser})=>{
 const title='STMSG UI withdrawn publication',a=await fixture(page,browser,title),tenantContext=await browser.newContext({baseURL:new URL(page.url()).origin}),tenant=await tenantContext.newPage()
 try {
  const pending=await messageProposal(page,a.pub,{kind:'TENANTS',wing:'',ids:[]},'STATEMENT'),approved=await messageProposal(page,a.pub,{kind:'TENANTS',wing:'',ids:[]},'STATEMENT');await messageApprove(a.reviewer,approved)
  const detail=await(await page.request.get('/api/financial-statements/'+a.file)).json();await messagePost(page,'/api/financial-statements/publications/'+a.pub+'/actions',{version:detail.publications.find((p:{id:string})=>p.id===a.pub).version,action:'REVOKED',reason:'Deliberately removed new access to this exact original.'})
  await openMessage(a.reviewer,pending);await expect(a.reviewer.getByRole('button',{name:'Review for approval',exact:true})).toHaveCount(0);await expect(a.reviewer.getByRole('alert')).toContainText('Source no longer available');await capture(a.reviewer,'revoked-before-review-1440')
  await openMessage(page,approved);await decision(page,'Run simulated delivery');await page.getByRole('button',{name:'Run simulated delivery',exact:true}).click();await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Skipped');await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).not.toContainText('Provider accepted');await capture(page,'revoked-queue-skipped-1440')
  await login(tenant,'Tenant');await openMessage(tenant,approved);await expect(tenant.getByRole('link',{name:'Open shared statement',exact:true})).toHaveCount(0);expect((await tenant.request.get('/api/financial-statements/'+a.file+'/download')).status()).toBe(404)
  const replacement=await statementOriginal(page,title+' replacement',originalCSV,'replacement.csv',{id:a.file,version:detail.version});await statementApprove(a.reviewer,replacement);await statementPublish(page,a.reviewer,replacement)
  await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);await compose(page,title+' replacement');await expect(page.getByText('Selected:')).toContainText('replacement');await capture(page,'replacement-original-choice-1440');await page.keyboard.press('Escape')
  const old=await page.request.post('/api/messages/preview',{headers:await financialHeaders(page),data:{source_kind:'STATEMENT',source_id:a.pub,channel:'EMAIL',target:{kind:'ALL',wing:'',ids:[]}}});expect(old.status()).toBe(404)
 }finally {await tenantContext.close();await a.close()}
})

test('statement source paging selected audiences and short phone controls preserve exact intersections',async({page,browser})=>{
 const a=await statementActors(page,browser),prefix='STMSG UI bounded sources'
 try {
  await messageSources(page,a.reviewer,prefix+' contact fixture')
  for(let i=0;i<13;i++){const id=await statementOriginal(page,prefix+' '+String(i).padStart(2,'0'));await statementApprove(a.reviewer,id);await statementPublish(page,a.reviewer,id,'ALL')}
  await page.setViewportSize({width:320,height:440});await navigate(page,'Messages');await page.getByRole('button',{name:'Prepare a message',exact:true}).click();await chooseOption(page,'Message source','Published financial statement');await page.getByRole('textbox',{name:'Search published message sources',exact:true}).fill(prefix);await expect(page.locator('.message-source-option')).toHaveCount(12)
  await page.getByRole('button',{name:'Next message sources page',exact:true}).click();await expect(page.locator('.message-source-option')).toHaveCount(1);await page.locator('.message-source-option').click();await chooseOption(page,'Recipient group','Selected people');await page.getByRole('textbox',{name:'Search message recipients',exact:true}).fill('Demo Owner A-101');await page.getByRole('checkbox',{name:'Demo Owner A-101',exact:true}).check();await page.getByRole('textbox',{name:'Search message recipients',exact:true}).fill('Demo Tenant A-103');await page.getByRole('checkbox',{name:'Demo Tenant A-103',exact:true}).check();await capture(page,'selected-statement-people-short-320')
  await page.getByRole('button',{name:'Remove Demo Owner A-101',exact:true}).click();await page.getByRole('textbox',{name:'Proposal reason',exact:true}).fill('Reviewed the exact one-person current publication and finance permission.');await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await expect(page.getByRole('region',{name:'Exact recipient counts'}).locator('strong')).toHaveText(['1','1','1','1','1','0'])
  await page.getByRole('checkbox',{name:previewCheck,exact:true}).check();await page.getByRole('button',{name:'Propose for separate review',exact:true}).scrollIntoViewIfNeeded();await expect(page.getByRole('button',{name:'Propose for separate review',exact:true})).toBeInViewport({ratio:1});await capture(page,'statement-preview-actions-short-320');await within(page)
  for(const [group,counts] of [['All current people',['153','153','3','3','2','150']],['Owners',['118','118','2','2','1','116']],['Tenants',['35','35','1','1','1','34']]] as const){await page.getByRole('button',{name:'Edit proposal',exact:true}).click();await chooseOption(page,'Recipient group',group);await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await expect(page.getByRole('region',{name:'Exact recipient counts'}).locator('strong')).toHaveText([...counts]);await expect(page.getByRole('checkbox',{name:previewCheck,exact:true})).not.toBeChecked()}
  await page.getByRole('button',{name:'Edit proposal',exact:true}).click();await chooseOption(page,'Recipient group','One wing');await chooseOption(page,'Recipient wing','Wing C');await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await expect(page.getByRole('region',{name:'Exact recipient counts'}).locator('strong')).toHaveText(['49','49','0','0','0','49']);await expect(page.getByRole('button',{name:'Propose for separate review',exact:true})).toBeDisabled();await capture(page,'wing-with-no-finance-destination-320')
  await page.getByRole('button',{name:'Edit proposal',exact:true}).click();await chooseOption(page,'Recipient group','Selected homes');await page.getByRole('textbox',{name:'Search message recipients',exact:true}).fill('A-101');await page.getByRole('checkbox',{name:'A-101',exact:true}).check();await chooseOption(page,'Delivery channel','WhatsApp · simulation');await capture(page,'selected-home-finance-whatsapp-320')
  let release=()=>{},ready=()=>{};const held=new Promise<void>(r=>{release=r}),started=new Promise<void>(r=>{ready=r})
  await page.route('**/api/messages/preview?*',async route=>{ready();await held;await route.fulfill({status:503,contentType:'application/json',body:'{"error":"temporarily_unavailable"}'})})
  try {await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await started;await expect(page.getByRole('combobox',{name:'Recipient group',exact:true})).toBeDisabled();await capture(page,'statement-preview-loading-short-320');release();await expect(page.getByRole('button',{name:'Try again',exact:true})).toBeVisible();await capture(page,'statement-preview-error-short-320')}finally {release();await page.unroute('**/api/messages/preview?*')}
  await page.getByRole('button',{name:'Try again',exact:true}).click();await expect(page.getByRole('region',{name:'Exact recipient counts'}).locator('strong')).toHaveText(['2','2','2','2','1','0']);await expect(page.getByRole('checkbox',{name:previewCheck,exact:true})).not.toBeChecked();await within(page);await page.keyboard.press('Escape')
 }finally {await a.close()}
})

test('lost statement proposal and approval responses freeze controls and replay one exact operation',async({page,browser})=>{
 const title='STMSG UI lost responses',a=await fixture(page,browser,title)
 try {
  await compose(page,title);await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await page.getByRole('checkbox',{name:previewCheck,exact:true}).check()
  let release!:()=>void,ready!:()=>void;const held=new Promise<void>(r=>{release=r}),started=new Promise<void>(r=>{ready=r}),payloads:string[]=[]
  await page.route('**/api/messages',async route=>{if(route.request().method()!=='POST'){await route.continue();return}payloads.push(route.request().postData()!);if(payloads.length===1){await route.fetch();ready();await held;await route.abort('failed')}else await route.continue()})
  await page.getByRole('button',{name:'Propose for separate review',exact:true}).click();await started;await expect(page.getByRole('button',{name:'Edit proposal',exact:true})).toBeDisabled();await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(1);release()
  await expect(page.getByRole('button',{name:'Retry this proposal',exact:true})).toBeEnabled();await capture(page,'lost-proposal-retry-1440');await page.getByRole('button',{name:'Retry this proposal',exact:true}).click();await expect(page.getByRole('dialog').locator('.contact-state').first()).toHaveText('Awaiting separate review');expect(payloads).toHaveLength(2);expect(payloads[1]).toBe(payloads[0]);await page.unroute('**/api/messages')
  const id=new URLSearchParams(page.url().split('?')[1]).get('message')!;await openMessage(a.reviewer,id);await decision(a.reviewer,'Review for approval');const approvals:string[]=[]
  await a.reviewer.route('**/api/messages/'+id+'/actions',async route=>{approvals.push(route.request().postData()!);if(approvals.length===1){await route.fetch();await route.abort('failed')}else await route.continue()})
  await a.reviewer.getByRole('button',{name:'Approve frozen proposal',exact:true}).click();await expect(a.reviewer.getByRole('button',{name:'Retry this decision',exact:true})).toBeEnabled();await expect(a.reviewer.getByRole('textbox',{name:'Decision reason',exact:true})).toBeDisabled();await a.reviewer.getByRole('button',{name:'Retry this decision',exact:true}).click();await expect(a.reviewer.getByRole('dialog').locator('.contact-state').first()).toHaveText('Approved for delivery');expect(approvals).toHaveLength(2);expect(approvals[1]).toBe(approvals[0]);await a.reviewer.unroute('**/api/messages/'+id+'/actions')
  const current=await(await page.request.get('/api/messages/'+id)).json();expect(current.version).toBe(2);expect(current.events.filter((x:{action:string})=>x.action==='APPROVED')).toHaveLength(1)
 }finally {await a.close()}
})

test('community-only permission does not authorise finance statements and current access controls link and search',async({page,browser})=>{
 const title='STMSG UI private source title',a=await fixture(page,browser,title),tenantContext=await browser.newContext({baseURL:new URL(page.url()).origin}),tenant=await tenantContext.newPage()
 try {
  await login(tenant,'Tenant');const profile=await(await tenant.request.get('/api/contacts/me')).json();await messagePost(tenant,'/api/contacts/me/actions',{version:profile.version,action:'OPTED_OUT',channel:'EMAIL',purpose:'FINANCE',reason:'Deliberately opted out of finance messages while keeping community consent.'})
  await compose(page,title);await chooseOption(page,'Recipient group','Tenants');await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await expect(page.getByRole('region',{name:'Exact recipient counts'}).locator('strong')).toHaveText(['35','35','0','0','0','35']);await expect(page.getByRole('button',{name:'Propose for separate review',exact:true})).toBeDisabled();await capture(page,'finance-consent-required-1440');await page.keyboard.press('Escape')
  await messageSources(page,a.reviewer,title+' restored contact fixture');const id=await messageProposal(page,a.pub,{kind:'TENANTS',wing:'',ids:[]},'STATEMENT');await messageApprove(a.reviewer,id);await openMessage(tenant,id);await expect(tenant.getByRole('dialog').getByRole('heading',{name:'Published financial statement',exact:true})).toBeVisible();await expect(tenant.getByRole('link',{name:'Open shared statement',exact:true})).toBeVisible()
  const privateSearch=await(await tenant.request.get('/api/messages?'+new URLSearchParams({kind:'STATEMENT',q:title}))).json();expect(privateSearch.total).toBe(0)
  const body=await(await tenant.request.get('/api/messages/'+id)).json();expect(body.source.publication_target).toBeUndefined();expect(body.source.version).toBe('');expect(body.source.title).toBe('Published financial statement')
  messagePrivateFixture("db.execute(\"UPDATE flat_memberships SET end_date='2026-01-01' WHERE resident_id='demo-tenant-A-103'\")")
  await openMessage(tenant,id);await expect(tenant.getByRole('link',{name:'Open shared statement',exact:true})).toHaveCount(0);expect((await tenant.request.get('/api/financial-statements/'+a.file+'/download')).status()).toBe(404)
 }finally {messagePrivateFixture("db.execute(\"UPDATE flat_memberships SET end_date=NULL WHERE resident_id='demo-tenant-A-103'\")");await tenantContext.close();await a.close()}
})
