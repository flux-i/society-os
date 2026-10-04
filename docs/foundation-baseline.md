# First local foundation · 4 October 2026

This records the original schema-1 milestone. The current schema-2 build and additional acceptance evidence are documented in [registry/identity baseline](registry-identity-baseline.md).

The first build is a working local Society OS preview with an illustrated overview, wing cards, search/occupancy filters, pagination and home/relationship details. The underlying fictional fixture has **118 flats, 3 wings, 154 people and 155 current/historical memberships**. No real resident data was imported.

Run it with `make run` and open `http://127.0.0.1:8080`. See [README](../README.md) for setup, checks and recovery commands. The current preview server is explicitly local and synthetic; it does not provide production authentication or financial entry/receipt workflows yet.

## Recorded checks

| Check | Result |
|---|---|
| Reproducible Go/frontend build | Pass |
| Go formatting, vet and race-enabled package tests | Pass |
| TypeScript check | Pass |
| Migration repeat/checksum protection and registry constraints | Pass |
| WAL, FULL, foreign keys and busy timeout on four simultaneous connections | Pass |
| Synthetic seed repeat, joint/multiple-flat owners and former-member handling | Pass |
| Liveness/readiness failure behaviour and bounded read-only API | Pass |
| Snapshot includes committed WAL changes, excludes later changes | Pass |
| Corrupted snapshot and existing database/WAL target rejection | Pass |
| Independent Python SHA-256 and restored-count verification | Pass |
| Three Playwright workflow tests | Pass |
| UI page width at 375, 768, 1024 and 1440 px | Fits viewport for overview and homes |
| Browser console/page error review | No issues in inspected journeys |
| Linux amd64/arm64 pure-Go compilation | Pass; target-host runtime tests remain pending |
| Go module verification / npm installation audit | Verified / zero reported npm vulnerabilities |

The browser cases exercise wing navigation, shared-owner search, no-results/clear, pagination, vacancy filters, joint-owner detail, native Escape/close, focus return, skip-to-content, request failure/retry and a 375px reduced-motion layout. Visual screenshots were inspected separately. Future identity/finance scenarios remain pending rather than being represented by passing placeholder tests.

## Local measurement

Environment: Apple M3 Pro, macOS ARM64, Go 1.27.0, Node 25.8.1. Embedded SQLite: **3.53.4**, pinned through modernc.org/sqlite v1.60.1; schema/application versions 1 / 0.1.0-dev.

The latest `make report` equivalent recorded **100 sequential warm local HTTP registry reads after 10 warmups**, on the isolated 118-flat fixture. Read p95 was **0.826 ms**. A 143,360-byte snapshot restored and verified locally in **25.536 ms**, preserving all fixture counts. The retained [JSON report](../reports/local/baseline.json) contains the samples, timestamp, engine settings and independent checksum evidence. Re-running the report changes these observations.

The representative database query benchmark was repeated three times: 248,261–257,403 ns/op, about 11.3 KB allocated and 209 allocations per operation. These are development measurements, not performance gates or production guarantees. Target hardware, concurrent workload, cold-start/mobile network behaviour and production recovery still need separate measurements.

## Visual review

The design uses locally packaged Instrument Serif/DM Sans, forest/ivory/citron tones, an original SVG neighbourhood illustration and distinct wing compositions. Status labels include text and sufficient measured foreground/background contrast. Focus states, native dialog behaviour, loading/empty/retry states and reduced motion are implemented.

Screenshots are local generated review artifacts:

- [Overview](../reports/local/overview-desktop.png)
- [Phone overview](../reports/local/overview-mobile.png)
- [Home registry](../reports/local/registry-desktop.png)
- [Home details](../reports/local/home-detail.png)

## Next acceptance slice

Implement identity, invitations/login, member/resource permissions and recovery with synthetic accounts. Prove cross-flat denial on detail/list/search paths; add privileged MFA and scoped registry administration. Then build transactional audit/jobs/private storage before implementing manual entries and immutable receipt PDFs.

Production host/state information, receipt examples, real registry approval, S3 encryption/off-site snapshots, two-custodian recovery and the annual-cost forecast remain pending for their dependent production decisions. The UI preview and local snapshot proof do not complete those gates.
