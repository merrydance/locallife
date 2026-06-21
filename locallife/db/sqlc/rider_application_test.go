package db

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

func createRandomRiderApplication(t *testing.T) RiderApplication {
	user := createRandomUser(t)
	return createRandomRiderApplicationWithUser(t, user.ID)
}

func createRandomRiderApplicationWithUser(t *testing.T, userID int64) RiderApplication {
	app, err := testStore.CreateRiderApplication(context.Background(), userID)
	require.NoError(t, err)
	require.NotEmpty(t, app)

	require.Equal(t, userID, app.UserID)
	require.Equal(t, "draft", app.Status)
	require.NotZero(t, app.ID)
	require.NotZero(t, app.CreatedAt)

	return app
}

func createRiderApplicationMediaAsset(t *testing.T, userID int64, category string) MediaAsset {
	now := time.Now().UnixNano()
	asset, err := testStore.CreateMediaAsset(context.Background(), CreateMediaAssetParams{
		ObjectKey:      fmt.Sprintf("test/rider-application/%d/%s-%d.jpg", userID, category, now),
		Visibility:     "private",
		MediaCategory:  category,
		MimeType:       "image/jpeg",
		FileSize:       1024,
		ChecksumSha256: fmt.Sprintf("%064d", now),
		UploadedBy:     userID,
		SourceClient:   "test",
	})
	require.NoError(t, err)
	return asset
}

// ==================== Create Tests ====================

func TestCreateRiderApplication(t *testing.T) {
	createRandomRiderApplication(t)
}

func TestCreateRiderApplication_DuplicateUserID(t *testing.T) {
	user := createRandomUser(t)

	// 第一次创建
	createRandomRiderApplicationWithUser(t, user.ID)

	// 同一用户重复创建应该失败（唯一约束：user_id）
	_, err := testStore.CreateRiderApplication(context.Background(), user.ID)
	require.Error(t, err, "同一用户重复创建申请应该返回错误")
	require.Equal(t, UniqueViolation, ErrorCode(err))
}

// ==================== Get Tests ====================

func TestGetRiderApplication(t *testing.T) {
	app1 := createRandomRiderApplication(t)

	app2, err := testStore.GetRiderApplication(context.Background(), app1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, app2)

	require.Equal(t, app1.ID, app2.ID)
	require.Equal(t, app1.UserID, app2.UserID)
	require.Equal(t, app1.Status, app2.Status)
	require.WithinDuration(t, app1.CreatedAt, app2.CreatedAt, time.Second)
}

func TestGetRiderApplicationByUserID(t *testing.T) {
	user := createRandomUser(t)
	app1 := createRandomRiderApplicationWithUser(t, user.ID)

	app2, err := testStore.GetRiderApplicationByUserID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, app2)

	require.Equal(t, app1.ID, app2.ID)
	require.Equal(t, app1.UserID, app2.UserID)
}

// ==================== Update Basic Info Tests ====================

func TestUpdateRiderApplicationBasicInfo(t *testing.T) {
	app := createRandomRiderApplication(t)

	realName := "张三"
	phone := "13812345678"

	updated, err := testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: realName, Valid: true},
		Phone:    pgtype.Text{String: phone, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, realName, updated.RealName.String)
	require.Equal(t, phone, updated.Phone.String)
	require.True(t, updated.UpdatedAt.Valid)
}

func TestUpdateRiderApplicationBasicInfo_OnlyDraft(t *testing.T) {
	app := createRandomRiderApplication(t)

	// 先提交申请
	// 需要先填充必填字段
	_, err := testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{},
		IDCardBackMediaAssetID:  pgtype.Int8{},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{},
	})
	require.NoError(t, err)

	// 提交
	submitted, err := testStore.SubmitRiderApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", submitted.Status)

	// 已提交状态不能更新
	_, err = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "李四", Valid: true},
	})
	require.Error(t, err, "已提交的申请不能修改")
}

// ==================== Update ID Card Tests ====================

func TestUpdateRiderApplicationIDCard(t *testing.T) {
	app := createRandomRiderApplication(t)

	ocrData := map[string]string{
		"name":      "张三",
		"id_number": "110101199001011234",
		"gender":    "男",
		"nation":    "汉",
		"valid_end": "20300101",
	}
	ocrJSON, _ := json.Marshal(ocrData)

	updated, err := testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{},
		IDCardBackMediaAssetID:  pgtype.Int8{},
		IDCardOcr:               ocrJSON,
		RealName:                pgtype.Text{String: "张三", Valid: true},
	})
	require.NoError(t, err)
	require.False(t, updated.IDCardFrontMediaAssetID.Valid)
	require.False(t, updated.IDCardBackMediaAssetID.Valid)
	require.NotEmpty(t, updated.IDCardOcr)
	require.Equal(t, "张三", updated.RealName.String)
}

func TestUpdateRiderApplicationIDCard_MergesOCRPayload(t *testing.T) {
	app := createRandomRiderApplication(t)

	frontOCR, err := json.Marshal(map[string]string{
		"name":      "张三",
		"id_number": "110101199001011234",
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:        app.ID,
		IDCardOcr: frontOCR,
		RealName:  pgtype.Text{String: "张三", Valid: true},
	})
	require.NoError(t, err)

	backOCR, err := json.Marshal(map[string]string{
		"valid_end": "2035.01.01",
	})
	require.NoError(t, err)

	updated, err := testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:        app.ID,
		IDCardOcr: backOCR,
	})
	require.NoError(t, err)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(updated.IDCardOcr, &payload))
	require.Equal(t, "张三", payload["name"])
	require.Equal(t, "110101199001011234", payload["id_number"])
	require.Equal(t, "2035.01.01", payload["valid_end"])
	require.Equal(t, "张三", updated.RealName.String)
}

// ==================== Update Health Cert Tests ====================

func TestUpdateRiderApplicationHealthCert(t *testing.T) {
	app := createRandomRiderApplication(t)

	updated, err := testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{},
	})
	require.NoError(t, err)
	require.False(t, updated.HealthCertMediaAssetID.Valid)
}

func TestUpdateRiderApplicationHealthCert_ClearsStaleReviewFailure(t *testing.T) {
	app := createRandomRiderApplication(t)
	idCardFrontAsset := createRiderApplicationMediaAsset(t, app.UserID, "id_card_front")
	idCardBackAsset := createRiderApplicationMediaAsset(t, app.UserID, "id_card_back")
	oldHealthCertAsset := createRiderApplicationMediaAsset(t, app.UserID, "health_cert")
	newHealthCertAsset := createRiderApplicationMediaAsset(t, app.UserID, "health_cert")
	_, err := testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "周松涛", Valid: true},
		Phone:    pgtype.Text{String: "15833712098", Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{Int64: idCardFrontAsset.ID, Valid: true},
		IDCardBackMediaAssetID:  pgtype.Int8{Int64: idCardBackAsset.ID, Valid: true},
		IDCardOcr:               []byte(`{"name":"周松涛","id_number":"132229197706017792","valid_end":"2035.03.01"}`),
	})
	require.NoError(t, err)
	_, err = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{Int64: oldHealthCertAsset.ID, Valid: true},
		HealthCertOcr:          []byte(`{"name":"错误姓名","valid_end":"2026.12.06"}`),
	})
	require.NoError(t, err)
	_, err = testStore.SubmitRiderApplication(context.Background(), app.ID)
	require.NoError(t, err)
	_, err = testStore.ReturnRiderApplicationToDraft(context.Background(), ReturnRiderApplicationToDraftParams{
		ID:           app.ID,
		RejectReason: pgtype.Text{String: "健康证姓名与身份证姓名不一致", Valid: true},
	})
	require.NoError(t, err)

	updated, err := testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{Int64: newHealthCertAsset.ID, Valid: true},
		HealthCertOcr:          []byte(`{"name":"周松涛","valid_end":"2026.12.06"}`),
	})
	require.NoError(t, err)
	require.False(t, updated.RejectReason.Valid)
	require.False(t, updated.ReviewedAt.Valid)
	require.Len(t, updated.ReviewSummary, 0)
}

func TestClearRiderApplicationHealthCert_ClearsStaleReviewFailure(t *testing.T) {
	app := createRandomRiderApplication(t)
	idCardFrontAsset := createRiderApplicationMediaAsset(t, app.UserID, "id_card_front")
	idCardBackAsset := createRiderApplicationMediaAsset(t, app.UserID, "id_card_back")
	healthCertAsset := createRiderApplicationMediaAsset(t, app.UserID, "health_cert")
	_, err := testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "周松涛", Valid: true},
		Phone:    pgtype.Text{String: "15833712098", Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{Int64: idCardFrontAsset.ID, Valid: true},
		IDCardBackMediaAssetID:  pgtype.Int8{Int64: idCardBackAsset.ID, Valid: true},
		IDCardOcr:               []byte(`{"name":"周松涛","id_number":"132229197706017792","valid_end":"2035.03.01"}`),
	})
	require.NoError(t, err)
	_, err = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{Int64: healthCertAsset.ID, Valid: true},
		HealthCertOcr:          []byte(`{"name":"错误姓名","valid_end":"2026.12.06"}`),
	})
	require.NoError(t, err)
	_, err = testStore.SubmitRiderApplication(context.Background(), app.ID)
	require.NoError(t, err)
	_, err = testStore.ReturnRiderApplicationToDraft(context.Background(), ReturnRiderApplicationToDraftParams{
		ID:           app.ID,
		RejectReason: pgtype.Text{String: "健康证姓名与身份证姓名不一致", Valid: true},
	})
	require.NoError(t, err)

	updated, err := testStore.ClearRiderApplicationHealthCert(context.Background(), app.ID)
	require.NoError(t, err)
	require.False(t, updated.HealthCertMediaAssetID.Valid)
	require.Len(t, updated.HealthCertOcr, 0)
	require.False(t, updated.RejectReason.Valid)
	require.False(t, updated.ReviewedAt.Valid)
	require.Len(t, updated.ReviewSummary, 0)
}

// ==================== Submit Tests ====================

func TestSubmitRiderApplication(t *testing.T) {
	app := createRandomRiderApplication(t)

	// 填充必填字段
	_, _ = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{},
		IDCardBackMediaAssetID:  pgtype.Int8{},
	})
	_, _ = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{},
	})

	// 提交
	submitted, err := testStore.SubmitRiderApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", submitted.Status)
	require.True(t, submitted.SubmittedAt.Valid)
}

// ==================== Approve Tests ====================

func TestApproveRiderApplication(t *testing.T) {
	app := createRandomRiderApplication(t)

	// 填充并提交
	_, _ = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{},
		IDCardBackMediaAssetID:  pgtype.Int8{},
	})
	_, _ = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{},
	})
	_, _ = testStore.SubmitRiderApplication(context.Background(), app.ID)

	// 审核通过
	approved, err := testStore.ApproveRiderApplication(context.Background(), ApproveRiderApplicationParams{
		ID: app.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "approved", approved.Status)
	require.True(t, approved.ReviewedAt.Valid)
}

// ==================== Return-To-Draft Tests ====================

func TestReturnRiderApplicationToDraft(t *testing.T) {
	app := createRandomRiderApplication(t)

	// 填充并提交
	_, _ = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{},
		IDCardBackMediaAssetID:  pgtype.Int8{},
	})
	_, _ = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{},
	})
	_, _ = testStore.SubmitRiderApplication(context.Background(), app.ID)

	// 审核未通过后退回草稿
	rejectReason := "身份证照片不清晰"
	returned, err := testStore.ReturnRiderApplicationToDraft(context.Background(), ReturnRiderApplicationToDraftParams{
		ID:           app.ID,
		RejectReason: pgtype.Text{String: rejectReason, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "draft", returned.Status)
	require.Equal(t, rejectReason, returned.RejectReason.String)
	require.True(t, returned.ReviewedAt.Valid)
	require.False(t, returned.SubmittedAt.Valid)
}

// ==================== Reset Tests ====================

func TestResetRiderApplicationToDraft(t *testing.T) {
	app := createRandomRiderApplication(t)

	// 填充并提交
	_, _ = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{},
		IDCardBackMediaAssetID:  pgtype.Int8{},
	})
	_, _ = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{},
	})
	_, _ = testStore.SubmitRiderApplication(context.Background(), app.ID)

	// 重置为草稿
	reset, err := testStore.ResetRiderApplicationToDraft(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "draft", reset.Status)
	require.False(t, reset.RejectReason.Valid)
	require.False(t, reset.SubmittedAt.Valid)
}

func TestResetSubmittedRiderApplicationToDraft(t *testing.T) {
	app := createRandomRiderApplication(t)

	_, _ = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{},
		IDCardBackMediaAssetID:  pgtype.Int8{},
	})
	_, _ = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{},
	})
	submitted, err := testStore.SubmitRiderApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", submitted.Status)
	require.True(t, submitted.SubmittedAt.Valid)

	reset, err := testStore.ResetRiderApplicationToDraft(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "draft", reset.Status)
	require.False(t, reset.RejectReason.Valid)
	require.False(t, reset.SubmittedAt.Valid)
	require.False(t, reset.ReviewedAt.Valid)
	require.False(t, reset.ReviewedBy.Valid)
}

// ==================== List Tests ====================

func TestListRiderApplications(t *testing.T) {
	// 创建几个申请
	for i := 0; i < 3; i++ {
		createRandomRiderApplication(t)
	}

	apps, err := testStore.ListRiderApplications(context.Background(), ListRiderApplicationsParams{
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(apps), 3)
}

func TestListRiderApplicationsByStatus(t *testing.T) {
	// 创建一个提交的申请
	app := createRandomRiderApplication(t)
	_, _ = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "测试", Valid: true},
		Phone:    pgtype.Text{String: "13900000000", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:                      app.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{},
		IDCardBackMediaAssetID:  pgtype.Int8{},
	})
	_, _ = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:                     app.ID,
		HealthCertMediaAssetID: pgtype.Int8{},
	})
	_, _ = testStore.SubmitRiderApplication(context.Background(), app.ID)

	// 按状态筛选
	apps, err := testStore.ListRiderApplications(context.Background(), ListRiderApplicationsParams{
		Status: pgtype.Text{String: "submitted", Valid: true},
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(apps), 1)

	for _, a := range apps {
		require.Equal(t, "submitted", a.Status)
	}
}

// ==================== Count Tests ====================

func TestCountRiderApplicationsByStatus(t *testing.T) {
	createRandomRiderApplication(t) // draft

	count, err := testStore.CountRiderApplicationsByStatus(context.Background(), "draft")
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))
}
