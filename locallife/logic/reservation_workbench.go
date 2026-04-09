package logic

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type MerchantReservationWorkbenchInput struct {
	OperatorUserID  int64
	ReservationDate time.Time
}

type MerchantReservationWorkbenchSummary struct {
	ReservationCount int64
	ActiveTableCount int64
}

type MerchantReservationWorkbenchStatusTotals struct {
	All       int64
	Pending   int64
	Paid      int64
	Confirmed int64
	CheckedIn int64
	Completed int64
	Cancelled int64
	Expired   int64
	NoShow    int64
	Exception int64
}

type MerchantReservationPrepReference struct {
	ReservationID   int64
	ReservationTime string
	TableNo         string
	ContactName     string
	Status          string
	Quantity        int16
}

type MerchantReservationPrepItem struct {
	Type             string
	DishID           *int64
	ComboID          *int64
	Name             string
	TotalQuantity    int64
	ReservationCount int64
	References       []MerchantReservationPrepReference
}

type MerchantReservationPrepSummary struct {
	TableCount    int64
	DishKinds     int64
	TotalQuantity int64
	Items         []MerchantReservationPrepItem
}

type MerchantReservationWorkbenchResult struct {
	Summary      MerchantReservationWorkbenchSummary
	StatusTotals MerchantReservationWorkbenchStatusTotals
	PrepSummary  MerchantReservationPrepSummary
}

func MerchantReservationWorkbench(
	ctx context.Context,
	store db.Store,
	input MerchantReservationWorkbenchInput,
) (MerchantReservationWorkbenchResult, error) {
	merchant, err := resolveMerchantForUser(ctx, store, input.OperatorUserID)
	if err != nil {
		return MerchantReservationWorkbenchResult{}, err
	}

	reservations, err := store.ListReservationsByMerchantAndDate(ctx, db.ListReservationsByMerchantAndDateParams{
		MerchantID:      merchant.ID,
		ReservationDate: pgtype.Date{Time: input.ReservationDate, Valid: true},
	})
	if err != nil {
		return MerchantReservationWorkbenchResult{}, err
	}

	totals := MerchantReservationWorkbenchStatusTotals{}
	activeTables := map[string]struct{}{}

	type prepAggValue struct {
		item    MerchantReservationPrepItem
		seenRes map[int64]struct{}
	}

	prepAgg := map[string]*prepAggValue{}
	var prepTotalQuantity int64

	for _, reservation := range reservations {
		totals.All++
		switch reservation.Status {
		case reservationStatusPending:
			totals.Pending++
		case reservationStatusPaid:
			totals.Paid++
		case reservationStatusConfirmed:
			totals.Confirmed++
		case reservationStatusCheckedIn:
			totals.CheckedIn++
		case reservationStatusCompleted:
			totals.Completed++
		case reservationStatusCancelled:
			totals.Cancelled++
		case reservationStatusExpired:
			totals.Expired++
		case reservationStatusNoShow:
			totals.NoShow++
		}

		if !isReservationWorkbenchOperationalStatus(reservation.Status) {
			continue
		}

		if reservation.TableNo != "" {
			activeTables[reservation.TableNo] = struct{}{}
		}

		items, err := store.ListReservationItems(ctx, reservation.ID)
		if err != nil {
			return MerchantReservationWorkbenchResult{}, err
		}

		reservationTime := formatReservationWorkbenchTime(reservation.ReservationTime)
		for _, item := range items {
			if !item.DishID.Valid && !item.ComboID.Valid {
				continue
			}

			key, summaryItem := buildPrepSummarySeed(item)
			entry, ok := prepAgg[key]
			if !ok {
				entry = &prepAggValue{
					item:    summaryItem,
					seenRes: map[int64]struct{}{},
				}
				prepAgg[key] = entry
			}

			entry.item.TotalQuantity += int64(item.Quantity)
			prepTotalQuantity += int64(item.Quantity)
			if _, seen := entry.seenRes[reservation.ID]; !seen {
				entry.item.ReservationCount++
				entry.seenRes[reservation.ID] = struct{}{}
			}
			entry.item.References = append(entry.item.References, MerchantReservationPrepReference{
				ReservationID:   reservation.ID,
				ReservationTime: reservationTime,
				TableNo:         reservation.TableNo,
				ContactName:     reservation.ContactName,
				Status:          reservation.Status,
				Quantity:        item.Quantity,
			})
		}
	}

	prepItems := make([]MerchantReservationPrepItem, 0, len(prepAgg))
	for _, value := range prepAgg {
		sort.Slice(value.item.References, func(i, j int) bool {
			return value.item.References[i].ReservationTime < value.item.References[j].ReservationTime
		})
		prepItems = append(prepItems, value.item)
	}

	sort.Slice(prepItems, func(i, j int) bool {
		if prepItems[i].TotalQuantity == prepItems[j].TotalQuantity {
			return prepItems[i].Name < prepItems[j].Name
		}
		return prepItems[i].TotalQuantity > prepItems[j].TotalQuantity
	})

	totals.Exception = totals.Cancelled + totals.Expired + totals.NoShow

	return MerchantReservationWorkbenchResult{
		Summary: MerchantReservationWorkbenchSummary{
			ReservationCount: totals.All,
			ActiveTableCount: int64(len(activeTables)),
		},
		StatusTotals: totals,
		PrepSummary: MerchantReservationPrepSummary{
			TableCount:    int64(len(activeTables)),
			DishKinds:     int64(len(prepItems)),
			TotalQuantity: prepTotalQuantity,
			Items:         prepItems,
		},
	}, nil
}

func isReservationWorkbenchOperationalStatus(status string) bool {
	return status != reservationStatusCancelled && status != reservationStatusExpired && status != reservationStatusNoShow
}

func formatReservationWorkbenchTime(value pgtype.Time) string {
	if !value.Valid {
		return ""
	}
	hours := value.Microseconds / 1000000 / 3600
	minutes := (value.Microseconds / 1000000 % 3600) / 60
	return time.Date(0, 1, 1, int(hours), int(minutes), 0, 0, time.UTC).Format("15:04")
}

func buildPrepSummarySeed(item db.ListReservationItemsRow) (string, MerchantReservationPrepItem) {
	name := "未命名"
	summary := MerchantReservationPrepItem{
		Type:       "dish",
		References: make([]MerchantReservationPrepReference, 0, 8),
	}

	if item.DishID.Valid {
		key := fmt.Sprintf("dish:%d", item.DishID.Int64)
		if item.DishName.Valid {
			name = item.DishName.String
		}
		dishID := item.DishID.Int64
		summary.DishID = &dishID
		summary.Name = name
		return key, summary
	}

	key := fmt.Sprintf("combo:%d", item.ComboID.Int64)
	summary.Type = "combo"
	if item.ComboName.Valid {
		name = item.ComboName.String
	}
	comboID := item.ComboID.Int64
	summary.ComboID = &comboID
	summary.Name = name
	return key, summary
}
