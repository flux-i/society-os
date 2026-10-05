# Maintenance cycles and receipt allocation — checkpoint evidence

**Verified locally on 5 October 2026: release 0.10.0-dev/schema 9.** The coherent captured gate passes 93 Go test declarations, 91 ordinary cases across 14 isolated synthetic suites and 20 actual native Chrome 154 WebMCP cases. The matching retained preview is ready at `http://127.0.0.1:8080`; account and ledger history survive the schema upgrade. Production acceptance remains separate.

The [expectation brief](maintenance-workflow.md) was saved before implementation. A Treasury operator prepares supplied per-home amounts and an explicit period/due date. A different currently eligible Treasury reviewer publishes the frozen proposal and the matching immutable ledger charges atomically. Preparing/publishing charges issues no receipt. Current financial scope controls proposals, amounts, home statements and bounded native reads.

## Executed evidence

- Formatting, vet, race, TypeScript and production build pass. The first full race run passed 93 declarations: database 250.511 s, backup 45.814 s, HTTP/server 66.457 s; the final unchanged-code run reused valid Go test cache results.
- Ten rendered maintenance cases pass, including bulk preparation of 118 explicitly supplied ₹1.00 amounts, separate review, rejection/withdrawal, original receipt allocation/correction, response-loss retries, stale concurrent decisions/allocations, resident scope and four viewport/menu journeys. The final coherent maintenance suite passed in 38.2 s; an earlier capture-corrected run passed in 37.5 s.
- Actual native Chrome 154 WebMCP passes 20 cases in the final coherent gate (30.5 s), adding three maintenance journeys. This fixture advertises 16 administrator tools, 14 financially entitled owner tools and ten for an unentitled tenant. The new tools only read bounded metadata; they cannot publish charges or allocate money.
- The Overview combined-source-outage expectation was updated from five to six independent sources and verifies unknown maintenance values. All ten affected Overview cases pass in 28.3 s. The original retained failure is a QA expectation correction, not a product defect.
- Windows amd64 cross-build passes. The executable has not run on the supplied production computer.

The coherent gate is `reports/local/maintenance-eval-final.log`. All 57 maintenance captures were inspected: the main 55 through ten contact sheets, with critical originals checked separately; two additional small-phone review-action captures received an affected-case rerun (4.8 s). No application code changed after the full gate. The populated 320px Overview original also includes the new maintenance panel. The private hash/dimension/inspection manifest is `reports/local/maintenance-review/review.json`. Targeted or repeated runs never add cases to the inventory.

## Financial behavior and scope

Exact paise examples compare independent constants: supplied ₹1,000.00 plus ₹750.25 creates ₹1,750.25 of charges and zero receipts. Explicit allocation of a ₹400.00 received entry leaves the first home ₹600.00 outstanding and preserves the original receipt identity/amount. Domain cases additionally cover opening credit without receipts, advance credit allocated across periods, source/charge reversals, linked allocation correction and simultaneous attempts that cannot overuse credit or overfill a charge.

Live active expected equals live allocated plus outstanding. Frozen requested amounts, reversed charges and available credit remain distinct. A future due date has unpaid amounts and zero overdue. A home statement includes other manually confirmed opening/charge/credit entries, while maintenance totals only include maintenance participation. Reversals retain the old receipt and allocation history, release ineffective links and never reassign credit automatically.

Proposal decisions and financial links use actor-bound exact-payload retry identities, writer reservation and current Treasury/factor authority. A changed retry, stale version or narrowed authority is rejected. Residents receive only published participation/current financial homes, excluding source notes, private review history and other homes. A native response captured before one home entitlement ends is discarded even when another home preserves the broad financial permission.

## Closed findings and QA corrections

Three repaired product groups closed: MA-01 future due dates rejected by the past-only entry-date validator; MA-02 tablet home-statement control overflow; MA-03 adjacent review action buttons without spacing. The independent future-date assertion, four viewport geometry checks and actual action captures verify the repairs. Process totals are 46 documented groups: 45 product/UI and one operational, up from 43 at account administration.

Incorrect seed-home expectations, bounded-term fixture setup, locator/wait corrections, native test ordering after fixture mutations and the obsolete Overview source count are QA corrections. Capturing a modal above a scrolled background clipped the screenshot while measured live bounds kept its close control visible; QA now stabilises/restores only the page background and asserts the live close bounds. Finite arrival/menu animations are allowed to finish before capture. Error/correction captures scroll their actual subject into view. None of these artifact/expectation corrections adds a product finding.

## Recovery and remaining work

The independently checked pre-upgrade schema-8 bundle is 471,040 bytes with SHA-256 `a17ac0fd2039f56a8680933d466d07b0ff877e6ea30e58a1aff9df40e5a22b0c`; fresh restore 16.703 ms on the development Mac. Hashes, integrity, foreign keys, four identities/three appointments/nine audit rows and empty restored temporary credentials agree.

The post-upgrade schema-9 bundle is 540,672 bytes, SHA-256 `13600b00a3b987acccae70a7792cc36436332a8708ac47d113afefc6f287d43e`; fresh restore 20.449 ms on the development Mac. Independent hashes, integrity, foreign keys, schema and empty temporary credentials pass. Twelve prior registry/account/factor/audit/ledger tables match the pre-upgrade rows exactly; nine audit rows are preserved. Private evidence: `reports/local/maintenance-recovery-verification.json`.

The separate meaningful maintenance recovery case includes published charges, received money/original receipt, active and corrected allocation history and factor/authority, excluding later reversals after restoring the checkpoint. The otherwise empty maintenance tables in the user's preview snapshot cannot prove that richer workflow by themselves.

[Upkeep tasks, assignments, vendors and asset/AMC/inspection deadlines](upkeep-baseline.md) subsequently passed their own 0.11 checkpoint, completing local step 4. [Fund campaigns and externally paid reports](collections-workflow.md) are next; the overall roadmap remains active. Actual Windows operation, real policies/finance authority, real data, provider onboarding and production infrastructure/custody acceptance are separate inputs. No payment initiation, real provider messages or runtime LLM is added.
