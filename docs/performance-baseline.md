# Measured loading and navigation — checkpoint evidence

**Locally accepted: 0.26.0-dev/schema 24, 8 October 2026.** The [before-code contract](performance-workflow.md) required at least 20% less initial JavaScript without losing current workflows, identity checks or native tools. The final build removes **635,243 decoded bytes (57.7681%)** from sign-in. Current full checks, directly inspected visuals, matching recovery and the retained preview pass. Personal [application `0efb51c8a21d165a329390f08e6d37bf4f446bcd`](https://github.com/flux-i/society-os/commit/0efb51c8a21d165a329390f08e6d37bf4f446bcd) is published and verified at `2026-10-08T14:28:33.213420+00:00`: personal account/author, public repository, credential-free origin, clean tree and GitHub API main match. Immutable `reports/local/checkpoints/performance-0.26-accepted` binds all **476 source files/704 hashes** to that application commit. A six-document metadata child records this proof without changing tested application/runtime or archive. The preceding 0.25 archive remains unchanged. Whole-plan work continues. Closed findings are **100 groups: 98 product/UI and two operational**; this checkpoint closes PERF01 and PERF02. Payload reduction itself is not a bug count.

## Implemented outcome

Fourteen operational screens load their code when opened. The workspace, navigation and sign-out remain usable during a slow or failed import. Accessible loading and failure states preserve the current location; deliberate reload recovers the same saved record. Ordinary module notifications do not automatically refresh an active form. Native registration loads only when the browser supports WebMCP and the current user is eligible; logout discards a late registration.

Nineteen fragment subscriptions now reconcile the current URL in a shared layout effect before first paint. This retains each existing screen's permission and source logic while fixing a link change during first module loading. No new financial authority, payment initiation, runtime LLM or offline private-data storage is introduced. Installation/offline support follows its own [before-code contract](pwa-workflow.md).

## Comparable measurements

Before editing application code, freeze the accepted 0.25 binary/assets and the measurement executable, helper and package lock. The final candidate uses that same executable, environment and fictional 118-home fixture: five separate cold/warm browser contexts, seven operational routes per context, 500 protected list reads per build and **zero domain mutations**. Authentication uses actual password/MFA and is measured separately. Service workers are blocked in both measurements.

| Measure | Accepted 0.25 | Final 0.26 |
|---|---:|---:|
| Initial decoded JavaScript, every sample | 1,099,644 bytes | 464,401 bytes |
| Initial transferred JavaScript, every sample | 1,099,944 bytes | 465,301 bytes |
| Cold sign-in median | 67.974958 ms | 59.215 ms |
| Warm current-overview median | 61.484125 ms | 62.70225 ms |
| Password/MFA to current overview median | 257.642208 ms | 251.022167 ms |
| Protected list median / p95, 500 reads | 0.858083 / 1.159709 ms | 0.883208 / 1.214416 ms |

The independent budget is at most **879,715 decoded bytes**. Final results establish the byte reduction. Five local timing samples vary and overlap; they establish no general latency or memory improvement. Earlier candidate reports, including warm medians 45.709917 and 67.3045 ms, remain preserved and excluded from final acceptance rather than substituted for the final result. No supplied Windows/8 GB computer, production network or constrained-resource performance claim is made.

Private before report SHA-256: `04c968db28a4ec0337c8cbb0866208226277ceea95d481f8d45ec978dece822d`. Final report: `2bf2a9595ebca33aa920409847da7cfaf49064754041fd32b1faca99fba88389`. The public [measurement script](../scripts/measure-loading.mjs) differs from the frozen executable only in its relative helper import.

## Executed checks and observed repairs

**262 Go declarations; 223 ordinary cases across 33 isolated suites; 70 actual native Chrome WebMCP cases across 13 isolated suites.** Formatting, vet, race tests, TypeScript and production build pass. Backend source differs from accepted 0.25 only in the CLI release string; unchanged Go packages legitimately reuse verified test cache. All final stages match **415 application inputs** and the **42-file current runtime**. Driver manifests also retain seven older unserved build outputs; those are not new cross-build evidence. Required stages pass separately; no literal aggregate `make eval` invocation is claimed.

| Final stage | UTC completion | Log SHA-256 |
|---|---|---|
| Formatting/vet/race/TypeScript | 14:05:23.475719 | `7a4e9fc1f92795e92ba72db8eeba93905e94cf3dfe8cd4032b71064f1c58aa35` |
| Production build | 13:58:31.766372 | `3ec34f32589cb6a5b8a50c2fa602e5fdc50d4e0820a4b251a748b8fce3507642` |
| Focused five browser cases, 23.1 s | 13:59:29.214876 | `19ceff227b1a0607ab52e20ab86aab81b8a22de30d30c4ac1bfa8e6573805c41` |
| Focused two actual native cases, 5.2 s | 13:59:10.533114 | `d2dc2520fa07e807cb3e9b2b23f2012df0a665fa2688aa859f2c1e3416120e38` |
| Complete ordinary suites | 14:20:53.414423 | `064baaa4595f7d929d1c80633a360bb913989e7e0cba8f3eba4c341ae6e89ac9` |
| Complete actual native suites | 14:08:39.069825 | `c9b602809e6d52e8db870ceb5aec4690d2d1598baba40f0ba3b961550d93d36e` |

All **24 final original PNGs** were directly opened: 23 ordinary and one actual native. Selected desktop, tablet, 375 px, 320 px and 320×440 journeys cover loading, failure, deliberately scrolled reload actions, recovered originals, opened menu selection/hover/keyboard/focus, preserved human input, logout and native discovery during loading.

| Finding | Observed failure | Repair and independent evidence |
|---|---|---|
| PERF01 | Recovered record heading overlapped the Close focus area at 320×440 | Scoped heading clearance, independently checked against Close geometry at four widths and directly viewed captures |
| PERF02 | Changing a fund/report link during the first lazy mount left the older fund dialog under the newer report URL | Shared immediate fragment reconciliation; the unchanged original collection test reproduces red and then passes, plus a controlled held-module case verifies the current report and revision without another receipt or cash effect |

The unchanged collection test retains independent version-4, 1000-paise allocation and 150025→149025 outstanding expectations. The new controlled case retains a rejected report, then a pending 1000-paise claim and unchanged 150025 outstanding with no receipt. A saved received-money record remains exactly 4321 paise with one original receipt after deliberate reload. Test-fixture corrections (flat response shape, valid REJECTED action and separate actor contexts) add no product findings. Earlier red/superseded cohorts remain private and excluded.

## Matching recovery and preview

Pre-upgrade 0.25 and post-upgrade 0.26 snapshots both use **schema 24**, **1,466,368 bytes**, SHA-256 `4b494b47ea0d7938af539d050254b3e74cf0841216413ce1dd25209670ea3b6a`. Matching restores take **23.33 ms before** and **33.883 ms after** on this machine. All **96 persistent tables**, rows, definitions, SQL objects and migration provenance remain exact; integrity and foreign-key checks pass. Restores purge four temporary credential tables. Both separately held keys remain unchanged. Existing source-identical populated backup cases retain financial originals/corrections, receipts, pending reviews, synthetic handoff and key coverage.

The retained pair `var/preview-releases/0.26` serves http://127.0.0.1:8080, detached **PID 49143 when started**; revalidate its exact command before process actions. Readiness and all **42 packaged/41 served hashes** verify. Developer data and keys are preserved; browser mutations used disposable fictional databases. Main JS is 449.78 kB/138.15 gzip, CSS 191.54/34.00; initial JavaScript also includes shared static dependencies measured above. Windows amd64 output is **23,321,088 bytes**, hash `9a874466a198ee70bf647b749219c81d72a93b3f3037b1d5398f60383b49fef5`, **compile-only**.

Private evidence is under `reports/local/performance-*`. The preceding immutable 0.25 archive retains **467 sources/809 hashes** unchanged. Whole-plan work continues through installation/offline safety, synthetic migration/pilot rehearsal and independent production preparation. Real identities/data, policy/custody, credentials, host/network evidence and human pilot inputs block only dependent activation or acceptance. Finite local checks do not establish every possible interaction or production acceptance.
