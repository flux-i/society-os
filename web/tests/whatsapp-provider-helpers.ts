import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { resolve,join } from 'node:path'
import { chooseOption,navigate } from './helpers'

export type ProviderSend={id:string;request:{messaging_product:string;recipient_type:string;to:string;type:string;template:{name:string;language:{code:string};components:{type:string;parameters:{type:string;text:string}[]}[]};biz_opaque_callback_data:string;messaging_account_id:string};authorization_valid:boolean;mode:string}
export async function providerControl(page:Page,data:Record<string,unknown>) {
  const origin=process.env.SOCIETY_WHATSAPP_FIXTURE
  if(!origin?.startsWith('http://127.0.0.1:'))throw new Error('An isolated numeric-loopback provider fixture is required.')
  const response=await page.request.post(origin+'/_control',{data});expect(response.status()).toBe(200);return response.json()
}
export async function providerStatus(page:Page):Promise<{send_count:number;sends:ProviderSend[]}> {
  const response=await page.request.get(process.env.SOCIETY_WHATSAPP_FIXTURE+'/_status');expect(response.status()).toBe(200);return response.json()
}
export async function providerProof(page:Page,id:string,state:string,bad_signature=false) {
  const response=await page.request.post(process.env.SOCIETY_WHATSAPP_FIXTURE+'/_proof',{data:{id,state,bad_signature}});expect(response.status()).toBe(200);return (await response.json()).callback_status as number
}
export async function providerRefresh(page:Page,id:string,state:'ACCEPTED'|'DELIVERED'|'READ') {
  const response=page.waitForResponse(value=>value.request().method()==='GET'&&new URL(value.url()).pathname==='/api/messages/'+id)
  await page.getByRole('button',{name:'Refresh delivery status',exact:true}).click()
  const current=await response;expect(current.status()).toBe(200);expect((await current.json()).outcomes[state]).toBe(1)
  const label={ACCEPTED:'Provider accepted',DELIVERED:'Delivery reported',READ:'Read reported'}[state]
  await expect(page.getByRole('region',{name:'Reported delivery outcomes'})).toContainText(label)
  await expect(page.getByText('Opening the current record…',{exact:true})).toHaveCount(0)
}
export async function providerCapture(page:Page,name:string) {
  if(process.env.SOCIETY_CAPTURE_UI!=='1')return
  const directory=resolve('../reports/local/whatsapp-provider-ui');mkdirSync(directory,{recursive:true,mode:0o700})
  await page.evaluate(()=>document.fonts.ready)
  await page.evaluate(async()=>{await Promise.all(document.getAnimations().filter(animation=>Number.isFinite(Number(animation.effect?.getComputedTiming().iterations))).map(animation=>animation.finished.catch(()=>{})))})
  // Full-page capture changes Chromium's viewport while a native modal is
  // open, dismissing Radix popovers and producing misleading tall-page shots.
  // Capture the actual viewport for modal/menu QA without altering controls.
  await page.screenshot({path:join(directory,name+'.png'),fullPage:await page.getByRole('dialog').count()===0})
}
export async function providerWithin(page:Page) {
  const dimensions=await page.evaluate(()=>({viewport:innerWidth,width:document.documentElement.scrollWidth,dialog:(document.querySelector('[role="dialog"]') as HTMLElement)?.getBoundingClientRect().toJSON()}))
  expect(dimensions.width).toBeLessThanOrEqual(dimensions.viewport+1)
  if(dimensions.dialog){expect(dimensions.dialog.x).toBeGreaterThanOrEqual(0);expect(dimensions.dialog.right).toBeLessThanOrEqual(dimensions.viewport+1)}
}
export async function prepareProviderUI(page:Page,title:string) {
  await navigate(page,'Messages');await page.getByRole('button',{name:'Prepare a message',exact:true}).click()
  await chooseOption(page,'Delivery channel','WhatsApp · provider test')
  await page.locator('.message-source-option').filter({hasText:title}).click()
  await chooseOption(page,'Recipient group','Owners')
  await page.getByRole('textbox',{name:'Proposal reason',exact:true}).fill('Reviewed this exact provider template, original link and two fictional permitted owners.')
}
export async function providerAction(page:Page,button:string) {
  const control=page.getByRole('button',{name:button,exact:true});await control.scrollIntoViewIfNeeded();await control.click()
  await page.getByRole('textbox',{name:'Decision reason',exact:true}).fill('Reviewed this exact frozen proposal, current permissions and the deliberately requested decision.')
  await page.getByRole('checkbox',{name:'I reviewed the current content, recipients, permissions and this decision’s effect.',exact:true}).check()
}
