# Measured loading and navigation — before-code expectations

Recorded 8 October 2026 while the move/contact checkpoint completes acceptance. Implementation follows that checkpoint. This brief does not establish performance improvement or acceptance.

## User outcome and authority

Keep the approved interface and all current permissions while reducing the JavaScript required to reach sign-in and the changing overview. Load less frequently used operational screens when a person opens them. Show a clear, accessible loading state and a useful retry if a screen's code cannot be loaded. Preserve the workspace, navigation, current account checks and unsaved forms through ordinary screen use.

A loading improvement grants no new access. The server remains the authority for every read, write, file and replay. Changing roles, memberships, MFA state or account must still clear protected dialogs and rediscover the current native WebMCP tools. A slow or failed module must not render another person's retained data or disable sign-out. Deep links must reach the same authorised record after loading; native navigation opens the same visible screen and never submits a form.

## Baseline and independent expectations

Measure the frozen accepted build before editing application code. Record its exact source/runtime hashes, browser/OS, viewport, sample count, cache setting and fixture. Use the same synthetic registry, browser settings and user journeys for before/after comparisons. Record initial transferred/decoded JavaScript, production chunk bytes and gzip sizes, and observed sign-in/overview/navigation timings separately. Include cold and warm loads and first/repeated operational navigation. Use several independent samples; report distributions and variance instead of a single favourable observation.

The initial JavaScript required for sign-in must fall by at least 20 percent against the measured baseline; verify actual browser requests and build assets, not just a new filename. The changing overview must not preload the complete operational application. Direct entry to each available screen must load its required code and remain usable. Avoid a brittle timing pass/fail on this developer machine; keep correctness and byte/request boundaries as deterministic checks. Do not claim the macOS measurements establish performance on the supplied Windows/8GB computer.

Code splitting may share common controls and domain helpers. Do not duplicate React or silently remove functionality to make the number smaller. Keep the existing forest/ivory typography, responsive menus, dialog scrolling and focus behaviour. A failure boundary must recover from a transient module error on deliberate retry; an update requiring a reload must explain that choice and preserve the current workflow's saved results. No automatic refresh may discard a pending or uncertain write.

## Executed coverage required

Run current backend checks, TypeScript and production build. Exercise actual desktop, tablet and small-phone controls, opened menus, keyboard and dialog behaviour. Cover deep-link entry, slow module loading, unavailable module/retry, denied scope, logout/account switch and current native WebMCP discovery/navigation. Use disposable databases for mutations. Capture and directly inspect final screenshot originals.

Run the complete ordinary and actual native WebMCP regressions against one frozen build/source manifest, retaining meaningful independent financial and retry expectations. Record before/after measurements, fixed findings and finite coverage limits. Preserve the preceding accepted archive and keys, prove matching recovery for this release and publish through the personal public repository only after local acceptance.

Offline/install/update support is a separate following checkpoint. This optimisation must not introduce a service worker or cache authenticated API responses, documents, receipts or personal records. Runtime LLMs, payment initiation, automatic billing, bank/Tally integration and live external delivery remain outside this slice.

## Loading API decisions checked before implementation

React defers a lazy component until rendering and retains its loader promise; a rejection reaches an error boundary. Vite documents browser limitations on retrying failed dynamic imports. Use a stable route-level loader and an accessible error boundary. A deliberate reload of the current screen may recover a failed or replaced chunk; explain that action and preserve the location. Never automatically reload an active form or uncertain write. Keep navigation and sign-out outside the loading boundary. These design decisions are our application of the documented behaviour. [React lazy reference](https://react.dev/reference/react/lazy), [Vite dynamic-import failures](https://vite.dev/guide/troubleshooting#failed-to-fetch-dynamically-imported-module), [Vite load-error handling](https://vite.dev/guide/build#load-error-handling).
