package logic

import (
	"context"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestCalculateBaofuPaymentFeeFenRoundsUpMerchantBorneFee(t *testing.T) {
	require.Equal(t, int64(30), CalculateBaofuPaymentFeeFen(10000))
	require.Equal(t, int64(1), CalculateBaofuPaymentFeeFen(1))
	require.Equal(t, int64(1), CalculateBaofuPaymentFeeFen(333))
	require.Equal(t, int64(0), CalculateBaofuPaymentFeeFen(0))
}

func TestCalculateBaofuProfitSharingAmountsMerchantBearsPaymentFee(t *testing.T) {
	result, err := CalculateBaofuProfitSharingAmounts(BaofuProfitSharingAmountInput{
		TotalAmountFen:      10000,
		DeliveryFeeFen:      500,
		PlatformRateBps:     200,
		OperatorRateBps:     300,
		HasRiderReceiver:    true,
		HasOperatorReceiver: true,
		RedirectMissingOperatorCommissionToPlatform: true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(500), result.RiderAmountFen)
	require.Equal(t, int64(9500), result.DistributableAmountFen)
	require.Equal(t, int64(200), result.PlatformCommissionFen)
	require.Equal(t, int64(300), result.OperatorCommissionFen)
	require.Equal(t, int64(30), result.PaymentFeeFen)
	require.Equal(t, int64(8970), result.MerchantAmountFen)
	require.False(t, result.OperatorCommissionRedirectedToPlatform)
}

func TestCalculateBaofuProfitSharingAmountsRedirectsMissingOperatorToPlatform(t *testing.T) {
	result, err := CalculateBaofuProfitSharingAmounts(BaofuProfitSharingAmountInput{
		TotalAmountFen:      10000,
		DeliveryFeeFen:      500,
		PlatformRateBps:     200,
		OperatorRateBps:     300,
		HasRiderReceiver:    true,
		HasOperatorReceiver: false,
		RedirectMissingOperatorCommissionToPlatform: true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(500), result.RiderAmountFen)
	require.Equal(t, int64(500), result.PlatformCommissionFen)
	require.Equal(t, int64(0), result.OperatorCommissionFen)
	require.Equal(t, int64(30), result.PaymentFeeFen)
	require.Equal(t, int64(8970), result.MerchantAmountFen)
	require.True(t, result.OperatorCommissionRedirectedToPlatform)
}

func TestCalculateBaofuProfitSharingAmountsRejectsNegativeMerchantAmount(t *testing.T) {
	_, err := CalculateBaofuProfitSharingAmounts(BaofuProfitSharingAmountInput{
		TotalAmountFen:      100,
		DeliveryFeeFen:      0,
		PlatformRateBps:     9900,
		OperatorRateBps:     100,
		HasOperatorReceiver: true,
		RedirectMissingOperatorCommissionToPlatform: true,
	})

	require.ErrorIs(t, err, ErrBaofuProfitSharingMerchantAmountNegative)
}

func TestResolveBaofuProfitSharingReceiversUsesCanonicalSharingMerID(t *testing.T) {
	store := &fakeBaofuProfitSharingReceiverStore{
		bindings: map[string]db.BaofuAccountBinding{
			"merchant:101": activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeMerchant, 101, "MER_CONTRACT", "MER_SHARE"),
			"rider:202":    activeBaofuReceiverBinding(db.BaofuAccountOwnerTypeRider, 202, "RIDER_CONTRACT", "RIDER_SHARE"),
			"operator:303": activeBaofuReceiverBinding(
				db.BaofuAccountOwnerTypeOperator,
				303,
				"OP_CONTRACT",
				"OP_SHARE",
			),
			"platform:0": activeBaofuReceiverBinding(db.BaofuAccountOwnerTypePlatform, 0, "PLATFORM_CONTRACT", "PLATFORM_SHARE"),
		},
	}

	receivers, err := ResolveBaofuProfitSharingReceivers(context.Background(), store, BaofuProfitSharingReceiverInput{
		MerchantID: 101,
		RiderID:    202,
		OperatorID: 303,
	})

	require.NoError(t, err)
	require.Equal(t, "MER_SHARE", receivers.MerchantSharingMerID)
	require.Equal(t, "RIDER_SHARE", receivers.RiderSharingMerID)
	require.Equal(t, "OP_SHARE", receivers.OperatorSharingMerID)
	require.Equal(t, "PLATFORM_SHARE", receivers.PlatformSharingMerID)
}

func TestResolveBaofuProfitSharingReceiversRejectsInactiveReceiver(t *testing.T) {
	store := &fakeBaofuProfitSharingReceiverStore{
		bindings: map[string]db.BaofuAccountBinding{
			"merchant:101": {
				OwnerType:    db.BaofuAccountOwnerTypeMerchant,
				OwnerID:      101,
				AccountType:  db.BaofuAccountTypeBusiness,
				OpenState:    db.BaofuAccountOpenStateProcessing,
				SharingMerID: pgtype.Text{String: "MER_SHARE", Valid: true},
			},
			"platform:0": activeBaofuReceiverBinding(db.BaofuAccountOwnerTypePlatform, 0, "PLATFORM_CONTRACT", "PLATFORM_SHARE"),
		},
	}

	_, err := ResolveBaofuProfitSharingReceivers(context.Background(), store, BaofuProfitSharingReceiverInput{MerchantID: 101})

	require.ErrorIs(t, err, ErrBaofuAccountReceiverRequired)
}

func TestResolveBaofuProfitSharingReceiversRejectsContractOnlyReceiver(t *testing.T) {
	store := &fakeBaofuProfitSharingReceiverStore{
		bindings: map[string]db.BaofuAccountBinding{
			"merchant:101": {
				OwnerType:   db.BaofuAccountOwnerTypeMerchant,
				OwnerID:     101,
				AccountType: db.BaofuAccountTypeBusiness,
				OpenState:   db.BaofuAccountOpenStateActive,
				ContractNo:  pgtype.Text{String: "MER_CONTRACT", Valid: true},
			},
			"platform:0": activeBaofuReceiverBinding(db.BaofuAccountOwnerTypePlatform, 0, "PLATFORM_CONTRACT", "PLATFORM_SHARE"),
		},
	}

	_, err := ResolveBaofuProfitSharingReceivers(context.Background(), store, BaofuProfitSharingReceiverInput{MerchantID: 101})

	require.ErrorIs(t, err, ErrBaofuAccountReceiverRequired)
}

type fakeBaofuProfitSharingReceiverStore struct {
	bindings map[string]db.BaofuAccountBinding
}

func (s *fakeBaofuProfitSharingReceiverStore) GetBaofuAccountBindingByOwner(_ context.Context, arg db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error) {
	if binding, ok := s.bindings[arg.OwnerType+":"+strconv.FormatInt(arg.OwnerID, 10)]; ok {
		return binding, nil
	}
	return db.BaofuAccountBinding{}, db.ErrRecordNotFound
}

func activeBaofuReceiverBinding(ownerType string, ownerID int64, contractNo string, sharingMerID string) db.BaofuAccountBinding {
	return db.BaofuAccountBinding{
		OwnerType:    ownerType,
		OwnerID:      ownerID,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: contractNo, Valid: contractNo != ""},
		SharingMerID: pgtype.Text{String: sharingMerID, Valid: sharingMerID != ""},
	}
}
