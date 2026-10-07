import { test,expect } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { chooseOption,login,navigate } from './helpers'
import { messageActors,messageSources,messageProposal,messageApprove,messagePrivateFixture,openMessage } from './messages-fixtures'
import { apiReceived,ensureMaintenanceReviewer,financialHeaders } from './maintenance-fixtures'
import { statementOriginal,statementApprove,statementPublish,originalCSV } from './statements-fixtures'
import { prepareProviderUI,providerAction,providerCapture,providerControl,providerProof,providerRefresh,providerStatus,providerWithin } from './whatsapp-provider-helpers'

test.setTimeout(90000)
test('provider preparation opens real menus at every layout freezes the actual template and requires a different reviewer',async({page,browser})=>{
  await providerControl(page,{reset:true});const a=await messageActors(page,browser)
  try {
    const title='WHATSAPP UI exact private notice';await messageSources(page,a.reviewer,title)
    await navigate(page,'Messages');await expect(page.locator('.message-channel-banner')).toContainText('WhatsApp provider test')
    await providerCapture(page,'provider-register-empty-1440');await prepareProviderUI(page,title)
    for(const [width,height] of [[1440,1000],[768,1024],[375,640],[320,640],[320,440]]) {
      await page.setViewportSize({width,height});const menu=page.getByRole('combobox',{name:'Delivery channel',exact:true});await menu.scrollIntoViewIfNeeded();await menu.click()
      const selected=page.getByRole('option',{name:'WhatsApp · provider test',exact:true});await expect(selected).toHaveAttribute('aria-selected','true');await selected.hover()
      await expect(page.getByRole('listbox')).toBeVisible();await providerCapture(page,`channel-menu-${width}-${height}`);await expect(page.getByRole('listbox')).toBeVisible();await page.keyboard.press('Escape');await expect(page.getByRole('listbox')).toHaveCount(0);await expect(menu).toBeFocused();await providerWithin(page)
    }
    await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click()
    await expect(page.getByRole('region',{name:'Reviewed WhatsApp template'})).toContainText('society_notice');await expect(page.getByRole('region',{name:'Reviewed WhatsApp template'})).toContainText('Marketing')
    await expect(page.getByRole('region',{name:'Exact message preview'})).toContainText('Your society has shared an update. Open it securely:');await expect(page.getByRole('region',{name:'Exact message preview'})).not.toContainText(title)
    for(const [width,height] of [[1440,1000],[768,1024],[375,640],[320,640],[320,440]]){
      await page.setViewportSize({width,height});await page.locator('.dialog-scroll').evaluate(element=>element.scrollTop=0);await providerCapture(page,`frozen-template-${width}-${height}`)
      await page.getByRole('region',{name:'Reviewed WhatsApp template'}).evaluate(element=>element.scrollIntoView({block:'center',behavior:'instant'}));await providerCapture(page,`reviewed-template-${width}-${height}`)
      await page.getByRole('region',{name:'Exact message preview'}).evaluate(element=>element.scrollIntoView({block:'center',behavior:'instant'}));await providerCapture(page,`exact-template-${width}-${height}`);await providerWithin(page)
    }
    await page.getByRole('checkbox',{name:'I reviewed this exact content, eligible recipients, unique destinations and omissions.',exact:true}).check()
    await page.getByRole('button',{name:'Propose for separate review',exact:true}).scrollIntoViewIfNeeded();await providerCapture(page,'preview-actions-320-440')
    await page.getByRole('button',{name:'Propose for separate review',exact:true}).click()
    await expect(page.getByRole('dialog').locator('.contact-state').first()).toHaveText('Awaiting separate review');await expect(page.getByRole('button',{name:'Review for approval',exact:true})).toHaveCount(0)
    const id=new URLSearchParams(page.url().split('?')[1]).get('message')!;const detail=await(await page.request.get('/api/messages/'+id)).json()
    expect(detail.provider_mode).toBe('CLOUD_FIXTURE');expect(detail.counts).toMatchObject({eligible_people:2,destinations:1});expect(detail.provider.category).toBe('MARKETING')
    await openMessage(a.reviewer,id);await a.reviewer.setViewportSize({width:320,height:440});await providerAction(a.reviewer,'Review for approval');await providerCapture(a.reviewer,'separate-review-320-440')
    await a.reviewer.getByRole('button',{name:'Approve frozen proposal',exact:true}).click();await expect(a.reviewer.getByRole('dialog').locator('.contact-state').first()).toHaveText('Approved for delivery')
  } finally {await a.close()}
})

test('portal Send locks held controls submits one exact provider request and advances only with signed delivery evidence',async({page,browser})=>{
  await providerControl(page,{reset:true});const a=await messageActors(page,browser);let release=()=>{}
  try {
    const source=await messageSources(page,a.reviewer,'WHATSAPP UI actual portal send'),id=await messageProposal(page,source,{kind:'OWNERS',wing:'',ids:[]},'NOTICE','WHATSAPP');await messageApprove(a.reviewer,id)
    const before=await providerStatus(page);await openMessage(page,id);await page.setViewportSize({width:320,height:440});await providerAction(page,'Send test message')
    await expect(page.getByRole('combobox',{name:'Simulation outcome',exact:true})).toHaveCount(0);await providerCapture(page,'portal-send-form-320-440')
    await page.getByRole('button',{name:'Send test message',exact:true}).scrollIntoViewIfNeeded();await providerCapture(page,'portal-send-actions-320-440')
    const held=new Promise<void>(resolve=>{release=resolve});let arrived!:()=>void;const started=new Promise<void>(resolve=>{arrived=resolve})
    await page.route('**/api/messages/'+id+'/dispatch',async route=>{const response=await route.fetch();arrived();await held;await route.fulfill({response}).catch(()=>{})})
    await page.getByRole('button',{name:'Send test message',exact:true}).click();await started
    await expect(page.getByRole('button',{name:'Close message',exact:true})).toBeDisabled();await expect(page.getByRole('textbox',{name:'Decision reason',exact:true})).toBeDisabled();await expect(page.getByRole('link',{name:'Open shared notice',exact:true})).toHaveCount(0);await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toBeVisible();await providerCapture(page,'portal-send-held-320-440')
    release();await page.unroute('**/api/messages/'+id+'/dispatch');await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Provider accepted')
    const after=await providerStatus(page);expect(after.send_count-before.send_count).toBe(1);const send=after.sends.at(-1)!
    expect(send.authorization_valid).toBe(true);expect(send.request).toMatchObject({messaging_product:'whatsapp',recipient_type:'individual',to:'+919000000101',type:'template',messaging_account_id:'333333',template:{name:'society_notice',language:{code:'en'}}})
    expect(send.request.template.components).toEqual([{type:'body',parameters:[{type:'text',text:new URL('/#community?notice='+source,page.url()).href}]}]);expect(send.request.biz_opaque_callback_data).toBeTruthy()
    let detail=await(await page.request.get('/api/messages/'+id)).json();expect(detail.outcomes.ACCEPTED).toBe(1);expect(detail.deliveries[0]).toMatchObject({attempts:1,delivered_at:0,read_at:0})
    expect(await providerProof(page,send.id,'delivered',true)).toBe(403);await providerRefresh(page,id,'ACCEPTED')
    expect(await providerProof(page,send.id,'delivered')).toBe(200);expect(await providerProof(page,send.id,'delivered')).toBe(200);await providerRefresh(page,id,'DELIVERED');await page.getByRole('region',{name:'Reported delivery outcomes'}).evaluate(element=>element.scrollIntoView({block:'center',behavior:'instant'}));await providerCapture(page,'signed-delivery-320-440')
    expect(await providerProof(page,send.id,'read')).toBe(200);expect(await providerProof(page,send.id,'failed')).toBe(200);await providerRefresh(page,id,'READ');await page.getByRole('region',{name:'Reported delivery outcomes'}).evaluate(element=>element.scrollIntoView({block:'center',behavior:'instant'}));await providerCapture(page,'read-with-late-failure-320-440')
    detail=await(await page.request.get('/api/messages/'+id)).json();expect(detail.deliveries[0].attempts).toBe(1);expect(detail.deliveries[0].read_at).toBeGreaterThan(0);expect((await providerStatus(page)).send_count).toBe(after.send_count)
    messagePrivateFixture("assert db.execute('SELECT COUNT(*) FROM receipts').fetchone()[0]==0;assert db.execute('SELECT COUNT(*) FROM simulation_messages').fetchone()[0]==0")
    const link=page.getByRole('link',{name:'Open shared notice',exact:true});await expect(link).toHaveAttribute('href','/#community?notice='+source);await link.scrollIntoViewIfNeeded();await providerCapture(page,'notice-source-link-320-440');await link.click();await expect(page).toHaveURL(new RegExp('notice='+source));await expect(page.getByRole('dialog').getByRole('heading',{name:'WHATSAPP UI actual portal send',exact:true})).toBeVisible()
  } finally {release();await a.close()}
})

test('a lost provider response remains unknown through human reconciliation until its opaque signed callback arrives',async({page,browser})=>{
  await providerControl(page,{reset:true,mode:'DROP_RESPONSE'});const a=await messageActors(page,browser)
  try {
    const source=await messageSources(page,a.reviewer,'WHATSAPP UI unresolved original handoff'),id=await messageProposal(page,source,{kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']},'NOTICE','WHATSAPP');await messageApprove(a.reviewer,id)
    const before=await providerStatus(page);await openMessage(page,id);await page.setViewportSize({width:375,height:640});await providerAction(page,'Send test message');await page.getByRole('button',{name:'Send test message',exact:true}).click()
    await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Needs reconciliation');await expect(page.getByRole('button',{name:'Send test message',exact:true})).toHaveCount(0);await providerCapture(page,'uncertain-handoff-375')
    const after=await providerStatus(page);expect(after.send_count-before.send_count).toBe(1);let detail=await(await page.request.get('/api/messages/'+id)).json();expect(detail.deliveries[0]).toMatchObject({provider_id:'',accepted_at:0,attempts:1})
    await providerAction(page,'Reconcile handoff');await page.getByRole('button',{name:'Reconcile this handoff',exact:true}).click();await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Needs reconciliation');await expect(page.getByRole('button',{name:'Send test message',exact:true})).toHaveCount(0)
    expect((await providerStatus(page)).send_count).toBe(after.send_count);expect(await providerProof(page,after.sends.at(-1)!.id,'read')).toBe(200)
    await providerRefresh(page,id,'READ');detail=await(await page.request.get('/api/messages/'+id)).json();expect(detail.deliveries[0]).toMatchObject({attempts:1,delivered_at:0});await providerCapture(page,'opaque-read-reconciliation-375')
  } finally {await a.close()}
})

test('lost browser response retries the accepted operation even when the provider template later pauses',async({page,browser})=>{
  await providerControl(page,{reset:true});const a=await messageActors(page,browser)
  try {
    const source=await messageSources(page,a.reviewer,'WHATSAPP UI accepted operation replay'),id=await messageProposal(page,source,{kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']},'NOTICE','WHATSAPP');await messageApprove(a.reviewer,id);await openMessage(page,id)
    const before=await providerStatus(page),bodies:string[]=[];let lost=true
    await page.route('**/api/messages/'+id+'/dispatch',async route=>{bodies.push(route.request().postData()!);const response=await route.fetch();if(lost){lost=false;await route.abort('failed')}else await route.fulfill({response})})
    await providerAction(page,'Send test message');await page.getByRole('button',{name:'Send test message',exact:true}).click();await expect(page.getByRole('button',{name:'Retry this decision',exact:true})).toBeEnabled();await expect(page.getByRole('button',{name:'Close message',exact:true})).toBeDisabled()
    await providerControl(page,{template:{id:'444444',status:'PAUSED'}});await providerCapture(page,'lost-browser-response-1440')
    await page.getByRole('button',{name:'Retry this decision',exact:true}).click();await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Provider accepted');expect(bodies).toHaveLength(2);expect(bodies[0]).toBe(bodies[1]);expect((await providerStatus(page)).send_count-before.send_count).toBe(1)
    await page.unroute('**/api/messages/'+id+'/dispatch')
  } finally {await a.close()}
})

test('definitive provider rejection permits a deliberate retry while rate limits and template changes block new sends',async({page,browser})=>{
  await providerControl(page,{reset:true,mode:'REJECTED'});const a=await messageActors(page,browser)
  try {
    const source=await messageSources(page,a.reviewer,'WHATSAPP UI deliberate rejection retry'),id=await messageProposal(page,source,{kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']},'NOTICE','WHATSAPP');await messageApprove(a.reviewer,id);await openMessage(page,id)
    await providerAction(page,'Send test message');await page.getByRole('button',{name:'Send test message',exact:true}).click();await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Definitively failed');await providerCapture(page,'definitive-rejection-1440')
    await providerControl(page,{mode:'RATE_LIMITED'});await providerAction(page,'Send test message');await page.getByRole('button',{name:'Send test message',exact:true}).click();await expect(page.getByRole('button',{name:'Send test message',exact:true})).toHaveCount(0);await expect(page.getByRole('region',{name:'Envelope outcomes'})).toContainText('Retry available after');await providerCapture(page,'provider-cooldown-1440')
    let detail=await(await page.request.get('/api/messages/'+id)).json();expect(detail.retryable_deliveries).toBe(0);expect(detail.deliveries[0].attempts).toBe(2)
    await providerControl(page,{mode:'ACCEPTED'});const other=await messageProposal(page,source,{kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']},'NOTICE','WHATSAPP');await messageApprove(a.reviewer,other);await openMessage(page,other)
    const before=await providerStatus(page);await providerControl(page,{template:{id:'444444',components:[{type:'BODY',text:'The wording changed after review. Open it securely: {{1}}'}]}})
    await providerAction(page,'Send test message');await page.getByRole('button',{name:'Send test message',exact:true}).click();await expect(page.getByRole('button',{name:/Reload.*review|Reload current|Reload/})).toBeVisible();expect((await providerStatus(page)).send_count).toBe(before.send_count);detail=await(await page.request.get('/api/messages/'+other)).json();expect(detail.outcomes.QUEUED).toBe(1);expect(detail.deliveries[0].attempts).toBe(0);await providerCapture(page,'changed-template-blocked-1440')
  } finally {await a.close()}
})

test('a paused template shows a recoverable preview error and residents cannot prepare or view private pending proposals',async({page,browser})=>{
  await providerControl(page,{reset:true});const a=await messageActors(page,browser),ownerContext=await browser.newContext({baseURL:new URL(page.url()).origin}),owner=await ownerContext.newPage()
  try {
    const title='WHATSAPP UI unavailable approved template',source=await messageSources(page,a.reviewer,title);await prepareProviderUI(page,title);await providerControl(page,{template:{id:'444444',status:'PAUSED'}})
    await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await expect(page.getByRole('alert')).toContainText('not currently approved');await page.setViewportSize({width:320,height:440});await page.getByRole('alert').scrollIntoViewIfNeeded();await providerCapture(page,'paused-template-error-320-440');await providerWithin(page)
    await providerControl(page,{reset:true});await page.getByRole('button',{name:'Try again',exact:true}).click();await expect(page.getByRole('region',{name:'Reviewed WhatsApp template'})).toBeVisible();await page.getByRole('button',{name:'Close message proposal',exact:true}).click()
    const id=await messageProposal(page,source,{kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']},'NOTICE','WHATSAPP');await login(owner,'Owner');await navigate(owner,'Messages');await expect(owner.getByRole('button',{name:'Prepare a message',exact:true})).toHaveCount(0)
    expect((await owner.request.get('/api/messages/'+id)).status()).toBe(404);const config=await(await page.request.get('/api/messages/config')).json();expect(JSON.stringify(config)).not.toMatch(/access_token|app_secret|verify_token|FICTIONAL_/)
    await messageApprove(a.reviewer,id);const own=await(await owner.request.get('/api/messages/'+id)).json();expect(own.provider_mode).toBe('CLOUD_FIXTURE');expect(own.provider).toBeUndefined();expect(own.counts.destinations).toBe(1)
  } finally {await ownerContext.close();await a.close()}
})

test('portal utility receipt and statement sends preserve finance authority exact originals and clickable current source links',async({page,browser})=>{
 const a=await messageActors(page,browser),ownerContext=await browser.newContext({baseURL:new URL(page.url()).origin,viewport:{width:320,height:440}}),tenantContext=await browser.newContext({baseURL:new URL(page.url()).origin,viewport:{width:320,height:440}}),owner=await ownerContext.newPage(),tenant=await tenantContext.newPage();let receiptMessage=''
 try{
  await providerControl(page,{reset:true});await messageSources(page,a.reviewer,'SUPPLEMENT fictional financial contacts');const received=await apiReceived(page,'432.19')
  const denied=await a.reviewer.request.post('/api/messages/preview',{headers:await financialHeaders(a.reviewer),data:{source_kind:'RECEIPT',source_id:received.receipt_id,channel:'WHATSAPP',target:{kind:'ALL',wing:'',ids:[]}}});expect(denied.status()).toBe(403)
  await ensureMaintenanceReviewer(page);await login(a.reviewer,'Committee')
  const file=await statementOriginal(page,'SUPPLEMENT PRIVATE exact tenant original');await statementApprove(a.reviewer,file);const pub=await statementPublish(page,a.reviewer,file)
  const before=await providerStatus(page)
  for(const {kind,source,title,template,destination,link,counts} of [
   {kind:'RECEIPT',source:received.receipt_id,title:received.receipt_number,template:'society_receipt',destination:'+919000000101',link:'/#receipts?entry='+received.id,counts:{target_people:153,source_people:2,eligible_people:2,destinations:1}},
   {kind:'STATEMENT',source:pub,title:'SUPPLEMENT PRIVATE exact tenant original',template:'society_statement',destination:'+919000000103',link:'/#statements?statement='+file,counts:{target_people:153,source_people:35,eligible_people:1,destinations:1}},
  ]){
   await navigate(page,'Messages');await page.getByRole('button',{name:'Prepare a message',exact:true}).click()
   await chooseOption(page,'Message source',kind==='RECEIPT'?'Original received-money receipt':'Published financial statement');await chooseOption(page,'Delivery channel','WhatsApp · provider test')
   await page.getByRole('textbox',{name:'Search published message sources',exact:true}).fill(title);await page.locator('.message-source-option').filter({hasText:title}).click()
   await page.getByRole('textbox',{name:'Proposal reason',exact:true}).fill('Reviewed this original, approved utility template, exact financial audience and deliberate handoff.')
   await page.getByRole('button',{name:'Preview exact recipients',exact:true}).click();await expect(page.getByRole('region',{name:'Reviewed WhatsApp template'})).toContainText(template);await expect(page.getByRole('region',{name:'Reviewed WhatsApp template'})).toContainText('Utility')
   const envelope=page.getByRole('region',{name:'Exact message preview'});await expect(envelope).not.toContainText('432.19');await expect(envelope).not.toContainText('A-101');await expect(envelope).not.toContainText('SUPPLEMENT PRIVATE')
   await page.setViewportSize({width:320,height:440});await envelope.evaluate(e=>e.scrollIntoView({block:'center',behavior:'instant'}));await providerCapture(page,'finance-'+kind.toLowerCase()+'-preview-320-440')
   await page.getByRole('checkbox',{name:'I reviewed this exact content, eligible recipients, unique destinations and omissions.',exact:true}).check();await page.getByRole('button',{name:'Propose for separate review',exact:true}).click()
   await expect(page.getByRole('dialog').locator('.contact-state').first()).toHaveText('Awaiting separate review');const id=new URLSearchParams(page.url().split('?')[1]).get('message')!;const response=await page.request.get('/api/messages/'+id);expect(response.status()).toBe(200);const proposed=await response.json();expect(proposed.source.id).toBe(source);expect(proposed.purpose).toBe('FINANCE');expect(proposed.counts).toMatchObject(counts)
   await messageApprove(a.reviewer,id);await openMessage(page,id);await providerAction(page,'Send test message')
   await page.getByRole('button',{name:'Send test message',exact:true}).click();await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText('Provider accepted')
   const sent=(await providerStatus(page)).sends.at(-1)!;expect(sent.request.to).toBe(destination);expect(sent.request.template.name).toBe(template);expect(sent.request.template.components).toEqual([{type:'body',parameters:[{type:'text',text:new URL(link,page.url()).href}]}])
   if(kind==='RECEIPT'){
    receiptMessage=id;await login(owner,'Owner');await openMessage(owner,id);const link=owner.getByRole('link',{name:'Open shared receipt',exact:true});await expect(link).toHaveAttribute('href','/#receipts?entry='+received.id);await link.scrollIntoViewIfNeeded();await providerCapture(owner,'own-receipt-source-link');await providerWithin(owner)
    const data=await(await owner.request.get('/api/messages/'+id)).json();expect(data.provider).toBeUndefined();expect(data.provider_mode).toBe('CLOUD_FIXTURE');await link.click();await expect(owner).toHaveURL(new RegExp('#receipts\\?entry='+received.id));await expect(owner.getByRole('dialog').getByRole('heading',{name:'A clear record.',exact:true})).toBeVisible();await expect(owner.getByRole('dialog')).toContainText('₹432.19')
    const download=owner.getByRole('button',{name:'Download PDF',exact:true});await expect(download).toBeEnabled({timeout:10000});const waiting=owner.waitForEvent('download');await download.click();const original=await waiting;expect(original.suggestedFilename()).toBe(received.receipt_number+'.pdf');expect(readFileSync((await original.path())!).subarray(0,5).toString()).toBe('%PDF-');await providerCapture(owner,'own-receipt-original-320-440')
   }else{
    await login(tenant,'Tenant');await openMessage(tenant,id);const link=tenant.getByRole('link',{name:'Open shared statement',exact:true});await expect(link).toHaveAttribute('href','/#statements?statement='+file);await link.scrollIntoViewIfNeeded();await providerCapture(tenant,'own-statement-source-link');await providerWithin(tenant);const data=await(await tenant.request.get('/api/messages/'+id)).json();expect(data.provider).toBeUndefined();expect((await tenant.request.get('/api/receipts/'+received.receipt_id+'/download')).status()).toBe(403)
    await link.click();await expect(tenant).toHaveURL(new RegExp('statement='+file));await expect(tenant.getByRole('dialog').getByRole('heading',{name:'SUPPLEMENT PRIVATE exact tenant original',level:2,exact:true})).toBeVisible();const waiting=tenant.waitForEvent('download');await tenant.getByRole('button',{name:'Download original',exact:true}).click();const original=await waiting;expect(readFileSync((await original.path())!)).toEqual(originalCSV);await providerCapture(tenant,'own-statement-original-320-440')
   }
   await page.keyboard.press('Escape')
  }
  expect((await providerStatus(page)).send_count-before.send_count).toBe(2)
  const unchanged=await(await page.request.get('/api/entries/'+received.id)).json();expect(unchanged.amount_paise).toBe(43219);expect(unchanged.receipt_id).toBe(received.receipt_id)
  const original=await page.request.get('/api/financial-statements/'+file+'/download');expect(original.status()).toBe(200);expect(await original.body()).toEqual(originalCSV)
  messagePrivateFixture("r=db.execute(\"UPDATE flat_memberships SET end_date='2026-01-01' WHERE resident_id='demo-owner-A-101'\");assert r.rowcount==2")
  await openMessage(owner,receiptMessage);await expect(owner.getByRole('link',{name:'Open shared receipt',exact:true})).toHaveCount(0);const former=await(await owner.request.get('/api/messages/'+receiptMessage)).json();expect(former.source.link).toBe('');expect(former.envelope).toBe('');expect((await owner.request.get('/api/receipts/'+received.receipt_id+'/download')).status()).toBe(403);await providerCapture(owner,'former-owner-receipt-link-denied-320-440')
 }finally{messagePrivateFixture("db.execute(\"UPDATE flat_memberships SET end_date=NULL WHERE resident_id='demo-owner-A-101'\")");await ownerContext.close();await tenantContext.close();await a.close()}
})
