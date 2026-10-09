import { expect } from '@playwright/test'
import type { Browser, BrowserContext, Page } from '@playwright/test'
import { createHmac } from 'node:crypto'
import { chmodSync, mkdirSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { chooseOption, navigate } from './helpers'
import { confirmImport, db, loginWorkspace, previewRegister } from './registry-import-fixtures'

export const rehearsalRegister = readFileSync(resolve('../testdata/registry-rehearsal-12.json'), 'utf8')
export const personalPassword = 'Fictional-personal-rehearsal-2026!'
export interface RehearsalPerson { page: Page; context: BrowserContext; name: string; email: string; recovery: string[] }
export interface RehearsalActors { registry: Page; finance: RehearsalPerson; secondFinance: RehearsalPerson; reviewer: RehearsalPerson; owner: RehearsalPerson; tenant: RehearsalPerson; close: () => Promise<void> }
export const artifactFolder = () => process.env.SOCIETY_REHEARSAL_ARTIFACTS ?? resolve(process.env.SOCIETY_BROWSER_ARTIFACTS!, 'rehearsal')

export function readDB(query: string, parameters: unknown[] = []) {
  if (!/^SELECT\b/i.test(query.trim())) throw new Error('The personal rehearsal permits independent database reads only.')
  return db(query, parameters)
}
export function entity(kind: 'HOME' | 'PERSON', source: string) {
  const value = readDB('SELECT entity_id FROM registry_import_entities WHERE entity_kind=? AND source_id=?', [kind, source])[0]?.[0]
  if (typeof value !== 'string') throw new Error('Missing supplied source identity: ' + source)
  return value
}
function authenticatorCode(secret: string) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
  const bits = [...secret].map(char => alphabet.indexOf(char).toString(2).padStart(5, '0')).join('')
  const bytes = Buffer.from(bits.match(/.{8}/g)!.map(value => parseInt(value, 2)))
  const counter = Buffer.alloc(8); counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)))
  const mac = createHmac('sha1', bytes).update(counter).digest(), offset = mac.at(-1)! & 15
  return String((mac.readUInt32BE(offset) & 0x7fffffff) % 1000000).padStart(6, '0')
}
export async function settle(page: Page) {
  await page.evaluate(async () => {
    await document.fonts.ready
    await Promise.allSettled(document.getAnimations().filter(animation => animation.effect?.getComputedTiming().iterations !== Infinity).map(animation => animation.finished))
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))
  })
}
export async function capture(page: Page, name: string) {
  await settle(page)
  const folder = artifactFolder(); mkdirSync(folder, { recursive: true, mode: 0o700 })
  const scroll = await page.evaluate(() => window.scrollY), modal = await page.getByRole('dialog').count() > 0
  if (modal) await page.evaluate(() => window.scrollTo({ top: 0, behavior: 'instant' }))
  try { const path = resolve(folder, name + '.png'); await page.screenshot({ path, fullPage: false }); chmodSync(path, 0o600) }
  finally { if (modal) await page.evaluate(y => window.scrollTo({ top: y, behavior: 'instant' }), scroll) }
}
export async function loginPerson(person: RehearsalPerson) {
  const page = person.page
  await page.goto('/'); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeEnabled()
  await page.getByLabel('Email address', { exact: true }).fill(person.email)
  await page.getByLabel('Password', { exact: true }).fill(personalPassword)
  const signingIn = page.waitForResponse(response => response.url().endsWith('/api/auth/login') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  const result = await signingIn; expect(result.status()).toBe(200); await result.finished()
  const me = await result.json()
  expect(me.is_demo).toBe(false)
  if (me.mfa_pending) {
    await expect(page.getByRole('button', { name: 'Use a preview code', exact: true })).toHaveCount(0)
    if (!me.mfa_enrolled) {
      await page.getByText('Enter a setup key instead', { exact: true }).click()
      await expect(page.locator('.setup-key')).not.toBeEmpty()
      await page.getByLabel('Authenticator code', { exact: true }).fill(authenticatorCode(await page.locator('.setup-key').innerText()))
      await page.getByRole('button', { name: 'Verify and continue', exact: true }).click()
      await expect(page.getByRole('heading', { name: 'Keep these close.' })).toBeVisible()
      person.recovery = await page.locator('.recovery-grid code').allTextContents()
      await page.getByRole('checkbox', { name: 'I have saved my recovery codes privately.' }).check()
      await page.getByRole('button', { name: 'Continue to workspace', exact: true }).click()
    } else {
      const value = person.recovery.shift(); if (!value) throw new Error('No retained personal rehearsal recovery code.')
      await page.getByRole('button', { name: 'Use a recovery code', exact: true }).click()
      await page.getByLabel('Recovery code', { exact: true }).fill(value)
      await page.getByRole('button', { name: 'Verify and continue', exact: true }).click()
    }
  }
  await expect(page.getByRole('button', { name: 'Sign out', exact: true })).toBeVisible()
  expect((await (await page.request.get('/api/auth/me')).json()).mfa_pending).toBe(false)
}
async function invitePerson(registry: Page, browser: Browser, name: string, email: string, appointment?: string): Promise<RehearsalPerson> {
  await navigate(registry, 'Access & invitations')
  await registry.getByRole('button', { name: 'Invite a person', exact: true }).click()
  await registry.getByLabel('Find a person', { exact: true }).fill(name)
  await chooseOption(registry, 'Person in the registry', name)
  await registry.getByRole('dialog').getByLabel('Email address', { exact: true }).fill(email)
  await registry.getByRole('checkbox', { name: 'I verified this person’s identity, email and home relationship.' }).check()
  await registry.getByLabel('Verification note', { exact: true }).fill('Verified this fictional personal identity against the supplied twelve-home register.')
  await registry.getByRole('button', { name: 'Create personal link', exact: true }).click()
  const link = await registry.getByLabel('Personal link', { exact: true }).inputValue()
  const context = await browser.newContext({ baseURL: new URL(registry.url()).origin, serviceWorkers: 'block' }), page = await context.newPage()
  const person = { page, context, name, email, recovery: [] as string[] }
  await page.goto(link); await expect(page.getByRole('heading', { name: 'Your place is ready.' })).toBeVisible()
  await page.getByLabel('New password', { exact: true }).fill(personalPassword)
  await page.getByLabel('Confirm new password', { exact: true }).fill(personalPassword)
  await page.getByRole('button', { name: 'Save my password', exact: true }).click()
  await page.getByRole('button', { name: 'Continue to sign in', exact: true }).click()
  await registry.getByRole('button', { name: 'Close invitation', exact: true }).click()
  if (appointment) {
    await registry.getByRole('searchbox', { name: 'Search accounts', exact: true }).fill(name)
    await registry.getByRole('button', { name: 'Manage access for ' + name, exact: true }).click()
    await registry.getByRole('button', { name: 'Add appointment', exact: true }).click()
    await chooseOption(registry, 'Appointment', appointment)
    await registry.getByRole('spinbutton', { name: 'Term (days)', exact: true }).fill('30')
    await registry.getByRole('textbox', { name: 'Reason', exact: true }).fill('Verified separate fictional appointment and thirty-day authority against the supplied register.')
    await registry.getByRole('dialog').getByRole('checkbox').check()
    await registry.getByRole('button', { name: 'Save appointment', exact: true }).click()
    await expect(registry.getByRole('dialog').locator('.form-success')).toBeVisible()
    await registry.getByRole('button', { name: 'Close account access', exact: true }).click()
    await registry.getByRole('searchbox', { name: 'Search accounts', exact: true }).fill('')
  }
  await loginPerson(person)
  return person
}
export async function createRehearsal(browser: Browser): Promise<RehearsalActors> {
  const context = await browser.newContext({ baseURL: process.env.SOCIETY_BROWSER_URL, serviceWorkers: 'block' }), registry = await context.newPage()
  const people: RehearsalPerson[] = []
  try {
    await loginWorkspace(registry); await previewRegister(registry, rehearsalRegister)
    await expect(registry.getByRole('definition')).toHaveText(['12', '11', '1', '12', '4'])
    await confirmImport(registry); await registry.getByRole('button', { name: 'Apply this register', exact: true }).click()
    await expect(registry.getByRole('heading', { name: 'Your register is here.' })).toBeVisible()
    const finance = await invitePerson(registry, browser, 'Sample Owner A 104', 'treasury-a@example.test', 'Treasurer'); people.push(finance)
    const secondFinance = await invitePerson(registry, browser, 'Sample Owner A 201', 'treasury-b@example.test', 'Treasurer'); people.push(secondFinance)
    const reviewer = await invitePerson(registry, browser, 'Sample Owner A 203', 'reviewer@example.test', 'Committee'); people.push(reviewer)
    const owner = await invitePerson(registry, browser, 'Sample Owner A 101', 'owner@example.test'); people.push(owner)
    const tenant = await invitePerson(registry, browser, 'Sample Tenant A 103', 'tenant@example.test'); people.push(tenant)
    return { registry, finance, secondFinance, reviewer, owner, tenant, close: async () => { await Promise.all(people.map(person => person.context.close())); await context.close() } }
  } catch (err) { await Promise.all(people.map(person => person.context.close())); await context.close(); throw err }
}
export async function homeFinance(page: Page, home = 'A-101') {
  if (await page.getByRole('dialog').count()) await page.getByRole('button', { name: 'Close details', exact: true }).click()
  await navigate(page, 'Homes & people'); await page.getByRole('button', { name: 'View home ' + home, exact: true }).click()
  await page.getByRole('button', { name: 'Financial access', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Financial access.' })).toBeVisible()
  await expect(page.getByText('Opening current financial access…', { exact: true })).toHaveCount(0)
}
export async function prepareVisibility(page: Page, name: string, decision = 'Allow household view') {
  await page.getByRole('button', { name: 'Review financial access for ' + name, exact: true }).click()
  await chooseOption(page, 'Household financial visibility', decision)
  await page.getByLabel('Financial access verification note', { exact: true }).fill('Verified the supplied current household relationship and this deliberate financial view.')
  await page.getByRole('checkbox', { name: 'I verified this person’s current household relationship and financial access.' }).check()
}
export async function visibility(page: Page, name: string, decision = 'Allow household view') {
  await prepareVisibility(page, name, decision)
  await page.getByRole('button', { name: decision, exact: true }).click()
  await expect(page.getByRole('status').filter({ hasText: 'is recorded. Current household access is refreshed below.' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Review financial access for ' + name, exact: true })).toBeEnabled()
}
export async function postAPI(page: Page, path: string, data: unknown, status = 200) {
  const me = await (await page.request.get('/api/auth/me')).json()
  const result = await page.request.post(path, { headers: { Origin: new URL(page.url()).origin, 'X-CSRF-Token': me.csrf_token }, data })
  expect(result.status(), await result.text()).toBe(status)
  return result.json()
}
