// Generated build membership is the entire storage authority. No API caching.
const PREFIX = 'society-public-'
const CACHE = PREFIX + STATIC.version
const pathKey = path => new URL(path, self.location.origin).href
const digest = async response => [...new Uint8Array(await crypto.subtle.digest('SHA-256', await response.clone().arrayBuffer()))].map(byte => byte.toString(16).padStart(2, '0')).join('')
const correctType = (path, response) => {
  const type = (response.headers.get('Content-Type') ?? '').split(';')[0].trim().toLowerCase()
  if (path === '/') return type === 'text/html'
  if (path.endsWith('.js')) return ['text/javascript', 'application/javascript'].includes(type)
  if (path.endsWith('.css')) return type === 'text/css'
  if (path.endsWith('.png')) return type === 'image/png'
  if (path.endsWith('.svg')) return type === 'image/svg+xml'
  if (path.endsWith('.webmanifest')) return ['application/manifest+json', 'application/json'].includes(type)
  return /\.woff2?$/.test(path) && ['font/woff', 'font/woff2', 'application/font-woff', 'application/font-woff2', 'application/x-font-woff', 'application/octet-stream'].includes(type)
}
async function remember(path, response) {
  if (!response.ok || response.redirected || response.type === 'opaque' || !correctType(path, response) || await digest(response) !== STATIC.assets[path]) return false
  const cache = await caches.open(CACHE)
  await cache.put(pathKey(path), response.clone())
  return true
}
self.addEventListener('install', event => {
  event.waitUntil((async () => {
    const results = await Promise.allSettled(STATIC.precache.map(async path => {
      const response = await fetch(path, { credentials: 'omit', cache: path.startsWith('/assets/') ? 'force-cache' : 'no-store' })
      if (!await remember(path, response)) throw new Error('The public shell could not be prepared.')
    }))
    if (results.some(result => result.status === 'rejected')) { await caches.delete(CACHE); throw new Error('The public shell could not be prepared.') }
  })())
})
self.addEventListener('activate', event => {
  event.waitUntil((async () => {
    const names = (await caches.keys()).filter(name => name.startsWith(PREFIX))
    const previous = names.filter(name => name !== CACHE).at(-1)
    await Promise.all(names.filter(name => name !== CACHE && name !== previous).map(name => caches.delete(name)))
    // No clients.claim(): an existing document keeps its current version.
  })())
})
self.addEventListener('fetch', event => {
  const request = event.request, url = new URL(request.url)
  if (request.method !== 'GET' || url.origin !== self.location.origin || url.search) return
  const path = url.pathname === '/index.html' ? '/' : url.pathname
  if (!Object.hasOwn(STATIC.assets, path)) return
  event.respondWith((async () => {
    const cache = await caches.open(CACHE)
    if (path !== '/') {
      const saved = await cache.match(pathKey(path))
      if (saved) return saved
    }
    try {
      const response = await fetch(request, { credentials: 'omit', cache: path.startsWith('/assets/') ? 'force-cache' : 'no-store' })
      await remember(path, response)
      return response
    } catch {
      const saved = await cache.match(pathKey(path))
      return saved ?? new Response('Reconnect to open this screen.', { status: 503, headers: { 'Content-Type': 'text/plain; charset=utf-8', 'Cache-Control': 'no-store' } })
    }
  })())
})
self.addEventListener('message', event => {
  if (event.data?.type !== 'SOCIETY_APPLY_UPDATE' || !event.source?.id || !event.ports[0]) return
  event.waitUntil((async () => {
    const clients = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
    if (clients.length !== 1 || clients[0].id !== event.source.id) {
      event.ports[0].postMessage({ ok: false, message: 'Close other Society tabs before applying the update.' })
      return
    }
    event.ports[0].postMessage({ ok: true })
    await self.skipWaiting()
  })())
})
