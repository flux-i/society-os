import type { Page } from '@playwright/test'
import { chooseOption } from './helpers'

export async function fillNumbering(page: Page, prefix = 'TRV-R2026A') {
  await page.getByLabel('Receipt prefix', { exact: true }).fill(prefix)
  await chooseOption(page, 'Sequence digits', '6 digits')
  await page.getByLabel('First sequence', { exact: true }).fill('1')
  await page.getByLabel('Last sequence', { exact: true }).fill('999999')
  await page.getByLabel('Supplied period', { exact: true }).fill('Supplied fictional period 2026–27')
  await page.getByLabel('Numbering policy evidence', { exact: true }).fill('independent-fictional-numbering-evidence-17')
  await page.getByLabel('Independent custody namespace reference', { exact: true }).fill('fictional-series-custody-namespace')
  await page.getByRole('textbox', { name: 'Preparation reason', exact: true }).fill('The current supplied numbering and independent custody records were checked.')
  await page.getByRole('checkbox', { name: 'I checked the supplied numbering bounds and independent custody evidence.' }).check()
}

export async function fillNumberingDecision(page: Page) {
  await chooseOption(page, 'Numbering decision', 'Approve supplied preparation')
  await page.getByRole('textbox', { name: 'Decision reason', exact: true }).fill('Another current person independently checked the exact supplied numbering and custody evidence.')
  await page.getByRole('textbox', { name: 'Independent review attestation', exact: true }).fill('I independently checked this frozen version and its supplied bounds.')
  await page.getByRole('checkbox', { name: 'I independently checked this exact supplied version and its evidence.' }).check()
}
