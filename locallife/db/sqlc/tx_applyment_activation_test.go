package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestApplymentSubMchActivationTxActivatesOrdinaryMerchantWithoutAccountAuthorizeState(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	_, err := testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: MerchantStatusBindbankSubmitted,
	})
	require.NoError(t, err)

	_, err = testStore.CreateMerchantPaymentConfig(context.Background(), CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   util.RandomString(12),
		Status:     MerchantPaymentConfigStatusPendingAuthorization,
	})
	require.NoError(t, err)

	applyment := createRandomEcommerceApplymentWithSubject(t, "merchant", merchant.ID)
	wechatApplymentID := pgtype.Int8{Int64: util.RandomInt(100000, 999999), Valid: true}
	subMchID := "sub_mch_" + util.RandomString(10)

	err = testStore.ApplymentSubMchActivationTx(context.Background(), ApplymentSubMchActivationTxParams{
		ApplymentID:       applyment.ID,
		WechatApplymentID: wechatApplymentID,
		SubjectType:       applyment.SubjectType,
		SubjectID:         applyment.SubjectID,
		SubMchID:          subMchID,
	})
	require.NoError(t, err)

	updatedApplyment, err := testStore.GetEcommerceApplyment(context.Background(), applyment.ID)
	require.NoError(t, err)
	require.Equal(t, "finish", updatedApplyment.Status)
	require.Equal(t, wechatApplymentID.Int64, updatedApplyment.ApplymentID.Int64)
	require.Equal(t, subMchID, updatedApplyment.SubMchID.String)
	require.False(t, updatedApplyment.AccountAuthorizeState.Valid)

	paymentConfig, err := testStore.GetMerchantPaymentConfig(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, subMchID, paymentConfig.SubMchID)
	require.Equal(t, MerchantPaymentConfigStatusActive, paymentConfig.Status)

	updatedMerchant, err := testStore.GetMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, MerchantStatusActive, updatedMerchant.Status)
}

func TestApplymentSubMchActivationTxCreatesMissingPaymentConfig(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	_, err := testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: MerchantStatusBindbankSubmitted,
	})
	require.NoError(t, err)

	applyment := createRandomEcommerceApplymentWithSubject(t, "merchant", merchant.ID)
	wechatApplymentID := pgtype.Int8{Int64: util.RandomInt(100000, 999999), Valid: true}
	subMchID := "sub_mch_" + util.RandomString(10)

	err = testStore.ApplymentSubMchActivationTx(context.Background(), ApplymentSubMchActivationTxParams{
		ApplymentID:       applyment.ID,
		WechatApplymentID: wechatApplymentID,
		SubjectType:       applyment.SubjectType,
		SubjectID:         applyment.SubjectID,
		SubMchID:          subMchID,
	})
	require.NoError(t, err)

	updatedApplyment, err := testStore.GetEcommerceApplyment(context.Background(), applyment.ID)
	require.NoError(t, err)
	require.Equal(t, "finish", updatedApplyment.Status)
	require.Equal(t, subMchID, updatedApplyment.SubMchID.String)

	paymentConfig, err := testStore.GetMerchantPaymentConfig(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, subMchID, paymentConfig.SubMchID)
	require.Equal(t, MerchantPaymentConfigStatusActive, paymentConfig.Status)

	updatedMerchant, err := testStore.GetMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, MerchantStatusActive, updatedMerchant.Status)
}
