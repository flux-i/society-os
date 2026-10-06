import { test, expect } from '@playwright/test'
import type { Page, Browser } from '@playwright/test'
import { mkdirSync, chmodSync } from 'node:fs'
import { resolve } from 'node:path'
import { execFileSync } from 'node:child_process'
import { login, navigate, chooseOption } from './helpers'
import { financialHeaders } from './maintenance-fixtures'

test.setTimeout(90000)
async function capture(page: Page, name: string) {
  if (!process.env.SOCIETY_CAPTURE_UI) return
  await page.evaluate(() => document.fonts.ready)
  await page.waitForFunction(() => { const root = document.querySelector('.contacts-page'); return !root || Number(getComputedStyle(root).opacity) >= .99 })
  const folder = resolve(process.env.SOCIETY_CONTACT_CAPTURE_ROOT ?? '../reports/local/contacts-review')
  mkdirSync(folder, { recursive: true, mode: 0o700 })
  const path = resolve(folder, name + '.png')
  await page.screenshot({ path, animations: 'disabled' }); chmodSync(path, 0o600)
}
async function within(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  const dialog = page.getByRole('dialog')
  if (await dialog.count()) {
    await expect(dialog.locator('.dialog-close')).toBeInViewport({ ratio: 1 })
    expect(await dialog.evaluate(element => element.querySelector('.dialog-scroll')!.getBoundingClientRect().top - element.querySelector('.dialog-close')!.getBoundingClientRect().bottom)).toBeGreaterThanOrEqual(8)
  }
}
async function actors(page: Page, browser: Browser) {
  await page.setViewportSize({ width: 1440, height: 1000 }); await login(page)
  const reviewerContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), ownerContext = await browser.newContext({ baseURL: new URL(page.url()).origin })
  const reviewer = await reviewerContext.newPage(), owner = await ownerContext.newPage()
  await login(reviewer, 'Committee'); await login(owner, 'Owner')
  return { reviewer, owner, close: async () => { await reviewerContext.close(); await ownerContext.close() } }
}
async function contactPost(page: Page, path: string, data: Record<string, unknown>) {
  const response = await page.request.post(path, { headers: await financialHeaders(page), data: { operation_key: crypto.randomUUID(), confirmed: true, ...data } })
  expect(response.status(), await response.text()).toBe(200); return response.json()
}
async function openContact(page: Page, id: string) {
  await page.goto('/#contacts?person=' + encodeURIComponent(id))
  await expect(page.getByRole('dialog').locator('.contact-state')).toBeVisible()
}
const attestation = 'I reviewed this person, destination, permission and this decision’s effect.'

test('contact cards opened filters selection keyboard and four-width layouts use the approved border and typography', async ({ page }) => {
  await login(page)
  for (const width of [1440, 768, 375, 320]) {
    await page.setViewportSize({ width, height: width <= 375 ? 640 : 1000 })
    await navigate(page, 'Contacts')
    await expect(page.locator('.result-count')).toHaveText('153 people in this view')
    await page.evaluate(() => scrollTo(0, 0))
    await capture(page, 'current-register-' + width)
    const style = await page.locator('.contact-card').first().evaluate(element => ({ border: getComputedStyle(element).borderLeftWidth, font: getComputedStyle(element.querySelector('h3')!).fontFamily }))
    expect(style.border).toBe('1px')
    expect(style.font).toContain('Instrument Serif')
    await page.getByRole('combobox', { name: 'Filter contacts by relationship' }).click()
    await expect(page.getByRole('option', { name: 'All people', exact: true })).toHaveAttribute('aria-selected', 'true')
    await page.getByRole('option', { name: 'Tenants', exact: true }).hover()
    await capture(page, 'opened-relationship-filter-' + width)
    await page.keyboard.press('End'); await page.keyboard.press('Enter')
    await expect(page.locator('.result-count')).toHaveText('35 people in this view')
    await chooseOption(page, 'Filter contacts by wing', 'Wing A')
    await expect(page.locator('.result-count')).toHaveText('12 people in this view')
    await page.getByRole('button', { name: 'Clear', exact: true }).click()
    await expect(page.locator('.result-count')).toHaveText('153 people in this view')
    await page.getByRole('textbox', { name: 'Search contacts by person' }).fill('No matching synthetic person')
    await expect(page.getByRole('heading', { name: 'No people match these choices.' })).toBeVisible()
    await page.getByRole('heading', { name: 'No people match these choices.' }).scrollIntoViewIfNeeded()
    await capture(page, 'filtered-empty-' + width); await within(page)
    await page.getByRole('button', { name: 'Clear', exact: true }).click()
  }
})

test('a resident registers reviewed contact choices and an independent reviewer verifies exact permission without posting money', async ({ page, browser }) => {
  const a = await actors(page, browser)
  try {
    await a.owner.setViewportSize({ width: 375, height: 640 })
    await navigate(a.owner, 'Your preferences')
    await expect(a.owner.locator('.result-count')).toHaveText('1 person in this view')
    await a.owner.getByRole('button', { name: 'Open contact Demo Owner A-101', exact: true }).click()
    await a.owner.getByRole('button', { name: 'Register contact', exact: true }).click()
    await a.owner.getByLabel('International WhatsApp number', { exact: true }).fill('+91 90000 00101')
    await a.owner.getByLabel('Email address', { exact: true }).fill('owner-contact@example.test')
    await chooseOption(a.owner, 'Preferred channel', 'Email')
    await a.owner.getByRole('checkbox', { name: 'Community notices on WhatsApp', exact: true }).check()
    await a.owner.getByRole('checkbox', { name: 'Permitted financial messages by email', exact: true }).check()
    await a.owner.getByLabel('Identity and permission source', { exact: true }).fill('PRIVATE resident permission and destination source C-01')
    await a.owner.getByLabel('Registration reason', { exact: true }).fill('PRIVATE supplied these fictional destinations and deliberate choices.')
    const choice = await a.owner.getByRole('region', { name: 'Communication permission', exact: true }).locator('label').first().evaluate(element => ({ display: getComputedStyle(element).display, gap: parseFloat(getComputedStyle(element).columnGap), height: element.getBoundingClientRect().height }))
    expect(choice.display).toBe('flex'); expect(choice.gap).toBeGreaterThanOrEqual(8); expect(choice.height).toBeGreaterThanOrEqual(44)
    await capture(a.owner, 'resident-registration-fields-375')
    await a.owner.getByRole('button', { name: 'Review contact', exact: true }).click()
    await expect(a.owner.getByRole('button', { name: 'Submit for verification', exact: true })).toBeDisabled()
    await a.owner.getByRole('checkbox', { name: 'I reviewed the destination and recorded permission for these choices.', exact: true }).check()
    await capture(a.owner, 'resident-registration-preview-375')
    await a.owner.getByRole('button', { name: 'Submit for verification', exact: true }).click()
    await expect(a.owner.getByRole('dialog').locator('.contact-state').first()).toHaveText('Awaiting verification')
    await expect(a.owner.getByRole('button', { name: 'Review verification', exact: true })).toHaveCount(0)
    await openContact(a.reviewer, 'demo-owner-A-101')
    await a.reviewer.getByRole('button', { name: 'Review verification', exact: true }).click()
    await a.reviewer.getByRole('textbox', { name: 'Verification reason', exact: true }).fill('PRIVATE checked the supplied person, destinations and individual permissions.')
    await a.reviewer.getByRole('checkbox', { name: attestation, exact: true }).check()
    await capture(a.reviewer, 'independent-verification-preview-1440')
    await a.reviewer.getByRole('button', { name: 'Verify contact and permission', exact: true }).click()
    await expect(a.reviewer.getByRole('dialog').locator('.contact-state').first()).toHaveText('Independently verified')
    const profile = await (await a.owner.request.get('/api/contacts/me')).json()
    expect(profile.phone).toBe('+919000000101')
    expect(profile.eligible).toEqual({ community_whatsapp: true, community_email: false, finance_whatsapp: false, finance_email: true })
    expect(profile.version).toBe(2); expect(profile.event_total).toBe(2)
    const records = await (await page.request.get('/api/entries')).json()
    expect(records.total).toBe(0)
    await within(a.owner); await within(a.reviewer)
  } finally { await a.close() }
})

test('specific immediate opt-out preserves other choices and a changed destination awaits a new independent review', async ({ page, browser }) => {
  const a = await actors(page, browser)
  try {
    const before = await (await a.owner.request.get('/api/contacts/me')).json()
    await contactPost(a.owner, '/api/contacts/me/register', { version: before.version, phone: '+919000000303', email: 'selected-contact@example.test', preferred_channel: 'EMAIL', community_whatsapp: true, community_email: true, finance_whatsapp: true, finance_email: true, consent_source: 'PRIVATE independently supplied permission C-03', reason: 'PRIVATE deliberately supplied all four individual communication permissions.' })
    let current = await (await a.reviewer.request.get('/api/contacts/demo-owner-A-101')).json()
    await contactPost(a.reviewer, '/api/contacts/' + current.id + '/actions', { version: current.version, action: 'VERIFIED', reason: 'PRIVATE checked the actual supplied person, destinations and permissions.' })
    await a.owner.setViewportSize({ width: 375, height: 640 }); await openContact(a.owner, 'me')
    await a.owner.getByRole('button', { name: 'Stop messages', exact: true }).click()
    await chooseOption(a.owner, 'Channel to stop', 'WhatsApp')
    await chooseOption(a.owner, 'Messages to stop', 'Financial messages')
    await a.owner.getByLabel('Change reason', { exact: true }).fill('Please stop this selected financial channel immediately.')
    await a.owner.getByRole('checkbox', { name: attestation, exact: true }).check()
    await chooseOption(a.owner, 'Messages to stop', 'Community notices')
    await expect(a.owner.getByRole('checkbox', { name: attestation, exact: true })).not.toBeChecked()
    await chooseOption(a.owner, 'Messages to stop', 'Financial messages')
    await a.owner.getByRole('checkbox', { name: attestation, exact: true }).check()
    await capture(a.owner, 'specific-stop-preview-375')
    await a.owner.getByRole('button', { name: 'Stop selected messages', exact: true }).click()
    await expect(a.owner.getByRole('status').filter({hasText:'Selected messages stopped.'})).toHaveText('Selected messages stopped.')
    current = await (await a.owner.request.get('/api/contacts/me')).json()
    expect(current.state).toBe('VERIFIED')
    expect(current.eligible).toEqual({ community_whatsapp: true, community_email: true, finance_whatsapp: false, finance_email: true })
    const verifiedBy = current.reviewed_by
    await a.owner.getByRole('button', { name: 'Change contact choices', exact: true }).click()
    await a.owner.getByLabel('International WhatsApp number', { exact: true }).fill('+919000000404')
    await a.owner.getByLabel('Registration reason', { exact: true }).fill('PRIVATE this new destination needs a new independent identity review.')
    await a.owner.getByRole('button', { name: 'Review contact', exact: true }).click()
    await a.owner.getByRole('checkbox', { name: 'I reviewed the destination and recorded permission for these choices.', exact: true }).check()
    await a.owner.getByRole('button', { name: 'Submit for verification', exact: true }).click()
    await expect(a.owner.getByRole('dialog').locator('.contact-state').first()).toHaveText('Awaiting verification')
    const changed = await (await a.owner.request.get('/api/contacts/me')).json()
    expect(changed.eligible).toEqual({ community_whatsapp: false, community_email: false, finance_whatsapp: false, finance_email: false })
    expect(changed.events[1].snapshot.reviewed_by).toBe(verifiedBy)
    expect(changed.events[1].snapshot.phone).toBe('+919000000303')
    await capture(a.owner, 'changed-destination-pending-375'); await within(a.owner)
  } finally { await a.close() }
})

async function pendingContact(owner: Page, phone = '+919000000505') {
  const x = await (await owner.request.get('/api/contacts/me')).json()
  return contactPost(owner, '/api/contacts/me/register', { version: x.version, phone, email: 'pending-contact@example.test', preferred_channel: 'EMAIL', community_whatsapp: true, community_email: true, finance_whatsapp: false, finance_email: true, consent_source: 'PRIVATE supplied person identity and independent permission C-05', reason: 'PRIVATE prepared this specific destination for a separate verification.' })
}
test('unknown verification outcome freezes dismissal and retries the identical operation exactly once', async ({ page, browser }) => {
  const a = await actors(page, browser)
  try {
    const { id } = await pendingContact(a.owner)
    await openContact(a.reviewer, id)
    await a.reviewer.getByRole('button', { name: 'Review verification', exact: true }).click()
    await a.reviewer.getByRole('textbox', { name: 'Verification reason', exact: true }).fill('PRIVATE reviewed this actual person and deliberate destinations separately.')
    await a.reviewer.getByRole('checkbox', { name: attestation, exact: true }).check()
    const original = await (await a.reviewer.request.get('/api/contacts/' + id)).json()
    const bodies: string[] = []
    await a.reviewer.route('**/api/contacts/' + id + '/actions', async route => {
      bodies.push(route.request().postData()!)
      if (bodies.length === 1) { await route.fetch(); await route.abort('failed') } else await route.continue()
    })
    await a.reviewer.getByRole('button', { name: 'Verify contact and permission', exact: true }).click()
    await expect(a.reviewer.getByRole('button', { name: 'Retry this decision', exact: true })).toBeVisible()
    await expect(a.reviewer.getByRole('button', { name: 'Close contact', exact: true })).toBeDisabled()
    await expect(a.reviewer.getByRole('textbox', { name: 'Verification reason', exact: true })).toBeDisabled()
    await a.reviewer.keyboard.press('Escape'); await expect(a.reviewer.getByRole('dialog')).toBeVisible()
    await capture(a.reviewer, 'unknown-verification-lock-1440')
    await a.reviewer.locator('.dialog-scroll').evaluate(element => { element.scrollTop = element.scrollHeight })
    await expect(a.reviewer.getByRole('button', { name: 'Retry this decision', exact: true })).toBeInViewport({ ratio: 1 })
    await capture(a.reviewer, 'unknown-verification-retry-scrolled-1440')
    await a.reviewer.getByRole('button', { name: 'Retry this decision', exact: true }).click()
    await expect(a.reviewer.getByRole('dialog').locator('.contact-state').first()).toHaveText('Independently verified')
    expect(bodies).toHaveLength(2); expect(bodies[0]).toBe(bodies[1])
    const current = await (await a.reviewer.request.get('/api/contacts/' + id)).json()
    expect(current.version).toBe(original.version + 1); expect(current.event_total).toBe(original.event_total + 1)
  } finally { await a.close() }
})
test('a stale contact decision reloads the new destination with unchecked attestation while preserving the human reason', async ({ page, browser }) => {
  const a = await actors(page, browser)
  try {
    const { id } = await pendingContact(a.owner)
    await a.reviewer.setViewportSize({ width: 375, height: 640 }); await openContact(a.reviewer, id)
    await a.reviewer.getByRole('button', { name: 'Review verification', exact: true }).click()
    const marker = 'PRIVATE human verification reason must survive the explicit stale reload.'
    await a.reviewer.getByRole('textbox', { name: 'Verification reason', exact: true }).fill(marker)
    await a.reviewer.getByRole('checkbox', { name: attestation, exact: true }).check()
    await pendingContact(a.owner, '+919000000606')
    await a.reviewer.getByRole('button', { name: 'Verify contact and permission', exact: true }).click()
    await expect(a.reviewer.getByRole('button', { name: 'Reload current record', exact: true })).toBeVisible()
    await capture(a.reviewer, 'stale-review-blocked-375')
    await a.reviewer.getByRole('button', { name: 'Reload current record', exact: true }).click()
    await expect(a.reviewer.getByRole('textbox', { name: 'Verification reason', exact: true })).toHaveValue(marker)
    await expect(a.reviewer.getByRole('checkbox', { name: attestation, exact: true })).not.toBeChecked()
    await expect(a.reviewer.getByRole('dialog').locator('.contact-destinations')).not.toContainText('+919000000505')
    await expect(a.reviewer.getByRole('dialog').locator('.contact-destinations')).toContainText('+919000000606')
    await expect(a.reviewer.getByRole('button', { name: 'Verify contact and permission', exact: true })).toBeDisabled()
    await capture(a.reviewer, 'fresh-review-unchecked-375')
    await a.reviewer.locator('.dialog-scroll').evaluate(element => { element.scrollTop = element.scrollHeight })
    await within(a.reviewer)
    await capture(a.reviewer, 'fresh-review-reason-unchecked-scrolled-375')
  } finally { await a.close() }
})
test('list and detail loading failures use local retry and current-home denial never reveals another resident contact', async ({ page, browser }) => {
  const a = await actors(page, browser)
  try {
    await page.setViewportSize({ width: 320, height: 640 })
    let failed = false
    await page.route('**/api/contacts?*', async route => { if (!failed) { failed = true; await route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":"unavailable"}' }) } else await route.continue() })
    await navigate(page, 'Contacts'); await expect(page.getByRole('heading', { name: 'Contacts could not be opened.' })).toBeVisible()
    await page.getByRole('heading', { name: 'Contacts could not be opened.' }).scrollIntoViewIfNeeded()
    await expect(page.locator('.contact-card')).toHaveCount(0); await capture(page, 'list-error-retry-320')
    await page.getByRole('button', { name: 'Try again', exact: true }).click(); await expect(page.locator('.contact-card')).not.toHaveCount(0)
    await page.unroute('**/api/contacts?*')
    let release: () => void = () => {}; const held = new Promise<void>(resolve => { release = resolve })
    await page.route('**/api/contacts/demo-owner-A-101?*', async route => { await held; await route.continue() })
    await page.getByRole('button', { name: 'Open contact Demo Owner A-101', exact: true }).click()
    await expect(page.getByRole('status')).toHaveText('Opening the current record…'); await capture(page, 'detail-loading-320'); release()
    await expect(page.getByRole('dialog').locator('.contact-state')).toBeVisible(); await page.keyboard.press('Escape')
    await page.unroute('**/api/contacts/demo-owner-A-101?*')
    const tenantContext = await browser.newContext({ baseURL: new URL(page.url()).origin }), tenant = await tenantContext.newPage()
    try {
      await login(tenant, 'Tenant'); await tenant.goto('/#contacts?person=demo-owner-A-101')
      await expect(tenant.getByRole('heading', { name: 'This record is unavailable.' })).toBeVisible()
      await expect(tenant.getByRole('dialog').locator('.contact-paper')).toHaveCount(0)
      await expect(tenant.getByRole('dialog')).not.toContainText('pending-contact@example.test')
      await capture(tenant, 'other-person-denied-1440')
    } finally { await tenantContext.close() }
  } finally { await a.close() }
})
test('former members retain their own opt-out and history while new registration and delivery eligibility end', async ({ page, browser }) => {
  const a = await actors(page, browser)
  try {
    await pendingContact(a.owner)
    const x = await (await a.reviewer.request.get('/api/contacts/demo-owner-A-101')).json()
    await contactPost(a.reviewer, '/api/contacts/' + x.id + '/actions', { version: x.version, action: 'VERIFIED', reason: 'PRIVATE checked the supplied current destination and permissions.' })
    const db = process.env.SOCIETY_BROWSER_DB!
    execFileSync('python3', ['-c', 'import sqlite3,sys;c=sqlite3.connect(sys.argv[1]);c.execute("update flat_memberships set end_date=\'2026-01-01\' where resident_id=\'demo-owner-A-101\'");c.execute("update sessions set reauthenticated_at=0 where user_id=\'demo-user-owner\'");c.commit()', db])
    await a.owner.setViewportSize({ width: 320, height: 480 }); await openContact(a.owner, 'me')
    await expect(a.owner.getByRole('dialog')).toContainText('This person has no current home relationship.')
    await expect(a.owner.getByRole('button', { name: 'Change contact choices', exact: true })).toHaveCount(0)
    await a.owner.getByRole('button', { name: 'Stop messages', exact: true }).click()
    await a.owner.getByLabel('Change reason', { exact: true }).fill('Stop all communication choices after my current home relationship ended.')
    await a.owner.getByRole('checkbox', { name: attestation, exact: true }).check()
    await a.owner.getByRole('button', { name: 'Stop selected messages', exact: true }).click()
    await expect(a.owner.getByRole('status').filter({hasText:'Selected messages stopped.'})).toHaveText('Selected messages stopped.')
    const current = await (await a.owner.request.get('/api/contacts/me')).json()
    expect(current.eligible).toEqual({ community_whatsapp: false, community_email: false, finance_whatsapp: false, finance_email: false })
    await capture(a.owner, 'former-member-stop-retained-320'); await within(a.owner)
  } finally {
    execFileSync('python3', ['-c', 'import sqlite3,sys;c=sqlite3.connect(sys.argv[1]);c.execute("update flat_memberships set end_date=NULL where resident_id=\'demo-owner-A-101\'");c.commit()', process.env.SOCIETY_BROWSER_DB!])
    await a.close()
  }
})

test('declined and withdrawn contacts retain paged history and dialog controls scroll and dismiss at all four widths', async ({ page, browser }) => {
  const a = await actors(page, browser)
  try {
    await pendingContact(a.owner, '+919000000808')
    await openContact(a.reviewer, 'demo-owner-A-101')
    await a.reviewer.getByRole('button', { name: 'Review verification', exact: true }).click()
    await chooseOption(a.reviewer, 'Verification decision', 'Decline verification')
    await a.reviewer.getByRole('textbox', { name: 'Verification reason', exact: true }).fill('The supplied contact identity could not be independently established.')
    await a.reviewer.getByRole('checkbox', { name: attestation, exact: true }).check()
    await a.reviewer.getByRole('button', { name: 'Decline verification', exact: true }).click()
    await expect(a.reviewer.getByRole('dialog').locator('.contact-state').first()).toHaveText('Verification declined')
    await openContact(a.owner, 'me')
    await a.owner.getByRole('button', { name: 'Withdraw registration', exact: true }).click()
    await a.owner.getByRole('textbox', { name: 'Change reason', exact: true }).fill('Withdrawing this declined registration while retaining its original history.')
    await a.owner.getByRole('checkbox', { name: attestation, exact: true }).check()
    await a.owner.getByRole('button', { name: 'Withdraw registration', exact: true }).click()
    await expect(a.owner.getByRole('dialog').locator('.contact-state').first()).toHaveText('Registration withdrawn')
    let x = await (await a.owner.request.get('/api/contacts/me')).json()
    expect(x.eligible).toEqual({ community_whatsapp: false, community_email: false, finance_whatsapp: false, finance_email: false })
    const before = x.event_total
    await a.owner.getByRole('button', { name: 'Close contact', exact: true }).click()
    await expect(a.owner.getByRole('dialog')).toHaveCount(0)
    // Independent repeated decisions provide more than one bounded history page.
    for (let i = 0; i < 23; i++) {
      await contactPost(a.owner, '/api/contacts/me/actions', { version: x.version, action: 'OPTED_OUT', channel: 'ALL', purpose: 'ALL', reason: 'Retained explicit communication opt-out history step ' + (i + 1) })
      x = await (await a.owner.request.get('/api/contacts/me')).json()
    }
    expect(x.event_total).toBe(before + 23)
    for (const width of [1440, 768, 375, 320]) {
      await a.owner.setViewportSize({ width, height: width <= 375 ? 480 : 1000 })
      await a.owner.getByRole('button', { name: 'Open contact Demo Owner A-101', exact: true }).click()
      await expect(a.owner.getByRole('dialog').locator('.contact-state')).toHaveText('Registration withdrawn')
      const activity = a.owner.getByRole('region', { name: 'Contact activity', exact: true })
      await expect(activity.locator('ol > li')).toHaveCount(20)
      await activity.getByRole('button', { name: 'Next contact activity page', exact: true }).click()
      await expect(activity.locator('ol > li').first()).toContainText('Retained explicit communication opt-out history step 3')
      await activity.locator('summary').first().click()
      await expect(activity.getByRole('region', { name: 'Registered contact choices', exact: true }).first()).toContainText('+919000000808')
      await capture(a.owner, 'retained-history-scrolled-' + width)
      await within(a.owner)
      await a.owner.getByRole('button', { name: 'Close contact', exact: true }).click()
      await expect(a.owner.getByRole('dialog')).toHaveCount(0)
      await expect(a.owner).toHaveURL(/#contacts$/)
      await expect(a.owner.getByRole('button', { name: 'Open contact Demo Owner A-101', exact: true })).toBeFocused()
    }
  } finally { await a.close() }
})
