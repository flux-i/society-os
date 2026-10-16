import { expect } from '@playwright/test'
import type { Browser, BrowserContext, Page } from '@playwright/test'
import { execFileSync, spawn } from 'node:child_process'
import { createHash, createHmac } from 'node:crypto'
import { chmodSync, closeSync, copyFileSync, mkdirSync, openSync, readFileSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:net'
import { dirname, join } from 'node:path'
import { login } from './helpers'
import { db } from './registry-import-fixtures'

const onePassword = 'Supplied-reviewer-one-2026!', twoPassword = 'Supplied-reviewer-two-2026!'
export const reviewFolder = () => process.env.SOCIETY_REVIEW_SCREENSHOTS ?? join(process.env.SOCIETY_BROWSER_ARTIFACTS!, 'recovery-review-originals')
export function reviewDB(path: string, query: string, parameters: unknown[] = []) {
  if (!path.includes('society-browser-')) throw new Error('Disposable synthetic review host required.')
  return JSON.parse(execFileSync('python3', ['-c', 'import sqlite3,json,sys\nc=sqlite3.connect(sys.argv[1]);r=c.execute(sys.argv[2],json.loads(sys.argv[3]));rows=r.fetchall();c.commit();print(json.dumps(rows));c.close()', path, query, JSON.stringify(parameters)], { encoding: 'utf8' })) as unknown[][]
}
export async function captureReview(page: Page, name: string) {
  const root = reviewFolder(); mkdirSync(root, { recursive: true, mode: 0o700 })
  await page.evaluate(async () => { await document.fonts.ready; await Promise.allSettled(document.getAnimations().filter(animation => animation.effect?.getComputedTiming().iterations !== Infinity).map(animation => animation.finished)); await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))) })
  const path = join(root, name + '.png'); await page.screenshot({ path, animations: 'disabled' }); chmodSync(path, 0o600)
}
function totp(secret: string) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567', bits = [...secret].map(char => alphabet.indexOf(char).toString(2).padStart(5, '0')).join('')
  const bytes = Buffer.from(bits.match(/.{8}/g)!.map(value => parseInt(value, 2))), counter = Buffer.alloc(8); counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)))
  const mac = createHmac('sha1', bytes).update(counter).digest(), offset = mac.at(-1)! & 15
  return String((mac.readUInt32BE(offset) & 0x7fffffff) % 1000000).padStart(6, '0')
}
export interface ReviewPerson { page: Page; context: BrowserContext; email: string; password: string; recovery: string[] }
export async function loginReviewer(person: ReviewPerson) {
  const page = person.page; await page.goto('/'); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeEnabled()
  await page.getByLabel('Email address', { exact: true }).fill(person.email); await page.getByLabel('Password', { exact: true }).fill(person.password)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Verify and continue', exact: true })).toBeVisible()
  const me = await (await page.request.get('/api/auth/me')).json()
  expect(me.recovery_required).toBe(true); expect(me.recovery_reviewer).toBe(true); expect(me.mfa_pending).toBe(true); expect(me.roles).toEqual([]); expect(me.is_demo).toBe(false)
  await expect(page.getByRole('button', { name: 'Use a preview code', exact: true })).toHaveCount(0)
  if (!me.mfa_enrolled) {
    await page.getByText('Enter a setup key instead', { exact: true }).click(); await expect(page.locator('.setup-key')).not.toBeEmpty()
    await page.getByLabel('Authenticator code', { exact: true }).fill(totp(await page.locator('.setup-key').innerText()))
    await page.getByRole('button', { name: 'Verify and continue', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Keep these close.' })).toBeVisible(); person.recovery = await page.locator('.recovery-grid code').allTextContents()
    await page.getByRole('checkbox', { name: 'I have saved my recovery codes privately.' }).check(); await page.getByRole('button', { name: 'Continue to workspace', exact: true }).click()
  } else {
    const code = person.recovery.shift(); if (!code) throw new Error('Private fictional recovery code exhausted.')
    await page.getByRole('button', { name: 'Use a recovery code', exact: true }).click(); await page.getByLabel('Recovery code', { exact: true }).fill(code)
    await page.getByRole('button', { name: 'Verify and continue', exact: true }).click()
  }
  await expect(page.getByRole('heading', { name: 'Back to community. One careful decision.' })).toBeVisible()
  const current = await (await page.request.get('/api/auth/me')).json(); expect(current.can_review_recovery).toBe(true); expect(current.fresh_authentication).toBe(true)
  for (const key of Object.keys(current).filter(key => key.startsWith('can_') && key !== 'can_review_recovery')) expect(current[key], key).toBe(false)
}

export async function reviewHost(browser: Browser, includeFailedWork = true) {
  const binary = process.env.SOCIETY_BROWSER_BINARY!, runtime = dirname(process.env.SOCIETY_BROWSER_DB!), root = join(reviewFolder(), 'matching-host')
  mkdirSync(root, { recursive: true, mode: 0o700 }); const source = await browser.newPage(); await login(source)
  const me = await (await source.request.get('/api/auth/me')).json(), headers = { Origin: new URL(source.url()).origin, 'X-CSRF-Token': me.csrf_token }
  const entry = async (kind: string, amount: string, post: boolean) => {
    const response = await source.request.post('/api/entries', { headers, data: { operation_key: crypto.randomUUID(), flat_id: 'demo-flat-A-101', kind, amount, date: '2026-10-10', description: 'Independently supplied recovery-review original ' + kind, payer: kind === 'RECEIVED' ? 'Fictional source owner' : '', method: kind === 'RECEIVED' ? 'BANK_TRANSFER' : '', reference: kind === 'RECEIVED' ? 'REVIEW-ORIGINAL-40000' : '' } })
    expect(response.status(), await response.text()).toBe(200); const id = (await response.json()).id
    if (post) expect((await source.request.post('/api/entries/' + id + '/post', { headers, data: { operation_key: crypto.randomUUID(), confirmed: true, reason: '' } })).status()).toBe(200)
    return String(id)
  }
  await entry('OPENING_DEBIT', '1500.01', true); const receiptEntry = await entry('RECEIVED', '400.00', true)
  const original = await (await source.request.get('/api/entries/' + receiptEntry)).json(); expect(original.amount_paise).toBe(40000)
  const receiptID = original.receipt_id
  await expect.poll(async () => (await source.request.get('/api/receipts/' + receiptID + '/download')).status(), { timeout: 15000 }).toBe(200)
  const pdf = await (await source.request.get('/api/receipts/' + receiptID + '/download')).body(), pdfSHA = createHash('sha256').update(pdf).digest('hex')
  expect((await source.request.post('/api/entries/' + receiptEntry + '/reverse', { headers, data: { operation_key: crypto.randomUUID(), confirmed: true, reason: 'Independent source correction retains the original 40000-paise receipt.' } })).status()).toBe(200)
  const draft = await entry('RECEIVED', '400.01', false)
  expect(db("SELECT SUM(CASE WHEN e.kind IN('OPENING_DEBIT','CHARGE') THEN e.amount_paise ELSE -e.amount_paise END) FROM entries e WHERE state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals WHERE entry_id=e.id)")[0][0]).toBe(150001)
  // Controlled synthetic old failed-capture metadata for the visible work desk.
  // No object ACK or external outcome is invented. Actual signed lost-response
  // capture/encrypted recovery remains covered by the separate full suites.
  const oldJob = 'controlled-retained-capture'.padEnd(43, 'x'), now = Math.floor(Date.now() / 1000)
  db("INSERT INTO offsite_policy VALUES(1,?,'FIXTURE',?,?)", ['d'.repeat(64), now - 120, now - 120])
  if (includeFailedWork) db("INSERT INTO offsite_backup_jobs(id,object_key,config_hash,captured_at,state,attempts,available_at,lease_token,lease_until,error_code) VALUES(?,?,?,?,'FAILED',5,0,'',0,'CAPTURE_FAILED')", [oldJob, 'controlled-private-capture/' + oldJob, 'd'.repeat(64), now - 120])
  // Disabled historical reference accounts make real 12-item paging observable.
  // They establish no current identity, password handover or role grants.
  for (let number = 0; number < 13; number++) db("INSERT INTO users(id,login,display_name,password_hash,status,created_at) SELECT ?,?,?,password_hash,'DISABLED',unixepoch('now') FROM users WHERE login='admin@demo.society'", ['controlled-retained-page-' + number, 'retained.page.' + number + '@example.test', 'Retained Paging Person ' + String(number).padStart(2, '0')])
  const bundle = join(root, 'source-bundle'), target = join(root, 'restored', 'society.db')
  const manifest = JSON.parse(execFileSync(binary, ['snapshot', '--db', process.env.SOCIETY_BROWSER_DB!, '--out', bundle], { encoding: 'utf8' }))
  const sourceSHA = createHash('sha256').update(readFileSync(join(bundle, 'society.db'))).digest('hex'); expect(sourceSHA).toBe(manifest.sha256)
  const restored = JSON.parse(execFileSync(binary, ['restore-check', '--snapshot', bundle, '--out', target], { encoding: 'utf8' })); expect(restored.status).toBe('restored_verified_quarantined'); expect(restored.manifest.schema_version).toBe(32)
  const origin = JSON.parse(execFileSync(binary, ['recover-origin', '--db', target], { encoding: 'utf8' })); expect(origin.source_sha256).toBe(manifest.sha256)
  expect(createHash('sha256').update(readFileSync(join(bundle, 'society.db'))).digest('hex')).toBe(sourceSHA)
  for (const name of ['mfa.key', 'messages.key']) { copyFileSync(join(runtime, 'keys', name), join(root, name)); chmodSync(join(root, name), 0o600) }
  const nomination = { format_version: 1, operation_id: 'browser-current-independent-reviewers', epoch_id: origin.id, source_sha256: origin.source_sha256, replaces_operation_id: '', valid_from: now - 1, valid_until: now + 86400,
    custodians: [1, 2].map(number => ({ identity_ref: 'fictional-current-custodian-' + number, evidence_ref: 'supplied-independent-current-authority-' + number, verified_at: now - 2 })),
    reviewers: [1, 2].map(number => ({ email: number === 1 ? 'reviewer.one@example.test' : 'reviewer.two@example.test', name: 'Current Reviewer ' + (number === 1 ? 'One' : 'Two'), account_id: '', identity_ref: 'fictional-distinct-review-person-' + number, evidence_ref: 'supplied-independent-reviewer-identity-' + number, verified_at: now - 2 })) }
  const nominationFile = join(root, 'private-nomination.json'), passwordsFile = join(root, 'private-reviewer-passwords.json')
  writeFileSync(nominationFile, JSON.stringify(nomination), { mode: 0o600 }); writeFileSync(passwordsFile, JSON.stringify({ passwords: [{ email: nomination.reviewers[0].email, password: onePassword }, { email: nomination.reviewers[1].email, password: twoPassword }] }), { mode: 0o600 })
  const nominated = JSON.parse(execFileSync(binary, ['recover-nominate', '--db', target, '--nomination-file', nominationFile, '--reviewer-passwords-file', passwordsFile], { encoding: 'utf8' })); expect(nominated.reviewers).toBe(2)
  const reservation = createServer(); await new Promise<void>((resolve, reject) => { reservation.once('error', reject); reservation.listen(0, '127.0.0.1', resolve) }); const port = (reservation.address() as { port: number }).port; await new Promise<void>(resolve => reservation.close(() => resolve()))
  const address = 'http://127.0.0.1:' + port, log = openSync(join(root, 'server.log'), 'w', 0o600)
  const server = spawn(binary, ['serve', '--demo', '--db', target, '--mfa-key-file', join(root, 'mfa.key'), '--message-key-file', join(root, 'messages.key'), '--addr', '127.0.0.1:' + port, '--web-dir', process.env.SOCIETY_BROWSER_WEB_DIR!], { stdio: ['ignore', log, log] })
  const contexts: BrowserContext[] = []
  const person = async (number: number) => { const context = await browser.newContext({ baseURL: address, serviceWorkers: 'block' }); contexts.push(context); return { context, page: await context.newPage(), email: nomination.reviewers[number - 1].email, password: number === 1 ? onePassword : twoPassword, recovery: [] } as ReviewPerson }
  const close = async () => {
    await Promise.allSettled(contexts.map(context => context.close())); await source.context().close()
    if (server.exitCode === null) { const exited = new Promise<void>(resolve => server.once('exit', () => resolve())); server.kill('SIGTERM'); const timer = setTimeout(() => server.kill('SIGKILL'), 6000); await exited; clearTimeout(timer) }; closeSync(log)
  }
  try {
  const one = await person(1), two = await person(2)
  await expect.poll(async () => { try { return (await fetch(address + '/ready')).status } catch { return 0 } }, { timeout: 15000 }).toBe(200)
  writeFileSync(join(root, 'matching-recovery-proof.json'), JSON.stringify({ manifest, restored, origin, nomination_receipt: nominated, original_receipt_id: receiptID, original_receipt_paise: 40000, original_pdf_sha256: pdfSHA, corrected_balance_paise: 150001, denied_draft_paise: 40001, retained_job: includeFailedWork ? oldJob : null, controlled_disabled_reference_accounts: 13, limits: 'Fictional current-person attestations and local matching restore; controlled historical reference accounts and optional old failed-capture metadata. No actual production authority, external ACK, target-host acceptance or cutover.' }, null, 2), { mode: 0o600 })
  return { one, two, target, root, receiptID, draft, oldJob, nomination, origin, close, renominate() {
    const current = Math.floor(Date.now() / 1000), file = join(root, 'private-replacement-nomination.json'), secrets = join(root, 'private-existing-reviewer-input.json')
    const replacement = { ...nomination, operation_id: 'browser-current-replacement-reviewers', replaces_operation_id: nomination.operation_id, valid_from: current - 1, valid_until: current + 86400, reviewers: nomination.reviewers.map(reviewer => ({ ...reviewer, account_id: String(reviewDB(target, 'SELECT id FROM users WHERE login=?', [reviewer.email])[0][0]), verified_at: current - 1 })) }
    writeFileSync(file, JSON.stringify(replacement), { mode: 0o600 }); writeFileSync(secrets, JSON.stringify({ passwords: [] }), { mode: 0o600 })
    return JSON.parse(execFileSync(binary, ['recover-nominate', '--db', target, '--nomination-file', file, '--reviewer-passwords-file', secrets], { encoding: 'utf8' }))
  }, revoke() {
    const file = join(root, 'private-current-revocation.json'); writeFileSync(file, JSON.stringify({ format_version: 1, operation_id: 'browser-current-nomination-revoked', epoch_id: origin.id, source_sha256: origin.source_sha256, nomination_id: nomination.operation_id, reason: 'Independent supplied custodians withdrew this current review appointment.', custodians: nomination.custodians }), { mode: 0o600 })
    return JSON.parse(execFileSync(binary, ['recover-revoke', '--db', target, '--revocation-file', file], { encoding: 'utf8' }))
  } }
  } catch (error) { await close(); throw error }
}

export async function fillAccessReview(page: Page, label = 'Supplied current household evidence') {
  await page.getByLabel('Person’s name', { exact: true }).fill('Independently Supplied Person')
  await page.getByLabel('Verified email', { exact: true }).fill('current.person@example.test')
  await page.getByLabel('Independent person reference', { exact: true }).fill('independently-supplied-person-17')
  await page.getByRole('combobox', { name: 'Proposed access', exact: true }).click(); await page.getByRole('option', { name: 'Household member', exact: true }).click()
  await page.getByRole('combobox', { name: 'Verified home', exact: true }).click(); await page.getByRole('option', { name: 'A-101', exact: true }).click()
  await page.getByRole('combobox', { name: 'Supplied relationship', exact: true }).click(); await page.getByRole('option', { name: 'Owner', exact: true }).click()
  await page.getByLabel('Independent evidence reference', { exact: true }).fill('independent-current-membership-evidence-17')
  await page.getByLabel('Review notes', { exact: true }).fill(label)
  await page.getByRole('checkbox', { name: 'I checked this supplied evidence and understand this prepares a decision while recovery remains held.' }).check()
}


export async function nestedReviewHost(host: Awaited<ReturnType<typeof reviewHost>>, browser: Browser, numbering = false, gap = false) {
  const binary = process.env.SOCIETY_BROWSER_BINARY!, root = join(host.root, 'nested-current-review'), bundle = join(root, 'source'), target = join(root, 'restored', 'society.db')
  mkdirSync(root, { mode: 0o700 })
  const manifest = JSON.parse(execFileSync(binary, ['snapshot', '--db', host.target, '--out', bundle], { encoding: 'utf8' }))
  expect(manifest.schema_version).toBe(32); expect(manifest.application_version).toBe('0.35.0-dev')
  expect(createHash('sha256').update(readFileSync(join(bundle, 'society.db'))).digest('hex')).toBe(manifest.sha256)
  const restored = JSON.parse(execFileSync(binary, ['restore-check', '--snapshot', bundle, '--out', target], { encoding: 'utf8' })); expect(restored.status).toBe('restored_verified_quarantined')
  const compare = `import sqlite3,json,sys

def enc(v):
 if isinstance(v,bytes):return {'blob_hex':v.hex()}
 raise TypeError(type(v).__name__)
def state(p):
 c=sqlite3.connect('file:'+p+'?mode=ro',uri=True)
 names=[r[0] for r in c.execute("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('sessions','account_tokens','mfa_pending','mfa_recovery_codes') ORDER BY name")]
 rows={n:sorted(json.dumps(r,default=enc,separators=(',',':')) for r in c.execute('SELECT * FROM '+n)) for n in names}
 objects=c.execute("SELECT type,name,sql FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%' ORDER BY type,name").fetchall()
 return c,rows,objects
a,ar,ao=state(sys.argv[1]);b,br,bo=state(sys.argv[2]);assert ao==bo and ar.keys()==br.keys() and len(ar)==119
for n in ar:
 if n not in ('recovery_epochs','recovery_epoch_heads','recovery_external_jobs'):assert ar[n]==br[n],n
for n in ('recovery_epochs','recovery_epoch_heads','recovery_external_jobs'):
 assert set(ar[n])<=set(br[n]),n
assert len(br['recovery_epochs'])==len(ar['recovery_epochs'])+1 and len(br['recovery_epoch_heads'])==len(ar['recovery_epoch_heads'])+1
assert b.execute('SELECT required FROM recovery_quarantine').fetchone()[0]==1
current=b.execute('SELECT epoch_id FROM recovery_epoch_heads ORDER BY sequence DESC LIMIT 1').fetchone()[0]
expected=json.loads(sys.argv[3]);assert b.execute('SELECT source_sha256,source_bytes,source_schema,source_application FROM recovery_epochs WHERE id=?',(current,)).fetchone()==(expected['sha256'],expected['bytes'],32,'0.35.0-dev')
assert b.execute('SELECT COUNT(*) FROM recovery_nominations WHERE epoch_id=?',(current,)).fetchone()[0]==0
for n in ('sessions','account_tokens','mfa_pending','mfa_recovery_codes'):assert b.execute('SELECT COUNT(*) FROM '+n).fetchone()[0]==0
assert b.execute('PRAGMA integrity_check').fetchone()[0]=='ok' and not b.execute('PRAGMA foreign_key_check').fetchall()
assert b.execute('SELECT amount_paise FROM entries WHERE id IN(SELECT entry_id FROM receipts)').fetchone()[0]==40000
assert b.execute("SELECT SUM(CASE WHEN kind IN('OPENING_DEBIT','CHARGE') THEN amount_paise ELSE -amount_paise END) FROM entries WHERE state='POSTED' AND id NOT IN(SELECT entry_id FROM entry_reversals)").fetchone()[0]==150001
print(json.dumps({'persistent_tables_with_provenance':len(ar),'prior_nominations':len(ar['recovery_nominations']),'prior_proposals':len(ar['recovery_proposals']),'prior_decisions':len(ar['recovery_decisions']),'prior_numbering_policies':len(ar['recovery_numbering_policies']),'prior_numbering_decisions':len(ar['recovery_numbering_decisions']),'prior_series_intents':len(ar['recovery_series_intents']),'prior_series_events':len(ar['recovery_series_events']),'prior_gap_preparations':len(ar['recovery_gap_preparations']),'prior_gap_decisions':len(ar['recovery_gap_decisions']),'historical_rows_and_objects_preserved':True,'new_epoch_has_no_current_nomination':True,'receipt_paise':40000,'balance_paise':150001}))`
  const proof = JSON.parse(execFileSync('python3', ['-c', compare, join(bundle, 'society.db'), target, JSON.stringify(manifest)], { encoding: 'utf8' }))
  expect(proof.prior_nominations).toBeGreaterThan(0)
  if (numbering) {
    expect(proof.prior_numbering_policies).toBeGreaterThan(0); expect(proof.prior_numbering_decisions).toBeGreaterThan(0)
    expect(proof.prior_series_intents).toBe(1); expect(proof.prior_series_events).toBe(3)
  } else if (gap) expect(proof.prior_gap_preparations).toBeGreaterThan(0)
  else expect(proof.prior_proposals).toBeGreaterThan(0)
  for (const name of ['mfa.key', 'messages.key']) copyFileSync(join(host.root, name), join(root, name))
  const reservation = createServer(); await new Promise<void>((resolve, reject) => { reservation.once('error', reject); reservation.listen(0, '127.0.0.1', resolve) }); const port = (reservation.address() as { port: number }).port; await new Promise<void>(resolve => reservation.close(() => resolve()))
  const origin = 'http://127.0.0.1:' + port, log = openSync(join(root, 'server.log'), 'w', 0o600)
  const server = spawn(binary, ['serve', '--demo', '--db', target, '--mfa-key-file', join(root, 'mfa.key'), '--message-key-file', join(root, 'messages.key'), '--addr', '127.0.0.1:' + port, '--web-dir', process.env.SOCIETY_BROWSER_WEB_DIR!], { stdio: ['ignore', log, log] })
  const context = await browser.newContext({ baseURL: origin, viewport: { width: 320, height: 440 } }), page = await context.newPage()
  const close = async () => { await context.close(); if (server.exitCode === null) { const exited = new Promise<void>(resolve => server.once('exit', () => resolve())); server.kill('SIGTERM'); const timer = setTimeout(() => server.kill('SIGKILL'), 6000); await exited; clearTimeout(timer) }; closeSync(log) }
  try {
    await expect.poll(async () => { try { return (await page.request.get('/ready')).status() } catch { return 0 } }, { timeout: 15000 }).toBe(200)
    await page.goto('/'); await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeEnabled()
    await page.getByLabel('Email address', { exact: true }).fill(host.one.email); await page.getByLabel('Password', { exact: true }).fill(host.one.password); await page.getByRole('button', { name: 'Sign in', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Your records. Kept together.', exact: true })).toBeVisible()
    const me = await (await page.request.get('/api/auth/me')).json(); expect(me.recovery_required).toBe(true); expect(me.recovery_reviewer).toBe(false); expect(me.can_review_recovery).toBe(false); expect(me.roles).toEqual([])
    for (const path of ['/api/recovery/status', '/api/recovery/proposals', '/api/flats', '/api/receipts/' + host.receiptID + '/download']) expect((await page.request.get(path)).status(), path).toBe(423)
    writeFileSync(join(root, 'nested-recovery-proof.json'), JSON.stringify({ manifest, restored, proof, copied_nomination_withheld: true, limits: 'Actual matching nested operational restore and personal password login with copied nominated identities; all old evidence remains retained, no nomination is current on the new epoch. No real production authority or final cutover.' }, null, 2), { mode: 0o600 })
    return { page, close, root, proof, target }
  } catch (error) { await close(); throw error }
}
