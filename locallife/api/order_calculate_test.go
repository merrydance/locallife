package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/merrydance/locallife/logic"
	"github.com/stretchr/testify/require"
)

type stubOrderQueryService struct {
	preview logic.OrderCalculationResult
	err     error
}

type captureOrderQueryService struct {
	stubOrderQueryService
	input logic.CalculateOrderPreviewInput
}

func (s stubOrderQueryService) GetUserOrder(context.Context, logic.GetUserOrderQueryInput) (logic.GetUserOrderQueryResult, error) {
	return logic.GetUserOrderQueryResult{}, nil
}

func (s stubOrderQueryService) ListUserOrders(context.Context, logic.ListUserOrdersQueryInput) (logic.ListUserOrdersQueryResult, error) {
	return logic.ListUserOrdersQueryResult{}, nil
}

func (s stubOrderQueryService) GetMerchantOrder(context.Context, logic.GetMerchantOrderQueryInput) (logic.GetMerchantOrderQueryResult, error) {
	return logic.GetMerchantOrderQueryResult{}, nil
}

func (s stubOrderQueryService) ListMerchantOrders(context.Context, logic.ListMerchantOrdersQueryInput) (logic.ListMerchantOrdersQueryResult, error) {
	return logic.ListMerchantOrdersQueryResult{}, nil
}

func (s stubOrderQueryService) GetMerchantOrderStats(context.Context, logic.GetMerchantOrderStatsQueryInput) (logic.GetMerchantOrderStatsQueryResult, error) {
	return logic.GetMerchantOrderStatsQueryResult{}, nil
}

func (s stubOrderQueryService) CalculateOrderPreview(context.Context, logic.CalculateOrderPreviewInput) (logic.OrderCalculationResult, error) {
	return s.preview, s.err
}

func (s *captureOrderQueryService) CalculateOrderPreview(_ context.Context, input logic.CalculateOrderPreviewInput) (logic.OrderCalculationResult, error) {
	s.input = input
	return s.preview, s.err
}

func TestCalculateOrderAPI_ReturnsPromotionEngineFields(t *testing.T) {
	selectedOptionID := int64(101)
	server := newTestServer(t, nil)
	server.orderQuerySvc = stubOrderQueryService{
		preview: logic.OrderCalculationResult{
			Subtotal:            1800,
			PackagingFee:        120,
			Packaging:           logic.CartPackagingState{Enabled: true, Required: true, Applicable: true, SelectedOptionID: &selectedOptionID, SelectionVersion: 8},
			DeliveryFee:         300,
			DeliveryFeeDiscount: 100,
			DiscountAmount:      250,
			TotalAmount:         1870,
			Promotions:          []logic.OrderPromotion{{Type: "merchant", Title: "满减", Amount: 250}},
			Items:               []logic.OrderCalculationItem{{Name: "鱼香肉丝", UnitPrice: 1800, Quantity: 1, Subtotal: 1800}},
			SuggestedVoucher:    &logic.SuggestedVoucher{ID: 12, Name: "推荐券", Amount: 200},
			LadderPromotions:    []logic.LadderPromotion{{RuleID: 9, Name: "满20减3", Threshold: 2000, Discount: 300, MissingNeed: 200}},
			VoucherTrials:       []logic.VoucherTrial{{VoucherID: 12, VoucherName: "推荐券", Amount: 200, TrialPayable: 1670}},
			PaymentAssessment:   logic.PaymentAssessment{IsBalancePayable: true, UsableBalance: 2200, PrincipalPart: 1000, BonusPart: 870, PaymentHint: "余额可覆盖本单"},
		},
	}

	user, _ := randomUser(t)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/orders/calculate?merchant_id=1&order_type=takeout", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			PackagingFee      int64                    `json:"packaging_fee"`
			Packaging         packagingPreviewResponse `json:"packaging"`
			TotalAmount       int64                    `json:"total_amount"`
			SuggestedVoucher  *logic.SuggestedVoucher  `json:"suggested_voucher"`
			LadderPromotions  []logic.LadderPromotion  `json:"ladder_promotions"`
			VoucherTrials     []logic.VoucherTrial     `json:"voucher_trials"`
			PaymentAssessment logic.PaymentAssessment  `json:"payment_assessment"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.Equal(t, int64(120), response.Data.PackagingFee)
	require.Equal(t, int64(1870), response.Data.TotalAmount)
	require.True(t, response.Data.Packaging.Enabled)
	require.True(t, response.Data.Packaging.Required)
	require.True(t, response.Data.Packaging.Applicable)
	require.NotNil(t, response.Data.Packaging.SelectedOptionID)
	require.Equal(t, selectedOptionID, *response.Data.Packaging.SelectedOptionID)
	require.Equal(t, int64(8), response.Data.Packaging.SelectionVersion)
	require.Equal(t, int64(120), response.Data.Packaging.Fee)
	require.NotNil(t, response.Data.SuggestedVoucher)
	require.Equal(t, int64(12), response.Data.SuggestedVoucher.ID)
	require.Len(t, response.Data.LadderPromotions, 1)
	require.Len(t, response.Data.VoucherTrials, 1)
	require.Equal(t, "余额可覆盖本单", response.Data.PaymentAssessment.PaymentHint)
}

func TestCalculateOrderAPIPassesLegacyPackagingFreezeToLogic(t *testing.T) {
	server := newTestServer(t, nil)
	server.config.PackagingLegacyDishFreezeEnabled = true
	captureSvc := &captureOrderQueryService{}
	server.orderQuerySvc = captureSvc

	user, _ := randomUser(t)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/orders/calculate?merchant_id=1&order_type=takeout", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, captureSvc.input.RejectLegacyPackagingDishes)
}
