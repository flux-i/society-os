# Upkeep, assets and vendor visits — expectation brief

Recorded 5 October 2026 before implementation, as the remaining operational portion of step 4 in the [society operations roadmap](society-operations-roadmap.md). Maintenance cycles/allocations were accepted in 0.10. This expectation brief preceded upkeep code; subsequent 0.11 acceptance is recorded separately in the [upkeep baseline](upkeep-baseline.md). All development data and provider interactions remain fictional.

## User outcome and authority

Keep planned service work, accountable assignments, vendor visits and asset deadlines together. Existing current Administrator/Committee operational authority manages this workspace; a Treasurer or Auditor appointment alone supplies no operational power. Financial postings and external messages remain separate workflows. Every read/write checks the current session, applicable factor and appointment inside the database transaction. Residents see only explicitly published resident-relevant work for their current audience; internal work, vendor contacts, contracts, private comments and handler history stay operational.

An operator maintains a private vendor directory and asset register, recording explicitly supplied contact details, location, contract/source reference, AMC start/end and next inspection date. Dates and relationships are reviewed before saving. Inactivation preserves previous records and history. A retired asset or inactive vendor cannot be selected for new work; historical links remain readable to current operators. Upcoming/expired dates are reminders based on those supplied dates, never a claim that an inspection or contract renewal happened.

A work item records a title, description, category, priority, due/visit date, optional active asset/vendor and an optional current eligible operational assignee. It may link an existing service case for operators without exposing that case or its reporter to residents. Assignment checks current eligibility at confirmation. A published interruption or task has an explicit resident-facing description and optional wing audience; publication does not expose vendor contacts, internal notes, private attachments, financial estimates or its linked complaint.

## Work and checking

Use an explicit workflow: Planned → In progress → Waiting / Ready for check → Done. Waiting can return to In progress. Ready for check records the submitting actor and evidence/comment; a different current operator confirms completion or returns it to work with a reason. The submitting actor cannot confirm their own completion. Cancellation and reopening retain their reasons, old states and actor history. A public update is intentional and separate from an internal work log; private changes cannot leak their text, version or activity count to the resident view.

Due dates and scheduled visits are independently supplied India calendar dates. Future work is upcoming, never overdue. Work done/cancelled no longer appears as an open operator obligation. Resident updates are intentional publication snapshots: title, public body, due date and status are frozen at publication, labelled with their publication date, and refreshed only by an explicit reviewed public update. Internal edits/checks never silently revise that snapshot. Unpublication removes it from current resident views immediately. Recurring servicing starts as an explicit repeat interval on the work record; the operator previews and confirms a new occurrence with its own date and assignment. No scheduler silently creates work, renews an AMC or shifts an inspection deadline after completion. Completion history is retained and a replacement occurrence is linked to its source.

Writes use actor-bound stable operation identities and expected versions. Lost responses can be retried unchanged without another task, event or completion. Changed retries, stale versions, ended appointments, removed factors and concurrent transitions fail with a useful reload path. Database guards preserve identity and append-only history. The overview displays scoped open/unassigned/overdue work, scheduled obligations and private asset deadlines with exact links; unavailable sources show unknown values and local retries.

## Independent checks

- A task due tomorrow has zero overdue count; an open task due yesterday counts once. Confirmed completion removes it from operator open/overdue counts while retaining the record. An explicit new public update removes a completed item from resident open counts; internal comments leave resident counts unchanged.
- Operator A submits work for checking. A cannot confirm it; currently eligible Operator B can. B returning it requires a reason and keeps the original check submission in history.
- Ending an assignee's appointment prevents new assignments and operational reads/writes; existing work shows the historical assignment and offers reassignment to a current eligible operator.
- An inactive vendor/retired asset is excluded from new-work choices but its existing linked work remains in operator history.
- A resident with an active Wing A home sees intentionally published Wing A/all-resident interruptions, never Wing B/internal work or contacts. Ending the last current eligible relationship removes that publication.
- A private note and its private update/version remain absent from resident HTTP/native reads, totals and activity. A separately supplied public update is visible only to its intended current audience.
- A repeat preview chooses a supplied new date and confirms exactly one new linked occurrence. The original task, completion and deadlines remain unchanged.
- Two stale concurrent transitions cannot both advance the same version; unchanged response-loss retries preserve the one resulting identity/event.
- Supplied AMC/inspection dates produce independent expected upcoming/overdue reminders. No reminder creates a charge, receipt, fine or message.

## Interface, evidence and acceptance

Preserve the approved forest/ivory, editorial typography and shared accessible selectors/dialogs. Use a clear work queue and a separate asset/vendor register, with private operational controls and a useful read-only resident view. Render actual creation, assignment, vendor/asset selection, work logging, separate completion check, cancel/reopen, publication and repeat controls. Check opened menus, selected/hovered/focused options, keyboard/dismissal, pending-action protection, scrolling and 1440px/768px/375px/320px layouts. Exercise empty/loading/error/retry/denied/stale states and inspect retained captures.

Add bounded read-only actual native WebMCP metadata with current audience checks; it must not publish, assign, complete or transmit anything. Run meaningful domain/HTTP checks, the coherent race/format/vet/TypeScript/build/rendered/native gate and recovery/schema-upgrade evidence. Keep the last accepted preview pinned during development. Publish only after the complete checkpoint passes, then proceed to fund campaigns and externally paid reports; upkeep acceptance alone does not finish the overall goal.
