import { test,expect } from '@playwright/test'
import { execute,names } from './native-webmcp-helpers'
import { statementActors,statementOriginal,statementApprove,statementPublish } from './statements-fixtures'
import { apiReceived } from './maintenance-fixtures'
import { messageActors,messageSources,messageProposal,messageApprove,openMessage } from './messages-fixtures'
import { providerAction,providerControl,providerStatus } from './whatsapp-provider-helpers'

test.setTimeout(90000)
test('actual native provider status is bounded private preserves the human Send form and never invokes the provider',async({page,browser})=>{
  await providerControl(page,{reset:true});const a=await messageActors(page,browser)
  try {
    const source=await messageSources(page,a.reviewer,'NATIVE WhatsApp private original'),id=await messageProposal(page,source,{kind:'OWNERS',wing:'',ids:[]},'NOTICE','WHATSAPP');await messageApprove(a.reviewer,id);await openMessage(page,id);await providerAction(page,'Send test message')
    const marker='PRIVATE_NATIVE_PROVIDER_FORM retain this deliberate human decision.';await page.getByRole('textbox',{name:'Decision reason',exact:true}).fill(marker)
    // Editing the reason correctly clears the prior confirmation. Establish
    // the intended enabled form before testing that native reads preserve it.
    await page.getByRole('checkbox',{name:'I reviewed the current content, recipients, permissions and this decision’s effect.',exact:true}).check()
    await expect(page.getByRole('button',{name:'Send test message',exact:true})).toBeEnabled()
    await expect.poll(()=>names(page)).toContain('society_read_message');const before=await providerStatus(page),writes:string[]=[];page.on('request',request=>{if(!['GET','HEAD'].includes(request.method()))writes.push(request.url())})
    const metadata=JSON.parse(await execute(page,'society_read_message',{message_id:id}));expect(metadata.provider_mode).toBe('CLOUD_FIXTURE');expect(metadata.state).toBe('APPROVED');expect(metadata.counts).toMatchObject({eligible_people:2,destinations:1});expect(metadata.outcomes).toEqual({QUEUED:1})
    for(const value of ['society_notice','444444','111111','222222','body','provider_id','recipients','envelope','PRIVATE_NATIVE','919000000101','access_token','app_secret'])expect(JSON.stringify(metadata)).not.toContain(value)
    expect((await names(page)).filter(name=>name.includes('message')).sort()).toEqual(['society_find_messages','society_read_message']);const list=JSON.parse(await execute(page,'society_find_messages',{}));expect(list.page_size).toBe(12);expect(list.items.every((value:{provider?:unknown})=>value.provider===undefined)).toBe(true)
    await expect(page.getByRole('textbox',{name:'Decision reason',exact:true})).toHaveValue(marker);await expect(page.getByRole('button',{name:'Send test message',exact:true})).toBeEnabled();expect(writes).toEqual([]);expect((await providerStatus(page)).send_count).toBe(before.send_count)
    await expect(execute(page,'society_read_message',{message_id:id,action:'DISPATCH'})).rejects.toThrow();expect((await providerStatus(page)).send_count).toBe(before.send_count)
  } finally {await a.close()}
})

test('actual native reads show uncertainty without fabricating delivery or resending',async({page,browser})=>{
  await providerControl(page,{reset:true,mode:'DROP_RESPONSE'});const a=await messageActors(page,browser)
  try {
    const source=await messageSources(page,a.reviewer,'NATIVE provider unresolved handoff'),id=await messageProposal(page,source,{kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']},'NOTICE','WHATSAPP');await messageApprove(a.reviewer,id);await openMessage(page,id);await providerAction(page,'Send test message');await page.getByRole('button',{name:'Send test message',exact:true}).click();await expect(page.getByRole('button',{name:'Reconcile handoff',exact:true})).toBeVisible()
    const before=await providerStatus(page),metadata=JSON.parse(await execute(page,'society_read_message',{message_id:id}));expect(metadata.outcomes).toEqual({UNKNOWN:1});expect(metadata.retryable_deliveries).toBe(0);expect(metadata.provider_mode).toBe('CLOUD_FIXTURE')
    expect((await providerStatus(page)).send_count).toBe(before.send_count);await expect(page.getByRole('button',{name:'Send test message',exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'Reconcile handoff',exact:true})).toBeVisible()
  } finally {await a.close()}
})


test('actual native utility finance metadata preserves human Send review and cannot expose or hand off financial originals',async({page,browser})=>{
  await providerControl(page,{reset:true});const a=await statementActors(page,browser)
  try {
    await messageSources(page,a.reviewer,'NATIVE utility finance original contacts');const received=await apiReceived(page,'432.19'),file=await statementOriginal(page,'PRIVATE_NATIVE_PROVIDER financial original');await statementApprove(a.reviewer,file);const pub=await statementPublish(page,a.reviewer,file)
    for(const [kind,source,targetPeople,sourcePeople,eligiblePeople] of [['RECEIPT',received.receipt_id,153,2,2],['STATEMENT',pub,153,35,1]] as const){
      const id=await messageProposal(page,source,{kind:'ALL',wing:'',ids:[]},kind,'WHATSAPP');await messageApprove(a.reviewer,id);await openMessage(page,id);await providerAction(page,'Send test message')
      const marker='PRIVATE_NATIVE_FINANCE preserve the independently reviewed human reason.';await page.getByRole('textbox',{name:'Decision reason',exact:true}).fill(marker);await page.getByRole('checkbox',{name:'I reviewed the current content, recipients, permissions and this decision’s effect.',exact:true}).check()
      const before=await providerStatus(page),writes:string[]=[];const listen=(request:{method:()=>string;url:()=>string})=>{if(!['GET','HEAD'].includes(request.method()))writes.push(request.url())};page.on('request',listen)
      const metadata=JSON.parse(await execute(page,'society_read_message',{message_id:id}));expect(metadata.provider_mode).toBe('CLOUD_FIXTURE');expect(metadata.purpose).toBe('FINANCE');expect(metadata.source_kind).toBe(kind);expect(metadata.counts).toMatchObject({target_people:targetPeople,source_people:sourcePeople,eligible_people:eligiblePeople,destinations:1});expect(metadata.outcomes).toEqual({QUEUED:1})
      for(const value of ['society_receipt','society_statement','111111','222222','432.19',source,'919000000101','919000000103','envelope','provider_id','PRIVATE_NATIVE'])expect(JSON.stringify(metadata)).not.toContain(value)
      await expect(page.getByRole('textbox',{name:'Decision reason',exact:true})).toHaveValue(marker);await expect(page.getByRole('button',{name:'Send test message',exact:true})).toBeEnabled();expect(writes).toEqual([]);expect((await providerStatus(page)).send_count).toBe(before.send_count)
      await expect(execute(page,'society_read_message',{message_id:id,action:'DISPATCH'})).rejects.toThrow();expect((await providerStatus(page)).send_count).toBe(before.send_count);page.off('request',listen);await page.keyboard.press('Escape')
    }
    const entry=await(await page.request.get('/api/entries/'+received.id)).json();expect(entry.amount_paise).toBe(43219);expect(entry.receipt_id).toBe(received.receipt_id)
  } finally {await a.close()}
})
