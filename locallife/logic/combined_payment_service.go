package logic

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
)

const (
	combinedOutTradePrefix = "CP"
	combinedOrderMaxCount  = 50
)

// CombinedPaymentService encapsulates combined payment order logic.
type CombinedPaymentService struct {
	store           db.Store
	ecommerceClient wechat.EcommerceClientInterface
	now             func() time.Time
}

// NewCombinedPaymentService creates a combined payment service.
func NewCombinedPaymentService(store db.Store, ecommerceClient wechat.EcommerceClientInterface) *CombinedPaymentService {
	return &CombinedPaymentService{
		store:           store,
		ecommerceClient: ecommerceClient,
		now:             time.Now,
	}
}

// CreateCombinedPaymentOrderInput defines the input for combined payment creation.
type CreateCombinedPaymentOrderInput struct {
	UserID   int64
	OrderIDs []int64
	ClientIP string
}

// CombinedSubOrder describes the combined payment sub order.
type CombinedSubOrder struct {
	OrderID        int64
	PaymentOrderID int64
	MerchantID     int64
	SubMchID       string
	Amount         int64
	OutTradeNo     string
	Description    string
}

// CreateCombinedPaymentOrderResult holds the result for combined payment creation.
type CreateCombinedPaymentOrderResult struct {
	CombinedPayment db.CombinedPaymentOrder
	SubOrders       []CombinedSubOrder
	PayParams       *wechat.JSAPIPayParams
}

type GetCombinedPaymentOrderInput struct {
	UserID            int64
	CombinedPaymentID int64
}

type GetCombinedPaymentOrderResult struct {
	CombinedPayment db.GetCombinedPaymentOrderWithSubOrdersRow
	PayParams       *wechat.JSAPIPayParams
}

type QueryCombinedPaymentOrderInput struct {
	UserID            int64
	CombinedPaymentID int64
}

type QueryCombinedPaymentWechatAmount struct {
	TotalAmount   int64
	PayerAmount   int64
	Currency      string
	PayerCurrency string
}

type QueryCombinedPaymentWechatSubOrder struct {
	MchID         string
	SubMchID      string
	SubAppID      string
	SubOpenID     string
	OutTradeNo    string
	TransactionID string
	TradeType     string
	TradeState    string
	BankType      string
	Attach        string
	SuccessTime   string
	Amount        QueryCombinedPaymentWechatAmount
}

type QueryCombinedPaymentWechatOrder struct {
	CombineOutTradeNo   string
	AggregateTradeState string
	SubOrders           []QueryCombinedPaymentWechatSubOrder
}

type QueryCombinedPaymentOrderResult struct {
	CombinedPayment db.GetCombinedPaymentOrderWithSubOrdersRow
	PayParams       *wechat.JSAPIPayParams
	WechatOrder     *QueryCombinedPaymentWechatOrder
}

type CloseCombinedPaymentOrderInput struct {
	UserID            int64
	CombinedPaymentID int64
}

type CloseCombinedPaymentOrderResult struct {
	CombinedPayment db.CombinedPaymentOrder
	SubOrders       []CombinedSubOrder
}

type combinedSubOrderPayload struct {
	OrderID             int64  `json:"order_id"`
	PaymentOrderID      int64  `json:"payment_order_id"`
	MerchantID          int64  `json:"merchant_id"`
	SubMchID            string `json:"sub_mch_id"`
	Amount              int64  `json:"amount"`
	OutTradeNo          string `json:"out_trade_no"`
	Description         string `json:"description"`
	ProfitSharingStatus string `json:"profit_sharing_status,omitempty"`
	MerchantName        string `json:"merchant_name,omitempty"`
	MerchantLogo        string `json:"merchant_logo,omitempty"`
	OrderNo             string `json:"order_no,omitempty"`
}

// CreateCombinedPaymentOrder creates a combined payment order.
func (svc *CombinedPaymentService) CreateCombinedPaymentOrder(ctx context.Context, input CreateCombinedPaymentOrderInput) (CreateCombinedPaymentOrderResult, error) {
	var result CreateCombinedPaymentOrderResult

	if svc.ecommerceClient == nil {
		return result, fmt.Errorf("ecommerce client: not configured")
	}

	orderIDs := dedupePositiveIDs(input.OrderIDs)
	sort.Slice(orderIDs, func(i, j int) bool { return orderIDs[i] < orderIDs[j] })
	if len(orderIDs) == 0 {
		return result, NewRequestError(http.StatusBadRequest, errors.New("invalid order ids"))
	}
	if len(orderIDs) > combinedOrderMaxCount {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("too many orders, max %d", combinedOrderMaxCount))
	}

	user, err := svc.store.GetUser(ctx, input.UserID)
	if err != nil {
		return result, fmt.Errorf("get user: %w", err)
	}
	if user.WechatOpenid == "" {
		return result, NewRequestError(http.StatusBadRequest, errors.New("wechat openid not found"))
	}

	expiresAt := svc.now().Add(30 * time.Minute)

	var txResult db.CreateCombinedPaymentTxResult
	for attempt := 1; attempt <= concurrentPaymentRetry; attempt++ {
		combineOutTradeNo, genErr := generateCombinedOutTradeNo()
		if genErr != nil {
			return result, fmt.Errorf("generate combine out trade no: %w", genErr)
		}

		txResult, err = svc.store.CreateCombinedPaymentTx(ctx, db.CreateCombinedPaymentTxParams{
			UserID:            input.UserID,
			OrderIDs:          orderIDs,
			CombineOutTradeNo: combineOutTradeNo,
			ExpiresAt:         expiresAt,
		})
		if err == nil {
			break
		}

		if errors.Is(err, db.ErrOrderPendingPaymentConflict) {
			resolved, handled, resolveErr := svc.resolveConcurrentCombinedPayment(ctx, input.UserID, orderIDs)
			if resolveErr != nil {
				return result, resolveErr
			}
			if handled {
				return resolved, nil
			}
			if attempt < concurrentPaymentRetry {
				continue
			}
			return result, NewRequestError(http.StatusConflict, errors.New("combined payment order is being recreated, please retry"))
		}

		if mapped := mapCombinedPaymentError(err); mapped != nil {
			return result, mapped
		}
		return result, fmt.Errorf("create combined payment: %w", err)
	}

	wechatSubOrders := make([]wechat.SubOrder, 0, len(txResult.OrderInfos))
	subOrders := make([]CombinedSubOrder, 0, len(txResult.OrderInfos))
	for _, info := range txResult.OrderInfos {
		description := fmt.Sprintf("%s - Order Payment", info.Merchant.Name)
		wechatSubOrders = append(wechatSubOrders, wechat.SubOrder{
			SubMchID:    info.PaymentConfig.SubMchID,
			Amount:      info.PaymentOrder.Amount,
			OutTradeNo:  info.PaymentOrder.OutTradeNo,
			Description: description,
			Attach:      info.PaymentOrder.Attach.String,
		})
		subOrders = append(subOrders, CombinedSubOrder{
			OrderID:        info.Order.ID,
			PaymentOrderID: info.PaymentOrder.ID,
			MerchantID:     info.Order.MerchantID,
			SubMchID:       info.PaymentConfig.SubMchID,
			Amount:         info.PaymentOrder.Amount,
			OutTradeNo:     info.PaymentOrder.OutTradeNo,
			Description:    description,
		})
	}

	combineResp, payParams, err := svc.ecommerceClient.CreateCombineOrder(ctx, &wechat.CombineOrderRequest{
		CombineOutTradeNo: txResult.CombinedPaymentOrder.CombineOutTradeNo,
		SubOrders:         wechatSubOrders,
		PayerOpenID:       user.WechatOpenid,
		ExpireTime:        expiresAt,
		SceneInfo: &wechat.CombineSceneInfo{
			PayerClientIP: input.ClientIP,
		},
	})
	if err != nil {
		// 微信下单失败：将本地创建的合单和子支付单标记为 closed，避免僵尸记录
		// 超时任务（payment_timeout）会兜底，但主动清理能减少脏数据积压
		cleanupCtx := context.Background() // 使用新 context，避免父 ctx 已取消时清理失败
		for _, info := range txResult.OrderInfos {
			_, _ = svc.store.UpdatePaymentOrderToClosed(cleanupCtx, info.PaymentOrder.ID)
		}
		_, _ = svc.store.UpdateCombinedPaymentOrderToClosed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		return result, fmt.Errorf("create combine order: %w", err)
	}
	if combineResp == nil || strings.TrimSpace(combineResp.PrepayID) == "" {
		cleanupCtx := context.Background()
		for _, info := range txResult.OrderInfos {
			_, _ = svc.store.UpdatePaymentOrderToClosed(cleanupCtx, info.PaymentOrder.ID)
		}
		_, _ = svc.store.UpdateCombinedPaymentOrderToClosed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		return result, fmt.Errorf("create combine order: empty prepay id")
	}

	updatedCombined, err := svc.store.UpdateCombinedPaymentOrderPrepay(ctx, db.UpdateCombinedPaymentOrderPrepayParams{
		ID:       txResult.CombinedPaymentOrder.ID,
		PrepayID: pgtype.Text{String: combineResp.PrepayID, Valid: true},
	})
	if err != nil {
		// 微信合单下单已成功但本地更新失败。
		// 与普通支付链路保持一致：标记本地子单和合单为 failed，并尝试关闭微信合单。
		// 微信合单即使不主动关闭，也会在过期后自动关闭（用户无 prepay_id 无法调起支付）。
		cleanupCtx := context.Background()
		for _, info := range txResult.OrderInfos {
			_, _ = svc.store.UpdatePaymentOrderToFailed(cleanupCtx, info.PaymentOrder.ID)
		}
		_, _ = svc.store.UpdateCombinedPaymentOrderToFailed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		if svc.ecommerceClient != nil {
			closeSubs := make([]wechat.SubOrderClose, 0, len(wechatSubOrders))
			for _, sub := range wechatSubOrders {
				closeSubs = append(closeSubs, wechat.SubOrderClose{MchID: sub.MchID, SubMchID: sub.SubMchID, SubAppID: sub.SubAppID, OutTradeNo: sub.OutTradeNo})
			}
			if closeErr := svc.ecommerceClient.CloseCombineOrder(cleanupCtx, txResult.CombinedPaymentOrder.CombineOutTradeNo, closeSubs); closeErr != nil {
				log.Warn().Err(closeErr).Str("combine_out_trade_no", txResult.CombinedPaymentOrder.CombineOutTradeNo).Msg("close wechat combine order after prepay update failure")
			}
		}
		return result, fmt.Errorf("update combined payment prepay: %w", err)
	}

	result.CombinedPayment = updatedCombined
	result.SubOrders = subOrders
	result.PayParams = payParams
	return result, nil
}

func (svc *CombinedPaymentService) resolveConcurrentCombinedPayment(
	ctx context.Context,
	userID int64,
	orderIDs []int64,
) (CreateCombinedPaymentOrderResult, bool, error) {
	var result CreateCombinedPaymentOrderResult

	for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
		combinedPaymentID := int64(0)
		retryLookup := false

		for _, orderID := range orderIDs {
			paymentOrder, err := svc.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
				OrderID:      pgtype.Int8{Int64: orderID, Valid: true},
				BusinessType: businessTypeOrder,
			})
			if err != nil {
				if errors.Is(err, db.ErrRecordNotFound) {
					if attempt < outTradeNoMaxRetry {
						retryLookup = true
						break
					}
					return result, false, nil
				}
				return result, true, fmt.Errorf("get latest payment order after concurrent combined conflict: %w", err)
			}

			if paymentOrder.Status != paymentStatusPending || !paymentOrder.CombinedPaymentID.Valid {
				return result, false, nil
			}

			if combinedPaymentID == 0 {
				combinedPaymentID = paymentOrder.CombinedPaymentID.Int64
				continue
			}

			if paymentOrder.CombinedPaymentID.Int64 != combinedPaymentID {
				return result, true, NewRequestError(http.StatusConflict, errors.New("orders are already preparing in another combined payment, please retry"))
			}
		}

		if retryLookup {
			if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
				return result, true, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
			}
			continue
		}

		if combinedPaymentID == 0 {
			return result, false, nil
		}

		combinedRow, err := svc.store.GetCombinedPaymentOrderWithSubOrders(ctx, combinedPaymentID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				if attempt < outTradeNoMaxRetry {
					if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
						return result, true, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
					}
					continue
				}
				return result, false, nil
			}
			return result, true, fmt.Errorf("get combined payment order after concurrent conflict: %w", err)
		}

		if combinedRow.UserID != userID {
			return result, true, NewRequestError(http.StatusForbidden, errors.New("combined payment order does not belong to you"))
		}
		if combinedRow.Status != paymentStatusPending {
			return result, false, nil
		}

		matches, matchErr := combinedPaymentMatchesOrders(combinedRow.SubOrders, orderIDs)
		if matchErr != nil {
			return result, true, fmt.Errorf("parse concurrent combined payment sub orders: %w", matchErr)
		}
		if !matches {
			return result, true, NewRequestError(http.StatusConflict, errors.New("orders are already preparing in another combined payment, please retry"))
		}

		payParams, signErr := svc.signExistingCombinedPaymentOrder(combinedRow)
		if signErr == nil && payParams != nil {
			result, buildErr := buildExistingCombinedPaymentResult(combinedRow, payParams)
			if buildErr != nil {
				return CreateCombinedPaymentOrderResult{}, true, buildErr
			}
			return result, true, nil
		}

		if attempt == outTradeNoMaxRetry {
			return result, true, NewRequestError(http.StatusConflict, errors.New("combined payment order is still preparing, please retry"))
		}

		if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
			return result, true, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
		}
	}

	return result, false, nil
}

func (svc *CombinedPaymentService) GetCombinedPaymentOrder(ctx context.Context, input GetCombinedPaymentOrderInput) (GetCombinedPaymentOrderResult, error) {
	combinedRow, err := svc.store.GetCombinedPaymentOrderWithSubOrders(ctx, input.CombinedPaymentID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return GetCombinedPaymentOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("combined payment order not found"))
		}
		return GetCombinedPaymentOrderResult{}, err
	}

	if combinedRow.UserID != input.UserID {
		return GetCombinedPaymentOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("combined payment order does not belong to you"))
	}

	return GetCombinedPaymentOrderResult{
		CombinedPayment: combinedRow,
	}, nil
}

func (svc *CombinedPaymentService) QueryCombinedPaymentOrder(ctx context.Context, input QueryCombinedPaymentOrderInput) (QueryCombinedPaymentOrderResult, error) {
	if svc.ecommerceClient == nil {
		return QueryCombinedPaymentOrderResult{}, fmt.Errorf("ecommerce client: not configured")
	}

	detail, err := svc.GetCombinedPaymentOrder(ctx, GetCombinedPaymentOrderInput{
		UserID:            input.UserID,
		CombinedPaymentID: input.CombinedPaymentID,
	})
	if err != nil {
		return QueryCombinedPaymentOrderResult{}, err
	}

	queryResp, err := svc.ecommerceClient.QueryCombineOrder(ctx, detail.CombinedPayment.CombineOutTradeNo)
	if err != nil {
		return QueryCombinedPaymentOrderResult{}, fmt.Errorf("query combine order: %w", err)
	}
	wechatOrder := mapCombinedWechatOrder(queryResp)

	var payParams *wechat.JSAPIPayParams
	if svc.shouldExposeCombinedPaymentPayParams(detail.CombinedPayment, wechatOrder) {
		payParams, err = svc.signExistingCombinedPaymentOrder(detail.CombinedPayment)
		if err != nil {
			return QueryCombinedPaymentOrderResult{}, fmt.Errorf("sign combined payment order: %w", err)
		}
	}

	return QueryCombinedPaymentOrderResult{
		CombinedPayment: detail.CombinedPayment,
		PayParams:       payParams,
		WechatOrder:     wechatOrder,
	}, nil
}

func (svc *CombinedPaymentService) CloseCombinedPaymentOrder(ctx context.Context, input CloseCombinedPaymentOrderInput) (CloseCombinedPaymentOrderResult, error) {
	if svc.ecommerceClient == nil {
		return CloseCombinedPaymentOrderResult{}, fmt.Errorf("ecommerce client: not configured")
	}

	combinedRow, err := svc.store.GetCombinedPaymentOrderWithSubOrders(ctx, input.CombinedPaymentID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return CloseCombinedPaymentOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("combined payment order not found"))
		}
		return CloseCombinedPaymentOrderResult{}, err
	}

	if combinedRow.UserID != input.UserID {
		return CloseCombinedPaymentOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("combined payment order does not belong to you"))
	}

	if combinedRow.Status != paymentStatusPending {
		return CloseCombinedPaymentOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("only pending combined payment orders can be closed"))
	}

	var subPayloads []combinedSubOrderPayload
	if err := json.Unmarshal(combinedRow.SubOrders, &subPayloads); err != nil {
		return CloseCombinedPaymentOrderResult{}, err
	}

	closeSubs := make([]wechat.SubOrderClose, 0, len(subPayloads))
	for _, sub := range subPayloads {
		if sub.SubMchID == "" || sub.OutTradeNo == "" {
			continue
		}
		closeSubs = append(closeSubs, wechat.SubOrderClose{SubMchID: sub.SubMchID, OutTradeNo: sub.OutTradeNo})
	}
	if len(closeSubs) == 0 {
		return CloseCombinedPaymentOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("no sub orders available to close"))
	}

	if err := svc.ecommerceClient.CloseCombineOrder(ctx, combinedRow.CombineOutTradeNo, closeSubs); err != nil {
		return CloseCombinedPaymentOrderResult{}, err
	}

	// 收集子单 OutTradeNo 用于事务关闭
	subOutTradeNos := make([]string, 0, len(subPayloads))
	for _, sub := range subPayloads {
		if sub.OutTradeNo != "" {
			subOutTradeNos = append(subOutTradeNos, sub.OutTradeNo)
		}
	}

	// 在单个事务中原子关闭合单 + 所有子支付单
	txResult, err := svc.store.CloseCombinedPaymentOrderTx(ctx, db.CloseCombinedPaymentOrderTxParams{
		CombinedPaymentOrderID: combinedRow.ID,
		SubOrderOutTradeNos:    subOutTradeNos,
	})
	if err != nil {
		return CloseCombinedPaymentOrderResult{}, err
	}

	resultSubs := make([]CombinedSubOrder, 0, len(subPayloads))
	for _, sub := range subPayloads {
		resultSubs = append(resultSubs, CombinedSubOrder{
			OrderID:        sub.OrderID,
			PaymentOrderID: sub.PaymentOrderID,
			MerchantID:     sub.MerchantID,
			SubMchID:       sub.SubMchID,
			Amount:         sub.Amount,
			OutTradeNo:     sub.OutTradeNo,
			Description:    sub.Description,
		})
	}

	return CloseCombinedPaymentOrderResult{CombinedPayment: txResult.CombinedPaymentOrder, SubOrders: resultSubs}, nil
}

func dedupePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func generateCombinedOutTradeNo() (string, error) {
	now := time.Now()
	dateStr := now.Format("20060102150405")

	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	buf := fmt.Sprintf("%08d", int(b[0])*1000000+int(b[1])*10000+int(b[2])*100+int(b[3]))
	return combinedOutTradePrefix + dateStr + buf[:8], nil
}

func buildExistingCombinedPaymentResult(combinedRow db.GetCombinedPaymentOrderWithSubOrdersRow, payParams *wechat.JSAPIPayParams) (CreateCombinedPaymentOrderResult, error) {
	subOrders, err := decodeCombinedSubOrders(combinedRow.SubOrders)
	if err != nil {
		return CreateCombinedPaymentOrderResult{}, err
	}

	return CreateCombinedPaymentOrderResult{
		CombinedPayment: db.CombinedPaymentOrder{
			ID:                combinedRow.ID,
			UserID:            combinedRow.UserID,
			CombineOutTradeNo: combinedRow.CombineOutTradeNo,
			TotalAmount:       combinedRow.TotalAmount,
			PrepayID:          combinedRow.PrepayID,
			TransactionID:     combinedRow.TransactionID,
			Status:            combinedRow.Status,
			PaidAt:            combinedRow.PaidAt,
			CreatedAt:         combinedRow.CreatedAt,
			ExpiresAt:         combinedRow.ExpiresAt,
		},
		SubOrders: subOrders,
		PayParams: payParams,
	}, nil
}

func decodeCombinedSubOrders(raw json.RawMessage) ([]CombinedSubOrder, error) {
	var payloads []combinedSubOrderPayload
	if err := json.Unmarshal(raw, &payloads); err != nil {
		return nil, err
	}

	subOrders := make([]CombinedSubOrder, 0, len(payloads))
	for _, payload := range payloads {
		subOrders = append(subOrders, CombinedSubOrder{
			OrderID:        payload.OrderID,
			PaymentOrderID: payload.PaymentOrderID,
			MerchantID:     payload.MerchantID,
			SubMchID:       payload.SubMchID,
			Amount:         payload.Amount,
			OutTradeNo:     payload.OutTradeNo,
			Description:    payload.Description,
		})
	}

	return subOrders, nil
}

func combinedPaymentMatchesOrders(raw json.RawMessage, orderIDs []int64) (bool, error) {
	var payloads []combinedSubOrderPayload
	if err := json.Unmarshal(raw, &payloads); err != nil {
		return false, err
	}

	if len(payloads) != len(orderIDs) {
		return false, nil
	}

	current := make([]int64, 0, len(payloads))
	for _, payload := range payloads {
		current = append(current, payload.OrderID)
	}
	requested := append([]int64(nil), orderIDs...)
	sort.Slice(current, func(i, j int) bool { return current[i] < current[j] })
	sort.Slice(requested, func(i, j int) bool { return requested[i] < requested[j] })

	for index := range requested {
		if current[index] != requested[index] {
			return false, nil
		}
	}

	return true, nil
}

func (svc *CombinedPaymentService) signExistingCombinedPaymentOrder(combinedRow db.GetCombinedPaymentOrderWithSubOrdersRow) (*wechat.JSAPIPayParams, error) {
	if !svc.canResumeCombinedPaymentLocally(combinedRow) {
		return nil, nil
	}
	if svc.ecommerceClient == nil {
		return nil, nil
	}
	payParams, err := svc.ecommerceClient.GenerateJSAPIPayParams(combinedRow.PrepayID.String)
	if err != nil {
		return nil, err
	}
	return payParams, nil
}

func (svc *CombinedPaymentService) canResumeCombinedPaymentLocally(combinedRow db.GetCombinedPaymentOrderWithSubOrdersRow) bool {
	if combinedRow.Status != paymentStatusPending {
		return false
	}
	if !combinedRow.PrepayID.Valid || strings.TrimSpace(combinedRow.PrepayID.String) == "" {
		return false
	}
	if combinedRow.ExpiresAt.Valid && !svc.now().Before(combinedRow.ExpiresAt.Time) {
		return false
	}
	return true
}

func (svc *CombinedPaymentService) shouldExposeCombinedPaymentPayParams(combinedRow db.GetCombinedPaymentOrderWithSubOrdersRow, wechatOrder *QueryCombinedPaymentWechatOrder) bool {
	if combinedRow.Status != paymentStatusPending {
		return false
	}
	if !combinedRow.PrepayID.Valid || strings.TrimSpace(combinedRow.PrepayID.String) == "" {
		return false
	}
	if combinedRow.ExpiresAt.Valid && !svc.now().Before(combinedRow.ExpiresAt.Time) {
		return false
	}
	if wechatOrder == nil {
		return false
	}
	return wechatOrder.AggregateTradeState == paymentStatusPending
}

func mapCombinedWechatOrder(queryResp *wechat.CombineQueryResponse) *QueryCombinedPaymentWechatOrder {
	if queryResp == nil {
		return nil
	}

	subOrders := make([]QueryCombinedPaymentWechatSubOrder, 0, len(queryResp.SubOrders))
	tradeStates := make([]string, 0, len(queryResp.SubOrders))
	for _, subOrder := range queryResp.SubOrders {
		tradeStates = append(tradeStates, subOrder.TradeState)
		subOrders = append(subOrders, QueryCombinedPaymentWechatSubOrder{
			MchID:         subOrder.MchID,
			SubMchID:      subOrder.SubMchID,
			SubAppID:      subOrder.SubAppID,
			SubOpenID:     subOrder.SubOpenID,
			OutTradeNo:    subOrder.OutTradeNo,
			TransactionID: subOrder.TransactionID,
			TradeType:     subOrder.TradeType,
			TradeState:    subOrder.TradeState,
			BankType:      subOrder.BankType,
			Attach:        subOrder.Attach,
			SuccessTime:   subOrder.SuccessTime,
			Amount: QueryCombinedPaymentWechatAmount{
				TotalAmount:   subOrder.Amount.TotalAmount,
				PayerAmount:   subOrder.Amount.PayerAmount,
				Currency:      subOrder.Amount.Currency,
				PayerCurrency: subOrder.Amount.PayerCurrency,
			},
		})
	}

	return &QueryCombinedPaymentWechatOrder{
		CombineOutTradeNo:   queryResp.CombineOutTradeNo,
		AggregateTradeState: aggregateCombinedTradeState(tradeStates),
		SubOrders:           subOrders,
	}
}

func aggregateCombinedTradeState(tradeStates []string) string {
	if len(tradeStates) == 0 {
		return "unknown"
	}

	allSuccess := true
	allClosed := true
	allRefund := true
	allPayError := true
	hasNotPay := false
	hasSuccess := false

	for _, tradeState := range tradeStates {
		normalized := strings.ToUpper(strings.TrimSpace(tradeState))
		switch normalized {
		case "SUCCESS":
			hasSuccess = true
			allClosed = false
			allRefund = false
			allPayError = false
		case "CLOSED":
			allSuccess = false
			allRefund = false
			allPayError = false
		case "REFUND":
			allSuccess = false
			allClosed = false
			allPayError = false
		case "PAYERROR":
			allSuccess = false
			allClosed = false
			allRefund = false
		case "NOTPAY":
			hasNotPay = true
			allSuccess = false
			allClosed = false
			allRefund = false
			allPayError = false
		default:
			allSuccess = false
			allClosed = false
			allRefund = false
			allPayError = false
		}
	}

	switch {
	case allSuccess:
		return "paid"
	case allClosed:
		return "closed"
	case allRefund:
		return "refunded"
	case allPayError:
		return "failed"
	case hasSuccess:
		return "partial"
	case hasNotPay:
		return "pending"
	default:
		return "mixed"
	}
}

func mapCombinedPaymentError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	switch {
	case containsAny(msg, []string{"does not belong to user"}):
		return NewRequestError(http.StatusForbidden, errors.New("order does not belong to you"))
	case containsAny(msg, []string{"status is", "expect pending"}):
		return NewRequestError(http.StatusBadRequest, errors.New("order is not in pending status"))
	case containsAny(msg, []string{"payment config invalid"}):
		return NewRequestError(http.StatusBadRequest, errors.New("merchant payment config invalid"))
	case containsAny(msg, []string{"has", "payment order"}):
		return NewRequestError(http.StatusBadRequest, errors.New("order has active payment order"))
	default:
		return nil
	}
}

func containsAny(msg string, needles []string) bool {
	for _, needle := range needles {
		if needle == "" {
			continue
		}
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}
