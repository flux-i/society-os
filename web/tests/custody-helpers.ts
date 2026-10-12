import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { createHash, randomBytes } from 'node:crypto'
import { chmodSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { db } from './registry-import-fixtures'
import { fixtureStatus } from './offsite-helpers'

export const custodyFolder = () => process.env.SOCIETY_CUSTODY_ARTIFACTS ?? join(process.env.SOCIETY_BROWSER_ARTIFACTS!, 'custody')
export const custodyReason = 'Independent fictional custodian verified and retained the original encrypted copy. ' + 'Reviewed custody evidence remains private. '.repeat(6).slice(0, 200)

export async function captureCustody(page: Page, name: string) {
  const root = custodyFolder(); mkdirSync(root, { recursive: true, mode: 0o700 })
  await page.evaluate(() => document.fonts.ready)
  const path = join(root, name + '.png'); await page.screenshot({ path, animations: 'disabled' }); chmodSync(path, 0o600)
}

export async function exhaustCustodyCandidate(page: Page) {
  await expect.poll(() => db('SELECT state,attempts FROM offsite_backup_jobs'), { timeout: 15000 }).toEqual([['FAILED', 1]])
  const id = String(db('SELECT id FROM offsite_backup_jobs')[0][0])
  for (let attempt = 2; attempt <= 5; attempt++) {
    db('UPDATE offsite_backup_jobs SET available_at=0 WHERE id=?', [id])
    await expect.poll(() => db('SELECT state,attempts FROM offsite_backup_jobs WHERE id=?', [id]), { timeout: 15000 }).toEqual([['FAILED', attempt]])
  }
  const peer = await fixtureStatus(page); expect(peer.puts).toBe(5); expect(peer.objects).toHaveLength(0)
  const original = db('SELECT id,object_key,config_hash,captured_at,state,attempts,available_at,lease_token,lease_until,encrypted_sha256,encrypted_bytes,object_version,acknowledged_at,error_code FROM offsite_backup_jobs WHERE id=?', [id])[0]
  const stage = join(dirname(process.env.SOCIETY_BROWSER_DB!), 'offsite-staging', id, 'snapshot.tar.gz.age')
  expect(createHash('sha256').update(readFileSync(stage)).digest('hex')).toBe(original[9])
  // Three synthetic exhausted history rows model the cap. Their byte custody
  // is not claimed; the actual candidate above has five real signed rejections.
  for (let index = 0; index < 3; index++) {
    const extra = randomBytes(32).toString('base64url')
    db("INSERT INTO offsite_backup_jobs(id,object_key,config_hash,captured_at,state,attempts,available_at,lease_token,lease_until,encrypted_sha256,encrypted_bytes,error_code) VALUES(?,?,?,?,'FAILED',5,0,'',0,?,128,'UPLOAD_DENIED')", [extra, 'recovery/society/fictional-capacity/' + extra, original[2], Number(original[3]) - 120 - index, 'd'.repeat(64)])
  }
  return { id, original, stage }
}

export function archiveCustodyCandidate(id: string, stage: string, label: string, resume = false) {
  const root = custodyFolder(); mkdirSync(root, { recursive: true, mode: 0o700 })
  const output = join(root, label)
  const args = ['offsite-archive', '--db', process.env.SOCIETY_BROWSER_DB!, '--custody-identities-file', resume ? process.env.SOCIETY_OFFSITE_SECOND_IDENTITY! : process.env.SOCIETY_OFFSITE_IDENTITY!, '--backup-id', id, '--out', output, '--reason', custodyReason, ...(resume ? ['--resume-archive'] : ['--encrypted-file', stage])]
  const proof = JSON.parse(execFileSync(process.env.SOCIETY_BROWSER_BINARY!, args, { encoding: 'utf8' }))
  expect(proof.status).toBe('independently_archived_and_disposed')
  const copy = readFileSync(join(output, 'snapshot.tar.gz.age'))
  expect(createHash('sha256').update(copy).digest('hex')).toBe(db('SELECT encrypted_sha256 FROM offsite_backup_jobs WHERE id=?', [id])[0][0])
  writeFileSync(join(root, label + (resume ? '-resume' : '') + '-proof.json'), JSON.stringify(proof, null, 2), { mode: 0o600 })
  return proof
}
