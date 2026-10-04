# Account security acceptance · 4 October 2026

Release `0.3.0-dev`, schema 3 adds administrator-assisted invitations/password recovery, authenticator verification, single-use recovery codes and account-security screens in the approved forest/ivory style.

| Check | Result |
|---|---|
| Formatting, vet and race-enabled Go tests | 31 backend tests pass |
| TypeScript and frontend build | Pass |
| Isolated Chromium acceptance | 10 journeys pass, including real TOTP enrollment, activation/reset, code replay denial and 375px forms |
| Independent cryptographic expectations | Published RFC 6238 SHA-1 vectors pass; encryption rejects wrong key, user and altered ciphertext |
| Schema-2 upgrade | Registry/audit preserved; old sessions revoked; privileged login requires MFA |
| Snapshot recovery | Independent hashes/counts match; encrypted confirmed factors survive; sessions, access links, pending setup and recovery codes are absent |
| Key loss | Startup rejects missing/wrong keys with existing factors; no silent regeneration |

The fresh registry remains 118 flats, 154 people and 155 memberships: 109 occupied / 9 vacant homes, 118 distinct owners / 35 tenants.

## Identity boundaries

Administrators choose a person with a current home relationship, attest verified identity/email/relationship and record a note. They hand over the returned link manually through a trusted channel. The portal sends no email/message. Resident access follows current homes; committee/administrator invitations have a 1–365 day role term. Subsequent role/account administration and finance permissions remain separate tasks.

Invitation links expire in one hour and reset links in 30 minutes. Random 256-bit tokens are stored as hashes, bind the authentication version and are consumed once in a serialized transaction. Reissuing invalidates older links. An ended relationship blocks invitation activation. Passwords require at least 12 characters and at most 256 bytes.

Password reset/change revokes all target-account sessions and preserves confirmed MFA. Enrollment revokes other devices. Sensitive identity actions require password and applicable factor confirmation within five minutes. Password-only privileged sessions can use their authentication controls but cannot access registry/account data.

TOTP uses HMAC-SHA1, six digits and 30-second steps with one adjacent step of tolerance. Consumed timesteps cannot be reused. Five failed attempts within 15 minutes block further factor attempts across sessions. Ten random recovery codes are displayed once and stored as hashes; concurrent use has exactly one successful consumer. Replacement invalidates older codes.

QR codes use the bundled [node-qrcode library](https://github.com/soldair/node-qrcode) locally. Recipient links use URL fragments, then remove the bearer token from the address/history and submit it in same-origin POST bodies. Generated credentials are excluded from request logs and audit metadata.

## Local exploration and recovery

The four public fictional seeded accounts alone have an explicit **Use a preview code** helper on the loopback-only, synthetic preview. Setup uses the pending TOTP; subsequent verification uses a two-minute, session-bound recovery code. This intentionally bypasses independent factor possession for these fictional identities. Invited identities cannot use the helper.

Confirmed secrets use AES-256-GCM with user-bound authenticated data. The local key lives separately at `var/keys/mfa.key` (0700 parent, 0600 file), outside snapshot bundles. Startup and readiness check decryptability. Preserve the matching key for restored factors; independently recoverable encrypted off-site key custody remains pending.

There is no web MFA-disable/reset endpoint. Local `recover-mfa --demo` requires two distinct named custodians and a reason, audits their attestation, revokes sessions/links/codes and requires reenrollment. It records supplied custodian identities; real-world authority verification and production custody acceptance remain required.

Snapshot copies purge sessions, access links and recovery codes before hashing. Session deletion removes pending setup. This prevents point-in-time restoration from reviving previously used credentials. Confirmed factors remain encrypted and need the separately held key; live credentials are unaffected.

## Measured local baseline

Ignored `reports/local/account-security-baseline.json` records macOS ARM64, Go 1.27.0, Node 25.8.1 and SQLite 3.53.4. One sequential authenticated client made 100 warm reads after ten warmups.

| Measurement | Observed |
|---|---|
| Registry p50 / p95 / maximum | 0.819 / 0.936 / 1.273 ms |
| Password login | 67.052 ms |
| Enrollment using the synthetic helper | 3.093 ms; excludes human/app interaction |
| Snapshot size / fresh restore | 253,952 bytes / 26.247 ms |

These are development measurements, not an SLA or an established performance improvement. Historical schema-2 evidence remains in [registry/identity acceptance](registry-identity-baseline.md).

## Remaining work

The server remains loopback-only and synthetic-only. Real contact/identity evidence, subsequent role/account administration, post-recovery current-access reconciliation, HTTPS, encrypted off-site protection and a representative pilot remain pending. Operation identities/jobs/private storage precede entries/receipts and resident workflows in the [execution backlog](../execution-backlog.md).

The supplied performance/evaluation articles guide bounded journeys, meaningful allow/deny checks and recorded baselines. Implementation references: [RFC 6238](https://www.rfc-editor.org/rfc/rfc6238), [OWASP MFA](https://cheatsheetseries.owasp.org/cheatsheets/Multifactor_Authentication_Cheat_Sheet.html), [OWASP password recovery](https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html), [Go authenticated encryption](https://pkg.go.dev/crypto/cipher#NewGCM).
