package logic

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
)

const (
	combinedOutTradePrefix = "CP"
	combinedOrderMaxCount  = 10
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
	combineOutTradeNo, err := generateCombinedOutTradeNo()
	if err != nil {
		return result, fmt.Errorf("generate combine out trade no: %w", err)
	}

	txResult, err := svc.store.CreateCombinedPaymentTx(ctx, db.CreateCombinedPaymentTxParams{
		UserID:            input.UserID,
		OrderIDs:          orderIDs,
		CombineOutTradeNo: combineOutTradeNo,
		ExpiresAt:         expiresAt,
	})
	if err != nil {
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
		CombineOutTradeNo: combineOutTradeNo,
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
			if closeErr := svc.ecommerceClient.CloseCombineOrder(cleanupCtx, combineOutTradeNo, closeSubs); closeErr != nil {
				log.Warn().Err(closeErr).Str("combine_out_trade_no", combineOutTradeNo).Msg("close wechat combine order after prepay update failure")
			}
		}
		return result, fmt.Errorf("update combined payment prepay: %w", err)
	}

	result.CombinedPayment = updatedCombined
	result.SubOrders = subOrders
	result.PayParams = payParams
	return result, nil
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

	return GetCombinedPaymentOrderResult{CombinedPayment: combinedRow}, nil
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
