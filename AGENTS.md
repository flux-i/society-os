# Society OS working agreements

Build one complete, reviewable workflow at a time. Preserve the approved forest/ivory interface, editorial typography and accessible shared controls.

At each checkpoint:

- Define the user outcome, permissions and expected results before implementation.
- Run the relevant backend checks, TypeScript check and production build.
- Render the implemented workflow in a browser and exercise its actual controls. Open dropdown menus and inspect selection, hover, focus and keyboard behavior. Cover desktop, tablet and small-phone layouts, internal dialog scrolling and dismissal.
- Check meaningful empty, loading, error, retry and pending-action states, plus permitted and denied access. Use independent expectations for financial amounts and retry behavior.
- Capture and visually inspect screenshots. Keep screenshots, databases, downloads, logs, keys and other local artifacts outside Git.
- Use disposable synthetic databases for browser mutations. Preserve the developer's existing preview data.
- Fix observed problems and rerun the affected checks before declaring the checkpoint complete. Record the executed coverage and its limits; passing tests alone do not prove every possible interaction.

On the current macOS development machine, Playwright/Chromium launches require unrestricted or unsandboxed execution. Do not first retry a browser launch inside a command sandbox. Keep browser QA scoped to the current local target.

The current product records manually supplied charges/opening balances and money already received. It does not initiate payments. Finance access must be separate from registry administration. Monetary values must be exact paise; confirmed entries, receipts and audit are preserved, with linked corrections. Runtime LLMs, gateways, bank automation, automatic billing and Tally integration are future work only when requested.

Production use depends on separate infrastructure, identity/custody, real-data and finance-policy acceptance. Missing production inputs do not prevent independent local development with fictional data.
