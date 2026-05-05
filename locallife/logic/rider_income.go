package logic

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	DefaultRiderIncomePageSize int32 = 20
	MaxRiderIncomePageSize     int32 = 50
)

type RiderIncomeStore interface {
	GetRiderByUserID(ctx context.Context, userID int64) (db.Rider, error)
	GetRiderProfitSharingStats(ctx context.Context, arg db.GetRiderProfitSharingStatsParams) (db.GetRiderProfitSharingStatsRow, error)
	GetRiderProfitSharingStatusSummary(ctx context.Context, arg db.GetRiderProfitSharingStatusSummaryParams) ([]db.GetRiderProfitSharingStatusSummaryRow, error)
	CountRiderProfitSharingOrders(ctx context.Context, arg db.CountRiderProfitSharingOrdersParams) (int64, error)
	ListRiderProfitSharingOrders(ctx context.Context, arg db.ListRiderProfitSharingOrdersParams) ([]db.ListRiderProfitSharingOrdersRow, error)
	GetRiderDailyIncome(ctx context.Context, arg db.GetRiderDailyIncomeParams) ([]db.GetRiderDailyIncomeRow, error)
}

type RiderIncomeService struct {
	store RiderIncomeStore
}

func NewRiderIncomeService(store RiderIncomeStore) *RiderIncomeService {
	return &RiderIncomeService{store: store}
}

type RiderIncomeSummary struct {
	TotalDeliveries       int64
	TotalRiderIncome      int64
	TotalDeliveryFee      int64
	TotalRiderGrossAmount int64
	TotalRiderPaymentFee  int64
	StatusSummary         []RiderIncomeStatusSummary
}

type RiderIncomeStatusSummary struct {
	Status           string
	OrderCount       int64
	RiderAmount      int64
	DeliveryFee      int64
	RiderGrossAmount int64
	RiderPaymentFee  int64
}

type RiderIncomeLedger struct {
	Items    []RiderIncomeLedgerItem
	Total    int64
	PageID   int32
	PageSize int32
	HasMore  bool
}

type RiderIncomeLedgerItem struct {
	ID                  int64
	PaymentOrderID      int64
	MerchantID          int64
	OrderID             int64
	OrderNo             string
	MerchantName        string
	Status              string
	TotalAmount         int64
	DeliveryFee         int64
	RiderGrossAmount    int64
	RiderPaymentFee     int64
	RiderAmount         int64
	DistributableAmount int64
	OutOrderNo          string
	SharingOrderID      string
	FinishedAt          *time.Time
	CreatedAt           time.Time
}

type RiderIncomeDailyItem struct {
	Date             time.Time
	DeliveryCount    int64
	DailyIncome      int64
	RiderGrossAmount int64
	RiderPaymentFee  int64
}

func (service *RiderIncomeService) GetSummary(ctx context.Context, userID int64, startAt, endAt time.Time) (RiderIncomeSummary, error) {
	rider, err := service.riderByUserID(ctx, userID)
	if err != nil {
		return RiderIncomeSummary{}, err
	}

	riderID := pgtype.Int8{Int64: rider.ID, Valid: true}
	stats, err := service.store.GetRiderProfitSharingStats(ctx, db.GetRiderProfitSharingStatsParams{
		RiderID: riderID,
		StartAt: startAt,
		EndAt:   endAt,
	})
	if err != nil {
		return RiderIncomeSummary{}, err
	}

	statusRows, err := service.store.GetRiderProfitSharingStatusSummary(ctx, db.GetRiderProfitSharingStatusSummaryParams{
		RiderID: riderID,
		StartAt: startAt,
		EndAt:   endAt,
	})
	if err != nil {
		return RiderIncomeSummary{}, err
	}

	return RiderIncomeSummary{
		TotalDeliveries:       stats.TotalDeliveries,
		TotalRiderIncome:      stats.TotalRiderIncome,
		TotalDeliveryFee:      stats.TotalDeliveryFee,
		TotalRiderGrossAmount: stats.TotalRiderGrossAmount,
		TotalRiderPaymentFee:  stats.TotalRiderPaymentFee,
		StatusSummary:         completeRiderIncomeStatusSummary(statusRows),
	}, nil
}

func (service *RiderIncomeService) ListLedger(ctx context.Context, userID int64, status string, startAt, endAt time.Time, pageID, pageSize int32) (RiderIncomeLedger, error) {
	normalizedStatus, err := NormalizeRiderIncomeStatus(status)
	if err != nil {
		return RiderIncomeLedger{}, err
	}

	rider, err := service.riderByUserID(ctx, userID)
	if err != nil {
		return RiderIncomeLedger{}, err
	}

	pageID, pageSize = NormalizeRiderIncomePage(pageID, pageSize)
	offset := (pageID - 1) * pageSize
	riderID := pgtype.Int8{Int64: rider.ID, Valid: true}
	statusArg := riderIncomeStatusArg(normalizedStatus)

	total, err := service.store.CountRiderProfitSharingOrders(ctx, db.CountRiderProfitSharingOrdersParams{
		RiderID: riderID,
		Status:  statusArg,
		StartAt: startAt,
		EndAt:   endAt,
	})
	if err != nil {
		return RiderIncomeLedger{}, err
	}

	rows, err := service.store.ListRiderProfitSharingOrders(ctx, db.ListRiderProfitSharingOrdersParams{
		RiderID: riderID,
		Status:  statusArg,
		StartAt: startAt,
		EndAt:   endAt,
		Offset:  offset,
		Limit:   pageSize,
	})
	if err != nil {
		return RiderIncomeLedger{}, err
	}

	items := make([]RiderIncomeLedgerItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, newRiderIncomeLedgerItem(row))
	}

	return RiderIncomeLedger{
		Items:    items,
		Total:    total,
		PageID:   pageID,
		PageSize: pageSize,
		HasMore:  int64(offset)+int64(len(items)) < total,
	}, nil
}

func (service *RiderIncomeService) GetDaily(ctx context.Context, userID int64, startAt, endAt time.Time) ([]RiderIncomeDailyItem, error) {
	rider, err := service.riderByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	rows, err := service.store.GetRiderDailyIncome(ctx, db.GetRiderDailyIncomeParams{
		RiderID: pgtype.Int8{Int64: rider.ID, Valid: true},
		StartAt: startAt,
		EndAt:   endAt,
	})
	if err != nil {
		return nil, err
	}

	items := make([]RiderIncomeDailyItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, RiderIncomeDailyItem{
			Date:             row.Date.Time,
			DeliveryCount:    row.DeliveryCount,
			DailyIncome:      row.DailyIncome,
			RiderGrossAmount: row.RiderGrossAmount,
			RiderPaymentFee:  row.RiderPaymentFee,
		})
	}
	return items, nil
}

func NormalizeRiderIncomeStatus(status string) (string, error) {
	normalizedStatus := strings.TrimSpace(status)
	if normalizedStatus == "" {
		return "", nil
	}
	switch normalizedStatus {
	case db.ProfitSharingOrderStatusPending,
		db.ProfitSharingOrderStatusProcessing,
		db.ProfitSharingOrderStatusFinished,
		db.ProfitSharingOrderStatusFailed:
		return normalizedStatus, nil
	default:
		return "", NewRequestError(http.StatusBadRequest, errors.New("invalid status"))
	}
}

func NormalizeRiderIncomePage(pageID, pageSize int32) (int32, int32) {
	if pageID <= 0 {
		pageID = 1
	}
	if pageSize <= 0 {
		pageSize = DefaultRiderIncomePageSize
	}
	if pageSize > MaxRiderIncomePageSize {
		pageSize = MaxRiderIncomePageSize
	}
	return pageID, pageSize
}

func (service *RiderIncomeService) riderByUserID(ctx context.Context, userID int64) (db.Rider, error) {
	rider, err := service.store.GetRiderByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.Rider{}, NewRequestError(http.StatusNotFound, errors.New("rider not found"))
		}
		return db.Rider{}, err
	}
	return rider, nil
}

func riderIncomeStatusArg(status string) pgtype.Text {
	if status == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: status, Valid: true}
}

func completeRiderIncomeStatusSummary(rows []db.GetRiderProfitSharingStatusSummaryRow) []RiderIncomeStatusSummary {
	statuses := []string{
		db.ProfitSharingOrderStatusPending,
		db.ProfitSharingOrderStatusProcessing,
		db.ProfitSharingOrderStatusFinished,
		db.ProfitSharingOrderStatusFailed,
	}
	byStatus := make(map[string]RiderIncomeStatusSummary, len(rows))
	for _, row := range rows {
		byStatus[row.Status] = RiderIncomeStatusSummary{
			Status:           row.Status,
			OrderCount:       row.OrderCount,
			RiderAmount:      row.RiderAmount,
			DeliveryFee:      row.DeliveryFee,
			RiderGrossAmount: row.RiderGrossAmount,
			RiderPaymentFee:  row.RiderPaymentFee,
		}
	}

	items := make([]RiderIncomeStatusSummary, 0, len(statuses))
	for _, status := range statuses {
		item, ok := byStatus[status]
		if !ok {
			item = RiderIncomeStatusSummary{Status: status}
		}
		items = append(items, item)
	}
	return items
}

func newRiderIncomeLedgerItem(row db.ListRiderProfitSharingOrdersRow) RiderIncomeLedgerItem {
	return RiderIncomeLedgerItem{
		ID:                  row.ID,
		PaymentOrderID:      row.PaymentOrderID,
		MerchantID:          row.MerchantID,
		OrderID:             row.OrderID.Int64,
		OrderNo:             row.OrderNo,
		MerchantName:        row.MerchantName,
		Status:              row.Status,
		TotalAmount:         row.TotalAmount,
		DeliveryFee:         row.DeliveryFee,
		RiderGrossAmount:    row.RiderGrossAmount,
		RiderPaymentFee:     row.RiderPaymentFee,
		RiderAmount:         row.RiderAmount,
		DistributableAmount: row.DistributableAmount,
		OutOrderNo:          row.OutOrderNo,
		SharingOrderID:      pgTextString(row.SharingOrderID),
		FinishedAt:          pgTimestampPtr(row.FinishedAt),
		CreatedAt:           row.CreatedAt,
	}
}

func pgTextString(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func pgTimestampPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	timestamp := value.Time
	return &timestamp
}
