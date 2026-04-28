package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	api "github.com/merrydance/locallife/api"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

type foodSafetyReportAPIResponse struct {
	IncidentID        int64  `json:"incident_id"`
	CaseID            *int64 `json:"case_id,omitempty"`
	MerchantSuspended bool   `json:"merchant_suspended"`
	SuspendDuration   *int   `json:"suspend_duration,omitempty"`
	Message           string `json:"message"`
}

type operatorFoodSafetyCaseListAPIResponse struct {
	Items []operatorFoodSafetyCaseItem `json:"items"`
	Total int64                        `json:"total"`
}

type operatorFoodSafetyCaseDetailAPIResponse struct {
	Case      operatorFoodSafetyCaseItem      `json:"case"`
	Incidents []operatorFoodSafetyIncidentAPI `json:"incidents"`
}

type operatorFoodSafetyCaseItem struct {
	ID                          int64   `json:"id"`
	MerchantID                  int64   `json:"merchant_id"`
	Status                      string  `json:"status"`
	PrimaryProductKey           string  `json:"primary_product_key"`
	PrimaryProductLabel         string  `json:"primary_product_label"`
	TriggerReason               string  `json:"trigger_reason"`
	InvestigationReport         *string `json:"investigation_report,omitempty"`
	MerchantRectificationReport *string `json:"merchant_rectification_report,omitempty"`
	Resolution                  *string `json:"resolution,omitempty"`
}

type operatorFoodSafetyIncidentAPI struct {
	ID      int64  `json:"id"`
	OrderID int64  `json:"order_id"`
	UserID  int64  `json:"user_id"`
	Status  string `json:"status"`
}

func TestFoodSafetyCaseClosedLoopIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)
	_, err := store.CreateMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)

	sharedDish := createIntegrationDish(t, store, merchant.ID)

	operatorUser := createIntegrationUser(t, store)
	operator := createIntegrationOperator(t, store, operatorUser.ID, region.ID)
	require.NotZero(t, operator.ID)

	reporters := []db.User{
		createIntegrationUser(t, store),
		createIntegrationUser(t, store),
		createIntegrationUser(t, store),
	}
	addresses := []db.UserAddress{
		createIntegrationUserAddress(t, store, reporters[0].ID, region.ID),
		createIntegrationUserAddress(t, store, reporters[1].ID, region.ID),
		createIntegrationUserAddress(t, store, reporters[2].ID, region.ID),
	}

	previousOrder := createIntegrationFoodSafetyOrder(t, store, reporters[0].ID, merchant.ID, sharedDish, addresses[0], "history")
	require.NotZero(t, previousOrder.ID)

	targetOrders := []db.Order{
		createIntegrationFoodSafetyOrder(t, store, reporters[0].ID, merchant.ID, sharedDish, addresses[0], "target-a"),
		createIntegrationFoodSafetyOrder(t, store, reporters[1].ID, merchant.ID, sharedDish, addresses[1], "target-b"),
		createIntegrationFoodSafetyOrder(t, store, reporters[2].ID, merchant.ID, sharedDish, addresses[2], "target-c"),
	}

	var triggerResp foodSafetyReportAPIResponse
	for idx, order := range targetOrders {
		resp := reportFoodSafetyIntegration(t, server, reporters[idx].ID, merchant.ID, order.ID)
		if idx < 2 {
			require.False(t, resp.MerchantSuspended)
			require.Nil(t, resp.CaseID)
		} else {
			triggerResp = resp
		}
	}

	require.True(t, triggerResp.MerchantSuspended)
	require.NotNil(t, triggerResp.CaseID)
	require.NotZero(t, triggerResp.IncidentID)
	require.NotEmpty(t, triggerResp.Message)

	merchantAfterSuspend, err := store.GetMerchant(ctx, merchant.ID)
	require.NoError(t, err)
	require.Equal(t, "suspended", merchantAfterSuspend.Status)

	profileAfterSuspend, err := store.GetMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)
	require.True(t, profileAfterSuspend.IsSuspended)
	require.True(t, profileAfterSuspend.IsTakeoutSuspended)

	caseList := listOperatorFoodSafetyCasesIntegration(t, server, operatorUser.ID)
	require.Len(t, caseList.Items, 1)
	require.Equal(t, *triggerResp.CaseID, caseList.Items[0].ID)
	require.Equal(t, merchant.ID, caseList.Items[0].MerchantID)
	require.Equal(t, "merchant-suspended", caseList.Items[0].Status)
	require.NotEmpty(t, caseList.Items[0].PrimaryProductKey)
	require.Equal(t, sharedDish.Name, caseList.Items[0].PrimaryProductLabel)

	caseDetail := getOperatorFoodSafetyCaseDetailIntegration(t, server, operatorUser.ID, *triggerResp.CaseID)
	require.Equal(t, *triggerResp.CaseID, caseDetail.Case.ID)
	require.Len(t, caseDetail.Incidents, 3)
	for _, incident := range caseDetail.Incidents {
		require.Equal(t, "merchant-suspended", incident.Status)
	}

	investigationReport := "运营商完成同菜品聚集性食安事件调查，已要求商户立即整改。"
	updatedCase := investigateOperatorFoodSafetyCaseIntegration(t, server, operatorUser.ID, *triggerResp.CaseID, investigationReport)
	require.Equal(t, "investigating", updatedCase.Status)
	require.NotNil(t, updatedCase.InvestigationReport)
	require.Equal(t, investigationReport, *updatedCase.InvestigationReport)

	merchantRectificationReport := "商户已完成问题批次下架、后厨消杀、人员复训，并提交整改记录。"
	resolution := "核查整改完成，恢复商户营业并继续观察。"
	resolvedCase := resolveOperatorFoodSafetyCaseIntegration(t, server, operatorUser.ID, *triggerResp.CaseID, investigationReport, merchantRectificationReport, resolution)
	require.Equal(t, "resolved", resolvedCase.Status)
	require.NotNil(t, resolvedCase.MerchantRectificationReport)
	require.Equal(t, merchantRectificationReport, *resolvedCase.MerchantRectificationReport)
	require.NotNil(t, resolvedCase.Resolution)
	require.Equal(t, resolution, *resolvedCase.Resolution)

	persistedCase, err := store.GetFoodSafetyCase(ctx, *triggerResp.CaseID)
	require.NoError(t, err)
	require.Equal(t, "resolved", persistedCase.Status)
	require.True(t, persistedCase.ResolvedAt.Valid)

	incidents, err := store.ListFoodSafetyIncidentsByCase(ctx, pgtype.Int8{Int64: *triggerResp.CaseID, Valid: true})
	require.NoError(t, err)
	require.Len(t, incidents, 3)
	for _, incident := range incidents {
		require.Equal(t, "resolved", incident.Status)
		require.True(t, incident.ResolvedAt.Valid)
	}

	merchantAfterResolve, err := store.GetMerchant(ctx, merchant.ID)
	require.NoError(t, err)
	require.Equal(t, "active", merchantAfterResolve.Status)

	profileAfterResolve, err := store.GetMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)
	require.False(t, profileAfterResolve.IsSuspended)
	require.False(t, profileAfterResolve.IsTakeoutSuspended)
}

func TestFoodSafetyReportDuplicateOrderReusesOpenIncident(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)
	_, err := store.CreateMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)

	sharedDish := createIntegrationDish(t, store, merchant.ID)
	reporter := createIntegrationUser(t, store)
	address := createIntegrationUserAddress(t, store, reporter.ID, region.ID)
	order := createIntegrationFoodSafetyOrder(t, store, reporter.ID, merchant.ID, sharedDish, address, "duplicate")

	firstResp := reportFoodSafetyIntegration(t, server, reporter.ID, merchant.ID, order.ID)
	require.False(t, firstResp.MerchantSuspended)

	secondResp := reportFoodSafetyIntegration(t, server, reporter.ID, merchant.ID, order.ID)
	require.Equal(t, firstResp.IncidentID, secondResp.IncidentID)
	require.False(t, secondResp.MerchantSuspended)
	require.Nil(t, secondResp.CaseID)
	require.Equal(t, "当前订单已有有效食安上报，已复用现有记录", secondResp.Message)

	incidents, err := store.ListMerchantFoodSafetyIncidents(ctx, db.ListMerchantFoodSafetyIncidentsParams{
		MerchantID: merchant.ID,
		CreatedAt:  time.Now().Add(-24 * time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, incidents, 1)
}

func TestFoodSafetyCaseResolutionDoesNotClearNonFoodSafetySuspension(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)
	_, err := store.CreateMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)

	sharedDish := createIntegrationDish(t, store, merchant.ID)

	operatorUser := createIntegrationUser(t, store)
	operator := createIntegrationOperator(t, store, operatorUser.ID, region.ID)
	require.NotZero(t, operator.ID)

	reporters := []db.User{
		createIntegrationUser(t, store),
		createIntegrationUser(t, store),
		createIntegrationUser(t, store),
	}
	addresses := []db.UserAddress{
		createIntegrationUserAddress(t, store, reporters[0].ID, region.ID),
		createIntegrationUserAddress(t, store, reporters[1].ID, region.ID),
		createIntegrationUserAddress(t, store, reporters[2].ID, region.ID),
	}

	_ = createIntegrationFoodSafetyOrder(t, store, reporters[0].ID, merchant.ID, sharedDish, addresses[0], "history-guard")
	targetOrders := []db.Order{
		createIntegrationFoodSafetyOrder(t, store, reporters[0].ID, merchant.ID, sharedDish, addresses[0], "guard-a"),
		createIntegrationFoodSafetyOrder(t, store, reporters[1].ID, merchant.ID, sharedDish, addresses[1], "guard-b"),
		createIntegrationFoodSafetyOrder(t, store, reporters[2].ID, merchant.ID, sharedDish, addresses[2], "guard-c"),
	}

	var triggerResp foodSafetyReportAPIResponse
	for idx, order := range targetOrders {
		resp := reportFoodSafetyIntegration(t, server, reporters[idx].ID, merchant.ID, order.ID)
		if idx == len(targetOrders)-1 {
			triggerResp = resp
		}
	}

	require.True(t, triggerResp.MerchantSuspended)
	require.NotNil(t, triggerResp.CaseID)

	investigationReport := "运营商已完成调查并准备结案。"
	updatedCase := investigateOperatorFoodSafetyCaseIntegration(t, server, operatorUser.ID, *triggerResp.CaseID, investigationReport)
	require.Equal(t, "investigating", updatedCase.Status)

	_, err = integrationPool.Exec(ctx, `
		UPDATE merchant_profiles
		SET suspend_reason = 'manual compliance hold',
		    takeout_suspend_reason = 'manual compliance hold'
		WHERE merchant_id = $1
	`, merchant.ID)
	require.NoError(t, err)

	_, err = integrationPool.Exec(ctx, `UPDATE merchants SET status = 'suspended' WHERE id = $1`, merchant.ID)
	require.NoError(t, err)

	resolvedCase := resolveOperatorFoodSafetyCaseIntegration(
		t,
		server,
		operatorUser.ID,
		*triggerResp.CaseID,
		investigationReport,
		"商户提交了整改说明，但仍处于人工合规冻结。",
		"案件结案，但不得覆盖现有的非食安暂停状态。",
	)
	require.Equal(t, "resolved", resolvedCase.Status)

	merchantAfterResolve, err := store.GetMerchant(ctx, merchant.ID)
	require.NoError(t, err)
	require.Equal(t, "suspended", merchantAfterResolve.Status)

	profileAfterResolve, err := store.GetMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)
	require.True(t, profileAfterResolve.IsSuspended)
	require.True(t, profileAfterResolve.IsTakeoutSuspended)
	require.Equal(t, "manual compliance hold", strings.TrimSpace(profileAfterResolve.SuspendReason.String))
	require.Equal(t, "manual compliance hold", strings.TrimSpace(profileAfterResolve.TakeoutSuspendReason.String))
}

func createIntegrationOperator(t *testing.T, store *db.SQLStore, userID, regionID int64) db.Operator {
	t.Helper()

	ctx := context.Background()
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            userID,
		RegionID:          regionID,
		Name:              "集成测试运营商",
		ContactName:       "食安运营",
		ContactPhone:      "13800138123",
		WechatMchID:       pgtype.Text{Valid: false},
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)

	_, err = store.AddOperatorRegion(ctx, db.AddOperatorRegionParams{
		OperatorID: operator.ID,
		RegionID:   regionID,
	})
	require.NoError(t, err)

	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          userID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	return operator
}

func createIntegrationFoodSafetyOrder(t *testing.T, store *db.SQLStore, userID, merchantID int64, dish db.Dish, address db.UserAddress, suffix string) db.Order {
	t.Helper()

	ctx := context.Background()
	result, err := store.CreateOrderTx(context.Background(), db.CreateOrderTxParams{
		CreateOrderParams: db.CreateOrderParams{
			OrderNo:                      fmt.Sprintf("fs-%s-%s", suffix, util.RandomString(10)),
			UserID:                       userID,
			MerchantID:                   merchantID,
			OrderType:                    "takeaway",
			AddressID:                    pgtype.Int8{Int64: address.ID, Valid: true},
			DeliveryContactNameSnapshot:  pgtype.Text{String: address.ContactName, Valid: true},
			DeliveryContactPhoneSnapshot: pgtype.Text{String: address.ContactPhone, Valid: true},
			DeliveryAddressSnapshot:      pgtype.Text{String: address.DetailAddress, Valid: true},
			DeliveryLongitudeSnapshot:    address.Longitude,
			DeliveryLatitudeSnapshot:     address.Latitude,
			DeliveryFee:                  0,
			Subtotal:                     dish.Price,
			DiscountAmount:               0,
			DeliveryFeeDiscount:          0,
			TotalAmount:                  dish.Price,
			Status:                       "paid",
			FulfillmentStatus:            "pending",
			Notes:                        pgtype.Text{String: "food safety integration", Valid: true},
		},
		Items: []db.CreateOrderItemParams{
			{
				DishID:    pgtype.Int8{Int64: dish.ID, Valid: true},
				Name:      dish.Name,
				UnitPrice: dish.Price,
				Quantity:  1,
				Subtotal:  dish.Price,
			},
		},
	})
	require.NoError(t, err)

	_, err = integrationPool.Exec(ctx,
		`UPDATE orders SET status = $2, fulfillment_status = $3, completed_at = NOW() WHERE id = $1`,
		result.Order.ID,
		db.OrderStatusCompleted,
		db.FulfillmentStatusCompleted,
	)
	require.NoError(t, err)

	updatedOrder, err := store.GetOrder(ctx, result.Order.ID)
	require.NoError(t, err)

	return updatedOrder
}

func reportFoodSafetyIntegration(t *testing.T, server *api.Server, userID, merchantID, orderID int64) foodSafetyReportAPIResponse {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"merchant_id":    merchantID,
		"order_id":       orderID,
		"incident_type":  "contamination",
		"description":    "顾客出现疑似食物中毒症状，申请平台介入排查。",
		"severity_level": 4,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/food-safety/report", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, integrationTokenMaker, userID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response foodSafetyReportAPIResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	return response
}

func listOperatorFoodSafetyCasesIntegration(t *testing.T, server *api.Server, operatorUserID int64) operatorFoodSafetyCaseListAPIResponse {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "/v1/operator/food-safety/cases?page=1&limit=10", nil)
	require.NoError(t, err)
	addAuthorization(t, req, integrationTokenMaker, operatorUserID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response operatorFoodSafetyCaseListAPIResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	return response
}

func getOperatorFoodSafetyCaseDetailIntegration(t *testing.T, server *api.Server, operatorUserID, caseID int64) operatorFoodSafetyCaseDetailAPIResponse {
	t.Helper()

	url := fmt.Sprintf("/v1/operator/food-safety/cases/%d", caseID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, req, integrationTokenMaker, operatorUserID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response operatorFoodSafetyCaseDetailAPIResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	return response
}

func investigateOperatorFoodSafetyCaseIntegration(t *testing.T, server *api.Server, operatorUserID, caseID int64, investigationReport string) operatorFoodSafetyCaseItem {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"investigation_report": investigationReport,
	})
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/operator/food-safety/cases/%d/investigate", caseID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, integrationTokenMaker, operatorUserID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response operatorFoodSafetyCaseItem
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	return response
}

func resolveOperatorFoodSafetyCaseIntegration(t *testing.T, server *api.Server, operatorUserID, caseID int64, investigationReport, merchantRectificationReport, resolution string) operatorFoodSafetyCaseItem {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"investigation_report":          investigationReport,
		"merchant_rectification_report": merchantRectificationReport,
		"resolution":                    resolution,
	})
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/operator/food-safety/cases/%d/resolve", caseID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, integrationTokenMaker, operatorUserID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response operatorFoodSafetyCaseItem
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	return response
}
