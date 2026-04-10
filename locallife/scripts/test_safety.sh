#!/usr/bin/env bash
set -euo pipefail

go test ./logic -run 'Test(CalculateOrderPreview|ComputeDeliveryQuote)' -count=1
go test ./db/sqlc -run 'Test(CreateOrderTxRejectsDuplicateActiveReservationOrder|CreateCombinedPaymentTx_ReturnsPendingPaymentConflict|CreateRefundOrderTx_CountsPendingAndProcessingRefunds|GrabOrderTx|UpdateDeliveryToPickupTx_RollsBackWhenOrderSyncFails)' -count=1 -p 1
go test ./api -run 'Test(CreateOrderAPI|CreatePaymentOrderAPI|GrabOrderAPI|StartPickupAPI|ConfirmPickupAPI|StartDeliveryAPI)' -count=1
