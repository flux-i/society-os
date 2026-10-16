import { test, expect } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { statementActors, statementOriginal, statementWorkbook, statementApprove, statementPublish, openStatement } from './statements-fixtures'
function labelledPDF() {
  const content = 'BT /F1 15 Tf 30 160 Td (Fictional income: INR 432.19) Tj ET'
  const objects = ['<< /Type /Catalog /Pages 2 0 R >>', '<< /Type /Pages /Kids [3 0 R] /Count 1 >>', '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 360 220] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>', '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>', '<< /Length ' + content.length + ' >>\nstream\n' + content + '\nendstream']
  let out = '%PDF-1.7\n'; const offsets = [0]
  objects.forEach((object, index) => { offsets.push(Buffer.byteLength(out)); out += `${index + 1} 0 obj\n${object}\nendobj\n` })
  const start = Buffer.byteLength(out); out += `xref\n0 ${offsets.length}\n0000000000 65535 f \n`
  offsets.slice(1).forEach(offset => { out += String(offset).padStart(10, '0') + ' 00000 n \n' })
  out += `trailer\n<< /Size ${offsets.length} /Root 1 0 R >>\nstartxref\n${start}\n%%EOF\n`
  return Buffer.from(out)
}
import { login } from './helpers'
test.use({ channel: 'chromium' })

test('light statement viewer preserves stored figures, pages CSV, renders XLSX and opens the protected PDF', async ({ page, browser }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  const actors = await statementActors(page, browser)
  const folder = resolve('../reports/local/rollout-review'); mkdirSync(folder, { recursive: true })
  try {
    const csv = Buffer.from('Description,Amount\n' + Array.from({ length: 62 }, (_, i) => `Item ${i + 1},99999999999999.01`).join('\n') + '\n')
    const id = await statementOriginal(page, 'Preview paged income', csv, 'paged-income.csv')
    await openStatement(page, id)
    await page.getByRole('button', { name: 'View original', exact: true }).click()
    const preview = page.getByRole('region', { name: 'Original statement preview', exact: true })
    await expect(preview.getByRole('cell', { name: 'Item 49', exact: true })).toBeVisible()
    await expect(preview.getByRole('cell', { name: 'Item 50', exact: true })).toHaveCount(0)
    await preview.getByRole('button', { name: 'Next worksheet page', exact: true }).click()
    await expect(preview.getByRole('cell', { name: 'Item 50', exact: true })).toBeVisible()
    await expect(preview.getByRole('cell', { name: '99999999999999.01', exact: true })).toHaveCount(13)
    await expect(preview.getByRole('button', { name: 'Next worksheet page', exact: true })).toBeDisabled()
    await page.setViewportSize({ width: 320, height: 568 })
    await preview.scrollIntoViewIfNeeded()
    await page.screenshot({ path: resolve(folder, 'statement-csv-phone.png') })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    let failed = true
    await page.route('**/api/financial-statements/' + id + '/preview?*', async route => { if (failed) await route.fulfill({ status: 503, json: { error: 'unavailable' } }); else await route.continue() })
    await preview.getByRole('button', { name: 'Previous worksheet page', exact: true }).click()
    await expect(preview.getByRole('alert')).toBeVisible()
    failed = false; await preview.getByRole('button', { name: 'Retry preview', exact: true }).click()
    await expect(preview.getByRole('cell', { name: 'Item 1', exact: true })).toBeVisible()
    await page.unroute('**/api/financial-statements/' + id + '/preview?*')
    await page.getByRole('button', { name: 'Close preview', exact: true }).click()
    await expect(preview).toHaveCount(0)
    await page.keyboard.press('Escape')
    const xlsx = await statementOriginal(page, 'Preview workbook', statementWorkbook(), 'workbook.xlsx')
    await openStatement(page, xlsx); await page.getByRole('button', { name: 'View original', exact: true }).click()
    await expect(preview.getByRole('cell', { name: '432.19', exact: true })).toBeVisible()
    await preview.getByRole('combobox', { name: 'Preview worksheet', exact: true }).click()
    await expect(page.getByRole('option', { name: 'Fictional income', exact: true })).toBeVisible()
    await page.keyboard.press('Escape')
    await preview.scrollIntoViewIfNeeded(); await page.screenshot({ path: resolve(folder, 'statement-xlsx-phone.png') })
    await page.keyboard.press('Escape')
    const pdf = await statementOriginal(page, 'Preview PDF original', labelledPDF(), 'statement.pdf')
    await openStatement(page, pdf)
    const response = page.waitForResponse(r => r.url().endsWith('/financial-statements/' + pdf + '/preview'))
    await page.getByRole('button', { name: 'View original', exact: true }).click()
    expect((await response).status()).toBe(200)
    await expect(page.locator('iframe[title="Original PDF preview"]')).toBeVisible()
    await page.setViewportSize({ width: 1440, height: 1000 }); await preview.scrollIntoViewIfNeeded()
    // Chromium's PDF plugin keeps its stream open after painting; networkidle
    // is not a completion signal for this native viewer. Allow one paint settle
    // for the visual capture after the protected response and frame are ready.
    await page.waitForTimeout(750)
    await page.screenshot({ path: resolve(folder, 'statement-pdf-desktop.png') })
    await page.keyboard.press('Escape')
    await statementApprove(actors.reviewer, id); await statementPublish(page, actors.reviewer, id)
    const context = await browser.newContext({ baseURL: new URL(page.url()).origin }); const tenant = await context.newPage()
    await login(tenant, 'Tenant'); await openStatement(tenant, id); await tenant.getByRole('button', { name: 'View original', exact: true }).click()
    await expect(tenant.getByRole('cell', { name: 'Item 1', exact: true })).toBeVisible()
    expect((await tenant.request.get('/api/financial-statements/' + xlsx + '/preview')).status()).toBe(404)
    await context.close()
  } finally { await actors.close() }
})
