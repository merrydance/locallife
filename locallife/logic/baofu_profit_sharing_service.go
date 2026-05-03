package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	BaofuPaymentFeeRateBps = 30
)

var (
	ErrBaofuProfitSharingInvalidAmount          = errors.New("baofu profit sharing amount input is invalid")
	ErrBaofuProfitSharingMerchantAmountNegative = errors.New("baofu profit sharing merchant amount is negative")
)

type BaofuProfitSharingAmountInput struct {
	TotalAmountFen                              int64
	DeliveryFeeFen                              int64
	PlatformRateBps                             int32
	OperatorRateBps                             int32
	HasRiderReceiver                            bool
	HasOperatorReceiver                         bool
	RedirectMissingOperatorCommissionToPlatform bool
}

type BaofuProfitSharingAmountResult struct {
	TotalAmountFen                         int64
	DeliveryFeeFen                         int64
	RiderAmountFen                         int64
	DistributableAmountFen                 int64
	PlatformRateBps                        int32
	OperatorRateBps                        int32
	PlatformCommissionFen                  int64
	OperatorCommissionFen                  int64
	PaymentFeeFen                          int64
	PaymentFeeRateBps                      int32
	MerchantAmountFen                      int64
	OperatorCommissionRedirectedToPlatform bool
}

type baofuProfitSharingReceiverStore interface {
	GetBaofuAccountBindingByOwner(ctx context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error)
}

type BaofuProfitSharingReceiverInput struct {
	MerchantID      int64
	RiderID         int64
	OperatorID      int64
	PlatformOwnerID int64
}

type BaofuProfitSharingReceiverResult struct {
	MerchantSharingMerID string
	RiderSharingMerID    string
	OperatorSharingMerID string
	PlatformSharingMerID string
}

func CalculateBaofuPaymentFeeFen(totalAmountFen int64) int64 {
	if totalAmountFen <= 0 {
		return 0
	}
	return (totalAmountFen*BaofuPaymentFeeRateBps + 9999) / 10000
}

func CalculateBaofuProfitSharingAmounts(input BaofuProfitSharingAmountInput) (BaofuProfitSharingAmountResult, error) {
	if input.TotalAmountFen < 0 || input.DeliveryFeeFen < 0 || input.PlatformRateBps < 0 || input.OperatorRateBps < 0 {
		return BaofuProfitSharingAmountResult{}, ErrBaofuProfitSharingInvalidAmount
	}

	result := BaofuProfitSharingAmountResult{
		TotalAmountFen:    input.TotalAmountFen,
		DeliveryFeeFen:    input.DeliveryFeeFen,
		PlatformRateBps:   input.PlatformRateBps,
		OperatorRateBps:   input.OperatorRateBps,
		PaymentFeeFen:     CalculateBaofuPaymentFeeFen(input.TotalAmountFen),
		PaymentFeeRateBps: BaofuPaymentFeeRateBps,
	}
	if input.HasRiderReceiver {
		result.RiderAmountFen = minInt64(input.DeliveryFeeFen, input.TotalAmountFen)
	}
	result.DistributableAmountFen = input.TotalAmountFen - result.RiderAmountFen
	if result.DistributableAmountFen < 0 {
		result.DistributableAmountFen = 0
	}

	result.PlatformCommissionFen = input.TotalAmountFen * int64(input.PlatformRateBps) / 10000
	result.OperatorCommissionFen = input.TotalAmountFen * int64(input.OperatorRateBps) / 10000
	if !input.HasOperatorReceiver && result.OperatorCommissionFen > 0 && input.RedirectMissingOperatorCommissionToPlatform {
		result.PlatformCommissionFen += result.OperatorCommissionFen
		result.OperatorCommissionFen = 0
		result.OperatorCommissionRedirectedToPlatform = true
	}

	result.MerchantAmountFen = result.DistributableAmountFen - result.PlatformCommissionFen - result.OperatorCommissionFen - result.PaymentFeeFen
	if result.MerchantAmountFen < 0 {
		return BaofuProfitSharingAmountResult{}, ErrBaofuProfitSharingMerchantAmountNegative
	}
	return result, nil
}

func ResolveBaofuProfitSharingReceivers(ctx context.Context, store baofuProfitSharingReceiverStore, input BaofuProfitSharingReceiverInput) (BaofuProfitSharingReceiverResult, error) {
	if store == nil || input.MerchantID <= 0 {
		return BaofuProfitSharingReceiverResult{}, ErrBaofuAccountReceiverRequired
	}

	platformOwnerID := input.PlatformOwnerID
	result := BaofuProfitSharingReceiverResult{}
	merchantReceiver, err := resolveBaofuProfitSharingReceiver(ctx, store, db.BaofuAccountOwnerTypeMerchant, input.MerchantID)
	if err != nil {
		return result, fmt.Errorf("merchant baofu receiver: %w", err)
	}
	result.MerchantSharingMerID = merchantReceiver

	if input.RiderID > 0 {
		riderReceiver, err := resolveBaofuProfitSharingReceiver(ctx, store, db.BaofuAccountOwnerTypeRider, input.RiderID)
		if err != nil {
			return result, fmt.Errorf("rider baofu receiver: %w", err)
		}
		result.RiderSharingMerID = riderReceiver
	}

	if input.OperatorID > 0 {
		operatorReceiver, err := resolveBaofuProfitSharingReceiver(ctx, store, db.BaofuAccountOwnerTypeOperator, input.OperatorID)
		if err != nil {
			return result, fmt.Errorf("operator baofu receiver: %w", err)
		}
		result.OperatorSharingMerID = operatorReceiver
	}

	platformReceiver, err := resolveBaofuProfitSharingReceiver(ctx, store, db.BaofuAccountOwnerTypePlatform, platformOwnerID)
	if err != nil {
		return result, fmt.Errorf("platform baofu receiver: %w", err)
	}
	result.PlatformSharingMerID = platformReceiver
	return result, nil
}

func resolveBaofuProfitSharingReceiver(ctx context.Context, store baofuProfitSharingReceiverStore, ownerType string, ownerID int64) (string, error) {
	binding, err := store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: ownerType,
		OwnerID:   ownerID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return "", ErrBaofuAccountReceiverRequired
		}
		return "", fmt.Errorf("get baofu account binding: %w", err)
	}
	if binding.OpenState != db.BaofuAccountOpenStateActive {
		return "", ErrBaofuAccountReceiverRequired
	}
	receiverID := baofuProfitSharingReceiverID(binding)
	if receiverID == "" {
		return "", ErrBaofuAccountReceiverRequired
	}
	return receiverID, nil
}

func baofuProfitSharingReceiverID(binding db.BaofuAccountBinding) string {
	if binding.SharingMerID.Valid {
		if receiverID := strings.TrimSpace(binding.SharingMerID.String); receiverID != "" {
			return receiverID
		}
	}
	if binding.ContractNo.Valid {
		return strings.TrimSpace(binding.ContractNo.String)
	}
	return ""
}
