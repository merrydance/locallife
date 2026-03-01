package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
)

const paymentTypeProfitSharing = "profit_sharing"

type RefundService struct {
	store         db.Store
	paymentFacade PaymentFacade
	taskScheduler TaskScheduler
	clock         Clock
	idGenerator   IDGenerator
}

func NewRefundService(
	store db.Store,
	paymentFacade PaymentFacade,
	taskScheduler TaskScheduler,
	clock Clock,
	idGenerator IDGenerator,
) *RefundService {
	if clock == nil {
		clock = SystemClock{}
	}
	if idGenerator == nil {
		idGenerator = DefaultIDGenerator{}
	}

	return &RefundService{
		store:         store,
		paymentFacade: paymentFacade,
		taskScheduler: taskScheduler,
		clock:         clock,
		idGenerator:   idGenerator,
	}
}

func (s *RefundService) CreateRefundOrder(ctx context.Context, input CreateRefundOrderInput) (CreateRefundOrderResult, error) {
	merchant, err := s.store.GetMerchantByOwner(ctx, input.ActorUserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return CreateRefundOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("you are not a merchant"))
		}
		return CreateRefundOrderResult{}, err
	}

	paymentOrder, err := s.store.GetPaymentOrder(ctx, input.PaymentOrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return CreateRefundOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("payment order not found"))
		}
		return CreateRefundOrderResult{}, err
	}

	if paymentOrder.Status != "paid" {
		return CreateRefundOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("payment order is not paid"))
	}

	if !paymentOrder.OrderID.Valid {
		return CreateRefundOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("payment order has no associated order"))
	}

	order, err := s.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
	if err != nil {
		return CreateRefundOrderResult{}, err
	}

	if order.MerchantID != merchant.ID {
		return CreateRefundOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to your merchant"))
	}

	if input.RefundAmount > paymentOrder.Amount {
		return CreateRefundOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("refund amount exceeds payment amount"))
	}

	outRefundNo := s.idGenerator.OutRefundNo(s.clock.Now())
	refundOrder, err := s.store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
		PaymentOrderID: input.PaymentOrderID,
		RefundType:     input.RefundType,
		RefundAmount:   input.RefundAmount,
		RefundReason:   pgtype.Text{String: input.RefundReason, Valid: input.RefundReason != ""},
		OutRefundNo:    outRefundNo,
		Status:         "pending",
	})
	if err != nil {
		return CreateRefundOrderResult{}, err
	}

	if paymentOrder.PaymentType == paymentTypeProfitSharing {
		if err := s.processProfitSharingRefund(ctx, paymentOrder, order, refundOrder, input); err != nil {
			return CreateRefundOrderResult{}, err
		}

		latest, getErr := s.store.GetRefundOrder(ctx, refundOrder.ID)
		if getErr == nil {
			refundOrder = latest
		}
		return CreateRefundOrderResult{RefundOrder: refundOrder}, nil
	}

	if s.paymentFacade != nil {
		wxRefund, refundErr := s.paymentFacade.CreateRefund(ctx, &wechat.RefundRequest{
			OutTradeNo:   paymentOrder.OutTradeNo,
			OutRefundNo:  outRefundNo,
			Reason:       input.RefundReason,
			RefundAmount: input.RefundAmount,
			TotalAmount:  paymentOrder.Amount,
		})
		if refundErr != nil {
			_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			return CreateRefundOrderResult{}, fmt.Errorf("wechat refund: %w", refundErr)
		}

		switch wxRefund.Status {
		case wechat.RefundStatusSuccess:
			_, _ = s.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
			_, _ = s.store.UpdatePaymentOrderToRefunded(ctx, paymentOrder.ID)
		case wechat.RefundStatusProcessing:
			_, _ = s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
				ID:       refundOrder.ID,
				RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: true},
			})
		}
	}

	latest, getErr := s.store.GetRefundOrder(ctx, refundOrder.ID)
	if getErr == nil {
		refundOrder = latest
	}

	return CreateRefundOrderResult{RefundOrder: refundOrder}, nil
}

func (s *RefundService) GetRefundOrder(ctx context.Context, input GetRefundOrderInput) (GetRefundOrderResult, error) {
	refundOrder, err := s.store.GetRefundOrder(ctx, input.RefundID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return GetRefundOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("refund order not found"))
		}
		return GetRefundOrderResult{}, err
	}

	paymentOrder, err := s.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return GetRefundOrderResult{}, err
	}

	if paymentOrder.UserID == input.ActorUserID {
		return GetRefundOrderResult{RefundOrder: refundOrder}, nil
	}

	if paymentOrder.OrderID.Valid {
		order, getOrderErr := s.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
		if getOrderErr == nil {
			merchant, getMerchantErr := s.store.GetMerchantByOwner(ctx, input.ActorUserID)
			if getMerchantErr == nil && order.MerchantID == merchant.ID {
				return GetRefundOrderResult{RefundOrder: refundOrder}, nil
			}
		}
	}

	return GetRefundOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("access denied"))
}

func (s *RefundService) ListRefundOrdersByPayment(ctx context.Context, input ListRefundOrdersByPaymentInput) (ListRefundOrdersByPaymentResult, error) {
	paymentOrder, err := s.store.GetPaymentOrder(ctx, input.PaymentOrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ListRefundOrdersByPaymentResult{}, NewRequestError(http.StatusNotFound, errors.New("payment order not found"))
		}
		return ListRefundOrdersByPaymentResult{}, err
	}

	if paymentOrder.UserID != input.ActorUserID {
		if paymentOrder.OrderID.Valid {
			order, getOrderErr := s.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
			if getOrderErr != nil {
				return ListRefundOrdersByPaymentResult{}, getOrderErr
			}
			merchant, getMerchantErr := s.store.GetMerchantByOwner(ctx, input.ActorUserID)
			if getMerchantErr != nil || order.MerchantID != merchant.ID {
				return ListRefundOrdersByPaymentResult{}, NewRequestError(http.StatusForbidden, errors.New("access denied"))
			}
		} else {
			return ListRefundOrdersByPaymentResult{}, NewRequestError(http.StatusForbidden, errors.New("access denied"))
		}
	}

	refundOrders, err := s.store.ListRefundOrdersByPaymentOrder(ctx, input.PaymentOrderID)
	if err != nil {
		return ListRefundOrdersByPaymentResult{}, err
	}

	return ListRefundOrdersByPaymentResult{RefundOrders: refundOrders}, nil
}

func (s *RefundService) ListProfitSharingReturnsByRefund(ctx context.Context, input ListProfitSharingReturnsByRefundInput) (ListProfitSharingReturnsByRefundResult, error) {
	merchant, err := s.store.GetMerchantByOwner(ctx, input.ActorUserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ListProfitSharingReturnsByRefundResult{}, NewRequestError(http.StatusForbidden, errors.New("you are not a merchant"))
		}
		return ListProfitSharingReturnsByRefundResult{}, err
	}

	refundOrder, err := s.store.GetRefundOrder(ctx, input.RefundID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ListProfitSharingReturnsByRefundResult{}, NewRequestError(http.StatusNotFound, errors.New("refund order not found"))
		}
		return ListProfitSharingReturnsByRefundResult{}, err
	}

	paymentOrder, err := s.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return ListProfitSharingReturnsByRefundResult{}, err
	}
	if !paymentOrder.OrderID.Valid {
		return ListProfitSharingReturnsByRefundResult{}, NewRequestError(http.StatusBadRequest, errors.New("refund order has no associated order"))
	}

	order, err := s.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
	if err != nil {
		return ListProfitSharingReturnsByRefundResult{}, err
	}
	if order.MerchantID != merchant.ID {
		return ListProfitSharingReturnsByRefundResult{}, NewRequestError(http.StatusForbidden, errors.New("refund order does not belong to your merchant"))
	}

	returns, err := s.store.ListProfitSharingReturnsByRefundOrder(ctx, refundOrder.ID)
	if err != nil {
		return ListProfitSharingReturnsByRefundResult{}, err
	}

	return ListProfitSharingReturnsByRefundResult{Returns: returns}, nil
}

func (s *RefundService) processProfitSharingRefund(
	ctx context.Context,
	paymentOrder db.PaymentOrder,
	order db.Order,
	refundOrder db.RefundOrder,
	input CreateRefundOrderInput,
) error {
	if s.paymentFacade == nil {
		_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		return NewRequestError(http.StatusInternalServerError, errors.New("ecommerce client not configured"))
	}

	profitSharingOrder, err := s.store.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
	if err != nil {
		_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		return NewRequestError(http.StatusBadRequest, errors.New("profit sharing order not found"))
	}
	if !profitSharingOrder.SharingOrderID.Valid || profitSharingOrder.SharingOrderID.String == "" {
		_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		return NewRequestError(http.StatusBadRequest, errors.New("profit sharing order id missing"))
	}

	paymentConfig, err := s.store.GetMerchantPaymentConfig(ctx, order.MerchantID)
	if err != nil {
		_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		return err
	}
	if paymentConfig.SubMchID == "" {
		_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		return NewRequestError(http.StatusBadRequest, errors.New("merchant sub mchid not configured"))
	}

	var operator db.Operator
	if profitSharingOrder.OperatorCommission > 0 {
		if !profitSharingOrder.OperatorID.Valid {
			_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			return NewRequestError(http.StatusBadRequest, errors.New("operator not found for profit sharing"))
		}
		op, getErr := s.store.GetOperator(ctx, profitSharingOrder.OperatorID.Int64)
		if getErr != nil {
			_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			return getErr
		}
		if !op.WechatMchID.Valid || op.WechatMchID.String == "" {
			_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			return NewRequestError(http.StatusBadRequest, errors.New("operator wechat mchid not configured"))
		}
		operator = op
	}

	riderOpenID := ""
	if profitSharingOrder.RiderAmount > 0 && profitSharingOrder.RiderID.Valid {
		rider, getRiderErr := s.store.GetRider(ctx, profitSharingOrder.RiderID.Int64)
		if getRiderErr != nil {
			_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			return getRiderErr
		}
		user, getUserErr := s.store.GetUser(ctx, rider.UserID)
		if getUserErr != nil {
			_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			return getUserErr
		}
		if user.WechatOpenid == "" {
			_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			return NewRequestError(http.StatusBadRequest, errors.New("rider wechat openid not configured"))
		}
		riderOpenID = user.WechatOpenid
	}

	hasProcessing := false
	processReturn := func(outReturnNo, returnAccountType, returnAccount, description string, amount int64, delay time.Duration) error {
		returnRecord, createErr := s.store.CreateProfitSharingReturn(ctx, db.CreateProfitSharingReturnParams{
			RefundOrderID:        refundOrder.ID,
			ProfitSharingOrderID: profitSharingOrder.ID,
			PaymentOrderID:       paymentOrder.ID,
			SubMchid:             paymentConfig.SubMchID,
			OutOrderNo:           profitSharingOrder.OutOrderNo,
			OutReturnNo:          outReturnNo,
			ReturnMchid:          returnAccount,
			Amount:               amount,
			Status:               "pending",
		})
		if createErr != nil {
			return createErr
		}

		returnResp, returnErr := s.paymentFacade.CreateProfitSharingReturn(ctx, &wechat.ProfitSharingReturnRequest{
			SubMchID:          paymentConfig.SubMchID,
			OrderID:           profitSharingOrder.SharingOrderID.String,
			OutOrderNo:        profitSharingOrder.OutOrderNo,
			OutReturnNo:       outReturnNo,
			ReturnAccountType: returnAccountType,
			ReturnAccount:     returnAccount,
			Amount:            amount,
			Description:       description,
		})
		if returnErr != nil {
			_, _ = s.store.UpdateProfitSharingReturnToFailed(ctx, db.UpdateProfitSharingReturnToFailedParams{
				ID:         returnRecord.ID,
				FailReason: pgtype.Text{String: returnErr.Error(), Valid: true},
			})
			return returnErr
		}

		switch returnResp.Result {
		case "SUCCESS":
			if returnResp.ReturnID != "" {
				_, _ = s.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
					ID:       returnRecord.ID,
					ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: true},
				})
			}
			_, _ = s.store.UpdateProfitSharingReturnToSuccess(ctx, returnRecord.ID)
		case "PROCESSING":
			_, _ = s.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
				ID:       returnRecord.ID,
				ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: returnResp.ReturnID != ""},
			})
			if s.taskScheduler != nil {
				_ = s.taskScheduler.ScheduleProfitSharingReturnResult(ctx, ProfitSharingReturnResultTaskInput{
					ProfitSharingReturnID: returnRecord.ID,
					OutReturnNo:           returnRecord.OutReturnNo,
					OutOrderNo:            returnRecord.OutOrderNo,
					SubMchID:              returnRecord.SubMchid,
					RefundOrderID:         returnRecord.RefundOrderID,
					RetryCount:            0,
					Delay:                 delay,
				})
			}
			hasProcessing = true
		case "FAILED":
			_, _ = s.store.UpdateProfitSharingReturnToFailed(ctx, db.UpdateProfitSharingReturnToFailedParams{
				ID:         returnRecord.ID,
				FailReason: pgtype.Text{String: returnResp.FailReason, Valid: returnResp.FailReason != ""},
			})
			return fmt.Errorf("profit sharing return failed")
		default:
			_, _ = s.store.UpdateProfitSharingReturnToFailed(ctx, db.UpdateProfitSharingReturnToFailedParams{
				ID:         returnRecord.ID,
				FailReason: pgtype.Text{String: "unknown return result", Valid: true},
			})
			return fmt.Errorf("profit sharing return unknown result")
		}

		return nil
	}

	delay := input.ProfitSharingReturnRetryInterval
	if delay <= 0 {
		delay = 30 * time.Second
	}

	if profitSharingOrder.PlatformCommission > 0 {
		outReturnNo := fmt.Sprintf("PR%dPL", refundOrder.ID)
		if returnErr := processReturn(outReturnNo, wechat.ReceiverTypeMerchant, s.paymentFacade.SpMchID(), "平台分账回退", profitSharingOrder.PlatformCommission, delay); returnErr != nil {
			_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			return NewRequestError(http.StatusInternalServerError, errors.New("profit sharing return failed"))
		}
	}
	if profitSharingOrder.OperatorCommission > 0 {
		outReturnNo := fmt.Sprintf("PR%dOP", refundOrder.ID)
		if returnErr := processReturn(outReturnNo, wechat.ReceiverTypeMerchant, operator.WechatMchID.String, "运营商分账回退", profitSharingOrder.OperatorCommission, delay); returnErr != nil {
			_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			return NewRequestError(http.StatusInternalServerError, errors.New("profit sharing return failed"))
		}
	}
	if profitSharingOrder.RiderAmount > 0 {
		outReturnNo := fmt.Sprintf("PR%dRD", refundOrder.ID)
		if returnErr := processReturn(outReturnNo, wechat.ReceiverTypePersonal, riderOpenID, "骑手分账回退", profitSharingOrder.RiderAmount, delay); returnErr != nil {
			_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			return NewRequestError(http.StatusInternalServerError, errors.New("profit sharing return failed"))
		}
	}

	if hasProcessing {
		return nil
	}

	wxRefund, err := s.paymentFacade.CreateEcommerceRefund(ctx, &wechat.EcommerceRefundRequest{
		SubMchID:     paymentConfig.SubMchID,
		OutTradeNo:   paymentOrder.OutTradeNo,
		OutRefundNo:  refundOrder.OutRefundNo,
		Reason:       input.RefundReason,
		RefundAmount: input.RefundAmount,
		TotalAmount:  paymentOrder.Amount,
	})
	if err != nil {
		_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		return fmt.Errorf("wechat ecommerce refund: %w", err)
	}

	switch wxRefund.Status {
	case wechat.RefundStatusSuccess:
		_, _ = s.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
		_, _ = s.store.UpdatePaymentOrderToRefunded(ctx, paymentOrder.ID)
	case wechat.RefundStatusProcessing:
		_, _ = s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: true},
		})
	default:
		_, _ = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
	}

	return nil
}
