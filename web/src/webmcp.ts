import { useEffect } from 'react'
import { APIError, request } from './api'
import type { User } from './api'

type Tool = {
  name: string
  description: string
  inputSchema: Record<string, unknown>
  annotations: { readOnlyHint: boolean; untrustedContentHint: boolean }
  execute: (input: Record<string, unknown>, options?: { signal?: AbortSignal }) => Promise<string>
}
type ModelContext = { registerTool: (tool: Tool, options: { signal: AbortSignal }) => Promise<void> }

// Optional browser API: ordinary browsers use the same screens without a polyfill.
// No tool posts money, approves a request, publishes content or changes permissions.
export function useSocietyTools(user: User, openHome: (id: string) => void) {
  useEffect(() => {
    const context = (document as Document & { modelContext?: ModelContext }).modelContext
    if (!context || user.mfa_pending) return
    const lifetime = new AbortController()
    const current = async (signal: AbortSignal) => {
      const me = await request<User>('/api/auth/me', signal).catch(error => {
        if (error instanceof APIError && error.status === 401) window.dispatchEvent(new Event('session-expired'))
        throw error
      })
      if (me.id !== user.id || me.mfa_pending || lifetime.signal.aborted) throw new Error('The signed-in account changed. Discover tools again.')
      return me
    }
    const add = (name: string, description: string, properties: Record<string, unknown>, required: string[], run: (input: Record<string, unknown>, signal: AbortSignal, me: User) => Promise<unknown>, readOnly = true) => {
      const tool: Tool = {
        name, description,
        inputSchema: { type: 'object', properties, required, additionalProperties: false },
        annotations: { readOnlyHint: readOnly, untrustedContentHint: true },
        execute: async (input, options) => {
          const signal = options?.signal ? AbortSignal.any([lifetime.signal, options.signal]) : lifetime.signal
          if (!input || typeof input !== 'object' || Object.keys(input).some(key => !(key in properties))) throw new Error('Unsupported tool arguments.')
          const me = await current(signal)
          const result = await run(input, signal, me)
          await current(signal)
          return JSON.stringify(result)
        },
      }
      void context.registerTool(tool, { signal: lifetime.signal }).catch(() => { /* Experimental API availability never blocks the portal. */ })
    }
    const queryText = (value: unknown) => {
      if (value !== undefined && (typeof value !== 'string' || value.length > 100)) throw new Error('Search text must be at most 100 characters.')
      return typeof value === 'string' ? value : ''
    }
    const pageNumber = (value: unknown) => {
      if (value !== undefined && (!Number.isInteger(value) || Number(value) < 1 || Number(value) > 10000)) throw new Error('Use a positive page number, at most 10000.')
      return value === undefined ? 1 : Number(value)
    }
    const search = { type: 'string', maxLength: 100, description: 'A home number or name from the signed-in account’s permitted results.' }
    const page = { type: 'integer', minimum: 1, maximum: 10000 }
    add('society_find_homes', 'Search the current account’s permitted homes. Returns one page; resident access follows current memberships.', {
      query: search, wing: { type: 'string', enum: ['', 'A', 'B', 'C'] }, occupancy: { type: 'string', enum: ['', 'OWNER_OCCUPIED', 'RENTED', 'VACANT'] }, page,
    }, [], async (input, signal) => {
      const wing = input.wing ?? ''; const occupancy = input.occupancy ?? ''
      if (!['', 'A', 'B', 'C'].includes(String(wing)) || !['', 'OWNER_OCCUPIED', 'RENTED', 'VACANT'].includes(String(occupancy))) throw new Error('Choose a supported wing and occupancy.')
      return request('/api/flats?' + new URLSearchParams({ q: queryText(input.query), building: String(wing), status: String(occupancy), page: String(pageNumber(input.page)), page_size: '12' }), signal)
    })
    add('society_open_home', 'Open an authorised home’s visible details for human review. This changes only the screen, and never edits the registry.', {
      home_id: { type: 'string', minLength: 1, maxLength: 100 },
    }, ['home_id'], async (input, signal) => {
      if (typeof input.home_id !== 'string' || !input.home_id || input.home_id.length > 100) throw new Error('A home identity is required.')
      if (document.querySelector('dialog[open]')) throw new Error('Close the current dialog before opening a home.')
      const home = await request<{ id: string }>('/api/flats/' + encodeURIComponent(input.home_id), signal)
      await current(signal)
      openHome(home.id)
      return { opened: home.id, next_step: 'Review the visible details. Changes require the ordinary form and confirmation.' }
    }, false)
    const views = ['homes', 'security', 'reviews', 'community', 'help', ...(user.can_read_registry ? ['overview'] : []), ...(user.can_manage_registry ? ['access'] : []), ...(user.can_read_records ? ['entries', 'receipts'] : [])]
    add('society_open_workspace', 'Open an available workspace screen. No data is submitted. Close any review dialog first to preserve unsaved work.', { screen: { type: 'string', enum: views } }, ['screen'], async (input, _signal, me) => {
      const screen = String(input.screen)
      if (!views.includes(screen) || (screen === 'overview' && !me.can_read_registry) || (screen === 'access' && !me.can_manage_registry) || (['entries', 'receipts'].includes(screen) && !me.can_read_records)) throw new Error('This screen requires current permission.')
      if (document.querySelector('dialog[open]')) throw new Error('Close the current dialog before navigating.')
      window.location.hash = screen
      return { screen }
    }, false)
    if (user.can_read_records) add('society_find_records', 'Read permitted manual entries or receipt states. Totals describe supplied records. This never confirms entries, creates receipts or initiates payments.', {
      query: search, home_id: { type: 'string', maxLength: 100 }, receipts_only: { type: 'boolean' }, page,
    }, [], async (input, signal, me) => {
      if (!me.can_read_records) throw new Error('Current financial permission is required.')
      if (input.home_id !== undefined && (typeof input.home_id !== 'string' || input.home_id.length > 100)) throw new Error('Use a valid home identity.')
      if (input.receipts_only !== undefined && typeof input.receipts_only !== 'boolean') throw new Error('Use a boolean receipt filter.')
      return request('/api/entries?' + new URLSearchParams({ q: queryText(input.query), home: String(input.home_id ?? ''), receipts: String(input.receipts_only ?? false), page: String(pageNumber(input.page)) }), signal)
    })
    add('society_find_requests', 'Read the current account’s private submissions or the authorised reviewer queue. Review decisions require the visible form and separate reviewer; this tool never approves an item.', { query: search, page }, [], async (input, signal) => request('/api/reviews?' + new URLSearchParams({ q: queryText(input.query), page: String(pageNumber(input.page)) }), signal))
    add('society_find_notices', 'Read approved notices matching current audience and memberships. Unapproved proposals and private reviewer notes are excluded.', { query: search, page }, [], async (input, signal) => request('/api/notices?' + new URLSearchParams({ q: queryText(input.query), page: String(pageNumber(input.page)) }), signal))
    add('society_find_complaints', 'Read the current account’s own service requests or the authorised handling queue. Case ownership is personal; sharing a home does not share another person’s case. Conversation text is not searched.', { query: search, page, status: { type: 'string', enum: ['', 'OPEN', 'ACKNOWLEDGED', 'IN_PROGRESS', 'WAITING', 'RESOLVED', 'CLOSED'] } }, [], async (input, signal) => {
      const status = input.status ?? ''
      if (!['', 'OPEN', 'ACKNOWLEDGED', 'IN_PROGRESS', 'WAITING', 'RESOLVED', 'CLOSED'].includes(String(status))) throw new Error('Choose a supported service status.')
      return request('/api/complaints?' + new URLSearchParams({ q: queryText(input.query), page: String(pageNumber(input.page)), status: String(status) }), signal)
    })
    add('society_read_complaint', 'Read an authorised service request and one page of its conversation. Resident results exclude staff-only notes and their counts; current handler permission is enforced by the server. This does not update or close a case.', { case_id: { type: 'string', minLength: 1, maxLength: 100 }, history_page: page }, ['case_id'], async (input, signal) => {
      if (typeof input.case_id !== 'string' || !input.case_id || input.case_id.length > 100) throw new Error('A service request identity is required.')
      return request('/api/complaints/' + encodeURIComponent(input.case_id) + '?' + new URLSearchParams({ history_page: String(pageNumber(input.history_page)) }), signal)
    })
    return () => lifetime.abort()
  }, [user.id, user.mfa_pending, user.can_read_registry, user.can_manage_registry, user.can_read_records, openHome])
}
