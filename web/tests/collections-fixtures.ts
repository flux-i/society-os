import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { financialHeaders } from './maintenance-fixtures'
import { careDate } from './upkeep-fixtures'
export async function apiFund(page:Page,title:string,extra:Record<string,unknown>={}){
 const response=await page.request.post('/api/collections',{headers:await financialHeaders(page),data:{operation_key:crypto.randomUUID(),title,purpose:'A reviewed fictional collection for shared garden and water work.',contribution_type:'FIXED',start_date:'2026-01-01',due_date:careDate(-1),source_reference:'PRIVATE supplied society resolution',note:'PRIVATE internal supporting note',lines:[{flat_id:'demo-flat-A-101',amount:'1000.00'},{flat_id:'demo-flat-A-102',amount:'500.25'}],confirmed:true,...extra}})
 expect(response.status(),await response.text()).toBe(200);return (await response.json()).id as string
}
export async function apiFundAction(page:Page,id:string,action:string){
 const previous=await(await page.request.get('/api/collections/'+id)).json()
 const response=await page.request.post('/api/collections/'+id+'/actions',{headers:await financialHeaders(page),data:{operation_key:crypto.randomUUID(),version:previous.version,action,reason:'PRIVATE separately reviewed the supplied fictional fund decision',confirmed:true}})
 expect(response.status(),await response.text()).toBe(200);return await(await page.request.get('/api/collections/'+id)).json()
}
export async function apiFundReport(page:Page,id:string,amount:string,reference:string,extra:Record<string,unknown>={}){
 const response=await page.request.post('/api/payment-reports',{headers:await financialHeaders(page),data:{operation_key:crypto.randomUUID(),campaign_id:id,flat_id:'demo-flat-A-101',amount,payment_date:'2026-01-01',payer:'PRIVATE fictional paid-by person',method:'BANK_TRANSFER',reference,comment:'PRIVATE supporting claim comment',evidence_id:'',version:0,confirmed:true,...extra}})
 expect(response.status(),await response.text()).toBe(200);return (await response.json()).id as string
}
export async function apiFundVerify(page:Page,id:string,identity:string,amount:string,extra:Record<string,unknown>={}){
 const previous=await(await page.request.get('/api/payment-reports/'+id)).json()
 const response=await page.request.post('/api/payment-reports/'+id+'/actions',{headers:await financialHeaders(page),data:{operation_key:crypto.randomUUID(),version:previous.version,action:'CONFIRMED',reason:'PRIVATE checked fictional external payment before confirmation',verification_source:'PRIVATE fictional bank source',payment_identity:identity,mode:'NEW',entry_id:'',allocation_amount:amount,confirmed:true,...extra}})
 expect(response.status(),await response.text()).toBe(200);return await(await page.request.get('/api/payment-reports/'+id)).json()
}
