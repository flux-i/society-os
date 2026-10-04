# Local registry and identity slice · 4 October 2026

Historical schema-2 checkpoint. The next delivered slice is recorded in [account-security acceptance](account-security-baseline.md).

The accepted visual design is preserved. The overview now shows homes, occupied/vacant homes and distinct active owners/tenants, with owner-occupied/rented home totals and per-wing people counts. The fresh fictional fixture has 118 flats, 109 occupied homes, 9 vacant homes, 118 owners and 35 tenants. Joint owners count as separate people, multi-home owners count once society-wide, and former/future memberships are excluded from active counts.

Schema 2 / application 0.2.0-dev adds working password sign-in for four fictional accounts, server-enforced current role/membership scope, occupancy updates, new/existing-person relationships, relationship ending, primary-contact replacement and immutable transactional change history. The [README](../README.md) explains the account chooser and workflows. No payment or finance-entry endpoint is implemented.

| Acceptance check | Result |
|---|---|
| Go formatting/vet/race tests and TypeScript | Pass |
| Go/frontend build and existing schema-1 preview upgrade | Pass; prior registry retained |
| Linux amd64/arm64 pure-Go builds | Pass; target-host runtime acceptance remains pending |
| Society and wing active people counts, joint/multi-home cases | Pass |
| Anonymous registry/detail/count/system/lookup/history denial | Pass |
| Owner limited to two homes; tenant limited to one; scoped search/counts | Pass |
| Tenant denied another home and former-tenant history | Pass |
| Resident and committee registry writes denied | Pass |
| Origin/CSRF, strict bounded JSON, logout and login throttling | Pass |
| Idle/absolute expiry, auth-version/account changes and expired roles | Pass |
| Ended membership revoked on existing session; other active home retained | Pass |
| Concurrent edits: one save and one conflict | Pass |
| Failed relationship changes leave no person/version/audit side effect | Pass |
| Audit actor/reason/state preserved; update/delete rejected | Pass |
| Snapshot preserves identity/audit, excludes bearer sessions and leaves live session intact | Pass |
| Independent snapshot/restore SHA-256, counts and engine verification | Pass |
| Six Playwright journeys on a separate fresh fictional database | Pass |
| Desktop screenshots and 375px login/overview/forms; modal/page overflow | Inspected; no overflow in checked journeys |

There are 21 backend acceptance tests and six browser journeys. The browser journeys exercise occupancy saves/history, linking an existing tenant, ending a relationship, retained historical records, account switching and cross-flat API denial, mobile login/form/stale-save reload, overview counts, search/filter/pagination, keyboard modal dismissal/focus, empty/retry behavior and reduced motion. Browser mutations run in a disposable database on a loopback port and leave the user's preview unchanged.

The original anonymous baseline was 0.826 ms read p95. The authenticated slice records 100 sequential warm HTTP reads after ten warmups: **0.910 ms p95**, with a **64.069 ms** password login. A **200,704-byte** snapshot restored and verified in **13.368 ms**. See the local [JSON report](../reports/local/registry-auth-baseline.json) for environment, samples, SHA-256 and connection settings. Authentication adds work and changes the measured journey; these observations are not evidence of a speed improvement or a production SLA. The earlier [foundation baseline](foundation-baseline.md) remains the historical reference.

Generated visual evidence:

- [Updated overview](../reports/local/overview-desktop.png)
- [Phone overview](../reports/local/overview-mobile.png)
- [Sign-in](../reports/local/login-desktop.png)
- [Phone sign-in](../reports/local/login-mobile.png)
- [Home management](../reports/local/manage-home-desktop.png)
- [Phone add-person form](../reports/local/add-person-mobile.png)

The server remains loopback-only and synthetic-only. Privileged MFA, verified invitation/contact flows, password/MFA recovery, audited identity/role administration, real-data policy/migration, HTTPS, encrypted off-site backups and custodial access reconciliation remain unfinished production requirements. The current administrator permission covers registry management; it does not establish financial posting permission. Subsequent work follows the [execution backlog](../execution-backlog.md).
