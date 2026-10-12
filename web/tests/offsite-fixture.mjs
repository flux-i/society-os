// Independent S3 protocol peer, bound only to a disposable numeric-loopback
// address. The application binary never contains these control endpoints.
import { createServer } from 'node:http'
import { createHash, createHmac, randomBytes } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const hash = data => createHash('sha256').update(data).digest('hex')
const keyed = (key, value) => createHmac('sha256', key).update(value).digest()
function validSignature(request, body, expectedID, secret) {
  const header = request.headers.authorization ?? ''
  if (!header.startsWith('AWS4-HMAC-SHA256 ')) return false
  const fields = Object.fromEntries(header.slice(17).split(', ').map(field => { const index = field.indexOf('='); return [field.slice(0, index), field.slice(index + 1)] }))
  const credential = (fields.Credential ?? '').split('/')
  const date = request.headers['x-amz-date'] ?? ''
  if (credential.length !== 5 || credential[0] !== expectedID || credential[2] !== 'ap-south-1' || credential[3] !== 's3' || credential[4] !== 'aws4_request' || date.slice(0, 8) !== credential[1] || request.headers['x-amz-content-sha256'] !== hash(body)) return false
  const url = new URL(request.url, 'http://127.0.0.1')
  const headers = (fields.SignedHeaders ?? '').split(';').map(name => `${name}:${String(request.headers[name] ?? '').trim().replace(/\s+/g, ' ')}\n`).join('')
  const query = [...url.searchParams].sort(([a], [b]) => a.localeCompare(b)).map(([name, value]) => `${encodeURIComponent(name)}=${encodeURIComponent(value)}`).join('&')
  const canonical = `${request.method}\n${url.pathname}\n${query}\n${headers}\n${fields.SignedHeaders}\n${hash(body)}`
  const scope = credential.slice(1).join('/')
  const toSign = `AWS4-HMAC-SHA256\n${date}\n${scope}\n${hash(canonical)}`
  let key = keyed('AWS4' + secret, credential[1]); key = keyed(key, credential[2]); key = keyed(key, 's3'); key = keyed(key, 'aws4_request')
  return keyed(key, toSign).toString('hex') === fields.Signature
}

export async function createOffsiteFixture(root, binary, initialMode = 'LOST_REPLY') {
  const directory = join(root, 'offsite-fixture')
  mkdirSync(directory, { mode: 0o700 })
  const writerSecret = randomBytes(32).toString('hex'), readerSecret = randomBytes(32).toString('hex')
  const identities = [join(directory, 'custodian-one.key'), join(directory, 'custodian-two.key')]
  const recipients = identities.map(path => JSON.parse(execFileSync(binary, ['offsite-keygen', '--out', path], { encoding: 'utf8' })).public_recipient)
  const objects = new Map()
  let mode = initialMode, puts = 0, gets = 0
  const server = createServer(async (request, response) => {
    try {
      let size = 0
      const pieces = []
      for await (const piece of request) { size += piece.length; if (size > 8 * 1024 * 1024) { response.writeHead(413); response.end(); return } pieces.push(piece) }
      const body = Buffer.concat(pieces), url = new URL(request.url, 'http://127.0.0.1')
      if (url.pathname === '/_fixture/status' && request.method === 'GET') {
        response.setHeader('Content-Type', 'application/json')
        response.end(JSON.stringify({ mode, puts, gets, objects: [...objects.values()].map(({ bytes, ...metadata }) => ({ ...metadata, byte_count: bytes.length })) }))
        return
      }
      if (url.pathname === '/_fixture/mode' && request.method === 'POST') {
        mode = JSON.parse(body.toString()).mode
        response.setHeader('Content-Type', 'application/json'); response.end(JSON.stringify({ mode })); return
      }
      const role = request.method === 'GET' ? 'reader' : 'writer'
      if (!validSignature(request, body, 'fixture-' + role, role === 'reader' ? readerSecret : writerSecret) || !url.pathname.startsWith('/recovery/society/')) { response.writeHead(403); response.end(); return }
      const key = url.pathname.slice(1)
      if (request.method === 'PUT') {
        puts++
        if (request.headers['if-none-match'] !== '*' || url.search) { response.writeHead(400); response.end(); return }
        if (mode === 'DENIED') { response.writeHead(403); response.end(); return }
        if (objects.has(key)) { response.writeHead(412); response.end(); return }
        const checksum = hash(body)
        if (request.headers['x-amz-checksum-sha256'] !== Buffer.from(checksum, 'hex').toString('base64')) { response.writeHead(400); response.end(); return }
        const object = { key, version: 'fixture-version-' + (objects.size + 1), sha256: checksum, bytes: body }
        objects.set(key, object)
        if (mode === 'LOST_REPLY') { request.socket.destroy(); return }
        response.setHeader('X-Amz-Version-Id', object.version)
        if (mode !== 'MISSING_CHECKSUM') response.setHeader('X-Amz-Checksum-Sha256', Buffer.from(checksum, 'hex').toString('base64'))
        response.writeHead(200); response.end(); return
      }
      if (request.method === 'GET') {
        gets++
        const object = objects.get(key)
        if (!object || url.searchParams.get('versionId') !== object.version || request.headers['x-amz-checksum-mode'] !== 'ENABLED') { response.writeHead(404); response.end(); return }
        response.setHeader('X-Amz-Version-Id', object.version)
        response.setHeader('X-Amz-Checksum-Sha256', Buffer.from(object.sha256, 'hex').toString('base64'))
        response.setHeader('Content-Length', object.bytes.length)
        response.writeHead(200); response.end(object.bytes); return
      }
      response.writeHead(405); response.end()
    } catch { response.writeHead(500); response.end() }
  })
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve) })
  const origin = `http://127.0.0.1:${server.address().port}`
  const storage = { mode: 'FIXTURE', region: 'ap-south-1', bucket: 'fictional-recovery', prefix: 'recovery/society', endpoint: origin }
  const config = join(directory, 'writer.json'), reader = join(directory, 'reader.json')
  for (const [path, role, secret] of [[config, 'writer', writerSecret], [reader, 'reader', readerSecret]]) writeFileSync(path, JSON.stringify({ format_version: 1, storage: { ...storage, access_key_id: 'fixture-' + role, secret_access_key: secret }, public_recipients: recipients }), { mode: 0o600 })
  return { origin, config, reader, identities, close: async () => { server.closeAllConnections(); await new Promise(resolve => server.close(resolve)) } }
}
