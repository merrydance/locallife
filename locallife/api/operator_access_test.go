package api

import (
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"go.uber.org/mock/gomock"
)

func expectActiveOperatorAuth(store *mockdb.MockStore, userID int64, operator db.Operator) {
	store.EXPECT().
		ListUserRoles(gomock.Any(), userID).
		AnyTimes().
		Return([]db.UserRole{{
			UserID:          userID,
			Role:            RoleOperator,
			Status:          "active",
			RelatedEntityID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
		}}, nil)
	store.EXPECT().
		GetOperatorByUser(gomock.Any(), userID).
		AnyTimes().
		Return(operator, nil)
}

func expectOperatorManagesRegion(store *mockdb.MockStore, operator db.Operator, regionID int64, manages bool) {
	store.EXPECT().
		CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
			OperatorID: operator.ID,
			RegionID:   regionID,
		}).
		AnyTimes().
		Return(manages, nil)
}

func expectOperatorManagedRegions(store *mockdb.MockStore, operator db.Operator, regionIDs ...int64) {
	rows := make([]db.ListOperatorRegionsRow, 0, len(regionIDs))
	for _, regionID := range regionIDs {
		rows = append(rows, db.ListOperatorRegionsRow{
			OperatorID: operator.ID,
			RegionID:   regionID,
			Status:     "active",
		})
	}

	store.EXPECT().
		ListOperatorRegions(gomock.Any(), operator.ID).
		AnyTimes().
		Return(rows, nil)
}
