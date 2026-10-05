# Account administration — local acceptance

Verified on 5 October 2026 as **0.9.0-dev / schema 8**, against the [expectation brief](account-administration-workflow.md). A current administrator can inspect bounded account history, grant/end explicit appointments and suspend/resume an account. Financial powers remain separate from registry/account administration.

| Executed check | Result |
|---|---|
| Go formatting, vet and race-enabled gate | 83 declarations passed; database 207.209 s, backup 38.420 s, HTTP/server 58.558 s |
| TypeScript and production build | Passed; no new dependency |
| Ordinary Chromium gate | 81 cases passed across 13 isolated synthetic suites |
| Account journeys and visual inspection | 11 cases; 51 distinct captures inspected through nine contact sheets, with critical original captures checked separately |
| Overview focus regression | Ten Overview cases passed; the additional delayed-refresh case preserves a destination focused before completion |
| Actual native Chrome 154 WebMCP | 17 cases passed in 23.6 s; 13 administrator tools, 11 financially entitled resident tools, 10 for an unentitled tenant |
| Windows amd64 cross-build | Passed; target executable not run on the supplied computer |
| Retained preview/readiness | Matching 0.9 binary/assets at `127.0.0.1:8080`, schema 8, ready 200; matching 0.8 retained |
| Consistent post-upgrade snapshot/fresh restore | Passed; 16.927 ms on the development Mac, with independent hashes/integrity/foreign-key/count checks |

The coherent captured gate is `reports/local/account-administration-eval-final.log`. A later QA capture correction scrolls the focused Overview link into the viewport before capture; that affected case passed separately in `reports/local/account-overview-focus-capture.log` (2.0 s). No application code changed after the complete gate. Repeated or targeted runs are not added to case totals.

## Permissions and preserved identity

Only current administrators receive `can_manage_accounts`. Committee members retain operational review/service handling and financial reads; Treasurer adds financial reads/posting and accounting documents; Accountant/auditor adds financial reads and permitted approved accounting documents without financial writes or registry-wide administration. Privileged appointments require a current confirmed authenticator. Residents retain their independent current home/finance entitlements after a role expires or ends.

Appointments start on confirmation and last an explicitly supplied 1–365 days. Mutations require a reason, authority/identity attestation, five-minute password/applicable-factor confirmation and expected access version. The actor, target/version check, writer reservation, successor guard and immutable audit share a transaction. Self-changes, overlapping duplicate terms and removal of the last active verified factor-enrolled administrator are denied. Expiry remains real: the successor guard does not prolong a term.

Grant/end invalidates the recipient's sessions and outstanding links. Suspension additionally ends all unrevoked appointments and recovery codes, preserving identity, factors, money and history. Explicit resumption restores activated sign-in eligibility only; old roles, links and sessions stay ended. A pending identity requires a newly verified invitation after resumption. Password recovery cannot bypass suspension.

Account reads expose bounded pages of 20 appointments/activity events with full counts and version. The two new native account tools return bounded metadata, excluding email, private verification/activity notes and security material. They offer no mutation. All APIs retain current server-side scope checks; client permission flags cannot authorise a write.

The principal read transaction produces an opaque access fingerprint covering the current role/membership set and each active relationship/finance entitlement. The frontend clears the previous workspace's loaded data when this scope changes, including when another home keeps a permission true. Unchanged checks preserve forms. Native reads check scope before and after the response and reject data captured under the former scope. The fingerprint is a cache boundary, not a credential.

## Findings and regression evidence

Five product groups closed:

- **AR-01:** actual Treasury appointments were labelled Committee; displayed roles now match their recorded powers.
- **AR-02:** old factor verification remained usable after removal of the current factor; pending/fresh authority now requires the currently enrolled factor. The retained domain RED required denial and got success.
- **AR-03:** an invalidated invitation still promised “Your place is ready”; unavailable/loading links now have accurate headings.
- **AR-04:** native tool registration retained obsolete role state after natural expiry, preventing valid personal reads; registration follows the current access signature.
- **AR-05:** rendered data and an in-flight native response outlived narrowed access. The retained financial RED showed ₹37.25 after Auditor expiry where the personal view required ₹0.00. A separate membership RED returned two homes after one entitlement ended. The shared scope boundary now removes old details/totals and rejects the old response; unchanged checks preserve an open entry. Both manifestations belong to one group.

**OV-03 follow-up:** delayed Refresh completion stole focus after the person moved to View receipts. Completion restores focus only if it remains on Refresh or the document body. This extends the existing Overview focus group and adds no new group.

Process totals are **43 documented groups: 42 product/UI and one operational**, up from 38. Fixture/selector/route/keyboard-wait corrections, capture positioning, the corrected recovery table name and an overly strict whole-audit-table comparison are QA corrections. The older snapshot's eight audit rows are unchanged; one legitimate preview MFA-recovery audit was subsequently appended. Audit preservation requires retaining prior rows, not suppressing later valid activity.

[Domain checks](../internal/database/account_administration_test.go) cover preserving upgrade, role/factor boundaries, validation, duplicates, self denial, current actor/expiry, concurrent versions, target-bound endings, recoverable successor protection, suspension/resumption, immutable history, pagination and stable versus changed home/finance scope. [HTTP checks](../internal/server/account_administration_test.go) cover scoped routes, strict payloads and write protections. [Rendered journeys](../web/tests/account-administration.spec.ts) exercise actual decisions, failures, busy saves, focus/dismissal, stale reload, pending invitation handover, pagination and opened menus at 1440×900, 1440×480, 768×1024, 375×812 and 320×568. These are specific executed cases, not every possible interaction at every size. [Native journeys](../web/tests/webmcp.spec.ts) execute real registered tools, cancellation/bounds, visible appointment changes, successor handover, expiry and the captured membership-read race.

The private capture manifest `reports/local/account-administration-review/review.json` records all 51 hashes/dimensions and inspection status. REDs, traces, screenshots, build artifacts, credentials and databases remain outside Git.

## Recovery and remaining scope

The schema-7 pre-upgrade bundle has **462,848 bytes**, SHA-256 `75dde24235b03aa45de1ce976baad559150162df265237adc0c5009e493a4729`, with an independently checked 18.9 ms fresh restore. The schema-8 post-upgrade bundle has **471,040 bytes**, SHA-256 `a17ac0fd2039f56a8680933d466d07b0ff877e6ea30e58a1aff9df40e5a22b0c`. Source/restored hashes, integrity, foreign keys and schema agree; counts remain three buildings, 118 flats, 154 people, 155 memberships, four identities and three seed appointments. Prior identity/grant/factor data and audit rows survive. Restored temporary sessions, links and recovery codes are empty. Retain the matching release and MFA key separately.

The separate [meaningful account recovery test](../internal/backup/account_administration_test.go) restores a nonempty auditor term, suspended identity/version, revoked treasury appointment and access history, excluding later resume/revoke decisions. It denies old credentials and checks new permitted login/read scope while retaining encrypted confirmed factors. The preview snapshot alone does not prove those changed-account cases.

Production authority/custodians, real policies/data, off-site recovery and actual Windows performance/operation remain separate acceptance. External Claude/Codex client execution and live provider messaging are not claimed. Next: [maintenance cycles and explicit receipt allocations](maintenance-workflow.md), then upkeep and the full operations roadmap. No payment initiation or runtime LLM is introduced.
