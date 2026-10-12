import { useEffect, useState } from 'react'
import { APIError, request } from './api'
import type { User } from './api'

export const canReadBackups = (user: User) => user.can_manage_accounts || user.can_manage_records
export type BackupPhase = 'NOT_CONFIGURED' | 'HEALTHY' | 'WARNING' | 'PAUSED' | 'CLOCK_ERROR'
export type BackupAttempt = { id: string; state: string; captured_at: number; acknowledged_at: number; attempts: number; error_code: string }
export type BackupStatus = {
  configured: boolean; mode: string; phase: BackupPhase; finance_paused: boolean; as_of: number
  capture_at: number; acknowledged_at: number; age_seconds: number | null; next_capture_at: number
  staging_full: boolean; counts: Record<string, number>; items: BackupAttempt[]
  page: number; page_size: number; total: number; status_key: string
}
export type BackupDetail = BackupAttempt & {
  events: { action: string; actor_kind: string; note: string; occurred_at: number }[]
  page: number; page_size: number; event_total: number
}
export const backupStateLabels: Record<string, string> = {
  CAPTURING: 'Preparing copy', SEALED: 'Encrypted copy', UPLOADING: 'Sending copy', VERIFIED: 'Confirmed', FAILED: 'Failed attempt', UNCERTAIN: 'Needs confirmation',
}
export const backupPhaseLabels: Record<BackupPhase, string> = {
  NOT_CONFIGURED: 'Protection not configured', HEALTHY: 'Records protected', WARNING: 'A fresh copy is needed', PAUSED: 'Financial changes paused', CLOCK_ERROR: 'Clock needs attention',
}
export const backupErrorLabels: Record<string, string> = {
  CAPTURE_FAILED: 'A verified copy could not be prepared.', UPLOAD_DENIED: 'Storage denied this attempt.', UPLOAD_REJECTED: 'Storage did not accept this attempt.',
  UPLOAD_UNCERTAIN: 'An independent custody check must confirm the original copy.', LEASE_EXPIRED: 'The previous attempt ended before a result could be confirmed.', STAGING_FULL: 'Local recovery staging has reached its retained-copy limit.',
}
export const backupTime = (value: number) => value ? new Date(value * 1000).toLocaleString('en-IN', { day: 'numeric', month: 'short', hour: 'numeric', minute: '2-digit' }) : 'No confirmed copy yet'
export const backupAge = (seconds: number | null) => seconds === null ? 'Not yet available' : seconds < 60 ? 'Less than a minute' : `${Math.floor(seconds / 60)} minutes`

export function useBackupStatus(user: User, state = '', page = 1) {
  const [data, setData] = useState<BackupStatus | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [revision, setRevision] = useState(0)
  const permitted = canReadBackups(user)
  useEffect(() => {
    const controller = new AbortController()
    setError(''); setLoading(true)
    if (!permitted) { setData(null); setLoading(false); return () => controller.abort() }
    const params = new URLSearchParams({ state, page: String(page) })
    void request<BackupStatus>('/api/backups?' + params, controller.signal).then(value => {
      if (!controller.signal.aborted) { setData(value); setLoading(false) }
    }).catch((failure: Error) => {
      if (controller.signal.aborted) return
      setData(null); setError(failure.message); setLoading(false)
      if (failure instanceof APIError && [401, 403].includes(failure.status)) window.dispatchEvent(new Event('session-recheck'))
    })
    return () => controller.abort()
  }, [permitted, user.scope_key, state, page, revision])
  useEffect(() => {
    if (!permitted) return
    const timer = window.setInterval(() => { if (document.visibilityState === 'visible') setRevision(value => value + 1) }, 30000)
    return () => window.clearInterval(timer)
  }, [permitted])
  return { data, error, loading, refresh: () => setRevision(value => value + 1) }
}
