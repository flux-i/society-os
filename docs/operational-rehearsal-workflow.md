# Household rehearsal — before-code understanding and checklist

Recorded 9 October 2026 while setup/import verification continues. Implement only after that checkpoint is accepted and published. The user's instruction to continue the complete plan autonomously authorizes this existing scope; real-data and production-policy acceptance remain separate.

## Intended outcome and assumptions

Prove that a newly configured society can use the portal through ordinary personal accounts, rather than relying on seeded demo permissions. Use a supplied 12-home fictional subset, verified invitations, real privileged MFA, separately appointed finance operators, independently checked manual records, different-person approvals, current household access and a retained restore. Follow the project working agreements and the approved forest/ivory controls. No payment initiation, automatic billing, external delivery or runtime LLM is introduced.

The subset comprises A-101 through A-304: 12 homes, eleven occupied, one vacant, twelve distinct current owners, four current tenants, seventeen people including the former tenant, and eighteen relationship rows including joint ownership. Stable source identities come from the independently supplied 118-home register. Registry import continues to grant no financial entitlement.

## Codebase reality and required decisions

`flat_memberships.can_view_finances` already scopes personal ledger, maintenance, collections, fine, document and export reads. `principalScope` includes that flag, so changing it can invalidate private screens and held native results. Current registry creation implicitly enables it for a new owner, while the initial import correctly leaves it disabled. There is no ordinary control for deliberately granting financial visibility to an imported household. Implement an explicit Treasury-owned visibility workflow; a registry officer's power to maintain people is insufficient to grant it.

`ReceiptSnapshot` and the PDF renderer preserve originals, but the renderer currently prints a fixed fictional/local-preview heading. Inspect the snapshot creation and rendering paths before implementation. New configured-workspace receipts must retain the supplied workspace identity and explicit fictional-rehearsal marker in their frozen source. Existing receipt snapshots and PDFs must keep their original identity; no retrospective relabelling or invented statutory fields is allowed.

Actual society identities, approved receipt/finance policy, custodian choices, production host/network and external credentials are unavailable. They block their dependent activation and representative acceptance, not this fictional development/rehearsal. Preserve the existing demo preview and every accepted archive.

## Verifiable implementation checklist

- [ ] Add a current, fresh Treasury-authorized household financial-visibility operation in the database/server — explicit enable/revoke, supplied verification reason, home version, immutable audit, transactional current membership checks and exact retry identity; registry-only staff cannot read or write the private visibility controls.
- [ ] Add the corresponding visible home workflow using shared controls — deliberate confirmation, clear household scope, no financial entitlement inferred from import, invitation or new membership, preserved pending/error/retry input, current-scope invalidation on revocation and no native mutation tool.
- [ ] Freeze workspace identity and rehearsal status into each newly issued receipt's immutable snapshot and render them — separately check PDF text and original/correction behaviour; old snapshots retain their legacy identity and format.
- [ ] Create the independently expected 12-home supplied fixture and establish personal registry, two separate Treasury, community reviewer, owner and tenant actors through bootstrap/invitation/appointment/MFA workflows. Do not patch permissions directly into the database to make the rehearsal pass.
- [ ] Rehearse an opening debit of ₹1,000.00, a supplied charge of ₹500.01 and money already received of ₹400.00 — drafts change no balance; confirmation gives exactly ₹1,100.01 due and one ₹400.00 receipt; repeated confirmation preserves one result. A linked reversal restores exactly ₹1,500.01 due while preserving that original receipt and its audit.
- [ ] Rehearse a separately approved expense proposal with no ledger/receipt effect, an audience-scoped notice, an approved document and a private resident report. Denied accounts cannot read private originals or another household's report; no approval itself pays or posts money.
- [ ] Exercise current owner/tenant access, explicit financial grant/revocation, ended membership and native reads against the same supplied register — no cross-home results, no import/approval/posting tools, no native navigation that discards an active human form.
- [ ] Verify a populated snapshot/restore with immutable receipt numbers, linked originals, supplied source identities, account/visibility audit, current access, separately held keys and no revived bearer sessions. Preserve unknown external handoffs for deliberate reconciliation.
- [ ] Run relevant backend formatting/vet/race, TypeScript/build, complete ordinary and actual native regressions, and a frozen-source rehearsal at desktop, tablet, 375 px, 320 px and 320×440. Open menus, inspect keyboard/focus/selection, scroll and dismiss dialogs, and directly review settled original screenshots and the generated receipt.
- [ ] Record actual findings, executed coverage and limits, retain matching recovery and publish the reviewed checkpoint through the personal flux-i repository; then continue with independent production adapters and operations preparation.

Items remain unchecked until their implementation, actual workflow and corresponding evidence pass. This rehearsal does not establish actual-host operation, external delivery, production finance policy or representatives' acceptance.
