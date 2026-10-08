import { expect } from '@playwright/test'
import type { Browser, Page } from '@playwright/test'
import { mkdirSync, chmodSync } from 'node:fs'
import { resolve } from 'node:path'
import { login, chooseOption } from './helpers'
import { apiMaintenance, apiPublish, apiReceived, ensureMaintenanceReviewer, financialHeaders } from './maintenance-fixtures'
import { messageSources, messagePost } from './messages-fixtures'
import type { MessageDetail } from '../src/messages'

export const reminderPreviewCheck='I reviewed this exact content, eligible recipients, unique destinations and omissions.'
export const reminderReviewCheck='I reviewed the current content, recipients, permissions and this decision’s effect.'
export async function reminderActors(page:Page,browser:Browser) {
  await page.setViewportSize({width:1440,height:1000});await login(page);await ensureMaintenanceReviewer(page)
  const context=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await context.newPage();await login(reviewer,'Committee')
  const notice=await messageSources(page,reviewer,'RMD fixture notice '+crypto.randomUUID())
  return {reviewer,notice,close:()=>context.close()}
}
export async function reminderMaintenance(page:Page,reviewer:Page,title:string,lines=[{flat_id:'demo-flat-A-101',amount:'1000.00'},{flat_id:'demo-flat-A-102',amount:'750.25'}]) {
  const id=await apiMaintenance(page,title,lines);await apiPublish(reviewer,id)
  const response=await page.request.get('/api/maintenance/'+id);expect(response.status(),await response.text()).toBe(200)
  return await response.json() as {id:string;lines:{flat_id:string;entry_id:string}[]}
}
export async function reminderAllocate(page:Page,source:string,charge:string,amount:string) {return messagePost(page,'/api/allocations',{source_id:source,charge_id:charge,amount,reason:'RMD independently checked the original credit and exact charge allocation.'})}
export async function reminderPayment(page:Page,charge:string,received:string,allocated:string) {const receipt=await apiReceived(page,received);await reminderAllocate(page,receipt.id,charge,allocated);return receipt}
export async function reminderMessage(page:Page,id:string) {const response=await page.request.get('/api/messages/'+id);expect(response.status(),await response.text()).toBe(200);return await response.json() as MessageDetail}
export async function reminderProposal(page:Page,kind:string,source:string,target={kind:'PEOPLE',wing:'',ids:['demo-owner-A-101']},basis='OUTSTANDING') {
  const input={source_kind:kind,source_id:source,channel:'EMAIL',target,reminder_basis:basis}
  const response=await page.request.post('/api/messages/preview',{headers:await financialHeaders(page),data:input});expect(response.status(),await response.text()).toBe(200);const preview=await response.json()
  return (await messagePost(page,'/api/messages',{...input,preview_hash:preview.preview_hash,reason:'RMD private exact source, current amount and selected people reviewed.'})).id as string
}
export async function reminderActionUI(page:Page,button:string,outcome='') {
  await page.getByRole('button',{name:button,exact:true}).click();if(outcome)await chooseOption(page,'Simulation outcome',outcome)
  await page.getByRole('textbox',{name:'Decision reason',exact:true}).fill('PRIVATE_RMD_UI reviewed the current exact source, recipients and deliberate decision.')
  await page.getByRole('checkbox',{name:reminderReviewCheck,exact:true}).check()
}
export async function reminderStartUI(page:Page,kind:string,title:string) {
  await page.goto('/#messages');await page.getByRole('button',{name:'Prepare a message',exact:true}).click();await chooseOption(page,'Message source',kind)
  await page.getByRole('textbox',{name:'Search published message sources',exact:true}).fill(title)
  await page.getByRole('button',{name:new RegExp('^(Outstanding reminder|Meeting acknowledgement) · '+title)}).click()
  await page.getByRole('textbox',{name:'Proposal reason',exact:true}).fill('PRIVATE_RMD_PROPOSAL exact supplied source, current criterion and intended homes reviewed.')
}
export async function reminderCapture(page:Page,name:string) {
  if(process.env.SOCIETY_CAPTURE_UI!=='1')return
  await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>Number.isFinite(Number(a.effect?.getComputedTiming().endTime))).map(a=>a.finished.catch(()=>{})))})
  const folder=resolve(process.env.SOCIETY_REMINDER_CAPTURE_ROOT??'../reports/local/reminders-ui');mkdirSync(folder,{recursive:true,mode:0o700});const path=resolve(folder,name+'.png');await page.screenshot({path,animations:'disabled'});chmodSync(path,0o600)
}
export async function reminderWithin(page:Page) {
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true)
  for(const role of ['dialog','listbox'] as const){const box=page.getByRole(role);if(await box.count()){const b=await box.boundingBox();expect(b!.x).toBeGreaterThanOrEqual(0);expect(b!.y).toBeGreaterThanOrEqual(0);expect(b!.x+b!.width).toBeLessThanOrEqual(page.viewportSize()!.width+1);expect(b!.y+b!.height).toBeLessThanOrEqual(page.viewportSize()!.height+1)}}
  if(await page.getByRole('dialog').count())await expect(page.getByRole('dialog').locator('.dialog-close')).toBeInViewport({ratio:1})
}
