import type { User } from '../api'
import { backupPhaseLabels, canReadBackups, useBackupStatus } from '../backups'
import { Icon } from './Icon'

export function BackupProtectionNotice({ user }: { user: User }) {
  const source = useBackupStatus(user)
  if (!canReadBackups(user) || source.loading || (!source.error && ((!source.data?.configured && !source.data?.finance_paused) || (source.data?.phase === 'HEALTHY' && !source.data.staging_full)))) return null
  const capacityAttention = source.data?.phase === 'HEALTHY' && source.data.staging_full
  return <aside className="backup-protection-notice" role="status"><Icon name="shield" /><div><strong>{source.error ? 'Recovery status unavailable' : capacityAttention ? 'Recovery staging needs attention' : backupPhaseLabels[source.data!.phase]}</strong><p>{source.error ? 'Check the current protection status before confirming financial changes.' : source.data!.finance_paused ? 'Drafts and existing records remain available while a current recovery copy is verified.' : capacityAttention ? 'The confirmed copy is current. Resolve retained copies or storage capacity before the next copy can be prepared.' : 'The last confirmed copy is at least 45 minutes old.'}</p></div><a href="#backups" className="text-link">Check protection<Icon name="arrow" /></a></aside>
}
