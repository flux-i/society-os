import type { Page } from '@playwright/test'
import { chooseOption } from './helpers'

export async function fillGap(page: Page, amount = '250.01', reference = 'supplied-received-17') {
  await page.getByLabel('Coverage evidence', { exact: true }).fill('Independently supplied fictional book covers the declared interval and identifies outstanding evidence.')
  await page.getByRole('checkbox', { name: 'The supplied evidence covers this whole interval.', exact: true }).check()
  await page.getByLabel('Item 1 book reference', { exact: true }).fill('fictional-account-book-2026')
  await page.getByLabel('Item 1 item reference', { exact: true }).fill(reference)
  await chooseOption(page, 'Item 1 home', 'A-101')
  await page.getByLabel('Item 1 amount', { exact: true }).fill(amount)
  await page.getByLabel('Item 1 evidence', { exact: true }).fill('The supplied book independently records this account money already received.')
  await page.getByLabel('Assessment reason', { exact: true }).fill('Compare the independently supplied account records while keeping originals and unresolved evidence intact.')
  await page.getByRole('checkbox', { name: 'I checked the supplied coverage, exact amounts and unresolved evidence.', exact: true }).check()
}

export async function addUnknownGapItem(page: Page) {
  await page.getByRole('button', { name: 'Add supplied item', exact: true }).click()
  await page.getByLabel('Item 2 book reference', { exact: true }).fill('fictional-fund-book-2026')
  await page.getByLabel('Item 2 item reference', { exact: true }).fill('unknown-fund-attribution-17')
  await chooseOption(page, 'Item 2 kind', 'Evidence only')
  await chooseOption(page, 'Item 2 evidence state', 'Unverified · needs investigation')
  await page.getByLabel('Item 2 evidence', { exact: true }).fill('A supplied voluntary fund attribution is unverified and remains outside the known comparison.')
}

export async function fillGapDecision(page: Page) {
  await chooseOption(page, 'Financial assessment decision', 'Approve supplied assessment')
  await page.getByLabel('Decision reason', { exact: true }).fill('Another current person checked this exact assessment, its known amounts and the unresolved fund evidence.')
  await page.getByLabel('Independent review attestation', { exact: true }).fill('I independently checked this frozen comparison and kept the unverified evidence unresolved.')
  await page.getByRole('checkbox', { name: 'I independently checked this frozen assessment and retained unresolved evidence.', exact: true }).check()
}
