import type { FundEvent } from './collections'
export {
  useFundLoad as useIncidentLoad,
  useFundWrite as useIncidentWrite,
  selectOptions
} from './collections'
export type Rule = {
  id: string
  replaces_id: string
  title: string
  text: string
  policy_reference: string
  effective_from: string
  effective_until: string
  fine_permitted: boolean
  state: string
  author_id?: string
  created_at: number
  updated_at: number
  version: number
  events?: FundEvent[]
  event_total?: number
  event_page?: number
  page_size?: number
}
export type RulePage = {
  items: Rule[]
  total: number
  page: number
  page_size: number
}
export type Incident = {
  id: string
  rule_id: string
  rule_title: string
  flat_id: string
  home: string
  reporter_id?: string
  incident_date: string
  comment: string
  picture_id: string
  state: string
  duplicate_of?: string
  notice_id?: string
  created_at: number
  updated_at: number
  version: number
}
export type IncidentDetail = Incident & {
  rule: Rule
  events: FundEvent[]
  event_total: number
  event_page: number
  page_size: number
  can_participate: boolean
  notices?: IncidentNotice[]
  notice_total?: number
  response_total?: number
}
export type IncidentPage = {
  items: Incident[]
  total: number
  page: number
  page_size: number
  can_report: boolean
}
export type IncidentNotice = {
  id: string
  incident_id?: string
  flat_id: string
  home: string
  rule_id: string
  incident_date: string
  title: string
  body: string
  response_by: string
  picture_id: string
  created_at: number
  version: number
  active: boolean
  can_respond: boolean
  rule: Rule
  responses: IncidentResponse[]
  response_total: number
  response_page: number
  page_size: number
}
export type IncidentResponse = {
  id: string
  actor?: string
  body: string
  created_at: number
}
export type NoticePage = {
  items: IncidentNotice[]
  total: number
  page: number
  page_size: number
}
export type Picture = {
  id: string
  filename: string
  media_type: string
  width: number
  height: number
  sha256: string
  preview_sha256: string
  created_at: number
}
export type IncidentOptions = {
  homes: {
    id: string
    label: string
  }[]
  can_report: boolean
}
export const ruleStates: Record<string, string> = {
  PENDING: 'Awaiting review',
  PUBLISHED: 'Published',
  RETIRED: 'Retired',
  DECLINED: 'Declined',
  WITHDRAWN: 'Withdrawn'
}
export const incidentStates: Record<string, string> = {
  REPORTED: 'Reported',
  NEEDS_INFO: 'More information needed',
  UNDER_REVIEW: 'Under review',
  DISMISSED: 'Dismissed',
  SUBSTANTIATED: 'Substantiated',
  WITHDRAWN: 'Withdrawn'
}
export const incidentActions: Record<string, string> = {
  NOTE: 'Private staff note',
  NEEDS_INFO: 'Ask reporter for information',
  UNDER_REVIEW: 'Begin review',
  DISMISSED: 'Dismiss case',
  SUBSTANTIATED: 'Substantiate case',
  WITHDRAWN: 'Withdraw report',
  REOPEN: 'Reopen review',
  ISSUE_NOTICE: 'Request household response',
  REMOVE_NOTICE: 'Remove response notice',
  DUPLICATE: 'Link a duplicate case',
  REPORTED: 'Incident reported',
  REVISED: 'Report revised',
  PROPOSED: 'Rule proposed',
  PUBLISHED: 'Rule published',
  DECLINED: 'Rule declined',
  RETIRED: 'Rule retired',
  REPLACED: 'Rule replaced'
}
export const incidentLink = (key: string) =>
  new URLSearchParams(window.location.hash.split('?')[1] ?? '').get(key) ?? ''
export const localToday = () =>
  new Intl.DateTimeFormat('en-CA', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    timeZone: 'Asia/Kolkata'
  }).format(new Date())
