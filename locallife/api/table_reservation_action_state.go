package api

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

type merchantReservationActionStateResponse struct {
	CanEdit          bool   `json:"can_edit"`
	CanCancel        bool   `json:"can_cancel"`
	CanConfirm       bool   `json:"can_confirm"`
	CanCheckIn       bool   `json:"can_check_in"`
	CanStartCooking  bool   `json:"can_start_cooking"`
	CanNoShow        bool   `json:"can_no_show"`
	CanComplete      bool   `json:"can_complete"`
	PrimaryActionKey string `json:"primary_action_key,omitempty"`
	ShowMoreActions  bool   `json:"show_more_actions"`
}

func newMerchantListReservationResponse(r db.ListReservationsByMerchantRow, staffRole string, now time.Time) reservationResponse {
	resp := reservationResponse{
		ID:              r.ID,
		TableID:         r.TableID,
		TableNo:         r.TableNo,
		TableType:       r.TableType,
		UserID:          r.UserID,
		MerchantID:      r.MerchantID,
		ReservationDate: r.ReservationDate.Time.Format("2006-01-02"),
		GuestCount:      r.GuestCount,
		ContactName:     r.ContactName,
		ContactPhone:    r.ContactPhone,
		Source:          r.Source.String,
		PaymentMode:     r.PaymentMode,
		DepositAmount:   r.DepositAmount,
		PrepaidAmount:   r.PrepaidAmount,
		RefundDeadline:  r.RefundDeadline,
		PaymentDeadline: r.PaymentDeadline,
		Status:          r.Status,
		CreatedAt:       r.CreatedAt,
		MerchantActionState: buildMerchantReservationActionStateResponse(
			r.Status,
			r.ReservationDate,
			r.ReservationTime,
			r.CookingStartedAt,
			staffRole,
			now,
		),
	}

	if r.ReservationTime.Valid {
		hours := r.ReservationTime.Microseconds / 1000000 / 3600
		minutes := (r.ReservationTime.Microseconds / 1000000 % 3600) / 60
		resp.ReservationTime = time.Date(0, 1, 1, int(hours), int(minutes), 0, 0, time.UTC).Format("15:04")
	}
	if r.Notes.Valid {
		resp.Notes = &r.Notes.String
	}
	if r.PaidAt.Valid {
		resp.PaidAt = &r.PaidAt.Time
	}
	if r.ConfirmedAt.Valid {
		resp.ConfirmedAt = &r.ConfirmedAt.Time
	}
	if r.CheckedInAt.Valid {
		resp.CheckedInAt = &r.CheckedInAt.Time
	}
	if r.CookingStartedAt.Valid {
		resp.CookingStartedAt = &r.CookingStartedAt.Time
	}
	if r.CompletedAt.Valid {
		resp.CompletedAt = &r.CompletedAt.Time
	}
	if r.CancelledAt.Valid {
		resp.CancelledAt = &r.CancelledAt.Time
	}
	if r.CancelReason.Valid {
		resp.CancelReason = &r.CancelReason.String
	}
	if r.UpdatedAt.Valid {
		resp.UpdatedAt = &r.UpdatedAt.Time
	}

	return resp
}

func buildMerchantReservationActionStateResponse(
	status string,
	reservationDate pgtype.Date,
	reservationTime pgtype.Time,
	cookingStartedAt pgtype.Timestamptz,
	staffRole string,
	now time.Time,
) *merchantReservationActionStateResponse {
	var reservationDateValue *time.Time
	if reservationDate.Valid {
		dateValue := reservationDate.Time
		reservationDateValue = &dateValue
	}

	var reservationTimeValue *time.Time
	if reservationTime.Valid {
		timeValue := time.Date(
			0,
			time.January,
			1,
			int(reservationTime.Microseconds/1000000/3600),
			int((reservationTime.Microseconds/1000000%3600)/60),
			0,
			0,
			time.Local,
		)
		reservationTimeValue = &timeValue
	}

	var cookingStartedAtValue *time.Time
	if cookingStartedAt.Valid {
		cookingStartedAtValue = &cookingStartedAt.Time
	}

	state := logic.ResolveMerchantReservationActionState(logic.MerchantReservationActionStateInput{
		Status:             status,
		ReservationDate:    reservationDateValue,
		ReservationTime:    reservationTimeValue,
		CookingStartedAt:   cookingStartedAtValue,
		StaffRole:          staffRole,
		Now:                now,
		CheckInEarlyMinute: ReservationCheckInEarlyMinutes,
		CheckInLateMinute:  ReservationCheckInLateMinutes,
	})

	return &merchantReservationActionStateResponse{
		CanEdit:          state.CanEdit,
		CanCancel:        state.CanCancel,
		CanConfirm:       state.CanConfirm,
		CanCheckIn:       state.CanCheckIn,
		CanStartCooking:  state.CanStartCooking,
		CanNoShow:        state.CanNoShow,
		CanComplete:      state.CanComplete,
		PrimaryActionKey: state.PrimaryActionKey,
		ShowMoreActions:  state.ShowMoreActions,
	}
}
