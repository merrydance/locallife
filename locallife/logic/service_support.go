package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now()
}

type DefaultIDGenerator struct{}

func (DefaultIDGenerator) OrderNo(now time.Time) (string, error) {
	dateStr := now.Format("20060102150405")
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	return dateStr + hex.EncodeToString(b)[:6], nil
}

func (DefaultIDGenerator) PickupCode(_ time.Time) (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	num := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	return fmt.Sprintf("%06d", num%1000000), nil
}

func (DefaultIDGenerator) OutTradeNo(prefix string, now time.Time) (string, error) {
	if prefix == "" {
		prefix = "P"
	}
	dateStr := now.Format("20060102150405")
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	return prefix + dateStr + hex.EncodeToString(b), nil
}

func (DefaultIDGenerator) OutRefundNo(now time.Time) (string, error) {
	dateStr := now.Format("20060102150405")
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	return "R" + dateStr + hex.EncodeToString(b), nil
}

type DefaultOrderPolicy struct{}

func (DefaultOrderPolicy) ValidateCreateInput(input CreateOrderCommandInput) error {
	switch input.OrderType {
	case "takeout":
		if input.AddressID == nil {
			return fmt.Errorf("address_id is required for takeout orders")
		}
	case "dine_in":
		if input.TableID == nil {
			return fmt.Errorf("table_id is required for dine-in orders")
		}
	case "reservation":
		if input.ReservationID == nil {
			return fmt.Errorf("reservation_id is required for reservation orders")
		}
	case "takeaway":
	default:
		return fmt.Errorf("unsupported order_type")
	}

	if len(input.Items) == 0 {
		return fmt.Errorf("items must not be empty")
	}

	for _, item := range input.Items {
		if item.DishID == nil && item.ComboID == nil {
			return fmt.Errorf("each item must have either dish_id or combo_id")
		}
		if item.DishID != nil && item.ComboID != nil {
			return fmt.Errorf("each item can only have one of dish_id or combo_id")
		}
	}

	return nil
}

type DefaultPaymentFacade struct {
	paymentClient   wechat.PaymentClientInterface
	ecommerceClient wechat.EcommerceClientInterface
	paymentService  *PaymentOrderService
	ledgerService   *PaymentLedgerService
	combinedService *CombinedPaymentService
}

func NewDefaultPaymentFacade(
	store db.Store,
	paymentClient wechat.PaymentClientInterface,
	ecommerceClient wechat.EcommerceClientInterface,
) PaymentFacade {
	return &DefaultPaymentFacade{
		paymentClient:   paymentClient,
		ecommerceClient: ecommerceClient,
		paymentService:  NewPaymentOrderService(store, paymentClient, ecommerceClient),
		ledgerService:   NewPaymentLedgerService(store),
		combinedService: NewCombinedPaymentService(store, ecommerceClient),
	}
}

func (f *DefaultPaymentFacade) CreatePaymentOrder(ctx context.Context, input CreatePaymentOrderInput) (CreatePaymentOrderResult, error) {
	return f.paymentService.CreatePaymentOrder(ctx, input)
}

func (f *DefaultPaymentFacade) CreateCombinedPaymentOrder(ctx context.Context, input CreateCombinedPaymentOrderInput) (CreateCombinedPaymentOrderResult, error) {
	return f.combinedService.CreateCombinedPaymentOrder(ctx, input)
}

func (f *DefaultPaymentFacade) GetCombinedPaymentOrder(ctx context.Context, input GetCombinedPaymentOrderInput) (GetCombinedPaymentOrderResult, error) {
	return f.combinedService.GetCombinedPaymentOrder(ctx, input)
}

func (f *DefaultPaymentFacade) QueryCombinedPaymentOrder(ctx context.Context, input QueryCombinedPaymentOrderInput) (QueryCombinedPaymentOrderResult, error) {
	return f.combinedService.QueryCombinedPaymentOrder(ctx, input)
}

func (f *DefaultPaymentFacade) CloseCombinedPaymentOrder(ctx context.Context, input CloseCombinedPaymentOrderInput) (CloseCombinedPaymentOrderResult, error) {
	return f.combinedService.CloseCombinedPaymentOrder(ctx, input)
}

func (f *DefaultPaymentFacade) GetPaymentOrder(ctx context.Context, input GetPaymentOrderInput) (GetPaymentOrderResult, error) {
	return f.paymentService.GetPaymentOrder(ctx, input)
}

func (f *DefaultPaymentFacade) QueryPaymentOrder(ctx context.Context, input QueryPaymentOrderInput) (QueryPaymentOrderResult, error) {
	return f.paymentService.QueryPaymentOrder(ctx, input)
}

func (f *DefaultPaymentFacade) ListPaymentOrders(ctx context.Context, input ListPaymentOrdersInput) (ListPaymentOrdersResult, error) {
	return f.paymentService.ListPaymentOrders(ctx, input)
}

func (f *DefaultPaymentFacade) ListPaymentLedger(ctx context.Context, input ListPaymentLedgerInput) (ListPaymentLedgerResult, error) {
	return f.ledgerService.ListPaymentLedger(ctx, input)
}

func (f *DefaultPaymentFacade) ClosePaymentOrder(ctx context.Context, input ClosePaymentOrderInput) (ClosePaymentOrderResult, error) {
	return f.paymentService.ClosePaymentOrder(ctx, input)
}

func (f *DefaultPaymentFacade) CreateRefund(ctx context.Context, req *wechat.RefundRequest) (*wechat.RefundResponse, error) {
	if f.paymentClient == nil {
		return nil, fmt.Errorf("payment client not configured")
	}
	return f.paymentClient.CreateRefund(ctx, req)
}

func (f *DefaultPaymentFacade) CreateEcommerceRefund(ctx context.Context, req *wechat.EcommerceRefundRequest) (*wechat.EcommerceRefundResponse, error) {
	if f.ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured")
	}
	return f.ecommerceClient.CreateEcommerceRefund(ctx, req)
}

func (f *DefaultPaymentFacade) ApplyEcommerceAbnormalRefund(ctx context.Context, req *wechat.EcommerceAbnormalRefundRequest) (*wechat.EcommerceRefundResponse, error) {
	if f.ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured")
	}
	return f.ecommerceClient.ApplyEcommerceAbnormalRefund(ctx, req)
}

func (f *DefaultPaymentFacade) CreateProfitSharingReturn(ctx context.Context, req *wechatcontracts.ProfitSharingReturnRequest) (*wechatcontracts.ProfitSharingReturnResponse, error) {
	if f.ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured")
	}
	return f.ecommerceClient.CreateProfitSharingReturn(ctx, req)
}

func (f *DefaultPaymentFacade) SpMchID() string {
	if f.ecommerceClient == nil {
		return ""
	}
	return f.ecommerceClient.GetSpMchID()
}

// resolveMerchantForUser returns the merchant associated with userID.
// The user may be the merchant owner or an active merchant_staff member.
// This is a named wrapper around store.GetMerchantByOwner; despite that
// name the underlying SQL has always supported staff via a LEFT JOIN on
// merchant_staff. Use this for staff-compatible operations.
func resolveMerchantForUser(ctx context.Context, store db.Store, userID int64) (db.Merchant, error) {
	if selectedMerchantID, ok := SelectedMerchantIDFromContext(ctx); ok {
		merchant, err := store.GetMerchant(ctx, selectedMerchantID)
		if err != nil {
			return db.Merchant{}, err
		}

		if merchant.OwnerUserID == userID {
			return merchant, nil
		}

		hasAccess, err := store.CheckUserHasMerchantAccess(ctx, db.CheckUserHasMerchantAccessParams{
			MerchantID: merchant.ID,
			UserID:     userID,
		})
		if err != nil {
			return db.Merchant{}, err
		}
		if !hasAccess {
			return db.Merchant{}, db.ErrRecordNotFound
		}

		return merchant, nil
	}

	return store.GetMerchantByOwner(ctx, userID)
}
