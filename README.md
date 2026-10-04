# Society OS

A thoughtful community portal for a 118-flat society. The working **local preview** includes a changing role-scoped overview, occupancy and owner/tenant counts on Homes & people, scoped homes, registry administration, invitations/password recovery, authenticator protection, manual entries/receipt PDFs, separate-reviewer requests, audience-scoped approved notices, personal service requests with private handler notes, validated private document versions and verified SQLite snapshot/restore tools.

The [next operations roadmap](docs/society-operations-roadmap.md) preserves the user's maintenance, fund campaigns/external-payment verification, targeted WhatsApp/email, approved rule/fine workflows and financial statement publication. The overview is verified under the [overview baseline](docs/overview-baseline.md); the other workflows are next development slices; the existing portal does not initiate payment or send provider messages.

## Run locally

Requirements: Go 1.27+, Node 22.12+ and npm. PDF-original validation also requires a system `qpdf`; this workspace was tested with qpdf 12.4.2, Go 1.27.0, Node 25.8.1 and npm 11.21.0. Go dependencies and frontend packages are pinned in `go.mod`/`go.sum` and `web/package-lock.json`. Install qpdf through the host package manager (macOS: `brew install qpdf`) and check `qpdf --version`. If it is absent, uploaded PDFs remain unavailable with a retryable check failure; the portal never approves unchecked bytes. The full validator checks require the actual executable.

```sh
make setup
make run
```

Open **http://127.0.0.1:8080**. `make run` builds the application, creates the isolated fictional registry if needed, and serves the frontend and API from one Go process. Stop with Ctrl+C. Repeated seeding preserves the same fixture without duplicating records.

The database lives at `var/demo/society.db`. An alternative isolated database can be selected with `make run DB=var/another-demo/society.db`. The preview accepts synthetic-marked databases, requires `--demo` and binds only to a loopback IP. Production deployment remains disabled. Existing previews migrate to schema 7 without replacing registry records. The earlier identity upgrade signs out old sessions.

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

Homes & people distinguishes homes from people: the fresh fixture contains **109 occupied homes, 9 vacant homes, 118 distinct active owners and 35 distinct active tenants**. Occupied homes comprise 74 owner-occupied and 35 rented homes. Joint owners count separately; multi-home owners count once society-wide; former/future memberships do not count as active. Wing counts deduplicate people within each wing, so summing wings need not equal the society total when someone owns homes in different wings.

For frontend hot reload, keep the Go server running and run `make dev` in another terminal. Vite serves a local development URL and proxies API calls to `127.0.0.1:8080`.

## Check and measure

```sh
make check
make bench
make report
```

`make check` verifies formatting, Go vet, race-enabled backend tests and TypeScript. Tests cover registry constraints/counts, transactional mutations, concurrent stale edits, immutable audit, cross-flat and role denial, session/account/membership expiry, logout, Origin/CSRF, login/factor throttling, link expiry/revocation/concurrent consumption, TOTP replay, single-use codes, key loss, recovery and engine/migration provenance. There are 72 backend test declarations, including exact financial amounts, concurrent operation retries, immutable corrections, leased receipt jobs, private PDF access, separate approvals, scoped notices, personal/private service conversations and document validation/version/quota/snapshot cases.

`make bench` runs the representative registry query three times. `make report` creates an independent fresh synthetic database, measures 100 warm local HTTP reads after ten warmups, snapshots the live database and restores it into a new environment. It verifies SHA-256 independently in Python and records measured times, engine settings and counts in `reports/local/account-security-baseline.json`. These are local measurements, not production performance commitments.

`make eval` is the checkpoint gate: backend formatting/vet/race tests, TypeScript/build, ordinary rendered browser journeys and actual native WebMCP discovery/execution in installed Chrome. Each ordinary suite gets an isolated synthetic server/database; native tests require Chrome with the WebMCP feature enabled. `make webmcp-check` runs that integration separately. The overview gate passes 69 ordinary browser cases and 12 native WebMCP cases. The browser API is optional for using the portal, and no tool approves, uploads, publishes or posts records. See [approval and notice evidence](docs/approvals-notices-baseline.md), [service-request evidence](docs/complaints-baseline.md), [document acceptance](docs/documents-baseline.md), [overview acceptance](docs/overview-baseline.md) and the ongoing [development process](docs/development-workflow.md).

Browser checks use a fresh fictional database on an ephemeral loopback port and leave your preview untouched:

```sh
cd web
npx playwright install chromium
npm run test:browser
```

Build first with `make build`, or use `make browser-check`. Follow this machine's `AGENTS.md` instruction to launch Playwright outside any command sandbox. The current workspace has unrestricted execution. The 69 browser cases cover scoped overview queues and balances, deep links, independent source failures/retries, the registry, all 118 home cards, opened dropdowns, manual entry/receipt/discard/reversal/retry workflows, invitations, password reset/session revocation, authenticators, separate review/publication, service requests and original-document upload/download/version/retry workflows across desktop, tablet, narrow phone and reduced-height layouts. The [UI review](docs/ui-review-baseline.md) records the original inventory and global Claude Code/Codex browser tooling; subsequent workflow baselines record the added controls.

**Overview** shows changing decisions, service work, document attention/deadlines and approved notices for the current account. Financial cards require their own entitlement and distinguish positive home balances from credits, confirmed money received in the last 30 India calendar days, draft work and receipt-generation attention. Each section has its own retry; unavailable values remain unknown. Refresh rereads current sources, and actionable rows open their exact permitted record.

**Entries & receipts** lets the officer save and review manual drafts, confirm supplied charges/opening balances or money already received, download private receipt PDFs, discard mistaken drafts and reverse confirmed entries with a reason. Balances derive from confirmed, unreversed entries in the selected home scope. The owner sees only financially permitted records. See [manual-record acceptance](docs/manual-records-baseline.md) and [our development process and issue inventory](docs/development-workflow.md).

**Requests & community** supports proposals, revisions and approval by a different reviewer. Approved notices reach only their current audience. Expense/registry proposals do not automatically post money or apply registry edits. **Help & repairs** lets residents report against their own active home and follow their own cases. Authorized handlers manage assignment/progress and staff-only notes; authors can confirm closure or reopen after resolution. Ended membership retains read-only personal case history.

**Documents** accepts fictional plain PDFs, PNGs and JPEGs with reserved size/checksum/quota, actual content checks, private originals and separate approval before sharing. A replacement preserves the previous approved version until approval; decline/withdraw/archive retain history. Current home/person/community/accounting scope controls metadata, counts, version history and downloads. Financial uploads require their own entitlement and never post money or issue a receipt. Limits: 20 MiB images, 4 MiB PDFs, 100 MiB per uploader and 1 GiB per society including reservations and retained bytes. Actual image decoding is capped at 8 megapixels. PDF checks use a bounded qpdf process; they do not establish antivirus coverage or an operating-system memory sandbox. The **synthetic local storage adapter** stores originals in SQLite so consistent snapshots include them. Planned production S3, containment/antivirus, retention policy, spreadsheets/OCR and attachment links remain pending.

To keep an already verified preview running while developing the next checkpoint, retain its binary and assets together:

```sh
python3 scripts/pin-preview.py --name 0.8
./var/preview-releases/0.8/society-server serve --demo --db var/demo/society.db --mfa-key-file var/keys/mfa.key --addr 127.0.0.1:8080 --web-dir var/preview-releases/0.8/web
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

Lost-factor recovery has no web route. In this synthetic environment a maintainer can record two verified custodian attestations:

```sh
./build/society-server recover-mfa --demo --db var/demo/society.db --user officer@example.test --custodian-one 'First custodian' --custodian-two 'Second custodian' --reason 'Verified identity and lost authenticator'
```

This records supplied custodians; it does not verify their real-world authority. It revokes sessions/links/codes and requires authenticator reenrollment. Actual society custody procedures still need acceptance.

## Current API

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
| `GET /api/flats/{id}/activity` | Registry officer's latest 30 home changes |

The frontend is served alongside the binary in `build/web`; production Node is unnecessary under this packaging decision. Sensitive data responses are not cached. Request logs contain method, route template, status and elapsed time, without query terms or record identities. No service-worker data caching is implemented.

Passwords use Argon2id; sessions use hashed random bearer tokens, HttpOnly/SameSite Strict cookies, 30-minute idle and 8-hour absolute expiry. State-changing requests require an exact Origin and session CSRF token. Account status, current role terms and memberships are checked on each request; mutations recheck identity/role inside the same transaction as the version check and audit. Invitations/password recovery and privileged MFA work locally; production contact verification, custodian recovery, role/account administration and HTTPS remain pending. See [account-security acceptance](docs/account-security-baseline.md).

## What follows

Next: private documents, subsequent role/account administration, PWA/migration, approved finance examples/receipt policy, production recovery and a representative pilot. Manual financial records describe given charges and money already received; the portal does not initiate payment.

Automated billing, bank matching, payment gateways and Tally integration are conditional future work. No runtime LLM API is required.

See the [execution backlog](execution-backlog.md), [delivery strategy](housing-society-execution-strategy.md), [full implementation plan](housing-society-digital-platform-plan.md) and [visual direction](docs/design-system.md).
