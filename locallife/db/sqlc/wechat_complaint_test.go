package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestListWechatComplaintsUseIDTieBreaker(t *testing.T) {
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	subMchID := "sub_" + util.RandomString(12)
	tiedComplaintTime := time.Now().UTC().Truncate(time.Microsecond)

	createComplaint := func(suffix string) WechatComplaint {
		complaint, err := testStore.UpsertWechatComplaint(context.Background(), UpsertWechatComplaintParams{
			ComplaintID:            "complaint_" + suffix + "_" + util.RandomString(8),
			ComplaintTime:          tiedComplaintTime,
			PayerOpenid:            pgtype.Text{String: "openid_" + suffix, Valid: true},
			ComplaintDetail:        "detail_" + suffix,
			ComplaintState:         "PENDING_RESPONSE",
			TransactionID:          pgtype.Text{String: "tx_" + suffix, Valid: true},
			OutTradeNo:             pgtype.Text{String: "out_" + suffix, Valid: true},
			SubMchID:               pgtype.Text{String: subMchID, Valid: true},
			MerchantID:             pgtype.Int8{Int64: merchant.ID, Valid: true},
			PayerComplaintFullInfo: false,
			Amount:                 1000,
			WxpayUpdateTime:        pgtype.Timestamptz{Time: tiedComplaintTime, Valid: true},
		})
		require.NoError(t, err)
		return complaint
	}

	firstComplaint := createComplaint("1")
	secondComplaint := createComplaint("2")

	byMerchant, err := testStore.ListWechatComplaintsByMerchant(context.Background(), ListWechatComplaintsByMerchantParams{
		MerchantID: pgtype.Int8{Int64: merchant.ID, Valid: true},
		Column2:    "",
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, byMerchant, 2)
	require.Equal(t, secondComplaint.ID, byMerchant[0].ID)
	require.Equal(t, firstComplaint.ID, byMerchant[1].ID)

	bySubMchID, err := testStore.ListWechatComplaintsBySubMchID(context.Background(), ListWechatComplaintsBySubMchIDParams{
		SubMchID: pgtype.Text{String: subMchID, Valid: true},
		Column2:  "",
		Limit:    2,
		Offset:   0,
	})
	require.NoError(t, err)
	require.Len(t, bySubMchID, 2)
	require.Equal(t, secondComplaint.ID, bySubMchID[0].ID)
	require.Equal(t, firstComplaint.ID, bySubMchID[1].ID)

	count, err := testStore.CountWechatComplaintsByMerchant(context.Background(), CountWechatComplaintsByMerchantParams{
		MerchantID: pgtype.Int8{Int64: merchant.ID, Valid: true},
		Column2:    "",
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}
