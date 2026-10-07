// A fresh fictional database keeps browser mutations out of the user's preview.
import { spawn, execFileSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, openSync, closeSync, rmSync, readdirSync, cpSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve, join, basename } from 'node:path'
import { createServer } from 'node:net'

// Each suite gets its own database, server and real login limiter. The complete
// gate must not weaken production throttling to accommodate repeated QA logins.
const allNative = process.argv.length === 3 && process.argv[2] === '--config=playwright.webmcp.config.ts'
if (process.argv.length === 2 || allNative) {
  const nativeSuite = file => file === 'webmcp.spec.ts' || file.startsWith('webmcp-')
  const suites = readdirSync(resolve('tests')).filter(file => file.endsWith('.spec.ts') && nativeSuite(file) === allNative).sort()
  for (const suite of suites) {
    process.stdout.write(`\nIsolated ${allNative ? 'native WebMCP' : 'browser'} suite: ${suite}\n`)
    const child = spawn(process.execPath, [resolve('tests/run-browser.mjs'), suite, ...(allNative ? [process.argv[2]] : [])], { stdio: 'inherit', env: process.env })
    const code = await new Promise(resolve => child.on('exit', code => resolve(code ?? 1)))
    if (code !== 0) process.exit(code)
  }
  process.stdout.write(`\nAll ${suites.length} isolated ${allNative ? 'native WebMCP' : 'browser'} suites passed.\n`)
  process.exit(0)
}

const root = mkdtempSync(join(tmpdir(), 'society-browser-'))
// Independent browser runs must not delete each other's traces/screenshots.
// Retain synthetic failure evidence privately after the disposable DB is removed.
const artifacts = resolve('../reports/local/browser-runs', basename(root))
mkdirSync(artifacts, { recursive: true, mode: 0o700 })
const db = join(root, 'society.db')
// Retain the tested runtime pair for this run. A later build must not remove
// files that the QA server is serving or replace its binary between restarts.
const binary = join(root, 'society-server'),webDir = join(root,'web')
const reservation = createServer()
await new Promise((resolve, reject) => { reservation.once('error', reject); reservation.listen(0, '127.0.0.1', resolve) })
const port = reservation.address().port
await new Promise(resolve => reservation.close(resolve))
const base = `http://127.0.0.1:${port}`
const logs = openSync(join(root, 'server.log'), 'w', 0o600)
let server
try {
  cpSync(resolve('../build/society-server'),binary)
  cpSync(resolve('../build/web'),webDir,{recursive:true})
  execFileSync(binary, ['seed-demo', '--demo', '--db', db], { stdio: 'ignore' })
  server = spawn(binary, ['serve', '--demo', '--db', db, '--mfa-key-file', join(root, 'keys', 'mfa.key'), '--addr', `127.0.0.1:${port}`, '--web-dir', webDir], { stdio: ['ignore', logs, logs] })
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
  const runner = spawn(process.execPath, ['node_modules/@playwright/test/cli.js', 'test', ...testArguments], { stdio: 'inherit', env: { ...process.env, SOCIETY_BROWSER_URL: base, SOCIETY_BROWSER_DB: db, SOCIETY_BROWSER_ARTIFACTS: artifacts } })
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
