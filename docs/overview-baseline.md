# Changing overview — local acceptance

Verified on 5 October 2026 as **0.8.0-dev / schema 7** against the [expectation brief](overview-workflow.md). This read-only aggregation checkpoint changes no deployed migration or stored financial record.

The overview now prioritises decisions, service requests, document work/deadlines, approved notices and confirmed supplied financial balances. Every MFA-cleared resident gets a personal Overview. Home/occupied/vacant/owner/tenant totals remain in Homes & people. The [operations roadmap](society-operations-roadmap.md) preserves the user's maintenance, campaigns, external-payment verification, fine/evidence, WhatsApp/email and financial-statement requirements; those workflows are subsequent work, not invented overview metrics.

| Executed check | Result |
|---|---|
| Go formatting, vet and race-enabled tests | 72 declarations passed |
| TypeScript and production build | Passed |
| Ordinary Chromium regression | 69 cases passed across 12 isolated synthetic suites |
| Overview journeys | Nine cases passed; 32 distinct final captures visually inspected in eight contact sheets, with detailed original/crop inspection |
| Actual native Chrome 154 WebMCP | 12 cases passed in 14.0 seconds; 11 eligible tools, 10 for an unentitled tenant |
| Windows amd64 cross-compilation | Passed; executable not run on the Windows target |
| Retained preview/readiness | Matching 0.8 binary/assets at `127.0.0.1:8080`; schema remains 7; ready returns 200 |
| Consistent snapshot/fresh restore | Passed; 34.637 ms on the development Mac |

## Data and permission contract

Five independent `GET /api/overview/{section}` reads provide current full counts and up to four metadata rows per section. Each uses one transaction with a reread session, MFA, role and membership scope. There is no periodic polling or browser cache of personal data. Loading/unavailable values are unknown, rather than zero; section retries preserve other usable sources. The combined attention list shows at most eight of those preview rows, with scoped total and explicit partial state. Existing section links expose full lists.

Financial reads require current explicit entitlement. Balances use unreversed confirmed entries exactly in paise, grouped by home before positive balances and credits are summed. Thus a credit in one home does not conceal another home's balance. Money received uses the labelled last 30 India calendar days, including today; supplied dates are not overdue-bill evidence. Draft work appears only for treasury operators. A pending/failed receipt PDF is a document-generation state, not failed payment.

Review work excludes self-approval and distinguishes own requests waiting for a reviewer from own changes requested. Service scope is personal to reporters or current handlers. Staff notes do not change a resident's visible timestamp, ordering or counts. Notices require approval/current audience. Deadline counts use the current approved non-archived contract/AMC head, so an unapproved replacement cannot hide an existing deadline. Document approval work requires an available original and a different eligible reviewer; interrupted originals are uploader work. Overview DTOs exclude original bytes, filenames, private conversation and review notes.

New request/entry deep links complement existing notice/case/file links. Closing clears the deep-link selection before a subsequent reload. Browser Back restores focus to the prior overview link, or the main content when the item has left the queue; refreshed controls retain keyboard focus after completion. Native `society_read_overview` reads the same section contract, rejects unentitled finance/unsupported input, respects cancellation/session lifetime and performs no writes. Residents can open Overview through the native workspace tool. External Claude/Codex client execution remains unverified.

## Evidence and repaired findings

Seven new domain declarations plus one HTTP declaration cover full counts beyond preview limits, exact home balances, reversals/drafts, calendar boundaries, self-review, current roles/memberships, hidden staff activity, notice audiences, validated originals, replacement/archive/expiry boundaries, pending MFA and revoked sessions. Nine browser journeys exercise every new destination, keyboard refresh, real report and file upload, separate decisions, scoped tenant content, exact financial amounts, deep-link close/reload/Back, each source failure, combined outage/partial recovery, delayed reads/abort, interrupted-file completion, four viewport sizes and opened case menus. Two new native journeys execute real registered tools after a UI report, preserve open dialogs, open the resident overview and deny stale/unentitled reads. Earlier workflow coverage remains in the full gate.

Three finding groups closed:

- **OV-01 — calendar:** UTC overview dates disagreed with existing Asia/Kolkata dates after midnight. The retained RED finance check expected 2,500 paise received and got zero. Aggregation now uses the society calendar; a fixed `2026-10-04T18:30:01Z` regression independently expects 5 October and a 6 September period start.
- **OV-02 — error presentation:** finance failure/retry duplicated between the attention queue and finance panel; a denied request heading still claimed to be opening. Each source now has one failure display; the linked request uses an unavailable heading and scoped 404 explanation.
- **OV-03 — focus:** returning from a case lost the originating focus, and disabling Refresh during loading lost its keyboard focus. History-bound focus restoration and refresh completion restore usable focus.

QA corrections are not product findings: unsupported labels/headings, an incorrect role fixture value/table, direct synthetic API changes without manual refresh, awaiting native asynchronous dialog URL cleanup, and full-page capture scroll positioning. Repeated runs are not added together. Current process total is **38 finding groups: 37 product/UI and one operational**.

Private evidence: `reports/local/overview-release-gate.log`, `reports/local/overview-calendar-red.log`, `reports/local/overview-review/review.json`. All 32 final screenshot hashes/dimensions and inspection status are recorded; captures and databases remain outside Git. [Domain checks](../internal/database/overview_test.go), [HTTP checks](../internal/server/overview_test.go), [rendered journeys](../web/tests/overview.spec.ts), [native WebMCP](../web/tests/webmcp.spec.ts).

The pre/post preview snapshots both contain 462,848 bytes with SHA-256 `302a3fe40703c4dbabd031e4481ec9284f158f10f29540f223c344edb2c2d03b`, independently checked with Python. Fresh restore retains three buildings, 118 flats, 154 people and 155 memberships. This snapshot has no seeded document originals; the separate existing original-byte recovery test still passes in this gate. Keep the MFA key separately. No target-Windows performance, production infrastructure, live provider messaging, real policies or off-site recovery is claimed.

Next: explicit role/account administration, then the ordered maintenance, collection, evidence/fine, messaging and statement slices. Production acceptance remains open; missing real inputs block dependent activation only.
