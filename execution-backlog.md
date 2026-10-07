# Society OS Execution Backlog

**Communication requirement, 7 October:** Staff compose, review, obtain separate approval and Send inside the portal. [Official provider preparation](docs/whatsapp-provider-baseline.md) is verified locally under the [saved contract](docs/portal-whatsapp-workflow.md), using numeric-loopback HTTP fixtures. Live delivery requires society-owned identities/credentials/templates, deployed HTTPS configuration and costed acceptance. The feature questions authorise independent preparation, not actual external messages or paid activation. Earlier manual-sharing proposals are historical; Messenger is not a required step in the requested workflow.

**Updated:** 7 October 2026
**Current state:** Locally accepted Go/React release **0.20/schema 19** adds official-protocol portal provider preparation to the preceding registry/identity/MFA, manual records/receipts, independent reviews, scoped notices/services/documents, changing overview, maintenance/allocations, upkeep, funds, incidents/fines, contacts, prepared originals, messaging and exports. [Provider acceptance](docs/whatsapp-provider-baseline.md) records **204 Go declarations, 183 ordinary cases/27 suites, 49 actual native cases/seven suites and 43 actually inspected captures**, matching schema-18/19 recovery, all 75 prior persistent tables preserved plus three additive tables, unchanged held keys and a pinned verified preview. Closed findings are **78 groups (76 product/UI, two operational)**; fixture corrections add no findings. Personal public [application publication](https://github.com/flux-i/society-os/commit/b52626e9f0c420cb647cee0a240f12ccb117dd34) is verified against API main, personal account/author and a clean tree. Its immutable private archive retains 354 matching sources and 753 verified file hashes; publication metadata follows in a separate docs-only commit. The preceding verified [0.19 application](https://github.com/flux-i/society-os/commit/0095bc6026ba8d4790818e3d9b1062a02b5d33ca) retains its immutable 336-source/565-hash archive. The whole plan remains active: reviewed community contacts/interruptions, meetings/acknowledgements/reminders, budget/move checklists, measured performance/PWA/migration and independent production preparation follow. Live provider, real identity/custody, policy, infrastructure and real-data acceptance remain separate. See [current checkpoint state](docs/next-session.md).

**Scope update:** The portal tracks manually entered records and generates receipts for money already received. Payment initiation, gateways and bank automation are excluded. Automated billing and Tally integration are conditional future work.

**Next implementation, 7 October:** [Reviewed service contacts and interruptions](docs/community-response-workflow.md) follows accepted provider preparation. Preserve the existing noticeboard; separately publish supplied contact authority and water/power/lift updates to frozen flat audiences, with explicit resolution and current Overview attention. Use disposable synthetic databases; preserve the pinned 0.20 preview and developer data. Live provider activation stays separate.

**Remaining independent work:** Provider preparation is locally verified under the saved [portal sending contract](docs/portal-whatsapp-workflow.md). Continue with emergency contacts and outage notices; meeting agenda/minutes and acknowledgement tracking; private reminders and delivery exceptions; budget versus recorded collections/expenses; and move-in/out/contact-update checks under the roadmap. Complete measured code splitting/performance and private-data-safe PWA/offline/logout/update behavior. Production preparation also needs a non-demo bootstrap/import path, configured provider/private-storage adapters, encrypted scheduled off-site backups, restore reconciliation, backup-age finance protection, operations alerts, deployment/runbooks and the cost worksheet. Prepare and test those with fictional data/local fixtures. Current local snapshots and synthetic delivery do not implement those production services. Synthetic migration and representative pilot rehearsals can proceed independently. Real society identities/data, policy/custodian decisions, credentials, host/network proof and the human pilot block only their dependent activation or acceptance; they do not replace this remaining implementation work.

**User priorities added 4 October:** [Society operations roadmap](docs/society-operations-roadmap.md) records the overview revamp, maintenance tracking, fund campaigns/external-payment reports, targeted registered-contact WhatsApp/email, private rule evidence/approved fines and prepared financial statement publication. Documents, overview, account administration, maintenance, upkeep, collections, incidents/fines, contacts, targeted synthetic messaging, prepared financial-original publication and exact statement messaging are verified locally. Scoped exports and provider preparation are verified; continue with the remaining ordered community slices. Explicit requests supersede older deferrals of resident payment reports/configured messaging; payment initiation remains excluded. Researched emergency, reminder and meeting additions retain source-linked priorities and acceptance conditions. Real provider activation remains pending society-owned setup.

Execute the [strategy](housing-society-execution-strategy.md) through small build milestones. Begin with a runnable local foundation using synthetic data. Confirm production infrastructure and accounting inputs alongside that work, then complete the gates in the [implementation plan](housing-society-digital-platform-plan.md).

## 1 Verified starting position

- Development workspace: `/Users/furqan/work/ps/trivedi` on macOS ARM64.
- Tested tools: Go 1.27.0, Node 25.8.1 and npm 11.21.0. Go dependencies and exact frontend versions are recorded in `go.mod`/`go.sum` and `web/package-lock.json`.
- The official `claude-api` skill is installed globally for Claude Code and Codex. Both global copies match the 81-file manifest; the skill appears in this session's available skills.
- Local Git, Go/React source, migrations, synthetic database and documented build/check/benchmark/report commands now exist. See [README](README.md) and [architecture decisions](docs/architecture-decisions.md).
- The local interface includes the approved changing overview, wing cards on Homes & people, searchable homes, occupied/vacant homes and active owners/tenants. Fictional sign-in, scoped resident homes, occupancy/membership administration and change history are working. [Visual direction](docs/design-system.md) records the user's requested design standard.
- The user supplied an i5-8300H / 8 GB Windows x64 computer with SSD/HDD storage; see [hardware evidence](docs/production-hardware.md). Exact OS/build, host operation, stable public origin and society state remain unconfirmed. Exact Tally details are future integration inputs. Reviewed examples of manual entries and the receipt format will be needed before the finance workflow is finalized.

The user supplied an Intel i5-8300H, 8 GB RAM and Windows x64 machine with SSD/HDD storage. Exact Windows version/build, host health/power/network, society state and exact Tally version remain pending. Proceed with local development and synthetic data while these remain pending; do not infer statutory charges or Tally compatibility from placeholders. The first finance workflow records given charges/opening balances and money already paid; it never initiates a transfer.

## 2 First build milestone

**Outcome:** A maintainer can build and run the application locally, load an isolated synthetic 118-flat registry, apply migrations, create a consistent SQLite snapshot and restore it into a fresh local environment.

This demonstrates the development foundation. Public HTTPS, off-site S3 protection, host power/reboot behavior and custodial recovery remain separate infrastructure acceptance requirements.

| Order | Task | Deliverable | Completion evidence | State |
|---|---|---|---|---|
| 1 | Establish the local project | Local Git setup, README, Go backend, React/Vite frontend, ignored local secrets/data and repeatable build commands | Clean build and local startup using documented commands | Complete locally |
| 2 | Prove the database build | Pinned modernc SQLite driver; actual embedded SQLite 3.53.4; migration runner; connection settings | Four simultaneous connections pass WAL/FULL/foreign-key/busy-timeout checks; migration provenance verified | Complete locally |
| 3 | Create the synthetic registry | 118 fictional flats, 154 people and 155 current/historical relationships | Unique flat count, repeat seeding, joint/multiple-flat owners and former-member cases pass | Complete locally |
| 4 | Add basic operating visibility | Liveness/readiness endpoints, structured error/timing events, release/schema version | Health remains live while closed-database readiness returns 503; logs exclude queries/record IDs | Complete locally |
| 5 | Prove consistent local recovery | Snapshot command, integrity checks, manifest and clean restore procedure | Committed WAL data restored; corruption/overwrite rejected; independent SHA-256/count checks pass | Complete locally |
| 6 | Record the first baseline | Build/test report and representative read timing | 100 warm local HTTP reads, repeated query benchmark and measured restore evidence | Complete locally |

Include meaningful checks for migrations, database settings and snapshot integrity. Implement authorization, resident-workflow and job assertions as those workflows become runnable. Keep unfinished scenarios visibly pending rather than filling the report with passing placeholders.

Completion evidence is recorded in the [foundation baseline](docs/foundation-baseline.md). Local snapshot/restore and registry browser checks pass; production S3 encryption, custodial recovery, alerts and real-data acceptance remain pending.

The next local slice is also complete. See [registry/identity evidence](docs/registry-identity-baseline.md).

| Delivered locally | Completion evidence |
|---|---|
| Relevant overview counts | Fresh fixture: 109 occupied / 9 vacant homes; 118 distinct active owners / 35 tenants; joint/multi-home/former cases checked |
| Fictional password sign-in and sessions | Argon2id, hashed sessions, bounded login work, logout/idle/absolute/account invalidation checks |
| Current server-side scopes | Resident list/detail/search/totals restricted to active homes; operator roles checked per request; former tenant history denied |
| Registry administration | Occupancy changes, new/existing people, date-bounded relationship ending, primary-contact replacement |
| Transactional history and stale edits | One successful write under concurrent edits; rejected writes leave no version/person/audit changes; history resists update/delete |
| Recovery and browser acceptance | Identity/audit restored with sessions excluded; six isolated browser workflows pass, including 375px forms and account switching |

The next identity slice is also complete locally. [Account-security evidence](docs/account-security-baseline.md) records 31 backend checks and ten browser journeys. It adds current-person invitations with manual identity/email attestation and handover, expiring single-use activation/reset, authenticator enrollment and required privileged verification, recovery-code replacement, reauthentication and an offline lost-factor recovery command. Snapshots purge temporary credentials while retaining encrypted confirmed factors; the MFA key is held separately. The four public demo identities alone have an explicit exploration helper.

Explicit role/account administration is also verified locally in schema 8: bounded appointments, account history, separate powers, successor protection, suspension/resumption and changed-scope invalidation. [Acceptance evidence](docs/account-administration-baseline.md) records 43 total repaired groups and meaningful account recovery. Real identity/custody acceptance and real-data migration remain pending.

The [rendered UI review](docs/ui-review-baseline.md) now includes an expanded interaction inventory after the user's dropdown screenshots exposed gaps in the first pass. It corrects filters, native menu styling, viewport navigation, dialog scrolling/actions/focus, authentication scroll position, independent security inputs and inline error/retry behavior while preserving the approved visual direction. Twenty-six Chromium checks pass, opening all 118 homes and all nine dropdown controls. Chrome DevTools MCP with native WebMCP support is installed globally for Claude Code and Codex; no runtime model API is required.

## 3 Inputs to collect during that milestone

| Input | Person to involve | Why it matters | Current state |
|---|---|---|---|
| Production computer model, OS, SSD and power/network setup | Technical maintainer | Deployment target, recovery and actual operating-cost measurements | CPU/RAM/Windows x64/SSD/HDD supplied; OS/build, health, operation and power/network checks pending |
| Society state and approved document/receipt policy | Committee and records/finance owner | Applicable retention and receipt fields; no charge rules inferred | Pending |
| Representative manual charge/payment entries and receipt format | Authorized finance operator and committee | Correct entry fields, derived balances and receipt output | Pending before finance acceptance |
| Canonical flat registry and current/historical membership rules | Registry officer | Invitations and correct resource access | Pending |
| Society-owned AWS/domain accounts and two custodians | Sponsor | Off-site protection, stable origin and recoverable credentials | Pending |

Unknown inputs do not prevent the local synthetic milestone. They block only dependent decisions. Record answers with evidence and dates; keep credentials outside documents and source control. Tally compatibility is required only if that integration is later requested.

## 4 Subsequent build order

| Milestone | Work | Completion evidence |
|---|---|---|
| Production infrastructure proof | Selected Linux host, stable external HTTPS, private S3, encrypted off-site snapshots, alerts and reboot/recovery exercise | Measured recoverability and complete annual forecast below ₹12,000 |
| Identity and registry | Invitations, sessions, privileged MFA, memberships and permission enforcement | Required allow/deny and account-recovery cases pass |
| Shared foundations | Transactional audit, operation identities, leased jobs and validated document versions | Failures/retries recover correctly and access restrictions hold |
| Manual records and receipts | Authorized entry forms, exact-paise amounts, given charges/opening balances, already-paid entries, derived balances, immutable receipts/PDFs and auditable corrections | Recorded amounts reconcile with approved examples; retries issue one receipt; PDF failures preserve saved entries |
| Resident workflows | Notices, complaints, permitted documents and mobile/PWA behavior | Audience/privacy, stale-client and account-switch checks pass |
| Migration and pilot | Approved registry/document/manual-entry data; 10–15 representative flats exercising entry/receipt and resident/committee workflows | Current-scope production gates pass before expansion to 118 flats |

Production infrastructure proof can remain pending while independent local features are built with fictional data. Missing hardware/state information blocks dependent production decisions, not the identity, registry or manual-entry prototypes. Keep the local build order sequential where workflows share permissions, migrations and posting logic.

## 5 Working agreement for execution

Use an AI coding agent to implement and investigate each bounded workflow. The user supplies priorities and product feedback; technical and accounting reviewers resolve the relevant implementation and business decisions. Routine local implementation and test iterations can proceed within the agreed task scope.

The two articles inform the development process from the first build milestone. The performance article supplies a measured improvement loop, and the evaluation article supplies checks on whether the measurement is trustworthy. Those methods apply to this portal even when its runtime makes no language-model calls. [Performance development loop](https://claude.dev/blog/how-we-made-claude-ai-faster/), [evaluation design](https://claude.dev/blog/automating-eval-design-and-hillclimbing/).

| Step | Agent work | Evidence produced |
|---|---|---|
| Define | Record one user journey, its state/permission requirements, inputs and completion condition | Task brief and concrete scenarios |
| Establish checks | Implement relevant assertions from reviewed expectations, exact manual amounts and permission rules | Repeatable grader/check command; known good and deliberately broken behavior distinguished |
| Build | Implement the smallest complete API/database/UI workflow that satisfies the task | Runnable code and a demonstrable user outcome |
| Measure | Run the same fixtures and record correctness, latency, resource use and failure states | Baseline report with environment and sample counts |
| Improve | Profile a bottleneck, make one attributable change, rerun affected checks and compare with the baseline | Before/after report and keep/revert decision |
| Preserve | Keep validated behavior checks and stable performance budgets in the project checks | Regressions detected on subsequent changes |
| Validate in use | Inspect the workflow during the pilot and revisit assumptions that synthetic cases missed | Pilot findings and new representative scenarios |

First build a correct baseline; optimize once that baseline exists. Required correctness checks should reach full pass. For optimization, use controlled repeated timings and stable query/allocation/render counts where relevant; prove a proxy reflects the actual user outcome before using it as a performance gate.

The runnable project now exposes `make check`, `make bench`, `make report` and local browser checks. [README](README.md) documents them. Keep baseline versions, fixtures and result formats consistent. Changes to expectations, grading rules or measurement environments require explicit review rather than being counted as application improvements.

For example, a document-download task is complete when an authorized resident can open the approved version, another flat's private document remains inaccessible, revoked access prevents new signed links, and S3 failures produce a recoverable state. Its next optimization task can reduce authorization-query or navigation time while those conditions continue to pass.

For the manual-entry slice, a fictional opening debit of ₹1,000 and confirmed received entry of ₹400 should produce a ₹600 balance and one ₹400 receipt. Repeated confirmation must preserve that result; a PDF/S3 failure must leave the entry saved and retry the same receipt. A draft or given charge alone must never create a received-money receipt. Review the expected amounts independently of the implementation.

Known functional/security requirements remain visible to implementers. Fresh acceptance cases and generated variations check generalization. For any later optimization of coding-agent prompts or instructions, separate development, validation and final evaluation cases and prevent the optimizer from reading final answers. The model-oriented hillclimb workflow must not become a way to hide or weaken mandatory application requirements.

Keep one implementation task active initially. Independent investigations can be organized separately once the baseline and review process work; code changes sharing migrations, transaction logic or permissions should be integrated sequentially.

Use the installed Claude evaluation guides when a concrete Claude-powered flow exists and its input/grader/run budget can be reviewed. V1's application correctness is checked with code and domain review throughout development.

The `claude-api` skill is optional development guidance, not an application dependency. No V1 workflow needs the Claude API, an Anthropic SDK or a Claude API key. If a later AI feature uses Claude, the skill can assist its implementation and evaluation; choosing that provider is a separate future decision.

Operation identities, leased jobs, manual entries/receipts, separate reviews/notices, private service requests, validated documents, changing overview and account administration are implemented and verified locally; their baselines retain evidence and limits. Maintenance cycles/allocations are now verified in schema 9 with explicit separate Treasury review, exact credit reconciliation and retained correction history. Upkeep subsequently passed its separate schema-10 checkpoint, followed by collection campaigns/external-payment confirmation and exemptions in schema 11. Private incident review/response notices subsequently passed schema 12, followed by separate fine issuance, appeals and linked corrections in schema 13. Contact registration/verification/opt-out subsequently passed schema 14. Targeted synthetic messaging subsequently passed schema 15, followed by prepared financial originals and deliberate publication in schema 16. Exact statement sharing and scoped finance exports are verified; official-protocol provider preparation is also locally verified; next are the remaining [operations roadmap](docs/society-operations-roadmap.md), alongside independent production/finance-policy preparation. Keep finance posting separate from registry/community review: approving an expense proposal does not post an entry or pay money. Production inputs block dependent decisions only.

## 6 Manual entry boundary and future automation

Current finance work uses manual input from authorized operators. An entry records the flat/payer, amount, date, payment method/reference, relevant description, source/evidence and actor. A receipt is issued for an entry that the authorized operator confirms represents money already received. Saving an unconfirmed entry does not prove receipt of money.

Store monetary values exactly in paise. Save the confirmed entry, receipt identity/number, audit and PDF job atomically; retries reuse the operation identity. Generate the PDF from a frozen snapshot. Preserve original confirmed entries/receipts and use linked corrections rather than silent edits/deletion. Residents can view their permitted records; finance-entry permission is separate from ordinary resident access.

Derive balances from approved given charges/opening balances and recorded paid entries. No statutory rate, interest formula or automatic bill schedule is inferred. Explicit same-home charge/credit allocations, retained corrections, excess credit and voluntary purpose attribution are implemented and verified; source caps prevent using money twice.

Future work, only when requested: payment gateways/initiation, bank APIs or automated matching, automated billing calculations and Tally import/export integration. The requested collections/payment verification and provider-neutral synthetic messaging are locally verified. Live provider setup is a separate activation input; official portal WhatsApp and remaining community workflows follow the [operations roadmap](docs/society-operations-roadmap.md).
