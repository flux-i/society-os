import { useEffect, useRef, useState } from 'react'
import type { User } from '../api'
import { Icon } from './Icon'
import type { IconName } from './Icon'

type Destination = { view: string; label: string }
type Group = { id: string; label: string; icon: IconName; items: Destination[] }
export function workspaceGroups(user: User): Group[] {
  return [
    { id: 'people', label: 'People & homes', icon: 'homes', items: [{ view: 'homes', label: user.can_read_registry ? 'Homes & people' : 'Your homes' }, ...(user.can_read_contacts ? [{ view: 'contacts', label: user.can_manage_contacts ? 'Contacts' : 'Your preferences' }] : [])] },
    { id: 'money', label: 'Money', icon: 'receipt', items: [...(user.can_read_records ? [{ view: 'maintenance', label: 'Maintenance' }, { view: 'entries', label: 'Entries' }, { view: 'receipts', label: 'Receipts' }, { view: 'payments', label: 'Payment reconciliation' }, { view: 'reports', label: 'Reports' }, { view: 'collections', label: 'Collections' }] : []), { view: 'statements', label: 'Statements' }] },
    { id: 'operations', label: 'Operations', icon: 'leaf', items: [{ view: 'help', label: 'Help & repairs' }, { view: 'reviews', label: user.can_review_requests ? 'Requests & approvals' : 'Your requests' }, { view: 'upkeep', label: 'Upkeep' }, { view: 'conduct', label: 'Rules & conduct' }, { view: 'fines', label: 'Fines & appeals' }] },
    { id: 'community', label: 'Community', icon: 'community', items: [{ view: 'community', label: 'Notices & meetings' }, { view: 'documents', label: 'Documents' }, { view: 'messages', label: 'Messages' }] },
    { id: 'administration', label: 'Administration', icon: 'shield', items: [...(user.can_manage_accounts ? [{ view: 'access', label: 'Access & invitations' }] : []), ...(user.can_manage_accounts || user.can_manage_records ? [{ view: 'backups', label: 'Backups & recovery' }] : [])] },
  ].filter(group => group.items.length) as Group[]
}
export function workspaceGroup(user: User, view: string) { return workspaceGroups(user).find(group => group.items.some(item => item.view === view)) }
export function WorkspaceSections({ user, view }: { user: User; view: string }) {
  const group = workspaceGroup(user, view)
  if (!group) return null
  return <nav className="workspace-sections" aria-label={group.label + ' sections'}>{group.items.map(item => <a key={item.view} href={'#' + item.view} aria-current={view === item.view ? 'page' : undefined}>{item.label}</a>)}</nav>
}
export function WorkspaceNavigation({ view, user, onAbout }: { view: string; user: User; onAbout: () => void }) {
  const [menuOpen, setMenuOpen] = useState(false)
  const menuButton = useRef<HTMLButtonElement>(null)
  const group = workspaceGroup(user, view)
  const manages = user.can_read_registry || user.can_read_all_records || user.can_manage_accounts || user.can_handle_complaints
  useEffect(() => { setMenuOpen(false) }, [view])
  const link = (href: string, label: string, icon: IconName, active: boolean) => <a href={href} className={'nav-item ' + (active ? 'active' : '')} aria-current={active ? 'page' : undefined}><Icon name={icon}/><span>{label}</span>{active && <span className="active-dot"/>}</a>
  return <aside className="sidebar" onKeyDown={event => { if (event.key === 'Escape') { setMenuOpen(false); menuButton.current?.focus() } }}>
    <a href="#overview" className="brand" aria-label="Society OS overview"><span className="brand-mark"><Icon name="homes"/></span><span>society<span className="brand-dot">.</span><small>A place for everything</small></span></a>
    <button ref={menuButton} className="mobile-nav-toggle" aria-expanded={menuOpen} aria-controls="workspace-navigation" onClick={() => setMenuOpen(value => !value)}>Menu<Icon name={menuOpen ? 'close' : 'menu'}/></button>
    <div id="workspace-navigation" className={'sidebar-space ' + (menuOpen ? 'menu-open' : '')} onClick={event => { if ((event.target as Element).closest('a')) { setMenuOpen(false); if (menuOpen) requestAnimationFrame(() => document.getElementById('main-content')?.focus({ preventScroll: true })) } }}>
      {manages && <nav className="workspace-mode" aria-label="Workspace view"><a href="#my-home" aria-current={view === 'my-home' ? 'page' : undefined}>My home</a><a href="#overview" aria-current={view !== 'my-home' ? 'page' : undefined}>Manage</a></nav>}
      <nav aria-label="Main navigation" className="nav-list">
        {link('#overview', 'Today', 'overview', view === 'overview')}
        {!manages && link('#my-home', 'My home', 'homes', view === 'my-home')}
        {workspaceGroups(user).map(item => <div key={item.id}>{link('#' + item.items[0].view, item.label, item.icon, group?.id === item.id)}</div>)}
      </nav>
      <nav className="workspace-support" aria-label="Account and help">{link('#guide', 'Help & permissions', 'document', view === 'guide')}{link('#security', 'Account security', 'shield', view === 'security')}</nav>
    </div>
    <div className="sidebar-bottom"><button className="workspace-profile" onClick={onAbout}><span className="avatar avatar-profile">{user.name.split(' ').map(part => part[0]).slice(0, 2).join('')}</span><span>{user.name}<small>Account & workspace</small></span><Icon name="chevron"/></button></div>
  </aside>
}
