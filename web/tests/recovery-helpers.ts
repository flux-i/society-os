import { expect } from '@playwright/test'
import type { Browser, Page } from '@playwright/test'
import { execFileSync, spawn } from 'node:child_process'
import { createHash } from 'node:crypto'
import { chmodSync, closeSync, copyFileSync, mkdirSync, openSync, readFileSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:net'
import { dirname, join } from 'node:path'
import { db } from './registry-import-fixtures'
import { fixtureStatus, reconcileCurrent } from './offsite-helpers'

export const recoveryFolder = () => process.env.SOCIETY_RECOVERY_ARTIFACTS ?? join(process.env.SOCIETY_BROWSER_ARTIFACTS!, 'recovery-originals')
export async function captureRecovery(page: Page, name: string) {
  const root = recoveryFolder(); mkdirSync(root, { recursive: true, mode: 0o700 })
  await page.evaluate(() => document.fonts.ready)
  const path = join(root, name + '.png'); await page.screenshot({ path, animations: 'disabled' }); chmodSync(path, 0o600)
}

export async function recoveredHost(page: Page, browser: Browser, label: string) {
  const root = join(recoveryFolder(), label); mkdirSync(root, { recursive: true, mode: 0o700 })
  await expect.poll(() => db('SELECT state FROM offsite_backup_jobs'), { timeout: 15000 }).toEqual([['UNCERTAIN']])
  const peer = await fixtureStatus(page), rows = db('SELECT id,object_key,encrypted_sha256,encrypted_bytes FROM offsite_backup_jobs')
  expect(peer.puts).toBe(1); expect(peer.objects).toHaveLength(1)
  const object = peer.objects[0]; expect(object.key).toBe(rows[0][1]); expect(object.sha256).toBe(rows[0][2]); expect(object.byte_count).toBe(rows[0][3])
  const runtime = dirname(process.env.SOCIETY_BROWSER_DB!), binary = process.env.SOCIETY_BROWSER_BINARY!
  const cipher = join(root, 'independently-checked-original.age')
  copyFileSync(join(runtime, 'offsite-staging', String(rows[0][0]), 'snapshot.tar.gz.age'), cipher); chmodSync(cipher, 0o600)
  expect(createHash('sha256').update(readFileSync(cipher)).digest('hex')).toBe(object.sha256)
  // The separate reader actually retrieves/decrypts the exact version. Its
  // independent checksum agrees with this unchanged candidate's ciphertext.
  const independent = await reconcileCurrent(page, label + '-exact-version')
  expect(independent.archive.job_id).toBe(rows[0][0])
  const target = join(root, 'host', 'society.db')
  const proof = JSON.parse(execFileSync(binary, ['recover-restore', '--encrypted-file', cipher, '--expected-sha256', object.sha256, '--custody-identities-file', process.env.SOCIETY_OFFSITE_SECOND_IDENTITY!, '--out', target], { encoding: 'utf8' }))
  expect(proof.status).toBe('restored_verified_quarantined'); expect(proof.recovery_required).toBe(true); expect(proof.manifest.schema_version).toBe(32)
  expect(proof.archive.job_id).toBe(rows[0][0])
  for (const key of ['mfa.key', 'messages.key']) { copyFileSync(join(runtime, 'keys', key), join(root, key)); chmodSync(join(root, key), 0o600) }
  const reservation = createServer(); await new Promise<void>((resolve, reject) => { reservation.once('error', reject); reservation.listen(0, '127.0.0.1', resolve) })
  const port = (reservation.address() as { port: number }).port; await new Promise<void>(resolve => reservation.close(() => resolve()))
  const origin = 'http://127.0.0.1:' + port, log = openSync(join(root, 'server.log'), 'w', 0o600)
  const server = spawn(binary, ['serve', '--demo', '--db', target, '--mfa-key-file', join(root, 'mfa.key'), '--message-key-file', join(root, 'messages.key'), '--addr', '127.0.0.1:' + port, '--web-dir', process.env.SOCIETY_BROWSER_WEB_DIR!], { stdio: ['ignore', log, log] })
  const context = await browser.newContext({ baseURL: origin, serviceWorkers: 'block' })
  const restoredPage = await context.newPage()
  await expect.poll(async () => { try { return (await fetch(origin + '/ready')).status } catch { return 0 } }, { timeout: 15000 }).toBe(200)
  writeFileSync(join(root, 'recovery-proof.json'), JSON.stringify({ independent_object: object, authenticated_restore: proof, source_original_preserved: createHash('sha256').update(readFileSync(cipher)).digest('hex') === object.sha256, limits: 'Fictional signed loopback storage and local host; no actual off-host custody or production release.' }, null, 2), { mode: 0o600 })
  return { page: restoredPage, context, origin, target, async close() {
    await context.close()
    if (server.exitCode === null) {
      const exited = new Promise<void>(resolve => server.once('exit', () => resolve())); server.kill('SIGTERM')
      const timer = setTimeout(() => server.kill('SIGKILL'), 6000); await exited; clearTimeout(timer)
    }
    closeSync(log)
  } }
}

// This is a deliberately controlled metadata boundary for held-tool/form
// races, separate from the authenticated operational restore above.
export function insertControlledQuarantine() {
  const epoch = 'Q'.repeat(43)
  db("INSERT INTO recovery_epochs VALUES(?,1,?,1585152,30,'controlled-browser-boundary',unixepoch('now')-60,unixepoch('now'))", [epoch, 'a'.repeat(64)])
  db('INSERT INTO recovery_epoch_heads SELECT COALESCE(MAX(sequence),0)+1,? FROM recovery_epoch_heads', [epoch])
  db("INSERT INTO recovery_external_jobs SELECT ?,'OFFSITE',id,state,'WITHHELD' FROM offsite_backup_jobs WHERE state!='VERIFIED'", [epoch])
  db("INSERT INTO recovery_external_jobs SELECT ?,'MESSAGE',id,state,'WITHHELD' FROM message_deliveries WHERE state IN('QUEUED','CLAIMED','FAILED','UNKNOWN')", [epoch])
}
