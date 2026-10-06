# Prepared financial statements and operational exports

Recorded before implementation, following the messaging checkpoint. The original preparation/review/publication slice is now [accepted locally in release 0.17/schema 16](statements-baseline.md); channel sharing and scoped operational exports remain the next sequential checkpoint. This contract preserves their intended outcomes and independent expectations. It implements section 6 of the [operations roadmap](society-operations-roadmap.md). Continue to preserve the accepted preview and use disposable fictional files and databases for development.

## User outcome

A finance operator uploads an accountant-prepared income statement, balance sheet, budget or audit report, records its period and source, and keeps the original private while it is checked. A different current finance reviewer approves a particular immutable version. The operator deliberately proposes publication to owners, tenants, all current residents or another supported bounded audience. A different eligible reviewer approves that exact version and audience. Residents then read or download the version explicitly shared with them. Sharing a statement gives no access to another home's money records, receipts or private evidence.

The portal labels the file as externally prepared. It does not evaluate or import its figures into the manual ledger, derive a statutory balance sheet or claim that an uploaded document reconciles the society's books. Publishing, downloading and sending a statement create no payment, charge, allocation or received-money receipt.

## Permissions and decisions

Finance preparation, internal review, publication decisions, publication withdrawal and society-wide financial exports require the separate current Treasury permission and recent privileged verification. Registry or contact administration alone is insufficient. Preparation and approval must have different eligible actors. Current account status, appointment, factors and authority are checked within each write and before replaying a successful operation.

Before publication, the uploader and current eligible finance reviewers may inspect the original and its private review history. After publication, a resident sees only the explicitly released title, period, author/source, version and original for their current audience. Drafts, failed validation, unapproved replacements and private review reasons remain hidden. Bounded lists, searches, totals, downloads, history and native reads use the same scope; rejected identities and guessed record IDs reveal no private metadata.

Publication freezes the original file identity/checksum, period, source, title and audience. Replacing a file creates another immutable version; the previously published version remains available until the replacement is separately approved and deliberately published. Withdrawal blocks new portal downloads and new delivery handoffs, while retaining originals, earlier decisions, download audit and delivery history. It cannot retrieve a copy already downloaded or accepted by a provider.

An actor-bound stable operation preserves one upload, review, publication or withdrawal through a lost response. The UI freezes edits and dismissal during an unresolved write, and retries the exact operation. Stale decisions preserve the human reason, reload the current version and require a fresh attestation.

## File handling

Reuse the bounded original-file reservation, checksum, validation lease, immutable storage and recovery workflow. Start with validated PDF originals and deliberately add XLSX and UTF-8 CSV support. Existing PNG/JPEG evidence behavior must remain intact. Reject unsupported binary XLS, macro-enabled formats, encrypted workbooks, malformed archives/XML, duplicate or traversal archive paths, excessive expansion and unsupported active or external relationships. Inspect worksheets within explicit row, column, cell, entry-count and expanded-byte bounds; never execute formulas, macros, embedded objects or external requests during validation. Retain accepted original bytes unchanged and serve them as attachments.

Validation unavailability is a visible recoverable state, not approval. Failed originals cannot be downloaded or published. Local synthetic storage and content checks do not constitute production malware containment or society-owned encrypted storage acceptance.

## Intentional channel sharing

Extend the approved messaging workflow with a financial-statement source only after publication is verified. The external wording uses an authenticated link to the exact deliberately published version. Resolve the intersection of publication audience, current home relationships, active linked accounts and independently verified finance-channel consent. A tenant who can read a deliberately shared statement does not acquire home-finance permission.

Preview targeted people, source-entitled people, verified permission, eligible people, unique destinations and omissions. Require a separate delivery reviewer. Recheck the publication/version, permission and contact at approval, claim and handoff; changed or withdrawn publications suppress unsent recipients rather than silently substituting a replacement. Preserve the existing truthful provider outcomes, signed callback checks, per-person shared-destination decisions, bounded retries and unknown-handoff reconciliation. Use the local simulation only until live providers are configured.

## Operational exports

Provide bounded exports of the existing manual ledger, maintenance allocations/statements, fund reconciliation and receipt register. Society-wide exports require Treasury authority. A resident's allowed export is restricted to their currently financially entitled homes; access to a published society statement does not widen that entitlement.

Choose an explicit date range/home or fund scope, display the scope and row count, and export a consistent current read. Preserve record identity, original receipt identity and linked corrections. Represent all monetary values from integer paise with exactly two decimal places; distinguish charges, original received money, allocations, available credit and reversals. A payment report awaiting verification is not received money, and allocations never add another collection.

Escape CSV fields correctly, preserve Unicode and neutralise formula-leading text in generated exports. Never rewrite the retained uploaded original to make an export. An export records its actor, scope and time without putting private transaction text into broad audit or logs. Interrupted or denied downloads report an error and retain a retry; they cannot show a fabricated successful download.

## Checkpoint expectations

Use independently specified fixtures and expected amounts before implementation. Verify private draft/upload validation, separate internal/publication/delivery review, self-review denial, stale and duplicate operations, replacement preserving the old original, controlled owner/tenant publication, ended membership, revoked finance authority, withdrawal and frozen version links. Include malformed and malicious spreadsheet fixtures, expansion/cell limits, inert formulas and exports with formula-leading names and Unicode.

Independently reconcile a small exact-paise ledger containing a one-paise amount, partial payment, excess credit, allocation and linked reversal. Check that original receipts stay unchanged and reported/unverified payments contribute no received total. Restore populated files/publications/exports/delivery proof using the matching release and separately held keys; invalidate old sessions.

Run relevant backend/race checks, TypeScript and the production build. Exercise the actual upload, source/audience menus, review, replacement, download, publication, withdrawal, export and simulated-sharing controls at desktop, tablet, 375px, 320px and short-phone sizes. Inspect loading, empty, denied, failed validation, retry and unresolved-write states; verify dialog scrolling, keyboard focus and dismissal. Capture and actually inspect screenshots. Actual native WebMCP exposes bounded permitted metadata only, preserves human forms and discards cancelled or stale results. Record executed coverage and limits before acceptance and preview promotion.

Production inputs remain separate: accountant-approved examples and periods, finance/retention policy, society-owned storage and key custody, live provider onboarding/templates, current identities and the actual Windows operating environment. Missing inputs do not stop this local synthetic workflow.

## Implementation checkpoints and independent fixtures

Release 0.16 messaging is accepted and published. Statement work starts with private originals, separate finance review and deliberate frozen publication, then extends the messaging source and scoped exports. These are sequential checkpoints; none is accepted merely because its contract or validator exists. The original/publication slice passes all local gates and matching recovery in 0.17. Keep the retained 0.17 preview while channel sharing and exports are built on disposable synthetic databases.

PDF checks reuse the actual bounded qpdf validator. Initial XLSX support is deliberately limited to plain worksheets, shared strings, styles and themes, with explicit entry, expansion, XML depth/token, row/column and cell limits. Unsupported charts, embedded objects, external relationships, macros and active formulas are rejected. Formulas are inspected but never evaluated. The package/worksheet/relationship expectations follow [Microsoft's SpreadsheetML structure](https://learn.microsoft.com/en-us/office/open-xml/spreadsheet/structure-of-a-spreadsheetml-document). CSV originals require bounded consistent UTF-8 fields and reject active formula text; ordinary negative numeric amounts remain valid. Generated exports also account for formula-leading Unicode/control characters and proper field quoting; [OWASP's CSV injection guidance](https://community.owasp.org/attacks/CSV_Injection) explains why quoting alone is insufficient. Accepted originals are retained unchanged.

The independent export fixture will include charges of 20,000, 43,219 and one paise; opening credit of 10,000 paise; received originals of 50,000 and 101 paise; and a linked reversal of the 101-paise original. Expected confirmed original receipts remain two with 50,101 paise, reversed money is 101 paise, usable received money is 50,000 paise, and the home balance is 3,220 paise (₹32.20). A pending reported payment contributes zero received money. Maintenance consumes 10,000 paise of opening credit and 10,000 of received money. A fund allocation of 10,000 is fully corrected and replaced by 6,000; expected available received credit is 34,000 paise. Those allocations do not increase received money. Verify source records/corrections independently rather than deriving expectations from exported totals.
