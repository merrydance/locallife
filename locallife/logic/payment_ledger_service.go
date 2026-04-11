package logic

import (
	"context"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

type PaymentLedgerEntry struct {
	ID             int64
	EntryType      string
	PaymentOrderID int64
	RefundOrderID  *int64
	OrderID        *int64
	BusinessType   string
	Amount         int64
	Status         string
	OccurredAt     time.Time
	CreatedAt      time.Time
}

type ListPaymentLedgerInput struct {
	UserID   int64
	PageID   int32
	PageSize int32
}

type ListPaymentLedgerResult struct {
	Entries    []PaymentLedgerEntry
	TotalCount int64
}

type PaymentLedgerService struct {
	store db.Store
}

func NewPaymentLedgerService(store db.Store) *PaymentLedgerService {
	return &PaymentLedgerService{store: store}
}

func (svc *PaymentLedgerService) ListPaymentLedger(ctx context.Context, input ListPaymentLedgerInput) (ListPaymentLedgerResult, error) {
	pageID := input.PageID
	pageSize := input.PageSize
	if pageID == 0 {
		pageID = 1
	}
	if pageSize == 0 {
		pageSize = 10
	}

	offset := (pageID - 1) * pageSize
	rows, err := svc.store.ListPaymentLedgerEntriesByUser(ctx, db.ListPaymentLedgerEntriesByUserParams{
		UserID: input.UserID,
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return ListPaymentLedgerResult{}, err
	}

	totalCount, err := svc.store.CountPaymentLedgerEntriesByUser(ctx, input.UserID)
	if err != nil {
		return ListPaymentLedgerResult{}, err
	}

	entries := make([]PaymentLedgerEntry, 0, len(rows))
	for _, row := range rows {
		entry := PaymentLedgerEntry{
			ID:             row.ID,
			EntryType:      row.EntryType,
			PaymentOrderID: row.PaymentOrderID,
			BusinessType:   row.BusinessType,
			Amount:         row.Amount,
			Status:         row.Status,
			OccurredAt:     row.OccurredAt,
			CreatedAt:      row.CreatedAt,
		}
		if row.RefundOrderID.Valid {
			entry.RefundOrderID = &row.RefundOrderID.Int64
		}
		if row.OrderID.Valid {
			entry.OrderID = &row.OrderID.Int64
		}
		entries = append(entries, entry)
	}

	return ListPaymentLedgerResult{Entries: entries, TotalCount: totalCount}, nil
}
