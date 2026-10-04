# Approvals, notices and native WebMCP — local acceptance

Verified on 4 October 2026: **0.5.0-dev / schema 5**, macOS ARM64, synthetic data. This is local development acceptance; real finance policy, pilot and production operation remain separate gates.

Residents submit maintenance, registry-change, expense and notice proposals. Administrators and committee reviewers must complete MFA; a different reviewer approves, declines or requests changes, with a reason and confirmation. Requested changes can be revised and resubmitted; authors can withdraw eligible requests. Immutable events retain the submitted content, actors and versions. Fresh authentication, current permissions, actor-bound retry identities and optimistic concurrency are checked in the write transaction.

An approved notice is published atomically to its selected current audience: residents, owners, tenants, committee or a wing. Resident-facing notice responses omit private review events. Archiving removes the notice from the noticeboard while preserving history. Protected deep links recheck access; WhatsApp prepares only a portal link for a person to send. The tests do not send messages externally.

Other approved proposals record permission for follow-up. They do not automatically change the registry, post money, issue a receipt or initiate a payment. Manual financial entry confirmation still follows its existing separate finance permission; any additional financial approval policy needs explicit accounting acceptance.

| Executed gate | Result |
|---|---|
| Formatting, Go vet and Go race tests | Passed; 46 Go test declarations |
| TypeScript and production build | Passed |
| Ordinary Chromium regression | 41 cases passed across 9 isolated suites |
| Native Chrome 154 WebMCP | 6 cases passed using real discovery and execution |
| Final review/UI capture rerun | 11 cases passed; 29 distinct review captures inspected |
| Windows amd64 cross-compilation | Passed; executable not yet run on the target machine |
| Preserved-preview upgrade and readiness | Schema 4 → 5; ready returns 200 |
| Fresh snapshot/restore | Verified; restore 17.918 ms on the development Mac |

The six native tools read scoped homes, records, requests and notices, or open an authorized home/workspace. There are no approval, posting, publication or permission-changing tools. Tests exercise bounded inputs, resident/tenant scope, dialog preservation, actual submission/publication states, logout, revoked sessions, pending MFA and a pre-aborted cancellation signal. Ordinary browsers do not require the experimental API. End-to-end use by an external Claude/Codex client is not claimed by these native browser tests.

The review journeys exercise separate review, revisions, all notice audience boundaries, decline, withdrawal, archiving, pagination, stale decisions, lost-response retries, busy dismissal, recovery and opened menus/keyboard at desktop, tablet, 375px and 320px. The complete regression preserves prior registry, identity and financial behavior. Passing this inventory does not establish every possible interaction or unbuilt workflow.

Six current product finding groups were repaired: editorial dialog headings, field/menu width/layout, optional-home clearing, short-screen navigation clipping, phone error/retry visibility and native-tool removal after session revocation. The [process record](development-workflow.md) distinguishes these groups from test-machine and capture problems. No measured development-time saving or production incident reduction is asserted.

The old pre-upgrade snapshot remains private. The new 364,544-byte SQLite snapshot has SHA-256 `79459392054128f363c54607b74f3cf9ca376e2a1edb6e9fa6d80da0d059055c` and restores 3 buildings, 118 flats, 154 people and 155 relationships. A separate recovery test restores an approved notice and its immutable decisions after the source notice was archived, while invalidating old sessions. The MFA key remains separate from snapshots. Snapshot directories, logs, fixture databases, captures and the local review manifest are ignored by Git.

Source checks: [domain](../internal/database/reviews_test.go), [HTTP/privacy](../internal/server/reviews_test.go), [recovery](../internal/backup/reviews_test.go), [rendered journeys](../web/tests/reviews.spec.ts), [native WebMCP](../web/tests/webmcp.spec.ts). Local evidence: `reports/local/reviews-eval-final.log`, `reports/local/reviews-capture-final.log`, `reports/local/reviews-review/review.json`.

Next: complaints with private staff notes and validated assignments/transitions, followed by private documents, role administration, PWA and migration/deployment acceptance.
