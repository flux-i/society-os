# Society OS

A thoughtful community portal for a 118-flat society. The working **local preview** includes a changing role-scoped overview, occupancy and owner/tenant counts on Homes & people, scoped homes, registry and account administration, invitations/password recovery, authenticator protection, manual entries/receipt PDFs, separate-reviewer requests, audience-scoped approved notices, personal service requests with private handler notes, validated private document versions, separately approved maintenance periods, explicit receipt/opening-credit allocations, private upkeep work/assets/vendors with intentional resident updates, approved fixed/voluntary funds, own externally paid reports, treasury verification/original receipt reconciliation, separately approved exemptions, independently published supplied rules, private picture-based incident review, frozen household response notices, separately authorised fines/appeals/linked corrections, registered contacts with independent verification and immediate communication opt-out, externally prepared PDF/CSV/XLSX financial originals with separate internal review and deliberate resident publication, separately reviewed meeting agendas/minutes/cancellations and exact personal acknowledgements, and verified SQLite snapshot/restore tools.

The [operations roadmap](docs/society-operations-roadmap.md) preserves maintenance, fund campaigns/external-payment verification, targeted WhatsApp/email, approved rule/fine workflows and financial statement publication. [Maintenance cycles and receipt allocation](docs/maintenance-baseline.md) are verified in release 0.10/schema 9. [Upkeep tasks, assets and vendor visits](docs/upkeep-baseline.md) are verified in release 0.11/schema 10. [Fund campaigns and externally paid reports](docs/collections-baseline.md) are verified in release 0.12/schema 11. [Private rules, incident review and household responses](docs/incidents-baseline.md) are verified in release 0.13/schema 12. [Separate fine issuance, responses, payment verification and corrections](docs/fines-baseline.md) are verified in release 0.14/schema 13. [Registered contacts, independent consent verification and immediate opt-out](docs/contacts-baseline.md) are verified in release 0.15/schema 14. [Targeted notices/private receipt sharing, separate approval and persistent synthetic delivery](docs/messaging-baseline.md) are verified in release 0.16/schema 15. [Prepared financial originals, separate review and deliberate publication](docs/statements-baseline.md) are verified in release 0.17/schema 16. [Exact published-statement messaging](docs/statement-messaging-baseline.md) is verified in release 0.18/schema 17. [Scoped operational finance exports](docs/finance-exports-baseline.md) are verified in release 0.19/schema 18. [Official-protocol portal provider preparation](docs/whatsapp-provider-baseline.md) is verified in release 0.20/schema 19 using numeric-loopback HTTP fixtures; [Reviewed service contacts and explicit interruption/restoration](docs/community-services-baseline.md) are verified in release 0.21/schema 20; [Meeting agendas, supplied minutes and exact personal acknowledgements](docs/meetings-baseline.md) are verified in release 0.22/schema 21. Earlier baselines retain their evidence. The portal does not initiate payment or send real provider messages.

## Run locally

Requirements: Go 1.27+, Node 22.12+ and npm. PDF-original validation also requires a system `qpdf`; this workspace was tested with qpdf 12.4.2, Go 1.27.0, Node 25.8.1 and npm 11.21.0. Go dependencies and frontend packages are pinned in `go.mod`/`go.sum` and `web/package-lock.json`. Install qpdf through the host package manager (macOS: `brew install qpdf`) and check `qpdf --version`. If it is absent, uploaded PDFs remain unavailable with a retryable check failure; the portal never approves unchecked bytes. The full validator checks require the actual executable.

```sh
make setup
make run
```

Open **http://127.0.0.1:8080**. `make run` builds the application, creates the isolated fictional registry if needed, and serves the frontend and API from one Go process. Stop with Ctrl+C. Repeated seeding preserves the same fixture without duplicating records.

The database lives at `var/demo/society.db`. An alternative isolated database can be selected with `make run DB=var/another-demo/society.db`. The preview accepts synthetic-marked databases, requires `--demo` and binds only to a loopback IP. Production deployment remains disabled. Existing previews migrate to schema 20 while preserving registry, account, finance and audit history. The earlier identity upgrade signs out old sessions.

Choose an account on the sign-in screen, then select **Sign in**. The fictional credentials are prefilled; every demo account uses the public preview password `Community-preview-2026!`.

| Account | Local preview access |
|---|---|
| `admin@demo.society` | Registry/history, invitations/recovery and an explicit additional treasury grant for entries/receipts; MFA required |
| `committee@demo.society` | Community registry/financial views, separate-reviewer requests and service handling; MFA required; cannot post financial records |
| `owner@demo.society` | Only the owner's two active homes and confirmed financial records, A-101 and A-102 |
| `tenant@demo.society` | Only the tenant's active home, A-103; no former-tenant history or finance entitlement |

**For the registry officer:** after Sign in, select **Use a preview code**, then **Verify and continue**. On first setup, save and acknowledge the displayed recovery codes. The shortcut is restricted to the four fictional accounts; invited identities use their own authenticator.

Open a home as the registry officer and choose **Manage home** to change occupancy, create/link a person or end a relationship. Every change requires a reason. **History** records the actor, time and before/after values. Concurrent stale saves are rejected and offer reload. People can be linked to multiple homes without duplicate person records; ended relationships are retained.

**Access & invitations** lets the officer select a current registry person, attest identity/email verification and issue a personal invitation for manual handover. Resident access follows current homes; committee/administrator invitations have a bounded role term. Invitations expire in one hour; recovery links in 30 minutes. Links work once; reissuing invalidates previous links. The portal sends no email/messages. Use fictional identities and example addresses here.

**Account security** supports authenticator setup, recovery-code replacement, five-minute identity confirmation and password changes. Reset/change signs out every target-account session and preserves MFA. Administrator access remains locked until factor verification succeeds. Passwords require at least 12 characters.

**Manage access** opens each account's bounded appointment/activity history. A current administrator can grant or end Administrator, Committee, Treasurer or Accountant/auditor appointments for 1–365 days, or explicitly suspend/resume an account. Changes require a reason, authority/identity attestation and fresh password/factor verification. Self-changes and removal of the last recoverable administrator are denied. Suspension ends credentials and appointments; resumption never revives them. Auditors read financial records without posting or registry-wide administration. Current scope checks clear old protected data when roles or home entitlements change, while unchanged checks preserve forms. See [account acceptance](docs/account-administration-baseline.md).

Homes & people distinguishes homes from people: the fresh fixture contains **109 occupied homes, 9 vacant homes, 118 distinct active owners and 35 distinct active tenants**. Occupied homes comprise 74 owner-occupied and 35 rented homes. Joint owners count separately; multi-home owners count once society-wide; former/future memberships do not count as active. Wing counts deduplicate people within each wing, so summing wings need not equal the society total when someone owns homes in different wings.

For frontend hot reload, keep the Go server running and run `make dev` in another terminal. Vite serves a local development URL and proxies API calls to `127.0.0.1:8080`.

## Check and measure

```sh
make check
make bench
make report
```

`make check` verifies formatting, Go vet, race-enabled backend tests and TypeScript. Tests cover registry constraints/counts, transactional mutations, concurrent stale edits, immutable audit, cross-flat and role denial, session/account/membership expiry, logout, Origin/CSRF, login/factor throttling, link expiry/revocation/concurrent consumption, TOTP replay, single-use codes, key loss, recovery and engine/migration provenance. There are 227 Go test declarations, including exact financial amounts, operation retries, immutable corrections, leased receipt jobs, private PDFs, separate approvals, scoped notices/service conversations, document validation/version/quota/recovery, account appointments/suspension, current authority/home-scope changes, maintenance/credit allocation, upkeep, collections/exemptions, private rules/incidents/fines/appeals, verified contacts/consent persistent messaging/restart/signed callbacks/nonempty recovery, and bounded financial-original validation, separate publication, current audiences, replacements, populated recovery and actor-bound exact operational CSV exports with current frozen scope, original/correction links, limits/quotas and formula protection. Full race tests retain real Argon2 password fixtures; the default package timeout is 30 minutes, configurable through `GO_TEST_TIMEOUT`, with serial packages through `GO_TEST_PACKAGE_PARALLEL=1`.

`make bench` runs the representative registry query three times. `make report` creates an independent fresh synthetic database, measures 100 warm local HTTP reads after ten warmups, snapshots the live database and restores it into a new environment. It verifies SHA-256 independently in Python and records measured times, engine settings and counts in `reports/local/account-security-baseline.json`. These are local measurements, not production performance commitments.

`make eval` is the checkpoint gate: backend formatting/vet/race tests, TypeScript/build, ordinary rendered browser journeys and actual native WebMCP discovery/execution in installed Chrome. Each ordinary and native suite gets an isolated synthetic server/database; native tests require Chrome with the WebMCP feature enabled. `make webmcp-check` runs that integration separately. Release 0.22 passes 197 ordinary cases across 29 isolated suites and 56 actual native WebMCP cases across nine suites. Required backend/TypeScript/build/ordinary/native stages passed separately after observed product repairs and QA corrections, followed by actual visual review and matching recovery. No literal green aggregate invocation is claimed. The browser API is optional for using the portal, and no tool approves, uploads, publishes, dispatches or posts records. See [meeting acceptance](docs/meetings-baseline.md) for source continuity, observed product repairs, QA corrections and limits, and the ongoing [development process](docs/development-workflow.md).

Browser checks use a fresh fictional database on an ephemeral loopback port and leave your preview untouched:

```sh
cd web
npx playwright install chromium
npm run test:browser
```

Build first with `make build`, or use `make browser-check`. Follow this machine's `AGENTS.md` instruction to launch Playwright outside any command sandbox. The current workspace has unrestricted execution. The 197 cases cover scoped overview queues/balances, deep links, source failures/retries, all 118 home cards, opened menus, manual entries/receipts, invitations/recovery/MFA, separate reviews, service requests, validated documents, current account/home scope, maintenance/allocations, upkeep, funds/external payment reconciliation, private rules/incidents/fines/appeals, contact verification/opt-out/history and targeted notice/receipt sharing with separate approval, unknown handoff recovery and truthful delivery outcomes, financial-original upload/review/publication/replacement/revocation with PDF/CSV/XLSX checks and unchanged downloads, exact published-statement finance-consent delivery with separate Treasury review/current-source suppression, and all four scoped export preview/confirmation/download/history workflows with exact amounts, stale/access/response-loss checks and bounded safe CSV. Meeting cases also exercise separate publication, exact personal acknowledgement, supplied minutes/cancellation/withdrawal, current/historical response status, stale/uncertain retries, private source suppression and real keyboard menus. Specific cases exercise desktop, tablet, narrow phone and reduced-height layouts. The [UI review](docs/ui-review-baseline.md) records the original inventory and global Claude Code/Codex browser tooling; subsequent baselines record added controls and finite coverage limits.

**Overview** shows changing decisions, service work, document attention/deadlines and approved notices for the current account. Financial cards require their own entitlement and distinguish positive home balances from credits, confirmed money received in the last 30 India calendar days, draft work and receipt-generation attention. Each section has its own retry; unavailable values remain unknown. Refresh rereads current sources, and actionable rows open their exact permitted record. Maintenance adds current pending reviews, published outstanding and explicit past-due amounts with its own source retry. Upkeep adds due/unassigned work, separate checks, visits and private AMC/inspection deadlines. Collections adds confirmed purpose money, fixed outstanding/past-due amounts, verification/exemption decisions and own requested clarification, with exact links and a separate source retry. Rules & conduct adds independent case/rule decisions, own requested clarification and current-home response notices; its ninth source stays unknown when unavailable. Fines adds separate decisions, own responses/clarification, explicit pauses and current source changes; its tenth source also retries independently without inventing zero values.

**Community services** preserves the Noticeboard and adds supplied help contacts and planned/current service interruptions, exact frozen-home publication, separate review, pending predecessor retention and explicitly approved restoration. Current affected homes and relevant private review decisions appear on Overview with an independent retry; an elapsed estimate stays unresolved. Native tools expose bounded timing/status metadata without phone numbers or private content. See [community service acceptance](docs/community-services-baseline.md).

**Meetings & minutes** retains supplied agendas, explicit times and frozen homes, different-person review, separately published minutes/cancellation and exact personal acknowledgements. Current expected responses remain distinct from historical originals. Native tools return bounded published status/own-response metadata and recheck held source results. See [meeting acceptance](docs/meetings-baseline.md).

**Entries & receipts** lets the officer save and review manual drafts, confirm supplied charges/opening balances or money already received, download private receipt PDFs, discard mistaken drafts and reverse confirmed entries with a reason. Balances derive from confirmed, unreversed entries in the selected home scope. The owner sees only financially permitted records. See [manual-record acceptance](docs/manual-records-baseline.md) and [our development process and issue inventory](docs/development-workflow.md).

**Finance exports** opens from Entries, Receipts, Maintenance and Collections. Choose source dates and current Society, all own financially entitled homes or one permitted home; preview exact totals/source/correction links, confirm an immutable snapshot and download its checksum-verified CSV. Current Treasury/fresh verification gates broad exports; ordinary registry/community/audit access does not grant them. Losing any frozen home denies an old personal snapshot; gaining a home does not expand it. Original receipt amounts and linked corrections stay identifiable. Saved copies have bounded history and explicit retry; a failed/corrupt download does not claim success. See [export acceptance](docs/finance-exports-baseline.md).

**Maintenance** previews supplied per-home amounts and an explicit period/due date before frozen submission. A different currently eligible Treasury reviewer publishes matching charges atomically; publication creates no receipt. Home statements distinguish charges, receipts/opening credits, available credit and retained allocation history. Explicit allocation/correction preserves original receipts and exact balances; lost-response retries do not duplicate charges or links. Residents see only published currently entitled homes. See [maintenance acceptance](docs/maintenance-baseline.md).

**Collections** previews fixed supplied contributions or voluntary participation before separate Treasury publication. Residents report money already paid with private validated evidence. A different reviewer verifies an external source/payment identity and creates or links one original receipt; duplicate reports add no cash. Zero/partial/excess allocations, voluntary purposes, linked corrections and separately approved exemptions preserve exact available credit and original receipts. Closed funds retain review of existing claims. See [collections acceptance](docs/collections-baseline.md).

**Rules & conduct** supports independently published supplied rules, private reports against a tagged home, validated pictures, retained corrections/duplicate links and a separate handler decision. Current household members receive only deliberately issued frozen notices and can submit their own replies. Original evidence and private notes remain scoped; reporting or substantiation creates no debt or receipt. See [incident acceptance](docs/incidents-baseline.md). **Fines & responses** separately prepares a supplied amount and frozen household notice, obtains an independent decision, resolves current responses and issues one charge. Household members can respond/appeal without financial access. Treasury verifies already-paid money against the shared payment identity; independent partial/full corrections release credit while retaining original cash and receipts. See [fine acceptance](docs/fines-baseline.md).

**Contacts** records supplied destinations and four default-off channel/purpose choices, previews registration, requires independent offline identity/permission review and supports immediate selective opt-out. Changed addresses/added permission remain pending until fresh review. Same-home people cannot see each other’s private profile; former members retain their own history and stop controls. The portal sends no code or message. See [contact acceptance](docs/contacts-baseline.md).

**Messages** previews exact source/audience/consent intersections, eligible people, unique destinations and omissions before separate approval. Notice sharing and private receipt sharing use distinct current permissions. Only approved batches have queued outcomes; accepted, delivered, explicit read, failed and unknown stay distinct. Unknown handoffs reconcile retained proof instead of blindly resending; definitive failures have bounded retries. Published financial statements use their own Finance source and a different Treasury delivery reviewer, with current publication/membership/verified-finance-consent checks and preserved unknown proof. The default preview uses persistent local simulation. An optional explicitly configured numeric-loopback provider fixture exercises official approved templates, portal Send, durable uncertain attempts and signed callbacks; the CLI cannot activate external delivery. Current source links open the permitted original notice, receipt or statement, disappear during held decisions and are removed when current entitlement ends. See [provider preparation acceptance](docs/whatsapp-provider-baseline.md). See [statement-messaging acceptance](docs/statement-messaging-baseline.md) and [earlier notice/receipt acceptance](docs/messaging-baseline.md).

**Financial statements** keeps externally prepared income statements, balance sheets, budgets and audit files private through bounded PDF/CSV/XLSX checks and separate finance approval. A separate publication decision freezes a chosen version and audience; residents see only their current deliberate publication. Replacements retain the old publication until separately released, and revocation blocks new downloads while retaining originals/history. Shared files add no ledger entries or receipts. See [statement acceptance](docs/statements-baseline.md).

**Upkeep** connects private assets/vendors, service work, assignments, dates and retained activity. A different current operator checks completion; repeated occurrences are explicitly previewed. Residents see frozen intentional updates for their current audience. Private activity never silently changes that snapshot. See [upkeep acceptance](docs/upkeep-baseline.md).

**Requests & community** supports proposals, revisions and approval by a different reviewer. Approved notices reach only their current audience. Expense/registry proposals do not automatically post money or apply registry edits. **Help & repairs** lets residents report against their own active home and follow their own cases. Authorized handlers manage assignment/progress and staff-only notes; authors can confirm closure or reopen after resolution. Ended membership retains read-only personal case history.

**Documents** accepts fictional plain PDFs, PNGs and JPEGs with reserved size/checksum/quota, actual content checks, private originals and separate approval before sharing. A replacement preserves the previous approved version until approval; decline/withdraw/archive retain history. Current home/person/community/accounting scope controls metadata, counts, version history and downloads. Financial uploads require their own entitlement and never post money or issue a receipt. Limits: 20 MiB images, 4 MiB PDFs, 100 MiB per uploader and 1 GiB per society including reservations and retained bytes. Actual image decoding is capped at 8 megapixels. PDF checks use a bounded qpdf process; they do not establish antivirus coverage or an operating-system memory sandbox. The **synthetic local storage adapter** stores originals in SQLite so consistent snapshots include them. Planned production S3, containment/antivirus, retention policy, general document spreadsheets/OCR and attachment links remain pending. The separate Financial statements workflow accepts deliberately bounded PDF/CSV/XLSX originals.

To keep an already verified preview running while developing the next checkpoint, retain its binary and assets together:

```sh
python3 scripts/pin-preview.py --name 0.22
./var/preview-releases/0.22/society-server serve --demo --db var/demo/society.db --mfa-key-file var/keys/mfa.key --message-key-file var/keys/messages.key --addr 127.0.0.1:8080 --web-dir var/preview-releases/0.22/web
```

Stop the earlier server before starting the retained one. The pin command refuses an existing name. Verify the build first, then pin it; the command packages files and does not run the checks. It does not copy databases or keys. Use a new name for the next verified release.

## Local recovery

```sh
make snapshot SNAPSHOT=var/snapshots/checkpoint-01
make restore-check SNAPSHOT=var/snapshots/checkpoint-01 RESTORED_DB=var/restored/checkpoint-01/society.db
./build/society-server inspect --db var/restored/checkpoint-01/society.db
```

Snapshots use `VACUUM INTO` to include committed WAL changes consistently, then remove sessions, access links and recovery codes from the private copy before hashing. Session deletion also removes pending authenticator setup. Identity and audit remain intact; the live user's session is unaffected, and restored users must sign in again. Each new private bundle contains `society.db` and a manifest with SHA-256, byte size, schema/application/engine versions, fixture identity, timestamp and counts. Verification includes SQLite integrity, foreign keys and migration checksums. Restoration refuses existing databases and WAL/SHM sidecars. Use the matching release for a snapshot's schema; older checkpoints require their matching release before an explicit upgrade. Confirmed authenticator factors survive encrypted; preserve the matching MFA key separately. Saved recovery codes intentionally do not survive restoration.

Derived PDFs live in a private `documents/` directory beside the database. On startup, missing/corrupt completed PDFs are regenerated from the preserved receipt snapshots and numbers.

Original library files belong to the local SQLite snapshot along with their checksum, scope, immutable versions and review events. Restore verifies original bytes in a separate recovery case and invalidates old sessions. Do not confuse regenerable receipt PDFs with irreplaceable uploaded originals.

These snapshots are local and unencrypted. Off-site encryption, S3, off-site key custody, external alerts and production power/reboot recovery are pending infrastructure work. The live database and generated snapshots/builds/reports are ignored by Git; do not put real resident data or usable secrets in this preview.

The MFA key lives at `var/keys/mfa.key`, outside snapshot bundles. Use `make run MFA_KEY_FILE=var/other-keys/mfa.key` for another private key path. A key is created only when no encrypted factors exist; startup refuses a missing/wrong key with existing factors. Protect and retain the key separately. Key files are private and ignored by Git; encrypted off-site custody is pending.

Messaging also requires its matching separately held signing key. The raw CLI defaults to `keys/messages.key` beside the selected database; `make run` and the retained preview explicitly use `var/keys/messages.key`. Override the Make variable with `MESSAGE_KEY_FILE=var/other-keys/messages.key` for another held key. Once a database has a messaging-key fingerprint, startup refuses a missing/wrong key and never silently rebinds history. Preserve that key outside snapshot bundles and supply it when starting a restored database. Restarted incomplete claims become unknown and require reconciliation. See [messaging recovery](docs/messaging-baseline.md).

Lost-factor recovery has no web route. In this synthetic environment a maintainer can record two verified custodian attestations:

```sh
./build/society-server recover-mfa --demo --db var/demo/society.db --user officer@example.test --custodian-one 'First custodian' --custodian-two 'Second custodian' --reason 'Verified identity and lost authenticator'
```

This records supplied custodians; it does not verify their real-world authority. It revokes sessions/links/codes and requires authenticator reenrollment. Actual society custody procedures still need acceptance.

## Current API

Community services adds protected `GET /api/community`, `/api/community/options` and `/api/community/{id}`, plus deliberate `POST /api/community`, `/api/community/{id}/proposals` and `/api/community/{id}/decisions`. The private desk requires current community authority; published reads require current audience access. Native `society_find_community_updates` and `society_read_community_update` expose status/timing only.

Scoped exports add protected `GET /api/finance-exports/choices`, `POST /api/finance-exports/preview`, snapshot creation/listing at `/api/finance-exports`, and actor-bound metadata/download at `/api/finance-exports/{id}` / `{id}/download`. Creation requires current scope, matching reviewed content and a stable actor-bound operation. Exact CSV bytes/checksum, original authority and frozen home scope are retained; current access is rechecked on replay/read/download. Native tools expose bounded export status metadata and cannot create or download snapshots.

Financial originals add protected `/api/financial-statements` list/reservation/detail, unchanged content upload, validation/internal-review actions, scoped targets, exact publication preview/proposal/decision and attachment download. `/api/overview/statements` supplies current actionable counts. These routes are distinct from existing `/api/statements/{home}` household balances. Current finance authority/factors, actor-bound operations, immutable versions, separate reviewers and deliberate resident publication are enforced. Native tools return bounded permitted status metadata and perform no downloads or decisions.

Contacts adds protected `GET /api/contacts`, bounded private `GET /api/contacts/{person}` and versioned `POST /api/contacts/{person}/register` / `/actions`. Residents read only their own profile; current community/registry authority gates broader reads/decisions. Own opt-out remains immediate. Native tools return bounded metadata without addresses or history.

Rules & conduct adds protected `/api/rules` list/create/detail/actions, `/api/incidents` list/create/detail/revision/actions, bounded `/api/incidents/options` and original-case choices, validated picture upload/scoped retrieval, household `/api/incident-notices` reads/responses and `/api/overview/incidents`. Current reporter, operational handler and household audiences are distinct. Writes use fresh authority, version checks, actor-bound operations and Origin/CSRF. Native tools expose bounded metadata; reports and substantiation post no money.

Collections adds protected `/api/collections` list/create/detail/actions, `/api/payment-reports` list/create/detail/revision/actions/private options/evidence, `/api/fund-waivers` proposals/detail/actions, `/api/fund-contributions` history/explicit assignment/reversal and `/api/overview/collections`. Treasury writes use version, actor-bound operation, Origin/CSRF and current factor authority. Residents read only current financial participation and own reports. Native reads expose bounded metadata and perform no writes.

Upkeep adds protected `/api/upkeep`, `/api/upkeep/options`, `/api/upkeep/tasks`, task detail/actions and operator-only `/api/upkeep/register/{kind}` detail/history. Current operational authority remains separate from finance; resident views use publication snapshots.

Maintenance adds protected `/api/maintenance` list/create, `/api/maintenance/{id}` bounded detail, `/api/maintenance/{id}/decision`, `/api/statements/{home}` and explicit `/api/allocations` / `{id}/reverse`. Lists/totals share current financial scope; writes require current Treasury/factor authority and Origin/CSRF. Native tools expose bounded metadata only.

| Endpoint | Purpose |
|---|---|
| `GET /health` | Minimal process liveness |
| `GET /ready` | Database/schema/MFA-key readiness; 503 when unavailable |
| `POST /api/auth/login` | Password sign-in; same-origin JSON request |
| `GET /api/auth/me` | Current identity/permissions and CSRF token; does not extend idle expiry |
| `POST /api/auth/logout` | Revoke current session |
| `POST /api/auth/mfa/setup`, `/api/auth/mfa/confirm`, `/api/auth/mfa/verify` | Authenticator enrollment/verification |
| `POST /api/auth/mfa/demo-code` | Explicit fictional-account helper |
| `POST /api/auth/reauthenticate` | Fresh password/factor confirmation |
| `POST /api/auth/recovery-codes` | Replace single-use recovery codes |
| `POST /api/auth/change-password` | Change password and revoke all sessions |
| `POST /api/auth/link`, `/api/auth/link/complete` | Inspect/redeem an invitation or reset link |
| `GET /api/admin/accounts` | Administrator account directory |
| `GET /api/admin/accounts/{id}` | Current administrator's bounded appointments/activity and expected access version |
| `POST /api/admin/accounts/{id}/roles` | Fresh, version-checked bounded appointment |
| `POST /api/admin/accounts/{id}/roles/{grant}/revoke` | End the target appointment while preserving history |
| `POST /api/admin/accounts/{id}/status` | Explicit suspension/resumption; never resurrect credentials or appointments |
| `POST /api/admin/invitations` | Invite a currently related registry person |
| `POST /api/admin/accounts/{id}/link` | Reissue invitation or assisted password recovery |
| `GET /api/overview/{section}` | Current scoped finance/reviews/service/notices/documents totals with up to four metadata rows |
| `GET /api/system` | Registry officer's application/schema/engine evidence |
| `GET /api/registry/summary` | Operator's community and wing occupancy/people totals |
| `GET /api/flats` | Authorized search/filter/pagination, with scoped totals |
| `GET /api/flats/{id}` | Authorized home and permitted current/historical relationships |
| `PATCH /api/flats/{id}` | Registry officer's version-checked occupancy change |
| `POST /api/flats/{id}/members` | Create/link a person and date-bounded relationship |
| `POST /api/flats/{id}/members/{membership}/end` | End a relationship while retaining history |
| `GET /api/entries` | Financially scoped totals, home options, search/filter/pagination |
| `POST /api/entries` | Retry-safe manual draft |
| `GET /api/entries/{id}` | Financially scoped record and receipt status |
| `POST /api/entries/{id}/post` | Treasury confirmation; atomic received receipt/job |
| `POST /api/entries/{id}/discard` | Discard a draft with preserved history |
| `POST /api/entries/{id}/reverse` | Linked immutable correction |
| `GET /api/receipts/{id}/download` | Current-scope private PDF download |
| `POST /api/receipts/{id}/retry` | Treasury retry of a failed PDF job |
| `GET`, `POST /api/reviews` | Current-scope proposal directory and retry-safe submission |
| `GET /api/reviews/{id}`, `POST /api/reviews/{id}/actions` | Private request history; revision, separate review and archive |
| `GET /api/notices`, `GET /api/notices/{id}` | Current-audience approved notices |
| `GET`, `POST /api/complaints` | Personal/handler case directory and retry-safe report |
| `GET /api/complaints/{id}` | Scoped case and visible history page |
| `POST /api/complaints/{id}/updates` | Public/private conversation or permitted version-checked decision |
| `GET`, `POST /api/documents` | Scoped library/counts and retry-safe upload reservation |
| `GET /api/documents/{id}` | Permitted metadata and paged immutable versions; private review trail only for uploader/reviewer |
| `POST /api/documents/{id}/content` | Original-author raw upload; reserved size/checksum and current entitlement checked |
| `POST /api/documents/{id}/actions` | Version-checked separate approval, decline, withdrawal, archive or unavailable-check retry |
| `GET /api/documents/{id}/download` | Current-scope validated original attachment |
| `GET /api/documents/subjects` | Document reviewer's bounded active-person lookup |
| `GET /api/people` | Registry officer's bounded person lookup |
| `GET /api/messages/config`, `GET /api/messages/summary` | Explicit local provider configuration and current-scope attention |
| `GET /api/messages/sources`, `GET /api/messages/targets` | Bounded currently eligible source and recipient choices |
| `GET /api/messages`, `GET /api/messages/{id}` | Scoped approved own history or eligible staff review/detail |
| `POST /api/messages/preview`, `POST /api/messages` | Exact consent/source intersection and retry-safe frozen proposal |
| `POST /api/messages/{id}/actions`, `POST /api/messages/{id}/dispatch` | Separate review/current authority and local persistent handoff |
| `POST /api/messages/{id}/deliveries/{delivery}/reconcile` | Current-authority resolution of an unknown handoff |
| `POST /simulation/message-events` | Strict bounded signed synthetic provider callback |
| `GET /providers/whatsapp/webhook` | Configured official-protocol verification challenge |
| `POST /providers/whatsapp/webhook` | Bounded original-body signed provider status batch |
| `GET /api/meetings` | Current published or authorised private meeting page |
| `GET /api/meetings/options` | Current community operator's flat-only reviewed area options |
| `GET /api/meetings/{id}` | Current permitted published record or private version/history |
| `GET /api/meetings/{id}/responses` | Current operator's independently paged current and historical personal responses |
| `POST /api/meetings` | Fresh community operator's exact agenda proposal |
| `POST /api/meetings/{id}/proposals` | Version-checked linked agenda/minutes/cancellation/withdrawal proposal |
| `POST /api/meetings/{id}/decisions` | Different reviewer's exact decision or own pending-proposal cancellation |
| `POST /api/meetings/{id}/acknowledgements` | Current eligible person's explicit exact-publication acknowledgement |
| `GET /api/flats/{id}/activity` | Registry officer's latest 30 home changes |

The frontend is served alongside the binary in `build/web`; production Node is unnecessary under this packaging decision. Sensitive data responses are not cached. Request logs contain method, route template, status and elapsed time, without query terms or record identities. No service-worker data caching is implemented.

Passwords use Argon2id; sessions use hashed random bearer tokens, HttpOnly/SameSite Strict cookies, 30-minute idle and 8-hour absolute expiry. State-changing requests require an exact Origin and session CSRF token. Account status, current role terms and memberships are checked on each request; mutations recheck identity/role inside the same transaction as the version check and audit. Invitations/password recovery, privileged MFA and role/account administration work locally; production contact verification, custodial authority and HTTPS remain pending. See [account-security acceptance](docs/account-security-baseline.md) and [account-administration acceptance](docs/account-administration-baseline.md).

## What follows

Next: [private reminders/delivery exceptions](docs/reminders-delivery-exceptions-workflow.md), budget/move checklists, measured performance/PWA/migration, independent production preparation and a representative pilot. Real finance examples/receipt policy remain acceptance inputs. Manual financial records describe supplied charges and money already received; the portal does not initiate payment.

Automated billing, bank matching, payment gateways and Tally integration are conditional future work. No runtime LLM API is required.

See the [execution backlog](execution-backlog.md), [delivery strategy](housing-society-execution-strategy.md), [full implementation plan](housing-society-digital-platform-plan.md) and [visual direction](docs/design-system.md).
