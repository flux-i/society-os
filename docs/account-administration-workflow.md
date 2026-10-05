# Account appointments and suspension — expectation brief

Recorded 5 October 2026 before implementation, following section 11 of the [platform plan](../housing-society-digital-platform-plan.md) and step 3 of the [operations roadmap](society-operations-roadmap.md). Verified locally as release 0.9/schema 8; see [acceptance evidence](account-administration-baseline.md). Checks use disposable synthetic data; production authority/custody acceptance remains separate.

## Outcome and permissions

A current administrator opens an account from Access & invitations, inspects its appointments and activity, grants or ends a bounded appointment, or suspends/resumes the identity with a verified reason. Residents, committee members, treasurers and auditors cannot administer accounts. Every identity mutation requires current administrator permission and password/applicable MFA confirmation within five minutes. The operator cannot change their own account or appointments through this workflow.

| Appointment | Powers added | Powers not added |
|---|---|---|
| Administrator | Registry/accounts; existing operational review, service handling and non-financial document administration | Financial posting; financial access requires its own existing entitlement |
| Committee | Existing community/registry and financial read scope, review, service handling and non-financial document administration | Accounts/role administration or financial posting |
| Treasurer | Financial reads, manual posting/corrections and accounting-document work; current registry read scope | Accounts/role administration and committee workflow powers |
| Accountant / auditor | Financial reads and permitted approved financial documents | Financial writes, registry-wide/person administration, operational review or service handling |

Appointments start on confirmation, expire after a supplied 1–365 days and remain in history after expiry/revocation. An approved-appointment/identity attestation and reason are required; the portal records this attestation rather than proving society authority. Duplicate overlapping appointments are rejected. Renewals are new appointments after the old one ends. Granting or ending an appointment signs the recipient out and invalidates outstanding personal links; confirmed MFA is preserved. No role is inferred from a spreadsheet, title, navigation item or home relationship. Residents retain their current independent home entitlements when a role ends.

## Safety and concurrency

At least one active, verified administrator with a confirmed authenticator and a current administrator appointment must remain after an administrator appointment ends or an administrator account is suspended. The last-administrator check, current actor scope, expected account version, grant/status change and immutable audit share the same write reservation/transaction. Concurrent edits produce one successful change and a reloadable conflict, with no duplicate grant or partial audit. Protecting against direct changes does not extend an expired term; the handover runbook must review expiry and independent custodial recovery.

Suspension revokes every target session, outstanding invitation/recovery link, temporary MFA setup and all unrevoked appointments, increments authentication/access versions, and preserves identity, confirmed MFA, financial records and historical audit. Suspended invitations remain visibly suspended. Resuming requires a fresh verified-identity attestation and reason, never revives sessions/links or old appointments, and preserves the password/confirmed MFA. An activated resident can sign in again under current memberships; an unactivated account needs a newly issued invitation after resumption. New appointments require separate explicit actions. Password recovery cannot bypass suspension.

Reads recheck current session, factor and administrator permission in the same snapshot as bounded account/grant/history data. Secrets, password/factor hashes and tokens never appear in metadata, history, logs or native tool output. Expected-version mutations cannot act on a different target or accept a supplied actor/derived permission. Existing seed appointments and historical migration checksums remain intact through the next migration.

Current identity checks also return an opaque access-scope fingerprint. It covers the active role and membership set, including relationship and financial entitlement when another home keeps an aggregate permission true. It is a cache boundary, never an authorisation credential. Changed scope clears previously loaded workspace data and open protected details; an unchanged check preserves forms. Native reads compare current scope before and after their read, rejecting responses captured before an entitlement ended. Credential freshness alone must not reset the workspace.

## Interface and acceptance

Preserve forest/ivory/clay colours, editorial headings, accessible shared selectors and native focus-managed dialogs. Give each account a clear Manage access action, accurate role labels, appointment expiry/revocation details and a bounded activity timeline. Appointment and suspension forms explain their actual effect before confirmation; show busy, denied, stale, empty, unavailable and retry states. Closing returns focus to the account opener. Internal scroll/actions and opened selectors must fit desktop, tablet, 375px and 320px widths, including short-height screens.

Before closure, verify meaningful backend expectations for exact role boundaries, pending-MFA and stale/revoked actor denial, expiry, duplicates, self-change denial, last-recoverable-administrator protection, concurrent writes, suspension/token/session revocation, explicit resumption, immutable audit and schema-7 upgrade preservation. Verify actual UI grant/end/suspend/resume, required acknowledgement/reason, opened menus/keyboard/focus, failures/reloads and account switching. Inspect captures. Actual native WebMCP must discover only entitled read tools, execute bounded metadata reads after visible mutations, preserve open forms, reject unsupported/cancelled input and deny stale/revoked/suspended access. It must expose no role/status mutation tool or private security/history content.

Complete the coherent domain/TypeScript/build/ordinary browser/native WebMCP gate, retained binary/assets, meaningful snapshot/restore and Windows cross-build. Target Windows, real custodians/authority and production infrastructure still require their own acceptance. Document executed evidence and actual findings; do not count fixture/assertion corrections as product defects.

After this checkpoint, continue approved maintenance cycles and explicit manual receipt allocations. The user’s fund, incident/fine, messaging and statement requirements remain preserved in the roadmap.
