# How we build and verify Society OS

**Recorded:** 4 October 2026. **Completed local checkpoint:** release `0.4.0-dev`, schema 4. The earlier expanded UI review covers release `0.3.0-dev`, schema 3.

Our approach is to build a complete user workflow, use it in a rendered browser, inspect what actually appears, fix the problems found and preserve the checks for the next change. The user sets the product direction and quality standard; the coding agent implements and investigates within that scope. Screenshots and concrete interactions keep that collaboration grounded in the application people will use.

This process has corrected **23 documented issue groups: 19 in the earlier UI review and four in the manual Entries/Receipts checkpoint**. The current full suite passes **34 Chromium tests and 39 backend tests**. The earlier expanded review opened **118 home cards across ten pages**, exercised **nine dropdown controls** and captured **36 screenshots**. These are different measures: tests, homes, controls and screenshots are coverage evidence, not additional bug counts. The issue inventories below make the corrected total auditable; three test/capture corrections remain separate.

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
3. **Run checks appropriate to the change.** `make check` runs formatting, Go vet, race-enabled backend tests and TypeScript. `make build` verifies the production frontend and Go binary. Domain expectations, permission denials, transactions and retries receive meaningful assertions.
4. **Exercise the real interface.** Sign in, navigate, open dialogs, click visible options, type, scroll, dismiss and return. A direct API call can prepare a fixture or prove server behavior; it cannot establish that the corresponding UI control is visible, clickable or understandable.
5. **Inspect rendered states.** Capture the visible viewport, wait for fonts and stable rendering, and inspect the screenshots. Open menus must be captured while open. Check the active, selected, hovered and focused states, plus empty, loading, failure, retry and pending actions.
6. **Classify failures before changing code.** Distinguish a product defect from a wrong test locator, an unstable capture or an incorrect expectation. Fix the actual cause and keep the original user outcome intact.
7. **Verify the repair and the surrounding workflow.** Rerun the affected checks after each change. Run the complete relevant suite before closing the checkpoint, including shared controls used by earlier workflows.
8. **Record what was executed and what remains unknown.** Keep the result, fixture, browser, viewports, observed issues, fixes and evidence locations. Report specific coverage rather than saying every possible click was checked.

The repository's [working agreements](../AGENTS.md) make this routine part of development. The [design system](design-system.md) keeps the approved forest/ivory palette, editorial typography and accessible controls consistent as the product grows.

### Working together

The user steers priorities, scope and visual judgment. The implementation agent turns those decisions into a runnable workflow, investigates failures and reports evidence. For this document, the user explicitly requested a separate agent: it reads the existing milestone reports and tests, reconciles the counts and owns this documentation file while implementation continues. Separate file ownership keeps that evidence work independent of changes to the live preview, migrations and posting logic.

Society representatives still need to approve real registry data, accounting examples, receipt policy and custody procedures. Those decisions stay visible in the backlog so local synthetic progress has a clear handoff to production acceptance.

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

The following table isolates the earlier `0.3.0-dev` UI/account-security evidence. The current manual-records results follow it; milestone totals are not added together.

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
- [ ] Fix reproduced problems, rerun affected checks and finish the relevant regression suite.
- [ ] Record finding counts, executed coverage, artifact locations and practical limits without inflating the totals.
- [ ] Publish or commit only source and documentation; keep databases, downloads, screenshots, logs, keys and credentials outside Git.

Typical local commands:

```sh
make check
make build
cd web
SOCIETY_CAPTURE_UI=1 npm run test:browser
```

For a targeted repair, append a relevant file such as `filters.spec.ts` or `records.spec.ts`; complete the relevant full run before closure. On this macOS machine, launch Playwright/Chromium through unrestricted or unsandboxed execution and keep it scoped to the local target. The current workspace uses unrestricted execution. [Isolated browser runner](../web/tests/run-browser.mjs).

## What remains outside this evidence

The completed review covers the specified local Chromium journeys and states. It does not establish every input, race timing, browser, assistive technology, physical device or operating-system UI behavior. Reduced-height emulation is not a real mobile-keyboard test. Screenshots support visual review but do not prove financial correctness or authorization.

Production hardware and HTTPS, approved real-data migration, society finance/receipt policy, custodial identity and recovery, encrypted off-site protection, operational monitoring and a representative pilot need their own evidence. Future workflows add their own acceptance cases. A checkpoint is complete when its stated local conditions are met; its report must keep those broader requirements visible. [Execution backlog](../execution-backlog.md).
