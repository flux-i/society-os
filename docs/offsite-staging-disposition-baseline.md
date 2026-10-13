# Custodied retained-copy archival baseline — 0.31

**Locally accepted:** 2026-10-12T11:04:48.146142+00:00, Go/React **0.31.0-dev/schema 28**. The [before-code contract](offsite-staging-disposition-workflow.md) is complete for this bounded workflow. Personal application [754ebb01](https://github.com/flux-i/society-os/commit/754ebb01490f38002704c9718268b020ecabe947) was verified on 2026-10-12T11:17:52.849312+00:00; documentation child [ba3d5161](https://github.com/flux-i/society-os/commit/ba3d516156a784c759d38f05a5a3d8d6ef85c3a3) followed at 11:19:33 UTC. Its immutable private evidence archive binds **565 source files / 862 verified hashes**, including all 77 reviewed originals and matching recovery. All sixteen preceding archives / 12,157 hashes were verified unchanged at publication. The whole goal continues. The broader [restore/reconciliation contract](restore-reconciliation-workflow.md) continues with restored-journal quarantine, current authority/external-job review and fresh recovery numbering.

## Implemented behavior

An offline custodian can copy an exhausted known-failure encrypted candidate to a fresh private custody directory outside active staging and its source directory. The command copies the exact encrypted bytes, fully decrypts and verifies the original job/configuration/capture identity and snapshot manifest, syncs the ciphertext/manifest/directories, then records one immutable disposition. The custody directory contains only unchanged ciphertext and versioned `custody.json`; private keys and plaintext databases are excluded.

A completed archive survives a failed or lost journal acknowledgement. Deliberate same-archive resume verifies that retained copy with either independently held identity and returns the one original disposition. Different archive identity, changed proof/note, unsafe members or an existing destination without explicit resume are refused. The source, failed job/hash/capture, original event history, receipts/counters and storage acknowledgements remain preserved.

Only sealed, unacknowledged `FAILED` candidates with five exhausted known-rejection attempts and no lease qualify. Uncertain, active, retryable, unsealed or already acknowledged originals remain retained. A committed disposition releases exactly one active journal slot; restart cleans only its matching staged directory, preserves unknown/uncertain evidence and permits a fresh job/key at the normal cadence. This is independent custody retention, without an S3 verification, capture-age refresh or financial-pause override.

Current operators see the separate custody count, retained-copy fact and private verification note alongside the unchanged failed-upload history. Residents remain denied. Native WebMCP exposes bounded current protection/count metadata, discards held results after disposition, preserves the human financial draft and exposes no archival mutation, private paths, keys, object/version/configuration identities, checksums or custody notes.

## Executed evidence

| Check | Verified result |
|---|---|
| Backend | **318 Go test declarations**, formatting, vet and complete race check |
| Ordinary browser | **256 cases / 40 isolated suites**, 199 cases in 32 retained passing prefix suites plus 57 in the corrected rehearsal and all seven following suites |
| Actual native WebMCP | **84 cases / 19 suites** |
| Direct visual review | **77 original PNG files: 72 ordinary and five native** |
| Current project/build | **487 inputs / 55 outputs**; seven outputs are historical non-web binaries; current runtime **48 files / 47 served web files** |
| Current full check | Ended 2026-10-12T11:02:02.569667+00:00, SHA `17878c5399208da35dca123db63b1bb16252e434e8ce33675233d7ff9d1df3fc` |
| Windows | amd64 **compile only**, 24411136 bytes, SHA `19b2fe4f89f5c47999b575cd9b0aa22fe4529b863e2d05f3a03d2eb51907b54b` |

Checks include fourteen independent ineligible domain/direct-SQL cases, three authenticated job/configuration/capture mismatches, nine changed/unsafe resume cases, wrong keys/corrupt/truncated bytes, genuine migration, conflicting simultaneous archives, CLI pre-I/O denials and HTTP privacy/financial-pause boundaries. An actual candidate receives five independently validated signed upload rejections with no stored object; three explicitly synthetic journal rows supply the browser's capacity boundary. Their separate physical ciphertext custody is not claimed.

The actual CLI retains a completed copy through an injected SQLite acknowledgement failure and deliberately resumes once with the second identity. Its failed original/history stay unchanged; the cap clears and one custody record appears while confirmation of a **40001-paise** human draft remains refused with no receipt. The native held read sees the changed count/slot state and is discarded; its human draft/unchecked review remain unchanged.

Opened selected/hover/focus menus, keyboard last-option/Escape, internal history scrolling, visible close, empty/loading/error/retry/pending and allowed/denied access are exercised at 1440×1000, 820×1050, 375×812, 320×740 and 320×440. Initial short protection/capacity/native/registry frames sometimes show only the heading above offscreen content; the short custody frame shows the card's upper portion. Separate longer card/history captures and actual geometry/oldest-event/close assertions supply that coverage. Overlapping files are counted as files, without claiming distinct states or every possible interaction. Projected age/capacity UI states have separate real domain expectations.

The first aggregate ordinary driver remains failed because a global rehearsal alert selector matched both an installation notice and the financial dialog. Only its 32 wholly passing suites count; partial failed rehearsal cases are excluded. Both alert assertions now target the actual dialog; the entire rehearsal and remaining suites pass. Native/prefix whole-project frames changed in two unexecuted Go test files and that unexecuted ordinary rehearsal spec. Current runtime and every actually executed UI/tool/support input match; the private collector proves that continuity rather than claiming an unchanged whole-project frame. Current build, corrected continuation and complete Go/type gate use unchanged current inputs with valid Go cache. Fresh current backup race takes 297.490 s; the preserved complete database/server runs take 1520.183 s / 199.810 s on exactly current package inputs. Earlier failed frames stay retained.

**CUD-01** is the one observed product repair: the actual native read initially omitted `archived_count`; its independent assertion failed, then passed after projection repair. The closed ledger is now **122 groups: 120 product/UI and two operational**. Proactive SQL eligibility/digest parity and hook-budget, responsive scroll settling, schema inventory and alert-selector corrections are not additional product groups. No pre-fix SQL red is fabricated.

## Matching recovery and retained preview

The actual schema 27→28 upgrade preserves all **103** prior persistent tables, every original row/SQL object/provenance entry and both separately held developer keys. Only the empty immutable-disposition table and its guards are added. All **104** current business tables restore with integrity/foreign-key checks and bearer credentials purged.

| Bundle | Version/schema | Bytes | SHA-256 | Restore |
|---|---|---:|---|---:|
| Before upgrade |0.30/27|1572864|`1722f3523c1c514ab307b320c5dfd140c8f2b411c5a2445955f40f30506d183d`|34.938ms|
| After upgrade |0.31/28|1585152|`85eba4009e31e4eb39327c28cce7b85cf23c5c0f5b1d52f53f018c4f01d068e6`|27.576ms|

A populated post-disposition matching snapshot independently restores all **105 tables including provenance**, one saved disposition and the unchanged exhausted original; it remains financially paused, cleans only the committed staged original, never sends a sixth upload and returns the saved same-archive retry. The full configured personal-account rehearsal preserves 104 business tables, SQL/BLOBs/private originals, the **40000-paise** receipt and **150001-paise** linked-reversal balance; fresh personal/MFA login works, the old bearer is rejected, A-102 remains the owner's financial scope and the ended tenant has zero homes. Separate populated encryption tests use either custody identity and exercise authentication/EOF/bounded extraction.

At acceptance, the pinned paired preview ran at http://127.0.0.1:8080 under `var/preview-releases/0.31`, PID 80867. All 48 packaged hashes and 47 served hashes matched; 104 tables/provenance and both keys survived startup. Prior 0.30 PID 54149 was identified exactly and stopped gracefully. The user's later background-process stop request was completed on 13 October 2026: PID 80867 exited after SIGTERM, port 8080 was clear and all project background processes were verified absent. The preview remains stopped. Existing developer data/keys, earlier releases/snapshots and accepted archives remain preserved. The retained synthetic workspace stays unconfigured and explicitly unprotected.

Actual off-host physical custody, custodian authority/adopted policy, live storage/HTTPS/Windows operation and production acceptance remain separate. Archival does not establish current grants, reconcile lost external jobs or reserve a fresh receipt series. No live bucket, paid service or external message was activated. Continue the remaining parent restore workflow after personal publication.
