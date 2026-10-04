# Rendered UI review — 4 October 2026

The portal was reviewed in Chromium using real sign-in, navigation, form and dialog interactions. Screenshots captured the visible viewport; page-wide captures alone had missed controls below the fold. All writes in browser checks use a fresh fictional database on an ephemeral loopback port.

| Observed issue | Resulting behavior |
|---|---|
| At 1280×720, the sidebar profile started at y=811, below the screen | The profile stays visible; navigation can scroll independently on shorter screens |
| Phone navigation clipped Account security | A labelled Menu exposes every permitted route with readable labels, keyboard dismissal and focus handling |
| After verification, the dashboard inherited the welcome screen's scroll position | Each authentication screen transition starts at the top; recovery-code enrollment also starts at the top |
| Long invitation and home forms hid their actions below the dialog viewport | Dialogs scroll internally; close controls, invitation actions and home form actions remain available |
| Native dialogs allowed background scrolling and treated interior blank clicks as dismissal | Background scrolling is locked; dismissal checks pointer origin and actual backdrop bounds; closing restores focus |
| Both current-password inputs used the same state | Identity confirmation and password change now have independent inputs |
| A changed person search retained a previous selected identity | Changing the search clears selection and verification; selection changes require verification again |
| Every invitation error offered Account security | That route appears for the actual reauthentication error |
| Replacing recovery codes said “Continue to workspace” while staying on the security page | The replacement flow says “Done, codes saved”; enrollment keeps its workspace action |
| Narrow desktop navigation compressed an icon to fit its label | Icons retain their width and navigation spacing fits the smaller sidebar |

The forest/ivory palette, Instrument Serif headings, original illustration and owner/tenant/occupancy information are preserved. The phone welcome panel is shorter, and metric labels keep their icons on the same line.

## Validation

The initial pass passed **14 browser checks**: ten established registry/identity journeys and four viewport regressions. That pass did not inspect open dropdown menus or inventory every control. The user's screenshots exposed that gap and prompted the expanded review below.

The review covers 1280×720 desktop, 1024×768 narrow desktop, 768×1024 tablet, 375×812 phone, 320×568 small phone and 375×500 reduced-height layouts. Assertions cover fully visible controls, navigation and focus, password independence, selection clearing, background scroll locking, interior clicks and viewport overflow. Reduced-height emulation checks layout resilience; it is not a physical-device keyboard test.

To reproduce the four UI checks and capture screenshots:

```sh
cd web
SOCIETY_CAPTURE_UI=1 npm run test:browser -- ui-review.spec.ts
```

Screenshots are generated under `reports/local/ui-review/after/`; the original captures are retained under `before/` on this machine. These local artifacts are ignored by Git and stored with private file permissions. Follow the global `AGENTS.md` browser-launch instruction: use unrestricted execution or an appropriate unsandboxed browser command. The current workspace uses unrestricted execution.

## Expanded interaction and dropdown review

The final TypeScript check, production build and complete **26-test Chromium suite pass**. The final full run took 1.1 minutes. Dropdown checks open the actual menu and click visible options; they no longer use Playwright's native `selectOption` shortcut.

| Further finding | Corrected behavior |
|---|---|
| Native grey/blue menus, uneven filter controls, heavy focus rings and doubled wing borders | All nine dropdown controls use the same forest/ivory menu. Filters and form triggers are 48px high, with consistent borders and subtle visible focus. A checkmark identifies selected options and wings; hover remains distinct |
| Overview's “See all homes” retained the last explored wing | Leaving the registry clears the one-time wing intent; the general homes routes show all permitted homes |
| Closing About or an invitation through an internal completion button lost opener focus | Dialog cleanup closes the native modal before restoring focus, so the underlying control is no longer inert |
| Tab cycling in About could move focus out of the dialog | Tab and Shift+Tab wrap between visible enabled controls |
| Failed account loading provided no retry action | The account error state offers a working “Try again” button |
| A person-lookup error remained after changing the search successfully | Lookup feedback has its own state and clears when the search changes |
| Security feedback appeared above a long page, away from the submitted form | Confirmation and errors appear in the relevant security card, including visible phone password-mismatch feedback |
| Voluntary authenticator enrollment said “Continue to workspace” while returning to security | Embedded enrollment says “Return to account security”; security transitions return to the top |
| Pagination could label the requested page while still displaying the previous response | The displayed page number follows the returned page data |

Replacing native form controls also required regression checks for native required-field validation, long lists, blank changes from the native form bridge, menu placement inside native dialogs, and preventing flex layouts from shrinking phone form triggers. These checks pass in the final implementation.

| Control/workflow group | Executed coverage |
|---|---|
| Welcome/sign-in | All four account choices, password show/hide, help open/close, invalid email, wrong password, pending sign-in and disabled account choices |
| Workspace | Brand, permitted navigation, mobile Menu, Escape, skip link, browser back/forward, profile/footer About, its completion button, Tab cycling, backdrop dismissal, sign-out and account switching |
| Overview/registry | All three wing routes and toggles, all filter options, combined wing/occupancy selections, clear and empty results, searches, failed loads and retry |
| Homes/pagination | Every one of the 118 home cards opened across ten pages, matching detail titles and restored focus; previous/next and first/last disabled states |
| Dropdowns | Wing, occupancy filter, home occupancy, person record, existing person, relationship, relationship to end, invitation person and invitation access; selected/hover states, click, outside dismissal, Escape, keyboard arrows/Home/End/typeahead, required selection and a scrolling 30-person menu |
| Home administration | Occupancy change/restore, existing-person reuse, creation of owner/family/authorized-occupant relationships, tenant relationship, primary contact, ending relationships, history, stale-save reload, detail/history retry |
| Accounts/invitations | Account search/empty/error/retry, pagination, all access roles and term input, changed-person verification reset, validation/reauthentication errors, clipboard success/fallback, selected link text, completion, pending invitation reissue and old-link rejection |
| Activation/recovery | Invitation activation, password reset, used-link rejection, return/continue to sign-in, revoked resident sessions and scoped homes |
| Security | Required and voluntary enrollment, setup key expand/collapse, failed setup/restart, real TOTP and preview verification, authenticator/recovery toggle, downloaded code contents, saved-code acknowledgement, replacement codes, single-use rejection, identity confirmation, password mismatch/change/sign-in/restore |

The desktop filter comparison uses **2048×1119**. Open menus are checked at 1280×720, 1024×768, 768×1024, 375×812 and 320×568; each of the seven form dropdowns is inspected inside its dialog at desktop and phone sizes. Earlier viewport checks also cover 375×500. Geometry assertions include matching filter heights/borders/radii, fully visible menus, no horizontal page overflow, and hit testing inside dialog menus to detect clipping.

This is a documented Chromium review of the implemented local workflows and the specified states. It does not establish every possible input, timing, browser or physical-device interaction. At this schema-3 checkpoint, Entries/Receipts/Community/Documents were upcoming disabled controls. Schema 4 adds Entries/Receipts; its separate coverage is recorded in [manual-record acceptance](manual-records-baseline.md).

To reproduce the complete suite with screenshots:

```sh
cd web
npm run build
SOCIETY_CAPTURE_UI=1 npm run test:browser
```

Expanded screenshots are stored privately in `reports/local/interaction-review/`. Key comparisons: [aligned desktop filters](../reports/local/interaction-review/filters-desktop-aligned.png), [selected Wing B and hovered Wing C](../reports/local/interaction-review/filters-desktop-wing-menu-hover.png), [small-phone occupancy menu](../reports/local/interaction-review/filters-320x568-occupancy-open.png), [phone invitation access menu](../reports/local/interaction-review/form-375-access-open.png), [phone relationship menu](../reports/local/interaction-review/form-375-relationship-to-end-open.png) and [visible password error](../reports/local/interaction-review/password-mismatch-phone.png).

## Global browser tooling

Official [Chrome DevTools MCP](https://github.com/ChromeDevTools/chrome-devtools-mcp) **1.10.1** is installed globally and configured as `chrome-devtools` in the user's Codex configuration and Claude Code user configuration. Existing clients need a restart or a new session to load the configuration. Installation notes and verification are stored in `~/.local/share/ai-browser-tools/`.

Verification connected to the server, discovered **32 tools**, launched Chrome 154, rendered the local portal and saved a screenshot. A temporary, read-only WebMCP test tool was registered, discovered, successfully executed and removed. Both clients use the same executable and flags. The browser uses a temporary isolated profile, with MCP usage statistics and CrUX URL submissions disabled.

[WebMCP](https://developer.chrome.com/docs/ai/webmcp) is a browser API. Current Chrome exposes its imperative API through [`document.modelContext`](https://developer.chrome.com/docs/ai/webmcp/imperative-api). WebMCP support is enabled in the automation browser, and DevTools MCP exposes discovery/execution tools. The portal does not yet publish application actions through WebMCP; ordinary browser clicks, rendering and screenshots work independently. No Claude API key or runtime model integration is needed for this tooling.
