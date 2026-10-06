import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { apiRule, apiRuleAction, apiIncident, apiIncidentAction } from './incidents-fixtures'
import { financialHeaders } from './maintenance-fixtures'
import { careDate } from './upkeep-fixtures'

export async function finePost(page: Page, path: string, data: Record<string, unknown>) {
  const response = await page.request.post(path, {
    headers: await financialHeaders(page),
    data: { operation_key: crypto.randomUUID(), confirmed: true, ...data }
  })
  expect(response.status(), await response.text()).toBe(200)
  return response.json()
}
export async function fineGet(page: Page, id: string) {
  const response = await page.request.get('/api/fines/' + id)
  expect(response.status(), await response.text()).toBe(200)
  return response.json()
}
export async function apiFineSource(admin: Page, reviewer: Page, reporter: Page, title: string, home = 'demo-flat-A-101') {
  const rule = await apiRule(admin, title + ' supplied policy')
  await apiRuleAction(reviewer, rule, 'PUBLISHED')
  const incident = await apiIncident(reporter, rule, { flat_id: home })
  await apiIncidentAction(reviewer, incident, 'SUBSTANTIATED')
  return incident
}
export async function apiFine(admin: Page, incident: string, title: string, extra: Record<string, unknown> = {}) {
  const source = await (await admin.request.get('/api/fine-sources/' + incident)).json()
  const amount = String(extra.amount ?? '250.25')
  const result = await finePost(admin, '/api/fines', {
    incident_id: incident, source_key: source.source_key, title, amount,
    policy_reference: 'Supplied fictional separate amount authority F-01',
    reason: 'PRIVATE supplied facts and the proposed amount are separately reviewed.',
    response_by: careDate(3), due_date: careDate(7),
    notice_body: 'A fictional fine of Rs' + amount + ' is proposed for your household account. Please respond.', ...extra
  })
  return result.id as string
}
export async function apiFineAction(page: Page, id: string, action: string, extra: Record<string, unknown> = {}) {
  const fine = await fineGet(page, id)
  return finePost(page, '/api/fines/' + id + '/actions', {
    version: fine.version, action,
    reason: 'PRIVATE deliberately reviewed this supplied decision and its intended effect.',
    ...(action === 'RESOLVE' ? { source_key: fine.current_source_key, response_count: fine.response_total, resolution: 'The supplied response context was independently reviewed before this fictional charge.', early_issue_reference: 'Supplied fictional decision permits issuance after this deliberate review.' } : {}),
    ...(action === 'ISSUE' ? { resolution_key: fine.resolution_key } : {}), ...extra
  })
}
export async function apiIssuedFine(admin: Page, reviewer: Page, reporter: Page, title: string, home = 'demo-flat-A-101') {
  const incident = await apiFineSource(admin, reviewer, reporter, title, home)
  const id = await apiFine(admin, incident, title)
  await apiFineAction(reviewer, id, 'NOTIFY')
  await apiFineAction(reviewer, id, 'RESOLVE')
  await apiFineAction(reviewer, id, 'ISSUE')
  return id
}
export async function apiFineReport(owner: Page, fine: string, extra: Record<string, unknown> = {}) {
  return (await finePost(owner, '/api/fine-reports', {
    fine_id: fine, amount: '100.00', payment_date: careDate(0), payer: 'PRIVATE fictional payer',
    method: 'UPI', reference: 'FINE-100-' + crypto.randomUUID(), comment: 'PRIVATE already-paid claim', evidence_id: '', ...extra
  })).id as string
}
export async function apiFineVerify(reviewer: Page, report: string, extra: Record<string, unknown> = {}) {
  const data = await (await reviewer.request.get('/api/fine-reports/' + report)).json()
  await finePost(reviewer, '/api/fine-reports/' + report + '/actions', {
    version: data.version, action: 'CONFIRMED', reason: 'PRIVATE checked an independent fictional payment source.',
    verification_source: 'PRIVATE external original statement', payment_identity: 'PRIVATE row ' + crypto.randomUUID(),
    mode: 'NEW', allocation_amount: '100.00', ...extra
  })
  return (await reviewer.request.get('/api/fine-reports/' + report)).json()
}
