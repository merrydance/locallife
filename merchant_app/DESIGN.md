# Design System Strategy: The Culinary Conductor

> Note: This file remains a creative direction source for merchant_app.
> The canonical engineering standard for implementation and review now lives at `.github/standards/flutter/FLUTTER_UI_DESIGN_STANDARDS.md`.

## 1. Overview & Creative North Star: "Precision Vitality"
In the high-velocity environment of a commercial kitchen, a merchant interface cannot afford to be a static spreadsheet. This design system is built upon the North Star of **Precision Vitality**. We reject the "flat and boxy" aesthetic of legacy POS systems in favor of an editorial, high-end experience that feels alive yet authoritative.

The system breaks the "template" look through **intentional asymmetry** and **tonal depth**. By utilizing a sophisticated scale of greens and high-impact status colors, we guide the merchant’s eye toward what matters most. We treat every order not as a row in a database, but as a prioritized "event" on a curated stage. 

Through the use of **Plus Jakarta Sans** for headlines and **Work Sans** for data-heavy body text, we balance a high-end editorial feel with industrial-grade legibility.

---

## 2. Colors: Tonal Logic vs. Structural Lines
Our palette is rooted in freshness (`primary`) and urgency (`secondary`, `tertiary`). However, the sophistication lies in the neutral space between them.

### The "No-Line" Rule
To achieve a premium, custom feel, **1px solid borders are strictly prohibited for sectioning.** Boundaries must be defined solely through background color shifts or tonal transitions. 
*   **Implementation:** Place a `surface-container-low` section on a `surface` background to define a sidebar. Use `surface-container-lowest` for cards to create a natural "lift" without a single line being drawn.

### Surface Hierarchy & Nesting
Treat the UI as a series of physical layers—like stacked sheets of fine, heavy-weight paper.
*   **Level 0 (Foundation):** `surface` (#f5fbf1) — The base canvas.
*   **Level 1 (Sections):** `surface-container-low` (#eff5ec) — Grouping areas.
*   **Level 2 (Active Cards):** `surface-container-lowest` (#ffffff) — High-priority interactive elements.
*   **Level 3 (Pop-overs/Modals):** `surface-bright` (#f5fbf1) — Floating over the UI.

### The "Glass & Gradient" Rule
Standard flat colors feel "out-of-the-box." To elevate the merchant experience:
*   **Hero CTAs:** Use a subtle linear gradient from `primary` (#006b31) to `primary-container` (#038740) at a 135-degree angle. This adds "soul" and a sense of physical depth.
*   **Floating Navigation:** Apply `backdrop-blur` (12px-20px) to semi-transparent versions of `surface` to create a "frosted glass" effect, ensuring the layout feels integrated and airy.

---

## 3. Typography: The Hierarchy of Urgency
We use typography as a directional tool. In a kitchen, size equals priority.

*   **The Hero (Pick-up Codes):** Use `display-lg` (Plus Jakarta Sans, 3.5rem) for order codes. It should be the most "aggressive" element on the screen.
*   **The Headline (Merchant Status):** `headline-md` (Plus Jakarta Sans, 1.75rem) provides an authoritative, editorial feel for section headers like "Incoming Orders."
*   **The Utility (Order Details):** `body-lg` and `body-md` (Work Sans) are utilized for item lists. The slight mechanical feel of Work Sans ensures high legibility under harsh kitchen lighting.
*   **The Labels:** `label-md` is reserved for metadata (e.g., "Ordered 2m ago"). Use `on-surface-variant` (#3e4a3f) to keep these secondary.

---

## 4. Elevation & Depth: Tonal Layering
Traditional drop shadows are often messy. We use **Ambient Shadows** and **Tonal Layering** to create a clean, high-end architecture.

*   **The Layering Principle:** Instead of shadows, stack containers. An "Accepted Order" card (`surface-container-lowest`) sitting on a "Pending" list (`surface-container-low`) creates immediate depth.
*   **Ambient Shadows:** If a "floating" action button is required, the shadow must be extra-diffused. 
    *   *Shadow Specs:* Blur: 32px, Y-offset: 8px, Color: `on-surface` at 6% opacity.
*   **The "Ghost Border" Fallback:** If accessibility requires a border, use the `outline-variant` (#bdcabb) at **15% opacity**. Never use 100% opaque borders.

---

## 5. Components

### Order Cards (The Core Component)
*   **Layout:** Forbid divider lines. Use `surface-container-lowest` (#ffffff) with a `xl` (1.5rem) corner radius.
*   **Spacing:** Use generous padding (24px) to allow the "Go" green (`primary`) to breathe.
*   **Interaction:** The entire card should have a subtle hover state transition to `surface-bright`.

### High-Impact Buttons
*   **Primary (Accept):** Large-scale, using the `primary` to `primary-container` gradient. Use `xl` roundedness for a "pill" look that feels friendly yet professional.
*   **Tertiary (Decline/Urgent):** Use `tertiary` (#b6171e) on a `tertiary-container` background for a high-contrast, "urgent" signal that doesn't feel "broken."

### Pick-up Code Chips
*   **Visual Style:** High-contrast `secondary-container` (#fc820c) with `on-secondary-container` text. 
*   **Radius:** `md` (0.75rem) to differentiate from the rounder "Action" buttons.

### Status Indicators
*   **Live Stream:** Use a pulsing `primary-fixed` dot to indicate the app is live. This "micro-animation" provides reassurance in a fast-paced environment.

---

## 6. Do’s and Don’ts

### Do:
*   **Do** use white space as a structural element. If elements feel too close, increase the container gap rather than adding a line.
*   **Do** use `primary-fixed-dim` for "Accepted" states to show completion without the visual "noise" of a vibrant green.
*   **Do** ensure all tap targets for "Accept/Decline" are at least 64px in height for grease-covered or gloved hands.

### Don’t:
*   **Don’t** use pure black (#000000). Always use `on-surface` (#171d17) to maintain the organic, premium green-toned aesthetic.
*   **Don’t** use standard Material Design "elevated" shadows. They feel like a template. Stick to tonal shifts.
*   **Don’t** use "Success Green" icons. Our brand `primary` is our success color. Keep the palette disciplined.