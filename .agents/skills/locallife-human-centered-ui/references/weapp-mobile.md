# Weapp Mobile

Use for WeChat Mini Program and touch-first mobile flows, including consumer, merchant, rider, operator, and platform mini-program surfaces.

## User Model

Mobile users are interruption-prone, network-sensitive, and touch-first. They expect obvious primary actions, preserved context after back/re-entry, short forms, and recoverable failures.

## Design Heuristics

- Make the primary action reachable with thumb-friendly controls and sufficient spacing.
- Keep page purpose obvious through structure and action hierarchy, not long explanation copy.
- Preserve return context: selected tab, filters, scroll position, draft form, payment/result state, and pending action state where relevant.
- Treat weak network as normal: show loading, retry, last known data, unknown result, and safe retry rules.
- Use the installed TDesign Mini Program components first; add local wrappers only when they own real layout, state, warning, summary, or danger containment.
- For forms, labels explain purpose; placeholders only add format, example, constraint, or state-specific hints.
- Reduce typing: use picker/select/stepper/search presets when the value is constrained or repetitive.

## Anti-Patterns

- Button-only payment or submit flows with no duplicate-tap, unknown-result, or re-entry handling.
- Text-heavy explanation cards on single-task pages.
- Tiny local text actions for row-level edit/delete/status where icon buttons or icon-led small buttons fit better.
- Native/custom controls where TDesign already provides the needed behavior.
- Rebuilding the same task's state and recovery logic across several pages.

## Implementation Check

- What happens if the user leaves WeChat, returns, or the network times out?
- Which context must survive back, refresh, tab switch, or foreground re-entry?
- Can the user complete the common path without typing unnecessary text?
- Are touch targets, spacing, safe area, and loading/empty/error states visible and stable?
