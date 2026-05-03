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
