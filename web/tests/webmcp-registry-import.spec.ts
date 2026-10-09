import { test,expect } from '@playwright/test'
import { execute,names } from './native-webmcp-helpers'
import { confirmImport,db,loginWorkspace,previewRegister,snapshot,suppliedRegister } from './registry-import-fixtures'
import { navigate } from './helpers'

test.describe.configure({mode:'serial'})
test.use({actionTimeout:10000})
test.setTimeout(90000)
const tool='society_read_registry_import'
const privateKeys=['society_key','source_key','digest','input_text','verification_note','administrator_email','password','Sample Owner','Sample Tenant']
function metadataOnly(value:unknown){for(const key of privateKeys)expect(JSON.stringify(value)).not.toContain(key)}

test('actual native import metadata makes no writes and protects the visible review and loaded file before human apply',async({page})=>{
 await loginWorkspace(page);await expect.poll(()=>names(page)).toContain(tool)
 const writes:string[]=[];page.on('request',r=>{if(!['GET','HEAD'].includes(r.method()))writes.push(r.url())})
 const empty=JSON.parse(await execute(page,tool,{}));expect(empty).toEqual({initial_import_available:true,registry_counts:{buildings:0,flats:0,residents:0,memberships:0},applied:null});metadataOnly(empty)
 await expect(execute(page,tool,{input_text:suppliedRegister})).rejects.toThrow();await expect(execute(page,tool,{confirmed:true})).rejects.toThrow();await expect(execute(page,'society_bootstrap_workspace',{})).rejects.toThrow();await expect(execute(page,'society_apply_registry_import',{})).rejects.toThrow();expect(writes).toEqual([])
 await execute(page,'society_open_workspace',{screen:'registry-import'});await expect(page.getByRole('heading',{name:'A considered beginning.'})).toBeVisible();await previewRegister(page);writes.length=0
 await expect(execute(page,'society_open_workspace',{screen:'overview'})).rejects.toThrow();metadataOnly(JSON.parse(await execute(page,tool,{})));await expect(page.getByRole('heading',{name:'Look a little closer.'})).toBeVisible();expect(writes).toEqual([])
 await confirmImport(page);const marker='The native read preserves this unfinished human verification note.';await page.getByLabel('Verification note',{exact:true}).fill(marker);metadataOnly(JSON.parse(await execute(page,tool,{})));await expect(execute(page,'society_open_workspace',{screen:'registry-import'})).rejects.toThrow();await expect(page.getByLabel('Verification note',{exact:true})).toHaveValue(marker);await snapshot(page,'native-human-import-form-desktop');expect(writes).toEqual([])
 await page.getByRole('button',{name:'Apply this register',exact:true}).click();await expect(page.getByRole('heading',{name:'Your register is here.'})).toBeVisible();const result=JSON.parse(await execute(page,tool,{}));expect(result).toMatchObject({initial_import_available:false,registry_counts:{buildings:3,flats:118,residents:154,memberships:155},applied:{counts:{homes:118,occupied:109,vacant:9,owners:118,tenants:35}}});metadataOnly(result);expect(db("SELECT (SELECT COUNT(*) FROM registry_imports),(SELECT COUNT(*) FROM audit_events WHERE action='REGISTRY_IMPORTED')")[0]).toEqual([1,1])
})

test('actual native held reads retain immutable import metadata after home edits and discard revoked authority',async({page})=>{
 await loginWorkspace(page);await expect.poll(()=>names(page)).toContain(tool)
 let release=()=>{}
 for(const kind of ['registry','authority']){
  let ready!:()=>void,calls=0;const entered=new Promise<void>(resolve=>ready=resolve),held=new Promise<void>(resolve=>release=resolve)
  await page.route('**/api/registry/import',async route=>{if(++calls>1){await route.continue();return}const response=await route.fetch();ready();await held;await route.fulfill({response}).catch(()=>{})})
  const read=execute(page,tool,{});await entered
  if(kind==='registry')db("UPDATE flats SET version=version+1 WHERE id=(SELECT entity_id FROM registry_import_entities WHERE entity_kind='HOME' AND source_id='a-101')")
  else db("UPDATE role_grants SET revoked_at=strftime('%s','now') WHERE role='ADMINISTRATOR'")
  release()
  if(kind==='registry'){
   // Retained import metadata intentionally stays immutable after a reviewed
   // home edit. A read of that unchanged import is valid, with no live names.
   metadataOnly(JSON.parse(await read))
  }else await expect(read).rejects.toThrow()
  await page.unroute('**/api/registry/import')
 }
 await expect.poll(()=>names(page)).not.toContain(tool);expect((await page.request.get('/api/registry/import')).status()).toBe(403);await expect(execute(page,'society_open_workspace',{screen:'registry-import'})).rejects.toThrow()
 db("UPDATE role_grants SET revoked_at=NULL WHERE role='ADMINISTRATOR'")
})

test('a normally invited current tenant has no native import tools or finance access and reads only A-103',async({page,browser})=>{
 await loginWorkspace(page)
 const me=await (await page.request.get('/api/auth/me')).json(),origin=new URL(page.url()).origin,headers={Origin:origin,'X-CSRF-Token':me.csrf_token}
 const person=db("SELECT entity_id FROM registry_import_entities WHERE entity_kind='PERSON' AND source_id='tenant-a-103'")[0][0]
 const invitation=await page.request.post('/api/admin/invitations',{headers,data:{resident_id:person,email:'native-tenant@example.test',role:'RESIDENT',term_days:90,identity_verified:true,note:'Verified the fictional current A-103 tenancy against the supplied register.'}});expect(invitation.status()).toBe(201);const link=await invitation.json()
 const context=await browser.newContext({baseURL:origin,serviceWorkers:'block'}),resident=await context.newPage()
 try{
  expect((await resident.request.post('/api/auth/link/complete',{headers:{Origin:origin},data:{token:link.token,password:'Fictional-native-tenant-password!'}})).status()).toBe(200)
  await resident.goto('/');await expect(resident.getByRole('button',{name:'Sign in',exact:true})).toBeEnabled();await resident.getByLabel('Email address',{exact:true}).fill('native-tenant@example.test');await resident.getByLabel('Password',{exact:true}).fill('Fictional-native-tenant-password!');await resident.getByRole('button',{name:'Sign in',exact:true}).click();await expect(resident.getByRole('button',{name:'Sign out',exact:true})).toBeVisible();await expect.poll(()=>names(resident)).toContain('society_find_homes');expect(await names(resident)).not.toContain(tool)
  const homes=JSON.parse(await execute(resident,'society_find_homes',{}));expect(homes.total).toBe(1);expect(homes.items[0]).toMatchObject({building_code:'A',number:'103'});await expect(execute(resident,'society_open_workspace',{screen:'registry-import'})).rejects.toThrow();expect((await resident.request.get('/api/registry/import')).status()).toBe(403);expect((await resident.request.get('/api/entries')).status()).toBe(403)
  await navigate(resident,'Your homes');await snapshot(resident,'native-current-imported-household-desktop');const signed=await (await resident.request.get('/api/auth/me')).json();expect((await resident.request.post('/api/auth/logout',{headers:{Origin:origin,'X-CSRF-Token':signed.csrf_token},data:{}})).status()).toBe(200);await expect(execute(resident,'society_find_homes',{})).rejects.toThrow();await expect.poll(()=>names(resident)).toEqual([])
 }finally{await context.close()}
})
