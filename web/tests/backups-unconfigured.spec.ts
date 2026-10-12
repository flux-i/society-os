import { test, expect } from '@playwright/test'
import { login,navigate,chooseOption } from './helpers'
import { captureOffsite } from './offsite-helpers'
import { pageFits } from './registry-import-fixtures'

test('an actually unconfigured workspace reports absent protection and an empty recovery history accurately',async({page})=>{
  await login(page)
  const actual=await (await page.request.get('/api/backups')).json()
  expect(actual.configured).toBe(false);expect(actual.phase).toBe('NOT_CONFIGURED');expect(actual.finance_paused).toBe(false);expect(actual.items).toEqual([]);expect(actual.total).toBe(0)
  for(const [width,height] of [[1440,1000],[820,1050],[375,812],[320,740],[320,440]]) {
    await page.setViewportSize({width,height});await navigate(page,'Backups & recovery')
    await expect(page.getByRole('heading',{name:'Protection not configured',exact:true})).toBeVisible()
    await expect(page.getByRole('heading',{name:'The first copy starts here.',exact:true})).toBeVisible()
    const filter=page.getByRole('combobox',{name:'Filter recovery attempts',exact:true})
    expect((await filter.boundingBox())!.height).toBeGreaterThanOrEqual(44)
    await pageFits(page);await page.locator('.backups-heading').scrollIntoViewIfNeeded();await captureOffsite(page,`unconfigured-protection-${width}-${height}`)
  }
  await chooseOption(page,'Filter recovery attempts','Confirmed copies')
  await expect(page.getByRole('heading',{name:'No attempts in this view.',exact:true})).toBeVisible()
  await page.getByRole('button',{name:'Show all attempts',exact:true}).click()
  await expect(page.getByRole('heading',{name:'The first copy starts here.',exact:true})).toBeVisible()
})
