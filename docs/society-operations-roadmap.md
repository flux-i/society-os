# Society operations: overview and next workflows

**Recorded:** 4 October 2026, from the user's explicit product direction.
**Status:** Accepted development scope; implementation and production acceptance are separate. Release 0.13/schema 12 is the verified preview with documents, changing overview, account administration, [maintenance cycles/allocations](maintenance-baseline.md), [upkeep work/assets/vendor deadlines](upkeep-baseline.md), [collection campaigns/payment verification/exemptions](collections-baseline.md) and [private rules/incident review/household responses](incidents-baseline.md). Separately authorised fines are next.
**Ordering:** Documents, the changing overview, account administration, maintenance/allocations, upkeep, collections and private incident review have passed local gates. Continue with [separate fine issuance](fines-workflow.md). Deliver the remaining workflows in the order below, with domain, rendered interaction, screenshot and actual native WebMCP checks at every checkpoint.

This document extends the [implementation plan](../housing-society-digital-platform-plan.md) and [execution backlog](../execution-backlog.md). It supersedes their earlier deferral of resident payment reports and configured messaging. Payment gateways, transfers initiated by the platform, bank automation and automatic statutory charge calculations remain outside scope. A collection campaign requests an externally paid contribution; it does not move money.

## 1. An overview that earns its space

The user wants critical, changing information instead of a front page dominated by numbers that remain unchanged for years. Preserve the approved forest/ivory design and editorial typography. Homes, occupancy, owners and tenants remain useful in Homes & people and a compact secondary summary.

| Visible priority | Committee/treasury view | Resident/tenant view | Action |
|---|---|---|---|
| Maintenance | Due this cycle, overdue amount and flats, confirmed collections | Own permitted dues, due date, reported payment awaiting confirmation | Open the relevant cycle or own statement |
| Special funds | Active campaigns, confirmed/partial/unpaid progress | Own requested contribution and confirmation status | Open campaign or report money already sent |
| Decisions | Pending requests, document versions, payment reports and rule cases | Own pending submissions and responses needed | Open the exact review or submission |
| Help and maintenance work | Unassigned/open/ageing cases, urgent tasks | Own active requests and latest public update | Open case or scheduled task |
| Communication | Published notices, acknowledgement deadlines, failed delivery | Latest permitted notices and requested acknowledgements | Read, acknowledge or inspect failed delivery |
| Upcoming obligations | AMC expiry, inspections, scheduled repairs, meetings | Resident-relevant dates and interruptions | Open supporting record |
| Recent changes | Scoped confirmations, approvals and resolutions | Changes to own records and permitted publications | Open source record |

First implement the rows supported by existing verified records, requests, complaints, notices and documents. Add maintenance, campaign and messaging measures when those modules exist. Do not invent an overdue date from an entry date, show invented collection totals, or substitute a zero for an unavailable metric. Give dates, periods and update times explicit labels. Each count must use the same current audience restrictions as its destination; finance totals require finance permission. An error in one source must leave other sources usable with a local retry. Empty states should offer a permitted next action.

**Acceptance:** Changing a record, approval, service case or publication changes the appropriate overview after refresh; every actionable tile opens its filtered destination or exact record. Account switching, ended membership and revoked roles remove restricted data/counts. Verify 1440px, 768px, 375px and 320px layouts, keyboard actions, pending/error/empty states and native WebMCP read scope. No static celebratory count displaces the attention queue.

## 2. Maintenance administration and tracking

Cover both money and upkeep: maintenance periods and approved given charges, opening dues, adjustments, partial receipts, excess credits, due dates, reminders, flat statements, paid/part-paid/unpaid status; repairs, recurring service tasks, assignments, vendor visits, work status and AMC/inspection deadlines.

An authorised operator prepares a cycle using committee-approved amounts and participating flats. A separate eligible reviewer publishes a frozen version before individual dues become visible. Store exact paise, the source/policy reference, explicit due date and a correction trail. There is no inferred statutory rate, interest, owner-versus-tenant liability or automatic penalty. Residents see only their entitled homes; operator-wide overdue lists are private.

Allocate confirmed received entries to approved maintenance charges explicitly and transactionally. Handle partial money, unallocated credit and linked corrections without silently changing historical receipts. Independently reconcile expected, allocated, outstanding and credit amounts. Reuse the verified manual receipt mechanism; creating a charge or submitting evidence never issues a received-money receipt.

**Acceptance:** Multiple periods, partial payments, an advance, a cancelled/corrected charge, future due dates, ended membership, duplicate confirmation and stale concurrent edits reconcile against independently specified amounts. Finance permissions and current flat relationships gate every list, total, export and receipt.

## 3. Special fund campaigns and external payment reports

Create a collection campaign for a repair, festival, improvement or other approved purpose: title, purpose, source approval, contribution type (fixed requested amount or voluntary), audience/participating flats, start/due dates and optional target. Preview participants and total requested amount before separate approval publishes it. Closing a campaign preserves its ledger and history.

Track **unpaid, part paid, payment reported / awaiting confirmation, confirmed paid, waived/exempt and corrected** separately. Residents can report money already sent with amount, date, method/reference, comment and private evidence. An authorised treasury reviewer verifies it against an external source or rejects/requests clarification. A screenshot alone is a claim. Confirmation creates or links exactly one received entry and one receipt through a stable operation identity; allocations cannot exceed the entry's usable amount. Never count both the report and its resulting entry as separate collections.

Show per-flat completion to eligible operators and each resident's own status. Optional society-wide aggregate progress excludes names and private transaction details. Resolve who owes a joint/shared flat once, rather than asking every resident to pay the full amount.

**Acceptance:** Two people reporting the same payment, one payment allocated across purposes, partial/extra payment, rejected evidence, reviewer revocation, a lost confirmation response, repeated retries and a reversed confirmed entry must preserve correct amounts and receipt identity. No bank API or gateway is needed.

## 4. Registered contacts and WhatsApp/email delivery

Support notices to all current residents, a wing, owners, tenants, selected flats or selected people; private receipt delivery to its currently entitled recipients; and intentional publication of approved financial statements. Maintain registered phone/email, verification/attestation, channel preference, opt-in/opt-out and recipient identity history. A phone number is not itself proof of identity or consent. Deduplicate shared contacts while preserving each authorised delivery purpose.

The composition screen previews content and exact recipient count before sending. Resolve the content audience and current membership again immediately before dispatch. A notice targeted at tenants must not go to all owners by accident. Message receipts and finance links must not expand the underlying document audience. Publish/share a specific immutable version; drafts and committee-only evidence cannot be mass-shared. Use authenticated portal links by default; warn within the sharing decision when a deliberately authorised original attachment will leave portal revocation control.

Build one provider-neutral delivery queue with configured WhatsApp/email adapters. Keep queued, provider-accepted, delivered, read (only when explicitly reported), failed, skipped/ineligible and opted-out states distinct. Preserve provider message IDs and signed webhook deduplication; use bounded retries and surface uncertain outcomes for reconciliation instead of claiming exactly-once delivery from an external provider. A clicked manual share link is only a manual share action, not confirmed delivery. Do not send real messages during development; exercise synthetic adapters and provider failure fixtures.

Meta's published policy requires recipient permission and opt-out handling, and approved templates for business-initiated messages outside the customer-service window. Provider onboarding, current templates/pricing, domain/number verification and society-owned credentials are live-channel activation inputs. Refresh them at integration time. [WhatsApp Business Messaging Policy, updated 23 September 2026](https://business.whatsapp.com/policy).

**Acceptance:** Owner/tenant targeting, changed membership/contact/consent while queued, attachment permission, failed/unconfigured providers, invalid webhook signatures, duplicate/out-of-order callbacks, bounced email and opt-out suppressions. No unconfigured provider may display a fake successful send. Sending tests remain synthetic until authorised live configuration is present.

## 5. Rule reports, evidence, approval and fines

Maintain a versioned rule register with committee-provided rule text, effective dates, authority/policy reference and whether a fine is permitted. Never infer fine powers or amounts; society state, approved rules and the responsible review process are not yet supplied.

Residents can capture/upload a picture, tag a flat, choose the broken rule and add an incident date and comment. Preserve the original private evidence and access audit; strip unnecessary location/device metadata from any shareable derivative. Reporting another flat requires a bounded flat-only selector, not its resident/contact directory. Avoid a public allegation feed. Reporters see their own submission; handlers see cases they can process; subjects see the allegation/evidence permitted for their response, with reporter identity protected according to the adopted policy.

The workflow is **report → review / request information → dismiss or substantiate → prepare fine → authorised decision/notice → issue → external payment confirmation or waiver/reversal**. The reporter cannot approve their own case. A supported incident does not itself debit a flat. An authorised fine decision records rule version, accountable flat/person, amount, reason, notice, response/appeal status and eligible approver before a separate finance posting is created. A dispute can pause the charge according to explicit policy. Preserve linked corrections/waivers and source evidence; do not delete accusations or money entries to hide an error.

**Acceptance:** Self-approval, duplicate reports, unapproved evidence, amended/expired rules, incorrect flat tags, malicious image content, stale decisions, appeals, waived/reversed fines and duplicate finance posting. No unsubstantiated report appears as unpaid debt or confirmed misconduct on the overview.

## 6. Income statements, balance sheets and accountant files

Finance operators may keep accounts in Excel or another tool. Upload their prepared income statements, balance sheets, budgets and audit reports with financial period, author/source, draft/final status and immutable versions. Start with safe PDF originals and add bounded, validated XLSX/CSV support deliberately; the current document validator does not accept spreadsheets. Do not run spreadsheet macros, external links or formulas during upload. Label files as externally prepared; uploaded figures are not silently imported into the portal ledger.

Finance staff can review internally, then deliberately publish a selected approved version to owners, tenants, all residents or another authorised audience and send it through configured channels. Record the publication/review decision and version shared. Tenants receiving the authorised statement do not gain access to private flat ledgers or transaction evidence. Registry administrators alone do not receive finance preparation privileges; a separate finance grant is required.

Provide scoped operational exports for the manual ledger, maintenance/fund reconciliation and receipts, with spreadsheet formula-injection protection. A future derived statutory balance sheet needs an approved chart of accounts, opening ledger and accountant reconciliation; an uploaded balance sheet does not prove those exist.

**Acceptance:** Draft hidden, separate reviewer, replacement version retains original, controlled tenant publication, unpublish prevents new portal downloads, revoked finance access, macro/external-link and oversized archive rejection, stable exports and message/link audience intersection.

## 7. Researched additions worth scheduling

Apartment-management vendors document assets, maintenance records, vendor expenses, budgets, emergency contacts and module-specific roles alongside collections and complaints. These sources suggest useful coverage, not proof our society needs every vendor feature. [ADDA accounting capabilities](https://ind.adda.io/society-accounting-software), [MyGate module/role inventory](https://adminfaq.mygate.com/articles/130320-what-are-the-different-types-of-admin-roles-available-on-the-dashboard).

Our prioritisation inferred from those capabilities and the user's needs:

| Addition | Reason | Position |
|---|---|---|
| Asset/AMC and inspection calendar | Lift/pump/fire-equipment service and expiry dates belong in the attention queue | Extend maintenance; upcoming obligations after document expiry data |
| Vendor directory, service visits and expense evidence | Connect an issue to accountable work and its approved cost | After maintenance and evidence links |
| Private reminders and delivery exception queue | Track missed dues/acknowledgements without exposing public defaulter lists | Alongside campaigns and messaging |
| Emergency contacts and outage notices | Residents need a quick path during water/power/service interruption | Small community slice after targeted notices |
| Budget versus recorded collections/expenses | Committee can identify shortfalls without pretending uploaded statements are a complete general ledger | After scoped finance exports |
| Meeting agenda/minutes and acknowledgement tracking | Preserve approved decisions and track required responses | After document publication and messaging |
| Move-in/out checklist and contact updates | Current occupants and recipient lists should stay accurate | Alongside account/role administration |

Visitor gate hardware, parking enforcement, amenity booking, elections, AI, payment gateways and a full accounting engine are candidates only if a later need is established. They are not implicit additions to this delivery.

## 8. Delivery order and evidence

1. **Verified locally:** private documents with full checks, browser captures, native WebMCP and recovery evidence.
2. **Verified locally:** changing overview with exact deep links, scoped full counts, partial failures, inspected captures and native WebMCP.
3. **Verified locally:** explicit role/account administration, preserving separate financial powers, bounded terms, suspension/resumption and current client/native scope.
4. **Verified locally:** [maintenance cycles and explicit charge/receipt allocations](maintenance-baseline.md), and [upkeep tasks, assets/vendors and deadlines](upkeep-baseline.md).
5. **Verified locally:** [fund campaigns and resident reports of external payments](collections-baseline.md), with separate treasury verification, original receipts and exemptions.
6. **Private evidence/rules verified locally:** [independent publication, incident review and frozen response notices](incidents-baseline.md), with 123 Go, 128 ordinary browser and 29 actual native WebMCP cases, 67 inspected final captures and schema-12 recovery. Continue the [separate authorised fine checkpoint](fines-workflow.md) using the existing financial posting contract. Its expectation brief is saved before code.
7. Deliver contact preferences, recipient targeting, synthetic provider queue and safe channel configuration; enable a real provider only when credentials/onboarding are available.
8. Extend financial statement types, safe spreadsheet originals, deliberate publication and configured sharing; add scoped exports and the prioritised asset/vendor/community slices.
9. Finish PWA/offline/update behavior, migration rehearsals and production/pilot acceptance. Prepare independent infrastructure steps throughout; real-data/policy/provider inputs block only dependent activation.

Every checkpoint records its defined permissions and amounts before implementation, regressions for observed failures, browser clicks/open menus/keyboard/viewport checks, inspected screenshots, actual native WebMCP allow/deny results and recovery implications. Public documents record executed cases and remaining gaps. No checkpoint or provider activation is complete merely because its plan is written.
