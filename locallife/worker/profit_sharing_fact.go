package worker

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

const (
	profitSharingFactBusinessObjectOrder       = "profit_sharing_order"
	profitSharingReturnFactBusinessObject      = "profit_sharing_return"
	profitSharingFactConsumerDomain            = "profit_sharing_domain"
	riderDepositPaymentFactBusinessObjectOrder = "payment_order"
	riderDepositPaymentFactConsumerDomain      = "rider_deposit_domain"
	riderDepositRefundFactBusinessObjectOrder  = "refund_order"
	riderDepositRefundFactConsumerDomain       = "rider_deposit_domain"
	orderRefundFactBusinessObjectOrder         = "refund_order"
	orderRefundFactConsumerDomain              = "order_domain"
	reservationRefundFactBusinessObjectOrder   = "refund_order"
	reservationRefundFactConsumerDomain        = "reservation_domain"
)

func recordProfitSharingQueryFact(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, profitSharingOrder db.ProfitSharingOrder, queryResp *wechatcontracts.ProfitSharingQueryResponse, finalResult, finalFailReason string) (*db.ExternalPaymentFactApplication, error) {
	if queryResp == nil {
		return nil, nil
	}
	service := logic.NewPaymentFactService(store)
	sharingOrderID := profitSharingFactSharingOrderID(profitSharingOrder, queryResp)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:    profitSharingOrder.OutOrderNo,
		ExternalSecondaryKey: paymentFactStringPtr(sharingOrderID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerProfitSharing),
		BusinessObjectType:   paymentFactStringPtr(profitSharingFactBusinessObjectOrder),
		BusinessObjectID:     paymentFactInt64Ptr(profitSharingOrder.ID),
		UpstreamState:        finalResult,
		TerminalStatus:       logic.NormalizeProfitSharingTerminalStatus(finalResult),
		Amount:               profitSharingFactAmount(queryResp),
		Currency:             "CNY",
		RawResource:          profitSharingQueryFactResource(paymentOrder, profitSharingOrder, queryResp, sharingOrderID, finalResult, finalFailReason),
		DedupeKey:            profitSharingQueryFactDedupeKey(profitSharingOrder.OutOrderNo, finalResult),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           profitSharingFactConsumerDomain,
			BusinessObjectType: profitSharingFactBusinessObjectOrder,
			BusinessObjectID:   profitSharingOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func recordProfitSharingCommandResponseFact(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, profitSharingOrder db.ProfitSharingOrder, resp *wechatcontracts.ProfitSharingResponse, commandType string, amount *int64) (*db.ExternalPaymentFactApplication, error) {
	if resp == nil {
		return nil, nil
	}
	service := logic.NewPaymentFactService(store)

	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		FactSource:           db.ExternalPaymentFactSourceCommandResponse,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:    profitSharingOrder.OutOrderNo,
		ExternalSecondaryKey: optionalPaymentFactStringPtr(resp.OrderID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerProfitSharing),
		BusinessObjectType:   paymentFactStringPtr(profitSharingFactBusinessObjectOrder),
		BusinessObjectID:     paymentFactInt64Ptr(profitSharingOrder.ID),
		UpstreamState:        resp.Status,
		TerminalStatus:       db.ExternalPaymentTerminalStatusUnknown,
		Amount:               amount,
		Currency:             "CNY",
		RawResource:          profitSharingCommandResponseFactResource(paymentOrder, profitSharingOrder, resp, commandType, amount),
		DedupeKey:            profitSharingCommandResponseFactDedupeKey(profitSharingOrder.OutOrderNo, commandType, resp.Status),
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func recordProfitSharingReturnQueryFact(ctx context.Context, store db.Store, returnRecord db.ProfitSharingReturn, queryResp *wechatcontracts.ProfitSharingReturnResponse) (*db.ExternalPaymentFactApplication, error) {
	if queryResp == nil {
		return nil, nil
	}
	service := logic.NewPaymentFactService(store)
	amount := profitSharingReturnFactAmount(returnRecord, queryResp)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:    returnRecord.OutReturnNo,
		ExternalSecondaryKey: optionalPaymentFactStringPtr(queryResp.ReturnID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerProfitSharing),
		BusinessObjectType:   paymentFactStringPtr(profitSharingReturnFactBusinessObject),
		BusinessObjectID:     paymentFactInt64Ptr(returnRecord.ID),
		UpstreamState:        queryResp.Result,
		TerminalStatus:       logic.NormalizeProfitSharingTerminalStatus(queryResp.Result),
		Amount:               amount,
		Currency:             "CNY",
		RawResource:          profitSharingReturnQueryFactResource(returnRecord, queryResp),
		DedupeKey:            profitSharingReturnQueryFactDedupeKey(returnRecord.OutReturnNo, queryResp.Result),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           profitSharingFactConsumerDomain,
			BusinessObjectType: profitSharingReturnFactBusinessObject,
			BusinessObjectID:   returnRecord.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func recordProfitSharingReturnCommandResponseFact(ctx context.Context, store db.Store, returnRecord db.ProfitSharingReturn, returnResp *wechatcontracts.ProfitSharingReturnResponse) (*db.ExternalPaymentFactApplication, error) {
	if returnResp == nil {
		return nil, nil
	}
	service := logic.NewPaymentFactService(store)
	amount := profitSharingReturnFactAmount(returnRecord, returnResp)

	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		FactSource:           db.ExternalPaymentFactSourceCommandResponse,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:    returnRecord.OutReturnNo,
		ExternalSecondaryKey: optionalPaymentFactStringPtr(returnResp.ReturnID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerProfitSharing),
		BusinessObjectType:   paymentFactStringPtr(profitSharingReturnFactBusinessObject),
		BusinessObjectID:     paymentFactInt64Ptr(returnRecord.ID),
		UpstreamState:        returnResp.Result,
		TerminalStatus:       db.ExternalPaymentTerminalStatusUnknown,
		Amount:               amount,
		Currency:             "CNY",
		RawResource:          profitSharingReturnCommandResponseFactResource(returnRecord, returnResp),
		DedupeKey:            profitSharingReturnCommandResponseFactDedupeKey(returnRecord.OutReturnNo, returnResp.Result),
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func recordProfitSharingReturnCommandErrorFact(ctx context.Context, store db.Store, returnRecord db.ProfitSharingReturn, commandErr error) (*db.ExternalPaymentFactApplication, error) {
	errorCode, errorMessage := workerPaymentCommandErrorFields(commandErr)
	failReason := workerStringValue(errorMessage)
	if failReason == "" && commandErr != nil {
		failReason = commandErr.Error()
	}

	service := logic.NewPaymentFactService(store)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelEcommerce,
		Capability:         db.ExternalPaymentCapabilityProfitSharing,
		FactSource:         db.ExternalPaymentFactSourceCommandResponse,
		ExternalObjectType: db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:  returnRecord.OutReturnNo,
		BusinessOwner:      paymentFactStringPtr(db.ExternalPaymentBusinessOwnerProfitSharing),
		BusinessObjectType: paymentFactStringPtr(profitSharingReturnFactBusinessObject),
		BusinessObjectID:   paymentFactInt64Ptr(returnRecord.ID),
		UpstreamState:      db.ExternalPaymentTerminalStatusFailed,
		TerminalStatus:     db.ExternalPaymentTerminalStatusUnknown,
		Amount:             paymentFactInt64Ptr(returnRecord.Amount),
		Currency:           "CNY",
		RawResource:        profitSharingReturnCommandErrorFactResource(returnRecord, errorCode, errorMessage, failReason),
		DedupeKey:          profitSharingReturnCommandResponseFactDedupeKey(returnRecord.OutReturnNo, db.ExternalPaymentCommandStatusRejected),
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func enqueueProfitSharingPaymentFactApplication(ctx context.Context, distributor TaskDistributor, application *db.ExternalPaymentFactApplication) {
	if distributor == nil || application == nil {
		return
	}
	applicationDistributor, ok := distributor.(PaymentFactApplicationTaskDistributor)
	if !ok {
		return
	}
	if err := applicationDistributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	); err != nil {
		log.Warn().Err(err).
			Int64("payment_fact_application_id", application.ID).
			Int64("payment_fact_id", application.FactID).
			Int64("profit_sharing_order_id", application.BusinessObjectID).
			Msg("enqueue profit sharing payment fact application from query failed; scheduler will retry")
	}
}

func enqueueProfitSharingReturnPaymentFactApplication(ctx context.Context, distributor TaskDistributor, application *db.ExternalPaymentFactApplication) {
	if distributor == nil || application == nil {
		return
	}
	applicationDistributor, ok := distributor.(PaymentFactApplicationTaskDistributor)
	if !ok {
		return
	}
	if err := applicationDistributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	); err != nil {
		log.Warn().Err(err).
			Int64("payment_fact_application_id", application.ID).
			Int64("payment_fact_id", application.FactID).
			Int64("profit_sharing_return_id", application.BusinessObjectID).
			Msg("enqueue profit sharing return payment fact application failed; scheduler will retry")
	}
}

func enqueueRiderDepositRefundPaymentFactApplication(ctx context.Context, distributor TaskDistributor, application *db.ExternalPaymentFactApplication) {
	if distributor == nil || application == nil {
		return
	}
	applicationDistributor, ok := distributor.(PaymentFactApplicationTaskDistributor)
	if !ok {
		return
	}
	if err := applicationDistributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	); err != nil {
		log.Warn().Err(err).
			Int64("payment_fact_application_id", application.ID).
			Int64("payment_fact_id", application.FactID).
			Int64("refund_order_id", application.BusinessObjectID).
			Msg("enqueue rider deposit refund payment fact application failed; scheduler will retry")
	}
}

func enqueueOrderRefundPaymentFactApplication(ctx context.Context, distributor TaskDistributor, application *db.ExternalPaymentFactApplication) {
	if distributor == nil || application == nil {
		return
	}
	applicationDistributor, ok := distributor.(PaymentFactApplicationTaskDistributor)
	if !ok {
		return
	}
	if err := applicationDistributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	); err != nil {
		log.Warn().Err(err).
			Int64("payment_fact_application_id", application.ID).
			Int64("payment_fact_id", application.FactID).
			Int64("refund_order_id", application.BusinessObjectID).
			Msg("enqueue order refund payment fact application failed; scheduler will retry")
	}
}

func enqueueReservationRefundPaymentFactApplication(ctx context.Context, distributor TaskDistributor, application *db.ExternalPaymentFactApplication) {
	if distributor == nil || application == nil {
		return
	}
	applicationDistributor, ok := distributor.(PaymentFactApplicationTaskDistributor)
	if !ok {
		return
	}
	if err := applicationDistributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	); err != nil {
		log.Warn().Err(err).
			Int64("payment_fact_application_id", application.ID).
			Int64("payment_fact_id", application.FactID).
			Int64("refund_order_id", application.BusinessObjectID).
			Msg("enqueue reservation refund payment fact application failed; scheduler will retry")
	}
}

func profitSharingFactSharingOrderID(profitSharingOrder db.ProfitSharingOrder, queryResp *wechatcontracts.ProfitSharingQueryResponse) string {
	if queryResp != nil && queryResp.OrderID != "" {
		return queryResp.OrderID
	}
	if profitSharingOrder.SharingOrderID.Valid {
		return profitSharingOrder.SharingOrderID.String
	}
	return ""
}

func profitSharingQueryFactDedupeKey(outOrderNo, upstreamState string) string {
	return "wechat:query:ecommerce:profit_sharing:" + outOrderNo + ":" + logic.NormalizeProfitSharingTerminalStatus(upstreamState)
}

func profitSharingCommandResponseFactDedupeKey(outOrderNo, commandType, terminalStatus string) string {
	return "wechat:command_response:ecommerce:profit_sharing:" + commandType + ":" + outOrderNo + ":" + terminalStatus
}

func profitSharingFactAmount(queryResp *wechatcontracts.ProfitSharingQueryResponse) *int64 {
	if queryResp == nil {
		return nil
	}
	var amount int64
	for _, receiver := range queryResp.Receivers {
		amount += receiver.Amount
	}
	if amount == 0 {
		return nil
	}
	return paymentFactInt64Ptr(amount)
}

func profitSharingCommandResponseAmount(profitSharingOrder db.ProfitSharingOrder) *int64 {
	amount := profitSharingOrder.PlatformCommission + profitSharingOrder.OperatorCommission + profitSharingOrder.RiderAmount
	if amount <= 0 {
		return nil
	}
	return paymentFactInt64Ptr(amount)
}

func profitSharingReturnFactAmount(returnRecord db.ProfitSharingReturn, queryResp *wechatcontracts.ProfitSharingReturnResponse) *int64 {
	if queryResp != nil && queryResp.Amount > 0 {
		return paymentFactInt64Ptr(queryResp.Amount)
	}
	if returnRecord.Amount > 0 {
		return paymentFactInt64Ptr(returnRecord.Amount)
	}
	return nil
}

func optionalPaymentFactStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func profitSharingQueryFactResource(paymentOrder db.PaymentOrder, profitSharingOrder db.ProfitSharingOrder, queryResp *wechatcontracts.ProfitSharingQueryResponse, sharingOrderID string, finalResult, finalFailReason string) []byte {
	receiverResults := make([]map[string]any, 0, len(queryResp.Receivers))
	for _, receiver := range queryResp.Receivers {
		receiverResults = append(receiverResults, map[string]any{
			"type":        receiver.Type,
			"amount":      receiver.Amount,
			"result":      receiver.Result,
			"fail_reason": receiver.FailReason,
			"detail_id":   receiver.DetailID,
		})
	}

	raw, err := json.Marshal(map[string]any{
		"payment_order_id":        paymentOrder.ID,
		"profit_sharing_order_id": profitSharingOrder.ID,
		"transaction_id":          paymentOrder.TransactionID.String,
		"sub_mch_id":              queryResp.SubMchID,
		"out_order_no":            profitSharingOrder.OutOrderNo,
		"order_id":                sharingOrderID,
		"query_status":            queryResp.Status,
		"result":                  finalResult,
		"fail_reason":             finalFailReason,
		"receiver_count":          len(receiverResults),
		"receiver_results":        receiverResults,
		"finish_amount":           queryResp.FinishAmount,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_order_no", profitSharingOrder.OutOrderNo).Msg("marshal profit sharing query fact resource failed")
		return nil
	}
	return raw
}

func profitSharingCommandResponseFactResource(paymentOrder db.PaymentOrder, profitSharingOrder db.ProfitSharingOrder, resp *wechatcontracts.ProfitSharingResponse, commandType string, amount *int64) []byte {
	raw, err := json.Marshal(map[string]any{
		"payment_order_id":        paymentOrder.ID,
		"profit_sharing_order_id": profitSharingOrder.ID,
		"transaction_id":          paymentOrder.TransactionID.String,
		"out_order_no":            profitSharingOrder.OutOrderNo,
		"order_id":                resp.OrderID,
		"command_type":            commandType,
		"status":                  resp.Status,
		"amount":                  amount,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_order_no", profitSharingOrder.OutOrderNo).Msg("marshal profit sharing command response fact resource failed")
		return nil
	}
	return raw
}

func profitSharingReturnQueryFactDedupeKey(outReturnNo, upstreamState string) string {
	return "wechat:query:ecommerce:profit_sharing_return:" + outReturnNo + ":" + logic.NormalizeProfitSharingTerminalStatus(upstreamState)
}

func profitSharingReturnCommandResponseFactDedupeKey(outReturnNo, terminalStatus string) string {
	return "wechat:command_response:ecommerce:profit_sharing_return:" + outReturnNo + ":" + terminalStatus
}

func profitSharingReturnQueryFactResource(returnRecord db.ProfitSharingReturn, queryResp *wechatcontracts.ProfitSharingReturnResponse) []byte {
	raw, err := json.Marshal(map[string]any{
		"profit_sharing_return_id": returnRecord.ID,
		"refund_order_id":          returnRecord.RefundOrderID,
		"out_order_no":             returnRecord.OutOrderNo,
		"out_return_no":            returnRecord.OutReturnNo,
		"sub_mch_id":               queryResp.SubMchID,
		"return_id":                queryResp.ReturnID,
		"amount":                   queryResp.Amount,
		"result":                   queryResp.Result,
		"fail_reason":              queryResp.FailReason,
		"transaction_id":           queryResp.TransactionID,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_return_no", returnRecord.OutReturnNo).Msg("marshal profit sharing return query fact resource failed")
		return nil
	}
	return raw
}

func profitSharingReturnCommandResponseFactResource(returnRecord db.ProfitSharingReturn, returnResp *wechatcontracts.ProfitSharingReturnResponse) []byte {
	raw, err := json.Marshal(map[string]any{
		"profit_sharing_return_id": returnRecord.ID,
		"refund_order_id":          returnRecord.RefundOrderID,
		"out_order_no":             returnRecord.OutOrderNo,
		"out_return_no":            returnRecord.OutReturnNo,
		"sub_mch_id":               returnResp.SubMchID,
		"return_id":                returnResp.ReturnID,
		"amount":                   returnResp.Amount,
		"result":                   returnResp.Result,
		"fail_reason":              returnResp.FailReason,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_return_no", returnRecord.OutReturnNo).Msg("marshal profit sharing return command response fact resource failed")
		return nil
	}
	return raw
}

func profitSharingReturnCommandErrorFactResource(returnRecord db.ProfitSharingReturn, errorCode, errorMessage *string, failReason string) []byte {
	raw, err := json.Marshal(map[string]any{
		"profit_sharing_return_id": returnRecord.ID,
		"refund_order_id":          returnRecord.RefundOrderID,
		"out_order_no":             returnRecord.OutOrderNo,
		"out_return_no":            returnRecord.OutReturnNo,
		"amount":                   returnRecord.Amount,
		"error_code":               workerStringValue(errorCode),
		"error_message":            workerStringValue(errorMessage),
		"fail_reason":              failReason,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_return_no", returnRecord.OutReturnNo).Msg("marshal profit sharing return command error fact resource failed")
		return nil
	}
	return raw
}
