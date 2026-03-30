package api

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const riderOperationalStatusSyncPageSize int32 = 200

func (server *Server) syncRiderOperationalStatusesByRegion(ctx context.Context, regionID int64) error {
	var offset int32

	for {
		riders, err := server.store.ListRidersByRegion(ctx, db.ListRidersByRegionParams{
			RegionID: pgtype.Int8{Int64: regionID, Valid: regionID > 0},
			Limit:    riderOperationalStatusSyncPageSize,
			Offset:   offset,
		})
		if err != nil {
			return fmt.Errorf("list riders by region %d: %w", regionID, err)
		}

		for _, rider := range riders {
			if _, err := db.ReconcileRiderOperationalStatus(ctx, server.store, rider); err != nil {
				return fmt.Errorf("reconcile rider %d in region %d: %w", rider.ID, regionID, err)
			}
		}

		if len(riders) < int(riderOperationalStatusSyncPageSize) {
			return nil
		}

		offset += int32(len(riders))
	}
}

func (server *Server) collectRidersByStatus(ctx context.Context, status string) ([]db.Rider, error) {
	var (
		offset int32
		all    []db.Rider
	)

	for {
		riders, err := server.store.ListRidersByStatus(ctx, db.ListRidersByStatusParams{
			Status: status,
			Limit:  riderOperationalStatusSyncPageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, fmt.Errorf("list riders by status %s: %w", status, err)
		}

		all = append(all, riders...)
		if len(riders) < int(riderOperationalStatusSyncPageSize) {
			return all, nil
		}

		offset += int32(len(riders))
	}
}

func (server *Server) syncAllRiderOperationalStatuses(ctx context.Context) error {
	approvedRiders, err := server.collectRidersByStatus(ctx, db.RiderStatusApproved)
	if err != nil {
		return err
	}

	activeRiders, err := server.collectRidersByStatus(ctx, db.RiderStatusActive)
	if err != nil {
		return err
	}

	for _, rider := range append(approvedRiders, activeRiders...) {
		if _, err := db.ReconcileRiderOperationalStatus(ctx, server.store, rider); err != nil {
			return fmt.Errorf("reconcile rider %d: %w", rider.ID, err)
		}
	}

	return nil
}
