import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { createHmac } from 'node:crypto'
import { mkdirSync, chmodSync } from 'node:fs'
import { resolve } from 'node:path'
import { completePreviewMFA, login, navigate, chooseOption } from './helpers'

async function capture(page: Page, name: string) {
  if (!process.env.SOCIETY_CAPTURE_UI) return
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.evaluate(() => document.fonts.ready)
  const folder = resolve('../reports/local/security-ui')
  mkdirSync(folder, { recursive: true, mode: 0o700 })
  const path = resolve(folder, name + '.png')
  await page.screenshot({ path, fullPage: true })
  chmodSync(path, 0o600)
}

async function invite(page: Page, name: string, email: string, role = 'RESIDENT') {
  await navigate(page, 'Access & invitations')
  await page.getByRole('button', { name: 'Invite a person', exact: true }).click()
  await page.getByLabel('Find a person').fill(name)
  await chooseOption(page, 'Person in the registry', name)
  await page.getByLabel('Email address', { exact: true }).fill(email)
  await chooseOption(page, 'Access', role)
  if (role === 'RESIDENT') await capture(page, 'invitation-desktop')
  await page.getByRole('checkbox', { name: 'I verified this person’s identity, email and home relationship.' }).check()
  await page.getByLabel('Verification note').fill('Verified fictional neighbour and email in person')
  await page.getByRole('button', { name: 'Create personal link', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Their door is open.' })).toBeVisible()
  const url = await page.getByLabel('Personal link', { exact: true }).inputValue()
  await page.getByRole('button', { name: 'Close invitation', exact: true }).click()
  return url
}
async function savePassword(page: Page, url: string, password: string) {
  await page.goto(url)
  await expect(page.getByLabel('New password', { exact: true })).toBeVisible()
  expect(new URL(page.url()).hash).toBe('') // Bearer link removed from address/history.
  await page.getByLabel('New password', { exact: true }).fill(password)
  await page.getByLabel('Confirm new password', { exact: true }).fill(password)
  await page.getByRole('button', { name: 'Save my password', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Make yourself at home.' })).toBeVisible()
  await page.getByRole('button', { name: 'Continue to sign in', exact: true }).click()
}
async function customLogin(page: Page, email: string, password: string) {
  await page.getByLabel('Email address').fill(email)
  await page.getByLabel('Password', { exact: true }).fill(password)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
}
function totp(text: string) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
  const bits = text.split('').map(c => alphabet.indexOf(c).toString(2).padStart(5, '0')).join('')
  const secret = Buffer.from((bits.match(/.{8}/g) ?? []).map(b => parseInt(b, 2)))
  const counter = Buffer.alloc(8); counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)))
  const hash = createHmac('sha1', secret).update(counter).digest()
  const offset = hash[hash.length - 1] & 15
  return String((hash.readUInt32BE(offset) & 0x7fffffff) % 1000000).padStart(6, '0')
}

test('password alone denies administrator data; recovery codes are saved once', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Verify and continue', exact: true })).toBeVisible()
  expect((await page.request.get('/api/admin/accounts')).status()).toBe(403)
  expect((await page.request.get('/api/flats')).status()).toBe(403)
  await capture(page, 'verification-desktop')
  await completePreviewMFA(page)
  await navigate(page, 'Account security')
  await expect(page.getByRole('heading', { name: 'Account security.' })).toBeVisible()
  await capture(page, 'security-desktop')
  await page.getByRole('button', { name: 'Replace recovery codes', exact: true }).click()
  await expect(page.locator('.recovery-grid code')).toHaveCount(10)
  const download = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Download codes', exact: true }).click()
  expect((await download).suggestedFilename()).toBe('society-recovery-codes.txt')
  await expect(page.getByRole('button', { name: 'Done, codes saved', exact: true })).toBeDisabled()
  await page.getByRole('checkbox', { name: 'I have saved my recovery codes privately.' }).check()
  await page.getByRole('button', { name: 'Done, codes saved', exact: true }).click()
  await expect(page.locator('.recovery-grid')).toHaveCount(0)
})

test('invite, activate scoped resident and reset password while revoking their session', async ({ page, browser }) => {
  await login(page)
  const url = await invite(page, 'Demo Owner B-101', 'neighbour@browser.test')
  const context = await browser.newContext()
  const recipient = await context.newPage()
  try {
    await savePassword(recipient, url, 'A memorable new password!')
    await customLogin(recipient, 'neighbour@browser.test', 'A memorable new password!')
    await navigate(recipient, 'Your homes')
    await expect(recipient.getByRole('status')).toHaveText('1 homes found')
    await expect(recipient.getByRole('button', { name: 'View home B-101', exact: true })).toBeVisible()
    expect((await recipient.request.get(new URL('/api/admin/accounts', url).toString())).status()).toBe(403)
    await page.getByRole('searchbox', { name: 'Search accounts', exact: true }).fill('neighbour@browser.test')
    await expect(page.locator('.account-row')).toHaveCount(1)
    await page.getByRole('button', { name: 'Password recovery', exact: true }).click()
    await page.getByRole('checkbox', { name: 'I verified this person’s identity, email and home relationship.' }).check()
    await page.getByLabel('Verification note').fill('Reconfirmed fictional neighbour using verified identity')
    await page.getByRole('button', { name: 'Create personal link', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Their door is open.' })).toBeVisible()
    const resetURL = await page.getByLabel('Personal link').inputValue()
    const resetContext = await browser.newContext()
    const reset = await resetContext.newPage()
    try {
      await savePassword(reset, resetURL, 'A different new password!')
      expect((await recipient.request.get(new URL('/api/flats', url).toString())).status()).toBe(401)
      await customLogin(reset, 'neighbour@browser.test', 'A different new password!')
      await navigate(reset, 'Your homes')
      await expect(reset.getByRole('status')).toHaveText('1 homes found')
      await reset.goto(resetURL)
      await expect(reset.getByRole('alert')).toContainText('expired or has already been used')
    } finally { await resetContext.close() }
    await recipient.goto(url)
    await expect(recipient.getByRole('alert')).toContainText('expired or has already been used')
  } finally { await context.close() }
})

test('invited administrator enrolls a real authenticator and uses a single recovery code', async ({ page, browser }) => {
  await login(page)
  const url = await invite(page, 'Demo Owner B-102', 'officer@browser.test', 'ADMINISTRATOR')
  const context = await browser.newContext()
  const recipient = await context.newPage()
  try {
    await savePassword(recipient, url, 'A memorable new password!')
    await customLogin(recipient, 'officer@browser.test', 'A memorable new password!')
    await expect(recipient.locator('.setup-key')).toHaveCount(1)
    await expect(recipient.getByRole('button', { name: 'Use a preview code', exact: true })).toHaveCount(0)
    expect((await recipient.request.get(new URL('/api/flats', url).toString())).status()).toBe(403)
    const secret = await recipient.locator('.setup-key').textContent()
    await recipient.getByLabel('Authenticator code').fill(totp(secret!))
    await recipient.getByRole('button', { name: 'Verify and continue', exact: true }).click()
    await expect(recipient.locator('.recovery-grid code')).toHaveCount(10)
    const recovery = await recipient.locator('.recovery-grid code').first().textContent()
    await recipient.getByRole('checkbox', { name: 'I have saved my recovery codes privately.' }).check()
    await recipient.getByRole('button', { name: 'Continue to workspace', exact: true }).click()
    await expect(recipient.getByRole('heading', { name: 'Good things, in order.' })).toBeVisible()
    await recipient.getByRole('button', { name: 'Sign out', exact: true }).click()
    await customLogin(recipient, 'officer@browser.test', 'A memorable new password!')
    await recipient.getByRole('button', { name: 'Use a recovery code', exact: true }).click()
    await recipient.getByLabel('Recovery code', { exact: true }).fill(recovery!)
    await recipient.getByRole('button', { name: 'Verify and continue', exact: true }).click()
    await expect(recipient.getByRole('button', { name: 'Sign out', exact: true })).toBeVisible()
    await recipient.getByRole('button', { name: 'Sign out', exact: true }).click()
    await customLogin(recipient, 'officer@browser.test', 'A memorable new password!')
    await recipient.getByRole('button', { name: 'Use a recovery code', exact: true }).click()
    await recipient.getByLabel('Recovery code', { exact: true }).fill(recovery!)
    await recipient.getByRole('button', { name: 'Verify and continue', exact: true }).click()
    await expect(recipient.getByRole('alert')).toContainText('verification code didn’t work')
    expect((await recipient.request.get(new URL('/api/flats', url).toString())).status()).toBe(403)
  } finally { await context.close() }
})

test('phone invitation and account security fit and the dialog returns keyboard focus', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 })
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await login(page)
  await navigate(page, 'Access & invitations')
  await expect(page.getByRole('heading', { name: 'A personal welcome.' })).toBeVisible()
  await expect(page.locator('.account-row')).not.toHaveCount(0)
  await capture(page, 'access-phone')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.getByRole('button', { name: 'Invite a person', exact: true }).click()
  expect(await page.getByRole('dialog').evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true)
  await capture(page, 'invitation-phone')
  await page.keyboard.press('Escape')
  await expect(page.getByRole('button', { name: 'Invite a person', exact: true })).toBeFocused()
  await navigate(page, 'Account security')
  await expect(page.getByRole('heading', { name: 'Account security.' })).toBeVisible()
  await capture(page, 'security-phone')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})
