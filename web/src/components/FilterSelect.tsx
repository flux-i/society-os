import * as Select from '@radix-ui/react-select'
import { useCallback, useEffect, useId, useRef, useState } from 'react'
import { Icon } from './Icon'

type SelectProps = {
  label: string; value: string; options: { value: string; label: string }[]; onChange: (value: string) => void; required?: boolean
}

function PortalSelect({ label, value, options, onChange, required, form = false }: SelectProps & { form?: boolean }) {
  const [container, setContainer] = useState<HTMLElement | undefined>()
  const [open, setOpen] = useState(false)
  const [invalid, setInvalid] = useState(false)
  const errorId = useId()
  const trigger = useRef<HTMLButtonElement | null>(null)
  const menu = useRef<HTMLDivElement | null>(null)
  const attach = useCallback((node: HTMLButtonElement | null) => { trigger.current = node; setContainer(node?.closest('dialog') ?? undefined) }, [])
  const items = form && required ? options.filter(option => option.value) : options
  useEffect(() => {
    if (!open) return
    const outside = (event: PointerEvent) => {
      if (event.target instanceof Node && !menu.current?.contains(event.target) && !trigger.current?.contains(event.target)) setOpen(false)
    }
    document.addEventListener('pointerdown', outside, true)
    return () => document.removeEventListener('pointerdown', outside, true)
  }, [open])
  return <span className="select-field" onInvalidCapture={event => { event.preventDefault(); setInvalid(true); requestAnimationFrame(() => { const first = trigger.current?.closest('form')?.querySelector<HTMLElement>('input:invalid, textarea:invalid, button[aria-invalid="true"]'); (first ?? trigger.current)?.focus() }) }}><Select.Root value={value || (form && required ? '' : 'all')} required={required} open={open} onOpenChange={setOpen} onValueChange={next => {
    if (form && !items.some(option => (option.value || 'all') === next)) return
    setInvalid(false)
    onChange(next === 'all' ? '' : next)
  }}>
    <Select.Trigger ref={attach} className={`filter-select ${form ? 'form-select' : ''}`} aria-label={label} aria-invalid={invalid || undefined} aria-describedby={invalid ? errorId : undefined}>
      <Select.Value placeholder={options.find(option => !option.value)?.label} /><Select.Icon className="filter-chevron"><Icon name="down" /></Select.Icon>
    </Select.Trigger>
    <Select.Portal container={container}><Select.Content ref={menu} className="filter-menu" position="popper" sideOffset={6} align="start" collisionPadding={12}>
      <Select.ScrollUpButton className="select-scroll-button"><Icon name="down" /></Select.ScrollUpButton>
      <Select.Viewport className="filter-menu-viewport">
        {items.map(option => <Select.Item className="filter-option" key={option.value || 'all'} value={option.value || 'all'}>
          <Select.ItemText>{option.label}</Select.ItemText>
          <Select.ItemIndicator className="filter-check"><Icon name="check" /></Select.ItemIndicator>
        </Select.Item>)}
      </Select.Viewport>
      <Select.ScrollDownButton className="select-scroll-button"><Icon name="down" /></Select.ScrollDownButton>
    </Select.Content></Select.Portal>
  </Select.Root>{invalid && <span id={errorId} className="form-error" role="alert">Choose an option to continue.</span>}</span>
}

export function FilterSelect(props: SelectProps) { return <PortalSelect {...props} /> }
export function FormSelect(props: SelectProps) { return <PortalSelect {...props} form /> }
