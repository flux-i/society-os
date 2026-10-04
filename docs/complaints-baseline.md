# Help and repairs — local acceptance

Verified on 4 October 2026: **0.6.0-dev / schema 6**, macOS ARM64, synthetic data. A resident can report an issue, follow their own conversation, and confirm closure or reopen after resolution. Current administrator/committee handlers acknowledge, assign, update, resolve and keep staff-only notes. See the [workflow expectations](complaints-workflow.md).

Sharing a flat does not share another person's case. An author whose membership ends retains read-only personal history; current membership is required for further participation. Handler roles and eligible assignees are checked again inside the write transaction. Decisions require fresh authentication. Original reports and updates are immutable; version checks and actor-bound operation identities prevent stale overwrites and duplicate retries. No case action creates a financial entry, receipt or payment.

Resident search, counts, history pagination, versions, update times and native browser tools exclude private handler notes. Public versions advance only for public changes, and update identities are opaque. A resident can add a public update while staff-only conversation changes without learning how many private notes exist. Archived or ended relationships never confer access to someone else's cases.

| Executed gate | Result |
|---|---|
| Formatting, Go vet and race-enabled tests | Passed; 53 Go test declarations |
| TypeScript and production build | Passed |
| Ordinary Chromium regression | 48 cases passed across 10 isolated synthetic suites |
| Native Chrome 154 WebMCP | 8 cases passed using actual discovery/execution |
| Final complaints capture rerun | 7 cases passed in 17.9 seconds; 32 distinct captures visually inspected through 6 derived sheets |
| Windows amd64 cross-compilation | Passed; executable not run on the Windows target |
| Preserved-preview upgrade/readiness | Schema 5 → 6; ready returns 200 |
| Snapshot/fresh restore | Verified; 19.487 ms measured on the development Mac |

Seven rendered service journeys cover reporting, assignment/priority, resolution/closure/reopening, private-note boundaries, required fields and long content, opened menus and keyboard use, lost-response retries, busy dismissal, loading/error recovery, stale updates, list/history pagination and narrow scrolling dialogs. Screens include 1440px desktop, 768px tablet, 375px and 320px phones. Tests use fresh databases and preserve the developer's preview. Passing this inventory does not establish every possible interaction or an unbuilt workflow.

Native WebMCP now advertises eight tools to eligible staff: scoped home, financial-record, review, notice and complaint reads, plus opening an authorized home/workspace. An unentitled resident is not offered financial reads. Complaint tests check actual handler/private and author/public results, unrelated-case denial, bounded input, pre-aborted cancellation and removal after session revocation. No tool submits, approves, assigns, closes, posts money or changes permissions. The experimental browser API is optional. External Claude/Codex client end-to-end execution remains unverified.

Three finding groups were repaired: error scrolling moving the dialog itself and hiding its header/close control; private-note counts inferred from public version increments; and the running preview serving mutable assets from a newer build than its backend. The preview now runs a retained, matching binary and assets under `var/preview-releases/0.6`. [The process record](development-workflow.md) distinguishes product/operational findings from fixture and screenshot-capture mistakes. The running preview keeps its own database and MFA key.

The pre-upgrade schema-5 snapshot remains private. The schema-6 snapshot is 409,600 bytes with SHA-256 `283a93c04ec0a30d5bbd50edf6b3f818b7f474dfe9ec76c90fce739d5c25e8f0`; a fresh restore preserves 3 buildings, 118 flats, 154 people and 155 relationships. A separate recovery test preserves public and private case updates while invalidating old sessions. Preserve the matching MFA key separately. Logs, database bundles, screenshots and the local evidence manifest are ignored by Git.

Sources: [domain cases](../internal/database/complaints_test.go), [HTTP/privacy cases](../internal/server/complaints_test.go), [recovery case](../internal/backup/complaints_test.go), [rendered journeys](../web/tests/complaints.spec.ts), [native WebMCP](../web/tests/webmcp.spec.ts). Private executed evidence: `reports/local/complaints-full-eval.log`, `reports/local/complaints-capture-final.log`, `reports/local/complaints-review/review.json`.

Next: validated private document versions and separate approval before shared publication, then role/account administration, PWA and migration/deployment acceptance. Attachments, actual response times, emergency contacts, vendor accounts, real finance/retention policy, Windows operation and production infrastructure remain separate pending work.
