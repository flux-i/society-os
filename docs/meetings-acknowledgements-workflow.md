# Meetings, minutes and personal acknowledgements — before-code expectations

Recorded 7 October 2026 while the reviewed service-contact/interruption checkpoint undergoes final acceptance. Implementation follows that checkpoint and its publication; this document defines the next complete community workflow in the [operations roadmap](society-operations-roadmap.md).

## The intended outcome

Community gives current residents a clear place to read a supplied meeting agenda, its scheduled time and location, a separately approved cancellation or published minutes. When the society deliberately asks for acknowledgement, a resident can acknowledge the exact currently published version. Acknowledgement records that personal portal action; it does not prove attendance, consent to a decision, a vote, quorum or a legal notice requirement.

Preserve the forest/ivory interface, editorial typography and shared accessible controls. Keep existing noticeboard and service links compatible. Present upcoming meetings and outstanding personal acknowledgements as changing attention, with exact deep links. Avoid a public list of residents who have not responded.

## Authority and audience

Current community operators prepare an agenda or linked revision. A different current community reviewer approves or declines the exact supplied version. Privileged writes require current authority and enrolled factors, recent confirmation, a bounded private reason, explicit confirmation, an expected current head and an actor-bound operation identity. Registry or community authority does not grant finance writes.

An area is all current homes, a supplied wing or an explicitly bounded selection of homes. Review the flat-only options and their exact hash before freezing those flat IDs. Publication does not silently expand when the registry changes. Every resident read and acknowledgement intersects that published audience with current household membership. Suspended accounts, ended membership and expired appointments lose their corresponding permissions immediately. Private proposal comments, reviewer reasons and acknowledgement identities remain in permitted operational views.

## Immutable agenda, minutes and cancellation

Keep an immutable proposal/version/event history and a current published head. Preparing or revising a private successor leaves the approved predecessor visible. Decline or own cancellation of a pending proposal preserves that predecessor. Only a different eligible reviewer's approval changes the published head.

An agenda supplies a title, plain-language agenda, location, explicit start and optional end. Store integer UTC instants in the supported range and display Asia/Kolkata during local development. End cannot precede start. Passage of time labels the scheduled meeting as past; it cannot manufacture minutes or prove that the meeting occurred.

Minutes are another separately reviewed version of the same meeting. Keep the approved original agenda, its times and frozen area. Require a supplied meeting-held instant and substantive plain-language minutes; reject a future held instant. Minutes do not automatically create entries, fines, charges, tasks or other enforceable records. Those require their own existing workflows and authority.

A cancellation is a supplied public update for separate review. Retain the original schedule and distinguish cancellation from minutes. An approved withdrawal hides new portal access and keeps history. Previously published content and historical acknowledgement evidence remain retained. A replacement, minutes or cancellation must be reviewed as new information; an old acknowledgement cannot be presented as acknowledgement of a successor.

## Exact personal acknowledgements

An operator deliberately chooses whether an agenda or minutes version asks for acknowledgement and may supply an explicit acknowledgement deadline. Show that deadline as supplied; do not infer one from a meeting date or turn a late response into a fine.

A current eligible resident sees their own current-version acknowledgement status and an explicit action after reading the exact publication. Recheck active identity, current home, published version and publication fingerprint inside the write transaction. Never accept an acknowledgement of a draft, hidden, withdrawn, superseded or unrelated publication. A linked pending proposal does not disable acknowledgement of its still-approved predecessor.

One personal acknowledgement belongs to one resident identity and published version. Another household member cannot acknowledge on their behalf. Actor-bound successful retries return the original action without duplicating it; a changed payload under the same operation identity conflicts. Retain the exact published version, current-home intersection at the action, actor and timestamp. A later membership change does not erase the original, but it ends access and removes that person from current expected-response counts. Show historical response totals separately from current response expectations.

Operators can inspect bounded current response status and retained originals under current community authority. Residents see only their own response status. Do not expose household phone/email, other people's responses or a private non-responder list in published content or native tools. Acknowledgement is a normal protected personal action with CSRF, current session and explicit confirmation; it does not grant publication authority or require a resident to obtain a privileged appointment.

## Overview and reminders

Use an independently retryable meeting source. For current residents it shows upcoming approved agendas within a bounded supplied-time window and their own current acknowledgements that remain outstanding. For operators it adds proposals awaiting their separate review and bounded response counts. Derive full counts independently of list pagination. Planned, past, cancelled, minutes and withdrawn states retain distinct meanings; an error leaves other Overview sources usable and displays an unknown value rather than zero.

Private acknowledgement reminders and delivery exceptions remain the next communication slice. This meeting checkpoint provides exact current response status and original links, and never dispatches on publication or acknowledgement. A later reminder must freeze an approved source and a deliberately reviewed audience, recheck current publication/version, membership, contact preference and response state before a new handoff, and reconcile an uncertain old handoff without duplicate sending. Keep that remaining work explicit until its own checkpoint passes.

## Recovery, browser and native acceptance

The additive migration preserves all 81 prior persistent tables, their exact rows and definitions, and migration provenance. Populated matching recovery preserves approved agenda, a pending successor, separately approved minutes/cancellation and exact personal acknowledgements. It invalidates temporary credentials and retains both separately held keys, original financial records and uncertain provider evidence. Browser mutations use disposable synthetic databases and never change the developer's preview.

Domain expectations cover current and stale authority, self-approval, draft privacy, frozen/current audience, exact time boundaries, pending predecessor, linked minutes/cancellation/withdrawal, personal acknowledgement isolation, superseded fingerprint conflicts, unchanged and changed retries, independent current versus historical response counts, and restored originals. Concurrent reviewers and concurrent identical acknowledgement requests must have one permitted durable outcome.

Rendered journeys must use actual controls for preparation, every menu, separate review, resident reading and acknowledgement, revisions, minutes, cancellation, retained history and denial. Inspect selected/hover/focus states, keyboard navigation and Escape, internal scrolling and busy/uncertain dismissal at desktop, tablet, 375/320 pixels and 320×440. Cover meaningful loading, empty, error, retry, conflict/reload and pending states. Capture and actually inspect the final viewport screenshots.

Native Chrome WebMCP exposes only bounded permitted meeting state, timing, area counts and the actor's own acknowledgement status. Omit agendas/minutes/private reasons, locations, resident identity lists and contact details. Native tools cannot acknowledge, prepare, approve, cancel, publish, navigate away from an active human form, or send. Exercise real discovery/execution, schemas, cancellation and held results after current membership, authority or session changes.

Accept only after formatting/vet/full race, TypeScript and production build, relevant and ordinary rendered browser checks, actual native WebMCP, final screenshot inspection and matching populated recovery. Record executed coverage and its limits. Society meeting policy, timezone, eligible acknowledgers and real identities remain supplied production inputs; this workflow does not infer statutory meeting, voting or quorum rules.

## Implementation decisions retained before code

Add a separate meeting resource/version/event model and immutable personal acknowledgement table. Reuse the existing flat-only area options and current community authority helpers without changing approved service/contact originals or their migration. A meeting has one published head and at most one pending successor. Area options, the selected current head and deliberately supplied content are reviewed before confirmation. Agenda revisions can deliberately change the area; minutes and cancellation retain their approved agenda's area, title, location and schedule.

The published-version fingerprint binds immutable content, frozen homes and the exact approval event. An acknowledgement names that version and fingerprint, includes explicit confirmation and uses a person identity plus version as its unique durable key. A second account for the same person cannot create a second acknowledgement. Actors cannot specify another person. A repeated action with a different operation key returns the person's existing exact acknowledgement; changed data under a previously accepted operation identity conflicts.

Check current active identity and household access before returning an accepted operation response. An already accepted exact retry may return its retained acknowledgement without writing again while current access to the meeting persists; it cannot acknowledge a successor. A first acknowledgement must match the currently published version/fingerprint and an explicitly requested acknowledgement. Pending replacements keep the approved predecessor actionable. Superseded versions, cancellation, withdrawal and ended household access cannot receive a new acknowledgement.

For the fictional local policy, eligible current responders are distinct people with an active portal account and a current membership intersecting the approved homes. Joint household members respond separately and a person in several approved homes counts once. Staff operational access alone does not allow personal acknowledgement without such membership. Show historical acknowledgements separately from the current eligible, acknowledged and outstanding counts; people without an activated account do not silently become expected portal responders. Production adoption must approve this eligibility policy.

The independent Overview source bounds upcoming agendas to the next 30 days, includes the actor's requested outstanding current-version acknowledgements regardless of meeting time and includes other operators' private proposals. Full counts are separate from at most four actionable rows. Coalesce a meeting that is both upcoming and awaiting the actor into one row while retaining separate count meanings. A current operator can use the published reading view or the private desk; resident metadata contains only their own acknowledgement status.

Native meeting tools return bounded IDs, state, published version/time, meeting times, home count and own acknowledgement-required/completed/deadline metadata. Current-source guards apply before and after held requests. No meeting location, agenda, minutes, respondent names, private proposal or action is exposed. The native navigation allowlist gains only deliberate human destinations while retaining the existing active-form protection.
