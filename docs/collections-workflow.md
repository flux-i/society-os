# Fund campaigns and externally paid reports — expectation brief

Recorded 5 October 2026 before implementation, following upkeep in the [operations roadmap](society-operations-roadmap.md). This is an expectation brief, not evidence of implementation or acceptance. All examples and development payments are fictional.

## Outcome and permissions

An eligible Treasury operator prepares an approved-purpose collection: title, resident-facing purpose, private approval/source reference, fixed requested contribution or voluntary contribution, start/due dates, optional target and explicit participating homes. Preview the homes and exact requested total before submission. A different current eligible Treasury reviewer approves the frozen proposal. Preparation and a resident payment report create no received-money receipt. Registry or operational appointment alone grants no campaign preparation or confirmation authority. Accountant/auditor access reads permitted financial metadata without posting or deciding. Current session, factor, appointment and financial-home scope are checked inside each transaction and again on retries.

Published fixed contributions create one immutable supplied charge per participating home atomically, preserving the cycle/ledger posting contract. Voluntary contributions have no invented debt or overdue amount. Residents see only published participation for their currently entitled financial homes and their own reports; internal source notes, another person's report/evidence and society-wide named unpaid lists remain private. Joint owners share one home contribution. Close/reopen decisions retain history and do not erase existing charges or receipt allocations.

## Reports and confirmation

A currently entitled resident reports money already sent: participating home, campaign, exact amount, supplied payment date, method, external reference and comment. Private evidence may be linked only through a validated original and current access; an image is supporting evidence, not bank verification. A reviewer can request clarification or reject with a reason; a revised submission retains its previous version and decision. The reporter cannot confirm their own payment. Closed campaigns accept no new claims while preserving review of existing claims and historical confirmations.

Confirmation records an explicitly supplied external verification source and payment identity. It either posts one received entry/receipt or links one existing compatible received entry. It allocates only the chosen usable amount to the chosen live charge for the same home. One payment may be split across purposes through explicit allocations, within the existing exact-paise contract; excess stays available credit. A resident claim alone never counts as collected money.

Two people can report the same external payment. A transactionally unique verified source/payment identity, existing-entry checks and immutable confirmation history prevent a second received entry or duplicate receipt. A duplicate is resolved against the existing confirmed source rather than counted as additional cash. An already allocated source cannot be overused. The operator reviews amount, date, home, method, entry identity and independently available credit before confirming; a source/ref collision with incompatible money or home fails visibly.

Writes use actor-bound stable operation identities, exact-payload retries, expected versions and writer reservation. Lost responses retry the same confirmation and preserve entry, receipt and allocation identities. A changed retry, ended authority, stale report or simultaneous conflicting decision fails with a useful reload path. Current authority is checked before returning a previous success. Reversing a confirmed source preserves its original receipt and report history, releases ineffective live allocations and updates current campaign status. It never silently reassigns money.

## Meaningful statuses and amounts

Distinguish unpaid, part paid, reported/awaiting confirmation, confirmed paid, waived/exempt and corrected. Pending claims are shown separately from confirmed allocations. An exemption/waiver needs an explicit source, reason and separate eligible decision before a linked financial correction; it is never inferred from a comment or uploaded picture. Frozen requested, live expected, confirmed allocated, outstanding, available credit and reversed amounts have separate labels. A voluntary contribution may be confirmed without a requested-debt status.

Independent examples:

- Fixed requests of ₹1,000.00 and ₹500.25 produce ₹1,500.25 requested, two charges and zero receipts. A future due date has zero overdue amount.
- A ₹400.00 claim leaves confirmed collection zero. Treasury confirmation and allocation to the first home produce one ₹400.00 receipt, ₹400.00 allocated and ₹1,100.25 outstanding overall. A second reporter of that same verified payment creates no extra entry, receipt or collected amount.
- A later ₹1,200.00 confirmed payment for the same first home can allocate only the first home's remaining ₹600.00 in this campaign; ₹600.00 remains available credit. It cannot settle another home's ₹500.25 debt. An explicit same-home allocation to another purpose is permitted within that credit.
- Reversing the original ₹400.00 received entry restores that home's ₹400.00 campaign outstanding while preserving its original receipt/report/correction history. No replacement receipt appears automatically.
- A supplied partial exemption changes live expected through a linked correction and retained decision; it never claims cash was received.

## Interface, native checks and recovery

Preserve the forest/ivory interface and accessible shared selectors/dialogs. Show a purposeful campaign directory, exact progress, scoped participant statement, private confirmation queue and deliberate two-stage previews. Exercise actual contribution/home/payment-method choices, separate approval, reporting/revision, confirmation/linking/rejection, duplicate resolution, closing and scoped receipt navigation. Include independently expected amounts rather than copying API totals.

Render desktop, tablet and 375px/320px phones; open menus and inspect selected/hover/focus/keyboard/dismissal states, internal scrolling, pending protection, source/choice errors, retry, empty/loading and stale conflicts. Inspect retained screenshots. Bounded native WebMCP reads must enforce current campaign/home/report scope and omit payment references, verification sources and evidence bytes; native tools cannot approve, report or confirm. Run the coherent domain/race/HTTP/TypeScript/build/browser/native gate, schema-preservation and rich recovery checks before replacing the accepted preview or publishing. Then continue the remaining roadmap without stopping.

## Limits that remain explicit

No payment gateway, transfer initiation, bank API, runtime LLM or live message is required. This records claims and independently verified money already received. Society finance authority, liability/exemption policy, receipt conventions and real reconciliation examples are still production acceptance inputs; fictional development does not establish those powers.
