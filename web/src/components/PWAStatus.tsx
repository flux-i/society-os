import { applyPortalUpdate, checkPortalUpdate, installPortal, requestConnectionCheck, usePortalConnection } from '../pwa'
import { Icon } from './Icon'

export function ConnectionNote() {
  const state = usePortalConnection()
  if (state.connection === 'online' && !state.waiting) return null
  return <aside className="dialog-connection-note" role="status"><strong>{state.connection !== 'online' ? 'Your connection is paused.' : 'An app update is ready.'}</strong><p>{state.connection !== 'online' ? 'Your open work stays here. Changes cannot be sent until we reconnect.' : 'Close this work before applying the update.'}</p>{state.connection !== 'online' && <button type="button" className="text-action" disabled={state.connection === 'checking'} onClick={requestConnectionCheck}>Check connection<Icon name="refresh"/></button>}{state.error && <p>{state.error}</p>}</aside>
}

export function PWAStatus({ workspace, onSignOut }: { workspace: boolean; onSignOut: () => void }) {
  const state = usePortalConnection(), disconnected = state.connection !== 'online'
  if (!disconnected && !state.pendingSignout && !state.waiting && !state.install) return null
  return <section className={'pwa-strip' + (workspace ? ' pwa-strip-workspace' : '')} aria-label="App connection and updates" role="status"><div><span className="pwa-strip-icon"><Icon name="leaf"/></span><div><strong>{state.pendingSignout ? 'Signed out on this device.' : disconnected ? 'Your connection is paused.' : state.waiting ? 'A fresh version is ready.' : 'Keep Society close.'}</strong><p>{state.pendingSignout ? 'Reconnect to finish closing the server session. Private screens stay cleared.' : disconnected ? 'Changes cannot be sent. Your open work stays here until you reconnect.' : state.waiting ? 'Update when your open work is finished.' : 'Add the portal to your home screen.'}</p></div></div><div className="pwa-strip-actions">{disconnected || state.pendingSignout ? <><button type="button" className="button button-dark" disabled={state.connection === 'checking'} onClick={requestConnectionCheck}>{state.connection === 'checking' ? 'Checking…' : 'Check connection'}<Icon name="refresh"/></button>{workspace && <button type="button" className="text-action" onClick={onSignOut}>Sign out now</button>}</> : state.waiting ? <button type="button" className="button button-dark" disabled={state.applying} onClick={()=>void applyPortalUpdate()}>{state.applying ? 'Updating…' : 'Update app'}<Icon name="refresh"/></button> : <button type="button" className="button button-dark" onClick={()=>void installPortal()}>Install app<Icon name="arrow"/></button>}</div>{state.error && <p className="pwa-strip-error" role="alert">{state.error}</p>}</section>
}

export function InstallHelp() {
  const state = usePortalConnection()
  return <details className="pwa-install-help"><summary>Install Society on your device</summary><div><p>In Chrome or Edge, choose Install app from the browser menu. On iPhone or iPad, open this portal in Safari, choose Share, then Add to Home Screen. The option depends on your browser and a secure connection.</p><p>Keep a connection to view private records or send changes. Installation saves the public app shell on this device.</p>{state.install && <button type="button" className="button button-dark" onClick={()=>void installPortal()}>Install app<Icon name="arrow"/></button>}<button type="button" className="text-action" onClick={()=>void checkPortalUpdate()}>Check for app updates<Icon name="refresh"/></button>{state.error && <p role="alert">{state.error}</p>}</div></details>
}
