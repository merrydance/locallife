# Rider Deposit Refund Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a focused recovery scan for stale rider-deposit refund orders that remain `pending` after an uncertain direct-refund create outcome, so the scheduler can re-query upstream and reattach the existing `out_refund_no` to an auditable fact/application path.

**Architecture:** Keep the change inside the existing refund recovery flow. Add one dedicated pending rider-deposit recovery query in `db/query/refund_order.sql`, then add a small scheduler branch that reuses the existing direct refund status query and payment-fact recording path. Preserve the original refund order and idempotency surface; the new scan only reclaims stale pending/unknown rows.

**Tech Stack:** Go, sqlc, existing `RefundRecoveryScheduler`, existing `PaymentFactService`, existing `wechat.DirectPaymentClientInterface`, existing gomock tests.

## Implementation Result

- Status: completed on 2026-06-22.
- Implemented: a focused `pending`/`unknown` rider-deposit refund recovery scan in `RefundRecoveryScheduler`, reusing the original `out_refund_no` and the existing fact/application path.
- Regenerated: `sqlc` and generated mocks.
- Validated: `go test ./worker -run 'TestRefundRecoverySchedulerRunOnce' -v`, `go test ./logic -run TestRiderDepositRefundService_SubmitWithdrawal -v`, `make sqlc`, and `make check-generated`.
- Residual risk: only focused worker/logic regressions were run; a real provider lost-callback replay remains unexercised outside unit tests.

---

### Task 1: Add a failing regression test for stale rider-deposit pending recovery

**Files:**
- Modify: `locallife/worker/refund_recovery_scheduler_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestRefundRecoverySchedulerRunOnceRecoversPendingRiderDepositRefunds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &riderDepositRefundFactApplicationRecorder{}
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	store.EXPECT().ListPaidUnrefundedPaymentOrders(gomock.Any(), int32(50)).Return([]db.PaymentOrder{}, nil)
	store.EXPECT().ListPaidUnrefundedReservationPaymentOrders(gomock.Any(), int32(50)).Return([]db.PaymentOrder{}, nil)
	store.EXPECT().ListPendingOrderRefundOrdersForRecovery(gomock.Any(), gomock.Any()).Return([]db.ListPendingOrderRefundOrdersForRecoveryRow{}, nil)
	store.EXPECT().ListPendingReservationRefundOrdersForRecovery(gomock.Any(), gomock.Any()).Return([]db.ListPendingReservationRefundOrdersForRecoveryRow{}, nil)
	store.EXPECT().ListPendingRiderDepositRefundOrdersForRecovery(gomock.Any(), gomock.Any()).Return([]db.ListPendingRiderDepositRefundOrdersForRecoveryRow{{
		ID:             61,
		PaymentOrderID: 91,
		RefundAmount:   20000,
		OutRefundNo:    "RFD_PENDING_RIDER_001",
		BusinessType:   db.ExternalPaymentBusinessOwnerRiderDeposit,
		PaymentChannel: db.PaymentChannelDirect,
	}}, nil)
	store.EXPECT().ListStuckProcessingRefundOrders(gomock.Any(), gomock.Any()).Return([]db.ListStuckProcessingRefundOrdersRow{}, nil)
	paymentClient.EXPECT().QueryRefund(gomock.Any(), "RFD_PENDING_RIDER_001").Return(&wechat.RefundResponse{
		RefundID: "WX_REFUND_PENDING_RIDER_001",
		Status:   wechat.RefundStatusSuccess,
	}, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, "RFD_PENDING_RIDER_001", arg.ExternalObjectKey)
			require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.Equal(t, db.ExternalPaymentBusinessOwnerRiderDeposit, arg.BusinessOwner.String)
			return db.ExternalPaymentFact{ID: 401, IsTerminal: true}, nil
		},
	)
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).Return(
		db.ExternalPaymentFactApplication{
			ID:                 501,
			FactID:             401,
			Consumer:           "rider_deposit_domain",
			BusinessObjectType: "refund_order",
			BusinessObjectID:   61,
			Status:             db.ExternalPaymentFactApplicationStatusPending,
		},
		nil,
	)

	scheduler := worker.NewRefundRecoveryScheduler(store, distributor, paymentClient)
	scheduler.RunOnce()

	require.Equal(t, []int64{501}, distributor.applicationIDs)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd locallife && go test ./worker -run TestRefundRecoverySchedulerRunOnceRecoversPendingRiderDepositRefunds -v`
Expected: fail because `ListPendingRiderDepositRefundOrdersForRecovery` does not exist yet and the scheduler has no rider-deposit pending scan.

- [ ] **Step 3: Do not implement code yet**

Keep the test red until the query and scheduler branch are added.

### Task 2: Add the rider-deposit pending recovery query and scheduler branch

**Files:**
- Modify: `locallife/db/query/refund_order.sql`
- Modify: `locallife/worker/refund_recovery_scheduler.go`
- Create: `locallife/worker/refund_recovery_scheduler_rider_deposit.go`

- [ ] **Step 1: Add the SQL query**

```sql
-- name: ListPendingRiderDepositRefundOrdersForRecovery :many
SELECT
    ro.id,
    ro.payment_order_id,
    ro.refund_amount,
    ro.out_refund_no,
    po.business_type,
    po.payment_channel
FROM refund_orders ro
JOIN payment_orders po ON po.id = ro.payment_order_id
LEFT JOIN external_payment_commands epc
    ON epc.provider = 'wechat'
   AND epc.channel = 'direct'
   AND epc.capability = 'direct_refund'
   AND epc.command_type = 'create_refund'
   AND epc.external_object_type = 'refund'
   AND epc.external_object_key = ro.out_refund_no
WHERE ro.status = 'pending'
  AND po.status = 'paid'
  AND po.business_type = 'rider_deposit'
  AND po.payment_channel = 'direct'
  AND ro.refund_type = 'rider_deposit'
  AND ro.created_at < sqlc.arg('created_before')
  AND (epc.id IS NULL OR epc.command_status = 'unknown')
ORDER BY ro.created_at ASC, ro.id ASC
LIMIT sqlc.arg('limit')::int;
```

- [ ] **Step 2: Wire the scheduler branch**

```go
func (s *RefundRecoveryScheduler) runOnce(ctx context.Context) {
	// ...
	s.recoverPendingRiderDepositRefunds(ctx)
	s.recoverStuckProcessingRefunds(ctx)
}
```

```go
func (s *RefundRecoveryScheduler) recoverPendingRiderDepositRefunds(ctx context.Context) {
	pendingRiderDepositRefundOrders, err := s.store.ListPendingRiderDepositRefundOrdersForRecovery(ctx, db.ListPendingRiderDepositRefundOrdersForRecoveryParams{
		CreatedBefore: time.Now().Add(-1 * time.Minute),
		Limit:         refundRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list pending rider deposit refund orders for recovery failed")
		return
	}

	for _, refundOrder := range pendingRiderDepositRefundOrders {
		if refundOrder.PaymentChannel != db.PaymentChannelDirect || refundOrder.BusinessType != db.ExternalPaymentBusinessOwnerRiderDeposit {
			continue
		}

		paymentOrder := db.PaymentOrder{
			ID:             refundOrder.PaymentOrderID,
			BusinessType:   refundOrder.BusinessType,
			PaymentChannel: refundOrder.PaymentChannel,
		}
		refund := db.RefundOrder{
			ID:             refundOrder.ID,
			PaymentOrderID: refundOrder.PaymentOrderID,
			RefundAmount:   refundOrder.RefundAmount,
			OutRefundNo:    refundOrder.OutRefundNo,
		}

		refundStatus, refundID, err := s.queryRefundStatus(ctx, paymentOrder, refund)
		if err != nil {
			log.Error().Err(err).Int64("refund_order_id", refundOrder.ID).Str("out_refund_no", refundOrder.OutRefundNo).Msg("query pending rider deposit refund status failed")
			continue
		}

		application, err := s.recordRiderDepositDirectRefundQueryFact(ctx, paymentOrder, refund, refundStatus, refundID)
		if err != nil {
			log.Error().Err(err).Int64("refund_order_id", refundOrder.ID).Str("out_refund_no", refundOrder.OutRefundNo).Msg("record pending rider deposit refund query fact failed")
			continue
		}
		if application == nil {
			continue
		}

		enqueueOrderPaymentFactApplication(ctx, s.distributor, application)
	}
}
```

- [ ] **Step 3: Keep the branch narrow**

Do not alter `SubmitWithdrawal`, `ResolveRefund`, or the existing stuck-processing scan semantics.

### Task 3: Regenerate sqlc and re-run the regression

**Files:**
- Modify generated sqlc artifacts under `locallife/db/sqlc/` as produced by `make sqlc`

- [ ] **Step 1: Regenerate sqlc**

Run: `cd locallife && make sqlc`

- [ ] **Step 2: Re-run the focused worker test**

Run: `cd locallife && go test ./worker -run TestRefundRecoverySchedulerRunOnceRecoversPendingRiderDepositRefunds -v`
Expected: PASS.

- [ ] **Step 3: Run the existing rider-deposit refund service tests**

Run: `cd locallife && go test ./logic -run TestRiderDepositRefundService_SubmitWithdrawal -v`
Expected: PASS.

### Task 4: Close out the audit record if the implementation holds

**Files:**
- Modify: `.github/review/open-findings.md`
- Modify: `.github/review/audit-log.md`
- Modify: `artifacts/production-risk-audit/state-sequencing-audit-snapshot-2026-06-16.md`

- [ ] **Step 1: Update the audit finding status**
- [ ] **Step 2: Add a short durable note describing the recovery scan that was added**
- [ ] **Step 3: Verify the audit wording still matches the implementation**
