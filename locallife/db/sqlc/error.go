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
var ErrGroupJoinRequestReviewConflict = errors.New("group join request review conflict")
var ErrMerchantAlreadyJoinedGroup = errors.New("merchant already joined group")

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
