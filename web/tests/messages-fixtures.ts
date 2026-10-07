import { expect } from '@playwright/test'
import type { Page, Browser } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { login } from './helpers'
import { financialHeaders } from './maintenance-fixtures'

export async function messagePost(page:Page,path:string,data:Record<string,unknown>) {
  const response=await page.request.post(path,{headers:await financialHeaders(page),data:{operation_key:crypto.randomUUID(),...(path==='/api/reviews'?{}:{confirmed:true}),...data}})
  expect(response.status(),await response.text()).toBe(200);return response.json()
}
export function messagePrivateFixture(script:string) {
  const db=process.env.SOCIETY_BROWSER_DB
  if(!db?.includes('society-browser-'))throw new Error('A disposable message database is required.')
  execFileSync('python3',['-c','import sqlite3,sys;db=sqlite3.connect(sys.argv[1]);db.execute("PRAGMA foreign_keys=ON");'+script+';db.commit()',db])
}
export async function messageActors(page:Page,browser:Browser) {
  await page.setViewportSize({width:1440,height:1000});await login(page)
  const context=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await context.newPage()
  await login(reviewer,'Committee')
  return {reviewer,close:()=>context.close()}
}
export async function messageSources(page:Page,reviewer:Page,title:string,audience='ALL_RESIDENTS') {
  messagePrivateFixture("db.execute(\"INSERT OR IGNORE INTO users(id,login,display_name,password_hash,status,resident_id,is_demo,verified_at,auth_version,created_at) SELECT 'message-joint-account','message-joint@example.test','Demo Joint Owner A-101',password_hash,'ACTIVE','demo-joint-owner',0,verified_at,1,created_at FROM users WHERE id='demo-user-owner'\")")
  for(const [id,email,phone] of [['demo-owner-A-101','shared-message@example.test','+919000000101'],['demo-joint-owner','shared-message@example.test','+919000000101'],['demo-tenant-A-103','tenant-message@example.test','+919000000103']]) {
    const current=await(await page.request.get('/api/contacts/'+id)).json()
    await messagePost(page,'/api/contacts/'+id+'/register',{version:current.version,phone,email,preferred_channel:'EMAIL',community_whatsapp:true,community_email:true,finance_whatsapp:true,finance_email:true,consent_source:'PRIVATE_MESSAGE_SOURCE supplied person, destination and separate permissions',reason:'PRIVATE_MESSAGE_REASON deliberately supplied this fictional communication choice.'})
    const pending=await(await reviewer.request.get('/api/contacts/'+id)).json()
    await messagePost(reviewer,'/api/contacts/'+id+'/actions',{version:pending.version,action:'VERIFIED',reason:'PRIVATE_MESSAGE_CHECK independently checked the person, destination and deliberate permission.'})
  }
  const notice=await messagePost(page,'/api/reviews',{kind:'NOTICE',title,body:'PRIVATE_MESSAGE_CONTENT fictional water service update, preserved only for the authorised source audience.',audience,building_code:''})
  await messagePost(reviewer,'/api/reviews/'+notice.id+'/decision',{version:1,decision:'APPROVED',reason:'Separately reviewed the fictional notice and intended audience.'})
  return notice.id as string
}
export async function messageProposal(page:Page,source:string,target={kind:'ALL',wing:'',ids:[] as string[]},kind='NOTICE',channel='EMAIL') {
  const input={source_kind:kind,source_id:source,channel,target}
  const response=await page.request.post('/api/messages/preview',{headers:await financialHeaders(page),data:input});expect(response.status(),await response.text()).toBe(200)
  const preview=await response.json()
  const result=await messagePost(page,'/api/messages',{...input,preview_hash:preview.preview_hash,reason:'PRIVATE_MESSAGE_PROPOSAL reviewed the exact content, recipient intersection and omissions.'})
  return result.id as string
}
export async function messageApprove(reviewer:Page,id:string) {
  const current=await(await reviewer.request.get('/api/messages/'+id)).json()
  await messagePost(reviewer,'/api/messages/'+id+'/actions',{version:current.version,action:'APPROVED',reason:'PRIVATE_MESSAGE_APPROVAL independently reviewed the frozen content and recipient decisions.'})
}
export async function openMessage(page:Page,id:string) {
  const target=new URL('/#messages?message='+id,page.url()).href
  if(page.url()===target)await page.reload();else await page.goto(target)
  await expect(page.getByRole('dialog').locator('.contact-state').first()).toBeVisible()
}
