import { test,expect } from '@playwright/test'
import { chooseRegister,confirmImport,db,dialogFits,loginWorkspace,pageFits,previewRegister,snapshot,suppliedRegister } from './registry-import-fixtures'
import { navigate } from './helpers'

test.describe.configure({mode:'serial'})
test.use({actionTimeout:10000})
test.setTimeout(120000)

test('non-demo real MFA, read-only preview, responsive menus, keyboard and internally scrolling reviews',async({page})=>{
 await loginWorkspace(page)
 const user=await (await page.request.get('/api/auth/me')).json()
 expect((await page.request.post('/api/auth/mfa/demo-code',{headers:{Origin:new URL(page.url()).origin,'X-CSRF-Token':user.csrf_token},data:{}})).status()).toBe(403)
 expect(db('SELECT COUNT(*) FROM users WHERE is_demo=1')[0][0]).toBe(0)
 await snapshot(page,'setup-empty-desktop')
 const download=page.waitForEvent('download');await page.getByRole('button',{name:'Download a register template',exact:true}).click();expect((await download).suggestedFilename()).toBe('society-register-template.json')
 await previewRegister(page)
 await expect(page.getByRole('definition')).toHaveText(['118','109','9','118','35'])
 expect(db('SELECT (SELECT COUNT(*) FROM flats),(SELECT COUNT(*) FROM registry_imports),(SELECT COUNT(*) FROM account_tokens)')[0]).toEqual([0,0,0])
 const sizes=[{name:'desktop',width:1440,height:1000},{name:'tablet',width:820,height:1050},{name:'phone375',width:375,height:812},{name:'phone320',width:320,height:740},{name:'short320',width:320,height:440}]
 for(const size of sizes){
  await page.setViewportSize(size);await page.getByRole('heading',{name:'Look a little closer.'}).scrollIntoViewIfNeeded();await pageFits(page);await snapshot(page,'preview-'+size.name)
  const select=page.getByRole('combobox',{name:'Proposed occupancy',exact:true});
  const search=page.getByRole('textbox',{name:'Search proposed homes or people',exact:true});await search.scrollIntoViewIfNeeded()
  const geometry=await page.locator('.import-filters').evaluate(element=>{const search=element.querySelector('.search-control')!,icon=search.querySelector('svg')!.getBoundingClientRect(),input=search.querySelector('input')!.getBoundingClientRect(),field=search.getBoundingClientRect(),trigger=element.querySelector('.filter-select')!.getBoundingClientRect(),container=element.getBoundingClientRect();return {field:{x:field.x,right:field.right,width:field.width,height:field.height},input:{x:input.x,right:input.right,y:input.y,height:input.height},icon:{right:icon.right,y:icon.y,height:icon.height},trigger:{x:trigger.x,right:trigger.right,width:trigger.width,height:trigger.height},container:{x:container.x,right:container.right,width:container.width},border:getComputedStyle(search).borderTopStyle}})
  expect(geometry.border).toBe('solid');expect(geometry.field.height).toBeGreaterThanOrEqual(44);expect(geometry.trigger.height).toBeGreaterThanOrEqual(44);expect(geometry.input.x).toBeGreaterThan(geometry.icon.right);expect(geometry.input.right).toBeLessThan(geometry.field.right);expect(Math.abs((geometry.icon.y+geometry.icon.height/2)-(geometry.input.y+geometry.input.height/2))).toBeLessThanOrEqual(2)
  if(size.width<=600){expect(geometry.field.x).toBe(geometry.container.x);expect(geometry.trigger.x).toBe(geometry.container.x);expect(geometry.field.width).toBe(geometry.container.width);expect(geometry.trigger.width).toBe(geometry.container.width)}
  await select.click();await expect(page.getByRole('option',{name:size.name==='desktop'?'All occupancy':'Rented',exact:true})).toHaveAttribute('aria-selected','true');await page.getByRole('option',{name:'Rented',exact:true}).hover();await snapshot(page,'occupancy-open-'+size.name)
  await page.keyboard.press('End');await expect(page.getByRole('option',{name:'Vacant',exact:true})).toBeFocused();await page.keyboard.press('ArrowUp');await expect(page.getByRole('option',{name:'Rented',exact:true})).toBeFocused();await page.keyboard.press('Enter');await expect(select).toContainText('Rented');await expect(page.getByText('35 proposed homes',{exact:true})).toBeVisible()
  await page.getByRole('button',{name:'Review proposed home A-103',exact:true}).click();await dialogFits(page);await expect(page.getByRole('dialog')).toContainText('Sample Former Tenant A 103');await snapshot(page,'proposed-home-'+size.name)
  await page.locator('dialog .dialog-scroll').evaluate(el=>el.scrollTo(0,el.scrollHeight));await dialogFits(page);await page.getByRole('button',{name:'Close proposed home',exact:true}).focus();await snapshot(page,'proposed-home-scrolled-'+size.name);await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);await expect(page.getByRole('button',{name:'Review proposed home A-103',exact:true})).toBeFocused()
  await page.getByRole('button',{name:'Review & apply',exact:true}).click();await dialogFits(page);await page.getByLabel('Verification note',{exact:true}).fill('Retained fictional verification note before any application.');await page.getByRole('checkbox',{name:'I checked this register and its source identities.'}).check();await snapshot(page,'confirmation-'+size.name)
  await page.locator('dialog .dialog-scroll').evaluate(el=>el.scrollTo(0,el.scrollHeight));await dialogFits(page);await page.getByRole('button',{name:'Apply this register',exact:true}).focus();await snapshot(page,'confirmation-scrolled-'+size.name);await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0)
 }
 await page.getByRole('textbox',{name:'Search proposed homes or people',exact:true}).fill('No such fictional home');await expect(page.getByRole('heading',{name:'No proposed homes match.'})).toBeVisible();await snapshot(page,'preview-empty-filter-320');await page.getByRole('button',{name:'Clear preview filters',exact:true}).click();await expect(page.getByText('118 proposed homes',{exact:true})).toBeVisible();await page.getByRole('button',{name:'Next proposed homes',exact:true}).click();await expect(page.getByText('Page 2 of 10',{exact:true})).toBeVisible();await page.getByRole('button',{name:'Previous proposed homes',exact:true}).click()
 expect(db('SELECT COUNT(*) FROM flats')[0][0]).toBe(0)
})

test('invalid rows, oversize and private fields stay uncommitted; loading/error/retry and stale confirmation are meaningful',async({page})=>{
 await loginWorkspace(page)
 const invalid=JSON.parse(suppliedRegister);invalid.relationships[0].start_date='2025-02-30';invalid.homes[0].building='missing'
 await previewRegister(page,JSON.stringify(invalid));await expect(page.getByRole('region',{name:'Check these rows'})).toContainText('relationships · row 1 · start_date');await expect(page.getByRole('region',{name:'Check these rows'})).toContainText('homes · row 1 · building');await expect(page.getByRole('button',{name:'Review & apply',exact:true})).toHaveCount(0);await snapshot(page,'validation-errors-desktop')
 await chooseRegister(page,'{"account_password":"do-not-echo-private-field"}');await page.getByRole('button',{name:'Validate & preview',exact:true}).click();await expect(page.getByRole('region',{name:'Check these rows'})).toBeVisible();await expect(page.getByRole('region',{name:'Check these rows'})).not.toContainText('do-not-echo-private-field')
 await page.getByLabel('Supplied register',{exact:true}).setInputFiles({name:'oversized.json',mimeType:'application/json',buffer:Buffer.alloc(2*1024*1024+1,120)});await expect(page.getByRole('alert')).toContainText('at most 2 MiB');expect(db('SELECT COUNT(*) FROM flats')[0][0]).toBe(0)
 let release!:()=>void;const held=new Promise<void>(resolve=>release=resolve)
 await page.route('**/api/registry/import/preview',async route=>{await held;await route.fulfill({status:503,contentType:'application/json',body:'{"error":"unavailable"}'})})
 await chooseRegister(page);await page.getByRole('button',{name:'Validate & preview',exact:true}).click();await expect(page.getByRole('button',{name:'Checking your register…',exact:true})).toBeDisabled();await snapshot(page,'preview-loading-desktop');release();await expect(page.getByRole('alert')).toContainText('could not be reached');await page.unroute('**/api/registry/import/preview');await page.getByRole('button',{name:'Validate & preview',exact:true}).click();await expect(page.getByRole('heading',{name:'Look a little closer.'})).toBeVisible()
 await confirmImport(page);db("INSERT INTO buildings VALUES('synthetic-concurrent','Z','Concurrent reviewed building')")
 await page.getByRole('button',{name:'Apply this register',exact:true}).click();await expect(page.getByRole('dialog')).toContainText('register changed after this preview');await expect(page.getByLabel('Verification note',{exact:true})).toHaveValue('Verified supplied fictional homes and source relationships against the register.');await snapshot(page,'stale-confirmation-desktop')
 await page.getByRole('button',{name:'Check saved result',exact:true}).click();await expect(page.getByRole('dialog')).toContainText('has not been recorded');await page.getByRole('button',{name:'Back to the preview',exact:true}).click();db("DELETE FROM buildings WHERE id='synthetic-concurrent'")
 expect(db('SELECT (SELECT COUNT(*) FROM flats),(SELECT COUNT(*) FROM registry_imports)')[0]).toEqual([0,0])
})

test('one deliberately applied register survives a lost reply and explicit retry; the result preserves one audit and source set',async({page})=>{
 await loginWorkspace(page);await previewRegister(page);await confirmImport(page)
 let writes=0,release!:()=>void;const held=new Promise<void>(resolve=>release=resolve)
 await page.route('**/api/registry/import/apply',async route=>{writes++;const result=await route.fetch();expect(result.status()).toBe(200);await held;await route.fulfill({response:result,body:'{'})})
 await page.getByRole('button',{name:'Apply this register',exact:true}).click();await expect(page.getByRole('button',{name:'Adding the register…',exact:true})).toBeDisabled();await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toBeVisible();await snapshot(page,'apply-pending-desktop')
 await expect.poll(()=>db('SELECT COUNT(*) FROM registry_imports')[0][0]).toBe(1);release();await expect(page.getByRole('dialog')).toContainText('connection was interrupted');await page.unroute('**/api/registry/import/apply');await expect(page.getByRole('button',{name:'Apply this register',exact:true})).toBeEnabled();expect(writes).toBe(1)
 // Deliberate retry keeps the exact operation identity held in the same form.
 await page.getByRole('button',{name:'Apply this register',exact:true}).click();await expect(page.getByRole('heading',{name:'Your register is here.'})).toBeVisible();await snapshot(page,'applied-register-desktop')
 expect(db("SELECT (SELECT COUNT(*) FROM flats),(SELECT COUNT(*) FROM residents),(SELECT COUNT(*) FROM flat_memberships),(SELECT COUNT(*) FROM registry_imports),(SELECT COUNT(*) FROM registry_import_entities),(SELECT COUNT(*) FROM audit_events WHERE action='REGISTRY_IMPORTED'),(SELECT COUNT(*) FROM flat_memberships WHERE can_view_finances=1)")[0]).toEqual([118,154,155,1,430,1,0])
 await page.reload();await expect(page.getByRole('heading',{name:'Your register is here.'})).toBeVisible();await page.setViewportSize({width:320,height:740});await pageFits(page);await snapshot(page,'applied-register-320');await page.getByRole('link',{name:'Explore your homes',exact:true}).click();await expect(page.getByRole('heading',{name:'Every home has a story.'})).toBeVisible();await expect(page.getByRole('region',{name:'Community at a glance'})).toContainText('118');await page.getByPlaceholder('Search a home or a person…').fill('Sample Owner A 101');await expect(page.getByText('2 homes found',{exact:true})).toBeVisible()
})

test('normal invitation and activation scope the supplied tenant to A-103 and deny import and finance access',async({page,browser})=>{
 await loginWorkspace(page);await navigate(page,'Access & invitations');await page.getByRole('button',{name:'Invite a person',exact:true}).click();await page.getByLabel('Find a person',{exact:true}).fill('Sample Tenant A 103');await page.getByRole('combobox',{name:'Person in the registry',exact:true}).click();await page.getByRole('option',{name:'Sample Tenant A 103',exact:true}).click();await page.getByRole('dialog').getByLabel('Email address',{exact:true}).fill('tenant@example.test');await page.getByRole('checkbox',{name:'I verified this person’s identity, email and home relationship.'}).check();await page.getByLabel('Verification note',{exact:true}).fill('Verified this fictional tenant against the imported current A-103 tenancy.');await page.getByRole('button',{name:'Create personal link',exact:true}).click();const link=await page.getByLabel('Personal link',{exact:true}).inputValue()
 const origin=new URL(page.url()).origin,context=await browser.newContext({baseURL:origin,serviceWorkers:'block'}),resident=await context.newPage()
 try{
  await resident.goto(link);await expect(resident.getByRole('heading',{name:'Your place is ready.'})).toBeVisible();await resident.getByLabel('New password',{exact:true}).fill('Fictional-tenant-password-2026!');await resident.getByLabel('Confirm new password',{exact:true}).fill('Fictional-tenant-password-2026!');await resident.getByRole('button',{name:'Save my password',exact:true}).click();await resident.getByRole('button',{name:'Continue to sign in',exact:true}).click();await resident.getByLabel('Email address',{exact:true}).fill('tenant@example.test');await resident.getByLabel('Password',{exact:true}).fill('Fictional-tenant-password-2026!');await resident.getByRole('button',{name:'Sign in',exact:true}).click();await navigate(resident,'Your homes');await expect(resident.getByText('1 homes found',{exact:true})).toBeVisible();await expect(resident.getByRole('button',{name:'Initial register',exact:true})).toHaveCount(0);await expect(resident.getByRole('link',{name:'Entries',exact:true})).toHaveCount(0)
  expect((await resident.request.get('/api/registry/import')).status()).toBe(403);expect((await resident.request.get('/api/registry/summary')).status()).toBe(403)
  await resident.getByRole('button',{name:'View home A-103',exact:true}).click();await expect(resident.getByRole('dialog')).toContainText('Sample Tenant A 103');await expect(resident.getByRole('dialog')).not.toContainText('Sample Former Tenant A 103');await snapshot(resident,'current-imported-tenant-desktop')
 }finally{await context.close()}
})


test('anonymous workspace configuration has loading and retry without exposing demo credentials or account data',async({browser})=>{
 const origin=process.env.SOCIETY_BROWSER_URL!,context=await browser.newContext({baseURL:origin,serviceWorkers:'block'}),page=await context.newPage()
 try{
  let release!:()=>void;const held=new Promise<void>(resolve=>release=resolve)
  await page.route('**/api/workspace',async route=>{await held;await route.fulfill({status:503,contentType:'application/json',body:'{"error":"unavailable"}'})})
  await page.goto('/');await expect(page.getByText('Checking your workspace…',{exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'Sign in',exact:true})).toBeDisabled();await expect(page.locator('.demo-accounts')).toHaveCount(0);await snapshot(page,'sign-in-configuration-loading');release();await expect(page.getByRole('button',{name:'Try workspace again',exact:true})).toBeVisible();await snapshot(page,'sign-in-configuration-error');await page.unroute('**/api/workspace');await page.getByRole('button',{name:'Try workspace again',exact:true}).click();await expect(page.getByRole('button',{name:'Sign in',exact:true})).toBeEnabled();await expect(page.getByLabel('Email address',{exact:true})).toHaveValue('');await expect(page.getByLabel('Password',{exact:true})).toHaveValue('');await page.getByRole('button',{name:'Show password',exact:true}).click();await expect(page.getByLabel('Password',{exact:true})).toHaveAttribute('type','text');await page.getByRole('button',{name:'Hide password',exact:true}).click();await page.setViewportSize({width:320,height:740});await pageFits(page);await snapshot(page,'non-demo-sign-in-320');const info=await (await page.request.get('/api/workspace')).json();expect(Object.keys(info).sort()).toEqual(['mode','name']);expect(info.mode).toBe('FICTIONAL_REHEARSAL')
 }finally{await context.close()}
})
