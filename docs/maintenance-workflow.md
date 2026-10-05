# Maintenance cycles and receipt allocation — expectation brief

Recorded 5 October 2026 before implementation, following step 4 of the [operations roadmap](society-operations-roadmap.md). Account administration is verified locally as release 0.9/schema 8; maintenance implementation and acceptance are pending. This checkpoint uses fictional source amounts and disposable databases. It does not establish society-approved rates, liability or finance policy.

## Outcome and authority

A treasury operator prepares a maintenance period with a title, period start/end, explicit due date, source/approval reference, participating flats and supplied amounts. Preview each amount and the exact total before submitting a frozen proposal. A different currently eligible treasury reviewer can approve and publish that version, atomically creating the given charge for each participating flat. Reject self-approval, changed/stale proposals and duplicate publication. Committee financial readers may inspect the proposal but cannot post by virtue of that appointment; financial posting remains explicitly treasury-authorised. An administrator alone cannot read society-wide financial data or publish charges.

Residents see published charges only for their currently financially entitled homes. Draft, declined and withdrawn cycles, private source notes, society-wide collections and another home's participation remain operator-only. Current permissions gate lists, totals, details, participant lookup and native tools in the same read snapshot. Membership/appointment changes discard stale client data using the shared access-scope contract.

Amounts are exact positive paise supplied per flat; the portal does not calculate a statutory rate, infer owner/tenant liability, apply interest or penalties, or automatically create another period. Due dates are independently supplied India calendar dates. A future due date is never overdue. A submitted proposal is frozen; decline/withdrawal preserves its history and any replacement is an explicit new proposal. A published cycle preserves its charges, decisions and source reference. Corrections use linked ledger reversals and an explicitly reviewed replacement, with history retained.

## Ledger and allocation

Publish through the existing immutable manual ledger rather than maintaining a second balance book. Charge creation never creates a received-money receipt. Confirming money already received continues to create the existing single immutable receipt. Allocation links a confirmed received entry or opening credit to a confirmed maintenance charge in the **same flat**; it neither creates nor renumbers a receipt. An operator explicitly chooses the source, destination, positive amount and reason. An operation identity binds the actor and exact payload, so repeated retries preserve one allocation and a changed retry fails.

Reserve the writer and recheck the current treasury actor, source and destination states and available amounts before allocation. Concurrent allocations must not overuse the credit or overfill a charge. Independently derive expected, allocated, outstanding and unallocated credit totals in paise. Preserve allocation history when an entry is reversed; an allocation whose source or destination is reversed has no current effect. An explicit allocation correction retains the original link and records its authorised reason. Other flats' credits never conceal this flat's arrears. An opening credit is labelled as an opening credit and never represented as money received or a new receipt.

When a published charge is corrected, keep its frozen published amount visible in history and distinguish it from the current active expected amount. Reconcile **active expected = live allocated + outstanding**, and report reversed charges separately. Available credit is unreversed confirmed credit less live allocations; a charge reversal makes its former allocation ineffective and releases that source amount for a new explicit allocation. Do not silently reassign it.

## Independent acceptance examples

| Supplied scenario | Expected result |
|---|---|
| Period one: A-101 ₹1,000.00 and A-102 ₹750.25 | Published expected total ₹1,750.25; two charges; zero receipts |
| A-101 received ₹400.00, allocated to period one | One ₹400.00 receipt; allocated ₹400.00; outstanding ₹600.00 |
| A-101 received another ₹800.00, allocated ₹600.00 | Period one paid; ₹200.00 available credit; receipt remains ₹800.00 |
| Period two: A-101 ₹900.00; allocate the ₹200.00 advance | Period two outstanding ₹700.00; no extra received entry or receipt |
| A-102 opening credit ₹100.25 allocated to period one | A-102 outstanding ₹650.00; no receipt for opening credit |
| Reverse the first ₹400.00 received entry | Its original receipt and allocation history remain; period-one A-101 outstanding becomes ₹400.00; period-two allocation is unchanged |
| Two simultaneous ₹300.00 attempts against ₹400.00 available | Exactly one succeeds; no negative credit or excess allocation |
| Charge supplied with a future due date | Unpaid amount appears; overdue amount is zero until that explicit due date has passed |
| Lost publish/allocation response and unchanged retry | Same cycle, charge and allocation identities; no duplicate ledger rows or receipts |

Also cover multiple periods, partial allocations across periods, reversed/corrected charges, rejected proposals, stale decisions, unsupported payloads, ended membership, unverified factors and role revocation during reads/writes. Compare expected amounts to explicit constants rather than repeating the implementation's calculation.

## Interface and checkpoint

Preserve the forest/ivory interface, editorial headings and shared accessible selectors/dialogs. Provide a maintenance workspace with period cards, clearly scoped expected/allocated/outstanding/overdue measures, a pending review queue and per-home statements. Forms show the supplied source and each participating home before submission; publishing explains the charges it creates. Allocation shows the original receipt or opening credit, remaining source amount, chosen charge balance and the post-action result. Residents get useful own-home empty and read-only views.

Verify actual preparation, separate review/publication, decline/withdrawal, credit allocation and correction controls; open selectors, keyboard/focus, dismissal, internal scrolling and desktop/tablet/375px/320px layouts. Exercise empty, delayed, interrupted, denied, stale and pending writes. Capture and inspect screenshots. Add bounded read-only native WebMCP tools that follow the same scope, reject cancellation/unsupported input and expose no financial mutation.

Before acceptance run the coherent Go race/format/vet, TypeScript/build, ordinary rendered and actual native WebMCP gate; prove preserving schema upgrade, snapshot/restore and Windows cross-build. Keep the verified live preview pinned during development. Record actual findings and evidence, then publish the accepted checkpoint to personal flux-i only.

## Remaining maintenance work

After cycle/allocation acceptance, complete upkeep tasks, assignments, scheduled vendor visits, asset/AMC and inspection deadlines with their own operational permissions, complete interaction checks and overview links. This brief does not mark that part of step 4 complete. Campaigns/external-payment reports, private incident/fine decisions, contacts/messaging, financial statement types/exports, community additions and PWA/migration/production preparation remain in the full roadmap.
