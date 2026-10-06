import { test, expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { mkdirSync, chmodSync } from 'node:fs'
import { resolve } from 'node:path'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { chooseOption, login, navigate } from './helpers'
import { financialHeaders } from './maintenance-fixtures'
import {
  apiRule,
  apiRuleAction,
  apiIncident,
  apiIncidentAction,
  apiPicture,
  scenePNG
} from './incidents-fixtures'
import { careDate } from './upkeep-fixtures'
async function capture(page: Page, name: string) {
  if (!process.env.SOCIETY_CAPTURE_UI) return
  await page.evaluate(() => document.fonts.ready)
  await page.evaluate(async () => {
    await Promise.allSettled(
      document
        .getAnimations()
        .filter((a) => a.effect?.getComputedTiming().iterations !== Infinity)
        .map((a) => a.finished)
    )
  })
  const folder = resolve(process.env.SOCIETY_INCIDENT_CAPTURE_ROOT ?? '../reports/local/incidents-review')
  mkdirSync(folder, { recursive: true, mode: 0o700 })
  const path = resolve(folder, name + '.png')
  await page.screenshot({ path, animations: 'disabled' })
  chmodSync(path, 0o600)
}
async function within(page: Page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth
    )
  ).toBe(true)
  if (await page.getByRole('dialog').count()) {
    await expect(
      page.getByRole('dialog').locator('.dialog-close')
    ).toBeInViewport({ ratio: 1 })
    if (await page.getByRole('dialog').locator('.incident-dialog').count()) {
      const separation = await page.getByRole('dialog').evaluate((dialog) => {
        const button = dialog
          .querySelector('.dialog-close')!
          .getBoundingClientRect()
        const content = dialog
          .querySelector('.dialog-scroll')!
          .getBoundingClientRect()
        return content.top - button.bottom
      })
      expect(
        separation,
        'the close control must have a separate row above scrolling content'
      ).toBeGreaterThanOrEqual(8)
    }
  }
}
async function decision(page: Page, choice: string) {
  await chooseOption(page, 'Incident decision', choice)
  await page
    .getByRole('textbox', {
      name:
        choice === 'Private staff note'
          ? 'Private staff note'
          : 'Reason for this decision',
      exact: true
    })
    .fill(
      'PRIVATE supplied facts were deliberately checked before this decision'
    )
  await page
    .getByRole('checkbox', {
      name: 'I reviewed this decision and its intended audience.',
      exact: true
    })
    .check()
  await page
    .getByRole('button', { name: 'Save incident decision', exact: true })
    .click()
}
async function ruleDecision(page: Page, choice: string) {
  await chooseOption(page, 'Rule decision', choice)
  await page
    .getByRole('textbox', { name: 'Reason for rule decision', exact: true })
    .fill('Checked the supplied fictional rule and its authority separately')
  await page
    .getByRole('checkbox', {
      name: 'I reviewed this decision and the supplied policy.',
      exact: true
    })
    .check()
  await page
    .getByRole('button', {
      name:
        choice === 'Publish rule'
          ? 'Publish reviewed rule'
          : 'Save rule decision',
      exact: true
    })
    .click()
}
async function responseNotice(page: Page) {
  await chooseOption(page, 'Incident decision', 'Request household response')
  await page
    .getByRole('textbox', { name: 'Household notice title', exact: true })
    .fill('Please clarify the corridor observation')
  await page
    .getByRole('textbox', { name: 'Household notice wording', exact: true })
    .fill(
      'A fictional observation relates to shared corridor access. Please provide your account.'
    )
  await page.getByLabel('Response by', { exact: true }).fill(careDate(4))
  await page
    .getByRole('textbox', { name: 'Reason for this decision', exact: true })
    .fill('PRIVATE officer identity and notes must stay with the handler')
  await page
    .getByRole('checkbox', {
      name: 'I reviewed this decision and its intended audience.',
      exact: true
    })
    .check()
  await page
    .getByRole('button', {
      name: 'Issue reviewed response notice',
      exact: true
    })
    .click()
  await expect(
    page.getByRole('heading', { name: 'Response notices', exact: true })
  ).toBeVisible()
  return (
    await (
      await page.request.get(
        '/api/incidents/' +
          new URLSearchParams(new URL(page.url()).hash.split('?')[1]).get(
            'case'
          )
      )
    ).json()
  ).notice_id as string
}
function isolatedDB() {
  const path = process.env.SOCIETY_BROWSER_DB
  if (!path || !path.includes('society-browser-'))
    throw new Error('A disposable fictional database is required')
  return path
}
function fixtureSQL(sql: string, args: unknown[] = []) {
  return JSON.parse(
    execFileSync(
      'python3',
      [
        '-c',
        'import sqlite3,sys,json;db=sqlite3.connect(sys.argv[1]);r=db.execute(sys.argv[2],json.loads(sys.argv[3]));out=r.fetchall();db.commit();print(json.dumps(out))',
        isolatedDB(),
        sql,
        JSON.stringify(args)
      ],
      { encoding: 'utf8' }
    )
  )
}
test('empty conduct and opened filters remain usable at four widths with no public allegation feed', async ({
  page,
  browser
}) => {
  await login(page)
  await navigate(page, 'Rules & conduct')
  await expect(
    page.getByRole('heading', { name: 'A little calm, for now.', exact: true })
  ).toBeVisible()
  for (const width of [1440, 768, 375, 320]) {
    await page.setViewportSize({ width, height: 900 })
    const menu = page.getByRole('combobox', {
      name: 'Filter incident status',
      exact: true
    })
    await menu.click()
    await expect(
      page.getByRole('option', { name: 'All statuses', exact: true })
    ).toHaveAttribute('aria-selected', 'true')
    await page
      .getByRole('option', { name: 'Under review', exact: true })
      .hover()
    await expect(page.getByRole('listbox')).toBeInViewport({ ratio: 1 })
    await within(page)
    await capture(page, 'empty-conduct-menu-' + width)
    await page.keyboard.press('Escape')
    await expect(menu).toBeFocused()
    await page
      .getByRole('tab', { name: 'Response notices', exact: true })
      .click()
    await expect(
      page.getByRole('heading', {
        name: 'Nothing needs your response.',
        exact: true
      })
    ).toBeVisible()
    await capture(page, 'empty-response-notices-' + width)
    await page.getByRole('tab', { name: 'Handling desk', exact: true }).click()
  }
  await page
    .getByRole('button', { name: 'Report an observation', exact: true })
    .click()
  await expect(
    page.getByRole('combobox', {
      name: 'Applicable published rule',
      exact: true
    })
  ).toContainText('No published rules yet')
  await expect(
    page.getByRole('button', { name: 'Review private report', exact: true })
  ).toBeDisabled()
  await capture(page, 'no-published-rule-320')
  await page
    .getByRole('button', { name: 'Close incident report', exact: true })
    .click()
  const context = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    tenant = await context.newPage()
  await login(tenant, 'Tenant')
  await navigate(tenant, 'Rules & conduct')
  await expect(
    tenant.getByRole('button', { name: 'Propose a rule', exact: true })
  ).toHaveCount(0)
  await expect(
    tenant.getByRole('tab', { name: 'Your reports', exact: true })
  ).toBeVisible()
  await context.close()
})

test('a supplied rule is reviewed published independently replaced and retired while its frozen original remains', async ({
  page,
  browser
}) => {
  test.setTimeout(60000)
  await login(page)
  await navigate(page, 'Rules & conduct')
  const title = 'Corridor rule ' + Date.now()
  await page
    .getByRole('button', { name: 'Propose a rule', exact: true })
    .click()
  await page
    .getByRole('textbox', { name: 'Rule title', exact: true })
    .fill(title)
  await page
    .getByRole('textbox', { name: 'Supplied rule text', exact: true })
    .fill(
      'A supplied fictional rule keeps shared corridor access clear for residents.'
    )
  await page
    .getByRole('textbox', { name: 'Policy / authority reference', exact: true })
    .fill('Supplied fictional resolution R-001')
  await page.getByLabel('Effective from', { exact: true }).fill('2026-01-01')
  await page
    .getByRole('checkbox', {
      name: 'The supplied policy permits proposing a fine.',
      exact: true
    })
    .check()
  await page
    .getByRole('button', { name: 'Review this rule', exact: true })
    .click()
  await expect(
    page.getByRole('button', { name: 'Submit rule for review', exact: true })
  ).toBeDisabled()
  await capture(page, 'new-rule-reviewed')
  await page
    .getByRole('checkbox', {
      name: 'I reviewed the supplied rule text, dates and authority.',
      exact: true
    })
    .check()
  const saved = page.waitForResponse(
    (r) => r.url().endsWith('/api/rules') && r.request().method() === 'POST'
  )
  await page
    .getByRole('button', { name: 'Submit rule for review', exact: true })
    .click()
  const id = (await (await saved).json()).id
  await expect(
    page.getByText('A different operator must publish your proposed rule.', {
      exact: true
    })
  ).toBeVisible()
  await page
    .getByRole('combobox', { name: 'Rule decision', exact: true })
    .click()
  await expect(
    page.getByRole('option', { name: 'Publish rule', exact: true })
  ).toHaveCount(0)
  await page.keyboard.press('Escape')
  const context = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    reviewer = await context.newPage()
  await login(reviewer, 'Committee')
  await reviewer.goto('/#conduct?rule=' + id)
  for (const width of [1440, 768, 375, 320]) {
    await reviewer.setViewportSize({ width, height: 900 })
    const menu = reviewer.getByRole('combobox', {
      name: 'Rule decision',
      exact: true
    })
    await menu.click()
    await reviewer
      .getByRole('option', { name: 'Publish rule', exact: true })
      .hover()
    await capture(reviewer, 'rule-decision-menu-' + width)
    await reviewer.keyboard.press('Escape')
    await expect(menu).toBeFocused()
    await within(reviewer)
  }
  await ruleDecision(reviewer, 'Publish rule')
  await expect(
    reviewer.getByRole('dialog').getByText('Published', { exact: true })
  ).toBeVisible()
  await capture(reviewer, 'published-rule-320')
  await reviewer
    .getByRole('button', { name: 'Prepare replacement', exact: true })
    .click()
  await reviewer
    .getByRole('textbox', { name: 'Supplied rule text', exact: true })
    .fill(
      'A supplied replacement policy preserves the earlier text with earlier cases.'
    )
  await reviewer
    .getByRole('button', { name: 'Review this rule', exact: true })
    .click()
  await reviewer
    .getByRole('checkbox', {
      name: 'I reviewed the supplied rule text, dates and authority.',
      exact: true
    })
    .check()
  const replacementSaved = reviewer.waitForResponse(
    (r) => r.url().endsWith('/api/rules') && r.request().method() === 'POST'
  )
  await reviewer
    .getByRole('button', { name: 'Submit rule for review', exact: true })
    .click()
  const replacement = (await (await replacementSaved).json()).id
  await page.goto('/#conduct?rule=' + replacement)
  await ruleDecision(page, 'Publish rule')
  expect(
    (await (await page.request.get('/api/rules/' + id)).json()).state
  ).toBe('RETIRED')
  await ruleDecision(page, 'Retire rule')
  await expect(
    page.getByRole('dialog').getByText('Retired', { exact: true })
  ).toBeVisible()
  expect(
    (await (await page.request.get('/api/rules/' + id)).json()).text
  ).toContain('keeps shared corridor access clear')
  await context.close()
})

test('a resident tags another flat replaces a bad camera file and retries a lost private report response once', async ({
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
  const title = 'Photo report policy ' + Date.now(),
    rule = await apiRule(page, title)
  await apiRuleAction(reviewer, rule, 'PUBLISHED')
  const residentContext = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    tenant = await residentContext.newPage()
  await login(tenant, 'Tenant')
  await navigate(tenant, 'Rules & conduct')
  await tenant
    .getByRole('button', { name: 'Report an observation', exact: true })
    .click()
  await tenant
    .getByRole('searchbox', { name: 'Find a home', exact: true })
    .fill('A-101')
  await chooseOption(tenant, 'Tagged home', 'Home A-101')
  await tenant
    .getByRole('searchbox', { name: 'Find a rule', exact: true })
    .fill(title)
  await chooseOption(tenant, 'Applicable published rule', title)
  await tenant
    .getByRole('textbox', { name: 'Your observation', exact: true })
    .fill('PRIVATE tenant report observing a fictional corridor obstruction.')
  await tenant.getByLabel('Incident date', { exact: true }).fill('2026-01-02')
  const input = tenant.getByLabel('Supporting picture (optional)', {
    exact: true
  })
  await expect(input).toHaveAttribute('capture', 'environment')
  await input.setInputFiles({
    name: 'bad.png',
    mimeType: 'image/png',
    buffer: Buffer.from('damaged unsupported image')
  })
  await expect(tenant.getByRole('alert')).toContainText('could not be read')
  await capture(tenant, 'camera-file-validation-error')
  await input.setInputFiles({
    name: 'परिसर-scene.png',
    mimeType: 'image/png',
    buffer: scenePNG()
  })
  await expect(tenant.getByRole('dialog').getByRole('status')).toContainText(
    'private picture ready'
  )
  const image = tenant.getByRole('img', {
    name: 'Validated picture supplied for this case',
    exact: true
  })
  await expect
    .poll(() =>
      image.evaluate(
        (el: HTMLImageElement) => el.complete && el.naturalWidth > 0
      )
    )
    .toBe(true)
  await tenant
    .getByRole('button', { name: 'Review private report', exact: true })
    .click()
  await tenant.setViewportSize({ width: 375, height: 900 })
  await within(tenant)
  await capture(tenant, 'private-report-reviewed-375')
  await tenant
    .getByRole('checkbox', {
      name: 'I reviewed the home, rule, date and supporting facts.',
      exact: true
    })
    .check()
  let release!: () => void,
    calls = 0
  const keys: string[] = []
  await tenant.route('**/api/incidents', async (route) => {
    if (route.request().method() !== 'POST') {
      await route.continue()
      return
    }
    keys.push(route.request().postDataJSON().operation_key)
    calls++
    if (calls === 1) {
      await new Promise<void>((resolve) => {
        release = resolve
      })
      await route.fetch()
      await route.abort('failed')
    } else await route.continue()
  })
  await tenant
    .getByRole('button', { name: 'Submit private report', exact: true })
    .click()
  await expect(
    tenant.getByRole('button', { name: 'Close incident report', exact: true })
  ).toBeDisabled()
  await tenant.keyboard.press('Escape')
  await expect(tenant.getByRole('dialog')).toBeVisible()
  await expect.poll(() => typeof release).toBe('function')
  await capture(tenant, 'report-submission-pending-375')
  release()
  await expect(
    tenant.getByRole('button', { name: 'Retry this report', exact: true })
  ).toBeEnabled()
  await expect(
    tenant.getByRole('button', { name: 'Edit report', exact: true })
  ).toBeDisabled()
  await capture(tenant, 'report-submission-lost-response-375')
  await tenant
    .getByRole('button', { name: 'Retry this report', exact: true })
    .click()
  await expect(
    tenant.getByRole('dialog').getByText('Reported', { exact: true })
  ).toBeVisible()
  expect(keys.length).toBe(2)
  expect(keys[0]).toBe(keys[1])
  const list = await (
    await tenant.request.get('/api/incidents?q=' + encodeURIComponent(title))
  ).json()
  expect(list.total).toBe(1)
  const id = list.items[0].id
  const detail = await (await tenant.request.get('/api/incidents/' + id)).json()
  expect(detail.flat_id).toBe('demo-flat-A-101')
  expect(detail.event_total).toBe(1)
  const original = await tenant.request.get(
    '/api/incident-pictures/' + detail.picture_id + '/original'
  )
  expect(
    createHash('sha256')
      .update(await original.body())
      .digest('hex')
  ).toBe(createHash('sha256').update(scenePNG()).digest('hex'))
  const preview = await tenant.request.get(
    '/api/incident-pictures/' + detail.picture_id + '/preview'
  )
  expect((await preview.body()).includes(Buffer.from('PRIVATE GPS'))).toBe(
    false
  )
  await capture(tenant, 'private-report-saved-375')
  await context.close()
  await residentContext.close()
})

test('independent human review shares a frozen notice and one immutable household response without sharing the original allegation', async ({
  page,
  browser
}) => {
  test.setTimeout(60000)
  await login(page)
  const reviewerContext = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    reviewer = await reviewerContext.newPage()
  await login(reviewer, 'Committee')
  const title = 'Fair review policy ' + Date.now(),
    rule = await apiRule(page, title)
  await apiRuleAction(reviewer, rule, 'PUBLISHED')
  const reporterContext = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    tenant = await reporterContext.newPage()
  await login(tenant, 'Tenant')
  const picture = await apiPicture(tenant),
    id = await apiIncident(tenant, rule, { picture_id: picture })
  await page.goto('/#conduct?case=' + id)
  for (const width of [1440, 768, 375, 320]) {
    await page.setViewportSize({ width, height: 900 })
    await page
      .getByRole('dialog')
      .locator('.dialog-scroll')
      .evaluate((content) => content.scrollTo({ top: 0, behavior: 'instant' }))
    await within(page)
    await capture(page, 'incident-close-controls-' + width)
    const menu = page.getByRole('combobox', {
      name: 'Incident decision',
      exact: true
    })
    await menu.click()
    await page
      .getByRole('option', { name: 'Substantiate case', exact: true })
      .hover()
    await expect(page.getByRole('listbox')).toBeInViewport({ ratio: 1 })
    await capture(page, 'incident-decision-menu-' + width)
    await page.keyboard.press('Escape')
    await expect(menu).toBeFocused()
    await within(page)
  }
  await decision(page, 'Private staff note')
  await expect(
    page
      .getByRole('region', { name: 'Incident activity', exact: true })
      .getByText('Private staff note', { exact: true })
  ).toBeVisible()
  const notice = await responseNotice(page)
  const ownerContext = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    owner = await ownerContext.newPage()
  await login(owner, 'Owner')
  expect((await owner.request.get('/api/incidents/' + id)).status()).toBe(404)
  expect(
    (
      await owner.request.get('/api/incident-pictures/' + picture + '/original')
    ).status()
  ).toBe(404)
  await navigate(owner, 'Rules & conduct')
  await owner
    .getByRole('tab', { name: 'Response notices', exact: true })
    .click()
  await owner
    .getByRole('button', {
      name: 'Open response notice Please clarify the corridor observation',
      exact: true
    })
    .click()
  await expect(
    owner
      .getByRole('dialog')
      .getByText(
        'PRIVATE fictional observation and original reporter description.',
        { exact: true }
      )
  ).toHaveCount(0)
  await expect(
    owner.getByRole('dialog').getByText('Demo Tenant A-103', { exact: true })
  ).toHaveCount(0)
  const before = await (
    await owner.request.get('/api/incident-notices/' + notice)
  ).json()
  await apiIncidentAction(page, id, 'NOTE')
  expect(
    await (await owner.request.get('/api/incident-notices/' + notice)).json()
  ).toEqual(before)
  for (const width of [1440, 768, 375, 320]) {
    await owner.setViewportSize({ width, height: 900 })
    await within(owner)
    await capture(owner, 'household-response-notice-' + width)
  }
  await owner
    .getByRole('textbox', { name: 'Your household response', exact: true })
    .fill(
      'PRIVATE household explanation retained for the responding person and handlers.'
    )
  await owner
    .getByRole('checkbox', {
      name: 'I reviewed my response to this notice.',
      exact: true
    })
    .check()
  let release!: () => void
  const keys: string[] = []
  await owner.route(
    '**/api/incident-notices/' + notice + '/responses',
    async (route) => {
      keys.push(route.request().postDataJSON().operation_key)
      if (keys.length === 1) {
        await new Promise<void>((resolve) => {
          release = resolve
        })
        await route.fetch()
        await route.abort('failed')
      } else await route.continue()
    }
  )
  await owner
    .getByRole('button', { name: 'Submit household response', exact: true })
    .click()
  await expect(
    owner.getByRole('button', { name: 'Close response notice', exact: true })
  ).toBeDisabled()
  await expect.poll(() => typeof release).toBe('function')
  await capture(owner, 'household-response-pending-320')
  release()
  await expect(
    owner.getByRole('button', { name: 'Retry this response', exact: true })
  ).toBeEnabled()
  await expect(
    owner.getByRole('button', { name: 'Retry this response', exact: true })
  ).toBeInViewport({ ratio: 1 })
  await capture(owner, 'household-response-lost-response-320')
  await owner
    .getByRole('button', { name: 'Retry this response', exact: true })
    .click()
  await expect(
    owner.getByRole('region', { name: 'Household responses', exact: true })
  ).toContainText('PRIVATE household explanation')
  expect(keys[0]).toBe(keys[1])
  expect(
    (await (await owner.request.get('/api/incident-notices/' + notice)).json())
      .response_total
  ).toBe(1)
  expect(
    (await tenant.request.get('/api/incident-notices/' + notice)).status()
  ).toBe(404)
  await page
    .getByRole('button', { name: 'Close incident details', exact: true })
    .click()
  await page.goto('/#conduct?notice=' + notice)
  await expect(
    page.getByRole('region', { name: 'Household responses', exact: true })
  ).toContainText('PRIVATE household explanation')
  await capture(page, 'handler-retained-subject-response')
  await page
    .getByRole('button', { name: 'Close response notice', exact: true })
    .click()
  await page.goto('/#conduct?case=' + id)
  await decision(page, 'Substantiate case')
  await expect(
    page.getByRole('dialog').getByText('Substantiated', { exact: true })
  ).toBeVisible()
  await capture(page, 'substantiated-case-no-charge')
  await ownerContext.close()
  await reporterContext.close()
  await reviewerContext.close()
})

test('independent decline withdrawal and replacement preparation retain rule history through scoped detail errors and dismissal', async ({
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
  const title = 'Declined supplied rule ' + Date.now(),
    declined = await apiRule(reviewer, title)
  await page.goto('/#conduct?rule=' + declined)
  await ruleDecision(page, 'Decline rule')
  await expect(
    page.getByRole('dialog').getByText('Declined', { exact: true })
  ).toBeVisible()
  await expect(
    page.getByRole('region', { name: 'Incident activity', exact: true })
  ).toContainText('Rule declined')
  const withdrawnTitle = 'Withdrawn supplied rule ' + Date.now(),
    withdrawn = await apiRule(page, withdrawnTitle)
  await page.goto('/#conduct?rule=' + withdrawn)
  await ruleDecision(page, 'Withdraw rule')
  await expect(
    page.getByRole('dialog').getByText('Withdrawn', { exact: true })
  ).toBeVisible()
  const publishedTitle = 'Replacement dismissal rule ' + Date.now(),
    published = await apiRule(page, publishedTitle)
  await apiRuleAction(reviewer, published, 'PUBLISHED')
  await page
    .getByRole('button', { name: 'Close rule details', exact: true })
    .click()
  await page.getByRole('tab', { name: 'Community rules', exact: true }).click()
  await page
    .getByRole('searchbox', { name: 'Search community rules', exact: true })
    .fill(publishedTitle)
  const opener = page.getByRole('button', {
    name: 'Open rule ' + publishedTitle,
    exact: true
  })
  await expect(opener).toBeVisible()
  await opener.click()
  await page
    .getByRole('button', { name: 'Prepare replacement', exact: true })
    .click()
  await page.setViewportSize({ width: 320, height: 568 })
  await page
    .getByRole('textbox', { name: 'Supplied rule text', exact: true })
    .fill(
      'Prepared replacement wording retained only until a deliberate submission.'
    )
  await page
    .getByRole('button', { name: 'Review this rule', exact: true })
    .click()
  await within(page)
  await capture(page, 'replacement-review-low-phone')
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(opener).toBeFocused()
  await page.route('**/api/rules/' + published + '?*', (route) =>
    route.fulfill({ status: 503, contentType: 'application/json', body: '{}' })
  )
  await opener.click()
  await expect(
    page.getByRole('heading', { name: 'Rule unavailable', exact: true })
  ).toBeVisible()
  await expect(
    page
      .getByRole('dialog')
      .getByText(
        'Fictional supplied rule: keep shared corridor access clear.',
        { exact: true }
      )
  ).toHaveCount(0)
  await capture(page, 'rule-detail-error-320')
  await page.unroute('**/api/rules/' + published + '?*')
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Try again', exact: true })
    .click()
  await expect(
    page
      .getByRole('dialog')
      .getByRole('heading', { name: publishedTitle, exact: true, level: 2 })
  ).toBeVisible()
  await capture(page, 'rule-detail-retry-320')
  await context.close()
})

test('a handler dismisses reopens and deliberately links a bounded original while its chosen case survives paging', async ({
  page,
  browser
}) => {
  test.setTimeout(60000)
  await login(page)
  const context = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    reporter = await context.newPage()
  await login(reporter, 'Committee')
  const title = 'Human duplicate selection rule ' + Date.now(),
    rule = await apiRule(page, title)
  await apiRuleAction(reporter, rule, 'PUBLISHED')
  const ids: string[] = []
  for (let i = 0; i < 14; i++) ids.push(await apiIncident(reporter, rule))
  const id = ids[0]
  await page.goto('/#conduct?case=' + id)
  await decision(page, 'Dismiss case')
  await expect(
    page.getByRole('dialog').getByText('Dismissed', { exact: true })
  ).toBeVisible()
  await decision(page, 'Reopen review')
  await expect(
    page.getByRole('dialog').getByText('Under review', { exact: true })
  ).toBeVisible()
  await chooseOption(page, 'Incident decision', 'Link a duplicate case')
  await page
    .getByRole('searchbox', { name: 'Find the original case', exact: true })
    .fill(title)
  const page1 = await (
    await page.request.get(
      '/api/incidents?' + new URLSearchParams({ q: title, page: '1' })
    )
  ).json()
  const target = page1.items.find((item: { id: string }) => item.id !== id)
  const choice = 'A-101 · 2 Jan 2026 · ' + target.id.slice(0, 6)
  await chooseOption(page, 'Retained original case', choice)
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Next original cases page', exact: true })
    .click()
  await expect(
    page.getByRole('dialog').getByText('Page 2 of 2', { exact: true })
  ).toBeVisible()
  await expect(
    page.getByRole('combobox', { name: 'Retained original case', exact: true })
  ).toContainText(choice)
  await page.setViewportSize({ width: 320, height: 700 })
  await page
    .getByRole('combobox', { name: 'Retained original case', exact: true })
    .click()
  await expect(
    page.getByRole('option', { name: choice, exact: true })
  ).toHaveAttribute('aria-selected', 'true')
  await capture(page, 'duplicate-original-retained-menu-320')
  await page.keyboard.press('Escape')
  await within(page)
  await page
    .getByRole('textbox', { name: 'Reason for this decision', exact: true })
    .fill(
      'The supplied fictional observation duplicates the deliberately selected retained original.'
    )
  await page
    .getByRole('checkbox', {
      name: 'I reviewed this decision and its intended audience.',
      exact: true
    })
    .check()
  await page
    .getByRole('button', { name: 'Save incident decision', exact: true })
    .click()
  const retained = await (await page.request.get('/api/incidents/' + id)).json()
  expect(retained.state).toBe('DISMISSED')
  expect(retained.duplicate_of).toBe(target.id)
  await expect(
    page.getByRole('button', { name: 'Open retained original', exact: true })
  ).toBeVisible()
  await capture(page, 'human-linked-duplicate-retained-320')
  await context.close()
})

test('home and rule choices retry locally and preserve deliberate selections across bounded pages at four widths', async ({
  page,
  browser
}) => {
  test.setTimeout(120000)
  await login(page)
  const context = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    reviewer = await context.newPage()
  await login(reviewer, 'Committee')
  const prefix = 'Bounded corridor rules ' + Date.now()
  for (let i = 1; i <= 13; i++) {
    const id = await apiRule(page, prefix + ' ' + String(i).padStart(2, '0'))
    await apiRuleAction(reviewer, id, 'PUBLISHED')
  }
  const residentContext = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    tenant = await residentContext.newPage()
  await login(tenant, 'Tenant')
  await navigate(tenant, 'Rules & conduct')
  let release!: () => void,
    homeCalls = 0,
    ruleCalls = 0
  await tenant.route('**/api/incidents/options?*', async (route) => {
    homeCalls++
    if (homeCalls === 1) {
      await new Promise<void>((resolve) => {
        release = resolve
      })
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: '{}'
      })
    } else await route.continue()
  })
  await tenant.route('**/api/rules?*', async (route) => {
    ruleCalls++
    if (ruleCalls === 1)
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: '{}'
      })
    else await route.continue()
  })
  await tenant
    .getByRole('button', { name: 'Report an observation', exact: true })
    .click()
  await expect(
    tenant.getByRole('combobox', { name: 'Tagged home', exact: true })
  ).toBeDisabled()
  await expect(
    tenant.getByRole('button', { name: 'Review private report', exact: true })
  ).toBeDisabled()
  await expect.poll(() => typeof release).toBe('function')
  await capture(tenant, 'report-choices-loading')
  release()
  await expect(
    tenant.getByRole('button', { name: 'Retry home choices', exact: true })
  ).toBeVisible()
  await expect(
    tenant.getByRole('button', { name: 'Retry rule choices', exact: true })
  ).toBeVisible()
  await capture(tenant, 'report-choices-error')
  await tenant
    .getByRole('button', { name: 'Retry home choices', exact: true })
    .click()
  await tenant
    .getByRole('button', { name: 'Retry rule choices', exact: true })
    .click()
  await tenant
    .getByRole('searchbox', { name: 'Find a home', exact: true })
    .fill('A-101')
  await chooseOption(tenant, 'Tagged home', 'Home A-101')
  await tenant
    .getByRole('searchbox', { name: 'Find a home', exact: true })
    .fill('C-1002')
  await expect(
    tenant.getByRole('combobox', { name: 'Tagged home', exact: true })
  ).toContainText('Home A-101')
  await tenant
    .getByRole('searchbox', { name: 'Find a rule', exact: true })
    .fill(prefix)
  const chosenRule = (
    await (
      await tenant.request.get(
        '/api/rules?' + new URLSearchParams({ q: prefix, state: 'PUBLISHED' })
      )
    ).json()
  ).items[0].title as string
  await chooseOption(tenant, 'Applicable published rule', chosenRule)
  await tenant
    .getByRole('button', { name: 'Next rule choices page', exact: true })
    .click()
  await expect(
    tenant.getByRole('dialog').getByText('Page 2 of 2', { exact: true })
  ).toBeVisible()
  await expect(
    tenant.getByRole('combobox', {
      name: 'Applicable published rule',
      exact: true
    })
  ).toContainText(chosenRule)
  for (const width of [1440, 768, 375, 320]) {
    await tenant.setViewportSize({ width, height: 700 })
    const menu = tenant.getByRole('combobox', {
      name: 'Applicable published rule',
      exact: true
    })
    await menu.click()
    await expect(
      tenant.getByRole('option', { name: chosenRule, exact: true })
    ).toHaveAttribute('aria-selected', 'true')
    await within(tenant)
    await expect(tenant.getByRole('listbox')).toBeInViewport({ ratio: 1 })
    await capture(tenant, 'report-rule-selection-page2-' + width)
    await tenant.keyboard.press('Escape')
    await expect(menu).toBeFocused()
  }
  await tenant
    .getByRole('textbox', { name: 'Your observation', exact: true })
    .fill('A deliberately retained observation while choices were retried.')
  await tenant.getByLabel('Incident date', { exact: true }).fill(careDate(1))
  expect(
    await tenant
      .getByLabel('Incident date', { exact: true })
      .evaluate((el: HTMLInputElement) => el.validity.rangeOverflow)
  ).toBe(true)
  await tenant
    .getByRole('button', { name: 'Review private report', exact: true })
    .click()
  await expect(
    tenant.getByRole('button', { name: 'Review private report', exact: true })
  ).toBeVisible()
  await tenant.getByLabel('Incident date', { exact: true }).fill('2026-01-02')
  await tenant
    .getByRole('button', { name: 'Review private report', exact: true })
    .click()
  await expect(
    tenant.getByRole('heading', { name: 'Review your report.', exact: true })
  ).toBeFocused()
  await capture(tenant, 'retained-report-selections-reviewed-320')
  await tenant.keyboard.press('Escape')
  await expect(tenant.getByRole('dialog')).toHaveCount(0)
  await residentContext.close()
  await context.close()
})

test('an uncertain picture upload locks dismissal and retries the same original while previews and downloads recover independently', async ({
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
  const title = 'Picture retry rule ' + Date.now(),
    rule = await apiRule(page, title)
  await apiRuleAction(reviewer, rule, 'PUBLISHED')
  await navigate(page, 'Rules & conduct')
  await page
    .getByRole('button', { name: 'Report an observation', exact: true })
    .click()
  const before = fixtureSQL('SELECT COUNT(*) FROM incident_pictures')[0][0]
  let release!: () => void
  const keys: string[] = []
  await page.route('**/api/incident-pictures/*/preview?*', (route) =>
    route.fulfill({ status: 503, body: '' })
  )
  await page.route('**/api/incident-pictures', async (route) => {
    keys.push(route.request().headers()['x-operation-key'])
    if (keys.length === 1) {
      await new Promise<void>((resolve) => {
        release = resolve
      })
      await route.fetch()
      await route.abort('failed')
    } else await route.continue()
  })
  const input = page.getByLabel('Supporting picture (optional)', {
    exact: true
  })
  await input.setInputFiles({
    name: 'lost-picture.png',
    mimeType: 'image/png',
    buffer: scenePNG()
  })
  await expect(
    page.getByRole('button', { name: 'Close incident report', exact: true })
  ).toBeDisabled()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect.poll(() => typeof release).toBe('function')
  release()
  await expect(
    page.getByRole('button', { name: 'Retry this picture', exact: true })
  ).toBeEnabled()
  await expect(input).toBeDisabled()
  await expect(
    page.getByRole('button', { name: 'Close incident report', exact: true })
  ).toBeDisabled()
  await page.setViewportSize({ width: 320, height: 700 })
  await capture(page, 'picture-lost-response-320')
  await page
    .getByRole('button', { name: 'Retry this picture', exact: true })
    .click()
  await expect(page.getByRole('dialog').getByRole('status')).toContainText(
    'private picture ready'
  )
  expect(keys[0]).toBe(keys[1])
  expect(fixtureSQL('SELECT COUNT(*) FROM incident_pictures')[0][0]).toBe(
    before + 1
  )
  const id = fixtureSQL('SELECT id FROM incident_pictures WHERE filename=?', [
    'lost-picture.png'
  ])[0][0] as string
  await chooseOption(page, 'Tagged home', 'Home A-101')
  await page
    .getByRole('searchbox', { name: 'Find a rule', exact: true })
    .fill(title)
  await chooseOption(page, 'Applicable published rule', title)
  await page.getByLabel('Incident date', { exact: true }).fill('2026-01-02')
  await page
    .getByRole('textbox', { name: 'Your observation', exact: true })
    .fill('A fictional private report with a retained picture.')
  await page
    .getByRole('button', { name: 'Review private report', exact: true })
    .click()
  await expect(
    page.getByRole('button', { name: 'Retry picture', exact: true })
  ).toBeVisible()
  await capture(page, 'picture-preview-error-320')
  await page.unroute('**/api/incident-pictures/*/preview?*')
  await page.getByRole('button', { name: 'Retry picture', exact: true }).click()
  await expect
    .poll(() =>
      page
        .getByRole('img', {
          name: 'Validated picture supplied for this case',
          exact: true
        })
        .evaluate(
          (el: HTMLImageElement) => el.complete && el.naturalWidth === 128
        )
    )
    .toBe(true)
  expect(
    (
      await page.request.get('/api/incident-pictures/' + id + '/preview')
    ).headers()['cache-control']
  ).toContain('no-store')
  const downloading = page.waitForEvent('download')
  await page
    .getByRole('link', { name: 'Download original', exact: true })
    .click()
  const download = await downloading
  expect(download.suggestedFilename()).toBe('lost-picture.png')
  const stream = await download.createReadStream()
  const bytes: Buffer[] = []
  for await (const chunk of stream!) bytes.push(chunk as Buffer)
  expect(Buffer.concat(bytes)).toEqual(scenePNG())
  await expect(
    page.getByRole('button', { name: 'Close incident report', exact: true })
  ).toBeEnabled()
  await capture(page, 'picture-preview-and-original-recovered-320')
  await context.close()
})

test('requested clarification preserves the original facts and rejects a changed home until the frozen household notice is removed', async ({
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
  const title = 'Correction policy ' + Date.now(),
    rule = await apiRule(page, title)
  await apiRuleAction(reviewer, rule, 'PUBLISHED')
  const residentContext = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    tenant = await residentContext.newPage()
  await login(tenant, 'Tenant')
  const id = await apiIncident(tenant, rule)
  await apiIncidentAction(page, id, 'NEEDS_INFO')
  const issued = await apiIncidentAction(page, id, 'ISSUE_NOTICE', {
    title: 'Please clarify the original home',
    body: 'A supplied fictional notice deliberately concerns the original tagged home.',
    response_by: careDate(4)
  })
  const before = await (await tenant.request.get('/api/incidents/' + id)).json()
  await apiIncidentAction(page, id, 'NOTE')
  expect(
    await (await tenant.request.get('/api/incidents/' + id)).json()
  ).toEqual(before)
  await tenant.goto('/#conduct?case=' + id)
  await tenant
    .getByRole('button', { name: 'Revise your report', exact: true })
    .click()
  await tenant
    .getByRole('searchbox', { name: 'Find a home', exact: true })
    .fill('A-102')
  await chooseOption(tenant, 'Tagged home', 'Home A-102')
  await tenant
    .getByRole('textbox', { name: 'Your observation', exact: true })
    .fill(
      'Corrected private observation: this concerns the adjacent fictional home.'
    )
  await tenant
    .getByRole('button', { name: 'Review private report', exact: true })
    .click()
  await tenant
    .getByRole('checkbox', {
      name: 'I reviewed the home, rule, date and supporting facts.',
      exact: true
    })
    .check()
  await tenant
    .getByRole('button', { name: 'Submit revised report', exact: true })
    .click()
  await expect(tenant.getByRole('alert')).toContainText(
    'A response notice is active'
  )
  await expect(
    tenant.getByRole('button', { name: 'Edit report', exact: true })
  ).toBeEnabled()
  await tenant.setViewportSize({ width: 375, height: 700 })
  await capture(tenant, 'frozen-notice-correction-denied-375')
  expect(
    (await (await tenant.request.get('/api/incidents/' + id)).json()).flat_id
  ).toBe('demo-flat-A-101')
  await apiIncidentAction(page, id, 'REMOVE_NOTICE')
  const saved = tenant.waitForResponse(
    (response) =>
      response.url().endsWith('/api/incidents/' + id + '/revision') &&
      response.request().method() === 'POST'
  )
  await tenant
    .getByRole('button', { name: 'Submit revised report', exact: true })
    .click()
  expect((await saved).status()).toBe(200)
  const corrected = await (
    await tenant.request.get('/api/incidents/' + id)
  ).json()
  expect(corrected.flat_id).toBe('demo-flat-A-102')
  expect(corrected.comment).toContain('Corrected private observation')
  expect(
    fixtureSQL(
      "SELECT json_extract(snapshot_json,'$.flat_id'),json_extract(snapshot_json,'$.comment') FROM incident_events WHERE incident_id=? AND action='REPORTED'",
      [id]
    )
  ).toEqual([
    [
      'demo-flat-A-101',
      'PRIVATE fictional observation and original reporter description.'
    ]
  ])
  const retained = await (
    await page.request.get('/api/incident-notices/' + issued.notice_id)
  ).json()
  expect(retained.home).toBe('A-101')
  expect(retained.active).toBe(false)
  await capture(tenant, 'private-report-correction-retained-375')
  await decision(tenant, 'Withdraw report')
  expect(
    (await (await tenant.request.get('/api/incidents/' + id)).json()).state
  ).toBe('WITHDRAWN')
  await apiIncidentAction(page, id, 'REOPEN')
  await tenant
    .getByRole('button', { name: 'Refresh report', exact: true })
    .click()
  await expect(
    tenant.getByRole('dialog').getByText('Under review', { exact: true })
  ).toBeVisible()
  await capture(tenant, 'withdrawn-report-reopened-375')
  await residentContext.close()
  await context.close()
})

test('a stale decision reload preserves the written reason but requires review of the newer record after bounded history paging', async ({
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
  const title = 'Stale review policy ' + Date.now(),
    rule = await apiRule(page, title)
  await apiRuleAction(reviewer, rule, 'PUBLISHED')
  const id = await apiIncident(reviewer, rule)
  for (let i = 0; i < 22; i++)
    await apiIncidentAction(page, id, 'NOTE', {
      reason: 'PRIVATE retained independent staff note number ' + i
    })
  await page.goto('/#conduct?case=' + id)
  await chooseOption(page, 'Incident decision', 'Begin review')
  const reason =
    'This prepared human reason must survive paging and a stale-record reload.'
  await page
    .getByRole('textbox', { name: 'Reason for this decision', exact: true })
    .fill(reason)
  const check = page.getByRole('checkbox', {
    name: 'I reviewed this decision and its intended audience.',
    exact: true
  })
  await check.check()
  await page
    .getByRole('button', { name: 'Next incident activity page', exact: true })
    .click()
  await expect(
    page
      .getByRole('region', { name: 'Incident activity', exact: true })
      .getByText('Page 2 of 2', { exact: true })
  ).toBeVisible()
  await expect(
    page.getByRole('textbox', { name: 'Reason for this decision', exact: true })
  ).toHaveValue(reason)
  await expect(check).toBeChecked()
  await apiIncidentAction(reviewer, id, 'WITHDRAWN')
  await page
    .getByRole('button', { name: 'Save incident decision', exact: true })
    .click()
  await expect(
    page.getByRole('button', { name: 'Reload current record', exact: true })
  ).toBeVisible()
  await page.setViewportSize({ width: 320, height: 700 })
  await page.getByRole('button', { name: 'Reload current record', exact: true }).scrollIntoViewIfNeeded()
  await expect(page.getByRole('button', { name: 'Reload current record', exact: true })).toBeInViewport({ ratio: 1 })
  await capture(page, 'stale-incident-decision-320')
  await page
    .getByRole('button', { name: 'Reload current record', exact: true })
    .click()
  await expect(
    page.getByRole('dialog').getByText('Withdrawn', { exact: true })
  ).toBeVisible()
  await expect(
    page.getByRole('textbox', { name: 'Reason for this decision', exact: true })
  ).toHaveValue(reason)
  await expect(check).not.toBeChecked()
  await expect(
    page.getByRole('button', { name: 'Save incident decision', exact: true })
  ).toBeDisabled()
  await page.locator('.incident-decision').scrollIntoViewIfNeeded()
  await expect(check).toBeInViewport({ ratio: 1 })
  await expect(page.getByRole('button', { name: 'Save incident decision', exact: true })).toBeInViewport({ ratio: 1 })
  await capture(page, 'stale-incident-reloaded-review-required-320')
  await context.close()
})

test('following a retained original clears the other record’s prepared decision and preserves independent report paging', async ({
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
  const title = 'Retained original policy ' + Date.now(),
    rule = await apiRule(page, title)
  await apiRuleAction(reviewer, rule, 'PUBLISHED')
  const original = await apiIncident(reviewer, rule),
    duplicate = await apiIncident(reviewer, rule)
  await apiIncidentAction(page, duplicate, 'DUPLICATE', {
    duplicate_of: original
  })
  for (let i = 0; i < 11; i++) await apiIncident(reviewer, rule)
  await navigate(page, 'Rules & conduct')
  await page
    .getByRole('searchbox', { name: 'Search incident reports', exact: true })
    .fill(title)
  await expect(page.getByText('13 records', { exact: true })).toBeVisible()
  await page
    .getByRole('button', { name: 'Next incident reports page', exact: true })
    .click()
  await expect(page.getByText('Page 2 of 2', { exact: true })).toBeVisible()
  expect(await page.locator('.incident-list-card').count()).toBe(1)
  await page.goto('/#conduct?case=' + duplicate)
  await chooseOption(page, 'Incident decision', 'Private staff note')
  await page
    .getByRole('textbox', { name: 'Private staff note', exact: true })
    .fill(
      'Prepared for the duplicate only; do not carry this onto its original.'
    )
  await page
    .getByRole('checkbox', {
      name: 'I reviewed this decision and its intended audience.',
      exact: true
    })
    .check()
  await page
    .getByRole('button', { name: 'Open retained original', exact: true })
    .click()
  await expect
    .poll(() => new URL(page.url()).hash)
    .toBe('#conduct?case=' + original)
  await expect(
    page.getByRole('dialog').getByText('Reported', { exact: true })
  ).toBeVisible()
  await expect(
    page.getByRole('combobox', { name: 'Incident decision', exact: true })
  ).toContainText('Choose the next step…')
  await expect(
    page.getByRole('textbox', { name: 'Reason for this decision', exact: true })
  ).toHaveValue('')
  await expect(
    page.getByRole('checkbox', {
      name: 'I reviewed this decision and its intended audience.',
      exact: true
    })
  ).not.toBeChecked()
  await page.setViewportSize({ width: 375, height: 700 })
  await page.locator('.incident-decision').scrollIntoViewIfNeeded()
  await expect(page.getByRole('combobox', { name: 'Incident decision', exact: true })).toBeInViewport({ ratio: 1 })
  await expect(page.getByRole('button', { name: 'Save incident decision', exact: true })).toBeDisabled()
  await capture(page, 'retained-original-clean-decision-375')
  await context.close()
})

test('ending only the tagged home removes a notice and evidence while an ended reporter retains text without new participation', async ({
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
  const title = 'Ended relationship policy ' + Date.now(),
    rule = await apiRule(page, title)
  await apiRuleAction(reviewer, rule, 'PUBLISHED')
  const residentContext = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    tenant = await residentContext.newPage()
  await login(tenant, 'Tenant')
  const picture = await apiPicture(tenant),
    id = await apiIncident(tenant, rule, { picture_id: picture })
  const issued = await apiIncidentAction(page, id, 'ISSUE_NOTICE', {
    title: 'A private current-home notice',
    body: 'A supplied fictional current-home notice requests the household account.',
    response_by: careDate(4)
  })
  const ownerContext = await browser.newContext({
      baseURL: new URL(page.url()).origin
    }),
    owner = await ownerContext.newPage()
  await login(owner, 'Owner')
  await owner.goto('/#conduct?notice=' + issued.notice_id)
  await expect(
    owner.getByRole('textbox', { name: 'Your household response', exact: true })
  ).toBeVisible()
  const entries = fixtureSQL('SELECT COUNT(*) FROM entries')[0][0],
    receipts = fixtureSQL('SELECT COUNT(*) FROM receipts')[0][0]
  try {
    fixtureSQL(
      "UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101' AND end_date IS NULL",
      [careDate()]
    )
    await owner
      .getByRole('button', { name: 'Refresh notice', exact: true })
      .click()
    await expect(
      owner
        .getByRole('dialog')
        .getByRole('heading', { name: 'Notice unavailable', exact: true })
    ).toBeVisible()
    await expect(
      owner.getByRole('textbox', {
        name: 'Your household response',
        exact: true
      })
    ).toHaveCount(0)
    await expect(
      owner.getByRole('img', {
        name: 'Validated picture supplied for this case',
        exact: true
      })
    ).toHaveCount(0)
    await owner.evaluate(() =>
      window.dispatchEvent(new Event('session-recheck'))
    )
    await expect(owner.getByRole('dialog')).toHaveCount(0)
    expect(
      (
        await owner.request.get('/api/incident-notices/' + issued.notice_id)
      ).status()
    ).toBe(404)
    expect(
      (
        await owner.request.get(
          '/api/incident-pictures/' + picture + '/preview'
        )
      ).status()
    ).toBe(404)
    expect(
      (await (await owner.request.get('/api/auth/me')).json()).can_read_records
    ).toBe(true)
    fixtureSQL(
      "UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-tenant-A-103' AND end_date IS NULL",
      [careDate()]
    )
    await tenant.goto('/#conduct?case=' + id)
    await expect(
      tenant
        .getByRole('dialog')
        .getByText(
          'PRIVATE fictional observation and original reporter description.',
          { exact: true }
        )
    ).toBeVisible()
    await expect(
      tenant.getByRole('button', { name: 'Revise your report', exact: true })
    ).toHaveCount(0)
    await expect(
      tenant.getByRole('img', {
        name: 'Validated picture supplied for this case',
        exact: true
      })
    ).toHaveCount(0)
    expect(
      (
        await tenant.request.get(
          '/api/incident-pictures/' + picture + '/original'
        )
      ).status()
    ).toBe(404)
    expect(
      (
        await tenant.request.post('/api/incidents', {
          headers: await financialHeaders(tenant),
          data: {
            operation_key: crypto.randomUUID(),
            version: 0,
            rule_id: rule,
            flat_id: 'demo-flat-A-101',
            incident_date: '2026-01-02',
            comment: 'Ended reporters cannot add another allegation.',
            picture_id: '',
            confirmed: true
          }
        })
      ).status()
    ).toBe(403)
    await tenant.setViewportSize({ width: 320, height: 700 })
    await capture(tenant, 'ended-reporter-retained-text-320')
    expect(fixtureSQL('SELECT COUNT(*) FROM entries')[0][0]).toBe(entries)
    expect(fixtureSQL('SELECT COUNT(*) FROM receipts')[0][0]).toBe(receipts)
  } finally {
    fixtureSQL(
      "UPDATE flat_memberships SET end_date=NULL WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101' AND end_date=?",
      [careDate()]
    )
    fixtureSQL(
      "UPDATE flat_memberships SET end_date=NULL WHERE resident_id='demo-tenant-A-103' AND end_date=?",
      [careDate()]
    )
    await ownerContext.close()
    await residentContext.close()
    await context.close()
  }
})
