import { createServer, request as forward } from 'node:http'

// Numeric loopback only. Records request metadata and byte counts, never bodies.
export async function createPwaFixture(target) {
  const backend = new URL(target)
  if (backend.hostname !== '127.0.0.1') throw new Error('PWA fixture requires numeric loopback.')
  let state = { revision: '', failedInstall: false, dropPost: '', failAPI: false }, requests = []
  const server = createServer((incoming, outgoing) => {
    const path = new URL(incoming.url, target).pathname
    if (path === '/__pwa-fixture') {
      if (incoming.method === 'GET') { outgoing.setHeader('Content-Type', 'application/json'); outgoing.end(JSON.stringify({ state, requests })); return }
      let text = ''; incoming.on('data', chunk => { text += chunk; if (text.length > 4096) incoming.destroy() }); incoming.on('end', () => {
        try { const next = JSON.parse(text); if (next.reset) requests = []; for (const key of Object.keys(state)) if (key in next) state[key] = next[key]; outgoing.setHeader('Content-Type', 'application/json'); outgoing.end('{}') } catch { outgoing.writeHead(400); outgoing.end() }
      }); return
    }
    if (state.failAPI && path.startsWith('/api/')) { incoming.resume(); outgoing.destroy(); return }
    const request = forward({ hostname: backend.hostname, port: backend.port, path: incoming.url, method: incoming.method, headers: incoming.headers }, response => {
      const parts = []; response.on('data', part => parts.push(part)); response.on('end', () => {
        let body = Buffer.concat(parts)
        if (path === '/sw.js' && (state.revision || state.failedInstall)) {
          const source = body.toString(), split = source.indexOf(';\n'), definition = JSON.parse(source.slice('const STATIC = '.length, split))
          definition.version += '-' + state.revision
          if (state.failedInstall) definition.assets['/icon-192.png'] = '0'.repeat(64)
          body = Buffer.from('const STATIC = ' + JSON.stringify(definition) + source.slice(split))
        }
        requests.push({ method: incoming.method, path, status: response.statusCode, bytes: body.length })
        if (state.dropPost && incoming.method === 'POST' && path === state.dropPost) {
          state.dropPost = ''
          outgoing.writeHead(response.statusCode, { ...response.headers, 'content-length': String(body.length + 8) })
          outgoing.flushHeaders(); outgoing.write(body.subarray(0, 5))
          setTimeout(() => outgoing.destroy(), 20)
          return
        }
        const headers = { ...response.headers, 'content-length': String(body.length) }; delete headers['transfer-encoding']
        outgoing.writeHead(response.statusCode, headers); outgoing.end(body)
      })
    })
    request.on('error', () => { if (!outgoing.headersSent) outgoing.writeHead(502); outgoing.end() })
    incoming.pipe(request)
  })
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  return { origin: 'http://127.0.0.1:' + server.address().port, close: () => new Promise(resolve => server.close(resolve)) }
}
