# Emergency contacts and service interruptions — before-code expectations

Recorded 7 October 2026 while the portal-provider checkpoint is undergoing acceptance. Implementation follows that checkpoint. This is the next community slice in the [operations roadmap](society-operations-roadmap.md), with its own checks and recovery evidence before acceptance.

## Resident outcome

Open Community to find the society's deliberately published help contacts and current water, power, lift or other service interruptions. An interruption explains the affected homes, when it starts, the expected end if known, and the latest approved update. Overview gives current affected residents a clear link to an active interruption; a closed or future interruption does not look like an active emergency.

Contact records describe committee-supplied service contacts, such as a caretaker or maintenance service. A named resident's registered private communication details never become a public directory entry implicitly. Contact publication needs a supplied authority/permission attestation, useful availability information and a separate reviewer. Do not invent official emergency numbers, advice, service availability or response guarantees. Telephone links are deliberate human actions; tests inspect them without placing calls.

Retain the approved forest/ivory interface, editorial typography and shared accessible controls. Community remains the familiar entry point. Use a calm, readable interruption card with service, timing and affected area, followed by quick contact actions; avoid an allegation feed or permanent overview counters for rarely changing data.

## Permissions and deliberate publication

Current community/registry operators can prepare a contact or interruption proposal and edit their own pending proposal. A different current eligible community reviewer approves or declines the exact version. Fresh privileged authority, reason, confirmation, expected version and actor-bound operation identity are required for protected writes. Registry authority grants no finance write. Ordinary residents cannot publish society-wide contacts or interruptions.

Residents read only published current information affecting their present homes. Committee operators can see the retained private proposals and history needed for review. Ended household membership, suspended accounts and expired appointments are rechecked on every read and inside mutations. Do not expose proposer/reviewer private comments or unrelated household/person/contact metadata in resident views or native tools.

Area choices are all current homes, one wing or a bounded set of homes, using flat-only choices. A published area snapshot retains the approved flat identifiers; the resident's current membership determines access. A proposal cannot name arbitrary people or silently switch to an expanded area. Revisions and changed scope require another independent review. Rejected or withdrawn content remains in history.

For urgent use, the operator can prepare a short proposal with the information currently known and obtain the usual separate approval. Do not add a bypass that makes the proposer their own approver. The society must supply its real operational escalation and out-of-hours review policy at production acceptance.

## Contact lifecycle

Prepare a labelled service contact with supplied telephone number, optional availability text, affected area and verification/permission attestation. Reviewers see the exact submitted text and intended visibility. Publication makes that version available to its current audience. A replacement preserves its predecessor; an old published contact is superseded only when the new version is approved. Withdrawal ends new portal access and retains the original record and decision.

Validation bounds every text field, rejects control characters and malformed phone numbers, and constructs telephone links from canonical validated values. No imports of private resident contacts, arbitrary external URLs, implicit WhatsApp sends or unreviewed contact substitutions are part of this workflow.

## Interruption lifecycle and time meaning

Prepare service type, title, plain-language description, supplied start time, optional estimated end and affected area. Store unambiguous instants and display them in the society's configured timezone; current local development uses Asia/Kolkata. Reject an estimated end before the supplied start. A planned interruption is visibly planned until its start; an elapsed estimate alone never proves the service is restored.

Approval publishes the exact original proposal. A later update or resolution is another reasoned, versioned proposal for separate review. Keep the currently published original visible while its replacement is pending. Approved resolution ends the active attention item and retains what was communicated. Withdrawal hides a published notice with an accountable history, rather than erasing it or pretending the service was restored.

An elapsed estimated end is labelled as an estimate that needs an update. Counts use the actual current scope and lifecycle, independently of pagination. Overview shows currently published interruptions that have started and remain unresolved, with bounded rows and exact deep links; future planned information remains available in Community. Real-world service restoration is supplied by people, not inferred from the clock or a provider callback.

## Messaging and other existing workflows

Published interruption text is available in the portal immediately. Reuse separately reviewed messaging only when an exact approved source and its audience can be retained safely. Do not auto-send when a contact or interruption is approved. If a message source changes or is withdrawn, a new handoff must be blocked or explicitly skipped under the existing frozen-source contract. A received message does not prove a service is restored.

Resident service complaints and internal upkeep work remain their existing workflows. An operator may link to a permitted original when supported, but cannot leak private handler notes or resident complaint evidence through an interruption. This slice does not post charges, create fines, initiate payments or manufacture financial records.

## Retained history and recovery

Use an additive schema migration. Preserve every prior persistent table's rows, column shape and migration provenance. Retain immutable original/version/publication/decision events and actor-bound operation responses; linked revisions cannot replace old evidence. A repeated successful operation returns the original permitted result, while changed details under the same operation identity conflict.

Restart and matching snapshot restore preserve the current approved contact/interruption, any pending replacement, approved resolution and exact area. Restore invalidates old sessions. Recheck current authority and household membership when recovered content is read. Synthetic browser mutations and uploaded artifacts stay outside Git and never alter the developer preview during QA.

## Independent acceptance

Backend expectations cover current permission and fresh-factor denial, self-approval, pending/draft privacy, cross-home denial, exact audience intersections, unsorted selection canonicalisation, stale competing reviews, successful/changed retries, original preservation during replacement, withdrawal, scheduled/started/overdue-estimate/resolved time boundaries and populated restore. No provider or finance change may occur from a read, publication or native call.

Rendered journeys cover operator preparation, all actual area/service menus and phone validation, resident published views and contact actions, separate approval, replacement/update/resolution, private history, held writes and dismissal, lost responses, conflicts/reload, and loading/empty/error/retry states. Inspect opened selection, hover, focus and Escape/keyboard behaviour at desktop, tablet, 375/320 pixels and 320×440. Scroll internally to review and submit, capture the actual viewport and inspect the final screenshots after finite entrance animations settle.

Actual native Chrome WebMCP may expose only bounded permitted service/contact status metadata. It cannot retrieve private phone numbers/comments, create proposals, approve, resolve, send or place calls. It must preserve an existing human form and discard held results after current audience, appointment or session changes. Update every exact metadata allowlist deliberately and keep private-data/no-write assertions.

Formatting/vet/race, TypeScript, production build, relevant and ordinary browser regression, actual native WebMCP, screenshot review and matching populated recovery must pass before this slice is accepted or promoted. Document executed coverage and its finite limits. Real service contacts, operating hours, society timezone/state, custody and escalation policy remain supplied production inputs.

## Implementation decisions retained before code

Community will keep its existing noticeboard and add service-interruption/help-contact sections, with an operational review desk for current community operators. Original notice links remain compatible. A contact action is a deliberate telephone link built from a validated canonical number; local browser checks intercept that action before operating-system handoff and independently inspect the exact link. No test places a call.

Use immutable resource versions and decision events with a small current-head record. A revision appends a new version; it never edits retained original content. Pending replacement, decline or withdrawal of a proposal leaves the approved predecessor visible. Only approval by a different current reviewer changes the published head. An approved contact withdrawal or interruption resolution remains retained but leaves current attention. Public reads return only the deliberately published version/time; private edits cannot change a resident's visible version, timestamps, search results or counts.

Versioned actions require fresh community authority, an expected current head, a bounded reason and an actor-bound operation identity. Validate and canonicalise explicit home selections before hashing retries. Check current authority before returning an accepted retry, then return that original operation identity without appending another version/event. Competing stale decisions cannot publish different successors. A pending revision belongs to its proposer; another operator can propose a replacement only after the previous pending action has ended.

Freeze all-home/wing/selected-home audiences as flat IDs at proposal time. Current resident membership intersects those approved IDs on every read. A home created later does not silently expand an old publication. Options contain flat labels and wing names only, with no person/phone inventory. Published contact numbers come only from the separately reviewed supplied number and permission attestation; no registry contact is copied implicitly.

The current area options carry a hash of their exact flat IDs/labels. The first proposal checks that reviewed option hash before freezing its area; changed homes or wing labels require reloading and reviewing the area. Accepted retries check current authority first and retain their original hash and audience even if the registry later changes.

Interruption times are explicit integer UTC instants with a bounded supported range. The form accepts local date/time in Asia/Kolkata and shows that timezone during review. An absent estimated end is unknown. Started-but-unresolved interruptions remain active after their estimate passes. Resolution requires a supplied restoration time and a separate approval; an automatic clock transition cannot restore service. Withdrawal remains distinct from restoration.

Expose bounded list/detail/options and deliberate proposal/decision endpoints. Add one independently retriable Overview source for current started unresolved interruptions and review decisions relevant to the actor; future planned and resolved items remain outside active-disruption counts. Original deep links reopen the exact currently permitted published resource. Native tools return permitted status/timing/area metadata only, without telephone numbers, bodies, private reasons, proposer identity or writes.

The matching migration/recovery rehearsal must preserve all 78 schema-19 persistent tables, column shapes and provenance, plus the newly published resources, pending replacement and explicit resolution in a separate populated fixture. Keep both held keys and unknown provider attempts intact. This checkpoint does not require or activate an external provider.
