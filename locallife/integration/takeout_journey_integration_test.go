package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	api "github.com/merrydance/locallife/api"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	baofuaggregatenotification "github.com/merrydance/locallife/baofu/aggregatepay/notification"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/scheduler"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type takeoutOrderResponse struct {
	ID        int64  `json:"id"`
	OrderType string `json:"order_type"`
	Status    string `json:"status"`
}

type takeoutDeliveryResponse struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

type takeoutPaymentOrderResponse struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	BusinessType string `json:"business_type"`
}

type dineInSessionResponse struct {
	Session struct {
		ID int64 `json:"id"`
	} `json:"session"`
}

type kitchenOrderStatusResponse struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

type reservationStatusResponse struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

type claimSubmitResponse struct {
	ClaimID                int64  `json:"claim_id"`
	Status                 string `json:"status"`
	DecisionStatus         string `json:"decision_status"`
	CompensationStatus     string `json:"compensation_status"`
	PayoutStatus           string `json:"payout_status"`
	CustomerActionRequired bool   `json:"customer_action_required"`
	CustomerAction         string `json:"customer_action"`
	ApprovedAmount         *int64 `json:"approved_amount"`
	CompensationSource     string `json:"compensation_source"`
	Reason                 string `json:"reason"`
}

type claimContinueResponse struct {
	ID                     int64  `json:"id"`
	Status                 string `json:"status"`
	DecisionStatus         string `json:"decision_status"`
	CompensationStatus     string `json:"compensation_status"`
	PayoutStatus           string `json:"payout_status"`
	CustomerActionRequired bool   `json:"customer_action_required"`
	CustomerAction         string `json:"customer_action"`
	ApprovedAmount         *int64 `json:"approved_amount"`
	Reason                 string `json:"reason"`
}

func requireAcceptedClaimSubmission(t *testing.T, claimResp claimSubmitResponse, approvedAmount int64) {
	t.Helper()

	require.NotZero(t, claimResp.ClaimID)
	require.Equal(t, "warned_waiting_customer_confirmation", claimResp.Status)
	require.Equal(t, "auto-adjudicated", claimResp.DecisionStatus)
	require.Equal(t, "awaiting_compensation", claimResp.CompensationStatus)
	require.Empty(t, claimResp.PayoutStatus)
	require.True(t, claimResp.CustomerActionRequired)
	require.Equal(t, "confirm_continue", claimResp.CustomerAction)
	require.NotNil(t, claimResp.ApprovedAmount)
	require.Equal(t, approvedAmount, *claimResp.ApprovedAmount)
	require.NotEmpty(t, claimResp.CompensationSource)
	require.NotEmpty(t, claimResp.Reason)
}

func requireAcceptedClaimContinue(t *testing.T, claimResp claimContinueResponse, claimID, approvedAmount int64) {
	t.Helper()

	require.Equal(t, claimID, claimResp.ID)
	require.Equal(t, "accepted", claimResp.Status)
	require.Equal(t, "auto-adjudicated", claimResp.DecisionStatus)
	require.Equal(t, "compensating", claimResp.CompensationStatus)
	require.Equal(t, "processing", claimResp.PayoutStatus)
	require.False(t, claimResp.CustomerActionRequired)
	require.Empty(t, claimResp.CustomerAction)
	require.NotNil(t, claimResp.ApprovedAmount)
	require.Equal(t, approvedAmount, *claimResp.ApprovedAmount)
	require.NotEmpty(t, claimResp.Reason)
}

func confirmClaimContinueIntegration(t *testing.T, server *api.Server, claimID, customerID, approvedAmount int64) claimContinueResponse {
	t.Helper()

	url := fmt.Sprintf("/v1/claims/%d/confirm-continue", claimID)
	rec := doJSON(t, server, http.MethodPost, url, nil, customerID)
	require.Equal(t, http.StatusOK, rec.Code)

	var continueResp claimContinueResponse
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &continueResp)
	requireAcceptedClaimContinue(t, continueResp, claimID, approvedAmount)
	return continueResp
}

type claimPayoutActionDetailSnapshot struct {
	ClaimID           int64  `json:"claim_id"`
	RecoveryDisputeID int64  `json:"recovery_dispute_id"`
	UserID            int64  `json:"user_id"`
	Amount            int64  `json:"amount"`
	OutBillNo         string `json:"out_bill_no"`
	TransferBillNo    string `json:"transfer_bill_no"`
	TransferState     string `json:"transfer_state"`
	LastError         string `json:"last_error"`
	TerminalFailure   bool   `json:"terminal_failure"`
}

func newFinishedClaimPayoutPaymentClient(t *testing.T, store *db.SQLStore, userID, amount int64, transferRemark, batchRemark string) *mockwechat.MockTransferClientInterface {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	user, err := store.GetUser(context.Background(), userID)
	require.NoError(t, err)

	mockPaymentClient := mockwechat.NewMockTransferClientInterface(ctrl)
	batchID := "batch-integration"
	outBatchNo := ""
	mockPaymentClient.EXPECT().GetAppID().AnyTimes().Return("integration-app")

	mockPaymentClient.EXPECT().
		CreateTransfer(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *wechatcontracts.DirectMerchantTransferCreateRequest) (*wechatcontracts.DirectMerchantTransferCreateResponse, error) {
			require.Equal(t, amount, req.TransferAmount)
			require.Equal(t, user.WechatOpenid, req.OpenID)
			require.Equal(t, user.FullName, req.UserName)
			require.Equal(t, transferRemark, req.TransferRemark)
			outBatchNo = req.OutBillNo
			_ = batchRemark
			return &wechatcontracts.DirectMerchantTransferCreateResponse{
				OutBillNo:      req.OutBillNo,
				TransferBillNo: batchID,
				State:          wechatcontracts.DirectMerchantTransferStateAccepted,
				CreateTime:     "2026-03-27T10:00:00+08:00",
			}, nil
		})

	mockPaymentClient.EXPECT().
		QueryTransferByOutBillNo(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, gotOutBillNo string) (*wechatcontracts.DirectMerchantTransferQueryResponse, error) {
			require.NotEmpty(t, outBatchNo)
			require.Equal(t, outBatchNo, gotOutBillNo)
			return &wechatcontracts.DirectMerchantTransferQueryResponse{
				MchID:          "integration-mch",
				AppID:          "integration-app",
				OutBillNo:      gotOutBillNo,
				TransferBillNo: batchID,
				State:          wechatcontracts.DirectMerchantTransferStateSuccess,
				TransferAmount: amount,
				TransferRemark: transferRemark,
				CreateTime:     "2026-03-27T10:00:00+08:00",
				UpdateTime:     "2026-03-27T10:01:00+08:00",
			}, nil
		})

	return mockPaymentClient
}

func newClaimRecoveryPaymentClient(t *testing.T, store *db.SQLStore, userID, amount int64, description string) *mockwechat.MockDirectPaymentClientInterface {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	user, err := store.GetUser(context.Background(), userID)
	require.NoError(t, err)

	mockPaymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	mockPaymentClient.EXPECT().
		CreateJSAPIOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(ctx context.Context, req *wechatcontracts.DirectJSAPIOrderRequest) (*wechatcontracts.DirectJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
			require.Equal(t, amount, req.TotalAmount)
			require.Equal(t, user.WechatOpenid, req.PayerOpenID)
			require.Equal(t, description, req.Description)
			require.NotEmpty(t, req.OutTradeNo)
			require.NotEmpty(t, req.Attach)
			return &wechatcontracts.DirectJSAPIOrderResponse{PrepayID: "prepay_claim_recovery_integration"}, &wechat.JSAPIPayParams{
				TimeStamp: "1711526400",
				NonceStr:  "claim_recovery_nonce",
				Package:   "prepay_id=prepay_claim_recovery_integration",
				SignType:  "RSA",
				PaySign:   "claim_recovery_sign",
			}, nil
		})

	return mockPaymentClient
}

func lookupPayoutActionForClaimOrRecoveryDispute(store *db.SQLStore, orderID, claimID, recoveryDisputeID int64) (db.BehaviorAction, claimPayoutActionDetailSnapshot, bool, error) {
	decisions, err := store.ListBehaviorDecisionsByOrder(context.Background(), pgtype.Int8{Int64: orderID, Valid: true})
	if err != nil {
		return db.BehaviorAction{}, claimPayoutActionDetailSnapshot{}, false, err
	}
	if len(decisions) == 0 {
		return db.BehaviorAction{}, claimPayoutActionDetailSnapshot{}, false, nil
	}

	for _, decision := range decisions {
		actions, actionErr := store.ListBehaviorActionsByDecision(context.Background(), decision.ID)
		if actionErr != nil {
			return db.BehaviorAction{}, claimPayoutActionDetailSnapshot{}, false, actionErr
		}
		for _, action := range actions {
			if action.ActionType != "payout" || action.TargetEntity != "user" {
				continue
			}
			var detail claimPayoutActionDetailSnapshot
			if err := json.Unmarshal(action.Detail, &detail); err != nil {
				return db.BehaviorAction{}, claimPayoutActionDetailSnapshot{}, false, err
			}
			if recoveryDisputeID > 0 && detail.RecoveryDisputeID == recoveryDisputeID {
				return action, detail, true, nil
			}
			if recoveryDisputeID == 0 && detail.ClaimID == claimID {
				return action, detail, true, nil
			}
		}
	}

	return db.BehaviorAction{}, claimPayoutActionDetailSnapshot{}, false, nil
}

func findPayoutActionForClaimOrRecoveryDispute(t *testing.T, store *db.SQLStore, orderID, claimID, recoveryDisputeID int64) (db.BehaviorAction, claimPayoutActionDetailSnapshot) {
	t.Helper()

	action, detail, found, err := lookupPayoutActionForClaimOrRecoveryDispute(store, orderID, claimID, recoveryDisputeID)
	require.NoError(t, err)
	require.Truef(t, found, "payout action not found for order=%d claim=%d recovery_dispute=%d", orderID, claimID, recoveryDisputeID)
	return action, detail
}

func requireNoPayoutActionForClaimOrRecoveryDispute(t *testing.T, store *db.SQLStore, orderID, claimID, recoveryDisputeID int64) {
	t.Helper()

	_, _, found, err := lookupPayoutActionForClaimOrRecoveryDispute(store, orderID, claimID, recoveryDisputeID)
	require.NoError(t, err)
	require.Falsef(t, found, "unexpected payout action found for order=%d claim=%d recovery_dispute=%d", orderID, claimID, recoveryDisputeID)
}

func completeClaimPayoutForClaim(t *testing.T, store *db.SQLStore, claimID, userID, amount int64) {
	t.Helper()

	ctx := context.Background()
	claim, err := store.GetClaim(ctx, claimID)
	require.NoError(t, err)

	payoutAction, _ := findPayoutActionForClaimOrRecoveryDispute(t, store, claim.OrderID, claimID, 0)
	mockPaymentClient := newFinishedClaimPayoutPaymentClient(t, store, userID, amount, "platform payout", "claim payout")
	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	processor.SetTransferClient(mockPaymentClient)
	payloadBytes, err := json.Marshal(worker.ClaimPayoutPayload{ActionID: payoutAction.ID})
	require.NoError(t, err)
	require.NoError(t, processor.ProcessTaskClaimPayout(ctx, asynq.NewTask(worker.TaskClaimPayout, payloadBytes)))
}

func processClaimBehaviorAction(t *testing.T, store *db.SQLStore, actionID int64) {
	t.Helper()

	payloadBytes, err := json.Marshal(worker.ClaimBehaviorActionPayload{ActionID: actionID})
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	task := asynq.NewTask(worker.TaskClaimBehaviorAction, payloadBytes)
	require.NoError(t, processor.ProcessTaskClaimBehaviorAction(context.Background(), task))
}

func triggerClaimRecoveryOverdueViaScheduler(t *testing.T, store *db.SQLStore, recoveryID int64) int64 {
	t.Helper()

	_, err := integrationPool.Exec(context.Background(), `UPDATE claim_recoveries SET due_at = $2 WHERE id = $1`, recoveryID, time.Now().Add(-2*time.Minute))
	require.NoError(t, err)

	distributor := &captureClaimBehaviorActionDistributor{}
	s := worker.NewClaimRecoveryScheduler(store, distributor)
	s.RunOnce()

	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)
	processClaimBehaviorAction(t, store, payloads[0].ActionID)
	return payloads[0].ActionID
}

type recoveryDisputeSubmitResponse struct {
	ID            int64  `json:"id"`
	ClaimID       int64  `json:"claim_id"`
	AppellantType string `json:"appellant_type"`
	AppellantID   int64  `json:"appellant_id"`
	Status        string `json:"status"`
}

type claimRecoveryStatusResponse struct {
	ID             int64   `json:"id"`
	Status         string  `json:"status"`
	RecoveryTarget *string `json:"recovery_target"`
}

type recoveryPayParamsResponse struct {
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

type claimRecoveryPaymentCreateResponse struct {
	Recovery       claimRecoveryStatusResponse `json:"recovery"`
	PaymentOrderID int64                       `json:"payment_order_id"`
	OutTradeNo     string                      `json:"out_trade_no"`
	Amount         int64                       `json:"amount"`
	Status         string                      `json:"status"`
	ExpiresAt      *time.Time                  `json:"expires_at"`
	PayParams      *recoveryPayParamsResponse  `json:"pay_params"`
}

func applyClaimRecoveryPaymentFact(t *testing.T, store *db.SQLStore, paymentOrderID int64, transactionID string) {
	t.Helper()

	updatedPayment, err := store.UpdatePaymentOrderToPaid(context.Background(), db.UpdatePaymentOrderToPaidParams{
		ID:            paymentOrderID,
		TransactionID: pgtype.Text{String: transactionID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, db.ExternalPaymentBusinessOwnerClaimRecovery, updatedPayment.BusinessType)

	consumer := "claim_recovery_domain"
	businessObjectType := "payment_order"
	businessOwner := db.ExternalPaymentBusinessOwnerClaimRecovery
	factResult, err := logic.NewPaymentFactService(store).RecordExternalPaymentFact(context.Background(), logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		FactSource:           db.ExternalPaymentFactSourceManualReconciliation,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    updatedPayment.OutTradeNo,
		ExternalSecondaryKey: &transactionID,
		BusinessOwner:        &businessOwner,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &paymentOrderID,
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               &updatedPayment.Amount,
		Currency:             "CNY",
		RawResource: []byte(fmt.Sprintf(
			`{"payment_order_id":%d,"business_type":"claim_recovery","transaction_id":%q}`,
			updatedPayment.ID,
			transactionID,
		)),
		DedupeKey: fmt.Sprintf("integration:claim_recovery_payment:%d:%s", updatedPayment.ID, transactionID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           consumer,
			BusinessObjectType: businessObjectType,
			BusinessObjectID:   paymentOrderID,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, factResult.Application)

	distributor := &captureClaimBehaviorActionDistributor{}
	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payloadBytes, err := json.Marshal(worker.PaymentFactApplicationPayload{ApplicationID: factResult.Application.ID})
	require.NoError(t, err)
	require.NoError(t, processor.ProcessTaskPaymentFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessPaymentFactApplication, payloadBytes)))

	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)
	processClaimBehaviorAction(t, store, payloads[0].ActionID)
}

func claimRecoveryEventTypesForIntegration(t *testing.T, store *db.SQLStore, recoveryID int64) []string {
	t.Helper()

	events, err := store.ListClaimRecoveryEventsByRecovery(context.Background(), recoveryID)
	require.NoError(t, err)
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.EventType)
	}
	return types
}

func claimRecoveryIDForIntegration(t *testing.T, store *db.SQLStore, claimID int64) int64 {
	t.Helper()

	recovery, err := store.GetClaimRecoveryByClaimID(context.Background(), claimID)
	require.NoError(t, err)
	return recovery.ID
}

type integrationBaofuAggregateClient struct {
	lastUnified aggregatecontracts.UnifiedOrderRequest
}

func (c *integrationBaofuAggregateClient) CreateUnifiedOrder(_ context.Context, req aggregatecontracts.UnifiedOrderRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	c.lastUnified = req
	return &aggregatecontracts.UnifiedOrderResult{
		MerchantID: req.MerchantID,
		TerminalID: req.TerminalID,
		OutTradeNo: req.OutTradeNo,
		TradeNo:    "BFPAY_" + util.RandomString(10),
		ResultCode: aggregatecontracts.BusinessResultCodeSuccess,
		ChannelReturn: aggregatecontracts.ChannelReturn{
			WechatPayData: json.RawMessage(`{"timeStamp":"1767225600","nonceStr":"nonce-baofu-integration","package":"prepay_id=baofu-integration","signType":"RSA","paySign":"pay-sign-baofu-integration"}`),
		},
	}, nil
}

func (c *integrationBaofuAggregateClient) QueryPayment(context.Context, aggregatecontracts.PaymentQueryRequest) (*aggregatecontracts.UnifiedOrderResult, error) {
	return nil, errors.New("not implemented in takeout journey integration test")
}

func (c *integrationBaofuAggregateClient) CreateProfitSharing(context.Context, aggregatecontracts.ShareAfterPayRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, errors.New("not implemented in takeout journey integration test")
}

func (c *integrationBaofuAggregateClient) QueryProfitSharing(context.Context, aggregatecontracts.ShareQueryRequest) (*aggregatecontracts.ShareResult, error) {
	return nil, errors.New("not implemented in takeout journey integration test")
}

func (c *integrationBaofuAggregateClient) CreateRefund(_ context.Context, req aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error) {
	return &aggregatecontracts.RefundResult{
		OriginTradeNo:    req.OriginTradeNo,
		OriginOutTradeNo: req.OriginOutTradeNo,
		OutTradeNo:       req.OutTradeNo,
		TradeNo:          "BFREFUND_" + util.RandomString(10),
		RefundAmountFen:  req.RefundAmountFen,
		TotalAmountFen:   req.TotalAmountFen,
		ResultCode:       aggregatecontracts.BusinessResultCodeSuccess,
		RefundState:      aggregatecontracts.RefundStateAccepted,
	}, nil
}

func (c *integrationBaofuAggregateClient) QueryRefund(context.Context, aggregatecontracts.RefundQueryRequest) (*aggregatecontracts.RefundResult, error) {
	return nil, errors.New("not implemented in takeout journey integration test")
}

func (c *integrationBaofuAggregateClient) CloseOrder(_ context.Context, req aggregatecontracts.OrderCloseRequest) (*aggregatecontracts.OrderCloseResult, error) {
	return &aggregatecontracts.OrderCloseResult{
		MerchantID: req.MerchantID,
		TerminalID: req.TerminalID,
		OutTradeNo: req.OutTradeNo,
		TradeNo:    req.TradeNo,
		ResultCode: aggregatecontracts.BusinessResultCodeSuccess,
	}, nil
}

func configureIntegrationBaofuAggregate(t *testing.T, server *api.Server) *integrationBaofuAggregateClient {
	t.Helper()

	client := &integrationBaofuAggregateClient{}
	server.SetBaofuAggregateClientForTest(client, logic.BaofuAggregateFacadeConfig{
		CollectMerchantID: "102004465",
		CollectTerminalID: "200005200",
		MiniProgramAppID:  "wx-integration-app",
		PaymentNotifyURL:  "https://api.example.com/v1/webhooks/baofu/payment",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})
	server.SetBaofuAggregatePaymentNotificationParserForTest(baofuaggregatenotification.NewParser())
	return client
}

func postIntegrationBaofuPaymentNotify(t *testing.T, server *api.Server, notificationID, outTradeNo string, amount int64) *httptest.ResponseRecorder {
	t.Helper()

	body := map[string]any{
		"notifyId":   notificationID,
		"notifyType": "PAYMENT",
		"merId":      "102004465",
		"terId":      "200005200",
		"outTradeNo": outTradeNo,
		"tradeNo":    "BFPAY_" + util.RandomString(10),
		"txnState":   "SUCCESS",
		"payCode":    aggregatecontracts.PayCodeWechatJSAPI,
		"succAmt":    amount,
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/baofu/payment", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func postIntegrationBaofuRefundNotify(t *testing.T, server *api.Server, notificationID, outRefundNo, refundID string, amount int64) *httptest.ResponseRecorder {
	t.Helper()

	body := map[string]any{
		"notifyId":    notificationID,
		"notifyType":  "REFUND",
		"merId":       "102004465",
		"terId":       "200005200",
		"outTradeNo":  outRefundNo,
		"tradeNo":     refundID,
		"refundState": aggregatecontracts.RefundStateSuccess,
		"resultCode":  aggregatecontracts.BusinessResultCodeSuccess,
		"txnTime":     time.Now().UTC().Format("20060102150405"),
		"succAmt":     amount,
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/baofu/refund", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func ensureIntegrationMerchantBaofuReady(t *testing.T, store *db.SQLStore, merchantID int64, subMchID string) {
	t.Helper()

	ctx := context.Background()
	binding, err := store.UpsertBaofuAccountBinding(ctx, db.UpsertBaofuAccountBindingParams{
		OwnerType:             db.BaofuAccountOwnerTypeMerchant,
		OwnerID:               merchantID,
		AccountType:           db.BaofuAccountTypeBusiness,
		LoginNo:               pgtype.Text{String: "login_" + util.RandomString(8), Valid: true},
		OpenState:             db.BaofuAccountOpenStateProcessing,
		WechatSubMchID:        pgtype.Text{String: subMchID, Valid: true},
		LastOpenTransSerialNo: pgtype.Text{String: "OPEN_" + util.RandomString(8), Valid: true},
		RawSnapshot:           []byte(`{}`),
	})
	require.NoError(t, err)
	_, err = store.MarkBaofuAccountBindingActive(ctx, db.MarkBaofuAccountBindingActiveParams{
		ID:           binding.ID,
		ContractNo:   pgtype.Text{String: "BCT_CONTRACT_" + util.RandomString(8), Valid: true},
		SharingMerID: pgtype.Text{String: "BCT_SHARING_" + util.RandomString(8), Valid: true},
		RawSnapshot:  []byte(`{}`),
	})
	require.NoError(t, err)
	report, err := store.UpsertBaofuMerchantReportProcessing(ctx, db.UpsertBaofuMerchantReportProcessingParams{
		OwnerType:   db.BaofuAccountOwnerTypeMerchant,
		OwnerID:     merchantID,
		ReportType:  db.BaofuMerchantReportTypeWechat,
		ReportNo:    "BMR_" + util.RandomString(10),
		BctMerID:    "BCT_MER_" + util.RandomString(8),
		RawSnapshot: []byte(`{}`),
	})
	require.NoError(t, err)
	_, err = store.MarkBaofuMerchantReportSucceeded(ctx, db.MarkBaofuMerchantReportSucceededParams{
		ID:            report.ID,
		SubMchID:      pgtype.Text{String: subMchID, Valid: true},
		PlatformBizNo: pgtype.Text{String: "PLAT_" + util.RandomString(8), Valid: true},
		RawSnapshot:   []byte(`{}`),
	})
	require.NoError(t, err)
	_, err = store.MarkBaofuMerchantReportAppletAuthSucceeded(ctx, report.ID)
	require.NoError(t, err)
}

func ensureIntegrationRiderBaofuReady(t *testing.T, store *db.SQLStore, riderID int64) {
	t.Helper()

	ctx := context.Background()
	binding, err := store.UpsertBaofuAccountBinding(ctx, db.UpsertBaofuAccountBindingParams{
		OwnerType:             db.BaofuAccountOwnerTypeRider,
		OwnerID:               riderID,
		AccountType:           db.BaofuAccountTypePersonal,
		LoginNo:               pgtype.Text{String: "login_" + util.RandomString(8), Valid: true},
		OpenState:             db.BaofuAccountOpenStateProcessing,
		WechatSubMchID:        pgtype.Text{String: "sub_mch_rider_" + util.RandomString(8), Valid: true},
		LastOpenTransSerialNo: pgtype.Text{String: "OPEN_" + util.RandomString(8), Valid: true},
		RawSnapshot:           []byte(`{}`),
	})
	require.NoError(t, err)
	_, err = store.MarkBaofuAccountBindingActive(ctx, db.MarkBaofuAccountBindingActiveParams{
		ID:           binding.ID,
		ContractNo:   pgtype.Text{String: "BCT_CONTRACT_" + util.RandomString(8), Valid: true},
		SharingMerID: pgtype.Text{String: "BCT_RIDER_" + util.RandomString(8), Valid: true},
		RawSnapshot:  []byte(`{}`),
	})
	require.NoError(t, err)
}

func ensureIntegrationPlatformBaofuReady(t *testing.T, store *db.SQLStore) {
	t.Helper()

	ctx := context.Background()
	binding, err := store.UpsertBaofuAccountBinding(ctx, db.UpsertBaofuAccountBindingParams{
		OwnerType:             db.BaofuAccountOwnerTypePlatform,
		OwnerID:               0,
		AccountType:           db.BaofuAccountTypeBusiness,
		LoginNo:               pgtype.Text{String: "login_" + util.RandomString(8), Valid: true},
		OpenState:             db.BaofuAccountOpenStateProcessing,
		WechatSubMchID:        pgtype.Text{String: "sub_mch_platform_" + util.RandomString(8), Valid: true},
		LastOpenTransSerialNo: pgtype.Text{String: "OPEN_" + util.RandomString(8), Valid: true},
		RawSnapshot:           []byte(`{}`),
	})
	require.NoError(t, err)
	_, err = store.MarkBaofuAccountBindingActive(ctx, db.MarkBaofuAccountBindingActiveParams{
		ID:           binding.ID,
		ContractNo:   pgtype.Text{String: "BCT_CONTRACT_" + util.RandomString(8), Valid: true},
		SharingMerID: pgtype.Text{String: "PLATFORM_SHARE_" + util.RandomString(8), Valid: true},
		RawSnapshot:  []byte(`{}`),
	})
	require.NoError(t, err)
}

func clearIntegrationBaofuFeeTables(t *testing.T) {
	t.Helper()

	if integrationPool == nil {
		return
	}
	ctx := context.Background()
	_, err := integrationPool.Exec(ctx, `DELETE FROM order_payment_fee_ledgers`)
	require.NoError(t, err)
	_, err = integrationPool.Exec(ctx, `DELETE FROM baofu_fee_ledger`)
	require.NoError(t, err)
}

func processCapturedPaymentFactApplicationPayload(t *testing.T, store *db.SQLStore, payload worker.PaymentFactApplicationPayload) {
	t.Helper()

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	task := asynq.NewTask(worker.TaskProcessPaymentFactApplication, payloadBytes)
	require.NoError(t, processor.ProcessTaskPaymentFactApplication(context.Background(), task))
}

func processPaymentSuccessTxForIntegration(t *testing.T, store *db.SQLStore, paymentOrderID int64) {
	t.Helper()
	_, err := store.ProcessPaymentSuccessTx(context.Background(), db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     paymentOrderID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
}

type roomAvailabilitySlot struct {
	Time      string `json:"time"`
	Available bool   `json:"available"`
}

type roomAvailabilityResponse struct {
	RoomID    int64                  `json:"room_id"`
	Date      string                 `json:"date"`
	TimeSlots []roomAvailabilitySlot `json:"time_slots"`
}

type scanTableResponse struct {
	Merchant struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	} `json:"merchant"`
	Table struct {
		ID      int64  `json:"id"`
		TableNo string `json:"table_no"`
	} `json:"table"`
}

type deliveryRecommendResponse struct {
	OrderID    int64 `json:"order_id"`
	MerchantID int64 `json:"merchant_id"`
}

type listTodayReservationsResponse struct {
	Reservations []reservationStatusResponse `json:"reservations"`
}

type cartResponse struct {
	ID         int64 `json:"id"`
	MerchantID int64 `json:"merchant_id"`
	TotalCount int   `json:"total_count"`
	Subtotal   int64 `json:"subtotal"`
}

type calculateCartResponse struct {
	Subtotal       int64 `json:"subtotal"`
	DeliveryFee    int64 `json:"delivery_fee"`
	TotalAmount    int64 `json:"total_amount"`
	MinOrderAmount int64 `json:"min_order_amount"`
}

// TestTakeoutJourneyB1Integration
// 外卖旅程（B1）端到端验收：下单 -> 支付成功推进 -> 商户接单/出餐 -> 骑手抢单/取餐/代取/送达 -> 用户确认完成。
//
// 说明：支付回调与异步 worker 在 integration harness 中未配置（taskDistributor=nil），
// 这里用 store 事务直接模拟“支付成功后置处理”：
// - UpdatePaymentOrderToPaid
// - ProcessPaymentSuccessTx
func TestTakeoutJourneyB1Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)

	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)
	_, err = store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_b1_001",
		Status:     "active",
	})
	require.NoError(t, err)
	configureIntegrationBaofuAggregate(t, server)
	ensureIntegrationMerchantBaofuReady(t, store, merchant.ID, "sub_mch_b1_001")

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)
	customerRow, err := store.GetUser(ctx, customer.ID)
	require.NoError(t, err)
	if customerRow.WechatOpenid == "" {
		_, err = integrationPool.Exec(ctx, `UPDATE users SET wechat_openid = $2 WHERE id = $1`, customer.ID, "wx_openid_"+util.RandomString(8))
		require.NoError(t, err)
	}

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	ensureIntegrationRiderBaofuReady(t, store, rider.ID)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) C端下单：/v1/orders
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
		require.Equal(t, "takeout", created.OrderType)
	}
	orderID := created.ID

	// 2) 创建支付单：/v1/payments
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "miniprogram",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
		require.Equal(t, "order", payment.BusinessType)
	}

	// 3) 模拟支付成功后置处理（仅置 paid，不创建 delivery，也不立即入池）
	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	paidOrder, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "paid", paidOrder.Status)

	_, err = store.GetDeliveryByOrderID(ctx, orderID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)
	_, err = store.GetDeliveryPoolByOrderID(ctx, orderID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)

	// 3) 商户接单：/v1/merchant/orders/:id/accept
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutOrderResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "preparing", resp.Status)
	}

	_, err = store.GetDeliveryByOrderID(ctx, orderID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)
	_, err = store.GetDeliveryPoolByOrderID(ctx, orderID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)

	// 4) 商户出餐完成：/v1/merchant/orders/:id/ready
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutOrderResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "ready", resp.Status)
	}

	poolItem, err := store.GetDeliveryPoolByOrderID(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, orderID, poolItem.OrderID)
	delivery, err := store.GetDeliveryByOrderID(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, orderID, delivery.OrderID)

	// 5) 骑手抢单：/v1/delivery/grab/:order_id
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutDeliveryResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, delivery.ID, resp.ID)
		require.Equal(t, "assigned", resp.Status)
	}

	delivery, err = store.GetDeliveryByOrderID(ctx, orderID)
	require.NoError(t, err)
	require.True(t, delivery.RiderID.Valid)
	require.Equal(t, rider.ID, delivery.RiderID.Int64)

	// 6) 骑手开始取餐：/v1/delivery/:delivery_id/start-pickup
	{
		url := fmt.Sprintf("/v1/delivery/%d/start-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutDeliveryResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "picking", resp.Status)
	}

	// 7) 骑手确认取餐：/v1/delivery/:delivery_id/confirm-pickup
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutDeliveryResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "picked", resp.Status)
	}

	// 8) 骑手开始代取：/v1/delivery/:delivery_id/start-delivery
	{
		url := fmt.Sprintf("/v1/delivery/%d/start-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutDeliveryResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "delivering", resp.Status)
	}

	// 9) 骑手上报当前位置：/v1/rider/location
	{
		body := map[string]any{
			"region_id": region.ID,
			"locations": []map[string]any{
				{
					"delivery_id": delivery.ID,
					"longitude":   116.3975,
					"latitude":    39.9084,
					"recorded_at": time.Now().UTC().Format(time.RFC3339),
					"source":      "integration_confirm_delivery",
				},
			},
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/location", body, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 10) 骑手确认送达：/v1/delivery/:delivery_id/confirm-delivery
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutDeliveryResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "delivered", resp.Status)
	}

	o, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "rider_delivered", o.Status)

	// 11) 用户确认收货：/v1/orders/:id/confirm
	{
		url := fmt.Sprintf("/v1/orders/%d/confirm", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutOrderResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "completed", resp.Status)
	}

	// 关键落库断言：订单完成、押金解冻
	o, err = store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", o.Status)

	updatedRider, err := store.GetRider(ctx, rider.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), updatedRider.FrozenDeposit)
}

// TestTakeoutJourneyB1WebhookIntegration
// 外卖旅程（B1）端到端验收：走宝付聚合支付回调路径 + 任务入队，再由 worker 处理支付成功推进。
func TestTakeoutJourneyB1WebhookIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)

	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)
	_, err = store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_b1_webhook_001",
		Status:     "active",
	})
	require.NoError(t, err)
	configureIntegrationBaofuAggregate(t, server)
	ensureIntegrationMerchantBaofuReady(t, store, merchant.ID, "sub_mch_b1_webhook_001")

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)
	customerRow, err := store.GetUser(ctx, customer.ID)
	require.NoError(t, err)
	if customerRow.WechatOpenid == "" {
		_, err = integrationPool.Exec(ctx, `UPDATE users SET wechat_openid = $2 WHERE id = $1`, customer.ID, "wx_openid_"+util.RandomString(8))
		require.NoError(t, err)
	}

	// 1) C端下单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}
	orderID := created.ID

	// 2) 创建支付单
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "miniprogram",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	paymentOrder, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	// 3) 走宝付聚合支付回调 + 任务入队
	distributor := &capturePaymentSuccessDistributor{}
	server.SetTaskDistributorForTest(distributor)
	defer server.SetTaskDistributorForTest(nil)

	notificationID := "notify_" + util.RandomString(8)
	rec := postIntegrationBaofuPaymentNotify(t, server, notificationID, paymentOrder.OutTradeNo, paymentOrder.Amount)
	require.Equal(t, http.StatusOK, rec.Code)

	updatedPayment, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", updatedPayment.Status)

	// 5) 运行支付成功任务，推进订单与代取单（不立即入池）
	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)

	processCapturedPaymentFactApplicationPayload(t, store, payloads[0])

	updatedPayment, err = store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", updatedPayment.Status)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "paid", order.Status)

	_, err = store.GetDeliveryByOrderID(ctx, orderID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)
	_, err = store.GetDeliveryPoolByOrderID(ctx, orderID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)
}

// TestTakeoutJourneyB0DeliveryRecommendIntegration
// 外卖旅程（B0）端到端验收：骑手推荐订单列表包含待代取订单。
func TestTakeoutJourneyB0DeliveryRecommendIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)
	_, err = store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_b7_001",
		Status:     "active",
	})
	require.NoError(t, err)
	configureIntegrationBaofuAggregate(t, server)
	ensureIntegrationMerchantBaofuReady(t, store, merchant.ID, "sub_mch_b7_001")
	configureIntegrationBaofuAggregate(t, server)
	ensureIntegrationMerchantBaofuReady(t, store, merchant.ID, "sub_mch_b7_001")

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}
	orderID := created.ID

	// 2) 直接落库支付单并模拟支付成功，避免 legacy native payment API 漂移
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("b0_recommend_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_recommend_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	_, err = store.GetDeliveryByOrderID(ctx, orderID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)
	_, err = store.GetDeliveryPoolByOrderID(ctx, orderID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)

	// 3) 商户接单并出餐后才进入代取池
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	_, err = store.GetDeliveryPoolByOrderID(ctx, orderID)
	require.NoError(t, err)

	// 4) 推荐订单列表
	url := "/v1/delivery/recommend?longitude=116.397&latitude=39.908"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp []deliveryRecommendResponse
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.NotEmpty(t, resp)

	found := false
	for _, item := range resp {
		if item.OrderID == orderID {
			found = true
			break
		}
	}
	require.True(t, found)
}

// TestTakeoutJourneyB0CartCalculateIntegration
// 外卖旅程（B0）端到端验收：购物车加购与试算。
func TestTakeoutJourneyB0CartCalculateIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	// 1) 加购
	addBody := map[string]any{
		"merchant_id": merchant.ID,
		"order_type":  "takeout",
		"dish_id":     dish.ID,
		"quantity":    2,
	}
	var cart cartResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/cart/items", addBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &cart)
		require.Equal(t, merchant.ID, cart.MerchantID)
		require.Equal(t, 2, cart.TotalCount)
		require.Greater(t, cart.Subtotal, int64(0))
	}

	// 2) 查询购物车
	{
		url := fmt.Sprintf("/v1/cart?merchant_id=%d&order_type=takeout", merchant.ID)
		rec := doGET(t, server, url, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var got cartResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &got)
		require.Equal(t, cart.ID, got.ID)
		require.Equal(t, 2, got.TotalCount)
	}

	// 3) 试算
	calcBody := map[string]any{
		"merchant_id": merchant.ID,
		"order_type":  "takeout",
	}
	var calc calculateCartResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/cart/calculate", calcBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &calc)
		require.Greater(t, calc.Subtotal, int64(0))
		require.GreaterOrEqual(t, calc.TotalAmount, calc.Subtotal)
	}
}

// TestTakeoutJourneyB4PaymentOrderTimeoutIntegration
// 外卖旅程（B4）支付单超时兜底：payment_order:timeout 关闭支付单并取消待支付订单。
func TestTakeoutJourneyB4PaymentOrderTimeoutIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)
	customerRow, err := store.GetUser(ctx, customer.ID)
	require.NoError(t, err)
	if customerRow.WechatOpenid == "" {
		_, err = integrationPool.Exec(ctx, `UPDATE users SET wechat_openid = $2 WHERE id = $1`, customer.ID, "wx_openid_"+util.RandomString(8))
		require.NoError(t, err)
	}

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 直接落库待支付单，覆盖超时关闭链路而不依赖旧 native 支付 API
	orderForPayment, err := store.GetOrder(ctx, created.ID)
	require.NoError(t, err)

	po, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: created.ID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("b4_order_%d_%d", created.ID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	// 3) 回拨支付单过期时间，触发 payment_order:timeout
	_, err = integrationPool.Exec(ctx, `UPDATE payment_orders SET expires_at = $2 WHERE id = $1`, po.ID, time.Now().Add(-10*time.Minute))
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	payloadBytes, err := json.Marshal(worker.PayloadPaymentOrderTimeout{PaymentOrderNo: po.OutTradeNo})
	require.NoError(t, err)
	if err := processor.ProcessTaskPaymentOrderTimeout(ctx, asynq.NewTask(worker.TaskPaymentOrderTimeout, payloadBytes)); err != nil {
		require.NoError(t, err)
	}

	updatedPO, err := store.GetPaymentOrder(ctx, po.ID)
	require.NoError(t, err)
	require.Equal(t, "closed", updatedPO.Status)

	updatedOrder, err := store.GetOrder(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", updatedOrder.Status)
}

// TestTakeoutJourneyB7MerchantRejectRefundIntegration
// 外卖旅程（B7）异常链路：商户拒单触发退款处理。
func TestTakeoutJourneyB7MerchantRejectRefundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)
	_, err = store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_b7_001",
		Status:     "active",
	})
	require.NoError(t, err)
	configureIntegrationBaofuAggregate(t, server)
	ensureIntegrationMerchantBaofuReady(t, store, merchant.ID, "sub_mch_b7_001")

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)
	customerRow, err := store.GetUser(ctx, customer.ID)
	require.NoError(t, err)
	if customerRow.WechatOpenid == "" {
		_, err = integrationPool.Exec(ctx, `UPDATE users SET wechat_openid = $2 WHERE id = $1`, customer.ID, "wx_openid_"+util.RandomString(8))
		require.NoError(t, err)
	}

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}
	orderID := created.ID

	// 2) 创建支付单并模拟支付成功
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "miniprogram",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_reject_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	latestPayment, err := store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: orderID, Valid: true},
		BusinessType: "order",
	})
	require.NoError(t, err)
	if latestPayment.Status != "paid" {
		_, err := integrationPool.Exec(ctx, `UPDATE payment_orders SET status = 'paid' WHERE id = $1`, latestPayment.ID)
		require.NoError(t, err)
	}

	// 3) 商户拒单
	{
		body := map[string]any{"reason": "out_of_stock"}
		url := fmt.Sprintf("/v1/merchant/orders/%d/reject", orderID)
		rec := doJSON(t, server, http.MethodPost, url, body, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp takeoutOrderResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "cancelled", resp.Status)
	}

	updatedOrder, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", updatedOrder.Status)

	_, err = store.GetDeliveryByOrderID(ctx, orderID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)
	_, err = store.GetDeliveryPoolByOrderID(ctx, orderID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)

	updatedPayment, err := store.GetPaymentOrder(ctx, latestPayment.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", updatedPayment.Status)

	refunds, err := store.ListRefundOrdersByPaymentOrder(ctx, latestPayment.ID)
	require.NoError(t, err)
	require.NotEmpty(t, refunds)
	refund := refunds[0]
	require.Equal(t, "processing", refund.Status)
	require.NotEmpty(t, refund.OutRefundNo)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	payloadBytes, err := json.Marshal(worker.RefundResultPayload{
		OutRefundNo:  refund.OutRefundNo,
		RefundStatus: "SUCCESS",
		RefundID:     "refund_order_reject_001",
	})
	require.NoError(t, err)
	require.NoError(t, processor.ProcessTaskRefundResult(ctx, asynq.NewTask(worker.TaskProcessRefundResult, payloadBytes)))

	refundedPayment, err := store.GetPaymentOrder(ctx, latestPayment.ID)
	require.NoError(t, err)
	require.Equal(t, "refunded", refundedPayment.Status)

	updatedRefund, err := store.GetRefundOrder(ctx, refund.ID)
	require.NoError(t, err)
	require.Equal(t, "success", updatedRefund.Status)
	require.Equal(t, "refund_order_reject_001", updatedRefund.RefundID.String)
}

// TestTakeoutJourneyB5PaymentRecoveryIntegration
// 外卖旅程（B5）回调丢失补偿：payment recovery scheduler 扫描 paid 未处理支付单并入队。
func TestTakeoutJourneyB5PaymentRecoveryIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建支付单并标记为 paid，但不执行 ProcessPaymentSuccessTx
	orderForPayment, err := store.GetOrder(ctx, created.ID)
	require.NoError(t, err)

	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: created.ID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("b5_order_%d_%d", created.ID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_recovery_001", Valid: true},
	})
	require.NoError(t, err)

	// backdate paid_at to pass recovery min age
	_, err = integrationPool.Exec(ctx, `UPDATE payment_orders SET paid_at = $2 WHERE id = $1`, payment.ID, time.Now().Add(-5*time.Minute))
	require.NoError(t, err)

	// 3) 触发 recovery scheduler 并断言 fact application 入队
	d := &capturePaymentFactApplicationDistributor{}
	recovery := worker.NewPaymentRecoveryScheduler(store, d)
	recovery.RunOnce()

	payloads := d.Payloads()
	require.Len(t, payloads, 1)
	require.NotZero(t, payloads[0].ApplicationID)
}

// TestTakeoutJourneyB6OrderPaymentTimeoutIntegration
// 外卖旅程（B6）订单支付超时兜底：order:payment_timeout 取消 pending 订单。
func TestTakeoutJourneyB6OrderPaymentTimeoutIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单（pending）
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 回拨创建时间，触发订单支付超时
	backTo := time.Now().Add(-time.Duration(worker.OrderPaymentTimeoutMinutes) * time.Minute).Add(-2 * time.Minute)
	_, err = integrationPool.Exec(ctx, `UPDATE orders SET created_at = $2 WHERE id = $1`, created.ID, backTo)
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	payloadBytes, err := json.Marshal(worker.PayloadOrderPaymentTimeout{OrderID: created.ID})
	require.NoError(t, err)
	if err := processor.ProcessTaskOrderPaymentTimeout(ctx, asynq.NewTask(worker.TaskOrderPaymentTimeout, payloadBytes)); err != nil {
		require.NoError(t, err)
	}

	updatedOrder, err := store.GetOrder(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", updatedOrder.Status)
}

// TestDineInJourneyA0ScanTableIntegration
// 堂食旅程（A0）端到端验收：扫码入口返回商户与桌台信息。
func TestDineInJourneyA0ScanTableIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	createIntegrationDish(t, store, merchant.ID)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{Valid: false})
	customer := createIntegrationUser(t, store)

	url := fmt.Sprintf("/v1/scan/table?merchant_id=%d&table_no=%s", merchant.ID, table.TableNo)
	rec := doGET(t, server, url, customer.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp scanTableResponse
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.Equal(t, merchant.ID, resp.Merchant.ID)
	require.Equal(t, table.ID, resp.Table.ID)
	require.Equal(t, table.TableNo, resp.Table.TableNo)
}

// TestDineInJourneyA1Integration
// 堂食旅程（A1）端到端验收：开台 -> 下单 -> 支付回调 -> 厨房制作/出餐 -> 结账离店。
func TestDineInJourneyA1Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{String: accessHash, Valid: true})

	// 1) 开台
	var openResp dineInSessionResponse
	{
		body := map[string]any{
			"table_id":   table.ID,
			"table_code": accessCode,
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/dining-sessions/open", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &openResp)
		require.NotZero(t, openResp.Session.ID)
	}

	// 2) 下单（堂食）
	createBody := map[string]any{
		"merchant_id": merchant.ID,
		"order_type":  "dine_in",
		"table_id":    table.ID,
		"items":       []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
		require.Equal(t, "dine_in", created.OrderType)
	}
	orderID := created.ID

	// 3) 直接落库支付单，避免旧 native 支付 API 兼容路径影响堂食链路
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	po, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("a1_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	// 4) 走支付回调 + 任务入队
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	mockPaymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	mockPaymentClient.EXPECT().
		DecryptPaymentNotification(gomock.Any()).
		Times(1).
		Return(&wechatcontracts.DirectPaymentNotificationResource{
			TransactionID: "integration_tx_dinein_001",
			OutTradeNo:    po.OutTradeNo,
			TradeState:    "SUCCESS",
			Amount: wechatcontracts.DirectOrderQueryAmount{
				Total:         po.Amount,
				PayerTotal:    po.Amount,
				Currency:      "CNY",
				PayerCurrency: "CNY",
			},
		}, nil)

	distributor := &capturePaymentSuccessDistributor{}
	server.SetDirectPaymentClientForTest(mockPaymentClient)
	server.SetTaskDistributorForTest(distributor)
	defer server.SetDirectPaymentClientForTest(nil)
	defer server.SetTaskDistributorForTest(nil)

	notificationID := "notify_dinein_" + util.RandomString(8)
	body := map[string]any{
		"id":            notificationID,
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "mock_ciphertext",
			"nonce":           "mock_nonce",
			"associated_data": "transaction",
			"original_type":   "transaction",
		},
		"summary": "success",
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Wechatpay-Timestamp", "1234567890")
	req.Header.Set("Wechatpay-Nonce", "test_nonce")
	req.Header.Set("Wechatpay-Signature", "test_signature")
	req.Header.Set("Wechatpay-Serial", "test_serial")

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)

	// 5) 处理支付成功任务
	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)

	payloadBytes, err := json.Marshal(payloads[0])
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	task := asynq.NewTask(worker.TaskProcessPaymentFactApplication, payloadBytes)
	require.NoError(t, processor.ProcessTaskPaymentFactApplication(ctx, task))

	paidOrder, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "paid", paidOrder.Status)

	// 6) 厨房开始制作/出餐
	{
		url := fmt.Sprintf("/v1/kitchen/orders/%d/preparing", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp kitchenOrderStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "preparing", resp.Status)
	}
	{
		url := fmt.Sprintf("/v1/kitchen/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp kitchenOrderStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "ready", resp.Status)
	}

	// 7) 结账离店
	{
		url := fmt.Sprintf("/v1/dining-sessions/%d/checkout", openResp.Session.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	updatedSession, err := store.GetDiningSession(ctx, openResp.Session.ID)
	require.NoError(t, err)
	require.Equal(t, "closed", updatedSession.Status)

	updatedTable, err := store.GetTable(ctx, table.ID)
	require.NoError(t, err)
	require.Equal(t, "available", updatedTable.Status)
}

// TestDineInJourneyA2InvalidTableCodeIntegration
// 堂食旅程（A2）端到端验收：桌台码错误拒绝开台。
func TestDineInJourneyA2InvalidTableCodeIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{String: accessHash, Valid: true})
	customer := createIntegrationUser(t, store)

	body := map[string]any{
		"table_id":   table.ID,
		"table_code": "9999",
	}
	rec := doJSON(t, server, http.MethodPost, "/v1/dining-sessions/open", body, customer.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestDineInJourneyA3MerchantOpenWithoutReservationIntegration
// 堂食旅程（A3）端到端验收：商户无预订不能代客开台。
func TestDineInJourneyA3MerchantOpenWithoutReservationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{String: accessHash, Valid: true})

	body := map[string]any{
		"table_id": table.ID,
	}
	rec := doJSON(t, server, http.MethodPost, "/v1/dining-sessions/open", body, merchantOwner.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestDineInJourneyA4CheckoutForbiddenIntegration
// 堂食旅程（A4）端到端验收：非商户用户结账被拒绝。
func TestDineInJourneyA4CheckoutForbiddenIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{String: accessHash, Valid: true})
	customer := createIntegrationUser(t, store)

	// 1) 客户开台
	var openResp dineInSessionResponse
	{
		body := map[string]any{
			"table_id":   table.ID,
			"table_code": accessCode,
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/dining-sessions/open", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &openResp)
		require.NotZero(t, openResp.Session.ID)
	}

	// 2) 客户尝试结账
	{
		url := fmt.Sprintf("/v1/dining-sessions/%d/checkout", openResp.Session.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, customer.ID)
		require.Equal(t, http.StatusForbidden, rec.Code)
	}
}

// TestDineInJourneyA5OrderPaymentTimeoutIntegration
// 堂食旅程（A5）端到端验收：堂食订单支付超时自动取消。
func TestDineInJourneyA5OrderPaymentTimeoutIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{String: accessHash, Valid: true})

	// 1) 开台
	{
		body := map[string]any{
			"table_id":   table.ID,
			"table_code": accessCode,
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/dining-sessions/open", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 2) 创建堂食订单（pending）
	createBody := map[string]any{
		"merchant_id": merchant.ID,
		"order_type":  "dine_in",
		"table_id":    table.ID,
		"items":       []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
		require.Equal(t, "dine_in", created.OrderType)
	}

	// 3) 回拨创建时间，触发订单支付超时
	backTo := time.Now().Add(-time.Duration(worker.OrderPaymentTimeoutMinutes) * time.Minute).Add(-2 * time.Minute)
	_, err = integrationPool.Exec(ctx, `UPDATE orders SET created_at = $2 WHERE id = $1`, created.ID, backTo)
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	payloadBytes, err := json.Marshal(worker.PayloadOrderPaymentTimeout{OrderID: created.ID})
	require.NoError(t, err)
	if err := processor.ProcessTaskOrderPaymentTimeout(ctx, asynq.NewTask(worker.TaskOrderPaymentTimeout, payloadBytes)); err != nil {
		require.NoError(t, err)
	}

	updatedOrder, err := store.GetOrder(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", updatedOrder.Status)
}

// TestReservationJourneyCAvailabilityIntegration
// 包间预订可用性（C1）端到端验收：营业时段内有可用时段，已有预订时段不可用。
func TestReservationJourneyCAvailabilityIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour)
	dateStr := reservationDate.Format("2006-01-02")
	dayOfWeek := int32(reservationDate.Weekday())

	openTime := pgtype.Time{Microseconds: int64(18*3600) * 1000000, Valid: true}
	closeTime := pgtype.Time{Microseconds: int64(21*3600) * 1000000, Valid: true}
	_, err = store.CreateBusinessHour(ctx, db.CreateBusinessHourParams{
		MerchantID:  merchant.ID,
		DayOfWeek:   dayOfWeek,
		OpenTime:    openTime,
		CloseTime:   closeTime,
		IsClosed:    false,
		SpecialDate: pgtype.Date{Valid: false},
	})
	require.NoError(t, err)

	availabilityURL := fmt.Sprintf("/v1/rooms/%d/availability?date=%s", room.ID, dateStr)
	{
		rec := doGET(t, server, availabilityURL, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp roomAvailabilityResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, room.ID, resp.RoomID)
		require.Equal(t, dateStr, resp.Date)
		require.NotEmpty(t, resp.TimeSlots)

		found := false
		for _, slot := range resp.TimeSlots {
			if slot.Time == "19:00" {
				require.True(t, slot.Available)
				found = true
				break
			}
		}
		require.True(t, found)
	}

	reservationDateTime := time.Date(reservationDate.Year(), reservationDate.Month(), reservationDate.Day(), 19, 0, 0, 0, time.Local)
	_, err = store.CreateReservationTx(ctx, db.CreateReservationTxParams{
		CreateTableReservationParams: db.CreateTableReservationParams{
			TableID:         room.ID,
			UserID:          customer.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: int64(19*3600) * 1000000, Valid: true},
			GuestCount:      2,
			ContactName:     "张三",
			ContactPhone:    "13800138001",
			PaymentMode:     "deposit",
			DepositAmount:   10000,
			PrepaidAmount:   0,
			RefundDeadline:  reservationDateTime.Add(-2 * time.Hour),
			PaymentDeadline: time.Now().Add(30 * time.Minute),
			Notes:           pgtype.Text{Valid: false},
			Status:          "pending",
		},
	})
	require.NoError(t, err)

	{
		rec := doGET(t, server, availabilityURL, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp roomAvailabilityResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		blocked := false
		for _, slot := range resp.TimeSlots {
			if slot.Time == "19:00" {
				require.False(t, slot.Available)
				blocked = true
				break
			}
		}
		require.True(t, blocked)
	}
}

// TestReservationJourneyCCheckInPendingIntegration
// 包间预订异常链路（C-CheckIn-Pending）：未支付预订签到被拒。
func TestReservationJourneyCCheckInPendingIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDateTime := time.Now().Add(20 * time.Minute)
	reservationDate := reservationDateTime.Format("2006-01-02")
	reservationTime := reservationDateTime.Format("15:04")

	// 1) 创建预订（未支付）
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 未支付直接签到
	{
		url := fmt.Sprintf("/v1/reservations/%d/checkin", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, customer.ID)
		require.Equal(t, http.StatusConflict, rec.Code)
	}
}

// TestReservationJourneyC1Integration
// 包间预订旅程（C1）端到端验收：创建预订 -> 支付回调 -> 商户确认 -> 顾客签到 -> 完结。
func TestReservationJourneyC1Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	_, err = store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_reservation_c1",
		Status:     "active",
	})
	require.NoError(t, err)
	configureIntegrationBaofuAggregate(t, server)
	ensureIntegrationMerchantBaofuReady(t, store, merchant.ID, "sub_mch_reservation_c1")

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订（定金模式）
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建预订支付单（宝付聚合支付）
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "miniprogram",
			"business_type": "reservation",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	// 3) 走宝付聚合支付回调 + 任务入队
	distributor := &capturePaymentSuccessDistributor{}
	server.SetTaskDistributorForTest(distributor)
	defer server.SetTaskDistributorForTest(nil)

	notificationID := "notify_reservation_" + util.RandomString(8)
	rec := postIntegrationBaofuPaymentNotify(t, server, notificationID, po.OutTradeNo, po.Amount)
	require.Equal(t, http.StatusOK, rec.Code)

	// 4) 处理支付成功任务
	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)

	payloadBytes, err := json.Marshal(payloads[0])
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	task := asynq.NewTask(worker.TaskProcessPaymentFactApplication, payloadBytes)
	require.NoError(t, processor.ProcessTaskPaymentFactApplication(ctx, task))

	paidReservation, err := store.GetTableReservation(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", paidReservation.Status)

	checkinTime := time.Now().Add(5 * time.Minute)
	_, err = store.UpdateReservation(ctx, db.UpdateReservationParams{
		ID:              created.ID,
		ReservationDate: pgtype.Date{Time: checkinTime, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: int64(checkinTime.Hour()*3600+checkinTime.Minute()*60) * 1000000, Valid: true},
	})
	require.NoError(t, err)

	// 5) 商户确认预订
	{
		url := fmt.Sprintf("/v1/reservations/%d/confirm", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "confirmed", resp.Status)
	}

	// 6) 顾客签到
	{
		url := fmt.Sprintf("/v1/reservations/%d/checkin", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "checked_in", resp.Status)
	}

	// 7) 商户完结
	{
		url := fmt.Sprintf("/v1/reservations/%d/complete", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "completed", resp.Status)
	}

	updatedTable, err := store.GetTable(ctx, room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", updatedTable.Status)
}

// TestReservationJourneyC2StartCookingIntegration
// 包间预订旅程（C2）端到端验收：商户对已确认预订起菜通知。
func TestReservationJourneyC2StartCookingIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	dish := createIntegrationDish(t, store, merchant.ID)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "full",
			"items":         []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	_, err = store.UpdateReservationStatus(ctx, db.UpdateReservationStatusParams{
		ID:     created.ID,
		Status: "confirmed",
	})
	require.NoError(t, err)

	// 2) 商户起菜通知
	{
		url := fmt.Sprintf("/v1/reservations/%d/start-cooking", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "confirmed", resp.Status)
	}
}

// TestReservationJourneyCTodayListIntegration
// 包间预订旅程（C3）端到端验收：商户获取今日预订列表。
func TestReservationJourneyCTodayListIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now()
	reservationTime := time.Date(reservationDate.Year(), reservationDate.Month(), reservationDate.Day(), 19, 0, 0, 0, time.Local)

	created, err := store.CreateReservationTx(ctx, db.CreateReservationTxParams{
		CreateTableReservationParams: db.CreateTableReservationParams{
			TableID:         room.ID,
			UserID:          customer.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: int64(19*3600) * 1000000, Valid: true},
			GuestCount:      2,
			ContactName:     "张三",
			ContactPhone:    "13800138001",
			PaymentMode:     "deposit",
			DepositAmount:   10000,
			PrepaidAmount:   0,
			RefundDeadline:  reservationTime.Add(-2 * time.Hour),
			PaymentDeadline: time.Now().Add(30 * time.Minute),
			Notes:           pgtype.Text{Valid: false},
			Status:          "confirmed",
		},
	})
	require.NoError(t, err)

	url := "/v1/reservations/merchant/today"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp listTodayReservationsResponse
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.NotEmpty(t, resp.Reservations)

	found := false
	for _, r := range resp.Reservations {
		if r.ID == created.Reservation.ID {
			found = true
			break
		}
	}
	require.True(t, found)
}

// TestReservationJourneyCPaymentTimeoutIntegration
// 包间预订异常链路（C-Timeout）：支付超时后自动取消预订。
func TestReservationJourneyCPaymentTimeoutIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	_, err = integrationPool.Exec(ctx,
		`UPDATE table_reservations SET refund_deadline = $2 WHERE id = $1`,
		created.ID,
		time.Now().Add(2*time.Hour),
	)
	require.NoError(t, err)

	// 2) 回写支付截止时间为过去，触发超时处理
	_, err = integrationPool.Exec(ctx,
		`UPDATE table_reservations SET payment_deadline = $2 WHERE id = $1`,
		created.ID,
		time.Now().Add(-5*time.Minute),
	)
	require.NoError(t, err)

	payloadBytes, err := json.Marshal(worker.PayloadReservationPaymentTimeout{ReservationID: created.ID})
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	task := asynq.NewTask(worker.TaskReservationPaymentTimeout, payloadBytes)
	require.NoError(t, processor.ProcessTaskReservationPaymentTimeout(ctx, task))

	updatedReservation, err := store.GetTableReservation(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", updatedReservation.Status)
}

// TestReservationJourneyCNoShowIntegration
// 包间预订异常链路（C-NoShow）：商户标记爽约后释放桌台。
func TestReservationJourneyCNoShowIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	_, err = store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_reservation_noshow",
		Status:     "active",
	})
	require.NoError(t, err)
	configureIntegrationBaofuAggregate(t, server)
	ensureIntegrationMerchantBaofuReady(t, store, merchant.ID, "sub_mch_reservation_noshow")

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建预订支付单（宝付聚合支付）
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "miniprogram",
			"business_type": "reservation",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	// 3) 走宝付聚合支付回调 + 任务入队
	distributor := &capturePaymentSuccessDistributor{}
	server.SetTaskDistributorForTest(distributor)
	defer server.SetTaskDistributorForTest(nil)

	notificationID := "notify_reservation_noshow_" + util.RandomString(8)
	rec := postIntegrationBaofuPaymentNotify(t, server, notificationID, po.OutTradeNo, po.Amount)
	require.Equal(t, http.StatusOK, rec.Code)

	// 4) 处理支付成功任务
	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)

	payloadBytes, err := json.Marshal(payloads[0])
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	task := asynq.NewTask(worker.TaskProcessPaymentFactApplication, payloadBytes)
	require.NoError(t, processor.ProcessTaskPaymentFactApplication(ctx, task))

	// 5) 商户确认预订
	{
		url := fmt.Sprintf("/v1/reservations/%d/confirm", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 6) 商户标记未到店
	{
		url := fmt.Sprintf("/v1/reservations/%d/no-show", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "no_show", resp.Status)
	}

	updatedTable, err := store.GetTable(ctx, room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", updatedTable.Status)
}

// TestReservationJourneyCCancelRefundIntegration
// 包间预订异常链路（C-Cancel）：退款截止前取消预订触发退款。
func TestReservationJourneyCCancelRefundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	_, err = store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_reservation_cancel",
		Status:     "active",
	})
	require.NoError(t, err)
	configureIntegrationBaofuAggregate(t, server)
	ensureIntegrationMerchantBaofuReady(t, store, merchant.ID, "sub_mch_reservation_cancel")

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建预订支付单（宝付聚合支付）
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "miniprogram",
			"business_type": "reservation",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	// 3) 直接标记支付成功（该路径在 C2-C5 已覆盖）
	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            po.ID,
		TransactionID: pgtype.Text{String: "tx_cancel_001", Valid: true},
	})
	require.NoError(t, err)
	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     po.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	currentReservation, err := store.GetTableReservation(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", currentReservation.Status)
	require.True(t, time.Now().Before(currentReservation.RefundDeadline))
	require.Greater(t, currentReservation.PrepaidAmount, int64(0))

	currentPayment, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", currentPayment.Status)
	require.Greater(t, currentPayment.Amount, int64(0))

	// 5) 取消预订（退款截止前）
	{
		body := map[string]any{
			"reason": "changed_plan",
		}
		url := fmt.Sprintf("/v1/reservations/%d/cancel", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "cancelled", resp.Status)
	}

	updatedPayment, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", updatedPayment.Status)

	refunds, err := store.ListRefundOrdersByPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)
	require.NotEmpty(t, refunds)
	refund := refunds[0]
	require.Equal(t, "pending", refund.Status)
	require.NotEmpty(t, refund.OutRefundNo)

	distributor := &capturePaymentFactApplicationDistributor{}
	server.SetTaskDistributorForTest(distributor)
	defer server.SetTaskDistributorForTest(nil)

	notificationID := "refund_notify_" + util.RandomString(8)
	rec := postIntegrationBaofuRefundNotify(t, server, notificationID, refund.OutRefundNo, "refund_notify_id_001", currentPayment.Amount)
	require.Equal(t, http.StatusOK, rec.Code)

	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)
	processCapturedPaymentFactApplicationPayload(t, store, payloads[0])

	refundedPayment, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)
	require.Equal(t, "refunded", refundedPayment.Status)

	updatedRefund, err := store.GetRefundOrder(ctx, refund.ID)
	require.NoError(t, err)
	require.Equal(t, "success", updatedRefund.Status)
	require.Equal(t, "refund_notify_id_001", updatedRefund.RefundID.String)

	updatedReservation, err := store.GetTableReservation(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", updatedReservation.Status)
	require.Equal(t, int64(0), updatedReservation.PrepaidAmount)
}

// TestReservationJourneyCCancelAfterDeadlineIntegration
// 包间预订异常链路（C-Cancel-Deadline）：退款截止后用户取消会被拒绝。
func TestReservationJourneyCCancelAfterDeadlineIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	_, err = store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_reservation_cancel_deadline",
		Status:     "active",
	})
	require.NoError(t, err)
	configureIntegrationBaofuAggregate(t, server)
	ensureIntegrationMerchantBaofuReady(t, store, merchant.ID, "sub_mch_reservation_cancel_deadline")

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建预订支付单（宝付聚合支付）
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "miniprogram",
			"business_type": "reservation",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            po.ID,
		TransactionID: pgtype.Text{String: "tx_cancel_deadline_001", Valid: true},
	})
	require.NoError(t, err)
	_, err = store.UpdateReservationStatus(ctx, db.UpdateReservationStatusParams{
		ID:     created.ID,
		Status: "paid",
	})
	require.NoError(t, err)

	_, err = integrationPool.Exec(ctx,
		`UPDATE table_reservations SET refund_deadline = $2 WHERE id = $1`,
		created.ID,
		time.Now().Add(-10*time.Minute),
	)
	require.NoError(t, err)

	// 3) 退款截止后取消
	{
		body := map[string]any{
			"reason": "too_late",
		}
		url := fmt.Sprintf("/v1/reservations/%d/cancel", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, body, customer.ID)
		require.Equal(t, http.StatusConflict, rec.Code)
	}

	updatedReservation, err := store.GetTableReservation(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", updatedReservation.Status)
}

// TestReservationJourneyCRefundNotifyIntegration
// 包间预订异常链路（C-Refund-Notify）：退款回调通知入队退款结果处理。
func TestReservationJourneyCRefundNotifyIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	_, err = store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_reservation_refund_notify",
		Status:     "active",
	})
	require.NoError(t, err)
	configureIntegrationBaofuAggregate(t, server)
	ensureIntegrationMerchantBaofuReady(t, store, merchant.ID, "sub_mch_reservation_refund_notify")

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建预订支付单（宝付聚合支付）并标记已支付
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "miniprogram",
			"business_type": "reservation",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            po.ID,
		TransactionID: pgtype.Text{String: "tx_refund_notify_001", Valid: true},
	})
	require.NoError(t, err)
	_, err = store.UpdateReservationStatus(ctx, db.UpdateReservationStatusParams{
		ID:     created.ID,
		Status: "paid",
	})
	require.NoError(t, err)

	outRefundNo := "refund_notify_" + util.RandomString(8)
	_, err = store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
		PaymentOrderID: po.ID,
		RefundType:     "miniprogram",
		RefundAmount:   po.Amount,
		RefundReason:   pgtype.Text{String: "预定取消退款", Valid: true},
		OutRefundNo:    outRefundNo,
		Status:         "processing",
	})
	require.NoError(t, err)

	// 3) 退款回调通知（宝付聚合渠道）
	distributor := &capturePaymentFactApplicationDistributor{}
	server.SetTaskDistributorForTest(distributor)
	defer server.SetTaskDistributorForTest(nil)

	notificationID := "refund_notify_" + util.RandomString(8)
	rec := postIntegrationBaofuRefundNotify(t, server, notificationID, outRefundNo, "refund_notify_id_001", po.Amount)
	require.Equal(t, http.StatusOK, rec.Code)

	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)
}

// TestRiderDepositRefundCallbackAccountingIntegration
// 骑手押金退款回调落账链路：押金支付成功 -> 提现冻结 -> 退款结果任务成功结算。
func TestRiderDepositRefundCallbackAccountingIntegration(t *testing.T) {
	_, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)

	_, err := store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)

	paymentOrder, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		UserID:         rider.UserID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "rider_deposit",
		Amount:         30000,
		OutTradeNo:     "rd_refund_notify_" + util.RandomString(10),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: "tx_rider_refund_notify_001", Valid: true},
	})
	require.NoError(t, err)

	payResult, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID: paymentOrder.ID,
	})
	require.NoError(t, err)
	require.True(t, payResult.Processed)

	prepareResult, err := store.PrepareRiderDepositRefundTx(ctx, db.PrepareRiderDepositRefundTxParams{
		RiderID: rider.ID,
		Amount:  30000,
		Remark:  "骑手押金提现",
	})
	require.NoError(t, err)
	require.Len(t, prepareResult.RefundPlans, 1)

	refundOrder := prepareResult.RefundPlans[0].RefundOrder
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payloadBytes, err := json.Marshal(&worker.RefundResultPayload{
		OutRefundNo:  refundOrder.OutRefundNo,
		RefundStatus: "SUCCESS",
		RefundID:     "rider_refund_notify_id_001",
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessRefundResult, payloadBytes)
	err = processor.ProcessTaskRefundResult(ctx, task)
	require.NoError(t, err)

	updatedRefund, err := store.GetRefundOrder(ctx, refundOrder.ID)
	require.NoError(t, err)
	require.Equal(t, "success", updatedRefund.Status)
	require.Equal(t, "rider_refund_notify_id_001", updatedRefund.RefundID.String)

	updatedRider, err := store.GetRider(ctx, rider.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), updatedRider.DepositAmount)
	require.Equal(t, int64(0), updatedRider.FrozenDeposit)

	credit, err := store.GetRiderDepositCreditByPaymentOrderID(ctx, paymentOrder.ID)
	require.NoError(t, err)
	require.Equal(t, "fully_refunded", credit.Status)
	require.Equal(t, int64(0), credit.RefundableAmount)
	require.Equal(t, int64(30000), credit.RefundedAmount)

	updatedPaymentOrder, err := store.GetPaymentOrder(ctx, paymentOrder.ID)
	require.NoError(t, err)
	require.Equal(t, "refunded", updatedPaymentOrder.Status)

	deposits, err := store.ListRiderDeposits(ctx, db.ListRiderDepositsParams{
		RiderID: rider.ID,
		Limit:   20,
		Offset:  0,
	})
	require.NoError(t, err)
	require.Len(t, deposits, 3)
	types := []string{deposits[0].Type, deposits[1].Type, deposits[2].Type}
	require.ElementsMatch(t, []string{"deposit", "freeze", "withdraw"}, types)
}

// TestClaimJourneyD1Integration
// 索赔旅程（D1）端到端验收：完成订单后提交索赔、顾客确认继续、平台完成赔付。
func TestClaimJourneyD1Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}

	claim, err := store.GetClaim(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, orderID, claim.OrderID)
	require.False(t, claim.PaidAt.Valid)
	require.Equal(t, db.ClaimStatusWaitingCustomerConfirmation, claim.Status)
	requireNoPayoutActionForClaimOrRecoveryDispute(t, store, orderID, claim.ID, 0)

	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)

	claim, err = store.GetClaim(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, db.ClaimStatusApproved, claim.Status)
	require.False(t, claim.PaidAt.Valid)

	payoutAction, payoutDetail := findPayoutActionForClaimOrRecoveryDispute(t, store, orderID, claim.ID, 0)
	require.Equal(t, "created", payoutAction.Status)
	require.Equal(t, customer.ID, payoutDetail.UserID)
	require.Equal(t, order.TotalAmount, payoutDetail.Amount)

	_, err = store.GetClaimRecoveryByClaimID(ctx, claim.ID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)

	completeClaimPayoutForClaim(t, store, claim.ID, customer.ID, order.TotalAmount)

	claim, err = store.GetClaim(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.True(t, claim.PaidAt.Valid)

	payoutAction, payoutDetail = findPayoutActionForClaimOrRecoveryDispute(t, store, orderID, claim.ID, 0)
	require.Equal(t, "success", payoutAction.Status)
	require.Equal(t, wechatcontracts.DirectMerchantTransferStateSuccess, payoutDetail.TransferState)
	require.NotEmpty(t, payoutDetail.OutBillNo)
	require.Equal(t, "batch-integration", payoutDetail.TransferBillNo)
	require.False(t, payoutDetail.TerminalFailure)
	require.Empty(t, payoutDetail.LastError)

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claim.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)
}

// TestClaimJourneyD2MerchantRecoveryDisputeIntegration
// 索赔旅程（D2）端到端验收：顾客确认继续后商户发起追偿争议，系统自动复核维持原判并恢复追偿。
func TestClaimJourneyD2MerchantRecoveryDisputeIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 4) 商户发起追偿争议
	var recoveryDisputeResp recoveryDisputeSubmitResponse
	{
		recoveryDisputeBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/recovery-disputes", recoveryDisputeBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryDisputeResp)
		require.NotZero(t, recoveryDisputeResp.ID)
		require.Equal(t, claimResp.ClaimID, recoveryDisputeResp.ClaimID)
		require.Equal(t, "merchant", recoveryDisputeResp.AppellantType)
		require.Equal(t, merchant.ID, recoveryDisputeResp.AppellantID)
		require.Equal(t, "rejected", recoveryDisputeResp.Status)
	}

	recoveryDispute, err := store.GetRecoveryDisputeByClaim(ctx, db.GetRecoveryDisputeByClaimParams{
		ClaimID:       claimResp.ClaimID,
		AppellantType: "merchant",
	})
	require.NoError(t, err)
	require.Equal(t, recoveryDisputeResp.ID, recoveryDispute.ID)
	require.Equal(t, "rejected", recoveryDispute.Status)
	require.True(t, recoveryDispute.ReviewedAt.Valid)

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)
}

// TestClaimJourneyD3RiderRecoveryDisputeIntegration
// 索赔旅程（D3）端到端验收：骑手追偿争议提交后系统自动复核并撤销错误追责。
func TestClaimJourneyD3RiderRecoveryDisputeIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	// 2) 直接落库支付单并模拟支付成功，避免 legacy native payment API 漂移影响 rider appeal 旅程
	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("d3_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d3_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立代取关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 7) 骑手发起追偿争议
	var recoveryDisputeResp recoveryDisputeSubmitResponse
	{
		recoveryDisputeBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "delivery handled properly with no issues",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/recovery-disputes", recoveryDisputeBody, riderUser.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryDisputeResp)
		require.NotZero(t, recoveryDisputeResp.ID)
		require.Equal(t, claimResp.ClaimID, recoveryDisputeResp.ClaimID)
		require.Equal(t, "rider", recoveryDisputeResp.AppellantType)
		require.Equal(t, rider.ID, recoveryDisputeResp.AppellantID)
		require.Equal(t, "approved", recoveryDisputeResp.Status)
	}

	recoveryDispute, err := store.GetRecoveryDisputeByClaim(ctx, db.GetRecoveryDisputeByClaimParams{
		ClaimID:       claimResp.ClaimID,
		AppellantType: "rider",
	})
	require.NoError(t, err)
	require.Equal(t, recoveryDisputeResp.ID, recoveryDispute.ID)
	require.Equal(t, "approved", recoveryDispute.Status)
	require.True(t, recoveryDispute.ReviewedAt.Valid)
}

// TestClaimJourneyD10MerchantRecoveryPayIntegration
// 索赔旅程（D10）端到端验收：商户创建追偿支付单并在支付成功后完成追偿。
func TestClaimJourneyD10MerchantRecoveryPayIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}

	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)
	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)
	require.True(t, recovery.RecoveryTarget.Valid)
	require.Equal(t, "merchant", recovery.RecoveryTarget.String)
	_, err = store.CreateMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)
	triggerClaimRecoveryOverdueViaScheduler(t, store, recovery.ID)
	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "overdue", recovery.Status)
	merchantProfile, err := store.GetMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)
	require.True(t, merchantProfile.IsTakeoutSuspended)
	require.True(t, merchantProfile.TakeoutSuspendReason.Valid)
	eventTypes := claimRecoveryEventTypesForIntegration(t, store, recovery.ID)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeOverdue)

	server.SetDirectPaymentClientForTest(newClaimRecoveryPaymentClient(t, store, merchantOwner.ID, recovery.RecoveryAmount, "商户索赔追偿支付"))
	defer server.SetDirectPaymentClientForTest(nil)

	// 4) 商户支付追偿单
	var payResp claimRecoveryPaymentCreateResponse
	{
		url := fmt.Sprintf("/v1/merchant/recoveries/%d/pay", recovery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payResp)
		require.Equal(t, "pending", payResp.Status)
		require.NotZero(t, payResp.PaymentOrderID)
		require.NotNil(t, payResp.PayParams)
	}

	applyClaimRecoveryPaymentFact(t, store, payResp.PaymentOrderID, "integration_tx_d10_001")

	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "paid", recovery.Status)
	merchantProfile, err = store.GetMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)
	require.False(t, merchantProfile.IsTakeoutSuspended)
	require.False(t, merchantProfile.TakeoutSuspendReason.Valid)
	eventTypes = claimRecoveryEventTypesForIntegration(t, store, recovery.ID)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeOverdue)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypePaymentStarted)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypePaid)
	require.Equal(t, db.ClaimRecoveryEventTypeClosed, eventTypes[len(eventTypes)-1])
}

// TestClaimJourneyD12RiderRecoveryPayIntegration
// 索赔旅程（D12）端到端验收：骑手创建追偿支付单并在支付成功后恢复接单。
func TestClaimJourneyD12RiderRecoveryPayIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	// 2) 直接落库支付单并模拟支付成功，避免 legacy native payment API 漂移影响 rider recovery 旅程
	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("d12_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d12_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立代取关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔（餐损类使 rider 为追偿对象）
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "damage",
			"claim_amount": order.TotalAmount,
			"claim_reason": "food was damaged during delivery",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)
	require.True(t, recovery.RecoveryTarget.Valid)
	require.Equal(t, "rider", recovery.RecoveryTarget.String)
	triggerClaimRecoveryOverdueViaScheduler(t, store, recovery.ID)
	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "overdue", recovery.Status)
	riderProfile, err := store.GetRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	require.True(t, riderProfile.IsSuspended)
	require.True(t, riderProfile.SuspendReason.Valid)
	eventTypes := claimRecoveryEventTypesForIntegration(t, store, recovery.ID)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeOverdue)

	server.SetDirectPaymentClientForTest(newClaimRecoveryPaymentClient(t, store, riderUser.ID, recovery.RecoveryAmount, "骑手索赔追偿支付"))
	defer server.SetDirectPaymentClientForTest(nil)

	// 7) 骑手支付追偿单
	var payResp claimRecoveryPaymentCreateResponse
	{
		url := fmt.Sprintf("/v1/rider/recoveries/%d/pay", recovery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payResp)
		require.Equal(t, "pending", payResp.Status)
		require.NotZero(t, payResp.PaymentOrderID)
		require.NotNil(t, payResp.PayParams)
	}

	applyClaimRecoveryPaymentFact(t, store, payResp.PaymentOrderID, "integration_tx_d12_recovery_001")

	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "paid", recovery.Status)
	riderProfile, err = store.GetRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	require.False(t, riderProfile.IsSuspended)
	require.False(t, riderProfile.SuspendReason.Valid)
	eventTypes = claimRecoveryEventTypesForIntegration(t, store, recovery.ID)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeOverdue)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypePaymentStarted)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypePaid)
	require.Equal(t, db.ClaimRecoveryEventTypeClosed, eventTypes[len(eventTypes)-1])
}

// TestClaimJourneyD13OperatorRecoveryViewIntegration
// 索赔旅程（D13）端到端验收：运营商查看追偿单详情。
func TestClaimJourneyD13OperatorRecoveryViewIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 4) 运营商查看追偿单
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/operator/recoveries/%d", claimRecoveryIDForIntegration(t, store, claimResp.ClaimID))
		rec := doGET(t, server, url, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "pending", recoveryResp.Status)
		require.NotNil(t, recoveryResp.RecoveryTarget)
		require.Equal(t, "merchant", *recoveryResp.RecoveryTarget)
	}
}

// TestClaimJourneyD14MerchantRecoveryViewIntegration
// 索赔旅程（D14）端到端验收：商户查看追偿单详情。
func TestClaimJourneyD14MerchantRecoveryViewIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 4) 商户查看追偿单
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/merchant/recoveries/%d", claimRecoveryIDForIntegration(t, store, claimResp.ClaimID))
		rec := doGET(t, server, url, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "pending", recoveryResp.Status)
		require.NotNil(t, recoveryResp.RecoveryTarget)
		require.Equal(t, "merchant", *recoveryResp.RecoveryTarget)
	}
}

// TestClaimJourneyD15RiderRecoveryViewIntegration
// 索赔旅程（D15）端到端验收：骑手查看追偿单详情。
func TestClaimJourneyD15RiderRecoveryViewIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	// 2) 直接落库支付单并模拟支付成功，避免 legacy native payment API 漂移影响 rider recovery 旅程
	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("d15_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d15_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立代取关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔（餐损类使 rider 为追偿对象）
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "damage",
			"claim_amount": order.TotalAmount,
			"claim_reason": "food was damaged during delivery",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 7) 骑手查看追偿单
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/rider/recoveries/%d", claimRecoveryIDForIntegration(t, store, claimResp.ClaimID))
		rec := doGET(t, server, url, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "pending", recoveryResp.Status)
		require.NotNil(t, recoveryResp.RecoveryTarget)
		require.Equal(t, "rider", *recoveryResp.RecoveryTarget)
	}
}

// TestClaimJourneyD16MerchantRecoveryForbiddenIntegration
// 索赔旅程（D16）端到端验收：商户查看他人追偿单应被拒绝。
func TestClaimJourneyD16MerchantRecoveryForbiddenIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	otherOwner := createIntegrationUser(t, store)
	otherMerchant, err := store.CreateMerchant(ctx, db.CreateMerchantParams{
		OwnerUserID:     otherOwner.ID,
		Name:            "集成测试餐厅-其他",
		Description:     pgtype.Text{String: "integration", Valid: true},
		Phone:           fmt.Sprintf("139%08d", util.RandomInt(0, 99999999)),
		Address:         "测试地址-" + util.RandomString(6),
		Latitude:        pgtype.Numeric{Valid: false},
		Longitude:       pgtype.Numeric{Valid: false},
		Status:          "approved",
		ApplicationData: []byte("{}"),
		RegionID:        region.ID,
	})
	require.NoError(t, err)
	_, err = store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: otherMerchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: otherMerchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	otherMerchant = ensureIntegrationMerchantCoords(t, store, otherMerchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 4) 非所属商户查看追偿单应被拒绝
	url := fmt.Sprintf("/v1/merchant/recoveries/%d", claimRecoveryIDForIntegration(t, store, claimResp.ClaimID))
	rec := doGET(t, server, url, otherOwner.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestClaimJourneyD17RiderRecoveryForbiddenIntegration
// 索赔旅程（D17）端到端验收：骑手查看他人追偿单应被拒绝。
func TestClaimJourneyD17RiderRecoveryForbiddenIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	otherRiderUser := createIntegrationUser(t, store)
	otherRider, err := store.CreateRider(ctx, db.CreateRiderParams{
		UserID:   otherRiderUser.ID,
		RealName: "测试骑手-其他",
		IDCardNo: fmt.Sprintf("11010119900101%04d", util.RandomInt(0, 9999)),
		Phone:    fmt.Sprintf("139%08d", util.RandomInt(0, 99999999)),
		RegionID: pgtype.Int8{Int64: region.ID, Valid: true},
	})
	require.NoError(t, err)
	_, err = store.CreateRiderProfile(ctx, otherRider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: otherRider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: otherRider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: otherRider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	// 2) 直接落库支付单并模拟支付成功，避免 legacy native payment API 漂移影响 rider recovery 旅程
	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("d17_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d17_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立代取关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔（餐损类使 rider 为追偿对象）
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "damage",
			"claim_amount": order.TotalAmount,
			"claim_reason": "food was damaged during delivery",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 7) 非所属骑手查看追偿单应被拒绝
	url := fmt.Sprintf("/v1/rider/recoveries/%d", claimRecoveryIDForIntegration(t, store, claimResp.ClaimID))
	rec := doGET(t, server, url, otherRiderUser.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestClaimJourneyD18OperatorRecoveryCrossRegionForbiddenIntegration
// 索赔旅程（D18）端到端验收：跨区域运营商查看追偿单应被拒绝。
func TestClaimJourneyD18OperatorRecoveryCrossRegionForbiddenIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	otherRegion := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          otherRegion.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 4) 跨区域运营商查看追偿单应被拒绝
	url := fmt.Sprintf("/v1/operator/recoveries/%d", claimRecoveryIDForIntegration(t, store, claimResp.ClaimID))
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestClaimJourneyD19MerchantRecoveryNotFoundIntegration
// 索赔旅程（D19）端到端验收：商户查看不存在的追偿单返回 404。
func TestClaimJourneyD19MerchantRecoveryNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	url := "/v1/merchant/recoveries/999999"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestClaimJourneyD20RiderRecoveryNotFoundIntegration
// 索赔旅程（D20）端到端验收：骑手查看不存在的追偿单返回 404。
func TestClaimJourneyD20RiderRecoveryNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err := store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	url := "/v1/rider/recoveries/999999"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestClaimJourneyD21OperatorRecoveryNotFoundIntegration
// 索赔旅程（D21）端到端验收：运营商查看不存在的追偿单返回 404。
func TestClaimJourneyD21OperatorRecoveryNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	url := "/v1/operator/recoveries/999999"
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestClaimJourneyD22MerchantRecoveryViewAfterPayIntegration
// 索赔旅程（D22）端到端验收：商户支付后查看追偿单状态。
func TestClaimJourneyD22MerchantRecoveryViewAfterPayIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}

	// 4) 商户支付追偿单
	{
		confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
		completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)
		recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
		require.NoError(t, err)
		server.SetDirectPaymentClientForTest(newClaimRecoveryPaymentClient(t, store, merchantOwner.ID, recovery.RecoveryAmount, "商户索赔追偿支付"))
		defer server.SetDirectPaymentClientForTest(nil)

		url := fmt.Sprintf("/v1/merchant/recoveries/%d/pay", recovery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var payResp claimRecoveryPaymentCreateResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payResp)
		applyClaimRecoveryPaymentFact(t, store, payResp.PaymentOrderID, "integration_tx_d22_001")
	}

	// 5) 商户查看追偿单状态
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/merchant/recoveries/%d", claimRecoveryIDForIntegration(t, store, claimResp.ClaimID))
		rec := doGET(t, server, url, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "paid", recoveryResp.Status)
	}
}

// TestClaimJourneyD23RiderRecoveryViewAfterPayIntegration
// 索赔旅程（D23）端到端验收：骑手支付后查看追偿单状态。
func TestClaimJourneyD23RiderRecoveryViewAfterPayIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	// 2) 直接落库支付单并模拟支付成功，避免 legacy native payment API 漂移影响 rider recovery 旅程
	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("d23_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d23_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立代取关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔（餐损类使 rider 为追偿对象）
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "damage",
			"claim_amount": order.TotalAmount,
			"claim_reason": "food was damaged during delivery",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 7) 骑手支付追偿单
	{
		recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
		require.NoError(t, err)
		server.SetDirectPaymentClientForTest(newClaimRecoveryPaymentClient(t, store, riderUser.ID, recovery.RecoveryAmount, "骑手索赔追偿支付"))
		defer server.SetDirectPaymentClientForTest(nil)

		url := fmt.Sprintf("/v1/rider/recoveries/%d/pay", recovery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var payResp claimRecoveryPaymentCreateResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payResp)
		applyClaimRecoveryPaymentFact(t, store, payResp.PaymentOrderID, "integration_tx_d23_recovery_001")
	}

	// 8) 骑手查看追偿单状态
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/rider/recoveries/%d", claimRecoveryIDForIntegration(t, store, claimResp.ClaimID))
		rec := doGET(t, server, url, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "paid", recoveryResp.Status)
	}
}

// TestClaimJourneyD24OperatorRecoveryViewAfterWaiveIntegration
// 索赔旅程（D24）端到端验收：申诉自动通过后运营商查看追偿单状态。
func TestClaimJourneyD24OperatorRecoveryViewAfterWaiveIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)
	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	_, err = store.CreateMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)
	triggerClaimRecoveryOverdueViaScheduler(t, store, recovery.ID)
	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "overdue", recovery.Status)
	merchantProfile, err := store.GetMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)
	require.True(t, merchantProfile.IsTakeoutSuspended)
	require.True(t, merchantProfile.TakeoutSuspendReason.Valid)

	decisions, err := store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: orderID, Valid: true})
	require.NoError(t, err)
	require.NotEmpty(t, decisions)
	_, err = integrationPool.Exec(ctx, `UPDATE behavior_decisions SET effective_status = 'superseded', updated_at = NOW() WHERE id = $1`, decisions[0].ID)
	require.NoError(t, err)

	// 4) 商户申诉自动通过并撤销追偿
	{
		recoveryDisputeBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "latest behavior decision no longer points to merchant",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/recovery-disputes", recoveryDisputeBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		var recoveryDisputeResp recoveryDisputeSubmitResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryDisputeResp)
		require.Equal(t, "approved", recoveryDisputeResp.Status)
	}

	// 5) 运营商查看追偿单状态
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/operator/recoveries/%d", claimRecoveryIDForIntegration(t, store, claimResp.ClaimID))
		rec := doGET(t, server, url, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "waived", recoveryResp.Status)
	}
	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	merchantProfile, err = store.GetMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)
	require.False(t, merchantProfile.IsTakeoutSuspended)
	require.False(t, merchantProfile.TakeoutSuspendReason.Valid)
	eventTypes := claimRecoveryEventTypesForIntegration(t, store, recovery.ID)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeOverdue)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeDisputed)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeWaived)
	require.Equal(t, db.ClaimRecoveryEventTypeClosed, eventTypes[len(eventTypes)-1])
}

// TestClaimJourneyD25MerchantRecoveryDisputeDetailNotFoundIntegration
// 索赔旅程（D25）端到端验收：非所属商户查看追偿争议详情返回 404。
func TestClaimJourneyD25MerchantRecoveryDisputeDetailNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	otherOwner := createIntegrationUser(t, store)
	otherMerchant, err := store.CreateMerchant(ctx, db.CreateMerchantParams{
		OwnerUserID:     otherOwner.ID,
		Name:            "集成测试餐厅-其他",
		Description:     pgtype.Text{String: "integration", Valid: true},
		Phone:           fmt.Sprintf("139%08d", util.RandomInt(0, 99999999)),
		Address:         "测试地址-" + util.RandomString(6),
		Latitude:        pgtype.Numeric{Valid: false},
		Longitude:       pgtype.Numeric{Valid: false},
		Status:          "approved",
		ApplicationData: []byte("{}"),
		RegionID:        region.ID,
	})
	require.NoError(t, err)
	_, err = store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: otherMerchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: otherMerchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	otherMerchant = ensureIntegrationMerchantCoords(t, store, otherMerchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 4) 商户发起追偿争议
	var recoveryDisputeResp recoveryDisputeSubmitResponse
	{
		recoveryDisputeBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/recovery-disputes", recoveryDisputeBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryDisputeResp)
		require.NotZero(t, recoveryDisputeResp.ID)
	}

	// 5) 非所属商户查看追偿争议详情
	url := fmt.Sprintf("/v1/merchant/recovery-disputes/%d", recoveryDisputeResp.ID)
	rec := doGET(t, server, url, otherOwner.ID)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestClaimJourneyD26RiderRecoveryDisputeDetailNotFoundIntegration
// 索赔旅程（D26）端到端验收：非所属骑手查看追偿争议详情返回 404。
func TestClaimJourneyD26RiderRecoveryDisputeDetailNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	otherRiderUser := createIntegrationUser(t, store)
	otherRider, err := store.CreateRider(ctx, db.CreateRiderParams{
		UserID:   otherRiderUser.ID,
		RealName: "测试骑手-其他",
		IDCardNo: fmt.Sprintf("11010119900101%04d", util.RandomInt(0, 9999)),
		Phone:    fmt.Sprintf("139%08d", util.RandomInt(0, 99999999)),
		RegionID: pgtype.Int8{Int64: region.ID, Valid: true},
	})
	require.NoError(t, err)
	_, err = store.CreateRiderProfile(ctx, otherRider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: otherRider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: otherRider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: otherRider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	// 2) 直接落库支付单并模拟支付成功，避免 legacy native payment API 漂移影响 rider appeal 旅程
	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("d26_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d26_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立代取关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 7) 骑手发起追偿争议
	var recoveryDisputeResp recoveryDisputeSubmitResponse
	{
		recoveryDisputeBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "delivery handled properly with no issues",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/recovery-disputes", recoveryDisputeBody, riderUser.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryDisputeResp)
		require.NotZero(t, recoveryDisputeResp.ID)
	}

	// 8) 非所属骑手查看追偿争议详情
	url := fmt.Sprintf("/v1/rider/recovery-disputes/%d", recoveryDisputeResp.ID)
	rec := doGET(t, server, url, otherRiderUser.ID)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestClaimJourneyD27OperatorRecoveryDisputeDetailNotFoundIntegration
// 索赔旅程（D27）端到端验收：跨区域运营商查看追偿争议详情返回 404。
func TestClaimJourneyD27OperatorRecoveryDisputeDetailNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	otherRegion := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          otherRegion.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 4) 商户发起追偿争议
	var recoveryDisputeResp recoveryDisputeSubmitResponse
	{
		recoveryDisputeBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/recovery-disputes", recoveryDisputeBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryDisputeResp)
		require.NotZero(t, recoveryDisputeResp.ID)
	}

	// 5) 跨区域运营商查看追偿争议详情
	url := fmt.Sprintf("/v1/operator/recovery-disputes/%d", recoveryDisputeResp.ID)
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestClaimJourneyD28RiderRecoveryViewAfterWaiveIntegration
// 索赔旅程（D28）端到端验收：骑手追偿单逾期封禁后，申诉自动通过会撤销追偿并释放封禁。
func TestClaimJourneyD28RiderRecoveryViewAfterWaiveIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("d28_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d28_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "damage",
			"claim_amount": order.TotalAmount,
			"claim_reason": "food was damaged during delivery",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)
	require.True(t, recovery.RecoveryTarget.Valid)
	require.Equal(t, "rider", recovery.RecoveryTarget.String)
	triggerClaimRecoveryOverdueViaScheduler(t, store, recovery.ID)
	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "overdue", recovery.Status)
	riderProfile, err := store.GetRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	require.True(t, riderProfile.IsSuspended)
	require.True(t, riderProfile.SuspendReason.Valid)

	decisions, err := store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: orderID, Valid: true})
	require.NoError(t, err)
	require.NotEmpty(t, decisions)
	_, err = integrationPool.Exec(ctx, `UPDATE behavior_decisions SET effective_status = 'superseded', updated_at = NOW() WHERE id = $1`, decisions[0].ID)
	require.NoError(t, err)

	var recoveryDisputeResp recoveryDisputeSubmitResponse
	{
		recoveryDisputeBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "latest behavior decision no longer points to rider",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/recovery-disputes", recoveryDisputeBody, riderUser.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryDisputeResp)
		require.Equal(t, "approved", recoveryDisputeResp.Status)
	}

	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/rider/recoveries/%d", claimRecoveryIDForIntegration(t, store, claimResp.ClaimID))
		rec := doGET(t, server, url, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "waived", recoveryResp.Status)
	}

	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "waived", recovery.Status)
	riderProfile, err = store.GetRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	require.False(t, riderProfile.IsSuspended)
	require.False(t, riderProfile.SuspendReason.Valid)
	eventTypes := claimRecoveryEventTypesForIntegration(t, store, recovery.ID)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeOverdue)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeDisputed)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeWaived)
	require.Equal(t, db.ClaimRecoveryEventTypeClosed, eventTypes[len(eventTypes)-1])
}

// TestClaimJourneyD29RiderRecoveryViewAfterRejectedRecoveryDisputeIntegration
// 索赔旅程（D29）端到端验收：骑手追偿单逾期封禁后，追偿争议被自动驳回会恢复到逾期追偿并保持封禁。
func TestClaimJourneyD29RiderRecoveryViewAfterRejectedRecoveryDisputeIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("d29_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d29_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "damage",
			"claim_amount": order.TotalAmount,
			"claim_reason": "food was damaged during delivery",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)
	require.True(t, recovery.RecoveryTarget.Valid)
	require.Equal(t, "rider", recovery.RecoveryTarget.String)
	triggerClaimRecoveryOverdueViaScheduler(t, store, recovery.ID)
	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "overdue", recovery.Status)
	riderProfile, err := store.GetRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	require.True(t, riderProfile.IsSuspended)
	require.True(t, riderProfile.SuspendReason.Valid)

	var recoveryDisputeResp recoveryDisputeSubmitResponse
	{
		recoveryDisputeBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "current behavior decision still points to rider responsibility",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/recovery-disputes", recoveryDisputeBody, riderUser.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryDisputeResp)
		require.Equal(t, "rejected", recoveryDisputeResp.Status)
	}

	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/rider/recoveries/%d", claimRecoveryIDForIntegration(t, store, claimResp.ClaimID))
		rec := doGET(t, server, url, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "overdue", recoveryResp.Status)
	}

	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "overdue", recovery.Status)
	riderProfile, err = store.GetRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	require.True(t, riderProfile.IsSuspended)
	require.True(t, riderProfile.SuspendReason.Valid)
	eventTypes := claimRecoveryEventTypesForIntegration(t, store, recovery.ID)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeOverdue)
	require.Contains(t, eventTypes, db.ClaimRecoveryEventTypeDisputed)
	require.Equal(t, db.ClaimRecoveryEventTypeResumed, eventTypes[len(eventTypes)-1])
}

// TestClaimJourneyD30MerchantRecoveryDisputeDuplicateIntegration
// 索赔旅程（D30）端到端验收：商户对同一索赔重复发起追偿争议返回 409。
func TestClaimJourneyD30MerchantRecoveryDisputeDuplicateIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 4) 商户首次发起追偿争议
	recoveryDisputeBody := map[string]any{
		"claim_id": claimResp.ClaimID,
		"reason":   "evidence shows no foreign object in dish",
	}
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/recovery-disputes", recoveryDisputeBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
	}

	// 5) 商户重复发起追偿争议
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/recovery-disputes", recoveryDisputeBody, merchantOwner.ID)
		require.Equal(t, http.StatusConflict, rec.Code)
	}
}

// TestReservationJourneyCConfirmBeforePaidIntegration
// 包间预订异常链路（C-Confirm-Pending）：未支付预订确认被拒。
func TestReservationJourneyCConfirmBeforePaidIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订（未支付）
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 商户确认预订
	{
		url := fmt.Sprintf("/v1/reservations/%d/confirm", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusConflict, rec.Code)
	}
}

// TestClaimJourneyD31RiderRecoveryDisputeDuplicateIntegration
// 索赔旅程（D31）端到端验收：骑手重复发起追偿争议返回已存在争议。
func TestClaimJourneyD31RiderRecoveryDisputeDuplicateIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	// 2) 直接落库支付单并模拟支付成功，避免 legacy native payment API 漂移影响 rider appeal 旅程
	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("d31_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d31_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立代取关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 7) 骑手首次发起追偿争议
	recoveryDisputeBody := map[string]any{
		"claim_id": claimResp.ClaimID,
		"reason":   "delivery handled properly with no issues",
	}
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/recovery-disputes", recoveryDisputeBody, riderUser.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
	}

	// 8) 骑手重复发起追偿争议
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/recovery-disputes", recoveryDisputeBody, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
}

// TestClaimJourneyD32RiderRecoveryDisputeDuplicateConflictIntegration
// 索赔旅程（D32）端到端验收：不同骑手对同一索赔发起追偿争议返回 403。
func TestClaimJourneyD32RiderRecoveryDisputeDuplicateConflictIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	otherRiderUser := createIntegrationUser(t, store)
	otherRider, err := store.CreateRider(ctx, db.CreateRiderParams{
		UserID:   otherRiderUser.ID,
		RealName: "测试骑手-其他",
		IDCardNo: fmt.Sprintf("11010119900101%04d", util.RandomInt(0, 9999)),
		Phone:    fmt.Sprintf("139%08d", util.RandomInt(0, 99999999)),
		RegionID: pgtype.Int8{Int64: region.ID, Valid: true},
	})
	require.NoError(t, err)
	_, err = store.CreateRiderProfile(ctx, otherRider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: otherRider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: otherRider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: otherRider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	// 2) 直接落库支付单并模拟支付成功，避免 legacy native payment API 漂移影响 rider appeal 旅程
	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("d32_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d32_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立代取关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
	}
	confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
	completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

	// 7) 骑手首次发起追偿争议
	recoveryDisputeBody := map[string]any{
		"claim_id": claimResp.ClaimID,
		"reason":   "delivery handled properly with no issues",
	}
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/recovery-disputes", recoveryDisputeBody, riderUser.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
	}

	// 8) 其他骑手重复发起追偿争议
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/recovery-disputes", recoveryDisputeBody, otherRiderUser.ID)
		require.Equal(t, http.StatusForbidden, rec.Code)
	}
}

// TestClaimJourneyD33OperatorRecoveryDisputeListByStatusIntegration
// 索赔旅程（D33）端到端验收：运营商按状态筛选追偿争议列表。
func TestClaimJourneyD33OperatorRecoveryDisputeListByStatusIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	createCompletedOrderAndClaim := func(claimReason string) (int64, int64, int64) {
		customer := createIntegrationUser(t, store)
		addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)
		createBody := map[string]any{
			"merchant_id":           merchant.ID,
			"order_type":            "takeout",
			"address_id":            addr.ID,
			"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
			"use_balance":           false,
			"delivery_fee":          int64(500),
			"delivery_distance":     int32(1000),
			"delivery_fee_discount": int64(0),
		}

		var created takeoutOrderResponse
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)

		_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
			OrderID:      created.ID,
			OldStatus:    "pending",
			OperatorID:   customer.ID,
			OperatorType: "user",
		})
		require.NoError(t, err)

		order, err := store.GetOrder(ctx, created.ID)
		require.NoError(t, err)

		var claimResp claimSubmitResponse
		claimBody := map[string]any{
			"order_id":     created.ID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": claimReason,
		}
		rec = doJSON(t, server, http.MethodPost, "/v1/claims", claimBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		requireAcceptedClaimSubmission(t, claimResp, order.TotalAmount)
		confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customer.ID, order.TotalAmount)
		completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customer.ID, order.TotalAmount)

		return created.ID, claimResp.ClaimID, customer.ID
	}

	approvedOrderID, approvedClaimID, _ := createCompletedOrderAndClaim("approved recovery dispute candidate")
	decisions, err := store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: approvedOrderID, Valid: true})
	require.NoError(t, err)
	require.NotEmpty(t, decisions)
	_, err = integrationPool.Exec(ctx, `UPDATE behavior_decisions SET effective_status = 'superseded', updated_at = NOW() WHERE id = $1`, decisions[0].ID)
	require.NoError(t, err)

	var approvedRecoveryDispute recoveryDisputeSubmitResponse
	{
		recoveryDisputeBody := map[string]any{
			"claim_id": approvedClaimID,
			"reason":   "recovery dispute approved by latest behavior decision",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/recovery-disputes", recoveryDisputeBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &approvedRecoveryDispute)
		require.Equal(t, "approved", approvedRecoveryDispute.Status)
	}

	rejectedOrderID, rejectedClaimID, _ := createCompletedOrderAndClaim("rejected recovery dispute candidate")
	decisions, err = store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: rejectedOrderID, Valid: true})
	require.NoError(t, err)
	require.NotEmpty(t, decisions)
	_, err = integrationPool.Exec(ctx, `
		UPDATE behavior_decisions
		SET responsible_party = 'merchant',
		    compensation_source = 'merchant',
		    decision_mode = 'merchant_recovery',
		    responsibility_domain = 'merchant_domain',
		    payout_mode = 'instant_paid',
		    effective_status = 'effective',
		    updated_at = NOW()
		WHERE id = $1
	`, decisions[0].ID)
	require.NoError(t, err)
	var rejectedRecoveryDispute recoveryDisputeSubmitResponse
	{
		recoveryDisputeBody := map[string]any{
			"claim_id": rejectedClaimID,
			"reason":   "recovery dispute rejected by current behavior decision",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/recovery-disputes", recoveryDisputeBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &rejectedRecoveryDispute)
		require.Equal(t, "rejected", rejectedRecoveryDispute.Status)
	}

	submittedOrderID, submittedClaimID, _ := createCompletedOrderAndClaim("submitted recovery dispute seed")
	_, err = store.CreateRecoveryDispute(ctx, db.CreateRecoveryDisputeParams{
		ClaimID:       submittedClaimID,
		AppellantType: "merchant",
		AppellantID:   merchant.ID,
		Reason:        "submitted recovery dispute seeded for operator filter",
		RegionID:      region.ID,
	})
	require.NoError(t, err)
	submittedRecoveryDispute, err := store.GetRecoveryDisputeByClaim(ctx, db.GetRecoveryDisputeByClaimParams{
		ClaimID:       submittedClaimID,
		AppellantType: "merchant",
	})
	require.NoError(t, err)
	_ = submittedOrderID

	// 7) 按 status=approved 查询
	{
		url := "/v1/operator/recovery-disputes?status=approved&page=1&limit=10"
		rec := doGET(t, server, url, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Disputes []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"disputes"`
			Total int64 `json:"total"`
		}
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.GreaterOrEqual(t, resp.Total, int64(1))
		found := false
		for _, a := range resp.Disputes {
			if a.ID == approvedRecoveryDispute.ID {
				found = true
				require.Equal(t, "approved", a.Status)
			}
		}
		require.True(t, found)
	}

	// 8) 按 status=submitted 查询
	{
		url := "/v1/operator/recovery-disputes?status=submitted&page=1&limit=10"
		rec := doGET(t, server, url, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Disputes []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"disputes"`
			Total int64 `json:"total"`
		}
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.GreaterOrEqual(t, resp.Total, int64(1))
		found := false
		for _, a := range resp.Disputes {
			if a.ID == submittedRecoveryDispute.ID {
				found = true
				require.Equal(t, "submitted", a.Status)
			}
		}
		require.True(t, found)
	}

	// 9) 按 status=rejected 查询
	{
		url := "/v1/operator/recovery-disputes?status=rejected&page=1&limit=10"
		rec := doGET(t, server, url, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Disputes []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"disputes"`
			Total int64 `json:"total"`
		}
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.GreaterOrEqual(t, resp.Total, int64(1))
		found := false
		for _, a := range resp.Disputes {
			if a.ID == rejectedRecoveryDispute.ID {
				found = true
				require.Equal(t, "rejected", a.Status)
			}
		}
		require.True(t, found)
	}
}

// TestClaimJourneyD34OperatorRecoveryDisputeListEmptyIntegration
// 索赔旅程（D34）端到端验收：运营商按追偿争议状态筛选返回空列表。
func TestClaimJourneyD34OperatorRecoveryDisputeListEmptyIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	url := "/v1/operator/recovery-disputes?status=approved&page=1&limit=10"
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Disputes []struct {
			ID int64 `json:"id"`
		} `json:"disputes"`
		Total int64 `json:"total"`
	}
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.Equal(t, int64(0), resp.Total)
	require.Len(t, resp.Disputes, 0)
}

// TestClaimJourneyD35MerchantRecoveryDisputeListEmptyIntegration
// 索赔旅程（D35）端到端验收：商户追偿争议列表空结果。
func TestClaimJourneyD35MerchantRecoveryDisputeListEmptyIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	url := "/v1/merchant/recovery-disputes?page_id=1&page_size=10"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Disputes []struct {
			ID int64 `json:"id"`
		} `json:"disputes"`
		Total int64 `json:"total"`
	}
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.Equal(t, int64(0), resp.Total)
	require.Len(t, resp.Disputes, 0)
}

// TestClaimJourneyD36RiderRecoveryDisputeListEmptyIntegration
// 索赔旅程（D36）端到端验收：骑手追偿争议列表空结果。
func TestClaimJourneyD36RiderRecoveryDisputeListEmptyIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err := store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	url := "/v1/rider/recovery-disputes?page_id=1&page_size=10"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Disputes []struct {
			ID int64 `json:"id"`
		} `json:"disputes"`
		Total int64 `json:"total"`
	}
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.Equal(t, int64(0), resp.Total)
	require.Len(t, resp.Disputes, 0)
}

// TestClaimJourneyD37OperatorRecoveryDisputeListPaginationIntegration
// 索赔旅程（D37）端到端验收：运营商追偿争议列表分页返回稳定。
func TestClaimJourneyD37OperatorRecoveryDisputeListPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	createRecoveryDispute := func(orderID int64, customerID int64, reason string) recoveryDisputeSubmitResponse {
		var claimResp claimSubmitResponse
		{
			body := map[string]any{
				"order_id":     orderID,
				"claim_type":   "foreign-object",
				"claim_amount": int64(1000),
				"claim_reason": "foreign object found in dish",
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customerID)
			require.Equal(t, http.StatusOK, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
			require.NotZero(t, claimResp.ClaimID)
		}
		confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customerID, int64(1000))
		completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customerID, int64(1000))

		var recoveryDisputeResp recoveryDisputeSubmitResponse
		{
			recoveryDisputeBody := map[string]any{
				"claim_id": claimResp.ClaimID,
				"reason":   reason,
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/merchant/recovery-disputes", recoveryDisputeBody, merchantOwner.ID)
			require.Equal(t, http.StatusCreated, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryDisputeResp)
			require.NotZero(t, recoveryDisputeResp.ID)
		}
		return recoveryDisputeResp
	}

	createOrder := func() (int64, int64) {
		customer := createIntegrationUser(t, store)
		addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)
		createBody := map[string]any{
			"merchant_id":           merchant.ID,
			"order_type":            "takeout",
			"address_id":            addr.ID,
			"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
			"use_balance":           false,
			"delivery_fee":          int64(500),
			"delivery_distance":     int32(1000),
			"delivery_fee_discount": int64(0),
		}

		var created takeoutOrderResponse
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)

		_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
			OrderID:      created.ID,
			OldStatus:    "pending",
			OperatorID:   customer.ID,
			OperatorType: "user",
		})
		require.NoError(t, err)
		return created.ID, customer.ID
	}

	orderID1, customerID1 := createOrder()
	recoveryDispute1 := createRecoveryDispute(orderID1, customerID1, "recovery dispute one")

	orderID2, customerID2 := createOrder()
	recoveryDispute2 := createRecoveryDispute(orderID2, customerID2, "recovery dispute two")

	page1URL := "/v1/operator/recovery-disputes?page=1&limit=1"
	page2URL := "/v1/operator/recovery-disputes?page=2&limit=1"

	var page1 struct {
		Disputes []struct {
			ID int64 `json:"id"`
		} `json:"disputes"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page1URL, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page1)
		require.Equal(t, int64(2), page1.Total)
		require.Len(t, page1.Disputes, 1)
	}

	var page2 struct {
		Disputes []struct {
			ID int64 `json:"id"`
		} `json:"disputes"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page2URL, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page2)
		require.Equal(t, int64(2), page2.Total)
		require.Len(t, page2.Disputes, 1)
	}

	ids := map[int64]bool{
		page1.Disputes[0].ID: true,
		page2.Disputes[0].ID: true,
	}
	require.True(t, ids[recoveryDispute1.ID])
	require.True(t, ids[recoveryDispute2.ID])
}

// TestClaimJourneyD38MerchantRecoveryDisputeListPaginationIntegration
// 索赔旅程（D38）端到端验收：商户追偿争议列表分页返回稳定。
func TestClaimJourneyD38MerchantRecoveryDisputeListPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	createOrder := func() (int64, int64) {
		customer := createIntegrationUser(t, store)
		addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)
		createBody := map[string]any{
			"merchant_id":           merchant.ID,
			"order_type":            "takeout",
			"address_id":            addr.ID,
			"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
			"use_balance":           false,
			"delivery_fee":          int64(500),
			"delivery_distance":     int32(1000),
			"delivery_fee_discount": int64(0),
		}

		var created takeoutOrderResponse
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)

		_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
			OrderID:      created.ID,
			OldStatus:    "pending",
			OperatorID:   customer.ID,
			OperatorType: "user",
		})
		require.NoError(t, err)
		return created.ID, customer.ID
	}

	createRecoveryDispute := func(orderID int64, customerID int64, reason string) recoveryDisputeSubmitResponse {
		var claimResp claimSubmitResponse
		{
			body := map[string]any{
				"order_id":     orderID,
				"claim_type":   "foreign-object",
				"claim_amount": int64(1000),
				"claim_reason": "foreign object found in dish",
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customerID)
			require.Equal(t, http.StatusOK, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
			require.NotZero(t, claimResp.ClaimID)
		}
		confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customerID, int64(1000))
		completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customerID, int64(1000))

		var recoveryDisputeResp recoveryDisputeSubmitResponse
		{
			recoveryDisputeBody := map[string]any{
				"claim_id": claimResp.ClaimID,
				"reason":   reason,
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/merchant/recovery-disputes", recoveryDisputeBody, merchantOwner.ID)
			require.Equal(t, http.StatusCreated, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryDisputeResp)
			require.NotZero(t, recoveryDisputeResp.ID)
		}
		return recoveryDisputeResp
	}

	orderID1, customerID1 := createOrder()
	recoveryDispute1 := createRecoveryDispute(orderID1, customerID1, "recovery dispute one")

	orderID2, customerID2 := createOrder()
	recoveryDispute2 := createRecoveryDispute(orderID2, customerID2, "recovery dispute two")

	page1URL := "/v1/merchant/recovery-disputes?page_id=1&page_size=1"
	page2URL := "/v1/merchant/recovery-disputes?page_id=2&page_size=1"

	var page1 struct {
		Disputes []struct {
			ID int64 `json:"id"`
		} `json:"disputes"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page1URL, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page1)
		require.Equal(t, int64(2), page1.Total)
		require.Len(t, page1.Disputes, 1)
	}

	var page2 struct {
		Disputes []struct {
			ID int64 `json:"id"`
		} `json:"disputes"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page2URL, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page2)
		require.Equal(t, int64(2), page2.Total)
		require.Len(t, page2.Disputes, 1)
	}

	ids := map[int64]bool{
		page1.Disputes[0].ID: true,
		page2.Disputes[0].ID: true,
	}
	require.True(t, ids[recoveryDispute1.ID])
	require.True(t, ids[recoveryDispute2.ID])
}

// TestClaimJourneyD39RiderRecoveryDisputeListPaginationIntegration
// 索赔旅程（D39）端到端验收：骑手追偿争议列表分页返回稳定。
func TestClaimJourneyD39RiderRecoveryDisputeListPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	createOrder := func() (int64, int64) {
		customer := createIntegrationUser(t, store)
		addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)
		createBody := map[string]any{
			"merchant_id":           merchant.ID,
			"order_type":            "takeout",
			"address_id":            addr.ID,
			"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
			"use_balance":           false,
			"delivery_fee":          int64(500),
			"delivery_distance":     int32(1000),
			"delivery_fee_discount": int64(0),
		}

		var created takeoutOrderResponse
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)

		orderForPayment, err := store.GetOrder(ctx, created.ID)
		require.NoError(t, err)

		payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
			OrderID:        pgtype.Int8{Int64: created.ID, Valid: true},
			UserID:         customer.ID,
			PaymentType:    "miniprogram",
			PaymentChannel: db.PaymentChannelDirect,
			BusinessType:   "order",
			Amount:         orderForPayment.TotalAmount,
			OutTradeNo:     fmt.Sprintf("d39_order_%d_%d", created.ID, util.RandomInt(1000, 9999)),
			ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
		})
		require.NoError(t, err)

		_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
			ID:            payment.ID,
			TransactionID: pgtype.Text{String: util.RandomString(12), Valid: true},
		})
		require.NoError(t, err)

		payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
			PaymentOrderID:     payment.ID,
			RiderAverageSpeed:  15000,
			DefaultPrepareTime: 20,
		})
		require.NoError(t, err)
		require.True(t, payRes.Processed)

		{
			url := fmt.Sprintf("/v1/merchant/orders/%d/accept", created.ID)
			rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
			require.Equal(t, http.StatusOK, rec.Code)
		}
		{
			url := fmt.Sprintf("/v1/merchant/orders/%d/ready", created.ID)
			rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
			require.Equal(t, http.StatusOK, rec.Code)
		}
		{
			url := fmt.Sprintf("/v1/delivery/grab/%d", created.ID)
			rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
			require.Equal(t, http.StatusOK, rec.Code)
		}

		_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
			OrderID:      created.ID,
			OldStatus:    "ready",
			OperatorID:   customer.ID,
			OperatorType: "user",
		})
		require.NoError(t, err)
		return created.ID, customer.ID
	}

	createRecoveryDispute := func(orderID int64, customerID int64, reason string) recoveryDisputeSubmitResponse {
		var claimResp claimSubmitResponse
		{
			body := map[string]any{
				"order_id":     orderID,
				"claim_type":   "foreign-object",
				"claim_amount": int64(1000),
				"claim_reason": "foreign object found in dish",
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customerID)
			require.Equal(t, http.StatusOK, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
			requireAcceptedClaimSubmission(t, claimResp, int64(1000))
		}
		confirmClaimContinueIntegration(t, server, claimResp.ClaimID, customerID, int64(1000))
		completeClaimPayoutForClaim(t, store, claimResp.ClaimID, customerID, int64(1000))

		var recoveryDisputeResp recoveryDisputeSubmitResponse
		{
			recoveryDisputeBody := map[string]any{
				"claim_id": claimResp.ClaimID,
				"reason":   reason,
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/rider/recovery-disputes", recoveryDisputeBody, riderUser.ID)
			require.Equal(t, http.StatusCreated, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryDisputeResp)
			require.NotZero(t, recoveryDisputeResp.ID)
		}
		return recoveryDisputeResp
	}

	orderID1, customerID1 := createOrder()
	recoveryDispute1 := createRecoveryDispute(orderID1, customerID1, "recovery dispute one")

	orderID2, customerID2 := createOrder()
	recoveryDispute2 := createRecoveryDispute(orderID2, customerID2, "recovery dispute two")

	page1URL := "/v1/rider/recovery-disputes?page_id=1&page_size=1"
	page2URL := "/v1/rider/recovery-disputes?page_id=2&page_size=1"

	var page1 struct {
		Disputes []struct {
			ID int64 `json:"id"`
		} `json:"disputes"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page1URL, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page1)
		require.Equal(t, int64(2), page1.Total)
		require.Len(t, page1.Disputes, 1)
	}

	var page2 struct {
		Disputes []struct {
			ID int64 `json:"id"`
		} `json:"disputes"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page2URL, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page2)
		require.Equal(t, int64(2), page2.Total)
		require.Len(t, page2.Disputes, 1)
	}

	ids := map[int64]bool{
		page1.Disputes[0].ID: true,
		page2.Disputes[0].ID: true,
	}
	require.True(t, ids[recoveryDispute1.ID])
	require.True(t, ids[recoveryDispute2.ID])
}

// TestClaimJourneyD40OperatorRecoveryDisputeListInvalidStatusIntegration
// 索赔旅程（D40）端到端验收：运营商追偿争议列表非法状态返回 400。
func TestClaimJourneyD40OperatorRecoveryDisputeListInvalidStatusIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	url := "/v1/operator/recovery-disputes?status=invalid&page=1&limit=10"
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD41MerchantRecoveryDisputeListInvalidPaginationIntegration
// 索赔旅程（D41）端到端验收：商户追偿争议列表非法分页返回 400。
func TestClaimJourneyD41MerchantRecoveryDisputeListInvalidPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	url := "/v1/merchant/recovery-disputes?page_id=0&page_size=10"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD42RiderRecoveryDisputeListInvalidPaginationIntegration
// 索赔旅程（D42）端到端验收：骑手追偿争议列表非法分页返回 400。
func TestClaimJourneyD42RiderRecoveryDisputeListInvalidPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err := store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	url := "/v1/rider/recovery-disputes?page_id=0&page_size=10"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD43OperatorRecoveryDisputeListDefaultPaginationIntegration
// 索赔旅程（D43）端到端验收：运营商追偿争议列表默认分页参数。
func TestClaimJourneyD43OperatorRecoveryDisputeListDefaultPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	url := "/v1/operator/recovery-disputes"
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Page  int32 `json:"page"`
		Limit int32 `json:"limit"`
	}
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.Equal(t, int32(1), resp.Page)
	require.Equal(t, int32(10), resp.Limit)
}

// TestClaimJourneyD44MerchantRecoveryDisputeListInvalidPageSizeIntegration
// 索赔旅程（D44）端到端验收：商户追偿争议列表超限 page_size 返回 400。
func TestClaimJourneyD44MerchantRecoveryDisputeListInvalidPageSizeIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	url := "/v1/merchant/recovery-disputes?page_id=1&page_size=51"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD45RiderRecoveryDisputeListInvalidPageSizeIntegration
// 索赔旅程（D45）端到端验收：骑手追偿争议列表超限 page_size 返回 400。
func TestClaimJourneyD45RiderRecoveryDisputeListInvalidPageSizeIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err := store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	url := "/v1/rider/recovery-disputes?page_id=1&page_size=51"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD46MerchantRecoveryDisputeDetailInvalidIDIntegration
// 索赔旅程（D46）端到端验收：商户追偿争议详情非法 ID 返回 400。
func TestClaimJourneyD46MerchantRecoveryDisputeDetailInvalidIDIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	_ = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	url := "/v1/merchant/recovery-disputes/invalid"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD47RiderRecoveryDisputeDetailInvalidIDIntegration
// 索赔旅程（D47）端到端验收：骑手追偿争议详情非法 ID 返回 400。
func TestClaimJourneyD47RiderRecoveryDisputeDetailInvalidIDIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err := store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	url := "/v1/rider/recovery-disputes/invalid"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD48OperatorRecoveryDisputeDetailInvalidIDIntegration
// 索赔旅程（D48）端到端验收：运营商追偿争议详情非法 ID 返回 400。
func TestClaimJourneyD48OperatorRecoveryDisputeDetailInvalidIDIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	url := "/v1/operator/recovery-disputes/invalid"
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestTakeoutJourneyB2Integration
// 外卖旅程（B2）端到端验收：骑手送达后 1 小时无索赔自动完成（兜底终点）。
//
// 说明：integration 环境不启动 cron，这里直接调用 scheduler.RunOnce() 触发一次扫描。
func TestTakeoutJourneyB2Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)

	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) C端下单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}
	orderID := created.ID

	// 2) 直接落库支付单并模拟支付成功，避免 legacy payment API 兼容路径影响自动完成旅程
	orderForPayment, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	payment, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:        pgtype.Int8{Int64: orderID, Valid: true},
		UserID:         customer.ID,
		PaymentType:    "miniprogram",
		PaymentChannel: db.PaymentChannelDirect,
		BusinessType:   "order",
		Amount:         orderForPayment.TotalAmount,
		OutTradeNo:     fmt.Sprintf("b2_order_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:      pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_b2_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单并推进到送达（rider_delivered）
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	delivery, err := store.GetDeliveryByOrderID(ctx, orderID)
	require.NoError(t, err)

	{
		url := fmt.Sprintf("/v1/delivery/%d/start-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/start-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		body := map[string]any{
			"region_id": region.ID,
			"locations": []map[string]any{
				{
					"delivery_id": delivery.ID,
					"longitude":   116.3975,
					"latitude":    39.9084,
					"recorded_at": time.Now().UTC().Format(time.RFC3339),
					"source":      "integration_confirm_delivery",
				},
			},
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/location", body, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	o, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "rider_delivered", o.Status)

	// 5) 回拨 rider_delivered_at 使其满足“送达超过 1 小时”
	backTo := time.Now().Add(-scheduler.TakeoutAutoCompleteAfter).Add(-2 * time.Minute)
	_, err = integrationPool.Exec(ctx, `UPDATE orders SET rider_delivered_at = $2 WHERE id = $1`, orderID, backTo)
	require.NoError(t, err)

	// 6) 触发 scheduler 扫描并断言自动完成
	s := scheduler.NewTakeoutAutoCompleteScheduler(store, nil)
	s.RunOnce()

	updated, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", updated.Status)
	require.True(t, updated.CompletedAt.Valid)

	// 幂等：重复扫描不应改变结果/报错
	s.RunOnce()
	updated2, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", updated2.Status)
}

type captureTaskDistributor struct {
	worker.NoopTaskDistributor

	mu    sync.Mutex
	calls []worker.BaofuProfitSharingPayload
}

func (d *captureTaskDistributor) DistributeTaskProcessBaofuProfitSharing(ctx context.Context, payload *worker.BaofuProfitSharingPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.calls = append(d.calls, *payload)
	}
	return nil
}

func (d *captureTaskDistributor) Calls() []worker.BaofuProfitSharingPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.BaofuProfitSharingPayload, len(d.calls))
	copy(out, d.calls)
	return out
}

type capturePaymentSuccessDistributor struct {
	worker.NoopTaskDistributor

	mu       sync.Mutex
	payloads []worker.PaymentFactApplicationPayload
}

type capturePaymentFactApplicationDistributor struct {
	worker.NoopTaskDistributor

	mu       sync.Mutex
	payloads []worker.PaymentFactApplicationPayload
}

type captureClaimBehaviorActionDistributor struct {
	worker.NoopTaskDistributor

	mu       sync.Mutex
	payloads []worker.ClaimBehaviorActionPayload
}

func (d *captureClaimBehaviorActionDistributor) DistributeTaskClaimBehaviorAction(ctx context.Context, payload *worker.ClaimBehaviorActionPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.payloads = append(d.payloads, *payload)
	}
	return nil
}

func (d *captureClaimBehaviorActionDistributor) Payloads() []worker.ClaimBehaviorActionPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.ClaimBehaviorActionPayload, len(d.payloads))
	copy(out, d.payloads)
	return out
}

func (d *capturePaymentSuccessDistributor) DistributeTaskProcessPaymentFactApplication(ctx context.Context, payload *worker.PaymentFactApplicationPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.payloads = append(d.payloads, *payload)
	}
	return nil
}

func (d *capturePaymentSuccessDistributor) Payloads() []worker.PaymentFactApplicationPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.PaymentFactApplicationPayload, len(d.payloads))
	copy(out, d.payloads)
	return out
}

func (d *capturePaymentFactApplicationDistributor) DistributeTaskProcessPaymentFactApplication(ctx context.Context, payload *worker.PaymentFactApplicationPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.payloads = append(d.payloads, *payload)
	}
	return nil
}

func (d *capturePaymentFactApplicationDistributor) Payloads() []worker.PaymentFactApplicationPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.PaymentFactApplicationPayload, len(d.payloads))
	copy(out, d.payloads)
	return out
}

type captureRefundResultDistributor struct {
	worker.NoopTaskDistributor

	mu       sync.Mutex
	payloads []worker.RefundResultPayload
}

type captureRecoveryDisputeResultDistributor struct {
	worker.NoopTaskDistributor

	mu       sync.Mutex
	payloads []worker.ProcessRecoveryDisputeResultPayload
}

func (d *captureRecoveryDisputeResultDistributor) DistributeTaskProcessRecoveryDisputeResult(ctx context.Context, payload *worker.ProcessRecoveryDisputeResultPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.payloads = append(d.payloads, *payload)
	}
	return nil
}

func (d *captureRecoveryDisputeResultDistributor) Payloads() []worker.ProcessRecoveryDisputeResultPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.ProcessRecoveryDisputeResultPayload, len(d.payloads))
	copy(out, d.payloads)
	return out
}

type captureSendNotificationDistributor struct {
	worker.NoopTaskDistributor

	mu       sync.Mutex
	payloads []worker.SendNotificationPayload
}

func (d *captureSendNotificationDistributor) DistributeTaskSendNotification(ctx context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.payloads = append(d.payloads, *payload)
	}
	return nil
}

func (d *captureSendNotificationDistributor) Payloads() []worker.SendNotificationPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.SendNotificationPayload, len(d.payloads))
	copy(out, d.payloads)
	return out
}

func (d *captureRefundResultDistributor) DistributeTaskProcessRefundResult(ctx context.Context, payload *worker.RefundResultPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.payloads = append(d.payloads, *payload)
	}
	return nil
}

func (d *captureRefundResultDistributor) Payloads() []worker.RefundResultPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.RefundResultPayload, len(d.payloads))
	copy(out, d.payloads)
	return out
}

// TestTakeoutJourneyB3Integration
// 外卖旅程（B3）端到端验收：完成后触发 profit_sharing 分账 + 恢复补偿兜底。
//
// integration harness 不配置队列/worker（taskDistributor=nil），这里按文档“兜底证据”口径验收：
// - 构造一条超龄 pending 的 profit_sharing_orders 记录
// - 触发 Baofu payment recovery scheduler 扫描
// - 断言它会尝试入队 baofu:process_profit_sharing（通过捕获 distributor 调用）
func TestTakeoutJourneyB3Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)

	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)
	ensureIntegrationPlatformBaofuReady(t, store)
	ensureIntegrationMerchantBaofuReady(t, store, merchant.ID, "sub_mch_b3_001")
	ensureIntegrationRiderBaofuReady(t, store, rider.ID)
	clearIntegrationBaofuFeeTables(t)
	t.Cleanup(func() {
		clearIntegrationBaofuFeeTables(t)
	})

	// 1) 下单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}
	orderID := created.ID

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	// 2) 创建宝付聚合支付单（直接落库以验收分账/恢复逻辑）
	po, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:               pgtype.Int8{Int64: orderID, Valid: true},
		UserID:                customer.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: db.OrderRequiresProfitSharing(order),
		BusinessType:          "order",
		Amount:                order.TotalAmount,
		OutTradeNo:            fmt.Sprintf("PS_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            po.ID,
		TransactionID: pgtype.Text{String: "integration_profit_sharing_tx_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     po.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	pso, err := logic.NewBaofuProfitSharingService(store).CreatePendingOrder(ctx, logic.BaofuProfitSharingOrderInput{
		PaymentOrderID:  po.ID,
		MerchantID:      merchant.ID,
		OperatorID:      0,
		PlatformOwnerID: 0,
		OrderSource:     "takeout",
		TotalAmountFen:  po.Amount,
		DeliveryFeeFen:  order.DeliveryFee,
		PlatformRateBps: 200, // 2%
		OperatorRateBps: 0,
		OutOrderNo:      fmt.Sprintf("BFPS%dO%d", po.ID, orderID),
	})
	require.NoError(t, err)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手代取到送达
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	delivery, err := store.GetDeliveryByOrderID(ctx, orderID)
	require.NoError(t, err)
	{
		url := fmt.Sprintf("/v1/delivery/%d/start-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/start-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		body := map[string]any{
			"region_id": region.ID,
			"locations": []map[string]any{
				{
					"delivery_id": delivery.ID,
					"longitude":   116.3975,
					"latitude":    39.9084,
					"recorded_at": time.Now().UTC().Format(time.RFC3339),
					"source":      "integration_confirm_delivery",
				},
			},
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/location", body, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 用户确认完成（外卖分账统一等待微信结算事件；integration 里这里只验证订单完成）
	{
		url := fmt.Sprintf("/v1/orders/%d/confirm", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	updatedOrder, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", updatedOrder.Status)

	// 6) 构造一条“超龄 completed”订单，验证 recovery 会尝试创建分账单并入队
	_, err = integrationPool.Exec(ctx, `UPDATE orders SET completed_at = $2, updated_at = $2 WHERE id = $1`, orderID, time.Now().Add(-20*time.Minute))
	require.NoError(t, err)
	_, err = integrationPool.Exec(ctx, `UPDATE profit_sharing_orders SET created_at = $2 WHERE id = $1`, pso.ProfitSharingOrder.ID, time.Now().Add(-20*time.Minute))
	require.NoError(t, err)

	d := &captureTaskDistributor{}
	recovery := worker.NewBaofuPaymentRecoveryScheduler(store, d)
	recovery.SetBaofuAggregateClient(&integrationBaofuAggregateClient{}, worker.BaofuProfitSharingWorkerConfig{
		CollectMerchantID: "102004465",
		CollectTerminalID: "200005200",
		ShareNotifyURL:    "https://api.example.com/v1/webhooks/baofu/share",
		RefundNotifyURL:   "https://api.example.com/v1/webhooks/baofu/refund",
	})
	recovery.RunOnce()

	calls := d.Calls()
	require.Len(t, calls, 1)
	require.NotZero(t, calls[0].ProfitSharingOrderID)
}

func ensureIntegrationOperatorRegionBinding(t *testing.T, userID int64) {
	t.Helper()

	if integrationStore == nil || userID <= 0 {
		return
	}

	ctx := context.Background()
	operator, err := integrationStore.GetOperatorByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return
		}
		require.NoError(t, err)
	}

	_, err = integrationStore.GetOperatorRegion(ctx, db.GetOperatorRegionParams{
		OperatorID: operator.ID,
		RegionID:   operator.RegionID,
	})
	if err == nil {
		return
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		require.NoError(t, err)
	}

	_, err = integrationStore.AddOperatorRegion(ctx, db.AddOperatorRegionParams{
		OperatorID: operator.ID,
		RegionID:   operator.RegionID,
	})
	require.NoError(t, err)
}

func doJSON(t *testing.T, server *api.Server, method, url string, body any, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	ensureIntegrationOperatorRegionBinding(t, userID)

	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		payload = b
	} else {
		payload = []byte("{}")
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.9:12345"
	addAuthorization(t, req, integrationTokenMaker, userID, time.Minute)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func doGET(t *testing.T, server *api.Server, url string, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	ensureIntegrationOperatorRegionBinding(t, userID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.RemoteAddr = "203.0.113.9:12345"
	addAuthorization(t, req, integrationTokenMaker, userID, time.Minute)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func numericE7(v int64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(v), Exp: -7, Valid: true}
}

func ensureIntegrationMerchantCoords(t *testing.T, store *db.SQLStore, merchantID int64) db.Merchant {
	t.Helper()

	_, err := integrationPool.Exec(context.Background(), `UPDATE merchants SET longitude = $2, latitude = $3 WHERE id = $1`,
		merchantID,
		numericE7(1163970000),
		numericE7(399085000),
	)
	require.NoError(t, err)

	m, err := store.GetMerchant(context.Background(), merchantID)
	require.NoError(t, err)
	require.True(t, m.Longitude.Valid)
	require.True(t, m.Latitude.Valid)
	return m
}

func createIntegrationDish(t *testing.T, store *db.SQLStore, merchantID int64) db.Dish {
	t.Helper()

	res, err := store.CreateDishTx(context.Background(), db.CreateDishTxParams{
		MerchantID:  merchantID,
		CategoryID:  pgtype.Int8{Valid: false},
		Name:        "集成测试菜品",
		Description: pgtype.Text{String: "integration", Valid: true},
		Price:       1999,
		MemberPrice: pgtype.Int8{Valid: false},
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   0,
		PrepareTime: 10,
	})
	require.NoError(t, err)
	return res.Dish
}

func createIntegrationUserAddress(t *testing.T, store *db.SQLStore, userID int64, regionID int64) db.UserAddress {
	t.Helper()

	addr, err := store.CreateUserAddress(context.Background(), db.CreateUserAddressParams{
		UserID:        userID,
		RegionID:      regionID,
		DetailAddress: "东华门街道 1 号",
		ContactName:   "张三",
		ContactPhone:  "13800138001",
		Longitude:     numericE7(1163975000),
		Latitude:      numericE7(399084000),
		IsDefault:     true,
	})
	require.NoError(t, err)
	return addr
}

func createIntegrationRider(t *testing.T, store *db.SQLStore, userID, regionID int64) db.Rider {
	t.Helper()

	phone := fmt.Sprintf("139%08d", util.RandomInt(0, 99999999))
	rider, err := store.CreateRider(context.Background(), db.CreateRiderParams{
		UserID:   userID,
		RealName: "测试骑手",
		IDCardNo: "110101199001011234",
		Phone:    phone,
		RegionID: pgtype.Int8{Int64: regionID, Valid: true},
	})
	require.NoError(t, err)
	return rider
}
