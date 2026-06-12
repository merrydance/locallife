# Review Rubric

Use for findings-first frontend review when judging whether a UI feels aligned with user habits and task needs.

## Findings Priority

1. Wrong user task or page boundary.
2. API/DTO/entity flattening that shapes the UI.
3. Missing or confused task-domain owner, state owner, or workflow owner.
4. Missing ViewState, recovery, retry, unknown-result, or re-entry behavior.
5. High-frequency path slowed by low-frequency features, repeated typing, or reset context.
6. Dangerous action, money/status/auth-sensitive path, or destructive flow without clear confirmation and recovery.
7. Business copy, status, filter, or action labels that are not understandable to the role.
8. Visual/system drift after structural issues are ruled out.

## Questions To Answer

- Who is the page for, and what is the primary task?
- What does the user need to know in the first 5 seconds?
- Is the most common action easiest, safest, and closest to the relevant data?
- Are defaults, remembered state, and return behavior aligned with repeated use?
- Are empty, error, weak-network, submitting, stale, and unknown-result states actionable?
- Does the page preserve context after refresh, return, or partial failure?
- Would removing explanatory copy still leave the page understandable?
- Are backend fields adapted into role-readable view models before rendering?

## Output Shape

Lead with findings. For each finding, include:

- Path or surface.
- User impact.
- Evidence from code or visible behavior.
- Expected user-centered behavior.
- Whether it is a baseline defect, upgrade opportunity, or residual risk.

If no issues are found, state remaining validation gaps, especially real-device, weak-network, re-entry, payment/result, and high-frequency workflow checks.
