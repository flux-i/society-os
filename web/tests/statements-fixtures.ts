import { expect } from '@playwright/test'
import type { Page,Browser } from '@playwright/test'
import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { login,navigate } from './helpers'
import { financialHeaders,ensureMaintenanceReviewer } from './maintenance-fixtures'
import { messagePost } from './messages-fixtures'

export const originalCSV=Buffer.from('Description,Amount\nFictional external income,432.19\nFictional expense,100.00\n')
export const uploadCheck='I checked this exact original, period, preparer and source. It stays private until separately reviewed and deliberately published.'
export const publicationCheck='I checked this exact original and audience. This proposes portal publication; sending a message follows its own permission and approval process.'
export const decisionCheck='I reviewed this exact original, the current permissions and this decision’s effect. Internal approval and publication remain separate decisions.'
export const publicationDecisionCheck='I reviewed this exact original, the current permissions and this decision’s chosen audience. Internal approval and publication remain separate decisions.'
export async function statementActors(page:Page,browser:Browser){
 await page.setViewportSize({width:1440,height:1000});await login(page);await ensureMaintenanceReviewer(page)
 const context=await browser.newContext({baseURL:new URL(page.url()).origin}),reviewer=await context.newPage();await login(reviewer,'Committee');return {reviewer,close:()=>context.close()}
}
export async function statementOriginal(page:Page,title:string,data:Buffer=originalCSV,filename='fictional-income.csv',replaces?:{id:string;version:number}){
 const inData={title,kind:'INCOME',period_start:'2026-09-01',period_end:'2026-09-30',prepared_by:'PRIVATE fictional statement preparer',source:'PRIVATE supplied external September worksheet',filename,size_bytes:data.length,sha256:createHash('sha256').update(data).digest('hex'),reason:'PRIVATE deliberate original source review',...(replaces?{replaces_id:replaces.id,version:replaces.version}:{})}
 const out=await messagePost(page,'/api/financial-statements',inData)
 const upload=await page.request.post('/api/financial-statements/'+out.id+'/content',{headers:{...await financialHeaders(page),'Content-Type':'application/octet-stream'},data});expect(upload.status(),await upload.text()).toBe(200)
 await expect.poll(async()=>{const x=await(await page.request.get('/api/financial-statements/'+out.id)).json();return x.validation}).not.toMatch(/^(PENDING|VALIDATING)$/)
 return out.id as string
}
export async function statementApprove(reviewer:Page,id:string){const x=await(await reviewer.request.get('/api/financial-statements/'+id)).json();expect(x.validation).toBe('AVAILABLE');await messagePost(reviewer,'/api/financial-statements/'+id+'/actions',{version:x.version,action:'APPROVED',reason:'PRIVATE separate review of this exact original and context'})}
export async function statementPublish(page:Page,reviewer:Page,id:string,kind='TENANTS'){
 const x=await(await page.request.get('/api/financial-statements/'+id)).json(),input={file_id:id,file_version:x.version,target:{kind,wing:'',ids:[]}}
 const preview=await page.request.post('/api/financial-statements/publication-preview',{headers:await financialHeaders(page),data:input});expect(preview.status(),await preview.text()).toBe(200)
 const p=await preview.json(),out=await messagePost(page,'/api/financial-statements/publications',{...input,preview_hash:p.preview_hash,reason:'PRIVATE deliberately shared original and audience'});await messagePost(reviewer,'/api/financial-statements/publications/'+out.id+'/actions',{version:1,action:'PUBLISHED',reason:'PRIVATE separate approval of this exact original and audience'});return out.id as string
}
export async function openStatement(page:Page,id:string){const target=new URL('/#statements?statement='+id,page.url()).href;if(page.url()===target)await page.reload();else await page.goto(target);await expect(page.getByRole('dialog').locator('.contact-state').first()).toBeVisible()}
export async function fillStatementUpload(page:Page,title:string,data:Buffer=originalCSV,filename='fictional-income.csv'){
 await navigate(page,'Statements');await page.getByRole('button',{name:'Add prepared statement',exact:true}).click();await page.getByLabel('Statement original',{exact:true}).setInputFiles({name:filename,mimeType:'text/csv',buffer:data})
 await page.getByRole('textbox',{name:'Statement title',exact:true}).fill(title);await page.getByLabel('Period from',{exact:true}).fill('2026-09-01');await page.getByLabel('Period to',{exact:true}).fill('2026-09-30');await page.getByRole('textbox',{name:'Prepared by',exact:true}).fill('Fictional external accountant');await page.getByRole('textbox',{name:'Source reference',exact:true}).fill('Supplied fictional September accounts');await page.getByRole('textbox',{name:'Upload reason',exact:true}).fill('Checked the fictional source, exact original and explicit period.')
}
export async function statementDecisionUI(page:Page,button:string,publication=false){await page.getByRole('button',{name:button,exact:true}).click();await page.getByRole('textbox',{name:'Statement decision reason',exact:true}).fill('Checked this exact original, current permissions and deliberate decision.');await page.getByRole('checkbox',{name:publication?publicationDecisionCheck:decisionCheck,exact:true}).check()}
// Independent minimal ordinary workbook package. No spreadsheet program or
// network opens or recalculates the supplied formulas during the test.
export function statementWorkbook(active=false):Buffer{return execFileSync('python3',['-c',`import io,zipfile,sys
out=io.BytesIO()
parts={
'[Content_Types].xml':'<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>',
'_rels/.rels':'<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="root" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>',
'xl/workbook.xml':'<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Fictional income" sheetId="1" r:id="sheet"/></sheets></workbook>',
'xl/_rels/workbook.xml.rels':'<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="sheet" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>',
'xl/worksheets/sheet1.xml':'<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>400</v></c></row><row r="2"><c r="A2"><v>32.19</v></c></row><row r="3"><c r="A3"><f>'+('WEBSERVICE(&quot;https://example.invalid&quot;)' if sys.argv[1]=='active' else 'SUM(A1:A2)')+'</f><v>432.19</v></c></row></sheetData></worksheet>'}
with zipfile.ZipFile(out,'w',zipfile.ZIP_DEFLATED) as z:
 for name,text in parts.items():z.writestr(name,text)
sys.stdout.buffer.write(out.getvalue())`,active?'active':'plain'])}
