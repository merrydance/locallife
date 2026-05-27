package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestMarkBaofuAccountBindingActiveWithFeeLedgerTx(t *testing.T) {
	binding, err := testStore.UpsertBaofuAccountBinding(context.Background(), UpsertBaofuAccountBindingParams{
		OwnerType:   BaofuAccountOwnerTypeRider,
		OwnerID:     time.Now().UnixNano(),
		AccountType: BaofuAccountTypePersonal,
		OpenState:   BaofuAccountOpenStateProcessing,
		RawSnapshot: []byte(`{}`),
	})
	require.NoError(t, err)

	sharingMerID := "CP_SHARE_" + time.Now().Format("150405.000000000")
	contractNo := "CP_QUERY_TRACE_" + time.Now().Format("150405.000000000")

	result, err := testStore.MarkBaofuAccountBindingActiveWithFeeLedgerTx(context.Background(), MarkBaofuAccountBindingActiveWithFeeLedgerTxParams{
		ActiveBinding: MarkBaofuAccountBindingActiveParams{
			ID:           binding.ID,
			ContractNo:   pgtype.Text{String: contractNo, Valid: true},
			SharingMerID: pgtype.Text{String: sharingMerID, Valid: true},
			RawSnapshot:  []byte(`{"state":"active"}`),
		},
		AccountOpenFeeLedger: CreateBaofuFeeLedgerParams{
			FeeType:            BaofuFeeTypeAccountOpenVerifyFee,
			PayerType:          BaofuFeePayerTypePlatform,
			PayerID:            pgtype.Int8{Valid: false},
			BusinessObjectType: "baofu_account_binding",
			BusinessObjectID:   binding.ID,
			Amount:             100,
			Status:             "recorded",
		},
	})
	require.NoError(t, err)
	require.Equal(t, BaofuAccountOpenStateActive, result.Binding.OpenState)
	require.Equal(t, sharingMerID, result.Binding.SharingMerID.String)
	require.Equal(t, int64(100), result.AccountOpenFeeLedger.Amount)

	gotLedger, err := testStore.GetBaofuFeeLedgerByBusinessObject(context.Background(), GetBaofuFeeLedgerByBusinessObjectParams{
		FeeType:            BaofuFeeTypeAccountOpenVerifyFee,
		BusinessObjectType: "baofu_account_binding",
		BusinessObjectID:   binding.ID,
	})
	require.NoError(t, err)
	require.Equal(t, result.AccountOpenFeeLedger.ID, gotLedger.ID)
}

func TestMarkMerchantBaofuAccountOpeningReadyTxActivatesApprovedMerchant(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	merchant, err := testStore.UpdateMerchantStatus(ctx, UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: MerchantStatusApproved,
	})
	require.NoError(t, err)
	flow := createBaofuOpeningFlowForTest(t, BaofuAccountOwnerTypeMerchant, merchant.ID, BaofuAccountTypeBusiness, BaofuAccountOpeningStateAppletAuthPending)
	subMchID := "190" + time.Now().Format("150405000000")

	result, err := testStore.MarkMerchantBaofuAccountOpeningReadyTx(ctx, MarkMerchantBaofuAccountOpeningReadyTxParams{
		PaymentConfig: UpsertMerchantPaymentConfigParams{
			MerchantID: merchant.ID,
			SubMchID:   subMchID,
			Status:     MerchantPaymentConfigStatusActive,
		},
		Flow: MarkBaofuAccountOpeningFlowReadyParams{
			ID:          flow.ID,
			RawSnapshot: []byte(`{"state":"ready"}`),
		},
	})

	require.NoError(t, err)
	require.Equal(t, BaofuAccountOpeningStateReady, result.Flow.State)
	require.Equal(t, MerchantStatusActive, result.Merchant.Status)
	require.Equal(t, subMchID, result.PaymentConfig.SubMchID)

	gotMerchant, err := testStore.GetMerchant(ctx, merchant.ID)
	require.NoError(t, err)
	require.Equal(t, MerchantStatusActive, gotMerchant.Status)
}

func TestMarkMerchantBaofuAccountOpeningReadyTxDoesNotResumeSuspendedMerchant(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	merchant, err := testStore.UpdateMerchantStatus(ctx, UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: MerchantStatusSuspended,
	})
	require.NoError(t, err)
	flow := createBaofuOpeningFlowForTest(t, BaofuAccountOwnerTypeMerchant, merchant.ID, BaofuAccountTypeBusiness, BaofuAccountOpeningStateAppletAuthPending)

	result, err := testStore.MarkMerchantBaofuAccountOpeningReadyTx(ctx, MarkMerchantBaofuAccountOpeningReadyTxParams{
		PaymentConfig: UpsertMerchantPaymentConfigParams{
			MerchantID: merchant.ID,
			SubMchID:   "190" + time.Now().Format("150405000001"),
			Status:     MerchantPaymentConfigStatusActive,
		},
		Flow: MarkBaofuAccountOpeningFlowReadyParams{
			ID:          flow.ID,
			RawSnapshot: []byte(`{"state":"ready"}`),
		},
	})

	require.NoError(t, err)
	require.Equal(t, BaofuAccountOpeningStateReady, result.Flow.State)
	require.Equal(t, MerchantStatusSuspended, result.Merchant.Status)

	gotMerchant, err := testStore.GetMerchant(ctx, merchant.ID)
	require.NoError(t, err)
	require.Equal(t, MerchantStatusSuspended, gotMerchant.Status)
}

func TestMarkBaofuAccountBindingAbnormal(t *testing.T) {
	binding, err := testStore.UpsertBaofuAccountBinding(context.Background(), UpsertBaofuAccountBindingParams{
		OwnerType:   BaofuAccountOwnerTypeRider,
		OwnerID:     time.Now().UnixNano(),
		AccountType: BaofuAccountTypePersonal,
		OpenState:   BaofuAccountOpenStateProcessing,
		RawSnapshot: []byte(`{}`),
	})
	require.NoError(t, err)

	updated, err := testStore.MarkBaofuAccountBindingAbnormal(context.Background(), MarkBaofuAccountBindingAbnormalParams{
		ID:          binding.ID,
		RawSnapshot: []byte(`{"state":"-1"}`),
	})

	require.NoError(t, err)
	require.Equal(t, BaofuAccountOpenStateAbnormal, updated.OpenState)
	require.JSONEq(t, `{"state":"-1"}`, string(updated.RawSnapshot))
}
