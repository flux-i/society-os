# Encrypted off-site backups — verified local workflow

**Local verification, 12 October 2026:** The [accepted baseline](offsite-backup-baseline.md) records 308 Go declarations, 253 ordinary cases, 83 actual native cases, 56 direct original reviews and matching 103-table recovery. Personal publication is pending; the matched private archive is prepared next.

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

- [x] Enumerate current money/counter/allocation transitions, retained replies and permitted draft/security paths; define before/after expectations before implementation.
- [x] Implement bounded, interoperable encrypted archives and independent custodial decrypt/verify/restore, including corruption, truncation, wrong keys, malicious paths and decompression limits.
- [x] Implement a configured private storage boundary and deterministic local fixture: request signing, checksums, conditional creation, version identity, denied credentials, timeout, lost reply and conflict handling.
- [x] Retain schedule/lease/attempt/acknowledgement history without logging sensitive payloads; verify restart, competing workers, bounded retry and staging cleanup.
- [x] Implement snapshot-age/clock checks and transactional financial protection across every relevant transition while preserving reads, drafts, access revocation and original saved replies.
- [x] Render the approved operator interface's actual loading/error/retry/pending/stale states and any authorized risk-decision form on desktop, tablet, 375 px, 320 px and 320×440; inspect menus, focus, keyboard, scrolling, dismissal and settled originals.
- [x] Add and execute actual native protection-status reads, current-authority/held-result checks and human-form preservation with no mutation or credential exposure.
- [x] Run backend formatting/vet/race, TypeScript/build and ordinary/native regressions against the same frozen inputs; verify encrypted populated recovery, prior migration preservation and separate held keys.
- [ ] Document finite results and limits, archive the matching release/evidence, verify personal publication and continue the next independent production workflow.

The first eight items passed local verification on 12 October 2026; see the [accepted baseline](offsite-backup-baseline.md). Personal publication is pending; the matched private archive is prepared next. The final publication/continuation item remains pending until its evidence is recorded. These preparations do not establish a live off-site recovery point, accepted production policy, verified representatives or actual-host operation.

## Implementation decisions after household publication

The preceding application is `6ad98a0dda1c510bf60e2535bd184d836cdd9ec8`, with verified metadata child `ddc67c4f4197937eac7aeed671391a0869c65936`. Its immutable 521-source/925-hash archive, paired 0.29 preview and held keys remain preserved while this workflow is developed.

The following SQL transitions are the independent protection inventory. Existing functions reserve the writer and return a saved operation result before attempting another effect. Additional database triggers will guard the effects themselves, including composite paths, so an omitted caller cannot bypass protection. Unconfigured fictional development remains explicitly unprotected; configuring an off-site destination binds protection and cannot silently be undone by omitting its configuration on restart.

| Confirmed effect | Existing paths | Database boundary |
|---|---|---|
| Manual supplied debit/credit or already-received entry | `PostEntry` | Insert/update a `POSTED` entry |
| Bulk maintenance or fixed-fund charges | Separate maintenance/fund publication | Insert `POSTED` entries; any refusal rolls back every participant and the decision |
| Verified fund/fine external-payment report | `confirmExternalReceived`, optionally current same-home allocation | Insert `POSTED` entry/receipt and allocation or contribution; insert verified external identity and transition a report to `CONFIRMED`, including a link with no new allocation; no partial report decision |
| Linked manual/fund/fine correction | `ReverseEntry`, separate exemption/fine-waiver decision | Insert entry reversal and optional replacement `POSTED` charge |
| Explicit credit allocation and correction | `AllocateCredit`, `ReverseAllocation` | Insert allocation or allocation reversal |
| Voluntary fund attribution and correction | `AttributeFundCredit`, `CorrectFundContribution` | Insert contribution or contribution reversal |
| Receipt identity | Shared `issueReceipt` | Receipt counter insert/update and original receipt insertion |
| Accepted supplied paid-expense record, correction or void | Separate `DecidePaidExpense` approval | Change the accepted version of a paid-expense resource |

Draft entries and discard, proposed maintenance/funds/fines/expenses, refusal/withdrawal, current household access revocation, reads, original receipt rendering and exact accepted operation replies remain permitted. Publishing a voluntary tracking campaign or a planning budget does not itself confirm money. Tests will independently check rollback of balances, counters, child records, audit and operation acknowledgements, rather than merely inspecting a guard call.

The first configured policy is immutable and binds destination, fixture/live mode and two distinct public X25519 recipients; upload credential rotation does not change the destination binding. The selected released dependencies are age `v1.3.2` and AWS's Go v2 SigV4 signer module `v1.47.2`, checked against their primary documentation. Use streaming gzip/tar/age with a strict three-file format, physical size limits, full authenticated EOF and fresh private extraction/restore destinations. Capture age uses the time before SQLite snapshot capture starts, conservatively distinct from the old manifest's completion time. [age documentation](https://pkg.go.dev/filippo.io/age@v1.3.2), [AWS signer documentation](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2@v1.47.2/aws/signer/v4).

Retain policy, attempts and immutable events in three new tables. A lease reserves one attempt, and a staged encrypted file retains the same key and checksum across a rejected-request retry. A lost reply, conflict or unverifiable success is uncertain and cannot become a usable point through automatic retry. A later independent custodian can retrieve an exact supplied object version, match the retained encrypted checksum, decrypt and verify it before recording reconciliation. Its original capture time still determines freshness. Bound local unconfirmed staging and attempts; storage exhaustion is visible and preserves the financial pause.

Required protection warns at age 45 minutes and pauses at 60 minutes, with missing usable acknowledgement or clock rollback/future capture also paused. Persist the clock high-water mark and apply the guard inside the same SQLite transaction as the effect. There is no risk override in this checkpoint: the actual committee policy needed to authorize one is unavailable. Operators can inspect bounded status/history and refresh; the interface and native tools contain no backup configuration, upload trigger, credentials, private keys or override.

The scheduler uses a 30-minute cadence and starts automatically only with explicit private off-site configuration. Tests use numeric-loopback fixtures and disposable databases. A production S3 destination requires signed HTTPS, create-only upload, exact SHA-256 acknowledgement and a non-null version ID; deletion/lifecycle and custodian-read authority belong to separate principals. Supply a lifecycle/IAM preparation template and record that its live application, independently held identities and replacement-host recovery still need acceptance. Do not activate any live service in this development checkpoint.

Further before-code review on 12 October distinguishes an unconfigured fictional workspace from `LOCAL_WORKSPACE`. Only explicitly synthetic development may confirm financial effects without a bound destination. A non-fictional workspace with no configured recovery point must already report a financial pause and refuse counter/money effects transactionally; missing configuration is not an implicit risk acceptance. Its drafts, security changes and appropriate reads continue. Define and test this default before completing the checkpoint.

Numeric-loopback fixture storage is reserved for synthetic development; it cannot bind protection for `LOCAL_WORKSPACE` or release its financial pause. Positive non-fictional domain expectations model a validated S3 acknowledgement separately from the fixture HTTP integration. No local fixture is presented as actual off-site protection.

An exhausted encrypted candidate still occupies a retained staging slot. Four uncertain or exhausted candidates can halt new captures until a custodied disposition; simply deleting files does not change this inventory. Byte-preserving archival and slot release for known exhausted candidates are explicitly saved in the subsequent restore/reconciliation contract, alongside restored handoff quarantine. This checkpoint does not establish production recovery activation.
