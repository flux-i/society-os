import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { financialHeaders } from './maintenance-fixtures'

export const careDate=(offset=0)=>new Intl.DateTimeFormat('en-CA',{year:'numeric',month:'2-digit',day:'2-digit',timeZone:'Asia/Kolkata'}).format(new Date(Date.now()+offset*86400000))
export async function apiWork(page:Page,title:string,extra:Record<string,unknown>={}){
 const response=await page.request.post('/api/upkeep/tasks',{headers:await financialHeaders(page),data:{operation_key:crypto.randomUUID(),title,body:'PRIVATE internal fictional work instructions',category:'WATER',priority:'HIGH',due_date:careDate(1),visit_date:careDate(1),repeat_days:30,confirmed:true,...extra}})
 expect(response.status(),await response.text()).toBe(200);return (await response.json()).id as string
}
export async function apiWorkAction(page:Page,id:string,action:string,extra:Record<string,unknown>={}){
 const read=await page.request.get('/api/upkeep/tasks/'+id);expect(read.status(),await read.text()).toBe(200);const item=await read.json()
 const response=await page.request.post('/api/upkeep/tasks/'+id+'/actions',{headers:await financialHeaders(page),data:{operation_key:crypto.randomUUID(),version:item.version,action,reason:'PRIVATE action reason checked fictional work evidence',confirmed:true,...extra}})
 expect(response.status(),await response.text()).toBe(200);return await(await page.request.get('/api/upkeep/tasks/'+id)).json()
}
export async function apiCareRecord(page:Page,kind:string,name:string,extra:Record<string,unknown>={}){
 const response=await page.request.post('/api/upkeep/register/'+kind,{headers:await financialHeaders(page),data:{operation_key:crypto.randomUUID(),version:0,name,category:'Water',source_reference:'PRIVATE fictional source contract',state:'ACTIVE',reason:'Checked fictional supplier and register evidence',confirmed:true,...(kind==='ASSET'?{location:'PRIVATE pump room'}:{contact:'PRIVATE supplier contact',phone:'+91 98765 43210',email:'fictional@example.test'}),...extra}})
 expect(response.status(),await response.text()).toBe(200);return (await response.json()).id as string
}
