# How we build and verify Society OS

**Recorded:** 4 October 2026; **updated:** 5 October 2026. **Last closed local checkpoint:** release `0.11.0-dev`, schema 10: upkeep tasks, separate completion checks, published resident snapshots and private registers. Earlier checkpoints cover expanded UI (`0.3.0-dev` / schema 3), manual Entries/Receipts (`0.4.0-dev` / schema 4), approvals/notices/WebMCP (`0.5.0-dev` / schema 5), complaints/service requests (`0.6.0-dev` / schema 6), private documents (`0.7.0-dev` / schema 7), changing Overview (`0.8.0-dev` / schema 7), role/account administration (`0.9.0-dev` / schema 8) and maintenance cycles/allocations (`0.10.0-dev` / schema 9).

**Next planned workflow:** fund campaigns and reports of money already paid externally. Its expectation brief is saved before code; implementation/acceptance and the remaining roadmap are still ahead.

Our approach is to build a complete user workflow, use it in a rendered browser, inspect what actually appears, fix the problems found and preserve the checks for the next change. The user sets the product direction and quality standard; the coding agent implements and investigates within that scope. Screenshots and concrete interactions keep that collaboration grounded in the application people will use.

The closed checkpoints have corrected **48 documented product/operational finding groups**: 19 in the earlier UI review, four in manual Entries/Receipts, six in approvals/notices/WebMCP, two complaint findings, one preview-isolation finding, three document findings, three Overview findings, five account findings, three maintenance findings and two upkeep findings. That is 47 product/UI groups and one operational group. The latest `0.11.0-dev` gate passes **101 Go test declarations, 102 ordinary Chromium cases and 23 actual native Chrome 154 WebMCP cases**. Historical `0.10.0-dev` totals were 46 corrected groups, 93 Go declarations, 91 ordinary cases and 20 native cases; `0.9.0-dev` had 43 groups, 83 Go declarations, 81 ordinary cases and 17 native cases; `0.8.0-dev` had 38 groups, 72 Go declarations, 69 ordinary cases and 12 native cases; `0.7.0-dev` had 35 groups, 64 Go declarations, 60 ordinary cases and ten native cases; `0.6.0-dev` had 32 groups, 53 Go tests, 48 ordinary cases and eight native cases; `0.5.0-dev` had 29 groups, 46 Go tests, 41 ordinary cases and six native cases; `0.4.0-dev` had 23 groups, 34 Chromium tests and 39 backend tests. Tests, homes, controls and screenshots are coverage evidence, not additional bug counts. Test/capture/environment corrections are recorded separately from product findings below.

## How the process developed

The initial direction was a society portal with a beautiful interface and useful community information. User feedback refined the overview to distinguish occupied/vacant homes from owners and rental tenants. Later scope clarification established that finance workflows would record manually supplied entries and money already received. The portal would not initiate payments; gateways, automated billing, bank automation and Tally integration would be future decisions.

We delivered the local foundation, scoped registry and account-security workflows in successive checkpoints. That made each slice usable while production hardware, society policy and real-data inputs remained unknown. Fictional data let local work continue without presenting those missing production decisions as settled.

The first rendered UI review passed **14 browser tests**, but its coverage was insufficient. It checked important journeys and viewport behavior without inspecting open dropdown menus or inventorying every control. The user's screenshots showed uneven alignment, heavy and doubled borders, and native grey/blue selection menus that broke the approved visual direction. A passing suite had answered the questions it contained; it had not answered those visual questions.

The response was to strengthen the checkpoint itself. We opened the actual menus, checked selected and hovered options separately, used the keyboard, measured geometry, checked menus inside dialogs and widened the control inventory. The expanded suite reached 26 tests and exposed additional navigation, focus, feedback and pagination problems. The user approved those fixes and asked us to keep this approach for subsequent checkpoints. [Detailed UI review and coverage](ui-review-baseline.md).

## The workflow we repeat

```mermaid
flowchart LR
    A[Define a user outcome and expected results] --> B[Build one complete workflow]
    B --> C[Run code and permission checks]
    C --> D[Render, interact and inspect screenshots]
    D --> E{Expected behavior and usable UI?}
    E -->|Problems found| F[Reproduce, classify and fix]
    F --> C
    E -->|Checks pass| G[Record evidence and limits]
    G --> H[Reviewable checkpoint and next workflow]
```

1. **Define the outcome before implementation.** Specify who can perform the action, the inputs, the expected result and relevant failure cases. A registry update must preserve its history and reject a stale edit. A received-money entry must issue one receipt after confirmation, even when a response is lost and the action is retried.
2. **Build the smallest complete workflow.** Connect the UI, API, permissions, persistence and recoverable states needed for that outcome. Avoid a screen that merely looks functional or a placeholder assertion that implies unfinished behavior works.
3. **Run checks appropriate to the change.** `make check` runs formatting, Go vet, race-enabled backend tests and TypeScript. `make build` verifies the production frontend and Go binary. The current `make eval` gate combines those checks/builds with ordinary browser suites and a separate native-WebMCP browser suite. Domain expectations, permission denials, transactions and retries receive meaningful assertions.
4. **Exercise the real interface.** Sign in, navigate, open dialogs, click visible options, type, scroll, dismiss and return. A direct API call can prepare a fixture or prove server behavior; it cannot establish that the corresponding UI control is visible, clickable or understandable.
5. **Inspect rendered states.** Capture the visible viewport, wait for fonts and stable rendering, and inspect the screenshots. Open menus must be captured while open. Check the active, selected, hovered and focused states, plus empty, loading, failure, retry and pending actions.
6. **Classify failures before changing code.** Distinguish a product defect from a wrong test locator, an unstable capture or an incorrect expectation. Fix the actual cause and keep the original user outcome intact.
7. **Verify the repair and the surrounding workflow.** Rerun the affected checks after each change. Run the complete relevant suite before closing the checkpoint, including shared controls used by earlier workflows.
8. **Record what was executed and what remains unknown.** Keep the result, fixture, browser, viewports, observed issues, fixes and evidence locations. Report specific coverage rather than saying every possible click was checked.

The repository's [working agreements](../AGENTS.md) make this routine part of development. The [design system](design-system.md) keeps the approved forest/ivory palette, editorial typography and accessible controls consistent as the product grows.

### Working together

The user steers priorities, scope and visual judgment. The implementation agent turns those decisions into a runnable workflow, investigates failures and reports evidence. For this document, the user explicitly requested a separate agent: it reads the existing milestone reports and tests, reconciles the counts and owns this documentation file while implementation continues. Separate file ownership keeps that evidence work independent of changes to the live preview, migrations and posting logic.

Society representatives still need to approve real registry data, accounting examples, receipt policy and custody procedures. Those decisions stay visible in the backlog so local synthetic progress has a clear handoff to production acceptance.

### Keeping scope changes durable

While the private-document checkpoint was being verified, the user expanded the product direction: an Overview focused on changing critical information; maintenance and special funds tracking externally sent money; targeted WhatsApp/email to registered contacts, including receipts; photo/flat/rule reports reviewed before fines; and externally prepared income statements/balance sheets uploaded and intentionally shared. The [society operations roadmap](society-operations-roadmap.md) records those requests, priorities, workflow states, permissions and acceptance checks. The main plan, backlog and strategy now reference it and supersede the earlier deferrals it changes, so the new scope survives beyond the conversation.

These are accepted development requirements, with implementation and production activation still pending. Overview follows the now-closed private-document gate, using verified records, approvals, complaints, notices and documents before adding metrics for later modules. Keep finance and audience restrictions on both a count and its destination; preserve the approved visual direction. Subsequent workflows receive the same bounded implementation, rendered interaction, screenshot, native-tool and recovery checkpoints. Writing the roadmap adds no completed feature, test pass or corrected finding. External payment tracking does not authorize platform-initiated transfers; live messaging still depends on configured society-owned channels and accepted recipient policy.

## Corrected findings at the completed UI checkpoint

The count is **10 findings from the initial review plus nine from the expanded review: 19 issue groups**. Each row below is counted once, even if it affected several viewports or controls. Related styling symptoms are grouped together; repeated checks or screenshots do not increase the count. The earlier milestones do not maintain a complete historical bug ledger, so this is a documented UI-review total, not an estimate of every defect ever corrected in the project.

| ID | Observed problem and user impact | Corrected behavior | Evidence |
|---|---|---|---|
| UI-01 | At 1280×720, the sidebar profile began below the visible screen | The profile remains available while navigation scrolls independently | [Viewport review](../web/tests/ui-review.spec.ts) |
| UI-02 | Phone navigation clipped Account security | A labelled Menu exposes permitted routes and handles Escape and focus | [Viewport review](../web/tests/ui-review.spec.ts), [shell interactions](../web/tests/interactions.spec.ts) |
| UI-03 | Verification inherited the welcome screen's scroll position | Authentication transitions and recovery-code enrollment begin at the top | [UI review](ui-review-baseline.md) |
| UI-04 | Long invitation and home forms hid their actions | Dialog bodies scroll internally, with accessible close and action controls | [Viewport review](../web/tests/ui-review.spec.ts) |
| UI-05 | Background scrolling and interior blank clicks made dialogs behave incorrectly | Background scroll is locked; backdrop dismissal checks the pointer's origin and bounds | [Viewport review](../web/tests/ui-review.spec.ts), [shell interactions](../web/tests/interactions.spec.ts) |
| UI-06 | Identity confirmation and password change shared a current-password value | Each form has independent input state | [Security form checks](../web/tests/ui-review.spec.ts) |
| UI-07 | Changing a person search retained the previously selected identity | Search changes clear the selection and verification; a new selection requires verification again | [Invitation checks](../web/tests/ui-review.spec.ts) |
| UI-08 | Every invitation error offered Account security, even when that route could not resolve it | The reauthentication route appears for the relevant error | [Lookup and invitation error checks](../web/tests/interactions.spec.ts) |
| UI-09 | Recovery-code replacement offered “Continue to workspace” but remained on security | Replacement says “Done, codes saved”; initial enrollment retains its workspace action | [Security interactions](../web/tests/interactions.spec.ts) |
| UI-10 | Narrow desktop navigation compressed an icon to fit a label | Icons retain their width and spacing fits the narrower sidebar | [Viewport review](../web/tests/ui-review.spec.ts) |
| UI-11 | Native menu styling, uneven filter geometry, heavy focus rings and doubled wing borders broke visual consistency | Nine dropdowns share the approved menu treatment; controls align at 48px; selected and hovered states are distinct | [Filter checks](../web/tests/filters.spec.ts), [form menus](../web/tests/form-controls.spec.ts) |
| UI-12 | “See all homes” retained the last explored wing | General homes routes reset that one-time wing intent | [Overview and registry interactions](../web/tests/interactions.spec.ts) |
| UI-13 | Internal dialog completion buttons lost focus on the opening control | Cleanup closes the native modal before restoring opener focus | [Dialog and invitation interactions](../web/tests/interactions.spec.ts) |
| UI-14 | Tab cycling in About could move focus outside the dialog | Tab and Shift+Tab wrap among visible, enabled controls | [Shell interactions](../web/tests/interactions.spec.ts) |
| UI-15 | Failed account loading offered no way to retry | The error state includes a working “Try again” action | [Account failure checks](../web/tests/interactions.spec.ts) |
| UI-16 | A person-lookup error remained after the search changed successfully | Lookup feedback has independent state and clears with a changed search | [Lookup failure checks](../web/tests/interactions.spec.ts) |
| UI-17 | Security feedback appeared far above the submitted form | Each security card displays its own visible confirmation or error, including phone password mismatch | [Phone security interactions](../web/tests/interactions.spec.ts) |
| UI-18 | Voluntary authenticator enrollment promised a workspace transition but returned to security | Embedded enrollment says “Return to account security” and resets scroll | [Optional enrollment checks](../web/tests/interactions.spec.ts) |
| UI-19 | Pagination could label a requested page while showing the previous response | The displayed page number follows the returned page data | [Registry pagination checks](../web/tests/interactions.spec.ts) |

UI-05 covers the original backdrop/background behavior; UI-13 and UI-14 cover separate completion-button and keyboard failures. UI-09 concerns recovery-code replacement; UI-18 concerns optional authenticator enrollment. They are related components, but distinct observed journeys. Replacement dropdown implementation details, such as native form bridging and dialog portals, are not added as extra bugs unless a separate reproduced finding is recorded.

## What the numbers establish

The following table isolates the earlier `0.3.0-dev` UI/account-security evidence. The closed manual-records results follow it; milestone totals are not added together.

| Measure | Completed scope | Interpretation |
|---|---|---|
| Corrected findings | 19 documented UI issue groups | The table above; a bounded, traceable total |
| Browser tests | 26 passing Chromium tests; final recorded run took 1.1 minutes | Automated regression checks for the specified scenarios; elapsed time describes that run |
| Homes opened | All 118 fixture home cards across ten pages | Detail titles and opener-focus return checked for each card |
| Dropdown controls | Nine: wing, occupancy filter, home occupancy, person record, existing person, relationship, relationship to end, invitation person and invitation access | Actual open menus, visible options and applicable keyboard/selection behavior exercised |
| Expanded captures | 36 PNGs in `reports/local/interaction-review/` | Visual evidence generated for review; the count does not imply every image represents a different defect |
| Backend acceptance | 31 tests at the account-security checkpoint | Registry constraints, permissions, sessions, transactions, audit, MFA, invitations and recovery covered; not 31 observed bugs |
| Fixture | 118 homes, 154 people, 155 current/historical memberships | Fictional repeatable input, including joint/multiple-home owners and former relationships |

The UI review includes 2048×1119 comparison captures, 1280×720 desktop, 1024×768 narrow desktop, 768×1024 tablet, 375×812 phone, 320×568 small phone and 375×500 reduced-height layouts. These sizes were used in specific checks; they are not a claim that every interaction ran at every size. Seven form dropdowns were inspected inside dialogs at desktop and phone sizes. Menu hit testing supplemented visibility assertions to detect clipping. [Executed inventory and limits](ui-review-baseline.md).

The historical checkpoint reports are [foundation](foundation-baseline.md), [registry/identity](registry-identity-baseline.md) and [account security](account-security-baseline.md). Their successive browser totals are milestones, not additive independent coverage. The expanded private artifact manifest records the 26-test result, 118 opened cards, nine controls and 36 capture filenames. Generated images and manifests remain ignored by Git; a fresh clone reproduces them through the capture command below rather than containing resident data or local screenshots.

The [manual-records checkpoint](manual-records-baseline.md), release `0.4.0-dev` / schema 4, passes **34 Chromium tests in 1.7 minutes: the original 26 plus eight finance journeys**. `make check` passes **39 backend tests with race detection**, formatting, vet and TypeScript; the production build also passes. After the final PDF-spacing adjustment, document/server race tests and two targeted real-receipt browser journeys passed again; the latter took 7.1 seconds.

Its private `reports/local/records-review/` evidence contains **29 browser screenshots, one receipt raster and one downloaded PDF**. Generated contact sheets are excluded from those original capture counts. Selected desktop entry/form/receipt screens, 320px menu states, phone failure feedback and the receipt raster were visually inspected. The capture count does not claim that every screenshot received the same detailed scrutiny. The new journeys cover draft creation/review/confirmation, download/reversal, lost-response retry, pending-dialog behavior, opened controls and validation, list/download recovery, resident/tenant scope, 13 matching drafts over two pages, failed-PDF retry and draft discard.

## Applying the same approach to manual records and receipts

The manual-records workflow is more than an entry form. An authorized finance operator saves a draft, reviews it and confirms the supplied facts. An optional source/evidence note records the supplied provenance. A draft does not affect the confirmed balance or issue a received-money receipt; discarding it preserves history without a balance or receipt effect. Registry administration alone does not authorize finance posting. A resident's home relationship does not automatically grant financial access.

The completed local checkpoint verifies the following expectations:

| Expected outcome | Why we verify it | Relevant implementation checks |
|---|---|---|
| Amounts are exact paise | ₹0.01 must remain one paise; malformed or overprecise values must not silently change | [Amount and ledger checks](../internal/database/records_test.go) |
| ₹1,000 opening debit and ₹400 confirmed received money produce ₹600 due | The expected result comes from an independent example, not the application's own balance calculation | [Draft, confirmation and reversal checks](../internal/database/records_test.go) |
| Lost responses and concurrent retries produce one draft/receipt | The same operation key must return the original result; changed input under that key must conflict | [Retry checks](../internal/database/records_test.go), [browser retry journey](../web/tests/records.spec.ts) |
| Confirmation preserves the entry, receipt identity, audit and PDF job together | A PDF failure must not undo a saved financial record or issue a new receipt number | [Transactional and job checks](../internal/database/records_test.go) |
| Corrections preserve the original and record a linked reversal | Silent edits/deletion would obscure what was originally confirmed and why it changed | [Immutable correction checks](../internal/database/records_test.go), [HTTP workflow](../internal/server/records_test.go) |
| Expired worker leases cannot publish stale results | A replacement worker must be able to recover work while an old worker is fenced out | [Lease recovery checks](../internal/database/records_test.go) |
| PDFs remain private and current finance scope is rechecked on download | Possessing a receipt identifier must not grant another home's data; revoked access must stop new downloads | [Scoped HTTP downloads](../internal/server/records_test.go), [private-file checks](../internal/documents/receipts_test.go) |
| The rendered PDF is readable and matches the frozen receipt data | A `%PDF-` header or successful generation alone does not prove the amount, wrapping and layout are correct | PDF rendering, text extraction and visual review at checkpoint closure |

These tests are engineering evidence for a fictional local workflow. Society-approved entry examples, receipt format, finance policy and real-world authority remain separate acceptance inputs.

Failure cases have explicit provenance. Browser checks inject failed HTTP responses and lost responses; the PDF-retry journey marks a receipt job `FAILED` in its disposable database, then uses the visible retry action and the real worker to regenerate it. Lease recovery, stale-worker rejection and private-file integrity are checked separately. This proves the specified recovery contracts without representing the fixture as a real disk outage or a production storage exercise.

### Findings corrected in the manual-records checkpoint

The new browser journeys and screenshot review reproduced **four additional product/visual findings**. Their repairs and relevant regression checks pass, bringing the documented closed total to **23 groups: 19 earlier plus four current**. As in the earlier table, related record-filter styling symptoms are counted as one group.

| ID | Reproduced problem | Expected repair and verification | Status |
|---|---|---|---|
| FIN-01 | A cross-home financial entry lookup returned 503 instead of the intended 404 | Map a scoped missing row to 404, without exposing the record or presenting it as a server outage; assert the exact HTTP status | Verified corrected |
| FIN-02 | PDF-download failure feedback appeared below the phone viewport | Place feedback by the submitted action and verify it is visible in the phone dialog; retry must still work | Verified corrected |
| FIN-03 | Later native invalid inputs displaced focus from the required home selector | Keep the first required visible home control focused and accessible when submission is blocked | Verified corrected |
| FIN-04 | Record filters inherited a flex layout despite grid-column rules, and status typography referenced an undefined font variable | Use the intended grid and full-width controls, a shorter all-homes label and the defined body-font token; inspect the resulting layouts | Verified corrected |

The review also found **three test/capture problems**: a search input was located as a textbox although its accessible role is `searchbox`; a screenshot was captured during an entry fade animation; and a pagination check inspected page two before awaiting its response. Correcting the locator, stabilizing capture and waiting for the returned page improve the evidence. These are not counted as application bugs or shipped product fixes.

The first six new browser cases all failed, but six failing tests did not mean six different product bugs: three exposed product defects, while a shared search-role locator affected the other three. After those repairs, five of six passed; the remaining pagination failure was the missing wait described above. Classifying the failure causes prevented changes to working product behavior just to satisfy incorrect test machinery. The final complete regression, including the subsequently added retry and discard journeys, passes.

## Closed checkpoint: approvals, notices and native WebMCP

Documentation continues while implementation and evaluation run. The user explicitly asked us to keep recording the process as we go. This section was updated during development, with partial results labelled as partial, then closed after the final gate and screenshot review passed. [Approvals/notices acceptance](approvals-notices-baseline.md).

### The delivered local user outcomes

Residents can submit maintenance, registry-change, expense and notice proposals. An authorized reviewer completes MFA and fresh identity confirmation and must be a different person for approval, decline or requested changes. Review events preserve actor, reason, version and submitted content; a stale decision must not overwrite another reviewer's completed action. Requested changes can be revised/resubmitted, and an author can withdraw an eligible request. Approval records a decision; maintenance and registry changes still require their relevant follow-up. An approved expense proposal authorizes follow-up within this workflow; it does not post a financial entry or move money. [Review implementation](../internal/database/reviews.go), [domain checks](../internal/database/reviews_test.go).

Notice proposals become visible only after separate approval. The ordinary notice view applies current resident/audience permissions and omits private review history. Owner-only, tenant-only, committee and selected-wing audiences have separate expectations. Archiving removes a notice from the published view. A WhatsApp share control prepares a link for a human to send; it does not send a message automatically. [Rendered review journeys](../web/tests/reviews.spec.ts), [notice HTTP checks](../internal/server/reviews_test.go).

The portal is also registering tools with the browser's real `document.modelContext` API. This advances the earlier global browser-tool installation into an application integration. Native Chrome tests discover and execute the registered tools; a mock registration or an application function called directly would not establish that boundary. Four tools read permitted homes, records, requests and notices; two open an authorized home or workspace screen. Approval, financial posting, publication and permission changes still use the ordinary visible forms. [Application registration](../web/src/webmcp.ts), [native browser journeys](../web/tests/webmcp.spec.ts).

Tool availability follows authentication, completed MFA and permissions. Execution rechecks the current account, validates bounded arguments and uses the ordinary server scopes. Dialogs must remain intact when navigation is rejected. Logout/session revocation must remove advertised tools as well as stop protected data from being returned; cancellation must stop the operation. Native-WebMCP coverage runs separately from ordinary browser coverage, with its own browser configuration and artifact directory. Browsers without the experimental API continue to use the normal screens.

### Findings recorded during this checkpoint

The following **six product/visual groups are corrected and verified**, bringing the recorded total to **29: the historical 23 plus these six**. The later narrow-phone select is a continuation of REV-02, not an additional counted group.

| ID | Observed finding | Repair recorded | Closure |
|---|---|---|---|
| REV-01 | Request dialogs displayed headings without the intended editorial styling | Apply the shared heading treatment and inspect the rendered dialog | Verified corrected |
| REV-02 | Review fields appeared inline or too narrow inside the dialog; a later phone capture exposed a remaining narrow select | Restore full-width, labelled field layout; exclude form selects from mobile filter-specific CSS and recheck narrow-screen controls | Verified corrected |
| REV-03 | Selecting a specific optional home provided no route back to shared/no-home | Add and exercise “Shared area / no specific home” in the same selector | Verified corrected |
| MCP-01 | Revocation stopped protected execution but left tools advertised | Clear the signed-in integration after revocation and assert an empty native tool inventory | Verified corrected |
| REV-04 | The expanded mobile Menu clipped later navigation on short screens | Allow the expanded menu to scroll and exercise all eight links on the short-screen layout | Verified corrected |
| REV-05 | A request error card and its retry action fell below the phone viewport | Center the feedback with immediate scrolling and assert that both the complete alert and retry button are in the viewport | Verified corrected |

The process also uncovered separate QA problems. They are recorded by cause, without being added to the product-defect total:

- Native Chrome masks the application's thrown error text; checks use rejection and resulting state rather than requiring unavailable exception wording.
- Concurrent browser suites shared an artifact location; ordinary and native-WebMCP outputs now have separate destinations.
- Mobile navigation was hidden when a helper chose its route, and keyboard-focus checks needed to await the resulting focus; helper branching and waits were corrected.
- An older assertion expected Community to remain a disabled “Next” item after Community became an implemented route; the expectation was updated to the current product scope.
- The expanded gate's many real logins hit the shared-IP throttle. Each ordinary spec suite now receives its own disposable database/server and real limiter. The production limiter was not weakened to make QA pass. [Isolated runner](../web/tests/run-browser.mjs).

Seven targeted review journeys and six native-WebMCP journeys passed; the first full `make eval` subsequently passed 41 ordinary browser cases across nine isolated suites and six native Chrome cases. Screenshot inspection then found the request retry feedback below the phone viewport and the remaining narrow select described in REV-02. The checkpoint stayed open while those repairs and stronger full-alert/retry visibility assertions were added. The final coherent gate passed after the repairs. This is another concrete example of why a passing test run and a completed visual review are separate conditions.

The final `make eval` passes formatting, Go vet, race-enabled tests covering **46 Go test declarations**, TypeScript and the production build, plus **41 ordinary Chromium cases across nine isolated suites and six native Chrome 154 WebMCP cases**. The final review/UI capture rerun passes **11 cases in 26.0 seconds**. That rerun repeats relevant cases; it is not added to the 41-case inventory. The private final gate log is `reports/local/reviews-eval-final.log`.

The seven review journeys cover separate review, notice revision/publication/audience/archive, decline/withdrawal, responsive opened menus/validation, lost-response retries, errors/stale decisions and pagination. The six native journeys cover actual discovery/execution, invalid inputs/dialog preservation/logout, resident/tenant scope, revocation, MFA/pre-aborted cancellation and reading a real submission-to-publication flow while its approval uses the visible form. Native coverage is tied to the tested experimental API/browser configuration; it does not establish all browsers or in-flight cancellation timings. End-to-end execution from an external Claude/Codex client remains unverified.

All **29 distinct review captures** were inspected through **five regenerated contact sheets**, with full-width phone selects and the complete phone alert/retry action confirmed. The sheets are derived review aids, not five additional independent captures or defects. The private manifest lives at `reports/local/reviews-review/review.json`; the [acceptance baseline](approvals-notices-baseline.md) records public evidence without committing those artifacts.

### Preserved preview and recovery evidence

The existing preview database was upgraded to `0.5.0-dev` / schema 5 and readiness returned HTTP 200. The schema-4 snapshot was retained before upgrade. A new private schema-5 snapshot, `var/snapshots/reviews-20261004`, is **364,544 bytes**, with verified SHA-256 `79459392054128f363c54607b74f3cf9ca376e2a1edb6e9fa6d80da0d059055c`. A fresh restore-check succeeded in **17.918 ms**, preserving three buildings, 118 flats, 154 people and 155 relationships. These are measurements of this local synthetic exercise.

A separate [recovery test](../internal/backup/reviews_test.go) verifies that approved notice content and review decisions survive while sessions are invalidated. The test establishes that workflow expectation; the snapshot size and timing describe the particular retained preview snapshot. A Windows amd64 cross-build also succeeds; target-machine execution remains pending.

### Hardware evidence arriving alongside development

The user supplied a Windows resource photograph: an Intel Core i5-8300H, 8 GB RAM, a 64-bit Windows/x64 system and SSD/HDD capacities in the 256 GB/500 GB classes. [Production hardware](production-hardware.md) records the relevant resources without the photograph or device/product identifiers. This improves the deployment inputs while preserving evidence privacy. Runtime performance, Windows operation, power/reboot behavior, backup recovery and concurrent-use acceptance on that actual machine remain to be demonstrated; development-Mac tests do not settle them.

## Closed checkpoint: complaints and service requests

**Status: local synthetic checkpoint complete, `0.6.0-dev` / schema 6.** This workflow follows [section 16 of the implementation plan](../housing-society-digital-platform-plan.md#16-complaints--service-requests). The final coherent gate and visual review passed after the repairs below. The earlier 29 closed groups plus two complaint findings and one operational preview finding bring the recorded total to 32. [Acceptance evidence](complaints-baseline.md), [workflow expectations](complaints-workflow.md).

The delivered outcome is a private home-linked case that its author can track through authorized handling, resolution and closure. Owning or occupying the same home does not automatically expose another person's case. The state set is `OPEN`, `ACKNOWLEDGED`, `IN_PROGRESS`, `WAITING`, `RESOLVED` and `CLOSED`; the server validates permitted transitions, their actor and required reasons.

| Expected outcome | Required acceptance evidence | Current status |
|---|---|---|
| A current resident creates a case for one of their active homes | Permitted creation succeeds; another home's identity and an ended membership cannot authorize a new case | Checked locally |
| The author sees their own case; a co-owner or other person in that home does not inherit it | List, detail, search and native browser-tool calls preserve the personal boundary, including direct identifier requests | Checked locally |
| MFA-protected administrators/committee handlers assign, prioritize and progress a case | Current role/factor permissions and the allowed transitions are checked on the server; stale versions cannot overwrite a completed change | Checked locally |
| Assignment names an active, role-eligible account | Expired, revoked or otherwise ineligible assignees are rejected; mentioning a vendor does not create an account or grant access | Checked locally |
| Residents add permitted comments, confirm resolved-case closure or request reopening with a reason | Actor, membership, state and reason rules permit the intended action and deny invalid transitions | Checked locally |
| Bounded immutable history preserves handling without exposing private staff notes | `STAFF_ONLY` notes are excluded from resident detail, visible-history counts, search, API responses and WebMCP results; private activity also leaves public metadata unchanged | Checked locally |
| An ended membership retains the author's own read-only history | Existing personal history remains readable under the explicit policy; creation, comments and case changes are denied after membership ends | Checked locally |
| Lost responses and concurrent retries preserve one recorded action | Actor-bound same-key replay returns the original result; changed payload conflicts; authorization is rechecked before replay | Checked locally |
| The complete case workflow remains usable across screen sizes | Actual forms, opened controls, history, feedback, retry, busy dismissal and keyboard behavior are rendered at desktop, tablet, 375px and 320px | Checked locally |

Earlier targeted runs passed seven visible complaint journeys and the expanded eight native WebMCP journeys. The final `SOCIETY_CAPTURE_UI=1 make eval` passes **53 Go tests with race detection and vet, TypeScript/build, 48 ordinary Chromium cases across ten fresh synthetic database suites and eight native Chrome 154 cases**. The native stage took **8.8 seconds**. The private full-gate log is `reports/local/complaints-full-eval.log`. Five complaint domain tests, one HTTP test and a separate recovery test extend the prior checks. [Domain](../internal/database/complaints_test.go), [HTTP/privacy](../internal/server/complaints_test.go), [visible journeys](../web/tests/complaints.spec.ts), [native reads](../web/tests/webmcp.spec.ts), [recovery](../internal/backup/complaints_test.go).

A subsequent capture-only change scrolls the internal dialog body to show the complete staff-note card. The complaint suite passes **seven of seven again in 17.9 seconds**, recorded in `reports/local/complaints-capture-final.log`; production code was unchanged after the full gate. All **32 distinct final screenshots** were reinspected through **six regenerated contact sheets**. Full-size close/error/stale states and the complete private-note card were also inspected. Headers, close controls, menu alignment and resident/staff content passed that review. Capture improvements, derived sheets and repeat test runs do not increase the product-finding or case totals.

API preparation alone does not establish the corresponding UI or native-tool boundary. Disposable synthetic databases, private captures and the complete checkpoint gate remain the working method. Any later attachments must follow the same visibility rules and receive separate document-validation/access evidence.

### Corrected findings and strengthened expectations

| ID | Reproduced finding | Repair/expectation recorded | Closure |
|---|---|---|---|
| CARE-01 | Error feedback's `scrollIntoView` could scroll the hidden-overflow native dialog itself, moving its heading and close control out of view | Scroll only the internal `.dialog-scroll` area; assert the complete close control is in the viewport and the dialog's own `scrollTop` stays zero | Verified corrected |
| CARE-02 | A resident received version 35 while seeing only two public updates, revealing activity in private staff notes | Use separate public version/time and public list ordering; give updates opaque identities; private comments leave resident metadata unchanged while public edits still support stale-write checks and actor-bound retries | Verified corrected |
| OPS-01 | The running verified backend served `build/web` while new frontend builds replaced those assets, allowing a mixed-version preview | Retain verified binary/assets together; the preview was first pinned to `0.5`, then handed over to verified `0.6` under `var/preview-releases/0.6`, preserving its database/MFA key | Verified for the running preview |

CARE-02 came from examining metadata, not just confirming that secret text was absent. The strengthened domain case adds 33 private notes and independently expects the resident's version/time to remain unchanged; after a public update, the resident sees version two and two visible history items, while staff can see all 35. Visibility filtering must occur before history pagination and counts. These distinctions also apply to API and native-tool responses. [Privacy expectations](../internal/database/complaints_test.go).

The QA fixture also initially attempted nonexistent email columns and was corrected to the actual `login`/`created_at` schema. That is a test-fixture correction, not an additional application defect. The staff-note capture adjustment is likewise evidence preparation. OPS-01 is an operational preview-isolation finding: pinning this running instance contains the mismatch; it does not claim that every future preview automatically receives immutable assets. The [pin-preview helper](../scripts/pin-preview.py) retains a binary/assets pair after it has been built and verified, refuses to overwrite an existing named release, and copies neither database nor keys. It does not run the evaluation gate itself.

### Coherent preview and recovery handoff

At the complaints checkpoint, the loopback preview used the pinned `0.6.0-dev` executable/assets and schema 6, with verified `0.5` retained. The pre-upgrade schema-5 snapshot, `pre-complaints-20261004`, has SHA-256 `79459392054128f363c54607b74f3cf9ca376e2a1edb6e9fa6d80da0d059055c`. The post-upgrade `complaints-20261004` snapshot is **409,600 bytes**, with SHA-256 `283a93c04ec0a30d5bbd50edf6b3f818b7f474dfe9ec76c90fce739d5c25e8f0`; its fresh restore succeeds in **19.487 ms**, retaining three buildings, 118 flats, 154 people and 155 memberships. These private local snapshot exercises do not establish target-host recovery time.

The new recovery test preserves both public and staff-only conversations while invalidating old sessions and keeping the private notes hidden from the restored resident view. Windows amd64 cross-compilation passes; execution on the supplied Windows computer remains pending. Native tools were exercised through the browser API; an external Claude/Codex client end-to-end run remains unverified. The current workflow neither initiates payment nor automatically posts an expense or applies a registry proposal. Private validated documents followed this checkpoint; their closed acceptance follows below.

Society emergency/security contacts and expected response-policy guidance still need acceptance. Submission must not claim that a person was notified, that a response deadline is guaranteed or that emergency work was dispatched. Automatic inactivity closure also requires an accepted policy before it can be enabled. These pending operational inputs remain separate from local implementation progress.

## Private documents: completed local checkpoint

**Local synthetic checkpoint complete, `0.7.0-dev` / schema 7.** The [document workflow brief](documents-workflow.md) defines the outcome from sections 9 and 17 of the platform plan: upload a private original, validate it, obtain a separate authorized review where sharing requires one, and retain its approved versions under current permissions. The final gate, rendered review and local recovery checks passed. The three document findings bring the historical 32 closed groups to 35. [Acceptance evidence](documents-baseline.md).

| Expected outcome | Required evidence before closure | Status |
|---|---|---|
| Private, home, society, committee, named-person and accounting scopes follow current entitlement | List, search, count, metadata, completion, decision, download and native reads deny unrelated or revoked access; financial categories require separate finance permission | Checked locally |
| Validation precedes sharing and download | Exact reserved size/SHA, actual image decoding, rejected active or encrypted PDFs, unsupported types and unavailable validation fail closed | Checked locally |
| Separate review preserves immutable originals | Current reviewer/MFA and fresh version/reason checks deny self-approval; replacement keeps the approved original until separate approval; withdrawal, decline and archive retain history | Checked locally |
| Retries and bounded processing preserve one operation | Actor-bound retry identities, quotas including retained/pending bytes, abandoned reservations, lease expiry, attempt limits and stale decisions have independent assertions | Checked locally |
| Original bytes survive local recovery | A consistent snapshot restores originals, checksums, versions and decisions while invalidating sessions | Checked in a dedicated synthetic original-byte test |
| The visible library is usable across screen sizes | Real uploads/downloads, opened controls, keyboard/focus, busy dismissal, errors/retry and desktop/tablet/320px/375px layouts are exercised and captured | Twelve document cases and final visual review pass |

The development adapter stores immutable original bytes and metadata/version history in **local synthetic SQLite**, so the existing consistent snapshot can include originals. Its implementation limits images to **20 MiB**, PDFs to **4 MiB**, retained/pending storage to **100 MiB per user** and **1 GiB per society**. These limits bound this local adapter; they do not establish production storage capacity. Uploading payment evidence creates neither a financial entry nor a receipt. Archiving stops new ordinary links while retaining originals; replacement does not silently overwrite the approved version.

Validation uses **30-second leases and at most three attempts**. PNG/JPEG files must actually decode within **8 million pixels and 8,192 pixels per dimension**. Installed qpdf **12.4.2** provides bounded PDF structure/active-content inspection with a **three-second deadline**, **8 MiB output cap**, **250-page limit** and parser limits; its JSON inspection does not decode stream payloads. These checks are neither antivirus nor proof of hard operating-system memory containment. Production S3/versioning, malware containment and off-site recovery of originals remain undelivered and disabled pending the society-owned AWS account and accepted policies. Local SQLite blobs must not be described as production S3. [Validation implementation](../internal/documents/validation.go), [document persistence checks](../internal/database/documents_test.go).

Native metadata search/detail tools bring the catalogue to **ten tool definitions**, with actual advertising governed by permissions. They return authorized metadata/version information rather than original bytes or download URLs. The gate separately passes **ten actual native Chrome WebMCP cases in 12.0 seconds**; catalogue size and test count are distinct measures. This proves the specified browser-API journeys, while an external Claude/Codex client end-to-end run remains unverified. [Native registration](../web/src/webmcp.ts), [native checks](../web/tests/webmcp.spec.ts).

### Corrected findings and verification

The following **three reproduced product groups are verified corrected** and counted once each. Multiple display repairs in DOC-03 are one presentation group; repeated tests and captures add no findings.

| ID | Observed problem | Repair recorded | Current evidence/status |
|---|---|---|---|
| DOC-01 | An uploader retained a former home's document access while a different home relationship remained active | Recheck membership in the document's specific current home rather than granting a general uploader/current-resident shortcut | Failing domain reproduction in `reports/local/documents-home-scope-red.log`, then regression/gate pass |
| DOC-02 | A phone contract upload rejected a valid future expiry because it reused a past-record date rule | Use a strict dedicated expiry calendar check from 1900 through 2100; retain existing past-record rules for historical entries | Failing domain reproduction in `reports/local/documents-expiry-red.log`, then domain/browser/gate pass |
| DOC-03 | Library display used incorrect shared hero/chips/history/principles classes; the status-filter label wrapped, and failed detail loading still said “Opening” | Restore approved shared styles; apply width to the actual select trigger instead of its `display: contents` wrapper and assert a single-line label; show “Document unavailable” after failed loading | Final captures inspected and browser/gate pass |

The first nine-case browser attempt was incomplete: **six passed, two failed and one was interrupted**. Those numbers do not establish nine accepted journeys or a product-defect count. QA corrections are recorded separately: an ambiguous category locator became exact; the fixture role label was corrected from “Committee member” to “Committee”; a reset buffer's aliased image fixture was corrected; and the synthetic active-PDF fixture received the required Resources dictionary. None is added to the product ledger. [Visible document journeys](../web/tests/documents.spec.ts), [fixtures](../web/tests/document-fixtures.ts).

The first full-gate attempt passed **64 Go test declarations with race detection/vet, TypeScript/build and all 12 document browser journeys**, producing **45 captures**, then stopped at an obsolete interaction assertion expecting the former disabled “Documents Next” button. The implemented navigation link was correct; the assertion now requires `href="#documents"`. This is a coverage-inventory update, not another product defect. Empty, detail-loading and detail-error recovery captures were added within existing cases. The final coherent `reports/local/documents-release-gate.log` passes **64 Go declarations with race detection/vet, TypeScript/build, 60 ordinary Chromium cases across 11 isolated suites and ten native cases**. Repeated runs are not added together.

The final visual evidence contains **48 distinct PNGs**. Forty-five were inspected through **eight contact sheets**; final changed/new captures were additionally viewed full-size. This is a reviewed state/viewport inventory, not 48 defects or proof of all possible inputs. The twelve document cases include visible upload/download, replacement/review/archive, evidence permissions, opened controls, keyboard, busy/retry/stale actions, pagination and empty/loading/error recovery. Private captures and the review manifest remain outside Git.

### Preview and original-byte recovery

At the documents checkpoint, the loopback preview was pinned to verified `0.7.0-dev` binary/assets and schema 7 at `127.0.0.1:8080`, with `/ready` returning 200 and the verified `0.6` pair retained. The pre-upgrade schema-6 snapshot has SHA-256 `0b811e08b12a1cda9fdd1ddbf39aab59f34ab011ab3dd23fd60d96d553f98e69`. The post-upgrade schema-7 snapshot is **462,848 bytes**, with SHA-256 `0b13b74ba9022276e251df58c3d15fcf11b611561003e851305bc7bc025e2e95`; a fresh restore succeeds in **17.323 ms**, retaining three buildings, 118 flats, 154 people and 155 memberships. This demo snapshot contains no seeded uploaded originals.

Original-byte recovery is established separately by the [document snapshot test](../internal/backup/documents_test.go): upload a fictional PNG, record a separate approval, snapshot and restore, reject the old session, sign in again and independently compare the downloaded bytes, checksum, approved state and revision. The demo restore timing is not an original-heavy performance benchmark or an off-site recovery claim. Windows amd64 cross-compilation passes; the actual Windows target has not been run.

Overview followed under the [published operations roadmap](society-operations-roadmap.md), committed as `7369921` under personal `flux-i`; the roadmap's other features remain planned. Real society files, production S3 custody, retention/hold policy, antivirus/containment and external-client integration still need separate acceptance. Metadata search is the initial document scope; OCR/text extraction and document links on notices or complaints need later bounded checkpoints.

## Changing Overview: completed local checkpoint

**Local synthetic checkpoint complete, `0.8.0-dev`, verified 5 October 2026; schema 7 unchanged.** The [Overview brief](overview-workflow.md) replaces a front page dominated by static home totals with changing information that leads to action: supplied financial balances, decisions, service cases, approved notices and document deadlines. Home/occupancy/owner/tenant counts remain on Homes & people. Every signed-in, MFA-cleared account gets Overview, while each section retains its own current permissions. [Acceptance evidence](overview-baseline.md).

The implementation has **five separately retriable sources**, full server-side counts and at most **four preview rows per source**. The combined attention queue shows at most eight preview rows with explicit partial scope and full-list destinations. Failed/loading sources show unknown rather than zero and leave other sections usable. Current audience and approved document-head rules control publications/deadlines; resident case metadata excludes staff-only activity. Financial balances are exact paise calculated per home before aggregation, so one home's credit cannot hide another's debit. The collection period is labelled; a supplied balance is not inferred overdue maintenance. Future maintenance, campaigns, messaging and fines supply metrics only after their workflows exist.

| Acceptance area | Executed checks | Status |
|---|---|---|
| Scoped source data and exact amounts | Full counts beyond four previews, separate review, current memberships/roles, hidden staff activity, per-home credits/reversals/drafts and calendar boundaries | Checked locally |
| Useful, recoverable navigation | Exact authorized destinations, full-list links, Back/refresh/close, denied/stale identities, independent loading/error/retry and refreshed state | Checked locally |
| Design and access across layouts | Desktop, tablet, 375px/320px controls, keyboard/focus, downstream menus/dialogs and native allow/deny reads | Checked locally |
| Preserved preview and recovery | Matching verified assets/binary, pre/post snapshots and fresh restore with schema unchanged | Checked locally |

### Corrected findings and final gate

| ID | Reproduced finding | Repair recorded | Current evidence/status |
|---|---|---|---|
| OV-01 | UTC aggregation disagreed with the existing Asia/Kolkata calendar around midnight, excluding today's received entry and misclassifying boundary-day contract expiry | Use the society calendar for the labelled day/collection window and deadline comparisons; assert the India-midnight boundary independently | Retained RED collection reproduction, then domain/HTTP/full-gate pass |
| OV-02 | Finance failure/retry appeared twice in the queue/panel, while denied request detail still claimed to be opening | Give each unavailable source one failure display; use an unavailable detail heading and scoped 404 explanation | Screenshot critique, browser regression and final review pass |
| OV-03 | Back from a linked case lost originating focus; disabled Refresh lost keyboard focus when reads completed | Restore focus to the prior link after Back, or main content when the item leaves the queue; restore Refresh focus after completion | Keyboard browser regressions and full-gate pass |

These are **three closed product groups**, bringing the earlier 35 groups to 38. The retained `reports/local/overview-calendar-red.log` expects 2,500 paise received but gets zero; the fixed boundary test independently expects 5 October and a 6 September period start at `2026-10-04T18:30:01Z`. Error/focus groups each include related manifestations of the recorded journey rather than counting every failed assertion. [Domain checks](../internal/database/overview_test.go), [HTTP checks](../internal/server/overview_test.go), [visible journeys](../web/tests/overview.spec.ts).

The coherent `SOCIETY_CAPTURE_UI=1 make eval` in `reports/local/overview-release-gate.log` exits successfully: **72 Go declarations with formatting/vet/race, TypeScript/build, 69 ordinary Chromium cases across 12 isolated suites and 12 actual native Chrome 154 WebMCP cases**. Seven new domain declarations and one HTTP declaration extend the backend inventory. The final nine-case Overview suite took **26.1 seconds**; the native stage took **14.0 seconds**. Native advertising has **11 tools for eligible users and ten for an unentitled tenant**; tool counts and executed cases are different measures. External Claude/Codex client execution remains unverified.

All **32 distinct final Overview screenshots** were inspected through **eight contact sheets**, plus full-size critical captures. The private `reports/local/overview-review/review.json` records hashes of the originals. Captures cover specified states/viewports rather than 32 defects or every future workflow. Unsupported labels/headings, incorrect role table/value fixtures, API fixture changes at the same URL without manual refresh, waiting for asynchronous native-dialog URL cleanup and instant scrolling to the top before capture were QA corrections, not extra product findings. Repeated runs and derived contact sheets do not increase totals.

### Preserved preview and recovery

At the Overview checkpoint, the loopback preview used pinned matching `0.8.0-dev` binary/assets at `127.0.0.1:8080`, with `/ready` returning 200, schema 7 unchanged and the verified `0.7` pair retained. The pre/post bundles, `pre-overview-20261005` and `overview-20261005`, are each **462,848 bytes**, with independently checked SHA-256 `302a3fe40703c4dbabd031e4481ec9284f158f10f29540f223c344edb2c2d03b`. A fresh restore succeeds in **34.637 ms** on the development Mac, preserving three buildings, 118 homes, 154 people and 155 memberships. The snapshots contain no seeded document originals; the meaningful original-byte recovery test separately passes in this full gate. Custody of the matching MFA key remains separate from the bundles.

Windows amd64 cross-compilation passes; the actual Windows target and production acceptance remain pending. Explicit role/account administration followed under the roadmap. Real provider configuration, society rules/policies, production custody and infrastructure still need their own evidence; local checkpoint closure does not claim those activations or all workflows are complete.

## Role/account administration: completed local checkpoint

**Local synthetic checkpoint complete, `0.9.0-dev` / schema 8.** The [account-administration brief](account-administration-workflow.md) was recorded on 5 October before implementation. A current administrator can inspect an account, grant/end a bounded appointment or suspend/resume it with an explicit reason. The coherent gate, final visual review, retained preview upgrade and recovery checks pass. Five account findings bring the earlier 38 groups to **43**; the OV-03 focus follow-up remains the same existing group. The verified checkpoint is published to public `flux-i/society-os` main as [commit fc9b01c](https://github.com/flux-i/society-os/commit/fc9b01c8bfd8f67dafc453a8943ca63fe6db41ca); repository ownership/visibility and the matching remote main were checked.

| Appointment | Delivered boundary, checked locally |
|---|---|
| Administrator | Registry/accounts and existing operational/non-financial document powers; financial posting is a separate entitlement |
| Committee | Existing community/registry, financial reads, review/service handling and non-financial document work; no account administration or financial posting |
| Treasurer | Financial reads, manual posting/corrections and accounting-document work, plus current registry read scope; no account administration or committee workflow powers |
| Accountant / auditor | Financial reads and permitted approved financial documents; no financial writes, registry-wide administration or operational handling |

Appointments start on confirmation, run **1–365 days** and retain history after ending. Duplicate overlapping appointments and self-changes are denied. Every mutation rechecks the current administrator, confirmed MFA factor and fresh password/applicable-factor confirmation within five minutes; expected version, reason, authority/identity attestation and immutable audit share one transaction. An attestation records the supplied authority rather than proving it. Ending an administrator appointment or suspending an account must leave an active, verified administrator with a confirmed authenticator and current appointment. This guard does not extend natural expiry; custodial handover still needs acceptance.

Suspension revokes target sessions, outstanding invitation/recovery links, temporary factor setup and appointments, while preserving identity, confirmed MFA, financial records and audit. Explicit resumption verifies identity/reason without reviving those sessions, links or appointments; any new appointment or invitation is a separate action. Schema-8 upgrade checks preserve schema-7 data, seed appointments and historical migration checksums.

Executed checks cover role boundaries, stale/revoked/pending-MFA actors, last-administrator/concurrent changes, suspension/resumption and meaningful recovery. Rendered journeys exercise grant/end/suspend/resume, required reasons/acknowledgements, opened selectors, keyboard/opener focus, busy/error/retry/conflict states and responsive dialogs. Native tools remain bounded reads, respect current/suspended scope and preserve open human forms; no role/status mutation tool is exposed.

| ID | Observed finding | Repair recorded | Status |
|---|---|---|---|
| AR-01 | AccountList labelled every non-administrator appointment “Committee”, including the demo Treasurer | Map each supported role to its accurate label in the role interface | Rendered role label and full gate verified |
| AR-02 | Account detail still succeeded after the confirmed MFA-factor row was removed, relying on an old verified session | Require a current confirmed factor when determining whether privileged MFA is pending | Retained factor RED, then targeted green and full-gate pass |
| AR-03 | An invitation invalidated by suspension still headed the unavailable page “Your place is ready” | Show separate unavailable/loading headings rather than promising access before the link is read | Failure screenshot/context, then browser/gate and final visual pass |
| AR-04 | Native tools retained stale appointment/permission state after Auditor expiry, repeatedly rejecting the resident's still-permitted personal home/finance reads | Refresh registration using the same complete permission and role signatures that tool execution compares; rediscover the current personal scope | Retained appointment-expiry native RED, then current-scope native/gate pass |
| AR-05 | Narrowed current access left the previous society-wide balance visible; a native read also returned a captured two-home response after one home entitlement ended while permission flags stayed true | Compute an opaque current-scope fingerprint in the authenticated transaction; use a shared frontend scope key to clear Workspace data and reject native results when scope changes before/after the read | Both scope REDs retained, then domain/ordinary/native gate and inspected current-scope captures pass |

These are **five verified account product groups**, counted once each. AR-02 is a current-factor authorization finding rather than a missing label or test expectation; its RED in `reports/local/account-factor-red.log` returned success where denial was required. AR-04's retained `reports/local/account-appointment-expiry-native-red.log` expects the resident's own A-101/A-102 homes but execution remains rejected. The independent green expectation gives an unrelated home a positive 3,725-paise charge: after Auditor expiry, filtered totals exclude it and direct detail access is denied while personal access remains. An initial assertion incorrectly expected directory denial instead of its established empty scoped response; that QA expectation is separate from the stale-registration defect. [Factor checks](../internal/database/account_administration_test.go), [role/invitation rendering](../web/src/components/Identity.tsx), [native scope checks](../web/tests/webmcp.spec.ts).

AR-05 strengthened that audit beyond registration and current server permissions. `reports/local/account-current-scope-native-red.log` reproduces a visible **₹37.25** society-wide balance after the auditor term ended and the server narrowed scope. `reports/local/account-membership-scope-native-red.log` independently returns a captured two-home result after one membership ended, even though the remaining personal permission flags stay true. Both concern invalidating data when actual scope changes, so they are one finding group. Screenshot/context/trace artifacts remain in the corresponding private RED folders. The final checks reject that obsolete response, remove stale visible details and preserve still-permitted personal access. A current endpoint's permission check alone does not invalidate a wider response already in flight or rendered.

The first resumed ten-case account browser run had **four passes and six failures**. Wrong home-card selection, a read-failure route that omitted query strings, outdated invitation assertions and a cascading leftover auditor appointment were QA corrections, not additional product findings. Later targeted passes and the preliminary **82 Go / 80 ordinary / 16 native** run were superseded by the two genuine scope REDs and stronger tests. They are retained investigation history, not final acceptance or additive test totals. The prior 72-declaration baseline included the command package's test; reconciling that inventory was not a defect.

### Final regression and visual evidence

The coherent `SOCIETY_CAPTURE_UI=1 make eval` in `reports/local/account-administration-eval-final.log` exits with zero: **83 Go declarations with formatting/vet/race, TypeScript/build, 81 ordinary Chromium cases across 13 isolated suites and 17 actual Chrome 154 native WebMCP cases**. Ordinary coverage includes **11 account cases and ten Overview cases**. The native stage took **23.6 seconds**; the targeted native scope run separately passed all 17 in **24.9 seconds**. Backend package timings in this gate were **207.209 seconds** for database, **38.420 seconds** for backup and **58.558 seconds** for server. These are measurements of this development run, not a controlled speed comparison.

The full gate also closes the **OV-03 follow-up**: after a delayed read, Refresh completion stole focus from a newly focused receipt link. The deterministic `reports/local/account-overview-focus-red.log` records the failure. Refresh now restores focus only when it remains on that control or the document body, preserving intervening navigation. This is strengthened coverage of the existing focus group and adds no account finding. A later capture-only adjustment scrolls the focused link into the viewport; the affected actual test passes again in **2.0 seconds**, recorded in `reports/local/account-overview-focus-capture.log`. No application code changed after the full gate, and the rerun adds no extra case.

All **51 final account captures** were inspected through **nine contact sheets**, including critical 320px originals, error/form states and current-scope detail/account-book/menu captures. The private `reports/local/account-administration-review/review.json` records original hashes/dimensions and completed inspection. Earlier account-book captures had shown loading; waiting for the loaded list and asserting page two's seven rows corrected the evidence expectation. That is QA work, not an extra product finding. Captures, contact sheets and repeat runs remain separate from bug and case counts.

### Verified preview and recovery

At the account checkpoint, the loopback preview was pinned to matching `0.9.0-dev` binary/assets and schema 8 at `127.0.0.1:8080`, with `/ready` returning 200 and verified `0.8` retained. The pre-upgrade schema-7 snapshot is **462,848 bytes**, with SHA-256 `75dde24235b03aa45de1ce976baad559150162df265237adc0c5009e493a4729`; its independent fresh restore took **18.9 ms**. The post-upgrade snapshot is **471,040 bytes**, with SHA-256 `a17ac0fd2039f56a8680933d466d07b0ff877e6ea30e58a1aff9df40e5a22b0c`; its fresh restore took **16.927 ms**. Independent integrity, foreign-key, schema-8 and matching-hash checks retain three buildings, 118 flats, 154 people, 155 memberships, four users and three grants; restored temporary credentials are cleared.

Upgrade checks preserve prior user/grant/factor rows and the eight pre-existing audit rows. The live preview legitimately appended one `MFA_RECOVERY_CODE_USED` audit after the older snapshot. Expecting whole-table audit equality was an incorrect QA assumption; the corrected check verifies preservation of prior rows. Using the proper `mfa_recovery_codes` table name was another verification correction. Neither is a product finding or a reason to remove valid history.

The meaningful [account snapshot test](../internal/backup/account_administration_test.go) also passes in the full gate. It restores nonempty auditor terms, suspended identity/version, revoked treasury appointment, confirmed factor and immutable access history; old sessions/recovery links are denied, while later resumption/revocation decisions stay beyond the snapshot boundary. Current permitted sign-in and auditor read scope are checked independently. Matching MFA-key custody remains separate from the bundle; local restore timings do not prove off-site or target-host recovery.

Windows amd64 cross-compilation passes; actual target execution, real custodian/authority acceptance and production infrastructure remain pending. Personal publication of `0.9` is verified. The [maintenance brief](maintenance-workflow.md) was saved before the following `0.10.0-dev` / schema 9 work: approved supplied cycles and explicit allocations of ledger credits first, then upkeep tasks. Creating charges or allocating existing credit must not invent a new receipt. Funds, incident/fine, messaging and statement work remains in the roadmap.

## Maintenance cycles and credit allocation: completed local checkpoint

**Local synthetic checkpoint complete, `0.10.0-dev` / schema 9.** This implements the previously saved [maintenance brief](maintenance-workflow.md): supplied per-home charges are reviewed by a separate currently eligible treasury operator, then published from a frozen proposal into the existing immutable ledger. Explicit same-home allocations use confirmed received money or opening credit and preserve receipt identities; charge publication and allocation issue no new received-money receipt. The coherent gate, final visual review and preserved-preview upgrade/recovery pass. Three findings bring the earlier 43 groups to **46**. [Acceptance evidence](maintenance-baseline.md). Publication to public personal `flux-i/society-os` main is verified as [commit e1bd2cd](https://github.com/flux-i/society-os/commit/e1bd2cd79f66cf6c75193709ea299f6ac688ab3a); Git and API remote hashes match.

Earlier targeted race-enabled checks passed **seven domain/migration tests in 27.628 seconds** and **one HTTP test in 6.234 seconds**. The independent checks cover separate treasury review, frozen proposals/retry identities, partial/advance/opening credit, preserved receipts, current scope, linked allocation correction/reversal, concurrent overspend prevention and schema-8 ledger/account-authority preservation. These are now retained by the full gate. [Domain checks](../internal/database/maintenance_test.go), [HTTP scope checks](../internal/server/maintenance_test.go).

| Independent fictional expectation | Executed backend evidence |
|---|---|
| ₹1,000.00 and ₹750.25 supplied charges publish ₹1,750.25 | Two confirmed charges; zero receipts from publication |
| Allocate ₹400.00 against a ₹1,000.00 charge | ₹600.00 outstanding; original received-money receipt remains unchanged |
| Competing ₹300.00 allocations use a source with ₹400.00 available | Exactly one succeeds; available credit cannot become negative |
| An explicitly supplied future due date has unpaid charges | Outstanding amount remains visible; overdue amount is zero before that date passes |

| ID | Observed finding | Repair recorded | Status |
|---|---|---|---|
| MA-01 | Reusing the past-only `validDate` helper rejected a valid future maintenance due date | Use strict maintenance calendar validation bounded from 1900 through 2100; retain the older past-record rules for their intended inputs | Retained future-date RED, then independent domain/browser/full-gate pass |
| MA-02 | The home-statement selector/button row overflowed to 848px in a 768px tablet viewport | Use a scoped responsive grid for the statement controls | Four-viewport geometry and full-gate pass; final captures inspected |
| MA-03 | Submit/edit buttons touched without an action gap | Add a wrapping flex layout with 14px gaps and a phone column | Actual action captures, phone rerun and final visual/gate pass |

These are **three verified product groups**, counted once each. Three new actual native cases cover maintenance publication/allocation metadata, bounded/denied/cancelled reads and captured statements after entitlement changes. The tested accounts advertise **16 tools for the administrator, 14 for the eligible owner and ten for the unentitled tenant**, according to current entitlements; those are tool counts, not case counts. [Visible journeys](../web/tests/maintenance.spec.ts), [native cases](../web/tests/webmcp.spec.ts).

The first native attempt passed 19 cases and failed one because newly seeded payment data changed an older empty-ledger expiry fixture's expectation. Maintenance cases now run after that fixture; all 20 reran green. This is QA fixture ordering, not another product defect. Apparent modal-close clipping was also a capture artifact: in a 720px viewport the live dialog spans y=24–696 and its close control y=42–86. The screenshot helper stabilizes/restores only background scrolling. Finite arrival/menu animations finish before capture; error/correction captures bring their subject into view. Incorrect seed-home expectations, bounded-term setup and locator/wait corrections are also QA work, without extra product findings.

Adding maintenance also made Overview a six-source read. The first captured gate stopped at an obsolete combined-outage expectation of five alerts. The corrected expectation is six, then five after the service source retries, with three unavailable maintenance values remaining unknown. The retained `reports/local/maintenance-eval-overview-outage-red.log` and trace document a QA expectation correction; no application change was needed. All ten affected Overview cases passed again before the coherent final gate.

### Final regression and visual evidence

The coherent `SOCIETY_CAPTURE_UI=1 make eval` in `reports/local/maintenance-eval-final.log` exits with zero: **93 Go declarations with formatting/vet/race, TypeScript/build, 91 ordinary Chromium cases across 14 isolated suites and 20 actual Chrome 154 native WebMCP cases**. The original full race run took **250.511 seconds** for database, **45.814 seconds** for backup and **66.457 seconds** for server; the final unchanged-code gate used their valid cached results. The final maintenance suite passes **ten cases in 38.2 seconds**, Overview passes all ten and the native stage takes **30.5 seconds**. Earlier targeted/capture runs are not added to these inventories.

All **57 maintenance captures** were inspected: the main **55 through ten contact sheets** plus critical originals, and two additional **320px/375px review-action originals**. A populated 320px Overview original showing the new maintenance panel was also inspected. The affected phone-action case passed again in **4.8 seconds** after a capture-only adjustment; application code was unchanged after the full gate. The private `reports/local/maintenance-review/review.json` records hashes, dimensions and completed inspection for all 57. Neither derivative sheets nor repeat capture runs add bugs or cases.

### Preserved preview and meaningful recovery

At the maintenance checkpoint, the loopback preview used matching retained `0.10.0-dev` binary/assets under `var/preview-releases/0.10` and schema 9, with `/ready` returning 200 and verified `0.9` retained. The pre-upgrade schema-8 bundle is **471,040 bytes**, SHA-256 `a17ac0fd2039f56a8680933d466d07b0ff877e6ea30e58a1aff9df40e5a22b0c`, and restores in **16.703 ms**. The post-upgrade schema-9 bundle is **540,672 bytes**, SHA-256 `13600b00a3b987acccae70a7792cc36436332a8708ac47d113afefc6f287d43e`, and restores in **20.449 ms**. Independent hash, integrity, foreign-key, schema and credential-clearing checks pass. Twelve prior tables preserve their exact rows, including four users, three grants, nine audit rows and two confirmed factors. Initially wrong temporary-table names were QA verification corrections.

The separate [maintenance recovery test](../internal/backup/maintenance_test.go) proves restoration of nonempty published charges, received money/original receipt, active/corrected allocations and factor/authority, excluding reversals made after the checkpoint. Empty maintenance tables in the preserved preview alone cannot prove that richer recovery. Restore timings describe the development Mac; they do not establish target-host or off-site recovery.

Windows amd64 cross-compilation passes; target Windows execution and real rates/liability/finance-policy, data and production-custody acceptance remain pending. Personal `flux-i` ownership/public visibility and `0.10` publication are verified. Cycles/allocation closure covered the financial slice; the following upkeep checkpoint handles the operational slice. These local results do not complete the overall plan or its production acceptance.

## Upkeep: completed local checkpoint

**Local synthetic checkpoint complete, `0.11.0-dev` / schema 10.** The [upkeep brief](upkeep-workflow.md) was saved before code. The delivered workflow includes private vendor/asset registers, explicit recurring task instances, current eligible assignments and scheduled visits. A different current operational handler checks completion. Residents see intentionally published snapshots for their current audience; private work/history stays operational. Existing Administrator/Committee powers govern this workspace, with no new financial powers, automatic billing or messages. The coherent gate, visual review and preserved-preview upgrade/recovery pass. [Acceptance evidence](upkeep-baseline.md). At this writeup, the last verified publication is `0.10`; `0.11` publication is pending.

The first five domain tests passed before the complete inventory grew to **eight new Go declarations**, including schema-9 preservation, HTTP and rich backup. Checks cover separate completion/history/reopening, frozen public snapshots/private activity, current assignments/register references, calendars/explicit repeats/bounded pages and concurrent version/replay authority. The first eight rendered cases produced **five passes and three failures**; asynchronous End-key focus, duplicate headings requiring a dialog-scoped locator and suite history invalidating a hardcoded Overview count were QA corrections. [Domain checks](../internal/database/upkeep_test.go), [visible journeys](../web/tests/upkeep.spec.ts).

The expanded 11-case attempt passed ten and failed one because a locator selected a hidden Radix option. Scoping it to the visible control repaired the affected assignment/reschedule/cancel/reopen/publication journey. That is another QA correction, not a product group; targeted repetitions are not added to final case totals. [HTTP checks](../internal/server/upkeep_test.go), [rich recovery](../internal/backup/upkeep_test.go).

The source registers `society_find_upkeep`/`society_read_upkeep` as audience-scoped reads for authenticated accounts, plus `society_find_upkeep_register`/`society_read_upkeep_register` for current operational Administrator/Committee access. The three new actual native cases initially had **two passes and one genuine UP-01 failure**; its repair is now verified in the coherent suite. The tested accounts advertise **20 tools for the administrator, 16 for the eligible owner and 12 for the unentitled tenant**. Four new read definitions and 23 executed native cases are different measures. Tools expose no writes, private body or vendor contacts. [Registration](../web/src/webmcp.ts).

| ID | Observed finding | Repair recorded | Status |
|---|---|---|---|
| UP-01 | A stale `?task` location selection reopened a denied work dialog after the resident's home scope ended | Clear obsolete URL selection centrally before the workspace remounts with changed user scope | Real native failure, then affected membership and full-gate pass |
| UP-02 | Failed work/register detail fetching retained a misleading loading heading | Show useful Work/Register unavailable headings for the error state | Ordinary error-heading assertions, full-gate pass and final error capture inspected |

These are **two verified product groups**, bringing the prior 46 to **48**. The coherent `SOCIETY_CAPTURE_UI=1 make eval` in `reports/local/upkeep-eval-final.log` exits with zero: **101 Go declarations with formatting/vet/race, TypeScript/build, 102 ordinary Chromium cases across 15 isolated suites and 23 actual Chrome 154 native WebMCP cases**. The upkeep suite passes **11 cases in 32.4 seconds**; native takes **35.3 seconds**. Backend package timings are **270.131 seconds** for database, **54.933** for backup, **69.346** for server, **4.764** for command and **1.634** for documents; security uses its valid cached result. These timings describe this run, not a controlled performance improvement.

All **55 current final captures** were inspected through **ten regenerated contact sheets**, plus the final 1440px queue, 320px work preview and work-detail error originals. The private `reports/local/upkeep-review/review.json` records original hashes/dimensions and gate/inspection evidence. Derived sheets and repeat runs are not added to bug or case counts.

### Matched preview and nonempty recovery

The pinned `0.11.0-dev` binary/assets match the final build across **18 retained files**; the loopback preview is ready at `127.0.0.1:8080` with schema 10 and `/ready` returning 200, while `0.10` remains retained. Windows amd64 cross-compilation passes; target Windows execution remains unverified.

The pre-upgrade schema-9 snapshot is **540,672 bytes**, SHA-256 `13600b00a3b987acccae70a7792cc36436332a8708ac47d113afefc6f287d43e`, and restores in **26.036 ms**. The post-upgrade schema-10 snapshot is **598,016 bytes**, SHA-256 `c0cf497726ba8c93fa4adea5eb0383dc8207d255aafd453d5eb06fbe1a1dfae8`, and restores in **20.565 ms**. Independent hash, integrity, foreign-key and zero restored temporary-credential checks pass; migration provenance 1–9 and exact rows from 17 prior tables are preserved. Private evidence is `reports/local/upkeep-recovery-verification.json`.

The rich backup/domain checks independently retain a public **PLANNED version 1** snapshot alongside private **DONE version 5** work, two separate operational actors, an inactive vendor, its linked asset/deadlines and work events. Later reopening/unpublication does not cross the recovery boundary, old sessions are rejected, and there are no ledger/receipt side effects. This nonempty case proves more than an otherwise empty new preview register. Explicit repetition creates a separately confirmed occurrence, preserving the original task and supplied deadlines; neither internal completion nor a restore silently rewrites the public snapshot.

Personal publication is pending its verified SHA. The [collections brief](collections-workflow.md) is saved before code for the next workflow: fund campaigns, reports of money already sent, separate treasury confirmation and preserved entry/receipt/allocation identities. It adds no accepted cases or findings yet. The overall roadmap, real finance/authority policy, target Windows, production custody and external-provider activation remain open; no transfer initiation or live message is added.

## Benefits supported by this work

- **Better visual and interaction coverage.** The user's screenshots exposed a blind spot in a passing suite. Actual open-menu checks now protect alignment, selection, hover, focus and dialog placement that value-only checks missed.
- **More usable recovery.** Account-load retry and feedback near the submitted security form remove documented dead ends. Pending-action checks reduce uncertainty while an operation is in flight.
- **Preserved design quality.** Shared controls let fixes apply consistently across the approved interface. Their replacement also received validation, scrolling, keyboard and dialog checks so visual improvements did not discard expected behavior.
- **Protected data and access boundaries.** Allow/deny cases check both visible permissions and server enforcement. Transaction, concurrency, replay and immutable-history checks protect outcomes that a screenshot cannot establish.
- **Repeatable investigation.** The browser runner creates a temporary synthetic database, uses an ephemeral loopback URL, and removes its server/database afterward. Mutations do not alter the developer's live preview. Private captures can be regenerated.
- **Reviewable progress.** A completed workflow includes code, observable behavior and a record of what was tested. New findings become scenarios for later checkpoints rather than disappearing after a one-time repair.

These are observed or directly supported process benefits. We have not measured development-hours saved, avoided production incidents, financial return, or a Society OS speedup attributable to this method. Local timings in the baseline documents describe different versions and journeys; they cannot be treated as a controlled before/after performance improvement.

## Connection to the supplied articles

The performance article describes choosing important user journeys, measuring from interaction through rendered result, validating benchmarks against user experience and preserving improvements with regression checks. We apply those principles through bounded portal workflows, recorded baselines and rendered checks. Its use of human steering also matches the user's role in setting scope and judging the interface. Its reported speed gains belong to that project. [How we made claude.ai faster](https://claude.dev/blog/how-we-made-claude-ai-faster/).

The evaluation article emphasizes representative tasks, explicit expectations, reliable graders, controlled environments and distinguishing legitimate application failures from flawed evaluation machinery. We apply that discipline to permission cases, independent amounts, disposable fixtures and the separate treatment of locator/capture mistakes. Its model-evaluation headroom and held-out optimization rules concern a different setting; mandatory application correctness still requires a full pass. [Automating eval design and hillclimbing](https://claude.dev/blog/automating-eval-design-and-hillclimbing/).

These articles inform how we develop and evaluate the portal. The documented process does not require the portal to call a Claude API or any runtime language model. Browser tooling supports rendering and inspection; configuring it alone does not establish UI acceptance. The portal's current scope has no runtime LLM dependency.

## Reusable checkpoint checklist

- [ ] Write the user outcome, allowed/denied roles, inputs, expected results and important failure cases.
- [ ] Confirm that implementation covers the complete workflow within the agreed scope.
- [ ] Run relevant backend checks, TypeScript and the production build.
- [ ] Use a disposable synthetic database for browser mutations; preserve the existing preview.
- [ ] Exercise visible controls, navigation, opened menus, selection, hover, focus, keyboard, scrolling and dialog dismissal.
- [ ] Check desktop, tablet and small-phone layouts where the workflow is used.
- [ ] Check meaningful empty, loading, error, retry and pending states; test permissions on the server as well as in the interface.
- [ ] Capture stable viewport screenshots, inspect them and render any generated output such as a receipt PDF.
- [ ] Separate product defects, test defects, environmental failures and unresolved domain-policy questions.
- [ ] For browser-tool integration, prove native registration/discovery/execution, current permissions, cancellation and removal after logout/revocation.
- [ ] Fix reproduced problems, rerun affected checks and finish the relevant regression suite.
- [ ] Record finding counts, executed coverage, artifact locations and practical limits without inflating the totals.
- [ ] Retain verified binary/assets together before handing over a running preview; keep mutable build output separate, snapshot before upgrade and retain the matching MFA key separately.
- [ ] Publish or commit only source and documentation; keep databases, downloads, screenshots, logs, keys and credentials outside Git.

The current full local gate:

```sh
make eval
```

To regenerate the ordinary UI captures:

```sh
make build
SOCIETY_CAPTURE_UI=1 npm run test:browser --prefix web
```

For a targeted repair, append a relevant file such as `filters.spec.ts` or `records.spec.ts`; complete the relevant full run before closure. On this macOS machine, launch Playwright/Chromium through unrestricted or unsandboxed execution and keep it scoped to the local target. The current workspace uses unrestricted execution. [Isolated browser runner](../web/tests/run-browser.mjs).

## What remains outside this evidence

The completed review covers the specified local Chromium journeys and states. It does not establish every input, race timing, browser, assistive technology, physical device or operating-system UI behavior. Reduced-height emulation is not a real mobile-keyboard test. Screenshots support visual review but do not prove financial correctness or authorization.

The available computer's basic resources are now recorded; target-host operation and performance acceptance remain pending. Production HTTPS, approved real-data migration, society finance/receipt policy, custodial identity and recovery, encrypted off-site protection, operational monitoring and a representative pilot need their own evidence. Future workflows add their own acceptance cases. A checkpoint is complete when its stated local conditions are met; its report must keep those broader requirements visible. [Execution backlog](../execution-backlog.md).
