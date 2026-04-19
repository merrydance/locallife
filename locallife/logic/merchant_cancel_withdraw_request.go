package logic

import (
	"errors"
	"fmt"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
)

const merchantApplymentOrganizationTypeEnterprise = "2"

func ValidateMerchantCancelWithdrawBusinessLicenseDeclaration(organizationType string, declaration string, proofMediaCount int) error {
	normalizedDeclaration := strings.ToUpper(strings.TrimSpace(declaration))
	switch normalizedDeclaration {
	case "", db.MerchantCancelWithdrawBusinessLicenseStatusActive, db.MerchantCancelWithdrawBusinessLicenseStatusCanceled, db.MerchantCancelWithdrawBusinessLicenseStatusRevoked:
	default:
		return fmt.Errorf("business_license_status_declaration must be ACTIVE, CANCELED, or REVOKED")
	}

	if strings.TrimSpace(organizationType) != merchantApplymentOrganizationTypeEnterprise {
		if normalizedDeclaration != "" {
			return errors.New("business_license_status_declaration is only allowed for enterprise merchants")
		}
		return nil
	}

	if normalizedDeclaration == db.MerchantCancelWithdrawBusinessLicenseStatusCanceled || normalizedDeclaration == db.MerchantCancelWithdrawBusinessLicenseStatusRevoked {
		if proofMediaCount == 0 {
			return errors.New("proof_media_asset_ids is required when enterprise business_license_status_declaration is CANCELED or REVOKED")
		}
	}

	return nil
}
