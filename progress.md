# Progress: WeApp User-Experience Function Audit

## 2026-05-17

- Read LocalLife root routing instructions and `.github` indexes.
- Selected Mini Program review route: `.github/instructions/weapp-mini-program.instructions.md` plus `.github/prompts/weapp-review.prompt.md`.
- Read the frontend architecture and Mini Program page delivery baselines.
- Created review plan files for this multi-step audit.
- Mapped app-level routes and standards: this is a multi-role Mini Program with consumer, merchant, rider, operator, platform, payment, order, dine-in, and reservation surfaces.
- Noted pre-existing dirty workspace changes in Baofoo settlement account files; audit will not revert them.
- Started static implementation audit. Default PATH did not expose `node`; will rerun script checks with `PATH="$HOME/.local/bin:$PATH"`.
- Re-ran route and static binding audits with local Node.
- Confirmed two user-facing incomplete action loops: dine-in checkout voucher/guest-count bindings and takeout cart unavailable-item removal.
- Checked payment workflow implementation; it uses backend status polling/requery after WeChat payment and has pending-confirmation result handling.
- Ran `PATH="$HOME/.local/bin:$PATH" npm run compile` from `weapp/`; TypeScript compilation exited 0.
- Converted the audit output into a bounded remediation plan in `task_plan.md`, covering two P0 defects, three P1 debt/gate tasks, and two P2 UX consistency tasks with scope, non-scope, touched files, acceptance criteria, validation, and residual risk.
- Pushed current workspace checkpoint as `10553cc8 chore: checkpoint current workspace`.
- Completed P0-1 dine-in checkout cleanup: removed stale guest-count stepper binding and local voucher popup wiring, changed merchant promo voucher event to the existing `onVoucherClaimed` refresh path, and removed unused component declarations/styles.
- Verified P0-1 with a targeted handler check, `PATH="$HOME/.local/bin:$PATH" npm run compile`, and `PATH="$HOME/.local/bin:$PATH" npm run lint`; all exited 0.
- Completed P0-2 takeout cart unavailable item removal: added `onRemoveUnavailable`, duplicate-tap item state, delete API call, local cart removal reuse, and user-facing failure Toast.
- Verified P0-2 with a targeted handler check, `PATH="$HOME/.local/bin:$PATH" npm run compile`, `PATH="$HOME/.local/bin:$PATH" npm run lint`, and `PATH="$HOME/.local/bin:$PATH" npm run gate:wxml-expression-safety`; all exited 0.
- Error note: one exploratory `rg` command for WXML dynamic loading/disabled expressions failed due to an unescaped `{` in the regex; use fixed-string or simpler quoted patterns next time.
