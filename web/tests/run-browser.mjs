// A fresh fictional database keeps browser mutations out of the user's preview.
import { spawn, execFileSync } from 'node:child_process'
import { mkdtempSync, openSync, closeSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve, join } from 'node:path'
import { createServer } from 'node:net'

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
  const runner = spawn(process.execPath, ['node_modules/@playwright/test/cli.js', 'test', ...process.argv.slice(2)], { stdio: 'inherit', env: { ...process.env, SOCIETY_BROWSER_URL: base, SOCIETY_BROWSER_DB: db } })
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
