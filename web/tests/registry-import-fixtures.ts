import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { createHmac } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { mkdirSync, readFileSync } from 'node:fs'
import { join,resolve } from 'node:path'
import { navigate } from './helpers'

export const suppliedRegister=readFileSync(resolve('../testdata/registry-import-118.json'),'utf8')
export const setupPassword='Fictional-setup-password-2026!'
let recoveryCodes:string[]=[]
function code(secret:string) {
 const alphabet='ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';let bits='';for(const letter of secret)bits+=alphabet.indexOf(letter).toString(2).padStart(5,'0')
 const bytes=Buffer.from(bits.match(/.{8}/g)!.map(x=>parseInt(x,2))),counter=Buffer.alloc(8);counter.writeBigUInt64BE(BigInt(Math.floor(Date.now()/30000)))
 const mac=createHmac('sha1',bytes).update(counter).digest(),offset=mac.at(-1)!&15,value=mac.readUInt32BE(offset)&0x7fffffff
 return String(value%1000000).padStart(6,'0')
}
export async function loginWorkspace(page:Page) {
 await page.goto('/')
 await expect(page.getByRole('heading',{name:'Welcome back.'})).toBeVisible()
 await expect(page.getByRole('button',{name:'Sign in',exact:true})).toBeEnabled()
 await expect(page.locator('.demo-accounts')).toHaveCount(0)
 await expect(page.getByLabel('Email address',{exact:true})).toHaveValue('')
 await expect(page.getByLabel('Password',{exact:true})).toHaveValue('')
 await page.getByLabel('Email address',{exact:true}).fill('registry@example.test');await page.getByLabel('Password',{exact:true}).fill(setupPassword)
 await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await expect(page.getByRole('button',{name:'Verify and continue',exact:true})).toBeVisible()
 const me=await (await page.request.get('/api/auth/me')).json()
 expect(me.is_demo).toBe(false);expect(me.mfa_pending).toBe(true);expect(me.can_manage_registry).toBe(false);expect(me.can_manage_records).toBe(false)
 await expect(page.getByRole('button',{name:'Use a preview code',exact:true})).toHaveCount(0)
 if(!me.mfa_enrolled){
  await page.getByText('Enter a setup key instead',{exact:true}).click();await expect(page.locator('.setup-key')).not.toBeEmpty();const secret=await page.locator('.setup-key').innerText()
  await page.getByLabel('Authenticator code',{exact:true}).fill(code(secret));await page.getByRole('button',{name:'Verify and continue',exact:true}).click()
  await expect(page.getByRole('heading',{name:'Keep these close.'})).toBeVisible();recoveryCodes=await page.locator('.recovery-grid code').allTextContents()
  await page.getByRole('checkbox',{name:'I have saved my recovery codes privately.'}).check();await page.getByRole('button',{name:'Continue to workspace',exact:true}).click()
 }else{
  const recovery=recoveryCodes.shift();if(!recovery)throw new Error('No retained fictional recovery code for this isolated suite.')
  await page.getByRole('button',{name:'Use a recovery code',exact:true}).click();await page.getByLabel('Recovery code',{exact:true}).fill(recovery);await page.getByRole('button',{name:'Verify and continue',exact:true}).click()
 }
 await expect(page.getByRole('button',{name:'Sign out',exact:true})).toBeVisible();const user=await (await page.request.get('/api/auth/me')).json()
 expect(user.can_manage_registry).toBe(true);expect(user.can_read_records).toBe(false);expect(user.can_manage_records).toBe(false)
 await expect(page.getByRole('link',{name:'Entries',exact:true})).toHaveCount(0)
 await navigate(page,'Homes & people');await page.getByRole('button',{name:'Initial register',exact:true}).click();await expect(page.getByRole('heading',{name:'A considered beginning.'})).toBeVisible()
 await expect(page.getByText('Opening your register…',{exact:true})).toHaveCount(0)
}
export async function chooseRegister(page:Page,text=suppliedRegister,name='supplied-rehearsal-register.json') {
 await page.getByLabel('Supplied register',{exact:true}).setInputFiles({name,mimeType:'application/json',buffer:Buffer.from(text)})
 await expect(page.getByRole('button',{name:'Validate & preview',exact:true})).toBeEnabled()
}
export async function previewRegister(page:Page,text=suppliedRegister) {
 await chooseRegister(page,text);await page.getByRole('button',{name:'Validate & preview',exact:true}).click();await expect(page.getByRole('heading',{name:'Look a little closer.'})).toBeVisible()
}
export function db(query:string,parameters:unknown[]=[]) {
 const path=process.env.SOCIETY_BROWSER_DB;if(!path||!path.includes('society-browser-'))throw new Error('Disposable synthetic browser database required.')
 return JSON.parse(execFileSync('python3',['-c','import sqlite3,json,sys\nc=sqlite3.connect(sys.argv[1]);r=c.execute(sys.argv[2],json.loads(sys.argv[3]));rows=r.fetchall();c.commit();print(json.dumps(rows));c.close()',path,query,JSON.stringify(parameters)],{encoding:'utf8'})) as unknown[][]
}
export async function snapshot(page:Page,name:string) {
 const folder=process.env.SOCIETY_BOOTSTRAP_SCREENSHOTS??join(process.env.SOCIETY_BROWSER_ARTIFACTS!,'registry-import-originals');mkdirSync(folder,{recursive:true,mode:0o700})
 await page.evaluate(()=>document.fonts.ready);await page.screenshot({path:join(folder,name+'.png'),fullPage:false})
}
export async function pageFits(page:Page) {expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)}
export async function dialogFits(page:Page) {
 const boxes=await page.locator('dialog[open]').evaluate(element=>{
  const close=element.querySelector('.dialog-close')!.getBoundingClientRect(),body=element.querySelector('.dialog-scroll')!.getBoundingClientRect(),dialog=element.getBoundingClientRect()
  return {bodyTop:body.top,closeBottom:close.bottom,left:dialog.left,right:dialog.right,top:dialog.top,bottom:dialog.bottom,width:innerWidth,height:innerHeight,padding:parseFloat(getComputedStyle(element.querySelector('.dialog-scroll')!).paddingLeft)}
 })
 expect(boxes.bodyTop).toBeGreaterThanOrEqual(boxes.closeBottom+10);expect(boxes.left).toBeGreaterThanOrEqual(10);expect(boxes.right).toBeLessThanOrEqual(boxes.width-10);expect(boxes.top).toBeGreaterThanOrEqual(10);expect(boxes.bottom).toBeLessThanOrEqual(boxes.height-10);expect(boxes.padding).toBeGreaterThanOrEqual(20)
 await pageFits(page)
}
export async function confirmImport(page:Page) {
 await page.getByRole('button',{name:'Review & apply',exact:true}).click();await expect(page.getByRole('dialog')).toBeVisible()
 await page.getByLabel('Verification note',{exact:true}).fill('Verified supplied fictional homes and source relationships against the register.')
 await page.getByRole('checkbox',{name:'I checked this register and its source identities.'}).check()
}
