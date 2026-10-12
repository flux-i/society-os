import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { chmodSync, mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { db } from './registry-import-fixtures'
import { chooseOption, navigate } from './helpers'

export const offsiteFolder = () => process.env.SOCIETY_OFFSITE_ARTIFACTS ?? join(process.env.SOCIETY_BROWSER_ARTIFACTS!, 'offsite-originals')
export async function captureOffsite(page: Page, name: string) {
  const folder = offsiteFolder(); mkdirSync(folder, { recursive: true, mode: 0o700 })
  await page.evaluate(() => document.fonts.ready)
  const path = join(folder, name + '.png'); await page.screenshot({ path, animations: 'disabled' }); chmodSync(path, 0o600)
}
export async function openProtection(page: Page) {
  await navigate(page, 'Backups & recovery')
  await expect(page.getByRole('heading', { name: 'Your records, safely kept.' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Financial changes paused' })).toBeVisible()
  await expect(page.locator('.backup-attempt')).toHaveCount(1)
  await page.evaluate(() => window.scrollTo(0, 0))
}
export async function fixtureStatus(page: Page) {
  const response = await page.request.get(process.env.SOCIETY_OFFSITE_FIXTURE! + '/_fixture/status')
  expect(response.status()).toBe(200)
  return response.json() as Promise<{ puts: number; gets: number; mode: string; objects: { key: string; version: string; sha256: string; byte_count: number }[] }>
}
export async function reconcileCurrent(page: Page, label = 'independent-custody') {
  const root = offsiteFolder(); mkdirSync(root, { recursive: true, mode: 0o700 })
  const rows = db("SELECT id,object_key,encrypted_sha256,encrypted_bytes,captured_at FROM offsite_backup_jobs WHERE state='UNCERTAIN'")
  expect(rows).toHaveLength(1)
  const fixture = await fixtureStatus(page), original = fixture.objects.find(item => item.key === rows[0][1])!
  expect(original).toBeTruthy(); expect(original.sha256).toBe(rows[0][2]); expect(original.byte_count).toBe(rows[0][3])
  const binary = process.env.SOCIETY_BROWSER_BINARY!, out = join(root, label)
  const proof = JSON.parse(execFileSync(binary, ['offsite-reconcile', '--db', process.env.SOCIETY_BROWSER_DB!, '--offsite-config-file', process.env.SOCIETY_OFFSITE_READER!, '--offsite-fixture', '--custody-identities-file', process.env.SOCIETY_OFFSITE_IDENTITY!, '--backup-id', String(rows[0][0]), '--object-version', original.version, '--out', out, '--reason', 'Independent fictional custodian retrieved, decrypted and checked the original exact version'], { encoding: 'utf8' }))
  expect(proof.status).toBe('exact_version_decrypted_verified_and_reconciled')
  expect(proof.archive.captured_at).toBe(rows[0][4])
  expect(db("SELECT state,object_version,encrypted_sha256,captured_at FROM offsite_backup_jobs WHERE id=?", [rows[0][0]])[0]).toEqual(['VERIFIED', original.version, rows[0][2], rows[0][4]])
  writeFileSync(join(root, label + '-proof.json'), JSON.stringify({ independent_input: rows[0], exact_version: original.version, proof }, null, 2), { mode: 0o600 })
  return proof
}
export async function receivedDraft(page: Page, description: string) {
  await navigate(page, 'Entries')
  await expect(page.getByRole('heading', { name: 'The entry book' })).toBeVisible()
  await page.getByRole('button', { name: 'Add an entry', exact: true }).click()
  await chooseOption(page, 'Entry home', 'Home A-101'); await chooseOption(page, 'Entry type', 'Money received')
  await page.getByRole('textbox', { name: 'Amount in rupees', exact: true }).fill('400.01')
  await page.getByLabel('Entry date', { exact: true }).fill('2026-10-09')
  await page.getByRole('textbox', { name: 'Description', exact: true }).fill(description)
  await page.getByRole('textbox', { name: 'Received from', exact: true }).fill('Fictional current owner')
  await chooseOption(page, 'Received via', 'UPI')
  await page.getByRole('textbox', { name: 'Reference', exact: true }).fill('OFFSITE-RECEIVED-001')
  await page.getByRole('button', { name: 'Save draft for review', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Review this draft.' })).toBeVisible()
}
