package api

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/token"
)

var (
	errOCRMediaUnauthorized  = errors.New("forbidden")
	errOCRMediaWrongCategory = errors.New("media asset category does not match OCR document")
	errOCRMediaNotConfirmed  = errors.New("media asset upload is not confirmed")
)

type authorizedOCRMediaAsset struct {
	Asset            db.MediaAsset
	ModerationStatus string
}

func (server *Server) loadAuthorizedOCRMediaAsset(
	ctx *gin.Context,
	authPayload *token.Payload,
	ownerType ocr.OwnerType,
	documentType ocr.DocumentType,
	side ocr.DocumentSide,
	mediaAssetID int64,
) (authorizedOCRMediaAsset, error) {
	asset, err := server.store.GetMediaAssetByID(ctx, mediaAssetID)
	if err != nil {
		return authorizedOCRMediaAsset{}, err
	}
	if asset.UploadedBy != authPayload.UserID {
		return authorizedOCRMediaAsset{}, errOCRMediaUnauthorized
	}
	if strings.ToLower(strings.TrimSpace(asset.UploadStatus)) != "confirmed" {
		return authorizedOCRMediaAsset{}, errOCRMediaNotConfirmed
	}
	if !ocrMediaCategoryAllowed(asset.MediaCategory, expectedOCRMediaCategories(ownerType, documentType, side)) {
		return authorizedOCRMediaAsset{}, errOCRMediaWrongCategory
	}

	moderationStatus := strings.ToLower(strings.TrimSpace(asset.ModerationStatus))
	if asset.Visibility == string(media.VisibilityPrivate) && isPrivateDocumentMediaModerationExempt(asset.MediaCategory) {
		moderationStatus = "approved"
	}
	return authorizedOCRMediaAsset{Asset: asset, ModerationStatus: moderationStatus}, nil
}

func expectedOCRMediaCategories(ownerType ocr.OwnerType, documentType ocr.DocumentType, side ocr.DocumentSide) []media.Category {
	switch ownerType {
	case ocr.OwnerTypeMerchantApplication:
		return expectedMerchantApplicationOCRMediaCategories(documentType, side)
	case ocr.OwnerTypeOperatorApplication:
		return expectedOperatorApplicationOCRMediaCategories(documentType, side)
	case ocr.OwnerTypeRiderApplication:
		return expectedRiderApplicationOCRMediaCategories(documentType, side)
	case ocr.OwnerTypeGroupApplication:
		return expectedGroupApplicationOCRMediaCategories(documentType, side)
	default:
		return nil
	}
}

func expectedMerchantApplicationOCRMediaCategories(documentType ocr.DocumentType, side ocr.DocumentSide) []media.Category {
	switch documentType {
	case ocr.DocumentTypeBusinessLicense:
		return []media.Category{media.CategoryBusinessLicense}
	case ocr.DocumentTypeFoodPermit:
		return []media.Category{media.CategoryFoodPermit}
	case ocr.DocumentTypeIDCard:
		return expectedIDCardMediaCategories(side)
	default:
		return nil
	}
}

func expectedOperatorApplicationOCRMediaCategories(documentType ocr.DocumentType, side ocr.DocumentSide) []media.Category {
	switch documentType {
	case ocr.DocumentTypeBusinessLicense:
		return []media.Category{media.CategoryBusinessLicense}
	case ocr.DocumentTypeIDCard:
		return expectedIDCardMediaCategories(side)
	default:
		return nil
	}
}

func expectedRiderApplicationOCRMediaCategories(documentType ocr.DocumentType, side ocr.DocumentSide) []media.Category {
	switch documentType {
	case ocr.DocumentTypeHealthCert:
		return []media.Category{media.CategoryHealthCert}
	case ocr.DocumentTypeIDCard:
		return expectedIDCardMediaCategories(side)
	default:
		return nil
	}
}

func expectedGroupApplicationOCRMediaCategories(documentType ocr.DocumentType, side ocr.DocumentSide) []media.Category {
	switch documentType {
	case ocr.DocumentTypeBusinessLicense:
		return []media.Category{media.CategoryGroupLicense, media.CategoryBusinessLicense}
	case ocr.DocumentTypeIDCard:
		return expectedIDCardMediaCategories(side)
	default:
		return nil
	}
}

func expectedIDCardMediaCategories(side ocr.DocumentSide) []media.Category {
	switch side {
	case ocr.DocumentSideFront:
		return []media.Category{media.CategoryIDCardFront}
	case ocr.DocumentSideBack:
		return []media.Category{media.CategoryIDCardBack}
	default:
		return nil
	}
}

func ocrMediaCategoryAllowed(actual string, expected []media.Category) bool {
	actual = strings.ToLower(strings.TrimSpace(actual))
	for _, category := range expected {
		if actual == string(category) {
			return true
		}
	}
	return false
}
