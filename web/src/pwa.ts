import { useSyncExternalStore } from 'react'

const SIGNOUT = 'society.pending-signout'
const readSignout = () => { if (typeof window === 'undefined') return false; if (window.location.hash === '#signed-out') return true; try { return localStorage.getItem(SIGNOUT) === '1' } catch { return false } }
type InstallPrompt = Event & { prompt(): Promise<void>; userChoice: Promise<{ outcome: string }> }
type State = { connection: 'online' | 'offline' | 'checking'; pendingSignout: boolean; waiting: boolean; applying: boolean; writes: number; error: string; install: InstallPrompt | null }
let state: State = { connection: typeof window === 'undefined' || navigator.onLine ? 'online' : 'offline', pendingSignout: readSignout(), waiting: false, applying: false, writes: 0, error: '', install: null }
const listeners = new Set<() => void>()
let registration: ServiceWorkerRegistration | undefined, started = false
const watched = new WeakSet<ServiceWorker>()
const update = (values: Partial<State>) => { state = { ...state, ...values }; listeners.forEach(fn => fn()) }
function watchRegistration(value: ServiceWorkerRegistration) {
  registration = value
  const waiting = () => update({ waiting: !!value.waiting })
  const installing = () => {
    const worker = value.installing
    if (!worker || watched.has(worker)) return
    watched.add(worker)
    worker.addEventListener('statechange', () => {
      waiting()
      if (worker.state === 'redundant') update({ error: 'The app update could not be prepared. Keep using this version and try again.' })
    })
  }
  waiting(); installing(); value.addEventListener('updatefound', installing)
}
export const usePortalConnection = () => useSyncExternalStore(fn => { listeners.add(fn); return () => { listeners.delete(fn) } }, () => state)
export const portalConnection = () => state
export const connectionLost = () => { if (state.connection !== 'offline') update({ connection: 'offline' }) }
export const connectionVerified = () => { if (state.connection !== 'online') update({ connection: 'online' }) }
export const requestConnectionCheck = () => { update({ connection: navigator.onLine ? 'checking' : 'offline', error: '' }); window.dispatchEvent(new Event('connection-retry')) }
export const markLocalSignout = () => {
  try { localStorage.setItem(SIGNOUT, '1') } catch { /* In-memory revocation still applies. */ }
  window.history.replaceState(null, '', '#signed-out')
  update({ pendingSignout: true }); window.dispatchEvent(new Event('local-signout'))
}
export const clearLocalSignout = () => { try { localStorage.removeItem(SIGNOUT) } catch { /* No private storage. */ }; if (window.location.hash === '#signed-out') window.history.replaceState(null, '', '#overview'); update({ pendingSignout: false }) }
export const setConnectionError = (error: string) => update({ error })
export function beginPortalRequest(path: string, method: string) {
  const revocation = path === '/api/auth/me' || path === '/api/auth/logout'
  if (!revocation && (state.pendingSignout || state.connection !== 'online')) throw new Error('Reconnect before continuing. Nothing was queued; check the saved result before retrying a change.')
  const writing = !['GET', 'HEAD'].includes(method.toUpperCase())
  if (writing && !revocation && state.applying) throw new Error('Wait for the app update before sending a change.')
  if (writing) update({ writes: state.writes + 1 })
  return () => { if (writing) update({ writes: Math.max(0, state.writes - 1) }) }
}

export function startPublicShell() {
  if (started) return
  started = true
  window.addEventListener('offline', connectionLost)
  window.addEventListener('online', requestConnectionCheck)
  document.addEventListener('submit', event => {
    if (state.connection === 'online' && !state.pendingSignout && !state.applying) return
    event.preventDefault(); event.stopImmediatePropagation()
    update({ error: 'Reconnect before sending this form. Your input stays here; nothing was queued.' })
  }, true)
  window.addEventListener('storage', event => {
    if (event.key !== SIGNOUT) return
    update({ pendingSignout: readSignout() })
    if (state.pendingSignout) window.dispatchEvent(new Event('local-signout'))
  })
  window.addEventListener('beforeinstallprompt', event => { event.preventDefault(); update({ install: event as InstallPrompt }) })
  window.addEventListener('appinstalled', () => update({ install: null }))
  if (!import.meta.env.PROD || !('serviceWorker' in navigator)) return
  const register = async () => {
    try {
      watchRegistration(await navigator.serviceWorker.register('/sw.js', { scope: '/', updateViaCache: 'none' }))
    } catch { update({ error: 'Installation support could not be prepared. You can keep using the portal online.' }) }
  }
  if (document.readyState === 'complete') void register()
  else window.addEventListener('load', () => void register(), { once: true })
}

export async function installPortal() {
  const prompt = state.install
  if (!prompt) return
  try { await prompt.prompt(); await prompt.userChoice } catch { update({ error: 'Use your browser’s install menu to add Society.' }) }
  update({ install: null })
}
export async function checkPortalUpdate() {
  try {
    if (!registration || (!registration.active && !registration.waiting && !registration.installing)) {
      if (!('serviceWorker' in navigator)) throw new Error('Installation is unavailable in this browser.')
      watchRegistration(await navigator.serviceWorker.register('/sw.js', { scope: '/', updateViaCache: 'none' }))
      update({ error: '' }); return
    }
    await registration.update(); update({ waiting: !!registration.waiting, error: '' })
  } catch { update({ error: 'The update could not be checked. Reconnect and try again.' }) }
}
export async function applyPortalUpdate() {
  if (state.connection !== 'online' || state.pendingSignout) { update({ error: 'Reconnect and finish signing out before updating.' }); return }
  if (state.writes || document.querySelector('dialog[open], form')) { update({ error: 'Close your open work before applying the update. Your current work stays here.' }); return }
  const worker = registration?.waiting
  if (!worker || state.applying) return
  update({ applying: true, error: '' })
  const channel = new MessageChannel()
  let accepted = false, replaced = false
  const changed = () => { replaced = true; if (accepted) window.location.reload() }
  navigator.serviceWorker.addEventListener('controllerchange', changed)
  const timer = window.setTimeout(() => { channel.port1.close(); navigator.serviceWorker.removeEventListener('controllerchange', changed); update({ applying: false, error: 'The update is still waiting. Close other Society tabs and try again.' }) }, 10000)
  channel.port1.onmessage = event => {
    if (event.data?.ok) { accepted = true; if (replaced) window.location.reload() }
    else { clearTimeout(timer); channel.port1.close(); navigator.serviceWorker.removeEventListener('controllerchange', changed); update({ applying: false, error: String(event.data?.message ?? 'The update is still waiting.') }) }
  }
  worker.postMessage({ type: 'SOCIETY_APPLY_UPDATE' }, [channel.port2])
}
