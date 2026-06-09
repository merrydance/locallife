package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestListMerchantGroupsUsesIDTieBreaker(t *testing.T) {
	owner := createRandomUser(t)
	region := createRandomRegion(t)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)
	keyword := "group_" + util.RandomString(8)

	createGroup := func(name string) MerchantGroup {
		group, err := testStore.CreateMerchantGroup(context.Background(), CreateMerchantGroupParams{
			Name:                name,
			OwnerUserID:         owner.ID,
			ContactPhone:        pgtype.Text{String: "13800000000", Valid: true},
			LicenseNumber:       pgtype.Text{String: "LIC-" + util.RandomString(8), Valid: true},
			LicenseMediaAssetID: pgtype.Int8{Valid: false},
			Address:             pgtype.Text{String: "测试地址", Valid: true},
			RegionID:            pgtype.Int8{Int64: region.ID, Valid: true},
			ApplicationData:     []byte(`{"source":"test"}`),
		})
		require.NoError(t, err)
		return group
	}

	firstGroup := createGroup(keyword + "_a")
	secondGroup := createGroup(keyword + "_b")

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE merchant_groups SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{firstGroup.ID, secondGroup.ID},
	)
	require.NoError(t, err)

	rows, err := testStore.ListMerchantGroups(context.Background(), ListMerchantGroupsParams{
		Column1: keyword,
		Limit:   2,
		Offset:  0,
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, secondGroup.ID, rows[0].ID)
	require.Equal(t, firstGroup.ID, rows[1].ID)
}

func TestUpdateGroupApplicationLicenseMergesApplicationDataPatch(t *testing.T) {
	user := createRandomUser(t)
	app, err := testStore.CreateGroupApplicationDraft(context.Background(), user.ID)
	require.NoError(t, err)

	app, err = testStore.UpdateGroupApplicationLicense(context.Background(), UpdateGroupApplicationLicenseParams{
		ID: app.ID,
		ApplicationData: []byte(`{
			"business_license_ocr": {
				"status": "done",
				"ocr_job_id": 501,
				"credit_code": "91310000123456789A"
			}
		}`),
	})
	require.NoError(t, err)

	app, err = testStore.UpdateGroupApplicationLicense(context.Background(), UpdateGroupApplicationLicenseParams{
		ID: app.ID,
		ApplicationData: []byte(`{
			"id_card_front_asset_id": 233,
			"id_card_front_ocr": {
				"status": "done",
				"ocr_job_id": 502,
				"name": "张三",
				"id_number": "110101199001011234"
			}
		}`),
	})
	require.NoError(t, err)

	var merged map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(app.ApplicationData, &merged))
	require.Contains(t, merged, "business_license_ocr")
	require.Contains(t, merged, "id_card_front_asset_id")
	require.Contains(t, merged, "id_card_front_ocr")

	var businessLicense struct {
		Status     string `json:"status"`
		OCRJobID   int64  `json:"ocr_job_id"`
		CreditCode string `json:"credit_code"`
	}
	require.NoError(t, json.Unmarshal(merged["business_license_ocr"], &businessLicense))
	require.Equal(t, "done", businessLicense.Status)
	require.Equal(t, int64(501), businessLicense.OCRJobID)
	require.Equal(t, "91310000123456789A", businessLicense.CreditCode)

	var idCardAssetID int64
	require.NoError(t, json.Unmarshal(merged["id_card_front_asset_id"], &idCardAssetID))
	require.Equal(t, int64(233), idCardAssetID)
}
