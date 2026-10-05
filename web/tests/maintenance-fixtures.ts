import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'

export async function financialHeaders(page:Page) {
  const me=await(await page.request.get('/api/auth/me')).json()
  return {Origin:new URL(page.url()).origin,'X-CSRF-Token':me.csrf_token}
}
export async function ensureMaintenanceReviewer(page:Page) {
  const details=await(await page.request.get('/api/admin/accounts/demo-user-committee')).json()
  if(details.grants.some((grant:{role:string;state:string})=>grant.role==='TREASURER'&&grant.state==='ACTIVE'))return
  const response=await page.request.post('/api/admin/accounts/demo-user-committee/roles',{headers:await financialHeaders(page),data:{version:details.version,confirmed:true,reason:'Verified fictional separate treasury reviewer for disposable maintenance checks',role:'TREASURER',term_days:30}})
  expect(response.status(),await response.text()).toBe(201)
}
export async function apiMaintenance(page:Page,title:string,lines=[{flat_id:'demo-flat-A-101',amount:'1000.00'},{flat_id:'demo-flat-A-102',amount:'750.25'}],due='2026-01-10') {
  const response=await page.request.post('/api/maintenance',{headers:await financialHeaders(page),data:{operation_key:crypto.randomUUID(),title,period_start:'2026-01-01',period_end:'2026-01-31',due_date:due,source_reference:'PRIVATE-SOURCE '+title,note:'PRIVATE supporting maintenance note',lines,confirmed:true}})
  expect(response.status(),await response.text()).toBe(200)
  return (await response.json()).id as string
}
export async function apiPublish(page:Page,id:string) {
  const response=await page.request.post('/api/maintenance/'+id+'/decision',{headers:await financialHeaders(page),data:{operation_key:crypto.randomUUID(),version:1,decision:'PUBLISHED',reason:'Separately reviewed each fictional supplied maintenance amount',confirmed:true}})
  expect(response.status(),await response.text()).toBe(200)
}
export async function apiReceived(page:Page,amount:string) {
  const headers=await financialHeaders(page)
  const created=await page.request.post('/api/entries',{headers,data:{operation_key:crypto.randomUUID(),flat_id:'demo-flat-A-101',kind:'RECEIVED',amount,date:'2026-01-01',description:'Fictional maintenance receipt already received',payer:'Demo Owner A-101',method:'BANK_TRANSFER',reference:'MAINT-QA-'+crypto.randomUUID()}})
  expect(created.status(),await created.text()).toBe(200)
  const id=(await created.json()).id as string
  expect((await page.request.post('/api/entries/'+id+'/post',{headers,data:{operation_key:crypto.randomUUID(),confirmed:true,reason:''}})).status()).toBe(200)
  return await(await page.request.get('/api/entries/'+id)).json()
}
