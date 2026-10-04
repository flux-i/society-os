# Private documents — local acceptance

Verified on 4 October 2026: **0.7.0-dev / schema 7**, macOS ARM64, fictional data. Residents can upload private originals, finish interrupted uploads, and request separate approval before sharing. Reviewers can approve a validated version, decline it or archive its collection. A replacement preserves the previous approved original until another eligible person approves the replacement. See the [workflow expectations](documents-workflow.md).

Original bytes, reserved size, SHA-256, filename, detected type, intended audience and source linkage remain immutable. Uploading evidence does not confirm a payment, post money or create a receipt. Financial documents require separate financial entitlement; registry administration alone does not grant it. Self-approval is denied. Mutations recheck current scope, fresh privileged authentication, version and actor-bound operation identity in the same write transaction. Lost reservation/upload/decision responses retry the same identity.

Current society/home/committee/accounting/named-person rules apply before metadata search, counts, pagination, version history and original downloads. Ended home membership loses that home's document access even if another home remains active. Approved named-person history remains readable by its named person; another occupant does not inherit it. Ordinary readers cannot see a pending replacement or private review trail. Archival blocks new ordinary audience links while preserving originals and authorised review history; already downloaded copies cannot be recalled.

| Executed gate | Result |
|---|---|
| Formatting, Go vet, race-enabled backend tests | Passed; 64 Go test declarations |
| TypeScript and production build | Passed |
| Ordinary Chromium regression | 60 cases passed across 11 isolated synthetic suites |
| Documents journeys | 12 cases passed; 48 distinct final captures visually inspected using 8 contact sheets plus full-size changed/new views |
| Actual native Chrome 154 WebMCP | 10 cases passed in 12.0 seconds |
| Windows amd64 cross-compilation | Passed; executable not run on the Windows target |
| Preserved-preview upgrade/readiness | Schema 6 → 7; matching retained binary/assets; ready returns 200 |
| Snapshot/fresh restore | Verified; 17.323 ms measured on the development Mac |

Seven document domain declarations cover scope and financial separation, immutable replacements/archive, checksum/size/actor-bound retries, concurrent stale decisions/current roles, quota/expiry/leases, future contract expiry and permitted version pagination. Validator tests use actual PNG decoding and the installed qpdf executable, including damaged images, pixel caps, wrong type, malformed/active PDFs and unavailable checks. HTTP assertions verify Origin/CSRF, scoped metadata/originals, Unicode attachment headers and private history. A separate snapshot test restores approved original bytes/checksum/revision and denies pre-restore sessions.

Twelve browser journeys use real file inputs and a native file-picker event, original downloads checked byte-for-byte, separate approval, replacement and archival, financial evidence without money posting, search and fourteen-version history pagination, required fields and opened/keyboard-operated menus, lost responses, busy dismissal protection, interrupted upload/checksum recovery, stale decisions, and empty/loading/list/detail/download error recovery. Screens cover 1440px desktop, 768px tablet, 375px and 320px phones. Passing this inventory does not establish every possible interaction, browser, physical device or production workflow.

Native WebMCP advertises ten tools to eligible staff: scoped homes, records, requests, notices, complaints and document metadata/version reads, plus opening an authorised home/workspace. Financial reads are omitted for unentitled tenants. Document cases check an actual uploaded original before/after visible approval, unrelated pending denial, public history privacy, bounded inputs, cancellation, protected open dialogs and session revocation. No tool uploads, approves, archives, posts money or changes permissions. The experimental API is optional; external Claude/Codex client end-to-end execution remains unverified.

Three finding groups were repaired: former-home uploader access surviving while another home remained active; future contract expiry rejected by a past-record date rule; and document visual/status presentation, including incorrect shared component classes, an ineffective filter-width selector and a failed-load heading still claiming to open. [The process record](development-workflow.md) separates these from fixture mistakes and updating an obsolete disabled-navigation assertion.

## Local processing and storage limits

Original files use a **synthetic development SQLite BLOB adapter**, so the existing consistent snapshot includes them. This is not the planned production S3 implementation. Per-upload caps are 20 MiB for PNG/JPEG and 4 MiB for PDFs; uploader/society caps are 100 MiB/1 GiB including reservations and retained bytes. Uncompleted reservations expire after 24 hours. Original bytes remain retained on withdrawal/decline/archive and consume quota; there is no routine purge UI.

Uploads have two concurrency slots. The sequential validator uses expiring leases, bounded attempts and recovery from a crashed lease. Images must fully decode within 8 megapixels and 8192px dimensions. PDF checking uses qpdf 12.4.2 with a three-second process deadline, capped output, structural complexity/page limits and inspection of canonical object dictionaries for unsupported active content. Unavailable checks fail closed and can be retried. Files are downloaded as private, uncached attachments with sandbox/nosniff headers; they are not embedded for execution. These checks are not antivirus coverage or a proven operating-system memory containment boundary. [QPDF CLI documentation](https://qpdf.readthedocs.io/en/stable/cli.html), [QPDF JSON documentation](https://qpdf.readthedocs.io/en/latest/json.html).

The pre-upgrade schema-6 bundle remains private. The schema-7 snapshot is 462,848 bytes with SHA-256 `0b13b74ba9022276e251df58c3d15fcf11b611561003e851305bc7bc025e2e95`; Python independently verified its hash. A fresh restore preserves 3 buildings, 118 flats, 154 people and 155 relationships. This preview snapshot does not contain seeded library uploads; original-byte restoration is independently demonstrated by the recovery test. Preserve the matching MFA key separately. Logs, bundles, screenshots and the inspection manifest are ignored by Git.

Sources: [domain checks](../internal/database/documents_test.go), [actual validators](../internal/documents/validation_test.go), [HTTP checks](../internal/server/documents_test.go), [original-byte recovery](../internal/backup/documents_test.go), [rendered journeys](../web/tests/documents.spec.ts), [native WebMCP](../web/tests/webmcp.spec.ts). Private final evidence: `reports/local/documents-release-gate.log`, `reports/local/documents-review/review.json`.

Next: the user's [changing overview and society operations roadmap](society-operations-roadmap.md), followed by its ordered role, maintenance, collection, fine, messaging and statement slices. Production S3/version/checksum/presigned access, antivirus/containment, approved retention and policy, real file examples, spreadsheet originals, attachment links, actual Windows operation and infrastructure/pilot acceptance remain pending.
