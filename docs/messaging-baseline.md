# Targeted messaging — accepted local checkpoint

**Verified:** 6 October 2026. **Release:** 0.16.0-dev, schema 15. The [before-code contract](messaging-workflow.md) follows [registered contacts and consent](contacts-baseline.md). This checkpoint implements a persistent local simulation for WhatsApp/email delivery. It sends no real provider messages and initiates no money transfer.

## Outcome and authority

An eligible community operator prepares sharing of an approved notice; a current Treasury operator prepares a private original receipt. They choose channel and all/wing/owner/tenant/selected-home/selected-person targeting, inspect the exact envelope and independently resolved audience, then submit a frozen proposal. A different currently eligible reviewer approves it. Preparation, review and dispatch require recent privileged verification; successful operation replay also rechecks current authority.

Targeting intersects current relationships, the source's published audience or financial entitlement, independently verified channel/purpose consent and a current active linked portal identity. The preview distinguishes target people, source-entitled people, verified permission, eligible people, deduplicated destinations and omissions. A multi-home person counts once. Shared destinations retain individual eligibility and attempt proof: one person's opt-out never becomes another person's reported delivery.

Notice envelopes use deliberately published titles and authenticated links. Receipt wording is generic and omits amount, payer, home and evidence. Registry/contact administration adds no Treasury authority or private receipt access. Residents see only their own approved delivery metadata; unapproved proposals, other recipients, private history and contact addresses remain hidden. Current source access governs retained links after a home entitlement ends.

Approval rechecks the frozen source/audience fingerprint. Changed previews require explicit refresh and separate approval. Claim and handoff recheck proposer, reviewer, dispatcher, account status, factors, appointments, source version, home entitlement, contact version and consent. A changed address suppresses the old unsent destination instead of replacing it silently.

## Delivery and recovery behavior

Prepared recipients are eligibility metadata, not queued delivery outcomes. Only an approved batch has actionable queued groups. Provider-accepted, delivered, explicitly reported read, failed, skipped, opted-out, cancelled and unknown remain distinct. A read event without a delivered event does not invent a delivery timestamp. Duplicate, changed, old-attempt and out-of-order callbacks preserve their proof without regressing current outcomes.

Actor-bound operations claim at most 25 groups. Definitive failures permit at most three attempts. Unknown handoffs require reconciliation before another attempt. Persistent simulation proof allows recovery of the same accepted handoff without another provider message; a restart with no handoff proof can establish a definite unsent failure and permit a bounded retry. Current authority remains required before recovery or successful replay. An absent adapter returns unavailable rather than fabricated success.

The simulation uses a separately held private 32-byte signing key, an immutable database fingerprint and bounded signed callback verification. Startup refuses a missing/wrong held key for bound history. The key is excluded from snapshots and Git. Callback verification includes strict JSON, an 8 KiB limit, signature equality, a five-minute timestamp window and event identity/content checks. This is local development behavior, not real-provider acceptance.

## Executed gates

| Gate | Result and practical limit |
|---|---|
| Formatting, vet and full Go race suite | Passed with 160 Go test declarations. The database package took 890.221 seconds using real Argon2 fixtures; the explicit default package timeout is now 20 minutes. |
| TypeScript and production build | Passed. JavaScript: 879.22 kB / 236.34 kB gzip; CSS: 138.21 kB / 26.33 kB gzip. The chunk-size warning remains for measured performance work. |
| Full ordinary browser regression | 157 cases passed across 23 isolated synthetic suites; final run ended 16:27:15 UTC with unchanged source/assets. |
| Actual native Chrome WebMCP | 39 cases passed across four isolated synthetic suites, including three messaging cases. |
| Additional affected visual checks | Eight messaging journeys and one private supplemental populated-state case passed. The supplemental case is not added to the 157-case catalog. |
| Actual screenshot inspection | 48 final captures viewed through 35 sheets/139 parts; twelve critical originals also viewed at original size. |
| Matching preview and backup/restore | Passed exact persistent-row, provenance, key-custody, integrity, foreign-key and served-asset checks. |

Required stages ran and passed separately; no literal green aggregate `make eval` invocation is claimed. Earlier failures remain retained. The first full race run exceeded Go's default ten-minute package timeout while real password fixtures were still running; it did not report an assertion or race failure. The final full race passed with the explicit timeout. During that run only frontend count alignment/wording changed; every Go source and migration remained unchanged. TypeScript, final build and affected/native checks cover that final UI. An older contact test then matched both a success status and a loading status; its success-specific locator was corrected, affected cases and the full ordinary rerun passed. That QA-only change leaves application and native source/build unchanged.

After preview promotion, the Make startup recipe was made explicit about `MESSAGE_KEY_FILE`, matching the already executed retained-preview command. Default and alternative paths were checked with `make -n run`; application source, build and check recipes stayed unchanged. `messaging-startup-command-verification.json` records this Makefile-only continuity check without claiming another full test invocation.

Private evidence includes `reports/local/messaging-gates-verification.json`, final stage logs/drivers, failing and affected logs, `messages-review-final/review.json`, recovery and preview verification. The exact source/build/evidence archive is `reports/local/checkpoints/messaging-0.16-accepted`. Captures, databases, keys, binaries and logs remain outside Git.

## Executed interaction coverage

Eight new ordinary cases exercise opened source/channel/audience/state menus, keyboard/hover/focus and actual decisions at desktop, tablet, 375px, 320px and reduced-height phones. They check independent counts (153 current people, 118 owners, 35 tenants, 52 Wing A people; two jointly entitled people sharing one destination), private receipt wording, separate approval, zero monetary changes, stale-consent refresh and retained reasons, approval/cancellation/decline/withdrawal, bounded choice/register/recipient/history pages, unavailable/loading/local retry, unconfigured dispatch, known failure limits and unknown handoff reconciliation.

Lost-response cases retain one operation, lock edits and dismissal and keep the exact retry reachable through internal scrolling. Overview attention is current-scope, includes only approved actual delivery outcomes and preserves unavailable values as unknown. Supplemental populated captures verify pending proposals alongside queued/unknown/accepted batches and own history; pending cards do not claim sends. Financial and shared-recipient expectations are independently supplied rather than calculated from the implementation.

`society_find_messages` and `society_read_message` execute Chrome's real `document.modelContext` API and return bounded permitted metadata, omitting source identifiers, content, destinations, people and private actor/history data. Checks cover exact 12+1 pagination, denied/pending/other-person access, unsupported inputs, cancellation, preserved unsaved human forms, no POST requests and discarded held results after scope/logout changes. Tool inventories are 47 for fictional staff, 39 for the financially entitled owner and 26 for the tenant. Discovery grants no additional authority.

The 48 captures were visually inspected after the final fixes, including open/hovered menus, short-phone count alignment, selected people, long histories, private receipt preview, locked unknown write, reconciliation and own delivery history. This is finite executed coverage, not a claim that every possible input, device, browser or interaction was tested.

## Findings closed

| Group | Observed problem and verified repair |
|---|---|
| MSG-01 | Attention scope had an ambiguous joined state and counted prepared, unapproved destinations as queued. Batch/detail/recipient projections also exposed prepared groups as outcomes. Scope is qualified; only approved/cancelled historical delivery states contribute actual outcomes. Pending/declined/withdrawn snapshots retain eligibility and decisions without invented sends. Independent failing-then-passing domain/browser assertions and final captures verify the repair. |
| MSG-02 | Message filters/input nesting produced duplicate borders, phone register headings clipped, and wrapped count labels shifted numeric baselines by 17.59375px. Shared control layout, stacked phone heading and equal-height count labels now pass border, text-visibility and one-pixel alignment expectations across four widths. Singular labels and original-size review confirm the final presentation. |

Cumulative accepted findings: **70 groups — 68 product/UI and two operational**. Fixture/locator/inventory/capture corrections and the package timeout are not extra product findings.

## Preview, recovery and remaining work

The verified 0.15 preview was stopped by its exact command/PID before migration. Its matching schema-14 pre-upgrade bundle, `var/snapshots/messaging-0.16-pre-upgrade-20261006`, is **974,848 bytes**, SHA-256 `ea3cb587cbb6d686d55806936888c51b25eea1b6643b79c8d7cfaeb8f40f4503`; matching restore took **19.729 ms**. The matching schema-15 post-upgrade bundle, `var/snapshots/messaging-0.16-20261006`, is **1,085,440 bytes**, SHA-256 `e60d565ae51679344036983b432672916c9dae1ec7e069b73e40bc24c35cbb9c`; restore took **27.217 ms**.

All 58 prior persistent tables preserve exact rows on migration; provenance 1–14 is unchanged and migration 15 adds nine empty tables. First startup adds only the separately held messaging-key fingerprint to existing metadata. All 67 post-upgrade persistent tables restore exactly, four temporary credential tables are purged, integrity/foreign keys pass and the existing separate MFA key is unchanged. The continuing preview has no synthetic message batches. A separate populated backup case recovers consent, an unresolved durable provider handoff, recipient attempt proof and original ₹432.19 money/receipt identity; old sessions are invalid and reconciliation preserves the original attempt without invented delivered/read outcomes.

The continuing retained `var/preview-releases/0.16` pair is ready at `http://127.0.0.1:8080`. All 18 packaged files equal the tested build; all 17 served files equal that pair. All 67 current persistent tables remain unchanged after startup. Both keys are held under `var/keys`, outside bundles. Restore times are development-Mac samples.

Windows amd64 cross-compilation passes: **21,293,056 bytes**, SHA-256 `958f5dd6468d28b92948dcdec787f1a3b0ae5faecfc43350888dd3f284b7bedd`. Actual Windows execution/performance remains untested. Next is [prepared financial statements, safe spreadsheet originals, deliberate publication/sharing and scoped exports](statements-workflow.md), followed by the remaining community and performance/PWA/pilot work. Live provider onboarding/templates/credentials and real identity, finance-policy, infrastructure/key-custody and Windows acceptance remain separate activation inputs.

The accepted application is published in the personal public [flux-i/society-os repository](https://github.com/flux-i/society-os/commit/46ee31e7ad159f4cff8c7ace5384b9cbd9dffac9). At application publication, personal account/owner/author and credential-free origin were verified, GitHub API main matched local HEAD, the worktree was clean and all 288 archived source files matched the commit. The private archive contains 721 hashed files. Evidence: `reports/local/messaging-publication.json`. Later documentation commits may advance main.
