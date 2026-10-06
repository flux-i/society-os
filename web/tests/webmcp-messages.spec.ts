import { test,expect } from '@playwright/test'
import { login } from './helpers'
import { names,execute } from './native-webmcp-helpers'
import type { ContextDocument } from './native-webmcp-helpers'
import { messageActors,messageSources,messageProposal,messageApprove,messagePrivateFixture,openMessage } from './messages-fixtures'

test.setTimeout(90000)
test('actual native messaging tools expose bounded status metadata preserve human forms and never submit deliveries',async({page,browser})=>{
  const a=await messageActors(page,browser)
  try {
    const source=await messageSources(page,a.reviewer,'NATIVE private messaging source');const id=await messageProposal(page,source)
    await expect.poll(()=>names(page)).toContain('society_read_message');await openMessage(a.reviewer,id)
    await a.reviewer.getByRole('button',{name:'Review for approval',exact:true}).click()
    const marker='PRIVATE_NATIVE_FORM leave this human decision unchanged.';await a.reviewer.getByRole('textbox',{name:'Decision reason',exact:true}).fill(marker)
    const writes:string[]=[];a.reviewer.on('request',r=>{if(!['GET','HEAD'].includes(r.method()))writes.push(r.url())})
    const metadata=JSON.parse(await execute(a.reviewer,'society_read_message',{message_id:id}))
    expect(Object.keys(metadata).sort()).toEqual(['id','source_kind','state','channel','purpose','simulation','version','snapshot_version','counts','outcomes','retryable_deliveries'].sort())
    expect(metadata.state).toBe('PENDING');expect(metadata.counts).toMatchObject({target_people:153,source_people:153,eligible_people:3,destinations:2,omitted_people:150});expect(metadata.simulation).toBe(true)
    for(const key of ['source_id','source','title','envelope','PRIVATE','destination','recipients','events','reviewed_by','proposed_by','reason','demo-owner-A-101','shared-message@example.test'])expect(JSON.stringify(metadata)).not.toContain('"'+key+'"')
    const list=JSON.parse(await execute(a.reviewer,'society_find_messages',{state:'PENDING'}));expect(list.total).toBe(1);expect(list.items).toEqual([metadata]);expect(list.page_size).toBe(12)
    const attention=JSON.parse(await execute(a.reviewer,'society_read_overview',{section:'messages'}));expect(attention.counts.awaiting_review).toBe(1);expect(attention.counts.QUEUED).toBe(0);expect(attention.items[0].title).toBe('Community message')
    await expect(a.reviewer.getByRole('textbox',{name:'Decision reason',exact:true})).toHaveValue(marker);await expect(execute(a.reviewer,'society_open_workspace',{screen:'messages'})).rejects.toThrow();expect(writes).toEqual([])
    expect((await names(page)).filter(name=>name.includes('message')).sort()).toEqual(['society_find_messages','society_read_message'])
    await a.reviewer.getByRole('button',{name:'Back to message',exact:true}).click();await messageApprove(a.reviewer,id)
    for(let i=0;i<12;i++)await messageProposal(page,source,{kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']})
    const bounded=JSON.parse(await execute(page,'society_find_messages',{}));expect(bounded.total).toBe(13);expect(bounded.items).toHaveLength(12);expect(JSON.parse(await execute(page,'society_find_messages',{page:2})).items).toHaveLength(1)
  }finally{await a.close()}
})

test('native delivery reads deny private proposals cross-person reads and discard held results after current membership changes',async({page,browser})=>{
  const a=await messageActors(page,browser),ownerContext=await browser.newContext({baseURL:new URL(page.url()).origin}),owner=await ownerContext.newPage();let release=()=>{}
  try {
    await login(owner,'Owner');const source=await messageSources(page,a.reviewer,'NATIVE current messaging audience');const id=await messageProposal(page,source,{kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']})
    await expect(execute(owner,'society_read_message',{message_id:id})).rejects.toThrow();const history=JSON.parse(await execute(owner,'society_find_messages',{}));expect(history.total).toBe(1);expect(history.items.every((x:{id:string})=>x.id!==id)).toBe(true)
    await messageApprove(a.reviewer,id);const own=JSON.parse(await execute(owner,'society_read_message',{message_id:id}));expect(own.counts.target_people).toBe(1);expect(own.counts.destinations).toBe(1)
    await openMessage(owner,id)
    let ready!:()=>void;const held=new Promise<void>(resolve=>{release=resolve}),captured=new Promise<void>(resolve=>{ready=resolve})
    await owner.route('**/api/messages/'+id,async route=>{const response=await route.fetch();ready();await held;await route.fulfill({response}).catch(()=>{})})
    const reading=execute(owner,'society_read_message',{message_id:id});await captured
    messagePrivateFixture("r=db.execute(\"UPDATE flat_memberships SET end_date='2026-01-01' WHERE resident_id='demo-owner-A-101'\");assert r.rowcount==2")
    release();await expect(reading).rejects.toThrow();await owner.unroute('**/api/messages/'+id);await expect(owner.getByRole('dialog')).toHaveCount(0)
    const former=JSON.parse(await execute(owner,'society_read_message',{message_id:id}));expect(former.counts.source_people).toBe(0);expect(former.counts.eligible_people).toBe(0);expect(JSON.stringify(former)).not.toContain('source_id')
  }finally{release();messagePrivateFixture("db.execute(\"UPDATE flat_memberships SET end_date=NULL WHERE resident_id='demo-owner-A-101'\")");await ownerContext.close();await a.close()}
})

test('native message inputs reject extra actions bounds cancellation and held reads from ended sessions',async({page})=>{
  await login(page);await expect.poll(()=>names(page)).toContain('society_find_messages')
  for(const [tool,args] of [
    ['society_find_messages',{page:0}],['society_find_messages',{page:10001}],['society_find_messages',{kind:'STATEMENT'}],['society_find_messages',{state:'DELIVERED'}],['society_find_messages',{destination:'private@example.test'}],
    ['society_read_message',{message_id:''}],['society_read_message',{message_id:'x'.repeat(101)}],['society_read_message',{message_id:'missing',action:'DISPATCH'}],
  ] as const)await expect(execute(page,tool,args)).rejects.toThrow()
  const cancelled=await page.evaluate(async()=>{const context=(document as ContextDocument).modelContext,tool=(await context.getTools()).find(x=>x.name==='society_find_messages')!,controller=new AbortController();controller.abort();const major=Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1]);try{await context.executeTool(tool,major<155?'{}':{},{signal:controller.signal});return false}catch{return true}});expect(cancelled).toBe(true)
  let release!:()=>void,ready!:()=>void;const held=new Promise<void>(resolve=>{release=resolve}),captured=new Promise<void>(resolve=>{ready=resolve})
  await page.route('**/api/messages?*',async route=>{const response=await route.fetch();ready();await held;await route.fulfill({response}).catch(()=>{})})
  const reading=execute(page,'society_find_messages',{});await captured
  try {await page.request.post('/api/auth/logout',{headers:await (await import('./maintenance-fixtures')).financialHeaders(page),data:{}});release();await expect(reading).rejects.toThrow();await expect.poll(()=>names(page)).toEqual([])}finally{release();await page.unroute('**/api/messages?*')}
})
