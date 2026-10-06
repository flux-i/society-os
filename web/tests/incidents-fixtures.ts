import { expect } from '@playwright/test'
import type { Page } from '@playwright/test'
import { deflateSync } from 'node:zlib'
import { financialHeaders } from './maintenance-fixtures'
export function scenePNG(): Buffer {
  const crc = (data: Buffer) => {
    let value = 0xffffffff
    for (const byte of data) {
      value ^= byte
      for (let i = 0; i < 8; i++)
        value = (value >>> 1) ^ (value & 1 ? 0xedb88320 : 0)
    }
    return (value ^ 0xffffffff) >>> 0
  }
  const chunk = (name: string, data: Buffer) => {
    const size = Buffer.alloc(4)
    size.writeUInt32BE(data.length)
    const body = Buffer.concat([Buffer.from(name), data]),
      sum = Buffer.alloc(4)
    sum.writeUInt32BE(crc(body))
    return Buffer.concat([size, body, sum])
  }
  const header = Buffer.alloc(13)
  header.writeUInt32BE(128, 0)
  header.writeUInt32BE(96, 4)
  header[8] = 8
  header[9] = 2
  const raw = Buffer.alloc(96 * (1 + 128 * 3))
  for (let y = 0; y < 96; y++) {
    for (let x = 0; x < 128; x++) {
      const i = y * (1 + 128 * 3) + 1 + x * 3
      raw[i] = x > 35 && x < 85 ? 93 : 230
      raw[i + 1] = x > 35 && x < 85 ? 118 : 231
      raw[i + 2] = x > 35 && x < 85 ? 76 : 213
    }
  }
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', header),
    chunk('tEXt', Buffer.from('Location\x00PRIVATE GPS device test fixture')),
    chunk('IDAT', deflateSync(raw)),
    chunk('IEND', Buffer.alloc(0))
  ])
}
export async function apiRule(
  page: Page,
  title: string,
  extra: Record<string, unknown> = {}
) {
  const response = await page.request.post('/api/rules', {
    headers: await financialHeaders(page),
    data: {
      operation_key: crypto.randomUUID(),
      title,
      text: 'Fictional supplied rule: keep shared corridor access clear.',
      policy_reference: 'Supplied fictional committee policy R-01',
      effective_from: '2026-01-01',
      effective_until: '',
      fine_permitted: true,
      replaces_id: '',
      confirmed: true,
      ...extra
    }
  })
  expect(response.status(), await response.text()).toBe(200)
  return (await response.json()).id as string
}
export async function apiRuleAction(page: Page, id: string, action: string) {
  const x = await (await page.request.get('/api/rules/' + id)).json()
  const response = await page.request.post('/api/rules/' + id + '/actions', {
    headers: await financialHeaders(page),
    data: {
      operation_key: crypto.randomUUID(),
      version: x.version,
      action,
      reason:
        'Separately checked the supplied fictional rule and its authority',
      confirmed: true
    }
  })
  expect(response.status(), await response.text()).toBe(200)
  return await (await page.request.get('/api/rules/' + id)).json()
}
export async function apiIncident(
  page: Page,
  rule: string,
  extra: Record<string, unknown> = {}
) {
  const response = await page.request.post('/api/incidents', {
    headers: await financialHeaders(page),
    data: {
      operation_key: crypto.randomUUID(),
      version: 0,
      rule_id: rule,
      flat_id: 'demo-flat-A-101',
      incident_date: '2026-01-02',
      comment:
        'PRIVATE fictional observation and original reporter description.',
      picture_id: '',
      confirmed: true,
      ...extra
    }
  })
  expect(response.status(), await response.text()).toBe(200)
  return (await response.json()).id as string
}
export async function apiIncidentAction(
  page: Page,
  id: string,
  action: string,
  extra: Record<string, unknown> = {}
) {
  const x = await (await page.request.get('/api/incidents/' + id)).json()
  const response = await page.request.post(
    '/api/incidents/' + id + '/actions',
    {
      headers: await financialHeaders(page),
      data: {
        operation_key: crypto.randomUUID(),
        version: x.version,
        action,
        reason:
          'PRIVATE supplied evidence was deliberately reviewed for this case',
        duplicate_of: '',
        title: '',
        body: '',
        response_by: '',
        confirmed: true,
        ...extra
      }
    }
  )
  expect(response.status(), await response.text()).toBe(200)
  return await (await page.request.get('/api/incidents/' + id)).json()
}
export async function apiPicture(page: Page) {
  const response = await page.request.post('/api/incident-pictures', {
    headers: {
      ...(await financialHeaders(page)),
      'Content-Type': 'application/octet-stream',
      'X-Operation-Key': crypto.randomUUID(),
      'X-Picture-Filename': 'private-scene.png'
    },
    data: scenePNG()
  })
  expect(response.status(), await response.text()).toBe(200)
  return (await response.json()).id as string
}
