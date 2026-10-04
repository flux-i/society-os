import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'

export async function chooseFilter(page: Page, name: string, value: string) {
  const labels: Record<string, string> = { A: 'Wing A', B: 'Wing B', C: 'Wing C', OWNER_OCCUPIED: 'Owner occupied', RENTED: 'Rented', VACANT: 'Vacant' }
  await page.getByRole('combobox', { name, exact: true }).click()
  await page.getByRole('option', { name: value ? labels[value] : name === 'Filter by wing' ? 'All wings' : 'All occupancy', exact: true }).click()
}

export async function chooseOption(page: Page, name: string, choice: string) {
  const labels: Record<string, string> = {
    OWNER_OCCUPIED: 'Owner occupied', RENTED: 'Rented', VACANT: 'Vacant', existing: 'Link an existing person',
    new: 'Create a new person', OWNER: 'Owner', TENANT: 'Tenant', FAMILY: 'Family member', AUTHORIZED_OCCUPANT: 'Authorized occupant',
    RESIDENT: 'Resident · their current homes', COMMITTEE: 'Committee · read the registry', ADMINISTRATOR: 'Administrator · manage registry',
  }
  await page.getByRole('combobox', { name, exact: true }).click()
  await page.getByRole('option', { name: labels[choice] ?? choice, exact: true }).click()
}

export async function navigate(page: Page, name: string) {
  const menu = page.getByRole('button', { name: 'Menu', exact: true })
  if (await menu.isVisible() && await menu.getAttribute('aria-expanded') === 'false') await menu.click()
  await page.getByRole('link', { name, exact: true }).click()
}

export async function completePreviewMFA(page: Page) {
  const identity = await page.request.get('/api/auth/me')
  const user = await identity.json() as { mfa_pending: boolean; mfa_enrolled: boolean }
  if (!user.mfa_pending) return
  await expect(page.getByRole('button', { name: 'Use a preview code', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Use a preview code', exact: true }).click()
  await page.getByRole('button', { name: 'Verify and continue', exact: true }).click()
  if (!user.mfa_enrolled) {
    await expect(page.getByRole('heading', { name: 'Keep these close.' })).toBeVisible()
    await page.getByRole('checkbox', { name: 'I have saved my recovery codes privately.' }).check()
    await page.getByRole('button', { name: 'Continue to workspace', exact: true }).click()
  }
}
export async function login(page: Page, account = 'Registry officer') {
  await page.goto('/')
  await page.getByRole('button', { name: new RegExp('^' + account) }).click()
  const signedIn = page.waitForResponse(response => response.url().endsWith('/api/auth/login'))
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  expect((await signedIn).status()).toBe(200)
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toHaveCount(0)
  await completePreviewMFA(page)
  await expect(page.getByRole('button', { name: 'Sign out', exact: true })).toBeVisible()
}
