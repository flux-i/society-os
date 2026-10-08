# Installation, offline state and safe updates — before-code expectations

Recorded 8 October 2026 during the loading checkpoint. Implement after that checkpoint is accepted and published. This brief records expected behavior, not completed functionality.

## User outcome and authority

Residents may install the same approved portal on supported browsers. The installed application keeps the forest/ivory interface and works normally online. An interrupted connection has a clear, accessible state; it never pretends a charge, received-money confirmation, approval or delivery succeeded. Reconnection uses the server's current identity, permissions and canonical results.

Installation and caching grant no new authority. Existing API authorization, fresh confirmation, immutable records and linked corrections remain unchanged. Native WebMCP must expose no retained private tools after logout and must obey the same offline/current-identity restrictions as the visible portal. No offline write queue, background financial submission, notification permission request or real external delivery belongs in this slice.

## Static cache boundary

Generate a manifest, appropriate existing-brand icons and a versioned public shell. Cache only explicitly allowlisted same-origin public build assets and the anonymous HTML shell. Never cache authenticated API responses, receipt/document downloads, uploads, signed destinations, private records, credentials or request bodies. A path beginning with a static prefix is insufficient: verify exact build membership, method, origin, query policy, successful response and content type. Cache only successful GET responses and do not intercept non-GET requests.

Precache the small initial shell and its actual static dependencies. Preserve the loading checkpoint's initial-byte budget: do not silently download every operational JavaScript chunk in the background. Cache visited allowlisted chunks as needed. Verify service-worker network traffic separately from page resource timing. Installation failure leaves the online application usable; unavailable or disabled service workers must not prevent ordinary use.

Use versioned caches with an application-specific prefix. Retain assets needed by current controlled clients during an update and remove obsolete application caches only when safe. Never remove another application's origin storage. No private payloads belong in localStorage, sessionStorage or IndexedDB as an offline convenience.

## Connection, logout and account changes

An offline reload displays an honest reconnect screen with no restored private user or records. During an already open session, label unavailable verification and disable mutations; keep any unsent human input in memory while the page remains open. Do not claim an in-flight operation failed solely because its response was lost. Existing uncertain-result handling and canonical reconciliation remain authoritative; reconnection must not automatically repeat the mutation.

Sign-out clears visible private state, open dialogs and native tools immediately, even without a connection. A nonsecret pending-sign-out marker may survive solely to ensure that reconnection revokes the old server session before protected UI or tools can return. Do not persist session cookies, CSRF values, user identities or payloads in that marker. Failure to revoke must remain visible and retryable while the protected UI stays cleared. Online sign-out must retain the existing server revocation behavior. Account switching, role changes and loss of membership must never restore cached private state.

Check actual request failure as well as browser online/offline signals. A successful authenticated round trip establishes current online access; a connectivity signal alone does not. Cached static assets may render the reconnect experience but cannot authorize reads or writes.

## Update behavior

New code may install and wait while a person works. Offer a deliberate update action with clear wording. Never automatically reload an open form, dialog, active submission or uncertain outcome. Conservatively block applying the update while work is open. Avoid forcing other tabs to reload: require an idle, unambiguous client situation or leave the update waiting with useful instructions. Closing all application tabs permits the browser's normal waiting-worker activation.

A failed update leaves the current application usable and provides an explicit retry. Applying a safe update preserves the current location and reloads current identity from the server. Current financial results, receipt identities and audit remain unchanged. Do not introduce periodic polling or bulk asset prefetch that reverses the preceding payload improvement.

## Independent checks and acceptance

Use disposable synthetic servers/databases and real browser service workers for these cases. Existing request-interception suites may explicitly block service workers to keep their network fixtures meaningful; dedicated ordinary and actual native WebMCP cases must enable and exercise the real worker, recording that difference.

Verify manifest/install help, exact cache inventory and actual worker network requests; no unvisited operational chunks; successful private reads/downloads leave no cached payload; offline first load/reload; open-form disconnection; lost-response reconciliation; immediate offline sign-out and reconnect revocation; account/scope changes; unavailable/failed registration; waiting, failed and deliberate updates; open-dialog and multiple-tab protection; old-asset retention and bounded cache cleanup. Use independent exact money/receipt expectations and count mutations to prove no automatic replay.

Render desktop, tablet, 375 px, 320 px and 320×440 layouts. Exercise actual menus, focus, keyboard, scrolling, dismissal, pending/error/retry controls and permitted/denied access. Capture and directly inspect final screenshots. Run backend formatting/vet/race, TypeScript/build, complete ordinary regressions and complete actual native WebMCP regressions against a frozen source/runtime, then matching recovery, preserved developer data/keys and personal publication. Record finite coverage and browser limitations; simulated desktop phone sizes do not establish physical Android/iOS installation or the supplied Windows computer's performance.

## Primary references and decisions

Service workers require a secure context (localhost is suitable for development), have an independent install/activate lifecycle and can cache public assets for an offline shell. These mechanisms do not determine our private-data policy; the stricter allowlist above is our product decision. [MDN service worker guide](https://developer.mozilla.org/en-US/docs/Web/API/Service_Worker_API/Using_Service_Workers).

A new worker normally waits while older clients remain. Forcing activation can mix application versions; automatic replacement is inappropriate for our pending financial forms. Use deliberate safe activation and retain the current client's assets. [Chrome service worker lifecycle](https://web.dev/articles/service-worker-lifecycle).
