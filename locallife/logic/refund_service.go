package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
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

// maybeMarkPaymentOrderRefunded 仅在累计退款额 >= 支付金额时才将支付单标记为 refunded，
// 避免部分退款错误终结支付单。
func (s *RefundService) maybeMarkPaymentOrderRefunded(ctx context.Context, paymentOrderID int64, paymentAmount int64) {
	totalRefunded, err := s.store.GetTotalRefundedByPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		log.Error().Err(err).Int64("payment_order_id", paymentOrderID).Msg("failed to get total refunded amount")
		return
	}
	if totalRefunded >= paymentAmount {
		if _, dbErr := s.store.UpdatePaymentOrderToRefunded(ctx, paymentOrderID); dbErr != nil {
			log.Error().Err(dbErr).Int64("payment_order_id", paymentOrderID).Msg("failed to mark payment order as refunded")
		}
	} else {
		log.Info().
			Int64("payment_order_id", paymentOrderID).
			Int64("total_refunded", totalRefunded).
			Int64("payment_amount", paymentAmount).
			Msg("partial refund: payment order not yet fully refunded")
	}
}

func (s *RefundService) CreateRefundOrder(ctx context.Context, input CreateRefundOrderInput) (CreateRefundOrderResult, error) {
	merchant, err := resolveMerchantForUser(ctx, s.store, input.ActorUserID)
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
	if !paymentOrderUsesEcommerceChannel(paymentOrder) &&
		!db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) &&
		!paymentOrderUsesBaofuAggregateChannel(paymentOrder) {
		return CreateRefundOrderResult{}, mainBusinessEcommerceOnlyError("发起退款")
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

	if paymentOrderUsesBaofuAggregateChannel(paymentOrder) {
		guard, err := s.store.GetBaofuPaymentOrderRefundGuardForUpdate(ctx, paymentOrder.ID)
		if err != nil {
			return CreateRefundOrderResult{}, fmt.Errorf("get baofu refund guard: %w", err)
		}
		if guard.HasStartedProfitSharing {
			return CreateRefundOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("订单已进入结算分账流程，不支持退款"))
		}
		outRefundNo, err := s.idGenerator.OutRefundNo(s.clock.Now())
		if err != nil {
			return CreateRefundOrderResult{}, fmt.Errorf("generate out refund no: %w", err)
		}
		txResult, err := s.store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
			PaymentOrderID: input.PaymentOrderID,
			RefundType:     input.RefundType,
			RefundAmount:   input.RefundAmount,
			RefundReason:   input.RefundReason,
			OutRefundNo:    outRefundNo,
		})
		if err != nil {
			if statusCode, ok := db.IsRefundRequestError(err); ok {
				return CreateRefundOrderResult{}, NewRequestError(statusCode, errors.Unwrap(err))
			}
			return CreateRefundOrderResult{}, fmt.Errorf("create baofu refund order: %w", err)
		}
		refundOrder := txResult.RefundOrder
		if err := s.processBaofuPreShareRefund(ctx, paymentOrder, refundOrder, input); err != nil {
			return CreateRefundOrderResult{}, err
		}
		if latest, getErr := s.store.GetRefundOrder(ctx, refundOrder.ID); getErr == nil {
			refundOrder = latest
		}
		return CreateRefundOrderResult{RefundOrder: refundOrder}, nil
	}

	outRefundNo, err := s.idGenerator.OutRefundNo(s.clock.Now())
	if err != nil {
		return CreateRefundOrderResult{}, fmt.Errorf("generate out refund no: %w", err)
	}

	// 使用事务原子性地校验累计退款额并创建退款单，消除并发超退竞态（#5）
	txResult, err := s.store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
		PaymentOrderID: input.PaymentOrderID,
		RefundType:     input.RefundType,
		RefundAmount:   input.RefundAmount,
		RefundReason:   input.RefundReason,
		OutRefundNo:    outRefundNo,
	})
	if err != nil {
		if statusCode, ok := db.IsRefundRequestError(err); ok {
			return CreateRefundOrderResult{}, NewRequestError(statusCode, errors.Unwrap(err))
		}
		return CreateRefundOrderResult{}, fmt.Errorf("create refund order: %w", err)
	}
	refundOrder := txResult.RefundOrder

	if err := s.processProfitSharingRefund(ctx, paymentOrder, order, refundOrder, input); err != nil {
		return CreateRefundOrderResult{}, err
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
			merchant, getMerchantErr := resolveMerchantForUser(ctx, s.store, input.ActorUserID)
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
			merchant, getMerchantErr := resolveMerchantForUser(ctx, s.store, input.ActorUserID)
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
	merchant, err := resolveMerchantForUser(ctx, s.store, input.ActorUserID)
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

func (s *RefundService) ApplyAbnormalRefund(ctx context.Context, input ApplyAbnormalRefundInput) (ApplyAbnormalRefundResult, error) {
	if s.paymentFacade == nil {
		return ApplyAbnormalRefundResult{}, fmt.Errorf("ecommerce client: not configured")
	}

	refundOrder, err := s.store.GetRefundOrder(ctx, input.RefundID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ApplyAbnormalRefundResult{}, NewRequestError(http.StatusNotFound, errors.New("refund order not found"))
		}
		return ApplyAbnormalRefundResult{}, err
	}

	if refundOrder.Status != "failed" {
		return ApplyAbnormalRefundResult{}, NewRequestError(http.StatusBadRequest, errors.New("refund order is not in failed state"))
	}
	if !refundOrder.RefundID.Valid || refundOrder.RefundID.String == "" {
		return ApplyAbnormalRefundResult{}, NewRequestError(http.StatusBadRequest, errors.New("refund order has no wechat refund id"))
	}

	paymentOrder, err := s.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return ApplyAbnormalRefundResult{}, err
	}
	if !paymentOrderUsesEcommerceChannel(paymentOrder) {
		return ApplyAbnormalRefundResult{}, NewRequestError(http.StatusBadRequest, errors.New("refund order is not an ecommerce refund"))
	}

	merchantID, err := s.resolveMerchantIDForRefund(ctx, paymentOrder)
	if err != nil {
		return ApplyAbnormalRefundResult{}, err
	}

	paymentConfig, err := s.store.GetMerchantPaymentConfig(ctx, merchantID)
	if err != nil {
		return ApplyAbnormalRefundResult{}, err
	}
	if paymentConfig.SubMchID == "" {
		return ApplyAbnormalRefundResult{}, NewRequestError(http.StatusBadRequest, errors.New("merchant sub mchid not configured"))
	}

	wxRefund, err := s.paymentFacade.ApplyEcommerceAbnormalRefund(ctx, &wechatcontracts.EcommerceAbnormalRefundRequest{
		RefundID:    refundOrder.RefundID.String,
		SubMchID:    paymentConfig.SubMchID,
		OutRefundNo: refundOrder.OutRefundNo,
		Type:        input.Type,
		BankType:    input.BankType,
		BankAccount: input.BankAccount,
		RealName:    input.RealName,
	})
	if err != nil {
		return ApplyAbnormalRefundResult{}, mapEcommerceAbnormalRefundError(err)
	}

	latestRefundOrder := refundOrder
	refundID := refundOrder.RefundID.String
	if wxRefund.RefundID != "" {
		refundID = wxRefund.RefundID
	}

	switch wxRefund.Status {
	case wechatcontracts.EcommerceRefundStatusSuccess:
		latestRefundOrder, err = s.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
		if err != nil {
			return ApplyAbnormalRefundResult{}, fmt.Errorf("update refund order to success: %w", err)
		}
		s.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount)
	case wechatcontracts.EcommerceRefundStatusProcessing:
		latestRefundOrder, err = s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
		})
		if err != nil {
			return ApplyAbnormalRefundResult{}, fmt.Errorf("update refund order to processing: %w", err)
		}
	case wechatcontracts.EcommerceRefundStatusClosed:
		latestRefundOrder, err = s.store.UpdateRefundOrderToClosed(ctx, refundOrder.ID)
		if err != nil {
			return ApplyAbnormalRefundResult{}, fmt.Errorf("update refund order to closed: %w", err)
		}
	default:
		latestRefundOrder, err = s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		if err != nil {
			return ApplyAbnormalRefundResult{}, fmt.Errorf("update refund order to failed: %w", err)
		}
	}

	return ApplyAbnormalRefundResult{
		RefundOrder:  latestRefundOrder,
		WechatRefund: *wxRefund,
	}, nil
}

func (s *RefundService) resolveMerchantIDForRefund(ctx context.Context, paymentOrder db.PaymentOrder) (int64, error) {
	if paymentOrder.OrderID.Valid {
		order, err := s.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
		if err != nil {
			return 0, err
		}
		return order.MerchantID, nil
	}

	if paymentOrder.ReservationID.Valid {
		reservation, err := s.store.GetTableReservation(ctx, paymentOrder.ReservationID.Int64)
		if err != nil {
			return 0, err
		}
		return reservation.MerchantID, nil
	}

	return 0, NewRequestError(http.StatusBadRequest, errors.New("payment order has no associated merchant"))
}

func (s *RefundService) processProfitSharingRefund(
	ctx context.Context,
	paymentOrder db.PaymentOrder,
	order db.Order,
	refundOrder db.RefundOrder,
	input CreateRefundOrderInput,
) error {
	if s.paymentFacade == nil {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		}
		return fmt.Errorf("ecommerce client: not configured")
	}

	profitSharingOrder, err := s.store.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
	if err != nil {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		}
		return NewRequestError(http.StatusBadRequest, errors.New("profit sharing order not found"))
	}
	if !profitSharingOrder.SharingOrderID.Valid || profitSharingOrder.SharingOrderID.String == "" {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		}
		return NewRequestError(http.StatusBadRequest, errors.New("profit sharing order id missing"))
	}

	paymentConfig, err := s.store.GetMerchantPaymentConfig(ctx, order.MerchantID)
	if err != nil {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		}
		return err
	}
	if paymentConfig.SubMchID == "" {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		}
		return NewRequestError(http.StatusBadRequest, errors.New("merchant sub mchid not configured"))
	}

	var operator db.Operator
	if profitSharingOrder.OperatorCommission > 0 {
		if !profitSharingOrder.OperatorID.Valid {
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return NewRequestError(http.StatusBadRequest, errors.New("operator not found for profit sharing"))
		}
		op, getErr := s.store.GetOperator(ctx, profitSharingOrder.OperatorID.Int64)
		if getErr != nil {
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return getErr
		}
		if !op.WechatMchID.Valid || op.WechatMchID.String == "" {
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return NewRequestError(http.StatusBadRequest, errors.New("operator wechat mchid not configured"))
		}
		operator = op
	}

	if profitSharingOrder.RiderAmount > 0 {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		}
		return NewRequestError(http.StatusBadRequest, errors.New("订单包含个人分账，当前不支持自动退款，请联系平台处理"))
	}

	hasProcessing := false
	pendingReturnFactApplications := make([]db.ExternalPaymentFactApplication, 0, 2)
	processReturn := func(outReturnNo, returnAccount, description string, amount int64, delay time.Duration) error {
		// 幂等检查：如果该 outReturnNo 已有记录，且状态为 success/processing，直接跳过
		// 这让 processProfitSharingRefund 在被重试时能从失败点继续，而非重新全量执行
		existingReturn, lookupErr := s.store.GetProfitSharingReturnByOutReturnNo(ctx, outReturnNo)
		if lookupErr == nil {
			switch existingReturn.Status {
			case "success":
				return nil // 已完成，跳过
			case "processing":
				hasProcessing = true
				return nil // 已在进行中，等待 recovery 跟踪
				// pending/failed 状态：继续向下重试
			}
		}

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

		returnResp, returnErr := s.createProfitSharingReturn(ctx, paymentOrder, &wechatcontracts.ProfitSharingReturnRequest{
			SubMchID:      paymentConfig.SubMchID,
			OrderID:       profitSharingOrder.SharingOrderID.String,
			TransactionID: paymentOrder.TransactionID.String,
			OutOrderNo:    profitSharingOrder.OutOrderNo,
			OutReturnNo:   outReturnNo,
			ReturnMchID:   returnAccount,
			Amount:        amount,
			Description:   description,
		})
		if returnErr != nil {
			if wechat.IsProfitSharingReturnProcessingError(returnErr) {
				log.Warn().
					Err(returnErr).
					Int64("profit_sharing_return_id", returnRecord.ID).
					Str("out_return_no", returnRecord.OutReturnNo).
					Msg("profit sharing return request reported ambiguous state, fallback to polling")

				if _, dbErr := s.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
					ID:       returnRecord.ID,
					ReturnID: pgtype.Text{},
				}); dbErr != nil {
					log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
				} else {
					recordProfitSharingReturnCommandUnknown(ctx, s.store, paymentOrder.PaymentChannel, returnRecord, returnErr)
				}
				if s.taskScheduler != nil {
					if schedErr := s.taskScheduler.ScheduleProfitSharingReturnResult(ctx, ProfitSharingReturnResultTaskInput{
						ProfitSharingReturnID: returnRecord.ID,
						OutReturnNo:           returnRecord.OutReturnNo,
						OutOrderNo:            returnRecord.OutOrderNo,
						SubMchID:              returnRecord.SubMchid,
						RefundOrderID:         returnRecord.RefundOrderID,
						RetryCount:            0,
						Delay:                 delay,
					}); schedErr != nil {
						log.Error().Err(schedErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to enqueue profit sharing return result task")
					}
				}
				hasProcessing = true
				return nil
			}

			recordProfitSharingReturnCommandRejected(ctx, s.store, paymentOrder.PaymentChannel, returnRecord, returnErr)
			application, factErr := recordProfitSharingReturnCommandErrorFact(ctx, s.store, paymentOrder.PaymentChannel, returnRecord, returnErr)
			if factErr != nil {
				return factErr
			}
			if application != nil {
				if applyErr := s.applyProfitSharingReturnFactApplication(ctx, application.ID); applyErr != nil {
					return applyErr
				}
			}
			return returnErr
		}

		switch returnResp.Result {
		case "SUCCESS":
			if _, dbErr := s.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
				ID:       returnRecord.ID,
				ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: returnResp.ReturnID != ""},
			}); dbErr != nil {
				log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
			} else {
				recordProfitSharingReturnCommandAccepted(ctx, s.store, paymentOrder.PaymentChannel, returnRecord, returnResp)
			}
			if _, factErr := recordProfitSharingReturnCommandResponseFact(ctx, s.store, paymentOrder.PaymentChannel, returnRecord, returnResp); factErr != nil {
				return factErr
			}
			if s.taskScheduler != nil {
				if schedErr := s.taskScheduler.ScheduleProfitSharingReturnResult(ctx, ProfitSharingReturnResultTaskInput{
					ProfitSharingReturnID: returnRecord.ID,
					OutReturnNo:           returnRecord.OutReturnNo,
					OutOrderNo:            returnRecord.OutOrderNo,
					SubMchID:              returnRecord.SubMchid,
					RefundOrderID:         returnRecord.RefundOrderID,
					RetryCount:            0,
					Delay:                 delay,
				}); schedErr != nil {
					log.Error().Err(schedErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to enqueue profit sharing return result task")
				}
			}
			hasProcessing = true
		case "PROCESSING":
			if _, dbErr := s.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
				ID:       returnRecord.ID,
				ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: returnResp.ReturnID != ""},
			}); dbErr != nil {
				log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
			} else {
				recordProfitSharingReturnCommandAccepted(ctx, s.store, paymentOrder.PaymentChannel, returnRecord, returnResp)
			}
			if s.taskScheduler != nil {
				if schedErr := s.taskScheduler.ScheduleProfitSharingReturnResult(ctx, ProfitSharingReturnResultTaskInput{
					ProfitSharingReturnID: returnRecord.ID,
					OutReturnNo:           returnRecord.OutReturnNo,
					OutOrderNo:            returnRecord.OutOrderNo,
					SubMchID:              returnRecord.SubMchid,
					RefundOrderID:         returnRecord.RefundOrderID,
					RetryCount:            0,
					Delay:                 delay,
				}); schedErr != nil {
					log.Error().Err(schedErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to enqueue profit sharing return result task")
				}
			}
			hasProcessing = true
		case "FAILED":
			failedErr := errors.New("profit sharing return failed")
			if returnResp.FailReason != "" {
				failedErr = errors.New(returnResp.FailReason)
			}
			recordProfitSharingReturnCommandRejected(ctx, s.store, paymentOrder.PaymentChannel, returnRecord, failedErr)
			if _, dbErr := s.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
				ID:       returnRecord.ID,
				ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: returnResp.ReturnID != ""},
			}); dbErr != nil {
				log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
			}
			if _, factErr := recordProfitSharingReturnCommandResponseFact(ctx, s.store, paymentOrder.PaymentChannel, returnRecord, returnResp); factErr != nil {
				return factErr
			}
			if s.taskScheduler != nil {
				if schedErr := s.taskScheduler.ScheduleProfitSharingReturnResult(ctx, ProfitSharingReturnResultTaskInput{
					ProfitSharingReturnID: returnRecord.ID,
					OutReturnNo:           returnRecord.OutReturnNo,
					OutOrderNo:            returnRecord.OutOrderNo,
					SubMchID:              returnRecord.SubMchid,
					RefundOrderID:         returnRecord.RefundOrderID,
					RetryCount:            0,
					Delay:                 delay,
				}); schedErr != nil {
					log.Error().Err(schedErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to enqueue profit sharing return result task")
				}
			}
			hasProcessing = true
		default:
			unknownResultErr := fmt.Errorf("unknown return result: %s", returnResp.Result)
			log.Warn().
				Err(unknownResultErr).
				Int64("profit_sharing_return_id", returnRecord.ID).
				Str("out_return_no", returnRecord.OutReturnNo).
				Str("result", returnResp.Result).
				Msg("profit sharing return request returned unknown result, fallback to polling")

			if _, dbErr := s.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
				ID:       returnRecord.ID,
				ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: returnResp.ReturnID != ""},
			}); dbErr != nil {
				log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
			} else {
				recordProfitSharingReturnCommandUnknown(ctx, s.store, paymentOrder.PaymentChannel, returnRecord, unknownResultErr)
				if _, factErr := recordProfitSharingReturnCommandResponseFact(ctx, s.store, paymentOrder.PaymentChannel, returnRecord, returnResp); factErr != nil {
					return factErr
				}
			}
			if s.taskScheduler != nil {
				if schedErr := s.taskScheduler.ScheduleProfitSharingReturnResult(ctx, ProfitSharingReturnResultTaskInput{
					ProfitSharingReturnID: returnRecord.ID,
					OutReturnNo:           returnRecord.OutReturnNo,
					OutOrderNo:            returnRecord.OutOrderNo,
					SubMchID:              returnRecord.SubMchid,
					RefundOrderID:         returnRecord.RefundOrderID,
					RetryCount:            0,
					Delay:                 delay,
				}); schedErr != nil {
					log.Error().Err(schedErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to enqueue profit sharing return result task")
				}
			}
			hasProcessing = true
			return nil
		}

		return nil
	}

	delay := input.ProfitSharingReturnRetryInterval
	if delay <= 0 {
		delay = 30 * time.Second
	}

	if profitSharingOrder.PlatformCommission > 0 {
		outReturnNo := fmt.Sprintf("PR%dPL", refundOrder.ID)
		if returnErr := processReturn(outReturnNo, s.platformProfitSharingReturnMchID(paymentOrder), "平台分账回退", profitSharingOrder.PlatformCommission, delay); returnErr != nil {
			if applyErr := s.applyPendingProfitSharingReturnFactApplications(ctx, pendingReturnFactApplications); applyErr != nil {
				return fmt.Errorf("apply pending profit sharing return facts: %w", applyErr)
			}
			// 单方失败：记录到 profit_sharing_return 记录，整体退款单标记为 partial_failed
			// ProfitSharingRecoveryScheduler 会扫描并重试失败的回退单
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return fmt.Errorf("platform profit sharing return: %w", returnErr)
		}
	}
	if profitSharingOrder.OperatorCommission > 0 {
		outReturnNo := fmt.Sprintf("PR%dOP", refundOrder.ID)
		if returnErr := processReturn(outReturnNo, operator.WechatMchID.String, "运营商分账回退", profitSharingOrder.OperatorCommission, delay); returnErr != nil {
			if applyErr := s.applyPendingProfitSharingReturnFactApplications(ctx, pendingReturnFactApplications); applyErr != nil {
				return fmt.Errorf("apply pending profit sharing return facts: %w", applyErr)
			}
			// 平台回退可能已成功，不在这里标记整体为 failed；仅标记本次尝试失败
			// 后续退款单维持 pending 等 recovery 扫描到 failed 的 profit_sharing_return 后重试
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return fmt.Errorf("operator profit sharing return: %w", returnErr)
		}
	}
	if applyErr := s.applyPendingProfitSharingReturnFactApplications(ctx, pendingReturnFactApplications); applyErr != nil {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		}
		return fmt.Errorf("apply profit sharing return terminal facts: %w", applyErr)
	}
	if hasProcessing {
		return nil
	}
	if len(pendingReturnFactApplications) > 0 {
		return nil
	}

	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		return s.processOrdinaryServiceProviderRefund(ctx, paymentOrder, refundOrder, paymentConfig.SubMchID, input)
	}

	wxRefund, err := s.paymentFacade.CreateEcommerceRefund(ctx, &wechatcontracts.EcommerceRefundRequest{
		SubMchID:    paymentConfig.SubMchID,
		OutTradeNo:  paymentOrder.OutTradeNo,
		OutRefundNo: refundOrder.OutRefundNo,
		Reason:      input.RefundReason,
		Amount: &wechatcontracts.EcommerceRefundRequestAmount{
			Refund:   input.RefundAmount,
			Total:    paymentOrder.Amount,
			Currency: wechatcontracts.EcommerceRefundCurrencyCNY,
		},
	})
	if err != nil {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		} else {
			recordRefundServiceEcommerceRefundCommandRejected(ctx, s.store, paymentOrder, refundOrder, err)
		}
		return mapEcommerceRefundCreateError(err)
	}

	if _, dbErr := s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: wxRefund.RefundID != ""},
	}); dbErr != nil {
		log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as processing")
	}
	recordRefundServiceEcommerceRefundCommandAccepted(ctx, s.store, paymentOrder, refundOrder, wxRefund.RefundID)

	return nil
}

func (s *RefundService) processBaofuPreShareRefund(ctx context.Context, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, input CreateRefundOrderInput) error {
	if s.paymentFacade == nil {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark baofu refund as failed")
		}
		return fmt.Errorf("baofu aggregate client: not configured")
	}

	req := aggregatecontracts.RefundBeforeShareRequest{
		OriginOutTradeNo: strings.TrimSpace(paymentOrder.OutTradeNo),
		OutTradeNo:       strings.TrimSpace(refundOrder.OutRefundNo),
		NotifyURL:        s.paymentFacade.BaofuRefundNotifyURL(),
		RefundAmountFen:  input.RefundAmount,
		TotalAmountFen:   input.RefundAmount,
		TransactionTime:  s.clock.Now().UTC().Format("20060102150405"),
		RefundReason:     strings.TrimSpace(input.RefundReason),
	}
	refundResp, err := s.paymentFacade.CreateBaofuRefund(ctx, req)
	if err != nil {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark baofu refund as failed")
		}
		recordBaofuRefundCommand(ctx, s.store, refundOrder, nil, db.ExternalPaymentCommandStatusRejected, err)
		return err
	}
	if refundResp == nil {
		err := errors.New("baofu refund returned empty result")
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark baofu refund as failed")
		}
		recordBaofuRefundCommand(ctx, s.store, refundOrder, nil, db.ExternalPaymentCommandStatusRejected, err)
		return err
	}

	refundID := strings.TrimSpace(refundResp.TradeNo)
	terminalStatus := aggregatecontracts.NormalizeRefundTerminalStatus(refundResp.RefundState)
	switch terminalStatus {
	case db.ExternalPaymentTerminalStatusSuccess:
		if _, dbErr := s.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("update baofu refund order to success: %w", dbErr)
		}
		s.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount)
		recordBaofuRefundCommand(ctx, s.store, refundOrder, refundResp, db.ExternalPaymentCommandStatusAccepted, nil)
	case db.ExternalPaymentTerminalStatusProcessing:
		if _, dbErr := s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("update baofu refund order to processing: %w", dbErr)
		}
		recordBaofuRefundCommand(ctx, s.store, refundOrder, refundResp, db.ExternalPaymentCommandStatusAccepted, nil)
	case db.ExternalPaymentTerminalStatusFailed:
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("update baofu refund order to failed: %w", dbErr)
		}
		recordBaofuRefundCommand(ctx, s.store, refundOrder, refundResp, db.ExternalPaymentCommandStatusRejected, nil)
		return NewRequestError(http.StatusBadGateway, errors.New("宝付退款受理失败，请稍后重试或联系平台处理"))
	default:
		if _, dbErr := s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("update baofu refund order to processing: %w", dbErr)
		}
		recordBaofuRefundCommand(ctx, s.store, refundOrder, refundResp, db.ExternalPaymentCommandStatusUnknown, nil)
	}
	return nil
}

func recordBaofuRefundCommand(ctx context.Context, store db.Store, refundOrder db.RefundOrder, refundResp *aggregatecontracts.RefundResult, status string, cause error) {
	snapshot := buildBaofuRefundCommandSnapshot(refundOrder, refundResp, cause)
	var secondary pgtype.Text
	var errorCode pgtype.Text
	if refundResp != nil {
		if tradeNo := strings.TrimSpace(refundResp.TradeNo); tradeNo != "" {
			secondary = pgtype.Text{String: tradeNo, Valid: true}
		}
		if code := strings.TrimSpace(refundResp.ErrorCode); code != "" {
			errorCode = pgtype.Text{String: code, Valid: true}
		}
	}
	_, err := store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		CommandType:          db.ExternalPaymentCommandTypeCreateRefund,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerOrder,
		BusinessObjectType:   pgtype.Text{String: "refund_order", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: refundOrder.ID, Valid: true},
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    strings.TrimSpace(refundOrder.OutRefundNo),
		ExternalSecondaryKey: secondary,
		CommandStatus:        status,
		LastErrorCode:        errorCode,
		SubmittedAt:          time.Now().UTC(),
		ResponseSnapshot:     snapshot,
	})
	if err != nil {
		log.Error().Err(err).Int64("refund_order_id", refundOrder.ID).Msg("failed to record baofu refund command")
	}
}

func buildBaofuRefundCommandSnapshot(refundOrder db.RefundOrder, refundResp *aggregatecontracts.RefundResult, cause error) []byte {
	snapshot := struct {
		Provider     string `json:"provider"`
		Operation    string `json:"operation"`
		OutRefundNo  string `json:"out_refund_no"`
		RefundTrade  string `json:"refund_trade_no,omitempty"`
		RefundState  string `json:"refund_state,omitempty"`
		ResultCode   string `json:"result_code,omitempty"`
		ErrorCode    string `json:"error_code,omitempty"`
		ErrorPresent bool   `json:"error_present,omitempty"`
	}{
		Provider:    db.ExternalPaymentProviderBaofu,
		Operation:   "order_refund",
		OutRefundNo: strings.TrimSpace(refundOrder.OutRefundNo),
	}
	if refundResp != nil {
		snapshot.RefundTrade = strings.TrimSpace(refundResp.TradeNo)
		snapshot.RefundState = strings.TrimSpace(refundResp.RefundState)
		snapshot.ResultCode = strings.TrimSpace(refundResp.ResultCode)
		snapshot.ErrorCode = strings.TrimSpace(refundResp.ErrorCode)
	}
	if cause != nil {
		snapshot.ErrorPresent = true
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return []byte(`{"provider":"baofu","operation":"order_refund"}`)
	}
	return raw
}

func (s *RefundService) processOrdinaryServiceProviderRefund(ctx context.Context, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, subMchID string, input CreateRefundOrderInput) error {
	wxRefund, err := s.paymentFacade.CreateOrdinaryServiceProviderRefund(ctx, ospcontracts.RefundCreateRequest{
		SubMchID:    subMchID,
		OutTradeNo:  paymentOrder.OutTradeNo,
		OutRefundNo: refundOrder.OutRefundNo,
		Reason:      input.RefundReason,
		NotifyURL:   s.paymentFacade.OrdinaryServiceProviderRefundNotifyURL(),
		Amount: ospcontracts.RefundAmountRequest{
			Refund:   input.RefundAmount,
			Total:    paymentOrder.Amount,
			Currency: ospcontracts.CurrencyCNY,
		},
	})
	if err != nil {
		log.Error().Err(LoggableError(err)).
			Int64("payment_order_id", paymentOrder.ID).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("out_refund_no", refundOrder.OutRefundNo).
			Msg("ordinary service provider refund create failed")
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		} else {
			recordRefundServiceEcommerceRefundCommandRejected(ctx, s.store, paymentOrder, refundOrder, err)
		}
		return mapOrdinaryServiceProviderRefundCreateError(err)
	}

	refundID := ""
	if wxRefund != nil {
		refundID = wxRefund.RefundID
	}
	if _, dbErr := s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
	}); dbErr != nil {
		log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as processing")
	}
	recordRefundServiceEcommerceRefundCommandAccepted(ctx, s.store, paymentOrder, refundOrder, refundID)

	return nil
}

func (s *RefundService) platformProfitSharingReturnMchID(paymentOrder db.PaymentOrder) string {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		return s.paymentFacade.OrdinaryServiceProviderMchID()
	}
	return s.paymentFacade.SpMchID()
}

func (s *RefundService) createProfitSharingReturn(ctx context.Context, paymentOrder db.PaymentOrder, req *wechatcontracts.ProfitSharingReturnRequest) (*wechatcontracts.ProfitSharingReturnResponse, error) {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		resp, err := s.paymentFacade.CreateOrdinaryServiceProviderProfitSharingReturn(ctx, ospcontracts.ProfitSharingReturnRequest{
			SubMchID:    req.SubMchID,
			OrderID:     req.OrderID,
			OutOrderNo:  req.OutOrderNo,
			OutReturnNo: req.OutReturnNo,
			ReturnMchID: req.ReturnMchID,
			Amount:      req.Amount,
			Description: req.Description,
		})
		if err != nil {
			return nil, err
		}
		return ordinaryProfitSharingReturnToWechatContract(resp, req.TransactionID), nil
	}
	return s.paymentFacade.CreateProfitSharingReturn(ctx, req)
}

func ordinaryProfitSharingReturnToWechatContract(resp *ospcontracts.ProfitSharingReturnResponse, transactionID string) *wechatcontracts.ProfitSharingReturnResponse {
	if resp == nil {
		return nil
	}
	return &wechatcontracts.ProfitSharingReturnResponse{
		SubMchID:      resp.SubMchID,
		OrderID:       resp.OrderID,
		OutOrderNo:    resp.OutOrderNo,
		OutReturnNo:   resp.OutReturnNo,
		ReturnID:      resp.ReturnID,
		ReturnMchID:   resp.ReturnMchID,
		Amount:        resp.Amount,
		Result:        string(resp.State),
		FinishTime:    resp.FinishTime,
		FailReason:    resp.FailReason,
		TransactionID: transactionID,
	}
}

func recordRefundServiceEcommerceRefundCommandAccepted(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, refundID string) {
	paymentCommandSvc := NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbRefundServiceEcommerceRefundCommandInput(
		paymentOrder,
		refundOrder,
		db.ExternalPaymentCommandStatusAccepted,
		stringPtrIfNotEmpty(refundID),
		nil,
		nil,
		refundServiceEcommerceRefundCommandSnapshot(map[string]string{
			"out_refund_no": refundOrder.OutRefundNo,
			"refund_id":     refundID,
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", refundOrder.OutRefundNo).
			Msg("record refund service ecommerce refund command accepted failed")
	}
}

func recordProfitSharingReturnCommandAccepted(ctx context.Context, store db.Store, paymentChannel string, returnRecord db.ProfitSharingReturn, returnResp *wechatcontracts.ProfitSharingReturnResponse) {
	returnID := ""
	result := ""
	if returnResp != nil {
		returnID = returnResp.ReturnID
		result = returnResp.Result
	}
	paymentCommandSvc := NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbProfitSharingReturnCommandInput(
		returnRecord,
		paymentChannel,
		db.ExternalPaymentCommandStatusAccepted,
		stringPtrIfNotEmpty(returnID),
		nil,
		nil,
		profitSharingReturnCommandSnapshot(map[string]string{
			"out_return_no": returnRecord.OutReturnNo,
			"out_order_no":  returnRecord.OutOrderNo,
			"return_id":     returnID,
			"result":        result,
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("profit_sharing_return_id", returnRecord.ID).
			Str("out_return_no", returnRecord.OutReturnNo).
			Msg("record profit sharing return command accepted failed")
	}
}

func recordProfitSharingReturnCommandUnknown(ctx context.Context, store db.Store, paymentChannel string, returnRecord db.ProfitSharingReturn, commandErr error) {
	errorCode, errorMessage := partnerPaymentCommandErrorFields(commandErr)
	paymentCommandSvc := NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbProfitSharingReturnCommandInput(
		returnRecord,
		paymentChannel,
		db.ExternalPaymentCommandStatusUnknown,
		nil,
		errorCode,
		errorMessage,
		profitSharingReturnCommandSnapshot(map[string]string{
			"out_return_no": returnRecord.OutReturnNo,
			"out_order_no":  returnRecord.OutOrderNo,
			"error_code":    stringValue(errorCode),
			"error_message": stringValue(errorMessage),
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("profit_sharing_return_id", returnRecord.ID).
			Str("out_return_no", returnRecord.OutReturnNo).
			Msg("record profit sharing return command unknown failed")
	}
}

func recordProfitSharingReturnCommandRejected(ctx context.Context, store db.Store, paymentChannel string, returnRecord db.ProfitSharingReturn, commandErr error) {
	errorCode, errorMessage := partnerPaymentCommandErrorFields(commandErr)
	paymentCommandSvc := NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbProfitSharingReturnCommandInput(
		returnRecord,
		paymentChannel,
		db.ExternalPaymentCommandStatusRejected,
		nil,
		errorCode,
		errorMessage,
		profitSharingReturnCommandSnapshot(map[string]string{
			"out_return_no": returnRecord.OutReturnNo,
			"out_order_no":  returnRecord.OutOrderNo,
			"error_code":    stringValue(errorCode),
			"error_message": stringValue(errorMessage),
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("profit_sharing_return_id", returnRecord.ID).
			Str("out_return_no", returnRecord.OutReturnNo).
			Msg("record profit sharing return command rejected failed")
	}
}

func recordProfitSharingReturnCommandResponseFact(ctx context.Context, store db.Store, paymentChannel string, returnRecord db.ProfitSharingReturn, returnResp *wechatcontracts.ProfitSharingReturnResponse) (*db.ExternalPaymentFactApplication, error) {
	if returnResp == nil {
		return nil, nil
	}

	amount := returnRecord.Amount
	if returnResp.Amount > 0 {
		amount = returnResp.Amount
	}

	result, err := NewPaymentFactService(store).RecordExternalPaymentFact(ctx, RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              paymentChannel,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		FactSource:           db.ExternalPaymentFactSourceCommandResponse,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:    returnRecord.OutReturnNo,
		ExternalSecondaryKey: stringPtrIfNotEmpty(returnResp.ReturnID),
		BusinessOwner:        stringPtrIfNotEmpty(db.ExternalPaymentBusinessOwnerProfitSharing),
		BusinessObjectType:   stringPtrIfNotEmpty(paymentFactBusinessObjectProfitSharingReturn),
		BusinessObjectID:     int64PtrIfNotZero(returnRecord.ID),
		UpstreamState:        returnResp.Result,
		TerminalStatus:       db.ExternalPaymentTerminalStatusUnknown,
		Amount:               int64PtrIfNotZero(amount),
		Currency:             "CNY",
		RawResource:          profitSharingReturnCommandResponseFactResource(returnRecord, returnResp),
		DedupeKey:            profitSharingReturnCommandResponseFactDedupeKey(paymentChannel, returnRecord.OutReturnNo, returnResp.Result),
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func recordProfitSharingReturnCommandErrorFact(ctx context.Context, store db.Store, paymentChannel string, returnRecord db.ProfitSharingReturn, commandErr error) (*db.ExternalPaymentFactApplication, error) {
	errorCode, errorMessage := partnerPaymentCommandErrorFields(commandErr)
	failReason := stringValue(errorMessage)
	if failReason == "" && commandErr != nil {
		failReason = commandErr.Error()
	}

	result, err := NewPaymentFactService(store).RecordExternalPaymentFact(ctx, RecordExternalPaymentFactInput{
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            paymentChannel,
		Capability:         db.ExternalPaymentCapabilityProfitSharing,
		FactSource:         db.ExternalPaymentFactSourceCommandResponse,
		ExternalObjectType: db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:  returnRecord.OutReturnNo,
		BusinessOwner:      stringPtrIfNotEmpty(db.ExternalPaymentBusinessOwnerProfitSharing),
		BusinessObjectType: stringPtrIfNotEmpty(paymentFactBusinessObjectProfitSharingReturn),
		BusinessObjectID:   int64PtrIfNotZero(returnRecord.ID),
		UpstreamState:      db.ExternalPaymentTerminalStatusFailed,
		TerminalStatus:     db.ExternalPaymentTerminalStatusUnknown,
		Amount:             int64PtrIfNotZero(returnRecord.Amount),
		Currency:           "CNY",
		RawResource:        profitSharingReturnCommandErrorFactResource(returnRecord, errorCode, errorMessage, failReason),
		DedupeKey:          profitSharingReturnCommandResponseFactDedupeKey(paymentChannel, returnRecord.OutReturnNo, db.ExternalPaymentCommandStatusRejected),
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (s *RefundService) applyPendingProfitSharingReturnFactApplications(ctx context.Context, applications []db.ExternalPaymentFactApplication) error {
	for _, application := range applications {
		if err := s.applyProfitSharingReturnFactApplication(ctx, application.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *RefundService) applyProfitSharingReturnFactApplication(ctx context.Context, applicationID int64) error {
	_, err := NewPaymentFactService(s.store).WithRefundCreator(s.paymentFacade).ApplyExternalPaymentFactApplication(ctx, applicationID)
	return err
}

func profitSharingReturnCommandResponseFactDedupeKey(channel, outReturnNo, terminalStatus string) string {
	return "wechat:command_response:" + channel + ":profit_sharing_return:" + outReturnNo + ":" + terminalStatus
}

func profitSharingReturnCommandResponseFactResource(returnRecord db.ProfitSharingReturn, returnResp *wechatcontracts.ProfitSharingReturnResponse) []byte {
	data, err := json.Marshal(map[string]any{
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
		return []byte(`{}`)
	}
	return data
}

func profitSharingReturnCommandErrorFactResource(returnRecord db.ProfitSharingReturn, errorCode, errorMessage *string, failReason string) []byte {
	data, err := json.Marshal(map[string]any{
		"profit_sharing_return_id": returnRecord.ID,
		"refund_order_id":          returnRecord.RefundOrderID,
		"out_order_no":             returnRecord.OutOrderNo,
		"out_return_no":            returnRecord.OutReturnNo,
		"amount":                   returnRecord.Amount,
		"error_code":               stringValue(errorCode),
		"error_message":            stringValue(errorMessage),
		"fail_reason":              failReason,
	})
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func int64PtrIfNotZero(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func dbProfitSharingReturnCommandInput(
	returnRecord db.ProfitSharingReturn,
	paymentChannel string,
	commandStatus string,
	externalSecondaryKey *string,
	lastErrorCode *string,
	lastErrorMessage *string,
	responseSnapshot []byte,
) RecordExternalPaymentCommandInput {
	businessObjectType := "profit_sharing_return"
	businessObjectID := returnRecord.ID
	return RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              paymentChannel,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		CommandType:          db.ExternalPaymentCommandTypeCreateProfitSharingReturn,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerProfitSharing,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharingReturn,
		ExternalObjectKey:    returnRecord.OutReturnNo,
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		LastErrorCode:        lastErrorCode,
		LastErrorMessage:     lastErrorMessage,
		ResponseSnapshot:     responseSnapshot,
	}
}

func profitSharingReturnCommandSnapshot(values map[string]string) []byte {
	filtered := make(map[string]string, len(values))
	for key, value := range values {
		if value != "" {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func recordRefundServiceEcommerceRefundCommandRejected(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, refundErr error) {
	paymentCommandSvc := NewPaymentCommandService(store)
	errorCode, errorMessage := refundServiceCreateRefundCommandErrorFields(paymentOrder.PaymentChannel, refundErr)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbRefundServiceEcommerceRefundCommandInput(
		paymentOrder,
		refundOrder,
		db.ExternalPaymentCommandStatusRejected,
		nil,
		errorCode,
		errorMessage,
		refundServiceEcommerceRefundCommandSnapshot(map[string]string{
			"out_refund_no": refundOrder.OutRefundNo,
			"error_code":    stringValue(errorCode),
			"error_message": stringValue(errorMessage),
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", refundOrder.OutRefundNo).
			Msg("record refund service ecommerce refund command rejected failed")
	}
}

func refundServiceCreateRefundCommandErrorFields(channel string, refundErr error) (*string, *string) {
	if channel == db.PaymentChannelOrdinaryServiceProvider {
		return partnerPaymentCommandErrorFields(refundErr)
	}
	return ecommerceRefundCommandErrorFields(refundErr)
}

func dbRefundServiceEcommerceRefundCommandInput(
	paymentOrder db.PaymentOrder,
	refundOrder db.RefundOrder,
	commandStatus string,
	externalSecondaryKey *string,
	lastErrorCode *string,
	lastErrorMessage *string,
	responseSnapshot []byte,
) RecordExternalPaymentCommandInput {
	businessObjectType := "refund_order"
	businessObjectID := refundOrder.ID
	return RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              paymentOrder.PaymentChannel,
		Capability:           refundServiceCreateRefundCapability(paymentOrder.PaymentChannel),
		CommandType:          db.ExternalPaymentCommandTypeCreateRefund,
		BusinessOwner:        refundServiceEcommerceRefundBusinessOwner(paymentOrder),
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    refundOrder.OutRefundNo,
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		LastErrorCode:        lastErrorCode,
		LastErrorMessage:     lastErrorMessage,
		ResponseSnapshot:     responseSnapshot,
	}
}

func refundServiceCreateRefundCapability(channel string) string {
	if channel == db.PaymentChannelOrdinaryServiceProvider {
		return db.ExternalPaymentCapabilityPartnerRefund
	}
	return db.ExternalPaymentCapabilityEcommerceRefund
}

func refundServiceEcommerceRefundBusinessOwner(paymentOrder db.PaymentOrder) string {
	if paymentOrder.BusinessType == businessTypeReservation || paymentOrder.ReservationID.Valid {
		return db.ExternalPaymentBusinessOwnerReservation
	}
	return db.ExternalPaymentBusinessOwnerOrder
}

func refundServiceEcommerceRefundCommandSnapshot(values map[string]string) []byte {
	filtered := make(map[string]string, len(values))
	for key, value := range values {
		if value != "" {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}
