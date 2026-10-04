# Private documents — workflow and acceptance expectations

This checkpoint follows sections 9 and 17 of the platform plan. These are the workflow expectations; executed local evidence and production gaps are recorded in the [document baseline](documents-baseline.md).

An authenticated current resident uploads a private document for themselves or requests separate approval before sharing with their current home or the society. Current document reviewers can prepare committee or scoped resident/home documents; financial documents require the separate finance permission. An uploader cannot approve their own file. The file must pass content validation before approval/download. Uploading payment evidence never confirms money received or issues a receipt.

Original bytes, verified SHA-256, detected type, filename, subject, category and intended audience are immutable per version. Replacement creates a linked version; the currently approved version remains available until another person approves the validated replacement. Declining, withdrawing or archiving preserves original versions and decisions. Routine UI never deletes a retained file. A separate accepted retention/hold process is required before purge.

| Scope | Entitlement after approval |
|---|---|
| Society residents | Current resident membership, or current document reviewer |
| Committee | Current document reviewer role |
| Home | Current membership in that home, or current document reviewer; financial categories additionally require financial entitlement |
| Named person | Named resident/author and explicit appropriate reviewer; another occupant never inherits it |
| Accounting | Current committee/treasurer financial permission; registry administration alone is insufficient |

Before approval, only the uploader and currently eligible separate reviewers can see the upload. Every list, search, count, metadata, upload completion, replacement, decision, download, version-history and native WebMCP read rechecks the same current scope. Ended membership retains only approved, named-person records belonging to that person; new uploads and shared-home access require a current relationship. Restricted identities return the same not-found response without filenames/count leaks.

Uploads reserve bounded size/quota with an actor-bound operation identity and an expiry. A completed upload must match the reserved size/checksum and current permission. Processing uses a leased, bounded validation job; failure does not expose bytes. PNG/JPEG must decode within dimension/pixel caps. PDF structure and active-content dictionaries are inspected using a bounded qpdf subprocess; unsupported/encrypted/active documents fail closed. File checks are not an antivirus guarantee. Downloads are attachments, never embedded executable content.

The local-only synthetic preview initially retains immutable original bytes in SQLite so its existing consistent snapshot includes uploaded documents and history. This is a development storage adapter, with explicit small per-file/user/society caps; it is not the planned production S3 store. Production remains disabled until private versioned S3, checksum/version verification, quota/expiry handling, bounded scanning and recovery of originals are exercised against the society-owned account. The adapter must preserve the same metadata/approval/scope contracts when storage moves to S3. No real files or resident data belong in this preview.

Required meaningful checks: all scope boundaries including current/revoked memberships and separate financial access; self-approval denial; unvalidated download denial; exact checksum/size, Unicode filenames and wrong type; actor-bound retries and stale decision races; replacement retaining the old approved version until approval; immutable original/version history; quotas, abandoned reservations and validation leases; restore retaining original bytes but invalidating sessions; real browser upload/download/menus/keyboard/error/retry on desktop/tablet/small phones; native browser reads hiding another person's pending/private files.

Production inputs still needed: approved categories and legal retention, actual file examples, antivirus/containment policy, S3 credentials/bucket policies and custodian recovery. Text extraction/OCR and attachment links to notices/cases are subsequent work; metadata search is sufficient for this first complete library workflow.
