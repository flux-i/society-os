# Service requests — workflow and acceptance expectations

Completed local synthetic milestone, following section 16 of the platform plan. [Executed acceptance](complaints-baseline.md) records the gate, captured UI, native WebMCP and recovery results.

A current resident reports a case against an active home, then reads their own conversation and adds updates. Sharing a home never shares another person's case. After a membership ends, the author retains read-only personal case history; they cannot submit, comment, close or reopen under that ended relationship. Handler access is a separate current administrator/committee permission requiring MFA.

Handlers acknowledge and assign cases, adjust priority, explain waiting, record progress and resolve. Assignment accepts only currently active, verified users with a current handler role. Residents confirm closure after resolution or reopen with a reason. Handlers can also close or reopen with an audited reason. Closed cases must reopen before further work. There is no automatic inactivity closure.

| Current status | Allowed next statuses |
|---|---|
| Open | Acknowledged, In progress, Waiting |
| Acknowledged | In progress, Waiting |
| In progress | Waiting, Resolved |
| Waiting | In progress, Resolved |
| Resolved | In progress, Closed |
| Closed | Open |

Every mutation validates the current record version and an actor-bound retry identity. Repeating a successful request returns the same identity without another case/update; reusing the identity with changed content fails. The original description, number and history remain immutable. Financial entries and receipts are untouched.

Public conversation and staff-only notes are separate visibility classes. Resident details, history counts, search and native WebMCP results exclude staff-only notes. Resident versions, update times and list ordering follow public changes; private notes do not disclose their count through a version jump or require an author to reload merely because handlers wrote privately. Update identities are opaque. History is paginated after applying visibility; hidden notes cannot make empty resident pages or inflate public counts. Queries and response histories remain bounded. Attachments will join the later validated private-document workflow; none are published by this milestone.

Meaningful acceptance cases: unrelated and same-flat authors denied; foreign-home creation denied; handler role expiry and session revocation enforced; invalid transitions and stale races rejected atomically; eligible assignment checked on save; staff notes hidden in APIs/search/native browser tools; owner closure/reopening; ended-member read-only history; Unicode/long content validation; same-key create/action retries; snapshot/restore preserving updates and invalidating sessions; rendered menus/forms/dialogs/error recovery at desktop, tablet and small phones.

The society must supply the actual response policy and emergency/security contact. The interface directs urgent safety issues to direct contact; a saved portal case does not promise dispatch, notifications or a response time. No vendor account, automatic external messaging or runtime model API is introduced.
