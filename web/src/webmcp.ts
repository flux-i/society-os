import { useEffect } from 'react'
import { APIError, request, userAccessScope } from './api'
import type { Account, AccountPage, User } from './api'
import type { AccountDetails } from './components/AccountAdministration'
import type { HomeStatement, MaintenanceCycle, MaintenanceDetail, MaintenancePage } from './maintenance'
import type { RegisterDetail, RegisterPage, Work, WorkDetail, WorkPage } from './upkeep'
import type { Incident, IncidentDetail, IncidentNotice, IncidentPage, NoticePage, Rule, RulePage } from './incidents'
import type { ContributionPage, Fund, FundDetail, FundPage, FundWaiver, PaymentReport, ReportPage, WaiverPage } from './collections'
import type { Fine, FineDetail, FinePage, FineNotice, FineNoticePage, FineReport, FineReportPage, FineAppeal, FineAppealPage, FineWaiver, FineWaiverPage } from './fines'
import type { Contact, ContactDetail, ContactPage } from './contacts'

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
  const accessScope = userAccessScope(user)
  useEffect(() => {
    const context = (document as Document & { modelContext?: ModelContext }).modelContext
    if (!context || user.mfa_pending) return
    const lifetime = new AbortController()
    const current = async (signal: AbortSignal) => {
      const me = await request<User>('/api/auth/me', signal).catch(error => {
        if (error instanceof APIError && error.status === 401) window.dispatchEvent(new Event('session-expired'))
        throw error
      })
      if (me.mfa_pending && !lifetime.signal.aborted) window.dispatchEvent(new Event('session-recheck'))
      if (me.id !== user.id || me.mfa_pending || lifetime.signal.aborted) throw new Error('The signed-in account changed. Discover tools again.')
      if (userAccessScope(me) !== accessScope) {
        window.dispatchEvent(new Event('session-recheck'))
        throw new Error('Current permissions changed. Discover tools again.')
      }
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
    const views = ['overview', 'homes', 'security', 'reviews', 'community', 'help', 'documents', 'upkeep', 'conduct', 'fines', ...(user.can_read_contacts ? ['contacts'] : []), ...(user.can_manage_accounts ? ['access'] : []), ...(user.can_read_records ? ['entries', 'receipts', 'maintenance', 'collections'] : [])]
    add('society_open_workspace', 'Open an available workspace screen. No data is submitted. Close any review dialog first to preserve unsaved work.', { screen: { type: 'string', enum: views } }, ['screen'], async (input, _signal, me) => {
      const screen = String(input.screen)
      if (!views.includes(screen) || (screen === 'contacts' && !me.can_read_contacts) || (screen === 'access' && !me.can_manage_accounts) || (['entries', 'receipts', 'maintenance', 'collections'].includes(screen) && !me.can_read_records)) throw new Error('This screen requires current permission.')
      if (document.querySelector('dialog[open]')) throw new Error('Close the current dialog before navigating.')
      window.location.hash = screen
      return { screen }
    }, false)
    const sections = ['reviews', 'service', 'notices', 'documents', 'upkeep', 'incidents', 'fines', ...(user.can_read_records ? ['finance', 'maintenance', 'collections'] : [])]
    add('society_read_overview', 'Read a current role-scoped overview section with full counts and at most four metadata rows. Financial amounts describe confirmed supplied records, not overdue bills. No evidence, private notes or file bytes are returned. This never approves, posts or sends anything.', { section: { type: 'string', enum: sections } }, ['section'], async (input, signal, me) => {
      const section = String(input.section)
      if (!sections.includes(section) || (['finance','maintenance','collections'].includes(section) && !me.can_read_records)) throw new Error('Choose a currently permitted overview section.')
      return request('/api/overview/' + section, signal)
    })
    const accountMetadata = (account: Account) => ({ id: account.id, name: account.name, state: account.state, roles: account.roles, mfa_enrolled: account.mfa_enrolled, active_homes: account.active_homes })
    if (user.can_read_contacts) {
      const metadata = (x: Contact) => ({ id: x.id, name: x.name, state: x.state, current: x.current, version: x.version, preferred_channel: x.preferred_channel, destinations_recorded: { whatsapp: !!x.phone, email: !!x.email }, permissions: { community_whatsapp: x.community_whatsapp, community_email: x.community_email, finance_whatsapp: x.finance_whatsapp, finance_email: x.finance_email }, eligible: x.eligible })
      add('society_find_contacts', 'Read at most twelve current people’s contact-status metadata with community authority, or only the signed-in person’s own retained profile. It excludes destinations, identity/permission references, home identifiers, actors, private reasons and history. Eligibility is a current snapshot; sending must independently recheck it. This cannot register, verify, opt in, opt out or send.', { query: search, relationship: { type: 'string', enum: ['', 'OWNER', 'TENANT'] }, wing: { type: 'string', enum: ['', 'A', 'B', 'C'] }, state: { type: 'string', enum: ['', 'NONE', 'PENDING', 'VERIFIED', 'DECLINED', 'WITHDRAWN'] }, page }, [], async (input, signal, me) => {
        if (!me.can_read_contacts) throw new Error('Current community or own-person permission is required.')
        const relationship = input.relationship ?? '', wing = input.wing ?? '', state = input.state ?? ''
        if (typeof relationship !== 'string' || !['', 'OWNER', 'TENANT'].includes(relationship) || typeof wing !== 'string' || !['', 'A', 'B', 'C'].includes(wing) || typeof state !== 'string' || !['', 'NONE', 'PENDING', 'VERIFIED', 'DECLINED', 'WITHDRAWN'].includes(state)) throw new Error('Choose a supported relationship, wing and contact state.')
        const data = await request<ContactPage>('/api/contacts?' + new URLSearchParams({ q: queryText(input.query), relationship, building: wing, state, page: String(pageNumber(input.page)) }), signal)
        return { items: data.items.map(metadata), total: data.total, page: data.page, page_size: data.page_size }
      })
      add('society_read_contact', 'Read one permitted contact-status and selected-preference snapshot. Destinations, source references, home identities, actors, private reasons and history are excluded. Residents read only their own profile, including after a relationship ends. Verification and opt-outs require the ordinary visible form; this sends nothing.', { contact_id: { type: 'string', minLength: 1, maxLength: 100 } }, ['contact_id'], async (input, signal, me) => {
        if (!me.can_read_contacts) throw new Error('Current community or own-person permission is required.')
        if (typeof input.contact_id !== 'string' || !input.contact_id || input.contact_id.length > 100) throw new Error('A permitted person identity is required.')
        return metadata(await request<ContactDetail>('/api/contacts/' + encodeURIComponent(input.contact_id), signal))
      })
    }
    const fineIdentity = (value: unknown) => { if (typeof value !== 'string' || !value || value.length > 100) throw new Error('A permitted fine record identity is required.'); return value }
    const optionalFine = (value: unknown) => value === undefined || value === '' ? '' : fineIdentity(value)
    const fineMetadata = (x: Fine) => ({ id: x.id, title: x.title, home: x.home, state: x.state, version: x.version, due_date: x.due_date, response_by: x.response_by, active_paise: x.active_paise, allocated_paise: x.allocated_paise, outstanding_paise: x.outstanding_paise, waived_paise: x.waived_paise, pause_until: x.pause_until ?? '' })
    const fineNoticeMetadata = (x: FineNotice) => ({ id: x.id, title: x.title, home: x.home, state: x.state, version: x.version, due_date: x.due_date, response_by: x.response_by })
    const fineAppealMetadata = (x: FineAppeal) => ({ id: x.id, fine: x.fine, home: x.home, state: x.state, version: x.version, pause_until: x.pause_until })
    add('society_find_fine_notices', 'Read at most twelve approved fine-notice metadata rows for current households or permitted financial review. Wording, response text/counts, proposed amounts, case identities, policies and private notes are excluded. A notice creates no charge; this tool cannot respond, issue, appeal or send anything.', { query: search, page }, [], async (input, signal) => {
      const data = await request<FineNoticePage>('/api/fine-notices?' + new URLSearchParams({ q: queryText(input.query), page: String(pageNumber(input.page)) }), signal)
      return { items: data.items.map(fineNoticeMetadata), total: data.total, page: data.page, page_size: data.page_size }
    })
    add('society_read_fine_notice', 'Read one currently permitted household notice’s frozen metadata. Wording, policy, responses, original incident, private identities and review versions are excluded. This never posts a household response or appeal.', { notice_id: { type: 'string', minLength: 1, maxLength: 100 } }, ['notice_id'], async (input, signal) => fineNoticeMetadata(await request<FineNotice>('/api/fine-notices/' + encodeURIComponent(fineIdentity(input.notice_id)), signal)))
    add('society_find_fine_appeals', 'Read one bounded page of own current-home appeal metadata, or permitted financial review metadata. Financial ledger access is not needed to read one’s own appeal. Accounts, policies, reasons, actors and private parent versions are excluded. This cannot pause collection or decide an appeal.', { fine_id: { type: 'string', maxLength: 100 }, page }, [], async (input, signal) => {
      const data = await request<FineAppealPage>('/api/fine-appeals?' + new URLSearchParams({ fine: optionalFine(input.fine_id), page: String(pageNumber(input.page)) }), signal)
      return { items: data.items.map(fineAppealMetadata), total: data.total, page: data.page, page_size: data.page_size }
    })
    add('society_read_fine_appeal', 'Read a currently permitted appeal’s status and supplied pause date. Appeal text, decision/policy notes, actors, events and private fine versions are excluded. Expired pauses need explicit human review; this tool makes no decision.', { appeal_id: { type: 'string', minLength: 1, maxLength: 100 } }, ['appeal_id'], async (input, signal) => fineAppealMetadata(await request<FineAppeal>('/api/fine-appeals/' + encodeURIComponent(fineIdentity(input.appeal_id)), signal)))
    if (user.can_read_records) {
      add('society_find_fines', 'Read twelve permitted fine metadata rows and exact scoped totals. Residents see only issued or corrected fines for financially entitled current homes. Source decisions, notice wording, responses, policy, actors and private review history are excluded. This cannot approve, issue, waive, allocate or initiate payments.', { query: search, state: { type: 'string', enum: ['', 'PENDING', 'NOTIFIED', 'ISSUED', 'WAIVED', 'DECLINED', 'WITHDRAWN'] }, page }, [], async (input, signal, me) => {
        if (!me.can_read_records) throw new Error('Current financial permission is required.')
        const state = String(input.state ?? ''); if (!['', 'PENDING', 'NOTIFIED', 'ISSUED', 'WAIVED', 'DECLINED', 'WITHDRAWN'].includes(state)) throw new Error('Choose a supported fine state.')
        const data = await request<FinePage>('/api/fines?' + new URLSearchParams({ q: queryText(input.query), state, page: String(pageNumber(input.page)) }), signal)
        return { items: data.items.map(fineMetadata), total: data.total, page: data.page, page_size: data.page_size, totals: data.totals }
      })
      add('society_read_fine', 'Read a currently permitted fine’s exact live charge, confirmed allocations, outstanding and explicit collection pause metadata. Private source, household text, responders, policies, actors, original case and review history are excluded. Every financial decision requires the ordinary reviewed form.', { fine_id: { type: 'string', minLength: 1, maxLength: 100 } }, ['fine_id'], async (input, signal, me) => { if (!me.can_read_records) throw new Error('Current financial permission is required.'); return fineMetadata(await request<FineDetail>('/api/fines/' + encodeURIComponent(fineIdentity(input.fine_id)), signal)) })
      const fineReportMetadata = (x: FineReport) => ({ id: x.id, fine_id: x.fine_id, fine: x.fine, home: x.home, amount_paise: x.amount_paise, payment_date: x.payment_date, state: x.state, current_state: x.current_state, version: x.version, receipt: x.receipt, entry_id: x.entry_id })
      add('society_find_fine_payments', 'Read one bounded page of fine-payment claim metadata. Own reports require current home and financial access. Payer, references, comments, evidence identifiers, verification identities, actors and decisions are excluded. Claims create no receipt; this cannot verify or record money.', { fine_id: { type: 'string', maxLength: 100 }, state: { type: 'string', enum: ['', 'PENDING', 'NEEDS_INFO', 'REJECTED', 'WITHDRAWN', 'CONFIRMED', 'DUPLICATE'] }, page }, [], async (input, signal, me) => {
        if (!me.can_read_records) throw new Error('Current financial permission is required.')
        const state = String(input.state ?? ''); if (!['', 'PENDING', 'NEEDS_INFO', 'REJECTED', 'WITHDRAWN', 'CONFIRMED', 'DUPLICATE'].includes(state)) throw new Error('Choose a supported report state.')
        const data = await request<FineReportPage>('/api/fine-reports?' + new URLSearchParams({ fine: optionalFine(input.fine_id), state, page: String(pageNumber(input.page)) }), signal)
        return { items: data.items.map(fineReportMetadata), total: data.total, page: data.page, page_size: data.page_size, counts: data.counts }
      })
      add('society_read_fine_payment', 'Read a currently permitted fine-payment claim’s amount, status and original receipt metadata. Private payer, payment identity, external source, evidence, comments, actors and decisions are excluded. Nothing is verified, allocated or received by this tool.', { report_id: { type: 'string', minLength: 1, maxLength: 100 } }, ['report_id'], async (input, signal, me) => { if (!me.can_read_records) throw new Error('Current financial permission is required.'); return fineReportMetadata(await request<FineReport>('/api/fine-reports/' + encodeURIComponent(fineIdentity(input.report_id)), signal)) })
      if (user.can_read_all_records) {
        const fineCorrectionMetadata = (x: FineWaiver) => ({ id: x.id, fine_id: x.fine_id, fine: x.fine, home: x.home, kind: x.kind, amount_paise: x.amount_paise, state: x.state, version: x.version })
        add('society_find_fine_corrections', 'Read twelve permitted financial-reviewer correction metadata rows. Ordinary resident finance does not grant private correction proposals. Policies, source versions, reasons, actors, events and replacement selectors are excluded. This cannot propose or approve corrections.', { fine_id: { type: 'string', maxLength: 100 }, page }, [], async (input, signal, me) => {
          if (!me.can_read_all_records) throw new Error('Current financial reviewer permission is required.')
          const data = await request<FineWaiverPage>('/api/fine-waivers?' + new URLSearchParams({ fine: optionalFine(input.fine_id), page: String(pageNumber(input.page)) }), signal)
          return { items: data.items.map(fineCorrectionMetadata), total: data.total, page: data.page, page_size: data.page_size }
        })
        add('society_read_fine_correction', 'Read one permitted financial-reviewer correction’s kind, amount and status. Sources, policy, reasons, actor identities, private parent versions and effects/history selectors are excluded. Original charges and receipts can change only through a separately reviewed human form.', { correction_id: { type: 'string', minLength: 1, maxLength: 100 } }, ['correction_id'], async (input, signal, me) => { if (!me.can_read_all_records) throw new Error('Current financial reviewer permission is required.'); return fineCorrectionMetadata(await request<FineWaiver>('/api/fine-waivers/' + encodeURIComponent(fineIdentity(input.correction_id)), signal)) })
      }
    }
    if (user.can_manage_accounts) {
      add('society_find_accounts', 'Read one page of account names, states and current appointments as an authorised administrator. Email addresses, security material and verification notes are excluded. This cannot invite, change roles, suspend or resume an account.', { query: { ...search, description: 'At most 100 characters from an account name or verified login.' }, page }, [], async (input, signal, me) => {
        if (!me.can_manage_accounts) throw new Error('Current account administration permission is required.')
        const result = await request<AccountPage>('/api/admin/accounts?' + new URLSearchParams({ q: queryText(input.query), page: String(pageNumber(input.page)), page_size: '12' }), signal)
        return { ...result, items: result.items.map(accountMetadata) }
      })
      add('society_read_account', 'Read bounded appointment metadata for a currently permitted account as an administrator. Email addresses, activity/verification notes, passwords, factors and tokens are excluded. Visible forms are required for all access changes.', { account_id: { type: 'string', minLength: 1, maxLength: 100 }, grant_page: page }, ['account_id'], async (input, signal, me) => {
        if (!me.can_manage_accounts) throw new Error('Current account administration permission is required.')
        if (typeof input.account_id !== 'string' || !input.account_id || input.account_id.length > 100) throw new Error('An account identity is required.')
        const result = await request<AccountDetails>('/api/admin/accounts/' + encodeURIComponent(input.account_id) + '?' + new URLSearchParams({ grant_page: String(pageNumber(input.grant_page)) }), signal)
        return { account: accountMetadata(result.account), version: result.version, grants: result.grants.map(grant => ({ id: grant.id, role: grant.role, state: grant.state, valid_from: grant.valid_from, valid_until: grant.valid_until, revoked_at: grant.revoked_at })), grant_total: result.grant_total, grant_page: result.grant_page, page_size: result.page_size }
      })
    }
    if (user.can_read_records) add('society_find_records', 'Read permitted manual entries or receipt states. Totals describe supplied records. This never confirms entries, creates receipts or initiates payments.', {
      query: search, home_id: { type: 'string', maxLength: 100 }, receipts_only: { type: 'boolean' }, page,
    }, [], async (input, signal, me) => {
      if (!me.can_read_records) throw new Error('Current financial permission is required.')
      if (input.home_id !== undefined && (typeof input.home_id !== 'string' || input.home_id.length > 100)) throw new Error('Use a valid home identity.')
      if (input.receipts_only !== undefined && typeof input.receipts_only !== 'boolean') throw new Error('Use a boolean receipt filter.')
      return request('/api/entries?' + new URLSearchParams({ q: queryText(input.query), home: String(input.home_id ?? ''), receipts: String(input.receipts_only ?? false), page: String(pageNumber(input.page)) }), signal)
    })
    if (user.can_read_records) {
      const identity = (value:unknown) => {
        if(typeof value!=='string'||!value||value.length>100)throw new Error('A permitted record identity is required.')
        return value
      }
      const cycleMetadata = (cycle:MaintenanceCycle) => ({ id:cycle.id,title:cycle.title,period_start:cycle.period_start,period_end:cycle.period_end,due_date:cycle.due_date,state:cycle.state,version:cycle.version,participants:cycle.participants,requested_paise:cycle.requested_paise,active_paise:cycle.active_paise,allocated_paise:cycle.allocated_paise,outstanding_paise:cycle.outstanding_paise,overdue_paise:cycle.overdue_paise,reversed_paise:cycle.reversed_paise })
      const fundMetadata = (fund:Fund) => ({id:fund.id,title:fund.title,contribution_type:fund.contribution_type,start_date:fund.start_date,due_date:fund.due_date,target_paise:fund.target_paise,state:fund.state,version:fund.version,participants:fund.participants,requested_paise:fund.requested_paise,active_paise:fund.active_paise,allocated_paise:fund.allocated_paise,outstanding_paise:fund.outstanding_paise,overdue_paise:fund.overdue_paise,waived_paise:fund.waived_paise,voluntary_paise:fund.voluntary_paise,pending_reports:fund.pending_reports,paid_homes:fund.paid_homes,partial_homes:fund.partial_homes,unpaid_homes:fund.unpaid_homes,exempt_homes:fund.exempt_homes})
      const reportMetadata = (report:PaymentReport) => ({id:report.id,campaign_id:report.campaign_id,campaign:report.campaign,flat_id:report.flat_id,home:report.home,amount_paise:report.amount_paise,payment_date:report.payment_date,state:report.state,current_state:report.current_state,version:report.version,receipt:report.receipt,entry_id:report.entry_id})
      add('society_find_collections','Read at most twelve fund metadata rows and scoped exact totals. Current home and financial access apply. Private source references, supporting notes, participant selectors and activity are excluded. This cannot publish a fund or initiate payments.',{query:search,home_id:{type:'string',maxLength:100},state:{type:'string',enum:['','PENDING','PUBLISHED','CLOSED','DECLINED','WITHDRAWN']},page},[],async(input,signal,me)=>{
        if(!me.can_read_records)throw new Error('Current financial permission is required.')
        const state=String(input.state??'');if(!['','PENDING','PUBLISHED','CLOSED','DECLINED','WITHDRAWN'].includes(state))throw new Error('Choose a supported fund state.')
        const result=await request<FundPage>('/api/collections?'+new URLSearchParams({q:queryText(input.query),home:input.home_id===undefined||input.home_id===''?'':identity(input.home_id),state,page:String(pageNumber(input.page))}),signal)
        return {items:result.items.map(fundMetadata),total:result.total,page:result.page,page_size:result.page_size,totals:result.totals}
      })
      add('society_read_collection','Read a permitted fund with at most twenty currently scoped participant lines. Voluntary funds create no debt. Private approval records and history are excluded. Separate human review is required for publication, verification and exemptions.',{campaign_id:{type:'string',minLength:1,maxLength:100},line_page:page},['campaign_id'],async(input,signal,me)=>{
        if(!me.can_read_records)throw new Error('Current financial permission is required.')
        const result=await request<FundDetail>('/api/collections/'+encodeURIComponent(identity(input.campaign_id))+'?line_page='+pageNumber(input.line_page),signal)
        return {...fundMetadata(result),lines:result.lines,line_page:result.line_page,page_size:result.page_size}
      })
      add('society_find_payment_reports','Read at most twelve payment-report metadata rows. Residents receive only their own reports for current financially entitled homes. Reported money is a claim until verified. Payer, reference, evidence, verification sources, identities, comments and review history are excluded. This cannot verify or issue a receipt.',{query:search,campaign_id:{type:'string',maxLength:100},home_id:{type:'string',maxLength:100},state:{type:'string',enum:['','PENDING','NEEDS_INFO','REJECTED','WITHDRAWN','CONFIRMED','DUPLICATE']},page},[],async(input,signal,me)=>{
        if(!me.can_read_records)throw new Error('Current financial permission is required.')
        const state=String(input.state??'');if(!['','PENDING','NEEDS_INFO','REJECTED','WITHDRAWN','CONFIRMED','DUPLICATE'].includes(state))throw new Error('Choose a supported report state.')
        const optional=(value:unknown)=>value===undefined||value===''?'':identity(value)
        const result=await request<ReportPage>('/api/payment-reports?'+new URLSearchParams({q:queryText(input.query),campaign:optional(input.campaign_id),home:optional(input.home_id),state,page:String(pageNumber(input.page))}),signal)
        return {items:result.items.map(reportMetadata),total:result.total,page:result.page,page_size:result.page_size,counts:result.counts}
      })
      add('society_read_payment_report','Read one currently permitted report’s claim amount, current status and linked original receipt metadata. Payer, external reference, photos/file identities, private comments and verification/decision history are excluded. Nothing is submitted or confirmed.',{report_id:{type:'string',minLength:1,maxLength:100}},['report_id'],async(input,signal,me)=>{
        if(!me.can_read_records)throw new Error('Current financial permission is required.')
        return reportMetadata(await request<PaymentReport>('/api/payment-reports/'+encodeURIComponent(identity(input.report_id)),signal))
      })
      add('society_find_fund_contributions','Read a bounded current-home page of voluntary purpose assignments and original receipt metadata. Private operator and correction reasons are excluded. This cannot assign, correct or receive money.',{campaign_id:{type:'string',maxLength:100},home_id:{type:'string',maxLength:100},page},[],async(input,signal,me)=>{
        if(!me.can_read_records)throw new Error('Current financial permission is required.')
        const optional=(value:unknown)=>value===undefined||value===''?'':identity(value)
        const result=await request<ContributionPage>('/api/fund-contributions?'+new URLSearchParams({campaign:optional(input.campaign_id),home:optional(input.home_id),page:String(pageNumber(input.page))}),signal)
        return {...result,items:result.items.map(item=>({id:item.id,campaign_id:item.campaign_id,campaign:item.campaign,home:item.home,amount_paise:item.amount_paise,receipt:item.receipt,state:item.state,created_at:item.created_at}))}
      })
      if(user.can_read_all_records){
        const waiverMetadata=(item:FundWaiver)=>({id:item.id,campaign_id:item.campaign_id,campaign:item.campaign,home:item.home,amount_paise:item.amount_paise,state:item.state,version:item.version})
        add('society_find_fund_exemptions','Read one bounded financial-reviewer page of exemption metadata. Sources, reasons, proposers, reviewers and history are excluded. Resident financial access does not grant private exemption records. This cannot propose or approve exemptions.',{campaign_id:{type:'string',maxLength:100},state:{type:'string',enum:['','PENDING','APPROVED','DECLINED','WITHDRAWN']},page},[],async(input,signal,me)=>{
          if(!me.can_read_all_records)throw new Error('Current financial reviewer permission is required.')
          const state=String(input.state??'');if(!['','PENDING','APPROVED','DECLINED','WITHDRAWN'].includes(state))throw new Error('Choose a supported exemption state.')
          const result=await request<WaiverPage>('/api/fund-waivers?'+new URLSearchParams({campaign:input.campaign_id===undefined||input.campaign_id===''?'':identity(input.campaign_id),state,page:String(pageNumber(input.page))}),signal)
          return {...result,items:result.items.map(waiverMetadata)}
        })
        add('society_read_fund_exemption','Read permitted financial-reviewer exemption metadata. Private reasons, source records and reviewer history are excluded. All exemptions require explicit visible forms and different-person review.',{waiver_id:{type:'string',minLength:1,maxLength:100}},['waiver_id'],async(input,signal,me)=>{
          if(!me.can_read_all_records)throw new Error('Current financial reviewer permission is required.')
          return waiverMetadata(await request<FundWaiver>('/api/fund-waivers/'+encodeURIComponent(identity(input.waiver_id)),signal))
        })
      }
      add('society_find_maintenance','Read one current financial-scope page of maintenance periods and exact filtered amounts. Residents receive published periods for their entitled homes. Private sources, notes, reviewers and all participant selectors are excluded. This cannot submit, approve, post or allocate.',{query:search,home_id:{type:'string',maxLength:100},state:{type:'string',enum:['','PENDING','PUBLISHED','DECLINED','WITHDRAWN']},page},[],async(input,signal,me)=>{
        if(!me.can_read_records)throw new Error('Current financial permission is required.')
        const state=String(input.state??'')
        if(!['','PENDING','PUBLISHED','DECLINED','WITHDRAWN'].includes(state))throw new Error('Use a supported maintenance state.')
        const home=input.home_id===undefined||input.home_id===''?'':identity(input.home_id)
        const result=await request<MaintenancePage>('/api/maintenance?'+new URLSearchParams({q:queryText(input.query),home,state,page:String(pageNumber(input.page))}),signal)
        return {items:result.items.map(cycleMetadata),total:result.total,page:result.page,page_size:result.page_size,totals:result.totals}
      })
      add('society_read_maintenance','Read a permitted maintenance period with at most twenty scoped home lines. Current membership and financial permissions apply. Private approval references, notes and history are excluded. Publication requires the visible form and a separate treasury reviewer.',{period_id:{type:'string',minLength:1,maxLength:100},line_page:page},['period_id'],async(input,signal,me)=>{
        if(!me.can_read_records)throw new Error('Current financial permission is required.')
        const result=await request<MaintenanceDetail>('/api/maintenance/'+encodeURIComponent(identity(input.period_id))+'?'+new URLSearchParams({line_page:String(pageNumber(input.line_page))}),signal)
        return {...cycleMetadata(result),lines:result.lines,line_page:result.line_page,page_size:result.page_size}
      })
      add('society_read_home_statement','Read one authorised home statement: exact totals plus at most twenty charges, credits and allocation links per page. Original receipts and opening credits are distinguished. Private operator and correction reasons are excluded. This never moves money or writes allocations.',{home_id:{type:'string',minLength:1,maxLength:100},charge_page:page,credit_page:page,allocation_page:page},['home_id'],async(input,signal,me)=>{
        if(!me.can_read_records)throw new Error('Current financial permission is required.')
        const result=await request<HomeStatement>('/api/statements/'+encodeURIComponent(identity(input.home_id))+'?'+new URLSearchParams({charge_page:String(pageNumber(input.charge_page)),credit_page:String(pageNumber(input.credit_page)),allocation_page:String(pageNumber(input.allocation_page))}),signal)
        return {...result,allocations:result.allocations.map(item=>({id:item.id,source_id:item.source_id,charge_id:item.charge_id,amount_paise:item.amount_paise,receipt:item.receipt,source_kind:item.source_kind,description:item.description,state:item.state,created_at:item.created_at,corrected_at:item.corrected_at}))}
      })
    }
    const careIdentity=(value:unknown)=>{if(typeof value!=='string'||!value||value.length>100)throw new Error('A permitted care record identity is required.');return value}
    const workMetadata=(work:Work)=>({id:work.id,title:work.title,category:work.category,priority:work.priority,state:work.state,due_date:work.due_date,visit_date:work.visit_date,version:work.version,audience:work.audience,building_code:work.building_code,published_at:work.published_at})
    add('society_find_upkeep','Read a current audience-scoped work page and full work counts. Residents receive frozen published updates for current homes; private work, activity and assignment details are excluded. This cannot assign, check completion, repeat, publish or send anything.',{query:search,state:{type:'string',enum:['','PLANNED','IN_PROGRESS','WAITING','READY_FOR_CHECK','DONE','CANCELLED']},page},[],async(input,signal)=>{
      const state=String(input.state??'');if(!['','PLANNED','IN_PROGRESS','WAITING','READY_FOR_CHECK','DONE','CANCELLED'].includes(state))throw new Error('Choose a supported work state.')
      const result=await request<WorkPage>('/api/upkeep?'+new URLSearchParams({q:queryText(input.query),state,page:String(pageNumber(input.page))}),signal)
      return {...result,items:result.items.map(workMetadata)}
    })
    add('society_read_upkeep','Read permitted work metadata. Residents see only the frozen public version. Descriptions, private events, vendor contacts, assignees and service links are excluded. All changes require the ordinary reviewed form.',{task_id:{type:'string',minLength:1,maxLength:100}},['task_id'],async(input,signal)=>workMetadata(await request<WorkDetail>('/api/upkeep/tasks/'+encodeURIComponent(careIdentity(input.task_id)),signal)))
    if(user.can_handle_complaints){
      const registerMetadata=(record:RegisterDetail|RegisterPage['items'][number])=>({id:record.id,kind:record.kind,name:record.name,category:record.category,state:record.state,version:record.version,amc_start:record.amc_start,amc_end:record.amc_end,inspection_date:record.inspection_date})
      const careKind=(value:unknown)=>{if(value!=='ASSET'&&value!=='VENDOR')throw new Error('Choose assets or vendors.');return value}
      add('society_find_upkeep_register','Read one bounded private register metadata page as a current operational officer. Treasury or auditor access alone does not grant it. Contacts, locations, contracts and private activity are excluded.',{kind:{type:'string',enum:['ASSET','VENDOR']},query:search,state:{type:'string',enum:['','ACTIVE','INACTIVE']},page},['kind'],async(input,signal,me)=>{
        if(!me.can_handle_complaints)throw new Error('Current operational permission is required.');const kind=careKind(input.kind),state=String(input.state??'');if(!['','ACTIVE','INACTIVE'].includes(state))throw new Error('Choose a supported register state.')
        const result=await request<RegisterPage>('/api/upkeep/register/'+kind+'?'+new URLSearchParams({q:queryText(input.query),state,page:String(pageNumber(input.page))}),signal)
        return {...result,items:result.items.map(registerMetadata)}
      })
      add('society_read_upkeep_register','Read asset or vendor metadata as a currently eligible operator. Private contacts, locations, contract/source references and activity are excluded. This cannot edit or retire a record.',{kind:{type:'string',enum:['ASSET','VENDOR']},record_id:{type:'string',minLength:1,maxLength:100}},['kind','record_id'],async(input,signal,me)=>{
        if(!me.can_handle_complaints)throw new Error('Current operational permission is required.')
        return registerMetadata(await request<RegisterDetail>('/api/upkeep/register/'+careKind(input.kind)+'/'+encodeURIComponent(careIdentity(input.record_id)),signal))
      })
    }
    add('society_find_requests', 'Read the current account’s private submissions or the authorised reviewer queue. Review decisions require the visible form and separate reviewer; this tool never approves an item.', { query: search, page }, [], async (input, signal) => request('/api/reviews?' + new URLSearchParams({ q: queryText(input.query), page: String(pageNumber(input.page)) }), signal))
    add('society_find_notices', 'Read approved notices matching current audience and memberships. Unapproved proposals and private reviewer notes are excluded.', { query: search, page }, [], async (input, signal) => request('/api/notices?' + new URLSearchParams({ q: queryText(input.query), page: String(pageNumber(input.page)) }), signal))
    const ruleMetadata = (rule:Rule) => ({id:rule.id,title:rule.title,state:rule.state,effective_from:rule.effective_from,effective_until:rule.effective_until,fine_permitted:rule.fine_permitted,version:rule.version})
    const incidentMetadata = (item:Incident) => ({id:item.id,rule_id:item.rule_id,rule_title:item.rule_title,home:item.home,incident_date:item.incident_date,state:item.state,version:item.version})
    const responseNoticeMetadata = (item:IncidentNotice) => ({id:item.id,title:item.title,home:item.home,incident_date:item.incident_date,response_by:item.response_by,active:item.active,version:item.version})
    add('society_find_rules','Read one bounded page of permitted rule metadata. Residents see published or retired rules; operators may inspect pending policy. Text, authority references, author identities and review events are excluded. This cannot publish, retire, propose or issue a fine.',{query:search,state:{type:'string',enum:['','PENDING','PUBLISHED','RETIRED','DECLINED','WITHDRAWN']},page},[],async(input,signal)=>{
      const state=String(input.state??'');if(!['','PENDING','PUBLISHED','RETIRED','DECLINED','WITHDRAWN'].includes(state))throw new Error('Choose a supported rule state.')
      const data=await request<RulePage>('/api/rules?'+new URLSearchParams({q:queryText(input.query),state,page:String(pageNumber(input.page))}),signal)
      return {items:data.items.map(ruleMetadata),total:data.total,page:data.page,page_size:data.page_size}
    })
    add('society_read_rule','Read permitted immutable rule metadata. Text, authority reference, authors and private review events are excluded. Ordinary reviewed forms are required for every policy change.',{rule_id:{type:'string',minLength:1,maxLength:100}},['rule_id'],async(input,signal)=>ruleMetadata(await request<Rule>('/api/rules/'+encodeURIComponent(careIdentity(input.rule_id)),signal)))
    const incidentState = (value:unknown) => {const state=String(value??'');if(!['','REPORTED','NEEDS_INFO','UNDER_REVIEW','DISMISSED','SUBSTANTIATED','WITHDRAWN'].includes(state))throw new Error('Choose a supported incident state.');return state}
    add('society_find_incidents','Read one page of the current person’s own reports or a currently authorised operational queue. A tagged home alone grants no private case access. Search uses rule title, home, date or case identity; comments are not searched. Reporter identities, comments, evidence, notices and private activity are excluded.',{query:search,state:{type:'string',enum:['','REPORTED','NEEDS_INFO','UNDER_REVIEW','DISMISSED','SUBSTANTIATED','WITHDRAWN']},page},[],async(input,signal)=>{
      const data=await request<IncidentPage>('/api/incidents?'+new URLSearchParams({q:queryText(input.query),state:incidentState(input.state),page:String(pageNumber(input.page))}),signal)
      return {items:data.items.map(incidentMetadata),total:data.total,page:data.page,page_size:data.page_size}
    })
    add('society_read_incident','Read own or currently authorised private incident metadata. It excludes report text, reporter identity, picture identities/bytes, response notices and all activity. This cannot report, decide, publish, respond or charge.',{case_id:{type:'string',minLength:1,maxLength:100}},['case_id'],async(input,signal)=>incidentMetadata(await request<IncidentDetail>('/api/incidents/'+encodeURIComponent(careIdentity(input.case_id)),signal)))
    add('society_find_incident_notices','Read one page of deliberately issued notices for the current person’s active home relationships. Removed notices are absent. Wording, rule text, reporter/handler identity, pictures and responses are excluded. This never shares or sends anything.',{query:search,page},[],async(input,signal)=>{
      const data=await request<NoticePage>('/api/incident-notices?'+new URLSearchParams({q:queryText(input.query),page:String(pageNumber(input.page))}),signal)
      return {items:data.items.map(responseNoticeMetadata),total:data.total,page:data.page,page_size:data.page_size}
    })
    add('society_read_incident_notice','Read a current-home response notice’s frozen metadata, or an authorised handler’s retained metadata. Wording, original report, evidence, identities, private case versions and responses are excluded. Responses require the ordinary human form.',{notice_id:{type:'string',minLength:1,maxLength:100}},['notice_id'],async(input,signal)=>responseNoticeMetadata(await request<IncidentNotice>('/api/incident-notices/'+encodeURIComponent(careIdentity(input.notice_id)),signal)))
    add('society_find_complaints', 'Read the current account’s own service requests or the authorised handling queue. Case ownership is personal; sharing a home does not share another person’s case. Conversation text is not searched.', { query: search, page, status: { type: 'string', enum: ['', 'OPEN', 'ACKNOWLEDGED', 'IN_PROGRESS', 'WAITING', 'RESOLVED', 'CLOSED'] } }, [], async (input, signal) => {
      const status = input.status ?? ''
      if (!['', 'OPEN', 'ACKNOWLEDGED', 'IN_PROGRESS', 'WAITING', 'RESOLVED', 'CLOSED'].includes(String(status))) throw new Error('Choose a supported service status.')
      return request('/api/complaints?' + new URLSearchParams({ q: queryText(input.query), page: String(pageNumber(input.page)), status: String(status) }), signal)
    })
    add('society_read_complaint', 'Read an authorised service request and one page of its conversation. Resident results exclude staff-only notes and their counts; current handler permission is enforced by the server. This does not update or close a case.', { case_id: { type: 'string', minLength: 1, maxLength: 100 }, history_page: page }, ['case_id'], async (input, signal) => {
      if (typeof input.case_id !== 'string' || !input.case_id || input.case_id.length > 100) throw new Error('A service request identity is required.')
      return request('/api/complaints/' + encodeURIComponent(input.case_id) + '?' + new URLSearchParams({ history_page: String(pageNumber(input.history_page)) }), signal)
    })
    add('society_find_documents', 'Search authorised document titles and filenames. Pending/private uploads and counts follow the signed-in account’s current scope; file contents are never returned. This does not upload, approve or archive a file.', { query: search, page }, [], async (input, signal) => request('/api/documents?' + new URLSearchParams({ q: queryText(input.query), page: String(pageNumber(input.page)) }), signal))
    add('society_read_document', 'Read permitted document metadata and one page of allowed versions. Private review history is excluded for ordinary readers. Original bytes and download URLs are not returned; approval requires a separate person using the visible form.', { document_id: { type: 'string', minLength: 1, maxLength: 100 }, history_page: page }, ['document_id'], async (input, signal) => {
      if (typeof input.document_id !== 'string' || !input.document_id || input.document_id.length > 100) throw new Error('A document identity is required.')
      return request('/api/documents/' + encodeURIComponent(input.document_id) + '?' + new URLSearchParams({ history_page: String(pageNumber(input.history_page)) }), signal)
    })
    return () => lifetime.abort()
  }, [user.id, user.mfa_pending, accessScope, openHome])
}
