import { test,expect } from '@playwright/test'
import { loginWorkspace,chooseRegister,db } from './registry-import-fixtures'
import { execute,names } from './native-webmcp-helpers'
import { captureOffsite } from './offsite-helpers'

test('actual native non-fictional-mode protection stays paused with no configuration and preserves the human register form',async({page})=>{
  await loginWorkspace(page);await chooseRegister(page)
  await expect.poll(()=>names(page)).toContain('society_read_recovery_protection')
  const writes:string[]=[]
  page.on('request',request=>{if(!['GET','HEAD'].includes(request.method()))writes.push(request.url())})
  const result=JSON.parse(await execute(page,'society_read_recovery_protection',{}))
  expect(result.configured).toBe(false);expect(result.mode).toBe('NONE');expect(result.phase).toBe('NOT_CONFIGURED');expect(result.finance_paused).toBe(true);expect(result.counts).toEqual({})
  await expect(page.getByRole('heading',{name:'A considered beginning.',exact:true})).toBeVisible()
  await expect(page.getByRole('button',{name:'Validate & preview',exact:true})).toBeEnabled()
  await expect(execute(page,'society_open_workspace',{screen:'backups'})).rejects.toThrow()
  expect(writes).toEqual([]);expect(db('SELECT COUNT(*) FROM receipt_counter')[0][0]).toBe(0)
  await page.setViewportSize({width:320,height:440});await captureOffsite(page,'native-unconfigured-nonfictional-human-form-short320')
})
