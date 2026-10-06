# Separate fines, household responses and preserved money — checkpoint evidence

**Verified locally: 0.14.0-dev/schema 13, 6 October 2026.** Backend, TypeScript/build, complete ordinary browser and actual native WebMCP checks pass. Final rendered inspection, matching preview promotion and pre/post-upgrade recovery also pass. The [expectation brief](fines-workflow.md) precedes implementation. This is synthetic local acceptance; real authority, notice periods, liability and finance policy still require society acceptance.

## Outcome and permissions

A substantiated incident creates no debt. A current Treasury operator separately prepares a manually supplied amount, authority reference, reason and frozen household wording. A different eligible reviewer authorises the notice; notice publication creates no charge or receipt. Current members of the home can read that deliberate notice and submit their own responses without gaining financial access. Private case notes, original evidence and other people's replies remain restricted.

Before issuance, an eligible operator must review the current source and all response pages, record the disposition and separately issue one immutable charge. New replies or material source changes invalidate the prepared resolution; reopening, dismissal or a corrected home prevents stale issuance. Private operational notes do not change household-visible ordering or history. A retired rule remains a retained historical source; its frozen permission does not authorise new incident reporting against an inactive rule.

Registry administration and operational case review do not grant Treasury powers. Writers require current roles, a current confirmed factor, recent identity confirmation, expected versions and actor-bound operation identities inside the transaction. Reporters and proposers cannot make their own independent decisions. Current authority and home access are checked again before a previously accepted operation is replayed.

## Exact money and correction boundaries

The independent example issues **₹250.25**, verifies **₹100.00** already received and leaves **₹150.25** outstanding. A submitted claim or picture does not prove receipt of money. Treasury verification creates or links one original received entry and receipt using the shared external-payment identity across collections and fines. A compatible duplicate creates no additional cash, receipt or allocation. Explicit allocations across maintenance, funds and fines cannot exceed the original entry's usable credit.

Appeals and explicit review pauses preserve the charge and cash history. A pause remains explicit until an authorised decision changes it; passing a date does not silently lift it. A nonfinancial household member can appeal and respond without seeing a private ledger. A fully waived fine can still retain a genuine prior paid report with zero required allocation.

A separate **₹75.25** approved partial correction reverses the old ₹250.25 charge and creates its linked **₹175.00** replacement. The old **₹100.00** allocation is released: the replacement starts with zero allocated and the original receipt's ₹100.00 becomes available credit. Deliberate reallocation is required. Full reversal leaves zero live fine debt while retaining the original received money and receipt. Neither operation fabricates a refund or edits historical cash. The database rejects direct charge reversal authorised only by a pending correction; the approved reviewer and linked correction are applied atomically.

Private evidence uses the existing validated document contract and bounded current-home choices. Actual browser checks upload and retrieve the original synthetic evidence bytes, open the preserved receipt and download its real PDF, independently checking content, filename and SHA-256.

## Executed checks and rendered inspection

All required gate stages were executed separately and exited zero; this checkpoint does **not** claim one combined `make eval` invocation:

| Stage | Executed result |
|---|---|
| `make check` | Formatting, Go vet, race-enabled checks and TypeScript; **137 Go test declarations**. Backup 91.055 s and database 525.590 s; other packages have valid cached passes. |
| `make build` | TypeScript, production assets and matching Go binary pass. JavaScript 826.81 kB / 224.20 kB gzip; CSS 123.61 kB / 24.11 kB gzip. The existing chunk-size warning remains for the later performance checkpoint. |
| Complete ordinary browser corpus | **141 cases across 21 isolated synthetic suites**. Fines: 11 cases, 38.7 s; fine overview: two cases, 5.0 s. |
| Actual Chrome native WebMCP | **33 cases across two isolated suites**. Four fine journeys take 11.5 s; the 29 core journeys take approximately 1.1 min. |
| Affected browser rerun | The same 11 fine cases pass in 32.2 s after stronger hit-testing and fixture/capture improvements; the final scrolled source-warning capture case passes in 3.0 s. These are not additional cases. |

Ordinary evidence: `reports/local/fines-browser-final.log`, SHA-256 `836f54099842b660d0f96b88941fdb69c94b199defb0aa9da45a3f01a84f92ae`, UTC 09:48:11.282047–09:55:56.429282 on 6 October, with completion/exit zero in `fines-browser-run.json`. Native evidence: `reports/local/fines-native-isolated-final.log`, SHA-256 `7d985d905c2345c0715199ae6be584a669f386eef970b1e90e58dc2d7d7dadd5`, UTC 09:50:59.370727–09:52:18.337971, with completion/exit zero in `fines-native-isolated-run.json`. Backend/build logs are `fines-full-check-second.log` and `fines-build-accepted.log`.

The expanded native corpus initially reached the existing login throttle when all 33 cases shared one server. Core and fine suites now use separate fresh synthetic servers/databases and independent rate-limit state. The security limit remains unchanged. This is test isolation work, not a product finding or an additional case. `make webmcp-check` retains actual Chrome discovery/execution and requires native WebMCP; there is no fallback counted as native.

New backend declarations cover fine preparation, separate decisions, source/reply changes, current scope, external-payment identity, allocation/correction invariants, scoped overview, populated schema-12 migration, strict HTTP and populated backup restoration. Existing fund/payment race checks also pass after shared payment logic was extracted.

Rendered coverage includes actual prepare/notice/respond/resolve/issue controls, optional evidence upload, independent money verification, compatible duplicate reconciliation, clarification/revision, withdrawal/decline, full and partial corrections, appeals/pauses, receipt download, ended-home denial, stale decisions and lost-response retries. Opened menus, selected/hover/focus/keyboard behavior, tab navigation, close controls, internal scrolling and retained written fields are exercised at **1,440px, 768px, 375px and 320px**, including a short phone. Pending unknown writes retain the same retry identity and prevent dismissal.

All **42 final captures** were actually inspected through **15 regenerated contact sheets** containing 60 parts and **22 critical originals**. Source-warning paging, open filters, correction previews, receipt downloads, private/household views and unknown-write locks are included. The final source-warning capture deliberately scrolls the blocking warning into view. Independent hit-testing confirms the dialog close control has its own reserved row and at least 8px separation from content. Hashes, dimensions, source/test manifests, executed logs and inspection mapping are preserved privately in `reports/local/checkpoints/fines-0.14-accepted`. No application changes follow the accepted application gates. Passing this finite inventory does not prove every possible interaction, native mobile browser, physical camera or real society policy.

Native tools expose read-only bounded metadata for fines, notices, paid reports, appeals and corrections. They omit private allegation/response bodies, evidence bytes, actor identities, policy text and private events. Household notice tools expose no monetary amount to a nonfinancial member. Expected complete synthetic inventories are **43 staff, 35 financially entitled owner and 22 tenant tools**. Argument bounds, cancellation, current scope before/after reads, ended membership and held-result discard are exercised. No native tool posts, approves, uploads, responds, issues or shares.

The tenth changing Overview source distinguishes independent decisions, own requested clarification, current-home responses and explicitly paused/changed cases. Financial totals require separate entitlement. Exact attention links lead to the appropriate permitted record; an unavailable fine source stays unknown and retries locally.

## Five closed findings

| Finding | Observed problem and repair |
|---|---|
| F-01 | Fine source/evidence route patterns conflicted at HTTP mux registration. A dedicated `/api/fine-sources` route removes the ambiguity; the actual HTTP workflow passes. |
| F-02 | Resident list ordering used a private update timestamp. Resident ordering now uses the public snapshot timestamp; independent order/history assertions pass. |
| F-03 | A pending correction could authorise a raw charge reversal before approval. An approved-reviewer guard and atomic linked-correction order now reject that bypass. |
| F-04 | An unknown issuance outcome locked fields but left Close enabled. All nine fine dialogs retain the dismissal lock until deliberate retry/reload resolves the outcome. |
| F-05 | A source-change warning on a later response page left attestation selected and enabled. Paging now persists invalidation, clears/disables attestation and blocks decisions until fresh reload, including a return to page one. |

F-03, F-04 and F-05 retain meaningful failing checks followed by affected passes and complete acceptance. F-02 was found during code review and then tested; no pre-repair rendered failure is claimed. All five are closed. Cumulative accepted findings are **66 groups: 64 product/UI and two operational**. Fixture/locator corrections, capture framing, the native isolation change and a correctly rejected pre-migration backup command are not additional findings.

## Matching preview and recovery

The retained `var/preview-releases/0.14` binary/assets pair serves `http://127.0.0.1:8080`, readiness 200. All **18 packaged files** equal the verified build and all **17 HTTP-served assets** equal the retained files. The same developer database and separately held MFA key remain in use; all **49 prior persistent application tables** have identical rows after migration and startup. Private read-only verification: `reports/local/fines-preview-verification.json`.

The actual pre-upgrade schema-12 bundle, `var/snapshots/fines-0.14-pre-upgrade-20261006`, is **823,296 bytes**, SHA-256 `26346374c505b49b1fe50d2bbf94830783d31fb513d51f37cdc0a67954d228a5`, restored with the matching 0.13 binary in **30.988 ms**. Explicit migration then adds schema 13. The actual post-upgrade bundle, `var/snapshots/fines-0.14-20261006`, is **950,272 bytes**, SHA-256 `007bc75e34c5f474911a64e5fbd7f186baa39317ded4b3570e744a90da194c36`, restored with the matching 0.14 binary in **25.095 ms**.

Independent comparisons verify all 49 prior tables, unchanged migration provenance 1–12, seven new fine tables, all **56 restored persistent tables**, schema-13 provenance, integrity/foreign keys and zero sessions/access tokens/recovery codes/pending factors in the restored environment. The source MFA key is preserved separately. Evidence: `reports/local/fines-recovery-verification.json`. Fine tables are empty in the developer preview; a separate populated recovery test proves original incident picture/preview bytes, private review/history, frozen notice/reply, the shared payment/original receipt identity, exact corrected amounts, released credit and explicit appeal pause. That case does not claim receipt-PDF cache bytes were backed up; existing receipt regeneration remains the recovery contract.

The newer snapshot command correctly rejected the unmigrated schema-12 source without mutation. The documented matching-version pre-snapshot, explicit migrate and matching-version post-snapshot sequence succeeds. These restore timings are development-Mac samples, not target-host commitments.

Windows amd64 cross-compilation passes: **20,914,176 bytes**, SHA-256 `548f573e87a68798a26362214143c5ba90cf2d98f32ab8579eee8040c64d2d02`. The executable has not run on the supplied Windows computer. Real finance/rule approval, identities/data, hosting/origin, custody/off-site protection, provider onboarding and representative Windows pilot acceptance remain separate.

Public personal publication is verified as [commit 3a88f2f](https://github.com/flux-i/society-os/commit/3a88f2fa1d1413cda2e303907ca5e057b314307d). GitHub API main matched the exact local application commit and the tree was clean at publication. Only redundant EOF blank lines in two native test files were normalised after verification; executable contents are unchanged and the frozen source/evidence archive remains untouched.

## Continue the active plan

Next: registered contacts/consent, exact recipient targeting and synthetic WhatsApp/email delivery, followed by prepared financial statements/spreadsheet publication, scoped exports, prioritised community additions and measured performance/PWA/deployment/pilot preparation. The [roadmap](society-operations-roadmap.md) preserves those requirements. Missing real inputs block dependent activation only. The portal continues to record manually supplied charges and money already received; no payment initiation, bank automation, runtime model or Tally integration is introduced.
