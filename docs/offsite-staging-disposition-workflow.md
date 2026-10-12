# Custodied retained-copy archival — next bounded checkpoint

Saved on 12 October 2026 after verified 0.30 publication, before implementation. Application `421c6d8c5099f2edbac64d7c3df5e707416dbecb` and documentation child `195c7bd064f6faf1153d3e9de86c4fab75483543` are preserved with the immutable 554-source/1,344-hash archive. The paired 0.30 preview and existing data/keys remain in place while disposable fictional fixtures are used.

This is the first complete subworkflow of the [restore/reconciliation contract](restore-reconciliation-workflow.md). Complete and publish it before implementing restored-journal quarantine, current-authority/external-job review and recovery numbering. These remain separate required work; this slice does not release restore mode or establish production custody.

## Outcome and code at contract time

Four retained encrypted uncertain or exhausted jobs can fill the four-slot staging cap. Deleting their files does not release their journal inventory. An independent custodian must preserve and decrypt/verify an exhausted known-failure candidate before an immutable disposition releases that candidate's active staging slot. Ordinary scheduled capture can then resume at its normal cadence with a fresh identity and key.

`ClaimOffsite` and the status reader count encrypted non-verified jobs. The worker's restart cleanup currently removes only verified originals. An archive contains the original job ID, configuration hash, capture time and verified snapshot manifest. `DecryptArchive` already checks the whole authenticated archive with bounded extraction; reuse it. Existing backup events and verified jobs are immutable. Add a disposition record without editing the source snapshot, failed job, existing event history, receipt, counter, acknowledgement or policy binding.

## Permission and preservation boundary

Use an offline custody command with a private identity file and a fresh private archive destination outside active staging. The running writer continues to receive public recipients only. No web form or native tool accepts decryption identities, archives a job, deletes a retained candidate or lifts financial protection. Current operators may read bounded disposition/history metadata; residents remain denied. Do not return private paths, keys, ciphertext hashes or configuration credentials through those reads.

Only an exhausted `FAILED` candidate with five attempts, no active lease, no storage acknowledgement and a retained encrypted hash/size is eligible. An uncertain upload, live claim, unsealed failure or verified point is denied. Retain the encrypted bytes exactly; decryption must match the retained job/configuration/capture identity and manifest before disposition. An archive never manufactures a usable S3 acknowledgement or refreshes capture age.

The archive contains the unchanged ciphertext and a versioned private custody manifest, without private keys or retained plaintext database files. Sync and verify the complete copy before committing disposition. Existing destinations are refused by default. If a completed copy survives a failed/uncertain database acknowledgement, a deliberate resume verifies that same archive identity and returns the saved disposition rather than making another effect. A different archive identity for an already disposed job conflicts. Source ciphertext and existing custody archives stay preserved on failure.

The scheduler excludes disposed jobs from active slot inventory and never retries their original upload. Restart cleanup may remove only the matching active staged directory after a committed verified disposition. Unknown directories and uncertain candidates remain retained. Independent actual-host custody and physical separation need later acceptance; local fixtures verify the byte and journal behavior only.

## Independent expectations

- Four encrypted exhausted candidates initially prevent a fresh capture; a verified disposition releases exactly one slot, keeps the original failed job/hash/capture/history unchanged, and permits a distinct scheduled job when the cadence allows.
- Wrong identity, changed/truncated ciphertext, mismatched job/configuration/capture, unsafe paths, existing destination, uncertain/live/unsealed/verified candidate and stale/concurrent disposal are rejected without changing inventory or financial records.
- A retained same-archive retry returns one original disposition. Failed/uncertain acknowledgement never deletes the complete archived copy or fabricates a second disposition.
- Archive alone leaves the existing financial pause and usable-point age unchanged. New snapshot protection requires its own ordinary validated acknowledgement.
- Operator UI distinguishes failed upload from independently retained encrypted bytes. Real filters, event pagination, loading/error/retry, fixed-header scrolling, Escape/focus and desktop/tablet/375/320/short320 layouts stay usable.
- Actual native reads expose bounded current protection/count metadata, discard a held result after disposition and preserve an open human financial form. Human history stays separately scoped; no native mutation or custody material is discoverable.
- Run domain checks, formatting/vet/race, TypeScript/build and ordinary/native regressions on matching inputs; inspect originals, preserve prior tables/provenance and matching recovery, archive and verify personal publication before continuing.

The bounded workflow is locally accepted in 0.31/schema 28 on 2026-10-12T11:04:48.146142+00:00; see the [executed baseline](offsite-staging-disposition-baseline.md). Immutable evidence archival and personal publication remain pending. Real custodian authority, adopted policy, physical off-host retention, actual Windows/HTTPS operation and the remaining restore-release workflow are not established by this contract.
