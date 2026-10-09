import { test, expect } from '@playwright/test'
import { capture, createRehearsal, entity, homeFinance, postAPI, prepareVisibility, readDB, visibility } from './rehearsal-fixtures'
import type { RehearsalActors } from './rehearsal-fixtures'
import { execute, names } from './native-webmcp-helpers'
import { navigate } from './helpers'

test.describe.configure({ mode:'serial' })
test.use({ actionTimeout:10000 })
test.setTimeout(120000)
let actors:RehearsalActors
test.beforeAll(async({browser})=>{actors=await createRehearsal(browser)})
test.afterAll(async()=>{await actors?.close()})

test('actual native tools respect normally imported personal households and withheld finance without exposing setup provenance or mutation tools',async()=>{
  const owner=actors.owner.page,tenant=actors.tenant.page,registry=actors.registry
  await expect.poll(()=>names(owner)).toContain('society_find_homes')
  expect(await names(owner)).not.toContain('society_find_records')
  const homes=JSON.parse(await execute(owner,'society_find_homes',{}));expect(homes.total).toBe(2)
  expect(homes.items.map((x:{number:string})=>x.number).sort()).toEqual(['101','102'])
  const tenancy=JSON.parse(await execute(tenant,'society_find_homes',{}));expect(tenancy.total).toBe(1);expect(tenancy.items[0].number).toBe('103')
  await expect(execute(owner,'society_find_records',{})).rejects.toThrow()
  await expect(execute(owner,'society_grant_household_finance',{home_id:entity('HOME','a-101')})).rejects.toThrow()
  const metadata=JSON.parse(await execute(registry,'society_read_registry_import',{}))
  expect(metadata.registry_counts).toEqual({buildings:1,flats:12,residents:17,memberships:18})
  for(const secret of ['source_key','society_key','digest','verification_note','password','Sample Owner'])expect(JSON.stringify(metadata)).not.toContain(secret)
  expect(readDB('SELECT (SELECT COUNT(*) FROM household_finance_actions),(SELECT COUNT(*) FROM entries),(SELECT COUNT(*) FROM receipts)')[0]).toEqual([0,0,0])
  await navigate(owner,'Your homes');await capture(owner,'native-imported-current-household-desktop')
})

test('actual native reads preserve a human financial review, reject discarded-form navigation, and follow explicit grants without cross-home records',async()=>{
  const finance=actors.finance.page,second=actors.secondFinance.page,owner=actors.owner.page
  await homeFinance(finance,'A-104')
  await expect(finance.getByRole('button',{name:'Review financial access for Sample Owner A 104',exact:true})).toBeDisabled()
  await finance.getByRole('button',{name:'Close details',exact:true}).click()
  await homeFinance(second,'A-104');await visibility(second,'Sample Owner A 104');await second.getByRole('button',{name:'Close details',exact:true}).click()
  // The separate personal grant changes even a staff actor's scope identity.
  // Start the human review under that current identity before native reads.
  await finance.reload();await expect.poll(()=>names(finance)).toContain('society_find_records')
  await homeFinance(finance);await prepareVisibility(finance,'Sample Owner A 101')
  const note=await finance.getByLabel('Financial access verification note',{exact:true}).inputValue(),writes:string[]=[]
  const observer=(r:{method:()=>string;url:()=>string})=>{if(!['GET','HEAD'].includes(r.method()))writes.push(r.url())}
  finance.on('request',observer)
  const records=JSON.parse(await execute(finance,'society_find_records',{}));expect(records.total).toBe(0)
  await expect(execute(finance,'society_open_workspace',{screen:'overview'})).rejects.toThrow()
  await expect(execute(finance,'society_find_records',{action:'GRANT'})).rejects.toThrow()
  await expect(finance.getByLabel('Financial access verification note',{exact:true})).toHaveValue(note)
  expect(writes).toEqual([]);finance.off('request',observer)
  await finance.setViewportSize({width:320,height:440});await finance.getByRole('button',{name:'Allow household view',exact:true}).scrollIntoViewIfNeeded();await capture(finance,'native-kept-financial-form-320-440')
  await finance.getByRole('button',{name:'Allow household view',exact:true}).click();await expect(finance.getByRole('status').filter({hasText:'is recorded. Current household access is refreshed below.'})).toBeVisible()
  await homeFinance(finance,'A-102');await visibility(finance,'Sample Owner A 101');await finance.getByRole('button',{name:'Close details',exact:true}).click()
  for(const [source,amount] of [['a-101','432.19'],['a-102','125.01'],['a-103','580.03']]){
    const draft=await postAPI(finance,'/api/entries',{operation_key:crypto.randomUUID(),flat_id:entity('HOME',source),kind:'RECEIVED',amount,date:'2026-10-09',description:'PRIVATE supplied native household receipt '+source,payer:'Supplied fictional payer',method:'BANK_TRANSFER',reference:'NATIVE-REHEARSAL-'+source})
    await postAPI(finance,'/api/entries/'+draft.id+'/post',{operation_key:crypto.randomUUID(),confirmed:true,reason:''})
  }
  await owner.reload();await expect.poll(()=>names(owner)).toContain('society_find_records')
  const own=JSON.parse(await execute(owner,'society_find_records',{}));expect(own.total).toBe(2);expect(own.credit_paise).toBe(55720)
  expect(own.items.map((x:{flat_id:string})=>x.flat_id).sort()).toEqual([entity('HOME','a-101'),entity('HOME','a-102')].sort())
  const denied=JSON.parse(await execute(owner,'society_find_records',{home_id:entity('HOME','a-103')}));expect(denied.total).toBe(0);expect(denied.items).toEqual([])
  expect(await names(actors.tenant.page)).not.toContain('society_find_records')
})

test('a held actual native financial response is discarded after a visible revocation while another home keeps read permission true',async()=>{
  const owner=actors.owner.page,finance=actors.finance.page,home=entity('HOME','a-101')
  const earlier=await(await owner.request.get('/api/auth/me')).json();expect(earlier.can_read_records).toBe(true)
  let release!:()=>void,entered!:()=>void,calls=0
  const held=new Promise<void>(resolve=>release=resolve),ready=new Promise<void>(resolve=>entered=resolve)
  await owner.route('**/api/entries?**',async route=>{if(++calls>1){await route.continue();return}const result=await route.fetch();expect(result.status()).toBe(200);entered();await held;await route.fulfill({response:result}).catch(()=>{})})
  const reading=execute(owner,'society_find_records',{home_id:home});await ready
  try{
    await homeFinance(finance);await visibility(finance,'Sample Owner A 101','Withhold household view');await finance.getByRole('button',{name:'Close details',exact:true}).click()
    const current=await(await owner.request.get('/api/auth/me')).json();expect(current.can_read_records).toBe(true);expect(current.scope_key).not.toBe(earlier.scope_key)
    release();await expect(reading).rejects.toThrow();await owner.unroute('**/api/entries?**')
    await expect.poll(()=>names(owner)).toContain('society_find_records')
    const retained=JSON.parse(await execute(owner,'society_find_records',{}));expect(retained.total).toBe(1);expect(retained.credit_paise).toBe(12501);expect(retained.items[0].flat_id).toBe(entity('HOME','a-102'))
    const denied=JSON.parse(await execute(owner,'society_find_records',{home_id:home}));expect(denied.total).toBe(0)
    expect(readDB('SELECT (SELECT COUNT(*) FROM entries),(SELECT COUNT(*) FROM receipts)')[0]).toEqual([3,3])
    await navigate(owner,'Entries');await owner.setViewportSize({width:375,height:812});await capture(owner,'native-current-remaining-home-375')
  }finally{release();await owner.unroute('**/api/entries?**')}
})
