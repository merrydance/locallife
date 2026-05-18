package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createBaofuOpeningProfileForTest(t *testing.T, ownerType string, ownerID int64, accountType string) BaofuAccountOpeningProfile {
	t.Helper()

	profile, err := testStore.UpsertBaofuAccountOpeningProfile(context.Background(), UpsertBaofuAccountOpeningProfileParams{
		OwnerType:               ownerType,
		OwnerID:                 ownerID,
		AccountType:             accountType,
		ProfileStatus:           BaofuAccountOpeningProfileStatusComplete,
		LegalName:               pgtype.Text{String: "测试主体", Valid: true},
		CertificateType:         pgtype.Text{String: "ID", Valid: true},
		CertificateNoCiphertext: pgtype.Text{String: "cipher-cert-" + util.RandomString(12), Valid: true},
		CertificateNoMask:       pgtype.Text{String: "110***********001", Valid: true},
		BankAccountNoCiphertext: pgtype.Text{String: "cipher-bank-" + util.RandomString(12), Valid: true},
		BankAccountNoMask:       pgtype.Text{String: "6222********001", Valid: true},
		BankMobileCiphertext:    pgtype.Text{String: "cipher-mobile-" + util.RandomString(12), Valid: true},
		BankMobileMask:          pgtype.Text{String: "138****0001", Valid: true},
		SourceSnapshot:          []byte(`{"source":"test"}`),
	})
	require.NoError(t, err)
	require.NotZero(t, profile.ID)
	return profile
}

func createBaofuOpeningFlowForTest(t *testing.T, ownerType string, ownerID int64, accountType string, state string) BaofuAccountOpeningFlow {
	t.Helper()

	profile := createBaofuOpeningProfileForTest(t, ownerType, ownerID, accountType)
	flow, err := testStore.CreateBaofuAccountOpeningFlow(context.Background(), CreateBaofuAccountOpeningFlowParams{
		OwnerType:               ownerType,
		OwnerID:                 ownerID,
		AccountType:             accountType,
		ProfileID:               pgtype.Int8{Int64: profile.ID, Valid: true},
		State:                   state,
		VerifyFeeAmount:         200,
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.NoError(t, err)
	require.NotZero(t, flow.ID)
	return flow
}

func createBaofuVerifyFeePaymentOrderForTest(t *testing.T, ownerType string, ownerID int64, status string) PaymentOrder {
	t.Helper()

	attach := fmt.Sprintf(
		"business:%s;owner_type:%s;owner_id:%d;purpose:initial_open",
		PaymentBusinessTypeBaofuAccountVerifyFee,
		ownerType,
		ownerID,
	)
	payer := createRandomUser(t)
	payment, err := testStore.CreatePaymentOrder(context.Background(), CreatePaymentOrderParams{
		UserID:                payer.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        PaymentChannelDirect,
		RequiresProfitSharing: false,
		BusinessType:          PaymentBusinessTypeBaofuAccountVerifyFee,
		Amount:                200,
		OutTradeNo:            "BFVF" + util.RandomString(24),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
		Attach:                pgtype.Text{String: attach, Valid: true},
	})
	require.NoError(t, err)

	if status == "paid" {
		payment, err = testStore.UpdatePaymentOrderToPaid(context.Background(), UpdatePaymentOrderToPaidParams{
			ID:            payment.ID,
			TransactionID: pgtype.Text{String: "420" + util.RandomString(20), Valid: true},
		})
		require.NoError(t, err)
	}

	return payment
}

func TestBaofuAccountOpeningProfileConstraintsAndUpsert(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()

	profile := createBaofuOpeningProfileForTest(t, BaofuAccountOwnerTypeRider, ownerID, BaofuAccountTypePersonal)
	require.Equal(t, BaofuAccountOpeningProfileStatusComplete, profile.ProfileStatus)
	require.Equal(t, "cipher-cert-", profile.CertificateNoCiphertext.String[:12])
	require.Equal(t, "110***********001", profile.CertificateNoMask.String)
	require.Equal(t, "cipher-bank-", profile.BankAccountNoCiphertext.String[:12])
	require.Equal(t, "6222********001", profile.BankAccountNoMask.String)

	updated, err := testStore.UpsertBaofuAccountOpeningProfile(ctx, UpsertBaofuAccountOpeningProfileParams{
		OwnerType:               BaofuAccountOwnerTypeRider,
		OwnerID:                 ownerID,
		AccountType:             BaofuAccountTypePersonal,
		ProfileStatus:           BaofuAccountOpeningProfileStatusIncomplete,
		CertificateNoCiphertext: pgtype.Text{String: "cipher-cert-updated", Valid: true},
		CertificateNoMask:       pgtype.Text{String: "110***********999", Valid: true},
		BankAccountNoCiphertext: pgtype.Text{String: "cipher-bank-updated", Valid: true},
		BankAccountNoMask:       pgtype.Text{String: "6222********999", Valid: true},
		SourceSnapshot:          []byte(`{"source":"updated"}`),
	})
	require.NoError(t, err)
	require.Equal(t, profile.ID, updated.ID)
	require.Equal(t, BaofuAccountOpeningProfileStatusIncomplete, updated.ProfileStatus)
	require.Equal(t, "cipher-cert-updated", updated.CertificateNoCiphertext.String)
	require.Equal(t, "110***********999", updated.CertificateNoMask.String)
	require.NotContains(t, string(updated.SourceSnapshot), "6222000000000999")

	_, err = testStore.UpsertBaofuAccountOpeningProfile(ctx, UpsertBaofuAccountOpeningProfileParams{
		OwnerType:      "customer",
		OwnerID:        time.Now().UnixNano(),
		AccountType:    BaofuAccountTypePersonal,
		ProfileStatus:  BaofuAccountOpeningProfileStatusComplete,
		SourceSnapshot: []byte(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, "23514", ErrorCode(err))

	_, err = testStore.UpsertBaofuAccountOpeningProfile(ctx, UpsertBaofuAccountOpeningProfileParams{
		OwnerType:      BaofuAccountOwnerTypeRider,
		OwnerID:        time.Now().UnixNano(),
		AccountType:    "platform",
		ProfileStatus:  BaofuAccountOpeningProfileStatusComplete,
		SourceSnapshot: []byte(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, "23514", ErrorCode(err))
}

func TestBaofuAccountOpeningFlowConstraints(t *testing.T) {
	ctx := context.Background()

	_, err := testStore.CreateBaofuAccountOpeningFlow(ctx, CreateBaofuAccountOpeningFlowParams{
		OwnerType:               "customer",
		OwnerID:                 time.Now().UnixNano(),
		AccountType:             BaofuAccountTypePersonal,
		State:                   BaofuAccountOpeningStateProfilePending,
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, "23514", ErrorCode(err))

	_, err = testStore.CreateBaofuAccountOpeningFlow(ctx, CreateBaofuAccountOpeningFlowParams{
		OwnerType:               BaofuAccountOwnerTypeRider,
		OwnerID:                 time.Now().UnixNano(),
		AccountType:             "platform",
		State:                   BaofuAccountOpeningStateProfilePending,
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, "23514", ErrorCode(err))

	_, err = testStore.CreateBaofuAccountOpeningFlow(ctx, CreateBaofuAccountOpeningFlowParams{
		OwnerType:               BaofuAccountOwnerTypeMerchant,
		OwnerID:                 time.Now().UnixNano(),
		AccountType:             BaofuAccountTypeBusiness,
		State:                   BaofuAccountOpeningStateOpeningProcessing,
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, "23514", ErrorCode(err))

	_, err = testStore.CreateBaofuAccountOpeningFlow(ctx, CreateBaofuAccountOpeningFlowParams{
		OwnerType:               BaofuAccountOwnerTypeRider,
		OwnerID:                 time.Now().UnixNano(),
		AccountType:             BaofuAccountTypePersonal,
		State:                   BaofuAccountOpeningStateOpeningProcessing,
		LoginNo:                 pgtype.Text{String: "LLBFOR" + util.RandomString(12), Valid: true},
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, "23514", ErrorCode(err))
}

func TestBaofuAccountOpeningFlowActiveUniquenessAndReplacement(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()

	flow := createBaofuOpeningFlowForTest(
		t,
		BaofuAccountOwnerTypeMerchant,
		ownerID,
		BaofuAccountTypeBusiness,
		BaofuAccountOpeningStateProfilePending,
	)

	_, err := testStore.CreateBaofuAccountOpeningFlow(ctx, CreateBaofuAccountOpeningFlowParams{
		OwnerType:               BaofuAccountOwnerTypeMerchant,
		OwnerID:                 ownerID,
		AccountType:             BaofuAccountTypeBusiness,
		State:                   BaofuAccountOpeningStateVerifyFeePending,
		VerifyFeeAmount:         200,
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, UniqueViolation, ErrorCode(err))

	processing, err := testStore.MarkBaofuAccountOpeningFlowOpeningProcessing(ctx, MarkBaofuAccountOpeningFlowOpeningProcessingParams{
		ID:                      flow.ID,
		OpenTransSerialNo:       pgtype.Text{String: "OPEN" + util.RandomString(18), Valid: true},
		LoginNo:                 pgtype.Text{String: "LLBFOM" + util.RandomString(12), Valid: true},
		ProviderRequestSnapshot: []byte(`{"needUploadFile":false}`),
		RawSnapshot:             []byte(`{"state":"processing"}`),
	})
	require.NoError(t, err)
	require.Equal(t, BaofuAccountOpeningStateOpeningProcessing, processing.State)

	ready, err := testStore.MarkBaofuAccountOpeningFlowReady(ctx, MarkBaofuAccountOpeningFlowReadyParams{
		ID:          flow.ID,
		RawSnapshot: []byte(`{"state":"ready"}`),
	})
	require.NoError(t, err)
	require.Equal(t, BaofuAccountOpeningStateReady, ready.State)

	replacement := createBaofuOpeningFlowForTest(
		t,
		BaofuAccountOwnerTypeMerchant,
		ownerID,
		BaofuAccountTypeBusiness,
		BaofuAccountOpeningStateProfilePending,
	)
	require.NotEqual(t, flow.ID, replacement.ID)
}

func TestBaofuAccountOpeningFlowFailedReplacementAndOpeningGuards(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	flow := createBaofuOpeningFlowForTest(
		t,
		BaofuAccountOwnerTypeRider,
		ownerID,
		BaofuAccountTypePersonal,
		BaofuAccountOpeningStateVerifyFeePending,
	)

	_, err := testStore.MarkBaofuAccountOpeningFlowOpeningProcessing(ctx, MarkBaofuAccountOpeningFlowOpeningProcessingParams{
		ID:                      flow.ID,
		OpenTransSerialNo:       pgtype.Text{String: "OPEN" + util.RandomString(18), Valid: true},
		LoginNo:                 pgtype.Text{String: "LLBFOR" + util.RandomString(12), Valid: true},
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, "23514", ErrorCode(err))

	paidPayment := createBaofuVerifyFeePaymentOrderForTest(t, BaofuAccountOwnerTypeRider, ownerID, "paid")
	processing, err := testStore.MarkBaofuAccountOpeningFlowOpeningProcessing(ctx, MarkBaofuAccountOpeningFlowOpeningProcessingParams{
		ID:                      flow.ID,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: paidPayment.ID, Valid: true},
		OpenTransSerialNo:       pgtype.Text{String: "OPEN" + util.RandomString(18), Valid: true},
		LoginNo:                 pgtype.Text{String: "LLBFOR" + util.RandomString(12), Valid: true},
		ProviderRequestSnapshot: []byte(`{"needUploadFile":false}`),
		RawSnapshot:             []byte(`{"state":"processing"}`),
	})
	require.NoError(t, err)
	require.Equal(t, BaofuAccountOpeningStateOpeningProcessing, processing.State)
	require.Equal(t, paidPayment.ID, processing.VerifyFeePaymentOrderID.Int64)

	failed, err := testStore.MarkBaofuAccountOpeningFlowFailed(ctx, MarkBaofuAccountOpeningFlowFailedParams{
		ID:             flow.ID,
		FailureCode:    pgtype.Text{String: "provider_failed", Valid: true},
		FailureMessage: pgtype.Text{String: "safe message", Valid: true},
		RawSnapshot:    []byte(`{"state":"failed"}`),
	})
	require.NoError(t, err)
	require.Equal(t, BaofuAccountOpeningStateFailed, failed.State)

	replacement := createBaofuOpeningFlowForTest(
		t,
		BaofuAccountOwnerTypeRider,
		ownerID,
		BaofuAccountTypePersonal,
		BaofuAccountOpeningStateVerifyFeePending,
	)
	require.NotEqual(t, flow.ID, replacement.ID)
}

func TestMarkBaofuAccountOpeningFlowReadyAllowsDuplicateFailedRecovery(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	flow := createBaofuOpeningFlowForTest(t, BaofuAccountOwnerTypeOperator, ownerID, BaofuAccountTypePersonal, BaofuAccountOpeningStateVerifyFeePending)
	payment := createBaofuVerifyFeePaymentOrderForTest(t, BaofuAccountOwnerTypeOperator, ownerID, "pending")
	flow, err := testStore.MarkBaofuAccountOpeningFlowOpeningProcessing(ctx, MarkBaofuAccountOpeningFlowOpeningProcessingParams{
		ID:                      flow.ID,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: payment.ID, Valid: true},
		OpenTransSerialNo:       pgtype.Text{String: "OPEN" + util.RandomString(18), Valid: true},
		LoginNo:                 pgtype.Text{String: "LLBFOO" + util.RandomString(12), Valid: true},
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.NoError(t, err)
	failed, err := testStore.MarkBaofuAccountOpeningFlowFailed(ctx, MarkBaofuAccountOpeningFlowFailedParams{
		ID:             flow.ID,
		FailureCode:    pgtype.Text{String: "BF00060", Valid: true},
		FailureMessage: pgtype.Text{String: "该子商户已开户，请勿重复提交", Valid: true},
		RawSnapshot:    []byte(`{"state":"failed","errorCode":"BF00060"}`),
	})
	require.NoError(t, err)

	ready, err := testStore.MarkBaofuAccountOpeningFlowReady(ctx, MarkBaofuAccountOpeningFlowReadyParams{
		ID:          failed.ID,
		RawSnapshot: []byte(`{"state":"ready"}`),
	})

	require.NoError(t, err)
	require.Equal(t, BaofuAccountOpeningStateReady, ready.State)
}

func TestListRecoverableBaofuAccountOpeningFlowsIncludesLatestDuplicateFailure(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()

	oldFlow := createBaofuOpeningFlowForTest(t, BaofuAccountOwnerTypeOperator, ownerID, BaofuAccountTypePersonal, BaofuAccountOpeningStateVerifyFeePending)
	oldPayment := createBaofuVerifyFeePaymentOrderForTest(t, BaofuAccountOwnerTypeOperator, ownerID, "pending")
	oldFlow, err := testStore.MarkBaofuAccountOpeningFlowOpeningProcessing(ctx, MarkBaofuAccountOpeningFlowOpeningProcessingParams{
		ID:                      oldFlow.ID,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: oldPayment.ID, Valid: true},
		OpenTransSerialNo:       pgtype.Text{String: "OPEN" + util.RandomString(18), Valid: true},
		LoginNo:                 pgtype.Text{String: "LLBFOO" + util.RandomString(12), Valid: true},
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.NoError(t, err)
	oldFailed, err := testStore.MarkBaofuAccountOpeningFlowFailed(ctx, MarkBaofuAccountOpeningFlowFailedParams{
		ID:             oldFlow.ID,
		FailureCode:    pgtype.Text{String: "BF00060", Valid: true},
		FailureMessage: pgtype.Text{String: "该子商户已开户，请勿重复提交", Valid: true},
		RawSnapshot:    []byte(`{"state":"failed","errorCode":"BF00060"}`),
	})
	require.NoError(t, err)
	_, err = testStore.UpdatePaymentOrderToClosed(ctx, oldPayment.ID)
	require.NoError(t, err)

	latestFlow := createBaofuOpeningFlowForTest(t, BaofuAccountOwnerTypeOperator, ownerID, BaofuAccountTypePersonal, BaofuAccountOpeningStateVerifyFeePending)
	latestPayment := createBaofuVerifyFeePaymentOrderForTest(t, BaofuAccountOwnerTypeOperator, ownerID, "pending")
	latestFlow, err = testStore.MarkBaofuAccountOpeningFlowOpeningProcessing(ctx, MarkBaofuAccountOpeningFlowOpeningProcessingParams{
		ID:                      latestFlow.ID,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: latestPayment.ID, Valid: true},
		OpenTransSerialNo:       pgtype.Text{String: "OPEN" + util.RandomString(18), Valid: true},
		LoginNo:                 pgtype.Text{String: "LLBFOO" + util.RandomString(12), Valid: true},
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.NoError(t, err)
	latestFailed, err := testStore.MarkBaofuAccountOpeningFlowFailed(ctx, MarkBaofuAccountOpeningFlowFailedParams{
		ID:             latestFlow.ID,
		FailureCode:    pgtype.Text{String: "BF00060", Valid: true},
		FailureMessage: pgtype.Text{String: "该子商户已开户，请勿重复提交", Valid: true},
		RawSnapshot:    []byte(`{"state":"failed","errorCode":"BF00060"}`),
	})
	require.NoError(t, err)

	nonDuplicateOwnerID := time.Now().UnixNano()
	nonDuplicateFlow := createBaofuOpeningFlowForTest(t, BaofuAccountOwnerTypeRider, nonDuplicateOwnerID, BaofuAccountTypePersonal, BaofuAccountOpeningStateVerifyFeePending)
	nonDuplicatePayment := createBaofuVerifyFeePaymentOrderForTest(t, BaofuAccountOwnerTypeRider, nonDuplicateOwnerID, "paid")
	nonDuplicateFlow, err = testStore.MarkBaofuAccountOpeningFlowOpeningProcessing(ctx, MarkBaofuAccountOpeningFlowOpeningProcessingParams{
		ID:                      nonDuplicateFlow.ID,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: nonDuplicatePayment.ID, Valid: true},
		OpenTransSerialNo:       pgtype.Text{String: "OPEN" + util.RandomString(18), Valid: true},
		LoginNo:                 pgtype.Text{String: "LLBFOR" + util.RandomString(12), Valid: true},
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.NoError(t, err)
	nonDuplicateFailed, err := testStore.MarkBaofuAccountOpeningFlowFailed(ctx, MarkBaofuAccountOpeningFlowFailedParams{
		ID:             nonDuplicateFlow.ID,
		FailureCode:    pgtype.Text{String: "BF00061", Valid: true},
		FailureMessage: pgtype.Text{String: "身份核验失败", Valid: true},
		RawSnapshot:    []byte(`{"state":"failed","errorCode":"BF00061"}`),
	})
	require.NoError(t, err)

	flows, err := testStore.ListRecoverableBaofuAccountOpeningFlows(ctx, ListRecoverableBaofuAccountOpeningFlowsParams{
		BeforeAt:   time.Now().Add(time.Minute),
		LimitCount: 100,
	})
	require.NoError(t, err)

	ids := make(map[int64]bool, len(flows))
	for _, flow := range flows {
		ids[flow.ID] = true
	}
	require.False(t, ids[oldFailed.ID])
	require.True(t, ids[latestFailed.ID])
	require.False(t, ids[nonDuplicateFailed.ID])
}
