import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { mkdirSync, chmodSync } from 'node:fs'
import { resolve } from 'node:path'
import { login, navigate, chooseOption } from './helpers'
import {
  apiRule,
  apiRuleAction,
  apiIncident,
  apiIncidentAction
} from './incidents-fixtures'
import { careDate } from './upkeep-fixtures'

async function overview(page: Page) {
  if (await page.getByRole('dialog').count())
    await page.getByRole('dialog').locator('.dialog-close').click()
  await navigate(page, 'Overview')
  await expect(
    page.getByRole('button', { name: 'Refresh overview', exact: true })
  ).toBeEnabled()
  await page
    .getByRole('button', { name: 'Refresh overview', exact: true })
    .click()
  await expect(
    page.getByRole('button', { name: 'Refresh overview', exact: true })
  ).toBeEnabled()
}
async function capture(page: Page, name: string) {
  if (!process.env.SOCIETY_CAPTURE_UI) return
  await page.evaluate(() => document.fonts.ready)
  const folder = resolve('../reports/local/incidents-review')
  mkdirSync(folder, { recursive: true, mode: 0o700 })
  const path = resolve(folder, name + '.png')
  await page.screenshot({
    path,
    fullPage: !(await page.getByRole('dialog').count()),
    animations: 'disabled'
  })
  chmodSync(path, 0o600)
}
test('independent rules private reviews and personal response attention open exact records while an unavailable source remains unknown', async ({
  page,
  browser
}) => {
  test.setTimeout(60000)
  await login(page)
  const context = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    reviewer = await context.newPage()
  await login(reviewer, 'Committee')
  const title = 'Overview supplied corridor policy ' + Date.now(),
    rule = await apiRule(reviewer, title)
  await overview(page)
  const panel = page.getByRole('region', {
    name: 'Rules, with care.',
    exact: true
  })
  await expect(
    panel.locator('.overview-maintenance-amounts strong')
  ).toHaveText(['0', '1', '0'])
  await page
    .getByRole('link', {
      name: 'Review a supplied rule: ' + title,
      exact: true
    })
    .click()
  expect(new URL(page.url()).hash).toBe('#conduct?rule=' + rule)
  await expect(
    page
      .getByRole('dialog')
      .getByRole('heading', { name: title, exact: true, level: 2 })
  ).toBeVisible()
  await capture(page, 'overview-exact-rule-review')
  await chooseOption(page, 'Rule decision', 'Publish rule')
  await page
    .getByRole('textbox', { name: 'Reason for rule decision', exact: true })
    .fill(
      'Reviewed the fictional supplied policy independently before publication.'
    )
  await page
    .getByRole('checkbox', {
      name: 'I reviewed this decision and the supplied policy.',
      exact: true
    })
    .check()
  await page
    .getByRole('button', { name: 'Publish reviewed rule', exact: true })
    .click()
  await expect(
    page.getByRole('dialog').getByText('Published', { exact: true })
  ).toBeVisible()
  expect(
    (await (await page.request.get('/api/rules/' + rule)).json()).state
  ).toBe('PUBLISHED')
  // Keep the fixture helper's independent-state comparison in this isolated suite.
  const ownRule = await apiRule(page, 'Own pending policy ' + Date.now())
  await apiRuleAction(reviewer, ownRule, 'DECLINED')
  const residentContext = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    tenant = await residentContext.newPage()
  await login(tenant, 'Tenant')
  const id = await apiIncident(tenant, rule)
  await overview(page)
  await expect(
    panel.locator('.overview-maintenance-amounts strong')
  ).toHaveText(['1', '0', '0'])
  await page
    .getByRole('link', {
      name: 'Review a private incident: ' + title,
      exact: true
    })
    .click()
  expect(new URL(page.url()).hash).toBe('#conduct?case=' + id)
  await capture(page, 'overview-exact-private-case')
  await apiIncidentAction(page, id, 'NEEDS_INFO')
  await overview(page)
  const staffAction = page.getByRole('link', { name: 'Review a private incident: ' + title, exact: true })
  await expect(staffAction).toBeVisible()
  await expect(page.getByRole('link', { name: 'Clarify an incident report: ' + title, exact: true })).toHaveCount(0)
  await capture(page, 'overview-staff-requested-information-action')
  await staffAction.click()
  expect(new URL(page.url()).hash).toBe('#conduct?case=' + id)
  await expect(page.getByRole('combobox', { name: 'Incident decision', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Revise your report', exact: true })).toHaveCount(0)
  await overview(tenant)
  await expect(
    tenant
      .getByRole('region', { name: 'Rules, with care.', exact: true })
      .locator('.overview-maintenance-amounts strong')
  ).toHaveText(['1'])
  await tenant
    .getByRole('link', {
      name: 'Clarify an incident report: ' + title,
      exact: true
    })
    .click()
  expect(new URL(tenant.url()).hash).toBe('#conduct?case=' + id)
  await expect(
    tenant.getByRole('button', { name: 'Revise your report', exact: true })
  ).toBeVisible()
  await capture(tenant, 'overview-exact-own-clarification')
  const noticeTitle = 'A deliberate household response request',
    issued = await apiIncidentAction(page, id, 'ISSUE_NOTICE', {
      title: noticeTitle,
      body: 'A supplied fictional notice deliberately requests this household’s response.',
      response_by: careDate(4)
    })
  const ownerContext = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    owner = await ownerContext.newPage()
  await login(owner, 'Owner')
  await overview(owner)
  await expect(
    owner
      .getByRole('region', { name: 'Rules, with care.', exact: true })
      .locator('.overview-maintenance-amounts strong')
  ).toHaveText(['1'])
  await owner
    .getByRole('link', {
      name: 'Respond to a household notice: ' + noticeTitle,
      exact: true
    })
    .click()
  expect(new URL(owner.url()).hash).toBe('#conduct?notice=' + issued.notice_id)
  await owner
    .getByRole('textbox', { name: 'Your household response', exact: true })
    .fill(
      'A supplied private household account, deliberately submitted in response.'
    )
  await owner
    .getByRole('checkbox', {
      name: 'I reviewed my response to this notice.',
      exact: true
    })
    .check()
  await owner
    .getByRole('button', { name: 'Submit household response', exact: true })
    .click()
  await expect(
    owner.getByRole('region', { name: 'Household responses', exact: true })
  ).toContainText('supplied private household account')
  await overview(owner)
  await expect(
    owner
      .getByRole('region', { name: 'Rules, with care.', exact: true })
      .locator('.overview-maintenance-amounts strong')
  ).toHaveText(['0'])
  await expect(
    owner.getByRole('link', {
      name: 'Respond to a household notice: ' + noticeTitle,
      exact: true
    })
  ).toHaveCount(0)
  await overview(page)
  for (const width of [1440, 768, 375, 320]) {
    await page.setViewportSize({ width, height: 900 })
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth
      )
    ).toBe(true)
    // Compare relative geometry in one frame, including while return-focus scrolling settles.
    const spacing = await panel.evaluate(current => {
      const previous = document.querySelector('[aria-labelledby="overview-notices-title"]')
      if (!previous) throw new Error('Noticeboard missing')
      return current.getBoundingClientRect().top - previous.getBoundingClientRect().bottom
    })
    expect(spacing).toBeGreaterThanOrEqual(16)
    await capture(page, 'incidents-overview-' + width)
  }
  await page.route('**/api/overview/incidents', (route) =>
    route.fulfill({ status: 503, contentType: 'application/json', body: '{}' })
  )
  await page
    .getByRole('button', { name: 'Refresh overview', exact: true })
    .click()
  await expect(
    panel.locator('.overview-maintenance-amounts strong')
  ).toHaveText(['—', '—', '—'])
  await expect(panel.getByRole('alert')).toBeVisible()
  await expect(
    page
      .getByRole('region', { name: 'Maintenance, in view.', exact: true })
      .locator('.overview-maintenance-amounts strong')
  ).toHaveText(['₹0.00', '₹0.00', '0'])
  await capture(page, 'incidents-overview-unavailable-320')
  await page.unroute('**/api/overview/incidents')
  await panel.getByRole('button').click()
  await expect(
    panel.locator('.overview-maintenance-amounts strong')
  ).toHaveText(['1', '0', '0'])
  await ownerContext.close()
  await residentContext.close()
  await context.close()
})
