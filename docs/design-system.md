# Society OS visual direction

The interface should feel like a considered place to belong: warm, editorial and quietly expressive. Use substantial breathing room and an architectural composition, with large Instrument Serif headings paired with clear DM Sans controls. The workspace remains recognizably an application, with visible navigation and short routes to real tasks.

Ivory surfaces, deep forest ink, pale citron highlights and soft clay accents carry the identity. Dark green creates one strong focal point on the overview. Keep borders subtle, use restrained shadows and avoid dense grids, corporate blue, decorative gradients and stacks of indistinguishable metric cards.

The neighbourhood illustration is original SVG, packaged locally. Fonts are packaged from pinned Fontsource dependencies; no external font requests or stock photography are required. Individual wing compositions and typographic scale extend the same identity across the registry and home detail sheet.

Controls must work with keyboard and touch. Use labelled inputs, 44-pixel minimum targets, visible focus, native modal focus handling, readable status text and clear loading, empty and error states. Honour reduced motion and preserve the layout at small phone widths and increased text sizes.

The UI/UX skill's generated community pattern informs warmth, prominent actions and a clear neighbourhood overview. Its default pink palette and onboarding marketing sections are intentionally replaced with this forest/ivory application design to match the user's requested direction. Only functioning registry features are active in the initial milestone; forthcoming workflows are labelled accordingly. Fictional data is visibly identified.

The approved overview now prioritizes homes, occupied/vacant homes and distinct active owners/tenants. Keep the home/people distinction explicit; a joint owner is a person, and a multi-home owner counts once society-wide. Owner-occupied/rented home totals appear as supporting context. Wing cards include both occupancy and people counts.

Sign-in extends the architectural illustration into a forest panel beside a quiet ivory form. Fictional account cards make differing permissions easy to explore. Home management uses the existing native modal, visible action choices, short labelled forms, inline confirmation and a readable change timeline. A resident sees their own homes; an officer sees management/history controls. All consequential access checks live on the server.

Access & invitations uses an airy account book, clear states and one primary invitation action. A native modal shows the personal link once. Security extends the ivory surfaces and serif headings into focused forms; authenticator setup and activation retain the illustrated welcome panel. Recovery codes stay visible until saving is acknowledged. Technical details belong only in optional setup-key instructions.

Viewport review is part of UI acceptance. Capture the visible screen and exercise its controls, including narrow desktops and reduced-height phone layouts. On desktop, preserve access to the sidebar profile and let navigation scroll independently. On phones and tablets, expose permitted routes through the labelled Menu. Modal headings and close controls remain available while the body scrolls; primary invitation actions stay in a separate footer. Lock background scrolling and restore the opening control's focus. See the [rendered UI review](ui-review-baseline.md) for regressions and evidence.

Dropdowns use [Radix Select](https://www.radix-ui.com/primitives/docs/components/select), with ivory menus, pale green selected rows, checkmarks, separate hover feedback and keyboard navigation. Search and dropdown triggers share a 48px height, restrained 1px borders and a soft focus treatment. Form dropdowns retain their height in column layouts and their native required-field behavior; validation focuses the visible trigger. Menus inside native dialogs are portalled into that dialog and checked for clipping. Review the menu while open, after selection, on hover and with keyboard input. A successful value change alone is insufficient visual acceptance.
