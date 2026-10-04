# Manual entries and receipts · local checkpoint

Release `0.4.0-dev`, schema 4 connects the entry book and receipt collection to persistent manual records. The portal records supplied amounts and money already received outside the platform. It does not initiate payment or calculate automatic bills.

## Working workflow

An authorized operator chooses a home and a given charge, opening amount due, opening credit or money-received entry. Amounts enter as decimal rupee strings and become integer paise without floating-point arithmetic. Dates cannot be future dates. Received entries require a payer, method and a non-cash reference; an optional source/evidence note records the provenance supplied by the operator.

Saving creates a draft with no balance effect and no receipt. The review dialog shows the stored home, amount, date, description and received details. Confirmation requires an explicit review checkbox; received entries also attest that the money was already received. A mistaken draft can be discarded with a reason, preserving its history without changing the balance.

Posting commits the entry, actor/time, immutable audit and operation result in one transaction. A received entry also commits its receipt number, frozen receipt snapshot and background PDF job in that transaction. Charges/opening entries never create received-money receipts. Search, home/status filters, pagination, detail, PDF download and linked reversal work in the rendered interface. A reversal records its actor/time/reason and excludes the original amount from current balances. The original entry and receipt remain preserved; reversed downloads are labelled as originals.

## Permissions

Registry administration alone cannot post financial records. An active `TREASURER` grant provides posting, draft discard, reversal and failed-PDF retry; MFA is required. `COMMITTEE` has a read-only community view. Residents see confirmed records only for current memberships whose `can_view_finances` flag is enabled. Ended membership, expired/revoked roles and revoked financial entitlement are rechecked on requests; financial writes repeat the permission check inside the reserved SQLite write transaction.

The CLI gives the existing fictional registry officer an explicit additional treasury grant. This is a synthetic demonstration of combined duties, not a rule that every administrator is a treasurer. The public owner account can view A-101/A-102 financial records; the tenant account's registry membership does not grant finance access. Invitation-based treasury assignment and production role custody remain part of subsequent role/account administration.

## Retry and file recovery

Create, confirm, discard and reverse operations retain an actor-bound operation key and a hash of the action/details. Identical retries return the first result; changed details using the same identity are rejected. Permission is rechecked before a replay. Browser retries retain their original keys and lock an uncertain draft's submitted values; known validation rejection allows correction.

Workers atomically claim a receipt job with a one-minute lease and a unique fencing token. A replacement worker can claim an expired lease; the old worker cannot publish afterward. Render/storage failures retry up to five attempts, then expose a failed state and operator retry. The confirmed entry and receipt number survive those failures. Shutdown waits for the local worker to stop before closing storage/database resources.

PDFs use pinned [gopdf](https://github.com/signintech/gopdf) and an embedded, licensed [Google Fonts DM Sans](https://github.com/google/fonts/tree/main/ofl/dmsans) asset. Long values wrap, with additional pages when needed. Files are addressed by SHA-256 under `documents/` beside the chosen database, outside `build/web`. The directory is 0700; files are 0600. `os.Root` confines file access, writes sync before publication, and reads verify content hashes. Downloads check current financial scope first. No static/private-file URL or signed link bypasses authorization.

The database snapshot retains immutable receipt snapshots and job identities. Serving a restored database checks completed PDFs and requeues missing/corrupt files for regeneration. These derived files are not included in the existing database-only snapshot bundle. Production off-site encrypted backup, externally issued-number reconciliation and current-access review after restore are still separate requirements.

## Independent acceptance results

`make check` passed all **39 backend tests** with the race detector, formatting, Go vet and TypeScript. The production build passed. The full **34-test Chromium suite** passed in **1.7 minutes**; the final PDF-spacing adjustment also passed document/server race tests and two targeted receipt browser journeys in 7.1 seconds.

The domain checks use independently specified amounts: an opening debit of ₹1,000 and a confirmed received amount of ₹400 produce a ₹600 balance and one ₹400 receipt. Draft creation and draft discard leave that balance unchanged. Replaying confirmation creates no second receipt; reversing the received entry restores the ₹1,000 balance while preserving the original receipt. A one-paise charge and one-paise opening credit cancel exactly.

Additional backend checks cover invalid/overprecision amounts, concurrent identical retries, changed-payload rejection, revoked treasury permission on replay, resident scope, financial-entitlement revocation, immutable posted/discarded/correction records, live/expired leases, stale-worker publication denial, failed-job retry, private-file permissions, integrity checks, path traversal, long PDF fields and database-only document regeneration.

Browser checks use disposable synthetic databases, including an explicit failed-worker fixture in that isolated database. The new journeys cover draft/review/confirm/download/reversal, lost create/confirm responses, disabled dismissal while pending, required selection, every entry type/method option, long home menus, list/download errors and retry, zero-amount correction, resident/tenant allow-deny, all 13 matching draft cards over two pages, a failed PDF retry with an unchanged identity, and discard with no balance/receipt effect. Menus/forms are inspected at 1440×900, 768×1024, 375×812 and 320×568; other journeys use their specified desktop/phone sizes.

The new review captured **29 browser PNGs**, plus one downloaded PDF and its raster. All browser captures were reviewed in contact sheets, with detailed inspection of representative desktop/form/menu/phone-feedback states and the extracted/rasterized PDF. These counts describe artifacts, not defects. Screenshots and the downloaded/rasterized receipt are kept privately under `reports/local/records-review/`. Run `SOCIETY_CAPTURE_UI=1 npm run test:browser --prefix web` after `make build` to reproduce captures. The process and defect inventory are documented in [our development workflow](development-workflow.md).

## Acceptance limits

This is a local synthetic workflow. Approved society entry examples, receipt format/numbering policy, real identity/finance access rules and production infrastructure are still pending. The receipt uses a fictional society name and explicit preview notice. No statutory rate, interest, automatic bill, bank matching, allocation or Tally behavior is inferred. PDF text has been reviewed with the embedded font for Latin text and the rupee symbol; full complex-script rendering and physical-device/cross-browser behavior are not established by this checkpoint.
