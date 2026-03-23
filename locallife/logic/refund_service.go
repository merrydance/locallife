package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

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
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return CreateRefundOrderResult{}, fmt.Errorf("wechat refund: %w", refundErr)
		}

		switch wxRefund.Status {
		case wechat.RefundStatusSuccess:
			if _, dbErr := s.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as success")
			}
			s.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount)
		case wechat.RefundStatusProcessing:
			if _, dbErr := s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
				ID:       refundOrder.ID,
				RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: true},
			}); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as processing")
			}
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
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		}
		return NewRequestError(http.StatusInternalServerError, errors.New("ecommerce client not configured"))
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

	riderOpenID := ""
	if profitSharingOrder.RiderAmount > 0 && profitSharingOrder.RiderID.Valid {
		rider, getRiderErr := s.store.GetRider(ctx, profitSharingOrder.RiderID.Int64)
		if getRiderErr != nil {
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return getRiderErr
		}
		user, getUserErr := s.store.GetUser(ctx, rider.UserID)
		if getUserErr != nil {
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return getUserErr
		}
		if user.WechatOpenid == "" {
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return NewRequestError(http.StatusBadRequest, errors.New("rider wechat openid not configured"))
		}
		riderOpenID = user.WechatOpenid
	}

	hasProcessing := false
	processReturn := func(outReturnNo, returnAccountType, returnAccount, description string, amount int64, delay time.Duration) error {
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
			if _, dbErr := s.store.UpdateProfitSharingReturnToFailed(ctx, db.UpdateProfitSharingReturnToFailedParams{
				ID:         returnRecord.ID,
				FailReason: pgtype.Text{String: returnErr.Error(), Valid: true},
			}); dbErr != nil {
				log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as failed")
			}
			return returnErr
		}

		switch returnResp.Result {
		case "SUCCESS":
			if returnResp.ReturnID != "" {
				if _, dbErr := s.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
					ID:       returnRecord.ID,
					ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: true},
				}); dbErr != nil {
					log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
				}
			}
			if _, dbErr := s.store.UpdateProfitSharingReturnToSuccess(ctx, returnRecord.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as success")
			}
		case "PROCESSING":
			if _, dbErr := s.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
				ID:       returnRecord.ID,
				ReturnID: pgtype.Text{String: returnResp.ReturnID, Valid: returnResp.ReturnID != ""},
			}); dbErr != nil {
				log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as processing")
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
			if _, dbErr := s.store.UpdateProfitSharingReturnToFailed(ctx, db.UpdateProfitSharingReturnToFailedParams{
				ID:         returnRecord.ID,
				FailReason: pgtype.Text{String: returnResp.FailReason, Valid: returnResp.FailReason != ""},
			}); dbErr != nil {
				log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as failed")
			}
			return fmt.Errorf("profit sharing return failed")
		default:
			if _, dbErr := s.store.UpdateProfitSharingReturnToFailed(ctx, db.UpdateProfitSharingReturnToFailedParams{
				ID:         returnRecord.ID,
				FailReason: pgtype.Text{String: "unknown return result", Valid: true},
			}); dbErr != nil {
				log.Error().Err(dbErr).Int64("profit_sharing_return_id", returnRecord.ID).Msg("failed to mark profit sharing return as failed")
			}
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
			// 单方失败：记录到 profit_sharing_return 记录，整体退款单标记为 partial_failed
			// ProfitSharingRecoveryScheduler 会扫描并重试失败的回退单
			log.Error().Err(returnErr).Int64("refund_order_id", refundOrder.ID).Msg("platform profit sharing return failed")
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return NewRequestError(http.StatusInternalServerError, errors.New("platform profit sharing return failed"))
		}
	}
	if profitSharingOrder.OperatorCommission > 0 {
		outReturnNo := fmt.Sprintf("PR%dOP", refundOrder.ID)
		if returnErr := processReturn(outReturnNo, wechat.ReceiverTypeMerchant, operator.WechatMchID.String, "运营商分账回退", profitSharingOrder.OperatorCommission, delay); returnErr != nil {
			log.Error().Err(returnErr).Int64("refund_order_id", refundOrder.ID).Msg("operator profit sharing return failed")
			// 平台回退可能已成功，不在这里标记整体为 failed；仅标记本次尝试失败
			// 后续退款单维持 pending 等 recovery 扫描到 failed 的 profit_sharing_return 后重试
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return NewRequestError(http.StatusInternalServerError, errors.New("operator profit sharing return failed"))
		}
	}
	if profitSharingOrder.RiderAmount > 0 {
		outReturnNo := fmt.Sprintf("PR%dRD", refundOrder.ID)
		if returnErr := processReturn(outReturnNo, wechat.ReceiverTypePersonal, riderOpenID, "骑手分账回退", profitSharingOrder.RiderAmount, delay); returnErr != nil {
			log.Error().Err(returnErr).Int64("refund_order_id", refundOrder.ID).Msg("rider profit sharing return failed")
			if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
				log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
			}
			return NewRequestError(http.StatusInternalServerError, errors.New("rider profit sharing return failed"))
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
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		}
		return fmt.Errorf("wechat ecommerce refund: %w", err)
	}

	switch wxRefund.Status {
	case wechat.RefundStatusSuccess:
		if _, dbErr := s.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as success")
		}
		s.maybeMarkPaymentOrderRefunded(ctx, paymentOrder.ID, paymentOrder.Amount)
	case wechat.RefundStatusProcessing:
		if _, dbErr := s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: true},
		}); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as processing")
		}
	default:
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		}
	}

	return nil
}
