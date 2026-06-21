package db

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	ForeignKeyViolation = "23503"
	UniqueViolation     = "23505"
)

var ErrRecordNotFound = pgx.ErrNoRows

var ErrClaimCompensationNotEligible = errors.New("claim is not eligible for compensation continuation")
var ErrClaimResponsibleRiderMissing = errors.New("claim rider recovery requires a concrete responsible rider")
var ErrGroupApplicationReviewConflict = errors.New("group application review conflict")
var ErrGroupJoinRequestApplicantMismatch = errors.New("group join request applicant mismatch")
var ErrGroupJoinRequestGroupMismatch = errors.New("group join request group mismatch")
var ErrGroupJoinRequestReviewConflict = errors.New("group join request review conflict")
var ErrMerchantAlreadyJoinedGroup = errors.New("merchant already joined group")
var ErrCustomizationTagUnavailable = errors.New("customization tag is unavailable")
var ErrDuplicateCustomizationOption = errors.New("duplicate customization option")
var ErrMerchantDishCategoryHasActiveDishes = errors.New("merchant dish category has active dishes")
var ErrMerchantDishCategoryNotLinked = errors.New("merchant dish category is not linked")
var ErrMerchantPackagingDefaultOptionUnavailable = errors.New("merchant packaging default option is unavailable")
var ErrTableDisabledForReservation = errors.New("table is disabled and cannot be reserved")
var ErrTableMerchantMismatchForReservation = errors.New("table merchant mismatch for reservation")
var ErrTableNotFoundForReservation = errors.New("table not found for reservation")
var ErrTableTypeNotReservable = errors.New("table type is not reservable")
var ErrReservationMerchantMismatch = errors.New("reservation merchant mismatch")
var ErrReservationTerminalState = errors.New("reservation cannot be modified in terminal state")
var ErrReservationTimeConflict = errors.New("reservation time conflict")
var ErrReservationGuestCountExceedsCapacity = errors.New("reservation guest count exceeds table capacity")
var ErrReservationMinimumSpendNotMet = errors.New("reservation minimum spend not met")
var ErrReservationInvalidOfflineCustomerContact = errors.New("reservation offline customer contact is invalid")

// ErrPaymentMissingOrderID indicates a payment_order with business_type=order has no order_id.
// Callers should skip retry and alert for manual intervention.
var ErrPaymentMissingOrderID = errors.New("payment_order.order_id is NULL for business_type=order")

var ErrUniqueViolation = &pgconn.PgError{
	Code: UniqueViolation,
}

func ErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
