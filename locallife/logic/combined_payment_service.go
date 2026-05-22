package logic

import (
	"context"
	"errors"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

const (
	combinedOutTradePrefix = "CP"
	combinedOrderMaxCount  = 50
)

// CombinedPaymentService keeps the combined-payment API boundary explicit.
// Main-business payment is Baofu-only now, and Baofu combined payment has not
// been enabled, so creation/query/close fail closed.
type CombinedPaymentService struct {
	store db.Store
	now   func() time.Time
}

func NewCombinedPaymentService(store db.Store, _ any) *CombinedPaymentService {
	return NewCombinedPaymentServiceWithBaofuUnsupported(store)
}

func NewCombinedPaymentServiceWithBaofuUnsupported(store db.Store) *CombinedPaymentService {
	return &CombinedPaymentService{
		store: store,
		now:   time.Now,
	}
}

type CreateCombinedPaymentOrderInput struct {
	UserID   int64
	OrderIDs []int64
	ClientIP string
}

type CombinedSubOrder struct {
	OrderID        int64
	PaymentOrderID int64
	MerchantID     int64
	SubMchID       string
	Amount         int64
	OutTradeNo     string
	Description    string
}

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
	MchID           string
	SubMchID        string
	SubAppID        string
	SubOpenID       string
	OutTradeNo      string
	TransactionID   string
	TradeType       string
	TradeState      string
	BankType        string
	Attach          string
	SuccessTime     string
	PromotionDetail []wechatcontracts.PartnerPromotionDetail
	Amount          QueryCombinedPaymentWechatAmount
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

func (svc *CombinedPaymentService) CreateCombinedPaymentOrder(context.Context, CreateCombinedPaymentOrderInput) (CreateCombinedPaymentOrderResult, error) {
	return CreateCombinedPaymentOrderResult{}, combinedPaymentUnsupportedError()
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

func (svc *CombinedPaymentService) QueryCombinedPaymentOrder(context.Context, QueryCombinedPaymentOrderInput) (QueryCombinedPaymentOrderResult, error) {
	return QueryCombinedPaymentOrderResult{}, combinedPaymentUnsupportedError()
}

func (svc *CombinedPaymentService) CloseCombinedPaymentOrder(context.Context, CloseCombinedPaymentOrderInput) (CloseCombinedPaymentOrderResult, error) {
	return CloseCombinedPaymentOrderResult{}, combinedPaymentUnsupportedError()
}

func combinedPaymentUnsupportedError() error {
	return NewRequestError(http.StatusServiceUnavailable, errors.New("宝付合单支付暂未开通，请分开支付"))
}
