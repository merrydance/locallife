# Baofoo Payment Codegraph Slices

This directory stores human-audited LocalLife codegraph slices for Baofoo/BaoCaiTong payment flows.

Important: these slices are variant-specific. LocalLife has multiple order, payment, refund, recovery, and profit-sharing paths; no single slice should be treated as "the" payment or refund chain.

## Artifact Types

- `*.slice.md`: review narrative, invariants, recovery paths, and refactor notes.
- `*.edges.json`: machine-readable graph nodes and directed edges. Edges should represent real route, call, write, enqueue, scheduler, transaction, or provider-call relationships. Non-relationships and warnings belong in the markdown slice, not in graph edges.

## Current Slices

- `flow-variant-index.md`: coverage map for Baofoo payment/refund/profit-sharing variants and missing high-value slices.
- `aggregate-payment.slice.md`: Baofoo aggregate payment success callback -> order paid -> profit-sharing bill/command -> share callback application.
- `refund.slice.md`: Baofoo pre-share refund command -> refund callback/query fact -> order/reservation refund terminalization -> outbox side effects.
