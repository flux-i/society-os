# Meeting agendas, minutes and personal acknowledgements — checkpoint evidence

**Locally accepted: 0.22.0-dev/schema 21, 8 October 2026.** The [before-code contract](meetings-acknowledgements-workflow.md) defines current authority, separate publication review, immutable versions and exact personal acknowledgements. Required gates, final visual review and matching recovery pass. Personal publication is verified for [application `e8d1d53`](https://github.com/flux-i/society-os/commit/e8d1d53aeba576bd010c4daa5c5187a57c28c299). The preceding [0.21 checkpoint](community-services-baseline.md) and its immutable archive remain accepted. Cumulative closed findings are **87 unique groups: 85 product/UI and two operational**.

## Implemented outcome

Community adds Meetings & minutes and a private meeting desk while preserving noticeboard and service links. Current community operators supply an agenda, schedule, location and exact frozen homes. A different current reviewer approves or declines the exact version. Pending replacements, declines and own proposal cancellation preserve the approved predecessor. Minutes and public cancellation are separately reviewed linked versions which retain the original agenda, schedule and area. A published withdrawal removes current access and retains the original and history.

Times are supplied UTC instants, displayed as Asia/Kolkata. Passage of a schedule labels a meeting as past; it cannot manufacture a held time or minutes. Supplied minutes require an explicit held time at or after the approved start and no later than now. Published minutes/cancellation cannot become a new agenda for another event.

An operator deliberately requests personal acknowledgement and may supply a deadline. A current eligible member acknowledges only the exact published version and approval fingerprint. Another person cannot respond on their behalf. One person in several homes counts once; an existing unique-account constraint remains protected. Privileged operational access alone does not grant a personal household response. New publications reset current-version acknowledgement status while retaining old originals. Current eligible/acknowledged/outstanding counts remain separate from historical response totals.

First writes recheck active identity, current membership, publication/fingerprint, requested acknowledgement, confirmation and actor-bound operation identity in the reserved writer. Successful retries retain the original action; changed payloads conflict. An already accepted exact retry can return its old action while current access persists, without acknowledging a successor. Pending private successors do not disable acknowledgement of their approved predecessor. Ended membership, cancellation and withdrawal prevent new responses.

The independent Overview source counts approved agendas within 30 days, the actor's outstanding current-version acknowledgements and other operators' proposals. Full counts are independent of at most four rows. An upcoming agenda and its personal response action share one published row; its private pending review remains a distinct destination. Errors show unknowns and offer a source-specific retry. Residents never receive other people's response lists, private proposal reasons or decision history.

Protected meeting routes preserve CSRF/origin checks, current scope and strict bounded decoding. Proposal/review writes require fresh privileged authority; a personal acknowledgement is an ordinary protected resident action. Meeting actions create no charge, fine, receipt, payment initiation, attendance, vote, consent or message handoff.

## Executed coverage and source continuity

Twelve new Go declarations bring the inventory to **227**. The focused race selection runs **13 declarations**: 11 database, one HTTP and one populated recovery, including the preserved historical schema-20 regression. It covers separate/self/stale authority, frozen/current audiences, private predecessors, exact version/fingerprint, unchanged/changed retries, concurrent decisions/identical acknowledgements, time/state boundaries, independently derived Overview counts, bounded publication/response/history pages and retained originals. The final focused cohort passes database 74.012 s, server 7.251 s and backup 13.121 s.

Complete formatting/vet/race/TypeScript passes in **1471.023 s**, ending `2026-10-08T06:46:53.357277+00:00`; log SHA-256 `8efb32245ed768149afb94611f6e700c4013ec4d07f1eb67e33b60af925b0e31`. Package durations include database 1004.609 s, backup 165.241 s and server 152.755 s; security uses its unchanged cached result. Real password hashing and validators remain enabled. Every current backend file stays exact throughout. Five frontend before-sources are strictly reconstructed and hashed in private continuity proof; the final build repeats TypeScript after the last copy repair. No literal green aggregate invocation is claimed.

All **197 ordinary cases across 29 isolated suites** pass with the current application and test manifest, ending `2026-10-08T06:58:49.726993+00:00`; log `673fb7ca2998ae91e318222291bbba2b4bdc506ed896bba7d0d3aae7d52cbcbb`. All **56 actual native Chrome WebMCP cases across nine suites** pass, ending `2026-10-08T06:50:53.511736+00:00`; log `2f123b5320fbe06d0ac783bda974d6df34de5872a66192174b5133b89c486362`.

Seven focused rendered journeys pass together, including actual agenda preparation, selected homes, different-account review, resident acknowledgement, supplied minutes, stale publication/revision and preserved human input, uncertain-response exact retry, private response counts, declines/cancellations/withdrawal, immediate denial and retained history. Menus are opened and selected, with hover/focus, Home/End/arrows/Enter, Escape, focus restoration, Tab trapping, pagination, internal scrolling and busy dismissal. Meaningful empty, loading, unavailable/retry, conflict and pending states are exercised at 1440 desktop, 768 tablet, 375/320 phones and 320×440.

All **59 final PNGs** are actually inspected in **159 native-scale parts across 40 viewed sheets**, plus eight separately opened originals. The final seven-case capture ends `2026-10-08T06:48:45.077961+00:00`; log `fbeec4b97575815e1fcaac3a6e97046806ac991322d74e4992066e85a5897f14`. Final controls, checkbox spacing, deadline labels, long titles, fixed close/header, selected-menu borders and corrected record-conflict copy are reviewed. These finite journeys/captures do not prove every possible interaction.

Four new native cases use real discovery/execution of `society_find_meetings`, `society_read_meeting`, meeting Overview and deliberate meeting navigation. Bounded outputs omit titles, agendas, minutes, locations, exact homes, fingerprints, private drafts/reasons and response identities. Tools preserve an open human form and cannot acknowledge, prepare, approve, publish or send. Current-source rereads reject held detail/list/Overview results after withdrawal; current identity/scope guards and abort checks remain active.

The current build, seven-case capture, full ordinary and full native stages share the same source manifest and unchanged **18-file runtime pair**. Earlier red/superseded stages remain private and excluded. Production build sizes are **1006.79 kB JavaScript/265.27 kB gzip** and **163.55 kB CSS/30.20 kB gzip**, with 182 modules. The chunk warning remains assigned to measured performance/code splitting. Windows amd64 cross-compilation produces **22,942,720 bytes**, SHA-256 `d7a273f7772f72d7e50995a0ea93dceedc7581658f0de6c608be32afd61890b8`; actual Windows/8 GB-host execution is untested.

## Findings and QA corrections

Six groups are closed after all required gates and matching recovery:

- **M-01:** Same-second retained acknowledgements used a random identity as their only ordering tie-breaker. A 21-version journey displaced the original. Descending immutable publication version now precedes that identity when timestamps tie.
- **M-02:** Nested shared checkboxes lacked `display:flex`, joining adjacent wording and losing intended gaps. Explicit shared row layout and nested meeting field layout restore spacing.
- **M-03:** Deadline help inside its label became part of the accessible name. A concise label and separate `aria-describedby` help restore exact field identification.
- **M-04:** A meeting revision effect depended on an unstable reload callback, producing a request storm. The shared callback now has a stable identity. A live observation counted 1663 successful and 400 failed list requests; the disposable raw server log was removed by cleanup, while tool observation and failed-run evidence remain. Do not claim that raw log was retained.
- **M-05:** A native held result returned after publication withdrawal because current identity alone did not recheck the source. Detail/list/Overview now reread the currently permitted source and reject changes. A real Chrome failing case and four passing native journeys retain the evidence.
- **M-06:** Meeting conflict routes fell through to a registry-specific “home” message. Adding the meeting prefix to the existing record-conflict branch corrects both stale proposal and personal acknowledgement views. Browser assertions and final screenshots verify the subject.

QA corrections add no product findings: a duplicate-account fixture is corrected to assert the existing unique identity protection; paging fixtures deliberately activate current members; recovery keys use the existing operation-key contract; filled textareas use exact textbox roles; repeated minutes wording is scoped to the actual section; keyboard tests await real Radix focus transitions. Private before-evidence `.go` files initially caused vet to enumerate mixed packages; byte-preserving `.go.txt` names corrected that setup before the full run. Checks, timeouts, immutable originals, exact amounts and denied-access expectations remain intact.

## Matching recovery and continuation

The genuine schema-20-to-21 migration preserves all **81 prior persistent table definitions, rows and migration provenance**, an approved contact/private replacement, original 43,219-paise receipt and uncertain provider evidence. Four meeting tables bring the persistent inventory to **85**. The historical 19-to-20 test still checks its genuine 81-table boundary before current schema verification.

Separate populated Go recovery preserves approved agenda/private pending successor, supplied minutes, public cancellation, three exact personal acknowledgements, the original receipt and both held keys. It purges old temporary credentials before fresh fixture login. Uncertain provider evidence is covered separately by migration and provider recovery; it is not claimed as co-populated in the new meeting recovery fixture.

Pre-upgrade schema 20, matching 0.21: `var/snapshots/meetings-0.22-pre-upgrade-20261008`; **1,269,760 bytes**, SHA-256 `d5ab7c2987a513e2bea3b0b77ae4fce64355ddfd1b78db9dcd438f5c47fd561b`; verified restore **27.079 ms**.

Post-upgrade schema 21, matching 0.22: `var/snapshots/meetings-0.22-20261008`; **1,327,104 bytes**, SHA-256 `d57efa8b8b0ea62f9c8d1e09d4aa87ffe172e5a8ca084b342045c77966e41c36`; verified restore **37.243 ms**.

Pinned preview `var/preview-releases/0.22` serves http://127.0.0.1:8080 under detached PID **8356 when started**. Revalidate its exact command before process action. Readiness 200, all 18 packaged/17 served hashes, all 85 persistent rows/column shapes/provenance, integrity/FK checks and both held key bindings are verified after startup. Four temporary credential tables are purged on restore. New meeting tables remain empty in the developer preview; browser QA uses disposable synthetic databases.

Continue with [private reminders/delivery exceptions](reminders-delivery-exceptions-workflow.md), then budget/move checklists, measured performance/PWA/migration and independent production/pilot preparation. Real identities, policies, custody, provider credentials, host/network proof and human pilot acceptance remain separate inputs. This checkpoint does not complete the whole plan.

## Personal publication

Application `e8d1d53aeba576bd010c4daa5c5187a57c28c299` is published to the personal public `flux-i/society-os` repository. At `2026-10-08T07:14:12.872228+00:00`, account, author, credential-free origin, clean tree and GitHub API main all verify. Immutable private `reports/local/checkpoints/meetings-0.22-accepted` retains **397 source files/765 verified hashes**; every archived source blob matches that application commit. The documentation-only child records this evidence without changing the tested application/runtime or rewriting the archive. Publication proofs remain private and the whole goal stays active.
