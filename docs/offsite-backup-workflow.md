# Encrypted off-site backups — next checkpoint understanding and checklist

Recorded 9 October 2026 while the household rehearsal's final gates run. Implement after that checkpoint is accepted and published. The existing instruction to complete the plan autonomously authorizes this independent preparation; real credentials, custodians, host tests and policy acceptance remain separate activation requirements.

## Outcome and codebase reality

The saved plan's Section 21 requires a consistent SQLite snapshot every 30 minutes, verified off-site retention, a warning at 45 minutes and a default pause on new financial effects at 60 minutes. Its proposed connected-operation RPO is one hour. A local snapshot alone does not satisfy that requirement.

`internal/backup` already creates and verifies a consistent snapshot, records checksums/application/schema identity and restores into a fresh destination. Existing restores purge temporary bearer credentials and preserve business records. The application retains exact receipt snapshots and private document originals; receipt PDFs can be regenerated. These paths provide a foundation, but there is no encrypted off-site schedule, usable-upload acknowledgement, backup-age guard or independently custodied restore in the current accepted releases.

Financial effects occur through manual posting/corrections and composite maintenance, collection and fine workflows. Identify every confirmed money/counter/allocation transition before adding the guard. `beginRecordWrite` is also used for drafts and household access, so a blanket prohibition there would incorrectly block permitted drafting and security revocation. Saved original replies must remain discoverable without creating another financial effect when protection is stale.

## Proposed implementation boundaries

Use an interoperable encrypted archive containing the verified manifest and snapshot, with bounded compression/decompression, checksums and explicit format/key identity. The running backup writer receives public recipient material; the private decryption identities are held independently by authorized custodians. The age Go library exposes recipient/identity encryption interfaces and a documented interoperable format. Pin and review the selected released dependency at implementation, rather than silently tracking the newest tag. [Primary library documentation](https://pkg.go.dev/filippo.io/age), [format specification](https://age-encryption.org/v1).

Create a reusable private object-storage boundary with a local HTTP fixture and a separately configured S3 adapter. Give the backup writer only its scoped upload authority. Use unique object keys, create-only requests, a validated upload checksum and the returned version identity. A lost or ambiguous response stays unconfirmed; neither an ETag nor a later retry conflict proves the original content. Independent custodial retrieval and decryption validate recovery. S3 documents conditional creation, request checksums and version IDs; policy must also prevent unauthorized deletion or removal of the current-version protection. [PutObject contract](https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html), [conditional-write behaviour](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html).

Retain attempts, leases, acknowledgement identity and operator audit. A successful upload's freshness is based on the snapshot's capture time, so uploading an old snapshot cannot manufacture a fresh recovery point. Bound retries and staging space; expose unavailable, pending, failed, uncertain, verified, stale and clock-error states accurately. An unauthenticated caller or resident receives no private storage/key/manifest information. Ordinary operator screens and a bounded native read may report protection status; no native tool configures custody, sends a backup or overrides a pause.

When configured protection becomes stale, reads and drafts continue while new authoritative money effects are denied transactionally. Reauthentication, retry and concurrent backup completion must preserve the original operation identity and independent paise expectations. Any risk override requires a separate verified, expiring, audited decision under the committee's accepted policy; never infer acceptance from elapsed time or an outage.

Prepare fixture configuration, scheduler/restart handling and deployment guidance without creating a live bucket, purchasing services, sending external notifications or selecting real custodians. Recovery-series reservation, unknown external-job reconciliation, private document-storage activation, off-host alerting and actual Windows/Linux/HTTPS acceptance follow as separately reviewable workflows. Existing snapshots and preview data/keys stay preserved.

## Verifiable checklist

- [ ] Enumerate current money/counter/allocation transitions, retained replies and permitted draft/security paths; define before/after expectations before implementation.
- [ ] Implement bounded, interoperable encrypted archives and independent custodial decrypt/verify/restore, including corruption, truncation, wrong keys, malicious paths and decompression limits.
- [ ] Implement a configured private storage boundary and deterministic local fixture: request signing, checksums, conditional creation, version identity, denied credentials, timeout, lost reply and conflict handling.
- [ ] Retain schedule/lease/attempt/acknowledgement history without logging sensitive payloads; verify restart, competing workers, bounded retry and staging cleanup.
- [ ] Implement snapshot-age/clock checks and transactional financial protection across every relevant transition while preserving reads, drafts, access revocation and original saved replies.
- [ ] Render the approved operator interface's actual loading/error/retry/pending/stale states and any authorized risk-decision form on desktop, tablet, 375 px, 320 px and 320×440; inspect menus, focus, keyboard, scrolling, dismissal and settled originals.
- [ ] Add and execute actual native protection-status reads, current-authority/held-result checks and human-form preservation with no mutation or credential exposure.
- [ ] Run backend formatting/vet/race, TypeScript/build and ordinary/native regressions against the same frozen inputs; verify encrypted populated recovery, prior migration preservation and separate held keys.
- [ ] Document finite results and limits, archive the matching release/evidence, verify personal publication and continue the next independent production workflow.

All items remain pending. These preparations do not establish a live off-site recovery point, accepted production policy, verified representatives or actual-host operation.
