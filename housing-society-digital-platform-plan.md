# Housing Society Digital Platform — Detailed Implementation Plan

**Communication requirement, 7 October:** Staff must compose, review, independently approve and Send inside the portal; the user rejects opening WhatsApp Messenger and pressing Send. The unimplemented manual receipt/statement sharing proposal is superseded by [official backend WhatsApp sending](docs/portal-whatsapp-workflow.md). Prepare the adapter and local HTTP fixtures independently; society-owned account/number, server-held credentials, approved templates, HTTPS callback and costed activation remain dependent live inputs. These feature/pricing questions do not authorise real external messages or a paid account activation. Dated Meta documentation and the October 2026 INR rate card are recorded in the contract; do not assume non-commercial use is exempt or apply the free service allowance to utility templates. Earlier manual-share plan references are historical and do not require a Messenger step in the new workflow.

**Society size:** 118 flats
**Primary constraint:** Keep recurring operating cost below **₹12,000/year**
**Preferred hosting model:** Self-host the application on an owned small computer; use cloud services only where they add clear value
**Target architecture:** Lightweight custom application instead of ERPNext/Frappe
**Document date:** 4 October 2026
**Revision:** Manual tracking plus society operations scope, updated 7 October 2026
**Confirmed input:** 118 flats; resident/user count to be established during registry migration
**Accounting:** Existing very old Tally installation; exact version/import compatibility pending
**Society state:** Pending for receipt/retention policy; statutory billing rules are future inputs

**Implementation status:** Locally accepted Go/React release **0.19/schema 18** adds scoped operational finance exports to the preceding registry/identity/MFA, manual money/receipts, independent reviews/notices, services/documents, changing overview, account administration, maintenance/allocations, upkeep, collections, incidents/fines, contacts, synthetic targeted messages and exact prepared-original publication/sharing. [Export acceptance](docs/finance-exports-baseline.md) records **190 Go declarations, 176 ordinary cases/26 suites, 46 actual native cases/six suites, 37 inspected final captures**, matching schema-17/18 recovery, unchanged 74 prior persistent tables plus the new export table, preserved held keys and the pinned verified preview. Closed findings are **77 groups (75 product/UI, two operational)**. Personal public 0.19 publication follows final documentation; earlier [0.18 application](https://github.com/flux-i/society-os/commit/229f28efd4648e6f2ccc754c18d191b2932acf97) and its immutable 321-source/553-hash archive remain retained. The whole plan remains active and unfinished: official portal WhatsApp, community additions, measured performance/PWA/migration and independent production preparation follow. Real identity/custody, policy, infrastructure, providers and data acceptance stay separate. See [current checkpoint state](docs/next-session.md).

**Scope addition:** The user's subsequent requests are preserved in the [society operations roadmap](docs/society-operations-roadmap.md): an actionable changing overview, maintenance, fund collection campaigns, resident external-payment reports, targeted WhatsApp/email, evidence-based rule reports and authorised fines, and externally prepared financial statements with intentional publication/sharing. This dated addendum defines the next slices and supersedes earlier deferrals of those specifically requested capabilities. No payment initiation is added; older sections remain conditional references for unrequested billing/bank/Tally automation.

---

## 1. Executive Summary

Build a small **Society OS** for **118 flats**, hosted on a society-owned low-power Linux computer. Establish the actual owner/tenant/family account count during migration; it is not assumed to be 118.

Use Go, SQLite, a React/Vite PWA, private AWS S3 for documents/off-site snapshots, and a tested public HTTPS ingress. Keep the existing very old Tally installation for formal accounting independently; application integration is conditional future work.

Version 1 covers tracking and monitoring: registry and memberships; invitation/login and scoped roles; manually entered given charges/opening balances and already-paid records; exact amounts and derived flat balances; immutable generated receipts/PDFs and auditable corrections; notices; complaints; permitted documents; authorized reports; and tested recovery. Authorized users supply the entries. The portal does not initiate payment.

**Scope boundary:** No payment gateway, bank transfer initiation, bank API/automated matching, automatic billing/rate/interest calculation or Tally integration is required. Resident reports of externally paid money and treasury verification are requested and verified locally under the operations roadmap. That later request supersedes older exclusions of resident payment reports in this document. Sections 12 and 14 and unrequested extended financial designs after Section 13.0 remain conditional future references. Section 13.0 defines the verified manual-entry workflow; maintenance/fund/fine allocations extend it through separate checkpoints. Do not scaffold unrequested automation as a launch dependency.

The application remains one modular process and one local SQLite database, with a small database-backed job queue. Separate scheduled backup and ingress services are operational dependencies, not an additional business-service platform. No PostgreSQL, Redis, Kubernetes, full ERP, native apps, payment gateway, vector database, or local mail server is needed initially.

Manual WhatsApp sharing is included. Targeted WhatsApp/email delivery is now planned explicitly in the operations roadmap, using synthetic provider checks before society-owned live channel configuration is supplied. Full-text extraction/OCR and AI remain optional later capabilities; core operation and assisted account recovery do not depend on them.

Mandatory recurring costs are measured incremental electricity, S3 storage/requests/applicable transfer, and any selected production ingress/domain cost. Existing broadband has zero incremental cost only if the society already funds a suitable connection. The complete annual forecast, including taxes and contingency, must remain below ₹12,000; development/capital spending and volunteer effort are recorded separately.

**Recovery contract:** frequent verified off-site snapshots target a 1-hour RPO in normal connected operation; they do not promise zero loss of recent changes. Restore includes financial/numbering reconciliation, current-access review, and paused external jobs before writes resume. Required MFA, authorization, audit, quotas, and recovery are V1 foundations.

**Proceeding decisions:** retain this small architecture and start a local synthetic foundation. Confirm production hardware/ingress, document/receipt policy, representative manual entries, recovery custodians and the all-in annual forecast before their dependent production gates. Tally compatibility and automated billing rules are needed only if those future modules are requested.

---

# 2. Goals

## 2.1 Primary goals

The system should:

1. Replace scattered Excel sheets and paper-based operational records.
2. Give every resident a simple digital portal.
3. Allow authorized officers to record given charges/already-paid entries, generate receipts, monitor records, and manage notices, complaints and documents.
4. Make society records searchable and auditable.
5. Keep documents securely outside the physical office using S3.
6. Continue using Tally initially for formal accounting.
7. Avoid recurring SaaS subscriptions wherever possible.
8. Stay comfortably below the ₹12,000/year recurring-cost ceiling.
9. Be simple enough for a small team to maintain, with two named recovery custodians and documented committee handover.
10. Leave room for AI features later without designing the entire system around AI today.

---

# 3. Non-Goals for Version 1

The first release should **not** try to become a full ERP.

Do not build the following initially:

- Payroll
- HR
- Inventory
- Purchase management
- Full general-ledger/double-entry accounting engine; V1 requires exact manually supplied amounts, derived balances, immutable receipts and auditable corrections
- Payment initiation/gateways, bank APIs/automated matching, automated billing calculation and Tally integration
- Detailed per-invoice allocations, advance-settlement automation and resident payment-claim workflows unless separately requested
- GST accounting engine
- Vendor procurement workflows
- Facility booking unless explicitly required
- Visitor management
- Intercom integration
- Biometric access
- Native Android/iOS app
- Full document-management platform
- Vector database
- Large-scale OCR pipeline
- Automated bank integrations
- Payment gateway integration
- Complex workflow engine
- Custom report designer
- Multi-society SaaS architecture

This scope discipline is important to keeping both cost and maintenance low.

---

# 4. Recommended Architecture

```text
Residents / approved society officers
                  |
             stable HTTPS
                  |
      tested direct ingress or public tunnel
                  |
        society-owned Linux computer
       +-------------------------------------+
       | Go: same-origin PWA + API            |
       | auth / current resource permissions |
       | registry / manual flat entries      |
       | already-paid records / receipts     |
       | notices / complaints / reports      |
       | document authorization / audit      |
       | bounded SQLite-backed workers       |
       |                 |                   |
       |          SQLite on local SSD        |
       +-------------------------------------+
                  |                |
       signed/version-pinned       | approved optional APIs
       direct browser uploads/GETs | SES / WhatsApp / AI
                  v                v
         private S3 documents   independent optional channels

Independent scheduled backup command -> encrypted consistent SQLite snapshots
                                      -> restricted S3 backup prefix
Society custodians -> offline documents/snapshots/keys + restore runbook
Existing old Tally continues independently; future integration requires a proof
```

Financial state changes, account entries, audit, and job creation commit together in SQLite. S3/PDF/email work happens after commit with retry/reconciliation states. S3 never decides which resident may access a document; the application grants access to the exact approved version.

Local hardware failure can cause downtime and a bounded recent-data recovery gap. Backup, numbering, and external-effect reconciliation are part of the architecture rather than post-launch additions.

---

# 5. Technology Stack

## 5.1 Backend — Go

Use a modular Go application with one long-running application process. Serve the built frontend and same-origin API from this process. Node.js is a build-time dependency only.

Go provides a small production footprint, good HTTP support, and straightforward deployment. Pin maintained dependencies and keep reproducible x86-64/ARM64 release builds for the selected hardware.

**Build decision:** choose and record the SQLite driver before development. A pure-Go driver can simplify cross-compilation; a CGO driver needs the appropriate C toolchain and an explicit static/dynamic linking strategy. Verify the SQLite engine embedded in the application, rather than relying on the installed command-line utility.

Choose a maintained PDF library and embed required fonts, templates, and static assets. Test resident names, rupee symbols, and the society's actual languages in printed receipts. Local OCR, if added later, may introduce separate OS tools and language packs.

Production application path:

```text
/opt/society/current/society-server
```

Docker is optional for development and unnecessary for the initial production deployment.

## 5.2 Frontend

**Initial decision:** React + Vite, served as a same-origin PWA by Go. Embed build assets in release binaries, or package them in the same versioned release directory. Choose one packaging method and use it consistently.

Server-rendered Go + HTMX remains an alternative if the implementation team prefers it, but selecting it is a recorded architecture change rather than a second frontend to build.

Use mobile-friendly, accessible forms with clear entry, received-money confirmation, and PDF-processing states. The PWA update/cache policy is defined in Section 28.

## 5.3 Database — SQLite

SQLite is suitable for 118 flats and their associated resident accounts. Keep the live database on a local SSD, never on a network filesystem or synchronized cloud folder.

Use:

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
PRAGMA synchronous = FULL;
```

- WAL mode persists at database level. Apply and verify connection-specific settings, including foreign keys, busy timeout, and synchronous mode, for every connection created by the Go connection pool.
- Use short transactions and a bounded connection pool. Start with a controlled writer path; never hold a write transaction while calling S3, generating a PDF, sending email, or running OCR.
- Retry only well-defined lock conflicts within a bounded deadline. Do not retry financial mutations without their operation identity.
- Monitor WAL size and checkpoint progress. Avoid long-lived report/read transactions that prevent checkpoint completion.
- Run embedded, versioned migrations; use primary/foreign keys, unique indexes, NOT NULL, and CHECK constraints for business invariants.
- Verify FTS5 support in the selected driver/build before enabling the later search phase.

**Required engine fix:** use a maintained SQLite release containing the WAL-reset corruption fix. SQLite documents the fix in 3.51.3 and later, with backports including 3.44.6 and 3.50.7. The bug can affect multiple connections writing/checkpointing concurrently. Record `SELECT sqlite_version()` from the application in release validation.

`FULL` protects committed WAL transactions against power loss subject to correct filesystem/storage behavior. It does not protect against SSD loss; off-site recovery has the separately stated recovery-point objective.

References: [SQLite WAL and WAL-reset fix](https://www.sqlite.org/wal.html), [synchronous settings](https://www.sqlite.org/pragma.html#pragma_synchronous), [foreign-key enforcement](https://www.sqlite.org/foreignkeys.html).

---

# 6. Local Hardware

## 6.1 Minimum practical specification

If an existing system is already available, use it unless it is unreliable.

```text
CPU:      2 cores
RAM:      4 GB
Storage:  64 GB SSD
Network:  Ethernet preferred
OS:       Linux
```

Recommended headroom:

```text
CPU:      4 cores
RAM:      8 GB
Storage:  128 GB SSD
Network:  Gigabit Ethernet
```

For this workload, reliability and SSD health matter more than CPU performance.

## 6.2 Suitable machines

- Existing mini PC
- Intel N100-class mini PC
- Old business mini PC
- Old desktop with SSD
- Raspberry Pi-class ARM machine
- Old laptop

A laptop with a tested healthy battery can bridge a short power outage for the computer. Its battery does not keep the router/ONU online; measure battery runtime and configure lid/suspend behavior.

## 6.3 Power cost

Annual electricity consumption:

```text
Annual kWh = (average watts × 24 × 365) / 1000
```

Illustrative annual usage:

| Average draw | Annual consumption |
|---:|---:|
| 10 W | 87.6 kWh |
| 15 W | 131.4 kWh |
| 20 W | 175.2 kWh |
| 30 W | 262.8 kWh |
| 50 W | 438 kWh |

At an **illustrative** ₹10/kWh tariff:

| Average draw | Illustrative annual electricity cost |
|---:|---:|
| 10 W | ₹876 |
| 15 W | ₹1,314 |
| 20 W | ₹1,752 |
| 30 W | ₹2,628 |
| 50 W | ₹4,380 |

Use the society's actual electricity tariff for budgeting. Measure incremental wall draw including UPS losses and any newly added router/ONU equipment rather than using CPU TDP or a charger label. Existing network draw is included only to the extent this project adds it. Test sustained load, SSD health, UPS runtime, laptop lid/suspend behavior, and boot after mains return on the selected hardware.

---

# 7. AWS Usage

AWS should be used only where it provides clear durability or infrastructure value.

## Use AWS for

- **S3 document storage**
- **SQLite off-site backups**
- Optional SES email
- Optional cloud OCR later if there is a clear reason

## Do not use AWS initially for

- EC2
- RDS
- Elastic Load Balancer
- API Gateway
- NAT Gateway
- EKS
- ECS
- ElastiCache
- OpenSearch
- CloudFront
- Managed databases

The owned local computer is the compute layer.

---

# 8. Public Internet Access and Stable Resident URL

Keep ingress separate from application business logic. Select a stable production hostname before inviting residents; it becomes the PWA origin, notice-link base, password-reset base, and S3 CORS origin.

## 8.1 Decision process

```text
Can the society provide tested public access to representative client networks?
    |
    +-- Public IPv4 or working dual-stack + acceptable router configuration
    |      -> free DDNS + Caddy is viable
    |
    +-- IPv6-only origin, CGNAT, blocked ports, or unsuitable direct exposure
           -> public tunnel with IPv4/IPv6 client reachability
                  +-- Tailscale Funnel only after plan/terms/cost approval
                  +-- owned domain + Cloudflare Tunnel production fallback
```

Public IPv6 at the server does not establish reachability from IPv4-only resident networks. Test mobile data on representative carriers, external Wi-Fi, an IPv4-only network, and IPv6 where available. Compare WAN/public IPv4 and check private/CGNAT ranges, but treat an actual external connection test as the proof.

## 8.2 Free DDNS + direct HTTPS

A hostname such as `https://mysociety.duckdns.org` can avoid a domain purchase when publicly reachable IPv4 or dual-stack networking is available.

Use Caddy, reliable A/AAAA updates, restrictive IPv4 and IPv6 firewalls, and tested certificate renewal. Do not publish an unusable AAAA record. Router reboot, changing delegated IPv6 prefixes, and ISP port restrictions must be tested.

HTTP-01 certificate validation requires reachable port 80; TLS-ALPN-01 requires 443; DNS-01 requires DNS-provider integration. If only HTTPS is exposed, explicitly prove renewal with TLS-ALPN-01 or DNS-01 rather than assuming HTTP validation will work. Port 80, if enabled, serves redirects/challenges only.

Direct access requires no tunnel subscription, but depends on the DDNS provider, ISP, router, and the society's operational ability to maintain them.

## 8.3 Tailscale Funnel

Funnel provides a public `*.ts.net` HTTPS URL without router forwarding or residents installing Tailscale. Public website visitors are not automatically tailnet users/seats.

As checked on 4 October 2026, Funnel is beta and has non-configurable bandwidth limits. Personal-plan suitability cannot be assumed for the society. Verify organizational eligibility, required operator seats, unattended device authentication/key-expiry handling, restart persistence, and the annual quote before choosing it. Do not put the resident population into the private administration tailnet merely to access the public portal.

## 8.4 Cloudflare Quick Tunnel

Use random `*.trycloudflare.com` URLs for development/demos only. Quick Tunnels are intended for testing, have temporary hostnames, a 200 in-flight-request limit, and no Server-Sent Events support. They are not the permanent resident URL.

## 8.5 Owned domain + Cloudflare Tunnel

Use a society-controlled domain configured with Cloudflare when the direct route is unsuitable and Funnel eligibility/cost is unattractive. Published applications require a Cloudflare domain. The tunnel connects outbound to the local application, avoiding a static IP and inbound router forwarding.

Budget the actual domain renewal price and applicable taxes. Confirm the chosen Cloudflare plan/features have no required subscription for this deployment. The public application authenticates residents itself; do not accidentally require a separately priced external identity/seat product for all residents.

Configure tunnel credentials, service startup, proxy trust, and a final catch-all rule that rejects unmatched hostnames. Serve approved public static assets only; bypass caching for authenticated application data.

## 8.6 Acceptance and later ingress changes

Verify boot persistence, external HTTPS, certificate renewal, IPv4/IPv6 client access, correct client-IP handling, and login/S3 flows on the selected origin. An origin change requires updated `PUBLIC_BASE_URL`, allowed hosts/origins, cookies, reset/notice links, CORS, and PWA installation guidance. It does not require rewriting billing logic.

References: [Caddy HTTPS](https://caddyserver.com/docs/automatic-https), [Funnel](https://tailscale.com/docs/features/tailscale-funnel), [Tailscale pricing](https://tailscale.com/pricing), [Quick Tunnels](https://developers.cloudflare.com/tunnel/get-started/quick-tunnels/), [production tunnel setup](https://developers.cloudflare.com/tunnel/get-started/).

---

# 9. S3 Document Architecture

Store original documents and generated PDFs in a private S3 bucket. SQLite stores authorization, metadata, validation state, and the exact object version to serve. Local processing may use bounded temporary files, removed after validation/OCR; original documents are not permanently stored in the application directory.

## 9.1 Bucket and credential boundaries

One bucket is sufficient initially, with separately controlled prefixes:

```text
society-prod-documents/
    objects/<opaque-document-id>/<opaque-upload-id>
    generated/receipts/<financial-year>/<opaque-receipt-id>.pdf
    exports/<opaque-batch-id>/
    backups/sqlite/<date>/<unique-snapshot-name>
    recovery/numbering-series/<unique-series-registry-record>
```

Categories, owners, and visibility come from SQLite, not path names. Avoid personal names/phone numbers in object keys. Never let a client choose an arbitrary S3 key, prefix, or version for signing.

A generated-invoice prefix is a future addition if invoice generation is requested.

The document identity may read/write approved document prefixes, including version-specific reads, but cannot access backups or alter bucket controls. The scheduled backup identity writes new backup objects and cannot delete versions or read historical backups. Restore/lifecycle administration uses separately controlled credentials. The numbering-recovery registry has restricted custodial access. A second bucket is optional if IAM/retention separation is easier operationally.

## 9.2 Configuration

- Block Public Access ON; ACLs disabled; default SSE-S3; versioning ON.
- Deny insecure transport, enforce least-privilege prefix permissions, and keep AWS root credentials off the server.
- Configure CORS for the exact approved production origin, required upload methods/headers, and any checksum/version headers needed by browser requests. Development origins belong to a separate development configuration.
- Configure backup, abandoned-upload, and noncurrent-version retention deliberately. Do not apply short-lived backup cleanup rules to legal documents.
- Application credentials cannot delete object versions, disable versioning, edit bucket policies, or change retention. Restricted custodial cleanup handles approved permanent deletion.

CORS enables browser access; it is not an authorization or upload-quota mechanism.

## 9.3 Upload state machine

```text
Authenticated, authorized upload request
-> database PENDING record + quota reservation + unique temporary object key
-> short-lived presigned upload
-> browser uploads directly to S3
-> authenticated /complete request
-> server verifies expected object/version, size, and checksum
-> VALIDATING: bounded content inspection/quarantine job
-> AVAILABLE or REJECTED
```

Initial limit: 20 MiB per ordinary resident document, configurable by category for approved committee uploads. Set per-user and society aggregate quotas, including pending uploads, before launch. No unlimited multipart upload in V1; approved larger archival files use a documented administrative path.

Prefer a presigned POST policy with a content-length range for browser size limits, or a presigned PUT whose signed constraints and conditional-write behavior are proven on the supported browsers. Bind checksum and required headers. Do not rely only on the size the browser reported.

`/complete` rechecks actor permission and ownership of the pending upload, verifies the expected object using S3 HEAD/checksum data, records its version ID, and is idempotent. A SHA-256 checksum verifies integrity, not file safety; inspect actual bytes/types in the validation worker. Never publish an object solely because the client reports success.

Presigned PUT URLs can be reused and overwrite a key until expiry. Prevent replacement of a validated document using immutable upload keys, suitable conditional writes, and version-pinned downloads. Any promotion/copy must use the validated source version.

Initial upload allowlist: PDF, JPEG, and PNG for resident documents. Additional committee formats need a specific validation policy. Reject executable/script/HTML/SVG, macro-enabled files, and archives by default. Keep parsers patched, bound page/image dimensions and decompression, and serve untrusted originals as attachments. Add malware scanning if the accepted formats/workflow require it; a MIME check does not prove a file harmless.

Expire abandoned pending uploads/quota reservations after 24 hours, with a job that safely removes temporary objects through restricted cleanup permissions. Define handling of successful uploads whose browser never calls `/complete`. Abort stale multipart uploads if an administrative multipart path is enabled.

## 9.4 Download and permission semantics

```text
GET /api/documents/{id}/download
-> authenticate + current operation/resource entitlement
-> require AVAILABLE and approved exact version
-> sign GET for stored key/version
-> browser downloads directly from S3
```

Initial URL lifetime: 2 minutes, bounded by credential validity and bucket policy. Signed URLs are bearer tokens: anyone holding one can use it until expiry, and a transfer started before expiry may finish afterward. App logout, role revocation, and membership changes stop new URL issuance but do not instantly revoke an existing URL.

Do not log/cache signed URLs or include them in permanent notices. For sensitive categories that need stronger per-request revocation, use an application-proxied download with its cost/availability tradeoff recorded. Permission changes never expose older versions automatically.

Use private/no-store responses for authorization/signing endpoints and suitable private object cache headers; specify Content-Disposition safely without trusting raw filenames.

References: [S3 presigned URLs](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html), [CORS](https://docs.aws.amazon.com/AmazonS3/latest/userguide/enabling-cors-examples.html), [POST size policy](https://docs.aws.amazon.com/AmazonS3/latest/developerguide/sigv4-HTTPPOSTConstructPolicy.html), [conditional writes](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html).

---

# 10. Core Data Model

This is a single-society application. Tables below are a conceptual schema; production migrations must define types, foreign keys, indexes, checks, and deletion behavior explicitly.

## 10.1 Common conventions

- Use opaque internal IDs. Public document numbers are separate from internal identities.
- Money is signed 64-bit integer paise with checked arithmetic. Fractional rates/quantities use canonical decimal strings or explicitly scaled integers and exact Go arithmetic; never floating-point money or SQLite NUMERIC coercion.
- Event timestamps are UTC. Accounting dates, billing periods, and membership dates are calendar dates interpreted in the configured society time zone, initially `Asia/Kolkata`.
- Include `created_at`, `updated_at` where mutable, and a version counter for records subject to concurrent editing.
- Financial source records and posted ledger entries are immutable. Corrections append linked reversing/replacement records.
- Archive referenced people/flats rather than cascading deletion of financial history.

## 10.2 Society, buildings, and flats

```text
societies: id, name, registration_number, address, state,
           timezone, financial_year_start_month, financial_year_start_day
buildings: id, name, code
flats: id, building_id, flat_number, floor, area_sqft_decimal,
       maintenance_category, parking_count, status
```

Require unique building codes and unique `(building_id, flat_number)`. Confirm the master list contains 118 distinct flats. Registry status does not automatically create charges. If billing automation is requested later, approve its treatment of vacant, inactive, disputed and exempt flats separately.

Current receipts snapshot the flat label and payer/recipient. Subsequent registry changes must not rewrite earlier receipts. Any future issued invoices also preserve their billing inputs and party snapshots.

## 10.3 Residents and memberships

```text
residents: id, full_name, phone_normalized, email_normalized, status,
           created_at, updated_at, archived_at
flat_memberships: id, flat_id, resident_id, relationship,
                  start_date, end_date, is_primary_contact,
                  can_view_finances, granted_by, financial_access_start_date
```

Relationships: `OWNER`, `TENANT`, `FAMILY`, `AUTHORIZED_OCCUPANT`.

Use half-open validity intervals: membership is active on/after `start_date` and before `end_date`; NULL end date means ongoing. Support joint owners and an owner with multiple flats. Primary contact is a communication preference, not ownership or a financial-access grant. Enforce the approved primary-contact and overlap rules with transactional validation and constraints where possible.

Names and contact details are not reliable deduplication keys. Shared household contacts and recycled phone numbers require manual review. Keep login identities separate from resident contact records.

## 10.4 Historical access policy

Initial policy:

- Current owners can see their flat's current account balance and permitted charge records; earlier payer identity, payment evidence, personal receipts, and resident-specific documents remain restricted.
- Tenants/family/authorized occupants get financial access only through an explicit, audited grant. Limit access to its recorded start date unless broader access is separately approved.
- A former occupant loses membership-based access when the relationship ends. Any access retained to their own historical receipts is an explicit person-based entitlement, not access to the former flat's whole account.
- Receipt access follows the named payer/recipient and explicit authorizations. Owners may receive redacted settlement information when needed without inheriting another person's private documents.
- Ending one membership does not disable other active memberships or an independently authorized role.
- Changes take effect for new API requests, exports, searches, and URL issuance. Existing presigned URLs have the bounded lifetime described in Section 9.

Store billed-party and payer/receipt-recipient snapshots and explicit document subjects. Approve this policy with the committee before importing historical personal records.

## 10.5 Identity and financial entities

Identity tables are defined in Section 11. Current manual entries, account effects and receipts follow Section 13.0, with numbering in Section 50. Sections 12, 13.1–13.4 and 14 describe conditional future billing, claims, bank matching, allocations and accounting integration.

Do not substitute a mutable outstanding-total column for the underlying financial records.

---

# 11. Authentication and Roles

## 11.1 Roles and permissions

Use explicit permissions; users may hold more than one role. Check both operation permission and resource/person/flat scope on every request. Role inheritance must be documented rather than inferred from the navigation menu.

| Role | Initial permissions |
|---|---|
| Resident | Authorized flat/account views, own receipts and complaints, audience-matched notices and permitted documents |
| Committee Member | Approved operational views, notice publishing and complaint handling; finance-entry or role-administration powers require a separate grant |
| Treasurer | Add/post given manual entries, confirm already-received money, make linked corrections and access authorized financial reports; committee operations where explicitly assigned |
| Administrator | Invitations/accounts, memberships, approved role assignments, configuration/categories; financial posting requires a separately assigned treasurer permission |
| Accountant/Auditor | Read/export approved financial records and reconciliation evidence; no resident administration or financial writes by default |

Grants have `valid_from`, `valid_until`, `granted_by`, and revocation fields. Committee/treasurer appointments have term expiry. Accountants have time-limited access. Prevent self-assignment of financial roles and accidental removal of the last recoverable administrator. Record the bootstrap and successor-administrator procedure.

## 11.2 Identity schema

```text
users: id, resident_id nullable, display_name, password_hash, status,
       password_changed_at, auth_version, created_at, disabled_at
login_identifiers: id, user_id, kind EMAIL|PHONE, normalized_value,
                   verified_at; unique(kind, normalized_value)
role_grants: id, user_id, role, valid_from, valid_until,
             granted_by, revoked_at, revoked_by
sessions: id, token_hash, user_id, auth_version, created_at,
          last_seen_at, idle_expires_at, absolute_expires_at, revoked_at
account_tokens: id, user_id, purpose INVITE|PASSWORD_RESET,
                token_hash, expires_at, consumed_at, created_by
mfa_factors: id, user_id, encrypted_totp_secret, confirmed_at, disabled_at
mfa_recovery_codes: id, user_id, code_hash, consumed_at
```

Prefer opaque random server-side sessions. Store hashes of session/reset/recovery tokens rather than usable tokens. Restrict identity-table access and exclude secrets from audit snapshots.

## 11.3 Login and sessions

Use verified phone/email + password. No paid SMS OTP dependency is required for ordinary login.

- Use Argon2id with parameters calibrated on the actual server. Bound concurrent password work to prevent resource exhaustion.
- Use Secure, HttpOnly, SameSite cookies; document CSRF handling for all mutations.
- Start with a 30-minute idle and 8-hour absolute privileged-session limit; resident sessions may use a documented longer policy. Reauthenticate for role, MFA, bank-details, and high-impact financial changes.
- Revoke sessions after password resets, disabling accounts, sensitive grant changes, or recovery restores. Recheck current grants/memberships on each request.
- Apply bounded login/reset/upload rate limits and generic authentication errors. Trust client-IP headers only from the configured ingress proxy.
- Normalize phone numbers and emails consistently; a contact edit does not automatically transfer a login identity to a new owner.

## 11.4 Required privileged MFA

Require TOTP MFA for administrator and treasurer accounts before production access. Require it for any committee account with user/role-administration powers. Store TOTP secrets encrypted using a recoverable key kept separately from data backups; provide single-use recovery codes and an audited lost-factor procedure.

## 11.5 Invitations and account recovery

No public self-registration grants access to a flat. The administrator invites a person, verifies their relationship, and issues a short-lived single-use activation token through an approved existing channel or in-person process.

When email is enabled, use verified-email, single-use password-reset links. The API separates reset request from token-based completion.

For phone-only users or email outages, permit an audited administrator-assisted recovery after identity/relationship verification. Generate a short-lived one-time token, deliver it through an approved channel, require a new password, and invalidate previous sessions. Administrators cannot read existing passwords. Privileged MFA recovery requires a second authorized officer or the documented offline recovery process.

Maintain two named society-controlled recovery custodians. Do not share one personal administrator account.

---

# 12. Automated Maintenance Billing as Conditional Future Work

Current V1 records supplied charges/opening balances without calculating statutory rates, interest or recurring bills. The rule engine and invoice-generation design below is retained for a future request and is not a current build/launch dependency.

## 12.1 Approved rule versions

Configure charges from the society's approved bylaws and accountant instructions. The society's state, tax treatment, and statutory charge rules remain launch decisions; never assume jurisdiction-specific percentages or exemptions.

```text
maintenance_components: id, name, calculation_type, active
maintenance_rule_versions: id, component_id, effective_from, effective_until,
                           amount_paise, rate_decimal, applicability_json,
                           approved_by, approved_at, approval_reference
```

Calculation types: `FIXED`, `PER_SQFT`, `PER_PARKING`, `MANUAL`.

Components may include maintenance, sinking/repair funds, parking, water, non-occupancy charges, and separately identified interest. Version any changes to calculation basis, rates, exemptions, and applicability; never mutate a version referenced by an issued invoice.

## 12.2 Calculation contract

Before coding, obtain worked examples covering ordinary, vacant, exempt, rented, joint-owner, and multiple-parking flats; rate changes; backdated corrections; and advance balances.

Define:

- Period boundaries, issue/due dates, holiday treatment, and the configured financial year.
- Eligibility independent of whether a flat currently has an occupant.
- Whether mid-period area, occupancy, parking, or rate changes are prorated. Initial default is the approved period-start snapshot unless the accountant requires another method.
- Exact quantity/rate precision and integer-paise rounding. Default to rounding each line to the nearest paise, half up, then summing rounded lines; accountant approval is required for any final whole-rupee rounding line.
- Interest basis, start date, rate, simple/compound method, eligible principal, previous payments, and whether tax applies. Interest is a dated, traceable line/charge event and is never silently recomputed inside an issued invoice.
- Tax lines and required invoice disclosures, if applicable; Tally continues to perform formal tax accounting. Excluding a full GST engine does not waive applicable billing requirements.

Use checked exact arithmetic. Store interest and rounding as identifiable lines; do not also add an independent invoice-level penalty field.

## 12.3 Billing cycles and invoices

```text
billing_cycles: id, period_start, period_end, issue_date, due_date,
                status DRAFT|GENERATED|ISSUED, generated_at, generated_by
invoices: id, flat_id, billing_cycle_id nullable, invoice_number nullable,
          invoice_kind PERIODIC|SUPPLEMENTAL, issue_date, due_date,
          billed_party_snapshot_json, flat_snapshot_json,
          total_paise, lifecycle_status DRAFT|ISSUED|CANCELLED,
          issued_at, issued_by, version
invoice_lines: id, invoice_id, rule_version_id nullable,
               line_kind CHARGE|INTEREST|TAX|ROUNDING,
               description, quantity_decimal, rate_decimal, amount_paise
```

At most one standard periodic invoice per `(flat_id, billing_cycle_id)`; supplemental invoices have a separate recorded reason. Prevent overlapping duplicate cycles for the same billing schedule. Generation has an operation identity and can be retried without creating duplicates.

Generate drafts, review a preview with flat count/component totals/exceptions, and explicitly issue. Issuance allocates the public number, freezes the invoice and snapshots, posts its account entry, writes the audit event, and queues any rendering/notification work in one transaction. A full monthly cycle normally contains 118 invoices unless documented eligibility rules exclude flats.

## 12.4 State and balances

Lifecycle status describes issuance/cancellation. Derive settlement separately: `UNPAID`, `PARTIALLY_PAID`, or `SETTLED`. Derive `is_overdue` from the society-local due date and remaining payable amount. An invoice can be partially paid and overdue simultaneously.

Outstanding amounts come from posted charges and active allocations defined in Section 13. A stored aggregate is only a reconciled cache, not an independently editable balance.

Issued invoice amounts, dates, identity snapshots, and numbers cannot be edited. Corrections use referenced credit/debit adjustments or a documented cancellation and replacement. A cancelled invoice remains in history, with compensating entries and released/reallocated credits; its original number is retained.

Backdated changes and closed financial periods require treasurer authorization, a reason, and accountant reconciliation.

---

# 13. Manual Financial Entries and Receipts

## 13.0 Current manual tracking contract

The portal records provided information about charges and money already received. It neither initiates a payment nor connects to a bank to collect or automatically verify money.

- Authorized finance operators enter the flat/payer, entry kind, amount in exact integer paise, effective/received date, method/reference where relevant, description, source/evidence and actor. Ordinary resident access does not automatically grant finance-entry permission.
- Entry kinds cover given charges/opening balances and already-paid records. Draft/unconfirmed entries remain distinct from confirmed received-money entries. The authorized operator confirms that a receipt source represents money already received; the interface must not label an unconfirmed assertion as bank-verified.
- Derive balances from approved posted charges/opening balances and recorded paid credits. Do not invent interest, taxes, statutory charges or automatic bill schedules. A negative net balance is a credit; detailed invoice-level allocations/advance settlement are added only if required later.
- Confirmation carries a stable operation identity. The entered record, its account effect, receipt identity/number when applicable, audit event and unique PDF job commit atomically. Repeated submissions cannot create a second credit or receipt.
- Issue a receipt only for a confirmed received-money entry, not for a charge or opening debit. Freeze the payer/flat/amount/date/reference snapshot and use the existing numbering/recovery controls in Section 50.
- Generate the receipt PDF from the frozen snapshot after commit. S3/render failure leaves the saved entry visible and the same receipt PDF pending/failed; retrying never posts money again or issues a new number.
- Preserve confirmed entries and issued receipts. Corrections/reversals require permission, a reason, linked compensating records and audit; do not silently edit/delete history. A correction in the portal does not initiate a refund or transfer.
- Reports distinguish provided charges, recorded received amounts, balances, corrections, unconfirmed entries and receipt/PDF states. Their figures describe entered records, with provenance and dates.
- Before financial production use, review representative given-entry examples, receipt fields, historical access and correction policy. Local implementation may use explicitly fictional examples while those inputs are pending. Existing Tally continues separately; no Tally import proof blocks this workflow.

The extended claims, bank matching, allocations and accounting model below is conditional future design. Reuse its integrity controls where applicable without implementing its additional workflows in V1.

### Current conceptual entities

Use a `manual_entries` source record with flat, kind, exact amount, effective date, method/reference where applicable, payer snapshot, description, source/evidence, actor and posting timestamps. Distinguish drafts from posted records. Charge amounts and received amounts are positive; opening balances can be signed. Validate bounds and checked arithmetic. A linked correction preserves its original source and reason.

Use append-only `account_entries` for signed balance effects, with one unique posting per source event and matching flat IDs. A posted charge increases the balance, a received-money entry decreases it, and an opening balance carries its approved sign. Drafts have no balance effect. Reports derive results from these entries.

Each system receipt references one confirmed received-money manual entry, with unique source and number/namespace constraints. Store immutable payer/flat/amount/date/reference snapshots and independent PDF state. A charge, opening balance or correction alone cannot masquerade as a new incoming-money receipt. Legacy document imports preserve provenance and do not create a new received-money entry.

Final migrations must define these constraints explicitly. Do not add the future bank, claim, invoice or allocation tables to make this manual workflow runnable.

## 13.1 Future reference: collection and claims

Residents pay the society's approved bank/UPI account using UPI, NEFT, IMPS, cheque, or permitted cash. Display verified bank details and a bank-issued QR. Changes require treasurer permission, MFA reauthentication, and an audit event.

A resident UTR/screenshot is a **claim**, not confirmation that the society received money. Claim attachments use the private validated-upload service.

```text
payment_claims: id, flat_id, submitted_by, amount_paise, claimed_payment_date,
                method, reference, evidence_document_id nullable,
                status SUBMITTED|MATCHED|REJECTED, matched_payment_id nullable
bank_accounts: id, label, account_details, approved_qr_document_id, active
bank_transactions: id, bank_account_id, bank_transaction_identity,
                   credited_date, amount_paise, method, reference,
                   evidence_reference, recorded_by
payments: id, flat_id, bank_transaction_id nullable, claim_id nullable,
          payer_snapshot_json, amount_paise, received_date, method,
          cash_register_reference nullable, cheque_details_json nullable,
          status PENDING|POSTED|REJECTED|REVERSED,
          recorded_by, verified_by, verified_at, replacement_of nullable
```

Require a bank-credit match for electronic payments, confirmed clearance for cheques, and the approved cash-register evidence for cash. Record receipt of an uncleared cheque as pending acknowledgement; issue the financial receipt only after the approved clearance check.

Normalize duplicate references per payment method and receiving account. The same credited bank transaction can fund at most one active posted payment; a correction uses a linked reversal/replacement. A blank UTR is not a global uniqueness key. A resident claim that matches an already recorded payment attaches to that payment rather than creating another credit.

## 13.2 Future reference: extended operational flat account

Maintain a small append-only receivables ledger while retaining Tally for formal accounting.

```text
opening_balances: id, flat_id, cutoff_date, signed_amount_paise,
                  original_due_date nullable, source_reference,
                  migration_batch_id, approved_by, approved_at
financial_adjustments: id, flat_id, related_invoice_id nullable,
                       kind CREDIT|DEBIT, amount_paise, effective_date,
                       reason, evidence_reference, related_credit_entry_id nullable,
                       posted_by, reversal_of nullable
account_entries: id, flat_id, effective_date, posted_at, event_kind,
                 delta_paise, source_type, source_id, reversal_of nullable
credit_allocations: id, flat_id, credit_entry_id, charge_entry_id,
                    amount_paise, allocated_at, allocated_by
allocation_releases: id, allocation_id, amount_paise, released_at,
                     released_by, reason, financial_event_id
```

Positive ledger deltas increase the flat's dues; negative deltas provide credit. Sources include issued invoices, signed opening balances, posted payments, approved adjustments, and referenced reversals. Enforce one posting per source/event, supported source relationships, and matching flat IDs.

`credit_allocations` includes payment allocations, opening advances, and credit adjustments. Allocate eligible net credit to invoices, opening debit balances, or supplemental debit charges. Released allocations remain in history. Never delete allocations to conceal a correction.

Default allocation is oldest due charge first, unless a treasurer records another instruction. Excess payment remains an identifiable unapplied advance. An advance already recorded at migration can settle a later invoice without creating a fictional payment or receipt.

Conservation rules:

- Flat balance equals the sum of posted ledger deltas; a negative balance is net credit.
- Net allocations cannot exceed either the eligible credit's remaining capacity after reversals/refunds or the target charge's remaining amount. Reversal/refund entries are not new allocatable invoice charges.
- All source/target entries belong to the same flat; cross-flat transfers require linked approved adjustments.
- Reversing a payment first releases its affected allocations, records the compensating ledger event, and reopens the affected dues.
- No financial record is silently edited after posting; corrections preserve the original actor, evidence, reason, and links.

If a later scope includes refund tracking, refunds are manually executed through the society's bank and recorded as approved REFUND events with outward-bank/cash evidence and a link to the original credit. They consume available net credit capacity; any required allocation release is explicit and atomic. Do not allocate the refund's compensating ledger entry as a new bill or issue an incoming-money receipt for it. AI and the portal do not independently initiate bank transfers.

## 13.3 Future reference: bank-matched posting and receipts

A verification request carries a stable operation identity. In one SQLite transaction:

```text
Match credited bank transaction / approved cash evidence
-> post payment and account credit exactly once
-> allocate approved available credits
-> reserve financial-year receipt number
-> create immutable receipt snapshot
-> append audit event
-> enqueue unique receipt-PDF job
-> commit
```

```text
receipts: id, receipt_kind SYSTEM|LEGACY, payment_id nullable,
          receipt_number, numbering_namespace, financial_year,
          legacy_source_reference nullable, issued_at, issued_by,
          recipient_user_id nullable,
          recipient_snapshot_json, flat_snapshot_json, amount_paise,
          status ISSUED|REVERSED|VOID,
          pdf_status PENDING|AVAILABLE|FAILED,
          document_id nullable, correction_reference nullable
```

Require a source payment for SYSTEM receipts, uniqueness of non-null payment IDs, and unique `(numbering_namespace, receipt_number)`. LEGACY receipts preserve the original printed number under an issuing-year/book/source namespace and do not create post-cutoff collections or a fabricated payment.

A receipt records money received, including an advance; later allocation of that advance does not generate another cash receipt. State whether the historical allocation breakdown appears on the PDF; if printed, preserve the issuance-time snapshot.

The worker renders the frozen receipt, uploads to an immutable S3 key, records the validated object/version, and marks PDF availability. During an S3 outage the posted payment remains visible and its PDF remains pending; retries reuse the same receipt identity and number.

Reference example: `RCPT/2026-27/000124`.

Reversal/void actions preserve the original record, PDF, number, and an explicit correction notice, with corresponding ledger/allocation treatment. Never issue a second receipt merely because rendering or delivery was retried. Numbering and restore reconciliation are defined in Section 50.

## 13.4 Future reference: expanded reconciliation and control

Daily during collection activity, compare confirmed bank/cash receipts, posted payments, receipt register, advances, and Tally export batches. Resolve unmatched claims and bank credits explicitly. Record who checked the reconciliation and any differences.

Treasurer verification and exceptional adjustments require MFA. Where the society has two finance officers, use a second reviewer for corrections/refunds and bank-details changes; otherwise document compensating accountant review. Do not make routine collection depend on an unavailable second officer.

---

# 14. Tally Integration as Conditional Future Work

V1 does not require application export/import integration with Tally. Existing formal accounting continues separately. Apply the compatibility proof below before a future integration is implemented.

## 14.1 Confirm the installed version first

**Confirmed user input:** the society uses a very old Tally installation. Its exact product, version, release, company settings, and supported import formats are still unknown.

Keep Tally as the formal accounting system. Society OS owns operational claims, invoices, flat account entries, and receipts; decide the formal accounting posting basis with the accountant.

Tally.ERP 9 documentation describes schema-specific XML import. Do not assume a generic CSV is importable into the installed version. CSV may be an accountant-approved manual-entry worksheet, with that manual workflow stated explicitly. Modern TallyPrime Excel/CSV features do not establish compatibility with an older installation.

Reference: [Tally.ERP 9 import format](https://help.tallysolutions.com/docs/te9rel60/Data_Management/Import_of_Data_Intro.htm).

## 14.2 Compatibility proof before integration implementation

During foundations work, obtain the accountant's approved mapping and test on a backup/copy of the Tally company:

- Existing flat, fund, income, bank, and adjustment ledger names; whether missing masters may be created.
- Invoice/accrual versus collection-only posting, financial-year/date settings, bill-wise references, voucher numbering, and applicable tax treatment.
- A normal invoice, payment, partial payment, advance, opening balance treatment, and reversal/correction.
- Importing the same batch twice and detecting/preventing duplicate vouchers.
- Voucher totals, per-flat closing balances, receipt references, and debit/credit balance after import.

Select version-compatible XML or a proven manual-entry export before finalizing the financial interface. Do not upgrade or write into live Tally data as part of the proof. Existing Tally records remain the source of pre-cutoff history unless explicitly migrated.

## 14.3 Export contract and traceability

```text
accounting_mappings: component/event/account -> approved Tally ledger/voucher
export_batches: id, period, format, mapping_version, created_by, created_at,
                file_document_id, checksum, status, accountant_acknowledged_at
export_batch_items: id, batch_id, source_type, source_id,
                    source_revision, external_voucher_identity, export_status
```

Export the agreed dates, voucher types, ledger legs, debit/credit amounts, flat/bill references, UTR, invoice/receipt number, and stable external identity. Formal voucher legs must balance under the accountant-approved mapping even though Society OS is not a general accounting engine.

A repeated download of a batch reproduces the same content. A correction creates a linked reversing/replacement export rather than silently changing a previously acknowledged batch. Do not label a record imported solely because a file was downloaded; record accountant acknowledgement/import evidence and any exceptions.

CSV remains available for reconciliation/reporting and uses spreadsheet-safe escaping. If manual entry is selected, acceptance includes the accountant's demonstrated process and agreed workload.

---

# 15. Notices

```text
notices: id, notice_number nullable, title, body_markdown, visibility,
         published_at, expires_at, attachment_document_id nullable,
         created_by, status DRAFT|PUBLISHED|ARCHIVED, version
notice_audience_targets: id, notice_id, target_type BUILDING|FLAT, target_id
```

Visibility: `ALL_RESIDENTS`, `OWNERS_ONLY`, `TENANTS_AND_OWNERS`, `COMMITTEE_ONLY`, `SPECIFIC_BUILDING`, `SPECIFIC_FLATS`. Validate that targeted notices have approved targets and untargeted notices do not acquire an accidental broader audience.

Evaluate current role/membership eligibility for listing, search, detail, attachment, and URL issuance. A notification-recipient snapshot records who was contacted; it does not grant permanent access to the notice or attachment.

Render plain text or a restricted, sanitized Markdown subset; no arbitrary embedded HTML/scripts. Publishing requires notice permission, a selected audience, and confirmation that attachments have compatible visibility. Record edits as revisions and audit publishing, audience changes, and archival. An attachment must not widen the notice's audience through another endpoint.

## WhatsApp in Version 1

A **Share on WhatsApp** button creates a prefilled message containing the canonical portal notice URL. The committee selects recipients in WhatsApp; the application does not infer delivery, readership, or recipient identity from clicking Share.

Share a portal link, not a long-lived S3 URL or private resident data. Protected notices still require login and authorization. Critical communications use the society's established fallback channel during portal outages.

Automated messaging remains a later, separately budgeted feature.

---

# 16. Complaints / Service Requests

```text
complaints: id, complaint_number, flat_id, resident_id, category,
            subject, description, priority, status, assigned_to nullable,
            created_at, updated_at, resolved_at nullable, closed_at nullable,
            version
complaint_updates: id, complaint_id, message, created_by, created_at,
                   visibility RESIDENT_VISIBLE|STAFF_ONLY
complaint_attachments: complaint_id, update_id nullable, document_id
```

Statuses: `OPEN`, `ACKNOWLEDGED`, `IN_PROGRESS`, `WAITING`, `RESOLVED`, `CLOSED`.

Categories initially include plumbing, lift, electrical, security, cleaning, water, parking, common area, and other; committee administrators may maintain the list.

## Ownership and transitions

- Residents create and see their own complaints and resident-visible updates. Shared-flat access is an explicit grant, not an automatic right to another person's complaint.
- Authorized committee handlers acknowledge, assign, prioritize, record waiting reasons, and resolve cases. Assignees are active authorized users; vendors do not gain accounts/access by being mentioned in an update.
- The resident may confirm closure or request reopening with a reason; committee closure/reopening is audited. Record any configured inactivity-closure policy before enabling it.
- Record assignment/status history rather than overwriting all evidence. Validate transitions and handle concurrent edits with the record version.
- Staff-only updates and their attachments remain hidden in resident views, APIs, exports, search, and future AI retrieval.
- Ended memberships remove flat-based access, while any retained own-history access follows the explicit personal-record policy.

Provide expected response guidance and the society's emergency/security contact route. The portal is not an emergency dispatch mechanism; an urgent complaint must not imply someone has been notified merely because it was submitted.

---

# 17. Documents

## 17.1 Metadata and validated versions

```text
documents: id, s3_key, s3_version_id, original_filename, detected_content_type,
           size_bytes, verified_sha256, category, visibility,
           flat_id nullable, subject_resident_id nullable,
           uploaded_by, created_at, available_at, archived_at,
           expiry_date nullable, validation_status, validation_error_code,
           replaces_document_id nullable, version
upload_requests: id, document_id, requested_by, expected_size_bytes,
                 expected_sha256, temporary_key, expires_at,
                 reserved_quota_bytes, completed_at
```

Validation states: `PENDING`, `VALIDATING`, `AVAILABLE`, `REJECTED`, `ABANDONED`. A replaced or archived object remains linked to its historical metadata and governed by retention policy.

Categories include receipt, invoice, notice, AGM minutes, audit report, contract, AMC, legal, circular, resident document, committee document, payment evidence, and accounting export.

## 17.2 Visibility constraints

| Visibility | Required subject/scope |
|---|---|
| ALL_AUTHORIZED_RESIDENTS | An available society document deliberately approved for current residents |
| COMMITTEE_ONLY | Explicit committee document permission; no automatic resident access |
| FLAT_SPECIFIC | A flat ID plus the approved current/historical financial or operational entitlement |
| RESIDENT_SPECIFIC | A named subject resident and explicit grants; never inherited by a new occupant |
| ACCOUNTING_ONLY | Financial read permission and approved accountant/treasurer scope |

Enforce valid category/visibility/subject combinations in validation and database constraints. Resident uploads default to restricted resident/flat scope and cannot self-publish to all residents. Receipt/payment-evidence access follows the Section 10 historical policy.

Creation, completion, listing, metadata retrieval, search, preview, downloads, replacement, export, and archival all use the same entitlement rules. Do not reveal restricted filenames or existence through error messages or counts.

## 17.3 Contracts and archival

Store structured expiry dates for contracts/AMCs only if expiry reporting is included; text search/AI is not the authoritative source of an expiry date. Permission changes take effect on new requests and indexes as described in Sections 9 and 18.

Permanent deletion is a restricted, audited retention process that includes all applicable object versions, extracted text, previews, and expired backup copies. Routine UI actions archive records; legal retention and holds are approved before purge.

---

# 18. Search

V1 includes authorized metadata filters and bounded searches on current notices/documents/complaints. Full extracted-text search is an optional Phase 7 enhancement, not a launch dependency.

Use SQLite FTS5 for later full-text search across notices, permitted complaint text, and extracted text from documents such as meeting minutes. Meeting minutes initially remain documents rather than requiring a separate meeting workflow.

- Join search results to current authorized source records before producing snippets, titles, counts, facets, previews, or external AI context.
- Index/reindex by document ID and exact validated version; archival, replacement, deletion, and visibility changes update or invalidate affected entries.
- Keep staff-only complaint notes separate from resident-visible content.
- Bound query length, result count, execution time, and extraction size. Parameterize SQL and handle malformed FTS syntax without exposing database errors.
- Test tokenization against the society's actual languages and scripts. Do not assume English stemming or default Unicode token boundaries give adequate local-language results.
- Record extraction method/version and source-page references so search/AI results can cite the original.

FTS5 avoids an additional search service at this scale. Verify driver/build support before Phase 7.

---

# 19. Text Extraction and OCR

OCR is optional Phase 7 work. Extract existing text from supported PDFs first; only scanned/image pages need OCR. Original uploaded versions remain intact in S3.

```text
AVAILABLE document/version -> extraction job
-> bounded temporary download -> text extraction / local OCR as needed
-> extracted text + page references + method/version in SQLite
-> authorized FTS5 indexing -> temporary-file cleanup
```

Record `NOT_REQUESTED`, `QUEUED`, `RUNNING`, `SUCCEEDED`, `FAILED`, or `UNSUPPORTED` processing status separately from document availability. A valid original document remains downloadable if OCR fails.

Use one resource-limited worker initially, with page/pixel/file limits, timeout, disk allowance, parser isolation, patched OS tools, and approved language packs. Test accuracy on actual local-language scans. Do not run a dedicated OCR server or allow processing to starve login, billing, or backups.

Extraction may temporarily use the SSD despite originals living in S3. Clean up on success, failure, and process restart. Treat extracted text as equally sensitive as its source and include it in retention/deletion controls.

Cloud OCR is a separately approved later option with per-page costing and an explicit data-disclosure decision.

---

# 20. Background Jobs Without Redis

A SQLite-backed queue is sufficient. Treat execution as at least once and make handlers safe to retry; external side effects do not become exactly once merely because SQLite is transactional.

```text
jobs: id, type, payload_json, operation_key unique, status,
      attempts, max_attempts, scheduled_at, locked_by,
      lease_expires_at, started_at, completed_at,
      last_error_code, last_error_redacted, created_at
```

States: `QUEUED`, `RUNNING`, `RETRY_WAIT`, `SUCCEEDED`, `FAILED`, `CANCELLED`.

- Claim a job atomically in a short transaction; never hold the transaction during execution.
- Use leases/heartbeats and recover expired RUNNING jobs after crashes/reboot.
- Use bounded exponential backoff with jitter and a finite attempt limit, then require a visible operator decision.
- Create jobs in the same transaction as the source business change: receipt record + audit + unique PDF job, for example.
- Bind operation keys to immutable source/version identities. Repeated receipt jobs reuse the same number, snapshot, and object identity.
- Limit worker concurrency initially to one document/PDF-processing job and a small separately bounded lightweight queue; tune after measuring the selected hardware.
- Persist external object/provider references before reporting completion. Reconcile ambiguous outcomes before retrying consequential work.
- Record email notification intent and delivery/provider references separately. An ambiguous provider timeout can cause a duplicate email; handle it explicitly rather than claiming guaranteed exactly-once delivery.
- Pause outbound/reposting jobs during restore until an operator reconciles already completed external effects.

Initial application-queue types: `VALIDATE_DOCUMENT`, `GENERATE_INVOICE_PDF`, `GENERATE_RECEIPT_PDF`, `GENERATE_EXPORT`, `SEND_EMAIL`, `CLEANUP_UPLOADS`. Scheduled backup runs through the separate timer/CLI with its narrower credentials, and records backup status for the dashboard. Later application jobs include extraction/OCR and optional reminders.

The application can run workers as goroutines. A systemd timer invokes a narrow backup command independently of the HTTP process, so an application hang does not suppress backup scheduling. Both processes use the fixed SQLite engine and the controlled connection/backup policy.

---

# 21. Backups

## 21.1 Recovery objectives and initial schedule

Initial target for committee approval:

- **RPO:** latest usable off-site SQLite snapshot no more than 1 hour old during normal connected operation. This is a bounded recovery gap, not a zero-data-loss promise.
- **RTO:** service restored within one working day after a replacement machine, custodians, and credentials are available. Procurement delay is additional and must be planned separately.
- Snapshot every 30 minutes; also snapshot after migration, before deployment/migration, and after an approved collection/reconciliation batch.
- Warn at 45 minutes since the last usable off-site snapshot; escalate at 60 minutes and pause new financial postings by default until backup protection is restored. Read-only views and drafts may continue. Any temporary override is an audited committee decision accepting increased recovery exposure.

Extended connectivity/provider outages can breach the target. Display degraded backup status and keep an explicit reconciliation procedure; do not claim a guarantee that the architecture cannot provide.

## 21.2 Consistent SQLite snapshot

Use the driver's SQLite online backup API or a proven `VACUUM INTO` path to create a consistent live snapshot. Do not copy only the live `.db` file while WAL writes continue.

```text
Live SQLite -> consistent snapshot -> integrity/foreign-key checks
-> compress -> client-side encrypt -> checksum + manifest
-> unique S3 backup key -> verify uploaded checksum/metadata
-> mark off-site snapshot usable
```

The manifest records snapshot time, schema/application version, engine version, original and encrypted-object checksums, key identifier, and reconciliation/numbering checkpoints. The write-only backup command supplies an S3-validated upload checksum and records the successful PUT response/version identity; it does not require permission to read historical snapshots. Ambiguous upload outcomes remain unconfirmed and are retried safely. Custodial restore checks independently download, verify, and decrypt the object. Encrypt with a documented recoverable format; keep decryption material with two authorized custodians, separate from the backup objects. Test decrypt/restore with custodial credentials rather than depending on the running server's environment.

Reference: [SQLite backup API and VACUUM INTO](https://www.sqlite.org/backup.html).

## 21.3 Retention and storage control

Initial retention:

| Snapshot class | Retention |
|---|---|
| 30-minute snapshots | 7 days |
| Daily checkpoint | 35 days |
| Monthly checkpoint | 12 months |
| Pre-migration/release checkpoint | 90 days, or longer while required for a recorded incident |

Monthly snapshots beyond 12 months require a defined operational/legal need; retain original financial documents according to their separate approved policy. Lifecycle rules are prefix/tag scoped and include noncurrent versions where necessary. Backup keys are unique rather than repeatedly overwriting one name. Expired cleanup uses custodial/lifecycle permissions, not application version-deletion permissions.

Forecast actual compressed snapshot size and retained count. An uncompressed 100 MB snapshot every hour without retention adds approximately 876 GB/year; a small live database alone does not imply a small backup bill.

## 21.4 Documents, configuration, and offline copies

S3 versioning protects against ordinary accidental overwrites/deletes but is not an independent backup against account compromise or privileged permanent deletion. Runtime credentials cannot delete versions or change bucket controls.

Keep reproducible source/releases, migrations, non-secret configuration, ingress setup, a credential-reissue procedure, and required encryption/MFA keys recoverable independently of the local server. Session secrets may be regenerated and all sessions invalidated; encryption keys for existing protected data must be recoverable.

Monthly, copy a verified encrypted database snapshot and all currently retained society documents to an encrypted offline drive stored separately. During migration/large document batches make an additional copy. If full-document copying is impractical, explicitly approve the narrower scope and resulting exposure; do not claim all documents are independently backed up.

## 21.5 Restore exercises

Perform a clean-environment restore in Phase 0, before launch, quarterly, and after meaningful backup/schema changes. Verify decryption, integrity, foreign keys, representative records, financial balances, document/version links, and the numbering/job/revocation reconciliation steps in Section 22.

A successfully uploaded file is a backup candidate; only an actual restore exercise validates the recovery procedure.

---

# 22. Disaster Recovery

Treat the local machine as replaceable. Periodic snapshots can lose changes after the last usable backup; retained entry sources and receipt evidence may permit reconstruction, but they are not a universal zero-loss guarantee.

## 22.1 Recovery runbook

1. Assign an incident owner, stop public writes, preserve any readable failed-disk evidence, and record the last known backup/reconciliation time.
2. Obtain replacement hardware and install the documented supported Linux setup. Restore the approved release and verify its checksum/version.
3. Recover/reissue restricted AWS and ingress credentials; recover required encryption keys from a custodian.
4. Retrieve and decrypt the newest verified snapshot. Run SQLite integrity and foreign-key checks and verify schema/application compatibility before migrations.
5. Start in `RESTORE_MODE`: public writes, financial posting, notifications and background side effects remain paused.
6. Reconcile posted manual entries against their source/evidence, recent receipt snapshots and surviving records. Identify lost or duplicated postings explicitly. Tally integration evidence is relevant only if a future integration exists.
7. Reconcile missing/orphan S3 documents and exact version links. Restore archived metadata where retention policy requires it; never expose all bucket contents as a recovery shortcut.
8. Invalidate all sessions and outstanding recovery tokens. Recheck current administrators, committee terms, disabled users, and memberships against the current approved register before enabling logins.
9. Reconcile receipt sequences and reserve a fresh recovery numbering series as described in Section 50. Never restart issuing from an older counter silently. Future invoice numbering follows the same controls.
10. Review expired job leases and external effects. Regenerate missing PDFs from retained snapshots; do not automatically resend old notifications or duplicate prior exports.
11. Configure the stable ingress URL, verify representative authorized resident/treasurer views, create a new off-site checkpoint, and record reconciliation differences.
12. Obtain the authorized incident owner's release of restore mode, reopen writes, and communicate any remaining limitations through the society's established channels.

## 22.2 Preparedness and acceptance

Maintain a tested replacement-machine procedure, current runbook, two named custodians, and account/domain ownership by the society. Record where offline copies and decryption materials are held without placing usable secrets in the plan.

RPO/RTO are the targets in Section 21. Procurement, unavailable custodians, missing keys, S3/account outages, and incomplete recent evidence can extend downtime or leave a recovery gap. The restore exercise must measure elapsed recovery time and identify these dependencies.

---

# 23. Security Design

## 23.1 Network and process boundary

- The Go application listens on `127.0.0.1:8080` behind the selected ingress. Expose only the documented public HTTPS route and any deliberately enabled certificate/redirect port.
- With a tunnel, no inbound router forwarding is required. Apply both IPv4 and IPv6 firewall policy; protect the local office network as well.
- SSH uses keys and approved private administration access or an in-person LAN procedure. Public unrestricted SSH is not part of this design. Budget any selected private-access product separately.
- Trust forwarded host/protocol/IP headers only from the configured proxy; validate Host/Origin against configuration. Do not permit a public client to choose password-reset origins or defeat rate limiting with a spoofed IP header.
- Run as a dedicated unprivileged user, restrict database/configuration access, protect the physical machine, and disable unused public debug/admin endpoints.

## 23.2 Application controls

Implement server-side resource authorization, scoped list/export queries, parameterized SQL, input/size validation, CSRF protection, Secure/HttpOnly/SameSite session cookies, and bounded sessions/rate limits. Argon2id cost and concurrency are measured on the server.

Use a restrictive Content Security Policy appropriate to the frontend, safe Markdown rendering, correct MIME/Content-Disposition, and response cache controls. Never cache authenticated records at the public proxy. Secrets, passwords, reset tokens, session tokens, MFA seeds, signed S3 URLs, payment evidence, and unnecessary identity data are redacted from ordinary logs.

Bank/QR, role, MFA, and high-impact finance changes require fresh authentication and an audit event. Database constraints and atomic transactions enforce financial invariants in addition to application validation.

## 23.3 Privileged MFA and recovery

MFA is mandatory for administrators/treasurers and accounts with identity-administration privileges. Procedures, token storage, expiry, and offline recovery follow Section 11. Do not treat AWS account MFA as protection for a leaked long-lived application access key; those keys need their own narrow permissions and rotation/revocation procedure.

## 23.4 Files and processing

Apply the Section 9 validated-upload state machine, enforced size/quota policy, allowlisted formats, immutable/version-pinned identity, bounded parsing, and attachment serving. Checksums prove integrity; malware/content checks and parser isolation address separate threats.

All metadata, previews, extraction, downloads, and cleanup obey the same resource entitlement/retention model. Do not execute content or trust user filenames as storage identities.

## 23.5 AWS and secrets

Use dedicated IAM identities limited by action and prefix. Separate document use, backup writing, and custodial restore/cleanup permissions. Runtime credentials cannot permanently delete historical backups, suspend versioning, or change bucket policy. Never place root credentials on the machine.

Keep secrets out of source control, frontend builds, and support exports. Root-owned environment/credential files have restrictive permissions; encrypted data keys have independent custodial recovery. Review roles/credentials at handover and after compromise, rotating or revoking promptly.

## 23.6 Public failures and degraded mode

Use generic authorization/authentication errors without revealing private record existence. Rate limits/quota errors explain the actionable limit without disclosing other users' data. Mark S3, email, and backup degradation explicitly; do not repeatedly restart a healthy financial application because an optional provider is unavailable.

---

# 24. Audit Logging

```text
audit_log: id, actor_user_id nullable, actor_role_snapshot,
           action, entity_type, entity_id, before_redacted_json,
           after_redacted_json, reason nullable, request_id,
           session_reference_hash nullable, created_at
```

Record financial posting/correction, allocations/releases, receipt issuance/reversal, invoice issuance/cancellation, rule/bank-detail changes, migration approvals, role/membership/account changes, recovery overrides, document publication/deletion, and accounting export acknowledgement.

Insert audit events in the same transaction as the associated database change. Workers record result/failure events with the same operation identity. Do not permit normal UI/API editing/deletion of audit entries; restrict audit viewing and exports to approved officers/accountants.

Never serialize entire users/sessions/credentials or upload-link responses into before/after JSON. Use an explicit field allowlist, record ID/reference, and the minimum personal data required to explain the change. A session reference is not a usable cookie/token.

Keep audit history in verified off-site backups and export periodic audit/reconciliation checkpoints using restricted storage. An SQLite log remains alterable by a sufficiently privileged machine/database operator; do not advertise cryptographic tamper-proofing without a separately implemented and verified scheme.

Archive and retain logs under the approved legal/operational policy; only custodial retention processing may purge eligible entries.

---

# 25. Resident Portal

Initial navigation: Home, My Records, Receipts, Notices, Complaints, Documents, My Flat and Profile.

The dashboard shows authorized flats, balances derived from given entries, permitted recent notices, own open complaints and receipt/PDF availability. A resident with multiple flats selects the intended flat explicitly before viewing records or submitting a complaint.

Distinguish unconfirmed/manual entries, confirmed received-money records and `receipt PDF pending/available/failed`. Residents can view permitted records; no payment-initiation or payment-claim flow is included in the current portal.

Historical invoice/receipt/documents follow Section 10 entitlements. New occupants do not inherit unrestricted personal records from earlier occupants. Profile contact changes do not silently transfer login identities or financial roles.

Provide assisted recovery help, accessible mobile forms, predictable error messages, and visible offline/degraded status. Residents can read their canonical records without AI, email, or automated WhatsApp.

---

# 26. Committee Portal

Navigation is permission-scoped: Dashboard, Residents/Flats, Add Entries, Records, Receipts, Notices, Complaints, Documents, Reports, Audit Log and Settings.

Committee membership alone does not authorize treasury, identity administration, or unrestricted resident records. The server enforces permissions independently of menu visibility.

Finance screens cover given charges/opening balances, recorded already-paid entries, confirmation, derived balances, generated receipts and linked corrections. Show the source, recorded actor, posting date and effective/received date where they differ. Automated billing, bank matching, allocations and Tally batch screens are future work.

Document/PDF/job states, backup age, restore mode, storage/quota usage, ingress status, and failed jobs are visible to the relevant operators. Financial actions require clear confirmation and display their immutable resulting reference; repeated clicks reuse the operation identity rather than duplicate a payment.

The audit view explains who changed what and why using redacted data. Current settings include audiences/categories, access terms and approved receipt configuration; privileged changes require MFA reauthentication. Billing-rule and bank/QR controls apply only to separately approved future features.

---

# 27. Reports and Reconciliation

Current reports cover supplied charges/opening balances, recorded received-money entries, derived flat balances, linked corrections, unconfirmed records and receipt/PDF availability. Totals must match the posted manual records and receipt sources. Registry, complaint and document reports below remain current scope. Invoice allocations, bank matching, automated billing and Tally reconciliation definitions are future references.

## 27.1 Current financial definitions

Every report states its period/as-of date, society time zone, effective/received-date basis and treatment of drafts, corrections and opening balances.

- **Given charges:** posted manually supplied charge amounts in the period, with corrections distinguished. No rate/interest calculation is implied.
- **Recorded received money:** posted amounts the authorized operator confirms were already received, with linked corrections distinguished. Drafts and opening credits are not new received money.
- **Flat balance:** sum of posted account deltas at the as-of date, showing debit outstanding and net credit separately.
- **Unconfirmed records:** drafts awaiting the authorized posting/confirmation action; excluded from posted balances and received-money totals.
- **Receipt register:** issued and corrected/void records with immutable number, received-money source entry and PDF state. Imported legacy receipts retain historical provenance and do not count as a second received amount.

Reports reconcile posted entry effects, derived balances and receipt sources with the approved given records. Backdated entries and corrections are visible; distinguish effective/received date from posting timestamp. Aging, per-invoice outstanding/allocations, automated bank reconciliation and Tally batch reports are future additions requiring approved inputs and definitions.

## 27.2 Initial reports

Financial: manual-entry register, per-flat balance/statement, recorded received amounts and corrections, unconfirmed-entry list, receipt register/PDF availability and reconciliation against given source records.

Registry: authorized flat/owner/tenant/vacancy lists. A resident directory is not automatically public to all residents.

Complaints: open cases, age, category, assignment, and permitted status history; exclude staff-only content from resident reports.

Documents: authorized category/recent lists and structured contract/AMC expiry dates when captured. Full OCR/AI is not required for these filters.

## 27.3 Export controls

Apply the same server-side entitlement rules to CSV/export jobs as to interactive views, including historical personal-data restrictions. Snapshot export inputs/version and use bounded pagination/streaming or a job for larger outputs.

Use proper quoting and spreadsheet formula-injection protection for human-readable CSV cells beginning with formula-triggering characters. Preserve exact amounts and source identities. If Tally integration is requested later, keep its machine-import formats separate and validate schema/escaping; downloading an export does not imply successful import.

---

# 28. PWA Instead of Native Mobile Apps

Use one responsive web application; native Android/iOS apps and app-store distribution are out of V1 scope.

## Install and browser support

Provide a web manifest, appropriate icons, stable HTTPS origin, and tested Add to Home Screen instructions. Test the agreed Android Chrome and iOS Safari versions plus desktop browsers; browser-specific installation behavior must be reflected in the help text.

Use accessible labels, focus handling, readable type, and clear form errors. Validate ordinary mobile connectivity and slow-network behavior with representative residents.

## Cache and update contract

- Cache versioned public static assets and a minimal offline shell only by default.
- Do not persist invoices, receipts, private documents, authenticated API responses, payment evidence, or presigned URLs in service-worker caches. Any later offline personal-data feature requires an explicit design change.
- Clear application state and any approved user-specific caches on logout/account change. Browser-downloaded files already saved by the user cannot be remotely erased.
- Display offline/degraded status and do not queue offline financial writes. Show confirmed server results rather than implying that an unacknowledged payment mutation succeeded.
- Version frontend/API contracts; detect a stale client and offer a safe reload. Do not interrupt a financial form or create duplicate submissions during an update.
- New service workers may wait for activation. Define activation/reload and old-asset retention rather than promising instant updates for every client.

Reference: [PWA service-worker updates](https://web.dev/learn/pwa/update).

---

# 29. Email

Email remains optional. Core login, assisted account recovery, financial posting, receipts, and notices work without an email provider.

If enabled, use SES or another deliberately selected sender; do not run a local mail server. SES needs verified sending identities and production access in the chosen region to send to arbitrary resident recipients. While in the sandbox, recipient verification and sending restrictions apply.

Before enabling:

- Verify the chosen sender/domain, configure supported authentication such as DKIM and the applicable SPF/DMARC arrangement, and test deliverability. Do not assume a no-domain ingress hostname is a usable sender domain.
- Obtain production access and suitable quotas; configure bounce/complaint handling, suppression, and an authorized reply/help address.
- Queue password resets, receipt-availability notices, and optional notices with a clear delivery status. Never expose another resident's address in a bulk recipient list.
- Use portal links, short-lived reset tokens, and approved minimum content; avoid sending private documents or permanent presigned URLs by default.
- Preserve administrator-assisted recovery during SES/outbound internet failure. MFA recovery still follows the stricter Section 11 procedure.
- Set volume/cost limits and reconcile ambiguous delivery retries as described in Section 20.

Administrative outage alerts need an approved external route that still works when the local server is down; local SES jobs alone cannot provide that.

References: [SES production access](https://docs.aws.amazon.com/ses/latest/dg/request-production-access.html), [verified identities](https://docs.aws.amazon.com/ses/latest/dg/verify-addresses-and-domains.html).

---

# 30. WhatsApp Roadmap

## Phase 1 — Manual sharing

Use committee-initiated prefilled share links with the canonical portal URL. API cost is zero; delivery/read status remains outside the application. Do not send private financial data in a generic society group or bypass portal permissions with public attachments.

## Later — Optional automation

Only after demonstrated value, verify current Meta WhatsApp Cloud API onboarding, phone-number requirements, recipient permission/opt-out rules, message-template/session restrictions, current country/category pricing, webhook security, and a complete annual forecast.

Possible approved outputs: maintenance reminders, payment confirmations, receipt-availability links, and notices. Require an authenticated event source, permission-safe recipient selection, per-event delivery identities, retry reconciliation, and spending/volume limits.

WhatsApp is an output channel. The portal retains canonical records, and urgent communication has an established fallback. Automated API prerequisites/prices are not represented as verified launch dependencies in this plan.

---

# 31. AI Roadmap

AI is optional Phase 8 work after core operations and permission-aware search are stable. It has its own budget and external-data-disclosure decision.

## Phase A — Search first

Use structured filters and, when justified, FTS5 plus extracted/OCR text. Contract expiry, balances, and complaint ages come from structured fields, not inferred text.

## Phase B — Document Q&A

```text
Authenticated question -> current entitlement filter
-> authorized document/version chunks with page references
-> approved external LLM API -> answer with source citations
```

Support questions such as AGM parking decisions or summaries of approved meeting documents. Bound retrieved context and costs, cite exact authorized versions/pages, and state when available evidence is insufficient. No vector database is needed initially.

Test retrieval and answers on actual authorized/unauthorized document cases before enabling residents. A source citation must itself remain accessible only to the authorized reader.

## Phase C — Structured read-only assistant

Use typed application functions for outstanding balances, collections, and complaint ages. Each tool rechecks authorization, date basis, argument limits, and permitted fields. Return exact computed amounts/dates rather than asking the model to calculate authoritative balances or generate unrestricted SQL.

Initial AI scope is read-only. Any later write workflow is a separate approved product/design change with deterministic validation and an explicit human confirmation path.

---

# 32. AI Security

Apply normal authorization before retrieval and again in every application tool. Do not let a model-supplied resident/flat ID choose another person's data. Apply the same policy to snippets, counts, citations, caches, and trace/log content.

Treat document text and user questions as untrusted data; instructions embedded in uploads cannot change permissions, system behavior, or allowed tools. Separate retrieved content from operating instructions and test prompt-injection cases.

AI cannot autonomously waive charges, issue/cancel invoices or receipts, verify/modify payments, refund money, change memberships/roles, or delete records. Authoritative amounts and state come from deterministic application functions.

Before any external API use, approve provider retention/training settings, regional/data-transfer implications, and the categories of data permitted to leave the society. Minimize/redact personal data, retain only necessary logs, and keep credentials server-side.

Set per-user/society request and cost limits, timeouts, and a disable switch. A failed or unavailable AI provider cannot block core workflows. Version prompts/retrieval behavior and evaluate groundedness, citations, authorization, injection resistance, and cost using representative cases.

---

# 33. Deployment Layout

```text
/opt/society/releases/<release-id>/
    society-server
    release-manifest.json
/opt/society/current -> approved release directory

/var/lib/society/
    society.db
    tmp/
    backup-staging/

/etc/society/
    society.env
    documents-credentials.env
    backup-credentials.env
    ingress configuration / credential references
```

Run under dedicated unprivileged service identities. Program/releases are read-only to the application user; state directories have restrictive ownership/modes, and environment/credential files are root-owned and readable only through the approved service setup. Protect database/WAL files and temporary processing data as personal/financial records.

Frontend/fonts/templates are packaged with the release. Source and reproducible release artifacts are held in a society-controlled remote repository/archive, with no secrets. Keep the recovery runbook, non-secret configuration, deployment commands, and supported hardware/OS requirements there.

Bound staging/temp/log growth. Remove abandoned temporary files safely after restart. Restrict application access to the paths and credentials required by its operation; independently scheduled backup uses its narrower credential set.

---

# 34. systemd Services and Releases

Conceptual application unit, to be verified on the selected Linux distribution:

```ini
[Unit]
Description=Society Portal
Wants=network-online.target
After=network-online.target

[Service]
User=society
Group=society
WorkingDirectory=/var/lib/society
StateDirectory=society
StateDirectoryMode=0700
UMask=0077
EnvironmentFile=/etc/society/society.env
EnvironmentFile=/etc/society/documents-credentials.env
ExecStart=/opt/society/current/society-server serve
Restart=on-failure
RestartSec=5
TimeoutStopSec=30
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/var/lib/society

[Install]
WantedBy=multi-user.target
```

Verify writable directory creation, SQLite WAL operation, temp processing, font access, and required network calls under the hardening policy. The server handles SIGTERM with bounded HTTP draining and worker cancellation; unfinished job leases recover on restart. `After=network-online.target` does not guarantee that internet/S3 is reachable.

Install separate persistent ingress and backup services. A systemd timer runs `society-server backup` every 30 minutes with the backup credential file, skips overlapping backup runs, and records success/failure. The CLI is a required implementation feature, not an assumed external command already present.

The backup command opens SQLite using the fixed engine and safe snapshot API; it does not copy a live database file. Scheduled backup continues independently of a hung HTTP worker. Off-host monitoring detects whole-machine failure.

## Release and migration procedure

Build/test a versioned release, verify its checksum, create a verified pre-migration checkpoint, and run a supported migration under controlled write access. Record schema/application versions, switch the current release atomically, and smoke-test authorized workflows before general use.

Do not assume an older binary can use a newly migrated schema. Prefer a compatible forward fix; any database rollback follows the restore/reconciliation procedure and its RPO consequences. Plan database changes for temporary coexistence with stale PWA clients. Keep the previous release and source available.

---

# 35. Configuration

Example non-secret settings:

```text
APP_ENV=production
APP_LISTEN=127.0.0.1:8080
DATABASE_PATH=/var/lib/society/society.db
AWS_REGION=ap-south-1
AWS_S3_BUCKET=...
PUBLIC_BASE_URL=https://<selected-stable-hostname>
ALLOWED_HOSTS=<selected-stable-hostname>
TRUSTED_PROXY_CONFIGURATION=<selected-ingress-only>
SOCIETY_TIMEZONE=Asia/Kolkata
MAX_RESIDENT_UPLOAD_BYTES=20971520
PRESIGNED_URL_TTL_SECONDS=120
BACKUP_WARN_AFTER_MINUTES=45
BACKUP_PAUSE_FINANCIAL_WRITES_AFTER_MINUTES=60
RESTORE_MODE=false
TMPDIR=/var/lib/society/tmp
```

Also configure approved financial-year boundaries, record/session policies, quotas, retention rules, sender settings if enabled, and the external alert route. Values are validated at startup; mandatory secret or security-setting failures produce safe, redacted diagnostics.

Separate secret settings include the document/backup IAM credentials, session protection material where required, and recoverable encryption keys for TOTP/protected data. Prefer opaque database-backed sessions; signing/encryption secrets are not interchangeable. Record key identifiers and rotation/recovery procedures without embedding usable secrets in this plan.

Never commit environment/credential files, decrypted backups, or production resident data. Provide `.env.example` with placeholders only. Origins, proxy trust, cookies, CORS, and links are reviewed together whenever ingress changes.

---

# 36. Repository Structure

```text
society-os/
├── cmd/society-server/          # serve, backup, migrate, restore-check commands
├── internal/
│   ├── auth/
│   ├── authorization/
│   ├── residents/
│   ├── flats/
│   ├── accounts/               # append-only effects of posted manual entries
│   ├── manualentries/
│   ├── receipts/
│   ├── notices/
│   ├── complaints/
│   ├── documents/
│   ├── reports/
│   ├── audit/
│   ├── jobs/
│   ├── backup/
│   ├── storage/
│   └── config/
├── migrations/
├── web/src/
├── templates/                  # immutable-version PDF templates/fonts
├── scripts/                    # narrowly scoped build/deploy/check tools
├── docs/                       # API contract, runbooks, decisions, mapping samples
├── testdata/                   # synthetic/redacted fixtures only
└── README.md
```

Keep one modular application and explicit SQL/query ownership. Record the SQLite driver/engine version, supported OS/architecture, FTS5 build support, PDF/font dependencies, and optional OCR tooling. CI builds and checks must not require production credentials or datasets.

Create modules as their current workflows are implemented. Billing automation, bank/claim/allocation workflows and Tally modules are future additions.

---

# 37. API Outline and Contracts

## 37.1 Common contract

Maintain a versioned OpenAPI or equivalent checked API contract. Define request/response fields, validation, permissions/resource scope, stable error codes, pagination, date/paise formats, and mutation state transitions before each module is implemented.

All financial mutations accept `Idempotency-Key`. Persist an actor/operation-scoped request identity and payload hash in the same transaction as the mutation. The same key/body reproduces the original result; a changed body conflicts. Recheck current authorization before returning a cached result. Database uniqueness also protects source financial events beyond the HTTP retry window.

Use record versions/If-Match or equivalent for concurrent mutable registry/notice/complaint edits. Accept and validate given manual amounts; calculate balances and posting effects on the server. Do not permit client-supplied roles, derived totals, confirmation status, actor IDs, S3 keys, or ledger entries to bypass authorization and state transitions. Bound page/query/export size and use a consistent UTC/event-date convention.

Return distinguishable retry/conflict/validation errors without exposing private record existence. Authorization/signing/financial responses are private/no-store. Document restore-mode and backup-protection errors as actionable operating states.

## 37.2 Authentication and administration

```text
POST /api/auth/login
POST /api/auth/logout
POST /api/auth/change-password
POST /api/auth/password-reset/request
POST /api/auth/password-reset/complete
POST /api/auth/invitations/accept
POST /api/auth/mfa/enroll
POST /api/auth/mfa/confirm
POST /api/auth/mfa/recovery
GET  /api/me
POST /api/admin/invitations
POST /api/admin/users/{id}/recovery
POST /api/admin/users/{id}/disable
POST /api/admin/role-grants
POST /api/admin/role-grants/{id}/revoke
```

## 37.3 Registry

```text
GET/POST      /api/buildings
GET/POST      /api/flats
GET/PUT       /api/flats/{id}
GET/POST      /api/residents
GET/PUT       /api/residents/{id}
POST          /api/flat-memberships
POST          /api/flat-memberships/{id}/end
GET           /api/flats/{id}/statement
```

## 37.4 Manual entries and receipts

```text
GET/POST /api/manual-entries
GET      /api/manual-entries/{id}
PUT      /api/manual-entries/{id}/draft
POST     /api/manual-entries/{id}/post
POST     /api/manual-entries/{id}/correct
GET      /api/receipts
GET      /api/receipts/{id}
GET      /api/receipts/{id}/download
```

Posting is an authorized state transition. For a received-money entry it includes the operator's explicit confirmation and atomic receipt creation; for a given charge/opening balance it posts the balance effect without a received-money receipt. Draft editing never changes posted history. Corrections preserve sources and require a reason and linked compensating effects. Receipt download delegates to the authorized document/version path; PDF availability is separate from posting.

## 37.5 Notices, complaints, documents

```text
GET/POST /api/notices
GET/PUT  /api/notices/{id}
POST     /api/notices/{id}/publish
POST     /api/notices/{id}/archive
GET/POST /api/complaints
GET      /api/complaints/{id}
POST     /api/complaints/{id}/assign
POST     /api/complaints/{id}/transition
POST     /api/complaints/{id}/updates
GET      /api/documents
GET      /api/documents/{id}
POST     /api/documents/upload-request
POST     /api/documents/{id}/complete
GET      /api/documents/{id}/download
POST     /api/documents/{id}/archive
```

## 37.6 Reports, export, and operations

```text
GET  /api/reports/outstanding
GET  /api/reports/manual-records
GET  /api/reports/collections
GET  /api/reports/receipt-register
GET  /api/reports/reconciliation
GET  /api/admin/jobs
POST /api/admin/jobs/{id}/retry
GET  /api/admin/audit-events
GET  /health
GET  /ready
```

Financial reports describe the given charges, recorded received amounts, linked corrections and derived balances, with unconfirmed records identified separately. Retry permissions do not authorize changing the underlying frozen financial record. Monitoring exposes minimal public health information and authenticated operational details.

Future API additions for automated billing, resident claims, bank matching, allocations or Tally require separately approved contracts based on Sections 12–14. No current endpoint initiates payment.

---

# 38. Migrating Existing Data

Current migration covers the approved registry, selected documents and any given opening balances/manual records needed at launch. Existing Tally remains independent; no Tally import/export or complete historical accounting migration is required. Use reviewed source references and avoid recording the same received amount as both an opening credit and a new receipt.

## 38.1 Agreed cutoff and source register

The registry and finance owners approve the canonical register, source inventory and cutoff for any financial records being migrated. Back up the sources; importing into the portal must not alter the existing accounting system.

Choose a brief source-entry freeze or documented delta capture so records entered during migration are not omitted/doubled. Preserve opening balances as opening entries, without creating new received-money receipts.

## 38.2 Registry cleanup

Create 118 uniquely identified flats with building/code, status and the approved optional registry fields. Map owners, joint owners, tenants/family, contact preferences, and date-bounded memberships separately. Billing classifications are future inputs if automation is requested.

Deduplicate by reviewed identity relationships, not name/phone alone. Retain source row references, resolve shared contacts/recycled phone numbers, and verify invitations against actual active occupants. Administrator/treasurer grants are not inferred from an Excel column without approval.

## 38.3 Given opening balances and manual records

If opening balances are supplied, import signed per-flat amounts with cutoff date, source evidence and finance-owner approval. Preserve credit balances; do not make every opening total positive. Import selected manual entries with their given dates, source references and confirmed/unconfirmed distinction. Do not infer aging or allocation breakdowns from a net balance.

Use an idempotent migration batch and dry run. Compare per-flat and aggregate amounts and net balances to the approved source register. No unexplained difference is acceptable; record each resolved exception and the sign-off evidence. Historical receipts keep their original identities and do not automatically trigger a new system receipt.

## 38.4 Documents and launch migration

Import high-value registration/legal documents, AGM minutes, audits, active contracts/AMCs, and recent notices/receipts first. Use validated uploads, checksums, exact versions, category/subject visibility, source references, and applicable retention. Historic receipt import preserves original numbers/issuers and does not create new payments.

Run the same migration twice in a clean test environment to prove deduplication. Before production cutover, reapply the final approved delta, reconcile, revoke test accounts, create an off-site checkpoint, and confirm restore readiness. Older archives can be added gradually without blocking core launch.

---

# 39. Implementation Phases, Owners, and Acceptance

## 39.1 Delivery responsibility and forecast

Assign a technical maintainer/release owner, registry officer, authorized finance operator, committee sponsor, document/complaint handlers and two recovery custodians. At least two people must be able to recover society-controlled accounts and keys.

Use an AI coding agent to implement bounded workflows with repeatable checks, measured baselines and reviewable evidence. The user/domain owners supply priorities and feedback; finance expectations and receipt policy require the relevant business review. Routine local implementation can proceed within the agreed scope.

The earlier 9–18 person-week estimate included advanced billing, allocations and Tally integration. It is no longer the active estimate for this manual tracking scope. Reforecast from the first runnable local milestone, actual review effort and remaining tasks. Production hardware/state/Tally information can be supplied later; synthetic development can proceed now.

| Phase | Current deliverable | Acceptance partner |
|---|---|---|
| 0 | Runnable local foundation and production infrastructure proof | Maintainer and custodians |
| 1 | Identity and canonical resident registry | Registry officer and maintainer |
| 2 | Audit, operation identities, jobs, private version-pinned documents and PDF foundations | Maintainer and document/finance owners |
| 3 | Manual charge/already-paid entries, balances, receipts and auditable corrections | Authorized finance operator and committee |
| 4 | Notices, resident views and mobile/PWA behavior | Committee sponsor and representative users |
| 5 | Complaints, document library and authorized metadata search | Committee handlers and document owner |
| 6 | Approved migration, representative pilot, recovery and handover | All operational owners |

## Phase 0 — Local foundation and infrastructure proof

First deliver a reproducible Go/PWA build, actual fixed SQLite engine/driver, migrations, synthetic 118-flat registry, health/readiness and a consistent local snapshot/restore exercise. This local proof is not production infrastructure acceptance.

Then prove the selected Linux host, stable external HTTPS, private S3/IAM separation, encrypted off-site snapshots, independent alerts, reboot/power-return behavior and custodian-held clean recovery. Acceptance includes actual power/storage/ingress costs and a complete annual forecast below ₹12,000.

## Phase 1 — Identity and resident registry

Deliver verified invitations, sessions/recovery, privileged MFA, role terms, memberships, historical-access policy and the canonical 118-flat register.

Acceptance: approved person/flat relationships; joint/multiple-flat cases; resident/former-occupant/committee/finance/admin/auditor permissions across API and files; documented two-custodian recovery. Ordinary resident permissions do not include adding or confirming financial entries.

## Phase 2 — Shared foundations

Deliver transactional audit, operation identities, leased/idempotent jobs, validated/version-pinned S3 uploads/downloads and receipt PDF templates/fonts. Test actual names, rupee symbols and required languages.

Acceptance: unauthorized/substituted uploads never publish; jobs recover after crashes without duplicate records; PDF processing is separate from entry confirmation; private versions, quotas and resource limits work. No Tally compatibility spike is required for this phase.

## Phase 3 — Manual records and receipts

Implement Section 13.0: provided charges/opening balances, already-paid records, authorized confirmation, exact-paise amounts, derived balances, immutable numbered receipts, recoverable PDFs, reports and linked corrections. No gateway/transfer initiation, bank automation, recurring billing engine, resident claim workflow or Tally integration is included.

Acceptance: approved given-entry examples reconcile by flat and aggregate; repeated/racing saves issue one record/receipt; charge or unconfirmed entries do not produce received-money receipts; corrections preserve originals; S3/PDF failure leaves confirmed records intact; restore does not reuse receipt numbers or replay external effects.

## Phase 4 — Notices and resident experience

Deliver permission-scoped dashboards/records, notices/audiences, safe attachments, manual WhatsApp links, install/help instructions, accessible mobile flows and PWA cache/update behavior. Email is optional.

Acceptance: notice and attachment audiences agree; shared links enforce authorization; offline/stale clients do not duplicate entries or expose a different user's records.

## Phase 5 — Complaints and document library

Deliver complaint assignment/transitions/history, private staff notes/attachments, authorized document metadata filters and archival.

Acceptance: allowed state changes, private-content boundaries and revoked permissions hold across API/export/search/files. OCR/AI is not required for document availability.

## Phase 6 — Migration, pilot and production gate

Approve and dry-run registry, document and supplied-entry migration. Pilot with approximately 10–15 representative flats and authorized operators. Exercise activation/recovery, manual entries/receipts/corrections, notices, complaints, document access, role changes, backup/recovery and support. A complete automated billing/collection/Tally cycle is not a current requirement.

Acceptance: Section 52 current-scope gates pass, no unexplained manual-record/receipt differences or critical privacy/recovery defects remain, operating/support effort and cost are acceptable, and named officers receive tested runbooks. Expand to 118 flats after the pilot decision.

## Conditional future enhancements

Assess full-text extraction/OCR and read-only AI when justified. Payment initiation/gateways, bank automation, automated billing/interest, detailed allocations and Tally integration require separate demand, approved rules, budget and acceptance. Their retained design references do not create current launch dependencies.

---

# 40. Testing Plan

Tests should verify financial/security/recovery behavior, using synthetic or properly redacted fixtures and the actual release engine/driver. No production credentials or resident identity documents belong in test fixtures.

## 40.0 Current manual entry and receipt checks

Use independently reviewed given-entry examples and synthetic fixtures to check exact amounts, entry/source dates, balances, confirmation/MFA permissions, one record/receipt under retries/races, changed-payload rejection, charge/unconfirmed entry restrictions, immutable snapshots, linked corrections, PDF/S3 outages and receipt numbering/recovery. Keep unconfirmed assertions separate from received-money records. A test or receipt does not initiate payment.

## 40.1 Extended financial invariants for future automation

Use accountant-approved examples and invariant/property checks for:

- Exact-paise rounding, fractional area/rate, interest boundaries, leap/date/FY rollover, rule changes, eligible/vacant/exempt flats, and frozen historical invoices.
- Whole cycles and repeated/concurrent generation/issue with unique per-flat/cycle posting.
- Partial payments, one credit across several charges, several credits against one invoice, advances, imported credit balances, and opening-age-unknown aging.
- Duplicate claims, a resident claim matching a treasurer-entered payment, repeated/racing verification, same request key with a changed payload, and one bank credit funding at most one active payment.
- Rejection, cleared/bounced cheques, payment reversal, allocation release/reallocation, cancellation/adjustment, manually evidenced refunds, and wrong-flat corrections.
- Conservation: sum of ledger deltas equals each reported flat balance; active allocations never exceed eligible capacities; receipt money reconciles to posted payment records and bank/cash evidence; accounting voucher legs balance under the approved mapping.
- Historical legacy numbering, FY rollover, PDF-job retries, and fresh recovery series after restore.

## 40.2 Authorization and privacy

Test every relevant list/detail/export/search/attachment/signing endpoint, not only guessed opaque IDs. Cover current owner, tenant with/without finance grant, family member, joint owner, multi-flat owner, departed occupant, committee term expiry, treasurer, administrator without finance permissions, and time-limited auditor.

Verify private staff notes, former-payer receipts, resident-specific documents, counts/snippets, and old versions remain restricted. Expired/revoked sessions cannot obtain new links or cached financial mutation results. Existing signed URLs have the documented bounded bearer-token semantics.

Test invitation/reset/MFA recovery, identifier reuse, revoked tokens, session invalidation, CSRF/Origin handling, trusted-proxy behavior, XSS-safe notice content, and CSV formula escaping.

## 40.3 Upload/PWA/jobs

Test wrong checksum, actual size beyond declared size, incorrect MIME/content, forbidden formats, huge images/pages, expired/reused upload URLs, replacement after validation, abandoned uploads, quota reservation/release, and complete calls by the wrong user.

Test app crash between job claim/external upload/local completion, expired leases, permanent failures, ambiguous email outcomes, and worker resource limits. Test logout/account switch, offline behavior, service-worker updates, old frontend/new API compatibility, and duplicate-submit handling.

## 40.4 Recovery and failure drills

Restore a real encrypted snapshot into a clean environment using custodian-held material. Verify integrity/foreign keys, schema/release compatibility, ledger totals, exact document versions, recent-event reconciliation, revoked-account review, new numbering series, and paused/reconciled jobs.

Simulate power/internet loss, full disk, failed snapshot/decryption, application hang/crash, S3 downtime, invalid/expired credentials, router reboot/address changes, certificate renewal, and ingress restart. Test backup-warning/write-pause behavior and the audited override path.

Run a modest concurrent-use workload on selected hardware, initially 20 active sessions with realistic reads/financial submissions, and record latency/errors/resource usage. Agree measured acceptance thresholds after Phase 0 rather than asserting an untested SLA. Browser QA follows the machine's applicable execution/sandbox instructions.

## 40.5 Conditional future Tally evidence

If Tally integration is requested later, prove its export/import on the exact installed version/company copy, including repeats and corrections. It is not a V1 launch gate. Current launch evidence comes from manual-entry/receipt expectations, permissions, migration and restore checks; rerun affected checks after material changes.

---

# 41. Monitoring and Alerts

Keep monitoring lightweight, with an independently reachable alert route and a named recipient/backup recipient. A dashboard on the failed host cannot notify its own failure.

- `/health` reports minimal process liveness and no secrets.
- `/ready` reports ability to serve the core API/SQLite. Optional email/AI failure does not make the whole application unhealthy; expose protected degraded-state details separately.
- An off-host checker monitors the production HTTPS route and a protected minimal backup/operations heartbeat every 5 minutes initially. Select an approved free/within-budget service or an independent existing machine; verify that it can alert when this server/internet is down.
- Record the chosen external monitor, notification channel, recipients, interval, and fees before launch. Do not rely only on notification jobs running on the local server.

Track free disk/SSD health, WAL/temp/log growth, memory, last usable off-site snapshot, failed/dead-letter jobs, pending PDFs/uploads, S3 failures, credential/certificate expiry, suspicious authentication activity, and actual/forecast infrastructure spend.

Initial thresholds: warn below 20% disk free or below 5 GB; escalate below 10% or below 2 GB and prevent unsafe writes/large jobs. Backup warning at 45 minutes and finance-write pause at 60 minutes follow Section 21. Receipt PDFs pending more than 30 minutes require operator review; provider outage status must distinguish them from missing posted payments.

Measure and adjust thresholds against real disk/job sizes. Record alert acknowledgement, incident ownership, and resolution; verify the alert channel during the pilot and quarterly.

---

# 42. Operating Procedures and Handover

## During manual entry activity

The authorized finance operator compares supplied records with posted entries, balances, corrections and the receipt register. Review pending/failed PDFs and resolve differences against the stated source/evidence. Corrections retain reasons, links and audit. Unconfirmed records remain distinct from recorded received money; the portal performs no automatic bank verification.

## Weekly

Review failed jobs, pending PDFs/uploads, backup freshness, alerts, open complaints, and storage/quota use. Check the appointed maintenance/finance backup person is reachable.

## Monthly

Review manual-record/receipt totals and corrections; review actual/forecast infrastructure bills including storage versions/backups; make the offline full-document/snapshot copy; check disk/SSD health, accounts approaching term expiry, and failed delivery/cleanup jobs. Existing formal accounting continues separately.

Apply routine OS/application updates through the tested release procedure. Enable compatible automatic security updates; urgent security fixes receive prompt triage/testing/deployment rather than waiting for the monthly meeting. Record reboot windows and verify ingress/backup startup afterward.

## Quarterly and after material changes

Perform the clean-environment restore exercise, external alert check, permission review, custodian key-recovery check, and runbook update. Revoke departed users/roles promptly when relationships change; do not defer that to a quarterly review. Rotate/reissue credentials when required by compromise, policy, or handover.

## Committee/maintainer handover

Transfer society-owned AWS/domain/repository/ingress accounts, contact routes, named permissions, and custody records. New officers demonstrate login/MFA recovery, manual-entry/receipt reconciliation, backup verification, and restore-mode operation. Revoke superseded grants/sessions and personal access. Record support/volunteer hours so the operating model remains sustainable.

---

# 43. Cost Model

## 43.1 Budget scope

The ceiling is **annual recurring infrastructure below ₹12,000**, including applicable taxes and selected ingress/domain/alert services. Existing broadband is zero incremental cost only after confirming that a suitable connection is already funded. Development time, hardware/UPS purchase, occasional replacement, and separately agreed maintenance labor are tracked outside this infrastructure figure; they are not assumed free.

## 43.2 Current priced inputs and illustrative subtotals

AWS's Mumbai S3 price list retrieved during review, published 28 September 2026, lists S3 Standard's first storage tier at $0.025/GB-month, ordinary PUT/COPY/POST/LIST at $0.005/1,000 requests, and GET/other ordinary Tier 2 requests at $0.0004/1,000. Recheck region, tiers, and applicable transfer/free allowances when deploying; promotional credits are not a permanent budget assumption.

The electricity arithmetic in Section 6 is correct. Illustrations below use ₹10/kWh and **an assumed planning exchange rate of ₹100/USD**, not a quoted current FX rate:

| Scenario | Annual electricity | Annual S3 storage | Subtotal before other costs/tax |
|---|---:|---:|---:|
| 20 W incremental draw; average billed storage 20 GB | ₹1,752 | ₹600 | ₹2,352 |
| 30 W incremental draw; average billed storage 100 GB | ₹2,628 | ₹3,000 | ₹5,628 |

These are scenarios, not a completed society quote. Average billed storage includes originals, retained old versions, backups, exports, and processing copies. Measure actual compressed snapshots and use the Section 21 retained-count model. Include growth over the year rather than charging only the final live database size.

Tailscale Standard currently lists $8/user/month. At the same illustrative exchange rate, one paid seat would be ₹9,600/year before tax; organizational eligibility/operator requirements can therefore change the economics materially. Public Funnel visitors do not automatically require seats. Obtain the actual applicable quote instead of assuming either a free organizational plan or 118 paid seats.

References: [AWS Mumbai regional price list](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonS3/current/ap-south-1/index.json), [S3 pricing categories](https://aws.amazon.com/s3/pricing/), [Tailscale plans](https://tailscale.com/pricing).

## 43.3 Required forecast worksheet

Before production selection, record units, measured/quoted value, source/date, tax basis, and annual total for:

1. Incremental server/UPS/new-network-device wall power and actual electricity tariff.
2. Average S3 originals + versions + retained backups + exports; PUT/GET/list volumes; applicable internet/cross-region transfer.
3. Actual domain **renewal**, paid ingress/operator seats, and any required external monitoring route.
4. Any incremental internet/public-IP service selected; none is presumed mandatory.
5. Optional SES/WhatsApp/AI limits and separately funded forecasts if enabled.
6. Applicable taxes, FX/payment conversion assumptions, and contingency.

Aim for a base forecast at or below ₹9,000 with at least ₹2,000 contingency, while the full forecast remains below ₹12,000. If it does not fit, change ingress/service choices or scope before launch rather than silently omitting a required cost. Free service terms and renewals are checked periodically.

---

# 44. Budget Guardrails

```text
Approved annual recurring infrastructure forecast < ₹12,000
```

Any service above roughly ₹250/month triggers explicit committee cost review, but smaller subscriptions must also be included in the aggregate forecast. Quote annual cost including applicable taxes, renewal prices, seats, request/storage/transfer usage, and foreign-currency assumptions.

Use per-user/society upload quotas and optional email/AI/WhatsApp volume/spend limits. Monitor actual and forecast AWS spend with alerts at agreed thresholds, initially 50%, 80%, and 100% of the allocated AWS budget. AWS Budgets notifications can lag incurred charges and do not create a guaranteed hard spending cap; app quotas and restricted IAM/resource access address different controls.

Do not automatically delete legal documents, stop backups, or disable core recovery to meet a bill target. Reduce optional features, investigate abuse/version growth, and escalate a forecast breach early. Keep invoices and the complete budget worksheet with the monthly operating review.

Before adding a service, record its purpose, expected usage, annual all-in cost, removal/migration path, accountable owner, and effect on the existing ceiling.

Reference: [AWS Budgets update/notification limitations](https://docs.aws.amazon.com/cost-management/latest/userguide/budgets-managing-costs.html).

---

# 45. Recording Money Already Received

The society receives money outside the portal. An authorized operator records the given amount, flat/payer, date, method/reference and source/evidence, then confirms a received-money entry under the approved manual policy. The portal generates its immutable receipt and derives the balance; it never initiates a transfer.

No gateway, bank API, resident payment-claim flow or automated bank matching is required. A supplied reference or screenshot alone is not automatic bank verification. An unconfirmed entry stays separate from a confirmed received-money record.

Review entry and receipt examples with the finance owner before acceptance. The receipt fields and wording must represent the operator's recorded confirmation accurately.

Payment execution, bank import/matching and resident claims are conditional future work. If requested, review their requirements, controls, external service costs and acceptance separately.

---

# 46. Availability and Recovery Expectations

This portal supports routine society administration. Emergency/security reporting retains a separate established contact route.

- Short outages are acceptable under a recorded support/communication procedure.
- A local power/internet/SSD failure can stop the portal, including new authorization links, even while S3 itself is available.
- S3 objects alone do not reconstruct users, permissions, manual entries, receipts or their metadata; usable database snapshots and independent keys/runbooks are essential.
- The initial connected-operation recovery targets are a 1-hour RPO and the conditional RTO in Section 21. Recent changes can be lost and require reconciliation/reconstruction where evidence exists; zero data loss is not promised.
- Extended provider/connectivity failure can breach backup freshness and pause financial posting. Optional notification/AI failure does not stop otherwise healthy core views.

The committee approves the recovery gap, outage support ownership, and replacement-hardware assumptions before production. If an absolute zero-loss or materially higher availability requirement is introduced, revisit synchronous off-site durability/hosting and its cost before accepting that requirement.

---

# 47. Power and Internet Resilience

Put the server, router, and fiber ONU/modem on a tested appropriately sized UPS. A laptop battery protects the laptop only; network equipment still needs power, and battery health/runtime must be measured.

Configure BIOS auto-power-on where supported, persistent ingress/application/backup startup, time synchronization, and laptop lid/suspend behavior if a laptop is chosen. Test a real brief mains outage and power return with synthetic data, confirming that jobs/SQLite recover and externally reachable HTTPS returns.

Record UPS runtime and shutdown behavior. Prevent an exhausted UPS from repeatedly power-cycling the machine. Verify router/DDNS/IPv6 changes after reboot and the continued ability to renew credentials/certificates.

A backup internet connection is optional and separately costed. The offline/degraded financial procedures and backup-freshness pause policy still apply; local office access is available only through an intentionally configured secure route, not an assumed bypass of the selected HTTPS ingress.

---

# 48. Privacy and Data Governance

Collect the minimum data required for society operation: approved names/contact details, flat relationships, payment/billing history, and specific operational records. Vehicle details or identity documents require an identified need and their own access/retention policy. Avoid collecting unnecessary documents or minors' details by default.

Before launch, record the society's legal/state context and check which Indian DPDP Act/Rules provisions apply and have commenced for the intended deployment. The notified rules have phased commencement; a plan dated October 2026 must not assume every substantive obligation is already in force or that the society is exempt.

Implement a clear resident-facing purpose/privacy notice, accountable contact, correction/access request process, approved retention schedule, and incident/breach response with applicable notifications. Determine the appropriate processing basis and any consent/guardian requirements for the actual records; legal applicability is a committee/legal-adviser decision rather than a hardcoded sector assumption.

Do not publish the resident directory or outstanding-payer names broadly by default. Limit accountant/committee exports, avoid personal data in shared links/logs, and apply historical-person versus current-flat access boundaries.

SSE-S3 protects cloud data at rest; it does not encrypt the local SQLite database or prevent an authorized key from downloading data. Record physical/filesystem protection and whether disk encryption is required; if used, test key recovery and unattended reboot implications. Sensitive backups/TOTP secrets use the independent recoverable encryption policy.

External OCR/AI/communications providers require an explicit disclosure/retention decision. Privacy controls cover extracted text, previews, object versions, exports, and backups as well as the visible original document.

Reference: [Government DPDP Rules notification and phased timeline](https://www.pib.gov.in/PressReleasePage.aspx?PRID=2190014&lang=2&reg=3).

---

# 49. Deletion, Archival, and Retention

Routine UI actions archive people/documents and preserve financial/audit records. Issued financial records are corrected by linked entries rather than deleted or edited. Archiving removes inappropriate routine access; it is not proof of permanent erasure.

Before launch, approve a category-specific schedule listing purpose, minimum legal/operational retention, start event, legal-hold handling, authorized custodian, and deletion/anonymization treatment. State/bylaws/accountant/legal input is required; this plan does not invent one universal statutory period.

Permanent document purge requires restricted custodial permission, confirmation of expiry/no hold, an audit event, and removal of eligible S3 versions, extracted text, previews, temporary files, and expired export copies. Do not grant ordinary application credentials version-deletion/bucket-control rights.

Database/audit information required for legal financial traceability may remain under a documented exception while unneeded personal fields are minimized/anonymized where permitted. Foreign-key and accounting references must remain valid.

Encrypted backups/offline copies follow their approved retention and may retain prior data until expiry. Record deletion requests/tombstones and reapply them when restoring an older backup so archived/deleted records are not silently republished. Explain the backup-retention window in the privacy procedure.

S3 versioning is recovery protection, not a substitute for lawful retention/deletion or independent offline copies.

---

# 50. Human-Friendly Numbering and Recovery Series

Normal examples:

```text
Invoice:   INV/2026-27/00123
Receipt:   RCPT/2026-27/00102
Complaint: CMP/2026/0048
Notice:    NTC/2026/0031
```

Use opaque internal IDs and independently constrained public numbers. Neither opacity nor sequence numbers are an authorization mechanism.

```text
numbering_series: id, document_kind, financial_year_or_year, series_code,
                 registered_at, registry_reference, status
number_sequences: series_id unique, next_ordinal
```

Allocate an ordinal in the same transaction as issuance, with a unique number/series constraint. Voids/cancellations retain their number and reason; do not recycle it. Record the configured FY boundary and its rollover tests. Legacy imported numbers retain their printed value with an explicit issuing-year/book/source namespace for unique identity.

## Restore protection

An older database counter may be behind already issued documents. Before reopening issuance, reconcile surviving receipt snapshots, source records/finance-owner evidence and independent numbering checkpoints. Do not assume S3 contains every receipt that was visible before the failure. Future invoice issuance uses the same recovery protection.

After a restore, reserve a **fresh recovery series**, for example `RCPT/2026-27/R01/000001`, approved under the accountant's numbering policy. Register/reserve the new series in the restricted off-site recovery registry using create-only/conditional-write semantics before use; future restores consult that registry and cannot reuse R01. If custodial registry access or the numbering policy cannot be verified, keep financial issuance paused.

The original numbering series remains closed/historical until its lost-event reconciliation is resolved. A new series prevents number reuse but does not reconstruct missing financial transactions; that remains a separate Section 22 task.

---

# 51. Development Rules

Keep a modular, understandable Go application with a same-origin PWA, explicit SQL, one local SQLite database, one private S3 bucket initially, and a bounded database-backed queue. Prefer maintained small dependencies and the standard library where appropriate.

Record architecture decisions for SQLite driver/engine, frontend packaging, PDF/fonts, ingress, exact manual amounts, membership/history policy, numbering and backup encryption. Rounding/calculation and Tally mapping decisions are future requirements if those modules are requested. Source-control migrations, synthetic fixtures, release/runbook changes, and non-secret configuration examples.

Enforce financial/resource invariants in transactions and database constraints. Keep mutation identities, audit, and jobs with their source transaction. Resource authorization precedes list, detail, export, download, search, and external AI retrieval. Shared UI helpers cannot replace server-side authorization.

Build/test supported hardware targets reproducibly. Run appropriate Go/frontend checks, migration checks, financial/security/recovery tests, dependency review, and release smoke tests. Do not commit production data, usable secrets, or decrypted backup artifacts.

Avoid adding microservices, Kafka, Redis, distributed caches, custom workflow/report designers, a general accounting engine, vector infrastructure, or container orchestration without a documented requirement and total-cost review. Separate ingress/backup commands are simple operational processes and have their own startup/recovery checks.

---

# 52. Version 1 Completion and Production Gates

V1 consists of the revised Phases 0–6: tracking/monitoring, manually entered records and generated receipts, with registry, notices, complaints and documents. Payment initiation, bank/billing automation, Tally integration, OCR and AI are conditional future work.

## 52.1 Resident behavior

Residents can activate/recover accounts, log in, see permitted flats/manual records/balances, retrieve authorized receipts and PDF states, read audience-matched notices, raise/track complaints and download permitted validated documents. Shared devices/account changes do not expose another person's cached data. No payment-initiation or resident payment-claim flow is required.

## 52.2 Officer behavior

Authorized officers can manage registry/memberships/terms, add provided charge/opening-balance and already-paid entries, confirm appropriate received-money entries, generate immutable receipts, make auditable linked corrections, monitor/report records and manage notices/complaints/documents. The application never initiates payment or marks a manual assertion as automatically bank-verified.

Finance-entry permissions are separate from committee/resident membership. Privileged confirmation and correction require the approved MFA/control policy.

## 52.3 Mandatory measured gates

- Canonical register contains 118 unique flats; migrated people/memberships, supplied entries and document sources are approved and traceable.
- Manual-entry/receipt examples and fields are reviewed; exact amounts, derived balances and receipt sources reconcile with no unexplained differences.
- Required manual-entry idempotency, correction, numbering, PDF/S3 failure, permission and recovery cases pass; no unresolved critical financial-record/privacy/recovery defect remains.
- All privileged production users have required MFA; current grants and two custodians are documented and recovery demonstrated.
- Stable production HTTPS works on representative client networks after reboot/address changes, with ingress/certificate persistence and trusted proxy behavior verified.
- Consistent encrypted snapshots upload on schedule; off-host alerts and approved stale-backup entry protection work; clean restore verifies manual records, receipt numbering, exact document versions, current access and paused/reconciled external jobs.
- Mobile/PWA installation, cache/logout/update, accessible forms and measured performance pass on supported devices.
- Privacy/retention, receipt configuration, support ownership, emergency contact and handover procedures are recorded. No statutory charge formula is inferred from the manual-entry workflow.
- Complete annual recurring forecast including taxes and contingency is below ₹12,000; optional services disable independently.
- Finance/registry owners, committee sponsor and technical maintainer record the pilot/production decision and evidence.

Tally import compatibility, automated bill calculations, bank feeds and payment initiation are not current production gates.

---

# 53. Recommended Initial Decisions

| Area | Initial decision |
|---|---|
| Society size | 118 flats; actual resident/login population established during migration |
| Hosting | Society-owned reliable low-power Linux computer |
| Backend/frontend | Go + React/Vite same-origin PWA |
| Database | Local SQLite WAL, FULL synchronization, fixed WAL-reset engine, bounded connections |
| Document storage | Private Mumbai S3, validated uploads and version-pinned short-lived downloads |
| Accounting | Retain very old Tally independently; application integration and compatibility proof are future work |
| Financial records | Given charges/opening balances and already-paid entries, exact amounts, derived balances and linked corrections |
| Payment execution | Outside portal; authorized users record money already received; no gateway or bank automation |
| Receipts | Immutable issued snapshots/numbers, retryable PDF availability, S3 storage |
| Access | Current and historical person/flat policy; term-based roles; privileged MFA required |
| Notices/complaints | Portal records + manual WhatsApp links; private complaint staff notes |
| V1 search | Authorized metadata/filter search |
| OCR/FTS5 | Optional Phase 7, tested actual languages and bounded workers |
| AI | Optional Phase 8, initially read-only approved external API |
| Ingress | Tested direct DDNS if suitable; otherwise approved Funnel or owned-domain Cloudflare Tunnel |
| Public/static IPv4 | Not required when a suitable public tunnel is selected |
| Domain | Conditional on chosen route; include actual renewal in the budget |
| Native apps / cloud compute / Redis / Postgres / Kubernetes | No initial requirement |
| Jobs | Leased SQLite queue, at-least-once execution with idempotent handlers |
| Backup | 30-minute consistent encrypted snapshots, explicit retention, monthly offline copies |
| Recovery | Proposed 1-hour connected-operation RPO, conditional RTO, reconciled restricted restore |
| Operating budget | Full annual recurring infrastructure < ₹12,000; development/capital/labor separately recorded |

Unresolved facts and approval evidence are tracked in Section 54 rather than being silently treated as confirmed.

---

# 54. Outstanding Decisions and Build Gates

## 54.1 Decision register

| Decision/input | Current status | Owner | Required before |
|---|---|---|---|
| Flat count | Confirmed by user: 118 flats | Registry officer | Canonical register approval |
| Actual resident/account population and contact conflicts | Pending migration review | Registry officer | Invitations/pilot |
| Society state, receipt fields/wording and retention requirements | State unspecified; representative examples/policy pending | Committee + finance/records owner | Receipt/retention acceptance before production |
| Representative given charges/paid entries and correction policy | Examples pending; synthetic development can proceed | Finance owner | Manual entry/receipt workflow acceptance |
| Automated tax/charge/interest basis | Deferred until automation is requested | Committee + accountant | Future billing rules only |
| Exact old Tally product/release/company settings | Pending; not a current core-build dependency | Accountant | Future Tally integration proof only |
| Accounting export mappings/import contract | Deferred until requested | Accountant + finance owner | Future Tally integration only |
| Historical owner/tenant/family receipt/document access | Initial policy in Section 10; committee confirmation pending | Committee + registry officer | Historical import/pilot |
| Machine/SSD health, wall power, UPS/network runtime | To be measured on selected hardware | Maintainer | Phase 0 acceptance and forecast |
| ISP/public reachability and stable ingress | To be tested from representative networks | Maintainer | Production hostname/invitations |
| AWS/domain/repository ownership and two custodians | Names/custody records pending | Committee sponsor | Real production data and recovery setup |
| RPO/RTO, backup-write pause and numbering recovery series | Proposed policy in Sections 21/50; acceptance pending | Committee + treasurer/accountant | Financial production use |
| Initial volumes/quotas and complete annual quote | Scenario subtotals only; worksheet pending | Maintainer + treasurer | Production route/service selection |
| External alert route and response owner | Provider/channel/names pending | Maintainer + sponsor | Pilot/launch |
| Development capacity, cost and calendar commitments | Planning estimates in Section 39; staffing pending | Sponsor + maintainer | Delivery commitment |

Record actual values, source/evidence references, decision dates, and approvers in the implementation repository/runbook. A pending item blocks only its dependent phase/workflow; infrastructure prototyping with synthetic data can proceed.

## 54.2 Networking selection

Test DDNS/direct access only if public reachability and router/firewall operation are acceptable, including IPv4-only residents. For CGNAT/IPv6-only origins or unsuitable direct exposure, quote eligible Funnel use or choose a society-controlled domain with Cloudflare Tunnel within the full annual forecast. Quick Tunnel stays development-only.

Selecting a stable origin includes CORS, cookies, trusted headers, certificates, PWA help, and all permanent portal links. No route is claimed permanently free or universally reachable without the actual proof.

---

# 55. Implementation and Launch Checklist

Checked implementation items below have local synthetic evidence; they do not close the separate production gates in Section 52. The accepted baselines and current candidate are linked in the [execution backlog](execution-backlog.md).

## Hardware and OS

- [ ] Select reliable owned hardware and verify SSD health, memory, Ethernet, incremental wall power, and storage headroom.
- [ ] Install supported current Ubuntu Server LTS or Debian stable; create unprivileged application/admin identities.
- [ ] Configure IPv4/IPv6 firewall, SSH keys/private access, time sync, compatible security updates, and physical access.
- [ ] Test UPS coverage for server/router/ONU, power-return boot, and laptop sleep behavior if applicable.

## Infrastructure and recoverability

- [ ] Select/prove stable production ingress and client-network reachability; test renewal/restart and proxy trust.
- [ ] Create private Mumbai S3, encryption/versioning/CORS, scoped document/backup/custodial identities, and retention/quotas.
- [ ] Confirm society-owned accounts, repository/releases, two custodians, and independent encrypted-key recovery.
- [x] Pin/test the actual Go SQLite engine including the WAL-reset fix and per-connection settings (local engine 3.53.4; four pooled connections checked).
- [x] Implement consistent local snapshot/manifest and matching-release restore-check commands, with provenance, integrity, foreign keys and credential invalidation verified.
- [ ] Complete encrypted scheduled off-site backup, production restore mode, off-host alerts and backup-age finance protection.
- [x] Measure matching-release local pre/post-upgrade restores while preserving original money, receipts, uploaded originals and held key bindings.
- [ ] Complete the clean production restore rehearsal and register adopted numbering policy/recovery series.

## Core application and manual records

- [x] Implement invitations, scoped identities/memberships/roles/terms, sessions/recovery and required MFA, with current-access checks.
- [x] Implement transactional audit, actor-bound mutation identities, leased local jobs and validated immutable local originals/receipt PDFs.
- [ ] Connect version-pinned private production storage and verify failure/recovery behavior with its configured credentials.
- [ ] Review representative given charges/already-paid records, receipt fields and authorized confirmation/correction policy.
- [x] Implement exact-paise manual entry validation and derived balances without inventing charge/interest rules.
- [x] Implement given entries, immutable originals, receipts/PDF states, linked corrections and local yearly receipt numbering/recovery.
- [x] Implement separate approved maintenance/funds, explicit credit allocations, private incidents/fines, scoped manual-record/receipt views, notices, service conversations and document/financial-original publication.
- [x] Complete scoped operational finance exports with current authority, exact original/correction links, independent amounts and matching local recovery in release 0.19.
- [ ] Complete official portal WhatsApp and remaining ordered community workflows in the roadmap.
- [ ] Implement PWA cache/update/logout/accessibility behavior and operator degraded-state views.

## Migration, budget, and pilot

- [ ] Approve 118 unique flats, people/relationships, historical access, cutoff, and idempotent source mappings.
- [ ] Reconcile any supplied per-flat/aggregate opening balances and manual records; preserve signs and do not invent aging or allocations.
- [ ] Import approved high-value documents and historical receipts without creating new financial movements.
- [ ] Complete the annual cost worksheet including taxes, versions/backups/transfer, renewals, monitoring, and contingency.
- [ ] Record privacy/retention, support and emergency channels, operating responsibilities, and staffing/calendar estimates.
- [ ] Test 10–15 representative pilot flats and manual-entry operators across entries/receipts, notices, complaints, files, roles and recovery; resolve critical defects.
- [ ] Repeat restore/numbering/job/access checks, make an off-site checkpoint, and record the Section 52 production decision.
- [ ] Roll out to the remaining flats with invitation/recovery/help instructions and monitored support.

## Later only

- [ ] Add payment execution, bank/billing automation or Tally integration only after a separate request and requirements/control proof. Requested manual allocations are already verified locally.
- [ ] Assess actual OCR/search demand and language samples before Phase 7.
- [ ] Verify FTS5 capability in the release driver/build before any future full-text search implementation.
- [ ] Activate requested WhatsApp/email delivery only after society-owned provider onboarding/configuration, current consent/templates and authorised real-send checks. Development delivery stays synthetic.
- [ ] Add runtime AI only after a separate request and provider/data/cost/evaluation decisions.

---

# 56. Success Criteria

The project succeeds when:

1. Approved registry, manual entries and documents have one traceable operational source for all 118 flats and current memberships.
2. Authorized users record given charges and already-paid entries accurately, and residents retrieve permitted receipts/records and track complaints.
3. Posted manual amounts, derived balances, corrections and receipt sources reconcile with no unexplained differences. No portal action initiates payment.
4. Notices/documents have consistent audience/resource permissions, including historical person-specific records.
5. Privileged MFA, role expiry/revocation, safe uploads and report/search boundaries pass representative tests.
6. Failed jobs/PDFs are visible and recoverable without duplicate entries, receipts or public numbers.
7. Real backup/restore exercises meet approved conditional targets and reconcile manual records, receipt numbering, current grants and external effects.
8. Complete annual forecast stays below ₹12,000; optional features do not become hidden core dependencies.
9. Resident adoption, complaint handling, manual-entry/receipt effort, support burden and maintenance capacity are reviewed after the pilot and first quarter.
10. Current runbooks and two custodians allow the next committee/maintainer to operate and recover the system.

Set adoption/response/support targets from the pilot baseline rather than inventing usage metrics before residents have tried the product.

---

# 57. Future Enhancements

Only after V1 is stable and the added work has an owner, measured need, and annual-cost/privacy review:

- Payment initiation/gateway integration if separately requested.
- Automated charge/bill/interest calculations, detailed allocations/advance settlement and Tally integration.
- Resident claims and bank-statement import/matching under a separately approved workflow.
- SES notifications if deferred at launch.
- Automated WhatsApp under verified current Meta rules and spending limits.
- FTS5/extraction/OCR with tested local-language support.
- Read-only, permission-scoped AI Q&A and structured queries.
- Contract/AMC reminder scheduling from structured expiry fields.
- Vehicle/parking register when operationally required.
- Amenity booking, visitor management, polls/voting, meeting workflows, digital NOC requests, vendor register, or expense tracking when explicitly justified.
- Formal accounting replacement only after a separate requirements/migration/control review.

Manual-entry accuracy, linked corrections, immutable receipt/recovery, audit and backup/restore remain V1 requirements. Detailed allocations/advance automation, automatic billing, bank matching and Tally integration are conditional future features.

---

# 58. Architecture Principle

> **Keep state small, durable, recoverable, and portable.**

The replaceable local host holds SQLite operational state; S3 holds validated document versions and off-site snapshots. Source/release artifacts, approved configuration, independently recoverable keys, society-controlled accounts, and operating/reconciliation runbooks complete the recovery assets.

The key relationships are financial source -> immutable ledger/audit -> retryable external job, and document metadata/entitlement -> exact validated object version. Portability requires preserving these links, not merely copying a database and bucket.

Use the simplest stack that meets the approved correctness, privacy, budget, and recovery objectives. Periodic backup and a single local host have a bounded recent-data/availability tradeoff; stronger guarantees require an explicit architectural and cost change.

---

# 59. External-Service and Engine Notes — Checked 4 October 2026

These checks establish current documented capabilities/constraints. They do not establish this society's ISP reachability, product eligibility, exact old-Tally compatibility, regulatory applicability, or a completed annual quote.

## AWS S3

Pay-as-you-go private storage supports time-limited uploads/downloads, versioning, checksums, CORS, POST size policies, and conditional writes. Presigned URLs are reusable bearer tokens; PUT can overwrite a key and role revocation does not immediately revoke an issued URL. Each retained version is billed as stored data.

The retrieved Mumbai regional price list is published 28 September 2026: Standard first-tier storage $0.025/GB-month; ordinary Tier 1 requests $0.005/1,000; ordinary Tier 2 requests $0.0004/1,000. Recheck deployment prices, transfer allowances, region, and taxes.

References: [pricing](https://aws.amazon.com/s3/pricing/), [regional price list](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonS3/current/ap-south-1/index.json), [presigned behavior](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html), [versioning](https://docs.aws.amazon.com/AmazonS3/latest/userguide/Versioning.html), [CORS](https://docs.aws.amazon.com/AmazonS3/latest/userguide/enabling-cors-examples.html), [POST size constraints](https://docs.aws.amazon.com/AmazonS3/latest/developerguide/sigv4-HTTPPOSTConstructPolicy.html), [conditional writes](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html).

## SQLite

WAL with FULL synchronization has stronger power-loss durability than NORMAL; foreign-key enforcement must be enabled on connections. Safe online snapshots use the backup API or VACUUM INTO. The 2026 WAL-reset fix is required in the actual embedded engine: 3.51.3+ or a documented maintained backport such as 3.44.6/3.50.7.

References: [WAL/reset bug](https://www.sqlite.org/wal.html), [synchronous](https://www.sqlite.org/pragma.html#pragma_synchronous), [foreign keys](https://www.sqlite.org/foreignkeys.html), [backup](https://www.sqlite.org/backup.html), [FTS5](https://www.sqlite.org/fts5.html).

## Tailscale Funnel

Funnel publicly exposes a local service through a `*.ts.net` hostname without residents installing Tailscale. It remains beta with non-configurable bandwidth limits. Personal-plan usage restrictions and organizational applicability require confirmation; Standard currently lists $8/user/month. Public visitors are not automatically operator/tailnet seats.

References: [Funnel](https://tailscale.com/docs/features/tailscale-funnel), [plans/eligibility](https://tailscale.com/pricing).

## Cloudflare and direct HTTPS

Quick Tunnels provide temporary/random hostnames for testing, with documented request/SSE limits. Publishing a normal production Cloudflare Tunnel application requires a domain configured with Cloudflare. Confirm the selected plan/features and domain renewal quote.

Caddy supports automatic HTTPS; HTTP-01 needs port 80 and TLS-ALPN-01 needs 443. IPv6 origin availability alone does not prove IPv4-only resident access.

References: [Quick Tunnels](https://developers.cloudflare.com/tunnel/get-started/quick-tunnels/), [production setup](https://developers.cloudflare.com/tunnel/get-started/), [Caddy validation](https://caddyserver.com/docs/automatic-https), [IPv6 certificate validation](https://letsencrypt.org/docs/ipv6-support/).

## Old Tally

Official Tally.ERP 9 documentation describes schema-specific XML import. Modern TallyPrime spreadsheet support does not prove import support on the user's very old installation; exact-version/company-copy proof remains pending.

References: [ERP 9 import](https://help.tallysolutions.com/docs/te9rel60/Data_Management/Import_of_Data_Intro.htm), [current Prime spreadsheet FAQ](https://help.tallysolutions.com/import-data-faq/).

## SES, PWA, budgets, and privacy

SES needs verified sender identities and region-specific production access for arbitrary recipients. New service workers can wait for activation; PWA updates require a deliberate policy. AWS budget notification updates can lag costs. India's DPDP Rules were notified with phased commencement; the society's legal context and effective launch requirements remain to be checked.

References: [SES production access](https://docs.aws.amazon.com/ses/latest/dg/request-production-access.html), [SES identities](https://docs.aws.amazon.com/ses/latest/dg/verify-addresses-and-domains.html), [PWA updates](https://web.dev/learn/pwa/update), [AWS budget limitations](https://docs.aws.amazon.com/cost-management/latest/userguide/budgets-managing-costs.html), [DPDP notification](https://www.pib.gov.in/PressReleasePage.aspx?PRID=2190014&lang=2&reg=3).

Automated WhatsApp pricing/onboarding and any chosen external AI provider are future checks, not verified V1 dependencies. Refresh relevant sources before implementing those optional features.

---

# 60. Final Recommended Build and Start Point

```text
118-flat society + associated owner/tenant/family users
                      |
       tested stable public HTTPS ingress
                      |
         society-owned reliable Linux host
     Go + same-origin React/Vite PWA + local SQLite
       registry / scoped accounts / required MFA
       given charges / already-paid entries / derived balances
       manual confirmation / receipts / linked corrections
       immutable entry/receipt history / leased jobs / audit
       notices / complaints / authorized reports and metadata search
                      |
      private S3 exact document versions and encrypted snapshots
                      |
       custodial offline copies + independent keys/runbooks

Existing old Tally continues independently; integration is future work
Optional later channels: SES / automated WhatsApp / OCR / read-only AI
```

Start with a runnable local Phase 0 using synthetic data and measurements. Build identity/security and shared job/storage/audit foundations, then authorized manual entries/receipts and resident workflows. Complete current-scope reconciliation, migration, recovery and a representative pilot before all-flat rollout. Tally compatibility is required only for a future integration.

The confirmed scale is 118 flats. State-specific rules, actual account population, Tally version, ingress/power/volume measurements, custody, and final annual quote remain recorded decisions, not invented facts.

The intended outcome is a maintainable operational society system within the recurring infrastructure ceiling, with explicit financial correctness, historical privacy boundaries, and realistic tested recovery. Full OCR/AI and additional modules follow demonstrated need rather than becoming launch prerequisites.
