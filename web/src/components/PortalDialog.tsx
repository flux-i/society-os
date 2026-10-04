import { useLayoutEffect, useRef } from 'react'
import type { MouseEvent, PointerEvent, ReactNode } from 'react'
import { Icon } from './Icon'

export function PortalDialog({ children, titleId, closeLabel, onClose, busy = false, className = '' }: {
  children: ReactNode; titleId: string; closeLabel: string; onClose: () => void; busy?: boolean; className?: string
}) {
  const dialog = useRef<HTMLDialogElement>(null)
  const opener = useRef<HTMLElement | null>(null)
  const pointerStartedOutside = useRef(false)
  useLayoutEffect(() => {
    const overflow = document.body.style.overflow
    if (!dialog.current?.open && document.activeElement instanceof HTMLElement) opener.current = document.activeElement
    document.body.style.overflow = 'hidden'
    dialog.current?.showModal()
    return () => {
      dialog.current?.close()
      document.body.style.overflow = overflow
      if (opener.current?.isConnected) opener.current.focus({ preventScroll: true })
    }
  }, [])
  const outside = (event: MouseEvent<HTMLDialogElement> | PointerEvent<HTMLDialogElement>) => {
    if (event.target !== event.currentTarget) return false
    const bounds = event.currentTarget.getBoundingClientRect()
    return event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom
  }
  return <dialog ref={dialog} className={`detail-dialog ${className}`} aria-labelledby={titleId} onClose={() => { if (!dialog.current?.open) onClose() }}
    onCancel={event => { if (busy) event.preventDefault() }}
    onKeyDown={event => {
      if (event.key !== 'Tab' || event.defaultPrevented) return
      const controls = Array.from(event.currentTarget.querySelectorAll<HTMLElement>('button, a[href], input, select, textarea, [tabindex="0"], summary')).filter(el => el.tabIndex >= 0 && !el.matches(':disabled') && el.getClientRects().length > 0)
      const first = controls[0], last = controls.at(-1)
      if (first && ((event.shiftKey && document.activeElement === first) || (!event.shiftKey && document.activeElement === last))) {
        event.preventDefault(); (event.shiftKey ? last : first)?.focus()
      }
    }}
    onPointerDown={event => { pointerStartedOutside.current = outside(event) }}
    onClick={event => { if (!busy && pointerStartedOutside.current && outside(event)) dialog.current?.close() }}>
    <button className="dialog-close" autoFocus aria-label={closeLabel} disabled={busy} onClick={() => dialog.current?.close()}><Icon name="close" /></button>
    {children}
  </dialog>
}
