# Changing overview — expectation brief

Accepted 4 October 2026 before implementation. This checkpoint replaces the static welcome/wing marketing overview with decisions, active cases, supplied financial records, approved notices and document deadlines. It implements the first overview slice in the [operations roadmap](society-operations-roadmap.md); future maintenance billing, campaigns, messaging and fines must supply actual data before their metrics appear.

## Outcomes and permissions

Every signed-in, MFA-cleared account gets an Overview. Reviewers see pending requests submitted by someone else separately from their own submissions awaiting a decision. A submitter sees their changes requested. Service handlers see active, urgent and unassigned cases; residents see only personally reported cases, including resolutions awaiting their closure. Staff-only notes must not influence resident-visible timestamps, ordering or counts.

Financial cards are present only with current explicit financial permission. Amounts derive from unreversed confirmed entries, exactly in paise. Positive balances are calculated per home before summing; one home's surplus cannot conceal another home's debit. Show supplied record balances and a labelled last-30-calendar-day collection period, never infer statutory dues or overdue status. Drafts appear as confirmation work only for treasury operators. Pending/failed receipt PDFs are actionable records, not unpaid money.

Notices follow approved state and current audience/membership. Contract/AMC deadlines follow the current approved document version and current library access. Document review counts require an available original and a separate eligible reviewer; invalid/unuploaded originals are the current uploader's work, not approval-ready. No file bytes, evidence, private conversation or reviewer notes belong in overview DTOs.

Each section has its own bounded read endpoint, server-side full counts and at most four preview rows, computed within one authenticated read transaction. The API rereads current roles, memberships, session and MFA. Source timestamps and calendar period are explicit. A failed/loading section shows unknown, not zero, and offers a local retry; other sections remain usable. No periodic background polling. Manual refresh and navigation reread current data.

## Controls and acceptance

Action rows open the exact permitted request, case, document or entry; full-list links remain available when previews are bounded. Back, refresh, close and direct deep links work, including denied/stale identities. Read-only native WebMCP can read the same section contracts and open Overview, including residents. It must reject unsupported section input, revoked sessions and unentitled finance, and preserve open human dialogs.

Keep the forest/ivory editorial design. The overview prioritises a compact today heading, changing metrics and an attention queue. Home/occupied/vacant/owner/tenant counts remain on Homes & people. Render empty, populated, loading, individual/combined source failure, permission-denied and refreshed states at desktop, tablet, 375px and 320px. Exercise every new link/button, keyboard focus, downstream dialog/menu controls and mobile navigation. Inspect captured screenshots; record actual executed cases, observed fixes and limitations without claiming unbuilt coverage.

Domain expectations include more than four qualifying records, self-review exclusion, a requester needing changes, current notice audiences, exact per-home balances with credits/reversals/drafts, boundary dates, archived/replaced documents, uploader versus reviewer work, hidden staff notes, revoked roles/memberships/sessions and pending MFA. The full release gate retains prior workflow checks. Capture pre/post preview snapshots and a restore rehearsal; no schema migration is expected for read-only aggregation.
