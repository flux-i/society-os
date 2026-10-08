import { test,expect } from '@playwright/test'
import { login } from './helpers'
import { names,execute } from './native-webmcp-helpers'
import type { ContextDocument } from './native-webmcp-helpers'
import { messagePost,messageApprove,messagePrivateFixture,openMessage } from './messages-fixtures'
import { meetingProposal,meetingApprove,meetingDetail,meetingPost } from './meetings-fixtures'
import { reminderActors,reminderMaintenance,reminderPayment,reminderAllocate,reminderMessage,reminderProposal,reminderReviewCheck } from './reminders-fixtures'

test.setTimeout(90000)
test('actual native reminder status discovery bounded pages privacy and exception navigation preserve human review',async({page,browser})=>{
 const actors=await reminderActors(page,browser)
 try{
  const cycle=await reminderMaintenance(page,actors.reviewer,'PRIVATE_NATIVE_RMD supplied maintenance')
  const id=await reminderProposal(page,'MAINTENANCE_REMINDER',cycle.id)
  await expect.poll(()=>names(page)).toContain('society_find_delivery_exceptions');await openMessage(actors.reviewer,id);await actors.reviewer.getByRole('button',{name:'Review for approval',exact:true}).click();const marker='PRIVATE_NATIVE_RMD leave this human review untouched.';await actors.reviewer.getByRole('textbox',{name:'Decision reason',exact:true}).fill(marker)
  const writes:string[]=[];actors.reviewer.on('request',r=>{if(!['GET','HEAD'].includes(r.method()))writes.push(r.url())})
  const metadata=JSON.parse(await execute(actors.reviewer,'society_read_message',{message_id:id}));expect(metadata).toMatchObject({source_kind:'MAINTENANCE_REMINDER',purpose:'FINANCE',state:'PENDING',counts:{target_people:1,eligible_people:1,destinations:1}})
  expect(Object.keys(metadata).sort()).toEqual(['id','source_kind','state','channel','purpose','simulation','provider_mode','version','snapshot_version','counts','outcomes','retryable_deliveries'].sort())
  for(const field of ['outstanding_paise','fingerprint','reminder_current_key','destination','source_id','homes','obligations','recipients','envelope','title'])expect(JSON.stringify(metadata)).not.toContain('"'+field+'":');expect(JSON.stringify(metadata)).not.toContain('PRIVATE_NATIVE_RMD');expect(JSON.stringify(metadata)).not.toContain('175025')
  const list=JSON.parse(await execute(actors.reviewer,'society_find_messages',{kind:'MAINTENANCE_REMINDER'}));expect(list.items).toEqual([metadata]);expect(list.total).toBe(1)
  const queue=JSON.parse(await execute(actors.reviewer,'society_find_delivery_exceptions',{}));expect(queue.total).toBe(0);expect(queue.items).toEqual([]);expect(queue.page_size).toBe(12)
  await expect(execute(actors.reviewer,'society_open_workspace',{screen:'delivery-exceptions'})).rejects.toThrow();await expect(actors.reviewer.getByRole('textbox',{name:'Decision reason',exact:true})).toHaveValue(marker);await expect(actors.reviewer.getByRole('checkbox',{name:reminderReviewCheck,exact:true})).not.toBeChecked();expect(writes).toEqual([])
  for(let i=0;i<12;i++)await reminderProposal(page,'MAINTENANCE_REMINDER',cycle.id)
  const bounded=JSON.parse(await execute(page,'society_find_messages',{kind:'MAINTENANCE_REMINDER'}));expect(bounded.total).toBe(13);expect(bounded.items).toHaveLength(12);expect(JSON.parse(await execute(page,'society_find_messages',{kind:'MAINTENANCE_REMINDER',page:2})).items).toHaveLength(1)
  await execute(page,'society_open_workspace',{screen:'delivery-exceptions'});await expect(page.getByRole('heading',{name:'No delivery exceptions in this view.',exact:true})).toBeVisible()
 }finally{await actors.close()}
})

test('native held reminder detail and lists discard results after exact personal acknowledgement and deny other people',async({page,browser})=>{
 const actors=await reminderActors(page,browser),context=await browser.newContext({baseURL:new URL(page.url()).origin}),owner=await context.newPage(),tenantContext=await browser.newContext({baseURL:new URL(page.url()).origin}),tenant=await tenantContext.newPage();let release=()=>{}
 try{
  await login(owner,'Owner');await login(tenant,'Tenant');expect(await names(owner)).not.toContain('society_find_delivery_exceptions');await expect(execute(owner,'society_open_workspace',{screen:'delivery-exceptions'})).rejects.toThrow()
  for(const tool of ['society_read_message','society_find_messages']){
   const meeting=await meetingProposal(page,'PRIVATE_NATIVE_RMD held personal '+tool,{scope:'HOMES',home_ids:['demo-flat-A-101']});await meetingApprove(actors.reviewer,meeting);const id=await reminderProposal(page,'MEETING_REMINDER',meeting);await expect(execute(owner,'society_read_message',{message_id:id})).rejects.toThrow();await messageApprove(actors.reviewer,id);await expect(execute(tenant,'society_read_message',{message_id:id})).rejects.toThrow()
   let ready!:()=>void;const held=new Promise<void>(r=>{release=r}),entered=new Promise<void>(r=>{ready=r}),pattern=tool==='society_read_message'?'**/api/messages/'+id:'**/api/messages?**';let calls=0
   await owner.route(pattern,async route=>{if(++calls>1){await route.continue();return};const response=await route.fetch();ready();await held;await route.fulfill({response}).catch(()=>{})})
   const reading=execute(owner,tool,tool==='society_read_message'?{message_id:id}:{kind:'MEETING_REMINDER'});await entered;const source=await meetingDetail(owner,meeting,false);await meetingPost(owner,'/api/meetings/'+meeting+'/acknowledgements',{version:source.version,fingerprint:source.acknowledgement.fingerprint});release();await expect(reading).rejects.toThrow();await owner.unroute(pattern)
   const current=JSON.parse(await execute(owner,'society_read_message',{message_id:id}));expect(current.counts.target_people).toBe(1);expect(current.source_kind).toBe('MEETING_REMINDER');expect(current.outcomes).toEqual({QUEUED:1})
  }
 }finally{release();await Promise.allSettled([actors.close(),context.close(),tenantContext.close()])}
})

test('native held exception status rejects changed exact amount and never retries or reconciles a failed handoff',async({page,browser})=>{
 const actors=await reminderActors(page,browser);let release=()=>{}
 try{
  const cycle=await reminderMaintenance(page,actors.reviewer,'PRIVATE_NATIVE_RMD exception amount',[{flat_id:'demo-flat-A-101',amount:'1000.00'}]),receipt=await reminderPayment(page,cycle.lines[0].entry_id,'10.00','0.01'),id=await reminderProposal(page,'MAINTENANCE_REMINDER',cycle.id);await messageApprove(actors.reviewer,id);await messagePost(page,'/api/messages/'+id+'/dispatch',{version:2,action:'DISPATCH',outcome:'REJECTED',reason:'PRIVATE_NATIVE_RMD deliberately record a definite failure.'})
  const before=JSON.parse(await execute(page,'society_find_delivery_exceptions',{state:'FAILED'}));expect(before.total).toBe(1);expect(before.items[0]).toMatchObject({message_id:id,state:'FAILED',attempts:1,can_retry:true,can_reconcile:false});for(const field of ['title','destination','current_key','eligibility_problem','PRIVATE_NATIVE_RMD','outstanding_paise','fingerprint','recipients'])expect(JSON.stringify(before)).not.toContain(field)
  let ready!:()=>void;const held=new Promise<void>(r=>{release=r}),entered=new Promise<void>(r=>{ready=r});let calls=0
  await page.route('**/api/messages/exceptions?**',async route=>{if(++calls>1){await route.continue();return};const response=await route.fetch();ready();await held;await route.fulfill({response}).catch(()=>{})});const reading=execute(page,'society_find_delivery_exceptions',{state:'FAILED'});await entered;await reminderAllocate(page,receipt.id,cycle.lines[0].entry_id,'0.01');release();await expect(reading).rejects.toThrow();await page.unroute('**/api/messages/exceptions?**')
  const writes:string[]=[];page.on('request',r=>{if(!['GET','HEAD'].includes(r.method()))writes.push(r.url())});const after=JSON.parse(await execute(page,'society_find_delivery_exceptions',{state:'FAILED'}));expect(after.items[0]).toMatchObject({attempts:1,can_retry:false,can_reconcile:false});expect((await reminderMessage(page,id)).deliveries[0].attempts).toBe(1);expect(writes).toEqual([])
  await expect(execute(page,'society_find_delivery_exceptions',{page:10001})).rejects.toThrow();await expect(execute(page,'society_find_delivery_exceptions',{state:'DELIVERED'})).rejects.toThrow();await expect(execute(page,'society_read_message',{message_id:id,dispatch:true})).rejects.toThrow()
 }finally{release();await actors.close()}
})

test('native abort and held authority change protect private delivery exceptions',async({page,browser})=>{
 const actors=await reminderActors(page,browser);let release=()=>{}
 try{
  const aborted=await page.evaluate(async()=>{const context=(document as ContextDocument).modelContext,tool=(await context.getTools()).find(t=>t.name==='society_find_delivery_exceptions')!,controller=new AbortController();controller.abort();const major=Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1]);try{await context.executeTool(tool,major<155?'{}':{},{signal:controller.signal});return false}catch{return true}});expect(aborted).toBe(true)
  let ready!:()=>void;const held=new Promise<void>(r=>{release=r}),entered=new Promise<void>(r=>{ready=r});let calls=0
  await actors.reviewer.route('**/api/messages/exceptions?**',async route=>{if(++calls>1){await route.continue();return};const response=await route.fetch();ready();await held;await route.fulfill({response}).catch(()=>{})});const reading=execute(actors.reviewer,'society_find_delivery_exceptions',{});await entered
  messagePrivateFixture("db.execute(\"UPDATE role_grants SET valid_until=strftime('%s','now')-1 WHERE user_id='demo-user-committee' AND role='TREASURER' AND revoked_at IS NULL\")");release();await expect(reading).rejects.toThrow();await actors.reviewer.unroute('**/api/messages/exceptions?**')
 }finally{release();messagePrivateFixture("db.execute(\"UPDATE role_grants SET valid_until=strftime('%s','now')+30*86400 WHERE user_id='demo-user-committee' AND role='TREASURER' AND revoked_at IS NULL\")");await actors.close()}
})
