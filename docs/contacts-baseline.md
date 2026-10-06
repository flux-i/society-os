# Registered contacts and communication choices — accepted local checkpoint

**Verified:** 6 October 2026. **Release:** 0.15.0-dev, schema 14. The [before-code contract](contacts-messaging-workflow.md) defines this workflow; [delivery](messaging-workflow.md) is the next complete workflow. The continuing preview uses a retained, tested binary/assets pair. Contacts are independently attested offline; this checkpoint sends no verification code or provider message.

## User outcome and permissions

A resident registers an international WhatsApp number/plain email, preferred channel and four independent choices: community/financial messages over WhatsApp/email. Choices default off. A registry officer can record supplied details with an explicit identity and permission source. A different current community reviewer verifies the recorded identity and choices; neither proposer nor target can self-verify. Adding permission or changing a destination creates a pending revision with zero delivery eligibility until review. Decline and withdrawal preserve the proposal and decision.

A person can immediately stop a selected channel/purpose or all messages. Restoring consent needs a new registration and separate review. A former member can read their own retained profile and stop messages; they cannot establish new delivery eligibility. Residents cannot read another person's addresses/history, including a joint owner or someone in the same home. Treasury/auditor appointments alone grant no contact directory. Contact powers add no financial authority; existing finance permissions remain separately enforced.

Registration and privileged review require current authority and recent identity confirmation. A signed-in person can stop or withdraw their own preferences without another recent-authentication challenge. Writer transactions check target relationship, factor/role, expected version and actor-bound operation identity before successful replay. Retried unknown responses preserve one change/event. Changed details under the same operation conflict. The page retains the human reason during an explicit stale-record reload and requires a fresh attestation of the newly loaded details.

Current people and history are paged. The fresh fixture has 153 distinct current people: 118 owners and 35 tenants. Wing A has 40 owners and 12 tenants; multi-home people count once. Contacts do not change login email, registry identity, financial entries or receipt history. Strict JSON rejects unknown fields and requests above 32 KiB. Phone input requires explicit international format; email validation rejects headers, control characters and display-name syntax. Private immutable contact events preserve prior snapshots. Broader audit metadata omits destinations, permission references and private reasons.

## Executed gates

| Gate | Result and limit |
|---|---|
| Backend formatting, vet and full race suite | Passed; 145 Go test declarations, including eight new contact/domain/migration/HTTP/recovery declarations. All 132 backend files in the original run manifest still match. |
| TypeScript and production build | Passed. Latest JS is 847.94 kB / 229.06 kB gzip; CSS is 128.31 kB / 24.89 kB gzip. The chunk-size warning remains for measured performance work. |
| Full ordinary browser regression | 149 cases passed across 22 isolated synthetic suites. Final driver records zero exit and unchanged source/assets. |
| Actual native Chrome WebMCP | 36 cases passed across three isolated synthetic suites. Final driver records zero exit and unchanged source/assets. |
| Final visual inspection | All 30 captures mapped through ten sheets/38 parts were actually viewed after the final checkbox fix; twelve critical originals were also viewed at original size. |
| Matching preview and recovery | Passed, including independent exact-row checks, migration provenance, served asset checks and separately retained MFA key. |

The first aggregate `make eval` passed backend/TypeScript/build but failed an older incident test that did not wait for its dialog to mount. Its synchronization was corrected; the affected journey and final ordinary regression passed. A native run then failed an older expectation of 35 owner tools after two legitimate contact tools were added. The corrected case checks the exact 37 permitted owner tools, and the full native rerun passed. These are QA fixture/inventory corrections, not additional product findings. No literal green aggregate `make eval` invocation is claimed. Only that native expectation changed after the full ordinary run; application source and build assets remained unchanged.

Private evidence: `reports/local/contacts-full-eval.log`, `contacts-browser-final.log`, `contacts-native-final.log`, their driver metadata and `contacts-gates-verification.json`. Earlier failing logs remain retained. The frozen private archive is `reports/local/checkpoints/contacts-0.15-accepted`.

## Contact interaction and native coverage

Eight new ordinary cases cover the following actual controls and results:

- Directory search and owner/tenant/wing/state filtering, exact distinct-person counts, opened custom selection, keyboard/focus/hover, one-pixel borders and empty results at 1440/768/375/320 widths.
- Resident registration, default-off choices, bounded fields, human preview/attestation, independently verified selected permissions and zero monetary/receipt changes.
- Selected financial WhatsApp opt-out retaining the other three permissions, changed-address suspension and immutable prior history.
- Server-commit/lost-response retry with the same request identity, locked fields/close/Escape and one version/event; retry remains reachable after internal scrolling.
- Stale decision/reload retaining the human reason, fresh destination, cleared attestation and disabled confirmation.
- Retryable list failure, detail loading, hidden stale results and denied other-person detail without private leakage.
- Former-member retained personal access and immediate stop in a 320 × 480 dialog.
- Actual decline/withdrawal, retained history across bounded pages, expansion/scrolling, close/hash dismissal and opener focus at all four widths, including short phones.

Three added native cases execute Chrome's real `document.modelContext` API. `society_find_contacts` and `society_read_contact` expose only bounded current-scope status/preference metadata. They omit addresses, identity/permission references, home lists, actors and private history. Checks cover exact filters/counts, unchanged unsaved human forms, no mutations, same-home privacy, unsupported inputs, cancellation and discarded held results after membership/logout changes. Accepted tool inventories are 45 for the full fictional staff account, 37 for the financial owner and 24 for the unentitled tenant. Discovery alone grants no authority.

The inspected captures include fields, custom selection, independent verification, selective stop, changed destinations, stale reload, unknown retry, paged history, loading/error/denial and former-member states. Some filenames ending in `1440` actually contain 1280 × 720 captures; actual dimensions are recorded in the manifest. Finite checks and captures do not prove every possible interaction, input, device, locale or browser.

## Findings closed

| Group | Observed problem and accepted fix |
|---|---|
| CT-01 | Contact controls initially referenced missing shared style values; borders/type/search and nested checkbox alignment/spacing were wrong. Shared tokens/control styles and explicit checkbox spacing/44px targets now pass meaningful failing-then-passing assertions and actual visual review. |
| CT-02 | A stale-record reload unmounted the human decision form and lost its private reason. The parent retains the draft; reload shows current details and clears the old attestation. The observed browser failure and affected/full passes are retained. |

Both groups are closed locally. Cumulative accepted findings: **68 groups — 66 product/UI and two operational**. Capture framing, locator corrections, legacy dialog synchronization and native inventory maintenance are not additional findings.

## Recovery and preview

Before promotion, the exact 0.14 preview command/PID was verified and stopped. Its matching schema-13 pre-upgrade bundle, `var/snapshots/contacts-0.15-pre-upgrade-20261006`, is **950,272 bytes**, SHA-256 `63bb76bd10d17172cdab20d8ae37e89e01187e551fc672799d613357f3516097`; matching 0.14 restore took **34.847 ms**. Explicit migration adds schema 14. The matching 0.15 post-upgrade bundle, `var/snapshots/contacts-0.15-20261006`, is **974,848 bytes**, SHA-256 `ea3cb587cbb6d686d55806936888c51b25eea1b6643b79c8d7cfaeb8f40f4503`; restore took **35.965 ms**.

Independent checks prove exact rows in all 56 prior persistent tables, unchanged migration provenance 1–13, two new tables, all 58 restored persistent tables, provenance 14, integrity/foreign keys, four empty temporary credential tables and the separately preserved matching MFA key. Contact tables are empty in the continuing preview; a separate populated backup case recovers verified destinations, consent, selected opt-out and retained history while excluding a later revision and invalidating prior sessions. Restore timings are development-Mac samples.

Readiness is 200 at `http://127.0.0.1:8080`. The retained `var/preview-releases/0.15` pair uses the same developer database/key. All 18 packaged files equal the tested build; all 17 HTTP-served files equal that pair. Prior rows remain unchanged after startup. Evidence: `contacts-recovery-verification.json` and `contacts-preview-verification.json` under `reports/local`.

Windows amd64 cross-compilation passes: **20,986,368 bytes**, SHA-256 `e66e00e29f6e78477586e4520984e22bbc70ff97f546467f7e8c2c80070738ab`. Actual execution/performance on the supplied Windows computer remains untested.

## Next and limits

Continue the [targeted synthetic delivery workflow](messaging-workflow.md): exact source/audience/consent preview, separate approval, persistent provider-neutral attempts, suppression and unknown-outcome reconciliation, signed/deduplicated synthetic callbacks and truthful states. No delivery implementation or live channel acceptance is claimed here. Real provider credentials/onboarding, approved templates, identity/policy acceptance, infrastructure/key custody and Windows/pilot inputs remain separate. Payment initiation, runtime LLMs, bank automation and Tally remain outside the current scope.
