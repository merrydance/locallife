package logic

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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

// CreateCombinedPaymentOrder creates a combined payment order.
func (svc *CombinedPaymentService) CreateCombinedPaymentOrder(ctx context.Context, input CreateCombinedPaymentOrderInput) (CreateCombinedPaymentOrderResult, error) {
	var result CreateCombinedPaymentOrderResult

	if svc.ecommerceClient == nil {
		return result, NewRequestError(http.StatusInternalServerError, errors.New("ecommerce client not configured"))
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
	combineOutTradeNo := generateCombinedOutTradeNo()

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
			MchID:       info.PaymentConfig.SubMchID,
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
		return result, fmt.Errorf("create combine order: %w", err)
	}

	updatedCombined, err := svc.store.UpdateCombinedPaymentOrderPrepay(ctx, db.UpdateCombinedPaymentOrderPrepayParams{
		ID:       txResult.CombinedPaymentOrder.ID,
		PrepayID: pgtype.Text{String: combineResp.PrepayID, Valid: true},
	})
	if err != nil {
		return result, fmt.Errorf("update combined payment prepay: %w", err)
	}

	result.CombinedPayment = updatedCombined
	result.SubOrders = subOrders
	result.PayParams = payParams
	return result, nil
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

func generateCombinedOutTradeNo() string {
	now := time.Now()
	dateStr := now.Format("20060102150405")

	b := make([]byte, 4)
	_, _ = rand.Read(b)
	buf := fmt.Sprintf("%08d", int(b[0])*1000000+int(b[1])*10000+int(b[2])*100+int(b[3]))
	return combinedOutTradePrefix + dateStr + buf[:8]
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
