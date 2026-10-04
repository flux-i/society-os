# Society OS

A thoughtful community portal for a 118-flat society. The working **local preview** includes an illustrated overview, occupied/vacant and active owner/tenant counts, sign-in, resident-scoped homes, registry administration, invitations/password recovery, authenticator protection, change history and verified SQLite snapshot/restore tools.

## Run locally

Requirements: Go 1.27+, Node 22.12+ and npm. This workspace was tested with Go 1.27.0, Node 25.8.1 and npm 11.21.0. Go dependencies and frontend packages are pinned in `go.mod`/`go.sum` and `web/package-lock.json`.

```sh
make setup
make run
```

Open **http://127.0.0.1:8080**. `make run` builds the application, creates the isolated fictional registry if needed, and serves the frontend and API from one Go process. Stop with Ctrl+C. Repeated seeding preserves the same fixture without duplicating records.

The database lives at `var/demo/society.db`. An alternative isolated database can be selected with `make run DB=var/another-demo/society.db`. The preview accepts synthetic-marked databases, requires `--demo` and binds only to a loopback IP. Production deployment remains disabled. Existing previews migrate to schema 3 without replacing registry records. The identity upgrade signs out old sessions.

Choose an account on the sign-in screen, then select **Sign in**. The fictional credentials are prefilled; every demo account uses the public preview password `Community-preview-2026!`.

| Account | Local preview access |
|---|---|
| `admin@demo.society` | Registry management/history, invitations and password recovery; MFA required |
| `committee@demo.society` | Community registry views; cannot change records |
| `owner@demo.society` | Only the owner's two active homes, A-101 and A-102 |
| `tenant@demo.society` | Only the tenant's active home, A-103; no former-tenant history |

**For the registry officer:** after Sign in, select **Use a preview code**, then **Verify and continue**. On first setup, save and acknowledge the displayed recovery codes. The shortcut is restricted to the four fictional accounts; invited identities use their own authenticator.

Open a home as the registry officer and choose **Manage home** to change occupancy, create/link a person or end a relationship. Every change requires a reason. **History** records the actor, time and before/after values. Concurrent stale saves are rejected and offer reload. People can be linked to multiple homes without duplicate person records; ended relationships are retained.

**Access & invitations** lets the officer select a current registry person, attest identity/email verification and issue a personal invitation for manual handover. Resident access follows current homes; committee/administrator invitations have a bounded role term. Invitations expire in one hour; recovery links in 30 minutes. Links work once; reissuing invalidates previous links. The portal sends no email/messages. Use fictional identities and example addresses here.

**Account security** supports authenticator setup, recovery-code replacement, five-minute identity confirmation and password changes. Reset/change signs out every target-account session and preserves MFA. Administrator access remains locked until factor verification succeeds. Passwords require at least 12 characters.

The overview distinguishes homes from people: the fresh fixture contains **109 occupied homes, 9 vacant homes, 118 distinct active owners and 35 distinct active tenants**. Occupied homes comprise 74 owner-occupied and 35 rented homes. Joint owners count separately; multi-home owners count once society-wide; former/future memberships do not count as active. Wing counts deduplicate people within each wing, so summing wings need not equal the society total when someone owns homes in different wings.

For frontend hot reload, keep the Go server running and run `make dev` in another terminal. Vite serves a local development URL and proxies API calls to `127.0.0.1:8080`.

## Check and measure

```sh
make check
make bench
make report
```

`make check` verifies formatting, Go vet, race-enabled backend tests and TypeScript. Tests cover registry constraints/counts, transactional mutations, concurrent stale edits, immutable audit, cross-flat and role denial, session/account/membership expiry, logout, Origin/CSRF, login/factor throttling, link expiry/revocation/concurrent consumption, TOTP replay, single-use codes, key loss, recovery and engine/migration provenance. There are 31 backend checks.

`make bench` runs the representative registry query three times. `make report` creates an independent fresh synthetic database, measures 100 warm local HTTP reads after ten warmups, snapshots the live database and restores it into a new environment. It verifies SHA-256 independently in Python and records measured times, engine settings and counts in `reports/local/account-security-baseline.json`. These are local measurements, not production performance commitments.

Browser checks use a fresh fictional database on an ephemeral loopback port and leave your preview untouched:

```sh
cd web
npx playwright install chromium
npm run test:browser
```

Build first with `make build`, or use `make browser-check`. Follow this machine's `AGENTS.md` instruction to launch Playwright outside any command sandbox. The current workspace has unrestricted execution. Twenty-six browser checks cover the registry, all 118 home cards, all nine dropdown menus, invitation activation, password reset/session revocation, authenticator enrollment, recovery-code reuse denial and rendered UI regressions across desktop, tablet, narrow phone and reduced-height layouts. The [UI review](docs/ui-review-baseline.md) records the control inventory, fixes, screenshot command and global Claude Code/Codex browser tooling.

## Local recovery

```sh
make snapshot SNAPSHOT=var/snapshots/checkpoint-01
make restore-check SNAPSHOT=var/snapshots/checkpoint-01 RESTORED_DB=var/restored/checkpoint-01/society.db
./build/society-server inspect --db var/restored/checkpoint-01/society.db
```

Snapshots use `VACUUM INTO` to include committed WAL changes consistently, then remove sessions, access links and recovery codes from the private copy before hashing. Session deletion also removes pending authenticator setup. Identity and audit remain intact; the live user's session is unaffected, and restored users must sign in again. Each new private bundle contains `society.db` and a manifest with SHA-256, byte size, schema/application/engine versions, fixture identity, timestamp and counts. Verification includes SQLite integrity, foreign keys and migration checksums. Restoration refuses existing databases and WAL/SHM sidecars. Use the matching release for a snapshot's schema; older checkpoints require their matching release before an explicit upgrade. Confirmed authenticator factors survive encrypted; preserve the matching MFA key separately. Saved recovery codes intentionally do not survive restoration.

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
| `GET /api/system` | Registry officer's application/schema/engine evidence |
| `GET /api/registry/summary` | Operator's community and wing occupancy/people totals |
| `GET /api/flats` | Authorized search/filter/pagination, with scoped totals |
| `GET /api/flats/{id}` | Authorized home and permitted current/historical relationships |
| `PATCH /api/flats/{id}` | Registry officer's version-checked occupancy change |
| `POST /api/flats/{id}/members` | Create/link a person and date-bounded relationship |
| `POST /api/flats/{id}/members/{membership}/end` | End a relationship while retaining history |
| `GET /api/people` | Registry officer's bounded person lookup |
| `GET /api/flats/{id}/activity` | Registry officer's latest 30 home changes |

The frontend is served alongside the binary in `build/web`; production Node is unnecessary under this packaging decision. Sensitive data responses are not cached. Request logs contain method, route template, status and elapsed time, without query terms or record identities. No service-worker data caching is implemented.

Passwords use Argon2id; sessions use hashed random bearer tokens, HttpOnly/SameSite Strict cookies, 30-minute idle and 8-hour absolute expiry. State-changing requests require an exact Origin and session CSRF token. Account status, current role terms and memberships are checked on each request; mutations recheck identity/role inside the same transaction as the version check and audit. Invitations/password recovery and privileged MFA work locally; production contact verification, custodian recovery, role/account administration and HTTPS remain pending. See [account-security acceptance](docs/account-security-baseline.md).

## What follows

Next: operation identities/jobs/private storage, then manual entries and receipt PDFs, notices/complaints/documents, production recovery and a representative pilot. Manual financial records describe given charges and money already received; the portal does not initiate payment.

Automated billing, bank matching, payment gateways and Tally integration are conditional future work. No runtime LLM API is required.

See the [execution backlog](execution-backlog.md), [delivery strategy](housing-society-execution-strategy.md), [full implementation plan](housing-society-digital-platform-plan.md) and [visual direction](docs/design-system.md).
