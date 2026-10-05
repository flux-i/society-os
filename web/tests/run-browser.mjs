// A fresh fictional database keeps browser mutations out of the user's preview.
import { spawn, execFileSync } from 'node:child_process'
import { mkdtempSync, openSync, closeSync, rmSync, readdirSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve, join } from 'node:path'
import { createServer } from 'node:net'

// Each suite gets its own database, server and real login limiter. The complete
// gate must not weaken production throttling to accommodate repeated QA logins.
if (process.argv.length === 2) {
  const suites = readdirSync(resolve('tests')).filter(file => file.endsWith('.spec.ts') && file !== 'webmcp.spec.ts').sort()
  for (const suite of suites) {
    process.stdout.write(`\nIsolated browser suite: ${suite}\n`)
    const child = spawn(process.execPath, [resolve('tests/run-browser.mjs'), suite], { stdio: 'inherit', env: process.env })
    const code = await new Promise(resolve => child.on('exit', code => resolve(code ?? 1)))
    if (code !== 0) process.exit(code)
  }
  process.stdout.write(`\nAll ${suites.length} isolated browser suites passed.\n`)
  process.exit(0)
}

const root = mkdtempSync(join(tmpdir(), 'society-browser-'))
const db = join(root, 'society.db')
const binary = resolve('../build/society-server')
const reservation = createServer()
await new Promise((resolve, reject) => { reservation.once('error', reject); reservation.listen(0, '127.0.0.1', resolve) })
const port = reservation.address().port
await new Promise(resolve => reservation.close(resolve))
const base = `http://127.0.0.1:${port}`
const logs = openSync(join(root, 'server.log'), 'w', 0o600)
let server
try {
  execFileSync(binary, ['seed-demo', '--demo', '--db', db], { stdio: 'ignore' })
  server = spawn(binary, ['serve', '--demo', '--db', db, '--mfa-key-file', join(root, 'keys', 'mfa.key'), '--addr', `127.0.0.1:${port}`, '--web-dir', resolve('../build/web')], { stdio: ['ignore', logs, logs] })
  const deadline = Date.now() + 15000
  while (true) {
    try { const response = await fetch(`${base}/ready`); if (response.ok) break } catch { /* local startup */ }
    if (server.exitCode !== null || Date.now() > deadline) throw new Error(`Isolated browser QA server could not start on ${base}`)
    await new Promise(resolve => setTimeout(resolve, 50))
  }
  // Playwright treats file arguments as regular expressions. Anchor a named
  // suite so overview.spec.ts cannot also run collections-overview.spec.ts
  // and mutate the database intended for the overview's empty-state checks.
  const testArguments = process.argv.slice(2).map(argument =>
    !argument.startsWith('-') && argument.endsWith('.spec.ts')
      ? '(?:^|[\\\\/])' + argument.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '$'
      : argument)
  const runner = spawn(process.execPath, ['node_modules/@playwright/test/cli.js', 'test', ...testArguments], { stdio: 'inherit', env: { ...process.env, SOCIETY_BROWSER_URL: base, SOCIETY_BROWSER_DB: db } })
  process.exitCode = await new Promise(resolve => runner.on('exit', code => resolve(code ?? 1)))
} finally {
  if (server && server.exitCode === null) {
    const finished = new Promise(resolve => server.once('exit', resolve))
    server.kill('SIGTERM')
    const timer = setTimeout(() => server.kill('SIGKILL'), 6000)
    await finished
    clearTimeout(timer)
  }
  closeSync(logs)
  rmSync(root, { recursive: true, force: true })
}
