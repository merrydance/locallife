package logic

import (
	"testing"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestValidateMerchantCancelWithdrawBusinessLicenseDeclarationRequiresProofForEnterpriseCanceledOrRevoked(t *testing.T) {
	err := ValidateMerchantCancelWithdrawBusinessLicenseDeclaration("2", db.MerchantCancelWithdrawBusinessLicenseStatusCanceled, 0)
	require.Error(t, err)
	require.ErrorContains(t, err, "proof_media_asset_ids is required")

	err = ValidateMerchantCancelWithdrawBusinessLicenseDeclaration("2", db.MerchantCancelWithdrawBusinessLicenseStatusRevoked, 0)
	require.Error(t, err)
	require.ErrorContains(t, err, "proof_media_asset_ids is required")
}

func TestValidateMerchantCancelWithdrawBusinessLicenseDeclarationAllowsEnterpriseActiveOrEmpty(t *testing.T) {
	require.NoError(t, ValidateMerchantCancelWithdrawBusinessLicenseDeclaration("2", "", 0))
	require.NoError(t, ValidateMerchantCancelWithdrawBusinessLicenseDeclaration("2", db.MerchantCancelWithdrawBusinessLicenseStatusActive, 0))
	require.NoError(t, ValidateMerchantCancelWithdrawBusinessLicenseDeclaration("2", db.MerchantCancelWithdrawBusinessLicenseStatusCanceled, 1))
}

func TestValidateMerchantCancelWithdrawBusinessLicenseDeclarationRejectsNonEnterpriseDeclaration(t *testing.T) {
	err := ValidateMerchantCancelWithdrawBusinessLicenseDeclaration("4", db.MerchantCancelWithdrawBusinessLicenseStatusCanceled, 1)
	require.Error(t, err)
	require.ErrorContains(t, err, "only allowed for enterprise merchants")
}

func TestValidateMerchantCancelWithdrawBusinessLicenseDeclarationRejectsUnsupportedValue(t *testing.T) {
	err := ValidateMerchantCancelWithdrawBusinessLicenseDeclaration("2", "UNKNOWN", 0)
	require.Error(t, err)
	require.ErrorContains(t, err, "must be ACTIVE, CANCELED, or REVOKED")
}
