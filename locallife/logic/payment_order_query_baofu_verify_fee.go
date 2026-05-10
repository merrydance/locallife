package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

func (svc *PaymentOrderService) markDirectBaofuVerifyFeeQueryPaymentPaid(ctx context.Context, paymentOrder db.PaymentOrder, transactionID string) (db.PaymentOrder, error) {
	if paymentOrder.Status == paymentStatusPaid {
		return paymentOrder, nil
	}
	transactionID = strings.TrimSpace(transactionID)
	updatedPaymentOrder, err := svc.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: transactionID, Valid: transactionID != ""},
	})
	if err == nil {
		return updatedPaymentOrder, nil
	}
	if errors.Is(err, db.ErrRecordNotFound) {
		currentPaymentOrder, getErr := svc.store.GetPaymentOrder(ctx, paymentOrder.ID)
		if getErr != nil {
			return db.PaymentOrder{}, fmt.Errorf("get direct baofu verify fee payment order after paid update conflict: %w", getErr)
		}
		if currentPaymentOrder.Status == paymentStatusPaid {
			return currentPaymentOrder, nil
		}
	}
	return db.PaymentOrder{}, fmt.Errorf("mark direct baofu verify fee payment order %d paid from query: %w", paymentOrder.ID, err)
}

func shouldDeferDirectPaymentQueryFactApplication(paymentOrder db.PaymentOrder, terminalStatus string) bool {
	return paymentOrder.BusinessType == db.PaymentBusinessTypeBaofuAccountVerifyFee && terminalStatus == db.ExternalPaymentTerminalStatusSuccess
}

func shouldSkipBaofuVerifyFeeQuerySuccessApplication(paymentOrder db.PaymentOrder, terminalStatus string, queryResp *wechatcontracts.DirectOrderQueryResponse) bool {
	if queryResp == nil || paymentOrder.BusinessType != db.PaymentBusinessTypeBaofuAccountVerifyFee || terminalStatus != db.ExternalPaymentTerminalStatusSuccess {
		return false
	}
	return queryResp.Amount.Total != paymentOrder.Amount
}
