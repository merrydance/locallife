package worker

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestShouldDispatchOrderProfitSharing(t *testing.T) {
	tests := []struct {
		name  string
		order db.Order
		want  bool
	}{
		{
			name: "takeout waits for settlement callback",
			order: db.Order{
				OrderType: "takeout",
			},
			want: false,
		},
		{
			name: "dine in ordinary order skips profit sharing",
			order: db.Order{
				OrderType: "dine_in",
			},
			want: false,
		},
		{
			name: "takeaway ordinary order skips profit sharing",
			order: db.Order{
				OrderType: "takeaway",
			},
			want: false,
		},
		{
			name: "reservation linked dine in still dispatches profit sharing",
			order: db.Order{
				OrderType:     "dine_in",
				ReservationID: pgtype.Int8{Int64: 9, Valid: true},
			},
			want: true,
		},
		{
			name: "other order types dispatch profit sharing",
			order: db.Order{
				OrderType: "delivery",
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldDispatchOrderProfitSharing(tc.order))
		})
	}
}