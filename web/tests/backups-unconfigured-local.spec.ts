import { test,expect } from '@playwright/test'
import { loginWorkspace,pageFits } from './registry-import-fixtures'
import { navigate } from './helpers'
import { captureOffsite } from './offsite-helpers'

test('a non-fictional-mode fixture requires protection before money effects and displays its actual pause on overview and recovery screens',async({page})=>{
  await loginWorkspace(page)
  const actual=await(await page.request.get('/api/backups')).json()
  expect(actual.configured).toBe(false);expect(actual.phase).toBe('NOT_CONFIGURED');expect(actual.finance_paused).toBe(true);expect(actual.items).toEqual([])
  await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0)
  for(const[width,height]of [[1440,1000],[375,812],[320,440]]) {
    await page.setViewportSize({width,height});await navigate(page,'Overview')
    await expect(page.locator('.backup-protection-notice')).toContainText('Protection not configured')
    await expect(page.locator('.backup-protection-notice')).toContainText('Drafts and existing records remain available')
    await page.locator('.backup-protection-notice').scrollIntoViewIfNeeded();await pageFits(page)
    await captureOffsite(page,`nonfictional-mode-required-overview-${width}-${height}`)
    await page.getByRole('link',{name:'Check protection',exact:false}).click()
    await expect(page.locator('.backup-protection-card')).toHaveClass(/backup-finance-paused/)
    await expect(page.locator('.backup-protection-copy')).toContainText('Off-site protection is required')
    await pageFits(page)
  }
})
